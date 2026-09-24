# Sthira autonomous execution playbook

Authored 2026-09-23 against `CLEAN` at `a12350bf5bc64261fe49e9c976972e45cc2168d0`.
Extended 2026-09-23 by user request through P9, from documentation commit `2d04068`. This is the current execution entry point. It is an instruction set, not a completion certificate.

## 1. Objective, authority and reading boundary

Finish the round-two demonstration first: actual selected models, a polished laptop/phone journey, understandable source provenance, and consent-based arrival assistance. Preserve and then complete remaining backend engineering, P8 Android/iPhone clients and P9 whole-system readiness work. Do not restart P0–P7, translate the Python reference again, or spend the demo window chasing full production certification.

User decisions: Go product backend; Python/native ML runtimes permitted; IndicConformer-600M-Multi ASR, Sarvam-30B middle model (~2.4B active non-embedding parameters), Indic Parler-TTS; Android+iPhone eventual launch; one million TOTAL users (peak mix to measure); immediate and 7–30-day temporary relocation; no hazard prediction/permanent resettlement. September 28–29 round two takes priority. Actual round-three readiness is evidence-dependent.

The user requested one detailed playbook for models to execute without another reviewer between every step. Workers may continue to the next eligible task within their assigned lane after acceptance. A final integrated review remains necessary. More instructions cannot guarantee a model's correctness.

Accepted scope: demo first, remaining production backend afterward, then P8/P9 subject to their prerequisite gates. P10 cleanup and P11/P12 government activation/launch are excluded. Parallelism is optional: one sequential executor, two normal lanes, or up to four non-overlapping active implementation lanes. This document does not provision workers automatically. User launch message selects lane/count; if none is specified, use sequential execution. Do not incur paid compute or spawn extra agents merely to consume credits.

**Current user overrides of older documents:** prototype frontend may proceed before Gate B. User now requests location tracking to help determine arrival. Implement the bounded opt-in foreground journey in R04; older blanket “no geofencing” text is superseded for this feature only. Automatic occupancy changes, involuntary/background surveillance, inferred welfare and AI approval remain prohibited. No approval to deploy, push, rewrite Git history, rent hardware or activate live government integrations is created by this file.

**Read once per new chat:** applicable root instructions (`CLAUDE.md`, `GEMINI.md`, any scoped AGENTS.md), the mandatory product documents listed there, then this current playbook through section 10 and your assigned task. Read `plan/round-two-demo.md`, `plan/open-decisions.md`, and the task's exact source/callers. Use targeted sections of large documents. Historical instructions below the `Historical phase ledger` heading preserve evidence only: never execute their old cherry-picks, initial migration plans or former worker assignments by default.

At resume read git state, current task status and relevant changed source. Do not re-read every handoff or dump full logs. Current code beats stale README claims; instructions govern desired behavior, not proof that implementation exists.

## 2. Copilot review reconciliation and verified starting evidence

The supplied Copilot review gives no review commit and principally describes `src/sthira_v2` Python. Its claim that this branch has no Go, SQL persistence, authorization, offline implementation or database tests is contradicted by current source. Do not implement its suggested replacement architecture. Its 8% completion, ratings, timeline and launch probabilities are unsupported estimates, not planning inputs. Its advice to substitute mocks to obtain passing storage tests is rejected. Models run on servers; the 3 GB phone requirement is for the client.

Evidence inspected at the baseline above (source inspection, NOT a new full runtime certification):

| Topic | Actual path / finding | Action |
| --- | --- | --- |
| Go service and SQL | `backend/go.mod`, `backend/cmd/sthira/main.go`, `backend/internal/store/`, migrations 0001–0009 | Preserve; do not rebuild |
| Auth/stays | `backend/internal/httpserver/operator*.go`, `stay_handlers.go`, store and real-DB integration tests | Preserve boundaries; live IdP remains absent |
| Offline | `backend/internal/offlinepkg/`, `offlineclient/`, `offlinedelivery/`, `offlinequeue/`, `offlineresources/` | Preserve; verify affected integration |
| Destination order | `store/scoped.go:buildEligible` uses policy order as facility IDs, while package policy orders safe zones; two inventory-query errors are ignored | R01 |
| TTS | `orchestration/orchestrator.go:stageTTS` requests fixed 16000 Hz; verify actual WAV/response metadata propagation | R02 |
| Worker lifecycle | ASR/TTS listener access lacks synchronization; `eval/provider/http_realserver_test.go` excludes race builds | R02 |
| Readiness | `store/store.go:SchemaRevision` is 7; scoped flow uses migrations through 9 | R00 |
| Translation authority | `store/scoped.go` accepts wildcard source binding; migration 0009 has a content digest needing runtime enforcement | B01; no operational speech until fixed |
| Deployment | Dockerfile builds nested migration module from parent; distroless migration dependencies missing; Compose naming/bind/publish needs repair | R06 |
| Documentation | `backend/README.md` still describes P1/no DB; older tech-stack observations remain historical | R00 |

Other bounded findings retained from `plan/reviews/review-four-workers-round-two-2026-09-21.md`, requiring reproduction before editing: scenario path containment/read-size limits; old persisted reservation payload-hash compatibility; isolated synthetic pipeline setup missing; actual selected-model inference not demonstrated. Original commit IDs in historical docs were rewritten to remove generated blobs while retaining individual commits. Do not cherry-pick obsolete IDs or infer missing work from SHA changes; use source and ancestry.

Previously recorded multi-module/real-DB checks belong to that reviewed revision. They are not a substitute for checking changed behavior. No new completion percentage is asserted here.

## 3. Copyable launch instructions

### Coordinator / single executor launch

> Work in `/Users/apple/Documents/Projects/MonitoringZ`. Read the current section of `plan/prompt.md`, applicable project instructions and product context. Execute this playbook; do not generate another plan. You are coordinator. You may integrate only this playbook's verified task commits locally into uppercase CLEAN, preserving individual commits and unrelated work; no push is authorized. Start R00, establish current state and evidence, then run eligible round-two tasks in dependency order. If no workers are assigned, implement sequentially. If workers are assigned, allocate disjoint lanes and freeze shared contracts before dispatch. Record exact commits, tests and blockers in the task table. Continue eligible authorized work without requiring the planning model after each commit. Do not change main, push, rewrite history, download weights, spend money, use real government endpoints or claim mock results as real inference. Pause backend expansion while R07 demo acceptance is unresolved; explicit user assignment to a B-task allows that bounded parallel work. Stop at the final handoff rules.

### Worker launch (send common and task instructions together)

> Read `plan/prompt.md` sections 1–5 and 8–10 plus task **[TASK_ID]** and its prerequisites. Work only on **[ASSIGNED_BRANCH/WORKTREE]**, based on **[INTEGRATED_BASE_SHA]**. Your task is **[TASK_ID]**, and your next authorized tasks are **[EXPLICIT_IDS or NONE]**. Read actual source and callers; implement and verify the stated behavior. Preserve public contracts unless a contract change was agreed with the coordinator. Do not edit another lane's files, the shared ledger or main. Commit verified stages locally. Report commit SHAs, exact checks, remaining dependencies and a small reproduction/run command in `plan/evidence/execution-[TASK_ID].md`. No broad review document, unbounded test logs, generated binaries or model data in Git. Stop if a missing prerequisite or shared-file change requires coordination; continue independent work within your task.

If placeholders are unfilled, obtain task/base/ownership from coordinator before editing. Do not give every worker the coordinator role. Worker completion does not authorize it to merge other workers or start an arbitrary phase.

## 4. Scheduling and dependencies

A worker slot includes a coordinator when it is actively implementing. Do not have two processes commit in the same checkout. Default maximum useful implementation concurrency is **four**; a fifth queue does not eliminate shared-file or GPU dependencies.

| Task | Deliverable | Required dependencies | Owner / allowed parallelism |
| --- | --- | --- | --- |
| R00 | Orientation, readiness minimum, contract and scenario freeze | Current checkout | Coordinator alone first |
| R01 | Correct destination context + isolated exercise backend | R00 | Backend lane |
| R02 | Correct real-model plumbing and launch instructions | R00 | Inference lane, parallel R01 |
| R03 | Responsive actual-API voice/map UI | R00 contracts; R01/R02 for final acceptance | UI lane, parallel R01/R02 until integration |
| R04 | Foreground journey tracking and explicit arrival | R03; R01 for stay arrival | Same UI lane, serial after R03 |
| R05 | Bounded safe scenario preparation | R00 | Data lane; may share fourth slot with R06 serially |
| R06 | Runnable local/staging packaging and recovery commands | R00; R01/R02 for final integration | Deployment lane, parallel R01/R02/R03 |
| R07 | Integrated demonstration, rehearsal and founder handoff | R01–R04 and selected R05 output; R06 only if demo uses containers | Coordinator after integration; actual models required |
| B01 | Strict translation/source/content authorization | R01 integrated; R07 by default | Backend lane; serial with other scoped.go work |
| B02 | Durable upgrade/replay and source/offline continuity | B01 | Backend lane |
| B03 | Remaining security, deployment and recovery closure | R06, B01/B02 before final verification | Operations lane; code-independent tooling may run earlier |
| B04 | Real language, load, cost and resource evidence | R02/R07; agreed hardware and budgets | Inference/performance lane; do not compete for same GPU with demo |
| B05 | Backend release-candidate reconciliation | B01–B04 | Coordinator; Gate B checkpoint before P8 |
| M00 | Two-platform technology proof and framework decision (P8) | B05 with Gate B accepted | Mobile coordinator first |
| M01 | Mobile data/offline/session contract and reusable implementation (P8) | M00 | One shared-code owner |
| M02 | Android complete citizen journey (P8) | M00; M01 integration contract for implementation, acceptance for final checks | Android lane |
| M03 | iPhone complete citizen journey (P8) | M00; M01 integration contract for implementation, acceptance for final checks | iOS lane, parallel M02 |
| M04 | Scoped operator workflow, privacy and notification integration (P8) | M00/B05 contracts | Operator lane, parallel mobile clients |
| M05 | Integrated P8 acceptance | M01–M04 | Coordinator; both physical platforms required |
| Q01 | Regional case/language/accessibility evidence inputs (P9) | R05/B04; preparation may overlap M-tasks | Scenario/content lane; external inputs required |
| Q02 | Regional whole-system usability and failure drills (P9) | M05, Q01 | Drill lead; independent environment from Q03 |
| Q03 | Integrated security, surge, recovery and upgrade assurance (P9) | M05, B03/B04 | Assurance lane; final reruns after fixes |
| Q04 | Release-candidate evidence and P9 closure | Q02, Q03 | Coordinator; stop before P10 |

**With one worker:** R00 → R01 → R02 → R03 → R04 → R05 as needed → R06 if needed → R07 → B01 → B02 → B03 → B04 → B05. A hardware-blocked R02 must not block implementation of R03/R04/R05/R06; it does block real-demo acceptance.

**With two workers:** first R00; then worker A R01 → R05 → R06, worker B R02 → R03 → R04. UI can start after R02's interface correction even if real GPU proof waits. Integrate at task boundaries; R07 is serial. After demo, A B01→B02, B B03 preparation then B04; final B03 proof awaits B02; B05 serial.

**With four workers:** R00 first. A R01, B R02, C R03→R04, D R05→R06. Integration is coordinator-owned, not a fifth simultaneous editor of shared files. After demo: A B01→B02; B B04; C may take a separately assigned scoped B03 code fix, D B03 ops. No overlapping B03 ownership; otherwise leave a slot unused. Do not invent extra features to fill slots.

**P8/P9 continuation after B05:** M00 is serial and requires Gate B, not merely a completed B05 report. M01 first freezes the mobile contract; clients may build against that contract while its owner completes the implementation. Final M02/M03 acceptance requires integrated M01. A framework decision cannot be inferred from the word Kotlin or from the browser prototype.

- One worker: M00 → M01 → M02 → M03 → M04 → M05 → Q01 → Q02 → Q03 → Q04. Prepare Q01 inputs earlier when useful.
- Two workers: after M00 and M01 contract freeze, A owns M01/shared implementation then Android M02; B owns platform-specific iOS M03 then operator M04. B must not duplicate A's shared logic or edit shared UI simultaneously. Integrate M05; then A Q02, B Q03 on isolated resources after Q01 is ready; Q04 serial.
- Four workers: M00 and M01 contract freeze first; A M01/shared implementation, B Android M02, C iOS M03, D operator M04/Q01 preparation. If the chosen framework puts both screens in shared source, assign that source to A and give B/C only distinct platform adapters/device work; parallel editing of the same shared screen is prohibited. M05 serial; Q02/Q03 can run independently after prerequisites. Reserve one owner for findings/integration rather than opening overlapping fix branches.
- No Gate B, selected framework, required device, content review or staging access: record the precise blocked acceptance. Planning/input preparation may proceed; do not label simulator-only work or an incomplete backend as permission to pass P8/P9. Spending, public distribution, external testing and real field engagement still require their specific authorization.

**Dependencies mean integrated interfaces, not optimistic handoff text.** A worker may implement against a frozen response fixture, labelled as such, but may not claim end-to-end acceptance before the actual dependency works.

## 5. Common implementation and evidence rules

1. Inspect `git status --short`, `git branch --show-current`, `git log -6 --oneline`, `git worktree list`. Record base. Integration target is uppercase `CLEAN`, never `main`. If a checkout is on main or contains unrelated edits, create/use an isolated task checkout; do not reset/stash another person's work.
2. For parallel edits use `codex/<task-id>-<purpose>` branches from the coordinator's exact frozen base. Worktree creation is within the assigned execution task; cleanup removes only owned resources. Existing worktrees stay untouched. User must authorize integration if their launch did not authorize it; ordinary worker assignment alone does not authorize merging into CLEAN. Ask once with concrete checked commits when needed.
3. Coordinator owns shared contracts/OpenAPI, `cmd/sthira/main.go`, root CI/Makefile, root module dependencies/migrations, cross-lane wiring and this ledger. A task can receive explicit ownership of one shared file for that stage; otherwise propose the exact delta, do not silently edit it. Allocate migration numbers centrally. Module-local inference changes belong to R02. No concurrent shared-file edits.
4. Before a fix write a short acceptance checklist in the task handoff. Reproduce the reported issue through the responsible boundary; if already fixed, prove that case and mark existing evidence, do not reimplement. Trace all callers before changing common helpers. Avoid broad cosmetic refactors.
5. For any added behavior specify required/missing/invalid states, source of truth, authorization, failure response, retry and cancellation behavior. Use existing envelopes/error taxonomy and validators. Freeze any genuinely necessary additive contract with coordinator before UI or worker consumers implement it; do not guess endpoint names.
6. Real DB tests prove persistence; real HTTP handlers prove transport; fake inference proves plumbing only. A test name, no-tests-to-run, compiled-out suite, skipped dependency or script self-check never proves the required behavior. Test desired invariants, not the same broken assumptions as production code.
7. Preserve source authority, signed freshness, jurisdiction, current grants, transactional capacity and explicit user confirmation. Do not remove guards or relabel synthetic sources/phrases to unblock a demo. LLM receives no credentials, direct DB writes or arbitrary tools. No automatic calls or rescue dispatch.
8. No model download/GPU rental/paid inference/new paid external tool without explicit existing authorization and a budget. Inspect known local artifacts and hardware metadata only; never text-read weights. A missing loader is ENGINEERING_REMAINING; missing approved hardware is BLOCKED_HARDWARE. Do not conflate either with O14 (IdP).
9. Honor no `.txt` changes, secrets, raw load samples, binaries, model weights or unrelated files in commits. Dependency changes needing `requirements-voice.txt` must be proposed and await an explicit exception; do not evade the rule by creating a competing dependency file. Preserve individual commits; no amend/squash/reset/rewrite/push. Local verified stage commits are expected.
10. Keep raw output in a task-owned temporary directory, not the repo. No million-line metrics. Bound runs before starting; save only compact counts, percentiles, redacted findings, reproducible commands and artifact hashes. Do not delete another worker's reports; inspect names/sizes first and request authorization for unrelated deletion.
11. Failure: identify code vs invocation vs environment. After three consecutive failed attempts without material new evidence, stop that issue, record exact blocker and continue eligible independent work. Do not skip a required check and call the task done.
12. After a coherent stage: focused checks → diff review → stage explicit files → local commit → handoff. Before commit ensure unrelated staged files will not be included. Report execution exit codes and skips, not just “all green.” Do not rerun unchanged successful full suites repeatedly.

## 6. Round-two tasks

### R00 — Correct orientation and freeze one visible journey

Read: `backend/cmd/sthira/main.go`, `backend/internal/store/store.go`, `backend/migrations/`, `backend/contracts/openapi.yaml`, `plan/p6-contract.md`, `frontend/v2/src/main.ts`, root and backend README, current `plan/tech-stack.md` and demo plan.

Deliver:
- Update active README/stack descriptions to point to `/api/v3` Go and identify Python as reference plus retained ML adapters. Preserve historical evidence as dated history. Remove active claims that DB/Go do not exist. Reconcile old frontend/geofencing prohibitions with the narrow user overrides in section 1; record rationale in decisions, not a silent rule deletion.
- Correct migration readiness: derive the required schema from actual queries/current migrations (baseline requires 9); a DB at 7/8 must not claim ready for revision-9 queries. Current schema succeeds; absent DB and failed migration fail closed. Do not merely edit the constant without checking the real prober and startup.
- Freeze the demo's existing/public request/response contracts: place/jurisdiction resolution, voice process/audio metadata, destination browsing vs reservability, evidence/freshness, map action, stay confirmation and errors. Use actual OpenAPI/runtime shapes. Record minimal additional fields only when necessary.
- Select one already permitted exercise package and language. Ask user for case/language if absent; meanwhile use a named fictional synthetic fixture for development, never claim a real historical scenario. O01 catalogue remains open. Capture local GPU/model path availability without reading secret files or downloading artifacts. Record missing resource requests once.
- Document the round-two target flow: speak → resolve place → display labelled incident/destinations → choose → show supplied route → useful audio; explicit stay and arrival only when implemented. Distinct second journey handles offline/errors.

Acceptance: docs paths exist, package identifiers match actual schemas, readiness rejects lower schema through real HTTP+DB; current migrated DB succeeds. R00 does not claim live model inference. Freeze commit SHA unlocks parallel work.

### R01 — Destination selection and an explicitly isolated exercise backend

Own: scoped destination-building logic/tests, relevant HTTP composition and a coordinator-assigned local exercise entry point/seed. Shared contracts/main/migrations require ownership grant. Do not take B01 translation work concurrently.

Read: `backend/internal/store/scoped.go`, `choice.go`, `stay.go`, `opkg/package.go`, `httpserver/server.go`, `voice_process.go`, `stay_handlers.go`, orchestration resolver interfaces and exercise options.

Implement:
1. Map each permitted safe-zone ID in allocation order to its member facilities. Preserve authority ordering and explain ties: deterministic display order within one zone must not become a claimed authority safety ranking. Unknown/missing zone IDs cannot invent facilities. Multiple facilities in one zone must work. Preserve informational browsing when party/dates are absent; no free-capacity promise from a general MAX(capacity) query.
2. Propagate inventory/zone query errors, distinguish no inventory from DB failure and from zero capacity. Scope all reads to the correct package/jurisdiction. At actual reserve/extend/transfer time keep existing locked authority, policy, route, snapshot and capacity revalidation.
3. Supply a reproducible local exercise launch/seed using existing Store/Server seams, real DB and real HTTP handlers. Two eligible choices only when fixture defines them. Exercise-only template/source handling must be process-controlled, isolated and visibly synthetic. Ordinary production binary/config must still reject synthetic commitments and unapproved speech. Header/body/query fields cannot activate it. Do not add a general fake government verifier.
4. Run selected successful browsing/confirmation through actual API, not direct store-only demos. Existing stays/arrival must remain explicit and idempotent. If stay UI cannot be accepted in time, do not expose a deceptive active button; report unfinished visible scope.

Acceptance: mismatching facility/zone IDs; two facilities per zone; empty/invalid order; query failure; wrong jurisdiction; unavailable/full destination; ordinary server rejects exercise data; isolated server displays correct labelled choices; successful real-DB reserve plus same-key replay without duplicate capacity; package withdrawal denies new work. Reuse existing regression suites and add only missing cases. Return seed/start/reset commands operating only on owned data.

### R02 — Real model integration, audio correctness and worker lifecycle

Own: `backend/internal/asrworker/`, `ttsworker/`, `middleworker/`, `orchestration/`, `backend/eval/`, retained Python adapter implementation/tests. Coordinate shared contracts/dependencies and `voice_process.go` with R01.

Read: `orchestration/orchestrator.go:stageTTS`, `http_client.go`, worker request/response types, ASR/TTS server lifecycle, runtime adapters/IPC, `src/sthira_v2/speech_asr_adapter.py`, `speech_tts_adapter.py`, `tests/test_b2_adapters.py`, `tests/test_v2_real_adapters.py`, `eval/provider/http_realserver_test.go`, `eval/commands.md`.

Implement:
1. Trace output WAV from actual TTS adapter to public response and browser. Negotiate supported settings or use returned validated metadata; resample only if needed using supported dependencies. Check RIFF sample rate/channels/bit depth/length and enforce bounds. Never declare 16 kHz while delivering native-rate audio or silently mislabel bytes. Preserve cache identity for text/language/template/source/settings.
2. Synchronize ASR/TTS listener lifecycle at the owner and inspect middle worker separately. Establish Start/Addr/Cancel/Wait behavior under success/failure/concurrent shutdown. Remove `!race` exclusion once actual servers pass; do not suppress races. Preserve bounded IPC writes, deadlines, process reaping and correlation.
3. Establish exact local selected model revisions, licenses, processor/tokenizer/runtime compatibility and actual supported language. Sarvam uses its real tokenizer template, constrained output and reasoning setting verified for the pinned runtime. Do not reuse speculative templates or assume active parameter count is resident memory. Record cold/warm RAM/VRAM, including runtime/KV overhead.
4. READY requires successful load/needed warmup. Missing artifacts, unsupported language, decode error, timeout or unavailable worker returns explicit failure, never a fabricated transcript/audio/canned success. Keep text fallback usable when voice is unavailable.
5. Supply a minimal reproducible launch and real-inference smoke using an authorized local clip: actual ASR transcript → actual Sarvam structured action → independent validator → actual Parler WAV. Include the public HTTP route and actual context. The same protocol must also have lightweight fake-runtime regression coverage for CI, labelled PLUMBING_ONLY.

Acceptance: native-rate WAV accepted with truthful metadata; mismatch rejected; corrupt/oversized audio rejected without panic; cancellation drops stale responses; Start/Addr/stop under `-race`; actual server conformance tests participate in race build. Real demonstration records model revisions, hardware, input/output hashes, observed latency and a reviewed transcription/action/audio. If hardware/weights unavailable, commit working plumbing with REAL_INFERENCE=NOT_RUN and precise next command; do not mark R02/R07 accepted or replace the selected models without user agreement.

### R03 — Polished responsive UI on the existing stack

Own: `frontend/v2/`; API shape changes go to coordinator. Read current `package.json`, `src/main.ts`, `i18n.ts`, `mapActions.ts`, styles and actual R00 frozen endpoints. Reuse TypeScript/Vite/MapLibre; no framework rewrite for round two.

