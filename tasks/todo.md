# Sthira v2 Migration Checklist

> **Historical Python/hackathon tracker — superseded 2026-09-19.** This file
> records the completed Python reference application and the old hackathon
> workflow as evidence only. It is **not** the active backlog. The active
> engineering sequence is the Go/mobile phase ledger in
> [`../plan/prompt.md`](../plan/prompt.md) (P0–P12); the governing plan is
> [`../plan/plan.md`](../plan/plan.md). Completed items below are historical
> evidence for the Python reference app under `src/`; they do not satisfy any
> Go, mobile or live-operation gate. Unchecked items below are superseded by
> the Go phases and remain here only as a record of what the Python app lacked.
## Current frontend task - first-use flow and visual correction

- [x] Restore the visible language then location setup flow on every fresh app load.
- [x] Recompose first use and the map guidance surface around clear emergency hierarchy, not dashboard-card patterns.
- [ ] Verify the reduced-motion path in a browser that exposes motion-preference emulation.

### Review - 2026-09-25

- The application now opens with language selection on each new page load, then advances to the explicit location choice before any map loads. Language preference is still retained for the interface itself.
- First-use now shares the civic-night visual language with the emergency map: the strong route action is reserved for the actual guidance screen, while setup uses clear choices and avoids generic form-card styling.
- Browser verification at the narrow preview viewport confirmed the language entry screen after refresh and the map's location-unavailable state. TypeScript type-checking, focused geometry tests (2 passed), and whitespace validation pass. Browser-level reduced-motion emulation remains to be checked.

## Current frontend task - mobile emergency-surface rework

- [x] Remove extruded building blocks from the 3D terrain view.
- [x] Unify the map tools, alert brief, and thumb-zone controls into one mobile visual system.
- [x] Prevent route and voice sheets from being obscured by persistent map controls.
- [x] Rework first-use selection and voice-sheet affordances to remove ambiguous secondary actions.
- [x] Verify narrow viewport layout, build, tests, then commit the completed change.

### Review - 2026-09-25

- Removed OpenFreeMap extrusion blocks entirely. The 3D view is now an honest terrain perspective with satellite imagery and no invented or flat building cuboids.
- Narrow-browser checks cover the language and location sequence, combined zone controls, the map surface, the unobstructed route sheet, the voice-control sheet, and 3D terrain rendering.
- `npm run build`, TypeScript checking, `tests/test_v2_map_geometry.py` (2 passed), and `git diff --check` pass.

## Current frontend task - mobile map decluttering

- [x] Collapse optional map layers behind a deliberate control and remove scattered duplicate route tools.
- [x] Keep device location useful for recentering without displaying it as a competing route marker.
- [x] Remove the large map brief, leaving the map as the primary mobile surface.
- [x] Make the voice launcher a friendly CSS character and reduce the mobile voice sheet to the essential interaction.
- [x] Make the mobile safe-route action focus the route without taking over the screen.
- [x] Verify narrow mobile, route, layers, and voice paths; run build and focused checks.

### Review - 2026-09-25

- Mobile now has one compact, top-right map control group. Zone choices appear only after Map Layers is opened, and the duplicate floating full-route action is gone.
- Browser location is retained solely for recentering. It is not drawn as a competing marker alongside the synthetic exercise route.
- The map has no large warning card. The thumb dock contains 112, the friendly Voice Map Control character, and a map-first route action.
- Narrow-browser checks confirmed the collapsed and expanded layer states, route activation without a directions sheet, and the concise voice-control sheet. `npm run build`, TypeScript checking, `tests/test_v2_map_geometry.py` (2 passed), and `git diff --check` pass.

## Current frontend task - mobile guidance flow correction

- [x] Restore step-by-step route guidance as a dedicated second mobile screen.
- [x] Keep browser location solely behind the explicit My Location action; load the scenario map by default.
- [x] Reduce onboarding spacing and type scale so both setup screens fit one mobile viewport.
- [x] Verify map, location, onboarding, and route-screen behavior.

### Review - 2026-09-25

- The map screen remains quiet and map-first. Tapping Route opens a full mobile guidance page with the route title and three existing step-by-step instructions, rather than placing another panel over the map.
- Device coordinates no longer select the map's initial camera. They are only used after the explicit My Location control is tapped.
- Browser screenshots confirm both setup screens fit the narrow viewport and that the route page has no top-bar overlap. `npm run build`, `tests/test_v2_map_geometry.py` (2 passed), and `git diff --check` pass.

## Current frontend task - mobile visual-system unification

