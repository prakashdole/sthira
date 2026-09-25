# Single-worker execution: repair integrated P5/P6, adopt Sarvam, complete P7 evidence

Review date: 2026-09-21. Reviewed implementation HEAD: `81a5eb9` on uppercase `CLEAN`. No product code was changed by this review. This document is the complete next execution prompt for ONE worker; no parallel agents are required.

## Review result and evidence

All twelve worker lanes and the integration commit are in history. That proves delivery of code, not acceptance. Keep P6/P7 IN_PROGRESS and gate B NOT_READY; reopen the outstanding P5 publication/queue engineering rows. Preserve completed client corrections and earlier evidence.

Verified in this review:

- Full backend test run with both `STHIRA_TEST_DSN` and `STHIRA_TEST_ADMIN_DSN` configured, using uniquely owned migrated databases: **703 test/subtest pass events, 0 fail, 0 skip**. This does not include uncompiled tagged suites. An earlier run without the admin setting had 672 passes, 9 failures and 22 skips; its failure details were not retained, so repeatability is not established by the later pass. Preserve diagnostics if it recurs; do not attribute failures to the environment without evidence.
- Separate modules: ASR 58, middle 53, TTS 60, evaluation 36 and load harness 5 pass events, no failures/skips. These are component tests, not real speech/model benchmarks.
- Explicit process suite on a uniquely owned migrated database, with both the crashtest build tag and process-test environment flag: `TestCrossProcessLastSpace` and `TestCrashAfterCommitBeforeResponse` both passed. Owned review databases were removed.
- A temporary probe marshalled `contracts.ScopedContext` into the actual middle worker type: `json: cannot unmarshal object into Go struct field ScopedContext.verified_routes of type []middleworker.RouteCandidate`.
- A temporary probe called the production validator used by `Process`: a proposal with schema `WRONG`, another request ID, empty data version and ZOOM direction `UPWARD`, steps 99 returned nil. The richer relationship validator is not a replacement for the full shape/enum/correlation checks.
- The two probes were outside the repository and removed. No giant raw metric files were read. Their filenames/sizes and small summaries are sufficient for cleanup.

Important observed integration gaps:

1. `backend/cmd/sthira/main.go` constructs `NewWorkers` but never calls `SnapshotHealth`. Initial ready flags are false. Tests explicitly warm health in helpers; the production path does not. Independently, the TTS server returns its internal health shape (`runtime_languages`, inventory, metrics) rather than the common `WorkerHealth` fields expected by the client.
2. `backend/internal/middleworker/wire.go` differs from `contracts/context.go`: route map versus slice and candidate field names differ. The integration tests use replacement handlers, so they do not establish compatibility with the actual worker servers.
3. `orchestration/orchestrator.go` validates the public request's raw body, then only calls `Enforce` on the typed model proposal. It never invokes complete proposal shape/flat validation at that boundary. `HTTPWorkerClient` uses permissive JSON decoding and discards unknown/duplicate/trailing content. Worker response correlation compares locally generated values, not consistently the returned request identifiers.
4. `orchestration/registry.go` marks built-in translations as approved and version 1 without approval evidence. `validateArgsAgainstSchema` is a stub. `synthesize.go` substitutes caller values, lacks explicit jurisdiction selection, and the production scoped resolver cannot satisfy its empty-jurisdiction fallback. Process revalidates before TTS but not after a slow synthesis. TTS errors currently discard the already validated display result. Audio bytes are decoded then discarded; returned audio hashes have no implemented client retrieval path.
5. ASR and TTS `SubprocessRuntime` implementations remain placeholders, not implemented adapters merely awaiting GPUs. ASR only implements WAV decoding despite advertising compressed formats elsewhere. These are internal work, distinct from procurement or language-review blockers.
6. `store/publisher.go` closes the authority-gate transaction before opening the persistence transaction. Comments claiming one transaction are incorrect. Card authority checks omit several manifest-equivalent validity/authorization checks. Trusted publication uses ordinary JSON unmarshal. `NewPublisher` and invalidation methods still have no production lifecycle callers. The handoff's requested integration work was not completed.
7. `store/scoped.go` sorts eligible choices by facility ID despite describing authority-supplied order. It also assumes one person and a one-day stay in its context query. These cannot silently represent an arbitrary user's temporary-stay request.
8. The queue success validator accepts unknown endpoints with any data object, does not bind returned event type to the submitted event, and uses permissive JSON parsing. Its improved reconciliation behavior should be retained while closing those gaps.
9. `loadmodel` benchmarks a separate in-memory fixture/dummy-worker application. It is harness evidence, not production backend throughput. `run_scenario.sh` always writes raw metrics and hides k6 failure with `|| true`. Security scripts and a module graph hash do not constitute completed scans or a standards-format SBOM. The drain rehearsal observes shutdown, not completion of a controlled in-flight operation.

