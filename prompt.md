---
title: Sthira v2 — End-to-End Autonomous Build Prompts
document_id: STHIRA-PROMPTS
version: 1.0
status: Ready for execution
as_of: 2026-09-11
---

# Sthira v2 End-to-End Build Prompts

## 1. How to use this file

Start with:

> Read `prompt.md` completely and execute Phase 0. Follow all standing instructions and stop only at the phase exit gate.

After a phase is genuinely complete, the user only needs to say:

> Move to the next phase.

On that instruction, the agent must reread this file, inspect the execution ledger and repository evidence, select the first phase that is not `DONE`, and execute that phase. It must not merely describe the work.

If a phase is blocked by government authorization, credentials, policy, hardware, or another external dependency, complete every safe synthetic/local part, mark the phase `BLOCKED_EXTERNAL`, document the exact evidence required, and continue only to work that does not falsely imply the blocker is resolved.

## 2. Standing master prompt

The following instructions apply to every phase:

1. Read `CLAUDE.md`, `GEMINI.md`, `prd.md`, `rules.md`, `trd.md`, `architecture.md`, `decisions.md`, `source-register.md`, `open-decisions.md`, `parameters.md`, `equations.md`, `phases.md`, and this file before making architectural changes.
2. Treat Sthira v2 as a citizen emergency-guidance bridge. Government systems remain authoritative for alerts, red zones, safe zones, routes, capacities, allocation policy, instructions, and emergency response.
3. Do not add hazard prediction, safe-zone prediction, route generation, satellite analysis, automatic arrival, a general-purpose chatbot, or private operational data.
4. Do not modify `.txt` files.
5. Preserve unrelated user changes. Inspect `git status` before editing.
6. Use a new `sthira_v2` application/domain boundary. Reuse safe infrastructure deliberately; do not perform a destructive mass rewrite of legacy v1.
7. Use synthetic, unmistakably labeled government-format fixtures until a source is `OPERATIONAL` under `source-register.md`.
8. Never hard-code a government policy value when the owner has not approved it. Use configuration, an explicit unknown state, and fail closed.
9. Write or update tests with each behavior. Test failure and adversarial states, not just happy paths.
10. Run the smallest relevant test set while iterating, then the complete applicable suite before closing the phase.
11. Verify user-visible changes in the running application at desktop and mobile widths, including keyboard and screen-reader semantics.
12. Record material decisions in `decisions.md`, unresolved policy in `open-decisions.md`, implementation status in `tasks/todo.md`, and meaningful lessons in `tasks/lessons.md`.
13. Do not claim a live government integration from documentation, a mocked response, a scraped page, or a synthetic fixture.
14. Do not claim production readiness while any relevant live-pilot gate remains open.
15. Do not commit, push, deploy, contact agencies, register accounts, or change external systems unless the user explicitly requests it.
16. Finish each phase with: outcome, files changed, migrations/configuration, tests run with results, running-app verification, remaining risks, external blockers, and the next phase number.

## 2.1 Anti-hallucination contract

These rules are mandatory and override any phase wording that could be read more broadly:

1. **Inspect before asserting.** Before naming a file, symbol, endpoint, dependency, command, database table, environment variable, test result, or existing behavior, locate it in the repository or an authoritative source. Use `rg`, file inspection, test output, generated OpenAPI, database inspection, or official documentation as evidence.
2. **Never invent external access.** Do not invent API URLs, credentials, authentication flows, schemas, quotas, licenses, government approvals, source coverage, update frequency, contacts, or live samples. If official documentation does not establish a detail, add a typed configuration/interface, fixture, validation rule, and explicit `BLOCKED_EXTERNAL` record.
3. **Separate four evidence classes.** Every data path and report must distinguish:
   - `SYNTHETIC_DEMO`: locally created test data;
   - `CAPTURED_OFFICIAL_SAMPLE`: an official artifact captured under known terms, not necessarily live;
   - `AUTHORIZED_SHADOW`: approved live data that cannot guide citizens;
   - `AUTHORIZED_OPERATIONAL`: approved live data permitted for citizen guidance.
   Never promote data between classes automatically.
