# Worker 1 — Browser Harness Repair
**Report:** `plan/worker-reports/round-3/worker-1/report.md`
**Worker:** Worker 1 (round 3)
**Timestamp:** 2026-09-27
**Status:** READY_FOR_REVIEW_UNVERIFIED
**HEAD:** d95df9e (docs: record handoff commit id)

---

## 1. Scope Inspected

### Files Read (source-backed evidence)
| File | SHA-256 |
|------|---------|
| `frontend/v2/src/main.ts` (dirty) | 00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09 |
| `backend/internal/httpserver/stay_http_integration_test.go` (dirty) | (see dirty state) |

### Prior Artifact Inspected
| File | SHA-256 |
|------|---------|
| `plan/worker-reports/round-2/minimax-b/browser_accept.py` (baseline) | 0ea14c8e46214a931672cb86b203f481b76782dbf25cc8639bd53670f6d44fb7 |
| `plan/worker-reports/round-2/minimax-b/report.md` | (assessed) |

### Dirty/Untracked Files (pre-existing, not modified by this worker)
- `frontend/v2/src/main.ts` — dirty (voice 503/504 fallback removed; selectors unchanged)
- `backend/internal/httpserver/stay_http_integration_test.go` — dirty (MiniMax A patch applied: `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives`)
- `backend/internal/asrworker/audio_mime_test.go`, `backend/internal/ttsworker/audio_mime_test.go` — dirty
- `frontend/v2/src/styles.css` — dirty
- `backend/internal/httpserver/citizen_ownership_regression_test.go` — untracked
- `plan/prompts/`, `plan/worker-reports/` — untracked

---

## 2. Existing Work vs New Contribution

### Existing Work (MiniMax B baseline)
- `browser_accept.py` at `plan/worker-reports/round-2/minimax-b/browser_accept.py` — functional harness with configurable base URL, viewport override, response-capture for duplicate detection, GPS arrival sequence, optional httpx API spot-check

### New Contribution (this worker)
Identified and fixed three correctness bugs in the baseline harness. All selectors were re-verified against `main.ts` (lines 1385–1567).

---

## 3. Findings

### Finding 1 — CRITICAL: Duplicate detection always passed (broken)

**File:** `minimax-b/browser_accept.py:118-141` (original `click_start_route_twice`)

**Root cause:** `posts_before` was declared as an empty local list and returned directly without ever being populated from `posts_to_reservations`. The response listener `on_response` appended to the module-level `posts_to_reservations`, but `click_start_route_twice` returned its own `posts_before = []` — which is always empty.

```python
# OLD (broken):
async def click_start_route_twice(page) -> tuple[int, str, str]:
    posts_before = []  # never populated — always []
    await page.locator('[data-action="route"]').first.click()
    await asyncio.sleep(0.5)
    # ... dismiss directions ...
    await page.locator('[data-action="route"]').first.click(force=True)
    await asyncio.sleep(1.0)
    # ...
    return len(posts_before), stay_id, facility_id  # always returns 0
```

Then in the main flow:
```python
# OLD: posts_before always empty
posts_before = len(posts_to_reservations)  # captured BEFORE setting up listener (always 0!)
await page.locator('[data-action="route"]').first.click(force=True)
# ...
new_posts = posts_after - posts_before  # posts_before=0, so new_posts == posts_after
duplicate_ok = (new_posts <= 1 and ...)  # POSTS_AFTER might be 0-2, so this always passed
```

The duplicate detection check would pass even when the server allowed two distinct POSTs (broken idempotency).

**Fix:** Capture `posts_before_second = len(posts_to_reservations)` **after** the first reservation but **before** the second click. Assert `new_posts == 0` (not `<= 1`).

**Impact:** Without this fix, a broken server that created duplicate reservations would be reported as passing.

---

### Finding 2 — Missing explicit waits for directions sheet

**File:** `minimax-b/browser_accept.py:126-131`

**Issue:** The directions sheet dismissal used a fixed-count polling loop with `asyncio.sleep(0.3)`:
```python
# OLD: arbitrary polling
for _ in range(5):
    if await page.locator('[data-action="directions-close"]').count():
        await page.locator('[data-action="directions-close"]').first.click()
        await asyncio.sleep(0.2)
    await asyncio.sleep(0.3)
```

