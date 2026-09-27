# Worker 4 — Final Report

**Report:** `plan/worker-reports/mac-final/worker-4/`
**Revision:** `e8836922d90a153e2d3e571efde2381b28aa1a43` (frontend: honest voice-outage message)
**Executed:** 2026-09-27
**Status:** NOT_RUN — services not available

---

## 1. Source Hashes

| File | SHA-256 (16 prefix) | Key symbols |
|------|---------------------|-------------|
| `frontend/v2/src/main.ts` | `27603c3c0b20cfda` | `submitReservation:845`, `keepPendingReservation` call at 892, `sessionStorage` restoration at 171-173, 870-872 |
| `frontend/v2/src/journey.ts` | `6f0d4e2710197665` | `keepPendingReservation:599` |
| `backend/internal/store/stay.go` | `a2d412776acb2cfd` | idempotency handling |
| `backend/internal/httpserver/stay_http_integration_test.go` | `bc0b48d6eb0e07df` | ownership regression |

---

## 2. Coverage Map

| Check | Worker 1 | Worker 2 | Worker 3 | prototype_accept | **This Worker 4** |
|-------|----------|----------|----------|-----------------|-------------------|
| Onboarding, guidance dest, text command | ✓ | | | ✓ core | |
| Route/reservation via visible control | ✓ | | | | |
| Duplicate-start protection | ✓ | | | | |
| GPS arrival with server acknowledgement | ✓ | | | | |
| 503/504 outage → assistant unavailable | | ✓ | | ✓ outage | |
| Recovery after outage | | ✓ bug | | ✓ outage | |
| Language-switch stale response guard | | | ✓ | | |
| **Lost reservation response + idempotency retry** | | | | | **NEW** |
| **Pending/accepted stay restoration on reload** | | | | | **NEW** |
| **Outage exit-code regression** | | ✓ bug | | ✓ | **NEW** |

---

## 3. Finding: Worker 2 Exit Code Bug

**File:** `plan/worker-reports/round-3/worker-2/outage_accept.py:77-216`

**Root cause:** `main()` returns `all_ok` which is initialized to `True` and only set to `False` if onboarding raises an exception. The `check()` function appends to the `CHECKS` list but never updates `all_ok`. Consequently, even when `CHECKS` contains failures, `all_ok` remains `True` and `sys.exit(0)` is returned.

```python
# Line 77-79:
async def main(base_url: str):
    voice_statuses = []
    all_ok = True   # ← only changed in onboarding exception handler

# check() function (lines 34-37) does NOT update all_ok:
def check(name: str, ok: bool, detail: str = "") -> None:
    CHECKS.append({"name": name, "pass": ok, "detail": detail})
    # all_ok never updated

# Final return (line 216):
return all_ok   # ← always True unless onboarding failed
```

**Impact:** The script reports PASS even when individual checks fail. Any CI gate using this script's exit code would pass incorrectly.

**Correct pattern (in prototype_accept.py line 132-134):**
```python
fails = [r for r in RESULTS if r["status"] != "PASS"]
return 1 if fails or not RESULTS else 0  # computes exit from actual check results
```

---

## 4. New Acceptance Checks

### 4a. Idempotency: Lost Reservation Response + Retry

**Scenario:** Server receives and commits the first reservation request before its response is lost. Client retries with the identical idempotency key and payload. Server replays the committed request instead of allocating a new stay.

**Guard chain in source:**
- `main.ts:806-813`: `pending_reservation` from sessionStorage triggers identical retry
- `main.ts:821-830`: idempotency key generated once per attempt, stored in `pending_reservation`
- `main.ts:845-899`: `submitReservation` — network exception (status=null) calls `keepPendingReservation` which returns `true`, keeping the pending record
- `journey.ts:599-603`: `keepPendingReservation(null, null)` returns `true` (network loss)

**Visible assertions:**
1. After dropped response: `pending_reservation` saved in sessionStorage with `idempotency_key`
2. After retry (page reload): `stay_id` is restored (server replayed first request)
3. `pending_reservation` cleared after successful retry
4. No page errors