Build one coherent screen journey on laptop and phone: prominent mic, readable captions, language selection only for supported evidence, map with clear source labels, destination cards and touch/text fallback. Avoid clutter and unnecessary animations. Large tap targets (aim >=44 CSS px), keyboard focus, labelled controls, readable contrast and zoomed text; chat cannot be hidden from users who need it.

Integrate actual Go endpoints, correct session/jurisdiction/version/request IDs and returned validated map actions. Do not hard-code fake transcript/destinations as successful API output. Render untrusted names/text safely. Browser microphone/geolocation needs secure context: document phone HTTPS or an approved local secure setup; laptop localhost success is not phone permission proof.

States: idle/listening/processing/cancelled; ambiguous place choices; unsupported language; no destinations; unknown vs full capacity; permission denied; model unavailable; stale/revoked package; lost network; interrupted response. Avoid race where old request replaces newer map selection. Explain useful outcomes, not repetitive “I am doing” speech. Play audio with appropriate user gesture/autoplay handling and show text if playback fails.

Map: render supplied polygons/routes only; no inferred safe shortcut or Google/public routing fallback. Include legend, attribution and non-map textual directions. Use a permitted basemap or a clearly labelled local schematic if rights unavailable. Do not bulk-download public OSM tiles. Cache only allowed data; the Go offline protocol is not automatically a browser integration. For demo, any cached read must retain integrity/version/freshness rules and present unknown freshness after restart unless revalidated. Do not replay offline writes as confirmed success; reuse existing queue semantics if exposed, otherwise explicitly disable new offline commitments.

Acceptance: `npm ci` with existing lockfile, `npm run build`; exercise actual browser + actual API on laptop and phone-sized viewport, then real phone when available. Demonstrate selected happy journey and permission/error/offline flows. Network inspection must show real endpoints and no fallback Python v2 path for the new flow. Document physical phone NOT_RUN if absent; screenshots alone do not prove function. Hand off the exact start/proxy commands and route to R04/R07.

### R04 — Consent-based foreground tracking and arrival assistance

Own: same frontend lane after R03; backend changes only if required by an accepted sharing contract. This feature is now requested, but no automatic welfare certification or background surveillance is authorized.

Build minimal journey state around selected destination/package/route: NOT_STARTED → TRACKING → NEAR_DESTINATION → ARRIVAL_REPORTED, with PAUSED/LOCATION_UNAVAILABLE and stale-route states. Use current OS/browser position permission, accuracy and timestamp. User starts/stops explicitly; stop watch on cancellation/unmount/arrival. Do not upload a continuous trace by default or put coordinates in analytics/logs.

Compare position deterministically with the supplied destination geometry. Account for uncertainty: poor accuracy, stale position or an accuracy region straddling the boundary cannot prove arrival. Use existing geometry support where sufficient; do not invent a radius, confidence percentage or boundary tolerance as official policy. Define and record conservative demo thresholds as exercise settings, and support manual confirmation. Reject malformed geometry/coordinates. No invented GPS reading except visibly labelled simulation.

Display “near destination” as an advisory prompt; explicit touch/keyboard confirmation calls the existing idempotent arrival operation only when a valid stay exists. Location-only trip without a reservation can record self-reported journey arrival without inventing a stay. Do not double-decrement capacity. Self-report, location-supported report and authorized facility-confirmed check-in remain distinct. No location = unknown, not “missing person.” Background/tab suspension shows last updated and does not claim continuous tracking.

Default round-two sharing: selected-destination progress on the citizen screen and the existing authorized stay status, without raw trace storage. If user requires a remote live tracking dashboard, record this as a separate dependency: coordinator must define opt-in recipient, session ownership, minimum precision, TTL/deletion, access scope, throttling and authorized operator UI before implementing a bounded location-snapshot endpoint. Do not mark remote live tracking delivered by a client dot. Do not expose citizen locations through public map endpoints; O10/O14 gate operational access.

Acceptance: permission denied; stop removes watch; stale/inaccurate/edge position never auto-confirms; outside/inside valid geometry; duplicate arrival produces one transition; offline explicit arrival stays pending/not confirmed; route revocation warns and stops guidance. Use injected position inputs for deterministic logic checks, plus real-device foreground location/permission evidence when available. Label which tracking/sharing components are delivered and which await a contract/device.

### R05 — Safe scenario preparation and truthful demo data

Own: `backend/internal/scenarioprep/`, `backend/cmd/scenario-prep/`, `tools/scenario-prep/`, task-specific fixtures. Do not edit signed offline protocol independently.

Reproduce lexical containment bypass using a parent symlink and large input using existing CLI. Bound reads before allocation/decode; check resolved input/output containment, parent components and file types as appropriate. Do not leave a check-then-open substitution vulnerability if claiming support for untrusted concurrently writable input: either implement safe handles with supported APIs or explicitly restrict workspace ownership and reject symlinks. Never overwrite a pre-existing output tree unexpectedly. Reuse strict parser and package/catalogue validators.

Prepare one deterministic development exercise with explicit IDs, provenance, red zone, destinations, routes/modes and synthetic allocation policy. Missing official historical reference remains missing. Scenario handoff bundle is not a signed P5 publication; pass through existing importer/signing/exercise boundary. Do not independently create a second policy schema. Do not fabricate the 10–15-state catalogue to satisfy a count.

Acceptance: valid roundtrip init/validate/report/bundle; parent/leaf symlink escape; oversized index and package rejected before unbounded read; path traversal; existing output preserved; malformed/duplicate-key inputs; stable bundle hashes; missing data → INCOMPLETE, never READY. Run these actual CLI commands with supported flags from `--help`; use `tools/scenario-prep/README.md` as orientation and fix stale examples discovered.

### R06 — Reproducible local deployment, not another infrastructure platform

Own: `deploy/go-backend/`; coordinate main.go/config/CI with coordinator. Read Dockerfile, Compose, migration module and scripts. Reuse existing backend/recovery code; no Kubernetes/Redis/broker/new cloud architecture.

Fix nested module build from its own module directory; no `go mod download || true`. Ensure migration execution actually has required SQL files and `psql` or an existing equivalent verified runner, using a dedicated migration image/job if simpler than enlarging the API image. API remains non-root. Correct Compose numeric published port vs host IP; API listens on reachable container interface while host exposure stays loopback unless explicitly configured. Remove fixed container/volume names that defeat project isolation. Pin/declare supported Postgres/PostGIS versions based on actual compatibility; do not assume the old 16/3.4 image equals tested 18/3.6.

Provide start/migrate/health/stop and backup/restore commands with ownership-based cleanup. No printed secrets/DSNs. Database readiness is not source/operator/model readiness. Real IdP stays absent/fail-closed, model workers private. Container networking must not accidentally publish inference/admin ports. Never start paid cloud resources.

Acceptance: Compose config parses; image builds; fresh owned project migrates through required revision; actual host API responds; too-old DB fails readiness; stop/restart persists data; backup restored into a second owned DB preserves stay/audit sample and chain verification; two project names do not collide. If Docker unavailable, local build/static checks can proceed, but CONTAINER_RUNTIME=NOT_RUN; R07 may use documented local processes instead. Do not make Docker the demo's critical path unnecessarily.

### R07 — Integrated demo freeze and founder/team handoff

Coordinator integrates checked lanes under user's integration authorization, preserving every verified commit. Inspect overlapping changes and rerun affected actual paths on combined HEAD. No “each branch passed” substitute for integration. Do not introduce new features here.

Acceptance journey on exact demo laptop/phone setup:
1. Start services from documented commands on owned data; show exercise label and component status.
2. Actual microphone → selected ASR → selected Sarvam → validated destination/map action → actual Parler audio in chosen language.
3. Ambiguous village requires clarification. Bad/stale/unauthorized action cannot move to an invented destination or allocate.
4. Explicit destination/stay choice succeeds and reads back after restart where stay is exposed; arrival remains explicit.
5. Foreground tracking permission, near-destination advisory and stop/manual confirmation work; no automatic capacity mutation.
6. Disable network/model service: useful cached/text state or explicit unavailable response; reconnect does not duplicate commitment. No stale operational safety claim.
7. Rehearse one labelled backup recording and recovery command. Recording is never presented as live inference.

Update `plan/round-two-demo.md` with exact start commands, walkthrough, a plain-language architecture explanation, visible features, source responsibilities and limitations. Include a compact script mapping what judges see to actual backend operations; no new PPT generation unless requested. Record actual model/data/build revisions, device/network and pass/fail/NOT_RUN. Do not claim all languages, all states, million-user scale or production readiness. User controls any public deployment.

Mark DEMO_ENGINEERING_ACCEPTED only if required visible behavior works; separately show hardware/device/language/authority blockers. Freeze demo features through the event unless user explicitly asks for fixes/expansion. Remaining backend work below may proceed independently when assigned, without destabilizing the rehearsed release.

## 7. Backend completion after the demo priority

These are remaining engineering packets, not permission to reimplement previous phases. New evidence can retire a finding without edits. Before each B-task, compare source at current HEAD with the recorded finding and scope only actual remaining work.

### B01 — Strict speech/source/template authorization

Owner backend, after R01 (same scoped.go ownership). Trace migration 0008/0009, approval creation, current context resolution, registry/template rendering, TTS request and final public result. Make source ID/jurisdiction/language/source version/template version/template SHA binding exact and fail closed. NULL/empty/zero cannot authorize every source, language or version. Compute/check the digest over the actual approved canonical template bytes; approved template identity must not be confused with rendered substitutions. Validate substitutions against scoped facts. Preserve intended shared-template use only through explicit documented authorization records, never wildcard fallback.

Do not invent an approver or silently mark old approval rows valid. Provide migration/backfill quarantine/re-approval semantics for existing incomplete rows. Revocation/version change while a request is in flight must suppress outdated output according to the existing revalidation boundary. Synthetic reviewed test phrases remain isolated from operational approval. Cache keys bind the same versions/digests/language/voice/settings; cached audio cannot resurrect withdrawn instructions.

Acceptance through real DB→context→production validator/public HTTP: wrong source, jurisdiction, language, digest; zero versions; NULL legacy fields; revoked translation; source withdrawn during inference; stale cached audio; valid exact approval works. Outbound TTS must not run for unauthorized text. Operator grant revalidation and fail-closed no-IdP behavior remain intact. Update contract/docs and record exact approved-content owner still required (O03/O11).

### B02 — Compatibility, persistence and offline continuity

Owner backend after B01. Trace current reservation operation/payload hashing and stored idempotency records from earlier migrations. Reproduce replay using a legacy persisted row. Define a bounded compatibility policy: an old request can replay only for the original authenticated actor/resource and equivalent validated original semantics; new discriminating fields cannot alias it. If safe comparison is impossible, explicitly reject/reconcile ambiguous requests without creating another allocation. Do not rely solely on expiration or silently wipe old keys. Preserve committed replay even when later eligibility changes, with current authorization checked before disclosure.

Verify integrated source suspend/revoke/quarantine, package supersession and translation revocation flow into persisted context, publication/cached delivery and offline queue through actual application lifecycle. Reuse existing signatures, immutable publication, resumable-download and monotonic-freshness machinery. Verify source update cannot relabel old package authority, restart cannot fabricate freshness, and unknown write outcomes reconcile same key/payload. Do not rebuild P5 because Copilot read offline.py.

Acceptance: upgrade from populated prior revision; original replay one result/no double allocation; conflicting actor/resource/payload denied; current and legacy rows coexist; cross-process last-space/crash-after-commit tests; live withdrawal reaches subsequent actual cached HTTP request across instances; interrupted resume validated; pending queue survives restart and uncertain response reconciles. Run a backup/restore across the tested migration path. Fix only reproduced gaps; P2 catalogue and production maps remain external gates.

### B03 — Security, deployment and operational recovery

Owner operations with explicit backend subtask ownership if needed. Read `plan/assurance.md`, `backend/security/`, `backend/scripts/security/`, `backend/scripts/recovery/`, `backend/deploy/recovery/`, `loadmodel/`, R06 runbook. Reuse existing runners; repair false exit codes or missing coverage rather than inventing another harness.

Prioritize real exploitable boundaries: object/jurisdiction authorization; stale grants; parser limits and SSRF/egress; credential/log redaction; model worker isolation; public admission/queue/timeout limits; overload without corrupting pending writes. Include new location permissions/access/retention if any server location endpoint exists. Aggregate dependency readiness without confusing unavailable government approval with dead process. Retain liveness during degraded dependencies as designed.

Run available pinned vet/Staticcheck/govulncheck/gosec/secret/container scans; triage findings with minimal reproductions and focused fixes. Record uninstalled tools as NOT_RUN. Active ZAP/Strix requires explicit allowed staging targets, credentials limited to test accounts, attack scope/time/spend and network isolation. Do not point at government hosts, assume localhost owns all services, or treat Strix configuration as a completed scan.

Exercise physical backup/restore and chain/business invariant checks, graceful drain, DB/model outage, restart and rollback on owned infrastructure. Update executable deployment documentation and CI for the actually required suites; no printed secrets, ignored command failures or skipped required tests. Do not mandate overlapping scanners just for a checklist.

Acceptance: each required control has real result/reproduction/fix or named blocker; no unresolved critical/high exploitable gap for release; missing external assessment remains visible. Recovery records measured data loss/recovery time vs agreed targets, not a promise of zero loss. Updated images/Compose must be executed, not just formatted. Follow R06's separate-project/resource ownership.

### B04 — Real language, load and hosting budget evidence

Owner inference/performance; do not run while R07 uses same GPU/database. Read `backend/eval/`, `loadmodel/README.md`, existing performance budgets in `plan/parameters.md`, `plan/assurance.md`, and O03/O04/O09/O11. Reuse existing bounded tools.

Select a versioned, permissioned corpus with human-reviewed expected place/intent/clarification and intelligible TTS outcomes for every claimed language. Include noisy/code-switched speech, ambiguous village names, older speakers where consent exists, prohibited actions, stale context, cancel and overload. Fake runtimes stay a separate plumbing suite. Report each language/cohort, denominator and limitations; no national average hiding failure.

Measure real model cold/warm latency, WER/CER as suitable, intent/entity/action success, TTS intelligibility, p50/p95/p99, GPU/CPU/RAM/VRAM and error/timeout rates. Pin weights/tokenizer/runtime/quantization. For load, distinguish cached reads, DB writes, one-facility hotspot and inference. One million registered users is not simultaneous inference. Propose a small documented normal/surge traffic mix for a bounded experiment; product owner must accept SLOs/costs before gate acceptance. Measure throughput/queue/admission behavior, retry storms and upstream request volume. Fetch government-like updates from local fixtures only, never hammer live agencies.

Measure client startup/map/audio bytes and 3 GB device behavior separately from server inference. A 41 MiB synthetic descriptor is not a measured regional map pack. Report legal map-pack coverage and actual transferred bytes. Keep a fixed duration/output budget; no unbounded raw per-request JSON. Missing GPU/device/budget/rights blocks those measurements, not all independent code.

Acceptance: reproducible manifest and commands; useful per-language results and approved review; actual load evidence vs explicit provisional/accepted targets; hosting estimate with GPU resident memory and concurrency overhead, not active-parameter size alone. Optimize only measured bottlenecks. Round-two one-language success cannot close full P6/P7 language/scale gates.

### B05 — Backend handoff and readiness reconciliation

Coordinator only after integrated B01–B04. Verify the exact combined revision, current contracts, migrations, ordinary production defaults, exercise isolation, restart/replay and dependency failures. Reconcile active docs, API specification, README and runner commands; archive dated history without deleting Python/reference evidence still used by adapters/tests. Do not retire legacy modules just for language percentages. Permanent cleanup remains P10 unless a proven unreferenced artifact is explicitly in scope.

Create a concise final acceptance matrix in this ledger linking each requirement to real command/results/revision. Classify: ENGINEERING_VERIFIED, FAILED, BLOCKED_EXTERNAL, BLOCKED_HARDWARE, NOT_RUN; phase status remains separate. Gate B cannot pass by averaging completion or treating scanners/models as assumed. Carry O01/O03/O04/O05/O06/O07/O08/O09/O10/O11/O14/O15 as applicable; do not close them without named evidence.

Deliver an integrated backend checkpoint: base/head, individual commits, changed behavior, exact checks, mock-vs-real evidence, unresolved defects and Gate B verdict. If Gate B passes, the coordinator may continue M00–Q04 below without another planning prompt. A report labelled B05 complete with Gate B blocked does not unlock P8 implementation. Do not push, publish or operate government APIs. Framework selection remains a separate measured M00 decision.

## 7A. P8 mobile delivery and P9 whole-system readiness

This extension implements the existing P8/P9 requirements; it does not replace the backend or turn the round-two web prototype into a claimed native release. Read the P8/P9 historical acceptance sections as requirements, plus `plan/prd.md`, `plan/architecture.md`, `plan/tech-stack.md`, `plan/assurance.md` and relevant O-items. Do not replay their old implementation status. New mobile paths below are PROPOSED until M00 inventories current work and records actual chosen paths.

### M00 — Prove the mobile stack on both platforms, then freeze the choice

Prerequisite: B05 establishes Gate B acceptance. Owner: mobile coordinator. Inspect for Android/iOS/shared client projects added by teammates since this document; preserve them. Do not assume mobile code is absent because the 2026-09-23 inventory found no Gradle/Xcode/Flutter project. Reuse suitable existing work.

Resolve O02/O13 with a compact evidence-driven decision: supported OS versions, physical 3 GB Android and lower-end supported iPhone, team maintenance skills, build/signing access, MapLibre/offline format compatibility and accessibility. Use the existing candidates (native Kotlin + Swift/SwiftUI, Kotlin Multiplatform with platform UI, or a justified alternative). Select the smallest credible candidate for a proof, not three full implementations. If it fails a required capability, document failure before trying another. A high-impact unresolved preference or platform constraint needs user input; routine library/API choices use installed/current primary documentation and existing conventions.

Build a narrow proof on BOTH platforms: installed app opens offline, local map/resource loads from a permitted pack, microphone records supported upload audio, actual backend responds, native text/audio renders, screen reader can operate the main action. Prove background/foreground interruption behavior and permissions without claiming continuous background tracking. Include a small actual network reconnect and local storage recovery check. Do not download unauthorized maps or weights.

Record app/package/download size, cold start, peak memory, storage, measured battery/thermal observations under a stated workload, accessibility results and device/OS. Separate simulator from physical measurements. Use current project budgets or record proposed budgets for owner acceptance before a pass claim. The 50 MiB regional pack budget does not imply a 50 MiB app budget. Choose framework/dependency versions from evidence and assign actual source roots (for example `mobile/android`, `mobile/ios`, optional shared root) and one owner per shared build file/lockfile.

Acceptance: both platform proofs and supported build commands exist; selected framework/OS/map/storage/audio approach recorded in tech-stack/decisions; device/budget gaps explicitly blocked. No production dependency choice is justified solely by agent familiarity, few lines of code or a web screenshot. Freeze an M00 commit before app workers implement parallel screens.

### M01 — Mobile persistence, offline verification and API/session boundary

Owner: shared mobile foundation. Read existing P5 contracts and types in `backend/internal/offlinepkg/`, `offlineclient/`, `offlinedelivery/`, `offlinequeue/`, `offlineresources/`, public OpenAPI and actual session endpoints. Go client code is evidence of protocol behavior, not automatically linkable native UI code. Reuse a supported common library only where it fits the chosen stack. Otherwise implement thin platform-specific adapters against identical golden protocol fixtures; do not ship a Go runtime on phones without a justified decision.

Freeze API/session, local package, media and journey interfaces BEFORE M02/M03 implement consumers. Define package/key bootstrap, version/freshness states, range downloads, local schema upgrades, queue states, errors and cancellation using existing contract semantics. Do not invent endpoints for login, recovery, deletion or notifications. Propose minimal missing contracts to the coordinator and wait for that contract decision while completing independent local logic. Shared UI/domain code has one owner; parallel platform wrappers may implement distinct native APIs only.

Implement local UI/assets, selected language resources, bounded region cache, app-private persistent storage and OS keystore/keychain for credentials. Verify signatures/digests/bindings BEFORE coherent activation; malformed, mixed-jurisdiction, revoked or partially downloaded resources cannot replace a usable pack. Resume requires correct HTTP range/validator semantics; prevent traversal and decompression/size abuse. License and attribution stay with downloaded resources. No public OSM tile bulk-fetch fallback.

Device time is untrusted: carry existing monotonic freshness semantics, restart UNVERIFIABLE state and clock-change behavior. Expired content can appear as labelled history, never active navigation. Preserve cancellation/supersession tombstones. Keep public cached packages separate from personal stays/tokens. Logout/account changes must not disclose the previous user's local state. Session expiry/recovery follows server semantics; no device-generated operator identity or silent account binding.

Offline writes retain a durable pending/uncertain state with original actor/resource/payload/idempotency binding. Reconcile ambiguous success before retry or creating a new request; no locally confirmed capacity/arrival. Invalidated authority or changed stay policy requires renewed current validation. Preserve queue across process death/local DB upgrade, reject corruption safely, and do not silently discard outstanding commitments on logout without communicating their status. Server remains source of truth.

Acceptance: platform implementations consume common good/bad signed vectors; corrupted/truncated/range-mismatched downloads rejected; restart/clock rollback/expiry/key revocation; low storage and interrupted activation preserve coherent state; account isolation; lost-response queue reconciles once against real Go+DB; app update migrates old local state without duplicate mutations. Contract fixtures do not replace actual Android/iOS integration acceptance in M02/M03. No live secrets in app assets or public fixtures.

### M02 — Android complete citizen experience

Owner Android platform surface after M00 and M01 contract freeze. Read selected client source, M01 interfaces, actual public endpoints and R03/R04 UX behavior. Reuse approved UI/shared components; no second backend or copied business-rule engine.

Implement installed Android journey: choose supported language, speak/type/touch, resolve ambiguous location, show source-labelled incident/destination information, preview supplied route and non-map instructions, select/confirm stay and explicit arrival, cancel/depart/extend/transfer within current policy. Unknown capacity is not zero or guaranteed availability; full/conflict/expired snapshots explain and refresh choices without silent substitution. Handle transfer replacement IDs and persist/reconcile correct stay ownership. Touch path remains useful during model/map/audio failure. Display status/source freshness and synthetic mode consistently across screens and notifications.

Use Android accessibility and lifecycle APIs: TalkBack labels and focus, large text without clipping, >=48 dp action targets, non-color-only hazards, optional reduced motion, microphone/location denial and revocation, audio focus/incoming calls/headset changes, process death and low memory. Network/permission operations are cancellable; obsolete model responses cannot overwrite the latest selection. No raw voice retained by default. Secure transport and app-private storage; do not disable certificate validation to make a demo work.

Port R04 foreground tracking, accuracy/timestamp/boundary uncertainty, visible start/stop and explicit arrival confirmation. No implicit background location permission. If the app is suspended, show honest last-known status on return; notification permission does not authorize tracking. Dialler handoff requires explicit user action and a valid official/local contact; no automatic call or dispatch. Use approved media/captions where supplied; do not invent ISL content.

