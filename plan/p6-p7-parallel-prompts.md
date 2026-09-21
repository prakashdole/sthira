# Parallel execution: finish P5, build P6, prepare P7

Prepared 2026-09-21 from clean branch `CLEAN`, HEAD `b9ffb69`. This is an execution plan, not phase acceptance. The current review inspected commits and source; it did not independently rerun the reported full test suite. Recheck HEAD before execution because another agent may still be working.

## Launch order

1. Give **Common instructions + Coordinator** to one chat first. Let it finish the contract freeze and report its commit.
2. Start up to **12 worker chats**, each receiving **Common instructions + exactly one worker prompt** below. Include the coordinator's contract commit. Each worker creates its own worktree from that commit. These are 12 separate workstreams, not 12 writers in one checkout.
3. If Terra still owns P5, send it the three P5 prompts as findings first. Either let Terra retain them, or explicitly hand each lane to another agent. Never run two implementations against the same lane.
4. Workers implement against the frozen contracts, with test doubles only for unavailable neighboring components. They must label this evidence honestly. Do not cherry-pick unfinished neighboring work to make an adapter appear integrated.
5. Return completed commits to the original coordinator and give it the **Integration** prompt. Only the coordinator integrates onto `CLEAN`. Independent P6 work can start before P5 closure; P6 integrated acceptance cannot bypass P5 defects. P7 preparation can start now; P7 gate B cannot pass before P6 acceptance.

The useful ceiling here is 12 workers: three P5, six P6, three P7. Further splitting would give multiple agents ownership of the same trust boundary or shared files. Running that many model/GPU/load jobs on one machine will not make them faster: serialize GPU benchmarks, load tests, and expensive scans; parallelize implementation and focused tests.

## Current P5 findings to reproduce, not dismiss as external blockers

- `backend/internal/offlineclient/sync.go`: the same-revision return does not establish the process freshness anchor after restart. It does not fully verify the cached card before deciding repair is unnecessary. Same-revision conflicting manifest content is not rejected. New revision handling resets expiry observations even when reusing the card.
- `backend/internal/offlineclient/storage.go`: `writeActiveGeneration` renames manifest and card separately. This is not a single atomic generation switch; interruption between renames can leave mixed files.
- The client's tombstone loader treats missing/zero-byte files as empty initial state, including after earlier use. Loss of recorded revocation knowledge must be distinguished from a genuinely fresh installation.
- `backend/internal/store/publication.go`: manifest/card ingestion uses ordinary JSON decoding and structure checks, compares two supplied checksum strings, but does not itself recompute the signed digest or verify the publisher. Record identity is not fully bound to payload identity. Identical republishing can overwrite withdrawal/quarantine metadata. Trace all callers to establish and repair the actual trusted boundary.
- `backend/internal/offlinedelivery/cache.go`: invalidation helpers have no production callers in the inspected tree. Source withdrawal is not automatically connected to them. Cache maps are unbounded; coalesced waiters cannot cancel their own wait.
- `backend/internal/offlinequeue/worker.go`: any 2xx is committed without validating an operation-specific success envelope. Exhausted 5xx responses become terminal failure, although a proxy/server failure can occur after commit. Uncertain authentication failures also need care: losing permission to inspect a result does not prove the earlier operation failed.

These are internal engineering concerns. O01/O05/O06/O07/O14 remain separate external gates. Do not claim that only external P5 blockers remain until the regressions below pass.

## Common instructions — include in EVERY new worker chat

You are implementing one bounded task for Sthira in `/Users/apple/Documents/Projects/MonitoringZ`. Read the governing instructions supplied in this chat and any applicable repository/parent instructions. Read `plan/p6-p7-parallel-prompts.md`, `plan/prompt.md`, `plan/decisions.md`, `plan/open-decisions.md`, `plan/tech-stack.md`, and the section-specific files below. Read only relevant implementation/callers/tests; no full-repository rewrite.

Workflow:

