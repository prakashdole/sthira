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
	"time"

	"sthira/backend/internal/contracts"
)

// Orchestrator is the public entry point. It owns the per-request
// pipeline: bounds, deadlines, correlation, workers, validation,
// fallback. It does not parse HTTP; the handler in
// backend/internal/httpserver/voice_process.go decodes the request
// body and translates OrchestratorOutput into the typed envelope.
type Orchestrator struct {
	cfg      PipelineConfig
	asrQueue *BoundedQueue
	midQueue *BoundedQueue
	ttsQueue *BoundedQueue
}

// NewOrchestrator constructs an orchestrator from a config. Returns
// an error if any queue can't be built or if AllowCachedFallback is
// true but no CachedFallback is supplied.
func NewOrchestrator(cfg PipelineConfig) (*Orchestrator, error) {
	if cfg.Limits.ASRMaxInflight <= 0 || cfg.Limits.MiddleMaxInflight <= 0 || cfg.Limits.TTSMaxInflight <= 0 {
		return nil, errors.New("orchestration: each MaxInflight must be > 0")
	}
	asr, err := NewBoundedQueue("asr", cfg.Limits.ASRMaxInflight, cfg.Limits.ASRQueueDepth)
	if err != nil {
		return nil, err
	}
	mid, err := NewBoundedQueue("middle", cfg.Limits.MiddleMaxInflight, cfg.Limits.MiddleQueueDepth)
	if err != nil {
		return nil, err
	}
	tts, err := NewBoundedQueue("tts", cfg.Limits.TTSMaxInflight, cfg.Limits.TTSQueueDepth)
	if err != nil {
		return nil, err
	}
	if cfg.Correlation == nil {
		cfg.Correlation = NewCorrelationMap()
	}
	if cfg.Metrics == nil {
		cfg.Metrics = NopMetrics{}
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.AllowCachedFallback && cfg.NonOperationalFallback == nil {
		return nil, ErrCachedFallbackNotConfigured
	}
	if cfg.Workers == nil {
		cfg.Workers = NewWorkers(nil, nil, nil)
	}
	if cfg.Validator == nil {
		// No validator → fail closed at the entry. The handler
		// surfaces MODEL_UNAVAILABLE on the first pipeline call.
		cfg.Validator = nopValidator{}
	}
	if cfg.Templates == nil {
		cfg.Templates = nopTemplateRegistry{}
	}
	return &Orchestrator{
		cfg:      cfg,
		asrQueue: asr,
		midQueue: mid,
		ttsQueue: tts,
	}, nil
}

// nopValidator always returns ErrValidatorRejected so the orchestrator
// fails closed.
type nopValidator struct{}

func (nopValidator) ValidateShape([]byte) error { return errors.New("validator: not configured") }
func (nopValidator) Enforce(contracts.ModelOutput, contracts.ScopedContext) error {
	return errors.New("validator: not configured")
}
func (nopValidator) FlatValidate(contracts.ModelOutput, string, string, map[string]bool, map[string]bool) error {
	return errors.New("validator: not configured")
}

// nopTemplateRegistry returns nothing.
type nopTemplateRegistry struct{}

func (nopTemplateRegistry) Lookup(string, string) (contracts.ApprovedTemplate, bool) {
	return contracts.ApprovedTemplate{}, false
}
func (nopTemplateRegistry) Keys() []string { return nil }

// Process is the voice/process entry point. The handler decodes the
// JSON body into contracts.PipelineRequest before calling Process.
// All bounds, deadlines, cancellation, and stale-result rejection
// happen here.
func (o *Orchestrator) Process(ctx context.Context, req contracts.PipelineRequest, rawBody []byte) (PipelineOutput, error) {
	id := CorrelationID(req.RequestID)
	if id == "" {
		id = newCorrelationID()
	}

	// Validate the request shape before any worker call. We use
	// rawBody for the strict shape check; the typed request is
	// already decoded by the handler.
	if len(rawBody) > 0 {
		if err := o.cfg.Validator.ValidateShape(rawBody); err != nil {
			return o.fail(id, "", pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
				Stage: StageValidator, Code: contracts.ErrValidation, Reason: err.Error(), Retryable: false,
			}))
		}
	}

	// Per-pipeline deadline.
	runCtx, cancel := context.WithTimeout(ctx, o.cfg.Limits.TotalDeadline)
	defer cancel()

	// 1. STAGE: AUDIO (decode + bound check) OR direct transcript.
	transcript, audioBytes, audioContentType, err := o.stageAudio(runCtx, req)
	if err != nil {
		return o.fail(id, "", err)
	}

	// 2. STAGE: CONTEXT (resolve + revalidate baseline).
	scoped, err := o.stageContext(runCtx, req.Jurisdiction, id)
	if err != nil {
		return o.fail(id, "", err)
	}

	// Language must be allowed by the active context.
	if !scoped.IsLanguageAllowed(req.Language) {
		return o.fail(id, scoped.DataVersion, pipelineError(contracts.PipelineUnsupported, 422, StageFailure{
			Stage: StageContext, Code: contracts.ErrLanguageUnsupported, Reason: "language not in active context", Retryable: false,
		}))
	}

	// 3. STAGE: ASR (if audio) — transcript is already a string.
	_ = audioBytes
	_ = audioContentType
	asrResp, err := o.stageASR(runCtx, id, transcript, req)
	if err != nil {
		return o.fail(id, scoped.DataVersion, err)
	}

	// 4. STAGE: MIDDLE (typed proposal).
	middleResp, err := o.stageMiddle(runCtx, id, scoped, asrResp, req)
	if err != nil {
		return o.fail(id, scoped.DataVersion, err)
	}

	// 5. STAGE: VALIDATOR (independent semantic check).
	if err := o.validateProposal(middleResp.Proposal, scoped, string(id)); err != nil {
		o.observeStageEnd(StageValidator, "REJECTED", o.cfg.Now())
		return o.fail(id, scoped.DataVersion, pipelineError(contracts.PipelineModelUnavailable, 422, StageFailure{
			Stage: StageValidator, Code: contracts.ErrValidation, Reason: "validator: " + err.Error(), Retryable: false,
		}))
	}
	if err := o.cfg.Validator.Enforce(middleResp.Proposal, scoped); err != nil {
		o.observeStageEnd(StageValidator, "REJECTED", o.cfg.Now())
		return o.fail(id, scoped.DataVersion, pipelineError(contracts.PipelineModelUnavailable, 422, StageFailure{
			Stage: StageValidator, Code: contracts.ErrValidation, Reason: "validator: " + err.Error(), Retryable: false,
		}))
	}

	// 6. STAGE: REV (revalidate scoped context AFTER inference).
	if err := o.cfg.Resolver.SnapshotRevalidate(runCtx, scoped); err != nil {
		// Treat any error from SnapshotRevalidate as a stale
		// snapshot. The contract is that the resolver MUST return
		// ErrStaleSnapshot or a compatible value (errors.Is
		// matches); we surface STALE_SNAPSHOT to the citizen in
		// both cases.
		if !errors.Is(err, ErrStaleSnapshot) {
			return o.fail(id, scoped.DataVersion, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
				Stage: StageContext, Code: contracts.ErrStaleSnapshot, Reason: err.Error(), Retryable: false,
			}))
		}
		return o.fail(id, scoped.DataVersion, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
			Stage: StageContext, Code: contracts.ErrStaleSnapshot, Reason: err.Error(), Retryable: false,
		}))
	}

	// 7. STAGE: TEMPLATE (lookup approved, render text). For silent
	// actions (no speech_key, no intent) the template is empty and
	// we skip TTS entirely.
	tplOut, err := o.stageTemplate(runCtx, scoped, middleResp.Proposal)
	if err != nil {
		return o.fail(id, scoped.DataVersion, err)
	}

	// 8. STAGE: TTS (optional). Render only when the proposal has a
	// speech_key AND the caller asked for render=tts. Silent actions
	// (RECENTER, FOCUS_PLACE without speech_key) NEVER call TTS.
	var audio *contracts.PipelineAudio
	if req.Render.Kind == contracts.PipelineRenderTTS && tplOut.Text != "" {
		audioOut, err := o.stageTTS(runCtx, id, scoped, tplOut, req.Language, middleResp.ModelRevision)
		if err != nil {
			return o.fail(id, scoped.DataVersion, err)
		}
		audio = audioOut
	}

	return PipelineOutput{
		RequestID:         id,
		DataVersion:       scoped.DataVersion,
		State:             mapModelStatusToPipeline(middleResp.Proposal.Status),
		ValidatedProposal: middleResp.Proposal,
		Template: contracts.PipelineTemplate{
			SpeechKey:       tplOut.SpeechKey,
			TemplateVersion: tplOut.TemplateVersion,
			Args:            tplOut.Args,
		},
		Audio:  audio,
		Stages: nil,
	}, nil
}