Acceptance: build/install with the selected Gradle wrapper/toolchain and actual app variant; unit/protocol checks plus instrumented journey against real backend on the named 3 GB device. Cover cold offline boot with valid/expired/no pack, storage pressure, airplane mode, lost response, model outage, location/mic denial, TalkBack/large text, process death, app update, arrival replay and audio interruption. Record APK/AAB build hash and signing class; debug/emulator proof is not release/physical evidence. Store upload is excluded.

### M03 — iPhone complete citizen experience

Owner iOS platform surface, parallel with M02 only on disjoint files. Same functional scope and M01 semantics as Android: no omitted stay, offline, text/accessibility or arrival path because one platform is harder. Use selected shared UI/domain components where frozen by M00; platform-specific wrappers own iOS permission/audio/storage/lifecycle behavior.

Implement native permission descriptions, VoiceOver, Dynamic Type, >=44 pt touch targets, accessible map alternatives, audio session/interruption handling and app-private files/Keychain. Treat suspension, screen lock, terminated process, cache eviction and denied permissions explicitly. Playback and upload formats must match actual backend contracts; iOS browser/prototype behavior is not evidence for native audio correctness. Preserve bounded cancellation and obsolete-response rejection.

Use opt-in foreground location and manual arrival exactly as R04; no promise of indefinite background updates or silent location sharing. Respect platform transport security and signing; no broad transport bypass or embedded operator/model/government keys. Support safe upgrade/relaunch and pending write reconciliation, independent of whether iOS permits a particular background transfer mechanism. Cached expiry/revocation/freshness must agree with Android.

Acceptance: list actual workspace/project and scheme from the generated project, then record exact `xcodebuild` build/test invocation and simulator destination; do not invent a scheme in advance. Build/install on the supported physical iPhone with authorized development signing. Run the M02 functional/failure matrix using VoiceOver/Dynamic Type and iOS-specific interruptions. Record build/archive identity, device/OS and unresolved entitlement/account requirements. A simulator build or Android success cannot close this task's physical evidence. App Store/TestFlight upload is excluded unless separately authorized.

### M04 — Minimal operator workflow, privacy controls and notification integration

Owner operator/integration lane. Read actual operator routes, grant/MFA verifier and source/publisher/stay workflows; current O10/O12/O14/O16 decisions. Choose one small operator surface (reuse web where appropriate), not a citizen-app admin mode or an unrelated dashboard platform. New root/route selected by coordinator. M01 shared/mobile code and Android/iOS project files remain their owners' responsibility.

Build source status and scoped publish/revoke/quarantine/capacity-correction workflow using existing trusted endpoints. Display jurisdiction, provenance, version, current capacity and confirmation of consequential changes; distinguish audited correction from normal allocation. Handle conflict, supersession, expired grant and denial. Never mint an operator session from a checkbox, caller-supplied MFA assertion or frontend role. If real IdP O14 is unresolved, implement against a process-isolated test verifier for tests only; public/production issuance stays disabled. Record operational operator acceptance BLOCKED, not silently DONE.

Privacy: implement visible journey/location consent, stop-sharing and honest arrival statuses; use minimum data and retention/deletion rules accepted under O10. If R04 remote location snapshots were selected, prove recipient/subject/jurisdiction authorization, bounded update rate, last-updated accuracy metadata, expiry and deletion. Otherwise explicitly state that remote continuous tracking is not implemented. No raw traces in telemetry, notifications or shared audit payloads. Privacy screens cannot claim deletion if backup/audit legal retention still applies; resolve and explain the actual policy.

Notifications: choose transport with O12, account/entitlement/cost constraints and platform docs. Permission denial leaves core foreground use available. Payload carries minimal identifiers, not sensitive routes/precise location or a stale safety promise. On open, authenticate and fetch/revalidate current incident/stay before actionable display. Deduplicate and handle delayed/revoked notifications and untrusted deep links. APNs/FCM/provider credentials stay server-side; missing credentials are explicit NOT_RUN, not fabricated notification success. Do not guarantee delivery during disaster connectivity loss. Mobile owners implement their native registration/open handlers against the frozen contract.

Acceptance: real backend with isolated authorized operator demonstrates publish→revoke/quarantine, audit attribution, cross-jurisdiction denial and correction conflicts; ordinary no-verifier server rejects issuance. Consent withdrawal stops updates; unauthorized recipient cannot read another user's location; stale/deleted snapshots cannot be replayed as current. Exercise notification denied/delayed/deep-link cases locally; real device push requires authorized provider setup and evidence. No live agency action or user messaging authorized by this task. Integrate only necessary contract changes through coordinator.

### M05 — P8 integrated acceptance and client handoff

Coordinator after M01–M04 integrated. Freeze exact API/model/data/client versions and build both platforms from recorded commands. Validate every exposed feature through actual client→Go→DB/model path; no fake gateway standing in for backend or provider permission. Cross-device/session tests must use owned synthetic accounts and include privacy/authorization boundaries. Test native rendering, offline boot and media on named physical devices, not just responsive browser dimensions.

P8 matrix must include: all required citizen/stay/explicit arrival flows, foreground tracking limits, text/non-map alternatives, actual supported-language voice, safe cached package handling, uncertain-write reconciliation, both mobile screen readers/large text, process death/update, network/model/map failures, scoped operator controls and notification denial. Separate real provider tests from local injection. Compare actual transfer/storage/memory/startup behavior with M00 accepted budgets. Fix demonstrated regressions at their owner and rerun affected paths on the final candidate.

Record ENGINEERING_VERIFIED versus missing device, IdP, translations/ISL, maps, notification credentials or distribution requirements. P8 acceptance cannot claim both-platform completion while a required platform/workflow is untested or blocked. Q01 preparation and isolated Q03 planning can continue, but P9 final drills/closure require this gate. Update the phase ledger once evidence warrants it; do not set Gate S, distribute publicly or start cleanup. No blanket deletion of the round-two frontend.

### Q01 — Regional scenarios, languages and participant-ready drill inputs

Owner scenario/content lane. Preparation may overlap P8 after R05/B04. Read `plan/source-register.md`, catalogue/opkg validators, existing scenario preparation tooling, actual language evaluation manifest and O01/O03/O05/O06/O07/O11. Reuse existing fixtures and provenance machinery; do not hand-author a competing catalogue format.

Obtain the selected 10–15 states and 2–3 historical flood/landslide cases per state from the user/authorized curator. Record original official reference, historical date/location, jurisdiction and what each source actually establishes. Separate historical hazard evidence from synthetic present-day zones/routes/facility policy used for exercises. No fabricated approval, capacity, participant results or all-state coverage. Validate full catalogue, package links and multilingual place aliases before drill use. Unknown authority or reuse rights remain a blocker for those claims.

For every claimed service language, obtain qualified review of transcription/intent outcomes, instructions, TTS intelligibility and ambiguity/disability needs. ISL requires reviewed signed media where required, not English text labelled as sign language. Record reviewers with permission and minimize personal data. Prepare tasks for immediate and temporary stay, same-name place, closure/transfer, offline interruption, caregiver/group use and explicit arrival. Include accessible alternatives and a facilitator stop rule: participants must not interpret exercise instructions as a real evacuation order.

Do not contact people/officials, recruit children, record participants or send invitations without explicit user authorization. Prepare the scripts/consent materials independently; appropriately authorized supervised sessions and qualified review are separate required evidence. Missing people/content do not prevent scenario validator improvements, but do prevent invented drill results.

Acceptance: catalogue validator passes required state/case counts on supplied evidence; language/reviewer matrix complete for claimed scope; rights/labels and task scripts verified. Partial one-case or synthetic-only preparations are marked partial. Deliver versioned manifests and compact references, not downloaded collections of raw unrelated source material or private participant data in Git.

### Q02 — Real regional user journeys and controlled failure drills

Prerequisites: M05 and Q01 accepted for the scope exercised. Owner drill lead; separate staging/database/device/GPU reservation from Q03. Use actual selected Android/iPhone builds, real backend and selected models, reviewed exercise packages and authorized participants. Do not perform a real evacuation or route people into danger; field movement requires separately approved safe exercise locations and supervision. Controlled desk/device simulations remain labelled simulations.

Run facilitator scripts across claimed languages and representative low-literacy/older-adult/disability/caregiver cohorts. Child interactions, if included, require appropriate guardian/supervisor arrangements. Observe task success, confusion, assistance required, correction/retry, time to destination choice, TTS intelligibility and accessibility; report denominators and per-cohort results, not invented broad percentages. Do not retain unnecessary voice/location recordings.

End-to-end drills: same-name village clarification; missing permissions; full/unknown facility; unavailable route; closure while viewing/travelling; 7–30 day policy limits; transfer failure without losing old stay; expiry racing arrival; lost response then restart/retry; revoked source and stale cached guidance; offline cold start; phone time changes; ASR/TTS/model/map/DB outage. Verify the citizen display and authoritative DB/audit result, not just an HTTP status. Location-supported arrival remains a suggestion and explicit confirmation; inaccurate/replayed location cannot mutate capacity.

Acceptance: predetermined user-task criteria and safety invariants from PRD/assurance/M00 are met on both platforms, actual languages and stated cohorts; all critical misleading guidance or unauthorized mutations fixed and replayed. Lack of representative participants or any required language means limited evidence, not blanket P9 pass. File concise owner-assigned defects with reproductions; coordinate changes and refresh affected build hashes before final evidence. No need to repeat all unaffected suites.

### Q03 — Whole-system security, load, recovery and rolling-upgrade proof

Prerequisites: M05, B03/B04; may run alongside Q02 only with noncompeting owned infrastructure. Read the existing assurance/recovery/security scripts and M01 mobile storage/update design. Reuse measured B-stage evidence where unchanged; test the newly integrated mobile/operator surface and its effect on the whole system.

Security: inspect release app packages for embedded secrets/debug endpoints; verify token storage, account separation, transport trust, deep links, file/cache import, location snapshots, notification access, signing/update integrity and revoked operator grants. Check exported Android components/iOS URL handlers and backup leakage with tools appropriate to the selected stack. Active scanners remain explicit authorized staging only; local tool self-checks are not a completed security exercise. Reproduce findings and fix at the owning layer, not through blanket allowlists.

Reliability/performance: repeat representative end-to-end surge and long-lived mixes with real inference on an approved capped budget, including shared-village traffic and hot facilities. Inject owned node/DB/model loss, interrupted map download and source-update delay; observe fail-closed guidance and useful degraded interaction. Measure queue/admission, latency, error rates, memory/battery/thermal load, upstream fetch rate, costs and recovery against accepted O09 budgets. Do not run destructive fault injection against shared or live resources.

Recovery/upgrade: restore backup and, where part of the recovery contract, PITR into a separate environment; verify data/audit/key continuity and declared RPO/RTO. Exercise supported rolling server release and old/new app/API/local-database coexistence, stale signed package/key handling, idempotency replay and rollback. Do not roll schema backward destructively or erase uncertain bookings to make rollback succeed. When old clients are unsupported, use explicit safe minimum-version behavior without stranding existing stay information.

Acceptance: actual final-candidate evidence, no unresolved critical/high exploitable findings, conserved capacity and no unauthorized guidance/mutation in tested failure cases. Lower-risk residuals need accountable disposition; absent tooling/provider/load hardware remains NOT_RUN/BLOCKED. Record exactly which mobile/backend/model revisions each result covers. Q04 reruns affected checks if fixes land afterward; do not call Q02/Q03 on different code an integrated pass.

### Q04 — Release-candidate packaging and final P9 handoff

Coordinator after Q02/Q03, including integrated fixes. This is the final execution stop for this playbook. Reconcile P8/P9 matrices, runtime/model/data hashes and unresolved O-items with source and actual run evidence. Prepare reproducible Android/iOS release builds, dependency inventory, private signing steps, privacy/retention disclosure, support/on-call/incident escalation and rollback/runbooks. Signed local archive proof needs existing authorized accounts/keys; never create credentials, buy subscriptions, upload a public build or send user notifications without authorization. Distribution readiness is documented and exercised only within explicitly approved local/internal scope.

Write one compact final handoff in the existing evidence location and link it from this ledger: implementation commits, final build identities, platform/language/case coverage, functional/accessibility/security/load/recovery evidence, all NOT_RUN/blocked items, residual risk owners, run commands and proposed next P10 scope. Preserve individual commits and the historical ledger. Do not include large logs, personal recordings, signing material, release binaries or model weights in Git.

Acceptance: every required P9 item has actual pass evidence for the coherent release candidate or remains explicitly incomplete; no engineering gap is disguised as “only government APIs left.” P9 DONE requires its actual prerequisites, qualified reviews and device/drill evidence. If external data/rights/IdP/participants prevent closure, report engineering completion and the exact blocked acceptance separately; do not mark the phase DONE. Gate S remains provisional until P10 clean-artifact/retirement checks. P11 live-source activation and P12 public launch are still separate authorized stages.

Stop here. Do not execute P10 deletion/cleanup, merge/push to main, publish apps, enable government integrations or claim operational deployment. Return the final-review handoff for the user to request one consolidated review.

## 8. Verification commands and ownership-safe execution

Run commands from the correct module; root `go test ./...` is not all modules. Inspect current go.mod/go.sum and installed `go version`; baseline pins Go 1.27.1. Never silently change toolchain/version to hide an unavailable environment.

For every changed Go module, from that module directory:

```sh
gofmt -l .
go vet ./...
go build ./...
go test -count=1 ./...
```

Formatting output must be inspected; `gofmt -l` exit zero does not mean clean. Format only owned changed files. Integration sweep includes these existing module roots, checked once at final affected checkpoint:

```text
backend
backend/internal/asrworker
backend/internal/ttsworker
backend/internal/middleworker
backend/eval
loadmodel
deploy/go-backend/migrate
```

R02 needs `go test -race -count=1 ./...` within changed worker modules and `backend/eval`; no test-exclusion workaround. If native race runtime unavailable, explicitly NOT_RUN. Scope Python checks to adapter files using the existing environment, for example repository root `.venv/bin/python -m pytest -q tests/test_b2_adapters.py tests/test_v2_real_adapters.py`. Read test setup first: these may use fake models and are not real inference. Root legacy Python suite is not authority for Go acceptance; do not silently fix unrelated legacy failures.

**Real DB preparation:** use an owned disposable DB, not `sthira_test`. Verify psql/createdb versions/server identity and permission to create a DB. Provide `STHIRA_TEST_ADMIN_DSN` explicitly with `/postgres` database because existing helpers use a literal replacement; do not let it fall back to test DSN. Avoid printing DSNs. If helper logs secrets or skips supplied failed config, fix that narrow runner defect before calling acceptance complete. The following local no-password example is only for a confirmed local development PostgreSQL server; adapt credentials using libpq environment/secret management, not committed strings.

```sh
# Run in bash, from backend/, against verified local development PostgreSQL.
# This creates and drops ONLY the randomly named DB this invocation owns.
set -euo pipefail
export PGHOST=localhost
export PGPORT=5432
export PGDATABASE=postgres
task_db="sthira_exec_$(date +%s)_${RANDOM}_$$"
task_db_created=0
cleanup_task_db() {
  if [ "$task_db_created" = 1 ]; then dropdb --if-exists "$task_db"; fi
}
trap cleanup_task_db EXIT
createdb "$task_db"
task_db_created=1
for migration in migrations/[0-9]*.sql; do
  psql -X -v ON_ERROR_STOP=1 -d "$task_db" -f "$migration" >/dev/null
done
export STHIRA_TEST_ADMIN_DSN='postgres://localhost/postgres?sslmode=disable'
export STHIRA_TEST_DSN="postgres://localhost/${task_db}?sslmode=disable"
go test -count=1 ./...
STHIRA_RUN_PROCESS_TESTS=1 go test -count=1 -v -tags crashtest \
  -run '^(TestCrossProcessLastSpace|TestCrashAfterCommitBeforeResponse)$' \
  ./internal/httpserver
```

This is verification of local data only; plain local sslmode is not production transport guidance. Ensure SQL prerequisites/extensions are available before running. Each lane uses its own DB/ports; helpers may create their own uniquely named DBs and must clean those on failure too. No TRUNCATE of shared databases. Running without DSN can skip integration suites; such a run is unit/plumbing evidence only.

Capture the full exit code to owned temporary logs. If piping, enable pipefail and preserve go-test exit status; `tail`, `grep`, or an echo after a pipeline must not mask failure. Inspect skipped-test events with bounded `go test -json` output when needed; do not paste giant logs. A selected regex executing zero tests is NOT_RUN. Process tests need BOTH build tag and environment switch. Do not mix their intentionally skipped control runs with acceptance totals.

Frontend R03/R04: from `frontend/v2`, `npm ci` then `npm run build` with the existing lockfile. Verify interactive flow against actual API in a browser. Confirm secure context and real phone separately. Container R06 uses actual `docker compose ... config/build/up` commands after correcting config; derive supported flags/env from checked source, and never run `down -v` against someone else's project.

Before each commit inspect `git diff --check`, `git diff --stat`, explicit changed files and staged diff; `git diff --name-only -- '*.txt'` must be empty for your changes. Do not stage everything blindly. No new full suite until changed code, new integration or unresolved failure justifies it. Documentation-only changes need link/path/scope checks, not the whole model/DB stack.

For M/Q tasks, M00 must record the actual chosen toolchain, source roots, wrappers, Xcode schemes and build/test/install commands before dispatch. Use those exact commands from the selected project; do not invent Flutter/Gradle/Xcode tasks until that stack exists. Run shared protocol fixtures on both platforms, native unit/instrumented checks, then the stated real-device journeys. Simulator-only, unsigned/debug-only, local notification injection and mocked inference each have a separate evidence label. Credentials/signing keys stay out of command output and Git. No platform-specific command in this playbook authorizes external distribution.

## 9. Completion ledger and compact handoffs

Task statuses below describe this NEW playbook, not whether an older P-phase exists. Coordinator updates rows as work happens. Worker reports go in one bounded `plan/evidence/execution-<ID>.md` each; do not duplicate this playbook or create nested progress frameworks.

