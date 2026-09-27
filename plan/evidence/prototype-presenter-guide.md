# Sthira Prototype Presenter Guide

**File:** `plan/evidence/prototype-presenter-guide.md`
**Inspected HEAD:** `fb36fe8` (CLEAN); created at `6861d42`, corrected at `41a0278`, banner and failure
behaviour re-verified against code and one owned browser run at `fb36fe8`.
**Evidence labels:** SOURCE INSPECTED = code read; OBSERVED = run described in
`presenter-fallback-script.md` (headless Chromium, ports 18810–18812, 2026-09-27); NOT_RUN = not
exercised. Real-model inference NOT_RUN. Failure-behaviour paragraphs and exact banner strings live in
`plan/evidence/presenter-fallback-script.md`; this guide does not repeat them.

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
| MapLibre GL satellite map with overlays | **IMPLEMENTED — SOURCE INSPECTED** | `main.ts:initMap` (1731); `mapData.json` |
| Red zone / relocation zone toggles | **IMPLEMENTED — SOURCE INSPECTED** | `SET_LAYER_VISIBILITY` dispatch; `updateMapDynamicProperties` (1675) |
| Voice console with mic / text input | **IMPLEMENTED — SOURCE INSPECTED** | `toggleLocalRecording` (1086), `sendVoiceOrText` (1303) |
| Language selection EN / ML / HI | **IMPLEMENTED — SOURCE INSPECTED** | `language` state; `speechLanguageTag` (109); i18n `words` object |
| `POST /api/v3/voice/process` | **IMPLEMENTED — SOURCE INSPECTED** | `main.ts:1324`; `voice_process.go:44` |
| Audio playback with integrity check | **IMPLEMENTED — SOURCE INSPECTED** | `verifyAudioIntegrity`; SHA-256 checksum |
| Audio replay invalidation on language/version change | **IMPLEMENTED — SOURCE INSPECTED** | `AudioPlaybackGuard`; `audioGuard.invalidate()` |
| Destination selection from guidance | **IMPLEMENTED — SOURCE INSPECTED** | `mapGuidanceDestinations` (`journey.ts:439`) |
| `resolveChoiceAgainstGuidance` matches facility_id, or a safe_zone_id that identifies exactly one facility | **UNIT TESTED** | `journey.ts` / `journey.test.ts` (ambiguous safe zone is refused, never first-match) |
| Reservation with snapshot_version binding | **IMPLEMENTED — SOURCE INSPECTED** | `buildReservationPayload` (`journey.ts:530`); `validateGuidanceSnapshot` (`journey.ts:383`) |
| Stale reservation rejection (409, no capacity mutation) | **TEST VERIFIED** | `prototype_scenarios_test.go:440–450` in `TestProtoSnapshotVersionTwoReservationAndStaleNoMutation`; suite run at `41a0278` (238 PASS, `prototype-browser-verification.md`) |
| Idempotent replay (no double capacity mutation) | **TEST VERIFIED** | `prototype_scenarios_test.go:467–476`, same test and run |
| Arrival confirmation with server acknowledgement | **IMPLEMENTED — SOURCE INSPECTED** | `evaluateArrivalConfirmation` (`journey.ts:291`); requires `serverResponse.ok` |
| No-arrival without stay (404) | **SOURCE INSPECTED — no test asserts it** | `store.ErrStayNotFound` → 404 `NOT_FOUND` (`stay_handlers.go:467`); the only arrival test is the 200 happy path (`stay_http_integration_test.go:450`) |
| GPS proximity evaluation | **IMPLEMENTED — SOURCE INSPECTED** | `evaluateProximity`; 150m threshold; 100m accuracy cap; 30s staleness |
| `evaluateArrivalConfirmation` vs `transitionOnArrival` | Distinction: `evaluateArrivalConfirmation` (`journey.ts:291`) handles server response, state checks, and retry logic; `transitionOnArrival` (`journey.ts:229`) is a pure state transition function with no server interaction | Both exist; different callers |
| Session token in Authorization header | **IMPLEMENTED — SOURCE INSPECTED** | `authHeaders` (258); `Bearer` token |
| Offline banner (`navigator.onLine`) | **IMPLEMENTED — SOURCE INSPECTED** | `checkRuntime` (520); `window.addEventListener('offline', …)` (2278) |
| Readiness status line | **FIXED — BROWSER-CHECKED** | `checkRuntime` now passes `data` (the envelope payload) to `evaluateReadinessState` (`audioGuidance.ts:474`); a healthy stack shows "Synthetic demo, system responding" (harness `core`; the same check fails on the previous code). Still blind to dead workers: `/health/ready` stays 200 `READY` (`workers.go:211`/`:247`) |
| Per-command failure message | **IMPLEMENTED — SOURCE INSPECTED** | `assistantUnavailable` set only in `sendVoiceOrText` (1348, 1385, 1395) — never by readiness. `commandUnavailable`, `backendUnavailable` and `recognitionUnavailable` exist in `i18n.ts` but are referenced nowhere and are never rendered |
| Language tag passed from UI (no auto-detection) | **IMPLEMENTED — SOURCE INSPECTED** | `speechLanguageTag(language)` → `hi-IN/en-IN/ml-IN` in pipeline request |
| Real ASR worker | **IMPLEMENTED — SOURCE INSPECTED** | `asrworker/` package; code exists |
| Real middle worker | **IMPLEMENTED — SOURCE INSPECTED** | `middleworker/` package; `SarvamModelID = sarvamai/sarvam-30b` in `sarvam_config.go` |
| Real TTS worker | **IMPLEMENTED — SOURCE INSPECTED** | `ttsworker/` package |
| Mock-workers pipeline | **IMPLEMENTED — SOURCE INSPECTED** | `cmd/mock-workers`; 7 configurable scenarios |
| Real-model end-to-end | **NOT_RUN** | AWS instance last recorded stopped (2026-09-27, `real-inference-launch-check.md`) |
| Safari browser automation | **NOT_RUN** | `safaridriver` requires user-enabled Develop menu option |
| ISL guidance panel | **IMPLEMENTED — SOURCE INSPECTED** | UI exists; "pending review" notice |
| Emergency dial `tel:112` | **IMPLEMENTED — SOURCE INSPECTED** | `triggerEmergencyDial`; browser telephony link |