- Integration branch is uppercase `CLEAN`, never `main`. Inspect status, branch, HEAD, and relevant diffs first. Obtain the coordinator's contract-freeze commit; create a unique `codex/<task-name>` branch and isolated sibling worktree from that commit. Report its absolute path and base commit. Do not switch or edit the shared checkout. If the freeze has not arrived, perform read-only investigation and identify questions; do not invent shared APIs.
- Preserve existing work, Python reference code, unrelated files, and all `.txt` files. No push, merge, amend, reset, history rewrite, or production deployment. Commit your own verified coherent stages locally and provide the IDs to the coordinator.
- Own only the files assigned by the coordinator. Shared `server.go`, `main.go`, OpenAPI, phase ledger, root CI/build files and shared dependency locks belong to the coordinator unless explicitly delegated. Propose exact edits for those files in your handoff. Request a contract amendment instead of defining incompatible duplicate types.
- Go owns product logic. Existing Python/native inference frameworks can run in isolated private workers. No mobile UI implementation, no government API calls, no invented safe zones/routes/capacity, no model-triggered reservations/calls, and no external paid inference fallback.
- Real data, licenses, routes, stay policy and IdP remain unavailable unless explicitly supplied. Synthetic fixtures must be visibly non-operational and enabled only through isolated test configuration. Do not manufacture approvals, language coverage, confidence scores, model revisions, benchmark results, or hardware availability.
- Do not install/download large model weights, rent GPUs, execute paid APIs, or run Strix without a concrete authorized budget/target. Inventory existing artifacts by metadata first. Missing hardware or reviewers blocks real inference/language acceptance, not contract implementation. Use the toolchain pinned in the checkout; verify unfamiliar runtime options against the selected installed/pinned version.
- Reuse existing schemas, strict parsers, validators, storage and failure patterns. No speculative service framework, message broker, generic plugin architecture, or new cache without need. Keep errors, resource budgets, cancellation and trust validation intact.
- Begin corrections with focused observable regressions. Verify failure for the intended reason, then implement. For new behavior, test the real affected boundary and label stubs accurately. Each agent uses its own temporary directory and uniquely named disposable database, applies all ordered migrations with error-stop, and cleans up only resources it created. Never truncate shared `sthira_test`, kill another agent's server, or run competing migrations on its database.
- Run formatting, relevant build/vet/tests and inspect the final diff. Report exact revision, commands, pass/fail/skip, real versus fake boundaries, remaining blockers and shared-file integration instructions. A missing tool/DB/GPU is NOT a passing check. Avoid piping tests through commands that hide their exit status. Stop after the assigned lane; do not declare P5/P6/P7 DONE.
- Put a compact handoff in the coordinator-assigned lane-specific Markdown file. Do not edit another agent's handoff or create duplicate global trackers. Stop after three unsuccessful attempts on the same issue without new evidence and report it.

## Coordinator — run FIRST

You coordinate contracts and later integration; do not implement all workers' tasks. Follow Common instructions, except work directly on a clean `CLEAN` checkout for the contract stage. Recheck Terra's activity/status before editing. Preserve unrelated work and stop if shared edits cannot be separated.

Read the P6/P7 sections of `plan/prompt.md`, `plan/voice-map-system-prompt.md`, `plan/parameters.md`, `plan/assurance.md`, `backend/internal/contracts/model.go`, `backend/internal/httpserver/{handlers,server}.go`, `backend/internal/store/{context,place,choice}.go`, and existing P5 public types. Inspect reference `src/sthira_v2/{local_voice,speech_stt,multimodal,voice_commands,voice_map}.py`.

Freeze `plan/p6-contract.md` with exact typed request/response and error boundaries, file ownership, base commit and acceptance matrix. Define only interfaces actually required by the six workers. Add minimal shared Go types/schema fixtures when necessary so parallel components compile against real contracts; no fake implementations or default READY providers.

Resolve these concrete boundaries BEFORE releasing workers:

