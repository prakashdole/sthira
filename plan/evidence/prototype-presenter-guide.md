# Sthira Prototype Presenter Guide

**File:** `plan/evidence/prototype-presenter-guide.md`
**Inspected revision:** `2d48319` (CLEAN). Revision history: created at `6861d42`; corrected at `41a0278`;
banner and failure behaviour re-verified against code and one owned browser run at `fb36fe8`; **this revision
reflects `2d48319` code** — `c6d96c3..2d48319` = `ee0b83a`, `debc39e`, `1271257`, `3c28504`, `b15cc4a`,
`2d48319`. Rows that describe pre-`ee0b83a` behaviour are pinned to `fb36fe8` rather than deleted.
**Line numbers** below are read from the committed tree at `2d48319`. The working copy currently carries
uncommitted in-flight work (middle-worker fixes and a `main.ts` change), so line numbers there can differ; if
one does, trust the committed revision named, not the offset.
**Evidence labels:** SOURCE = code or commit citation read for this revision; OBSERVED(<commit or command>) =
a run described in `presenter-fallback-script.md` (the failure-state run is OBSERVED(`fb36fe8`), headless
Chromium, ports 18810–18812, 2026-09-27 — it predates `ee0b83a`, so anything `ee0b83a` changed is relabelled
SOURCE, never as observed); PENDING = planned or in progress, not in this revision; NOT_RUN = not exercised.
Real-model inference NOT_RUN. Failure-behaviour paragraphs and exact banner strings live in
`plan/evidence/presenter-fallback-script.md`; this guide does not repeat them.
**Do not cite** `plan/evidence/prototype-browser-verification.md` for worker readiness: its "Current" section is
the 14-section / 106 PASS run from **before** `ee0b83a` and its "Defect 2 (latched worker readiness)" is now
FIXED. If its figures are quoted at all, name the revision they describe (`fb36fe8`).

---

## 1. Architecture — Plain Language

### What the prototype demonstrates

The prototype demonstrates how authority-supplied guidance would be validated and presented through a voice interface. This exercise uses synthetic data; operational sources and approvals remain pending.

A citizen opens the web app, selects a language, optionally grants location, and sees a map with synthetic evacuation overlays. They can speak a command or use the text input; the prototype shows how the UI would apply approved guidance and record a reservation.

### Client / server division

**Browser (TypeScript):**
- Renders map via MapLibre GL, tracks GPS, records audio via `MediaRecorder`
- Maintains UI state only; never writes directly to PostgreSQL
- Applies `validateGuidanceSnapshot`, `buildReservationPayload`, `resolveChoiceAgainstGuidance` as **additional UI safeguards**
- The **security boundary** is the Go backend: PostgreSQL writes require a server acknowledgement via `/api/v3/reservations` or `/api/v3/reservations/{id}/events`

**Go backend:**
- Handles all persistent state: sessions, reservations, stays, event log
- Validates model proposals via `Orchestrator.validateProposal` (`orchestrator.go:1043`) — the server-side enforcement that prevents unauthorized actions regardless of client state
- Validates template/audio via `Orchestrator.stageTemplate` (`orchestrator.go:594`)
- Returns structured `VoiceProposal` responses; the model cannot invoke reservation or arrival endpoints directly

### Registered HTTP endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v3/sessions` | Public | Creates citizen session, returns Bearer token |
| `POST` | `/api/v3/places/resolve` | Public | Ambiguity-aware place lookup; 409 with candidates on multiple matches |
| `POST` | `/api/v3/guidance/query` | Public | Eligible destinations (read-only) |
| `POST` | `/api/v3/reservations` | Session | Creates reservation + stay; idempotency-keyed |
| `GET` | `/api/v3/reservations/{id}` | Session | Read own reservation |
| `POST` | `/api/v3/reservations/{id}/events` | Session | arrive/cancel/depart/extend/transfer |
| `POST` | `/api/v3/voice/transcriptions` | Session | Audio → transcript |
| `POST` | `/api/v3/voice/process` | Session | Full pipeline |
| `POST` | `/api/v3/voice/speech` | Session | Template → audio |
| `GET` | `/health/ready` | Public | Readiness probe |

Routes: `server.go:333–352` (mux), `voice_process.go:44–46` (voice route constants).

### What `/health/ready` reports about the model workers