4. **No fake implementation.** Do not use placeholder returns, hard-coded success, random values, silent exception swallowing, empty `except`, commented-out requirements, `pass`, untracked `TODO`, or mocks in production paths to make tests green.
5. **No fake integration.** A fixture-backed adapter must say `fixture` or `demo` in configuration, API metadata, UI, logs, and tests. It must not use names such as `live`, `connected`, `real-time`, or `officially integrated`.
6. **No speculative abstraction.** Create an interface only when at least one current implementation or test seam needs it. Prefer a small cohesive module over factories, registries, base classes, event buses, or microservices with one consumer.
7. **No duplicated domain truth.** CAP lifecycle, source state, facility state, assignment state, capacity arithmetic, and voice intents each have one canonical definition. Frontend display types are generated from or explicitly mapped to backend contracts.
8. **No unsafe convenience.** Do not weaken validation, authorization, idempotency, transaction isolation, expiry, provenance, accessibility, or demo labeling to simplify implementation.
9. **No test manipulation.** Do not delete, skip, loosen, or rewrite a legitimate test merely to obtain green output. If a test represents superseded v1 behavior, isolate it with a documented migration decision rather than silently changing its assertion.
10. **No unverified dependency.** Before adding a package or model, verify the exact package/repository, maintained version, license, platform support, security implications, and why existing dependencies or standard library are insufficient. Pin compatible bounds and update the dependency record.
11. **No broad destructive change.** Do not delete legacy modules, fixtures, migrations, or tests until v2 replacement behavior is verified and the phase explicitly authorizes removal. Never modify `.txt` files.
12. **No hidden fallback.** When authoritative data is absent or invalid, return the documented unavailable/degraded state. Never fall back to private data, guessed geometry, stale data beyond policy, or model-generated instructions.
13. **No fabricated verification.** Report only commands actually run and observations actually made. Include exit status and concise output. “Should pass,” “appears correct,” and code inspection alone do not satisfy a runtime exit gate.
14. **Stop on contradiction.** If two normative documents conflict, do not guess. Use the most recent accepted decision only when precedence is unambiguous; otherwise record the conflict in `open-decisions.md` and stop the affected behavior.
15. **Keep scope exact.** Implement only the current phase and prerequisites required to make it correct. Do not begin later phases because they are convenient.

## 2.2 Mandatory implementation loop

For every phase, perform this sequence:

1. **Establish baseline**
   - Run `git status --short`.
   - Locate applicable code/tests with `rg`.
   - Run the smallest existing baseline test set and record failures before editing.
   - Read current contracts and migrations that the phase will touch.
2. **Write a phase checklist**
   - Copy every numbered requirement and required test from the phase into `tasks/todo.md`.
   - Mark nothing complete without repository or command evidence.
3. **Design the narrow change**
   - State the files/modules to add or modify.
   - Reuse existing safe code only after inspecting it.
   - Add an ADR before implementation when changing the target technical baseline or a safety boundary.
4. **Implement vertically**
   - Add/adjust a failing test for one behavior.
   - Implement the smallest production code that satisfies it.
   - Run the focused test.
   - Repeat until the phase behavior is complete.
5. **Verify integration**
   - Run all affected backend and frontend tests.
   - Run the full suite before phase completion.
   - For UI work, start the real application and exercise the relevant flows in a browser at mobile and desktop widths.
   - For persistence/concurrency work, use the real PostgreSQL/PostGIS transaction path, not only an in-memory substitute.
6. **Audit the diff**
   - Run `git diff --check`.
   - Review `git diff` for secrets, unrelated edits, debug output, placeholders, dead code, duplicate logic, unhandled errors, and misleading claims.
   - Confirm `git diff --name-only -- '*.txt'` is empty.
7. **Close honestly**
   - Update contracts, OpenAPI, migrations, documentation, todo evidence, and the execution ledger.
   - Mark `DONE` only if every exit condition passes.
   - Otherwise use `BLOCKED_EXTERNAL` or leave `IN_PROGRESS` with the exact failing evidence.

## 2.3 Production-code quality rules

- Use typed domain objects at boundaries; do not pass unvalidated dictionaries through business logic.
- Keep transport, validation, domain decisions, persistence, and presentation separate.
- Use dependency injection for clocks, identifiers, repositories, HTTP clients, and model adapters so tests remain deterministic.
- Use timezone-aware UTC internally and preserve the official source timezone for display/audit.
- Use stable machine-readable error codes; do not make clients parse prose.
- Use database constraints as well as application validation for uniqueness, foreign keys, non-negative capacity, legal state values, and idempotency keys.
- Do not use process-global mutable dictionaries as production storage.
- Do not perform network calls inside database transactions.
- Bound HTTP connections, retries, response sizes, XML sizes, audio sizes, worker queues, and inference concurrency.
- Use explicit timeouts on every network/model call.
- Sanitize rendered text and disallow executable HTML/scripts from imported artifacts.
- Keep secrets out of code, fixtures, logs, snapshots, screenshots, and committed environment files.
- Return safe public errors while preserving correlation IDs for operators.
- Include docstrings only where they explain a non-obvious invariant; prefer clear code over narrative comments.
- Remove debug prints, unused imports, unreachable code, and speculative flags before closing a phase.
- A new endpoint is incomplete without validation, authorization classification, error behavior, OpenAPI contract, tests, and observability.
- A new UI control is incomplete without loading, empty, error, disabled, keyboard, screen-reader, reduced-motion, and localization behavior where applicable.

