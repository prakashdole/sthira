// Module middleworker is the private P6 middle-model (constrained LLM)
// worker. It implements the frozen private protocol described in
// plan/p6-contract.md and consumes the typed envelopes
// contracts.MiddleWorkerRequest / contracts.MiddleWorkerResponse /
// contracts.WorkerHealth. The package is isolated under its own go.mod
// so the orchestrator and other Go code can compile and ship without
// taking on the heavy dependencies of a model runtime.
//
// Design constraints (worker prompt):
//
//   - The worker never imports the orchestrator's contracts package; the
//     JSON wire format is the contract. Hand-rolled structs in this
//     module mirror contracts.* fields verbatim so that a malformed or
//     non-conforming wire envelope is detected here, not in shared Go.
//   - Bounded context and bounded output. The first candidate is
//     Qwen3-4B-Instruct-2507 (Apache-2.0, non-thinking, ~4B parameters).
//     The adapter passes the per-request context cap and the
//     per-request max-output-token cap; the model runtime is responsible
//     for honoring them. JSON-constrained decoding is NOT a safety
//     gate — the Go validator on the orchestrator side is. The
//     worker only enforces the wire shape.
//   - The worker binds to 127.0.0.1 only. Bearer token auth is required
//     when STHIRA_MIDDLE_WORKER_TOKEN is set; absence fails closed at
//     startup. No public network exposure.
//   - Warm-or-not: readiness is gated on actual artifact loading, not
//     process liveness. Until LoadAndVerify completes, /health reports
//     warm=false, ready=false.
//   - No paid external fallback, no tool execution, no network-enabled
//     model tools, no unbounded retries. One Transcribe call per
//     request; transport failures are reported as a typed UNAVAILABLE
//     state. Cancellation via ctx is honored at every boundary.
//   - Output is bounded: max output tokens, max response bytes, strict
//     JSON-schema decode, fail-closed on extra text, malformed JSON,
//     duplicate keys, unknown enum values, missing required fields,
//     unsupported schema, or output-token overrun.
//
// Real vLLM / model artifact execution is BLOCKED until the model
// weights are inventoried and a private GPU is available. Tests run
// against a deterministic stub runtime and against a fake HTTP
// transport; the production deployment requires the explicit
// real-inference evidence recorded in plan/ (or a successor) before
// this worker is allowed to report Ready=true.
module sthira/backend/internal/middleworker

go 1.23