## Standing instructions

You are the single execution worker. Work sequentially in `/Users/apple/Documents/Projects/MonitoringZ` on **`CLEAN`**, never `main`. Inspect branch, status, recent commits and applicable instructions before edits. If the integration has changed since `81a5eb9`, inspect the delta and avoid overwriting later fixes. Preserve unrelated work, existing worktrees and all `.txt` files. No subagents, push, merge, history rewrite, production deployment or live government calls.

Read this document, `plan/p6-contract.md`, the active P5/P6/P7 sections of `plan/prompt.md`, `plan/voice-map-system-prompt.md`, `plan/parameters.md`, `plan/assurance.md`, `plan/open-decisions.md` and relevant code/callers/tests. Historical handoff claims are evidence to check, not instructions to preserve bugs. Read selectively; do not load raw metrics or scan model tensors.

Use one compact acceptance matrix in the existing phase ledger. For each stage: reproduce the concrete failure, fix at the responsible boundary using existing code, verify, inspect the diff, then create a local coherent commit. Do not claim a mock proves inference or a helper test proves production wiring. Stop after three unsuccessful attempts on the same issue without new evidence; record the exact blocker and continue independent stages where possible.

Use uniquely named disposable databases/directories/ports. Never truncate shared `sthira_test`, stop the team's PostgreSQL server, or delete another task's resources. Apply all ordered migrations with `ON_ERROR_STOP=1`. Test helpers must not hard-code a migration list that silently misses future migrations. Keep diagnostic output bounded and retain only compact failure evidence.

No new message brokers, generic provider frameworks, speculative caches, or ML-framework rewrite in Go. Existing Python/native inference dependencies may run privately behind Go. Fix broken integration rather than creating a second parallel architecture.

## Stage 1 — remove generated-log bloat and make tests report failures

Remove the seven tracked `loadmodel/reports/*/metrics.json` raw time-series files from the working tree and index. They are generated samples, not source or required runtime assets. Keep small `summary.json`, useful fixture-stat summaries, scripts, baseline methodology and concise findings. Inspect filenames/sizes only; do not read the million-line files. Do not delete arbitrary JSON files or model/tokenizer assets.

Add a narrow ignore rule for these raw outputs and appropriate generated logs/binaries. Change `loadmodel/scripts/run_scenario.sh` so summary-only output is the default. Make raw samples an explicit diagnostic opt-in written to an ignored or external artifact directory, with a size/time bound. Preserve k6's failing exit status after cleanup. Fixture readiness checks must fail when startup failed rather than accidentally testing another process on a fixed port.

Check `git ls-files` and a small mocked runner failure to prove raw metrics are no longer tracked/recreated by default and a failing k6 run fails the runner. A full load run is unnecessary for this change.

Deleting tracked files in a new commit does NOT remove their blobs from Git history. Inspect local/remote tracking metadata without pushing. Report whether raw-metric commits are reachable from known remote refs; refresh refs only if needed and available, without changing branches. GitHub generally rejects ordinary Git files above its per-file limit. Do not push oversized historical blobs, switch to LFS unnecessarily, force-push, reset, amend or filter history. If history cleanup is required, provide a precise separate proposal that accounts for team branches/worktrees and waits for explicit rewrite authorization. Continue code stages meanwhile.

Commit this cleanup independently.

## Stage 2 — record the user's model selection accurately

User-selected stack:

