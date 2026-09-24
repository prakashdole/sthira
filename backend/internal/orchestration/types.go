package orchestration

import (
	"errors"
	"time"

	"sthira/backend/internal/contracts"
)

// Stage is the canonical name of a pipeline stage. Used as the map key
// for stage-level admission and for the correlation map. Values match
// the OpenAPI `stage_failures` enum so they can be returned verbatim.
type Stage string

const (
	StageAudio     Stage = "audio"
	StageASR       Stage = "asr"
	StageContext   Stage = "context"
	StageMiddle    Stage = "middle"
	StageValidator Stage = "validator"
	StageTemplate  Stage = "template"
	StageTTS       Stage = "tts"
	StageRender    Stage = "render"
)

// StageFailure reports the first stage that failed in a pipeline run.
// StageFailures is appended in pipeline order so the caller can see
// the full trail; the first element is the entry point of failure.
type StageFailure struct {
	Stage     Stage
	Code      string // stable contract error code (contracts.ErrXxx)
	Reason    string // human-readable; never PII; bounded to one short clause
	Retryable bool
}

// PipelineError wraps a stage failure with a typed exit state for the
// caller. The State field maps to one of the contracts.PipelineState*
// constants for voice/process; for transcription/speech the handler
// can map to its own state via stateFromCode.
type PipelineError struct {
	State      contracts.PipelineState
	Failures   []StageFailure
	HTTPStatus int
}

// Error implements error so the orchestrator can return early.
func (e *PipelineError) Error() string {
	if e == nil {
		return "nil pipeline error"
	}
	if len(e.Failures) == 0 {
		return "pipeline error"
	}
	f := e.Failures[0]
	return string(f.Stage) + ": " + f.Code + " — " + f.Reason
}

// Is allows errors.Is to detect a PipelineError via the State field.
// Standard usage:
//
//	var pe *PipelineError
//	if errors.As(err, &pe) && pe.State == contracts.PipelineModelUnavailable { ... }
func (e *PipelineError) Is(target error) bool {
	other, ok := target.(*PipelineError)
	if !ok {
		return false
	}
	return e.State == other.State
}

// pipelineError constructs a PipelineError with a single failure.
func pipelineError(state contracts.PipelineState, status int, f StageFailure) *PipelineError {
	return &PipelineError{State: state, HTTPStatus: status, Failures: []StageFailure{f}}
}

// Limits defines the documented ceilings for the voice pipeline.
// Per-stage deadlines add up to <= TotalDeadline; budgets below
// the total leave headroom for context revalidation and audio
// transfer.
type Limits struct {
	TotalDeadline   time.Duration
	ASRDeadline     time.Duration
	MiddleDeadline  time.Duration
	TTSDeadline     time.Duration
	ContextDeadline time.Duration

	ASRMaxInflight    int
	MiddleMaxInflight int
	TTSMaxInflight    int

	ASRQueueDepth    int
	MiddleQueueDepth int
	TTSQueueDepth    int

	// MaxAudioCompressedBytes matches contracts.PipelineMaxAudioCompressedBytes.
	MaxAudioCompressedBytes int64
	// MaxAudioDecodedSeconds matches contracts.PipelineMaxAudioDecodedSeconds.
	MaxAudioDecodedSeconds float64
	// MaxTranscriptUTF8Bytes matches contracts.PipelineMaxTranscriptUTF8Bytes.
	MaxTranscriptUTF8Bytes int
	// MaxRawModelBytes is the documented ceiling for raw model
	// proposal JSON bodies at the strict decoder. Larger payloads
	// are rejected before any worker call.
	MaxRawModelBytes int
}

// DefaultLimits returns the documented starting budgets. These are
// engineering targets; they are NOT measured results and they MUST
// not be promoted to SLOs without a measurement pass.
func DefaultLimits() Limits {
	return Limits{
		TotalDeadline:   8 * time.Second,
		ASRDeadline:     3 * time.Second,
		MiddleDeadline:  6 * time.Second,
		TTSDeadline:     4 * time.Second,
		ContextDeadline: 250 * time.Millisecond,

		ASRMaxInflight:    4,
		MiddleMaxInflight: 2,
		TTSMaxInflight:    4,

		ASRQueueDepth:    8,
		MiddleQueueDepth: 4,
		TTSQueueDepth:    8,

		MaxAudioCompressedBytes: contracts.PipelineMaxAudioCompressedBytes,
		MaxAudioDecodedSeconds:  contracts.PipelineMaxAudioDecodedSeconds,
		MaxTranscriptUTF8Bytes:  contracts.PipelineMaxTranscriptUTF8Bytes,
		MaxRawModelBytes:        64 * 1024, // 64 KiB raw proposal JSON; worker responses carry the bulk.
	}
}

// ErrPipelineCanceled is returned when the orchestrator detects a
// client cancellation. The handler maps this to contracts.ErrInferenceCancelled
// and HTTP 499.
var ErrPipelineCanceled = errors.New("orchestration: pipeline canceled")

// ErrQueueSaturated is returned when a stage admission queue is full.
// The handler maps this to contracts.ErrQueueSaturated and HTTP 503.
var ErrQueueSaturated = errors.New("orchestration: queue saturated")

// ErrStaleSnapshot is returned when SnapshotRevalidate detects a version
// change since the ScopedContext was issued.
var ErrStaleSnapshot = errors.New("orchestration: scoped snapshot is stale")

// ErrModelUnavailable is the catch-all for ASR/middle/TTS worker
// unavailability (warm=false, error response, no health response).
var ErrModelUnavailable = errors.New("orchestration: worker unavailable")

// ErrModelTimeout is returned when a per-stage deadline expires.
var ErrModelTimeout = errors.New("orchestration: per-stage deadline expired")

// ErrValidatorRejected is returned when the independent validator
// rejects the model's proposal.
var ErrValidatorRejected = errors.New("orchestration: validator rejected the proposal")

// ErrTemplateUnknown is returned when a template key is not in the
// approved registry.
var ErrTemplateUnknown = errors.New("orchestration: template key is not approved")

// ErrTemplateArgsInvalid is returned when template args fail the
// per-template JSON Schema check.
var ErrTemplateArgsInvalid = errors.New("orchestration: template args invalid")

// ErrCameraOnly is returned when a request explicitly asks the
// camera-only path (no model inference) under the agreed contract.
// The handler should bypass model work and return an empty OK state
// after applying the requested camera control.
var ErrCameraOnly = errors.New("orchestration: camera-only path; model bypass requested")

// ErrFallbackCached is returned when the orchestrator decided to use
// the cached non-operational fallback (text/touch, last-known-good
// guidance) because all inference stages are unavailable.
var ErrFallbackCached = errors.New("orchestration: cached non-operational fallback")

// ErrAudioUnavailable is returned for audio decode failures (the
// codec is unsupported, multi-channel, etc.).
var ErrAudioUnavailable = errors.New("orchestration: audio unavailable")

// PipelineStateModelUnavailable returns the canonical MODEL_UNAVAILABLE
// state. Exposed for callers that need to construct a synthetic
// PipelineError from a non-pipeline error.
func PipelineStateModelUnavailable() contracts.PipelineState {
	return contracts.PipelineModelUnavailable
}