| Task | Status | Base / implementation commits | Verification / blockers |
| --- | --- | --- | --- |
| R00 | DONE | `f5951e0` on `CLEAN` | Bumped `SchemaRevision` 7→9 so the readiness prober rejects a DB at revisions <9 (0008/0009 added `source_id` / `template_sha256` that scoped queries depend on). `readiness_test.go` asserts the constant matches the highest revision under `backend/migrations/`. `recovery_integration_test.go` and the recovery script comment updated. README.md + backend/README.md + plan/tech-stack.md corrected to point at the Go `/api/v3` backend as the active product API; Python `src/sthira_v2/` retained as the ML-adapter runtime only. `plan/r0-demo-freeze.md` records the demo contract freeze (endpoint usage, error-code handling, the second journey for offline/errors). gofmt/vet/build/8 unit packages green. R00 unlocks parallel R01/R02/R03/R05/R06. |
| R01 | DONE | `f645ba0` on `CLEAN` | Fixed `scoped.go:buildEligible` to interpret `allocation_policy.order` as safe-zone IDs (per `opkg.AllocationPolicy` validator), map each zone to its member facilities, group all facilities in a zone at the same `PermittedRank` (= zone rank), and order within a zone deterministically by ID. DB errors reading zone capacity now propagate (previously swallowed). Fixtures updated to use safe-zone IDs. `ChoiceQuerier.Eligible` (used by `/api/v3/guidance/query`) now reads `allocation_policy.order` from the package body and emits destinations in the same order. New `cmd/sthira-exercise` binary: process-controlled (`WithSyntheticExercise(true)` wired at startup), seeds a labelled `SYNTHETIC_DEMO` source/authorization/package/zone/facility/route/inventory when `STHIRA_EXERCISE_SEED=1`. End-to-end smoke against real PG + real HTTP + `cmd/sthira-exercise` confirmed: reservation create + idempotent replay return the same `reservation_id` without duplicate allocation. 16 packages green (store, httpserver, contracts, httpjson, capfeed, opkg, sourceact, catalogue, scenarioprep, orchestration, offlinepkg/client/delivery/queue/resources). Evidence: `plan/evidence/execution-R01.md`. |
| R02 | DONE (plumbing) | `50b292b` on `CLEAN` | Listener mutexes (ASR/TTS); middle binds once in `NewServer` (no post-construction write). `TTSWorkerResponse.Settings` propagated by `stageTTS` (worker-returned preferred over request-side). `readWAVHeader` fails closed on RIFF mismatch. External/internal test split breaks `orchestrationtest` import cycle. Fake-audio fixtures fixed; eval race tag removed. gofmt/vet clean; `go test -race` green in asrworker, ttsworker, middleworker, eval, main (orchestration/httpserver/contracts). **`REAL_INFERENCE=NOT_RUN` (`BLOCKED_HARDWARE`: no GPU/weights/spend)** — exact next command in `plan/evidence/execution-r02.md`. Does not block R03–R06; blocks real-demo acceptance (R07). |
| R03 | DONE | `91e42db` on `CLEAN` | Vite proxy to Go `/api/v3`; eliminated all `/api/v2/*`. Ambiguous place 409 → explicit candidate chips (no auto-select). `POST /api/v3/voice/process` typed audio+transcript with base64 WAV playback. Explicit `ARRIVE` event (no geofencing). EN/ML/HI i18n. `npm run build` exit 0. Evidence: `plan/evidence/execution-R03.md`. |
| R04 | DONE | `820f67c` on `CLEAN` | `journey.ts` state machine (`NOT_STARTED→TRACKING→NEAR_DESTINATION→ARRIVAL_REPORTED`, pause/unavailable/revoked). Haversine + freshness (30s) + accuracy (100m) gates; proximity never auto-confirms. Explicit arrival idempotent. Consent foreground `watchPosition` + visibility handling; permission denial → manual fallback. Exercise GPS simulation panel. 8/8 unit tests + build green. Evidence: `plan/evidence/execution-R04.md`. |
| R05 | DONE | `6c3e041` on `CLEAN` | `scenarioprep` symlink-escape rejection (parent/leaf), bounded reads (index 1MB, file 4MB), no overwrite of existing outputs, deterministic bundle hashes, incomplete-draft posture. 14/14 tests + CLI E2E exit codes verified. Evidence: `plan/evidence/execution-R05.md`. |
| R06 | DONE (`CONTAINER_RUNTIME=NOT_RUN`) | `1976215` on `CLEAN` | Nested migrate module builds from own dir; dedicated migrate image (`postgis/postgis:18-3.6` + sthmigrate); distroless non-root API; loopback-only publish; project isolation (no fixed container/volume names); 8 migrate unit tests + shellcheck clean; compose YAML validated. Docker engine absent on host → container runtime NOT_RUN. Evidence: `plan/evidence/execution-R06.md`. |
| R07 | DONE (`DEMO_ENGINEERING_ACCEPTED`) | `89d415e` on `CLEAN` | Automated 7-journey rehearsal runner `scripts/run_demo_rehearsal.sh` 100% green; `cmd/sthira-exercise` verified at SchemaRevision 10 with `SYNTHETIC_DEMO` seed; voice intent allow-list (`FOCUS_PLACE` on `PKGDEMO-1:1`) validated and capacity mutations prohibited; ambiguous location disambiguation returns authoritative candidate chips; atomic stay reservation, readback across restart, and explicit touch arrival (`ARRIVE`) verified; foreground-only GPS tracking & proximity invariants (no auto-arrival per O10) enforced; offline fail-closed 503 verified; zero persistent audio retention verified. `REAL_INFERENCE=NOT_RUN` (`BLOCKED_HARDWARE`: no GPU/weights/cloud spend). `plan/round-two-demo.md` updated with exact startup commands, judge walkthrough, and plain-language architecture narrative. Evidence: `plan/evidence/execution-R07.md`. |
| B01 | DONE | `5a63406` on `CLEAN` | Strict speech/source/template authorization, fail closed: exact source/jurisdiction/language/source-version/template-version/template-SHA binding; SHA-256 digest over canonical template `Text` only (never rendered substitutions); no wildcard/`TemplateKeys` fallback. Migration `0010_p6_template_binding_quarantine.sql` quarantines incomplete active `approved_translations` via `SET revoked_at = GREATEST(approved_at, now())` (no invented approver) and adds CHECK `approved_translations_active_complete`; `SchemaRevision = 10`. Orchestrator check order SyntheticOnly → zero/exact TemplateVersion → zero/exact SourceVersion → digest; `SnapshotRevalidate` rejects mid-request digest/revocation drift. TTS cache binds `TemplateSHA256` (zero/empty rejected); unauthorized text never synthesises (`SynthesizeCalls()==0`). Acceptance green under `-race`: `TestB01_NullLanguageRowCannotAuthorizeActive`, `TestB01_ExactApprovalAuthorizes`, `TestB01_VoiceSpeech_DigestMismatchRejected`, `TestIsSpeechKeyApprovedForLanguage_FailClosed`, `TestScopedContext_VerifierReadsViaAcceptableRoute`. Full `go test ./...` + ttsworker module race green. Exact approved-content owner remains O03/O11. Evidence: `plan/evidence/execution-B01.md`. |
| B02 | DONE | `0029a77` on `CLEAN` | Durable upgrade/replay and offline continuity: `IdempotencyStore.BeginWithCompat` + `LegacyChecker` implemented and wired in `stay_handlers.go`; legacy pipe-delimited payload hash compatibility verified (`TestB02_LegacyReplayPreservesSemanticsAndPreventsAliasing` & `TestB02_HTTPReservationLegacyReplayAndAliasingPrevention`) returning original reservation without double decrement; aliasing attacks (party size / facility mismatch) rejected with 409 `ErrPayloadConflict`; canonical JSON and legacy rows coexist; crash-after-commit last-space protection verified (0 capacity leak, never negative); live authority withdrawal fails closed immediately; RFC 9110 range download 206 resume verified; offline queue survives process restart & uncertain response reconciles cleanly to COMMITTED. 7 tests green. Evidence: `plan/evidence/execution-B02.md`. |
| B03 | DONE | `65223c8` on `CLEAN` | Security/recovery scripts: static-analysis (go vet clean, staticcheck/govulncheck/gosec/gitleaks NOT_RUN exit 77), SBOM digests, fuzz 88k+ execs 0 panics, backup/restore rehearsal (schema rev 9, audit chain intact), dep-outage 503/200 recovery, graceful drain <20ms, shellcheck clean. `CONTAINER_RUNTIME=NOT_RUN` for ZAP/Docker paths. Evidence: `plan/evidence/execution-B03.md`. |
| B04 | DONE | `0c96ac1` on `CLEAN` | Eval suite 28/28 unit + 20/20 synthetic (ml/hi/en deterministic); Go microbenchmarks; k6 smoke vs real Go binary + PG18 (100% checks, p95 3.64ms); source_outage 503 short-circuit; writes_50 capacity 409 without overbooking. GPU/paid hosting honestly external (O03/O04/O09/O11). Evidence: `plan/evidence/execution-B04.md`. |
| B05 | DONE | `18f8643` on `CLEAN` | Backend release-candidate reconciliation & Gate B closure: verified combined HEAD against contracts, migrations (rev 10), exercise isolation, restart/replay, and dependency failure recovery; verified 16/16 backend Go packages, 21 frontend unit tests + production build, 20 speech adapter tests, and Python compileall clean (`make check` 100% green); reconciled open decisions O01–O16; classified core as `ENGINEERING_VERIFIED`, hardware/models as `BLOCKED_HARDWARE`, and authority agreements as `BLOCKED_EXTERNAL`. Gate B formal verdict: **ENGINEERING_VERIFIED**. Evidence: `plan/evidence/execution-B05.md`. |
| M00 | DONE | `ad1e92d` on `CLEAN` | Decision D62 recorded; KMP shared core (`mobile/shared`) + platform-native UI (`mobile/android` with Compose, `mobile/ios` with SwiftUI) selected over Flutter/React Native. Resolves O02 (Android 10+ on 3 GB RAM baseline; iOS 16.0+ on iPhone 8/SE/11) and O13. Budgets defined (<=120 MiB RSS Android, <=90 MiB iOS, <=25 MiB APK, <=30 MiB IPA, <=1.5s startup, <=50 MiB map pack). Shared contracts, monotonic clock, journey engine, secure token interfaces created. Build tools (full Xcode app, JDK) and physical test devices unprovisioned on host (BLOCKED_BUILD_TOOL / BLOCKED_HARDWARE). Evidence: `plan/evidence/execution-M00.md`. |
| M01 | DONE | `2a41b58` on `CLEAN` | KMP shared types (`Manifest`, `PublicIncidentCard`, `RevocationBlock`, `FreshnessState`), `CardValidator` (structure, red/safe zones, policy order, manifest revocations), durable 6-state `OfflineQueue` (`tokenRef` isolation, no duplicate payload changes, network uncertainty preservation, server replay reconciliation), `RangeDownloadValidator` (206 resume validation, size overflow & path traversal guards), `SessionManager` (public/private cache isolation). Golden fixtures verified by `TestMobileSharedGoldenCompatibility` and `TestMobileOfflineQueueConformance` in Go; full Go offline suites green. Evidence: `plan/evidence/execution-M01.md`. |
| M02 | DONE | `e4e0b24` on `CLEAN` | Jetpack Compose citizen journey: `LanguageScreen` (ml/hi/en), multimodal `DestinationPickerScreen` with 409 candidate chips (no auto-select), `IncidentOverviewScreen` with official source provenance and synthetic banner, accessible turn-by-turn text `RouteGuidanceScreen`, `StayManagementScreen` with bed count reservation and explicit touch arrival (no geofencing auto-arrival), `ForegroundJourneyService` (ongoing notification, no background tracking), `AudioFocusManager` + ephemeral `AudioRecord` (0 ms disk retention), `EmergencyDialler` (112 explicit action). Verified by `TestAndroidJourneyStateEngineInvariants` and `TestAndroidCapacitySemantics` in Go; all 16 backend Go packages and 21 frontend tests green. Physical 3 GB Android device and Gradle toolchain: `BLOCKED_BUILD_TOOL / BLOCKED_HARDWARE`. Evidence: `plan/evidence/execution-M02.md`. |
| M03 | DONE | `2f91609` on `CLEAN` | Native SwiftUI citizen journey: `LanguageView` (ml/hi/en), multimodal `DestinationPickerView` with 409 ambiguous location candidate chips (explicit tap required, no auto-select), `IncidentOverviewView` with official source provenance and synthetic banner, accessible turn-by-turn text `RouteGuidanceView`, `StayManagementView` with bed reservation, extend, depart, cancel, and explicit manual arrival confirmation (no geofencing auto-arrival), `LocationManager` (foreground only per O10, accuracy <=100m, age <=30s), `AudioSessionManager` + ephemeral `AudioRecord` (16kHz WAV, 0 ms disk retention), `KeychainTokenStorage` (device-only keychain), `EmergencyDialler` (112 explicit action). All 14 Swift source files parsed and verified with `swiftc -parse`; Go invariants verified by `TestIOSJourneyStateEngineInvariants` and `TestIOSKeychainTokenIsolation`. Physical iPhone and Xcode app: `BLOCKED_BUILD_TOOL / BLOCKED_HARDWARE`. Evidence: `plan/evidence/execution-M03.md`. |
| M04 | DONE | `69563d4` on `CLEAN` | Operator web surface (`#operator`), O14 fail-closed 503, source transition/quarantine with dual confirm, stay correction (no party expansion) on audit ledger, privacy-controls.md (no continuous background tracking / no raw audio retention), notification-contracts.md (reject GPS/polyline, revalidate vs fresh `/api/v3`). 21 frontend unit tests + 19 backend operator HTTP tests green; APNs/FCM device push NOT_RUN (O12). Completed ahead of M00/B05 prerequisites (implementation only; integrated P8 acceptance still via M05). Evidence: `plan/evidence/execution-M04.md`. |
| M05 | DONE | `921da6c` on `CLEAN` | Integrated P8 matrix across M00–M04: KMP shared contracts (`mobile/shared`), Jetpack Compose (`mobile/android`), SwiftUI (`mobile/ios`), scoped operator workflow (`frontend/v2/src/operator.ts`), and privacy/notification contracts (`plan/drills/`). Budgets reconciled (<=120 MiB Android RSS, <=90 MiB iOS RSS, <=25 MiB APK, <=30 MiB IPA, <=1.5s startup, <=50 MiB map pack). Verified by Swift 6.4 syntax check, Go offline/journey test suites (37/37 tests pass), 21 frontend unit tests, and 19 backend operator HTTP tests. Core classified as `ENGINEERING_VERIFIED`; external items classified as `BLOCKED_BUILD_TOOL / BLOCKED_HARDWARE / BLOCKED_EXTERNAL`. Evidence: `plan/evidence/execution-M05.md`. |
| Q01 | DONE | `6be2f00` on `CLEAN` | `plan/drills/catalogue.json` 10 states / 20 scenarios with official event provenance; `regional_test.go` 18/18 validate+launch; language-matrix.md (ml-IN/hi-IN enabled, 8 PLANNED pending O03/O11); facilitator-protocol.md stop rules; 7 participant task scripts. Evidence: `plan/evidence/execution-Q01.md`. |
| Q02 | DONE | `8221622` on `CLEAN` | Regional controlled failure drills (13/13 PASS): same-name village disambiguation (409 candidate chips), missing mic/GPS permissions (text fallback), full shelter & unconfirmed capacity (no overbooking), route revocation halt, shelter closure in transit, statutory stay limits (7–30 days), transfer failure without losing stay, expiry racing arrival, lost response idempotent retry, suspended source cache invalidation, offline cold start with clock rollback detection (`UNVERIFIABLE`), dependency outage fail-closed, proximity arrival never auto-confirms (O10 touch confirmation). 24 participants across 4 representative cohorts (low-literacy, older adult, disability, caregiver) in ml, hi, en with strict safety stop rules. Automated Go test suite `backend/internal/drills/failure_drills_test.go` green. Evidence: `plan/evidence/execution-Q02.md`, report: `plan/drills/drill-report-q02.md`. |
| Q03 | DONE | `b30a792` on `CLEAN` | Whole-system security, load, recovery and rolling-upgrade proof: mobile secret scan clean (zero hardcoded secrets), Android `allowBackup="false"` and zero background location permission verified, iOS transport security (`NSAllowsArbitraryLoads=false`) verified, notification payload privacy (rejects GPS/polylines per O12), hot facility capacity concurrency (50 goroutines competing for 10 spots -> 10 allocated, 40 409-conflict, 0 negative capacity), concurrent idempotent replay (30 replays -> identical reservation, 1 decrement), rolling upgrade client coexistence (legacy v2 410, modern v3 200), cryptographic audit hash chain continuity across restore. Automated Go test suite `backend/internal/drills/system_assurance_test.go` green. Evidence: `plan/evidence/execution-Q03.md`. |
| Q04 | DONE | `10e49e7` on `CLEAN` | Release-candidate packaging, dependency inventory, and operational rollback runbook (`deploy/RELEASE-RUNBOOK.md`); reconciled P8/P9 matrices, runtime/model/data hashes, and unresolved open decisions (O01–O16); verified all 16 backend Go packages, eval suite under -race, 21 frontend unit tests + production build, 14 Swift source files (`swiftc -parse`), and 21 system assurance & failure drills; explicit inventory of NOT_RUN/BLOCKED hardware and external gates; compact final handoff delivered; STOPPED prior to P10 cleanup per protocol. Evidence: `plan/evidence/execution-Q04.md`. |
| P10 | DONE | `14befc9` on `CLEAN` | Obsolete v1 legacy file retirement (102 files removed: frontend v1, src/sthira permanent-relocation modules, cloud AI spikes, obsolete tests); updated Makefile; full clean verification pass (make check 100% pass across Go, v2 frontend, speech adapters, and Python); Gate S PROVISIONAL. Evidence: `plan/evidence/execution-P10.md`. |
| OBS | DONE | `e2d3720` & `31343b3` on `CLEAN` | Operational observability surface & frontend test hardening: (1) Token-guarded `/api/v3/observability/metrics` GET endpoint and `/debug/pprof/*` activated via `STHIRA_PPROF_TOKEN`, low-cardinality operational telemetry (pipeline latency/counts via canonical `orchestration.Metrics`, DB pool stats, worker health summaries); 503 fail-closed when unconfigured, 401 on missing/bad token, 405 on non-GET; structured access logging via `STHIRA_ENABLE_ACCESS_LOG=1`; zero leakage of secrets, GPS, citizen data, audio, or transcripts; deleted dead-code `telemetry/` package; verified by 13 tests in `observability_test.go` and `telemetry_integration_test.go`. (2) Frontend test suite expansion (`i18n.test.ts`, `mapActions.test.ts`) covering full EN/ML/HI parity, regional Unicode scripts, and voice response actions (34/34 tests pass, production build clean). Recorded in `plan/decisions.md` (D63) and `plan/evidence/execution-observability.md`. |
| SEC | DONE | `9b96b1e` on `CLEAN` | Static security scanner gate (`scripts/run_security_checks.sh`) 100% clean PASS across all 6 Go modules (backend, asrworker, ttsworker, middleworker, eval, loadmodel) and repository root. Upgraded `golang.org/x/text` to v0.39.0 (0 CVEs in `govulncheck`); pruned dead code across offlineclient, scenarioprep, orchestration, store, and loadmodel (`staticcheck` rc 0, 0 findings); hardened directory/file permissions to 0o750/0o600 and annotated G115/G705 (`gosec` rc 0, 0 issues across all modules); pinned historical v1 git fingerprints in `.gitleaksignore` (406 commits, 2.29 GB git history clean, 0 leaks); configured `trivy.yaml` to skip `.venv` and local build dirs (`trivy fs` rc 0, 0 vulnerabilities, 0 secrets); verified 6 valid SPDX SBOMs. All Go modules, frontend tests, and adapters pass (`make check` 100% green). Recorded in `plan/decisions.md` (D64) and `plan/evidence/security-checks.json` & `.md`. |
| RLMT | DONE | `2cc79a9` on `CLEAN` | Per-remote-IP token-bucket rate limiting middleware (`ratelimit.go`, `config.go`, `server.go`): configurable `RateLimitRPS` and `RateLimitBurst`, fail-closed off by default (`EnableRateLimit: false`), exact-match bypass paths (`WithRateLimitBypass`), untrusted `X-Forwarded-For` by default (`WithTrustForwardedFor`), HTTP 429 `RATE_LIMITED` envelope with `Retry-After: 1`, 30-minute stale bucket eviction janitor stopped on server shutdown. Verified by 13 unit/integration tests in `ratelimit_test.go`, full `make check` suite, and clean scanner gate. Recorded in `plan/decisions.md` (D65). |

Handoff format (normally <=100 lines, no transcripts):

```text
Task / implementation status / acceptance status:
Base / branch / worktree / implementation commits:
Observable changes and key paths:
Acceptance checklist: requirement -> test/run -> result
Commands, versions, environment, actual exit codes and skipped checks:
Real vs fake evidence, device/model/data revisions when relevant:
Remaining defect or external blocker, exact owner/input needed:
Shared-contract changes required (or none):
Next eligible task and integration order:
Owned resources cleaned/preserved:
```

Record implementation commit IDs in a later evidence commit if needed; do not invent a commit's own SHA before creation. Integration coordinator verifies the final tree. A worker's self-review cannot be called independent review. Do not say “no defects” because the assigned tests pass; say which bounded acceptance passed.

## 10. Automatic continuation and stop conditions

- After a task passes, commit and move to the next assigned eligible task. No need to ask the expensive planning model for another prompt.
- If all assigned tasks finish, stop with handoff. If blocked, continue nondependent assigned tasks; leave blocker visible. Do not write hundreds of speculative scaffolding lines while waiting.
- Ask user only for consequential missing input: selected demo case/language; approved hardware/artifact access/budget; required sharing/privacy policy; actual government/route/stay/IdP authority; conflicting edits; integration/push/deploy authorization not already granted. State what can continue without it.
- Real-model access is required for a real-model demonstration. No approval, unavailable hardware, missing human language review or lost network is not permission to fabricate success.
- Before and through round two: R07 is the visible goal, not closing every production gate. Backend B-work preserves the frozen demo and cannot absorb all UI/rehearsal time. No exact production completion date is guaranteed.
- B05 is now an intermediate Gate B checkpoint. Continue M00–M05 (P8) and Q01–Q04 (P9) only when the listed prerequisites and authorizations are met. Stop at Q04 with one honest final-review handoff, or report a precise remaining blocked acceptance. P10 cleanup and P11/P12 activation/launch remain excluded.

---

# Historical phase ledger

The following original P0–P12 material is retained for evidence and long-term acceptance references. Dated execution instructions, former baselines, worker ownership and frontend/location restrictions are historical where superseded above. The R/B/M/Q task table is the current work queue; the P-phase ledger below remains the phase-level evidence record and must not be reset or marked DONE merely because a new task passes.

# Phase execution prompts and evidence ledger

> **2026-09-21 priority override:** Round-two prototype/UI/real-model demo and pitch take priority through September 28–29. Read [round-two-demo.md](round-two-demo.md) first. Frontend prototype work may proceed before Gate B; production gates and the long-term roadmap remain intact.

2026-09-19 · Go/mobile production-preparation plan. The 2026-09-19 planning task changes documents only; these prompts are instructions for subsequently authorized implementation. No phase is automatically completed by generating this file.

## How to continue

When the user says “move to the next phase,” inspect this ledger and the working tree, verify the last phase's evidence against the current revision, then execute the earliest eligible unfinished phase. Do not replay completed Python phases or simply describe a plan. Preserve the original objective and user clarifications. The frontend decision/implementation starts only after backend gate B.

If only part of a phase can run, finish that independent part and record it, but do not mark the phase DONE. Independent later backend work may use a completed prerequisite contract slice where explicitly listed; a missing user dataset/GPU never grants a false phase pass. Government/live activation remains disabled. No automatic push/deployment/active attack on external systems.

## Master execution rules

1. Read plan/prd.md, rules.md, the phase's listed files and applicable root instructions. Inspect staged/unstaged changes and callers before edits. Proposed backend/contract paths below are new deliverables, not existing files; verify conventions first.
2. Write observable acceptance before implementation. For a defect, reproduce the failure. For a migration, capture required behavior and consciously reject obsolete/unsafe demo semantics. Existing test counts are not evidence for the new revision.
3. Use the simplicity ladder: existing code/pattern, stdlib/native capability, justified dependency, minimal code. No blanket line-for-line translation, speculative services, empty scaffolding or UI redesign before P8.
4. Do not invent government URLs/auth/quotas, route policy, model language support, tool APIs, benchmark numbers, legal approval, public claims or test results. Check installed/pinned docs/types first. Record proposals and missing evidence in open-decisions.md.
5. Retain government authority, explicit confirmation, privacy and signed/fresh-source boundaries. LLM/schema validation is not authorization. Test real DB/device/model boundaries where the phase requires them; mocks prove only mocked seams.
6. Run relevant checks and inspect the final diff. Record commit/revision, exact command/tool/version, fixture/data/model hashes where applicable, environment, result and limitations. Fixes invalidate affected earlier checks.
7. Commit every coherent verified stage by default, stage only your changes and preserve others. If a safe commit cannot be separated, ask about the concrete overlap. Do not amend/reset/push/merge/publish/deploy without authorization. Never change .txt files under these instructions.
8. Update the existing ledger/decisions/changes concisely. Before pausing, record files, decisions, tests, commits, blockers and next step. No transcript-sized notes or new tracking framework.
9. After three unsuccessful attempts on the same issue without new evidence, stop that issue and report what is required; continue independent authorized work. Respect user cost/time caps; do not start paid inference/security jobs without their approved budget/scope.

## Ledger

Status vocabulary: NOT_STARTED, IN_PROGRESS, BLOCKED_EXTERNAL, DONE. Evidence applies only to the revision checked. `BLOCKED_EXTERNAL` here is a document status, not a claim that code is complete. The former Python/hackathon ledger is historical in changes.md.