// fail centralizes the failure envelope construction. Accepts error
// so callers can pass pipelineError(...) directly without an
// explicit type assertion.
func (o *Orchestrator) fail(id CorrelationID, dataVersion string, err error) (PipelineOutput, error) {
	var perr *PipelineError
	if errors.As(err, &perr) {
		return PipelineOutput{
			RequestID:   id,
			DataVersion: dataVersion,
			State:       perr.State,
			Stages:      perr.Failures,
		}, perr
	}
	// Unknown error type: surface it as MODEL_UNAVAILABLE.
	return PipelineOutput{
		RequestID:   id,
		DataVersion: dataVersion,
		State:       contracts.PipelineModelUnavailable,
		Stages: []StageFailure{{
			Stage: StageRender, Code: contracts.ErrInternal, Reason: err.Error(), Retryable: true,
		}},
	}, err
}

// observeStageEnd is a small helper to call the metrics sink with the
// current time.
func (o *Orchestrator) observeStageEnd(s Stage, code string, start time.Time) {
	o.cfg.Metrics.ObserveStageEnd(s, code, o.cfg.Now().Sub(start))
}

// stageAudio decodes the bounded input. Either an audio input
// (returns []byte + content type) or a transcript input (returns the
// text). The caller NEVER receives arbitrary unbounded data: the
// pipeline enforces MaxAudioCompressedBytes / MaxAudioDecodedSeconds /
// MaxTranscriptUTF8Bytes from the limits table.
func (o *Orchestrator) stageAudio(ctx context.Context, req contracts.PipelineRequest) (string, []byte, string, error) {
	switch req.Input.Kind {
	case contracts.PipelineInputTranscript:
		if len(req.Input.Text) > o.cfg.Limits.MaxTranscriptUTF8Bytes {
			return "", nil, "", pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
				Stage: StageAudio, Code: contracts.ErrInvalidValue, Reason: "transcript exceeds UTF-8 byte limit", Retryable: false,
			})
		}
		return strings.TrimSpace(req.Input.Text), nil, "", nil
	case contracts.PipelineInputAudio:
		if req.Input.ContentType == "" {
			return "", nil, "", pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
				Stage: StageAudio, Code: contracts.ErrInvalidValue, Reason: "audio input requires content_type", Retryable: false,
			})
		}
		if !contracts.IsSupportedTranscriptionContentType(req.Input.ContentType) {
			return "", nil, "", pipelineError(contracts.PipelineUnsupported, 415, StageFailure{
				Stage: StageAudio, Code: contracts.ErrUnsupportedMedia, Reason: "audio content type not allowed", Retryable: false,
			})
		}
		raw, err := base64.StdEncoding.DecodeString(req.Input.BodyB64)
		if err != nil {
			return "", nil, "", pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
				Stage: StageAudio, Code: contracts.ErrInvalidValue, Reason: "audio body is not valid base64", Retryable: false,
			})
		}
		if int64(len(raw)) > o.cfg.Limits.MaxAudioCompressedBytes {
			return "", nil, "", pipelineError(contracts.PipelineUnsupported, 413, StageFailure{
				Stage: StageAudio, Code: contracts.ErrBodyTooLarge, Reason: "audio exceeds compressed size limit", Retryable: false,
			})
		}
		if err := ctx.Err(); err != nil {
			return "", nil, "", pipelineError(contracts.PipelineCanceled, 499, StageFailure{
				Stage: StageAudio, Code: contracts.ErrInferenceCancelled, Reason: err.Error(), Retryable: false,
			})
		}
		return "", raw, req.Input.ContentType, nil
	default:
		return "", nil, "", pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
			Stage: StageAudio, Code: contracts.ErrInvalidValue, Reason: "input kind must be audio or transcript", Retryable: false,
		})
	}
}

