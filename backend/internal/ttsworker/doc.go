// Package ttsworker is the private P6 Text-to-Speech (TTS) worker.
//
// It implements the frozen private protocol from plan/p6-contract.md and
// consumes the typed envelopes from contracts.TTSWorkerRequest /
// contracts.TTSWorkerResponse / contracts.WorkerHealth via JSON only.
// The package is isolated under its own go.mod so the orchestrator and
// other Go code can compile and ship without taking on the heavy
// dependencies of a neural runtime.
//
// Design constraints (worker prompt):
//
//   - Approved-template render is the only input path. The Go renderer
//     produces the text from a frozen template key and a structurally
//     validated argument object. Arbitrary user / model prose never
//     reaches the synthesis endpoint. Test/demo (synthetic-only)
//     templates carry an unapproved status flag that gates their use
//     outside isolated test configuration; no filler acknowledgments for
//     map movement are shipped.
//
//   - Warm, bounded, cancellable synthesis with actual codec/output
//     byte and decoded-duration limits. Structured unavailable states
//     (UNAVAILABLE, AUDIO_UNAVAILABLE, UNSUPPORTED_LANGUAGE, TIMEOUT,
//     CANCELED) are returned without retry; the orchestrator degrades to
//     the validated on-screen text fallback. The worker NEVER returns a
//     canned transcript / dummy audio as a successful synthesis.
//
//   - Parler-TTS artifact inventory (model_id, revision, license,
//     hardware, voices, languages, remote-code flag, runtime SHA placeholders)
//     is exposed via /health. Inventory PendingCritical != empty and any
//     unverified digest refuse Ready=true.
//
//   - The audio cache identity MUST include (text, template_key,
//     template_version, source_version, language, model_revision,
//     voice_revision, synthesis_settings) so a parameter change or a
//     source withdrawal deterministically invalidates the cache. Cache
//     storage is bounded (LRU + max bytes); entries expire on TTL.
//
//   - Audio invalidation on source withdrawal is wired through a real
//     source-version check / event source. A purge helper tested alone is
//     not sufficient: the worker must drop cached audio when the
//     source_version is observed to have advanced, EVEN WHEN the cache
//     was updated milliseconds earlier or another goroutine is mid-serve.
//     A request that races withdrawal never receives audio.
//
//   - On audio failure the validated on-screen response from the
//     orchestrator remains. The worker does not invent translations,
//     ISL approval or emergency wording; no extra audio publication
//     protocol that conflicts with P5 signed resources is built.
//
//   - No retained raw audio: synthesized PCM bytes are scrubbed before
//     the response returns. Cache storage keeps only canonical
//     content-addressed identifiers (SHA-256) and the bytes themselves;
//     decoded sample buffers are zeroed before the runtime returns.
//
// The runtime adapter (Python or native) is wired at startup via the
// Runtime interface; production deployment is required to provide a
// real Runtime that satisfies the contract. Tests run against a
// deterministic stub runtime; the blocked real-inference evidence is
// recorded in plan (or a successor) before this worker is allowed to
// report Ready=true to the orchestrator.
package ttsworker

// This file is intentionally non-empty so the package documents the
// constraints directly. All implementation lives in audio.go,
// cache.go, inventory.go, runtime.go, server.go, templates.go and
// worker.go in this directory.
