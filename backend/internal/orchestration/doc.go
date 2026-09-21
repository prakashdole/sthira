// Package orchestration owns the P6 voice pipeline. It runs in the main
// backend module (no private deps) and is the seam between the public
// /api/v3 HTTP boundary and the typed private worker protocols.
//
// Pipeline shape:
//
//	audio → ASR (or direct transcript) → scoped context → middle worker
//	→ independent validator → approved template → optional TTS
//
// Hard constraints (from plan/p6-contract.md and the lane prompt):
//
//   - Go owns correlation IDs, bounds every stage, and controls every
//     stage. The orchestrator NEVER accepts a client-supplied proposal as
//     a substitute for running the production pipeline. The legacy
//     /api/v3/voice/commands endpoint is unchanged; the new pipeline is
//     served by /api/v3/voice/process.
//
//   - ASR, middle, TTS each carry a separate bounded admission budget
//     (queue depth + concurrency), enforced by the BoundedQueue primitive.
//     Total and per-stage deadlines are honored; cancellation propagates
//     through context.Context to every stage.
//
//   - Results that arrive after a withdrawal or cancellation are
//     discarded; correlation map keyed by request_id and stage keeps
//     stale results from being delivered as guidance. The validator
//     runs against the freshly-revalidated scoped context, not the one
//     the model saw.
//
//   - After inference and before returning usable guidance or audio,
//     the orchestrator revalidates the source version / template
//     version / route / template eligibility. A failure returns an
//     explicit unavailable/clarify state with no unsafe action. Voice
//     results never turn into a reservation, arrival, transfer,
//     emergency call, or capacity mutation — that is enforced by what
//     the validator allows the proposal to reference, not by the
//     pipeline running fewer stages.
//
//   - Text/touch and cached non-operational fallback remain available
//     on inference failure; simple explicit camera controls bypass
//     model work under the agreed contract. Silence is allowed; the
//     orchestrator never synthesizes progress chatter.
//
//   - Metrics are low-cardinality timing/queue/error counters. The
//     orchestrator NEVER logs audio bytes, transcripts, locations, or
//     credential material. Workers receive no government credentials
//     and have no database write capabilities.
//
// This package is the orchestrator-side seam; the public handler file
// under backend/internal/httpserver/voice_process.go mounts it onto
// /api/v3/voice/{transcriptions,process,speech}. Coordinator owns
// server.go / main.go / OpenAPI wiring.
package orchestration
