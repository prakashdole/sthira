package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"sthira/backend/internal/contracts"
)

// Transcribe is the /api/v3/voice/transcriptions entry point: ASR
// only, no middle model, no validator. The request body is the raw
// bounded audio bytes; the orchestrator reuses the ASR admission
// queue and per-stage deadline plumbing. The response is a typed
// contracts.TranscriptionResponse.
//
// The orchestrator does NOT store the transcript server-side; the
// caller (the client, the pipeline) owns persistence. It does NOT
// log audio bytes; the metrics sink is low-cardinality.
func (o *Orchestrator) Transcribe(ctx context.Context, req contracts.TranscriptionRequest, raw []byte) (contracts.TranscriptionResponse, error) {
	if !contracts.IsSupportedTranscriptionContentType(req.ContentType) {
		return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineUnsupported, 415, StageFailure{
			Stage: StageASR, Code: contracts.ErrUnsupportedMedia, Reason: "audio content type not allowed", Retryable: false,
		})
	}
	if req.Language == "" {
		return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
			Stage: StageASR, Code: contracts.ErrInvalidValue, Reason: "language is required", Retryable: false,
		})
	}
	if int64(len(raw)) > o.cfg.Limits.MaxAudioCompressedBytes {
		return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineUnsupported, 413, StageFailure{
			Stage: StageASR, Code: contracts.ErrBodyTooLarge, Reason: "audio exceeds compressed size limit", Retryable: false,
		})
	}
	// Per-stage deadline.
	asrCtx, cancel := StageDeadline(ctx, o.cfg.Limits.ASRDeadline)
	defer cancel()
	release, err := o.asrQueue.Acquire(asrCtx)
	if err != nil {
		if errors.Is(err, ErrQueueSaturated) {
			o.cfg.Metrics.ObserveQueueReject(StageASR)
			return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
				Stage: StageASR, Code: contracts.ErrQueueSaturated, Reason: "asr queue saturated", Retryable: true,
			})
		}
		return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageASR, Code: contracts.ErrInferenceCancelled, Reason: err.Error(), Retryable: false,
		})
	}
	defer release()
	if !o.cfg.Workers.IsReady(StageASR) {
		return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageASR, Code: contracts.ErrModelUnavailable, Reason: "asr worker not ready", Retryable: true,
		})
	}
	id := CorrelationID(req.RequestID)
	if id == "" {
		id = newCorrelationID()
	}
	gen := o.cfg.Correlation.Claim(id, StageASR)
	o.cfg.Correlation.MarkInflight(id, StageASR, gen)
	b64 := base64.StdEncoding.EncodeToString(raw)
	resp, err := o.cfg.Workers.ASR.Transcribe(asrCtx, contracts.ASRWorkerRequest{
		RequestID:      string(id),
		Language:       req.Language,
		ContentType:    req.ContentType,
		AudioB64:       b64,
		ByteSize:       int64(len(raw)),
		DeadlineMillis: o.cfg.Limits.ASRDeadline.Milliseconds(),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineModelUnavailable, 504, StageFailure{
				Stage: StageASR, Code: contracts.ErrModelTimeout, Reason: "asr deadline exceeded", Retryable: true,
			})
		}
		return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageASR, Code: contracts.ErrModelUnavailable, Reason: err.Error(), Retryable: true,
		})
	}
	if !ValidateWorkerCorrelation(string(id), gen, o.cfg.Correlation, StageASR) {
		o.cfg.Metrics.ObserveStaleDrop(StageASR)
		return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageASR, Code: contracts.ErrInferenceCancelled, Reason: "asr response stale", Retryable: false,
		})
	}
	if resp.State != contracts.TranscriptionOK {
		return contracts.TranscriptionResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageASR, Code: mapASRStateToCode(resp.State), Reason: string(resp.State), Retryable: true,
		})
	}
	return contracts.TranscriptionResponse{
		RequestID:      string(id),
		DataVersion:    resp.RequestID, // the worker echoes its own request id; we use the typed envelope's request_id
		Language:       resp.Language,
		Text:           resp.Text,
		Confidence:     resp.Confidence,
		Alternatives:   resp.Alternatives,
		State:          resp.State,
		ModelRevision:  resp.ModelRevision,
		ArtifactDigest: resp.ArtifactDigest,
	}, nil
}

