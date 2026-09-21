# P5 uncertain-write outcome validation — handoff

**Owner:** Worker 3 (`p5-queue-final`)
**Base commit:** `2fe0fee` (coordinator freeze)
**Worktree:** `/private/tmp/mz-worktrees/p5-queue-final`

Lane focus: the queue must never fabricate success and never fabricate
definitive failure. Each dispatch's outcome must be classified by what
the wire actually shows — a 2xx envelope, a 4xx/5xx code, an auth
response, or an absence of any answer.

## What changed

### `backend/internal/offlinequeue/worker.go`

1. **2xx envelope validation.** A 2xx status is no longer sufficient to
   mark `COMMITTED`. The worker now calls `validateSuccessEnvelope` which
   decodes the body as `/api/v3` and requires:
   - valid JSON
   - no `errors[].code` field
   - non-null `data` JSON object
   - endpoint-specific identifiers:
     - `POST /api/v3/reservations` → `data.reservation_id` AND `data.stay_id`
     - `POST /api/v3/reservations/{id}/events` → `data.stay_id` AND a recognized `type` (ARRIVE|CANCEL|DEPART|EXTEND|TRANSFER)
   Any defect → `PENDING_RECONCILIATION`. We cannot prove the server
   committed without a trustworthy envelope; a false-positive "committed"
   here is the most expensive mistake the queue can make.

2. **5xx retry exhaustion → `PENDING_RECONCILIATION`, not `FAILED_PERM`.**
   The worker still retries 5xx within `MaxAttempts`. On exhaustion the
   entry stays `PENDING_RECONCILIATION` so the next drain can keep
   trying with the same idempotency key; the server's idempotency store
   resolves the duplication.

3. **401/403 → `PENDING_RECONCILIATION`, not `FAILED_PERM`.** Auth failures
   never mark `FAILED_PERM`. The original key is the only safe way to ask
   the server whether a prior attempt committed; telling the user to
   "submit a duplicate under a new key" would create a second reservation.
   The `LastError` field carries the auth code so a UI can surface a
   re-auth prompt without inventing a key.

4. **429 → retry within budget.** `429` (and the `RATE_LIMITED` envelope
   code) are transient; they route to the retry-within-budget branch.
   Budget exhaustion on transient client codes also routes to
   `PENDING_RECONCILIATION`, not `FAILED_PERM`.

5. **Stale codes:** `STALE_VERSION` and `EXPIRED` only. `VALIDATION_FAILED`
   is NOT stale-by-construction; it routes to `FAILED_PERM` so the user
   can inspect.

6. **Permanent codes:** the list still includes
   `IDEMPOTENCY_CONFLICT`, `CAPACITY_CONFLICT`, `NOT_FOUND`,
   `ROUTE_UNVERIFIED`, `ROUTE_UNAVAILABLE`, `AMBIGUOUS_PLACE`,
   `DATA_UNAVAILABLE`, `MODEL_UNAVAILABLE`, `LANGUAGE_UNSUPPORTED`,
   `MALFORMED_JSON`, `VALIDATION_FAILED`, `DUPLICATE_KEY`,
   `TRAILING_DATA`, `BODY_TOO_LARGE`, `DEPTH_EXCEEDED`,
   `UNKNOWN_FIELD`, `INVALID_VALUE`, `METHOD_NOT_ALLOWED`,
   `UNSUPPORTED_MEDIA_TYPE`. `CAPACITY_CONFLICT` is permanent because
   auto-retrying the same payload against the same committed hold cannot
   succeed; the user must inspect and re-confirm with a new selection.
   `FORBIDDEN` and `RATE_LIMITED` moved out — auth is its own
   `isAuthFailure` classifier and rate-limit is its own transient
   classifier.

7. **Selection-expired reconciliation preserved.** A
   `PENDING_RECONCILIATION` entry whose `SelectionExpiry` is in the past
   is STILL dispatched; only the server can answer whether the prior
   attempt committed. Confirmed by `TestSubmit_SelectionExpiredForReconciliationStillDispatches`.

8. **Crash-recovery semantics preserved.** `IN_FLIGHT` → `PENDING_RECONCILIATION`
   on `ResetStuckInFlight`. The next `Submit` re-marks the entry
   `IN_FLIGHT` (skipping `SetInFlight` when the entry is already
   `IN_FLIGHT` to avoid the self-conflict error). The next drain
   dispatches; the server's idempotency store resolves the duplication.