## 2.4 Required phase completion report

Use exactly these headings:

```text
Phase:
Status: DONE | IN_PROGRESS | BLOCKED_EXTERNAL
Implemented:
Files changed:
Data/migrations:
Configuration:
Verification commands and results:
Running-app verification:
Requirements satisfied:
Unresolved risks:
External evidence still required:
Next phase:
```

Do not include a success statement above this report if the status is not `DONE`.

## 3. Target technical baseline

- Backend: Python 3.10+, FastAPI, Pydantic v2, SQLAlchemy 2, Alembic, PostgreSQL 16 with PostGIS.
- Background work: a minimal worker abstraction; use an in-process test worker first and add Redis/Celery only when Phase 3 load/reliability evidence requires it.
- Frontend: Vite, TypeScript, semantic HTML, CSS design tokens, MapLibre GL JS, service worker/PWA manifest.
- Contracts: JSON Schema/OpenAPI plus raw CAP XML retention.
- Tests: pytest, integration tests against PostgreSQL/PostGIS where required, frontend unit tests, browser end-to-end tests, accessibility checks, and concurrency tests.
- Speech: self-hosted AI4Bharat IndicConformer-600M-multilingual; proposed self-hosted Indic Parler-TTS after its activation gate.
- Maps/data: government-supplied geometry and approved routes; government basemap/tiles only after authorization. Synthetic local vector tiles or simple GeoJSON in demo.
- Operations: structured logs, metrics, health/readiness endpoints, trace/request IDs, source freshness, audit events, backup/restore rehearsal, and runbooks.

Any change to this baseline requires an ADR with rationale, consequences, and rejected alternatives.

## 4. Execution ledger

Update this table only after verifying the corresponding exit gate.

| Phase | Name | Status | Evidence |
| --- | --- | --- | --- |
| 0 | Repository pivot and safety baseline | DONE | v2 boundary, profile guard, legacy warning; .venv/bin/python -m pytest -q 186 passed |
| 1 | v2 foundation and contracts | DONE | strict contracts, synthetic fixture, OpenAPI smoke, Makefile; make check: 203 passed |
| 2 | Persistence, audit, and operational states | NOT_STARTED | — |
| 3 | SACHET-compatible CAP alert backbone | NOT_STARTED | — |
| 4 | Government operational-package ingestion | NOT_STARTED | — |
| 5 | Citizen emergency interface | NOT_STARTED | — |
| 6 | Assignment, arrival, and capacity integrity | NOT_STARTED | — |
| 7 | Official context connectors and source health | NOT_STARTED | — |
| 8 | Voice Map Control with IndicConformer | NOT_STARTED | — |
| 9 | TTS, multilingual content, ISL, and accessibility | NOT_STARTED | — |
| 10 | Offline operation, emergency calling, and notifications | NOT_STARTED | — |
| 11 | Security, privacy, resilience, and observability | NOT_STARTED | — |
| 12 | End-to-end assurance and deployment packaging | NOT_STARTED | — |
| 13 | Authorized shadow pilot | BLOCKED_EXTERNAL | Requires government agreements and live samples |
| 14 | Controlled citizen pilot | BLOCKED_EXTERNAL | Requires Phase 13 approval and 24×7 operations |

Allowed statuses: `NOT_STARTED`, `IN_PROGRESS`, `BLOCKED_EXTERNAL`, `DONE`.

---

# Phase prompts

## Phase 0 — Repository pivot and safety baseline

### Prompt

Audit the current repository against Sthira v2. Do not implement product features yet.