// Synthesize is the /api/v3/voice/speech entry point: TTS only, with
// template-key validation against the current scoped context. The
// worker receives only Go-approved text (the rendered template),
// never arbitrary user text.
//
// The source_version is matched against the current scoped context's
// SourceVersion; a mismatch yields STALE_VERSION. The template key
// must be in the active context's TemplateKeys.
func (o *Orchestrator) Synthesize(ctx context.Context, req contracts.TTSRequest, rawBody []byte) (contracts.TTSResponse, error) {
	if req.Jurisdiction == "" {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
			Stage: StageContext, Code: contracts.ErrInvalidValue, Reason: "jurisdiction is required", Retryable: false,
		})
	}
	if req.SpeechKey == "" {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrInvalidValue, Reason: "speech_key is required", Retryable: false,
		})
	}
	if req.Language == "" {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrInvalidValue, Reason: "language is required", Retryable: false,
		})
	}
	if req.SourceVersion <= 0 {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
			Stage: StageContext, Code: contracts.ErrInvalidValue, Reason: "source_version must be a positive integer", Retryable: false,
		})
	}
	// Validate raw JSON shape before any worker call.
	if len(rawBody) > 0 {
		if err := o.cfg.Validator.ValidateShape(rawBody); err != nil {
			return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 422, StageFailure{
				Stage: StageValidator, Code: contracts.ErrValidation, Reason: err.Error(), Retryable: false,
			})
		}
	}

	sc, err := o.cfg.Resolver.Resolve(ctx, req.Jurisdiction)
	if err != nil {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 503, StageFailure{
			Stage: StageContext, Code: contracts.ErrDataUnavailable, Reason: "scoped context unavailable: " + err.Error(), Retryable: true,
		})
	}

	if req.SourceVersion != sc.SourceVersion {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
			Stage: StageContext, Code: contracts.ErrStaleVersion, Reason: "source_version is stale", Retryable: false,
		})
	}

	// Revalidate snapshot before synthesis.
	if err := o.cfg.Resolver.SnapshotRevalidate(ctx, sc); err != nil {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
			Stage: StageContext, Code: contracts.ErrStaleSnapshot, Reason: "snapshot stale before synthesis: " + err.Error(), Retryable: false,
		})
	}

	if !sc.IsSpeechKeyApprovedForLanguage(req.SpeechKey, req.Language) {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template key not approved for language in jurisdiction", Retryable: false,
		})
	}
	if !sc.IsLanguageAllowed(req.Language) {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineUnsupported, 422, StageFailure{
			Stage: StageContext, Code: contracts.ErrLanguageUnsupported, Reason: "language not in active context", Retryable: false,
		})
	}

	// Lookup the approved template.
	tpl, ok := o.cfg.Templates.Lookup(req.SpeechKey, req.Language)
	if !ok {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template not registered for language", Retryable: false,
		})
	}
	if tpl.SyntheticOnly {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template is synthetic-only", Retryable: false,
		})
	}
	if tpl.TemplateVersion == 0 || tpl.TemplateVersion != sc.TemplateVersion {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrStaleVersion, Reason: fmt.Sprintf("template version %d does not match active context %d", tpl.TemplateVersion, sc.TemplateVersion), Retryable: false,
		})
	}
	if tpl.SourceVersion == 0 || tpl.SourceVersion != sc.SourceVersion {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrStaleVersion, Reason: fmt.Sprintf("template source version %d does not match active context %d", tpl.SourceVersion, sc.SourceVersion), Retryable: false,
		})
	}
	// B01: digest over approved canonical template bytes, not rendered text.
	wantSHA, ok := sc.TemplateDigest(req.SpeechKey, req.Language)
	if !ok || wantSHA == "" {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template digest not approved for language", Retryable: false,
		})
	}
	if got := sha256HexOfString(tpl.Text); got != wantSHA {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template digest mismatch", Retryable: false,
		})
	}

	// Server-derived names/counts/IDs cannot be invented by speech_args.
	for k, v := range req.Args.Args {
		if s, ok := v.(string); ok && s != "" {
			if strings.HasSuffix(k, "_id") || k == "target" || k == "place" {
				if !sc.IsKnownID(s) {
					return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
						Stage: StageTemplate, Code: contracts.ErrValidation, Reason: fmt.Sprintf("arg %q value %q is not known to active context", k, s), Retryable: false,
					})
				}
			}
		}
	}

	// Render with the validated args only. The TTS worker receives
	// rendered text only; it does NOT receive arbitrary user input.
	rendered, err := renderTemplateArgs(tpl, req.Args)
	if err != nil {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrValidation, Reason: err.Error(), Retryable: false,
		})
	}

	// Per-stage deadline.
	ttsCtx, cancel := StageDeadline(ctx, o.cfg.Limits.TTSDeadline)
	defer cancel()
	release, err := o.ttsQueue.Acquire(ttsCtx)
	if err != nil {
		if errors.Is(err, ErrQueueSaturated) {
			o.cfg.Metrics.ObserveQueueReject(StageTTS)
			return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
				Stage: StageTTS, Code: contracts.ErrQueueSaturated, Reason: "tts queue saturated", Retryable: true,
			})
		}
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageTTS, Code: contracts.ErrInferenceCancelled, Reason: err.Error(), Retryable: false,
		})
	}
	defer release()
	if !o.cfg.Workers.IsReady(StageTTS) {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageTTS, Code: contracts.ErrModelUnavailable, Reason: "tts worker not ready", Retryable: true,
		})
	}
	id := CorrelationID(req.RequestID)
	if id == "" {
		id = newCorrelationID()
	}
	gen := o.cfg.Correlation.Claim(id, StageTTS)
	o.cfg.Correlation.MarkInflight(id, StageTTS, gen)
	resp, err := o.cfg.Workers.TTS.Synthesize(ttsCtx, contracts.TTSWorkerRequest{
		RequestID:       string(id),
		SpeechKey:       req.SpeechKey,
		Language:        req.Language,
		Text:            rendered,
		SourceVersion:   req.SourceVersion,
		TemplateVersion: tpl.TemplateVersion,
		TemplateSHA256:  wantSHA,
		Settings:        req.Settings,
		DeadlineMillis:  o.cfg.Limits.TTSDeadline.Milliseconds(),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 504, StageFailure{
				Stage: StageTTS, Code: contracts.ErrModelTimeout, Reason: "tts deadline exceeded", Retryable: true,
			})
		}
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageTTS, Code: contracts.ErrModelUnavailable, Reason: err.Error(), Retryable: true,
		})
	}
	if !ValidateWorkerCorrelation(string(id), gen, o.cfg.Correlation, StageTTS) {
		o.cfg.Metrics.ObserveStaleDrop(StageTTS)
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageTTS, Code: contracts.ErrInferenceCancelled, Reason: "tts response stale", Retryable: false,
		})
	}
	if resp.State != contracts.TTSOK {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageTTS, Code: mapTTSStateToCode(resp.State), Reason: string(resp.State), Retryable: true,
		})
	}
	audioBytes, err := base64.StdEncoding.DecodeString(resp.AudioB64)
	if err != nil {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 500, StageFailure{
			Stage: StageTTS, Code: contracts.ErrInternal, Reason: "tts audio base64 invalid", Retryable: false,
		})
	}
	sum := sha256.Sum256(audioBytes)
	checksum := hex.EncodeToString(sum[:])
	if checksum != resp.ChecksumSHA256 {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 500, StageFailure{
			Stage: StageTTS, Code: contracts.ErrInternal, Reason: "tts checksum mismatch", Retryable: false,
		})
	}

	// Revalidate snapshot after synthesis before audio delivery.
	if err := o.cfg.Resolver.SnapshotRevalidate(ctx, sc); err != nil {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
			Stage: StageContext, Code: contracts.ErrStaleSnapshot, Reason: "snapshot stale after synthesis: " + err.Error(), Retryable: false,
		})
	}

	return contracts.TTSResponse{
		RequestID:       string(id),
		SpeechKey:       req.SpeechKey,
		Language:        req.Language,
		SourceVersion:   req.SourceVersion,
		TemplateVersion: tpl.TemplateVersion,
		ModelRevision:   resp.ModelRevision,
		VoiceRevision:   resp.VoiceRevision,
		AudioID:         checksum,
		ContentType:     resp.ContentType,
		ByteSize:        int64(len(audioBytes)),
		ChecksumSHA256:  checksum,
		CacheHit:        false,
		State:           resp.State,
		Settings:        req.Settings,
		AudioB64:        resp.AudioB64,
	}, nil
}

