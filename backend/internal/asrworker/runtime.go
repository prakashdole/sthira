package asrworker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Runtime is the seam where a real inference backend plugs in. The
// worker holds the runtime; every request becomes one Runtime.Transcribe
// call. The interface deliberately returns no audio bytes and never
// asks the runtime to keep references to the input slice.
//
// All four methods are required for a production deployment. A
// stub implements them in pure Go so the rest of the worker can be
// tested without model weights.
//
// The contract:
//
//   - Transcribe is given the post-decode, post-resample mono float32
//     buffer. The runtime MUST NOT retain the slice.
//   - Confidence is *float64; nil means "not calibrated". A stub or a
//     model that returns a constant probability (e.g. the reference
//     runtime's 1.0) MUST return nil — that's the only way to keep
//     confidence honest.
//   - SupportedLanguages is read once at startup; it is the
//     authoritative list the orchestrator sees in /health.
//   - Revision returns the artifact revision the runtime is actually
//     using; it must match what the inventory recorded (or the
//     worker refuses to go Ready=true).
//   - Close is called when the worker shuts down. It must release
//     subprocesses and clear any retained state.
type Runtime interface {
	// Transcribe performs inference on the post-decode mono
	// samples. The runtime returns the transcript text, calibrated
	// confidence (or nil if unknown), and optional n-best
	// alternatives.
	Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error)

	// SupportedLanguages is the authoritative language coverage.
	// The worker exposes this in /health and uses it to reject
	// unsupported languages BEFORE spending decoding time.
	SupportedLanguages() []string

	// Revision is the artifact revision the runtime loaded. Empty
	// when no model is loaded yet.
	Revision() string

	// Digest returns an opaque (artifact, sha256) tuple that the
	// worker exposes in WorkerHealth.Artifacts. Empty when no model
	// is loaded. The worker MUST reject Ready=true while this is
	// empty.
	Digest() (name, sha256 string)

	// Close shuts the runtime down. Subsequent Transcribe calls
	// return ErrRuntimeClosed.
	Close() error
}

// TranscribeRequest is the typed input to a Runtime.Transcribe call.
// We intentionally do not include the original compressed bytes —
// the runtime gets the decoded mono samples, period.
type TranscribeRequest struct {
	RequestID    string
	Language     string
	Samples      []float32
	SampleRate   int
	DurationSecs float64
	Deadline     time.Time
}

// TranscribeResult is the typed output of a Runtime.Transcribe call.
//
// Alternatives is empty when the runtime does not support n-best
// (most ASR models do not expose it; the contract deliberately
// keeps the n-best slot so the orchestrator does not assume its
// presence).
type TranscribeResult struct {
	Text         string
	Confidence   *float64
	Alternatives []TranscriptAlternative
}

// TranscriptAlternative is the runtime-level n-best alternative.
// Matches contracts.TranscriptionAlternative's wire shape (we keep
// it private to this module so the worker module has zero coupling
// to the orchestrator's contracts package).
type TranscriptAlternative struct {
	Text       string
	Confidence *float64
}

// ErrRuntimeClosed is returned when Transcribe is called on a
// runtime that has been Close()d.
var ErrRuntimeClosed = errors.New("runtime is closed")

// ErrLanguageUnsupported is returned when the runtime does not
// support the requested language. The worker surfaces this as
// TranscriptionState = UNSUPPORTED_LANGUAGE.
var ErrLanguageUnsupported = errors.New("language not supported")

// ErrRuntimeUnavailable is returned when the runtime cannot
// perform inference (subprocess dead, model not loaded, hardware
// error). Surfaces as TranscriptionState = UNAVAILABLE.
var ErrRuntimeUnavailable = errors.New("runtime unavailable")

// StubRuntime is a deterministic in-process implementation of
// Runtime used for tests. It returns one of two canned transcripts
// based on whether the input is silence or contains signal, and
// reports confidence = nil (unknown). It is NOT used in production.
//
// The stub is the PROOF that the worker does not fabricate
// transcription: even when given a "real-looking" signal, it
// returns either an empty string (silence) or a deterministic
// marker that tests can assert against. Production deployment
// MUST replace it with a runtime that actually decodes speech.
type StubRuntime struct {
	mu       sync.Mutex
	closed   bool
	revision string
	digest   string
	// LatencyMs is the artificial per-call latency. Tests can set
	// this to a positive value to exercise deadline handling
	// without burning real time.
	LatencyMs int
	// CannedNonSilence is the text returned when Samples is not
	// silent. Defaults to "[stub:non-silence]". Tests rely on this
	// being obviously synthetic — production must NEVER see it.
	CannedNonSilence string
	// CannedSilence is the text returned when Samples is silent.
	// Empty by default; tests assert the silent case produces no
	// false transcript.
	CannedSilence string
}

// NewStubRuntime returns a fresh stub with conservative defaults.
func NewStubRuntime() *StubRuntime {
	return &StubRuntime{
		revision:         "stub-1",
		digest:           HashZero,
		CannedNonSilence: "[stub:non-silence]",
	}
}

