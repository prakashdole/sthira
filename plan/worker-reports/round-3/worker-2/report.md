# Worker 2 Report — Port Outage Regression to Installed Tooling
**Report**: `plan/worker-reports/round-3/worker-2/report.md`
**Worker**: Worker 2
**Timestamp**: 2026-09-27T00:00:00Z
**HEAD**: `d95df9e609454877a91b7c82146b3c6368181b17`
**main.ts SHA-256**: `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09`

---

## 1. Scope Inspected

| Area | Source | Finding |
|------|--------|---------|
| Voice outage fix (503/504 handler) | `frontend/v2/src/main.ts:1298–1303` | Fix already applied — no `resolvePlace` fallback for transcript |
| MODEL_UNAVAILABLE catch block | `frontend/v2/src/main.ts:1362–1368` | Sets `commandError = words[language].commandUnavailable` + `voiceFeedbackKey = 'backendUnavailable'` |
| Prior minimax-e candidate.patch | `plan/worker-reports/round-2/minimax-e/candidate.patch` | Same fix; base SHA-256 `ed9dbcd5bbc7a63c46f42d1d63d03157d9644830842213324bbfca11d2294253` |
| Existing outage.py | `/tmp/w3keep/outage.py` | Uses `kill` of mock worker; different approach (process kill vs HTTP interception) |
| Existing accept.py | `/tmp/w3keep/accept.py` | Baseline for onboarding flow and voice console selectors |

**Dirty files** (not modified):
- `backend/internal/asrworker/audio_mime_test.go`, `backend/internal/ttsworker/audio_mime_test.go`
- `backend/internal/middleworker/eval/` corpus changes
- `frontend/v2/src/styles.css`
- `backend/internal/httpserver/stay_http_integration_test.go`

---

## 2. Deliverables

| Artifact | Path |
|----------|------|
| Python Playwright regression script | `plan/worker-reports/round-3/worker-2/outage_accept.py` |
| This report | `plan/worker-reports/round-3/worker-2/report.md` |

---

## 3. Findings

### F1. Fix Already Applied — No Patch Needed

**Evidence**: `frontend/v2/src/main.ts:1298–1303` reads:
```typescript
if (!res.ok) {
  if (res.status === 503 || res.status === 504) {
    throw new Error('MODEL_UNAVAILABLE');
  }
  throw new Error(`Voice pipeline returned ${res.status}`);
}
```

The `resolvePlace` fallback for transcript input inside the 503/504 handler is **absent** — it was removed by minimax-e's patch. The catch block at line 1362–1368 correctly handles `MODEL_UNAVAILABLE` by setting `commandError = words[language].commandUnavailable`.

**Conclusion**: No source patch is warranted. Status for source: **NO_CHANGE_NEEDED**.

### F2. Regression Script — `outage_accept.py`

The Reticle script (`voice-outage-fallback.reticle.js`) was the original regression artifact but requires Reticle initialization. The assignment requires a Python Playwright equivalent using existing tooling.

The new script (`outage_accept.py`) uses `page.route` to intercept `**/api/v3/voice/process`, injects 503 via `route.fulfill(status=503)`, then:

1. **Phase 1** (503 injected): Submits text → checks `.command-error` visible with "unavailable" and NOT "not found"
2. **Phase 2**: Confirms the intercepted status was 503
3. **Phase 3**: `page.unroute` removes interception → submits again → no `.command-error`

**Selector decisions**:
- `.command-error` class (line 1558 in `main.ts`): `class="command-error" role="alert"` — confirmed in source, not invented
- `#command-input` (line 1560): the text input inside the voice console form
- `[data-command-form] button[type="submit"]`: submit button inside the voice console form

**Uncertainty**: The negative control ("not found" must not appear) depends on the i18n word for `commandUnavailable`. If that word contains "not found" in some locale, the assertion would false-fail. Current EN/HI/ML translations use "unavailable" / "उपलब्ध नहीं" / "ലഭ്യമല്ല" — none contain "not found".

---

## 4. Artifact

### `outage_accept.py`

**Base hashes for reference** (script is new, not a patch):
- `frontend/v2/src/main.ts`: `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09` (current)
- `frontend/v2/src/main.ts` (pre-fix base): `ed9dbcd5bbc7a63c46f42d1d63d03157d9644830842213324bbfca11d2294253` (minimax-e candidate.patch base)

No source patch — fix already applied. The script is the regression artifact.

---

## 5. Verification

**Execution tests NOT_RUN — deferred by user.**

### Later Commands

```bash
# Prerequisites
pip install playwright
playwright install chromium

# Stack must be running (Vite dev server + sthira-exercise + mock-workers on 127.0.0.1:18492)
python3 plan/worker-reports/round-3/worker-2/outage_accept.py
python3 plan/worker-reports/round-3/worker-2/outage_accept.py http://127.0.0.1:18492/
```

### Expected Outcomes

| Check | Expected |
|-------|----------|
| onboarding completes | PASS |
| command-error visible during 503 | PASS |
| error text contains "unavailable" | PASS |
| error text does NOT contain "not found" | PASS (negative regression control) |
| voice/process received 503 | PASS |
| after unroute: no command-error | PASS |
| no uncaught page errors | PASS |

### Negative Control

| Scenario | Expected |
|----------|----------|
| Submit text during 503 BEFORE the fix | `commandResponse = 'Location "Show my route" not found...'` — fails "not found" check |
| Submit text during 503 AFTER the fix | `commandError = "Voice Map Control is unavailable..."` — passes both assertions |

---

## 6. Status

**Status: NO_CHANGE_NEEDED** (source fix already applied)

**New artifact**: `outage_accept.py` — regression harness using Python Playwright `page.route` for HTTP status injection.

**Next action for Opus**: Execute `outage_accept.py` against the integrated revision. The script tests the already-applied fix; it should pass. If it fails, the fix may have been reverted or the i18n strings changed.
