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
- [x] Add SYNTHETIC_DEMO Wayanad fixture with one alert, one red zone, three safe zones, routes, capacities, and English/Malayalam instructions.
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
- [ ] Add database/migrations in Phase 2.

### Phase 2 — persistence, audit, and operational states

- [x] Add SQLAlchemy 2 typed models and repositories for official fact versions, source artifacts/states, assignments, capacity events, and audit events.
- [x] Add PostgreSQL/PostGIS-shaped Alembic migration scaffolding without embedded credentials.
- [x] Add effective-time/system-time supersession and historical reconstruction.
- [x] Add append-only audit hash chaining with tamper verification.
- [x] Add database constraints for provenance, versions, capacity, party size, and legal stored states.
- [x] Add focused server-free metadata, bitemporal, and audit tests.
- [x] Add v2 API readiness endpoint with fail-closed database, artifact, source, and migration states.
- [ ] Exercise upgrade/downgrade and spatial SRID checks on a real PostgreSQL 16/PostGIS service (external database unavailable in this environment).
- [ ] Add operational readiness endpoints after the real database/artifact-store configuration exists.

- [ ] Inventory reusable infrastructure and isolate legacy v1 modules.
- [ ] Create v2 domain contracts and synthetic government-format fixtures.
- [ ] Implement SACHET-compatible CAP ingestion and lifecycle.
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

The Markdown baseline is ready for code planning. The current application is not yet migrated and must not be represented as a live emergency system.
