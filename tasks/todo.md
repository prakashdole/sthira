# Sthira v2 Migration Checklist

## Documentation pivot

- [x] Replace permanent-relocation product scope with citizen emergency guidance.
- [x] Define government-data-only authority boundary.
- [x] Research and register official government sources.
- [x] Define Voice Map Control and AI4Bharat model boundaries.
- [x] Define explicit arrival and capacity semantics.
- [x] Mark geofence arrival as future-only.
- [x] Rewrite every Markdown file and leave text files untouched.
- [x] Add the end-to-end autonomous phase prompts in `prompt.md`.

## Code migration — not started

### Phase 0 — repository pivot and safety baseline

- [x] Inspect legacy backend, frontend, fixtures, tests, dependencies, and startup path.
- [x] Record REUSE/ADAPT/ISOLATE/REMOVE_LATER inventory from three independent audits.
- [x] Add isolated sthira_v2 namespace and /api/v2/status boundary.
- [x] Add DEMO/SHADOW/PILOT/PRODUCTION runtime profiles with fail-closed pilot/production guard.
- [x] Label legacy /ui/ as V1 synthetic demo and not emergency guidance.
- [x] Update package metadata to citizen emergency guidance.
- [x] Preserve all legacy tests and .txt files.
- [ ] Add repeatable CI workflow and full v2 application shell in Phase 1.

### Phase 1 — v2 foundation and contracts

- [x] Add strict Pydantic v2 contracts and legal state transitions.
- [x] Add CAP alert, provenance, freshness, geometry, zone, route, instruction, session, assignment, arrival, and capacity models.
- [x] Add a current-dated `SYNTHETIC_DEMO` flood fixture with one alert, one red zone, three safe zones, stored routes, capacities, and English/Malayalam instructions.
- [x] Add fixture integrity validation and focused tests.
- [x] Expose v2 OpenAPI status smoke coverage.
- [x] Add Makefile checks for Python compile, frontend syntax, full tests, and v2 tests.
- [x] Verify full suite: 203 passed, 2 existing dependency deprecation warnings.
- [x] Add isolated Vite v2 frontend foundation with responsive synthetic emergency guidance shell and localhost dev command.
- [x] Add Codex-like Voice Map Control launcher and model-ready voice-to-text panel with explicit demo state.
- [x] Add satellite-ready map surface with explicit government-basemap authorization guard; no unapproved provider connected.
- [x] Apply government-style red-alert hierarchy, severity words/icons, large emergency actions, multilingual controls, and bundled Noto fonts.
- [x] Add explicit connection/offline state and keyboard-accessible alert details with synthetic source, issue, and expiry metadata.
- [x] Complete tactical full-viewport emergency PWA composition with MapLibre local synthetic canvas, floating guidance drawer, route/shelter metrics, voice panel, and arrival preview.
- [x] Refine tactical PWA into centered device frame with consolidated status pill, richer local geospatial layers, required test hooks, and non-overlapping controls.
- [x] Replace device frame with full-screen spatial HUD: desktop left command panel, floating glass islands, telemetry widget, and mobile bottom sheet.
- [x] Inject interactive arrival party stepper, 112 confirmation, route camera animation, voice command chips, directions drawer, multimodal assist previews, map layer controls, and tactile press states.
- [x] Reset citizen UI to civic evacuation hierarchy: deadline first, destination second, one dominant route action, plain-language directives, human arrival copy, and rescue escalation.
- [ ] Add database/migrations in Phase 2.

### Phase 2 — persistence, audit, and operational states

- [x] Add SQLAlchemy 2 typed models and repositories for official fact versions, source artifacts/states, assignments, capacity events, and audit events.
- [x] Add PostgreSQL/PostGIS-shaped Alembic migration scaffolding without embedded credentials.
- [x] Add effective-time/system-time supersession and historical reconstruction.
- [x] Add append-only audit hash chaining with tamper verification.
- [x] Add database constraints for provenance, versions, capacity, party size, and legal stored states.
- [x] Add focused server-free metadata, bitemporal, and audit tests.
- [x] Add v2 API readiness endpoint with fail-closed database, artifact, source, and migration states.
- [ ] Exercise upgrade/downgrade and spatial SRID checks on a real PostgreSQL 16/PostGIS service (external database unavailable in this environment; ODN-007).
- [ ] Add operational readiness endpoints after the real database/artifact-store configuration exists.

### Mandatory reconciliation checkpoint and hackathon map slice — execution evidence

- [x] Preserved pre-existing working-tree changes and confirmed no `.txt` file diff.
- [x] Reconciled the preferred `origin/main` Vite frontend into local `main` without restoring the duplicate static v2 frontend.
- [x] Ported the isolated synthetic CAP, operational-package, allocation/capacity, offline, source-health, deterministic voice, deployment, and acceptance slices.
- [x] Added Esri World Imagery as a satellite-style visual-only basemap, India overview camera, versioned local scenario JSON, GeoJSON overlays, attribution, and fail-soft imagery messaging.
- [x] Added strict ID-only map-action validation with bounded camera actions and reduced-motion handling.
- [x] Verified desktop and 390x844 browser flows: initial map, attribution, stored-route action, Voice Map Control panel, alert-area action, keyboard-visible accessible names, and synthetic date/disclaimer.
- [x] Add typed synthetic-scenario API response and API-backed frontend loading/provenance validation.
- [x] Verify API success and service-failure states in the running browser; no guidance is shown when the scenario cannot be verified.
- [ ] Complete runtime verification of every remaining degraded/error matrix item before closing the checkpoint.