1. Inspect git state, Python package structure, frontend, tests, fixtures, configuration, and startup path.
2. Produce a machine-readable inventory of legacy v1 modules grouped as `REUSE`, `ADAPT`, `ISOLATE`, or `REMOVE_LATER`. Include reasons and dependencies.
3. Establish a v2 namespace and routing prefix without breaking the current test suite.
4. Add a visible v1 legacy/demo warning where the old interface remains reachable.
5. Add configuration profiles `DEMO`, `SHADOW`, `PILOT`, and `PRODUCTION`; default to `DEMO`.
6. Add a startup guard that prevents `PRODUCTION` when required source, database, security, and authorization configuration is absent.
7. Add CI-ready commands for backend tests, frontend checks, formatting/linting, and dependency installation.
8. Update package metadata from the permanent-relocation description to Sthira Citizen Emergency Guidance without renaming historical Python classes yet.

### Required tests

- Existing tests still pass or any deliberately isolated legacy failures are documented and approved.
- v2 import/startup smoke test.
- production guard rejects incomplete configuration.
- demo mode is unmistakably labeled.
- no `.txt` diff.

### Exit gate

There is a safe v2 boundary, repeatable local setup, honest legacy labeling, green baseline tests, and a written migration inventory. Mark Phase 0 `DONE`.

## Phase 1 — v2 foundation and contracts

### Prompt

Implement contract-first domain foundations under `src/sthira_v2`.

1. Define strict Pydantic models and enums for:
   - `OfficialAlert` with complete CAP identity/lifecycle fields.
   - `OperationalZoneVersion` for `RED` and `SAFE`.
   - `SafeZoneVersion`, `ApprovedRouteVersion`, and `OfficialInstructionSet`.
   - `CitizenSession`, `Assignment`, `ArrivalConfirmation`, and `CapacityEvent`.
   - source provenance, validation state, freshness state, localization, and API envelope/error.
2. Use timezone-aware UTC timestamps, explicit CRS, GeoJSON-compatible geometry, constrained party sizes, stable identifiers, and version fields.
3. Define legal state transitions from `trd.md`; reject illegal transitions.
4. Generate or expose JSON Schema/OpenAPI examples.
5. Create small synthetic Wayanad fixtures for one active alert, one red zone, three safe zones, approved routes, capacity, and English/Malayalam instructions. Label every object `SYNTHETIC_DEMO`.
6. Ensure imported text is data, never executable prompt/instruction to the application.

### Required tests

- Valid/invalid contract fixtures.
- timezone, expiry, CRS, geometry, capacity, and party-size boundaries.
- illegal state transitions.
- provenance required on every official-looking fact.
- serialization round trips and OpenAPI schema snapshot.

### Exit gate

All v2 domain contracts are stable, documented, synthetic fixtures validate, and no application workflow depends on legacy relocation models. Mark Phase 1 `DONE`.

## Phase 2 — Persistence, audit, and operational states

### Prompt

Add production-shaped persistence without adding premature distributed infrastructure.

1. Introduce SQLAlchemy 2 repositories and Alembic migrations for v2 entities.
2. Use PostgreSQL/PostGIS in integration/production profiles. Permit SQLite or in-memory repositories only for narrow unit tests where spatial/locking semantics are irrelevant.
3. Store original source artifacts with checksum, media type, received time, authority, and retention class.
4. Implement effective-time and system-time versioning for alerts, zones, routes, facilities, and instructions.
5. Implement append-only audit events with request ID, actor/source, action, object/version, timestamp, result, and tamper-evident hash chaining.
6. Add source state lifecycle: `DISCOVERED -> ACCESS_REQUESTED -> SAMPLE_ACQUIRED -> VALIDATED -> AUTHORIZED -> OPERATIONAL -> SUSPENDED/RETIRED`.
7. Add health/readiness checks for database, artifact storage, active source configuration, and migrations.
8. Ensure personal location and voice data are absent from ordinary logs.

### Required tests

- Alembic upgrade from empty database and downgrade where safe.
- PostGIS geometry/CRS validation.
- bitemporal supersession and historical reconstruction.
- audit hash-chain verification and tamper detection.
- production readiness fails without operational source configuration.

### Exit gate

The v2 storage model can reconstruct which government-versioned facts and actions existed at any tested time. Mark Phase 2 `DONE`.

## Phase 3 — SACHET-compatible CAP alert backbone

### Prompt

Build the primary alert ingestion path using synthetic and captured-authorized fixtures only.

1. Implement a hardened CAP 1.2 XML parser:
   - disable external entities and unsafe expansion;
   - preserve raw XML;
   - parse `identifier`, `sender`, `sent`, `status`, `msgType`, `scope`, references, info blocks, language, category, event, response type, urgency, severity, certainty, effective/onset/expires, headline, description, instruction, area polygons/circles, and resources.
