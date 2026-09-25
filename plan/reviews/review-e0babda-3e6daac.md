# Review of Worker A e0babda and Worker B 3e6daac

Date: 2026-09-21. Verdict: NOT ACCEPTED; integration and corrections remain. No product code changed by this review.

## Checkout facts

CLEAN remains de0b4e1. A is in `/Users/apple/Documents/Projects/MonitoringZ-wa` on codex/p567-backend-corrections at e0babda. B is in `/private/tmp/mz-worktrees/p567-inference-corrections` on codex/p567-inference-corrections at 3e6daac. Neither lane is integrated. Both branched from b0b9772, not the prompt commit; cherry-pick their actual new commits onto CLEAN's descendant rather than replacing its tree (which would lose prompt/review documents).

A commits inspected: 2747ca5, 0ce0171, d1dc5a6, aee4df9, e0babda. B commits inspected: 5e98cf5, f3c747b, 40a95c8, 7e2f0ad, 3e6daac. B has pre-existing uncommitted modifications to plan/evidence/security-checks.json and security-checks.md. They were inspected selectively and preserved; these are not part of the committed review baseline.

## Confirmed failures and gaps

1. **New public pipeline regression (A).** orchestrator.go Process decodes raw PipelineRequest into contracts.ModelOutput. The real handler passes the original request body, so jurisdiction/input/render are rejected. Existing TestVoiceProcess_HandleProcessHappyPath fails: 400 UNKNOWN_FIELD, `json: unknown field "jurisdiction"`, expected 200. The orchestration package alone passes, showing why component tests were insufficient. User request limits also must not be confused with the 64 KiB model-response limit.

2. **Publication lifecycle still incomplete (A).** Publisher.WithObserver is a stored field, not production lifecycle integration; production main has no NewPublisher/observer wiring despite comments saying otherwise. Added promotion/withdrawal wrappers and an in-process fan-out bus do not prove cross-process invalidation or authority enforcement. Original promotion/current-selection/bypass/legacy attribution gaps remain. TestA3_LegacyUnattributedManifestNotPromoted only reads a row and logs the defect; it never calls PromoteManifest. Its name is not evidence of denial.

3. **New database tests fail (A).** On a unique real PostgreSQL/PostGIS database migrated through revision 7, TestA3_LockOrderProbe and TestA3_LifecycleInvalidatesAcrossInstances fail inserting into nonexistent `artifacts`. Running both packages also exposes duplicate manifest primary key MNF-LEGACY-1. A's handoff ran with DSN unset. The lock probe is not a controlled lock-order interleaving even after fixtures are repaired. Connection errors with an explicitly supplied DSN must fail, not skip.

4. **Template and eligibility corrections partial (A).** Built-ins now synthetic and extra nonempty action fields rejected; preserve those improvements. Raw model-field presence is still not proven by typed zero-value checks. Template/source lifecycle and caller argument fact binding remain incomplete. scoped.go still assumes a one-day interval, sorts IDs and writes PermittedRank; replacing PartySize=1 with 0 makes zero-free inventory pass `free < partySize`. A comment calling zero unconstrained does not provide an unknown-eligibility representation or accepted ranking policy.

5. **New malformed-WAV panic (B).** Real Ogg/WebM B1 tests pass. However decodeRIFFWAV treats 0xffffffff as an unknown size for any chunk, then slices the fmt chunk without a bound. A 44-byte RIFF/WAVE with fmt size 0xffffffff panics with slice bounds [:4294967315] capacity 44. Temporary direct-decoder regression reproduced this and was removed. Unknown-length handling must be limited to the trusted streaming data case, with safe bounds for every chunk.

6. **Real model integration still not implemented correctly (B).** ASR _load_model opens ONNX sessions but never executes them; active audio returns literal `[unverified:real-inference-stub]`. TTS uses a generic AutoModel and generate(text=..., language=..., voice=...) with an assumed .wav result; no verified selected-model API/tokenizer path or matching dependency pin is supplied. Both readiness handlers report ready before any model load. A real subprocess probe using only dummy `{}` files at the required paths returned ready for both ASR and TTS. Languages/revisions and comments claiming O03/O11 resolved are not approval evidence. This is an internal implementation gap, separate from unavailable weights/GPU or language approval.

7. **IPC remains incompletely bounded (B, source inspection).** Both dispatchers hold mutexes across blocking stdin.Write before starting per-call timeout/select; a child not reading can block cancellation and Close. Close itself writes shutdown before killing. ASR starts the demux before registering the startup waiter (race); malformed-response paths assign demuxErr without the mutex used by readers. Malformed/unknown messages do not consistently kill/reap/reset as comments claim. Correlation improved, but the handoff's complete-exchange serialization description is inaccurate. Focused TTS B3 race tests pass; these missing failure paths still need targeted proof.

8. **Evaluation still violates real API (B).** Middle sends source_version and data_version_hint, neither present in PipelineRequest. It decodes PipelineResponseWire directly rather than standard data envelope. A probe supplying an actual-shaped data envelope returned empty status with nil error. TTS mixes public speech and private synthesis fields, sends empty text to a worker expected to synthesize rendered text, and invents source/template version 1. Tests still use replacement handlers. Endpoint-specific actual-handler conformance is missing.

9. **Sarvam config still speculative (B).** A new hand-written Gemma-style template is now wired as an override; code itself calls it a placeholder requiring verification. A Jinja variable set to false but never consulted does not establish model thinking control. Exact template/revision and compatible serving options need evidence; invalid tokens need not fail loudly. Preserve model selection, remove invented deployment guarantees.

10. **P7 smoke/security gaps remain (B).** Smoke runner still references uninitialized STHIRA_PID in its EXIT trap, uses second-resolution resource names without ownership flags, and seeds no successful business data. New business smoke uses an unseeded jurisdiction/query, checks body length rather than actual candidates, and does not make every check failure fatal. Security script still invokes Trivy without a findings exit code and reports aggregate PASS with required scanners NOT_RUN. Reporting binary versions is not pinning. Preserve improved status-0 handling and corrected root SPDX downloadLocation.

## Checks run in this review

- A orchestration package: PASS.
- A existing public pipeline happy path: FAIL, 400 unknown jurisdiction.
- A new store/offlinedelivery TestA3 tests with real migrated disposable DB: FAIL on nonexistent table and shared fixture ID collision; one misleading legacy test passes without attempting promotion.
- B actual compressed-audio TestB1 tests: PASS.
- B TTS TestB3 tests with race detector: PASS (templates package had no selected tests, not extra coverage).
- B malformed fmt-chunk probe: FAIL by panic.
- B real adapter subprocess ready probes with dummy artifacts: false READY reproduced for both.
- B evaluation standard-envelope probe: FAIL, empty outcome without error; outbound unsupported fields observed.

Temporary probes and owned DB removed. No broad load, real model inference, GPU benchmarks, full combined suite or scanner audit was claimed or run. Stop evidence collection here: acceptance is already disproved. Further broad green counts cannot resolve these concrete failures.

Next task: plan/next-action-integration-repair.md. One worker owns the combined correction and final verification so endpoint mismatches cannot be left between lanes. Do not advance to P8 or declare gate B ready.