- [x] Replace character-style Speak launcher with the shared microphone icon.
- [x] Give dock controls one shape, height, and text grammar.
- [x] Normalize mobile sheet surfaces, close controls, type, and action hierarchy.
- [x] Verify narrow map, voice, emergency confirmation, and route pages.

### Review - 2026-09-25

- Speak now uses the same SVG microphone family as Voice Map Control. No emoji or character icon remains.
- All dock controls use the same dark surface, height, radius, and stacked icon-or-code grammar. 112 is an outlined emergency code without duplicate visible copy.
- Mobile voice, route, and modal surfaces now share navy material, close-button geometry, muted supporting type, and blue action treatment. Browser checks cover narrow map and voice states; build, geometry tests (2 passed), and whitespace validation pass.

## Current frontend task - hierarchy and material system

- [x] Make Call 112 one clear, dominant emergency action.
- [x] Establish distinct display, UI-label, and body typography roles using bundled fonts.
- [x] Define shared sheet geometry with semantic visual tiers for voice, route, and blocking emergency confirmation.
- [x] Verify narrow mobile states and run build/tests.

### Review - 2026-09-25

- Call 112 is now a filled red dock action with a phone icon and explicit verb. Its confirmation uses a red critical surface and a single red dialler action.
- The bundled Noto Sans 800 weight supplies display hierarchy; small labels use tracked UI styling; body instructions remain readable at the regular text weight.
- Voice, route, and modal screens share spacing, close controls, radii, and navy materials, while accent rules communicate voice, route, or urgent-call stakes. Narrow-browser checks cover the map and emergency confirmation. `npm run build`, `tests/test_v2_map_geometry.py` (2 passed), and `git diff --check` pass.

## Current frontend task - component contract correction

- [x] Standardize bottom-dock icon treatment.
- [x] Make critical confirmation fully opaque and reserve modal accent chrome for transient sheets.
- [x] Separate display, route, voice, and emergency type roles.
- [x] Verify then commit mobile-system work.

### Review - 2026-09-25

- All dock icons now use the same bare SVG treatment. Call 112 stays dominant through its filled red action, not an inconsistent icon badge.
- Critical confirmation is an opaque red surface. Voice uses rounded blue-accented sheet chrome. Route is intentionally full-screen navigation without modal accent treatment.
- Typography roles now distinguish large emergency/route display headlines from compact voice UI headings and tracked labels. Browser confirmation, build, geometry tests (2 passed), and whitespace validation pass.

## Current frontend task - map presence and zone motion

- [x] Show consented device location as a map dot with a distinct orientation halo.
- [x] Replace static zone polygons with restrained, semantically distinct perimeter and pulse motion.
- [x] Verify device, zone, reduced-motion, and build paths; commit.

### Review - 2026-09-25

- Consented device coordinates now render as a blue location dot with a breathing orientation halo; the device label remains hidden so it does not compete with the exercise route.
- Red zones use a strong dashed perimeter and red atmospheric pulse. Relocation zones use a softer green dashed boundary and slower pulse. Both remain synthetic exercise overlays, not live forecasts.
- Narrow-browser confirmation shows the red-zone boundary and layer toggle. Animation is bypassed when reduced motion is active. `npm run build`, geometry tests (2 passed), and `git diff --check` pass.

## Current frontend task - voice-first shell

- [x] Add startup screen, single bottom Speak action, top-left Call Help, and three call choices.
- [x] Remove consumer-facing local-model wording from the voice surface.
- [x] Add explicitly synthetic hospital marker to the synthetic relocation area.
- [ ] Add spoken language selection once backend supports language detection before transcription.

### Review - 2026-09-25

- Voice-first shell now starts before language and location setup, and map controls use one large Speak action. Call Help exposes rescue, ambulance, and 112 handoffs.
- The existing backend transcriber requires a preselected Malayalam or Hindi language and rejects English microphone capture, so spoken language selection cannot be truthfully connected yet.
- Hospital data is absent from the current synthetic package. The rendered point is named `Synthetic hospital`; replace it with approved backend coordinates before operational use. Build and geometry tests (2 passed) pass.


## Current frontend task - 3D map perspective

- [x] Add a MapLibre 2.5D camera toggle and mouse/touch tilt controls.
- [x] Keep the existing north-up reset and camera paths predictable.
- [x] Verify the frontend build and map geometry tests.

### Review

- [x] The 3D control toggles a 58 degree pitch and 18 degree bearing. Recenter restores 2D north-up orientation. `npm run build` and `tests/test_v2_map_geometry.py` pass.

## Documentation pivot

