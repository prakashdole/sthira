// Runtime is the seam where the actual middle-model inference is
// plugged in. Three implementations ship:
//
//   - StubRuntime — deterministic; reads a fixture file and returns
//     it as a successful proposal. Used by tests. Never reports Ready
//     in production.
//   - SubprocessRuntime — structural stub. First Propose call returns
//     ErrRuntimeUnavailable. Production deployment must replace this
//     with a real adapter; until then, the orchestrator surfaces
//     PipelineState=MODEL_UNAVAILABLE.
//   - HTTPClientRuntime — production-shaped call through Client to
//     the pinned private vLLM endpoint.

package middleworker

import (
	"context"
	"errors"
)

// Runtime is the inference seam. Implementations MUST be safe for
// concurrent use: the Worker dispatches calls through a goroutine
// pool of fixed size, and a single Runtime instance backs them.
type Runtime interface {
	// Propose performs one inference call. The implementation
	// MUST honor ctx cancellation. The returned Proposal is
	// wire-ready; the worker wraps it into a ResponseEnvelope.
	Propose(ctx context.Context, req RequestEnvelope) (*ProposeOutput, error)
	// Revision returns the loaded model's revision identifier
	// (e.g. "Qwen3-4B-Instruct-2507@sha256:..."). Empty until
	// LoadAndVerify completes.
	Revision() string
	// Digest returns the artifact digest string reported in
	// /health. May be empty until LoadAndVerify completes.
	Digest() (name string, sha256 string)
	// Languages returns the languages this runtime supports.
	// Empty means "unknown"; the worker fails closed until the
	// runtime reports at least one language.
	Languages() []string
}

// ErrRuntimeUnavailable is the typed sentinel that the SubprocessRuntime
// returns. The worker maps it to MiddleStateUnavailable.
var ErrRuntimeUnavailable = errors.New("middleworker: runtime unavailable")

// ErrInjectionDetected is the typed sentinel a runtime may return when
// the user payload contains a string that looks like an instruction
// attempting to override the system prompt. The worker surfaces it as
// MiddleStateMalformed (NOT a successful proposal). This is NOT a
// safety gate — the orchestrator's validator is — but it keeps the
// model adapter honest about what crossed the boundary.
var ErrInjectionDetected = errors.New("middleworker: prompt-injection-shaped input")
