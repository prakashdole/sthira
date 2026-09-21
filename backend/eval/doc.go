// Package evalroot documents the P6 evaluation harness.
//
// eval is a strictly-evaluation directory. It owns the corpus schema, the
// runner, the deterministic harness-validator provider, and a small HTTP
// provider that drives real workers via the frozen wire contracts. It does
// NOT own production validators, providers, or worker adapters; those live
// in their respective worker modules.
//
// Three things must stay true at all times:
//
//  1. No real personal data in Git. Every case carries Provenance with
//     Kind in {SYNTHETIC, CONSENTED}. Only SYNTHETIC fixtures are
//     committed; CONSENTED cases are loaded behind a -consent-manifest
//     flag and never live in source control.
//  2. Deterministic provider validates the harness only. A real run is
//     opt-in via flags; missing reviewers/samples yield NOT_EVALUATED,
//     never zero-failures or 100% success.
//  3. The case Expected outcome is fixed BEFORE execution. The runner
//     can update per-case timings but never rewrites the expected
//     field. A test run that drifts from expected fails honestly.
package evalroot