// renderTemplateArgs substitutes validated args into the template's
// rendered text. The args object is validated against the template's
// ArgSchema; any injection characters or extra args cause failure.
func renderTemplateArgs(tpl contracts.ApprovedTemplate, args contracts.SpeechArgs) (string, error) {
	argSchema, hasSchema := parseArgSchema(tpl.ArgSchema)
	if hasSchema && len(argSchema) > 0 {
		for k, expectedType := range argSchema {
			v, ok := args.Args[k]
			if !ok {
				return "", fmt.Errorf("missing required template arg: %s", k)
			}
			switch expectedType {
			case "int":
				if _, ok := toInt(v); !ok {
					return "", fmt.Errorf("arg %q must be an integer", k)
				}
			case "string":
				s, ok := v.(string)
				if !ok || s == "" {
					return "", fmt.Errorf("arg %q must be a non-empty string", k)
				}
				if !validIDChar(s) {
					return "", fmt.Errorf("arg %q contains invalid characters or delimiters", k)
				}
			default:
				s := fmt.Sprintf("%v", v)
				if !validIDChar(s) {
					return "", fmt.Errorf("arg %q contains invalid characters", k)
				}
			}
		}
		for k := range args.Args {
			if _, ok := argSchema[k]; !ok {
				return "", fmt.Errorf("unexpected arg: %s", k)
			}
		}
	} else if len(args.Args) > 0 {
		return "", fmt.Errorf("template does not accept args but got %d", len(args.Args))
	}

	out := tpl.Text
	for k, v := range args.Args {
		if !isValidArgKey(k) {
			return "", fmt.Errorf("invalid arg key: %s", k)
		}
		val := argValueString(v)
		if !validIDChar(val) && val != "" {
			return "", fmt.Errorf("arg value for %s contains invalid characters", k)
		}
		key := "{" + k + "}"
		out = strings.ReplaceAll(out, key, val)
	}
	if strings.Contains(out, "{") || strings.Contains(out, "}") {
		return "", errors.New("template text contains unreplaced placeholder or forbidden delimiter")
	}
	return out, nil
}