- ASR: **IndicConformer-600M-Multi**; resolve/pin the exact installed AI4Bharat artifact variant and language assets rather than treating the friendly name as a revision.
- Middle: **Sarvam-30B**, `sarvamai/sarvam-30b`.
- TTS: **Indic Parler-TTS**; resolve/pin the actual artifact and voices.

The user now explicitly supersedes the earlier Qwen-first/~5–6B total-model preference. Record that decision once; do not ask again whether Sarvam is allowed. Update active stack, contract, candidate manifest, serving/evaluation configuration and next-phase instructions. Preserve historical Qwen evidence as historical. Do not globally replace historical model names.

Primary reference checked during review: https://huggingface.co/sarvamai/sarvam-30b — model card declares Apache-2.0 and 2.4B **non-embedding active** parameters in the 30B MoE model. Verify exact revision/tokenizer/license/deployment support before implementation. User preference selects the model; it does not prove its accuracy or throughput on Sthira.

Capacity planning must account for total weights: approximately 60 GB decimal BF16 weights or 15 GB ideal 4-bit weights before scales, KV cache, activations, runtime and concurrency overhead. Active parameter count is not resident weight memory. Verify supported quantization, tensor/expert parallelism and offload behavior on actual pinned vLLM/hardware; never promise that a 2.4B-active model fits like a 2.4B dense model.

Keep output constrained to the existing display-action contract. Sarvam cannot decide safe land, invent routes, authorize capacity or perform calls/writes. Configure and test reasoning/output behavior so internal reasoning cannot leak into the JSON action channel or exceed budgets. No weight download, GPU rental or paid inference without concrete existing authorization; record missing hardware/budget instead of inventing a benchmark.

## Stage 3 — close the remaining P5 publication and queue boundaries

Read `store/publisher.go`, publication.go, source lifecycle, offlinedelivery/cache.go, publication_adapter.go, the P5 handoffs and queue worker/real HTTP tests.

Publication:

- Use existing strict offlinepkg parsers and real signature checks before publication. Validate nil/size/depth/duplicate/unknown/trailing input at the actual boundary.
- Perform authority validation, required row locks and persistence in **one transaction**. Bind source, package, jurisdiction, version, effective/expiry, supersession, current authorization and signing authority. Check card and manifest paths consistently. Do not fix the race by changing comments or rechecking outside the write transaction.
- Concurrent identical publication under the same identity must succeed idempotently; conflicting content must fail with the intended conflict classification. Test the same revision, not distinct revisions mislabeled as contention.
- Wire trusted publication and explicit staged-to-current promotion into the real authorized application lifecycle. Do not leave Store bypasses as the active path. Bind publication records to authoritative source/package lifecycle using the minimal necessary migration if required, with legacy-data handling. Do not blindly implement the handoff's suggested NOT NULL migration without inspecting populated data.
- Wire withdrawal/quarantine/supersession through actual lifecycle → cached HTTP delivery. Preserve signed revocation delivery. Test a cached/in-flight fetch racing withdrawal and subsequent reads from another instance under the documented consistency bound. A test manually calling Invalidate does not establish production propagation. Account for CDN immutable cards via current signed manifests and their explicit validity, not a claim of instantaneous global deletion.

Queue:

- Retain uncertain outcome/5xx/auth recovery behavior. Strictly validate responses for an explicit endpoint/operation allowlist. Unknown operations and 202-like nonterminal responses cannot become COMMITTED merely because data exists.
- Bind response semantics to submitted event/resource, including replacement IDs for transfer. A CANCEL response must not confirm an ARRIVE request. Reject duplicate keys, malformed error arrays and wrong-operation results without discarding uncertainty or allowing duplicate submission.

Regressions must use real signatures, DB transactions and the actual HTTP publication/write boundaries. Include expiry/withdrawal after validation but before persistence, stale republish, same-key response loss/restart, invalid success response and authorized reconciliation. Reopen only the relevant P5 rows; do not redo completed offline-client work without a failing regression.

## Stage 4 — connect the actual worker servers and enforce complete validation

Read main.go, orchestration/{http_client,workers,registry,orchestrator}.go, all three actual worker servers and common wire types. Their routes match; the defect is payload/lifecycle compatibility, not renaming endpoints.