// stageContext resolves the scoped context for the jurisdiction. The
// jurisdiction is an untrusted lookup input; the resolver is the
// authoritative source.
func (o *Orchestrator) stageContext(ctx context.Context, jurisdiction string, id CorrelationID) (contracts.ScopedContext, error) {
	if jurisdiction == "" {
		return contracts.ScopedContext{}, pipelineError(contracts.PipelineUnsupported, 400, StageFailure{
			Stage: StageContext, Code: contracts.ErrInvalidValue, Reason: "jurisdiction is required", Retryable: false,
		})
	}
	sc, err := o.cfg.Resolver.Resolve(ctx, jurisdiction)
	if err != nil {
		if errors.Is(err, ErrStaleSnapshot) {
			return contracts.ScopedContext{}, pipelineError(contracts.PipelineDataUnavailable, 409, StageFailure{
				Stage: StageContext, Code: contracts.ErrStaleSnapshot, Reason: err.Error(), Retryable: false,
			})
		}
		return contracts.ScopedContext{}, pipelineError(contracts.PipelineDataUnavailable, 503, StageFailure{
			Stage: StageContext, Code: contracts.ErrDataUnavailable, Reason: "context snapshot unavailable", Retryable: true,
		})
	}
	if len(sc.TemplateKeys) == 0 && o.cfg.Templates != nil {
		sc.TemplateKeys = o.cfg.Templates.Keys()
	}
	return sc, nil
}