2. Implement an HTTP adapter with ETag/`If-None-Match`, `200` replacement, `304` cached reuse, timeouts, backoff, jitter, rate limits, and circuit breaking.
3. Deduplicate by CAP identifier and correctly apply `Update`, `Cancel`, and reference relationships.
4. Validate sender allow-list, geography, times, schema, and artifact checksum. Quarantine failures.
5. Build:
   - `GET /api/v2/alerts/active`
   - `GET /api/v2/alerts/{id}`
   - operator feed-health/quarantine endpoints.
6. Return source, issue/expiry, freshness, and degraded states in every citizen response.

### Required tests

- 200, 304, new alert, duplicate, update, cancel, expiry, malformed XML, XXE payload, oversized payload, wrong sender, invalid polygon, timeout, and stale-cache cases.
- API contract and source-to-response trace.
- no valid cache replacement on failed refresh.

### Exit gate

Synthetic CAP alerts safely traverse fetch, raw storage, validation, lifecycle, API, and audit. No live-SACHET claim is made without authorization. Mark Phase 3 `DONE`.

## Phase 4 — Government operational-package ingestion

### Prompt

Implement the authenticated/versioned DDMA package that supplies all facts Sthira must not predict.

1. Specify a JSON/GeoJSON package manifest containing authority, jurisdiction, effective/expiry times, alert linkage, red zones, safe zones, facilities, total capacity, operational status, accessibility, approved routes, ordered landmark instructions, allocation-policy reference/order, local emergency contacts, language assets, checksums, and signature metadata.
2. Implement import, schema validation, checksum/signature verification interface, CRS/bounds checks, topology checks, cross-reference checks, version conflict detection, quarantine, preview, publish, supersede, cancel, and rollback-to-prior-valid-view.
3. Require an authenticated, jurisdiction-scoped operator for publish/correct/cancel.
4. Never infer missing capacity, route, status, safety, or allocation policy.
5. Serve active zones/routes/facilities through citizen guidance APIs.
6. Render synthetic GeoJSON locally until a government basemap decision closes ODN-007.

### Required tests

- invalid/missing signature, checksum, geometry, route link, capacity, language asset, policy, jurisdiction, expiry, and conflicting versions.
- unauthorized/cross-jurisdiction publication.
- atomic publication: clients see either prior or complete new package, never a partial mixture.
- superseded route disappears from new guidance but remains auditable.

### Exit gate

An authorized synthetic operational package can be imported, reviewed, published, superseded, and reconstructed without prediction. Mark Phase 4 `DONE`.

## Phase 5 — Citizen emergency interface

### Prompt

Replace the legacy satellite-screening page with the v2 mobile-first emergency experience.

1. Create a Vite/TypeScript frontend or document an ADR if adapting the existing static frontend is demonstrably safer.
2. Implement states: no active alert, active alert, location permission denied, manual-place selection, assignment pending, route active, stale/conflicting data, full/closed facility, arrival question, arrived, and offline cache.
3. Active-alert screen must show official hazard/event, urgency/severity/certainty, affected area, issue/expiry, source, last refresh, approved instruction, and labeled official precipitation/context when available.
4. Map must show only official/synthetic citizen position, red zone, assigned safe zone, and approved route. Include source/version legend.
5. Provide a fully equivalent non-map route with ordered steps, landmarks, destination name/contact/accessibility, data age, and emergency call action.
6. Use calm high-contrast design, large touch targets, one dominant next action, reduced motion, responsive layout, and explicit demo labeling.
7. Remove satellite-screening, candidate-site, permanent-relocation, Sarvam, and ASL claims from the active v2 UI.

### Required tests

- component/unit tests for every state.
- API error, stale data, no map, no geolocation, no JavaScript enhancement, and slow network behavior.
- keyboard order, accessible names, live regions, color contrast, zoom, reduced motion.
- running-app browser verification at representative mobile and desktop sizes.

### Exit gate

A citizen can understand a synthetic alert and reach complete official-style guidance without voice or a functioning map. Mark Phase 5 `DONE`.

## Phase 6 — Assignment, arrival, and capacity integrity

### Prompt

Implement the government-policy-driven assignment and capacity ledger.