- Add a bounded worker-health startup/refresh/recovery path. Production must become usable when correctly configured workers are actually warm, and become unavailable/recover when they fail/restart. Tests must launch the real app entrypoint instead of manually warming internal state that production never touches.
- Make actual middle context and TTS health conform to the single frozen wire schema. Reuse common definitions where feasible or provide generated/fixture conformance checks; do not maintain undocumented divergent structs. Verify nested candidate/route data, health/model/language fields, and response envelopes across module boundaries.
- Through the real `HTTPWorkerClient`, strictly decode bounded worker responses and validate complete proposal shape, schema version, required fields, status/intent/action enums, request ID, exact data version, language, count/size limits, tagged action shape and semantic relationships. Preserve raw proposal bytes until structural checks finish. Use existing validators; do not rely only on EnforceScopedContext or JSON validity.
- Check returned ASR/middle/TTS request IDs against server-generated correlation, not just locally generated IDs against each other. Validate artifact/language/version expectations. Unknown confidence stays unknown. Cancellation and obsolete requests must release budgets and prevent late output delivery.
- Retain actual source policy ordering, not alphabetical facility sorting. Context construction must distinguish general destination browsing from a party/date-specific stay choice. Do not manufacture one-person/one-day eligibility or promise nearest ordering without authoritative route-length data.
- Make worker auth/transport configuration fail closed for exposed/private deployment as designed. Empty tokens must not silently turn a network worker public. Do not expose pprof or model endpoints to citizen ingress.

First regression cases are the two review probes above. Then launch the **actual worker HTTP server implementations** with deterministic inference runtimes solely for contract tests. Exercise health, decode, cancellation, saturation and malicious worker output through the production client and entrypoint. Such tests prove integration, not ASR/TTS/model quality. Existing replacement-handler tests may remain but cannot satisfy this acceptance row.

## Stage 5 — finish scoped approved speech, fallback and usable audio

- Replace `validateArgsAgainstSchema` and permissive substitutions with the existing validated template implementation. Bind template key/language/version/approval to the current jurisdiction/source; arguments come from typed validated facts/IDs. Caller-supplied free text and model prose cannot become emergency speech.
- Built-in generated translations must remain synthetic/pending review. Load genuinely approved content only from recorded evidence. Default version 1 and `SyntheticOnly=false` are not approval. Missing content review blocks dependent speech but should preserve lawful non-operational UI/error fallback.
- Give `/voice/speech` an explicit jurisdiction and authoritative version contract; remove the empty/any-jurisdiction resolver fallback. Version zero cannot bypass checks. Update OpenAPI/clients/tests together with an explicit compatibility decision.
- Revalidate source/package/template/route state after slow inference **and after synthesis before delivery**, including revocation without version increments. Wire audio invalidation to the actual source/template lifecycle, not only an in-memory test broadcaster.
- Preserve validated text/display actions when optional TTS fails. Silent camera/display actions must not need TTS availability. Do not make a failed optional audio stage erase the useful response.
- Deliver playable audio via one bounded documented mechanism: inline bytes or a real retrievable scoped audio resource. An audio hash with no retrieval endpoint is not delivered speech. Enforce actual format/duration/sample/size and retention/cache policy; reuse P5 resource semantics where applicable without falsely signing unapproved content.

Test two jurisdictions with identical keys/versions, unapproved translation, injected template arg, source withdrawal during synthesis, invalid returned audio, unavailable TTS with useful text preserved, and retrieval/playable decode of the exact bytes produced. These are backend tests; do not start mobile UI work.

## Stage 6 — implement actual inference adapters and the compressed-audio path

Inspect the selected local artifact metadata first. ASR/TTS subprocess classes currently always refuse; replace placeholders with a small pinned Python/native worker bridge that actually loads the selected model once and executes inference. Keep runtime lifecycle, bounded IPC/requests, errors, cleanup and readiness honest. Do not claim this implementation is blocked solely by missing government APIs.

