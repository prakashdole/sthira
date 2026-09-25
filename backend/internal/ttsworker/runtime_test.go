package ttsworker

import (
	"testing"
)

// TestStubRuntimeRefusesToSynthesize: the stub never fabricates an
// audio byte stream; every Synthesize call returns
// ErrRuntimeUnavailable.
func TestStubRuntimeRefusesToSynthesize(t *testing.T) {
	r := NewStubRuntime()
	defer r.Close()
	if got := r.Revision(); got != "" {
		t.Fatalf("stub revision must be empty, got %q", got)
	}
	if got := r.Languages(); got != nil {
		t.Fatalf("stub languages must be nil, got %v", got)
	}
	if got := r.Voices(); got != nil {
		t.Fatalf("stub voices must be nil, got %v", got)
	}
	_, err := r.Synthesize(RequestContext{}, "hello", "en-IN", "")
	if err == nil || err != ErrRuntimeUnavailable {
		t.Fatalf("expected ErrRuntimeUnavailable, got %v", err)
	}
}

// TestSubprocessRuntimeRefusesToSynthesize: the structural stub
// for the production subprocess adapter returns
// ErrRuntimeUnavailable until authorized artifacts land. Wiring the
// real adapter is gated on the authorized-artifact acceptance
// criteria.
func TestSubprocessRuntimeRefusesToSynthesize(t *testing.T) {
	r := NewSubprocessRuntime()
	defer r.Close()
	_, err := r.Synthesize(RequestContext{}, "hello", "en-IN", "")
	if err == nil || err != ErrRuntimeUnavailable {
		t.Fatalf("expected ErrRuntimeUnavailable, got %v", err)
	}
}

// TestSubprocessRuntimeHasNoLanguages: production wires a real
// adapter with at least one SupportedLanguages entry; the stub
// exposes none, so the worker refuses Ready=true.
func TestSubprocessRuntimeHasNoLanguages(t *testing.T) {
	r := NewSubprocessRuntime()
	if got := r.Languages(); got != nil {
		t.Fatalf("subprocess stub languages must be nil, got %v", got)
	}
	if got := r.Voice("en-IN"); got != "" {
		t.Fatalf("subprocess stub voice must be empty, got %q", got)
	}
}