1. Accept explicit party size and citizen-session ID without requiring civil identity.
2. Filter only published, active, open safe zones with an active approved route, matching jurisdiction/alert, and sufficient reservable capacity.
3. Order candidates only by the versioned government allocation policy. If absent, return `ASSIGNMENT_UNAVAILABLE`.
4. In one ACID transaction, lock the relevant capacity record, recheck availability, create the assignment/reservation, and append the ledger event.
5. Implement:
   - `POST /api/v2/assignments`
   - `GET /api/v2/assignments/{id}`
   - `POST /api/v2/assignments/{id}/arrival-confirmations`.
6. Arrival `YES` converts/resolves reservation to occupied exactly once for the party size. `NO` makes no occupancy decrement and offers help.
7. Implement expiry, authorized cancellation/release, facility closure, reassignment, and compensating correction events.
8. Voice must not be accepted as arrival confirmation.

### Required tests

- simultaneous last-place assignments, transaction retries, repeated taps, repeated HTTP requests, stale client, party-size changes, closed/full facility, assignment expiry, correction, and database restart.
- invariant/property tests: remaining capacity never negative; every delta traceable; one arrival per assignment/idempotency key.
- end-to-end UI confirmation in English and Malayalam fixtures.

### Exit gate

Load/concurrency tests prove no overbooking or double-decrement, and the complete assignment-to-arrival flow works. Mark Phase 6 `DONE`.

## Phase 7 — Official context connectors and source health

### Prompt

Add connector interfaces and verified samples for official context without claiming unavailable live access.

1. Implement a generic government-source adapter contract with authority, product, coverage, timestamps, units, schema version, attribution, health, and raw artifact.
2. Implement the IMD adapter first for the exact approved APIs needed for district warning/nowcast/rainfall. Preserve observation versus forecast, station/area, units, issue time, and validity.
3. Add adapter skeletons and fixture tests for CWC/NWIC, GSI Bhusanket, INCOIS, FSI, NCS, KSDMA, NDEM/Bhuvan, and district operational packages.
4. Do not scrape around authentication, CAPTCHAs, portal controls, or undocumented bulk limits.
5. Add source health metrics: last success, last valid artifact, latency, status, age, circuit state, quarantine count, and operational owner.
6. Define conflict presentation/precedence exactly as `source-register.md`; never silently merge contradictory warnings.
7. Add operator diagnostics and citizen-safe degraded messaging.

### Required tests

- unit/contract fixtures per adapter.
- incorrect units, missing timestamps, changed schema, out-of-coverage station, stale data, source outage, and disagreement.
- confirm supporting precipitation cannot create/change severity, zone, route, or assignment.

### Exit gate

IMD works with permitted fixture/sample data; all other adapters have honest states and contracts. Mark live-dependent items `BLOCKED_EXTERNAL` rather than fabricating them. Mark Phase 7 `DONE` when local scope is complete.

## Phase 8 — Voice Map Control with IndicConformer

### Prompt

Implement self-hosted speech-to-text and deterministic map control.

1. Create an isolated inference adapter for `ai4bharat/indic-conformer-600m-multilingual`; pin the exact model revision, checksum, runtime, dependencies, decoding strategy, and license record.
2. Never download or execute unreviewed remote model code in production. Build a reproducible model-acquisition/deployment procedure.
3. Accept short, explicitly recorded utterances; enforce media type, duration, size, sample rate/channel conversion, timeout, rate limit, and immediate raw-audio deletion.
4. Return language, transcript, confidence/decoder metrics, request ID, and no-authority disclaimer.
5. Build a deterministic multilingual grammar for:
   `SHOW_MY_LOCATION`, `SHOW_ALERT_AREA`, `SHOW_SAFE_ZONE`, `SHOW_ROUTE`, `FOCUS_PLACE`, `PAN`, `ZOOM_IN`, `ZOOM_OUT`, `RECENTER`, `REPEAT_INSTRUCTION`, `CHANGE_LANGUAGE`, and `OPEN_EMERGENCY_CALL`.
6. Resolve places only from the active official gazetteer/operational package.
7. Animate map camera only when reduced motion is not requested. Show transcript and interpreted command.
8. Low confidence, ambiguity, unsupported language, missing slot, or prohibited request must clarify/fail with no side effect.
9. `OPEN_EMERGENCY_CALL` opens the UI confirmation sheet only. Voice cannot dial or confirm arrival.

### Required tests

- text-command corpus before model integration.
- per-language clean/noisy/accent/code-switch command evaluation with false-action rate.
- prompt-injection phrases, arbitrary instructions, place ambiguity, homophones, silence, long audio, malformed audio, denial-of-service limits.
- full touch/keyboard parity and microphone-denied flow.
- latency/memory/GPU benchmark on target hardware.