SOURCE(`ee0b83a`): the `models` subsystem reads the **live per-stage ready flag** the orchestrator gates dispatch
on — `workers.IsReady(stage)` (`backend/internal/httpserver/voice_wiring.go:66`,
`backend/internal/orchestration/workers.go:227`) — not the last health snapshot, which a dead worker keeps
forever. All three stages (`StageASR`, `StageMiddle`, `StageTTS`) are always listed, so a dead stage cannot be
dropped from the count; `Warm` and `Languages` remain snapshot-derived and informational
(`voice_wiring.go:60–71`). `readiness.go:208–220` sets `allReady = false` in **both** models-NOT_READY branches
(`no workers reporting` and `unready worker stages: <stage>(not ready)`), so a dead stage clears the top-level
status. Regression test `backend/internal/httpserver/readiness_models_test.go` fails if `readiness.go` is
reverted (commit message of `ee0b83a`). How fastly that flips after a worker dies is **NOT_RUN** — it depends on
the worker's own death detection and no interval is claimed here.

### Backend proposal validation (security boundary)

`Orchestrator.validateProposal` (`orchestrator.go:1043`) enforces:
- Schema version matches
- Request ID correlation (proposal must echo server request ID)
- Data version alignment with server snapshot
- Status enum: `OK | CLARIFY | UNSUPPORTED | DATA_UNAVAILABLE | ERROR` only
- Intent required for OK; null for non-OK
- Actions required for OK; empty for non-OK
- `CLARIFY` requires non-empty `clarification_ids`; other statuses require empty
- Action count ≤ 5 (`MaxModelActions = 5`); clarification IDs ≤ 3 (`MaxClarificationIDs = 3`)
- Per-action type shape enforcement (forbidden extra fields rejected)

This validation runs **server-side** before any response is returned. Frontend validation (`validateVoiceResponse` in `mapActions.ts`) is a secondary UI safeguard only.

### Backend guarantees for reservations

- **Atomic writes** — `store.InTx` wraps reservation and event creation
- **Idempotency keys** — `POST /reservations` and `POST /events` accept `idempotency_key`; replay returns original result without re-mutating capacity
- **Snapshot versioning** — reservations embed `snapshot_version`; stale-version requests (bound to older version than currently published) receive 409 Conflict **without** mutating capacity. Existing committed reservations are unaffected. A version bump does not invalidate them.
- **Database driver** — `github.com/jackc/pgx/v5` (`go.mod:5`)

### Voice pipeline (mock vs real)

```
Browser (MediaRecorder, webm/opus)
  → POST /api/v3/voice/process
    → Orchestrator.Run()
      → ASR worker [mock-workers: returns fixed transcript; real: IndicConformer ONNX]
      → Middle worker [mock-workers: returns configured scenario JSON; real: Sarvam-30B via vLLM]
         Request config: MaxOutputTokens=512 (orchestrator.go:567)
         Output: validated by validateProposal (orchestrator.go:1043)
      → Template stage [Orchestrator.stageTemplate, orchestrator.go:594]
         Validates language binding, template approval, digest, versions
      → TTS worker [mock-workers: returns pre-generated WAV; real: Indic Parler-TTS]
          Worker output is PCM 16-bit mono WAV at the native sample rate reported by the TTS adapter. 22050 Hz is a legacy fallback; confirm the actual rate from runtime readiness and generated WAV metadata during real-model verification.
          Cache key: sha256 hex of the JSON encoding of the TTSCacheKey struct (SpeechKey, Language, Args, SourceVersion, TemplateVersion, TemplateSHA256, ModelRevision, VoiceRevision, Settings) — `backend/internal/contracts/tts.go:86-99`, implemented in `backend/internal/ttsworker/cache.go`
```

**Mock note:** In mock mode, the ASR returns a fixed transcript (`"Meppadi"` / Malayalam text) regardless of what the citizen actually says. The mock middle worker returns the **configured scenario** (default/silent-zoom/destination-choice/arrival-confirm/clarify/data-unavailable/worker-failure), not a response to arbitrary speech. Audio is pre-generated WAV.

### Map-only operations (no persistent write)

`SET_LAYER_VISIBILITY`, `ZOOM`, `PAN`, `RECENTER`, `FOCUS_FEATURE`, `HIGHLIGHT_FEATURE`, `FIT_FEATURES`, `OPEN_PANEL`

### Persistent write operations

`POST /api/v3/reservations` → reservation + stay; `POST /api/v3/reservations/{id}/events` with `type: ARRIVE` → arrival record

---

## 2. Capability Table — Accurate Classifications