- [x] Replace permanent-relocation product scope with citizen emergency guidance.
- [x] Define government-data-only authority boundary.
- [x] Research and register official government sources.
- [x] Define Voice Map Control and AI4Bharat model boundaries.
- [x] Define explicit arrival and capacity semantics.
- [x] Mark geofence arrival as future-only.
- [x] Rewrite every Markdown file and leave text files untouched.
- [x] Add the end-to-end autonomous phase prompts in `../plan/prompt.md`.

## Code migration — Python reference app (historical)

The sections below track the Python reference application, not the Go product
backend. They are retained as evidence of what was built and verified in
Python.

### Phase 0 — repository pivot and safety baseline

- [x] Inspect legacy backend, frontend, fixtures, tests, dependencies, and startup path.
- [x] Record REUSE/ADAPT/ISOLATE/REMOVE_LATER inventory from three independent audits.
- [x] Add isolated sthira_v2 namespace and /api/v2/status boundary.
- [x] Add DEMO/SHADOW/PILOT/PRODUCTION runtime profiles with fail-closed pilot/production guard.
- [x] Label legacy /ui/ as V1 synthetic demo and not emergency guidance.
- [x] Update package metadata to citizen emergency guidance.
- [x] Preserve all legacy tests and .txt files.
- [ ] Add repeatable CI workflow and full v2 application shell in Phase 1.

### Phase 1 — v2 foundation and contracts (Python reference)

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

### Phase 2 — persistence, audit, and operational states (Python reference)

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

- [ ] Close applicable items in `../plan/open-decisions.md`.
- [ ] Obtain government permissions and operational samples.
- [ ] Obtain DDMA/KSDMA safe-zone, route, capacity, allocation, and update SOP.
- [ ] Establish 24×7 government operational ownership and manual fallback.
- [ ] Complete privacy, legal, security, language, accessibility, and incident-response approvals.

## Review

The current application is a locally runnable synthetic emergency-guidance demo and must not be represented as a live emergency system.

## Frontend integration pass — 2026-09-24

- [x] Replace the chat-first interaction with constrained Voice Map Control.
- [x] Connect the interface to documented voice-command, assignment, and arrival contracts with safe degraded states; expose the approved-audio and ISL pending states without inventing a backend endpoint.
- [x] Preserve text, keyboard, and reduced-motion paths and localize every new interface string.
- [x] Verify TypeScript build, targeted map/action checks, and the browser control flow.

### Review — 2026-09-24

- `npm run build` passes and `tests/test_v2_map_geometry.py` passes (2 tests).
- Browser checks confirm the voice-control sheet, unavailable-command handling, Malayalam localization, ISL text fallback, and honest arrival failure state.
- The frontend is prepared for `/api/v2/voice/commands` and assignment/arrival routes, which are not exposed by the currently running backend process.

## Mobile emergency experience redesign — planned

- [x] Audit the current Vite mobile layout, bundle, and non-map fallback before changing the flow.
- [x] Add a first-run onboarding flow: language selection, clear location consent, and an equally clear manual-location path when permission is denied or unavailable.
- [x] Make the post-onboarding screen map-first, with a persistent voice launcher at the bottom and a compact emergency-action tray.
- [x] Replace the current "I reached the shelter" interaction with the user-approved next emergency action; preserve the backend arrival contract until its removal is explicitly confirmed.
- [x] Evolve Voice Map Control from a launcher into a guided, accessible voice interaction while retaining transcription, text, and no-microphone fallbacks.
- [x] Keep the desktop layout compatible without making it the primary design target.
- [x] Add lightweight, feedback-only motion and reduced-motion support; do not add real-time claims or unsupported feeds.
- [x] Measure the production build and verify the primary flows at a narrow mobile viewport and a desktop viewport.

### Success criteria

- First use selects a language and makes location use understandable and optional.
- The emergency map, one prominent voice action, and urgent actions are reachable with one hand on a phone.
- No screen promises live, official, or safety-critical guidance that the backend cannot verify.
- The interface remains usable with location denied, microphone denied, a failed map, and reduced motion.
- The redesign introduces no unapproved external API or heavy visual runtime.

### Review — 2026-09-25

- First use now requires a language selection, then offers browser location permission with an explicit continue-without-location path. Device coordinates are not mixed into the synthetic route.
- Mobile is a full-map emergency surface with a compact route brief, 112 action, central voice action, and route control. The full guidance panel remains on desktop.
- The visible arrival flow was removed. Its backend API contract was not changed.
- MapLibre is dynamically imported only after onboarding. The entry JavaScript is now 66.81 kB (17.93 kB gzip); MapLibre remains a deferred 279.98 kB gzip map chunk.
- Verified in-browser at the default narrow viewport and at 1366×900, then restored the default viewport. `npm run build`, `tests/test_v2_map_geometry.py`, and `git diff --check` pass.