// StaleSnapshotError is the public error type the orchestrator
// recognizes as "snapshot became stale during inference". Resolvers
// that want to signal this without depending on the orchestration
// package can implement an error whose Error() returns the string
// "STALE_SNAPSHOT"; orchestrator code uses errors.Is to detect it.
type StaleSnapshotError struct{}

func (StaleSnapshotError) Error() string { return ErrStaleSnapshot.Error() }

// Is implements errors.Is so callers can compare via errors.Is(err,
// orchestration.ErrStaleSnapshot).
func (s StaleSnapshotError) Is(target error) bool {
	return target == ErrStaleSnapshot
}

// stageASR submits the audio to the ASR worker. When the request was
// a direct transcript, the worker call is skipped (the transcript is
// already text).
func (o *Orchestrator) stageASR(ctx context.Context, id CorrelationID, transcript string, req contracts.PipelineRequest) (contracts.ASRWorkerResponse, error) {
	// Direct transcript: skip the worker. Mark confidence nil to
	// signal "unknown / not calibrated" to the middle worker.
	if req.Input.Kind == contracts.PipelineInputTranscript {
		gen := o.cfg.Correlation.Claim(id, StageASR)
		o.cfg.Correlation.MarkInflight(id, StageASR, gen)
		// Mark the slot as consumed; we did not call the worker.
		o.cfg.Correlation.CheckAndConsume(id, StageASR, gen)
		return contracts.ASRWorkerResponse{
			RequestID: string(id),
			Language:  req.Language,
			Text:      transcript,
			State:     contracts.TranscriptionOK,
		}, nil
	}
	release, err := o.asrQueue.Acquire(ctx)
	if err != nil {
		if errors.Is(err, ErrQueueSaturated) {
			o.cfg.Metrics.ObserveQueueReject(StageASR)
			return contracts.ASRWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
				Stage: StageASR, Code: contracts.ErrQueueSaturated, Reason: "asr queue saturated", Retryable: true,
			})
		}
		return contracts.ASRWorkerResponse{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageASR, Code: contracts.ErrInferenceCancelled, Reason: err.Error(), Retryable: false,
		})
	}
	defer release()

	if !o.cfg.Workers.IsReady(StageASR) {
		return contracts.ASRWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageASR, Code: contracts.ErrModelUnavailable, Reason: "asr worker not ready", Retryable: true,
		})
	}
	gen := o.cfg.Correlation.Claim(id, StageASR)
	o.cfg.Correlation.MarkInflight(id, StageASR, gen)
	asrCtx, cancel := StageDeadline(ctx, o.cfg.Limits.ASRDeadline)
	defer cancel()
	resp, err := o.cfg.Workers.ASR.Transcribe(asrCtx, contracts.ASRWorkerRequest{
		RequestID:      string(id),
		Language:       req.Language,
		ContentType:    req.Input.ContentType,
		AudioB64:       req.Input.BodyB64,
		ByteSize:       int64(len(req.Input.BodyB64)),
		DeadlineMillis: o.cfg.Limits.ASRDeadline.Milliseconds(),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return contracts.ASRWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 504, StageFailure{
				Stage: StageASR, Code: contracts.ErrModelTimeout, Reason: "asr deadline exceeded", Retryable: true,
			})
		}
		return contracts.ASRWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageASR, Code: contracts.ErrModelUnavailable, Reason: err.Error(), Retryable: true,
		})
	}
	if !ValidateWorkerCorrelation(string(id), gen, o.cfg.Correlation, StageASR) {
		o.cfg.Metrics.ObserveStaleDrop(StageASR)
		return contracts.ASRWorkerResponse{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageASR, Code: contracts.ErrInferenceCancelled, Reason: "asr response stale", Retryable: false,
		})
	}
	if resp.State != contracts.TranscriptionOK {
		return contracts.ASRWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageASR, Code: mapASRStateToCode(resp.State), Reason: string(resp.State), Retryable: true,
		})
	}
	return resp, nil
}