1. Go owns request ID, requested jurisdiction lookup, context version, allowed languages, typed candidate IDs, route/facility relationships, permitted order, response-template keys and freshness. The current flat `KnownIDs` validator is insufficient for P6 semantics. Define a richer trusted context without duplicating authoritative store logic.
2. Keep the existing `/api/v3/voice/commands` validation contract compatible, or document a deliberate versioned migration. Freeze the transcript/audio entry points, transcript confidence nullable/unknown semantics, audio format and content limits, response envelopes, and cancellation correlation. Do not let the client upload its own trusted context or declare model success.
3. Separate private ASR/middle/TTS workers with warm lifecycles and bounded queues. Freeze private protocol, health states, authentication/segmentation expectations, deadlines, error taxonomy and metrics. The proposed limits in parameters.md are starting budgets, not measurements. TTS receives only Go-approved text and versions, never arbitrary public input.
4. Define how context is revalidated after slow inference, how stale/out-of-order results are discarded, and how withdrawn audio is invalidated. Define P5 integration seams without changing P5 wire formats or copying offline-client internals.
5. Assign P6 workers separate directories: context/validation; ASR worker; middle adapter/deployment; TTS/templates; corpus/evaluation; Go orchestration. Assign shared dependency files and database migrations to one owner. P5 publication may propose source lifecycle changes; P6 context owns context/place files, not source lifecycle files. Record these boundaries.
6. Allocate distinct ownership and evidence paths for P7 security, performance and recovery. Only the coordinator wires shared HTTP/server/entrypoint/build/CI/contract changes. Workers provide compile-tested components and exact integration deltas.

Update the ledger only to authorize independent P6 engineering and P7 preparation while acceptance remains gated. Preserve the existing P5 correction history; do not erase failures or claim closure. Verify types/fixtures and commit the contract stage. Return the commit and explicit worker launch instructions. Stop until workers return; do not invent their results.

## Worker 1 — P5 client trust, freshness and coherent disk activation

Task name: `p5-client-final`. Own `backend/internal/offlineclient/` only. If Terra owns this lane, hand these findings to Terra instead of duplicating implementation.

Read `plan/p5-contract.md`, P5 correction history, and the complete client sync/storage/transport flow. Preserve the completed download and reference-binding work.

Reproduce and correct as one cohesive client change:

- Restart with a valid cached generation, successfully sync the same unchanged revision, then read it: freshness must recover under the documented trusted-time assumptions, without renewing validity. Cache corruption must be detected and repaired or explicitly blocked; comparing a claimed digest field is not verifying bytes/signature.
- Reject conflicting bytes under the same signed immutable manifest identity. A freshly fetched conflicting same-revision manifest must not silently replace trust/tombstone meaning or be called unchanged.
- Preserve observed expiry/high-water trust across retries, restart and a newer manifest referencing the same expired card. A wall-clock rollback must not resurrect that card. New valid content can recover only under the explicit trust policy.
- Replace two independent active-file renames with one coherent active-generation selection using the simplest durable design. Stage and verify complete required content, make the durable switch, then reclaim old generations. Handle file/directory sync failures; do not swallow them. Keep revocation knowledge durable even when new content cannot activate.
- Inject interruption/failure before and after every activation boundary. On reopen, either the old coherent generation or new coherent generation is selected, never a mixed pair presented as usable. Retest revoked/corrupt tombstones and same-revision eviction recovery.
- Once storage has recorded revocation knowledge, deleting or truncating the tombstone file must not silently become a fresh empty trust store. Test missing and zero-byte files as well as malformed JSON; only a genuinely new store may initialize empty.

Use actual signatures plus HTTP/disk/reopen tests for integrated assertions. Fakes may isolate a parser but cannot be the only trust proof. Preserve optional resource validation and actual interrupted-resume behavior. Report coverage and limits; do not expand into a native mobile client.

## Worker 2 — P5 authoritative publication and withdrawal delivery

Task name: `p5-publication-final`. Own `backend/internal/store/publication.go`, its tests, `backend/internal/offlinedelivery/`, and `backend/internal/httpserver/publication_adapter.go`. Any source lifecycle/migration edits require coordinator allocation first.

Read all callers of PublishManifest/PublishCard/PublishResource, the canonical signature implementation in offlinepkg, source activation and authoritative package lifecycle, and delivery cache wiring.

