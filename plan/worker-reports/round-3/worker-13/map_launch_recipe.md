# Map asset and render acceptance — launch recipe and browser probe

**Assignment**: Worker 13, Round 3
**Source**: prototype-browser-verification.md (line 46), vite.config.ts, package.json,
frontend/v2/src/main.ts (initMap), frontend/v2/src/mapActions.ts (layerMap)

---

## Root Cause of the 403

Prototype browser verification ran against a **`git archive HEAD` temporary copy** of the
frontend. In that configuration:

```
original-frontend/
  node_modules/  → real directory, contents installed

git archive HEAD | tar -xf -  →  copy-frontend/
  node_modules/  → **symlink** pointing back to original-frontend/node_modules
  (or empty if symlink couldn't resolve)
```

When Vite serves the copy, MapLibre loads its inline style (no external JSON).
However, MapLibre also requests assets from its package directory at runtime:
sprite images, fonts, and style references resolved relative to the `maplibre-gl` package
path. In the copy, those symlinked `node_modules` paths do not resolve correctly, causing
HTTP 403 on requests like `/node_modules/maplibre-gl/dist/sprite@2x.png`.

**This is a verification-setup artifact, not a source defect.** The working frontend
(`npm run dev` from the actual project directory) has a real `node_modules/` with
installed packages and MapLibre assets. No Vite config change, no filesystem allowlist,
and no `allow_origins` modification is needed.

---

## Launch Recipe

### Correct: serve from the actual project directory

```bash
# In terminal 1 — backend services (already documented elsewhere)
cd /Users/apple/Documents/Projects/MonitoringZ/backend
# start sthira-exercise, mock-workers per their instructions

# In terminal 2 — frontend dev server
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm install          # ensure node_modules are real files, not symlinks
npm run dev          # serves on http://127.0.0.1:5173
```

Vite reads `frontend/v2/node_modules/` directly. All MapLibre package assets
resolve to real files and return 200.

### Incorrect: serve from a git-archive copy or bare checkout

```bash
# DO NOT — causes 403 on MapLibre runtime assets
git archive HEAD | tar -xf - -C /tmp/frontend-copy
cd /tmp/frontend-copy/frontend/v2
npm run dev          # node_modules is a broken symlink in the copy
```

The `vite.config.ts` `server.host`/`port` and `proxy` settings are correct as-is.
The `allowedHosts` restriction is a production safety feature, not related to this issue.

---

## Browser Probe

`map_probe.py` — a Python Playwright script that:
1. Navigates to the app and completes onboarding
2. Waits for the map container and canvas to exist
3. Intercepts all failed network requests (4xx/5xx)
4. Separates MapLibre asset failures from other failures
5. Reports each class independently

Does not prove human map quality; labels network emulation as such.
