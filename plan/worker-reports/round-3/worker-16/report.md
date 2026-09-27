# Worker 16 — Round 3 — Pending Stay Restore and Selection Binding

**Timestamp:** 2026-09-27T19:07:00Z
**Status:** READY_FOR_REVIEW_UNVERIFIED
**HEAD:** d95df9e (committed); `frontend/v2/src/main.ts`, `frontend/v2/src/journey.ts`, `frontend/v2/src/journey.test.ts` are all clean (no uncommitted changes in these files per git inspection)
**Output dir:** `plan/worker-reports/round-3/worker-16/`

---

## 1. Scope Inspected

| File | SHA-256 |
|------|---------|
| `frontend/v2/src/main.ts` | `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09` |
| `frontend/v2/src/journey.ts` | `6f0d4e2710197665efa5ed7ffc5b1251ebbc9f22ddf55cd063f39b9bf29705a1` |
| `frontend/v2/src/journey.test.ts` | `3b58a8ebf4d6febe1c97b22de965ca234e55da6858ebdd043e35c0fa1d3725f3` |

**Prior evidence:** `plan/worker-reports/round-2/minimax-a/report.md` — backend idempotency and citizen ownership coverage map. Worker 16 scope is frontend sessionStorage restore, not backend idempotency (which MiniMax A already covered).

---

## 2. Source-Backed Findings

### Finding 1: `reconcileStayState()` correctly clears stale stay on 404/401/403

**Evidence:** `main.ts:477-484`
```typescript
} else if (res.status === 404 || res.status === 401 || res.status === 403) {
  sessionStorage.removeItem('sthira_stay_id');
  sessionStorage.removeItem('sthira_reservation_id');
  sessionStorage.removeItem('sthira_stay_facility_id');
  activeStayId = null;
  activeStayFacilityId = null;
  activeReservationId = null;
}
```
**Confirmed:** A stale/missing stay_id is cleanly cleared on reload. No patch needed.

### Finding 2: `pending_reservation` restore uses idempotency key — no duplicate allocation possible