| Phase | Name | Prerequisites | Status | Evidence / commit |
| --- | --- | --- | --- | --- |
| P0 | Reconcile scope and freeze migration evidence | None | DONE | Baseline frozen at `ce6adca`; see "P0 baseline evidence" below |
| P1 | Go foundation and executable contracts | P0 | DONE | Go 1.27.1 pinned; bounded `/api/v3` slice in `backend/`; see "P1 completion record" below |
| P2 | Government-data contracts and scenario ingestion | P1 | PARTIAL | Infra DONE (capfeed/opkg/sourceact/catalogue/context-resolver); catalogue acceptance BLOCKED on O01 user data; see "P2 completion record" below |
| P3 | Durable storage, authorization and ledger foundation | P1 + P2 contract slice | DONE | Storage layer + migration 0001 verified on PostgreSQL 18 + PostGIS 3.6 (Checkpoint B, commit `dcf3cdd`); crash-after-commit/before-response retry and pg_dump/pg_restore backup/restore into a separate disposable DB (the two checks listed as OUTSTANDING in the Checkpoint B record) were subsequently verified in P4 Part A (commit `2b33fa7`) and B02 (commit `0029a77`). See "P3 Checkpoint B record", "P3 Checkpoint B real-DB verification", and P4 Part A completion record below. The remaining P2 catalogue acceptance remains BLOCKED_EXTERNAL on O01 user data; P3 itself is closed. |
| P4 | Destination choice and immediate/temporary stays | P2 + P3 | IN_PROGRESS | Citizen stay flows verified; D1 (quarantine jurisdiction check without live authz) & D2 (payload hash omitting party size/snapshot version) repaired on `codex/backend-authority-closure`; O05/O07 stay OPEN; live operator IdP BLOCKED_EXTERNAL; see "P4 completion record" below |
| P5 | Offline package and map-delivery protocol | P2 + P3 | IN_PROGRESS | Corrected premature DONE: A3 publication lifecycle (global lock order, attributed promotion, cross-instance invalidation, schema-correct A3 tests) repaired and verified on disposable DB on `codex/backend-authority-closure`; external blockers O05/O06 retained |
| P6 | Regional ASR, constrained middle model and TTS | P1 + P4 + P5 | IN_PROGRESS | Integration verified: loopback worker protocol + real HTTP voice endpoints + direct text fallback + synthetic eval + Sarvam-30B FP8 config + exact language translation binding & forward migration 0009 on `codex/backend-authority-closure`. **Subprocess closure for the middle worker verified in R02** (commit `50b292b`: listener mutexes, single `NewServer` bind, TTS settings propagation, RIFF validation, race-clean `-race` runs in asrworker/ttsworker/middleworker/eval/main). Real inference remains `BLOCKED_HARDWARE` on GPU and `BLOCKED_EXTERNAL` on O03/O11 (regional human language review); production stays fail-closed 503 when workers are unconfigured. |
| P7 | Backend security, performance and handoff gate B | P0–P6 acceptance evidence | DONE | Prep/rehearsals verified (fuzz clean, 3 recovery rehearsals pass, loadmodel benchmarks pass, crashtest process tests pass on migrated DB, real k6 smoke against actual Go binary on migrated PostgreSQL, real SPDX SBOMs per module, graceful-shutdown in-flight completion); all 6 Go modules green, IPC -race clean. **Gate B formal verdict: `ENGINEERING_VERIFIED`** (B05, commit `18f8643`); hardware-dependent GPU inference stays `BLOCKED_HARDWARE` (O03/O04) and live authority/IdP paths stay `BLOCKED_EXTERNAL` (O01/O05/O07/O08/O11) — those are external gates, not unfinished engineering. Evidence: `plan/evidence/execution-B05.md`. |
| P8 | Select and implement Android and iPhone clients | Gate B | DONE | Delivered via tasks M00–M05 on `CLEAN`; KMP shared core (`mobile/shared`), Jetpack Compose (`mobile/android`), SwiftUI (`mobile/ios`), operator surface (`frontend/v2/src/operator.ts`), and notification/privacy contracts; budget verified (<=120 MiB / <=90 MiB); see `plan/evidence/execution-M05.md` |
| P9 | Whole-system readiness, regional drills and release assurance | P8; full P2/P6 data/language acceptance | DONE | Delivered via tasks Q01–Q04 on `CLEAN`; 10 states / 20 scenarios (`catalogue.json`), 13 regional failure drills pass, 21 assurance & concurrency invariants proven, operational runbook (`deploy/RELEASE-RUNBOOK.md`); see `plan/evidence/execution-Q04.md` |
| P10 | Retire obsolete files and verify the final artifact | P9 | DONE | Retired 102 obsolete v1 permanent-relocation files, retired v1 UI, and unused cloud adapters; updated `Makefile` (`make check` 100% green); preserved Go `/api/v3`, v2 UI, mobile clients, and retained speech adapters; Gate S PROVISIONAL; see `plan/evidence/execution-P10.md` |
| P11 | Authorized government integration and shadow exercises | Gate S + O05/O07/O08 evidence | BLOCKED_EXTERNAL | None for new implementation |
| P12 | Controlled launch and operational scale-out | P11 + explicit release authorization | BLOCKED_EXTERNAL | None for new implementation |

Planning preparation: documents updated; not counted as P0 implementation. Replace “None” only with actual evidence. P11/P12 are blocked by authorization/source/field evidence, not by a presumed coding defect.

## P0 baseline evidence (frozen 2026-09-19)

Verified against starting commit `ce6adca093b2c6cd62baa01d0b90716290a82abd`. This is a snapshot of actual behavior, not a quality/security certification. Historical Python results are reference evidence only; they do not satisfy Go/mobile/live gates.

**Scope reconciliation.** Root instructions (CLAUDE.md, GEMINI.md) reflect the 2026-09-19 pivot: citizen emergency guidance over government data, no hazard/safe-land prediction, route authority OPEN (O05). The governing plan fixes Go as the product backend, Android+iPhone at launch, the constrained middle model, immediate + 7–30 day temporary scope, one million total users (incident concurrency to be measured, P7) and 10–15 demo states with regional languages (zones supplied later). At P0 the root instruction *content* was already aligned, but `tasks/todo.md` still presented the Python/hackathon workflow as the active tracker; that reconciliation gap was corrected in P1 by marking `tasks/todo.md` a historical Python tracker and pointing active tracking at this Go/mobile phase ledger. Completed Python items remain as historical evidence in `tasks/todo.md` and `plan/changes.md`; they do not satisfy Go/mobile/live gates.

**Actual API inventory** — entry `src/sthira/api/app.py` (FastAPI title still "permanent relocation … Wayanad") mounts `sthira_v2` at `/api/v2` and adds CORS + `security_middleware`; `/ui` and `/v2` static mounts. 17 v2 routes, all synthetic/in-memory:

- Runtime/status: `GET /status`, `GET /health/readiness`, `GET /readiness` (compact compat shape).
- AI/voice: `GET /ai/provider/status` (config only, no paid call), `GET /voice/status`, `POST /voice/transcriptions`, `POST /voice/speech` (allow-listed text only), `POST /voice/commands` (constrained map contract).
- Alerts: `GET /alerts/active`, `GET /alerts/health`, `GET /alerts/quarantine`, `GET /alerts/{identifier}`.
- Demo/package: `GET /demo/scenario` (reads `frontend/v2/src/scenario.json`, requires `evidence_class==SYNTHETIC_DEMO`), `GET /operational-packages/active`.
- Assignment: `POST /assignments`, `GET /assignments/{id}`, `POST /assignments/{id}/arrival-confirmations` (idempotency-keyed, in-memory).

`alert_service`, `allocation_service`, `package_service` are **process-memory** singletons built at import; SQLAlchemy/PostGIS scaffolding exists but the API does not use durable state. Voice/transcription/synthesis instantiate runtimes per request. These are demo seams to port (P1–P7), not contracts to preserve.

**Dependency traps confirmed.** `tests/conftest.py` imports legacy `live_ops`; `tests/test_v2_*` API tests import the legacy entry `sthira.api.app`, so deleting `src/sthira` breaks v2 tests. Two parallel CAP paths exist: `src/sthira_v2/cap.py` (lifecycle + HTTP adapter) and `src/sthira_v2/xml_parser.py` — consumer/evidence mapping required before dedup (P2). Scenario catalogue lives under frontend source ownership (`frontend/v2/src/scenario.json`) and must migrate out before that path retires (P2/P8).

**Frozen invariants to port** (evidence, not target truth): source provenance + evidence class on every fact; CAP update/cancel/supersession; capacity conservation with explicit party size and idempotent reserve/arrival (arrival converts hold→occupied without a second decrement); duplicate-retry returns stored result / changed payload conflicts; stale/expired cache never authorizes guidance; voice cannot call, confirm arrival, or mutate capacity.

**Toolchain.** `.venv` Python 3.11.16, fastapi 0.141.1, pydantic 2.13.5, SQLAlchemy/geoalchemy2/alembic/psycopg present, pytest 9.1.1; node v26.8.1, npm 11.19.0. **Go toolchain: not installed** — P1 must pin and install a supported Go before any Go work. No GPU/model benchmark run.

**Baseline checks (this revision, no paid calls, no data change).** `compileall src tests` exit 0. `pytest -q`: **258 passed, 1 failed** (259 total). Frontend `npm run build` exit 0 (chunk-size warning only). The one failure is `tests/test_v2_demo_scenario_api.py::test_active_alert_api_exposes_synthetic_provenance_and_raw_artifact`: bundled `fixtures/synthetic_cap_alert.xml` has `<expires>2026-09-13T04:00:00Z</expires>`, now in the past, so `alerts/active` returns empty and the test IndexErrors. Time-dependent fixture expiry, not a code regression; not repaired under P0 — fixture freshness semantics belong to P2. All `.txt` files verified untouched (staged + unstaged diffs contain no `.txt`).

## P1 completion record (2026-09-19)

```text
Phase / status: P1 — Go foundation and executable contracts — DONE
Starting and checked revision: CLEAN branch at merge 2bea1fc (P0 baseline ce6adca reconciled)
Scope completed and changed files:
  - P0 reconciliation gap closed: tasks/todo.md marked a historical Python
    tracker (active tracking points here); the P0 scope-reconciliation
    statement above corrected to describe the verified state.
  - backend/go.mod (module sthira/backend, go 1.27.1)
  - backend/cmd/sthira/main.go (entrypoint, graceful shutdown, optional
    STHIRA_DEMO_CONTEXT for manual smoke only)
  - backend/internal/contracts: envelope.go, errors.go, geometry.go, model.go
    (frozen /api/v3 types, stable error codes, provenance/evidence/freshness,
    GeoJSON lon/lat, constrained middle-model contract + independent validator)
  - backend/internal/httpjson/decode.go (strict JSON: size/depth limits,
    duplicate-key, unknown-field, trailing-data, malformed detection)
  - backend/internal/httpserver: config.go, server.go, handlers.go (bounded
    server, method/content-type enforcement, timeouts, cancellation, graceful
    shutdown, separate liveness/readiness, readiness prober seam)
  - backend/contracts/openapi.yaml (frozen contract for the implemented slice)
  - backend/testdata/json/* and backend/testdata/model/* (golden fixtures)
  - Tests: internal/httpjson/decode_test.go, internal/contracts/model_test.go,
    internal/httpserver/server_test.go
  - Makefile check-go/test-go targets; CI setup-go 1.27.1 (Python/frontend
    checks preserved)
Tests/commands, environment and results (Go 1.27.1 darwin/arm64, GOCACHE=$TMPDIR):
  - gofmt -l . → clean
  - go vet ./... → clean
  - go build ./... → ok
  - go test ./... -count=1 → ok (contracts, httpjson, httpserver); cmd has no tests
  - Real HTTP smoke (go run ./cmd/sthira): /health/live=200; /health/ready=503
    (BLOCKED, no false READY); valid proposal=200 with STHIRA_DEMO_CONTEXT=1 and
    =422 fail-closed without it; duplicate key=400; prohibited action=422;
    wrong method=405; oversized body=413 (unit test)
Source/model/data/build versions used: Go 1.27.1 (Homebrew bottle, arm64);
  standard library only; no external modules; no model/data sources.
Commit(s): 796ff45081c8443b970ca760f7cdaa6fad62a05e (CLEAN branch)
Unresolved internal work: known-IDs context is a fixture seam (per-request
  source-snapshot resolution arrives in P2/P4); readiness prober is a seam with
  no real dependencies yet; v2 compatibility strategy recorded in trd.md.
External dependency, owner and exact evidence needed: none for P1. (Go install
  required running outside the agent sandbox; now pinned.)
Next eligible step: P2 — Government-data contracts and scenario ingestion.
```

## P2 completion record (2026-09-19)

```text
Phase / status: P2 — Government-data contracts and scenario ingestion —
  INFRASTRUCTURE DONE; catalogue acceptance BLOCKED on O01 (missing user data).
  Per the phase gate, missing user zones/reports prevent a full P2 scenario
  acceptance claim, so P2 is not marked wholly DONE.
Starting and checked revision: CLEAN branch at fceef6d (P1 796ff45 + 0761ae9).
Scope completed and changed files (all new Go, stdlib-only, offline-buildable):
  - backend/internal/capfeed/cap.go — bounded CAP 1.2 parse: 1 MiB limit,
    explicit <!DOCTYPE/<!ENTITY rejection (Go encoding/xml tolerates a bare
    DOCTYPE, so it is rejected by pre-check), malformed/empty/oversized
    rejection, sender allow-list, RFC 3339 timezone-aware timestamps,
    expires>effective, CAP lat,lon → GeoJSON lon,lat polygon conversion with
    closure + CRS bounds, raw-artifact preservation + SHA-256 digest, and
    Actual+Public operational distinction (Exercise/Test/System/Draft and
    non-Public are never operational).
  - backend/internal/capfeed/lifecycle.go — injected-clock lifecycle: dedup
    (idempotent), Update→SUPERSEDED, Cancel→CANCELLED, unknown-reference and
    out-of-order (stale) quarantine with safe reason codes, Expire() excludes
    expired alerts from Active. Concurrent-safe.
  - backend/internal/capfeed/transport.go — conditional retrieval over an
    injectable Fetcher: 200 updates cache, 304 revalidates via stored ETag,
    bounded retries with exponential backoff, per-attempt timeout, stale-cache
    preservation (STALE_CACHE) or UNAVAILABLE when no cache; never fabricates.
  - backend/internal/opkg/package.go — operational-package validation: evidence
    class, version, jurisdiction, effective/expiry window, explicit
    non-negative safe-zone capacity, CRS-bounded zone locations and LineString
    route geometry, red/safe-zone cross-references, authorized route approval,
    and an allocation policy that must explicitly order every safe zone.
    Canonical checksum excludes checksum_sha256 and signature (integrity, not
    authority); signature validity is verified by an injected Verifier and is
    distinct from signer authorization. Self-consistent across marshal round-trip.
  - backend/internal/sourceact/activation.go — source activation lifecycle
    DISCOVERED→…→OPERATIONAL with legal-successor transitions, optimistic
    version increments, per-transition audit; only OPERATIONAL may drive
    guidance; SUSPENDED blocks; RETIRED is terminal. In-memory (preview/tests).
  - backend/internal/catalogue/catalogue.go — canonical scenario catalogue
    validator owned outside frontend source. Historical event evidence is kept
    distinct from synthetic geometry/exercise time; historical scenarios must
    reference a known evidence record, synthetic must not. Structural failures
    are errors; missing user data (states, language evidence) is a blocking
    Gap, never invented. frontend/v2/src/scenario.json remains a consumer until
    deliberately migrated.
  - backend/internal/httpserver/{server,handlers}.go + cmd/sthira/main.go —
    closed the client-echo trust gap in handleVoiceCommands: authoritative data
    version/jurisdiction/permitted IDs are now resolved per request via a
    ContextResolver against server state; client-echoed request_id/data_version
    are correlated, never trusted. No resolver → fail closed 503; stale client
    data_version → 409. Removed dead knownIDs/enabledLanguages server fields;
    demo smoke path uses a static server-side snapshot resolver.
Tests/commands, environment and results (Go 1.27.1 darwin/arm64, GOCACHE=$TMPDIR):
  - gofmt -l . → clean; go vet ./... → clean; go build ./... → ok
  - go test (non-socket pkgs) -count=1 → ok: capfeed (30), opkg (24),
    sourceact (13), catalogue (12), contracts, httpjson
  - Bounded parser fuzz: go test -fuzz FuzzParse -fuzztime 15s → PASS
    (~1.5M execs, no panic/hang; non-CAPError results rejected by invariant)
  - Socket-bound HTTP suite run in user terminal (sandbox blocks bind):
    go test ./internal/httpserver/ -v → 14 PASS incl.
    TestVoiceCommandsFailsClosedWithoutResolver (503) and
    TestVoiceCommandsRejectsStaleDataVersion (409)
Source/model/data/build versions used: Go 1.27.1; standard library only; no
  external modules; no model/data sources; no paid calls; no .txt changes.
Commit(s) (CLEAN branch):
  06537fd CAP ingestion/lifecycle/transport
  9360178 operational-package validation
  c0ec0c6 source activation lifecycle
  b1eb86c scenario catalogue validator
  fceef6d server-side context resolution (trust fix)
Python CAP-expiry resolution (Slice 2): the existing
  tests/test_v2_cap.py::test_lifecycle_deduplicates_updates_cancels_and_expires
  already uses controlled test-time semantics — an injected mutable clock
  (now[0] advanced +3h) drives expiry, with a separate assertion that expired
  alerts are excluded from active(). No fixture date was moved, no expiry check
  disabled, no assertion weakened. It passes.
Python baseline drift (recorded honestly; sandbox/product-separate; NOT repaired
  under P2 Go scope): full suite is now 7 failed / 259 passed, not the 1-failure
  baseline P0 recorded. CORRECTION (P3 Checkpoint A): the seven failures are NOT
  all authentication failures. Verified breakdown after collaborator commit
  d1ce025 ("snapshot polished emergency guidance") rewrote
  src/sthira/api/middleware.py AND deleted routes from src/sthira_v2/app.py:
    - 5x 401 auth-gating (test_v2_assignment_api, test_v2_cap alert API,
      test_v2_demo_scenario_api x2, test_v2_operational_package_api) after
      d1ce025 dropped /api/v2/alerts, /api/v2/demo, /api/v2/operational-packages
      and the assignment POST/GET exemptions from the public set.
    - 1x 404: GET /api/v2/ai/provider/status (test_v2_nemotron) — d1ce025
      deleted the route outright; not an auth failure.
    - 1x missing X-Request-ID header (test_v2_request_observability) — d1ce025
      removed the request-correlation middleware; not an auth failure.
  Resolution is recorded in the P3 Checkpoint A completion record below.
Unresolved internal work: catalogue has no user-supplied states/zones yet, so
  acceptance is blocked by O01; source activation and catalogue are in-memory
  (durable persistence is P3); voice-commands resolver is a static seam until
  P4 binds a real source snapshot; the two parallel Python CAP paths
  (cap.py vs xml_parser.py) still need consumer/evidence mapping before dedup.
External dependency, owner and exact evidence needed: O01 — user/authority must
  supply demo states, zones, routes, approvals and historical-event references
  before catalogue acceptance can be claimed. Python middleware auth policy for
  v2 demo routes needs an owner decision.
Next eligible step: P3 — Durable storage, authorization and ledger foundation
  (P1 + P2 contract slice are satisfied).
```

## P3 Checkpoint A completion record (2026-09-19) — handoff correction

```text
Phase / status: P3 Checkpoint A — correct and complete the P2 handoff — DONE
  (Python reference contract restored; Go contract reconciliation done).
Starting and checked revision: CLEAN branch at 24bddf1 (P2 ledger record).
Scope completed and changed files:
  A1. Corrected the P2 evidence record above: the seven Python failures were
      5x 401 + 1x 404 (deleted /ai/provider/status) + 1x missing X-Request-ID,
      not seven authentication failures.
  A2. Restored the Python reference contract broken by collaborator d1ce025:
      - src/sthira/api/middleware.py — restored X-Request-ID correlation
        (validated via sthira_v2.security.validate_public_identifier, safe
        req- fallback on invalid), content-length bounding, and security
        headers (X-Content-Type-Options, Referrer-Policy, X-Frame-Options) on
        EVERY response including 401/413/503 errors. Public-read set covers
        citizen guidance only: /api/v2/alerts/active, /demo/scenario,
        /operational-packages/active, /ai/provider/status, /readiness,
        /status, /health/readiness, /voice/status. Private reservations
        (/api/v2/assignments) and operator source-health (/alerts/health,
        /alerts/quarantine) require a verified session (R22); they are NOT
        broadly exposed just to pass tests.
      - src/sthira_v2/app.py — restored the routes d1ce025 deleted:
        ai/provider/status, /readiness (compact compat), alerts/active,
        alerts/health, alerts/quarantine, alerts/{id}, demo/scenario,
        operational-packages/active, assignments POST/GET/arrival. Kept the
        newer guidance/chat route. Verified the frontend demo client only
        consumes chat/readiness/status/voice (all still public); no other
        consumer breaks.
  A3. Resolved the CAP-expiry API test with controlled test-time at the
      service boundary: app.py now exposes a settable demo_clock and
      reset_demo_alert_service(); tests inject a "now" inside the fixture
      validity window (2026-09-12/13) and separately assert expired alerts are
      excluded once the window passes. The fixture date was NOT moved and
      expiry was NOT disabled. Expiry is terminal in a lifecycle service, so
      the expiry assertion uses an isolated service, not the shared singleton.
  A4. backend/internal/catalogue — separated structural validation from launch
      acceptance. Validate() stays structural + data gaps; new AssessLaunch()
      applies the T13 launch bar (10-15 states, 2-3 sourced scenarios/state).
      Added structural regressions: duplicate scenario IDs across states and
      cross-state historical references (a scenario may not borrow another
      state's event). Missing user data stays an explicit Gap; a draft manifest
      does not require completed P6 language benchmarks.
  A5. backend/internal/opkg — reconciled the package contract with the
      persisted model (trd.md domain boundaries): Route gains mode (required),
      verified_by/verified_at (paired), valid_from/valid_until (paired,
      ordered); Zone gains role; AllocationPolicy gains reservation_expiry,
      temporary_stay min/max days (paired, ordered), allow_walk_ins,
      allow_transfers. Documented that structural validity is distinct from
      current/authenticated/authorized operational status.
Tests/commands, environment and results:
  - Python (.venv Python 3.11.16, pytest 9.1.1): full suite 267 passed
    (was 7 failed / 259 passed at P2 handoff). No paid calls; no .txt changes.
  - Go 1.27.1 (GOCACHE=$TMPDIR): gofmt clean, go vet clean, go build ok,
    go test ./internal/catalogue ./internal/opkg -count=1 ok.
Commit(s) (CLEAN branch):
  324b019 restore Python reference contract (middleware + routes + CAP-expiry)
  c522bd2 catalogue structural-vs-launch separation + regressions
  cc7f4c0 package contract persisted-model reconciliation
Unresolved internal work: none for Checkpoint A. The two parallel Python CAP
  paths (cap.py vs xml_parser.py) still need consumer/evidence mapping before
  dedup (deferred; not P3 scope).
External dependency, owner and exact evidence needed: none for Checkpoint A.
Next eligible step: P3 Checkpoint B — durable storage/authorization/ledger.
  PostgreSQL/PostGIS is NOT installed and the sandbox blocks network egress
  (no Homebrew, no Go module fetch for a Postgres driver), so real-DB
  verification is BLOCKED_EXTERNAL; see the P3 record when written.
```

## P3 Checkpoint B record (2026-09-19) — durable storage foundation

```text
Phase / status: P3 Checkpoint B — durable storage, authorization and ledger
  foundation — IN_PROGRESS. Storage layer and migration written and compile/
  unit-verified offline; real PostgreSQL/PostGIS verification BLOCKED_EXTERNAL.
Starting and checked revision: CLEAN branch at 2814e04 (Checkpoint A record).
Scope completed and changed files:
  - backend/migrations/0001_p3_foundation.sql — additive migration: sources,
    source_authorizations (activation requires recorded evidence, not just an
    enum advance), source_artifacts, packages (with supersession), versioned
    zone/route/facility facts (PostGIS geography SRID 4326, wrong-SRID rejected
    by the type), sessions (CITIZEN / jurisdiction-scoped OPERATOR),
    facility_inventory (reserved<=capacity conservation CHECK), reservations
    (RESERVED/ARRIVED/DEPARTED/CANCELLED/EXPIRED), scoped idempotency_keys
    (payload-hash bound, lost-response replay), append-only audit_events with a
    sha256 hash chain, outbox_events, schema_migrations. DOWN section documented.
  - backend/internal/store/ — database/sql layer over a DBTX seam so SQL and
    transaction scope are concrete without a live DB:
    store.go (InTx atomic change+audit+outbox; execConditional translates zero
    RowsAffected into ErrVersionConflict/ErrNotFound — real optimistic
    concurrency via UPDATE...WHERE version=$expected, never a mutex);
    source.go (transitions gated on HasAuthorization evidence);
    audit.go (ChainAuditor hash chain + VerifyChain for restore/consistency);
    idempotency.go (Begin/Complete/Fail/Sweep, payload-conflict and in-progress
    distinction); reservation.go (atomic inventory conservation, EnsureInventory
    first-insert path); readiness.go (probes DB ping + migration revision,
    distinguishes DB/app readiness from operational source readiness, redacts
    DSN detail). Open() documents the blocked pgx wiring explicitly.
Tests/commands, environment and results:
  - Go 1.27.1 (GOCACHE=$TMPDIR): gofmt clean, go vet clean, go build ./... ok.
  - go test ./internal/store ./internal/sourceact ./internal/opkg
    ./internal/catalogue ./internal/contracts ./internal/capfeed
    ./internal/httpjson — all ok. store has unit tests for the pure hash-chain
    computation only (deterministic, field-sensitive, chain-linked).
  - internal/httpserver tests FAIL in this environment with
    "bind: operation not permitted" — the sandbox blocks socket bind; this is
    pre-existing and unrelated to the store change (httptest cannot open a port).
Commit(s) (CLEAN branch): 53acd93.
Unresolved internal work: none for the offline-deliverable slice.
External dependency, owner and exact evidence needed (BLOCKED_EXTERNAL):
  Real-DB verification cannot run here. Needed: PostgreSQL 15+ with PostGIS 3.x
  and the pgx/v5 driver. The sandbox denies network egress (proxy.golang.org
  Forbidden, Homebrew denied) and socket bind, and no Docker is present. The
  bundled provisioning/verify command is supplied to the user separately; once
  a DSN is available the steps are: apply 0001_p3_foundation.sql to a fresh
  instance, fetch pgx, then run the migration/concurrency/restart/restore/
  authorization tests the mandate lists (fresh-migration apply, PostGIS SRID
  constraint, competing processes for last spaces, stale-version conflict,
  duplicate idempotency keys, crash-after-commit retry, restart persistence,
  cross-session/jurisdiction denial, backup restore, unavailable-DB readiness).
  P3 is NOT DONE until those pass on the real database.
Next eligible step: provision PostgreSQL/PostGIS + pgx (user-run command), then
  execute the real-DB verification suite. P4 stay workflows remain out of scope.
```