| Feature | Classification | Evidence |
|---|---|---|
| MapLibre GL satellite map with overlays | **IMPLEMENTED — SOURCE** | `main.ts:initMap` (1732); `mapData.json` |
| Red zone / relocation zone toggles | **IMPLEMENTED — SOURCE** | `SET_LAYER_VISIBILITY` dispatch; `updateMapDynamicProperties` (1676) |
| Voice console with mic / text input | **IMPLEMENTED — SOURCE** | `toggleLocalRecording` (1087), `sendVoiceOrText` (1304) |
| Language selection EN / ML / HI | **IMPLEMENTED — SOURCE** | `language` state; `speechLanguageTag` (109); i18n `words` object |
| `POST /api/v3/voice/process` | **IMPLEMENTED — SOURCE** | `main.ts:1325`; `voice_process.go:44` |
| Audio playback with integrity check | **IMPLEMENTED — SOURCE** | `verifyAudioIntegrity`; SHA-256 checksum |
| Audio replay invalidation on language/version change | **IMPLEMENTED — SOURCE** | `AudioPlaybackGuard`; `audioGuard.invalidate()` |
| Destination selection from guidance | **IMPLEMENTED — SOURCE** | `mapGuidanceDestinations` (`journey.ts:439`) |
| `resolveChoiceAgainstGuidance` matches facility_id, or a safe_zone_id that identifies exactly one facility | **UNIT TESTED** | `journey.ts` / `journey.test.ts` (ambiguous safe zone is refused, never first-match) |
| A destination with no coordinates never implies proximity to another shelter, and a real reservation is still created | **BROWSER-VERIFIED — SOURCE(`1271257`)** | `STHIRA_EXERCISE_SEED_COORDLESS=1` seeds a reservable coordinate-less `FACDEMO-2`; harness `destination-identity` reserves it with GPS on `FACDEMO-1`'s coordinates and requires no near/arrival prompt, with a `FACDEMO-1` positive control; a copy of the frontend with a shelter-coordinate fallback fails 3 checks (`plan/evidence/browser/prototype_accept.py:807–860`). `plan/TODO.md:42` is `[x]` (set by `2d48319`). No longer unit-only |
| Reservation with snapshot_version binding | **IMPLEMENTED — SOURCE** | `buildReservationPayload` (`journey.ts:530`); `validateGuidanceSnapshot` (`journey.ts:383`) |
| Stale reservation rejection (409, no capacity mutation) | **TEST VERIFIED** | `prototype_scenarios_test.go:440–450` in `TestProtoSnapshotVersionTwoReservationAndStaleNoMutation`; suite run at `41a0278` (238 PASS, `prototype-browser-verification.md`) |
| Idempotent replay (no double capacity mutation) | **TEST VERIFIED** | `prototype_scenarios_test.go:467–476`, same test and run |
| Arrival confirmation with server acknowledgement | **IMPLEMENTED — SOURCE** | `evaluateArrivalConfirmation` (`journey.ts:291`); requires `serverResponse.ok` |
| No-arrival without stay (404) | **SOURCE — no test asserts it** | `store.ErrStayNotFound` → 404 `NOT_FOUND` (`stay_handlers.go:467`); the only arrival test is the 200 happy path (`stay_http_integration_test.go:450`) |
| GPS proximity evaluation | **IMPLEMENTED — SOURCE** | `evaluateProximity`; 150m threshold; 100m accuracy cap; 30s staleness |
| `evaluateArrivalConfirmation` vs `transitionOnArrival` | Distinction: `evaluateArrivalConfirmation` (`journey.ts:291`) handles server response, state checks, and retry logic; `transitionOnArrival` (`journey.ts:229`) is a pure state transition function with no server interaction | Both exist; different callers |
| Session token in Authorization header | **IMPLEMENTED — SOURCE** | `authHeaders` (258); `Bearer` token |
| Offline banner (`navigator.onLine`) | **IMPLEMENTED — SOURCE** | `checkRuntime` (520); `window.addEventListener('offline', …)` (2279) |
| Readiness status line — envelope | **FIXED — BROWSER-CHECKED** | `checkRuntime` now passes `data` (the envelope payload) to `evaluateReadinessState` (`audioGuidance.ts:474`); a healthy stack shows "Synthetic demo, system responding" (`responding`, `i18n.ts:31`; harness `core` — the same check fails on the previous code) |
| Readiness status line — worker death | **SOURCE(`ee0b83a`), not visible on the line** | `/health/ready` now reports a dead stage honestly (`models` → `unready worker stages: <stage>(not ready)`, top status `NOT_READY`; §1). The line itself still does not move: `main.ts` has exactly two `checkRuntime()` call sites — `main.ts:2250` (bootstrap) and `main.ts:2275` (window `online`) — and **none after a command**. So a mid-demo worker death is visible on the *next command* (503 `MODEL_UNAVAILABLE`, `assistantUnavailable`), not on the status line. A refresh after a failed or successful command is **PENDING** (planned with Worker 2) and **NOT_RUN** |
| Per-command failure message | **IMPLEMENTED — SOURCE** | `assistantUnavailable` set only in `sendVoiceOrText` (1349, 1386, 1396) — never by readiness. `commandUnavailable`, `backendUnavailable` and `recognitionUnavailable` exist in `i18n.ts` but are referenced nowhere and are never rendered |
| Language tag passed from UI (no auto-detection) | **IMPLEMENTED — SOURCE** | `speechLanguageTag(language)` → `hi-IN/en-IN/ml-IN` in pipeline request |
| Real ASR worker | **IMPLEMENTED — SOURCE** | `asrworker/` package; code exists |
| Real middle worker | **IMPLEMENTED — SOURCE** | `middleworker/` package; `SarvamModelID = sarvamai/sarvam-30b` in `sarvam_config.go` |
| Real TTS worker | **IMPLEMENTED — SOURCE** | `ttsworker/` package |
| Mock-workers pipeline | **IMPLEMENTED — SOURCE** | `cmd/mock-workers`; 7 configurable scenarios; brought up by `tools/demo/demo.sh` (`b15cc4a`) |
| Known middle-worker defects (real workers only) | **OPEN / PENDING — SOURCE(`debc39e`)** | Nine `TestDefect_*` tests in `backend/internal/middleworker/lifecycle_test.go` reproduce them against fake adapters and skip unless `LC_DEFECT_REPRO=1`; with it set, all nine fail. Verbatim from the commit message: dead vLLM endpoint still READY; runtime panic kills the process; dispatch racing shutdown panics; empty transcript yields an OK proposal; empty language admitted; checksum-less artifact advertised; timeout counter; drain status; non-idempotent LoadAndVerify. `plan/prompt.md` (CURRENT EXECUTOR HANDOFF — 2026-09-27 23:35) puts the demo-relevant ones next, Worker 1 is fixing them. **Do not claim any is fixed.** None of this applies to the default synthetic stack (`mock-workers`) a demo runs. Distinct from `ee0b83a`, which is about the *per-stage ready flag*, not these internal defects — neither implies the other is resolved |
| HI/ML wording in the dialogs and error messages | **DRAFT, PENDING native review — SOURCE(`3c28504`)** | `// HI/ML: DRAFT_REQUIRES_NATIVE_REVIEW` at `i18n.ts:40`, `:80`, `:120`, covering the reservation-confirm dialog (`confirmReservationTitle` … `noDestinationInfo`), the arrival dialog (`arrivedSafely` … `needHelp`), the destination and alert-details rows, and the reservation/arrival/guidance error messages (`stayLockedPrefix` … `unverifiedFacilityShortSuffix`, including `staleGuidance`, `unconfirmedPrefix`/`unconfirmedSuffix`, `noStayReservation`, `arrivalRejected`, `arrivalNetworkError`). English output is byte-identical after the move (commit message). A separate claim from the ASR/TTS quality gap: no human language evaluation is recorded either way |
| Real-model end-to-end | **NOT_RUN** | AWS instance last recorded stopped (2026-09-27, `real-inference-launch-check.md`) |
| Safari browser automation | **NOT_RUN** | `safaridriver` requires user-enabled Develop menu option |
| ISL guidance panel | **IMPLEMENTED — SOURCE** | UI exists; "pending review" notice |
| Emergency dial `tel:112` | **IMPLEMENTED — SOURCE** | `triggerEmergencyDial`; browser telephony link |