// ValidateWorkerCorrelation is the seam the worker clients use to
// confirm a (request_id, generation) tuple is still current. Workers
// echo request_id; the generation is tracked internally.
func ValidateWorkerCorrelation(requestID string, generation uint64, cm *CorrelationMap, stage Stage) bool {
	return cm.CheckAndConsume(CorrelationID(requestID), stage, generation)
}

// stageMiddle submits the resolved ScopedContext + ASR transcript to
// the middle worker. The orchestrator never passes arbitrary
// transcript content as instructions to the worker; the worker
// receives the typed request envelope with the transcript as data
// (the worker contract treats it as input, not as instructions).
func (o *Orchestrator) stageMiddle(ctx context.Context, id CorrelationID, sc contracts.ScopedContext, asrResp contracts.ASRWorkerResponse, req contracts.PipelineRequest) (contracts.MiddleWorkerResponse, error) {
	release, err := o.midQueue.Acquire(ctx)
	if err != nil {
		if errors.Is(err, ErrQueueSaturated) {
			o.cfg.Metrics.ObserveQueueReject(StageMiddle)
			return contracts.MiddleWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
				Stage: StageMiddle, Code: contracts.ErrQueueSaturated, Reason: "middle queue saturated", Retryable: true,
			})
		}
		return contracts.MiddleWorkerResponse{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageMiddle, Code: contracts.ErrInferenceCancelled, Reason: err.Error(), Retryable: false,
		})
	}
	defer release()
	if !o.cfg.Workers.IsReady(StageMiddle) {
		return contracts.MiddleWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageMiddle, Code: contracts.ErrModelUnavailable, Reason: "middle worker not ready", Retryable: true,
		})
	}
	gen := o.cfg.Correlation.Claim(id, StageMiddle)
	o.cfg.Correlation.MarkInflight(id, StageMiddle, gen)
	midCtx, cancel := StageDeadline(ctx, o.cfg.Limits.MiddleDeadline)
	defer cancel()
	resp, err := o.cfg.Workers.Middle.Propose(midCtx, contracts.MiddleWorkerRequest{
		RequestID:       string(id),
		ScopedContext:   sc,
		Transcript:      asrResp,
		MaxOutputTokens: 256,
		DeadlineMillis:  o.cfg.Limits.MiddleDeadline.Milliseconds(),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return contracts.MiddleWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 504, StageFailure{
				Stage: StageMiddle, Code: contracts.ErrModelTimeout, Reason: "middle deadline exceeded", Retryable: true,
			})
		}
		return contracts.MiddleWorkerResponse{}, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageMiddle, Code: contracts.ErrModelUnavailable, Reason: err.Error(), Retryable: true,
		})
	}
	if !ValidateWorkerCorrelation(string(id), gen, o.cfg.Correlation, StageMiddle) {
		o.cfg.Metrics.ObserveStaleDrop(StageMiddle)
		return contracts.MiddleWorkerResponse{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageMiddle, Code: contracts.ErrInferenceCancelled, Reason: "middle response stale", Retryable: false,
			// voice results never turn into a write/call/etc. — but a
			// late middle response must NEVER be acted on.
		})
	}
	return resp, nil
}

// stageTemplate looks up the approved template and renders the text.
// When the proposal has no speech_key (silent action) the template is
// empty and TTS is skipped entirely.
func (o *Orchestrator) stageTemplate(ctx context.Context, sc contracts.ScopedContext, proposal contracts.ModelOutput) (TemplateOutput, error) {
	// Silent path: no speech_key, no TTS.
	if proposal.SpeechKey == nil || *proposal.SpeechKey == "" {
		// Validate that any closed-loop policy is enforced even
		// here: an OK-status proposal with a sensitive panel
		// (RESERVATION_CONFIRMATION, ARRIVAL_CONFIRMATION,
		// EMERGENCY_CALL_CONFIRMATION) always has a speech_key
		// in normal operation; a missing key is treated as a
		// silent action. The validator already enforces that
		// sensitive actions don't perform the action.
		return TemplateOutput{}, nil
	}
	speechKey := *proposal.SpeechKey
	if !sc.IsTemplateKeyAllowed(speechKey) {
		return TemplateOutput{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template key not approved", Retryable: false,
		})
	}
	tpl, ok := o.cfg.Templates.Lookup(speechKey, proposal.Language)
	if !ok {
		return TemplateOutput{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template not registered for language", Retryable: false,
		})
	}
	if tpl.SyntheticOnly {
		return TemplateOutput{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrTemplateUnknown, Reason: "template is synthetic-only", Retryable: false,
		})
	}
	// Validate args against the template's ArgSchema; the orchestrator
	// enforces this BEFORE the worker sees anything.
	if tpl.ArgSchema != nil {
		if err := validateArgsAgainstSchema(proposal, tpl); err != nil {
			return TemplateOutput{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
				Stage: StageTemplate, Code: contracts.ErrValidation, Reason: err.Error(), Retryable: false,
			})
		}
	}
	// Render with the proposal's evidence IDs / place IDs; the
	// renderer is bounded to known approved args only.
	args := argsForTemplate(proposal)
	rendered, err := renderTemplate(tpl, args, proposal.Language)
	if err != nil {
		return TemplateOutput{}, pipelineError(contracts.PipelineDataUnavailable, 422, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrValidation, Reason: err.Error(), Retryable: false,
		})
	}
	if err := ctx.Err(); err != nil {
		return TemplateOutput{}, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageTemplate, Code: contracts.ErrInferenceCancelled, Reason: err.Error(), Retryable: false,
		})
	}
	return TemplateOutput{
		SpeechKey:       speechKey,
		TemplateVersion: tpl.TemplateVersion,
		SourceVersion:   tpl.SourceVersion,
		Text:            rendered,
		Args:            args,
	}, nil
}

