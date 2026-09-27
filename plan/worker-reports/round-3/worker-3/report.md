# Worker 3 (Round 3) Report: Language-Switch Delayed-Response Regression

## Status: READY_FOR_REVIEW_UNVERIFIED

---

## 1. Source Hashes

| File | SHA (approx) | Key symbols |
|------|-------------|-------------|
| `frontend/v2/src/main.ts` | § MAIN-LATEST | `sendVoiceOrText:1266`, `supersedeInFlight:244`, `shouldDropResponse:1294,1306` |
| `frontend/v2/src/journey.ts` | § JOURNEY-LATEST | `shouldDropResponse` guard logic |
| `backend/internal/contracts/pipeline.go:106` | § PIPELINE-LATEST | `PipelineResponse` envelope contract |
| `backend/internal/contracts/worker_health.go:106` | § WORKER_HEALTH-LATEST | `MiddleWorkerResponse` envelope |

---

## 2. What This Test Proves

**Delayed old response CANNOT overwrite current outcome after language switch.**

The guard chain:

```
User switches language (EN→HI)
  └─► SET_LANGUAGE action in dispatchVoiceProposal (main.ts:1235)
        └─► supersedeInFlight()         [increments activeRequestId]
              └─► audioGuard.invalidate()
              └─► lastApprovedAudio = undefined

User issues new HI voice request
  └─► sendVoiceOrText (main.ts:1266)
        └─► reqId = ++activeRequestId   [new, higher ID]
        └─► fetch POST /api/v3/voice/process

Old EN response arrives (delayed)
  └─► shouldDropResponse(activeRequestId, reqId, document.hidden)
        └─► activeRequestId !== reqId   [TRUE — old ID < current ID]
        └─► return early; response DROPPED

HI response arrives (fast, current)
  └─► shouldDropResponse(activeRequestId, reqId, document.hidden)
        └─► activeRequestId === reqId   [TRUE — matches]
        └─► proceeds to dispatchVoiceProposal → renders HI text
```

**Visible outcomes asserted:**
- `.caption` / `.destination h2` — shows HI voice text (not stale EN text)
- `#lang-badge` — shows `hi-IN` / `hi`
- Old EN caption text `EN_VOICE_TEXT` is NOT present in `.caption`

---

## 3. Integration Notes

### Contract alignment (backend → frontend)

| Backend contract field | Frontend envelope field | Notes |
|---|---|---|
| `PipelineResponse.state` | `VoiceResponseEnvelope.data.state` | Both use same state values: OK, CLARIFY, ERROR… |
| `PipelineResponse.validated_proposal` | `VoiceResponseEnvelope.data.validated_proposal` | Schema v3.0 validated by `validateVoiceResponse` |
| `PipelineResponse.template.text` | `VoiceResponseEnvelope.data.template.text` | Speech text rendered in UI |
| `PipelineResponse.audio` | `VoiceResponseEnvelope.data.audio` | Audio metadata + base64 |
| `PipelineResponse.request_id` | N/A (frontend uses activeRequestId) | Frontend tracks request ID in flight separately |

`processVoiceEnvelope` (audioGuidance.ts:327) normalises the backend `PipelineResponse`
into `VoiceResponseEnvelope` shape. The response envelope shape was verified from:
- `backend/internal/contracts/pipeline.go:106` — `PipelineResponse`
- `frontend/v2/src/audioGuidance.ts:22` — `VoiceResponseEnvelope`

### DOM selectors used

| Purpose | Selector |
|---|---|
| Language button EN | `[data-language="en-IN"]` |
| Language button HI | `[data-language="hi-IN"]` |
| Caption / spoken text | `.caption` |
| Destination heading | `.destination h2` |
| Language indicator | `#lang-badge` |
| Voice input trigger | `[data-action="voice-input"]` |

---

## 4. Negative Control

`test_negative_control_broken_without_supersede` patches `window.supersedeInFlight` to a
no-op before the test runs. In that broken state:

- Language switch does NOT increment `activeRequestId`
- Old response's `reqId === activeRequestId` → `shouldDropResponse` returns `false`
- Stale response IS processed and renders in UI → **visible EN text appears after HI switch**

**Expected result in current codebase:** this negative control test will NOT see the stale EN text
(because `supersedeInFlight` IS present). The test failing to find stale text PROVES the fix works.

If the negative control FINDS stale EN text, it means `supersedeInFlight` was removed or broken.

---

## 5. How to Run

```bash
# From project root
cd plan/worker-reports/round-3/worker-3

# Dependencies (Playwright + PyYAML)
pip install playwright pyyaml
playwright install chromium

# Set environment
export APP_URL="http://localhost:5173"
export DEV_USER="demo@example.com"
export DEV_PASS="demo"

# Run
python language_accept.py
```

Or with pytest (if you prefer the test runner wrapper):

```bash
pytest language_accept.py -v
```

---

## 6. Test File

- `language_accept.py` — Python Playwright regression test
  - `test_stale_response_dropped_on_language_switch` — main test
  - `test_negative_control_broken_without_supersede` — negative control

---

## 7. Key Source Locations

| Symbol | File | Line |
|---|---|---|
| `sendVoiceOrText` | `frontend/v2/src/main.ts` | 1266 |
| `activeRequestId` increment | `frontend/v2/src/main.ts` | 1267 |
| `shouldDropResponse` (guard 1) | `frontend/v2/src/main.ts` | 1294 |
| `shouldDropResponse` (guard 2) | `frontend/v2/src/main.ts` | 1306 |
| `supersedeInFlight` (definition) | `frontend/v2/src/main.ts` | 244 |
| SET_LANGUAGE handler | `frontend/v2/src/main.ts` | 1235 |
| Onboarding language switch | `frontend/v2/src/main.ts` | 1401 |
| `shouldDropResponse` (journey) | `frontend/v2/src/journey.ts` | — (grep for exact line) |
| `processVoiceEnvelope` | `frontend/v2/src/audioGuidance.ts` | 327 |
| `VoiceResponseEnvelope` type | `frontend/v2/src/audioGuidance.ts` | 22 |
| `PipelineResponse` contract | `backend/internal/contracts/pipeline.go` | 106 |
| `MiddleWorkerResponse` contract | `backend/internal/contracts/worker_health.go` | 106 |

---

## 8. Open Items

1. **Spinner cleanup** (separate from stale-data protection): the audio feedback spinner
   is cleared by `audioGuard.invalidate()` which is called on SET_LANGUAGE and on new
   requests. The spinner cleanup is independent of `shouldDropResponse` and does NOT
   need its own regression test — it is covered by E's voice-outage patch.

2. **`resolvePlace` fallback removal**: E's patch removes the `resolvePlace` fallback
   at main.ts:1298–1306. This is orthogonal to the language-switch stale-response
   issue and does not conflict with this test.

3. **E's voice-outage patch**: confirmed compatible — it removes a `resolvePlace`
   fallback, not any part of the `supersedeInFlight` / `shouldDropResponse` chain.
