# Presenter Fallback Script — one paragraph per failure

**File:** `plan/evidence/presenter-fallback-script.md` · **Revision:** CLEAN `2d48319`

**SOURCE** = code citation. **OBSERVED(<commit or command>)** = a run I made. **PENDING** = planned or in
progress, not in this revision. **NOT_RUN** = never exercised. The one failure-state run behind (a)–(e) is
**OBSERVED(`fb36fe8`)**, 2026-09-27 ~21:10–21:20 IST, headless Chromium, owned synthetic stack:
127.0.0.1:18810 (`sthira-exercise`, fresh DB, migrations 0001–0010), :18811
(`mock-workers -scenario destination-choice`), :18812 (`npm run dev` proxying :18810); stopped and DB dropped
afterwards. It predates `ee0b83a`, so any behaviour below that `ee0b83a` changed is re-labelled SOURCE, never
re-labelled as observed. `c6d96c3..2d48319` = `ee0b83a`, `debc39e`, `1271257`, `3c28504`, `b15cc4a`, `2d48319`.

**Bring the stack up with `./tools/demo/demo.sh start` / `status` / `stop`** (`tools/demo/README.md`): mock
workers 127.0.0.1:18880, `sthira-exercise` :18881, Vite :18882, `DEMO_SCENARIO` default
`destination-choice`; scenarios `default`, `silent-zoom`, `destination-choice`, `arrival-confirm`, `clarify`,
`data-unavailable`, `worker-failure`; state and logs in `/tmp/sthira-demo/run/`. Do not hand-start a stack at
these ports for a demo. `--real-workers` reads `STHIRA_{ASR,MIDDLE,TTS}_URL` and optional `STHIRA_*_TOKEN`
from the shell; tokens are never printed, logged or stored. `DEMO_SCENARIO=worker-failure` is the cheapest way
to reach state (b) on purpose.

Banner copy comes from `runtimeCopy()` (`frontend/v2/src/main.ts:269`) fed by `evaluateReadinessState`
(`frontend/v2/src/audioGuidance.ts:474`); per-command failures set `commandError` in `sendVoiceOrText`
(`main.ts:1304`), never readiness. Never quote `commandUnavailable`, `backendUnavailable` or
`recognitionUnavailable`; they exist in `i18n.ts:32` but are never rendered. Localized copies are in
`i18n.ts` — read them there, never retype them.

## (a) Go backend stopped

**Banner (OBSERVED(`fb36fe8`)):** "Local interface, backend not connected" (`localDisconnected`,
`i18n.ts:31`); `/health/ready` is unreachable, so `evaluateReadinessState` returns `blocked`/`disconnected`
(`audioGuidance.ts:478`). **Command (OBSERVED(`fb36fe8`)):** "Voice Map Control is unavailable. The displayed
route and emergency call option still work." (`assistantUnavailable`, `i18n.ts:32`). Through the Vite dev proxy
the failed voice request surfaces as HTTP 500 — not 503. No page errors. Map toggles, 3D, language switch and
the 112 link with the backend down: **NOT_RUN** at every revision including `2d48319`.

## (b) mock-workers stopped, backend still up

**Readiness (SOURCE(`ee0b83a`)) — this changed; the older note is obsolete.** `WorkerHealthSummaries` now takes
`*orchestration.Workers` and takes `Ready` from the live per-stage flag `workers.IsReady(stage)`
(`backend/internal/httpserver/voice_wiring.go:66`, `backend/internal/orchestration/workers.go:227`) instead of
the latched last snapshot, and always lists all three stages (`voice_wiring.go:60`) so a dead stage cannot be
dropped from the count; `Warm` and `Languages` remain snapshot-derived and informational. `readiness.go` now
sets `allReady = false` in both models-NOT_READY branches — `no workers reporting`
(`readiness.go:208-213`) and `unready worker stages: <stage>(not ready)` (`readiness.go:214-220`) — so a dead
stage clears the top-level status. Regression test `backend/internal/httpserver/readiness_models_test.go`
(137 lines) fails if `readiness.go` is reverted. **Net presenter effect:** with the workers down,
`/health/ready` is `NOT_READY`, `models` detail is `unready worker stages: ...(not ready)` (or
`no workers reporting` if every stage is down), not `all 3 worker stages ready` as observed at `fb36fe8`. How
fast that flips is **NOT_RUN** — it depends on the worker's own death detection, and no interval is claimed
here. **Command (OBSERVED(`fb36fe8`)):** `POST /api/v3/voice/process` → 503 `MODEL_UNAVAILABLE`, "middle worker
not ready" (`backend/internal/orchestration/orchestrator.go:556`); the UI shows the same `assistantUnavailable`
line, and the next command returns 200 with the scenario template once the workers restart. The 503 "voice
pipeline is unavailable: model worker is not configured" (`voice_process.go:95`) is the *unconfigured* worker
case, not this one.

## (c) Browser offline

**Banner (OBSERVED(`fb36fe8`), Playwright `set_offline` + window `offline` event):** "Offline, saved demo
guidance" (`offline`, `i18n.ts:31`), set in the `offline` handler (`main.ts:2280`). No command error, no page
error. The banner clears on the window `online` event, which re-runs `checkRuntime` (`main.ts:2275`); no
recovery time is claimed, none was measured. Still true at `2d48319` (SOURCE).

## (d) Microphone permission denied

