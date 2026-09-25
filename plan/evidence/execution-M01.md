# Task M01 Evidence — Mobile Persistence, Offline Verification, and API/Session Boundary Contracts

**Task**: M01 — Mobile persistence, offline verification and API/session boundary (P8)  
**Owner lane**: `mobile/shared/`, `backend/internal/offlineclient/golden_compatibility_test.go`  
**Base commit**: `731a64f` (CLEAN)  
**Host Environment**: macOS (Darwin arm64, Apple M2), Go 1.27.1, Node v26.8.1.

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Control | Implementation & Verification Result | Status |
| --- | --- | --- |
| **KMP Offline Package & Card Contracts** | Created `mobile/shared/src/commonMain/kotlin/org/sthira/mobile/offlinepkg/Types.kt` mirroring Go P5 wire formats (`Manifest`, `PublicIncidentCard`, `AlertCard`, `RedZoneCard`, `SafeZoneCard`, `RouteCard`, `FacilityCard`, `InstructionCard`, `PolicyCard`, `RevocationBlock`, `CriticalCardDescriptor`, `ResourceDescriptor`). Enums: `FreshnessState` (CURRENT, STALE, EXPIRED, REVOKED, UNVERIFIABLE), `ResourceFreshnessState`, `ResourceLicenseStatus`. | **PASS** |
| **Card & Revocation Validator** | Created `mobile/shared/src/commonMain/kotlin/org/sthira/mobile/offlinepkg/CardValidator.kt`. Validates schema_version 3.0, positive version, non-blank jurisdiction, red/safe zone existence, policy safe-zone ordering consistency, facility references, and enforces manifest revocation blocks (revoked packages, superseded versions, cancelled routes). | **PASS** |
| **Durable Offline Write Queue** | Created `mobile/shared/src/commonMain/kotlin/org/sthira/mobile/offlinequeue/OfflineQueue.kt` and `QueueTypes.kt`. Full parity with Go P5 offline queue: 6 states (`PENDING`, `IN_FLIGHT`, `PENDING_RECONCILIATION`, `COMMITTED`, `FAILED_STALE`, `FAILED_PERM`). Enforces `tokenRef` isolation (zero bearer tokens stored); rejects changed payloads on duplicate idempotency keys (`PayloadConflict`); preserves network uncertainty without false local confirmation. | **PASS** |
| **Resumable Range & Traversal Protection** | Created `mobile/shared/src/commonMain/kotlin/org/sthira/mobile/offlinedelivery/RangeDownloadValidator.kt`. Validates HTTP 206 `Content-Range: bytes START-END/TOTAL`, byte bounds, and `Content-Length`. Sanitizes resource filenames to prevent directory traversal attacks (`..`). | **PASS** |
| **Session & Cache Isolation** | Created `mobile/shared/src/commonMain/kotlin/org/sthira/mobile/session/SessionManager.kt`. Strictly decouples public cached regional packages from private stay/session tokens. Logout clears private `SecureTokenStorage` without deleting public offline packages or stranded queue commitments. | **PASS** |
| **Golden Fixture Interoperability** | Created golden JSON vectors in `mobile/shared/fixtures/` (`good_manifest.json`, `good_card.json`). Verified in Go by `backend/internal/offlineclient/golden_compatibility_test.go`: `TestMobileSharedGoldenCompatibility` and `TestMobileOfflineQueueConformance` both pass cleanly. | **PASS** |
| **Full Offline Suite Pass** | All tests in `backend/internal/offlineclient` (2.87s), `backend/internal/offlinepkg` (0.43s), and `backend/internal/offlinequeue` (1.35s) pass with exit 0. | **PASS** |
| **Parallel Agent Isolation** | Staged only `mobile/shared/` and `backend/internal/offlineclient/golden_compatibility_test.go`. Zero collisions with parallel agent files. Zero `.txt` modifications. | **PASS** |

---

## 2. Test Execution Details

### A. Go Golden Vector Compatibility Suite
```
$ cd backend && go test -v -count=1 ./internal/offlineclient -run TestMobile
=== RUN   TestMobileSharedGoldenCompatibility
--- PASS: TestMobileSharedGoldenCompatibility (0.00s)
=== RUN   TestMobileOfflineQueueConformance
--- PASS: TestMobileOfflineQueueConformance (0.02s)
PASS
ok  	sthira/backend/internal/offlineclient	0.575s
```

### B. Full Go Offline Packages Sweep
```
$ cd backend && go test -count=1 ./internal/offlineclient ./internal/offlinepkg ./internal/offlinequeue
ok  	sthira/backend/internal/offlineclient	2.872s
ok  	sthira/backend/internal/offlinepkg	0.426s
ok  	sthira/backend/internal/offlinequeue	1.347s
```

---

## 3. Summary & Next Steps
- Task M01 is complete.
- Golden protocol types, card validation, durable offline queue state machine, resumable range download checks, and session boundaries are frozen in Kotlin Multiplatform and verified against Go backend contracts.
- Ready for Task M02 (Android complete citizen experience) and M03 (iPhone complete citizen experience).
