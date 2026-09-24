package offlinequeue

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// outcomeTestHelpers — small helpers to keep the regression bodies focused.

// enqueueValidOp enqueues a deterministic reservation.create operation whose
// selection window has not yet expired, so the worker actually dispatches.
func enqueueValidOp(t *testing.T, s *Store, d HTTPDispatcher, tokens TokenProvider, now time.Time) PendingOperation {
	t.Helper()
	tokensForTest(t, tokens)
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	return op
}

func tokensForTest(t *testing.T, p TokenProvider) {
	t.Helper()
	if mt, ok := p.(*MemoryTokenStore); ok {
		mt.Put("session-A", "tok-A")
	}
}

// scriptedDispatcher returns a dispatcher whose response depends on the call
// index. Each scheduled (status, body, err) tuple is consumed in order.
type scriptedDispatcher struct {
	calls []recordedCall
	steps []scriptStep
}

type scriptStep struct {
	status int
	body   string
	err    error
}

func scripted(steps ...scriptStep) *scriptedDispatcher {
	return &scriptedDispatcher{steps: steps}
}

func (d *scriptedDispatcher) PostJSON(_ context.Context, method, path, token string, body []byte) (int, []byte, error) {
	d.calls = append(d.calls, recordedCall{Method: method, Path: path, Token: token, Body: append([]byte(nil), body...)})
	idx := len(d.calls) - 1
	if idx >= len(d.steps) {
		// Default: 201 success with a synthetic but valid reservation.create
		// envelope, so a test that schedules too few steps still terminates.
		return 201, []byte(`{"request_id":"r1","data":{"reservation_id":"RES-1","stay_id":"STAY-1"}}`), nil
	}
	s := d.steps[idx]
	return s.status, []byte(s.body), s.err
}

// successfulReservationCreateBody builds a valid /api/v3 success envelope for a
// reservation.create endpoint. Tests use it as the "known-good" baseline.
func successfulReservationCreateBody() string {
	return `{"request_id":"r1","schema_version":"3.0","generated_at":"2026-09-21T00:00:00Z","data_version":"PKG:1","source_status":"CURRENT","data":{"reservation_id":"RES-1","stay_id":"STAY-1"}}`
}

// === Regressions ===

// TestSubmit_Malformed200BodyNotCommitted: a 2xx response whose body does not
// parse as a valid /api/v3 success envelope must NOT mark COMMITTED; the
// operation moves to PENDING_RECONCILIATION because we cannot prove the server
// committed or rejected.
func TestSubmit_Malformed200BodyNotCommitted(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	d := scripted(scriptStep{status: 200, body: "this-is-not-json", err: nil})
	w, err := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewReplayWorker: %v", err)
	}
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, err := w.Submit(context.Background(), op)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation (malformed 2xx is uncertain)", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePendingReconciliation {
		t.Fatalf("state: got %s, want PENDING_RECONCILIATION", after.State)
	}
	if after.RetryCount != 1 {
		t.Errorf("retry count: got %d, want 1", after.RetryCount)
	}
}