### Exit gate

Voice reliably moves/focuses the synthetic map within approved per-language thresholds and cannot cause prohibited side effects. If hardware/model access is unavailable, finish the adapter/parser/UI with fixtures and mark inference `BLOCKED_EXTERNAL`.

## Phase 9 — TTS, multilingual content, ISL, and accessibility

### Prompt

Complete multimodal guidance without overstating language support.

1. Resolve ODN-011: identify the exact Indic Parler-TTS artifact, version, license, parameter size, supported languages, inference runtime, hardware, and security posture.
2. Integrate it behind an internal adapter only for languages that pass comprehensibility, pronunciation, latency, and license gates.
3. Synthesize only approved instruction text. Cache by content hash, language, voice/version, and instruction version. Purge speech when the source instruction is cancelled/superseded.
4. Provide visible play/pause/repeat, transcript, captions, volume-independent visual instruction, and text fallback.
5. Implement complete English and Malayalam pilot UI/content coverage; no partial language switch.
6. Add an ISL media component using authority-approved video/animation with captions and transcript. Do not call text ISL and do not label ISL as ASL.
7. Run WCAG 2.2 AA and applicable GIGW checks with keyboard, screen reader, switch-like interaction, 200%/400% zoom, contrast, focus, reduced motion, and cognitive-load review.
8. Conduct human review for emergency meaning; machine translation cannot publish authoritative instructions by itself.

### Required tests

- missing/unsupported TTS language, synthesis failure, cancelled instruction cache, audio unavailable, slow inference.
- localization key completeness and layout expansion.
- captions/transcript/keyboard/screen-reader tests.
- documented human review checklist and unresolved external media approvals.

### Exit gate

Every critical action is complete in text; approved languages have verified audio; ISL is either approved and functional or honestly marked pending. Mark Phase 9 accordingly.

## Phase 10 — Offline operation, emergency calling, and notifications

### Prompt

Make the emergency flow useful during network and service failure.

1. Add a PWA manifest and service worker with an explicit caching strategy.
2. Cache the last valid unexpired alert, assignment, destination, route steps, instruction assets, source/version, and emergency numbers. Never cache secrets or unnecessary precise-location history.
3. Display offline, last-updated, expiry, and stale states. Expired guidance cannot be presented as current.
4. Implement user-confirmed `tel:112` and approved local-number handoff. Show the number when dialing is unavailable.
5. Never claim call connection, dispatch, responder visibility, or location sharing.
6. Create a notification abstraction, but activate push/SMS only through an authorized government arrangement. Demo notifications must remain local and labeled.
7. Handle an alert update/cancel while a cached route is open; invalidate superseded instructions safely.
8. Provide printable/downloadable low-bandwidth route card only from the current official package.

### Required tests

- airplane mode, intermittent network, service-worker update, corrupt cache, expired alert, cancelled route, clock skew, storage eviction, denied notifications, unsupported dialler.
- no production notification from demo/shadow.
- browser-installed PWA smoke test on target platforms where available.

### Exit gate

The citizen retains honest, source-stamped emergency essentials during network loss and can initiate an explicit emergency call. Mark Phase 10 `DONE`.

## Phase 11 — Security, privacy, resilience, and observability

### Prompt

Harden the complete system and prove its degraded behavior.

1. Implement operator OIDC/SAML-ready authentication boundary, MFA requirement, RBAC/ABAC, jurisdiction scoping, and deny-by-default authorization.
2. Protect APIs with strict validation, request/body limits, rate limiting, safe CORS, CSRF policy where applicable, secure headers, secret management, dependency pinning/SBOM, and artifact scanning.
3. Threat-model SSRF, XML attacks, malicious CAP text/resources, stored/reflected XSS, SQL injection, broken access control, replay, source impersonation, stale guidance, capacity races, voice prompt injection, model denial of service, and supply-chain compromise.
4. Implement privacy controls for foreground location, manual location, raw voice deletion, session expiry, retention jobs, access logs, correction/deletion workflow, and DPIA evidence placeholders.
5. Add structured logs, metrics, dashboards/queries, alerts, source freshness, capacity conflicts, inference latency/failure, and user-safe correlation IDs.
6. Add database backup/restore, artifact recovery, source outage, speech outage, map outage, notification outage, and complete application rollback runbooks.
7. Run dependency, secret, static, dynamic/API, and authorization tests available locally; document tool limits.