## Visual-system correction — in progress

- [x] Replace the template-like desktop panel and generic map control styling with one intentional emergency visual system.
- [x] Recompose first-use language and location setup so it feels like part of the product, not a generic form.
- [x] Use browser geolocation coordinates for the user's own map recenter action without falsely connecting them to the synthetic route.
- [x] Re-verify phone, laptop, language, location-denied, and reduced-motion paths.
- [x] Commit the completed frontend redesign on `CLEAN`.

## Real 3D map correction — in progress

- [x] Remove the invented scenario buildings.
- [x] Validate real building coverage through OpenFreeMap's published TileJSON instead of guessed tile URLs.
- [x] Add measured terrain elevation, mapped building footprints, and visible provider attribution.
- [x] Verify terrain rendering and route controls in the running browser and production build.
- [ ] Obtain a licensed photogrammetry source and verify regional coverage for photographic building façades.

### Correction evidence

- Removed all eight invented building footprints. OpenFreeMap TileJSON provides the versioned tile URL and a maximum native zoom of 14; a decoded scenario tile contains actual mapped building polygons.
- Terrain uses Terrarium elevation tiles at real scale. Browser diagnostics confirmed ground elevation around 773 metres at the scene center and a route draped over the hillside.
- Runtime terrain switching produced intermittent empty frames. Initializing terrain with the map style fixes that, retaining the selected center and zoom while switching views.
- Fixed the mobile route button binding so both desktop and mobile route actions open guidance. No new rendering dependency was added.
- Final production build and geometry tests pass; browser checks cover terrain-on rendering, 2D reset, and mobile route activation. Photorealistic mesh integration remains pending a licensed source and coverage verification.

The earlier claim of absent OpenFreeMap coverage was invalid: the test used an incorrect tile URL and unsupported zooms. Textured Google Earth-style city meshes require a separate photogrammetry source; extruded footprints alone are not that feature.

## Previous 3D map pass — superseded

- [x] Add real Esri vector building footprints as an extrusion layer over the existing satellite imagery.
- [x] Make the 3D control enter a genuine building scene and keep the approved route visible above it.
- [x] Add one quiet, fully translated grounding line that supports action without obscuring emergency guidance.
- [ ] Verify reduced-motion behavior in a browser that exposes motion-preference emulation.

### Review — 2026-09-25

- The 3D control now lifts the MapLibre camera to a street-level zoom and turns on shaded `fill-extrusion` layers. Esri building footprints render when the provider has coverage.
- The current synthetic route tile has no Esri building footprints, so the demo also renders a separate, explicitly illustrative set of synthetic building context blocks. The Esri satellite base and all emergency route/zone semantics remain unchanged.
- The route layers remain ordered above the extrusion layers. The new grounding line is present in English, Malayalam, and Hindi.
- `npm run build`, `tests/test_v2_map_geometry.py`, `git diff --check`, and narrow-browser 2D/3D/localization verification pass. The existing reduced-motion duration boundary remains in use; it still needs browser-level preference emulation verification.

### Review — 2026-09-25

- Replaced the dashboard visual language with a cohesive civic-night frame, calm off-white guidance surface, direct route hierarchy, connected map controls, and a single dominant action.
- First-use language and location screens now use the same visual system rather than generic form cards.
- Browser location coordinates remain in memory only. When consent succeeds, the map adds a distinct device marker and “My location” recentres to it. The synthetic exercise route remains separately labelled. When consent is denied or unavailable, the interface says so directly.
- Verified the mobile map surface, desktop sidebar composition, voice panel, Malayalam localization, and location-unavailable path in browser. `npm run build`, `tests/test_v2_map_geometry.py`, and `git diff --check` pass.


## Current execution evidence — 2026-09-12

## Voice and emergency emphasis correction — 2026-09-25

- [x] Give the startup voice action an explicit icon/text grid so its label cannot collapse on narrow phones.
- [x] Make Call Help a larger, high-contrast red emergency control; preserve its three dialler choices.
- [x] Add restrained motion and mic framing to Speak without obscuring map content or ignoring reduced-motion settings.
- [x] Verify onboarding, voice control, emergency choices, production build, geometry tests, and whitespace check.

### Review — 2026-09-25

- Browser verification at the default phone viewport confirms Begin remains legible, Speak opens the focused voice sheet, and Call Help opens rescue, ambulance, and 112 choices.
- `npm run build`, `python -m pytest tests/test_v2_map_geometry.py -q`, and `git diff --check` pass.

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
