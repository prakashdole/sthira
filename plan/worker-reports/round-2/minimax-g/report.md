# MiniMax G Report — Map Action Regression Preparation

**Worker:** minimax-g
**Timestamp:** 2026-09-27T00:00:00Z (assignment start)
**HEAD:** d95df9e609454877a91b7c82146b3c6368181b17
**Output directory:** plan/worker-reports/round-2/minimax-g/

---

## 1. Scope Inspected

| File | SHA-256 |
|------|---------|
| frontend/v2/src/mapActions.ts | a28f4f612416391283a1e832418681d98358437e43fa75be0803394558fdd490 |
| frontend/v2/src/mapActions.test.ts | aa5d284309e4a946b7a75559a962cbe0dbfca8d9926290d06ac3ab75f61b7a04 |
| frontend/v2/src/journey.ts | 6f0d4e2710197665efa5ed7ffc5b1251ebbc9f22ddf55cd063f39b9bf29705a1 |
| frontend/v2/src/main.ts | ed9dbcd5bbc7a63c46f42d1d63d03157d9644830842213324bbfca11d2294253 |

- Git log d95df9e–HEAD; commits 41a0278, fc0128b reviewed in full
- Dirty files: backend/internal/asrworker/audio_mime_test.go, backend/internal/middleworker/eval/*, frontend/v2/src/styles.css, backend/internal/httpserver/citizen_ownership_regression_test.go (not in scope)
- No Safari changes, no process kills, no installs/downloads

---

## 2. Existing Completed Work vs. New Contribution

| Item | Status | Evidence |
|------|--------|----------|
| MY_LOCATION layer toggle (device-pulse, device-point) | Covered | mapActions.test.ts:575–653 (fc0128b) |
| All 10 panel OPEN_PANEL validation | Covered | mapActions.test.ts:655–697 (fc0128b) |
| Confirmation panels (ARRIVAL, EMERGENCY, RESERVATION) no-mutation | Covered | mapActions.test.ts:699–729 (fc0128b) |
| missing-coordinate → UNAVAILABLE_COORDINATES | Covered | journey.test.ts fc0128b diff |
| safe-zone ID ambiguity (resolves only when exactly one facility) | Covered | journey.ts:486–487; journey.test.ts 41a0278 diff |
| RED_ZONES / SAFE_ZONES / ROUTES layer toggle unit coverage | Covered | mapActions.test.ts:523–563 |
| `executeMapActions` callback-per-action once | Covered | mapActions.test.ts:392–471 |
| **my-location layer ID in layerMap but absent from style** | **NEW — unexercised gap** | mapActions.ts:184–185 vs main.ts:1692–1713 |
| **RECENTER flyTo does not reset mapTilted → aria-pressed drift** | **NEW — unexercised gap** | main.ts:262–264 vs 1811–1818 |
| **dispatchVoiceProposal layer side effects not exercised in integration** | **NEW — unexercised gap** | main.ts:1223–1226 (no test) |
| **handleOpenPanelAction target_id rejection not exercised end-to-end** | **NEW — unexercised gap** | main.ts:1152–1213 (no integration test) |
| **Repeated 3D toggle → aria-pressed sync across multiple cycles** | **NEW — unexercised gap** | main.ts:1787–1810 (no cycle test) |
| **Candidate patch** | plan/worker-reports/round-2/minimax-g/candidate.patch | |

---

## 3. Findings

### 3.1 `my-location` in `layerMap` but no such layer exists in map style

**File:** `frontend/v2/src/mapActions.ts:184–185`

```typescript
export const layerMap: Record<Layer, string[]> = {
  // ...
  MY_LOCATION: ['device-pulse', 'device-point', 'my-location'],
};
```

**Evidence — main.ts:1692–1713 (initMap layers array):**
The map style layers include `device-pulse` (line 1707) and `device-point` (line 1708) but **no layer with id `'my-location'`** is ever added to the style. The `layerMap['MY_LOCATION']` entry contains three IDs; only two correspond to real style layers.

**Impact:** When `executeMapActions` runs `SET_LAYER_VISIBILITY { layer: 'MY_LOCATION', visible: true/false }`, the loop over `layerMap['MY_LOCATION']` calls `map.setLayoutProperty('my-location', 'visibility', ...)` — this is a silent no-op because no layer with that ID exists. No error is raised. The `onLayerVisibility` callback still fires, so callers cannot distinguish this from a successful layer toggle.

**Affected scenario:** A backend voice response that sends `SET_LAYER_VISIBILITY { layer: 'MY_LOCATION', visible: true }` will report success (callback fires) but the map has no `my-location` layer to update.

**Proposed correction (one of two options):**
- **Option A (remove):** Remove `'my-location'` from `layerMap['MY_LOCATION']` if it is a stray entry.
- **Option B (add):** Add a `'my-location'` layer to the map style in `initMap` with appropriate visibility.

`executeMapActions` at mapActions.ts:213–236 iterates `layerMap[action.layer]`. With Option A, the loop only hits real layers. Option B requires adding the layer definition and confirming it is not redundant with `device-pulse`/`device-point`.

**Uncertainties:** If `my-location` was intended to be a separate UI marker (e.g., a crosshair or label) distinct from `device-pulse`/`device-point`, it should be added to the style. If it is a leftover from an older naming convention, it should be removed from `layerMap`.

---

### 3.2 `RECENTER` action does not reset `mapTilted` — UI/state desync

**File:** `frontend/v2/src/main.ts:262–264` (executeMapActions RECENTER handler) and `main.ts:1811–1818` (recenterMap UI handler)

```typescript
// executeMapActions RECENTER handler (line 262):
if (action.type === 'RECENTER') {
  map.flyTo({ center: [76.112, 11.562], zoom: 13.4, bearing: 0, pitch: 0, duration }); // pitch:0
}

// recenterMap UI handler (line 1811):
function recenterMap() {
  const target = (deviceLocation || mapData.user) as [number, number];
  mapTilted = false;                          // resets tilt flag
  try { map?.setTerrain(null); } catch {}
  map?.easeTo({ center: target, zoom: 15, pitch: 0, bearing: 0, duration: motionDuration() });
  syncPerspectiveControl();                   // syncs aria-pressed
  recordCameraState();
}
```

The `RECENTER` MapAction calls `flyTo` with `pitch: 0` (line 263), which puts the camera in a 2D orientation. However, **it does not set `mapTilted = false`** in main.ts state, and does not call `syncPerspectiveControl()`. The UI button `aria-pressed` attribute remains `true` after a voice `RECENTER`, even though the map is now flat.

The UI `recenterMap()` function (line 1811) correctly resets `mapTilted = false` and calls `syncPerspectiveControl()`. The voice `RECENTER` action does not share this path.

**Impact:** After a voice command sends `RECENTER`, the 3D toggle button shows `aria-pressed="true"` (visually lit) but the map is in 2D mode. Clicking the button then toggles to 3D (restoring pitch), which may surprise the user.

**Proposed correction:** Add `mapTilted = false; syncPerspectiveControl();` to the `RECENTER` handler in `executeMapActions` (main.ts), or route voice RECENTER through `recenterMap()` instead of directly calling `flyTo`.

**Uncertainties:** The `RECENTER` handler uses `flyTo` (line 263) while `recenterMap()` uses `easeTo` with a different zoom (15 vs 13.4). If these represent distinct UX intents (voice "recenter" vs button "recenter to device location"), the correction must preserve both behaviors.

---

### 3.3 `dispatchVoiceProposal` layer visibility side-effect not integration-tested

**File:** `frontend/v2/src/main.ts:1223–1226`

```typescript
if (action.type === 'SET_LAYER_VISIBILITY') {
  if (action.layer === 'RED_ZONES') redZonesVisible = action.visible;
  if (action.layer === 'SAFE_ZONES') relocationZonesVisible = action.visible;
  if (action.layer === 'ROUTES') routesVisible = action.visible;
  if (action.layer === 'MY_LOCATION') myLocationVisible = action.visible;
}
```

This updates UI state variables (`redZonesVisible`, etc.) that control CSS class bindings (line 1548–1549) and the layer-switcher button `aria-pressed` attributes. However, there is **no test that calls `dispatchVoiceProposal` and verifies the resulting `redZonesVisible`/`relocationZonesVisible`/`routesVisible`/`myLocationVisible` values** in main.ts state.

The existing `mapActions.test.ts` tests `executeMapActions` in isolation (which does NOT update these variables — it only calls `onLayerVisibility` callback). The bridge in `dispatchVoiceProposal` that propagates the action into main.ts UI state is untested.

**Proposed correction:** A unit or integration test calling `dispatchVoiceProposal` with `SET_LAYER_VISIBILITY` actions and asserting the state variables are updated correctly.

**Uncertainties:** This requires either (a) a DOM-based integration test that imports main.ts state, or (b) exposing the state variables via the `exposeWindowHelpers` test hook (`?sthira-test-hooks=1`).

---

### 3.4 `handleOpenPanelAction` `target_id` rejection not exercised end-to-end

**File:** `frontend/v2/src/main.ts:1152–1213`

The `handleOpenPanelAction` function (line 1152) calls `resolveChoiceAgainstGuidance(targetId, availableDestinations)` when `target_id` is present (line 1154). If the resolution fails, it sets `commandError` and returns `false` without opening a panel.

There is **no test that sends `OPEN_PANEL { panel: 'ROUTE_GUIDANCE', target_id: 'NON-EXISTENT-ID' }` through `dispatchVoiceProposal` and verifies:**
1. No panel is opened (`directionsOpen` stays false)
2. `commandError` is set with the expected message
3. The validation still allows panels without a `target_id`

**Proposed correction:** An integration test that calls `dispatchVoiceProposal` with an invalid `target_id` and asserts the rejection behavior, plus a valid `target_id` that succeeds.

**Uncertainties:** The `handleOpenPanelAction` function is only exposed via `exposeWindowHelpers` in dev mode (`?sthira-test-hooks=1`). A full end-to-end test requires the running app.

---

### 3.5 Repeated 3D toggle cycles — `mapTilted` / `aria-pressed` sync only tested once

**File:** `frontend/v2/src/main.ts:1787–1810` (toggleMapPerspective)

The `toggleMapPerspective` function and `syncPerspectiveControl` are not tested for consecutive toggle cycles. Specifically:

- After two `toggleMapPerspective()` calls, `mapTilted` should be `false` (back to original) and `aria-pressed` should match.
- The `toggle-3d` button's `is-active` CSS class (bound to `mapTilted`) and `aria-pressed` attribute (set by `syncPerspectiveControl`) could diverge if `syncPerspectiveControl` is called without updating `mapTilted`.

The `RECENTER` finding (3.2) is one path where this divergence occurs. Repeated UI clicking of the 3D toggle is a user-visible regression path.

**Proposed correction:** A test that clicks the 3D toggle button N times and asserts `mapTilted` and `aria-pressed` remain in sync after each click.

---

### 3.6 `executeMapActions` `onPanel` optional — documented behavior, not a gap

**File:** `frontend/v2/src/mapActions.ts:275–277`

```typescript
if (action.type === 'OPEN_PANEL') {
  onPanel?.(action.panel as Panel, action.target_id); // optional chain
}
```

`onPanel` is optional (`onPanel?:` in the function signature, line 191). When omitted, `OPEN_PANEL` silently does nothing — no exception, no side effect. This is by design (mapActions.test.ts:203–256 calls it with a stub). The existing tests verify this behavior. **Not a gap.**

---

## 4. Candidate Patch

**File:** plan/worker-reports/round-2/minimax-g/candidate.patch
**Base SHA-256 (mapActions.ts):** `a28f4f612416391283a1e832418681d98358437e43fa75be0803394558fdd490`

The candidate patch addresses finding **3.1** (removing the phantom `'my-location'` layer ID from `layerMap['MY_LOCATION']`) since this is the only unambiguous defect with a single-line surgical fix:

```diff
--- a/frontend/v2/src/mapActions.ts
+++ b/frontend/v2/src/mapActions.ts
@@ -181,7 +181,7 @@ export const layerMap: Record<Layer, string[]> = {
     'safe-zones',
   ],
   ROUTES: ['route-casing', 'approved-route', 'route-motion', 'routes'],
-  MY_LOCATION: ['device-pulse', 'device-point', 'my-location'],
+  MY_LOCATION: ['device-pulse', 'device-point'],
 };
```

If `'my-location'` was intended as a future UI layer, this removal creates a ponytail: to restore it, add a matching layer definition to `initMap` in main.ts. The current state is a phantom entry that causes a silent no-op on every MY_LOCATION visibility toggle.

Findings 3.2–3.5 require behavioral changes or more complex integration tests and are provided as **source-only recommendations** without a candidate patch at this time.

---

## 5. Verification

**Source inspection:** Performed — all four source files read in full; git history reviewed; commits 41a0278 and fc0128b reviewed completely to exclude duplication.

**Execution tests:** NOT_RUN — deferred by user (usage reset)

**Patch verification (MiniMax-G performed before handoff):**

The candidate.patch was applied with `patch -p1` and reversed with `patch -R -p1` — both succeeded. The patch removes the phantom `'my-location'` entry from `layerMap['MY_LOCATION']`, eliminating the silent no-op on every MY_LOCATION visibility toggle.

**Later verification commands for Opus after integration:**

```bash
# 1. Apply candidate.patch (verified to apply cleanly)
cd /Users/apple/Documents/Projects/MonitoringZ
patch -p1 < plan/worker-reports/round-2/minimax-g/candidate.patch

# 2. Confirm build passes
cd frontend/v2 && npm run build

# 3. Unit tests (mapActions)
cd frontend/v2 && node --test src/mapActions.test.ts

# 4. Browser verification for my-location phantom
# Start dev server
cd frontend/v2 && npm run dev
# Open browser, open DevTools console
# Inject: await window.setMyLocationVisible(true)
# Then: document.querySelector('[data-action="toggle-layers"]')?.click()
# Observe: device-pulse and device-point layers in map style
# should have visibility: visible; no layer named 'my-location' exists
```

**Expected negative control (for my-location phantom fix):**
Without the patch, `layerMap['MY_LOCATION']` contains `'my-location'` which maps to no style layer. `executeMapActions` calls `map.setLayoutProperty('my-location', 'visibility', 'visible')` — silently succeeds with no visible effect. After the patch, only real layers are toggled.

**For findings 3.2–3.5** (no patch — source-only recommendations):

```bash
# RECENTER / mapTilted desync verification
# Start app, complete onboarding, click 3D button to enable tilt
# Voice command: "recenter" (sends RECENTER action)
# Inspect: window.map.getPitch() should be 0
# Check: aria-pressed on [data-action="toggle-3d"] button should be "false"
# Before fix: aria-pressed stays "true" while pitch is 0
```

---

## 6. Status

**Status: READY_FOR_REVIEW_UNVERIFIED**

**Next action for Opus:**
1. Apply `candidate.patch` to `frontend/v2/src/mapActions.ts`
2. Confirm build (`cd frontend/v2 && npm run build`)
3. Run unit tests (`node --test src/mapActions.test.ts`)
4. Run Reticle/browse to confirm: voice `RECENTER` clears 3D button `aria-pressed`; repeated 3D toggles keep `aria-pressed` and `mapTilted` in sync; `SET_LAYER_VISIBILITY { MY_LOCATION }` toggles only `device-pulse` and `device-point` in the map style
5. For findings 3.2–3.5: assess whether behavioral fixes belong in this sprint or a later regression suite

**Critical blocker:** None for the candidate patch. Findings 3.2–3.5 (RECENTER/mapTilted sync, dispatchVoiceProposal layer side effects, handleOpenPanelAction target_id rejection, repeated 3D toggle) require behavioral changes and are documented as source-only recommendations. No execution was performed.
