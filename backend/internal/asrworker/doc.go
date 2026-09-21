// Package asrworker is the private P6 ASR (Automatic Speech Recognition)
// worker. It implements the frozen private protocol described in
// plan/p6-contract.md and consumes the typed envelopes from
// contracts.ASRWorkerRequest / contracts.ASRWorkerResponse /
// contracts.WorkerHealth. The package is isolated under its own
// go.mod so the orchestrator and other Go code can compile and ship
// without taking on the heavy dependencies of a neural runtime.
//
// Design constraints (worker prompt):
//
//   - Warm or not (readiness tied to actual artifact loading, not to
//     process liveness alone).
//   - Bounded queue + bounded concurrency, never leak goroutines.
//   - Per-request deadline and cancellation; a worker that misses a
//     deadline returns a typed UNAVAILABLE/TIMEOUT state.
//   - No public network exposure: the server is bound to a caller-
//     chosen loopback address; the caller (orchestrator) supplies it.
//   - Confidence is reported only when the underlying runtime exposes
//     a calibrated value. A stub or a model that returns a constant
//     1.0 is reported as "unknown" (nil *float64).
//   - No retained raw audio on disk. Audio bytes are decoded into a
//     transient float32 buffer; the buffer and the original bytes are
//     overwritten before the response is returned. If a downstream
//     pipeline keeps references, that is its problem; we explicitly
//     label what survives the worker and what does not.
//   - Decompression-abuse / malformed-input rejection happens on the
//     actual decoded bytes (channels, sample rate, sample count,
//     duration), not on the client-declared Content-Type / size.
//
// This is the runtime side of the ASR boundary. The HTTP handler
// routes the request in; the worker lifecycle owns concurrency and
// readiness; the runtime interface is the seam where a real inference
// backend (Python subprocess today; Go-native later) is plugged in.
// Tests run against a deterministic stub runtime; production
// deployment requires the blocked real-inference evidence recorded
// in plan (or a successor) before this worker is allowed to report
// Ready=true to the orchestrator.
package asrworker