### 4b. Stay Restoration After Page Reload

**Scenario:** After a successful reservation, reopening the page restores the stay from sessionStorage and rebinds the destination.

**Guard chain in source:**
- `main.ts:171-173`: On load, `activeStayId` / `activeStayFacilityId` restored from sessionStorage
- `main.ts:329`: Destination binding refreshed for the restored stay
- `main.ts:870-874`: After reservation success, stay + facility stored, destination rebound

**Visible assertions:**
1. After reservation: `sthira_stay_id` and `sthira_stay_facility_id` in sessionStorage
2. After reload: same `stay_id` and `facility_id` restored
3. Route button not disabled (stay is active)
4. Directions sheet auto-opens for restored accepted stay
5. No page errors

### 4c. Outage Exit-Code Regression

**Scenario:** Verify that 503/504 injection produces correct visible behavior and that the test harness itself returns nonzero exit codes on failure (the Worker 2 bug).

**Verified in prototype_accept.py (already passing):**
- 503: `.command-error` visible with "unavailable" text, NOT "not found"
- 504: same behavior
- Recovery: next request succeeds with 200 and clears error

**Worker 4 adds:** explicit exit-code verification — `sys.exit(1)` when any check fails.

---

## 5. Script Artifact

**Path:** `plan/worker-reports/mac-final/worker-4/worker4_accept.py`

**Syntax verification:** ✓ (Python 3 py_compile passed)

**Execution requires:**
```bash
# Stack running on:
# - Vite dev server: http://127.0.0.1:18492 (or 5173)
# - Backend: http://127.0.0.1:8080
# - PostgreSQL with demo seed data

python3 plan/worker-reports/mac-final/worker-4/worker4_accept.py http://127.0.0.1:18492/ --json results.json
```

**Exit behavior:** 0 if all checks PASS, 1 if any FAIL or NOT_RUN.

---

## 6. Remaining Acceptance Gaps

| Gap | Description | Known Missing Workers |
|-----|-------------|----------------------|
| Real microphone / TTS audio quality | Synthetic stack only | — |
| Safari browser | Chromium only | — |
| Multi-tab concurrent sessions | Single-page harness | — |
| Real geolocation on physical device | Playwright emulation only | — |
| Workers 21, 29 results | Not present in round-3 reports | 21, 29 |

### Evidence Gaps for Workers 21/29
No artifacts found in `plan/worker-reports/round-3/` for workers 21 or 29. These are absent from the round-3 integration record.

---

## 7. Build Verification (no runtime)

**Backend Go build:** ✓ `go build ./...` clean
**Backend tests:** ✓ `go test ./internal/store/...` 0.889s PASS; `go test ./internal/httpserver/...` 26.436s PASS
**Frontend build:** ✓ `npm run build` — 1.41s, index.js 124kB, index.css 163kB

These confirm source integrity but do not replace browser journey evidence.

---

## 8. NOT_RUN Reason (continued)

Services not running (ports 18492, 8080, 5173 all inactive). Cannot execute browser-based Playwright tests without the full stack.

**Commands that would execute when services are available:**
```bash
# Terminal 1: PostgreSQL (assumed running)
# Terminal 2: Backend
cd backend && go run ./cmd/sthira/

# Terminal 3: Mock workers (if needed)
# Terminal 4: Frontend
cd frontend/v2 && npm run dev

# Terminal 5: Run Worker 4 checks
python3 plan/worker-reports/mac-final/worker-4/worker4_accept.py http://127.0.0.1:18492/
```

---

## 9. Cross-Cutting Issue for Opus

**Finding:** Worker 2's `outage_accept.py` exit-code bug means any CI gate relying on its exit code would pass even with failures. The bug is in the return value of `main()` — it ignores the `CHECKS` list entirely.

**Fix required in Worker 2's script:**
```python
# Replace line 216: return all_ok
# With:
passed = sum(1 for c in CHECKS if c["pass"])
failed = sum(1 for c in CHECKS if not c["pass"])
return 1 if failed else 0
```