- Establish one real trusted publication boundary. Strictly parse public schemas; recompute canonical digests; verify actual signatures against configured authorized keys; bind record jurisdiction/package/revision/ID to signed content. Missing trust configuration fails closed. Reuse offlinepkg verification; do not write a second cryptographic implementation. If raw persistence remains internal, make it unreachable as an unchecked publication path.
- Identical concurrent retries succeed idempotently; conflicting immutable content or identity fails predictably. Record bytes and metadata cannot disagree. Stale publication replay must not undo quarantine/withdrawal or restore CURRENT from caller-supplied flags.
- Bind publication eligibility to actual source/package authority. Exercise source suspension/quarantine/revocation through the real lifecycle boundary, then GET through the configured cached HTTP delivery path. Do not manually call invalidation in the acceptance test and claim automatic propagation.
- Preserve an authenticated signed withdrawal/tombstone channel so hiding an obsolete card does not prevent clients learning that cached guidance was revoked. Document the consistency/propagation bound, including multiple instances and external caches. Use a simple enforceable solution, not a speculative distributed invalidation service.
- Bound cache entries/bytes or remove unnecessary caching; permit each coalesced waiter to cancel without hanging behind the leader. Prevent an in-flight stale fetch from repopulating a withdrawn entry.

Add real-DB/HTTP/signature tests for tamper, wrong jurisdiction, unauthorized/revoked key, concurrent identical/conflicting publication, stale replay after withdrawal, cache race and cancellation. Use isolated synthetic sources; no government credentials. Provide shared wiring changes to the coordinator.

## Worker 3 — P5 uncertain-write outcome validation

Task name: `p5-queue-final`. Own `backend/internal/offlinequeue/` only.

Read worker.go, queue.go, store.go, dispatcher.go and actual successful/error responses in stay_handlers.go/OpenAPI. Trace every supported queued operation before choosing validation rules.

- A 2xx status is not proof of commitment. Validate the bounded strict success envelope and operation-specific result identifiers/state before marking COMMITTED. Empty, malformed, wrong-operation and unexpected accepted-but-not-completed responses must not fabricate success. Handle transition errors rather than ignoring them.
- A 5xx/proxy error or exhausted transport retry budget is not proof that the operation failed. Preserve uncertain outcomes with the same immutable payload/key until a definitive server result is obtained. Keep retries bounded per run without discarding reconciliation state.
- Reauthentication/authorization failure while reconciling may block access to an already committed result; represent this without telling the user to submit a duplicate under a new key. Do not bypass server authorization to reconcile. Clearly distinguish known rejection of a new operation from loss of access to an earlier result.
- Preserve crash-recovered IN_FLIGHT uncertainty, selection-expired reconciliation, replacement-operation safeguards and token non-persistence.

Add actual HTTP tests: commit then proxy 502/503; malformed 200; restart; expired local selection; temporarily unavailable authentication; authorized same-key recovery. Assert no duplicate reservations/capacity/audit and that the queue never displays false confirmation or false definitive failure. Existing simple lost-connection tests must still pass. Update misleading retry comments and lane evidence only.

## Worker 4 — P6 scoped context and independent semantic validation

Task name: `p6-context`. Own coordinator-assigned context/validator files, normally `backend/internal/contracts/model*`, `backend/internal/store/context*`, `backend/internal/store/place*`, and a dedicated context package if justified by actual reuse. Do not edit source/publication/HTTP wiring.

Read the frozen P6 contract, voice-map prompt, existing strict JSON boundary, persisted packages/routes/facilities/policies and place aliases. Preserve API compatibility as agreed by coordinator.

- Build typed scoped candidates and semantic constraints from one authoritative version. Reuse existing eligible choice and route logic rather than inventing safety ranking. Ambiguous names produce bounded candidate pages; unknown names remain unknown. Do not guess a jurisdiction or call public geocoders.
- Validate raw output shape as well as values: known status, required/null fields, strict action variants, forbidden extra fields, duplicate/trailing/oversized JSON, supported action language, allowed intent/action combinations, legal target kinds/panels, exact choice order, approved speech keys and evidence that supports the action.
- Merely appearing in KnownIDs is insufficient: a facility ID cannot serve as a route, and a valid route ID cannot authorize a closed, stale or wrong-destination route. Preserve synthetic/non-operational restrictions and confirmation-only sensitive actions.
- Bind outputs to the server request/snapshot. Provide a revalidation operation for the orchestrator after inference; a withdrawn or superseded context must not produce current guidance.