---

## 3. Five-Minute Demonstration Script

> **Prerequisite:** See the maintained launch procedure in `plan/prompt.md`. Do not duplicate it here. The presenter should start the configured mock-worker scenario before the demo and keep the scenario stable throughout.

### Bring-up — use `demo.sh`, not a hand-started stack (SOURCE(`b15cc4a`))

```sh
./tools/demo/demo.sh start    # build, DB, mock workers + exercise + Vite, wait READY
./tools/demo/demo.sh status   # per-component pid/port/URL, database, /health/ready
./tools/demo/demo.sh stop     # kill only this run's pids, drop only its database
```

127.0.0.1 only: mock workers **18880** (`DEMO_WORKER_PORT`), `sthira-exercise` **18881** (`DEMO_BACKEND_PORT`),
Vite **18882** (`DEMO_UI_PORT` — this is the browser URL, not 5173). `DEMO_SCENARIO` defaults to
`destination-choice`. State and logs in `/tmp/sthira-demo/run/` (`state.env` holds pids, db name and
`started_at` — never a token or DSN). `--real-workers` reads `STHIRA_{ASR,MIDDLE,TTS}_URL` and optional
`STHIRA_*_TOKEN` from the shell; tokens are never printed, logged or stored. `DEMO_SCENARIO=worker-failure` is
the cheapest way to reach the workers-down state on purpose.

### Mock Mode Notice (say before starting)
*"In this demo, the AI models are simulated. The mock-worker returns pre-configured responses regardless of what is spoken or typed. This exercises the API contracts and UI logic. The same UI would connect to real models."*