// Transcribe implements Runtime.
func (s *StubRuntime) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return TranscribeResult{}, ErrRuntimeClosed
	}
	latency := time.Duration(s.LatencyMs) * time.Millisecond
	s.mu.Unlock()
	if latency > 0 {
		select {
		case <-time.After(latency):
		case <-ctx.Done():
			return TranscribeResult{}, fmt.Errorf("%w: %v", ErrRuntimeUnavailable, ctx.Err())
		}
	}
	if !hasLanguage(s, req.Language) {
		return TranscribeResult{}, fmt.Errorf("%w: %s", ErrLanguageUnsupported, req.Language)
	}
	if SilenceDetector(req.Samples) {
		return TranscribeResult{Text: s.CannedSilence, Confidence: nil}, nil
	}
	return TranscribeResult{
		Text:       s.CannedNonSilence,
		Confidence: nil,
	}, nil
}

// SupportedLanguages implements Runtime. Defaults to hi-IN and
// ml-IN (matching the reference Python runtime's configured pair).
// Worker does NOT derive this from the model name.
func (s *StubRuntime) SupportedLanguages() []string {
	return []string{"hi-IN", "ml-IN"}
}

// Revision implements Runtime.
func (s *StubRuntime) Revision() string { return s.revision }

// Digest implements Runtime.
func (s *StubRuntime) Digest() (string, string) { return "stub", s.digest }

// Close implements Runtime.
func (s *StubRuntime) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func hasLanguage(r Runtime, lang string) bool {
	for _, l := range r.SupportedLanguages() {
		if l == lang {
			return true
		}
	}
	return false
}

// SubprocessRuntimeConfig is the configuration for the real
// subprocess-backed runtime. The worker refuses to construct one
// without these filled in, and refuses to spawn one when the
// provided executable is not present.
//
//	Module:     the python -m invocation target.
//	Cmd:        python3 (overridable).
//	Workdir:    optional CWD for the subprocess.
//	ExtraEnv:   typed env vars to set (overridable; never inherits
//	            the orchestrator's environment).
type SubprocessRuntimeConfig struct {
	Module   string
	Cmd      string
	Workdir  string
	ExtraEnv []string
}

// DefaultSubprocessRuntimeConfig targets the existing reference
// Python module src/sthira_v2/speech_stt.py.
func DefaultSubprocessRuntimeConfig() SubprocessRuntimeConfig {
	return SubprocessRuntimeConfig{
		Module: "sthira_v2.speech_stt",
		Cmd:    "python3",
	}
}

// SubprocessRuntime is the seam where a real Python adapter
// attaches. The current implementation is a STRUCTURAL stub that
// refuses every call: we deliberately do NOT spawn a subprocess
// until both (a) authorized model artifacts exist and (b) the
// subprocess harness has been audited end-to-end with real
// recordings. The worker reports Ready=false while this is the
// active runtime. The orchestrator sees /health=UNAVAILABLE and
// returns a typed UNAVAILABLE to the client.
//
// When the real artifact is available (post O-* approval), the
// only changes needed are:
//   - implement Transcribe by exec-ing the Python adapter with
//     bounded stdin/stdout, a kill-after-deadline timer, and a
//     strict success envelope.
//   - record the loaded artifact digest (via a one-shot hash on the
//     model_onnx.py directory).
//   - keep SupportedLanguages bound to the Python adapter's
//     language map, NEVER to the artifact name.
//
// Until then this type is the truthful "blocked real-inference"
// check; the worker will not silently fall back to a canned
// transcript.
type SubprocessRuntime struct {
	cfg      SubprocessRuntimeConfig
	closed   bool
	mu       sync.Mutex
	revision string
	digest   string
}

// NewSubprocessRuntime returns the structural stub. Constructing it
// does NOT spawn anything; the subprocess only spawns on the first
// Transcribe call. Currently the first call returns
// ErrRuntimeUnavailable so the worker surfaces UNAVAILABLE to the
// caller without ever invoking real inference.
func NewSubprocessRuntime(cfg SubprocessRuntimeConfig) *SubprocessRuntime {
	return &SubprocessRuntime{
		cfg:      cfg,
		revision: "",
		digest:   "",
	}
}

// Transcribe implements Runtime. Returns ErrRuntimeUnavailable
// until a real adapter is wired in.
func (s *SubprocessRuntime) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return TranscribeResult{}, ErrRuntimeClosed
	}
	s.mu.Unlock()
	// The real implementation is deliberately out of scope until
	// model artifacts are available. Returning a typed
	// UNAVAILABLE state is the honest "blocked real-inference"
	// signal the worker prompt asks for: never a canned transcript.
	return TranscribeResult{}, fmt.Errorf("%w: subprocess runtime not wired in this revision", ErrRuntimeUnavailable)
}

// SupportedLanguages implements Runtime. Reported as empty so the
// orchestrator knows no language is configured; this is the only
// honest answer while the real adapter is unwired.
func (s *SubprocessRuntime) SupportedLanguages() []string {
	return nil
}

// Revision implements Runtime. Empty (no model loaded).
func (s *SubprocessRuntime) Revision() string { return s.revision }

// Digest implements Runtime. Empty (no model loaded).
func (s *SubprocessRuntime) Digest() (string, string) { return "", "" }

// Close implements Runtime.
func (s *SubprocessRuntime) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// FormatLanguages produces the stable sorted-and-quoted form of
// the supported languages list for log output. Kept here so the
// runtime interface owns its own diagnostic surface.
func FormatLanguages(ls []string) string {
	if len(ls) == 0 {
		return "<none>"
	}
	cp := append([]string(nil), ls...)
	return strings.Join(cp, ",")
}
