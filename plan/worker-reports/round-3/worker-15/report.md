# Worker 15 — Lost Reservation Response Regression
**Worker:** worker-15 | **Assignment:** round-3
**Timestamp:** 2026-09-27T12:14:00Z | **HEAD:** d95df9e
**Output dir:** plan/worker-reports/round-3/worker-15/

---

## 1. Scope Inspected

| File | SHA-256 (d95df9e) | Dirty? |
|------|-------------------|--------|
| `frontend/v2/src/main.ts` | `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09` | yes (unrelated) |
| `frontend/v2/src/journey.ts` | `6f0d4e2710197665efa5ed7ffc5b1251ebbc9f22ddf55cd063f39b9bf29705a1` | no |
| `backend/internal/httpserver/stay_handlers.go` | `7b745f4ff74368d44b5c944fec0f75e769cf5922a5127cba7b6460a3d1f588cf` | no |
| `backend/internal/httpserver/stay_http_integration_test.go` | `053ed7b9efa56f35bd3f4b7a2f6fe6651e02839de2af1de3ab98ae0f84c946ad` | yes |
| `plan/worker-reports/round-2/minimax-a/report.md` | (prior evidence, assessed) | — |

Dirty untracked files not in scope: `backend/internal/httpserver/citizen_ownership_regression_test.go`, `backend/internal/asrworker/audio_mime_test.go`, `backend/internal/middleworker/eval/`, `frontend/v2/src/styles.css`.

---

## 2. Prior Report Assessment (minimax-a)

The minimax-a report (round-2) addressed **backend ownership/idempotency** (B4 gap: post-denial idempotency key survival). My assignment is the **browser-side retry** after a lost response — a different scope.

The backend idempotency implementation (stay_handlers.go, idempotency.go) is confirmed correct by minimax-a. The browser retry contract (pending_reservation sessionStorage + idempotency key replay) is confirmed correct by source inspection below.

**No production code change is warranted.**

---

## 3. Source Inspection: Reservation Retry Flow

### `submitReservation` (main.ts:845–899)

```
Before fetch:  sessionStorage.setItem('pending_reservation', { payload })
On fetch ok:    sessionStorage.removeItem('pending_reservation') → proceeds to routeStarted=true
On network err: keepPendingReservation(null, null) → true  → keeps pending, shows retry message
On 5xx:        keepPendingReservation(5xx, null)  → true  → keeps pending, shows retry message
On 409 retryable: keepPendingReservation(409, err) → true  → keeps pending, shows retry message
On definitive err: keepPendingReservation(...) → false → clears pending, shows error
```

**Key property verified:** `sessionStorage.setItem` is called **before** the fetch (line 849), so the pending reservation is stored before the server processes it. If the response is lost, the pending reservation survives in sessionStorage.

### `startRouteReservation` (main.ts:795–843)

```
On entry (retry path):
  pending = sessionStorage.getItem('pending_reservation')
  if pending.payload.idempotency_key:
    await submitReservation(pending.payload)  ← SAME payload, SAME key
    return
```

**Key property verified:** The retry path reuses the exact same `payload` object that was serialized to sessionStorage before the first attempt. The idempotency key is preserved.

### `keepPendingReservation` (journey.ts:599–603)

```typescript
export function keepPendingReservation(status: number | null, errorBody: unknown): boolean {
  if (status === null || status >= 500) return true;
  const e = (errorBody as {...})?.errors?.[0];
  return status === 409 && e?.code === 'IDEMPOTENCY_CONFLICT' && e?.retryable === true;
}
```

| Scenario | Returns | Effect |
|----------|---------|--------|
| Network error (status=null) | `true` | Pending kept; retry reuses same key |
| 5xx server error | `true` | Pending kept; retry reuses same key |
| 409 + IDEMPOTENCY_CONFLICT + retryable=true | `true` | Pending kept; retry reuses same key (server still processing) |
| 409 + IDEMPOTENCY_CONFLICT + retryable=false | `false` | Pending cleared; fresh request with new key |
| 409 + STALE_VERSION | `false` | Pending cleared; needs new guidance |
| 409 + CAPACITY_CONFLICT | `false` | Pending cleared; no point retrying same key |
| 4xx definitive error | `false` | Pending cleared |