- [ ] Inventory reusable infrastructure and isolate legacy v1 modules.
- [ ] Create v2 domain contracts and synthetic government-format fixtures.
- [x] Implement synthetic SACHET-compatible CAP parsing, lifecycle, raw-artifact API output, ETag/304 cache seam, retries, and quarantine tests.
- [ ] Implement operational package validation for zones, routes, facilities, capacities, and policy.
- [ ] Build citizen alert/map/non-map experience.
- [ ] Implement assignment and capacity ledger with race/idempotency tests.
- [ ] Implement arrival Yes/No flow.
- [ ] Implement 112/local official dialler confirmation.
- [ ] Implement deterministic text command grammar and map controller.
- [ ] Deploy and benchmark IndicConformer for pilot languages.
- [ ] Verify and integrate exact Indic Parler-TTS model.
- [ ] Produce/review Malayalam, English, and ISL emergency content.
- [ ] Add offline, stale, conflict, closed/full, and service-failure states.
- [ ] Run accessibility, security, load, disaster-recovery, and live-browser verification.

## Live pilot blockers

- [ ] Close applicable items in `open-decisions.md`.
- [ ] Obtain government permissions and operational samples.
- [ ] Obtain DDMA/KSDMA safe-zone, route, capacity, allocation, and update SOP.
- [ ] Establish 24×7 government operational ownership and manual fallback.
- [ ] Complete privacy, legal, security, language, accessibility, and incident-response approvals.

## Review

The current application is a locally runnable synthetic emergency-guidance demo and must not be represented as a live emergency system.

## Current execution evidence — 2026-09-12

- [x] Operational-package manifest now requires authority, jurisdiction, version, effective/expiry window, checksum, facilities, allocation policy, emergency contacts, geometry, and cross-references.
- [x] Synthetic package checksum verification, authenticated demo publication, version conflict detection, supersession, cancellation, rollback, and active-package API are covered by tests.
- [x] Assignment API requires alert/session/idempotency linkage and rejects payload conflicts; local concurrent allocation and arrival tests pass.
- [x] Fixture-only source adapter validates timestamps, units, provenance, schema metadata, and source identity; known live connectors remain `BLOCKED_EXTERNAL`.
- [x] Deterministic voice parser covers allow-listed map intents, known synthetic place aliases, confirmation-only emergency-call intent, low-confidence rejection, and prohibited/ranking/prompt-injection rejection.
- [x] TTS artifact gate/cache purge semantics, pending ISL status with text fallback, offline expiry/cancellation cache, privacy deletion/expiry, and degraded-mode runbook are implemented and tested.
- [x] Local AI4Bharat voice endpoints are wired to the demo UI: short recordings are transcribed locally, and only approved English/Malayalam synthetic instruction text can be synthesized locally.
- [x] Direct local Indic Parler-TTS smoke synthesis produced a 114,732-byte WAV from the approved English synthetic instruction on 2026-09-12; first cold loading/generation took about 28 seconds on this Mac and an in-process content-hash cache repeat completed in 0 seconds.
- [x] Transcript map control now crosses a constrained backend boundary. Azure GPT-4.1 mini may phrase a response when configured, but the deterministic local grammar constructs the only executable action plan and the browser validates it again.
- [x] Live Azure command smoke request returned `SHOW_SAFE_ZONE` with exactly three deterministic actions targeting `SZ-DEMO-01`; no provider-generated coordinate, route, or unknown ID was accepted.
- [x] Voice Map Control exposes a microphone-free `Demo transcript` field for the same constrained map-action path, so the presentation remains usable when recording is denied or unavailable.
- [x] The installed IndicConformer artifact’s language masks were inspected: Hindi (`hi-IN`) and Malayalam (`ml-IN`) are supported by this demo runtime; English microphone capture now fails visibly before recording instead of passing an invalid model key. English text/TTS and demo-transcript control remain available.
- [x] Corrected Malayalam runtime-key smoke execution completed locally. A synthetic TTS-generated Malayalam clip transcribed as empty text, so it is recorded only as runtime compatibility—not an ASR accuracy claim. Real recorded-utterance accuracy remains a pilot/hardware evaluation gate.
- [ ] Exercise browser microphone media compatibility, real local TTS playback, Azure-backed command explanation, and the complete voice failure matrix only after the implementation pass is declared complete.
- [x] CI workflow and Makefile v2 build target added; latest `make test`: 259 passed, 2 dependency deprecation warnings; frontend `npm run build` passes; deterministic `/api/v2/voice/commands` smoke check returns only validated safe-zone actions with Azure disabled; `git diff --check` and `.txt` guard pass.
- [x] The demo runbook includes the exact two-terminal local presentation commands and clearly states the local TTS cold-start and English ASR limitation.
- [x] Browser evidence: India overview, attribution, route/arrival flow with one reservation and one arrival POST, voice panel, alert-area action, API-unavailable fail-closed state, and recovery after backend restoration.
- [x] Latest local browser inspection at `http://127.0.0.1:5173/` renders the synthetic flood alert, Synthetic Ward 8 School assignment, synthetic disclaimer, text-first directives, emergency-call handoff, MapLibre surface, and language controls without real-world route wording.
- [ ] Real PostgreSQL 16/PostGIS migration/SRID/locking/restore evidence remains unavailable.
- [ ] Authorized government endpoints, operational samples, SOPs, source ownership, model artifacts/hardware, TTS/ISL approvals, and pilot sign-offs remain external blockers.
