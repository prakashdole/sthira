# Sthira v2 demo runbook

The demo profile is synthetic only. It must not be changed to `SHADOW`, `PILOT`,
or `PRODUCTION` without the source, database, authentication, security,
language, accessibility, and government-ownership gates recorded in `prompt.md`.

## Failure handling

- If the scenario API fails, keep the citizen screen at the verification/error
  state. Do not display cached guidance as current.
- If OpenFreeMap fails, retain local GeoJSON overlays, text route steps, and the
  synthetic disclaimer. Do not infer operational facts from the basemap.
- If speech fails or confidence is low, keep touch and keyboard controls active
  and apply no map action.
- If an emergency call is requested, show confirmation and open only the device
  dialler after explicit action. Never claim connection or dispatch.
- If a cached alert or route expires or is cancelled, invalidate it and show the
  unavailable/stale state.

## Recovery evidence required before activation

PostgreSQL 16/PostGIS migration and restore rehearsal, source artifact recovery,
authorized government samples, speech benchmark results, approved language/ISL
review, security/privacy assessment, and pilot-owner sign-off are external gates.
