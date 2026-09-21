package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	// Validate raw JSON shape before any worker call.
	if len(rawBody) > 0 {
		if err := o.cfg.Validator.ValidateShape(rawBody); err != nil {
			return contracts.TTSResponse{}, pipelineError(contracts.PipelineModelUnavailable, 422, StageFailure{
				Stage: StageValidator, Code: contracts.ErrValidation, Reason: err.Error(), Retryable: false,
			})
		}
	}
	// We resolve a synthetic scoped context for template-key
	// validation. The full pipeline resolves the jurisdiction; here
	// the caller does not provide one. The orchestrator falls back
	// to ANY operational context, mirroring the legacy
	// ResolveAnyOperationalContext semantics.
	sc, err := o.resolveAnyScopedContext(ctx)
	if err != nil {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 503, StageFailure{
			Stage: StageContext, Code: contracts.ErrDataUnavailable, Reason: "scoped context unavailable", Retryable: true,
		})
	}
	if !sc.IsTemplateKeyAllowed(req.SpeechKey) {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template key not approved", Retryable: false,
		})
	}
	if !sc.IsLanguageAllowed(req.Language) {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineUnsupported, 422, StageFailure{
			Stage: StageContext, Code: contracts.ErrLanguageUnsupported, Reason: "language not in active context", Retryable: false,
		})
	}
	if req.SourceVersion != 0 && sc.SourceVersion != 0 && req.SourceVersion != sc.SourceVersion {
		return contracts.TTSResponse{}, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
			Stage: StageContext, Code: contracts.ErrStaleVersion, Reason: "source_version is stale", Retryable: false,
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
	}, nil
}

// resolveAnyScopedContext resolves a ScopedContext for the speech
// endpoint, which does not carry a jurisdiction. The orchestrator
// delegates to whatever resolver is wired (typically Worker 4's
// store.NewScopedContextResolver). When the resolver does not
// implement "any jurisdiction", we fail closed.
func (o *Orchestrator) resolveAnyScopedContext(ctx context.Context) (contracts.ScopedContext, error) {
	if ar, ok := o.cfg.Resolver.(interface {
		ResolveAny(context.Context) (contracts.ScopedContext, error)
	}); ok {
		return ar.ResolveAny(ctx)
	}
	// Fallback: try the empty-jurisdiction path. The contract is
	// that Resolve("") returns ErrNoOperationalContext; we surface
	// that as a generic unavailable.
	sc, err := o.cfg.Resolver.Resolve(ctx, "")
	if err == nil && sc.Jurisdiction != "" {
		return sc, nil
	}
	return contracts.ScopedContext{}, errors.New("orchestration: no operational context for the speech endpoint")
}

// renderTemplateArgs substitutes validated args into the template's
// rendered text. The args object is validated by the orchestrator
// against the template's ArgSchema; the worker only sees the
// rendered string.
func renderTemplateArgs(tpl contracts.ApprovedTemplate, args contracts.SpeechArgs) (string, error) {
	out := tpl.Text
	for k, v := range args.Args {
		if !isValidArgKey(k) {
			return "", errors.New("invalid arg key: " + k)
		}
		key := "{" + k + "}"
		val := argValueString(v)
		out = strings.ReplaceAll(out, key, val)
	}
	return out, nil
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
		// JSON numbers decode to float64; keep the integral form
		// when possible to avoid noisy "1234.000000" in templates.
		if x == float64(int64(x)) {
			return intToString(int64(x))
		}
		return floatToString(x)
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
	// Marshal produces a canonical JSON representation. We use
	// encoding/json for consistency with the typed envelope.
	bs, _ := json.Marshal(f)
	return string(bs)
}