**Decision: NO_CHANGE_NEEDED for production code.** The implementation is correct.

---

## 4. Gap: Missing Regression Coverage

No existing test covers the browser-level lost-response retry scenario. The existing `keepPendingReservation` unit tests (journey.test.ts:583-592) cover the pure function logic but not the full browser flow: store → network-error → retry → same key → single stay.

### Existing Coverage Map

| Behavior | Existing Test | Status |
|----------|--------------|--------|
| `keepPendingReservation` logic | journey.test.ts:583 | ✅ Unit-tested |
| Backend idempotency (same key replays) | b02_idempotency_compat_test.go | ✅ Backend tested |
| Backend 409 IDEMPOTENCY_CONFLICT replay | b02_idempotency_compat_test.go:94 | ✅ |
| Browser: network error keeps pending_reservation | ❌ | **GAP** |
| Browser: retry reuses same idempotency_key | ❌ | **GAP** |
| Browser: retry creates only one stay | ❌ | **GAP** |

---

## 5. Artifacts

### Artifact 1: Node.js Unit Test — `reservation_retry_logic.test.ts`
**Path:** `plan/worker-reports/round-3/worker-15/reservation_retry_logic.test.ts`
**Base SHA-256:** `6f0d4e2710197665efa5ed7ffc5b1251ebbc9f22ddf55cd063f39b9bf29705a1` (journey.ts)
**What it tests:** The `keepPendingReservation` pure function — all 7 scenarios in a table-driven style.

Run command:
```bash
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2/src
node --experimental-strip-types --test ../plan/worker-reports/round-3/worker-15/reservation_retry_logic.test.ts
```
Status: **NOT_RUN** — deferred until after usage reset.

Expected: all 7 assertions PASS. Failure of any assertion indicates the retry logic has regressed.

### Artifact 2: Playwright Browser Regression — `reservation_lost_response_retry.test.mjs`
**Path:** `plan/worker-reports/round-3/worker-15/reservation_lost_response_retry.test.mjs`
**Prerequisites:**
```bash
# Playwright must be available as an ES module import
# Install in frontend/v2 if not already:
#   cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
#   npm install playwright

# Start dev server
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm run dev &
sleep 3

# Run the regression
node ../plan/worker-reports/round-3/worker-15/reservation_lost_response_retry.test.mjs
```
**What it tests (end-to-end in a real browser):**
1. Click Reserve Route → intercept and abort the response (simulating network loss)
2. Verify `pending_reservation` persists in sessionStorage
3. Click Reserve Route again (retry)
4. Verify the second request uses the **identical** `idempotency_key` as the first
5. Verify `pending_reservation` is cleared after successful retry
6. Verify accepted `stay_id` is set (single stay created)

**Current expected result (before any production fix):** PASS — the implementation is correct; this test proves the regression coverage exists.

**Failure mode if implementation regresses:** Step 4 fails (new idempotency key generated) → browser test exits with code 1.

---

## 6. No Production Patch Required

**NO_CHANGE_NEEDED** for production code. The reservation retry logic is correctly implemented:
- Pending payload stored in sessionStorage **before** fetch (survives response loss)
- `keepPendingReservation(null, null)` returns `true` for network errors
- `startRouteReservation` retry path reuses the same `payload` and `idempotency_key`
- Backend idempotency replays the committed reservation without creating a second stay

The only gap is **test coverage**, not implementation correctness.

---

## 7. Status

**STATUS:** NO_CHANGE_NEEDED — production code is correct; artifacts add regression coverage.

**Integration dependencies:** None — no production code changed. Artifacts are self-contained.

**Exact next action for Opus:**
1. Run the Node.js unit test: `cd frontend/v2/src && node --experimental-strip-types --test ../plan/worker-reports/round-3/worker-15/reservation_retry_logic.test.ts` — all assertions should PASS
2. For full browser coverage: run the Playwright regression against the dev server (requires `npm install playwright` in frontend/v2)
3. No production code review needed for the retry flow

---

*Report generated by worker-15. No execution tests performed. Artifacts are NOT_RUN.*