Test adversarial model JSON, imported prompt injections, cross-jurisdiction same-name aliases, unsupported language, route/facility mismatch, reordered choices, arbitrary speech, invented IDs and withdrawal during inference. Include real-DB context tests and verify zero consequential writes. Synthetic test language data is not language acceptance.

## Worker 5 — P6 ASR worker and audio boundary

Task name: `p6-asr`. Own the contract-assigned private ASR worker directory and its isolated dependency manifest/tests. Read local_voice.py, speech_stt.py, requirements-voice.txt and artifact metadata; do not import huge tensors just to inventory them.

- Inventory exact IndicConformer artifacts/runtime/tokenizer hashes, license evidence, language identifiers and remote-code requirements. Retain useful assets, but do not copy the reference runtime's fabricated confidence=1.0 or infer all-language support from the model name.
- Implement the frozen private protocol with warm model loading, bounded queue/concurrency, deadlines/cancellation, readiness tied to actual artifact loading, and no public network exposure. Confidence is unknown unless supported and calibrated.
- Validate actual compressed format, bytes, decoded duration, channels, sample rate and decoded sample/memory limits. MIME and client-declared duration are insufficient. Use a maintained decoder/resampler already justified for the runtime. Reject malformed/truncated/decompression-abuse input and bound subprocesses if used.
- No retained raw audio by default, payload logs or crash/temp-file leftovers. Returning or freeing a Python bytes reference is not proof that all copies vanished; accurately describe the retention policy and inspect actual files/logs.
- On unsupported language/model outage, return a structured unavailable state usable by text/touch fallback, never a canned transcript as real ASR.

Test real codec decoding/resampling and worker lifecycle without requiring model weights for every unit test. Run available real recordings through the actual model separately and record hardware/artifact/accuracy/latency evidence. If unavailable, deliver runnable adapter and explicit blocked real-inference checks, not a fake pass. Coordinate GPU use with the middle/TTS workers.

## Worker 6 — P6 middle-model adapter and private vLLM serving

Task name: `p6-middle`. Own the frozen middle adapter package and dedicated inference deployment files. No shared Go entrypoint or dependency-lock edits without allocation.

Read the frozen contract, model schema, voice-map-system-prompt and tech-stack model decision. First candidate is Qwen3-4B-Instruct-2507, not an implicitly approved final model. Stay within the approximate 5–6B ceiling.

- Build a bounded Go client for the selected private vLLM API; verify structured-output syntax against the pinned version. Reuse the frozen schema. Enforce context/output/HTTP limits, deadline/cancellation and explicit unavailable/malformed responses. No paid fallback, tool execution, credentials, network-enabled model tools or unbounded retries.
- Treat transcripts and imported text as data. Supply only scoped server candidates and approved keys. Keep independent semantic validation outside the model adapter; JSON-constrained decoding is not a safety gate.
- Pin actual image/model/tokenizer/revision and document licenses and supported quantization. No invented image tags/digests or silent remote-code trust. Warm serving; private network; deployment starts unavailable if required configuration is missing.
- Create a repeatable evaluation invocation for BF16 and a supported quantized candidate on the same reviewed corpus. Preserve individual failures and resource data. Do not extrapolate concurrency from weight size or claim real vLLM success from a fake HTTP server.

Adapter tests cover timeout, cancellation, oversized/malformed/extra-text responses, unavailable schema support and injection-shaped input. Real vLLM smoke/evaluation runs only when hardware/artifacts are available within authorized scope; otherwise record the exact blocker and reproducible commands. No further model shopping unless measured failures justify it.

## Worker 7 — P6 approved templates, TTS and audio invalidation

