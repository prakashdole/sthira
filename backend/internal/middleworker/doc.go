// Package middleworker is the private P6 middle-model worker. It owns
// the bounded Go adapter to a private vLLM HTTP endpoint and the warm
// server that speaks the frozen contracts.MiddleWorkerRequest /
// contracts.MiddleWorkerResponse / contracts.WorkerHealth protocol.
//
// The package is isolated under its own go.mod; it does not import the
// orchestrator's contracts package. The wire format IS the contract.
//
// What ships:
//
//   - Client — bounded HTTP client to a private vLLM
//     /v1/chat/completions endpoint. Strict per-request context/output
//     caps, per-call deadline, no retries, no paid fallback, fail-closed
//     on oversized/malformed/extra-text/unsupported-schema responses.
//   - JSON-constrained decoding — extracts the assistant message
//     verbatim and runs a bounded strict-JSON decode against the frozen
//     schema. Extra text, markdown fences, leading prose, duplicate
//     keys, missing required fields, unknown enums all return
//     ErrMalformed. The decode does NOT validate semantics; the
//     orchestrator's independent validator does that.
//   - Runtime interface — three implementations:
//   - StubRuntime — deterministic; reads a fixture file and
//     returns it as a successful proposal. Used by tests; never
//     reports Ready in production.
//   - SubprocessRuntime — structural stub. First Propose call
//     returns ErrRuntimeUnavailable. The orchestrator surfaces
//     PipelineState=MODEL_UNAVAILABLE and never a fake proposal.
//     This is the explicit "blocked real-inference" surface.
//   - HTTPClientRuntime — production-shaped call through Client.
//     Real deployment requires the pinned vLLM runtime and
//     artifact hash (see MANIFEST below).
//   - Worker — warm lifecycle with bounded queue + bounded concurrency,
//     deadline-aware dispatch, language allow-list. /health reports
//     warm=true only after LoadAndVerify completes.
//   - Server — loopback HTTP server with three endpoints:
//   - GET /health → WorkerHealth
//   - POST /v1/chat/completions → MiddleWorkerResponse (typed)
//   - POST /shutdown → 200 on drain, 503 on deadline exceeded
//     Bearer-token auth from STHIRA_MIDDLE_WORKER_TOKEN; absent token
//     fails closed at startup.
//   - Eval driver under eval/ — repeatable invocation for BF16 and a
//     supported quantized candidate (AWQ-int4) against the reviewed
//     corpus. Records individual per-case outcomes and resource data.
//     Does NOT call paid endpoints; does NOT extrapolate concurrency
//     from weight size.
//
// Pinned model + runtime (subject to real-inference verification):
//
//   - model_id:  sarvamai/sarvam-30b (adopted stack candidate per D59;
//     supersedes earlier Qwen/Qwen3-4B-Instruct-2507 candidate)
//   - license:   Apache-2.0 (verified from the model card, source
//     register S11)
//   - params:    30B MoE total (128 experts, top-6 routed, 2.4B active
//     non-embedding parameters)
//   - thinking:  disabled (enable_thinking=false in chat template;
//     non-thinking structured JSON output)
//   - vLLM/SGLang: pinned via the deployed image tag (see Eval image
//     reference). Structured-output JSON-schema is supported
//     per source register S02. Requires vLLM PR #33942 / fork / hotpatch or SGLang.
//   - tokenizer: ships with the model repo (no separate tokenizer pin).
//   - trust_remote_code: true (declared in upstream model card).
//   - quantizations evaluated: FP8 (selected format, ~30 GB weights),
//     BF16 (reference ~60 GB). Actual artifact presence and SHA-256
//     are NOT_EVALUATED until hardware is available — see MANIFEST
//     and eval/HARDWARE_BLOCKER.md. Capacity planning accounts for
//     resident weight memory, not active parameter count.
//
// The schema fixture in schema/model_output.schema.json is the
// authoritative JSON Schema for the model's structured-output mode.
// vLLM consumes it directly when guided generation is enabled. The
// worker also runs the same decode + bound checks on the response
// so a vLLM version that drifts from the wire contract is caught
// before the orchestrator ever sees the proposal.
package middleworker