---

## 3. Five-Minute Demonstration Script

> **Prerequisite:** See the maintained launch procedure in `plan/prompt.md`. Do not duplicate it here. The presenter should start the configured mock-worker scenario before the demo and keep the scenario stable throughout.

### Mock Mode Notice (say before starting)
*"In this demo, the AI models are simulated. The mock-worker returns pre-configured responses regardless of what is spoken or typed. This exercises the API contracts and UI logic. The same UI would connect to real models."*

### Recommended scenario: `destination-choice`
This returns `SHOW_CHOICES FACDEMO-1`. Whether audio is attached depends on the mock TTS path; do not promise audio in the mock demo. Start with:
```
mock-workers -scenario destination-choice -port <PORT>
```

**Do not depend on speech recognition.** The mock ASR always returns `"Meppadi"` / Malayalam text regardless of input.

---

**0:00–0:20 — Launch**

- Open `http://127.0.0.1:5173` in Safari
- Onboarding screen with Begin button

**0:20–0:40 — Language**

- Tap Begin → HI → Continue
- Labels switch to Hindi

**0:40–1:10 — Map controls**

- 3D toggle, layer toggles (red zones, relocation zones)
- These are local; no server call

**1:10–1:40 — Voice command (mock scenario)**

- With `destination-choice` scenario running:
  - Tap mic or text input, press a suggestion button
  - Expected: the FACDEMO-1 choice is shown with its caption. OBSERVED (EN, `fb36fe8`): the command
    returns 200 and the template text "Destination choices are displayed on screen." is shown; hi-IN and
    ml-IN were not re-observed in that pass
  - The mock returns `SHOW_CHOICES [FACDEMO-1]` regardless of what was typed, and the browser applies it
    only when the proposal `data_version` equals the displayed guidance (`PKGDEMO-1:1`)
- If the workers are down the command shows the same unavailable message and no map change; the status
  line does not move (`presenter-fallback-script.md` §b)

**1:40–2:10 — Destination chip**

- Shows FACDEMO-1 from guidance query
- Illustrative facilities cannot be reserved (local `buildReservationPayload` check)

**2:10–2:40 — Reservation flow**

- Tap **Start safe route**
- UI shows "Reserving..." (a hardcoded literal at `main.ts:1601`, not an i18n string; pending state, not
  immediate success)
- On 201: "Route active" badge, directions panel
- On 409/422: error message; `buildReservationPayload` rejects illustrative or stale versions locally before the fetch

**2:40–3:00 — Arrival**