### Recommended scenario: `destination-choice` (the `demo.sh` default)
This returns `SHOW_CHOICES FACDEMO-1`. Whether audio is attached depends on the mock TTS path; do not promise audio in the mock demo. Do not hand-start `mock-workers -scenario … -port …`; `./tools/demo/demo.sh start` already runs it on 18880 with this scenario. Scenarios available: `default`, `silent-zoom`, `destination-choice`, `arrival-confirm`, `clarify`, `data-unavailable`, `worker-failure`.

**Do not depend on speech recognition.** The mock ASR always returns `"Meppadi"` / Malayalam text regardless of input.

---

**0:00–0:20 — Launch**

- Open `http://127.0.0.1:18882` (SOURCE: `tools/demo/demo.sh:31`; the old 5173 Vite default is not what `demo.sh` starts)
- Onboarding screen with Begin button

**0:20–0:40 — Language**

- Tap Begin → HI → Continue
- Labels switch to Hindi. The Hindi and Malayalam wording in the dialogs and error messages is a **DRAFT**
  awaiting native review (`DRAFT_REQUIRES_NATIVE_REVIEW`, `i18n.ts:40`, `:80`, `:120`) — do not call it approved copy

**0:40–1:10 — Map controls**

- 3D toggle, layer toggles (red zones, relocation zones)
- These are local; no server call

**1:10–1:40 — Voice command (mock scenario)**

- With `destination-choice` running:
  - Tap mic or text input, press a suggestion button
  - Expected: the FACDEMO-1 choice is shown with its caption. The caption is the backend's own
    `destination_options` template text for the selected language (`backend/internal/orchestration/registry.go:116`
    for `en-IN`) — it is a template registry string, **not** an i18n key, so do not quote it from memory
  - OBSERVED(`fb36fe8`, EN): the command returned 200 and that template text was shown
  - hi-IN and ml-IN are now covered by the browser harness section `hi-ml` (SOURCE(`1271257`)): it submits a
    command in each language and compares the rendered text **byte-for-byte against the backend's own
    `data.template.text`**, so the check cannot pass by asserting an English string; it then reserves and records
    an arrival through the visible controls in both languages. The integrated run recorded with `1271257` was
    **77 PASS, 0 FAIL** (commit message). No number beyond that is claimed
  - The mock returns `SHOW_CHOICES [FACDEMO-1]` regardless of what was typed, and the browser applies it
    only when the proposal `data_version` equals the displayed guidance (`PKGDEMO-1:1`)
- If the workers are down the command shows the same unavailable message and no map change; the status
  line does not move (SOURCE — only two `checkRuntime()` call sites, none after a command; a post-command
  refresh is PENDING/NOT_RUN). `presenter-fallback-script.md` §b

**1:40–2:10 — Destination chip**

- Shows FACDEMO-1 from guidance query
- Illustrative facilities cannot be reserved (local `buildReservationPayload` check)
- If asked about a shelter that arrives without coordinates: that destination is still reservable and never
  borrows another facility's position — browser-proved by the `destination-identity` section
  (`1271257`, `plan/TODO.md:42`), not unit-only

**2:10–2:40 — Reservation flow**

- Tap **Start safe route** (`startRoute`, `i18n.ts:11`)
- UI shows "Reserving..." — this is the `reserving` i18n key (`i18n.ts:41`, used at `main.ts:1602` and
  `:1640`), **not** a hardcoded literal; a pending state, not immediate success
- On 201: "Route active" badge (`routeActive`, `i18n.ts:11`), directions panel
- On 409/422: error message; `buildReservationPayload` rejects illustrative or stale versions locally before the fetch

**2:40–3:00 — Arrival**

- The arrival dialog opens with the heading **Has everyone arrived safely?** (`arrivedSafely`, `i18n.ts:35`) and
  a party-size stepper; the confirm button reads **Confirm arrival now** (`confirmArrivalPrompt`, `i18n.ts:39`).
  The secondary button is **Call help** (`callHelp`). Do not say "We have arrived": that key has been renamed in the
  working tree and is not the string on screen at `2d48319`.
- `evaluateArrivalConfirmation` requires `activeStayId` from sessionStorage and `serverResponse.ok`
- No success state until server 200

**3:00–3:30 — Language switch**

- Tap EN in language switcher
- All labels switch; `audioGuard.invalidate()` prevents replay of old audio

**3:30–4:00 — Backend unavailable (rehearse this, do not improvise)**

- Stop the Go backend (leave Vite up; `./tools/demo/demo.sh stop` stops the whole stack, which ends the demo)
- Status line: `localDisconnected` — "Local interface, backend not connected" (`i18n.ts:31`) — OBSERVED(`fb36fe8`)
- A submitted command shows `assistantUnavailable` — "Voice Map Control is unavailable. The displayed
  route and emergency call option still work." (`i18n.ts:32`) — OBSERVED(`fb36fe8`). There is no 503: through
  the Vite dev proxy the failed request returns 500
