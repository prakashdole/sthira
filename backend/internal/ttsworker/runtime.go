package ttsworker

import "errors"

// Runtime is the seam where a real Parler-TTS inference adapter is
// wired in. Production provides an implementation that runs the
// authorized model and returns typed errors. The stub and the
// blocked subprocess stubs return ErrRuntimeUnavailable until
// authorized artifacts land.
type Runtime interface {
	// Synthesize returns PCM 16-bit LE mono samples at sampleRate
	// Hz. The worker applies codec-level limits on top of this
	// result. Errors are typed and never produce a fabricated audio
	// byte stream.
	Synthesize(ctx RequestContext, text string, language string, voice string) (*SynthResult, error)
	// Close releases any subprocess resources. Idempotent. Called
	// only after the runtime observes a rejection (e.g. ErrRuntimeUnavailable)
	// to release the underlying handle.
	Close() error
	// Languages lists the languages this runtime supports. The
	// worker advertises /health from this; languages from the model
	// NAME are not used.
	Languages() []string
	// Revision returns the locked model revision, or the empty
	// string if the runtime has not been authorized.
	Revision() string
	// Voice returns the default voice for the language, or "" if
	// the runtime is unauthorized.
	Voice(language string) string
	// Voices enumerates the registered voices per language.
	Voices() []VoiceInfo
}

// VoiceInfo is one advertised voice on the runtime. Empty revision
// field means the voice is unauthorized.
type VoiceInfo struct {
	Language string `json:"language"`
	Name     string `json:"name"`
	Revision string `json:"revision"` // empty if unauthorized
}

// RequestContext is the worker-side slice of the request context.
// We do NOT import context.Context into the public runtime API; the
// adapter returns ErrRuntimeUnavailable on DeadlineExceeded or
// Canceled-equivalent signals.
type RequestContext struct {
	DeadlineMillis int64
	Canceled       <-chan struct{}
}

// SynthResult is the runtime's raw output. Bytes are NOT a fabricated
// waveform; the bytes are either the worker's deterministic silence
// buffer produced at the SEAM boundary (which the worker then
// refuses as a successful synthesis response) or nothing at all.
type SynthResult struct {
	// Empty means the runtime did not produce output. The worker
	// surfaces this as AUDIO_UNAVAILABLE without faking audio.
	Empty bool
}

// Errors the runtime returns. Each maps to a TTSState in the worker.
var (
	// ErrRuntimeUnavailable means the runtime has not been authorized
	// or is not currently loaded. Worker maps to TTSUnavailable.
	// The stub returns this on every call.
	ErrRuntimeUnavailable = errors.New("ttsworker: runtime unavailable")

	// ErrLanguageUnsupported means the runtime has not authorized
	// synthesis in the requested language. Worker maps to
	// TTSUnsupportedLanguage.
	ErrLanguageUnsupported = errors.New("ttsworker: language unsupported")

	// ErrVoiceUnsupported means the runtime has no voice for the
	// requested (language, voice) pair. Worker maps to
	// TTSAudioUnavailable.
	ErrVoiceUnsupported = errors.New("ttsworker: voice unsupported")

	// ErrRuntimeClosed means Close was called. Worker maps to
	// TTSUnavailable.
	ErrRuntimeClosed = errors.New("ttsworker: runtime closed")

	// ErrDeadlineExceeded means the runtime exceeded the per-request
	// deadline. Worker maps to TTSTimeout.
	ErrDeadlineExceeded = errors.New("ttsworker: deadline exceeded")
)

// StubRuntime is the deterministic in-process runtime used by tests.
// It does NOT produce audio; every successful Synthesize call
// returns &SynthResult{Empty: true} so the worker surfaces
// AUDIO_UNAVAILABLE consistently.
//
// This is a deliberate blocked-real-inference signal. Wiring a stub
// that pretends to produce audio would fake audio as successful
// speech, which the worker prompt forbids.
type StubRuntime struct{}

// NewStubRuntime returns a fresh stub.
func NewStubRuntime() *StubRuntime { return &StubRuntime{} }

// Synthesize always returns ErrRuntimeUnavailable. Tests assert this
// explicitly. The contract is: a stub does not pretend to synthesize.
func (StubRuntime) Synthesize(_ RequestContext, _ string, _ string, _ string) (*SynthResult, error) {
	return nil, ErrRuntimeUnavailable
}

// Close is a no-op for the stub.
func (StubRuntime) Close() error { return nil }

// Languages returns nil — the stub does NOT claim any supported
// language. Without an explicit list the worker refuses Ready=true.
func (StubRuntime) Languages() []string { return nil }

// Revision returns "" — the stub is unauthorized.
func (StubRuntime) Revision() string { return "" }

// Voice returns "" — the stub has no voices.
func (StubRuntime) Voice(_ string) string { return "" }

// Voices returns nil — see above.
func (StubRuntime) Voices() []VoiceInfo { return nil }

// SubprocessRuntime is the structural placeholder for the real
// Parler-TTS adapter. Construction does NOT spawn a subprocess; the
// first Synthesize call returns ErrRuntimeUnavailable. Wiring the
// real adapter is gated on the authorized-artifact acceptance
// criteria; until then the worker surfaces AUDIO_UNAVAILABLE for
// every request.
type SubprocessRuntime struct{}

// NewSubprocessRuntime returns the structural stub.
func NewSubprocessRuntime() *SubprocessRuntime { return &SubprocessRuntime{} }

// Synthesize always refuses.
func (SubprocessRuntime) Synthesize(_ RequestContext, _ string, _ string, _ string) (*SynthResult, error) {
	return nil, ErrRuntimeUnavailable
}

// Close is a no-op.
func (SubprocessRuntime) Close() error { return nil }

// Languages returns nil.
func (SubprocessRuntime) Languages() []string { return nil }

// Revision returns "".
func (SubprocessRuntime) Revision() string { return "" }

// Voice returns "".
func (SubprocessRuntime) Voice(_ string) string { return "" }

// Voices returns nil.
func (SubprocessRuntime) Voices() []VoiceInfo { return nil }