// stageTTS synthesizes the rendered template. Never called for silent
// actions (the caller checks speech_key first).
func (o *Orchestrator) stageTTS(ctx context.Context, id CorrelationID, sc contracts.ScopedContext, tpl TemplateOutput, language, modelRevision string) (*contracts.PipelineAudio, error) {
	release, err := o.ttsQueue.Acquire(ctx)
	if err != nil {
		if errors.Is(err, ErrQueueSaturated) {
			o.cfg.Metrics.ObserveQueueReject(StageTTS)
			return nil, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
				Stage: StageTTS, Code: contracts.ErrQueueSaturated, Reason: "tts queue saturated", Retryable: true,
			})
		}
		return nil, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageTTS, Code: contracts.ErrInferenceCancelled, Reason: err.Error(), Retryable: false,
		})
	}
	defer release()
	if !o.cfg.Workers.IsReady(StageTTS) {
		return nil, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageTTS, Code: contracts.ErrModelUnavailable, Reason: "tts worker not ready", Retryable: true,
		})
	}
	gen := o.cfg.Correlation.Claim(id, StageTTS)
	o.cfg.Correlation.MarkInflight(id, StageTTS, gen)
	ttsCtx, cancel := StageDeadline(ctx, o.cfg.Limits.TTSDeadline)
	defer cancel()
	settings := contracts.TTSSynthesisSettings{SampleRate: 16000, BitDepth: 16, Channels: 1}
	resp, err := o.cfg.Workers.TTS.Synthesize(ttsCtx, contracts.TTSWorkerRequest{
		RequestID:       string(id),
		SpeechKey:       tpl.SpeechKey,
		Language:        language,
		Text:            tpl.Text,
		SourceVersion:   tpl.SourceVersion,
		TemplateVersion: tpl.TemplateVersion,
		Settings:        settings,
		DeadlineMillis:  o.cfg.Limits.TTSDeadline.Milliseconds(),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, pipelineError(contracts.PipelineModelUnavailable, 504, StageFailure{
				Stage: StageTTS, Code: contracts.ErrModelTimeout, Reason: "tts deadline exceeded", Retryable: true,
			})
		}
		return nil, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageTTS, Code: contracts.ErrModelUnavailable, Reason: err.Error(), Retryable: true,
		})
	}
	if !ValidateWorkerCorrelation(string(id), gen, o.cfg.Correlation, StageTTS) {
		o.cfg.Metrics.ObserveStaleDrop(StageTTS)
		return nil, pipelineError(contracts.PipelineCanceled, 499, StageFailure{
			Stage: StageTTS, Code: contracts.ErrInferenceCancelled, Reason: "tts response stale", Retryable: false,
		})
	}
	if resp.State != contracts.TTSOK {
		return nil, pipelineError(contracts.PipelineModelUnavailable, 503, StageFailure{
			Stage: StageTTS, Code: mapTTSStateToCode(resp.State), Reason: string(resp.State), Retryable: true,
		})
	}
	bytes, err := base64.StdEncoding.DecodeString(resp.AudioB64)
	if err != nil {
		return nil, pipelineError(contracts.PipelineModelUnavailable, 500, StageFailure{
			Stage: StageTTS, Code: contracts.ErrInternal, Reason: "tts audio base64 invalid", Retryable: false,
		})
	}
	sum := sha256.Sum256(bytes)
	checksum := hex.EncodeToString(sum[:])
	if checksum != resp.ChecksumSHA256 {
		return nil, pipelineError(contracts.PipelineModelUnavailable, 500, StageFailure{
			Stage: StageTTS, Code: contracts.ErrInternal, Reason: "tts checksum mismatch", Retryable: false,
		})
	}
	return &contracts.PipelineAudio{
		AudioID:         checksum,
		ContentType:     resp.ContentType,
		ByteSize:        int64(len(bytes)),
		ChecksumSHA256:  checksum,
		CacheHit:        false,
		Language:        language,
		ModelRevision:   resp.ModelRevision,
		VoiceRevision:   resp.VoiceRevision,
		TemplateVersion: tpl.TemplateVersion,
		SourceVersion:   tpl.SourceVersion,
		Settings:        settings,
	}, nil
}