// TestSubmit_Empty200BodyNotCommitted: a 2xx response with no body (e.g., a
// proxy that swallowed the body before forwarding) must NOT mark COMMITTED.
// PENDING_RECONCILIATION, because the server MAY have committed and the body
// was lost; we cannot tell without another dispatch.
func TestSubmit_Empty200BodyNotCommitted(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	d := scripted(scriptStep{status: 200, body: "", err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePendingReconciliation {
		t.Fatalf("state: got %s, want PENDING_RECONCILIATION", after.State)
	}
}

// TestSubmit_200WithErrorsArrayNotCommitted: a 2xx response that contains an
// errors[].code is NOT a success; the worker must NOT mark COMMITTED. The
// queue must preserve the operation so a follow-up dispatch (or a manual
// reconcile) can learn the server's verdict.
func TestSubmit_200WithErrorsArrayNotCommitted(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	body := `{"request_id":"r1","schema_version":"3.0","data":{"x":1},"errors":[{"code":"AMBIGUOUS_PLACE","message":"ambiguous","field":"place_id","retryable":false}]}`
	d := scripted(scriptStep{status: 200, body: body, err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation (200 with errors[])", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State == StateCommitted {
		t.Fatalf("must not be COMMITTED; 200 with errors[] is not a success")
	}
}

// TestSubmit_MissingOperationSpecificIDsNotCommitted: a 200 envelope whose
// data field is non-null but missing operation-specific identifiers
// (reservation_id + stay_id for reservation.create) must not mark COMMITTED.
func TestSubmit_MissingOperationSpecificIDsNotCommitted(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	body := `{"request_id":"r1","data":{"only_garbage":1}}`
	d := scripted(scriptStep{status: 200, body: body, err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation", outcome)
	}
}

// TestSubmit_5xxExhaustionStaysReconcilable: a sequence of 5xx/proxy failures
// that exhausts the retry budget must NOT mark FAILED_PERM; the operation
// stays in PENDING_RECONCILIATION because the server MAY have committed and
// we cannot prove the failure was definitive. The next drain keeps trying.
func TestSubmit_5xxExhaustionStaysReconcilable(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	steps := make([]scriptStep, 5)
	for i := range steps {
		steps[i] = scriptStep{status: 502, body: "bad gateway", err: nil}
	}
	d := scripted(steps...)
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation on 5xx exhaustion", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePendingReconciliation {
		t.Fatalf("state: got %s, want PENDING_RECONCILIATION", after.State)
	}
	if after.RetryCount < 3 {
		t.Errorf("retry count: got %d, want >=3", after.RetryCount)
	}
}

// TestSubmit_AuthFailureOnReconciliationKeepsKey: a 401/403 returned during a
// reconciliation dispatch must NOT mark FAILED_PERM; doing so would tell the
// user to "retry with a new key", which would create a duplicate
// reservation/capacity on the server. Instead the entry stays in
// PENDING_RECONCILIATION with an auth marker so a UI can prompt for re-auth
// and replay with the same key.
func TestSubmit_AuthFailureOnReconciliationKeepsKey(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	body := `{"request_id":"r1","errors":[{"code":"FORBIDDEN","message":"session expired","retryable":false}]}`
	d := scripted(scriptStep{status: 401, body: body, err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation (auth failure must NOT be perm)", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePendingReconciliation {
		t.Fatalf("state: got %s, want PENDING_RECONCILIATION", after.State)
	}
	if !strings.Contains(after.LastError, "FORBIDDEN") && !strings.Contains(after.LastError, "auth") {
		t.Errorf("LastError must surface the auth code, got %q", after.LastError)
	}
}

// TestSubmit_AuthRecoverySameKey: after an auth failure, refreshing the token
// and replaying the SAME operation id + key must succeed and produce a
// committed entry. The user is NEVER told to invent a new key.
func TestSubmit_AuthRecoverySameKey(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	d := scripted(
		scriptStep{status: 401, body: `{"errors":[{"code":"FORBIDDEN","message":"expired"}]}`, err: nil},
		scriptStep{status: 201, body: successfulReservationCreateBody(), err: nil},
	)
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	// First attempt: auth failure → PENDING_RECONCILIATION.
	outcome1, _ := w.Submit(context.Background(), op)
	if outcome1 != outcomePendingReconciliation {
		t.Fatalf("first outcome: got %v, want outcomePendingReconciliation", outcome1)
	}

	// Refresh the token; same operation id + key.
	tokens.Put("session-A", "tok-A-refreshed")

	// Re-load the on-disk op (id, key, payload immutable).
	after, _ := s.Get(context.Background(), op.ID)
	outcome2, _ := w.Submit(context.Background(), after)
	if outcome2 != outcomeCommitted {
		t.Fatalf("recovery outcome: got %v, want outcomeCommitted", outcome2)
	}
	final, _ := s.Get(context.Background(), op.ID)
	if final.State != StateCommitted {
		t.Fatalf("final state: got %s, want COMMITTED", final.State)
	}
}

// TestSubmit_429IsRetryable: HTTP 429 (rate-limited) is a transient 4xx; the
// worker must NOT mark FAILED_PERM on a single 429. It must retry within
// budget. (Previously RATE_LIMITED was in the permanent set, which
// contradicted the transient nature of a 429.)
func TestSubmit_429IsRetryable(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	rateLimitBody := `{"errors":[{"code":"RATE_LIMITED","message":"slow down","retryable":true}]}`
	d := scripted(
		scriptStep{status: 429, body: rateLimitBody, err: nil},
		scriptStep{status: 429, body: rateLimitBody, err: nil},
		scriptStep{status: 201, body: successfulReservationCreateBody(), err: nil},
	)
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomeCommitted {
		t.Fatalf("outcome: got %v, want outcomeCommitted after 429 retries recover", outcome)
	}
}

// TestSubmit_StaleDoesNotIncludeGenericValidation: VALIDATION_FAILED is NOT
// automatically a stale-selection signal. A generic validation error must
// remain FAILED_PERM so the user can inspect and correct the request — only
// STALE_VERSION and EXPIRED are stale-by-construction.
func TestSubmit_StaleDoesNotIncludeGenericValidation(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	body := `{"errors":[{"code":"VALIDATION_FAILED","message":"missing field"}]}`
	d := scripted(scriptStep{status: 400, body: body, err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePerm {
		t.Fatalf("outcome: got %v, want outcomePerm (VALIDATION_FAILED is permanent, not stale)", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StateFailedPerm {
		t.Fatalf("state: got %s, want FAILED_PERM", after.State)
	}
}

// TestSubmit_SelectionExpiredWithoutServerContact: a PENDING entry whose
// selection_expiry has lapsed by the worker's clock must transition to
// FAILED_STALE without contacting the server.
func TestSubmit_SelectionExpiredWithoutServerContact(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	// The store's Enqueue refuses a past selection, so we enqueue with a
	// future expiry and advance the clock past it before Submit.
	enqAt := now.Add(-1 * time.Hour) // 11:00
	s := newTestStore(t, func() time.Time { return enqAt })
	tokens := NewMemoryTokenStore()
	d := &silentDispatcher{}
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)

	op := validOp(enqAt)
	op.SelectionExpiry = enqAt.Add(10 * time.Minute) // 11:10
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomeStale {
		t.Fatalf("outcome: got %v, want outcomeStale", outcome)
	}
	if d.calls.Load() != 0 {
		t.Errorf("server must not be contacted for expired selection, got %d calls", d.calls.Load())
	}
}

// TestSubmit_SelectionExpiredForReconciliationStillDispatches: a
// PENDING_RECONCILIATION entry whose selection_expiry has lapsed by the
// worker's clock must STILL be dispatched. We cannot prove the hold expired
// without asking the server, and the server's idempotency store resolves the
// duplicate. Confirms the prompt's "selection-expired reconciliation"
// preservation.
func TestSubmit_SelectionExpiredForReconciliationStillDispatches(t *testing.T) {
	enqAt := time.Date(2026, 9, 21, 11, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return enqAt })
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	d := scripted(scriptStep{status: 201, body: successfulReservationCreateBody(), err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)

	op := validOp(enqAt)
	op.SelectionExpiry = enqAt.Add(10 * time.Minute)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Manually move it to PENDING_RECONCILIATION (simulating a previous
	// transport failure).
	got, _ := s.Get(context.Background(), op.ID)
	if err := got.SetPendingReconciliation(enqAt, 0, "simulated prior drop"); err != nil {
		t.Fatalf("SetPendingReconciliation: %v", err)
	}
	if err := s.UpdateState(context.Background(), got); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomeCommitted {
		t.Fatalf("outcome: got %v, want outcomeCommitted (reconciliation must dispatch even when local selection looks expired)", outcome)
	}
}

// TestSubmit_TransportErrorDuringInflight: a transport-level error (network
// drop, context timeout) during dispatch leaves the operation in
// PENDING_RECONCILIATION, NOT FAILED_PERM, so the next drain can replay with
// the same idempotency key.
func TestSubmit_TransportErrorDuringInflight(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	d := scripted(scriptStep{status: 0, body: "", err: errors.New("connection reset by peer")})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePendingReconciliation {
		t.Fatalf("state: got %s, want PENDING_RECONCILIATION", after.State)
	}
}

// TestSubmit_RecoveryInflightSameKeyCommits: after a worker crash leaves an
// IN_FLIGHT entry on disk, the next worker (ResetStuckInFlight) moves it to
// PENDING_RECONCILIATION; the next Submit dispatches it; the server's
// idempotency store resolves the duplicate and returns success. The user
// sees a single committed reservation, not a duplicate.
func TestSubmit_RecoveryInflightSameKeyCommits(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Simulate "first worker crashed mid-flight": mark IN_FLIGHT on disk.
	got, _ := s.Get(context.Background(), op.ID)
	if err := got.SetInFlight(); err != nil {
		t.Fatalf("SetInFlight: %v", err)
	}
	if err := s.UpdateState(context.Background(), got); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}

	// New worker boots; the dispatcher now returns a valid success envelope.
	d := scripted(scriptStep{status: 201, body: successfulReservationCreateBody(), err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	reset, err := s.ResetStuckInFlight(context.Background())
	if err != nil {
		t.Fatalf("ResetStuckInFlight: %v", err)
	}
	if reset != 1 {
		t.Fatalf("ResetStuckInFlight: got %d, want 1", reset)
	}

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomeCommitted {
		t.Fatalf("outcome: got %v, want outcomeCommitted", outcome)
	}
	final, _ := s.Get(context.Background(), op.ID)
	if final.State != StateCommitted {
		t.Fatalf("state: got %s, want COMMITTED", final.State)
	}
}

// TestSubmit_PayloadImmutableAcrossRecovery: the worker's recovery path must
// not silently rewrite the identity the user confirmed (idempotency key,
// endpoint, method, snapshot version, selection expiry, payload hash). The
// payload bytes themselves are reformatted by the store (json.MarshalIndent);
// the hash and key are the durable identity. Confirms that a recovered entry
// replays with the SAME identity the user confirmed.
func TestSubmit_PayloadImmutableAcrossRecovery(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	op := validOp(now)
	originalKey := op.IdempotencyKey
	originalEndpoint := op.Endpoint
	originalMethod := op.Method
	originalSnapshot := op.SnapshotVersion
	originalExpiry := op.SelectionExpiry
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// The store's Enqueue populates PayloadHash from (key + payload); capture
	// the persisted hash now so we can compare after Submit.
	enqueued, _ := s.Get(context.Background(), op.ID)
	originalHash := enqueued.PayloadHash
	d := scripted(scriptStep{status: 201, body: successfulReservationCreateBody(), err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	if _, err := w.Submit(context.Background(), op); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	after, _ := s.Get(context.Background(), op.ID)
	if after.IdempotencyKey != originalKey {
		t.Errorf("idempotency key changed: %q -> %q", originalKey, after.IdempotencyKey)
	}
	if after.PayloadHash != originalHash {
		t.Errorf("payload hash changed: %q -> %q", originalHash, after.PayloadHash)
	}
	if after.Endpoint != originalEndpoint {
		t.Errorf("endpoint changed: %q -> %q", originalEndpoint, after.Endpoint)
	}
	if after.Method != originalMethod {
		t.Errorf("method changed: %q -> %q", originalMethod, after.Method)
	}
	if after.SnapshotVersion != originalSnapshot {
		t.Errorf("snapshot version changed: %d -> %d", originalSnapshot, after.SnapshotVersion)
	}
	if !after.SelectionExpiry.Equal(originalExpiry) {
		t.Errorf("selection expiry changed: %v -> %v", originalExpiry, after.SelectionExpiry)
	}
}

// silentDispatcher satisfies HTTPDispatcher without speaking. Used in tests
// that want to assert the worker never contacts the server.
type silentDispatcher struct{ calls atomicInt }

type atomicInt struct{ v int }

func (a *atomicInt) Inc()      { a.v++ }
func (a *atomicInt) Load() int { return a.v }

func (s *silentDispatcher) PostJSON(_ context.Context, _, _, _ string, _ []byte) (int, []byte, error) {
	s.calls.Inc()
	return 0, nil, errors.New("silent dispatcher must not be called")
}

// TestSubmit_MissingTokenSkips: a queued operation whose TokenRef cannot be
// resolved is skipped (not dispatched); no retry, no definitive failure.
func TestSubmit_MissingTokenSkips(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore() // empty
	d := &silentDispatcher{}
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomeSkipped {
		t.Fatalf("outcome: got %v, want outcomeSkipped", outcome)
	}
	if d.calls.Load() != 0 {
		t.Errorf("server must not be contacted when token is missing, got %d calls", d.calls.Load())
	}
}

// TestSubmit_NonJSONSuccessBodyNotCommitted: defensive — body that parses as
// valid JSON but is not the /api/v3 envelope shape (e.g., an HTML 200 from an
// upstream proxy) must not mark COMMITTED.
func TestSubmit_NonJSONSuccessBodyNotCommitted(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	d := scripted(scriptStep{status: 200, body: `<html><body>ok</body></html>`, err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation (non-JSON 200)", outcome)
	}
}

// TestSubmit_ReservationCreateWithoutStayIDNotCommitted: an envelope that
// passes the basic shape check but is missing the operation-specific
// stay_id must not mark COMMITTED; the user's intended reservation is not
// fully represented in the response.
func TestSubmit_ReservationCreateWithoutStayIDNotCommitted(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, func() time.Time { return now })
	tokens := NewMemoryTokenStore()
	body := `{"request_id":"r1","data":{"reservation_id":"RES-1"}}` // missing stay_id
	d := scripted(scriptStep{status: 200, body: body, err: nil})
	w, _ := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, func() time.Time { return now })
	w.SetSleep(noSleep)
	op := enqueueValidOp(t, s, d, tokens, now)

	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation", outcome)
	}
}

func TestValidateSuccessEnvelope_StatusRules(t *testing.T) {
	goodBody := []byte(`{"request_id":"r1","data":{"reservation_id":"RES-1","stay_id":"STAY-1"}}`)
	// 200 and 201 are accepted
	if ok, _, reason := validateSuccessEnvelope("POST", "/api/v3/reservations", 200, goodBody, nil); !ok {
		t.Fatalf("expected status 200 accepted, got %s", reason)
	}
	if ok, _, reason := validateSuccessEnvelope("POST", "/api/v3/reservations", 201, goodBody, nil); !ok {
		t.Fatalf("expected status 201 accepted, got %s", reason)
	}
	// 202 is rejected (asynchronous/non-terminal)
	if ok, _, _ := validateSuccessEnvelope("POST", "/api/v3/reservations", 202, goodBody, nil); ok {
		t.Fatal("expected status 202 rejected")
	}
	// 204 is rejected
	if ok, _, _ := validateSuccessEnvelope("POST", "/api/v3/reservations", 204, goodBody, nil); ok {
		t.Fatal("expected status 204 rejected")
	}
}

func TestValidateSuccessEnvelope_Allowlist(t *testing.T) {
	body := []byte(`{"request_id":"r1","data":{"foo":"bar"}}`)
	// Non-allowlisted endpoints rejected
	for _, badPath := range []string{"/api/v3/places/resolve", "/api/v3/guidance/query", "/api/v3/unknown", "/api/v3/reservations/events"} {
		if ok, _, _ := validateSuccessEnvelope("POST", badPath, 200, body, nil); ok {
			t.Fatalf("expected endpoint %s rejected", badPath)
		}
	}
}

func TestValidateSuccessEnvelope_DuplicateKeysAndTrailingData(t *testing.T) {
	// Duplicate key in top-level
	dupTop := []byte(`{"request_id":"r1","data":{"reservation_id":"R","stay_id":"S"},"request_id":"r2"}`)
	if ok, _, reason := validateSuccessEnvelope("POST", "/api/v3/reservations", 200, dupTop, nil); ok {
		t.Fatalf("expected duplicate key rejected, got success")
	} else if !strings.Contains(reason, "duplicate key") {
		t.Fatalf("expected duplicate key reason, got %q", reason)
	}

	// Duplicate key inside data
	dupInner := []byte(`{"request_id":"r1","data":{"reservation_id":"R","stay_id":"S","stay_id":"S2"}}`)
	if ok, _, reason := validateSuccessEnvelope("POST", "/api/v3/reservations", 200, dupInner, nil); ok {
		t.Fatalf("expected duplicate key inside data rejected, got success")
	} else if !strings.Contains(reason, "duplicate key") {
		t.Fatalf("expected duplicate key reason, got %q", reason)
	}

	// Trailing data
	trailing := []byte(`{"request_id":"r1","data":{"reservation_id":"R","stay_id":"S"}} trailing`)
	if ok, _, reason := validateSuccessEnvelope("POST", "/api/v3/reservations", 200, trailing, nil); ok {
		t.Fatalf("expected trailing data rejected, got success")
	} else if !strings.Contains(reason, "trailing data") {
		t.Fatalf("expected trailing data reason, got %q", reason)
	}
}

func TestValidateSuccessEnvelope_StayEventsBindingAndTransfer(t *testing.T) {
	path := "/api/v3/reservations/STAY-99/events"
	submittedPayload := []byte(`{"type":"TRANSFER","new_facility_id":"FAC-2","snapshot_version":1}`)

	// Mismatched stay_id
	badStayID := []byte(`{"request_id":"r1","data":{"stay_id":"STAY-MISMATCH","type":"TRANSFER","new_stay_id":"STAY-100","new_reservation_id":"RES-100"}}`)
	if ok, _, reason := validateSuccessEnvelope("POST", path, 200, badStayID, submittedPayload); ok {
		t.Fatal("expected stay_id mismatch rejection")
	} else if !strings.Contains(reason, "does not match path stay_id") {
		t.Fatalf("unexpected reason: %s", reason)
	}

	// Mismatched event type
	mismatchedType := []byte(`{"request_id":"r1","data":{"stay_id":"STAY-99","type":"ARRIVE"}}`)
	if ok, _, reason := validateSuccessEnvelope("POST", path, 200, mismatchedType, submittedPayload); ok {
		t.Fatal("expected event type mismatch rejection")
	} else if !strings.Contains(reason, "does not match submitted payload type") {
		t.Fatalf("unexpected reason: %s", reason)
	}

	// TRANSFER missing new_stay_id
	missingNewStay := []byte(`{"request_id":"r1","data":{"stay_id":"STAY-99","type":"TRANSFER","new_reservation_id":"RES-100"}}`)
	if ok, _, reason := validateSuccessEnvelope("POST", path, 200, missingNewStay, submittedPayload); ok {
		t.Fatal("expected transfer missing new_stay_id rejection")
	} else if !strings.Contains(reason, "transfer event response missing") {
		t.Fatalf("unexpected reason: %s", reason)
	}

	// TRANSFER missing new_reservation_id
	missingNewRes := []byte(`{"request_id":"r1","data":{"stay_id":"STAY-99","type":"TRANSFER","new_stay_id":"STAY-100"}}`)
	if ok, _, reason := validateSuccessEnvelope("POST", path, 200, missingNewRes, submittedPayload); ok {
		t.Fatal("expected transfer missing new_reservation_id rejection")
	} else if !strings.Contains(reason, "transfer event response missing") {
		t.Fatalf("unexpected reason: %s", reason)
	}

	// TRANSFER complete with replacement IDs
	goodTransfer := []byte(`{"request_id":"r1","data":{"stay_id":"STAY-99","type":"TRANSFER","new_stay_id":"STAY-100","new_reservation_id":"RES-100"}}`)
	if ok, _, reason := validateSuccessEnvelope("POST", path, 200, goodTransfer, submittedPayload); !ok {
		t.Fatalf("expected valid transfer accepted, got %s", reason)
	}

	// ARRIVE complete
	arrivePayload := []byte(`{"type":"ARRIVE","stay_id":"STAY-99"}`)
	goodArrive := []byte(`{"request_id":"r1","data":{"stay_id":"STAY-99","type":"ARRIVE"}}`)
	if ok, _, reason := validateSuccessEnvelope("POST", path, 200, goodArrive, arrivePayload); !ok {
		t.Fatalf("expected valid arrive accepted, got %s", reason)
	}
}
