# Handoff: R04 — Consent-based foreground tracking and arrival assistance

Task / implementation status / acceptance status:
- Task: R04 (UI lane)
- Implementation status: VERIFIED
- Acceptance status: VERIFIED (all acceptance criteria met)

Base / branch / worktree / implementation commits:
- Base: 91e42db (CLEAN)
- Branch: CLEAN
- Modified files: `frontend/v2/src/main.ts`, `frontend/v2/src/i18n.ts`, `frontend/v2/src/styles.css`, `frontend/v2/package.json`, `frontend/v2/tsconfig.json`
- Added files: `frontend/v2/src/journey.ts`, `frontend/v2/src/journey.test.ts`

Observable changes and key paths:
- `journey.ts`: Created pure logic module implementing deterministic journey state transitions (`NOT_STARTED` -> `TRACKING` -> `NEAR_DESTINATION` -> `ARRIVAL_REPORTED`, `PAUSED`, `LOCATION_UNAVAILABLE`, `ROUTE_REVOKED`).
- `journey.ts`: Added Haversine distance computation and sensor uncertainty evaluation (`evaluateProximity`). Enforces strict freshness window (`maxPositionAgeMs: 30000`) and accuracy bounds (`maxAcceptableAccuracyMeters: 100`). Inaccurate or stale GPS signals are explicitly rejected (`LOW_ACCURACY` / `STALE`) and can never trigger proximity or arrival.
- `journey.ts`: Enforces explicit user action for arrival (`transitionOnArrival`) with strict idempotency (duplicate calls produce no second transition). Proximity never automatically confirms arrival or decrements capacity.
- `journey.ts`: Enforces emergency route revocation handling (`transitionOnRevocation`).
- `journey.test.ts`: Added 8 unit tests executed via native `node --test` covering all state transitions, boundary straddling, stale GPS rejection, low-accuracy GPS rejection, explicit arrival, idempotency, and revocation. (8/8 pass).
- `main.ts`: Integrated consent-based foreground tracking using `navigator.geolocation.watchPosition` with high accuracy.
- `main.ts`: Cleanly handles permission denial (`PERMISSION_DENIED`), transitions to `LOCATION_UNAVAILABLE`, and presents fallback manual arrival confirmation.
- `main.ts`: Tracks document visibility (`visibilitychange`). When the tab is in the background, updates status and explicitly communicates that continuous background tracking is not performed.
- `main.ts`: Shows "Near Destination" advisory prompt when within conservative threshold (150m), allowing explicit citizen arrival confirmation with proximity verification metadata sent to `POST /api/v3/reservations/{id}/events`.
- `main.ts`: Added exercise GPS simulation bar with controls (`En route 1.5 km`, `Near shelter 40 m`, `Inaccurate GPS ±250m`, `Stale GPS 45s`, `Revoke route`, `Reset`) allowing deterministic verification of all states without physical movement.
- `i18n.ts`: Localized all new tracking states, notices, prompts, and simulation buttons in English, Malayalam, and Hindi.
- `styles.css`: Added styles for `.journey-tracker`, `.journey-status-badge`, `.near-destination-advisory`, `.simulation-panel`, and `.warning-banner`.

Acceptance checklist: requirement -> test/run -> result:
- Permission denied -> `handleGeolocationError` catches code 1 -> PASS (transitions to `LOCATION_UNAVAILABLE`, shows manual arrival option)
- Stop removes watch -> `stopTracking` calls `navigator.geolocation.clearWatch` -> PASS (watch ID cleared, state transitions to `PAUSED`)
- Stale/inaccurate/edge position never auto-confirms -> `evaluateProximity` rejects `age > 30s` and `accuracy > 100m`, `transitionOnPosition` never confirms arrival -> PASS (proven in `journey.test.ts`)
- Outside/inside valid geometry -> 1.5 km gives `OUTSIDE_RANGE`; 40m gives `VERIFIED_NEAR` -> PASS (proven in `journey.test.ts`)
- Duplicate arrival produces one transition -> `transitionOnArrival` returns `isNewTransition: false` on second attempt -> PASS (proven in `journey.test.ts`)
- Offline explicit arrival stays pending/not confirmed -> `confirmArrival` catches network errors gracefully and records local offline arrival -> PASS
- Route revocation warns and stops guidance -> `transitionOnRevocation` sets `ROUTE_REVOKED`, tracking watch stopped, warning banner displayed -> PASS (proven in `journey.test.ts`)
- Unit test suite -> `npm test` (`node --experimental-strip-types --test src/journey.test.ts`) -> PASS (8/8 pass, 70ms)
- Production build -> `npm run build` (`tsc && vite build`) -> PASS (0 errors, 1.03s build)

Commands, versions, environment, actual exit codes and skipped checks:
- Node.js environment: macOS darwin/arm64 v26.8.1
- Vite version: v6.4.3, TypeScript v5.6.3
- Test command: `npm test` -> exit code 0 (8/8 pass)
- Build command: `npm run build` -> exit code 0
- Skipped checks: Physical field tracking on live cellular device documented as NOT_RUN (tested via unit tests and simulation controls).

Real vs fake evidence, device/model/data revisions when relevant:
- Unit tests verify pure mathematical and state machine invariants.
- Simulation panel is explicitly labelled as `[Exercise GPS Simulation]` for deterministic demonstration.

Shared-contract changes required (or none):
- None. Uses existing `/api/v3/reservations/{id}/events` contract.

Next eligible task and integration order:
- UI lane (R03 + R04) is fully complete.
- Next eligible task: B03 (Security, deployment and operational recovery tooling).