**Message (SOURCE, previously OBSERVED(`fb36fe8`)):** `commandError = words[language].micStopped` at
`main.ts:1104` → "Microphone input stopped. Check permission or enter a command below." (`micStopped`,
`i18n.ts:32`). Harness `recorder` check "i. permission denied: not listening, message shown"
(`plan/evidence/browser/prototype_accept.py:491`, description at `:484`) records no live track,
`aria-pressed="false"` and that message; PASS recorded at `329633b` in
`plan/evidence/prototype-browser-verification.md`. No latency is claimed. Recovery: grant permission in the
browser, then reload.

## (e) Autoplay blocked

**No error banner.** The clip is created but never plays; the citizen taps to listen. Harness `audio-denied`
checks "browser denied autoplay: clip created but never played, no error shown"
(`plan/evidence/browser/prototype_accept.py:571`) and tap-to-replay on the following line; PASS recorded at
`329633b`.

## Localized dialog and error text are drafts, not approved copy

**SOURCE(`3c28504`).** `frontend/v2/src/i18n.ts` carries the marker comment
`// HI/ML: DRAFT_REQUIRES_NATIVE_REVIEW` at lines 40, 80 and 120, covering the newly moved
reservation-confirm dialog (`confirmReservationTitle` … `noDestinationInfo`), the arrival dialog
(`arrivedSafely` … `needHelp`), the destination and alert-details rows, and the reservation/arrival/guidance
error messages (`stayLockedPrefix` … `unverifiedFacilityShortSuffix`, including `staleGuidance`,
`unconfirmedPrefix`/`unconfirmedSuffix`, `noStayReservation`, `arrivalRejected`, `arrivalNetworkError`).
English output is byte-identical after the move (commit message of `3c28504`; `frontend/v2/src/i18n.test.ts`
exists). Every Hindi or Malayalam string in those key groups is a **DRAFT** awaiting native review.

## Destination identity without coordinates is browser-proved

**SOURCE(`1271257`, `2d48319`).** `STHIRA_EXERCISE_SEED_COORDLESS=1` (process env, exercise binary only) seeds
a reservable `FACDEMO-2` with no coordinates. The `destination-identity` harness section now reserves FACDEMO-2
with GPS on FACDEMO-1's coordinates and requires no near-destination or arrival prompt, with a FACDEMO-1
positive control; a copy of the frontend with a shelter-coordinate fallback fails 3 checks
(`plan/evidence/browser/prototype_accept.py:827-858`). `plan/TODO.md:42` is `[x]`. Do not repeat the older
claim that this case is unit-tested only.

## What the presenter must not claim

- **The status line still does not react to workers dying mid-demo, but for a different reason now.** The
  envelope bug that made a healthy stack read "Demo ready, live sources not connected" is fixed. What remains
  is refresh timing, not readiness logic: `frontend/v2/src/main.ts` has exactly two `checkRuntime()` call
  sites — the bootstrap block (`main.ts:2250`) and the window `online` handler (`main.ts:2275`) — and **none
  after a voice command** (SOURCE). So even though `/health/ready` now honestly reports `NOT_READY` for a dead
  stage, the line keeps its last value until something else re-checks. A refresh after a failed or successful
  command is **PENDING** (planned with Worker 2) and **NOT_RUN**. **SOURCE:** the status-line keys are
  `responding` "Synthetic demo, system responding", `blocked` "Demo ready, live sources not connected",
  `localDisconnected` "Local interface, backend not connected", `offline` "Offline, saved demo guidance",
  `checking` "Checking system readiness" (`i18n.ts:31`); a failed command sets `commandError` instead
  (`micStopped` / `assistantUnavailable`, `i18n.ts:32`). If the workers die mid-scenario, **the failed command
  itself is the only honest signal at that moment — do not promise the line will change.** Do not cite
  `plan/evidence/prototype-browser-verification.md` for worker readiness: its "Current" section is the
  14-section / 106 PASS run from before `ee0b83a` and its "Defect 2 (latched worker readiness)" is now FIXED
  by `ee0b83a`.

- **Do not demo a real-model voice pipeline as trustworthy yet.** The nine known middle-worker defects are
  **OPEN** (SOURCE(`debc39e`)): nine `TestDefect_*` tests in `backend/internal/middleworker/lifecycle_test.go`
  reproduce them against fake adapters and skip unless `LC_DEFECT_REPRO=1`, with all nine failing when enabled —
  dead vLLM endpoint still READY; runtime panic kills the process; dispatch racing shutdown panics; empty
  transcript yields an OK proposal; empty language admitted; checksum-less artifact advertised; timeout
  counter; drain status; non-idempotent LoadAndVerify. Fake adapters fix nothing; `plan/prompt.md` (CURRENT
  EXECUTOR HANDOFF — 2026-09-27 23:35) says the demo-relevant ones come next and Worker 1 is fixing them, so
  treat them as **PENDING**, not fixed. Practical consequence: with real workers the middle stage can report
  READY while the vLLM endpoint is dead, and an empty transcript can produce an OK proposal. None of this
  applies to the default synthetic stack (`DEMO_SCENARIO` with `mock-workers`), which is what a demo runs.

- **Do not present Hindi or Malayalam wording as approved.** The dialog and error key groups moved in `3c28504`
  are marked `DRAFT_REQUIRES_NATIVE_REVIEW` (`i18n.ts:40, :80, :120`). No human language evaluation is
  recorded; English is unchanged.

- **NOT_RUN at `2d48319`:** real microphone, audible quality, Safari, real models, true 200 % zoom, real tab
  backgrounding, and any visible control exercised while the backend, the workers or the network are down.
  `plan/evidence/prototype-browser-verification.md:50-58` explains the first two gaps are not provable in
  headless Chromium at all: `document.hidden` cannot be forced true there, and `device_scale_factor=2` scales
  rasterisation without reflow, which is not browser zoom. Both need a human at a real browser.