This could pass without the element ever appearing if the server is slow (the count check returns 0 for up to 1.5s). The direction sheet also auto-appears after a successful reservation (main.ts:877 `directionsOpen = true`), so it should appear reliably.

**Fix:** Added `await page.wait_for_selector('[data-action="directions-close"]', timeout=8000)` after the first route click to confirm the reservation was actually accepted before attempting dismissal.

---

### Finding 3 — Onboarding could hang without geolocation fallback

**File:** `minimax-b/browser_accept.py:69-88` (`complete_onboarding`)

**Issue:** The harness waits for `[data-action="onboarding-complete"]` with no fallback. If the emulated geolocation context fails to resolve within the Playwright timeout (or location permission is denied in the environment), the button never appears and the test hangs.

The `main.ts` template at line 1394 shows two variants:
- Primary (location ready): `data-action="onboarding-complete"` + text "Start Guidance"
- Fallback (location unavailable): `data-action="onboarding-complete"` + text "Continue without location"

The `locationStatus` transitions to `'unavailable'` when `navigator.geolocation.getCurrentPosition` fails (main.ts:299).

**Fix:** Added a try/except with a 3-second fallback that clicks the secondary button containing "Continue".

---

### Finding 4 — GPS watch started automatically; explicit click may be no-op

**File:** `minimax-b/browser_accept.py:151-155` (`gps_arrival_sequence`)

**Observation:** `startTracking()` is called inside `submitReservation()` (main.ts:883) after the POST succeeds. This sets `journeyState = 'TRACKING'` before the route click handler returns. Since `trackingButtonHtml` (main.ts:446) shows `start-tracking` only when `journeyState` is not `'TRACKING'/'NEAR_DESTINATION'/'ARRIVAL_REPORTED'/'ROUTE_REVOKED'`, the button may never be visible after a successful reservation.

The harness tries to click it with a pass-through:
```python
try:
    await page.wait_for_selector('[data-action="start-tracking"]', timeout=8000)
    await page.locator('[data-action="start-tracking"]').first.click()
except Exception:
    pass  # button absent — tracking may be auto-started or N/A in demo mode
```

This is benign (the `except` always fires after the `wait_for_selector` times out, so the GPS watch is already started). However, the 8-second wait before the fallback adds 8s to every run even when unnecessary.

**Fix:** Removed the explicit `start-tracking` click from `gps_arrival_sequence`. The GPS watch is already running. Only `arrival-open` needs to be waited for.

---

### Finding 5 — `wait_for_network` helper defined but never used

**File:** `minimax-b/browser_accept.py:48-62`

The `wait_for_network` helper was defined but never called in the harness. Removed it.

---

## 4. Verified Selectors (unchanged from MiniMax B baseline)

All selectors confirmed in `main.ts` at HEAD (dirty changes only affect voice fallback code, not selectors):

| Selector | Element | main.ts line |
|---------|---------|---------------|
| `[data-action="onboarding-start"]` | Begin onboarding | 1385 |
| `[data-onboarding-language="EN"]` | Language option EN | 1387 |
| `[data-action="onboarding-language-next"]` | Language step continue | 1389 |
| `[data-action="onboarding-complete"]` | Location step complete | 1394–1395 |
| `[data-testid="emergency-card"]` | Guidance card | 1448 |
| `[data-action="voice-open"]` | Voice console launcher | 1443, 1551 |
| `.voice-console` | Voice console panel | 1548 |
| `[data-action="voice-close"]` | Voice console close | 1549 |
| `#command-input` | Voice console text input | 1554 |
| `[data-command-form] button[type="submit"]` | Voice form submit | 1554 |
| `[data-action="route"]` | Start Route button | 1525 |
| `[data-action="directions-close"]` | Directions sheet close | 1558 |
| `[data-action="arrival-open"]` | Arrival trigger | 1462 |
| `[data-action="arrival-yes"]` | Confirm arrival button | 1561 |
| `[data-action="start-tracking"]` | GPS tracking button | 446 |

