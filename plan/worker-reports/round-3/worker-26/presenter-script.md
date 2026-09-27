# Worker 26 — Mock Demo Presenter Script (Five-Minute Rehearsal Draft)

**Worker:** worker-26
**Timestamp:** 2026-09-27T02:40:00Z
**HEAD:** d95df9e609454877a91b7c82146b3c6368181b17
**Output directory:** plan/worker-reports/round-3/worker-26/

---

## Prior Evidence Used

- `plan/evidence/prototype-presenter-guide.md` §3 (existing five-minute script, `6861d42` base, corrected at `41a0278`)
- `plan/evidence/prototype-browser-verification.md` (browser evidence at `e066056` / `fc0128b`)
- `plan/evidence/prototype-presenter-guide.md` §2 (capability table, evidence classifications)

---

## Presenter Script: Five-Minute Mock Demo

> **Prerequisite:** Start `mock-workers -scenario destination-choice` before the demo. Keep the scenario unchanged throughout. The mock ASR always returns `"Meppadi"` regardless of spoken input. Do not promise audio quality or model accuracy.

---

### Before starting — say this:

> *"In this demo the AI models are simulated. The mock-worker returns pre-configured responses regardless of what is spoken or typed. This exercises the API contracts and UI logic. The same interface would connect to real models in an authorized inference environment."*

---

### 0:00–0:20 — Launch

- Open `http://127.0.0.1:5173` in the browser
- Onboarding screen with **Begin** button

---

### 0:20–0:40 — Language selection

- Tap **Begin** → select **Hindi** → **Continue**
- All labels switch to Hindi

**Evidence:** IMPLEMENTED — SOURCE INSPECTED (frontend unit tests PASS at `e066056`)

---

### 0:40–1:10 — Map controls

- Tap **3D toggle**, **red zones**, **relocation zones**
- These are local map-only operations; no server call

**Evidence:** IMPLEMENTED — SOURCE INSPECTED; browser acceptance PASS at `e066056` (no horizontal overflow, no console errors)

---

### 1:10–1:40 — Voice command (mock scenario)

- With `destination-choice` scenario running, tap **mic** or text input, or tap a suggestion chip
- Expected: the `FACDEMO-1` choice appears on the map with its caption

> **What to expect from mock behavior:**
> - The mock ASR returns `"Meppadi"` regardless of what you say or type
> - The mock middle worker returns the **configured scenario** (`SHOW_CHOICES FACDEMO-1`), not a response to the actual spoken words
> - Audio may or may not be present depending on the mock TTS path — **do not promise audio quality**
> - The browser applies the proposal only when its `data_version` matches the displayed guidance (`PKGDEMO-1:1`)

> **Fallback (if backend unavailable):** An error banner appears. Map controls, 3D toggle, language switch, and emergency call continue to work.

**Evidence:** Mock plumbing curl PASS at `e066056` — proposals carry correct `data_version=PKGDEMO-1:1`; guidance matches

---

### 1:40–2:10 — Destination chip

- The chip shows `FACDEMO-1` from the guidance query result
- **Note:** In the synthetic demo, the illustrative facility cannot be reserved (local `buildReservationPayload` check refuses illustrative IDs before sending any server request)

---

### 2:10–2:40 — Reservation flow

- Tap **Start safe route**
- UI shows **"Reserving..."** — this is a **pending state**, not immediate success
- Server returns **201 Created** → UI shows "Route active" badge and opens the directions panel
- Server returns **409/422** → error message; no capacity was moved (snapshot version mismatch is rejected atomically, without mutation)

> **Key point to narrate:** *"The reservation only succeeds after the server acknowledges it. The citizen sees a pending state, and only confirmed reservations appear as active."*

**Evidence:** IMPLEMENTED — SOURCE INSPECTED (`evaluateArrivalConfirmation` requires `serverResponse.ok`); browser acceptance PASS at `e066056` for the full reserve-then-arrive flow; idempotent replay TEST VERIFIED at `6861d42`

---

### 2:40–3:00 — Arrival confirmation

- With an active reservation, tap **Confirm arrival**
- UI shows a confirmation prompt or spinner
- Server returns **200 OK** → arrival is recorded
- **No success state without server acknowledgement**

> **Key point to narrate:** *"Arrival is recorded only after the server confirms it. The system does not assume arrival from GPS proximity alone — the citizen confirms and the server validates."*

