// SubprocessRuntime — structural stub. The first Propose call returns
// ErrRuntimeUnavailable. The orchestrator surfaces
// PipelineState=MODEL_UNAVAILABLE and never a fake proposal.
//
// This runtime is the explicit "real-inference is blocked" surface.
// Production deployment replaces this with a real adapter wired to
// the pinned vLLM endpoint through Client. Until that adapter exists,
// the worker reports Warm=false and Ready=false in /health and
// refuses dispatch.

package middleworker

import "context"

// SubprocessRuntime is the structural stub.
type SubprocessRuntime struct{}

// NewSubprocessRuntime returns the blocked-real-inference stub. It
// never returns a successful proposal. The /health response advertises
// Warm=false until a real adapter replaces this stub.
func NewSubprocessRuntime() *SubprocessRuntime {
	return &SubprocessRuntime{}
}

// Propose always returns ErrRuntimeUnavailable.
func (s *SubprocessRuntime) Propose(_ context.Context, _ RequestEnvelope) (*ProposeOutput, error) {
	return nil, ErrRuntimeUnavailable
}

// Revision returns the empty string; the worker treats empty
// revisions as "model not loaded".
func (s *SubprocessRuntime) Revision() string { return "" }

// Digest returns the empty string pair; same reason.
func (s *SubprocessRuntime) Digest() (string, string) { return "", "" }

// Languages returns an empty slice; the worker treats this as
// "no languages reported" and fails closed.
func (s *SubprocessRuntime) Languages() []string { return nil }