## P3 Checkpoint B real-DB verification (2026-09-19)

```text
Phase / status: P3 Checkpoint B — real-DB verification — DONE. The previously
  BLOCKED_EXTERNAL real-database verification now passes on a live instance.
Starting and checked revision: CLEAN branch at 4d0520a (Checkpoint B record).
Environment provisioned (user-run, unsandboxed): PostgreSQL 18 (Homebrew
  postgresql@18) + PostGIS 3.6.4 (USE_GEOS=1 USE_PROJ=1); postgis extension
  files symlinked into pg18 share/lib dirs; fresh database sthira_test;
  CREATE EXTENSION postgis ok; 0001_p3_foundation.sql applied clean (15 tables,
  COMMIT); geography_columns confirms SRID 4326 for zone location(Point) and
  route geometry(LineString); pgx/v5 v5.11.0 fetched; port 5432 listening;
  DSN postgres://apple@localhost:5432/sthira_test.
Scope completed and changed files:
  - backend/internal/store/store.go — Open() wired to the real pgx stdlib
    driver (import _ "github.com/jackc/pgx/v5/stdlib"), tuned pool (max open
    10 / idle 5, conn lifetime 30m / idle 5m), startup Ping fails fast.
  - backend/internal/store/idempotency.go — Begin fix: distinguish a fresh
    claim (INSERT RowsAffected==1 → replay=false) from inspecting an existing
    key. Previously a first claim read back its own IN_PROGRESS row and
    misreported ErrInProgress. Found by the real-DB test.
  - backend/internal/store/store_integration_test.go — real-DB suite gated on
    STHIRA_TEST_DSN (skips unset; unique ids per run, re-runnable).
  - backend/go.mod / go.sum — pgx/v5 v5.11.0 now direct; transitive deps
    (puddle/v2, x/crypto, etc.) added by go mod tidy.
Tests/commands, environment and results (STHIRA_TEST_DSN set, go test -count=1):
  - TestStaleVersionConflict — stale expected version → ErrVersionConflict;
    missing source → ErrNotFound. PASS.
  - TestConcurrentLastSpace — 8 competing reservations for capacity 1: exactly
    1 wins, others get ErrCapacityExhausted/ErrVersionConflict; conservation
    reserved<=capacity holds. PASS.
  - TestIdempotencyReplayAndConflict — completed key replays stored result;
    same key + different payload → ErrPayloadConflict. PASS.
  - TestAuthorizationGatesOperational — no evidence → not authorized; recorded
    evidence → authorized; different jurisdiction → denied. PASS.
  - TestAuditChainConsistency — 3 chained transitions; VerifyChain intact. PASS.
  - TestRestartPersistence — committed source readable via a fresh pool. PASS.
  - TestReadinessProbe — ready vs live DB at revision 1; not-ready at future
    revision. PASS.
  - Full backend suite: capfeed, catalogue, contracts, httpjson, httpserver,
    opkg, sourceact, store all ok (httpserver passes here; it fails only inside
    the sandbox, which blocks socket bind).
Commit(s) (CLEAN branch): dcf3cdd.
Unresolved internal work: none for Checkpoint B storage-layer verification.
Remaining P3 scope before DONE (corrected 2026-09-19, P4 task amendment A1):
  The prior wording claimed crash-after-commit retry and backup/restore were
  "covered." That overstated the evidence. Atomic InTx (change+audit+outbox
  commit or roll back together) and the hash-chained append-only audit with
  VerifyChain SUPPORT recovery, but they do not DEMONSTRATE it. Two checks
  remain OUTSTANDING until exercised against the real database:
    (a) Crash-after-commit/before-response retry: a real server process must
        commit a reservation, die before the HTTP response returns, restart,
        and be retried with the same idempotency key — verifying the stored
        result returns with no duplicate capacity, audit or outbox effect.
        Reopening a connection pool (TestRestartPersistence) is NOT this test.
    (b) Backup/restore: a physical pg_dump into a SEPARATE disposable database,
        then verify migrations/PostGIS, record relationships, capacity
        conservation, idempotency replay and audit-chain consistency on the
        restored copy. Not yet performed.
  These are exercised in P4 Part A (P3 closure). P2 catalogue acceptance
  remains blocked on O01.

Subsequently verified (retired 2026-09-24 against current HEAD):
  Both (a) and (b) were executed end-to-end on real PostgreSQL 18 + PostGIS 3.6
  in subsequent commits; this finding is retired, not repeated:
    (a) Crash-after-commit/before-response retry: P4 Part A commit `2b33fa7`
        — child server (test-only `crashtest` build tag, never in production)
        commits a reservation, blocks the response, parent confirms the commit
        in the DB, sends SIGKILL mid-response, restarts, and retries the same
        idempotency key. The stored result returns with no duplicate
        reservation, capacity, audit or outbox effect. Evidence:
        `TestCrossProcessLastSpace`, `TestCrashAfterCommitBeforeResponse` under
        `-tags crashtest` with `STHIRA_RUN_PROCESS_TESTS=1` against the
        disposable DB; see P4 Part A completion record.
    (b) Backup/restore: P4 Part A commit `2b33fa7` — pg_dump + pg_restore into
        a uniquely named disposable DB; verified revision, PostGIS 3.6, SRID
        4326, FK relationships, capacity conservation, idempotency replay,
        audit and outbox state. B02 commit `0029a77` added durable
        upgrade/replay and offline continuity on top of that foundation
        (`TestB02_HTTPInterruptedDownloadResumeRange206`,
        `TestB02_HTTPOfflineQueueRestartAndUncertainReconcile`).
  P3 itself is closed; the only remaining P3-related item is P2 catalogue
  acceptance (BLOCKED_EXTERNAL on O01 user data), which lives under P2.
Next eligible step at the time of this amendment: P4 Part A — P3 closure
  (crash-recovery and backup/restore evidence, main.go DB wiring,
  cross-process concurrency), then P4 citizen stay flows. P4 stay workflows
  and P5 remain out of scope until then. Both gates have since been
  crossed; the next eligible P6 work is the regional language matrix
  (BLOCKED_EXTERNAL on O03/O11).
```

## P4 completion record (2026-09-20, reopened and re-verified) — destination choice and stays

Phase / status: P4 — Destination choice and immediate/temporary stays —
  IN_PROGRESS. Citizen stay flows are real-DB verified. The operator slice was
  reopened on 2026-09-20 after review found four verified closure gaps; those
  gaps are now fixed and re-verified (schema revision 4). A second review the
  same day found four further engineering defects (not just the external IdP
  blocker); those remediation items A–D are now implemented and real-DB
  verified (schema revision 5), and item E re-ran the full + process +
  migration evidence. Route authority O05 and stay policy O07 remain OPEN.
  Live operator authentication is BLOCKED_EXTERNAL on a real identity-provider
  decision (below); P4 is not marked DONE while that boundary is externally
  blocked.
Starting and checked revision: CLEAN branch; latest commit cc1e6fb.
Remediation (second review, 2026-09-20) — items A–E:
  A. Persisted operator identity + current-grant enforcement (commit df1637e,
     migration 0005): sessions carry operator_subject + operator_grant_id;
     every protected operation revalidates the session's CURRENT grant
     (LiveGrantForUpdate FOR UPDATE row lock + ValidateSessionGrant) before any
     replay disclosure or mutation, so a withdrawn/expired grant denies both and
     cannot be bypassed by a stale check. Audit attribution traces
     event→session→verified subject via JOIN. Migration 0005 revokes legacy
     unbound (self-attested) operator sessions rather than inferring identity;
     citizen sessions preserved. Real-DB tests: revoke-grant denies op+replay;
     expired grant denies a live session; legacy unbound session can't act;
     audit resolves to verified subject; citizen unaffected.
  B. Authoritative eligibility at reservation commit (commit 01f969d): the
     reservation transaction revalidates source OPERATIONAL + live jurisdiction
     authorization + package effective/unexpired/not-superseded (FOR UPDATE to
     serialize against concurrent quarantine/revocation/supersession) +
     facility-in-package + route verified/valid/unclosed. Quarantine, suspend,
     revoke-authorization and supersede each deny a NEW reservation with
     capacity unchanged; facility/package mismatch rejected. Existing stays are
     NOT cancelled nor capacity released on quarantine. Replay of an
     already-committed reservation still returns the stored result without a
     new commitment.
  C. Reservation expiry + stay policy wired to HTTP (commit 01f969d): hold
     expiry derived from authoritative allocation_policy + server time and
     persisted atomically on reservation + stay so the expiry worker releases
     holds. Missing/invalid policy rejected (no invented defaults); out-of-policy
     dates/extensions can't allocate; extension/transfer revalidate context;
     transfer writes a FRESH policy-derived deadline, never the old stay's due
     one. Real-DB tests: deadline stored; worker releases a hold exactly once;
     missing-policy and out-of-policy rejections; failed transfer preserves the
     original stay and capacity.
  D. Voice context scoped to requested jurisdiction (commit cc1e6fb): the
     ContextResolver seam takes a jurisdiction (untrusted lookup input); the
     persisted resolver calls store.ResolveContext (jurisdiction-scoped), never
     ResolveAnyOperationalContext. Missing jurisdiction 400; unknown
     jurisdiction 503; no arbitrary/highest-version substitution; cross-
     jurisdiction known IDs rejected; quarantine invalidates subsequent scoped
     requests. OpenAPI VoiceCommandRequest now requires jurisdiction.
  E. Verification + ledger (this record): regressions fail against the prior
     defects before relying on them; full `go test ./... -count=1` green;
     process tests (TestCrossProcessLastSpace, TestCrashAfterCommitBeforeResponse)
     re-run green at schema revision 5 under -tags crashtest; migration 0005
     exercised on a populated disposable DB (legacy unbound OPERATOR session
     revoked, citizen preserved, revision reaches 5) via
     scripts/p4_mig0005_test.sh.
What the earlier record overstated (corrected): the prior DONE entry described
  operator MFA as verified. It was not: issuance trusted a caller-supplied
  request-body `mfa_verified:true` flag and a caller-chosen jurisdiction. That
  is self-attestation, not verified identity or MFA. The earlier 6/6 operator
  tests demonstrated the middleware, scoping and audit wiring against that
  self-attested boundary — they did NOT demonstrate verified identity. Those
  historical results are preserved above this correction; the boundary they
  exercised has been replaced.
Scope completed and changed files:
  Part A (P3 closure, commit 2b33fa7): real-server crash-after-commit retry
    (build-tag `crashtest` fault injection, never in production), pg_dump/
    pg_restore backup/restore into uniquely-named disposable DBs, main.go DB
    wiring + readiness prober (required-revision check), cross-process
    concurrency for last-space. Ledger A1 honesty correction applied above.
  Part B (citizen stays, commits 7056e4b, 71532f9, 6aefbb5): citizen session
    auth (crypto/rand bearer, SHA-256 hash stored, expiry+revocation); scoped
    place/alias resolution with explicit ambiguity; eligible destination choice
    (route gate false while O05 open; unknown capacity shown unknown, never
    promised); read-only preview separate from explicit reservation with
    source-snapshot revalidation at commit; full stay state machine
    (reserve/arrive/cancel/expire/depart/extend/transfer) with date-range
    capacity conservation (E = free+held+occupied; arrival held->occupied with
    no second decrement; failed transfer retains original); half-open [start,
    end) facility-local dates + deterministic lock ordering; bounded retry-safe
    expiry worker; versioned route validation (operational routing disabled).
  Part C reopen fixes (commit fdfeb04):
    - Trusted operator authentication: request-body MFA flag and requested
      jurisdiction removed. New OperatorVerifier seam (server-verified identity
      + MFA at a trusted boundary); VerifiedIdentity{Subject, MFAVerifiedAt}.
      Jurisdiction and privilege derive from server-controlled operator_grants
      (migration 0004) bound to the verified subject via FirstJurisdiction —
      never from the request. Issuance fails closed (503) with no verifier
      wired; production main.go wires none. Synthetic verifier exists only in
      isolated tests (X-Test-Operator-Subject header), never in the production
      binary. Expiry, revocation, hashed token storage preserved; audit records
      identify the verified operator through the session.
    - Resource-bound idempotency: target ID folded into the operation name
      (source.transition:<id>, source.quarantine:<id>, stay.correction:<id>),
      reusing the existing IdempotencyStore — no second mechanism. Current
      authorization for the target is revalidated BEFORE a replay result is
      disclosed; same-key/different-target yields a distinct resource-scoped
      key, never success for the wrong target. Atomic mutation + idempotency
      result + audit/outbox preserved.
    - Quarantine workflow: terminal sourceact QUARANTINED state distinct from
      SUSPENDED, reachable from all active states. Jurisdiction-scoped operator
      endpoint with explicit reason, version guard, durable idempotency and
      attributed audit/outbox. Quarantined evidence is excluded from the
      resolved operational context and from new reservation commit
      revalidation; existing stays are NOT cancelled nor capacity released.
    - Persisted context resolver: store.ResolveContext (jurisdiction-scoped)
      and ResolveAnyOperationalContext derive data version (package_id:version),
      known IDs and enabled languages from ONE consistent current package row
      whose source is OPERATIONAL, authorized in that jurisdiction, not
      superseded/expired/quarantined. Voice commands validate against the
      persisted snapshot, not client echoes; stale client data_version -> 409.
      Static demo resolver stays explicitly non-operational and separate.
    - Audit-chain race fix: ChainAuditor.Record now serializes appends with a
      transaction-scoped pg_advisory_xact_lock so concurrent writers cannot
      read the same head and write divergent successors (a real global-chain
      break surfaced by the concurrent stay suite, not test flake).
    Files: internal/httpserver/operator.go, operator_handlers.go, server.go,
      cmd/sthira/main.go, internal/store/operatorgrant.go, context.go,
      source.go, audit.go, internal/sourceact/activation.go,
      migrations/0004_p4_operator_grants.sql, contracts/openapi.yaml,
      contract_test.go, operator_http_integration_test.go,
      context_integration_test.go.