**Evidence:** IMPLEMENTED — SOURCE INSPECTED (`evaluateArrivalConfirmation` requires `serverResponse.ok`); browser acceptance PASS at `e066056` (arrival recorded after server acknowledgement)

---

### 3:00–3:30 — Language switch

- Tap **EN** in the language switcher
- All labels switch to English; `audioGuard.invalidate()` prevents replay of old audio

> **What to expect:** The previously generated audio is not replayed after a language switch — the guard invalidates it.

**Evidence:** IMPLEMENTED — SOURCE INSPECTED (`AudioPlaybackGuard`; `audioGuard.invalidate()`); browser acceptance PASS at `e066056`

---

### 3:30–4:00 — Backend unavailable

- Stop the Go backend (or stop `sthira-exercise`)
- Error banner: **"Voice Map Control is not connected yet"** (or similar)
- **Layer toggles, 3D, language switch, emergency call `tel:112` continue to work**
- Voice commands, reservations, and arrival confirmation fail

> **Fallback narration:** *"The map controls and emergency call work without the backend. Voice commands and reservations require the server — this is the designed degradation path."*

**Evidence:** IMPLEMENTED — SOURCE INSPECTED (`evaluateReadinessState`; `runtime === 'blocked'/'offline'`); browser outage/recovery PASS at `e066056` (503 on worker stop; 200 on restart)

---

### 4:00–4:30 — Source details

- Tap **Source details** (or equivalent panel trigger)
- Shows: DEMO-EXERCISE / PKGDEMO-1, freshness indicator, date range
- Explicit **"Synthetic demo package"** notice is displayed

**Evidence:** IMPLEMENTED — SOURCE INSPECTED; browser acceptance PASS at `e066056`

---

### 4:30–5:00 — Summary narration

> *"This prototype demonstrates a voice-guided evacuation interface over authority-supplied guidance. It runs in a synthetic exercise configuration. Real government integration requires operational source agreements, authorized inference infrastructure, and human language evaluation before quality claims in any language."*

---

## Honest Q&A Notes (for presenter preparation)

| Question | Honest answer |
|----------|--------------|
| Are you predicting hazards? | No. The prototype renders synthetic exercise data. Operational hazard data comes from government sources. |
| Who approves safe zones? | The architecture supports government-authorized sources. The prototype uses synthetic data. |
| Does GPS prove physical safety? | No. GPS shows device location with accuracy limits. Arrival requires explicit server acknowledgement. |
| Does it work offline? | Partially. Map tiles require internet. Layer toggles, language switch, and emergency call work without backend. |
| Are AI models actually running? | No — this demo uses `mock-workers` with pre-generated responses. Real inference requires an authorized GPU session. |
| Which parts have humans evaluated? | No human language evaluation (ASR accuracy, TTS intelligibility) has been recorded for Hindi or Malayalam. |

---

## Evidence Classification Reference

| Item | Evidence class |
|------|---------------|
| Map, voice console, language switch, emergency dial | IMPLEMENTED — SOURCE INSPECTED (unit tests PASS; browser acceptance PASS at `e066056`) |
| Reservation → arrival with server acknowledgement | IMPLEMENTED — SOURCE INSPECTED; browser journey PASS at `e066056` |
| Idempotent writes, stale-version rejection | TEST VERIFIED (Go integration tests pass; idempotency + stale-snapshot tests at `6861d42`) |
| Real ASR / Sarvam / TTS pipeline | NOT_RUN (AWS GPU instance stopped; no current real inference) |
| Human speech quality review | NOT_RUN (no Hindi/Malayalam evaluation recorded) |

---

## What to Avoid Claiming

- Government approval or live partnership
- Map rendering quality (map tiles require internet; tile fetching verified as artifact-only in current setup)
- Native mobile app (no Android/iOS build verified)
- Human-reviewed language quality (no evaluation recorded)
- Real model accuracy or latency (no real inference performed)
- Operational deployment readiness

---

## Status

**Status: NO_CHANGE_NEEDED** (document creation only — no production source modification)

**Artifact:** `plan/worker-reports/round-3/worker-26/presenter-script.md` — five-minute rehearsal draft

**Integration risk:** LOW — this is a documentation artifact only; does not modify production source or tests

**Next action for Opus:** Review `presenter-script.md` against the current UI in the integrated frontend before the presentation. Refresh final integrated commit from `git log` on `CLEAN` branch after integration is confirmed.
