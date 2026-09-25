package asrworker

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestStubRuntime_Transcribe_UnsupportedLanguage: the stub rejects
// a language it does not support at the typed-error level.
func TestStubRuntime_Transcribe_UnsupportedLanguage(t *testing.T) {
	r := NewStubRuntime()
	_, err := r.Transcribe(context.Background(), TranscribeRequest{
		RequestID: "R-1",
		Language:  "ta-IN",
		Samples:   []float32{0.1, 0.2},
	})
	if !errors.Is(err, ErrLanguageUnsupported) {
		t.Fatalf("expected ErrLanguageUnsupported, got %v", err)
	}
}

// TestStubRuntime_Transcribe_SilenceProducesEmptyText: silence at
// the input produces an empty transcript. This is the strongest
// possible "we don't fabricate" guarantee at the runtime level.
func TestStubRuntime_Transcribe_SilenceProducesEmptyText(t *testing.T) {
	r := NewStubRuntime()
	r.CannedSilence = "" // explicit
	res, err := r.Transcribe(context.Background(), TranscribeRequest{
		Language: "hi-IN",
		Samples:  make([]float32, 160),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "" {
		t.Errorf("silence must produce empty text, got %q", res.Text)
	}
	if res.Confidence != nil {
		t.Errorf("silence must produce nil confidence, got %v", res.Confidence)
	}
}

// TestStubRuntime_Transcribe_NonSilenceIsCanned: when not silent
// the stub returns its canned marker. Tests assert this; production
// must NEVER see this string because we never deploy the stub.
func TestStubRuntime_Transcribe_NonSilenceIsCanned(t *testing.T) {
	r := NewStubRuntime()
	res, err := r.Transcribe(context.Background(), TranscribeRequest{
		Language: "ml-IN",
		Samples:  []float32{0.1, -0.1, 0.1, -0.1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != r.CannedNonSilence {
		t.Errorf("non-silence must produce canned marker; got %q", res.Text)
	}
	// Confidence remains nil because the stub never claims a
	// calibrated probability.
	if res.Confidence != nil {
		t.Errorf("stub confidence must be nil; got %v", *res.Confidence)
	}
}

// TestStubRuntime_Transcribe_HonorsDeadline: when the stub is set
// to a latency greater than the caller's deadline, Transcribe
// returns before the latency completes.
func TestStubRuntime_Transcribe_HonorsDeadline(t *testing.T) {
	r := NewStubRuntime()
	r.LatencyMs = 500
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := r.Transcribe(ctx, TranscribeRequest{
		Language: "hi-IN",
		Samples:  []float32{0.1, 0.2},
	})
	elapsed := time.Since(start)
	if elapsed > 250*time.Millisecond {
		t.Errorf("transcribe ran past deadline: %v", elapsed)
	}
	if err == nil {
		t.Errorf("expected context error from deadline")
	}
}

// TestStubRuntime_CloseRejectsSubsequentCalls: closing the stub
// produces ErrRuntimeClosed on every later call.
func TestStubRuntime_CloseRejectsSubsequentCalls(t *testing.T) {
	r := NewStubRuntime()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := r.Transcribe(context.Background(), TranscribeRequest{
		Language: "hi-IN",
		Samples:  []float32{0.1},
	})
	if !errors.Is(err, ErrRuntimeClosed) {
		t.Fatalf("expected ErrRuntimeClosed, got %v", err)
	}
}

// TestStubRuntime_SupportedLanguagesHonorsRestrictivePair: the
// stub's supported languages must be hi-IN and ml-IN only.
// This proves Worker 5 does not infer all-language support from
// the model name.
func TestStubRuntime_SupportedLanguagesHonorsRestrictivePair(t *testing.T) {
	r := NewStubRuntime()
	got := r.SupportedLanguages()
	if len(got) != 2 {
		t.Fatalf("expected 2 languages, got %d (%v)", len(got), got)
	}
	want := map[string]bool{"hi-IN": true, "ml-IN": true}
	for _, l := range got {
		if !want[l] {
			t.Errorf("unexpected language: %q", l)
		}
	}
}

// TestSubprocessRuntime_BlockedByDefault: the subprocess stub must
// return ErrRuntimeUnavailable for every call until the real
// adapter is wired in. The orchestrator sees UNAVAILABLE state and
// the public handler returns a typed unavailable response.
func TestSubprocessRuntime_BlockedByDefault(t *testing.T) {
	cfg := DefaultSubprocessRuntimeConfig()
	r := NewSubprocessRuntime(cfg)
	res, err := r.Transcribe(context.Background(), TranscribeRequest{
		Language: "hi-IN",
		Samples:  []float32{0.1, 0.2},
	})
	if err == nil {
		t.Fatalf("expected error from blocked subprocess runtime")
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("expected ErrRuntimeUnavailable, got %v", err)
	}
	if res.Text != "" {
		t.Errorf("blocked runtime must NOT produce text, got %q", res.Text)
	}
}

// TestSubprocessRuntime_ReportsZeroArtifactMetadata: Revision and
// Digest are empty until a model is actually loaded. The orchestrator
// uses these to refuse Ready=true while they are empty.
func TestSubprocessRuntime_ReportsZeroArtifactMetadata(t *testing.T) {
	r := NewSubprocessRuntime(DefaultSubprocessRuntimeConfig())
	if r.Revision() != "" {
		t.Errorf("Revision must be empty pre-load: got %q", r.Revision())
	}
	name, digest := r.Digest()
	if name != "" || digest != "" {
		t.Errorf("Digest must be empty pre-load: got (%q, %q)", name, digest)
	}
	if got := r.SupportedLanguages(); len(got) != 0 {
		t.Errorf("SupportedLanguages must be empty pre-load; got %v", got)
	}
}

// TestSubprocessRuntime_CloseIsIdempotent: closing the blocked
// subprocess runtime is a no-op.
func TestSubprocessRuntime_CloseIsIdempotent(t *testing.T) {
	r := NewSubprocessRuntime(DefaultSubprocessRuntimeConfig())
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close twice: %v", err)
	}
}