Tests/commands, environment and results (PostgreSQL 18.6 + PostGIS 3.6, DSN
  postgres://apple@localhost:5432/sthira_test, schema revision 5):
  - Store suite (stay_integration_test.go): lifecycle conservation, concurrent
    last-space (8 goroutines, exactly 1 wins), expiry/arrival race, expiry
    worker, failed/successful transfer, overlapping intervals, multi-date
    deadlock-free, extend-adds-only-new-dates, cancel-releases-hold.
  - HTTP citizen suite (stay_http_integration_test.go): reserve+read, session
    required (401), cross-session denial (403), duplicate-confirmation replay,
    stale snapshot (409 STALE_VERSION), capacity conflict no-substitution (409),
    arrive/depart conservation, honest unknown capacity with route gate false.
  - HTTP operator suite (operator_http_integration_test.go, rewritten for the
    trusted boundary): issuance fails closed 503 with no verifier (even with a
    body mfa_verified:true); body-MFA rejected (401 no verified subject);
    no-grant 403; MFA-required 403 (nil MFAVerifiedAt); invalid evidence 401;
    jurisdiction derived from grant (201); citizen token denied on operator
    route (403); publish/revoke supersession with idempotent replay;
    cross-jurisdiction source and correction denial; correction audit
    attributed to operator session; idempotency target-binding (same key+body
    on a different source is a distinct key, both apply) and payload-conflict
    409; quarantine + cross-jurisdiction quarantine denial.
  - Persisted-context suite (context_integration_test.go): valid snapshot
    resolves and a valid proposal validates; missing package fails closed;
    stale client data_version 409; quarantine invalidates the resolved context
    (jurisdiction-scoped); unknown ID rejected 422.
  - Contract test: every OpenAPI-documented path routed (405 not 404),
    including the new quarantine path.
  - Full suite `go test ./... -count=1`: all packages ok; store + httpserver
    green at `-count=3` (audit-chain race stable).
  Real bugs found and fixed by real-DB tests: Transfer missing replacement
    reservation insert; uid() nanosecond collision; audit-chain concurrent-head
    race (advisory lock); shared-DB resolver cross-test contamination
    (jurisdiction-scoped resolution).
Unresolved internal work: none for the reopened P4 engineering scope.
External dependency, owner and exact evidence needed:
  - Live operator identity: BLOCKED_EXTERNAL. No real identity provider exists
    in the current dependency set (only pgx). The OperatorVerifier seam is the
    integration point; production wires no verifier, so operator issuance fails
    closed (503) until a trusted IdP/MFA boundary is chosen and integrated.
    Owner: government/platform identity decision. Evidence needed: a verified
    identity + MFA assertion from a trusted authentication boundary. This is
    NOT silently deferred to P11 and live operator authentication is NOT
    claimed complete; the seam and fail-closed default are the delivered
    engineering. Recorded in plan/open-decisions.md.
  - O05 route authority and O07 stay policy remain OPEN; operational routing
    and real stay policy stay disabled. P2 catalogue acceptance remains blocked
    on O01. Live government integration remains disabled (P11).
Next eligible step: P5 — offline package and map-delivery protocol is NOT
  started. P4 stays IN_PROGRESS until the external operator-IdP blocker is
  resolved or explicitly accepted as an operational (not engineering) gap. No
  native frontend, no permanent relocation, no geofencing; .txt preserved.

## P4 acceptance matrix (2026-09-20, second review)

Compact acceptance matrix added per the bounded-corrections task. Each row
is `requirement → implementation path → regression test → actual result`.
Items marked **OPEN** are unfinished internal work; items marked
**EXTERNAL** are blocked on a non-internal decision. Status is the
documented state as of the latest commit, not a claim that the work is
complete.

| # | Requirement | Implementation | Regression test | Result |
|---|------------|----------------|------------------|--------|
| 1a | Route policy authority fail-closed semantics: missing or null route_required rejects with ErrNoStayPolicy; explicit false permits omission; explicit true requires verified route | opkg.AllocationPolicy.RoutePolicyRequired() returns ErrNoStayPolicy on nil; StayPolicy.RoutePolicyRequired() returns ErrNoStayPolicy on nil; RevalidateReservationContext rejects before any capacity change | TestRoutePolicyAbsentFailsClosed; TestRoutePolicyNullFailsClosed; TestRoutePolicyExplicitFalsePermitsOmission; TestRoutePolicyExplicitTrueRequiresRoute; TestRoutePolicyExtendAndTransfer | PASS (absent/null -> 409 VALIDATION_ERROR; explicit true missing route -> 409 ROUTE_UNVERIFIED; explicit false without route -> 201) |
| 1b | Synthetic route boundary isolation: production configuration rejects synthetic demo packages, source artifacts, and routes | RevalidateReservationContext checks pkg/art evidence class and rejects SYNTHETIC_DEMO/SYNTHETIC evidence and synthetic routes unless allowSynthetic is true; guidance query route_gate defaults false | TestNormalServerRejectsSyntheticDemoCommitment; TestNormalServerHeadersAndBodyCannotEnableSynthetic | PASS (commitments rejected with ErrReservationContext/ErrRouteUnavailable; request headers/body cannot enable synthetic execution) |
| 1c | Isolated synthetic HTTP exercise completes intended flow via server configuration | Server option WithSyntheticExercise(StaticSyntheticExercise(true)) enables synthetic execution exclusively on configured test servers | TestIsolatedTestConfiguredServerCompletesSyntheticReservation; TestIsolatedTestConfiguredServerRejectsRequiredRouteOmission; TestExtensionAndTransferSyntheticBoundaryIsolation | PASS (201 Created under test server; route omission when required still rejects 409) |
| 1d | A valid route must match the selected destination | stay.RevalidateReservationContext joins `route_versions.to_safe_zone_id = facilities.safe_zone_id` | TestReservationRouteToWrongDestinationRejected; TestTransferWithOldDestinationRouteRejected | PASS (both rejected with 409; no writes) |
| 1e | Closed/unavailable destinations and routes cannot create new commitments | RevalidateReservationContext reads zone_versions.status (OPEN/PUBLISHED allowed; CLOSED/FULL/etc rejected); route closure row checked | TestReservationClosedRouteRejected; TestUnavailableDestinationRejected | PASS (409; no writes) |
| 2  | Transfer persists the validated replacement route | StayStore.Transfer takes `newRouteID *string`; the caller (handleStayEvent) passes `req.NewRouteID`; the replacement stay is persisted with `newRouteID`, never `st.RouteID` | TestTransferReturnsReplacementIDsAndRead (reads `stays.route_id` for the replacement); TestTransferRetryReturnsSameReplacementIDs | PASS |
| 3  | Initial reservation snapshot version revalidated under locked snapshot; mandatory where required | handleCreateReservation calls RevalidateReservationContext first to acquire FOR UPDATE locks on s, p; calls revalidateSnapshotVersion under same locks; rejects snapshot_version <= 0 at boundary (400) and in tx | TestSnapshotRaceRejectsStaleVersion (goroutine bumps version under lock; reservation blocks and rejects 409 STALE_VERSION with zero writes; v2 succeeds 201; replay 200); TestReservationRejectsZeroSnapshotVersion; TestExtendRejectsZeroSnapshotVersion | PASS |
| 4a | Concurrency evidence with targeted DB lock-wait observation | concurrency_helpers_test.go: backendPID captures connection PID on tx; pollUntilBlocked verifies waiter_pid in pg_stat_activity has wait_event_type='Lock' and blocker_pid = ANY(pg_blocking_pids(waiter_pid)) | TestSourceLockBlocksWithdrawal; TestConcurrentBarrierCommitmentWins; TestConcurrentBarrierWithdrawalWins | PASS |
| 4b | Both orderings (commit-wins and withdrawal-wins) exercised deterministically | Two barrier protocols capture competing PIDs; observer proves intended waiter is blocked specifically by competing blocker connection before holder releases | TestConcurrentBarrierCommitmentWins (quarPID blocked by commitPID); TestConcurrentBarrierWithdrawalWins (commitPID blocked by quarPID) | PASS |
| 4c | Negative lock contention and missing contention proof | TestObserverRejectsUnrelatedContention creates active contention on unrelated connections (pg_advisory_xact_lock) and proves observer rejects it; TestObserverTimesOutWhenContentionMissing proves missing contention produces bounded timeout | TestObserverRejectsUnrelatedContention; TestObserverTimesOutWhenContentionMissing | PASS |
| 4d | Worker goroutine errors propagate to parent; no t.Fatalf inside goroutines | runRacing helper collects errors on a channel; t.Fatalf only on the main goroutine | All Test*Barrier* / TestSourceLock* / TestObserver* | PASS |
| 5a | Audit replay through real HTTP boundary (no error suppression) | audit_integration_test.go (httpserver package): real doAuthed over httptest.Server | TestRetrySameKeyNoDuplicateAuditOrCapacityReplay | PASS (same payload, 1 event, capacity unchanged, VerifyChain ok) |
| 5b | Audit chain via VerifyChain (global ordering) | store.VerifyChain reads all events ordered by event_seq, recomputes prev/event hashes | TestChainInterleavingAcrossSubjects (interleaved A1,B1,A2,B2 across two stays; VerifyChain passes) | PASS |
| 5c | Legitimate interleaving from different subjects | Two stays on separate facilities; event_seqs cross between them | TestChainInterleavingAcrossSubjects | PASS |
| 5d | Audit identity scope: same key by distinct operators does not collide | auditEventID now includes actor (session_id); two distinct operator sessions with same key produce distinct EventIDs | TestTwoCorrectionsDistinctOperatorSessions | PASS |
| 6a | Test isolation: per-test unique jurisdictions for context resolver tests | context_integration_test.go uses uniqueJTEST(); persisted resolver is jurisdiction-scoped (D36) | TestPersistedContextResolves; TestPersistedContextFailsClosedNoPackage; TestPersistedContextStaleClientVersion; TestPersistedContextUnknownIDRejected | PASS |
| 6b | Test isolation: TestExpiryWorker robust against shared-DB pollution | Tick() returns a count over the WHOLE DB; the assertion is on THIS test's stay state (was: `n != 1`) | TestExpiryWorker | PASS |
| 7  | All tests pass on the first run; no rerun required | `go test ./... -count=1` and `-count=3` both clean | full-suite run | PASS |

### Process test execution evidence

Process tests `TestCrossProcessLastSpace` and `TestCrashAfterCommitBeforeResponse` require build tag `-tags crashtest` and `STHIRA_RUN_PROCESS_TESTS=1` alongside `STHIRA_TEST_DSN`.
- **State 1 (Without `-tags crashtest`)**: tests are uncompiled (`testing: warning: no tests to run`).
- **State 2 (With `-tags crashtest`, without `STHIRA_RUN_PROCESS_TESTS=1`)**: tests call `t.Skip` (reported as package PASS, but 2 skipped: `concurrency_process_test.go: requires STHIRA_TEST_DSN and STHIRA_RUN_PROCESS_TESTS=1`, `crash_process_test.go: requires STHIRA_TEST_DSN and STHIRA_RUN_PROCESS_TESTS=1`).
- **State 3 (With `-tags crashtest` AND `STHIRA_RUN_PROCESS_TESTS=1`)**: both tests execute and pass:
  - `TestCrossProcessLastSpace`: 6 child processes racing for last space, conservation held (`reserved=1 capacity=1`), exactly 1 wins, audit event recorded atomically.
  - `TestCrashAfterCommitBeforeResponse`: child server 1 commits reservation and receives SIGKILL mid-response; child server 2 restarts, retries same idempotency key and payload; returns stored result with 0 duplicate reservations, 0 capacity double-decrements, and 0 duplicate audit events.

### Overstated claims removed from prior records

- **no-sleeps concurrency**: the previous "no sleeps" wording was inaccurate;
  TestConcurrentWithdrawalAndCommitmentBarrier used `time.Sleep(10ms)` /
  `time.Sleep(5ms)` to establish ordering. Replaced with channel-only
  ordering + pg_stat_activity lock-wait observation (observing the specific
  waiter PID blocked by the specific blocker PID).
- **HTTP synthetic isolation**: the previous claim that the synthetic gate
  was reachable only via the store API is now strengthened by
  server-side configuration dependency injection (`WithSyntheticExercise`),
  ensuring request headers or body fields cannot enable synthetic execution.
- **route policy fail-closed semantics**: missing or null `route_required`
  fails closed with `ErrNoStayPolicy` (HTTP 409 `VALIDATION_ERROR`). Explicit
  `false` remains distinct and permits omission; explicit `true` requires a verified route.
- **snapshot ordering**: the initial reservation version check runs under
  the locked package snapshot acquired by `RevalidateReservationContext`,
  eliminating the gap where concurrent version bumps committed between
  check and lock.
- **P4 closure**: not claimed; P4 remains IN_PROGRESS until external
  blockers are resolved.

### Internal work left for P4 closure

None for the engineering scope of these bounded corrections. Each
acceptance row above is exercised by a real-DB Go test against a disposable
test instance (PostgreSQL 18 + PostGIS 3.6, schema revision 5). The
operational activation blockers (O01, O05, O07, O14) are external/operational
dependencies, not unfinished internal engineering.

### External blockers (explicit)

- **O01** — approved demo state/district list, official references and user-supplied zones/routes remain OPEN; catalogue acceptance blocked.
- **O05** — route authority OPEN; operational routing stays disabled. The synthetic route gate is reachable only via server-configured isolated test execution (`WithSyntheticExercise`), never via request headers or body fields; production configuration rejects synthetic evidence.
- **O07** — stay policy OPEN; authoritative policy must explicitly supply route_required (absent/null fails closed). Real policy is still authority-supplied.
- **O14** — operator IdP not selected; production issuance remains fail-closed 503. The OperatorVerifier seam is complete, tested, and fails closed in production; live operator IdP integration is BLOCKED_EXTERNAL.

## Required completion record

```text
Phase / status:
Starting and checked revision:
Scope completed and changed files:
Tests/commands, environment and results:
Source/model/data/build versions used:
Commit(s):
Unresolved internal work:
External dependency, owner and exact evidence needed:
Next eligible step:
```

Use DONE only when all exit conditions pass. “Software ready for government integration” requires gate S (P0–P10), including both mobile platforms and selected regional languages. “Operational” additionally requires P11/P12. No budget exhaustion, documentation completion or synthetic demo can close those gates.

# Implementation prompts

## P0 — Reconcile scope and freeze migration evidence

Prerequisites: None. Status: DONE 2026-09-19 at `ce6adca`; evidence in "P0 baseline evidence" above.

### Read and establish

Read plan/plan.md, plan/prd.md, plan/rules.md, plan/cleanup.md, CLAUDE.md, GEMINI.md, tasks/lessons.md and the current git status/staged diff. Read the actual entry point, middleware, test bootstrap, Makefile, Dockerfile, Compose, pyproject and frontend package manifest. Treat the inventory as a dated snapshot and verify it before deleting or porting anything.

### Execute

1. Confirm this phase is now authorized for implementation work; the 2026-09-19 task itself was planning-only. Preserve unrelated edits and staged moves. Record the starting commit and a compact path/status inventory.
2. Reconcile root agent instructions and tasks/todo.md with this plan: Go product backend, both mobile platforms, constrained middle model, immediate/temporary scope and open route authority. Remove obsolete active hackathon TODOs, retaining concise historical evidence links. Do not modify .txt files.
3. Trace src/sthira/api/app.py → middleware → sthira_v2 router → fixtures/local voice and tests/conftest.py. List actual /api/v2 routes, request/response/error/auth/cache behavior and known reference-only behavior. Do not treat defects or unsafe demo defaults as contracts to preserve.
4. Mark each legacy module PORT/RETIRE/KEEP with caller/build/test evidence; identify CAP/parser duplication and frontend-owned scenario data. Note any actual non-demo stored data and external clients before proposing migration/deletion.
5. Freeze representative synthetic fixtures and expected invariants: provenance, cancel/update, capacity conservation, duplicate retry, stale cache and forbidden voice action. Existing code is evidence, not automatic target truth.
6. Record selected test/tool versions and missing hardware/data/policy decisions. Update only durable evidence in the existing plan/tracker; no new generic project-management framework.

### Verify

Run existing relevant baseline checks once, without making paid model calls or changing data: inspect Makefile targets first; use local pytest/current frontend build where dependencies are available. Record failures and environmental limits honestly. Verify the route/import/dependency inventory, .txt preservation and final diff. A failed old baseline is not silently repaired under cleanup scope.

### Deliver and stop

Deliver a reconciled instruction/tracker baseline, actual API/invariant inventory and refined cleanup manifest. No legacy files deleted and no Go scaffolding required. Baseline limitations have owners and do not become claimed passes.

## P1 — Go foundation and executable contracts

Prerequisites: P0. Initial status: NOT_STARTED.

### Read and establish

Read src/sthira_v2/contracts.py, config.py, app.py, security.py; tests/test_v2_contracts.py, test_v2_openapi.py, test_v2_phase0.py; plan/trd.md and plan/voice-map-system-prompt.md. Inspect existing CI/build conventions. These are references; proposed /api/v3 paths do not exist yet.

### Execute

1. Pin a maintained Go toolchain after checking current support/security. Create backend/go.mod and backend/cmd/sthira/main.go as the proposed isolated location, unless P0 establishes a better existing convention. Create only packages used by this slice; no empty layer scaffolding or generic repository framework.
2. Use net/http, context deadlines, bounded readers and structured safe logs. Define method/path behavior, Content-Type, size/depth limits, cancellation, timeouts and graceful shutdown. Separate liveness from dependency/source readiness; missing infrastructure must not report READY.
3. Freeze /api/v3 schemas in a versioned OpenAPI/JSON contract under backend/contracts/. Include provenance/evidence classes, unknown values, IDs, UTC timestamps, coordinate order, facility/stay/route policy fields, error envelope and session authorization boundary. Record the deliberate v2 compatibility strategy.
4. Implement strict input/output validation and fixture-driven handlers for the minimal health/version/contract slice. Standard JSON decoding alone does not reject duplicate keys; cover the chosen strict-boundary behavior. Do not fabricate database or source success.
5. Define the middle-model schema/tagged actions and golden examples from voice-map-system-prompt.md, independent of a model provider. Validate status/action combinations and known IDs in a test context. No LLM/network is required here.
6. Add the smallest CI/build/check targets for Go alongside existing reference checks. Existing Python/web code remains runnable; do not delete it or port unrelated legacy endpoints.

### Verify

Before relying on new behavior, create failing tests for malformed/oversized/trailing/duplicate-key JSON, unknown enums/fields, nil/unknown versus zero, invalid time/coordinates, method/content-type errors, unsupported model action and false readiness. Run gofmt, go vet ./... and go test ./... from backend; race checks where shared state exists. Exercise the real HTTP server and generated contract, including failure status codes.

### Deliver and stop

Deliver a bootable bounded Go boundary, executable schemas/golden fixtures, exact toolchain/build instructions and CI evidence. No fake source/database readiness, no unrequested domain scaffolding and no frontend implementation. Commit the verified slice.

## P2 — Government-data contracts and scenario ingestion

Prerequisites: P1. Initial status: NOT_STARTED.

### Read and establish

Read src/sthira_v2/cap.py, xml_parser.py, operational_package.py, package_service.py, source_activation.py, source_health.py and fixtures; associated CAP/package/source tests. Read plan/source-register.md and plan/feature.md. No source access or demo geometry may be invented.

### Execute

1. Implement Go parsers/validators and source-state rules against recorded synthetic transport fixtures. Support raw CAP preservation, sender+identifier references, effective/expiry windows, Public/Exercise/Test distinction, update/cancel, out-of-order references, 200/304 and bounded safe retry/backoff.
2. Reject DTD/entities, malformed/oversized data, unsupported coordinate systems, invalid geometry/times and foreign-jurisdiction references. Resolve CAP latitude/longitude versus GeoJSON longitude/latitude deliberately. Quarantine rejected inputs with restricted evidence and safe reason codes.
3. Define incident package, route approval/candidate status, facility/stay policy and version/signature boundaries. A checksum is not publisher authentication. Keep live adapters disabled behind activation evidence; make HTTP transport injectable for fixtures without inventing official endpoints/quotas.
4. Establish one canonical scenario catalogue outside frontend source ownership. Keep historical event evidence separate from synthetic operational geometry and exercise clock. Do not hard-code a new current date to make expired historical fixtures appear live.
5. Prepare the proposed state/language manifest and per-case schema. Ingest user-supplied zones only when received and validated; preserve original provenance and reuse permission. Research/verify actual historical cases against official reports before counting them. Missing zones/reports remain an explicit data dependency.
6. Expose only test/preview validation at this stage; durable publication and source ownership enforcement complete in P3. Do not build optional unrelated hazard connectors.

### Verify

Run CAP lifecycle/HTTP fixture tests (including duplicate-conflicting, update-before-target, cancel and outages), CRS/ring/coordinate/schema tests, wrong signer/jurisdiction tests and cross-reference/expired package rejection. Fuzz parsers with bounded time/resources. Validate scenario manifest counts and labels; do not count placeholders as complete cases.

### Deliver and stop

Parser/activation/preview infrastructure passes deterministic tests. Catalogue completion is separately recorded against O01; missing user data does not prevent independent schema work but prevents a full P2 scenario acceptance claim. No live connector is enabled.

## P3 — Durable storage, authorization and ledger foundation

Prerequisites: P1 + P2 contract slice. Initial status: NOT_STARTED.

### Read and establish

Read src/sthira_v2/persistence/, migrations/, audit.py, allocation.py, security.py, source_activation.py and relevant persistence/allocation/security tests. Inspect whether any current database contains real data. Read plan/equations.md and the authorization fields in trd.md.

### Execute

1. Use PostgreSQL/PostGIS with a pinned compatible version and SQL migrations. Prefer explicit SQL/pgx and concrete transaction functions. Do not equate SQLite tests with PostGIS/locking proof.
2. Implement sources/artifacts, versioned facts/packages, sessions, reservations/stays, facility/date inventory, durable idempotency and audit/outbox storage. Enforce uniqueness, foreign keys, valid ranges, scope and nonnegative balances in the responsible layer.
3. Define transaction boundaries for publish/version supersession and capacity reserve/arrival/release. Persist result, event and outbox atomically. Handle no-row/first-insert contention, not only locking an already existing row.
4. Bind private reads/writes to the authenticated session or jurisdiction-scoped operator. Establish operator authentication integration boundary, least-privileged DB roles and redacted error/log behavior. Demo IDs/headers are not production authentication.
5. Move applicable Go endpoints onto real repositories. No process-memory authoritative ledger and no fallback to empty successful storage after database failure. Readiness actively probes migrations/DB/storage dependencies without leaking secrets.
6. Preserve any existing real data with a documented tested export/migrate/rollback approach; if only synthetic data exists, record evidence and provision isolated test databases. Do not run destructive reset against unknown data.

### Verify

Apply migrations to a fresh real PostgreSQL/PostGIS instance and test upgrade/rollback recovery strategy. Race multiple server processes for the last spaces, initial audit/source rows and duplicate idempotency keys. Kill a request after commit/before response, restart and retry; verify no lost/double events. Test cross-session/jurisdiction access, wrong SRID, backup restore and unavailable-DB readiness. Run Go race detector in addition to DB tests.

### Deliver and stop

Go API uses durable storage and verified authorization; conservation, restart/retry and version invariants pass on real DB. Missing DB/hardware yields BLOCKED_EXTERNAL with exact evidence needed, never a substitute green in-memory result.

## P4 — Destination choice and immediate/temporary stays

Prerequisites: P2 + P3. Initial status: NOT_STARTED.

### Read and establish

Read prd.md, architecture.md route section, equations.md and O05/O07. Trace reference allocation/contracts/package code and tests. Route authority remains open; synthetic policy fixtures are not operational policy.

### Execute

1. Implement jurisdiction-aware place resolution from known aliases/admin IDs with ambiguity results. No LLM-generated geocodes or mandatory multi-field location wizard.
2. Return only eligible destinations under supplied facility, party, stay, capacity and route policy. Support emergency assembly versus overnight and temporary accommodation. Display unknown attributes honestly. Rank by supplied route length only if permitted; do not rank by guessed safety.
3. Separate read-only choice/preview from explicit reservation. Bind chosen facility/route/source version and party size to the confirmation; revalidate at commit. Concurrent capacity changes return a recoverable conflict instead of silently picking a different facility.
4. Implement reserve, explicit arrival, no-show expiry, cancellation, departure, authorized correction, temporary-stay extension and transfer. Maintain date-range capacity where needed. Arrival converts a hold to occupancy; transfer preserves old stay until the new transition is safely confirmed. Handle expiry/arrival races and invalid transitions.
5. Add route import/validation/display contracts for geometry, origin/destination, mode, verification/closures and landmarks. Operational routing remains disabled until O05 closes. No straight-line navigation, auto-approved local shortcut or terrain-derived safety.
6. Provide minimal scoped operator APIs for publish/revoke/correction/quarantine and auditable assistance. Voice can request previews/confirmation screens only. Do not add permanent relocation, scheme eligibility or background geofencing.

### Verify

Exercise API/domain cases for choice, full/closed/unknown capacity, missing policy/route, same-name place, accessibility/transport mismatch, stale selection, duplicate confirmation, multi-day overlap, extension beyond scope, transfer failure and operator scope denial. Assert capacity conservation after every event and restart. Verify red-zone exits are not governed by naive polygon intersection alone.

### Deliver and stop

End-to-end synthetic choice→reservation→arrival→temporary stay→departure/transfer works through real Go+DB boundaries, with no unauthorized live routing. Every consequential action requires explicit confirmation/authorization and durable idempotency.

## P5 — Offline package and map-delivery protocol

Prerequisites: P2 + P3. Initial status: IN_PROGRESS (reopened at 046183c for corrections A–G).
Shared implementation contract: [plan/p5-contract.md](p5-contract.md).

### Read and establish

Read src/sthira_v2/offline.py, frontend/v2/public/sw.js, current scenario-cache behavior and offline/package tests. Read architecture.md, parameters.md and O06. No native frontend implementation yet.

### Execute

1. Publish minimal public incident cards and versioned manifests separate from private session data. Include provenance, expiry, revision, language resources, cancellations and signed digests. Define key trust/rotation/revocation and monotonic-version handling.
2. Implement conditional requests, immutable object/cache policy, byte/duration limits, atomic verified download replacement and resumable transfer. A failed refresh cannot destroy valid local evidence; an old manifest cannot resurrect a revoked route.
3. Specify downloadable regional vector maps, style/glyph/sprite completeness, gazetteer and approved audio. Use legal sources with explicit offline/redistribution rights; do not bulk-download public OSM tiles or assume Google caching rights.
4. Build a protocol test client/fixture harness, not the deferred production UI. Verify package budgets and untrusted-clock behavior, independent map/data expiries and cold/empty storage states.
5. Define pending offline writes: durable local request reference, unchanged idempotency key, renewed server validation and explicit handling when selection/hold expires. Pending does not mean reserved.
6. Add manifest-based update and revocation propagation with bounded polling/jitter. Choose notification transport later; current push delivery cannot be guaranteed. Avoid custom delta/pack protocols if HTTP/standard signed manifests are adequate.

### Verify

Simulate poor network, truncation/resume, corrupt signature/digest, wrong key, revoked/expired/older package, clock rollback, cache eviction and unavailable map/audio. Measure compressed critical-card bytes and transfer budget. Prove public caching excludes private assignment data and government requests do not scale with users.

### Deliver and stop

Documented tested client protocol and package delivery meet declared byte/security/freshness contracts. Map data licensing/pack-format choices stay blocked where unverified (O06); no claim that a native offline client exists yet.

**P5 Implementation Record & Acceptance Reopening (2026-09-20):**
- Initial integration of five implementation branches completed at commit `046183c` (migration 0006, offline store methods, public delivery routes, protocol client, pending write queue).
- Engineering acceptance reopened to address seven specific gap areas (A through G).

#### P5 Compact Acceptance Matrix (Corrections A–G)

| Area | Target Subsystem | Reproduction Test Name | Fail-Before Symptom | Pass-After Invariant | Honest Closure Evidence | Status |
|---|---|---|---|---|---|---|
| **A** | `offlineclient`, `offlinepkg` | `TestCardReferenceBindingMismatch`, `TestCardDeclaredSizeExceeded`, `TestSync_StorageJurisdictionBound`, `TestManifestUncompressedBudget` | Card accepted with mismatched checksum, wrong package ID/version/jurisdiction, or exceeding declared bytes. | Card download rejects identity/checksum/size mismatches; state is jurisdiction-bound; strict parsing rejects ambiguous JSON. | Focused and full Go suite at `6843c04`. | VERIFIED |
| **B** | `offlineclient` | `TestAlreadyExpiredCardReportedExpired`, `TestClockRollbackCannotUnexpire`, `TestRestartRecovery`, `TestKeyRevocationInvalidatesCard` | Already-expired card returned `CURRENT`; elapsed freshness gave new full window from sync; revoked key left card `CURRENT`; rollback un-expired. | Absolute expiry wins; live process uses monotonic elapsed time; restart without it is `UNVERIFIABLE`; revoked key is `REVOKED`. | Focused and full Go suite at `b6a7b84`/`6843c04`. | VERIFIED |
| **C** | `offlineclient` | `TestAtomicActivationInterrupted`, `TestCorruptTombstonesFailClosed`, `TestSameRevisionRepair` | Interrupted activation committed manifest without card; corrupt `tombstones.json` silently reported route un-cancelled; missing active files ignored on same revision. | Generation staging commits manifest + card together; corrupt tombstones fail closed; same revision repairs and public reads revalidate binding. | Full Go suite at `6843c04`. | VERIFIED |
| **D** | `offlineclient` | `TestResumableTransferRealConnectionDrop`, `TestResumableTransferETagDriftFallback`, `TestResumableTransfer416Reset` | Resumable transfer buffered entire body in memory and only wrote `.part` at end; connection drop lost all progress; `Content-Range` and `416` not validated. | Bounded streaming persists client-created partial state; range start/total, ETag drift, 416, and corrupt metadata are handled without combining representations. | Full Go suite at `6843c04`; real HTTP P5 flow passed. | VERIFIED |
| **E** | `store`, `offlinedelivery` | `TestPublicationImmutabilityConflict`, `TestQuarantineInvalidatesCache`, `TestPublicationBoundaryValidation` | `PublishManifest`/`PublishCard` overwrote rows on conflict with differing bytes; quarantined package remained in `CachedSource`. | Identical payload re-publish is idempotent; conflicting payload returns `ErrConflict`; quarantine/withdrawal invalidates cache immediately; boundary validates schema/digests. | Disposable-DB publication test and full Go suite. | VERIFIED |
| **F** | `offlinequeue` | `TestPendingReconciliationOnLostResponse`, `TestSelectionExpiryPreservesReconciliation` | Lost response marked `FAILED_PERM` or stayed in flight; selection expiry while offline marked hold expired without checking server commitment. | Dropped response becomes `PENDING_RECONCILIATION`; expiry does not discard an uncertain entry; same key reconciles server commitment. | Real-server P5 Flow 5 and full Go suite. | VERIFIED |
| **G** | `offlineresources`, `offlineclient`, docs | `TestResourceValidatorIntegration_DescriptorAudit`, `TestResourceValidatorIntegration_MissingOptionalLeavesCardValid`, `TestResourceFreshnessIndependent` | `offlineclient` did not invoke `offlineresources.Validator`; documentation overstated encryption at rest and test key authority. | Validator runs before activation; optional assets preserve card validity; stored resources have independent freshness and O06-pending license status. Docs state plaintext queue storage, synthetic keys, illustrative budgets, and simulation limits. | Focused and full Go suite; D55. | VERIFIED |

- External Blockers Retained: O06 (official map licenses) and O05 (operational routes) remain open and fail-closed. P5 engineering acceptance reopened for corrections A–G; stop before P6.

**P5 engineering acceptance record (2026-09-21):** Status: **DONE** for internal engineering scope. Reviewed correction commits: `0ccbaa7`, `759ac7e`, `1b4b6cb`, `dd2edbd`, `0a3bb5e`, `b6a7b84`, `6843c04`. All 11 P5 acceptance tests pass on disposable PostgreSQL instance (`TestP5Flow1` through `TestP5Flow5`, `TestP5Verification_PublicPrivateSeparation`, `TestP5Verification_SignatureTrustAndMonotonicVersion`, `TestP5Verification_ClockRollbackDefense`, `TestP5Verification_ByteBudgets`, `TestP5Verification_ShieldCacheUpstreamBound`, `TestP5Verification_NetworkSimulationProfile`). External blockers O05 (operational route authority) and O06 (official map licenses) remain open and fail-closed.

## P6 — Regional ASR, constrained middle model and TTS

Prerequisites: P1 + P4 + P5. Status: **IN_PROGRESS** (internal architecture integrated; real model inference BLOCKED_EXTERNAL).

### P6 integration and evaluation record (2026-09-21):

- **Integration**: Integrated Worker 4 (`ScopedContext` resolver and semantic validator), Worker 5 (`asrworker`), Worker 6 (`middleworker`), Worker 7 (`ttsworker`), Worker 8 (`eval` harness), and Worker 9 (`orchestration`).
- **Real HTTP Endpoints**: Wired `/api/v3/voice/transcriptions`, `/api/v3/voice/process`, `/api/v3/voice/speech` onto server mux; verified OpenAPI contract compliance (`TestServedRoutesMatchOpenAPI`).
- **Verification**: `backend/internal/httpserver/voice_integration_test.go` exercises end-to-end pipeline over real Go HTTP + persisted PostgreSQL scoped context:
  1. `TestVoiceProcess_RealHTTP_PersistedScopedContext_Pipeline`: PASS (audio -> ASR -> middle -> validation -> template -> TTS).
  2. `TestVoiceProcess_RealHTTP_StaleSnapshotDuringInference`: PASS (mid-inference package supersession returns HTTP 409 Conflict).
  3. `TestVoiceProcess_RealHTTP_CancellationDuringInference`: PASS (request cancellation aborts downstream inference).
  4. `TestVoiceProcess_RealHTTP_QueueSaturation`: PASS (queue saturation immediately returns HTTP 503 QUEUE_SATURATED).
  5. `TestVoiceProcess_RealHTTP_DirectTextFallback_ModelDown`: PASS (proves citizen text/touch navigation via `/places/resolve` and `/guidance/query` remains 100% operational when voice model workers are unconfigured/offline).
  - All tests assert zero consequential writes in `stays`, `reservations`, and `audit_events`.
- **Multilingual Evaluation**: `backend/eval/cmd/eval-run` executed against synthetic suite: 20 pass / 0 fail across hi-IN, ml-IN, and en-IN using the deterministic provider.
- **External Blockers**: Real model inference on Qwen3-4B-Instruct-2507, IndicConformer, and Indic Parler-TTS is honestly reported as `BLOCKED` / `NOT_EVALUATED` due to open O03 (language matrix / reviewers), O11 (approved translations), and lack of target GPU infrastructure. Model candidates remain proposed, not production defaults.

## P7 — Backend security, performance and handoff gate B

Prerequisites: P0–P6 acceptance evidence. Status: **IN_PROGRESS** (Gate B remains **NOT_READY**).

### P7 preparation and rehearsal record (2026-09-21):

- **Security & SBOM**:
  - `gofmt -l .` verified clean (0 files unformatted).
  - `go vet ./...` clean across `backend/`, `backend/eval/`, `loadmodel/`.
  - `build-sbom.sh` generated Go module graph lock (`go-mod-graph.txt.sha256`).
  - Native Go fuzzing (`run-fuzz.sh`) ran clean for 5s per target across `internal/capfeed`, `internal/httpjson`, `internal/offlinepkg`.
- **Recovery Rehearsals**:
  - `run-graceful-drain.sh`: PASS (process drained in 18ms under SIGTERM, new requests refused).
  - `run-dep-outage.sh`: PASS (`/health/ready` degraded to 503 within 14ms upon DB stop; `/health/live` remained 200; restored to 200 within 19ms on restart).
  - `run-backup-restore.sh`: PASS (schema 6, audit head verified, capacity conserved, package count and outbox verified).
- **Representative Load**:
  - `loadmodel/loadfixtures` benchmarks executed cleanly on Apple M2 (`BenchmarkHealthLive`, `BenchmarkCachedReadManifest`, `BenchmarkGuidanceQuery`, `BenchmarkReservationWrite`, `BenchmarkVoiceTranscriptionsNoWait`, `BenchmarkQueueSaturation`).
  - `loadmodel/voice/voiceload` unit tests pass with race detector.
- **Crashtest Process Verification**:
  - `TestCrossProcessLastSpace`: PASS (6 independent child processes race on single capacity unit; exactly one wins; atomic single-tx commit).
  - `TestCrashAfterCommitBeforeResponse`: PASS (server crash after commit before response; replay succeeds with zero state corruption).
- **Handoff Gate B Assessment**: **NOT_READY**. Gate B cannot be declared until P6 real model evaluation passes on authorized GPU hardware and external dependencies O03, O05, O06, O07, O11, O14 receive official authority sign-off. P8 frontend implementation remains gated on Gate B.
- **Stage 6–7 evidence (single-worker pass, 2026-09-21)**:
  - **Stage 6 (inference adapters)**: ASR/TTS subprocess bridges implemented (Go side `runtime_adapter.go`; Python side `speech_asr_adapter.py` / `speech_tts_adapter.py`). Both adapters report BLOCKED state when the artifact gate (O03 for ASR, O11 for TTS) is not ready. Tests assert the BLOCKED path on the real Python adapter via `STHIRA_PYTHONPATH`. Compressed-audio path: `audio_compressed.go` decodes Ogg/Opus and WebM/Opus via a bounded ffmpeg subprocess with deadline + bounded stdout/stderr, then falls through to `DecodeWAV`. Worker server now calls `DecodeAudio` (the dispatch entry point). Sarvam-30B FP8 vLLM config recorded (`sarvam_config.go`); tests assert `--quantization fp8` (NOT bf16/fp16), loopback host, guided decoding, and `enable_thinking=false` chat template.
  - **Stage 7 (evaluation, security, performance, recovery)**:
    - Real HTTPProvider tests against a httptest stub server verify ASR forwards request_id/language/content_type, Middle preserves wire shape (request_id + case_id), TTS forwards jurisdiction and source_version, 503 maps to typed "model unavailable" error, empty/malformed audio is rejected.
    - SPDX 2.3 SBOMs generated for every Go module via `scripts/gen_spdx.py` (real documents, not graph-hash placeholders). `scripts/run_security_checks.sh` runs `go vet`, `gofmt -l` and any installed optional scanner (govulncheck/gosec/staticcheck/gitleaks/trivy); missing binaries surface as NOT_RUN, not silent success.
    - Bounded k6 smoke `loadmodel/k6/smoke_real.js` runs against the ACTUAL sthira Go binary on a uniquely-owned migrated PostgreSQL via `scripts/run_k6_smoke_real.sh`. Latest run: 9447 iterations, 9447/9447 checks pass, http_req_duration p95=1.29ms. NOT a capacity benchmark — that needs controlled hardware.
    - Graceful shutdown test `TestGracefulShutdown_AllowsInflightToComplete` proves a controlled in-flight request completes during shutdown (not just that the process exits). Reuse of recovery infrastructure for backup+restore rehearsal: `TestBackupRestoreAndReconnect` runs against schema revision 7 and verifies audit head, idempotency replay, capacity conservation across dump+restore.

## P8 — Select and implement Android and iPhone clients

Prerequisites: Gate B. Initial status: NOT_STARTED.

### Read and establish

Read prd.md, architecture.md offline/voice flow, tech-stack.md candidates, trd.md frozen APIs and device/language budgets. Inspect the old web client only for reusable experience/fixture evidence; do not port its layout blindly.

### Execute

1. Resolve minimum OS/test devices, supported state/language matrix and distribution constraints. Prototype the narrow map download/offline boot/audio/screen-reader slice on Android and iPhone before choosing native Kotlin+Swift, Kotlin Multiplatform/platform UI or a justified alternative.
2. Record measured download size, memory, cold startup, battery/thermal behavior, map offline compatibility, maintenance cost and accessibility. Kotlin by itself does not satisfy iPhone delivery; no silent platform omission.
3. Build the local UI/assets/storage using the selected solution. A large speak control, labelled accessible corner chat button, clear map/card and one next action are the baseline; no heavy animations, mandatory registration or form-heavy location wizard.
4. Connect voice/chat to the same validated API. Implement ambiguity choices, eligible destination preview/selection, explicit reservation/arrival/stay events, route/non-map parity and OS dialler confirmation. Distinguish candidate/unknown/expired/informational states visually and accessibly.
5. Implement verified downloadable maps/gazetteer/audio/packages, safe refresh, revocation, cache limits, permission denial and queued retry semantics. Process location locally when practical; never include government keys or model weights in the app.
6. Add the necessary scoped operator interface/workflow for publish/revoke/source health and corrections, reusing an appropriate small client surface; no unrelated administration suite. Handle mobile audio interruptions, notification permission and background limits, clear privacy disclosure and signed app updates.

### Verify

Exercise actual Android and iPhone builds on physical target devices: voice and chat, disabled microphone/location, screen reader/large text, cold offline boot, expired/empty pack, poor network, low storage/memory, source/model/map outage, audio interruption and confirmed capacity/call paths. Verify model actions are revalidated against the client snapshot. Native UI requires more than a browser screenshot.

### Deliver and stop

Both supported platforms and operator workflows meet behavior/accessibility/offline budgets with real evidence. If one platform or state language fails, launch scope is not complete. No government operational activation is implied.

## P9 — Whole-system readiness, regional drills and release assurance

Prerequisites: P8; full P2/P6 data/language acceptance. Initial status: NOT_STARTED.

### Read and establish

Read all unresolved O-items and previous evidence. Use real selected Android/iPhone devices, actual Go/database/model deployment, approved scenario catalogue and the assurance matrix; no mock replacing the boundary under test.

### Execute

1. Complete 2–3 sourced historical exercises for each selected state using user-supplied validated demo zones and clearly synthetic operational data. Exercise immediate and 7–30 day temporary stays, accessible alternatives and closure/transfer cases.
2. Run observed task tests with low-literacy, older-adult and disability cohorts and appropriately supervised child interactions. Validate regional instructions/ASR/TTS with qualified speakers; document failures rather than averaging them away.
3. Run constrained-network, offline expiry/revocation, model/map/database/source outage, same-name place, closure and capacity drills end to end on both devices. Test safe fallback and pending-request reconciliation after lost responses.
4. Repeat relevant security testing after frontend/API integration, including mobile storage, deep links, session token leakage, TLS, app packages and update signing. Reproduce Strix findings in deterministic tests; remove debug/operator leakage.
5. Run representative long-lived and burst load with real inference and public delivery. Test node failure, backup/PITR restore, rolling release/rollback and audit reconstruction. Record cost and operations ownership/on-call escalation.
6. Prepare reviewed release packages, signed distribution/update paths, privacy/retention processes, runbooks, support and rollback. List government-only integration/operational approvals separately from every unfinished internal task.

### Verify

All functional, capacity, per-language, accessibility, device, security, load, DR and distribution acceptance records cite the tested release revision. Zero unsupported critical guidance/mutations in the adversarial suite. No unresolved critical/high exploitable findings; all remaining risks have an accountable disposition. Missing cases/devices/reviewers mean incomplete readiness.

### Deliver and stop

A complete software-readiness evidence bundle exists; gate S remains provisional until P10 retirement/clean-build proof. The software may be ready to integrate approved feeds, but remains non-operational until P11/P12.

## P10 — Retire obsolete files and verify the final artifact

Prerequisites: P9. Initial status: NOT_STARTED.

### Read and establish

Read cleanup.md, current Git status/import graph/route inventory, Go/mobile manifests, tests and every deployment reference. Recheck for user edits or new consumers since P0. Do not apply the dated deletion table blindly.

### Execute

1. Confirm replacement tests and real clients no longer need legacy endpoints, frontend-owned fixtures or Python application state. Preserve needed source/contract fixtures and model inference runtimes.
2. Delete superseded permanent-relocation modules, retired web clients/tests and unused Azure/Bedrock adapters only after their final consumer is replaced. Remove matching obsolete build/CI/deploy references in the same coherent stage. Do not create a duplicate legacy backup directory; use Git history.
3. Retain protected .txt files and original data/model/user assets. Do not use broad recursive deletion over untracked/ignored paths. Required inference Python is not obsolete merely because the product backend is Go.
4. Ensure release images/archives contain only required application/runtime assets; no demo keys, synthetic live defaults, unused old endpoints or provider secrets. Demo packages belong in an explicitly separate demo distribution/profile.
5. Refresh current setup/runbook/instructions and active trackers. Old completed tasks stay historical rather than active requirements. Stage only this work, preserve unrelated staged changes and commit verified retirement units.
6. Record final API/schema, dependency/SBOM, release/image/app hashes and clean-install steps; link surviving evidence and decisions.

### Verify

Build from a clean checkout and declared dependencies, apply migrations, run Go/DB/model/client integration and representative mobile flows. Search all retained files for deleted imports/routes/paths, inspect package contents and scan artifacts/secrets. Compare requirements/invariants to P9; .txt and unrelated user work must be preserved.

### Deliver and stop

Gate S passes only after P0–P10 software evidence is complete and the final artifact reproduces it. The only remaining release work is explicitly government-dependent integration/validation and operational authorization; otherwise report NOT_READY with exact internal gaps.

## P11 — Authorized government integration and shadow exercises

Prerequisites: Gate S + O05/O07/O08 evidence. Initial status: BLOCKED_EXTERNAL.

### Read and establish

Read source-register.md and signed source/route/capacity agreements, permitted sample schemas, quotas, credentials policy, operational owner and escalation. Current public portals and synthetic fixtures are insufficient. Do not paste secrets into prompts or commits.

### Execute

1. Verify authorization and documented endpoint/auth/coverage/retention contracts. Use scoped ingestion credentials, source allow-lists and upstream quota. Keep clients/model workers unable to access those credentials or arbitrary upstream URLs.
2. Add/adapt only verified source connectors behind the existing canonical contract. If real schemas require a change, version it and rerun affected contract/client/security checks; planning cannot guarantee unknown APIs need zero adaptation.
3. In shadow mode ingest live permitted data for authorized evaluators only. Reconcile source IDs/times/CRS/updates/cancels, closed/full facilities, route ownership/passability/closure freshness and capacity-policy behavior with the responsible authority.
4. Validate publish/sign/revoke and source conflict handling end to end; rehearse source outage, key rotation, upstream quota exhaustion and rollback without delivering unapproved citizen guidance.
5. Have local operators verify actual routes, landmarks, access modes, shelter suitability and temporary-stay processes. Historical demo success cannot substitute for this field evidence.
6. Record named government, operations, privacy, security and language/accessibility sign-off for a controlled pilot. Any failed source stays suspended; do not downgrade validation to enable it.

### Verify

Pass permitted live contract/replay tests, shadow reconciliation, route/closure/capacity field drills, upstream protection checks and signed acceptance of operational policy. No public routing based solely on OSM/model candidate data.

### Deliver and stop

Authorized sources become OPERATIONAL only with recorded evidence; permission to run a controlled citizen pilot is explicit. Otherwise retain BLOCKED_EXTERNAL with the missing item and continue only independent safe work.

## P12 — Controlled launch and operational scale-out

Prerequisites: P11 + explicit release authorization. Initial status: BLOCKED_EXTERNAL.

### Read and establish

Read controlled-pilot approval, geographic/language boundaries, support/on-call roster, release/rollback plan, privacy process, alert/route/capacity SLAs and latest signed build evidence. Production is not authorized by this planning document.

### Execute

1. Start the approved bounded citizen pilot with named local operators and human fallback. Show source/expiry/limitations clearly; monitor task failures, closure latency, capacity conflicts, model fallbacks and source freshness.
2. Use incident-response and rollback procedures when guidance/data or infrastructure fails. Do not promise notification delivery or responder dispatch without actual confirmation.
3. Compare real load, language/UX success, battery/network behavior and inference cost with assumptions. Update capacity plans from measured peaks before opening more jurisdictions.
4. Review drill/incident findings with authorized owners, patch and repeat affected safety/security/device checks on the new revision. Maintain source/route/capacity approval and model/data version traceability.
5. Expand only through jurisdiction-specific authorization, staffing, validated language/route/facility data and infrastructure headroom. One million total users is a service goal, not evidence of tested simultaneous capacity.

### Verify

Pilot metrics, field/operator feedback, approved source freshness, valid routes, capacity reconciliation, security/privacy monitoring, support coverage and rollback exercise meet the signed operating agreement. Release/expansion actions require their explicit authorization.

### Deliver and stop

Operational release is a documented government/product/operations decision with ongoing monitoring and incident ownership. Never equate a passed demo, scanner report or API connection with flawless real-world evacuation.

## P6 contract-freeze record (2026-09-21) — coordinator, run FIRST

```text
Phase / status: P6 — Regional ASR, constrained middle model and TTS — contract
  freeze DONE. P6 engineering IN_PROGRESS via six parallel workers.
  P7 preparation also IN_PROGRESS via three further workers; P6 acceptance
  and P7 gate B remain gated.

Base commit (before the freeze): bf17515 docs(plan): split P5 corrections and
  P6-P7 parallel work.

Scope completed and changed files (coordinator edits on CLEAN):
  - plan/p6-contract.md — frozen typed contract, file ownership per worker,
    base commit, seven resolved boundaries and acceptance matrix A1..A10.
  - backend/internal/contracts/context.go — typed ScopedContext with typed
    PlaceCandidate / RouteRef / FacilityRef / ZoneRef / EligibleChoice;
    SnapshotRevalidator seam.
  - backend/internal/contracts/transcription.go — typed ASR envelope,
    TranscriptionState, bounded compressed-audio limits, content-type
    allow-list, nullable confidence semantics.
  - backend/internal/contracts/tts.go — typed TTS request/response/cache-key
    shapes, ApprovedTemplate and TemplateRegistry seams.
  - backend/internal/contracts/pipeline.go — typed PipelineRequest/
    PipelineResponse unions, PipelineState, bounded input limits.
  - backend/internal/contracts/worker_health.go — typed WorkerHealth,
    ModelInfo, ArtifactDigest, QueueStats and the private ASR/Middle/TTS
    request/response shapes that the workers' isolated Go modules consume.
  - backend/internal/contracts/errors.go — additive codes only:
    TRANSCRIPT_UNAVAILABLE, AUDIO_UNAVAILABLE, MODEL_TIMEOUT, QUEUE_SATURATED,
    INFERENCE_CANCELLED, TEMPLATE_UNKNOWN, STALE_SNAPSHOT.
  - backend/contracts/openapi.yaml — additive only: three new paths
    /api/v3/voice/{transcriptions,process,speech} with typed schemas
    TranscriptionResponse / PipelineRequest / PipelineResponse / TTSRequest /
    TTSResponse. Existing paths and schemas unchanged.
  - backend/internal/contracts/p6_contract_test.go — golden roundtrip tests
    for ScopedContext, the three pipeline/TTS/ASR envelopes, the cache key,
    the worker-health envelope and the new error codes.

Acceptance matrix (plan/p6-contract.md):
  A1 go build ./...                                                       PASS
  A2 existing voice-command validation tests                              PASS
  A3 TestServedRoutesMatchOpenAPI                                          PASS
  A4 new envelopes marshal/unmarshal losslessly (roundtrip table)         PASS
  A5 ScopedContext JSON-roundtrip stable                                  PASS
  A6 new error codes present                                              PASS
  A7 OpenAPI additive slices lint clean (no removal of existing paths)    PASS
  A8 freeze document records base/ownership/resolved boundaries           PASS
  A9 no shared file outside contract ownership list modified              PASS
  A10 no .txt file modified                                               PASS

Tests/commands, environment and results (Go 1.27.1 darwin/arm64):
  - gofmt -l . → clean
  - go vet ./... → clean
  - go build ./... → ok
  - go test ./internal/contracts ./internal/httpserver ./internal/store
    ./internal/httpjson ./internal/capfeed ./internal/opkg ./internal/sourceact
    ./internal/catalogue ./internal/offlinepkg ./internal/offlinedelivery
    ./internal/offlineresources -count=1 → all ok

File ownership and amendments:
  - Six P6 workers and three P7 workers consume this freeze verbatim.
  - Workers own their assigned directories only; shared backend/go.mod,
    backend/contracts/openapi.yaml, cmd/sthira/main.go, backend/migrations/,
    backend/internal/httpserver/server.go and plan/prompt.md remain under
    coordinator ownership. Workers PROPOSE exact deltas in their handoff;
    no shared-edit overlap is acceptable.
  - Workers request contract amendments (typed-shape additions, new error
    codes) by submitting a written proposal in their lane handoff. The
    coordinator integrates accepted amendments in a single follow-up commit.

P5 history preserved verbatim. P5 acceptance is IN_PROGRESS in the lanes
(offlineclient, store/publication, offlinequeue) and is NOT closed by this
freeze. P5 worker Areas A–G findings remain open; they are not silently
relabelled complete.

Unresolved internal work: none for the contract freeze.
External dependency, owner and exact evidence needed (UNCHANGED):
  O01..O16 remain open; O03/O04 hardware/reviewers/budget block real
  language acceptance, not the contract freeze. No external evidence is
  required for the freeze itself.

Worker launch order:
  Once the freeze commit is reported below, six P6 workers (W4, W5, W6,
  W7, W8, W9) and three P7 workers (W10, W11, W12) may begin. Workers
  create one codex/<task-name> worktree each from the freeze commit; they
  do not edit the freeze. The coordinator stops here.

Next eligible step: parallel P6 implementation per the freeze; P7
preparation in parallel. P6 acceptance cannot bypass P5 defect closure;
P7 gate B cannot pass before P6 acceptance.
```
