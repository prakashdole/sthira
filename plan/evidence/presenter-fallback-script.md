# Presenter Fallback Script — one paragraph per failure

**File:** `plan/evidence/presenter-fallback-script.md` · **Revision:** CLEAN `fb36fe8`

**OBSERVED** = my run, 2026-09-27 ~21:10–21:20 IST, headless Chromium, owned synthetic stack: 127.0.0.1:18810
(`sthira-exercise`, fresh DB, migrations 0001–0010), :18811 (`mock-workers -scenario destination-choice`),
:18812 (`npm run dev` proxying :18810); stopped and DB dropped afterwards. **SOURCE** = code citation only. **NOT_RUN** = not exercised.
Banner copy comes from `runtimeCopy()` (`frontend/v2/src/main.ts:269`) fed by `evaluateReadinessState`
(`audioGuidance.ts:474`); per-command failures set `commandError` in `sendVoiceOrText` (`main.ts:1303`), never readiness.
Never quote `commandUnavailable`, `backendUnavailable` or `recognitionUnavailable`; localized copies are in
`i18n.ts` (ML 68–69, HI 105–106) — read them there, never retype them.

## (a) Go backend stopped

**Banner (OBSERVED):** "Local interface, backend not connected" (`localDisconnected`, `i18n.ts:31`); `/health/ready` is
unreachable, so readiness is blocked/disconnected (`audioGuidance.ts:478`). **Command (OBSERVED):** "Voice Map Control is
unavailable. The displayed route and emergency call option still work." (`assistantUnavailable`, `i18n.ts:32`). Through the
Vite dev proxy the failed voice request surfaces as HTTP 500 — not 503. No page errors. Map toggles, 3D, language switch and
the 112 link with the backend down: NOT_RUN here.

## (b) mock-workers stopped, backend still up

**Banner:** unchanged from healthy — since the readiness fix (see below) "Synthetic demo, system responding"; before it, "Demo ready, live sources not connected" (OBSERVED at `fb36fe8`). Either way it does not react to the dead workers: `/health/ready` still returns 200
`READY`, `models: "all 3 worker stages ready"`: the refresh loop marks a stage unready in a boolean but leaves the last
snapshot (`backend/internal/orchestration/workers.go:211`, `:247`) which readiness then reports
(`backend/internal/httpserver/voice_wiring.go:60`). **Command (OBSERVED):** `POST /api/v3/voice/process` → 503
`MODEL_UNAVAILABLE`, "middle worker not ready"; the UI shows the same `assistantUnavailable` line, and the next command
returns 200 with the scenario template once the workers restart. The 503 "voice pipeline is unavailable: model worker is not configured"
(`voice_process.go:95`) is the *unconfigured* worker case, not this one.

## (c) Browser offline

**Banner (OBSERVED, Playwright `set_offline` + window `offline` event):** "Offline, saved demo guidance" (`offline`,
`i18n.ts:31`), set at `main.ts:2278`. No command error, no page error. The banner clears on the window `online` event, which
re-runs `checkRuntime` (`main.ts:2272`); no recovery time is claimed, none was measured.

## (d) Microphone permission denied

**Message (SOURCE, previously OBSERVED):** `commandError = words[language].micStopped` at `main.ts:1103` → "Microphone input
stopped. Check permission or enter a command below." (`i18n.ts:32`). Harness `recorder` check "i. permission denied: not
listening, message shown" (`plan/evidence/browser/prototype_accept.py:438`) records no live track, `aria-pressed="false"`
and that message; PASS at `329633b` in `prototype-browser-verification.md`. No latency is claimed. Recovery: grant
permission in the browser, then reload.

## (e) Autoplay blocked

**No error banner.** The clip is created but never plays; the citizen taps to listen. Harness `audio-denied` checks "browser
denied autoplay: clip created but never played, no error shown" and tap-to-replay (`prototype_accept.py:518`, `:523`); PASS
at `329633b`.

## What the presenter must not claim

- **The status line is not a worker-health indicator.** The envelope bug that made a healthy stack show "Demo ready,
live sources not connected" is fixed (`checkRuntime` now reads `data.status`; harness `core` check). But with the workers
dead the backend still reports 200 `READY` (latched snapshot, above), so the line keeps saying "system responding". Only a
failed command reveals the outage.

- **NOT_RUN:** real microphone, audible quality, Safari, real models, true 200 % zoom, real tab backgrounding, and any
visible control exercised while the backend, the workers or the network are down — see `prototype-browser-verification.md`.
