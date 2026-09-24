package orchestration

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"sthira/backend/internal/contracts"
)

// WorkerClient is the orchestrator-side seam that talks to one private
// worker (ASR, middle, or TTS). The orchestrator never imports the
// worker's package; the production implementation is a typed HTTP
// client whose URL and bearer token come from server-side
// configuration (NEVER from the request body). The interface exists
// so component tests can substitute a fake; the integration stage
// wires the real worker.
type WorkerClient interface {
	// Health returns the worker's last known health. It is safe to
	// call concurrently; Health does not panic on a nil worker.
	Health(ctx context.Context) (contracts.WorkerHealth, bool)
	// Transcribe (ASR-only) submits a bounded audio upload.
	Transcribe(ctx context.Context, req contracts.ASRWorkerRequest) (contracts.ASRWorkerResponse, error)
	// Propose (middle-only) submits a typed proposal request.
	Propose(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error)
	// Synthesize (TTS-only) submits a typed synthesis request.
	Synthesize(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error)
}

// HealthProber is the orchestrator's view of a worker; it returns the
// last known health without exposing the underlying client. Used in
// circuit-breaker checks before dispatch.
type HealthProber interface {
	LastHealth() (contracts.WorkerHealth, bool)
}

// ScopedContextResolver is the seam Worker 4 implements. The
// orchestrator calls Resolve once per pipeline run (or once per
// cross-stage revalidation) and feeds the typed result into the
// middle worker and the independent validator.
type ScopedContextResolver interface {
	Resolve(ctx context.Context, jurisdiction string) (contracts.ScopedContext, error)
}

// ScopedRevalidator is the seam Worker 4 implements for stale-snapshot
// detection. Called after every slow stage returns.
type ScopedRevalidator interface {
	SnapshotRevalidate(ctx context.Context, sc contracts.ScopedContext) error
}

// ScopedResolver combines Resolve + SnapshotRevalidate. The
// orchestrator consumes this combined seam; the production wiring is
// Worker 4's store.NewScopedContextResolver.
type ScopedResolver interface {
	ScopedContextResolver
	ScopedRevalidator
}

// TemplateRegistry is the read-only seam Worker 4 produces and Worker 7
// consumes. Lookup returns the rendered, approved text for a
// (speech_key, language) pair. The orchestrator calls Lookup after
// validation, never before.
type TemplateRegistry interface {
	Lookup(speechKey, language string) (contracts.ApprovedTemplate, bool)
	Keys() []string
}

// VoiceValidator is the orchestrator-side seam that runs the
// independent semantic check on the model's proposal. The production
// implementation is Worker 4's contracts.EnforceScopedContext +
// contracts.ValidateModelOutputShape; the orchestrator consumes the
// interface only.
type VoiceValidator interface {
	// ValidateShape performs the raw-JSON shape pre-check (no
	// duplicate keys, no unknown fields, no trailing data).
	ValidateShape(raw []byte) error
	// EnforceScopedContext enforces the typed semantic rules from
	// the resolved ScopedContext.
	Enforce(out contracts.ModelOutput, sc contracts.ScopedContext) error
	// FlatValidate is the legacy P1/P4 shape-only validator against
	// the flat KnownIDs map. It is the fallback used when the
	// scoped-context resolver is unavailable; in that mode the
	// orchestrator fails closed at the pipeline entry and never
	// reaches FlatValidate. Provided for the integration stage to
	// route to the existing contracts.ValidateModelOutput.
	FlatValidate(out contracts.ModelOutput, requestID, dataVersion string, known map[string]bool, langs map[string]bool) error
}

// PipelineOutput is the orchestrator's success envelope. The HTTP
// handler maps it to contracts.PipelineResponse.
type PipelineOutput struct {
	RequestID         CorrelationID
	DataVersion       string
	State             contracts.PipelineState
	ValidatedProposal contracts.ModelOutput
	Template          contracts.PipelineTemplate
	Audio             *contracts.PipelineAudio
	Stages            []StageFailure // populated on failure; empty on success
}