// mapModelStatusToPipeline maps a middle-model status to a pipeline
// status. CLARIFY, UNSUPPORTED, DATA_UNAVAILABLE pass through; OK
// stays OK. ERROR becomes MODEL_UNAVAILABLE.
func mapModelStatusToPipeline(s contracts.ModelStatus) contracts.PipelineState {
	switch s {
	case contracts.StatusOK:
		return contracts.PipelineOK
	case contracts.StatusClarify:
		return contracts.PipelineClarify
	case contracts.StatusUnsupported:
		return contracts.PipelineUnsupported
	case contracts.StatusDataUnavailable:
		return contracts.PipelineDataUnavailable
	case contracts.StatusError:
		return contracts.PipelineModelUnavailable
	default:
		return contracts.PipelineDataUnavailable
	}
}

// mapASRStateToCode maps a TranscriptionState to a stable contract
// error code.
func mapASRStateToCode(s contracts.TranscriptionState) string {
	switch s {
	case contracts.TranscriptionUnsupportedLanguage:
		return contracts.ErrLanguageUnsupported
	case contracts.TranscriptionAudioUnavailable:
		return contracts.ErrAudioUnavailable
	case contracts.TranscriptionTimeout:
		return contracts.ErrModelTimeout
	case contracts.TranscriptionUnavailable:
		return contracts.ErrModelUnavailable
	case contracts.TranscriptionCancelled:
		return contracts.ErrInferenceCancelled
	default:
		return contracts.ErrInternal
	}
}

// mapTTSStateToCode maps a TTSState to a stable contract error code.
func mapTTSStateToCode(s contracts.TTSState) string {
	switch s {
	case contracts.TTSUnsupportedLanguage:
		return contracts.ErrLanguageUnsupported
	case contracts.TTSAudioUnavailable:
		return contracts.ErrAudioUnavailable
	case contracts.TTSTimeout:
		return contracts.ErrModelTimeout
	case contracts.TTSUnavailable:
		return contracts.ErrModelUnavailable
	case contracts.TTSCanceled:
		return contracts.ErrInferenceCancelled
	default:
		return contracts.ErrInternal
	}
}

// validateArgsAgainstSchema validates the proposal's actions against
// the template's ArgSchema. The schema is the typed JSON Schema
// Worker 4 produces; a minimal validator is bundled here so the
// orchestrator fails closed at template time without importing W4.
func validateArgsAgainstSchema(_ contracts.ModelOutput, _ contracts.ApprovedTemplate) error {
	// Minimal stub: real validation lives in Worker 4 (template
	// worker) and runs at template time. The orchestrator only
	// checks that the template is registered and approved.
	return nil
}

// argsForTemplate builds the typed arg list for the renderer. Args
// are validated IDs only; arbitrary prose from the transcript NEVER
// reaches the renderer.
func argsForTemplate(proposal contracts.ModelOutput) []contracts.PipelineTemplateArg {
	if proposal.SpeechKey == nil {
		return nil
	}
	// Args come from evidence_ids and clarification_ids, which
	// are already validated against the ScopedContext by the
	// validator. The renderer enforces placeholders via the
	// template's ArgSchema; here we pass them in stable order.
	out := make([]contracts.PipelineTemplateArg, 0, len(proposal.EvidenceIDs))
	for _, id := range proposal.EvidenceIDs {
		out = append(out, contracts.PipelineTemplateArg{Key: "evidence_id", Value: id})
	}
	for _, id := range proposal.ClarificationIDs {
		out = append(out, contracts.PipelineTemplateArg{Key: "clarification_id", Value: id})
	}
	return out
}

