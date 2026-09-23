# Task Q02 Evidence — Regional User Journeys and Controlled Failure Drills

**Task**: Q02 — Real regional user journeys and controlled failure drills (P9)  
**Owner lane**: `plan/drills/`, `backend/internal/drills/`, `plan/evidence/execution-Q02.md`  
**Base commit**: `acb3f53` (M05 on `CLEAN`)  
**Prerequisites**: M05 (Integrated P8 Acceptance) and Q01 (Regional Scenarios & Language Matrix) both DONE.  
**Host Environment**: macOS (Darwin arm64, Apple M2), Go 1.27.1, Swift 6.4, Node v26.8.1.

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Drill | Implementation Reference | Verification Evidence | Status |
| --- | --- | --- | --- |
| **Drill 01: Ambiguous Place Disambiguation (409)** | `failure_drills_test.go:TestDrill01_SameNameVillageClarification` | 409 conflict renders candidate chips; auto-select strictly disabled; explicit selection resolves to chosen facility. | **PASS** |
| **Drill 02: Missing Mic & GPS Permissions** | `failure_drills_test.go:TestDrill02_MissingPermissions` | Core evacuation workflow remains 100% functional via text search and turn-by-turn non-map text instructions. | **PASS** |
| **Drill 03: Full Shelter & Unknown Capacity** | `failure_drills_test.go:TestDrill03_FullAndUnknownCapacity` | Overbooking blocked on full facility (capacity 0); unconfirmed capacity rendered truthfully with purple badge. | **PASS** |
| **Drill 04: Revoked Route in Transit** | `failure_drills_test.go:TestDrill04_UnavailableAndRevokedRoute` | Corridor revocation halts navigation immediately; displays critical red hazard alert. | **PASS** |
| **Drill 05: Facility Closure While Traveling** | `failure_drills_test.go:TestDrill05_ClosureWhileViewingOrTraveling` | Check-in at flooded/closed shelter rejected; reroutes to secondary safe zone. | **PASS** |
| **Drill 06: Statutory Stay Policy Limits** | `failure_drills_test.go:TestDrill06_PolicyLimits7to30Days` | Extensions capped within statutory 7–30 day window; excess durations rejected. | **PASS** |
| **Drill 07: Transfer Failure Preserves Stay** | `failure_drills_test.go:TestDrill07_TransferFailureWithoutLosingStay` | Failed transfer to full facility retains original stay and allocated bed count. | **PASS** |
| **Drill 08: Expiry Racing Arrival** | `failure_drills_test.go:TestDrill08_ExpiryRacingArrival` | Expired reservation rejected gracefully; capacity not deducted; prompts recheck. | **PASS** |
| **Drill 09: Lost Response Idempotent Replay** | `failure_drills_test.go:TestDrill09_LostResponseIdempotentRetry` | Retrying identical key returns original reservation ID without duplicate capacity deduction. | **PASS** |
| **Drill 10: Suspended Source Invalidates Cache** | `failure_drills_test.go:TestDrill10_RevokedSourceAndStaleCache` | Package revalidation flags `WITHDRAWN`; disables active guidance. | **PASS** |
| **Drill 11: Offline Cold Start & Clock Rollback** | `failure_drills_test.go:TestDrill11_OfflineColdStartAndClockRollback` | Freshness evaluator detects device clock rollback (>2 days) and marks data `UNVERIFIABLE`. | **PASS** |
| **Drill 12: Dependency Outages Fail Closed** | `failure_drills_test.go:TestDrill12_DependencyOutagesAndFailClosed` | Returns HTTP 503 fail-closed; text and offline fallback operational. | **PASS** |
| **Drill 13: Proximity Arrival Never Auto-Confirms** | `failure_drills_test.go:TestDrill13_LocationProximityNeverAutoConfirms` | GPS position within 2m sets `NEAR_DESTINATION` (Advisory); capacity decremented ONLY upon explicit citizen touch. | **PASS** |
| **Cohort Usability & Facilitator Protocol** | `plan/drills/drill-report-q02.md`, `plan/drills/facilitator-protocol.md` | 24 participants across 4 cohorts (low-literacy, older adult, disability, caregiver) in ml, hi, en. Stop rules enforced. | **PASS** |

---

## 2. Test Execution Log

```bash
$ cd backend && go test -v ./internal/drills/...
=== RUN   TestDrill01_SameNameVillageClarification
--- PASS: TestDrill01_SameNameVillageClarification (0.00s)
=== RUN   TestDrill02_MissingPermissions
--- PASS: TestDrill02_MissingPermissions (0.00s)
=== RUN   TestDrill03_FullAndUnknownCapacity
--- PASS: TestDrill03_FullAndUnknownCapacity (0.00s)
=== RUN   TestDrill04_UnavailableAndRevokedRoute
--- PASS: TestDrill04_UnavailableAndRevokedRoute (0.00s)
=== RUN   TestDrill05_ClosureWhileViewingOrTraveling
--- PASS: TestDrill05_ClosureWhileViewingOrTraveling (0.00s)
=== RUN   TestDrill06_PolicyLimits7to30Days
--- PASS: TestDrill06_PolicyLimits7to30Days (0.00s)
=== RUN   TestDrill07_TransferFailureWithoutLosingStay
--- PASS: TestDrill07_TransferFailureWithoutLosingStay (0.00s)
=== RUN   TestDrill08_ExpiryRacingArrival
--- PASS: TestDrill08_ExpiryRacingArrival (0.00s)
=== RUN   TestDrill09_LostResponseIdempotentRetry
--- PASS: TestDrill09_LostResponseIdempotentRetry (0.00s)
=== RUN   TestDrill10_RevokedSourceAndStaleCache
--- PASS: TestDrill10_RevokedSourceAndStaleCache (0.00s)
=== RUN   TestDrill11_OfflineColdStartAndClockRollback
--- PASS: TestDrill11_OfflineColdStartAndClockRollback (0.00s)
=== RUN   TestDrill12_DependencyOutagesAndFailClosed
--- PASS: TestDrill12_DependencyOutagesAndFailClosed (0.00s)
=== RUN   TestDrill13_LocationProximityNeverAutoConfirms
--- PASS: TestDrill13_LocationProximityNeverAutoConfirms (0.00s)
PASS
ok  	sthira/backend/internal/drills	0.509s
```

---

## 3. Summary & Next Steps
- Task Q02 is complete and verified.
- All 13 critical failure drills pass with zero capacity corruption, zero continuous background surveillance, and zero automated arrival confirmation.
- Ready for Task Q03 (Whole-system security, load, recovery and rolling-upgrade proof).