### Required tests

- cross-jurisdiction and privilege-escalation suite.
- malicious imported content rendered inert.
- retention deletes permitted personal data without breaking non-personal audit integrity.
- backup/restore and recovery-time evidence.
- load/soak/failover tests using the pilot profile.

### Exit gate

No critical/high unresolved security issue, privacy behavior is enforceable, and rehearsed degraded modes preserve honest guidance. Mark Phase 11 `DONE`.

## Phase 12 — End-to-end assurance and deployment packaging

### Prompt

Produce a complete locally deployable, production-shaped release candidate without claiming a live pilot.

1. Build a full synthetic incident journey:
   government-format CAP alert -> validation -> citizen notification/view -> official zone/facility/route -> assignment -> multilingual/map/voice guidance -> 112 confirmation -> arrival Yes/No -> capacity update -> correction/cancel -> audit reconstruction.
2. Add browser end-to-end tests for normal, accessibility, offline, stale, full shelter, route cancellation, source disagreement, microphone denied, TTS failure, and repeated arrival.
3. Produce container/deployment manifests, environment-variable reference, migration command, health/readiness checks, resource sizing, model-volume strategy, rollback, backup/restore, and operator runbook.
4. Ensure default deployment is `DEMO`; production startup remains impossible without approved operational configuration.
5. Generate an acceptance matrix mapping every FR/NFR/RUL to tests or an explicit external blocker.
6. Remove active v2 UI references to satellite prediction, permanent relocation, candidate sites, Sarvam, and ASL.
7. Run full backend/frontend/browser/security/accessibility test suites and record exact results.
8. Update all documentation and the execution ledger to match actual implementation.

### Exit gate

A new developer can set up and run the full synthetic system from documented commands; the acceptance matrix is complete; all local gates pass; external/live blockers are explicit. Mark Phase 12 `DONE`.

## Phase 13 — Authorized shadow pilot

### Prompt

Execute this phase only after the user provides evidence that the applicable government agreements, credentials, endpoints, permitted samples, data-protection review, and named operational contacts exist.

1. Validate evidence against every connector activation gate in `source-register.md`.
2. Configure secrets outside git and activate feeds in `SHADOW`, never directly in `PILOT`.
3. Compare raw official artifacts with normalized and displayed output.
4. Measure freshness, latency, availability, conflicts, coverage, operator workload, ASR/TTS performance, and capacity reconciliation.
5. Run incident tabletop and technical game-day exercises with government owners.
6. Record approval, rejection, or corrective actions per source and feature.

### Exit gate

Named government, security, privacy, accessibility, language, and operations owners sign the controlled-pilot readiness record. Otherwise remain `BLOCKED_EXTERNAL`.

## Phase 14 — Controlled citizen pilot

### Prompt

Execute only with explicit government authorization and a Phase 13 sign-off.

1. Limit geography, users, time window, supported hazards/languages, and operating hours exactly to the approval.
2. Provide 24×7 escalation, manual fallback, kill switch, rollback, and public pilot labeling.
3. Monitor source freshness, guidance delivery, assignment/capacity integrity, emergency-call handoffs, accessibility failures, security/privacy events, and near misses.
4. Do not experiment with unapproved guidance during an active incident.
5. Produce an evidence-based `GO`, `ITERATE`, or `STOP` report. Do not scale automatically.

### Exit gate

The competent authority accepts the pilot report and explicitly authorizes the next operational scope. Otherwise stop safely.

---

# Completion protocol

When the user says “move to the next phase”:

1. Inspect the ledger and repository evidence.
2. Run the mandatory implementation loop in §2.2; do not rely only on the ledger text.
3. Verify the previous phase is truly complete; repair it first if its exit evidence is false or stale.
4. Mark the selected phase `IN_PROGRESS`.
5. Execute its prompt autonomously and only within its defined scope.
6. Run every locally possible required test.
7. Update documentation and evidence.
8. Mark `DONE` only when its exit gate passes; otherwise use `BLOCKED_EXTERNAL` or `IN_PROGRESS`.
9. Return the required completion report from §2.4 and identify the next phase.

When all locally executable phases through Phase 12 are `DONE`, say clearly:

> The complete synthetic, production-shaped application is built and verified locally. Live operation is not active. Phases 13–14 require government authorization, operational data, credentials, and sign-off.

Never reinterpret “move to the next phase” as permission to bypass an external authorization, deploy publicly, use live personal data, or make a production emergency claim.
