# Task B02 Evidence — Compatibility, Persistence and Offline Continuity

**Task**: B02 — Compatibility, persistence and offline continuity  
**Owner lane**: `backend/internal/{store,httpserver,offlinequeue,offlinedelivery}`  
**Host Environment**: macOS (Darwin arm64), Go 1.27.1, local PostgreSQL 18.6 (`STHIRA_TEST_ADMIN_DSN="postgres://localhost:5432/postgres?sslmode=disable"`, `STHIRA_TEST_DSN="postgres://localhost:5432/postgres?sslmode=disable"`).

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Control | Implementation & Verification Result | Status |
| --- | --- | --- |
| **Trace current reservation operation/payload hashing and stored idempotency records** | `reservationPayloadHash` computes SHA-256 over `canonicalReservationPayload` (includes `PartySize`, `SnapshotVersion`). Legacy hash format `legacyReservationPayloadHash` (`sessID\|facID\|pkgID\|routeID\|startDate\|endDate\|idemKey`) traced and reproduced. | **PASS** |
| **Reproduce replay using a legacy persisted row** | Legacy row seeded with pipe-delimited payload hash and `COMPLETED` state; replay with identical parameters returns original `reservation_id` and `stay_id` without creating duplicate allocation. Verified in `TestB02_LegacyReplayPreservesSemanticsAndPreventsAliasing` and `TestB02_HTTPReservationLegacyReplayAndAliasingPrevention`. | **PASS** |
| **Define bounded compatibility policy & prevent aliasing** | In `IdempotencyStore.BeginWithCompat` and `stay_handlers.go`: legacy hash replay allowed only if stored reservation matches original session ID, facility ID, and party size. Replay with changed party size (aliasing attack on legacy hash) strictly rejected with `ErrPayloadConflict` (HTTP 409). | **PASS** |
| **Preserve committed replay even when later eligibility changes** | Replay of committed reservation returns existing confirmed reservation without re-validating live capacity, with caller authentication verified before disclosure. | **PASS** |
| **Current and legacy rows coexist** | Canonical JSON SHA-256 rows and legacy pipe SHA-256 rows coexist in `idempotency_keys` table without collision or aliasing. Verified in `TestB02_LegacyAndCanonicalIdempotencyCoexist`. | **PASS** |
| **Cross-process last-space / crash-after-commit tests** | Last capacity unit (=1): Winner commits reservation, client loses connection / crashes; replay returns stored result without decrementing capacity again (remaining=0). Concurrent loser receives capacity conflict (HTTP 409); capacity never becomes negative. Verified in `TestB02_LastSpaceCrashAfterCommit`. | **PASS** |
| **Live withdrawal reaches subsequent actual cached delivery across instances** | Suspending/quarantining source immediately causes `SnapshotRevalidate` and `GetCard` to fail closed across instances without serving stale cached content. Verified in `TestB02_LiveAuthorityWithdrawalFlowsIntoDelivery`. | **PASS** |
| **Interrupted resume validated (RFC 9110 HTTP 206)** | Partial range `bytes=0-49` (HTTP 206, `Content-Range: bytes 0-49/total`), resumed range `bytes=50-` (HTTP 206); reassembled content matches full card bit-for-bit. Out-of-bounds range `bytes=999999-` returns HTTP 416. Verified in `TestB02_HTTPInterruptedDownloadResumeRange206`. | **PASS** |
| **Pending queue survives restart & uncertain response reconciles** | `offlinequeue.Store` directory survives process restart; in-flight operation experiencing transport timeout moves to `PENDING_RECONCILIATION`; re-drain resends identical idempotency key and reconciles to `COMMITTED` with stored server result. Verified in `TestB02_HTTPOfflineQueueRestartAndUncertainReconcile`. | **PASS** |

---

## 2. Test Execution Details

### A. Store Continuity Suite (`backend/internal/store/b02_continuity_test.go`)
```text
$ STHIRA_TEST_ADMIN_DSN="postgres://localhost:5432/postgres?sslmode=disable" go test -v ./internal/store -run "TestB02_"
=== RUN   TestB02_LegacyAndCanonicalIdempotencyCoexist
--- PASS: TestB02_LegacyAndCanonicalIdempotencyCoexist (1.01s)
=== RUN   TestB02_LegacyReplayPreservesSemanticsAndPreventsAliasing
--- PASS: TestB02_LegacyReplayPreservesSemanticsAndPreventsAliasing (0.93s)
=== RUN   TestB02_LastSpaceCrashAfterCommit
--- PASS: TestB02_LastSpaceCrashAfterCommit (0.98s)
=== RUN   TestB02_LiveAuthorityWithdrawalFlowsIntoDelivery
--- PASS: TestB02_LiveAuthorityWithdrawalFlowsIntoDelivery (0.94s)
PASS
ok  	sthira/backend/internal/store	4.483s
```

### B. HTTP Idempotency Compatibility & Resume Suite (`backend/internal/httpserver/b02_idempotency_compat_test.go`)
```text
$ STHIRA_TEST_DSN="postgres://localhost:5432/postgres?sslmode=disable" go test -v ./internal/httpserver -run "TestB02_"
=== RUN   TestB02_HTTPReservationLegacyReplayAndAliasingPrevention
--- PASS: TestB02_HTTPReservationLegacyReplayAndAliasingPrevention (0.07s)
=== RUN   TestB02_HTTPInterruptedDownloadResumeRange206
--- PASS: TestB02_HTTPInterruptedDownloadResumeRange206 (0.01s)
=== RUN   TestB02_HTTPOfflineQueueRestartAndUncertainReconcile
--- PASS: TestB02_HTTPOfflineQueueRestartAndUncertainReconcile (0.03s)
PASS
ok  	sthira/backend/internal/httpserver	0.764s
```

### C. Full Backend Regression
```text
$ go test ./...
ok  	sthira/backend/internal/capfeed	(cached)
ok  	sthira/backend/internal/catalogue	(cached)
ok  	sthira/backend/internal/contracts	(cached)
ok  	sthira/backend/internal/drills	(cached)
ok  	sthira/backend/internal/httpjson	(cached)
ok  	sthira/backend/internal/httpserver	14.087s
ok  	sthira/backend/internal/offlineclient	(cached)
ok  	sthira/backend/internal/offlinedelivery	(cached)
ok  	sthira/backend/internal/offlinepkg	(cached)
ok  	sthira/backend/internal/offlinequeue	(cached)
ok  	sthira/backend/internal/offlineresources	(cached)
ok  	sthira/backend/internal/opkg	(cached)
ok  	sthira/backend/internal/orchestration	(cached)
ok  	sthira/backend/internal/scenarioprep	(cached)
ok  	sthira/backend/internal/sourceact	(cached)
ok  	sthira/backend/internal/store	0.448s
```

---

## 3. Commit Artifacts & Verification Summary

- `backend/internal/store/idempotency.go`: Added `BeginWithCompat` and `LegacyChecker` interface.
- `backend/internal/httpserver/stay_handlers.go`: Added `legacyReservationPayloadHash` and wired `BeginWithCompat` into `handleCreateReservation` with party size and facility validation.
- `backend/internal/store/b02_continuity_test.go`: 4 new automated store continuity and concurrency tests.
- `backend/internal/httpserver/b02_idempotency_compat_test.go`: 3 new automated HTTP integration tests.
- Classification: **ENGINEERING_VERIFIED**.