Task name: `p6-tts`. Own the frozen template/TTS packages, private TTS worker and isolated manifest/tests.

Read multimodal.py and existing local synthesis/cache references; inspect actual Indic Parler-TTS artifacts without assuming language intelligibility. Use the frozen template and worker protocol.

- Go renders reviewed template keys using validated context arguments. No arbitrary model prose or public user text reaches the synthesis endpoint. Test/demo templates carry unapproved/synthetic status until human review exists. No filler acknowledgments for map movement.
- Implement warm, bounded, cancellable synthesis with actual codec/output limits and structured unavailable states. Inventory artifact revisions/hashes/licenses/voices/languages and remote-code requirements. Do not fake audio as successful speech.
- Reuse approved prerecorded common speech where available. Cache identity includes text/template/source version, language, model/voice revision and synthesis settings. Bound storage and expiration. Withdrawn guidance must invalidate audio through a real wired source-version check/event, including requests racing withdrawal; a purge helper tested alone is insufficient.
- Audio failure must preserve the validated on-screen response. Do not invent translations, ISL approval or emergency wording. No extra audio publication protocol that conflicts with P5 signed resources.

Test template-injection attempts, unknown keys, wrong language, changed/withdrawn source, stale cache, cancellation and malformed worker audio. Separate actual intelligibility review and real synthesis timing from protocol tests. Coordinate hardware use; record reviewer/artifact blockers honestly.

## Worker 8 — P6 corpus, language matrix and evaluation harness

Task name: `p6-evaluation`. Own a dedicated evaluation directory, corpus schema/fixtures and lane-specific language/artifact evidence. No production validators or provider code.

Read O01/O03/O11, voice-map-system-prompt and parameters. The user has not finalized the demo state/language list. Build extensible corpus infrastructure; never claim a guessed list is approved.

- Define each case with provenance/consent/license, language/dialect/cohort, input recording or explicitly synthetic transcript, scoped fixture context, expected IDs/intents/clarification/refusal and approved template outcome. Expected results are fixed before execution. Keep train/development/evaluation separation and avoid recording personal information in Git.
- Cover ambiguous village names, code-switching, unknown locality/language, noisy/clipped audio, nearest/other-route requests, claimed shortcuts, arrival/reservation/call requests, malicious source instructions, invented IDs, expired snapshots, cancellation and model outage. Human child/elder speech requires proper consent; do not fabricate it with TTS and call it human evidence.
- Implement a runner using the frozen ASR/middle/TTS protocols with stage and end-to-end measurements. Report critical entity/intent correctness, false acceptance, clarification, validation rejection, useful-action latency, per-language/cohort denominators and uncertainty. Keep warm/cold runs and synthetic/real inputs separate.
- Record artifact hashes, hardware, configuration and failures. A deterministic provider validates the harness only. Require separate flags/labels for real inference runs; missing samples/reviewers yields NOT_EVALUATED, not zero failures or 100% success.

Deliver a small runnable representative synthetic suite now, exact collection/review requirements for missing real languages, and commands for running the final integrated system. Do not duplicate worker adapters or invent emergency translations.

## Worker 9 — P6 Go orchestration, bounded work and fallback

Task name: `p6-orchestration`. Own the frozen orchestration package and dedicated new HTTP handler files/tests; coordinator owns shared server/main/OpenAPI wiring.

Build against the frozen worker/context interfaces. Read current voice handler, session model, P5 fallback representations and parameter budgets.

- Connect audio→ASR or direct transcript→scoped context→middle→independent validator→approved template/TTS. Go generates correlation IDs, bounds input and context, and controls every stage. Do not accept client-supplied proposals as a substitute for running the new production pipeline.
- Give ASR/middle/TTS separate bounded admission budgets. Enforce total and per-stage deadlines, cancellation, obsolete-result rejection and overload responses without unbounded goroutines or durable queues for stale voice requests. Reuse small primitives rather than building a scheduling framework.
- Revalidate current source/version/route/template eligibility after inference and before returning usable guidance/audio. Failure returns an explicit unavailable/clarify state with no unsafe action. Never turn a voice result into a reservation, arrival, transfer, emergency call or capacity mutation.
- Text/touch and cached non-operational fallback remain available on inference failure. Simple explicit camera controls bypass model work under the agreed contract. Silence is allowed; do not synthesize progress chatter.
- Add low-cardinality timing/queue/error metrics without audio/transcript/location/token contents. Ensure no model workers get government credentials or database write capabilities.

