// Package provider defines the harness-side interface for the P6
// voice-pipeline. Two implementations exist:
//
//   - Deterministic: a rule-based stand-in that respects every
//     invariant of the contract and lets the harness self-validate
//     without standing up workers. It is NOT real ASR/middle/TTS; it
//     produces answers from rules keyed on the case's Category and
//     expected Outcome. Used only for harness development.
//   - HTTP: a thin client that speaks the frozen wire shapes to
//     workers via HTTP. The orchestrator-side handler is a generic
//     /api/v3 envelope; the worker process owns the adapters.
//
// The interface is intentionally narrow — reuses the same fields the
// orchestrator sends, so swapping providers does not invent new
// shapes. Provider implementations never modify the request.
package provider

import (
	"context"
	"errors"
	"sync"
	"time"

	"sthira/backend/eval/corpus"
)

// Mode reports how a provider classifies itself. Useful for the runner
// to refuse to produce real-rate metrics when only a Deterministic
// provider is present.
type Mode string

const (
	ModeDeterministic Mode = "DETERMINISTIC" // harness validation only
	ModeHTTP          Mode = "HTTP"          // real worker run
)

// ASROutcome captures the transcribed text. Mirrors the
// TranscriptionResponse.State enum without re-declaring it.
type ASROutcome struct {
	State      string
	Text       string
	Language   string
	Confidence *float64
	RevisionID string
}

// MiddleOutcome mirrors the PipelineResponse. The runner cares about
// the shape, not the network semantics.
type MiddleOutcome struct {
	Status      string
	Intent      string
	Language    string
	Actions     []string
	SpeechKey   string
	ClarifyIDs  []string
	EvidenceIDs []string
}

// TTSOutcome mirrors the audio delivery outcome.
type TTSOutcome struct {
	State      string
	SpeechKey  string
	ByteSize   int
	DurationMS int
}

// Provider is the seam through which the runner asks for
// pipeline-shaped answers. The implementation must:
//   - Be deterministic when Mode == Deterministic (same input → same
//     output, no time/seed drift across calls).
//   - Pass context through, including deadlines.
//   - Never panic on malformed input; return errors with class.
type Provider interface {
	Mode() Mode
	ASR(ctx context.Context, req ASRRequest) (ASROutcome, error)
	Middle(ctx context.Context, req MiddleRequest) (MiddleOutcome, error)
	TTS(ctx context.Context, req TTSRequest) (TTSOutcome, error)
}

// ASRRequest is the harness-side projection of a stage-1 call.
type ASRRequest struct {
	RequestID   string
	Language    string
	AudioB64    string
	ContentType string
	Case        corpus.Case
}

// MiddleRequest mirrors PipelineRequest envelope fields. The runner
// builds it from a corpus.Case + an ASROutcome.
type MiddleRequest struct {
	RequestID  string
	Language   string
	Transcript string
	Context    map[string]any
	Case       corpus.Case
}

// TTSRequest is the typed TTS shape trimmed to fields the harness
// cares about.
type TTSRequest struct {
	RequestID     string
	SpeechKey     string
	Language      string
	Args          map[string]any
	SourceVersion int
	Case          corpus.Case
}

// StageTimings captures per-stage latency for a single case. All times
// are wall-clock milliseconds; the runner stores zero when a stage is
// skipped (e.g. CLARIFY never reaches TTS).
type StageTimings struct {
	ASRMS      int64
	MiddleMS   int64
	TTSMS      int64
	TotalMS    int64
	BudgetUsed int64 // budget ms after smoothing
}

// Result is the runner's reconciliation record. ColdRun says the
// first dispatch for this (language,category) tuple — warm-up
// dispatches are recorded separately, not in this slice. WarmedUp
// flags a discarded warm-up result so the reporter never counts it.
type Result struct {
	Case             corpus.Case
	Outcome          string // observed status
	IntentObserved   string // observed middleware intent
	ActionsObserved  []string
	SpeechObserved   string
	ClarifyObserved  []string
	EvidenceObserved []string
	Stage            StageTimings
	Reconciled       bool   // observed matches Expected.Outcome + at-least-one of intents/speech/clarify
	ReconcileDetail  string // human-readable why/why-not
	Error            string // error class if the pipeline errored
	ProviderMode     Mode
	ColdRun          bool // first dispatch for this (language,category)
	WarmedUp         bool // warm-up, results discarded by the runner
}

// --- shared helpers ------------------------------------------------

// ErrUnsupported is returned by providers that refuse a request before
// the pipeline runs. The runner maps this to NOT_EVALUATED at the case
// level — not a failure, an unmeasured.
var ErrUnsupported = errors.New("provider refused")

// NewTimings is a zero helper.
func NewTimings() StageTimings { return StageTimings{} }

// now returns time.Now; centralized for tests.
var now = time.Now
var nowMu sync.Mutex

// Since returns monotonic-safe ms elapsed. Test code overrides now().
func Since(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
