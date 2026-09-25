# Task M04 Evidence — Scoped Operator Workflow, Privacy Controls, and Notification Integration

**Task**: M04 — Minimal operator workflow, privacy controls, and notification integration (P8)  
**Owner lane**: `frontend/v2/src/operator*`, `plan/drills/privacy-controls.md`, `plan/drills/notification-contracts.md`  
**Base commit**: `6be2f00` (CLEAN)  
**Host Environment**: macOS (Darwin arm64, Apple M2), Node v26.8.1, Go 1.27.1, PostgreSQL 18.0 (Homebrew).

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Control | Implementation & Verification Result | Status |
| --- | --- | --- |
| **Small Operator Web Surface** | Created `frontend/v2/src/operator.ts` mounted at `#operator` route in `frontend/v2/src/main.ts`. Provides clean administrative interface for scoped source status inspection, lifecycle transitions, quarantine, and stay capacity corrections. Does not leak admin controls into citizen flow. | **PASS** |
| **Honest Fail-Closed O14 Boundary** | Confirmed production operator issuance returns HTTP 503 `operator issuance unavailable: no trusted identity verifier configured`. Operator UI explicitly displays amber/red O14 status notice and blocks production minting. Allows test-isolated synthetic verifier (`X-Test-Operator-Subject`) for drill testing only. | **PASS** |
| **Scoped Source Transitions** | Implemented `validateSourceTransition` and `requestSourceTransition` for `OPERATIONAL`, `SUSPENDED`, and `RETIRED` states. Requires source ID, target state, reason, and idempotency key. Enforces dual confirmation dialog for consequential public guidance changes. | **PASS** |
| **Consequential Source Quarantine** | Implemented `validateSourceQuarantine` and `requestSourceQuarantine`. Requires detailed justification (>=10 chars) and confirmation dialog. Immediately restricts source from operational context resolution without deleting existing stays. | **PASS** |
| **Auditable Stay Correction** | Implemented `validateStayCorrection` and `requestStayCorrection`. Enforces positive integer, prohibits party size expansion (`new_party_size <= current`), and mandates substantive administrative rationale committed to the tamper-evident SHA-256 audit ledger. | **PASS** |
| **Privacy Controls (O10)** | Documented in `plan/drills/privacy-controls.md`: zero continuous background location tracking, foreground opt-in with immediate stop-sharing, zero raw audio retention, explicit touch-only arrival confirmation. Explicitly discloses that no remote continuous tracking dashboard is implemented. Clarifies statutory audit log retention vs ephemeral zero-retention citizen telemetry. | **PASS** |
| **Minimal Notification Contract (O12)** | Documented in `plan/drills/notification-contracts.md` and implemented in `validateNotificationPayload` and `revalidateNotification`. Rejects payloads bearing GPS coordinates or route polylines. Enforces client-side revalidation against fresh `/api/v3` server state before presenting alerts. Stale/revoked alerts are suppressed. | **PASS** |
| **APNs / FCM Provider Credentials** | Confirmed provider credentials reside strictly server-side. Local device push is recorded as `NOT_RUN` pending enterprise account provisioning (O12). | **NOT_RUN (Documented)** |
| **Test Coverage & Build** | All 21 frontend unit tests pass (13 operator tests + 8 journey tests). `npm run build` passes with exit code 0. All 19 backend operator HTTP integration tests pass on live PostgreSQL. | **PASS** |
| **Parallel Agent Isolation** | Staged only task-owned files (`frontend/v2/src/operator*`, `frontend/v2/package.json`, `frontend/v2/src/main.ts`, `plan/drills/privacy-controls.md`, `plan/drills/notification-contracts.md`, `plan/evidence/execution-M04.md`). Zero collisions with parallel agent's active files in `orchestration/`, `asrworker/`, `ttsworker/`, or `plan/evidence/execution-r00.md`. Zero `.txt` modifications. | **PASS** |

---

## 2. Test Execution Details

### A. Frontend Unit Test Suite (`npm test`)
```
$ cd frontend/v2 && npm test

> sthira-v2-ui@0.1.0 test
> node --experimental-strip-types --test "src/journey.test.ts" "src/operator.test.ts"

✔ computeDistanceMeters calculates accurate distance between two points (0.488709ms)
✔ evaluateProximity: detects citizen near destination with high accuracy and freshness (0.119625ms)
✔ evaluateProximity: rejects position if accuracy is too low/uncertain (> 100m) (0.068584ms)
✔ evaluateProximity: rejects stale position (> 30s old) (0.064167ms)
✔ evaluateProximity: marks en-route citizen outside proximity threshold (> 150m) (0.064666ms)
✔ transitionOnPosition: state changes appropriately between TRACKING and NEAR_DESTINATION (0.071458ms)
✔ transitionOnArrival: enforces explicit user arrival and strict idempotency (0.071583ms)
✔ transitionOnRevocation: immediately marks route revoked unless already arrived (0.049208ms)
✔ validateStayCorrection: rejects party size < 1 (0.527333ms)
✔ validateStayCorrection: rejects non-integer party size (0.070291ms)
✔ validateStayCorrection: rejects party size greater than current (cannot expand capacity) (0.056375ms)
✔ validateStayCorrection: rejects missing or too-short reason (0.105375ms)
✔ validateStayCorrection: accepts valid reduction with detailed reason (0.072167ms)
✔ validateSourceTransition: enforces non-empty source ID and allowed target states (0.079667ms)
✔ validateSourceTransition: rejects transition to identical state (0.060209ms)
✔ validateSourceTransition: accepts valid transition (0.053709ms)
✔ validateSourceQuarantine: enforces detailed consequential justification (0.078583ms)
✔ validateNotificationPayload: accepts minimal identifier payload (0.985209ms)
✔ validateNotificationPayload: rejects sensitive GPS coordinates and route geometries (O12) (0.100959ms)
✔ revalidateNotification: accepts fresh incident and rejects stale or revoked incident (0.116ms)
✔ requestOperatorSession: handles 503 fail-closed when no IdP configured (12.79575ms)
ℹ tests 21
ℹ suites 0
ℹ pass 21
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 98.851417
```