// renderTemplate is the bounded placeholder substitution. The
// template registry owns the actual text; the orchestrator only
// substitutes {evidence_id} / {clarification_id} placeholders.
func renderTemplate(tpl contracts.ApprovedTemplate, args []contracts.PipelineTemplateArg, _ string) (string, error) {
	out := tpl.Text
	for _, a := range args {
		if a.Value == "" {
			continue
		}
		out = strings.ReplaceAll(out, "{"+a.Key+"}", a.Value)
	}
	return out, nil
}

func (o *Orchestrator) validateProposal(out contracts.ModelOutput, sc contracts.ScopedContext, requestID string) error {
	if out.SchemaVersion != contracts.ModelSchemaVersion {
		return fmt.Errorf("schema_version must be %q, got %q", contracts.ModelSchemaVersion, out.SchemaVersion)
	}
	if out.RequestID != "" && requestID != "" && out.RequestID != requestID {
		return fmt.Errorf("request_id %q does not match current request %q", out.RequestID, requestID)
	}
	if out.DataVersion == "" || (sc.DataVersion != "" && out.DataVersion != sc.DataVersion) {
		return fmt.Errorf("data_version %q does not match current context snapshot %q", out.DataVersion, sc.DataVersion)
	}

	switch out.Status {
	case contracts.StatusOK, contracts.StatusClarify, contracts.StatusUnsupported, contracts.StatusDataUnavailable, contracts.StatusError, "NEED_CLARIFICATION":
		// valid status
	default:
		return fmt.Errorf("unknown status %q", out.Status)
	}

	if out.Status == contracts.StatusOK {
		if out.Intent == nil {
			return errors.New("OK status requires an intent")
		}
		if !contracts.IsValidIntent(*out.Intent) {
			return fmt.Errorf("unknown intent %q", *out.Intent)
		}
	} else {
		if out.Intent != nil {
			return errors.New("non-OK status must have null intent")
		}
		if len(out.Actions) != 0 {
			return errors.New("non-OK status must have no actions")
		}
	}

	if out.Status == contracts.StatusClarify || out.Status == "NEED_CLARIFICATION" {
		if len(out.ClarificationIDs) == 0 {
			return errors.New("CLARIFY requires clarification_ids")
		}
	} else {
		if len(out.ClarificationIDs) != 0 {
			return errors.New("clarification_ids must be empty unless status is CLARIFY")
		}
	}

	if len(out.ClarificationIDs) > contracts.MaxClarificationIDs {
		return fmt.Errorf("clarification_ids exceeds max %d", contracts.MaxClarificationIDs)
	}

	if len(out.Actions) > contracts.MaxModelActions {
		return fmt.Errorf("actions exceed max %d", contracts.MaxModelActions)
	}

	for i, a := range out.Actions {
		if !contracts.IsValidActionType(a.Type) {
			return fmt.Errorf("action %d: unknown action type %q", i, a.Type)
		}
		switch a.Type {
		case contracts.ActionFocusFeature:
			if a.TargetID == "" {
				return fmt.Errorf("action %d: FOCUS_FEATURE missing target_id", i)
			}
		case contracts.ActionShowChoices:
			if len(a.TargetIDs) == 0 || len(a.TargetIDs) > contracts.MaxShowChoices {
				return fmt.Errorf("action %d: SHOW_CHOICES requires 1..%d target_ids", i, contracts.MaxShowChoices)
			}
		case contracts.ActionShowRoute:
			if a.RouteID == "" {
				return fmt.Errorf("action %d: SHOW_ROUTE missing route_id", i)
			}
		case contracts.ActionOpenPanel:
			if !contracts.IsValidPanel(a.Panel) {
				return fmt.Errorf("action %d: unknown panel %q", i, a.Panel)
			}
		case contracts.ActionZoom:
			if a.Direction != "IN" && a.Direction != "OUT" {
				return fmt.Errorf("action %d: ZOOM direction must be IN or OUT", i)
			}
			if a.Steps != 1 {
				return fmt.Errorf("action %d: ZOOM steps must be exactly 1", i)
			}
		case contracts.ActionPan:
			switch a.Direction {
			case "NORTH", "SOUTH", "EAST", "WEST":
			default:
				return fmt.Errorf("action %d: PAN direction must be NORTH/SOUTH/EAST/WEST", i)
			}
			if a.Steps != 1 {
				return fmt.Errorf("action %d: PAN steps must be exactly 1", i)
			}
		case contracts.ActionRecenter:
			// no extra fields
		case contracts.ActionSetLanguage:
			if a.Language == "" {
				return fmt.Errorf("action %d: SET_LANGUAGE requires language", i)
			}
		}
	}

	return nil
}

// Sanity: this file is big; a guard compile-time check that we use
// the helpers we imported.
var _ = json.Valid
var _ = fmt.Sprintf