Provide handler wiring deltas for the coordinator. Component tests may use worker doubles but must include stale-snapshot changes during inference, cancellation at every stage, queue saturation, no-credential/no-write checks and no TTS for silent actions. Mark real end-to-end provider tests pending integration; do not declare P6 complete from these tests.

## Worker 10 — P7 early security and supply-chain evidence

Task name: `p7-security-prep`. Own dedicated security scripts/configuration/report files and narrowly assigned new security test files. Shared CI edits and production fixes go through coordinator ownership.

Read assurance.md, current Go routes/auth/ingestion/publication/queue and frozen P6 worker boundary. Build a concise current threat model and explicit pending P6 section, not an imaginary deployed architecture.

- Prepare/run pinned Go vet/race/fuzz, Staticcheck, govulncheck, targeted gosec, Trivy and Gitleaks as appropriate; reuse tools and avoid redundant scanners. Generate SBOM evidence for actual dependencies/images available. Record versions and snapshot dates. Do not print secrets or suppress findings wholesale.
- Test real role/object/jurisdiction boundaries on isolated fixtures. Include signing, hostile JSON/XML/audio, SSRF/redirect/egress, oversized input, token/session leakage and source credential isolation. Coordinate P5 findings with their owners; do not implement competing patches.
- Prepare authenticated ZAP staging checks with explicit local allowlisted target and synthetic accounts. Actual Strix execution remains blocked on O15's model/spend/target decision; provide the concrete run configuration and stop conditions rather than invoking arbitrary paid inference.
- Classify findings by reproduction, affected revision, exploitability and ownership. Limit active testing to disposable owned staging; never scan government/production systems. New inference services remain pending until integrated.

Deliver runnable checks, redacted findings and exact CI deltas. Label this P7 preparation; no final security certificate or gate B pass. Do not spend hours running overlapping tools to generate a cosmetic score.

## Worker 11 — P7 load model and performance harness

Task name: `p7-performance-prep`. Own dedicated load/benchmark scripts and reports; propose production instrumentation changes to coordinator.

Read parameters.md, equations.md, actual routes and DB operations. One million means registered/total users, not concurrent requests. Implement explicit workload profiles: proposed 10K/50K/100K active sessions, read/cache/write/voice rates, cold cache, one-facility hotspot and source outage. Convert rates mathematically and label assumptions; do not try 100K local VUs blindly.

- Build k6 scenarios against isolated synthetic HTTP/DB fixtures with actual success/failure/capacity assertions, stable idempotency rules and representative geography skew. Do not silently benchmark 401/503 responses as successful work. Separate cached public delivery, authenticated writes and inference.
- Collect p50/p95/p99 latency, throughput, error categories, CPU/memory, DB query/lock pressure and queue depths. Use pprof/benchstat only on controlled private targets. Define run duration, warmup, load-generator capacity and stop thresholds; run a bounded local smoke first.
- Voice-load harness may target the frozen protocol, but dummy-model throughput is not GPU throughput. Prepare later real-worker measurements and replica/failure-reserve/cost calculations. Missing O04/O09 hardware/budget/traffic approval stays explicit.
- Do not optimize based on static guesses. Report measured bottlenecks and specific owner patches; do not add Redis, brokers or indexes speculatively. No shared-host heavy run while another agent measures inference.

Deliver runnable workload definitions and truthful baseline at the tested revision. Final surge sizing and gate B remain pending integrated P6 and representative hardware.

## Worker 12 — P7 backup, recovery and lifecycle rehearsal

Task name: `p7-recovery-prep`. Own dedicated recovery scripts/runbooks/tests and isolated deployment rehearsal files. No shared main.go, readiness.go or migration changes without coordinator allocation.