### B. Frontend Production Build (`npm run build`)
```
$ cd frontend/v2 && npm run build

> sthira-v2-ui@0.1.0 build
> tsc && vite build

vite v6.4.3 building for production...
✓ 20 modules transformed.
dist/index.html                                                            0.58 kB │ gzip:   0.35 kB
dist/assets/index-B98m57XD.css                                           122.96 kB │ gzip:  17.02 kB
dist/assets/index-CNP5WMTw.js                                          1,132.39 kB │ gzip: 307.18 kB
✓ built in 1.28s
```

### C. Backend Operator Integration Suite (PostgreSQL 18 + PostGIS 3.6)
```
$ cd backend && go test -v -count=1 ./internal/httpserver -run TestOperator
=== RUN   TestOperatorIssuanceFailsClosedNoVerifier
--- PASS: TestOperatorIssuanceFailsClosedNoVerifier (0.02s)
=== RUN   TestOperatorIssuanceRejectsBodyMFA
--- PASS: TestOperatorIssuanceRejectsBodyMFA (0.00s)
=== RUN   TestOperatorIssuanceNoGrant
--- PASS: TestOperatorIssuanceNoGrant (0.01s)
=== RUN   TestOperatorIssuanceMFARequired
--- PASS: TestOperatorIssuanceMFARequired (0.01s)
=== RUN   TestOperatorIssuanceInvalidEvidence
--- PASS: TestOperatorIssuanceInvalidEvidence (0.00s)
=== RUN   TestOperatorIssuanceDerivesJurisdiction
--- PASS: TestOperatorIssuanceDerivesJurisdiction (0.01s)
=== RUN   TestOperatorSessionRequiresMFAOnUse
--- PASS: TestOperatorSessionRequiresMFAOnUse (0.01s)
=== RUN   TestOperatorPublishRevokeSupersession
--- PASS: TestOperatorPublishRevokeSupersession (0.02s)
=== RUN   TestOperatorCrossJurisdictionDenied
--- PASS: TestOperatorCrossJurisdictionDenied (0.01s)
=== RUN   TestOperatorStayCorrectionAudit
--- PASS: TestOperatorStayCorrectionAudit (0.03s)
=== RUN   TestOperatorCorrectionCrossJurisdictionDenied
--- PASS: TestOperatorCorrectionCrossJurisdictionDenied (0.02s)
=== RUN   TestOperatorIdempotencyTargetBinding
--- PASS: TestOperatorIdempotencyTargetBinding (0.01s)
=== RUN   TestOperatorIdempotencyPayloadConflict
--- PASS: TestOperatorIdempotencyPayloadConflict (0.01s)
=== RUN   TestOperatorQuarantine
--- PASS: TestOperatorQuarantine (0.01s)
=== RUN   TestOperatorQuarantineCrossJurisdiction
--- PASS: TestOperatorQuarantineCrossJurisdiction (0.01s)
=== RUN   TestOperatorGrantRevokedDeniesOperationAndReplay
--- PASS: TestOperatorGrantRevokedDeniesOperationAndReplay (0.01s)
=== RUN   TestOperatorGrantExpiredDeniesSession
--- PASS: TestOperatorGrantExpiredDeniesSession (3.01s)
=== RUN   TestOperatorAuditResolvesToVerifiedIdentity
--- PASS: TestOperatorAuditResolvesToVerifiedIdentity (0.02s)
=== RUN   TestOperatorQuarantine_AbsentOrExpiredAuthz_Denied
--- PASS: TestOperatorQuarantine_AbsentOrExpiredAuthz_Denied (0.02s)
PASS
ok  	sthira/backend/internal/httpserver	3.923s
```

---

## 3. Summary & Next Steps
- Task M04 is verified and complete.
- Live operator authentication remains honestly fail-closed (`BLOCKED_EXTERNAL` under O14).
- Production push notifications remain `NOT_RUN` under O12; the client revalidation invariant and payload privacy boundaries are fully implemented and verified.
- Ready for mobile persistence / framework tasks (M00/M01) and drill execution (Q02).