- Layer toggles, 3D, language switch, emergency call with the backend down: **NOT_RUN** at every revision
  including `2d48319` — do not promise it live; say it is designed to work locally
- Do not quote `commandUnavailable` ("Voice Map Control is not connected yet. The visible map controls still
  work."); it is never rendered

**4:00–4:30 — Source details**

- Tap Source details
- Shows DEMO-EXERCISE / PKGDEMO-1, freshness, dates
- Explicit notice: "This screen uses a synthetic demo package. It is not an active government alert."
  (`demoNotice`, `i18n.ts:27`)

**4:30–5:00 — Summary narration**
*"This prototype demonstrates a voice-guided evacuation interface over authority-supplied guidance. It runs in a synthetic exercise configuration. Real government integration requires operational source agreements and an authorized inference environment."*

---

## 4. Judge Q&A — Honest Answers

**Are you predicting hazards?**
No. The prototype renders synthetic exercise data. Operational hazard data would come from government sources; this demo uses synthetic zones/routes only.

**Who approves safe zones and routes in the system?**
The architecture supports government-authorized sources. The prototype demo uses synthetic data. Operational approval workflows are not yet integrated.

**Does GPS prove physical safety?**
No. GPS shows device location. Accuracy limits (100m cap) and proximity thresholds (150m) are conservative estimates, not safety guarantees. Arrival confirmation requires explicit server acknowledgement.

**Does it work offline?**
Partially. Map tiles require the internet (Esri ArcGIS). Layer toggles, language switch, and emergency call work without backend. Voice commands and reservations require the backend.

**Are the AI models actually running?**
No — the demo runs `mock-workers` with pre-generated responses (`./tools/demo/demo.sh start`, scenario `destination-choice`). Real model execution requires an authorized GPU session on AWS. See `plan/evidence/real-inference-launch-check.md` for the preflight checklist.

**If the real models were running, would you trust them?**
Not for this demo, and here is exactly why. Nine known middle-worker defects are still **OPEN / PENDING**
(`debc39e`): with real workers the middle stage can report READY while the vLLM endpoint is dead, and an empty
transcript can produce an OK proposal. None of that applies to the synthetic stack a demo runs. The `ee0b83a`
readiness fix is about the *per-stage ready flag* in `/health/ready` — it does not fix these internal defects,
and neither claim should be used to imply the other is resolved.

**Does the system notice a model worker that dies mid-demo?**
The backend does: since `ee0b83a`, `/health/ready` reports the dead stage (`NOT_READY`, `unready worker stages:
<stage>(not ready)`) instead of a latched READY. The **status line in the UI does not re-check** — `main.ts`
calls `checkRuntime()` only at startup and on the window `online` event, never after a command — so the honest
signal at that moment is the failed command itself. A post-command refresh is PENDING and NOT_RUN.

**What happens if a shelter has no coordinates?**
Nothing is invented. A destination the authority returns without coordinates is still offered and reservable, and
it never borrows another facility's position: the browser harness reserves the coordinate-less `FACDEMO-2` with
GPS standing on `FACDEMO-1` and requires no near-destination or arrival prompt, with a `FACDEMO-1` positive
control (`1271257`; a build with a shelter-coordinate fallback fails 3 checks). This is browser-proved, not
unit-only.

**Which parts have humans evaluated?**
Two separate answers, and neither is favourable.
1. **ASR/TTS quality:** no human language evaluation (ASR transcript accuracy, TTS intelligibility) has been
   recorded for any language. Malayalam rows in the worksheet require native-speaker review before PASS.
2. **On-screen Hindi/Malayalam copy:** the dialog and error wording moved into i18n in `3c28504` is marked
   `DRAFT_REQUIRES_NATIVE_REVIEW` (`i18n.ts:40`, `:80`, `:120`). It is a draft, not approved copy. English
   output is unchanged.

---

## 5. Real-Speech Worksheet (NOT_RUN until evidence supplied)

All rows remain NOT_RUN. Human evaluation required for HI and ML before any claim of ASR/TTS quality.

Keep the two language claims apart: a native-speaker review of *what is written* in the dialogs (the
`3c28504` drafts) does not supply the missing evidence for *what is heard* (ASR accuracy, TTS intelligibility),
and neither substitutes for the other.

---

## 6. Corrections Made

| Issue | Original | Corrected |
|---|---|---|
| Evidence classification | "Implemented — verified" for browser features | "IMPLEMENTED — SOURCE" (code read) or "TEST VERIFIED" (actual test result) or "OBSERVED(<commit>)" (a run) or "PENDING" / "NOT_RUN" |
| Inspected baseline | `13a8037` | `2d48319` (current CLEAN HEAD); created at `6861d42`, corrected at `41a0278`, browser re-verified at `fb36fe8`. Rows describing pre-`ee0b83a` behaviour are pinned to `fb36fe8`, not deleted |
| Reservation timing | "Route active appears immediately" | Shows "Reserving..." pending state; `routeStarted=true` only after `res.ok` (`main.ts:879`) |
| Local validation vs server error | Treating local `buildReservationPayload` rejection as producing HTTP 409/422 | Local rejection occurs before fetch; server rejection produces actual HTTP status |
| Authority framing | Implied live government integration | Explicit: synthetic exercise data; operational sources pending |
| `evaluateArrivalConfirmation` attribution | Attributed to `transitionOnArrival` | Clarified: `evaluateArrivalConfirmation` (journey.ts:291) handles server response + state; `transitionOnArrival` (journey.ts:229) is pure state transition |
| TTS sample rate | "44,100 Hz WAV" | Worker output is PCM 16-bit mono WAV at the native sample rate reported by the TTS adapter. 22050 Hz is a legacy fallback; confirm actual rate from runtime readiness and generated WAV metadata during real-model verification |
| TTS device placement | Stated as fact | Not freshly verified for the current integrated revision; confirm during authorized real-model session |
| 512 token limit | "enforcing: max 512 tokens" | Request configuration (MaxOutputTokens=512, orchestrator.go:567), not verified output behavior |
| Thinking/prose suppression | Stated as enforced | Desired, not verified; no runtime evidence of compliance |
| Cache key composition | `template_key + language + data_version` | Actual: sha256 hex of the JSON encoding of the full TTSCacheKey struct (`backend/internal/contracts/tts.go:86-99`) including SpeechKey, Language, Args, SourceVersion, TemplateVersion, TemplateSHA256, ModelRevision, VoiceRevision, Settings |
| Mock-worker behavior | Implied speech understanding | ASR returns fixed transcript; middle returns configured scenario only |
| `stageTemplate` path | Cited as `orchestration/template.go` | Actually `Orchestrator.stageTemplate` in `orchestrator.go:594` |
| `validateProposal` path | Cited as `orchestration/validate.go` | Actually `Orchestrator.validateProposal` in `orchestrator.go:1043` |
| Backend down vs workers down (as observed at `fb36fe8`) | Both described as "503" and as one `assistantUnavailable` banner | Backend down → `localDisconnected` status line, unreachable `/health/ready`, HTTP 500 through the Vite proxy. Workers down → **at `fb36fe8`** `/health/ready` stayed 200 `READY` (latched health snapshot, `workers.go:211`/`:247`), only the voice request was 503 `MODEL_UNAVAILABLE`. OBSERVED(`fb36fe8`) — the workers-down half is superseded by `ee0b83a`, see the next row |
| Which key shows on failure | `assistantUnavailable` attributed to readiness; `commandUnavailable` quoted as the banner | `assistantUnavailable` is set only by a command attempt (`main.ts:1349/1386/1396`); `commandUnavailable`, `backendUnavailable` and `recognitionUnavailable` are never rendered |
| Status line as a health indicator (as observed at `fb36fe8`) | Presented `blocked` as a live-sources warning | Envelope bug fixed (healthy → "Synthetic demo, system responding"). At `fb36fe8` it still could not detect dead workers (latched readiness); only a failed command did |
| `commandUnavailable` quote | "Voice Map Control is not connected yet" (truncated value) | Full EN value is "Voice Map Control is not connected yet. The visible map controls still work." — and it is never rendered; do not say it on stage |
| Line references | 6 of 8 `main.ts`/`journey.ts` line numbers stale | Re-verified: `initMap` 1731, `sendVoiceOrText` 1303, `checkRuntime` 520, `authHeaders` 258, `buildReservationPayload` journey.ts:530, `routeStarted` 878, tts cache key `contracts/tts.go:86-99`. Re-verified again at `2d48319`, where `3c28504` moved the dialog strings: `initMap` 1732, `updateMapDynamicProperties` 1676, `toggleLocalRecording` 1087, `sendVoiceOrText` 1304, fetch `/voice/process` 1325, `assistantUnavailable` 1349/1386/1396, `offline` listener 2279, `checkRuntime` call sites 2250 and 2275, `t.reserving` 1602 and 1640, `routeStarted` 879 |
| Destination identity (first pass) | Not represented | A coordinate-less destination is covered by a unit test and by the harness `destination-identity` section (`840d8ea`), which never asserts absence of proximity output; the browser claim stayed unproven (`plan/TODO.md` §B) — kept as history; superseded by the next row |
| No-arrival without stay | Marked TEST VERIFIED via `prototype_scenarios_test.go` | That file has no arrival case; 404 comes from `store.ErrStayNotFound` (`stay_handlers.go:467`) and is SOURCE only |
| Worker readiness in `/health/ready` (at `fb36fe8`) | Readiness summarised each stage's last health snapshot, and an unready models subsystem never cleared the top-level status, so a dead worker read READY | FIXED by `ee0b83a`: `Ready` comes from the live `workers.IsReady(stage)` the orchestrator gates dispatch on, all three stages are always listed, and both models-NOT_READY branches set `allReady = false` (`voice_wiring.go:55–73`, `readiness.go:208–220`). `backend/internal/httpserver/readiness_models_test.go` fails if `readiness.go` is reverted. How fast it flips is NOT_RUN |
| Status line vs dead workers | Implied the status line would show a dead worker once readiness was fixed | The line still does not re-check: `main.ts` has exactly two `checkRuntime()` call sites (2250 bootstrap, 2275 window `online`) and none after a command. A post-command refresh is PENDING (planned with Worker 2) and NOT_RUN |
| Demo bring-up | Told the presenter to open `http://127.0.0.1:5173` and hand-start `mock-workers -scenario destination-choice -port <PORT>` | `./tools/demo/demo.sh start` / `status` / `stop` (`b15cc4a`): mock workers 18880, `sthira-exercise` 18881, Vite **18882**; `DEMO_SCENARIO` default `destination-choice`; state and logs in `/tmp/sthira-demo/run/` |
| "Reserving..." as a hardcoded literal | "UI shows 'Reserving...' (a hardcoded literal at `main.ts:1601`, not an i18n string)" | It is the `reserving` i18n key (`i18n.ts:41`, used at `main.ts:1602` and `:1640`) and is present in the EN, ML and HI blocks |
| Arrival button name | "Tap **Confirm arrival**" | The arrival dialog is headed **Has everyone arrived safely?** (`arrivedSafely`, `i18n.ts:35`) and its confirm button reads **Confirm arrival now** (`confirmArrivalPrompt`, `i18n.ts:39`); the secondary button is **Call help** (`callHelp`). "We have arrived" is not a button label at `2d48319` — that key was renamed in the working tree |
| Quoted `destination_options` template text | Quoted "Destination choices are displayed on screen." as a UI string | It is not in `i18n.ts` at all — it is the backend template registry text (`backend/internal/orchestration/registry.go:116`). Never quote it from memory; read the registry |
| hi-IN / ml-IN template text | "not re-observed in that pass" | Browser-proved by harness section `hi-ml` (`1271257`): the rendered text is compared byte-for-byte with the backend's own `data.template.text` in both languages. Integrated run recorded with that commit: 77 PASS, 0 FAIL |
| Destination identity (current) | "browser claim stays unproven (`plan/TODO.md` §B)" | Browser-proved (`1271257`, `2d48319`): `STHIRA_EXERCISE_SEED_COORDLESS=1` seeds a reservable coordinate-less `FACDEMO-2`; the `destination-identity` section reserves it with GPS on `FACDEMO-1` and requires no near/arrival prompt, with a positive control at `FACDEMO-1`; a shelter-coordinate fallback fails 3 checks. `plan/TODO.md:42` is `[x]` |
| HI/ML dialog and error wording | Not represented; the i18n move was absent | DRAFT, not approved copy (`3c28504`): `DRAFT_REQUIRES_NATIVE_REVIEW` at `i18n.ts:40`, `:80`, `:120` covers the reservation-confirm dialog, the arrival dialog, the destination/alert-details rows and the reservation/arrival/guidance error messages. English output byte-identical. Separate from the ASR/TTS quality gap — do not merge the two claims |
| Nine middle-worker defects | Implied real-model readiness was trustworthy | OPEN / PENDING (`debc39e`): nine `TestDefect_*` tests, `LC_DEFECT_REPRO=1` to enable, all nine fail. Per `plan/prompt.md` (2026-09-27 23:35) Worker 1 is fixing the demo-relevant ones. Do not claim any is fixed |
| Local controls with the backend down | "Layer toggles, 3D, language switch, emergency call still functional" stated as observed | NOT_RUN at every revision including `2d48319`; designed to work locally, never exercised with the backend down |
| Worker-readiness evidence citation | Cited `plan/evidence/prototype-browser-verification.md` as current | That document's "Current" section is the 14-section / 106 PASS run from before `ee0b83a` and its "Defect 2" is now FIXED; quote its figures only with the revision they describe |

---

*Guide corrected at `2d48319` against code in `c6d96c3..2d48319` (`ee0b83a`, `debc39e`, `1271257`, `3c28504`,
`b15cc4a`, `2d48319`) plus one owned browser run at `fb36fe8`. Unstaged. Documentation only — no runtime
acceptance claimed; real-model inference, Safari, real microphone, audible quality, post-command status-line
refresh, and every visible control exercised while the backend, the workers or the network are down are
NOT_RUN.*
