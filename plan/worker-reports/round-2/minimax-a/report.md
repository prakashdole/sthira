# MiniMax A — Citizen Ownership Evidence Closure
## Round 2 Assignment Report

**Timestamp:** 2026-09-27
**HEAD:** d95df9e (docs: record handoff commit id)
**Worker:** MiniMax A
**Output dir:** plan/worker-reports/round-2/minimax-a/

---

## 1. Scope Inspected

### Files Read (source-backed evidence)
| File | SHA-256 |
|------|---------|
| backend/internal/httpserver/citizen_ownership_regression_test.go | 2954eefe823c4d442223f92e515011b5826db3b30c2950544cce0d9d3fbe236e |
| backend/internal/httpserver/stay_http_integration_test.go | 9698428f9c681197167821b4d8b5487aca8e62c134dbc41b5fcc3281c382e18e |
| backend/internal/httpserver/b02_idempotency_compat_test.go | 98bbee5a78a4514347be29160d3110ba2417b001e0fd249b23bf017981c326d6 |
| backend/internal/store/idempotency.go | 54114bdc0ff8b170e396875c89fbc24615f63fa08432c2c963f72fbaf093a4ad |
| backend/internal/httpserver/stay_handlers.go | 7b745f4ff74368d44b5c944fec0f75e769cf5922a5127cba7b6460a3d1f588cf |

### Dirty/Untracked Files (pre-existing, not modified)
- `backend/internal/asrworker/audio_mime_test.go` — dirty
- `backend/internal/middleworker/eval/corpus.go` — dirty
- `backend/internal/middleworker/eval/main.go` — dirty
- `backend/internal/middleworker/eval/synthetic_v2.jsonl` — dirty
- `backend/internal/middleworker/eval/main_test.go` — dirty
- `backend/internal/ttsworker/audio_mime_test.go` — dirty
- `frontend/v2/src/styles.css` — dirty
- `backend/internal/httpserver/citizen_ownership_regression_test.go` — **untracked** (new file, this assignment's subject)
- `plan/prompts/` — untracked
- `plan/worker-reports/` — untracked

---

## 2. Existing Completed Work vs New Contribution

### Existing Work
- `citizen_ownership_regression_test.go` — documentation anchor with full coverage map, written by prior worker (round 1). It lists all existing real-DB tests covering each ownership/idempotency case and skips when STHIRA_TEST_DSN is absent.

### New Contribution
This round identified two specific gaps in the coverage map that are not explicitly verified by any existing test assertion:

1. **B4 gap**: No test asserts that session A can successfully use an idempotency key after session B's cross-session denial (B's ARRIVE/CANCEL on A's stay returns 403, then A's own event with the same idempotency key succeeds).
2. **C1/C2 cross-session isolation**: The existing `TestB02_HTTPReservationLegacyReplayAndAliasingPrevention` partially covers C1 but only asserts that B does NOT get A's reservation ID; it does not assert B's outcome (new reservation vs rejection) or that A's idempotency key remains usable by A after B's different-session attempt.

---

## 3. Source-Backed Coverage Table

### A — Owner Read / Cross-Session Read Denial

| ID | Claim | Existing Test | Assertion | Verified |
|----|-------|--------------|-----------|----------|
| A1 | Owner can read their own reservation | `TestHTTPCreateReservationAndRead` (stay_http_integration_test.go:190) | HTTP 200 + `Cache-Control: no-store` header + `state=RESERVED` | ✓ |
| A2 | Foreign session GET → 403 | `TestHTTPCrossSessionDenial` (stay_http_integration_test.go:275) | HTTP 403 | ✓ |
| A3 | No token → 401 | `TestHTTPRequiresSession` (stay_http_integration_test.go:233) | HTTP 401 for missing and garbage token | ✓ |
| A4 | No private leak on foreign GET | `handleGetReservation` (stay_handlers.go:512-514) | `if st.SessionID != sess.SessionID` → "not the owning session" 403 | ✓ (source) |

### B — Cross-Session Event Denial, State Conservation, Post-Denial Recovery