// TemplateOutput is what the template stage produces. The orchestrator
// looks up the ApprovedTemplate, validates args, and renders the
// approved text. Only the rendered text reaches TTS.
type TemplateOutput struct {
	SpeechKey       string
	TemplateVersion int
	SourceVersion   int
	Text            string
	Args            []contracts.PipelineTemplateArg
}

// AudioBytes is the TTS renderer's payload. The handler returns the
// bytes alongside the JSON envelope (audio/wav with the same
// checksum_sha256).
type AudioBytes struct {
	ContentType    string
	Bytes          []byte
	ChecksumSHA256 string
	VoiceRevision  string
	ModelRevision  string
}

// Workers is the bundle of three worker clients and a small circuit
// breaker (per-worker ready state) used to short-circuit dispatch
// when a worker reports Warm=false.
type Workers struct {
	ASR    WorkerClient
	Middle WorkerClient
	TTS    WorkerClient

	// readyMu protects the ready flag below. A worker is marked
	// not-ready when its health response shows ready=false or warm=false.
	readyMu  sync.RWMutex
	asrReady bool
	midReady bool
	ttsReady bool
	lastASR  contracts.WorkerHealth
	lastMid  contracts.WorkerHealth
	lastTTS  contracts.WorkerHealth
	// readyCount counts the cumulative number of successful
	// ready-state transitions (used by tests).
	readyCount atomic.Uint64
}

// NewWorkers returns an empty worker bundle. ASR, middle, TTS are all
// required; pass nil for any worker you intend to disable (the
// orchestrator then surfaces MODEL_UNAVAILABLE for that stage).
func NewWorkers(asr, mid, tts WorkerClient) *Workers {
	return &Workers{
		ASR:    asr,
		Middle: mid,
		TTS:    tts,
	}
}

// SnapshotHealth fetches the worker's current /health and updates the
// internal ready flag. Returns the new health snapshot. A nil client
// returns the zero WorkerHealth and ready=false.
func (w *Workers) SnapshotHealth(ctx context.Context, which Stage) (contracts.WorkerHealth, error) {
	if w == nil {
		return contracts.WorkerHealth{}, errors.New("orchestration: workers bundle is nil")
	}
	var c WorkerClient
	switch which {
	case StageASR:
		c = w.ASR
	case StageMiddle:
		c = w.Middle
	case StageTTS:
		c = w.TTS
	default:
		return contracts.WorkerHealth{}, errors.New("orchestration: stage has no worker")
	}
	if c == nil {
		w.markNotReady(which)
		return contracts.WorkerHealth{}, ErrModelUnavailable
	}
	h, ok := c.Health(ctx)
	if !ok {
		w.markNotReady(which)
		return contracts.WorkerHealth{}, ErrModelUnavailable
	}
	ready := h.Ready && h.Warm && len(h.SupportedLanguages) > 0
	w.readyMu.Lock()
	defer w.readyMu.Unlock()
	switch which {
	case StageASR:
		w.lastASR = h
		w.asrReady = ready
	case StageMiddle:
		w.lastMid = h
		w.midReady = ready
	case StageTTS:
		w.lastTTS = h
		w.ttsReady = ready
	}
	if ready {
		w.readyCount.Add(1)
	}
	return h, nil
}

// markNotReady is internal: it sets the worker to not-ready and is
// safe to call without holding the mutex.
func (w *Workers) markNotReady(which Stage) {
	w.readyMu.Lock()
	defer w.readyMu.Unlock()
	switch which {
	case StageASR:
		w.asrReady = false
	case StageMiddle:
		w.midReady = false
	case StageTTS:
		w.ttsReady = false
	}
}

// IsReady reports the last known ready flag for the worker. It does
// not perform a health probe; callers should call SnapshotHealth
// before IsReady if they need fresh state.
func (w *Workers) IsReady(which Stage) bool {
	if w == nil {
		return false
	}
	w.readyMu.RLock()
	defer w.readyMu.RUnlock()
	switch which {
	case StageASR:
		return w.asrReady
	case StageMiddle:
		return w.midReady
	case StageTTS:
		return w.ttsReady
	default:
		return false
	}
}

