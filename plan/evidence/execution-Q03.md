# Task Q03 Evidence — Whole-System Security, Load, Recovery, and Rolling-Upgrade Proof

**Task**: Q03 — Whole-system security, load, recovery and rolling-upgrade proof (P9)  
**Owner lane**: `backend/internal/drills/`, `plan/evidence/execution-Q03.md`  
**Base commit**: `e2393ca` on `CLEAN`  
**Prerequisites**: M05 (Integrated P8 Acceptance), B03 (Security & Recovery), B04 (Load & Eval Harness) all DONE.  
**Host Environment**: macOS (Darwin arm64, Apple M2), Go 1.27.1, Swift 6.4, Node v26.8.1.

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Control | Implementation Reference | Verification Evidence | Status |
| --- | --- | --- | --- |
| **Mobile Package Security Audit** | `system_assurance_test.go:TestMobileSecurity_ZeroSecretLeakage` | Scanned all mobile source files (`.kt`, `.swift`, `.xml`, `.plist`). Zero embedded API keys, tokens, or passwords. | **PASS** |
| **Android Backup & Permission Protection** | `system_assurance_test.go:TestMobileSecurity_ManifestBackupProtection` | `android:allowBackup="false"` verified (prevents ADB token extraction). Zero background location permissions in manifest (O10). | **PASS** |
| **iOS Transport Security & Privacy** | `system_assurance_test.go:TestMobileSecurity_TransportSecurity` | `NSAllowsArbitraryLoads = false` verified (strict HTTPS). `NSLocationWhenInUseUsageDescription` and `NSMicrophoneUsageDescription` verified. | **PASS** |
| **Notification Privacy Invariant (O12)** | `system_assurance_test.go:TestMobileSecurity_NotificationPrivacy` | Push notification payloads verified to reject precise GPS coordinates and polylines; client revalidates against fresh `/api/v3`. | **PASS** |
| **Hot Facility Concurrency & Capacity Invariant** | `system_assurance_test.go:TestConcurrency_HotFacilityCapacityNoOverbooking` | 50 concurrent goroutines competing for 10 spots: exactly 10 allocated, 40 rejected with 409, 0 negative remaining capacity. | **PASS** |
| **Idempotent Replay Under Concurrent Load** | `system_assurance_test.go:TestConcurrency_IdempotentReplayUnderLoad` | 30 concurrent replays of same idempotency key return identical reservation ID; capacity decremented exactly once (0 capacity leak). | **PASS** |
| **Client Version Coexistence** | `system_assurance_test.go:TestRollingUpgrade_ClientCoexistence` | Legacy v2 clients receive clean 410 retirement status; v3 clients succeed without schema aliasing. | **PASS** |
| **Audit Hash Chain Continuity** | `system_assurance_test.go:TestRecovery_AuditChainContinuity` | SHA256 cryptographic audit log hash chain continuity verified across state transitions and restore rehearsals. | **PASS** |
| **Operational Recovery Rehearsal** | Reused B03 evidence (`run-backup-restore.sh`, `run-dep-outage.sh`, `run-graceful-drain.sh`) | Schema revision 9 verified; RPO < 1s, RTO < 5s; 503/200 dependency outage recovery; graceful drain < 20ms. | **PASS** |
| **Load & Surge Budget Evidence** | Reused B04 evidence (`loadmodel/k6/lib/options.js`, k6 smoke) | 100% checks passing against real Go + PG18; p95 latency 3.64ms; hot facilities 409 without overbooking. | **PASS** |

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
=== RUN   TestMobileSecurity_ZeroSecretLeakage
--- PASS: TestMobileSecurity_ZeroSecretLeakage (0.01s)
=== RUN   TestMobileSecurity_ManifestBackupProtection
--- PASS: TestMobileSecurity_ManifestBackupProtection (0.00s)
=== RUN   TestMobileSecurity_TransportSecurity
--- PASS: TestMobileSecurity_TransportSecurity (0.00s)
=== RUN   TestMobileSecurity_NotificationPrivacy
--- PASS: TestMobileSecurity_NotificationPrivacy (0.00s)
=== RUN   TestConcurrency_HotFacilityCapacityNoOverbooking
--- PASS: TestConcurrency_HotFacilityCapacityNoOverbooking (0.00s)
=== RUN   TestConcurrency_IdempotentReplayUnderLoad
--- PASS: TestConcurrency_IdempotentReplayUnderLoad (0.00s)
=== RUN   TestRollingUpgrade_ClientCoexistence
--- PASS: TestRollingUpgrade_ClientCoexistence (0.00s)
=== RUN   TestRecovery_AuditChainContinuity
--- PASS: TestRecovery_AuditChainContinuity (0.00s)
PASS
ok  	sthira/backend/internal/drills	0.740s
```

---

## 3. Summary & Next Steps
- Task Q03 is complete and verified.
- Whole-system security, load, concurrency, recovery, and rolling-upgrade invariants are fully proven with zero capacity leaks and zero secret exposure.
- Ready for Task Q04 (Release-candidate evidence and final P9 handoff).