| ID | Claim | Existing Test | Assertion | Verified |
|----|-------|--------------|-----------|----------|
| B1 | Foreign session ARRIVE → 403 | `TestHTTPCrossSessionDenial` (stay_http_integration_test.go:280) | HTTP 403 on ARRIVE event | ✓ |
| B2 | Foreign session CANCEL → 403 | `TestHTTPCrossSessionDenial` (stay_http_integration_test.go:280) | HTTP 403 on CANCEL event | ✓ |
| B3 | No state change on denial | ownership check inside `InTx` (stay_handlers.go:597-603) | `SELECT session_id FROM stays WHERE stay_id=$1` → `errNotOwner` returned, no state mutation | ✓ (source) |
| B4 | A's event key works after B denied | — | **GAP**: no explicit assertion that A's idempotency key remains usable after B's rejection | **MISSING** |

### C — Idempotency Scope Isolation

| ID | Claim | Existing Test | Assertion | Verified |
|----|-------|--------------|-----------|----------|
| C1 | B cannot replay A's idempotency key | `TestB02_HTTPReservationLegacyReplayAndAliasingPrevention` (b02_idempotency_compat_test.go:107-119) | B's OK response does NOT return A's resID | Partial — B outcome not asserted |
| C2 | B cannot poison A's idempotency key | idempotency.go:47 composite key `(scope, operation, idem_key)` | Different scope → INSERT succeeds, no conflict | ✓ (source) |
| C3 | Same-session replay is consistent | `TestHTTPDuplicateConfirmationReplay` (stay_http_integration_test.go:289) | HTTP 200 + same stay ID both times + `held=1` | ✓ |
| C4 | Changed payload → 409 | `TestB02_HTTPReservationLegacyReplayAndAliasingPrevention` (b02_idempotency_compat_test.go:94) | HTTP 409 `ErrIdempotencyConflict` | ✓ |

### D — No Client-Supplied Identity Override

| ID | Claim | Existing Test | Assertion | Verified |
|----|-------|--------------|-----------|----------|
| D1 | `createReservationRequest` has no session_id/jurisdiction/role | stay_handlers.go:247-258 struct definition | No such fields in struct | ✓ (source) |
| D1 | `stayEventRequest` has no session_id/jurisdiction/role | stay_handlers.go:527-534 struct definition | No such fields in struct | ✓ (source) |

---

## 4. Findings

### Finding 1 — `citizen_ownership_regression_test.go` is a documentation-only skip (NOT a regression test)

The file comment explicitly states: *"This test skips when STHIRA_TEST_DSN is not set. Coverage map: A1...D1. To run the full ownership/idempotency suite: go test -v -count=1 -run '...' ./internal/httpserver/"*

The `TestCitizenOwnershipRegression_CoverageMap` function:
- When STHIRA_TEST_DSN is unset: skips without running any assertions
- When STHIRA_TEST_DSN is set: only emits `t.Log` lines describing other tests — it runs zero assertions of its own

**Impact:** This file is not a regression test. It cannot fail and cannot catch regressions. It only lists test names.

**Recommendation:** The file has documentation value as a coverage map/index but should either:
- (a) Be renamed to `citizen_ownership_coverage_map.go` (non-test file) so it cannot be run as a test, OR
- (b) Keep as-is with a clear comment that it is not a test and will always pass/skip

### Finding 2 — B4 (post-denial recovery) is not explicitly asserted (consequential gap)

No existing test verifies that after session B's cross-session event is denied with 403, session A can successfully use their idempotency key to operate on the same stay.

**Root cause:** `TestHTTPCrossSessionDenial` ends at line 285 after B's denial. It does not continue to verify A's subsequent successful use of the idempotency key.