// parseArgSchema normalizes the any ArgSchema value into a key -> type map.
func parseArgSchema(schema any) (map[string]string, bool) {
	if schema == nil {
		return nil, false
	}
	out := make(map[string]string)
	switch s := schema.(type) {
	case map[string]string:
		for k, v := range s {
			out[k] = v
		}
		return out, true
	case map[string]any:
		for k, v := range s {
			if str, ok := v.(string); ok {
				out[k] = str
			} else {
				out[k] = fmt.Sprintf("%v", v)
			}
		}
		return out, true
	default:
		bs, err := json.Marshal(schema)
		if err == nil {
			var m map[string]string
			if err := json.Unmarshal(bs, &m); err == nil {
				return m, true
			}
		}
		return nil, false
	}
}

// validIDChar returns true when the value looks like a typed ID.
// Forbids control characters, angle brackets, curly braces, quotes,
// and backslashes so substituted text cannot smuggle SSTI payloads.
func validIDChar(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c == '-' || c == '_' || c == '.' || c == ':':
		default:
			return false
		}
	}
	return true
}

func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		if x != float64(int(x)) {
			return 0, false
		}
		return int(x), true
	case string:
		var i int
		n, err := fmt.Sscanf(x, "%d", &i)
		if err == nil && n == 1 {
			return i, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// isValidArgKey accepts the canonical arg key shape (letters, digits,
// underscore, dash, dot). Anything else is rejected.
func isValidArgKey(k string) bool {
	if k == "" || len(k) > 64 {
		return false
	}
	for _, r := range k {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}

// argValueString converts an arbitrary JSON value to a string the
// template renderer substitutes in. Numbers, booleans, and strings
// are formatted; nil and complex types produce an error.
func argValueString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return intToString(int64(x))
		}
		return floatToString(x)
	case int:
		return intToString(int64(x))
	case int32:
		return intToString(int64(x))
	case int64:
		return intToString(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return ""
	}
}

func intToString(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func floatToString(f float64) string {
	bs, _ := json.Marshal(f)
	return string(bs)
}
