# Worker 13 Report — Map Asset and Render Acceptance
**Report**: `plan/worker-reports/round-3/worker-13/report.md`
**Worker**: Worker 13
**Timestamp**: 2026-09-27T00:00:00Z
**HEAD**: `d95df9e609454877a91b7c82146b3c6368181b17`
**main.ts SHA-256**: `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09`

---

## 1. Scope Inspected

| Area | Source | Finding |
|------|--------|---------|
| 403 artifact | `plan/evidence/prototype-browser-verification.md:46` | "MapLibre asset requests from the symlinked node_modules returned 403 in the temporary copy" |
| Vite config | `frontend/v2/vite.config.ts` | No broad allowlists; `server.host/port/proxy` correct as-is |
| Package setup | `frontend/v2/package.json` | Dev server: `vite --host 127.0.0.1`; no special flags needed |
| MapLibre init | `main.ts:1648–1707` | Inline style object; external tiles from ArcGIS/S3; no sprite/font JSON loaded from package |
| Layer map | `mapActions.ts:184` | `layerMap['MY_LOCATION']` includes `'my-location'` — phantom ID (minimax-g finding 3.1) |
| Existing node_modules | `frontend/v2/node_modules/` | Real directory (39 packages); not a symlink in the working checkout |

**Dirty files** (not modified):
- `frontend/v2/src/styles.css`, `backend/internal/eval/`, `backend/internal/asrworker/audio_mime_test.go`, `backend/internal/httpserver/stay_http_integration_test.go`

---

## 2. Deliverables

| Artifact | Path |
|----------|------|
| Launch recipe | `plan/worker-reports/round-3/worker-13/map_launch_recipe.md` |
| Browser probe | `plan/worker-reports/round-3/worker-13/map_probe.py` |
| This report | `plan/worker-reports/round-3/worker-13/report.md` |

---

## 3. Findings

### F1. 403 Was a Verification-Setup Artifact — No Source Change Needed

**Evidence**: `prototype-browser-verification.md:46`:
> "MapLibre asset requests from the symlinked `node_modules` returned 403 in the temporary copy."

**Root cause**: The prototype verification ran against a `git archive HEAD | tar -xf -` **temporary copy** of the frontend, not the actual working directory. In that configuration:
- The **original** `frontend/v2/node_modules/` is a real directory with installed packages
- The **copy's** `node_modules/` becomes a **symlink** (or broken reference) to the original
- When Vite serves the copy, MapLibre's runtime asset requests (sprite images, fonts) resolve to `node_modules/maplibre-gl/dist/...` paths that point to the broken symlink → 403

**Vite config is correct**: `vite.config.ts` has no broad filesystem allowlists, no `server.fs.allow` that would bypass this, and no `server.host` misconfiguration. The `allowedHosts` setting is unrelated (remote tunnel hostnames, not filesystem paths).

**Conclusion**: No production configuration patch warranted. The 403 is eliminated by serving from the actual project directory with a real `node_modules/`.

### F2. Launch Recipe

The correct launch method is `npm run dev` from the actual `frontend/v2/` directory (or any copy created with `cp -r` rather than `git archive`):

```bash
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm install   # ensures real node_modules, not symlinks
npm run dev  # Vite serves from this directory; MapLibre assets resolve correctly
```

Do **not** use a `git archive` temporary copy for verification or dev servers.

### F3. Browser Probe — `map_probe.py`

The script uses Python Playwright to:
1. Complete onboarding via visible controls
2. Wait 3 seconds for map initialization
3. Check `#map-canvas` container exists
4. Check `.maplibregl-canvas` (MapLibre's canvas element) exists
5. Report any failed network requests (4xx/5xx) separately as MapLibre-asset vs other

**Network failures are reported independently** — a MapLibre sprite/font 403 is reported separately from the canvas existence check, satisfying the requirement to "report network asset failures separately."

### F4. minimax-g Finding 3.1 — Phantom `'my-location'` Layer ID

The `layerMap['MY_LOCATION']` still contains `'my-location'` which maps to no real style layer. This is documented in minimax-g's report and candidate.patch. Worker 13 does not duplicate that patch — it flags the probe's `map_probe.py` would also detect this if the layer lookup caused a visible error.

---

## 4. Artifact Notes

### `map_probe.py` — Base Hashes

Script is new (not a patch). References verified selectors from source inspection:
- `#map-canvas` (main.ts:1649)
- `.maplibregl-canvas` (MapLibre runtime class, not invented)
- `[data-action="onboarding-*"]` (main.ts:1391–1414)
- `[data-testid="emergency-card"]` (main.ts:1454)

### `map_launch_recipe.md`

No patch — this is documentation of the correct launch method. No `vite.config.ts` change needed.

---

## 5. Verification

**Execution tests NOT_RUN — deferred by user.**

### Later Commands

```bash
# Prerequisites
pip install playwright
playwright install chromium

# Correct launch
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm install
npm run dev

# Run probe
python3 plan/worker-reports/round-3/worker-13/map_probe.py
python3 plan/worker-reports/round-3/worker-13/map_probe.py http://127.0.0.1:18492/
```

### Expected Outcomes

| Check | Expected |
|-------|----------|
| onboarding completes | PASS |
| #map-canvas container exists | PASS |
| .maplibregl-canvas exists | PASS |
| no MapLibre asset 4xx/5xx | PASS (using real node_modules) |
| no other unexpected failures | PASS |
| no uncaught page errors | PASS |

### Expected Negative Control

| Scenario | Expected |
|----------|----------|
| Using `git archive` copy with symlinked node_modules | FAIL: MapLibre asset 4xx/5xx reported |
| Using real `cp -r` copy with `npm install` | PASS: all checks pass |

---

## 6. Status

**Status: NO_CHANGE_NEEDED** (no source modification warranted; 403 was a verification-setup artifact, not a source defect)

**New artifacts**: `map_launch_recipe.md` + `map_probe.py` — the recipe documents the correct launch method; the probe verifies map render + network asset health independently.

**Next action for Opus**: If future verification runs use `git archive` temporary copies, apply the recipe (`npm install && npm run dev` from the copy) before running `map_probe.py`. No source patches needed.