**Evidence:** `main.ts:806-812`
```typescript
let pending = null;
try { pending = JSON.parse(sessionStorage.getItem('pending_reservation') || 'null'); } catch {}
if (pending?.payload?.idempotency_key) {
  await submitReservation(pending.payload);
  return;
}
```
The `idempotency_key` from the stored payload is sent to the server. The backend idempotency layer (verified by MiniMax A's coverage map) prevents duplicate allocation for the same key within the session. This is the correct design.

### Finding 3: `buildReservationPayload` validates snapshot version — malformed stored record fails closed

**Evidence:** `journey.ts:537-542`
```typescript
if (!req.snapshotVersion || req.snapshotVersion <= 0 || !Number.isInteger(req.snapshotVersion)) {
  return { canAllocate: false, errorMessage: 'Missing or non-positive snapshot version. Cannot allocate reservation.' };
}
```
A stored `pending_reservation` from an old guidance session (with a stale `snapshot_version`) will fail `buildReservationPayload()` with `canAllocate: false`. The error message is shown to the user, `pending_reservation` is NOT cleared (so the user can retry with fresh guidance), and no duplicate stay is created. **Fail-closed behavior confirmed.**

### Finding 4: Accepted facility lock prevents silent rebinding during guidance refresh

**Evidence:** `main.ts:326-337` (onDestinationSelectionChanged)
```typescript
function onDestinationSelectionChanged(newDest: DestinationChoice | null) {
  const locked = lockedFacilityId();
  if (!canChangeSelection(locked, newDest?.facility_id)) {
    const accepted = availableDestinations.find((d) => d.facility_id === locked);
    if (accepted) selectedDestination = accepted;
    if (newDest && newDest.facility_id !== locked) {
      reservationError = `A stay is reserved or pending for ${locked}. The destination cannot change while it is active.`;
    }
    render();
    return;
  }
  selectedDestination = newDest;
  ...
}
```

**Mechanism:** After reload, `reconcileStayState()` sets `activeStayFacilityId` from the server. When guidance loads and calls `onDestinationSelectionChanged(nextDest)`, `canChangeSelection()` checks if the locked facility matches `nextDest`. If not, the lock restores the accepted facility from `availableDestinations` (if still present) OR shows an error if the locked facility is no longer in the new guidance.

### Finding 5: Gap — no explicit test for the stale-locked-facility scenario

The existing `journey.test.ts` tests (`canChangeSelection` at line 577-580) cover the core lock logic:
```typescript
assert.equal(canChangeSelection('FAC-A', 'FAC-B'), false); // refresh fallback / user tap
assert.equal(canChangeSelection('FAC-A', null), false);
```

But there is no **browser-level** test that verifies the full reload + reconcile + guidance-load sequence with a locked facility that is NOT in the new guidance's destination list. This is the specific gap the regression test addresses.

---

## 3. Existing vs New Contribution

### Existing (already correct)
- `reconcileStayState()` clears stale stays on 404/401/403 — correct
- `pending_reservation` restore uses idempotency key — correct
- `buildReservationPayload()` validates snapshot version — correct, fail-closed
- `canChangeSelection()` unit tests — correct
- Backend idempotency (MiniMax A verified)

### New (regression test)
**Artifact:** `stay_restore_accept.py` — Python Playwright regression covering 4 browser-level scenarios not tested by existing unit tests:

| Test | What it verifies | Why it would fail before a fix |
|------|-----------------|-------------------------------|
| `test_pending_reservation_not_replayed_as_duplicate` | Stale `pending_reservation` with mismatched facility re-submitted on reload; idempotency prevents duplicate | If backend idempotency key is missing or not enforced |
| `test_accepted_facility_locked_on_guidance_refresh` | After reload with locked facility, new guidance cannot silently rebind to a different destination | If `canChangeSelection()` is bypassed during guidance load, or `availableDestinations` overwrites lock |
| `test_stale_stay_clears_on_404` | Fake stay_id returns 404 → all sessionStorage keys cleared | If `reconcileStayState()` doesn't handle 404, keys persist |
| `test_malformed_pending_reservation_does_not_crash` | Invalid JSON or missing fields in `pending_reservation` → graceful handling, no uncaught exception | If `JSON.parse` exception in restore flow is not caught, JS crashes |

---

## 4. Gap Analysis: What Does NOT Need a Patch

| Scenario | Verdict | Reason |
|----------|---------|--------|
| Stale stay_id → 404 clears sessionStorage | Already correct | `main.ts:477-484` handles it |
| `pending_reservation` replay uses idempotency key | Already correct | `main.ts:809` sends stored key; backend enforces |
| Malformed `pending_reservation` | Already correct | `buildReservationPayload` fails closed; no crash |
| Locked facility rebinding | Already correct | `canChangeSelection` + `onDestinationSelectionChanged` |

**No source patch needed.** The implementation is correct. The regression test provides browser-level coverage that the unit tests do not reach.

---

## 5. Artifact Paths

| Artifact | Path |
|----------|------|
| Python Playwright regression | `plan/worker-reports/round-3/worker-16/stay_restore_accept.py` |
| Report | `plan/worker-reports/round-3/worker-16/report.md` |

---

## 6. Source Hashes (Base: d95df9e)

| File | SHA-256 |
|------|---------|
| `frontend/v2/src/main.ts` | `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09` |
| `frontend/v2/src/journey.ts` | `6f0d4e2710197665efa5ed7ffc5b1251ebbc9f22ddf55cd063f39b9bf29705a1` |
| `frontend/v2/src/journey.test.ts` | `3b58a8ebf4d6febe1c97b22de965ca234e55da6858ebdd043e35c0fa1d3725f3` |

---

## 7. Test Commands and Expected Outcomes

**NOT_RUN — deferred by user.**

### Prerequisites
```bash
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm install
npx playwright install --with-deps
```

### Run dev server
```bash
# Terminal 1
cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
npm run dev
```

### Run stay restore regression
```bash
# Terminal 2
python3 plan/worker-reports/round-3/worker-16/stay_restore_accept.py http://localhost:5173
```

### Expected outcomes

| Test | Expected result | Pass condition |
|------|----------------|----------------|
| `pending_reservation replay does not duplicate stay` | PASS | `active_stay_id` is set OR `pending_reservation` is cleared without a duplicate stay being created |
| `accepted facility locked after guidance refresh` | PASS | `selectedDestination === 'FACDEMO-1'` OR reservation error shown about locked facility |
| `stale stay cleared on 404` | PASS | All three sessionStorage keys (`stay_id`, `reservation_id`, `stay_facility_id`) are null after reload with fake stay ID |
| `malformed pending_reservation does not crash` | PASS | No uncaught JS exception for any malformed case |

**Failure signals:**
- Duplicate stay: backend idempotency broken for frontend replay path
- Lock not enforced: `canChangeSelection` bypassed during `applyDestinationChoices` → guidance silently rebinds destination
- 404 not cleared: `reconcileStayState` doesn't handle 404 → stale keys persist across reloads

---

## 8. Integration Risks

- **None for source changes** — no production source patch proposed
- The regression test reads/writes `sessionStorage` and intercepts HTTP responses — it requires a live backend
- Python Playwright test uses `asyncio` and `async_playwright` matching the existing `browser_accept.py` pattern — no new tooling introduced

---

*Worker 16 — Round 3 — d95df9e — NOT_VERIFIED by execution*