**Proposed correction:** Add a new test `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` that:
1. A creates reservation with idempotency key `"replay-key"`
2. B attempts ARRIVE on A's stay with same idempotency key → 403
3. A issues ARRIVE on their own stay with `"replay-key"` → 200 (replay from A's idempotency scope)
4. A's arrive is acknowledged without double-decrement of capacity

This exercises the actual production path: the idempotency store key `(A_session, "stay.arrive", "replay-key")` is independent of B's failed `(B_session, ...)` INSERT attempt.

### Finding 3 — C1 cross-session isolation lacks B's outcome assertion (minor gap)

`TestB02_HTTPReservationLegacyReplayAndAliasingPrevention` (b02_idempotency_compat_test.go:107-119) checks that B's OK response does NOT return A's reservation ID, but does not assert whether B's request succeeded as a new reservation or was rejected. The test accepts any non-200 response or any 200 that doesn't return A's IDs. This is acceptable as a security-focused assertion (correct behavior: no leak), but does not fully characterize B's outcome.

**Impact:** Low — the critical property (no cross-session data disclosure) is verified. B's outcome (new reservation vs capacity conflict) depends on availability and is orthogonal to the security property.

---

## 5. Candidate Patch

A narrowly scoped Go integration test patch is provided at:

`plan/worker-reports/round-2/minimax-a/candidate.patch`

The patch adds one new test `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` to `stay_http_integration_test.go`. The test exercises the B4 gap using existing helpers (`newStayServer`, `seedHTTPPackageFacility`, `createSession`, `doAuthed`, `decodeEnvelope`) and existing infrastructure (real DB via STHIRA_TEST_DSN).

**Base hashes (unchanged):**
```
stay_http_integration_test.go  9698428f9c681197167821b4d8b5487aca8e62c134dbc41b5fcc3281c382e18e
b02_idempotency_compat_test.go  98bbee5a78a4514347be29160d3110ba2417b001e0fd249b23bf017981c326d6
idempotency.go                 54114bdc0ff8b170e396875c89fbc24615f63fa08432c2c963f72fbaf093a4ad
stay_handlers.go              7b745f4ff74368d44b5c944fec0f75e769cf5922a5127cba7b6460a3d1f588cf
```

**Integration dependencies:** None new — uses existing `newStayServer` pattern, existing `STHIRA_TEST_DSN` environment, existing `reservationBody` and `doAuthed` helpers.

**What the patch does NOT change:**
- No modification to production code
- No changes to existing test assertions
- No changes to the coverage-map documentation file
- Does not address the documentation-only nature of `citizen_ownership_regression_test.go`

---

## 6. Verification Plan

**Source inspection performed:** Yes — stay_handlers.go:247-534 (request structs), stay_handlers.go:375 (idempotency scope call), stay_handlers.go:512-514 (owner check), stay_handlers.go:597-603 (ownership inside InTx), idempotency.go:44-62 (composite key structure).

**Execution tests: NOT_RUN — deferred by user (usage reset in ~30 min).**

### Post-reset commands and prerequisites

**Prerequisites:**
```bash
# Requires a seeded PostgreSQL instance
# Set STHIRA_TEST_DSN pointing to it
export STHIRA_TEST_DSN="postgres://user:pass@localhost:5432/sthira_test?sslmode=disable"
```

**Run the full ownership/idempotency suite:**
```bash
cd backend/internal/httpserver
go test -v -count=1 -run 'TestHTTPCrossSessionDenial|TestHTTPDuplicateConfirmationReplay|TestB02_HTTPReservationLegacyReplayAndAliasingPrevention|TestHTTPRequiresSession|TestHTTPCreateReservationAndRead|TestHTTPStaleSnapshotRejected|TestHTTPCapacityConflictNoSubstitution|TestHTTPArriveDepartFlow' ./internal/httpserver/
```

**Run the new B4 gap test (after patch applied):**
```bash
cd backend/internal/httpserver
go test -v -count=1 -run 'TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives' ./internal/httpserver/
```

**Expected outcome for new test:**
```
=== RUN   TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives
    --- PASS: TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives
```

**Expected negative control:** If the idempotency scope isolation were broken and B could interfere with A's key, the test would fail at step 3 (A's replay would return an error or a different stay ID).

---

## 7. Status

**Status: READY_FOR_REVIEW_UNVERIFIED**

### Recommendation
The `citizen_ownership_regression_test.go` file should be retained as documentation. A decision should be made whether to rename it to avoid confusion (it is not a test that can fail). The B4 gap patch is ready for Opus to review and integrate. The two existing gaps (B4, C1 partial) are the minimal remaining surface; all other claims in the coverage map are substantiated by source + existing tests.

### Exact Next Action for Opus
1. Review `plan/worker-reports/round-2/minimax-a/candidate.patch`
2. Apply patch if accepted, or file issue to address B4 gap separately
3. Consider renaming `citizen_ownership_regression_test.go` to `citizen_ownership_coverage_map.go` to eliminate confusion about its nature (not a test)

---

## Artifact Paths

| Artifact | Path |
|----------|------|
| This report | `plan/worker-reports/round-2/minimax-a/report.md` |
| Candidate patch | `plan/worker-reports/round-2/minimax-a/candidate.patch` |
| Coverage map (unchanged) | `backend/internal/httpserver/citizen_ownership_regression_test.go` |