// HealthSnapshot returns the last known WorkerHealth for the stage.
// Returns zero, false when no snapshot is available.
func (w *Workers) HealthSnapshot(which Stage) (contracts.WorkerHealth, bool) {
	if w == nil {
		return contracts.WorkerHealth{}, false
	}
	w.readyMu.RLock()
	defer w.readyMu.RUnlock()
	switch which {
	case StageASR:
		return w.lastASR, w.lastASR.Ready
	case StageMiddle:
		return w.lastMid, w.lastMid.Ready
	case StageTTS:
		return w.lastTTS, w.lastTTS.Ready
	default:
		return contracts.WorkerHealth{}, false
	}
}

// LanguagesFor returns the languages the named worker reports it
// supports. Used by the orchestrator to fail closed when the user's
// language isn't on the worker's allow list.
func (w *Workers) LanguagesFor(which Stage) []string {
	h, ok := w.HealthSnapshot(which)
	if !ok {
		return nil
	}
	return h.SupportedLanguages
}

// PipelineConfig bundles the runtime dependencies the orchestrator
// needs. The handler constructs one per server; tests construct one
// per scenario.
type PipelineConfig struct {
	Limits      Limits
	Workers     *Workers
	Resolver    ScopedResolver
	Validator   VoiceValidator
	Templates   TemplateRegistry
	Correlation *CorrelationMap
	Metrics     MetricsRecorder
	Now         func() time.Time
	// AllowCameraOnly, when true, lets the orchestrator accept the
	// "simple explicit camera control" bypass (no model inference).
	AllowCameraOnly bool
	// AllowCachedFallback, when true, lets the orchestrator accept
	// the cached non-operational fallback when inference is fully
	// unavailable. Disabled in production by default.
	AllowCachedFallback bool
	// NonOperationalFallback supplies the cached snapshot the
	// fallback path uses. Required when AllowCachedFallback=true.
	NonOperationalFallback CachedFallback
}

// MetricsRecorder is the low-cardinality metrics sink the orchestrator
// writes to. Production wires the OpenTelemetry/Prom exporter;
// component tests pass a fake.
type MetricsRecorder interface {
	ObserveStageStart(stage Stage)
	ObserveStageEnd(stage Stage, code string, d time.Duration)
	ObserveStaleDrop(stage Stage)
	ObserveQueueReject(stage Stage)
	ObserveWorkerHealth(stage Stage, ready bool, warm bool)
}

// NopMetrics is the no-op metrics sink used when none is configured.
type NopMetrics struct{}

// ObserveStageStart implements MetricsRecorder.
func (NopMetrics) ObserveStageStart(Stage) {}

// ObserveStageEnd implements MetricsRecorder.
func (NopMetrics) ObserveStageEnd(Stage, string, time.Duration) {}

// ObserveStaleDrop implements MetricsRecorder.
func (NopMetrics) ObserveStaleDrop(Stage) {}

// ObserveQueueReject implements MetricsRecorder.
func (NopMetrics) ObserveQueueReject(Stage) {}

// ObserveWorkerHealth implements MetricsRecorder.
func (NopMetrics) ObserveWorkerHealth(Stage, bool, bool) {}

// CachedFallback supplies the cached non-operational fallback the
// orchestrator uses when all inference is unavailable. The cached
// snapshot is read-only, jurisdiction-scoped, and NEVER mutated by
// the orchestrator. Voice results are not written here.
type CachedFallback interface {
	// Lookup returns the cached proposal and template for a
	// jurisdiction+language. False when no cache entry exists.
	Lookup(ctx context.Context, jurisdiction, language string) (CachedEntry, bool)
}

// CachedEntry is the read-only fallback envelope. It mirrors
// PipelineOutput but with state CachedNonOperational so the UI can
// label it as such. The orchestrator tags it explicitly and never
// returns it as OK.
type CachedEntry struct {
	Proposal  contracts.ModelOutput
	Template  contracts.PipelineTemplate
	IssuedAt  string
	SourceRev int
}

// ErrCachedFallbackNotConfigured is returned when the caller enables
// the cached fallback but does not supply one.
var ErrCachedFallbackNotConfigured = errors.New("orchestration: cached fallback enabled but not configured")
