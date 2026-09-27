# Worker 17 Report — Cross-Citizen Regression Patch Review

**Timestamp:** 2026-09-27T16:NN UTC
**HEAD:** d95df9e609454877a91b7c82146b3c6368181b17
**Working Tree:** Dirty (stay_http_integration_test.go modified)
**Source Hashes:**
- `stay_http_integration_test.go` (dirty/working tree): `053ed7b9efa56f35bd3f4b7a2f6fe6651e02839de2af1de3ab98ae0f84c946ad`
- `stay_http_integration_test.go` (base/HEAD): `c3ebcf0...` (different from minimax-a's base `9698428f...` — the file changed between rounds)
- `citizen_ownership_regression_test.go` (unchanged from minimax-a's read): `2954eefe823c4d442223f92e515011b5826db3b30c2950544cce0d9d3fbe236e`

---

## 1. Scope Inspected

| File | SHA-256 | State |
|---|---|---|
| `backend/internal/httpserver/stay_http_integration_test.go` | `053ed7b9...` (dirty) | Has `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` |
| `backend/internal/httpserver/citizen_ownership_regression_test.go` | `2954eefe...` | Documentation-only skip test |
| `plan/worker-reports/round-2/minimax-a/candidate.patch` | — | Proposed adding the same test to same file |
| `plan/worker-reports/round-2/minimax-a/report.md` | — | Prior evidence |

---

## 2. Key Finding: Candidate Already Integrated

The `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` test is **already present** in `stay_http_integration_test.go` as an uncommitted dirty change (lines 287-332). The minimax-a candidate.patch does NOT need to be applied — the integration was done independently (likely by Opus or another worker).

**Working tree test (lines 287-332):**
```go
func TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives(t *testing.T) {
    s, st := newStayServer(t)
    srv := httptest.NewServer(s.Handler())
    defer srv.Close()
    pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
    _, tokenA := createSession(t, srv)
    _, tokenB := createSession(t, srv)

    // A creates reservation (idempotency key "res-a")
    recA := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", tokenA,
        reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "res-a", snap))
    // ... extracts created.StayID via decodeEnvelope + json.Marshal/Unmarshal

    // B attempts ARRIVE on A's stay with idempotency key "shared-key" → 403
    evB := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", tokenB,
        `{"type":"ARRIVE","idempotency_key":"shared-key"}`)
    require(t, evB.code == http.StatusForbidden)

    // A uses the same "shared-key" on their own stay → 200
    evA := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", tokenA,
        `{"type":"ARRIVE","idempotency_key":"shared-key"}`)
    require(t, evA.code == http.StatusOK)

    // Final capacity: held=0, occupied=1
    var held, occ int
    st.DB().QueryRowContext(...).Scan(&held, &occ)
    require(t, held == 0 && occ == 1)
}
```

**Working tree vs candidate.patch differences:**
- **Idempotency key naming**: Working tree uses `"res-a"` for reservation creation and `"shared-key"` for events. Candidate uses `"replay-key"` throughout. This is a naming difference only — both exercise the same B4 scenario.
- **Final assertion**: Working tree asserts `held=0 && occ=1` (tighter). Candidate asserts only `occ != 1`. The working tree version is STRONGER.
- **Structural**: Same test logic, same helpers, same two-session architecture.

---

## 3. Candidate.patch Assessment

### 3.1 Validation Against Existing Tests

**Helper semantics verified:**
- `newStayServer(t)` → creates `*Server` + `*testdb.T` with real PostgreSQL
- `seedHTTPPackageFacility(t, st, 3, ...)` → seeds facility with capacity 3, returns (pkgID, facID, snapVersion)
- `createSession(t, srv)` → creates citizen session, returns (sessionID, bearerToken) with real Authorization header
- `doAuthed(t, srv, method, path, token, body)` → makes HTTP request with `Authorization: Bearer <token>`, returns `*httpResponse`
- `decodeEnvelope(t, rec)` → decodes JSON envelope `{"status":..., "data":...}`
- `reservationBody(...)` → builds correctly-formed reservation payload

**Two real sessions:** ✅ tokenA and tokenB are separate sessions via `createSession` — different auth headers.

**Actual HTTP authorization:** ✅ `Authorization: Bearer <token>` header sent on all requests via `doAuthed`.

**B4 gap coverage:** ✅ This is the only test that exercises B4 (post-denial recovery of owner's idempotency key). No existing test covers this path.

**Duplicate test check:** ✅ No existing test named `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` in HEAD. The only other cross-session denial test is `TestHTTPCrossSessionDenial` (which stops at B's denial and does not continue to A's recovery).

### 3.2 Coverage Map Poseurs

`citizen_ownership_regression_test.go` (`TestCitizenOwnershipRegression_CoverageMap`):
- Skips when `STHIRA_TEST_DSN` is absent — zero assertions
- When `STHIRA_TEST_DSN` is set: only logs test names via `t.Log` — zero assertions
- **This is NOT a regression test.** It cannot fail and cannot catch regressions. It is a documentation index.

This was already noted in minimax-a's report and remains unaddressed. The file is not a duplicate test of `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` (different file, different purpose).

---

## 4. Specific Correction: Diagnostic Improvement

Per the prompt's specific correction:

> "It ignores JSON marshal/unmarshal errors and does not assert nonempty stay_id explicitly; determine the smallest diagnostic improvement."

**Analysis:**

The stay_id extraction code (working tree lines 304-308):
```go
var created struct {
    StayID string `json:"stay_id"`
}
b, _ := json.Marshal(decodeEnvelope(t, recA).Data)
_ = json.Unmarshal(b, &created)
```

The errors from `json.Marshal` and `json.Unmarshal` are discarded with `_`. The `created.StayID` is used immediately in the URL path for subsequent requests. If `created.StayID` were empty:
- The URL would be `/api/v3/reservations//events` (double slash)
- The server would likely return 404 (not the 200/403 we assert)
- The subsequent `held=0/occ=1` capacity check would fail or reference wrong facility

**Smallest diagnostic improvement:**
Assert `created.StayID != ""` after the unmarshal, before using it in the URL. This is a single `if` check and makes the implicit assumption explicit:

```go
b, mErr := json.Marshal(decodeEnvelope(t, recA).Data)
if mErr != nil {
    t.Fatalf("marshal envelope data: %v", mErr)
}
if uErr := json.Unmarshal(b, &created); uErr != nil {
    t.Fatalf("unmarshal stay_id: %v", uErr)
}
if created.StayID == "" {
    t.Fatalf("stay_id is empty — reservation creation response missing ID")
}
```

**Note on existing pattern:** This is consistent with how other tests in the file handle unmarshal errors (typically `t.Fatalf` on error). E.g., `TestHTTPCreateReservationAndRead` uses the same `created.StayID` pattern with error checking.

---

## 5. NO_CHANGE_NEEDED Assessment

**NOT APPROPRIATE — duplicate patch would be wrong.**

The test already exists in the working tree. Applying the candidate.patch again would create a duplicate function definition and cause a compilation error (`function already defined`). Opus should not apply the candidate.patch.

**The only warranted change** is the diagnostic improvement (assert non-empty stay_id + handle marshal/unmarshal errors), which is a single file edit to `stay_http_integration_test.go`.

---

## 6. Candidate.patch vs Working Tree Comparison

| Aspect | minimax-a candidate.patch | Working tree (already applied) |
|---|---|---|
| Test name | `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` | Same ✅ |
| Sessions | Two (`tokenA`, `tokenB`) | Same ✅ |
| HTTP auth | `doAuthed` with Bearer token | Same ✅ |
| B's ARRIVE → 403 | ✅ | Same ✅ |
| A's ARRIVE → 200 | ✅ | Same ✅ |
| Capacity assertion | `occ != 1` (weaker) | `held=0 && occ=1` (stronger) ✅ |
| stay_id non-empty | Not asserted | Not asserted |
| JSON errors | Ignored (`_`) | Ignored (`_`) |
| Idempotency keys | `"replay-key"` for reservation + events | `"res-a"` for reservation, `"shared-key"` for events |

**Verdict:** Working tree version is strictly better (stronger capacity assertion). No re-application needed.

---

## 7. Integration Prerequisites and Test Name

**Test name:** `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives`

**Prerequisites:**
```bash
export STHIRA_TEST_DSN="postgres://user:pass@localhost:5432/sthira_test?sslmode=disable"
cd backend/internal/httpserver
go test -v -count=1 -run 'TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives' ./internal/httpserver/
```

**Expected result (before any fix to marshal handling):**
- Test should PASS with current working tree code (B gets 403, A gets 200, held=0/occupied=1)
- The diagnostic improvement is optional — the test is functionally correct without it

**Existing related tests (for full ownership suite):**
```bash
go test -v -count=1 -run 'TestHTTPCrossSessionDenial|TestHTTPDuplicateConfirmationReplay|TestB02_HTTPReservationLegacyReplayAndAliasingPrevention|TestHTTPRequiresSession|TestHTTPCreateReservationAndRead|TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives' ./internal/httpserver/
```

---

## 8. Status

**Status: READY_FOR_REVIEW_UNVERIFIED**

The `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` test from minimax-a's candidate.patch is already present in the working tree version of `stay_http_integration_test.go` (dirty change, not yet committed). The working tree version is functionally equivalent but with a stronger final assertion (`held=0 && occ=1` vs `occ != 1`).

The only warranted improvement is adding explicit `stay_id` non-empty assertion and proper handling of `json.Marshal`/`json.Unmarshal` errors — a minimal ~5 line change to the test.

**Do NOT apply minimax-a/candidate.patch** — it would create a duplicate function definition.

**Exact next action for Opus:**
1. Verify `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` in working tree matches the described behavior
2. Optionally add the `created.StayID` non-empty assertion and marshal error handling as the diagnostic improvement
3. Do NOT apply `plan/worker-reports/round-2/minimax-a/candidate.patch` — the test is already integrated