- Tap **Confirm arrival**
- `evaluateArrivalConfirmation` requires `activeStayId` from sessionStorage and `serverResponse.ok`
- No success state until server 200

**3:00–3:30 — Language switch**

- Tap EN in language switcher
- All labels switch; `audioGuard.invalidate()` prevents replay of old audio

**3:30–4:00 — Backend unavailable (rehearse this, do not improvise)**

- Stop the Go backend
- Status line: `localDisconnected` — "Local interface, backend not connected" (OBSERVED)
- A submitted command shows `assistantUnavailable` — "Voice Map Control is unavailable. The displayed
  route and emergency call option still work." (OBSERVED). There is no 503: through the Vite dev proxy
  the failed request returns 500
- Layer toggles, 3D, language switch, emergency call still functional
- Do not quote `commandUnavailable` ("Voice Map Control is not connected yet …"); it is never rendered

**4:00–4:30 — Source details**

- Tap Source details
- Shows DEMO-EXERCISE / PKGDEMO-1, freshness, dates
- Explicit notice: "This screen uses a synthetic demo package. It is not an active government alert."
  (`demoNotice`)

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
No — the demo runs `mock-workers` with pre-generated responses. Real model execution requires an authorized GPU session on AWS. See `plan/evidence/real-inference-launch-check.md` for the preflight checklist.

**Which parts have humans evaluated?**
No human language evaluation (ASR transcript accuracy, TTS intelligibility) has been recorded for any language. Malayalam rows in the worksheet require native-speaker review before PASS.

---

## 5. Real-Speech Worksheet (NOT_RUN until evidence supplied)

All rows remain NOT_RUN. Human evaluation required for HI and ML before any claim of ASR/TTS quality.

---

## 6. Corrections Made

| Issue | Original | Corrected |
|---|---|---|
| Evidence classification | "Implemented — verified" for browser features | "IMPLEMENTED — SOURCE INSPECTED" (source only) or "TEST VERIFIED" (actual test result) or "NOT_RUN" |
| Inspected baseline | `13a8037` | `fb36fe8` (actual HEAD); created at `6861d42`, corrected at `41a0278` |
| Reservation timing | "Route active appears immediately" | Shows "Reserving..." pending state; `routeStarted=true` only after `res.ok` (`main.ts:878`) |
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
| Backend down vs workers down | Both described as "503" and as one `assistantUnavailable` banner | Backend down → `localDisconnected` status line, unreachable `/health/ready`, HTTP 500 through the Vite proxy. Workers down → `/health/ready` stays 200 `READY` (latched health snapshot, `workers.go:211`/`:247`), only the voice request is 503 `MODEL_UNAVAILABLE`. OBSERVED both |
| Which key shows on failure | `assistantUnavailable` attributed to readiness; `commandUnavailable` quoted as the banner | `assistantUnavailable` is set only by a command attempt (`main.ts:1348/1385/1395`); `commandUnavailable`, `backendUnavailable` and `recognitionUnavailable` are never rendered |
| Status line as a health indicator | Presented `blocked` as a live-sources warning | Envelope bug fixed (healthy → "system responding"). It still does not detect dead workers (latched readiness); only a failed command does |
| No-arrival without stay | Marked TEST VERIFIED via `prototype_scenarios_test.go` | That file has no arrival case; 404 comes from `store.ErrStayNotFound` (`stay_handlers.go:467`) and is SOURCE INSPECTED only |
| `commandUnavailable` quote | "Voice Map Control is not connected yet" (truncated value) | Full EN value is "Voice Map Control is not connected yet. The visible map controls still work." — and it is never rendered; do not say it on stage |
| Line references | 6 of 8 `main.ts`/`journey.ts` line numbers stale | Re-verified: `initMap` 1731, `sendVoiceOrText` 1303, `checkRuntime` 520, `authHeaders` 258, `buildReservationPayload` journey.ts:530, `routeStarted` 878, tts cache key `contracts/tts.go:86-99` |
| Destination identity | Not represented | A coordinate-less destination is covered by a unit test and by the harness `destination-identity` section (`840d8ea`), which never asserts absence of proximity output; the browser claim stays unproven (`plan/TODO.md` §B) |

---

*Guide corrected at `fb36fe8` against code and one owned browser run. Unstaged. Documentation only — no
runtime acceptance claimed; real-model inference, Safari, real microphone and audible quality NOT_RUN.*