Read existing real-DB/process tests, all migrations, readiness/graceful-shutdown code and P5 publication storage. Preserve existing P3/P4 evidence and add actual operational evidence where absent.

- Build and run an owned disposable source DB→pg_dump→separate restored DB exercise. Verify schema revision, representative source/package/publication/stay state, committed idempotency replay, capacity conservation and audit-chain verification after reconnecting a real server to the restored DB. Merely writing a runbook or checking dump exit status is not restore verification.
- Exercise dependency outage, restart, cancellation and graceful drain with bounded requests. Reuse crash-after-commit and cross-process-last-space tests instead of duplicating their logic. No service may falsely report readiness during missing required dependencies.
- Measure restore and recovery times on declared hardware/data volume. Distinguish backup restoration, process restart and true database failover. If a replica/promotion environment is unavailable, supply a concrete failover rehearsal and mark it NOT_RUN; do not relabel restart as HA.
- Keep backups/logs synthetic and private; verify scripts cannot target/drop shared databases by default. Cleanup only uniquely owned resources, including on test failure. Do not kill the team's PostgreSQL instance.

Deliver reproducible rehearsal commands/results and precise shared wiring findings. P6 worker-loss recovery is a later integration check. O09 recovery budgets remain proposals until approved; no disaster-recovery certification from a tiny fixture.

## Integration — return to the original coordinator AFTER workers finish

Read all lane handoffs and commits. Recheck `CLEAN` and outstanding Terra work. Integrate verified commits in dependency order, preserving user changes. User authorizes local cherry-picks for these assigned workers; no push/main/history rewrite. Inspect each diff before cherry-picking; resolve shared wiring yourself without discarding safety tests.

1. Close P5 regressions first: real crypto/publication→cached HTTP→disk/restart; actual interruption/resume; actual committed operation→lost/malformed/proxy response→restart/reconciliation. Test withdrawal and clock rollback, not only happy path. Do not call private methods in place of the claimed application lifecycle. Keep external blockers separate.
2. Integrate typed context/validator, ASR/middle/TTS, orchestration and corpus. Wire production entrypoints, private workers, configuration, versioned contracts, deadlines and telemetry. Incomplete model configuration stays unavailable. Update OpenAPI and contract-route tests. No test-only provider or synthetic bypass in ordinary production defaults.
3. Run new voice endpoints through real Go HTTP and persisted scoped context. Demonstrate actual ASR→vLLM→validated display actions→approved TTS on available declared hardware. During inference, change/withdraw the package, cancel requests and saturate queues; assert no stale guidance or consequential writes. Test direct text fallback independently of model health.
4. Run the multilingual evaluation; label missing language/human-review/hardware evidence BLOCKED or NOT_EVALUATED. No aggregate score hides a failing cohort. Reconcile artifact/license/provider choices in decisions.md and tech-stack.md without turning proposals into proven facts.
5. Apply P7 CI/telemetry/runbook changes, rerun security and recovery evidence on the integrated revision, then conduct representative load only with an agreed environment/budget. Include GPU outage and inference queue behavior now. Strix stays gated on its concrete scope/budget approval.
6. Run formatting, `go vet ./...`, `go build ./...`, full Go tests with a uniquely owned migrated DB, relevant race/fuzz checks, and explicit process tests with BOTH `-tags crashtest` and `STHIRA_RUN_PROCESS_TESTS=1`. Run isolated inference-worker tests with pinned dependencies. Report excluded/skipped suites; never count an uncompiled process test as passing.
7. Update the existing phase ledger/acceptance matrix, decisions and open decisions once, as sole owner. Separate component engineering, integrated engineering, real model/language evidence and operational activation. Each claim cites its tested commit. P7 gate B remains NOT_READY while required internal/real-inference/security/load/recovery evidence is absent. Stop before P8.

Final handoff: integrated commits, tested revision, observable behavior, exact pass/fail/skip, unresolved findings with owner, external decisions and next eligible work. Do not say “no defects remain”; say precisely which acceptance matrix was exercised.