Implement the negotiated compressed input codec(s) and tested resampling, or explicitly restrict the contract until implementation is ready. The current 512 KiB/20s target cannot be met by 20s of 16kHz mono 16-bit PCM (640,000 sample bytes before headers). MIME checks or a hand-written WAV parser do not establish WebM/Ogg support. Use a maintained decoder with decoded sample/duration/memory limits; avoid unnecessary codec frameworks.

Wire Sarvam through the pinned private vLLM adapter, verifying the selected runtime's architecture and structured-output support, chat template, reasoning controls, token limits and quantization compatibility. Keep inference isolated from government credentials, DB writes and arbitrary tools.

Add actual runtime/worker startup commands and dependency/artifact locks. With available authorized hardware/assets, run one real recording through ASR → scoped Go context → Sarvam → validated actions/template → Indic Parler-TTS, and decode/listen/review the result with qualified language evidence. If hardware or weights are unavailable, complete implementation/contract verification and record exact NOT_RUN checks; do not substitute canned transcripts, silence or deterministic proposals as real inference.

## Stage 7 — make evaluation, security, performance and recovery evidence real

- Run the evaluation HTTP provider against the actual integrated endpoints/envelopes. Verify it extracts nested response data, passes jurisdiction/version/correlation correctly and checks both expected IDs and unsafe acceptance. Deterministic 20/20 results only test the synthetic harness. Keep O03/O11 missing real corpus/review separate; never invent consent or regional language acceptance.
- Exercise language-specific ambiguity, code-switching, noise, clipped audio and prohibited instructions. Record exact model/data/hardware hashes, denominators, latency/memory/throughput and failures. Do not choose a server count from the total user count or active parameter count.
- Keep current loadfixture results labelled in-memory/dummy baseline. Add bounded k6 smoke against the actual Go binary + migrated PostgreSQL + actual publication/stay paths; validate successful business responses and capacity invariants. Do not benchmark expected 401/503 as successful work. Larger load and real-worker capacity need controlled hardware/budget; never run 100K VUs on the team laptop by default.
- Run the pinned available static/security checks and report absent tools as NOT_RUN. Produce an actual supported SPDX/CycloneDX SBOM when claiming SBOM completion; a hash of go mod graph is only an inventory checksum. Test scanner failures/missing executables are not swallowed. Do not install every redundant scanner or invoke Strix without the agreed target/model/spend scope.
- Reuse recovery infrastructure, but prove a controlled in-flight request completes or is safely reconciled during graceful shutdown, not merely that the process exits. Restore an owned backup into a distinct DB and exercise the real server's idempotent replay, audit and capacity checks. Keep true DB failover and real worker-loss recovery distinct from process restart or backup restore. Missing infrastructure remains NOT_RUN.

## Stage 8 — final verification, ledger reconciliation and stop

Run relevant focused regressions after each stage. At integration completion:

1. Formatting, build and vet for every actual Go module: backend, its ASR/middle/TTS submodules, backend/eval and loadmodel. Root `go test ./...` does not traverse nested modules.
2. Backend tests with both test DSNs, ordered migrated owned DBs and retained compact failure details; explicit process tests using `-tags crashtest` and `STHIRA_RUN_PROCESS_TESTS=1`. Run recovery's separate tag/env requirements when claimed. Distinguish uncompiled/skipped tests from passing tests.
3. Actual worker-server protocol tests and the two hostile-output regressions, publication lifecycle races, queue reconciliation and speech fallback/retrieval checks. Preserve P3/P4 conservation/crash evidence.
4. Real inference/language/performance/security/restore checks only to the extent actually run. Record hardware/time/budget blockers without claiming the software is complete except for government integration.
5. Verify no raw metrics/log/model binaries/secrets enter staged changes. No history rewrite or push. Update active plan/model decisions and acceptance rows using tested commits; remove contradictory DONE claims while preserving historical evidence. Do not rewrite every plan file unnecessarily.

Final handoff: commit IDs by stage, exact revision checked, behavior fixed, tests passed/failed/skipped/not run, remaining internal defects, external decisions and next eligible action. Do not start P8 or declare gate B ready while the required P6/P7 evidence is incomplete. If only hardware or reviewer decisions remain, identify precisely what input is needed to run the remaining checks; do not manufacture another code phase.