---

## 5. Artifact Paths

| Artifact | Path |
|---------|------|
| Corrected harness | `plan/worker-reports/round-3/worker-1/browser_accept.py` |
| This report | `plan/worker-reports/round-3/worker-1/report.md` |
| Baseline (MiniMax B, for reference) | `plan/worker-reports/round-2/minimax-b/browser_accept.py` |

**Base hashes:**
```
frontend/v2/src/main.ts         00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09
minimax-b/browser_accept.py     0ea14c8e46214a931672cb86b203f481b76782dbf25cc8639bd53670f6d44fb7
worker-1/browser_accept.py      5d7b28142311419747ee3874e699878e92707e8505dd9e72cdd227749ccf72e1
```

---

## 6. Verification Plan

**Source inspection performed:** Yes — main.ts lines 280–305 (location flow), 840–900 (submitReservation), 445–450 (tracking button), 1385–1567 (all selectors), 875–885 (directionsOpen auto-set).

**Execution tests: NOT_RUN — deferred by user (usage reset in ~10 min).**

### Post-reset commands and prerequisites

**Prerequisites:**
```bash
# One-time setup
pip install playwright httpx
playwright install chromium  # or: npx playwright install chromium

# Stack must be running:
# - PostgreSQL seeded with demo data
# - sthira-exercise backend on port 8080 (or 18492 via Vite proxy)
# - mock-workers in destination-choice scenario
# - Vite dev server on port 18492 (or 5173)
```

**Run harness:**
```bash
cd /Users/apple/Documents/Projects/MonitoringZ
python3 plan/worker-reports/round-3/worker-1/browser_accept.py http://127.0.0.1:18492/

# Mobile viewport
python3 plan/worker-reports/round-3/worker-1/browser_accept.py http://127.0.0.1:18492/ --viewport=390,844
```

**Expected outcomes:**

| Check | Expected | Failure signal |
|-------|----------|---------------|
| window hooks absent without opt-in | PASS | `window.setActiveStayId is not undefined` |
| onboarding completes to guidance card | PASS | Timeout waiting for emergency-card |
| guidance destination rendered | PASS | "Demo Safe Facility" not found |
| text command submitted and processed | PASS | Input never re-enabled |
| route start creates stay in session storage | PASS | stay_id or facility_id null |
| repeated Start Route creates no second reservation | PASS | new_posts != 0 OR stay changed |
| arrival recorded after server acknowledgement | PASS | "Recorded at" not found |
| no uncaught page errors | PASS | Console error lines present |
| API reservation+arrival spot-check | PASS | HTTP non-200 or wrong dest |

**Expected negative control:**
- Without server running: route click returns HTTP 503 → `route start creates stay in session storage` FAIL
- With `?sthira-test-hooks=1` set: `window hooks absent without opt-in` FAIL

**Expected failure before fix (duplicate detection):**
- If server allowed two POSTs on duplicate click (broken idempotency):
  - Old harness: `new_posts <= 1` would pass with `new_posts=1`
  - New harness: `new_posts == 0` FAILS correctly

---

## 7. Status and Next Action

**Status: READY_FOR_REVIEW_UNVERIFIED**

### Key corrections in new harness vs baseline
1. **Duplicate detection fixed**: Now captures `posts_to_reservations` length before second click and asserts `new_posts == 0`
2. **Directions sheet wait**: Explicit `wait_for_selector` instead of polling with fixed sleeps
3. **Onboarding fallback**: Secondary "continue without location" button path when GPS unavailable
4. **GPS auto-start noted**: Removed redundant explicit `start-tracking` click (watch started inside `submitReservation`)
5. **Unused helper removed**: `wait_for_network` was defined but never called

### Next action for Opus
1. Review `plan/worker-reports/round-3/worker-1/browser_accept.py`
2. Stage and run against the integrated stack to confirm all checks pass
3. The duplicate-detection fix is the most important: the old harness would have passed even with broken idempotency

---

*Worker 1 — browser harness repair — d95df9e — NOT_VERIFIED by execution*