9. **Token non-persistence preserved.** `TokenRef` is the only token
   reference; the bytes never touch disk. `MemoryTokenStore` is in-memory
   only; the worker dies, the tokens vanish.

10. **Misleading retry comments updated.** The old comment "Retry-exhaustion
    is NOT a terminal failure here" was inconsistent with the code path
    that marked `FAILED_PERM` on exhaustion. The new doc-comment matches
    the new code: retry exhaustion preserves the entry in
    `PENDING_RECONCILIATION`.

### `backend/internal/offlinequeue/worker_test.go`

- Updated the default `recordingDispatcher` response to a valid `/api/v3`
  success envelope so existing tests that rely on the default
  (TestSubmitCommits, TestSubmitSameKeyReplayDoesNotDoubleCommit, …)
  keep working.
- Renamed `TestSubmitTransientRetriesThenPerm` →
  `TestSubmitTransientRetriesThenReconciles` and updated its assertions
  to match the new behavior: 503 retry exhaustion → `PENDING_RECONCILIATION`.
- Updated `TestDrainQueueProcessesAll` and `TestUncertainOutcome_ReconciliationReplay`
  to return valid envelopes on the success path.

### `backend/internal/offlinequeue/worker_regression_test.go` (new)

17 non-DB regression tests covering:
- `TestSubmit_Malformed200BodyNotCommitted`
- `TestSubmit_Empty200BodyNotCommitted`
- `TestSubmit_200WithErrorsArrayNotCommitted`
- `TestSubmit_MissingOperationSpecificIDsNotCommitted`
- `TestSubmit_5xxExhaustionStaysReconcilable`
- `TestSubmit_AuthFailureOnReconciliationKeepsKey`
- `TestSubmit_AuthRecoverySameKey` — same key + refresh token + replay → commit
- `TestSubmit_429IsRetryable`
- `TestSubmit_StaleDoesNotIncludeGenericValidation`
- `TestSubmit_SelectionExpiredWithoutServerContact`
- `TestSubmit_SelectionExpiredForReconciliationStillDispatches`
- `TestSubmit_TransportErrorDuringInflight`
- `TestSubmit_RecoveryInflightSameKeyCommits`
- `TestSubmit_PayloadImmutableAcrossRecovery`
- `TestSubmit_MissingTokenSkips`
- `TestSubmit_NonJSONSuccessBodyNotCommitted`
- `TestSubmit_ReservationCreateWithoutStayIDNotCommitted`

## Verification

```
go vet ./...                                              → clean
gofmt -l .                                                 → clean
go test ./internal/offlinequeue -count=1                   → ok (1.2s)
go test -tags integration ./internal/offlinequeue -count=1 → ok (9.0s; full HTTP suite over disposable PostgreSQL)
go test ./... -count=1 (full backend)                       → ok
```

No `.txt` files modified. No edits to shared files outside
`backend/internal/offlinequeue/`.

## What was NOT changed (and why)

- `cmd/sthira/main.go`: no production wiring changes. The queue's existing
  constructor (`NewReplayWorker`) keeps the same surface; nothing in
  the entrypoint or `httpserver` package needs to know about the new
  envelope validator.
- `backend/internal/offlinequeue/queue.go`: the state machine and
  immutability rules are unchanged. `PENDING_RECONCILIATION` is already
  reversible; the new validator lives entirely in the worker layer.
- `backend/internal/offlinequeue/store.go`: unchanged. `ResetStuckInFlight`
  and the durable on-disk format are unchanged.
- `backend/internal/offlinequeue/dispatcher.go`: unchanged. The
  dispatcher's job is to send and read raw bytes; the worker decides
  what the bytes mean.
- P5 wire formats: unchanged. The server's `/api/v3` envelope shape is
  the source of truth; the queue now actually reads what it claims to
  read.

## Integration deltas for the coordinator

None. The worker is internal to the `offlinequeue` package; the public
API of `ReplayWorker.Submit`, `DrainQueue`, and the `DrainReport`
counters is unchanged. `outcomeCommitted` / `outcomeStale` /
`outcomePerm` / `outcomeSkipped` / `outcomePendingReconciliation`
identifiers are preserved; only the conditions that produce them are
tightened.

The P5 acceptance matrix entry for "uncertain write outcome validation"
now has real-DB evidence via the existing
`TestHTTPReconciliationPreservesExactlyOneCommitment` integration
test (still green), plus the new
`backend/internal/offlinequeue/worker_regression_test.go` non-DB tests.
Lane evidence update only — no shared edits required.