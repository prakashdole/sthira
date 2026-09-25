package offlinequeue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingDispatcher captures every dispatch the worker makes. It returns
// pre-programmed status codes / bodies / errors from the queue. The dispatcher
// is concurrency-safe; tests may schedule many calls per operation.
type recordingDispatcher struct {
	mu       sync.Mutex
	calls    []recordedCall
	respond  func(call int, status int, body string, err error) (int, []byte, error)
	received chan recordedCall
}

type recordedCall struct {
	Method string
	Path   string
	Token  string
	Body   []byte
}

func newRecordingDispatcher(t *testing.T) *recordingDispatcher {
	t.Helper()
	return &recordingDispatcher{
		received: make(chan recordedCall, 32),
		respond: func(call int, status int, body string, err error) (int, []byte, error) {
			return status, []byte(body), err
		},
	}
}

func (r *recordingDispatcher) PostJSON(ctx context.Context, method, path, token string, body []byte) (int, []byte, error) {
	r.mu.Lock()
	idx := len(r.calls)
	r.calls = append(r.calls, recordedCall{Method: method, Path: path, Token: token, Body: append([]byte(nil), body...)})
	respond := r.respond
	r.mu.Unlock()
	select {
	case r.received <- recordedCall{Method: method, Path: path, Token: token, Body: append([]byte(nil), body...)}:
	default:
	}
	if respond == nil {
		return 0, nil, errors.New("recordingDispatcher: no respond set")
	}
	// Default response is a valid /api/v3 success envelope for reservation.create
	// (request_id + data.reservation_id + data.stay_id). Tests that exercise
	// error paths or malformed envelopes override this via SetRespond.
	return respond(idx, 201, `{"request_id":"r1","schema_version":"3.0","generated_at":"2026-09-21T00:00:00Z","data_version":"PKG:1","source_status":"CURRENT","data":{"reservation_id":"RES-X","stay_id":"STAY-X"}}`, nil)
}

func (r *recordingDispatcher) callsSnapshot() []recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// noSleep returns immediately; tests do not need real backoff time.
func noSleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func newTestWorker(t *testing.T, d HTTPDispatcher, tokens TokenProvider, now func() time.Time) (*ReplayWorker, *Store) {
	t.Helper()
	s := newTestStore(t, now)
	w, err := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, now)
	if err != nil {
		t.Fatalf("NewReplayWorker: %v", err)
	}
	w.SetSleep(noSleep)
	return w, s
}

// TestSubmitCommits: a 2xx response marks the operation COMMITTED, captures
// the server response, and bumps nothing else.
func TestSubmitCommits(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, fixedClock(now))

	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	outcome, err := w.Submit(context.Background(), op)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if outcome != outcomeCommitted {
		t.Fatalf("outcome: got %v, want outcomeCommitted", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StateCommitted {
		t.Fatalf("state: got %s, want COMMITTED", after.State)
	}
	if after.LastStatusCode != 201 {
		t.Fatalf("status: got %d, want 201", after.LastStatusCode)
	}
	if string(after.ServerResult) == "" {
		t.Fatalf("server result not captured")
	}
}

// TestSubmitSameKeyReplayDoesNotDoubleCommit: after a successful commit the
// operation is terminal; a second Submit on the same entry is a no-op
// (outcomeSkipped) and the server is NOT contacted again.
func TestSubmitSameKeyReplayDoesNotDoubleCommit(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, fixedClock(now))

	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := w.Submit(context.Background(), op); err != nil {
		t.Fatalf("first Submit: %v", err)
	}
	firstCalls := len(d.callsSnapshot())
	outcome, err := w.Submit(context.Background(), op)
	if err != nil {
		t.Fatalf("second Submit: %v", err)
	}
	if outcome != outcomeSkipped {
		t.Fatalf("second outcome: got %v, want outcomeSkipped", outcome)
	}
	if len(d.callsSnapshot()) != firstCalls {
		t.Fatalf("second Submit contacted server (no double commitment)")
	}
}

// TestSubmitStaleSnapshot: a STALE_VERSION envelope classifies the entry as
// FAILED_STALE. The user must re-confirm with a new key.
func TestSubmitStaleSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	d := newRecordingDispatcher(t)
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 409, []byte(`{"errors":[{"code":"STALE_VERSION","message":"snapshot changed"}]}`), nil
	}
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, fixedClock(now))

	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomeStale {
		t.Fatalf("outcome: got %v, want outcomeStale", outcome)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StateFailedStale {
		t.Fatalf("state: got %s, want FAILED_STALE", after.State)
	}
	if after.LastStatusCode != 409 {
		t.Fatalf("status: got %d, want 409", after.LastStatusCode)
	}
}

// TestSubmitSelectionExpiredWithoutServerContact: an offline-too-long entry
// is FAILED_STALE locally without contacting the server. The server cannot
// be asked "is this still valid?"; the queue fails closed.
//
// The realistic sequence is: the client enqueued while the selection window
// was valid (clock at T-1h) and time has now passed (clock at T). We
// emulate that by enqueuing under one clock and submitting under a later one.
func TestSubmitSelectionExpiredWithoutServerContact(t *testing.T) {
	enqueueAt := time.Date(2026, 9, 20, 11, 0, 0, 0, time.UTC)
	nowAt := enqueueAt.Add(2 * time.Hour) // selection expired at 12:00
	clock := enqueueAt
	now := func() time.Time { return clock }

	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, now)

	op := validOp(enqueueAt)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Advance the clock past the selection expiry.
	clock = nowAt
	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomeStale {
		t.Fatalf("outcome: got %v, want outcomeStale", outcome)
	}
	if got := len(d.callsSnapshot()); got != 0 {
		t.Fatalf("server was contacted for expired selection: %d calls", got)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if !strings.Contains(after.LastError, ErrSelectionExpired.Error()) {
		t.Fatalf("last error: got %q want contains %q", after.LastError, ErrSelectionExpired.Error())
	}
}

// TestSubmitCapacityConflict: a CAPACITY_CONFLICT is permanent for THIS key.
// The worker does not retry; the user must re-confirm with a new key.
func TestSubmitCapacityConflict(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	d := newRecordingDispatcher(t)
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 409, []byte(`{"errors":[{"code":"CAPACITY_CONFLICT","message":"full"}]}`), nil
	}
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, fixedClock(now))

	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePerm {
		t.Fatalf("outcome: got %v, want outcomePerm", outcome)
	}
	if got := len(d.callsSnapshot()); got != 1 {
		t.Fatalf("capacity conflict should not be retried, got %d calls", got)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StateFailedPerm {
		t.Fatalf("state: got %s, want FAILED_PERM", after.State)
	}
}

// TestSubmitTransientRetriesThenReconciles: a 503 response is transient;
// the worker retries within budget, and on retry exhaustion the entry stays in
// PENDING_RECONCILIATION — NOT FAILED_PERM — because the server may have
// committed before going unhealthy. The next drain keeps trying with the
// same idempotency key; the server's idempotency store resolves the
// duplication. No silent success and no false definitive failure.
func TestSubmitTransientRetriesThenReconciles(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	d := newRecordingDispatcher(t)
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 503, []byte(`{"errors":[{"code":"INTERNAL","message":"try later"}]}`), nil
	}
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, fixedClock(now))

	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	outcome, _ := w.Submit(context.Background(), op)
	if outcome != outcomePendingReconciliation {
		t.Fatalf("outcome: got %v, want outcomePendingReconciliation after exhausted retries (5xx is uncertain, not perm)", outcome)
	}
	if got := len(d.callsSnapshot()); got != 3 {
		t.Fatalf("attempts: got %d, want 3 (MaxAttempts)", got)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePendingReconciliation {
		t.Fatalf("state: got %s, want PENDING_RECONCILIATION", after.State)
	}
	if after.RetryCount != 3 {
		t.Fatalf("retry count: got %d, want 3", after.RetryCount)
	}
}

// TestSubmitTokenMissingSkips: a missing token means the queue cannot
// submit; the entry is left PENDING for the next drain (no failure is
// recorded — token absence is a worker-level issue, not an operation one).
func TestSubmitTokenMissingSkips(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	// Token deliberately absent.
	w, s := newTestWorker(t, d, tokens, fixedClock(now))

	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	outcome, err := w.Submit(context.Background(), op)
	if err == nil {
		t.Fatalf("expected token lookup error")
	}
	if outcome != outcomeSkipped {
		t.Fatalf("outcome: got %v, want outcomeSkipped", outcome)
	}
	if got := len(d.callsSnapshot()); got != 0 {
		t.Fatalf("server contacted despite missing token: %d", got)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePending {
		t.Fatalf("state: got %s, want PENDING", after.State)
	}
}

// TestSubmitContextCancelStops: ctx cancellation between attempts returns
// promptly with the context error; the entry is left PENDING so a future
// drain can resume.
func TestSubmitContextCancelStops(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	d := newRecordingDispatcher(t)
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 0, nil, errors.New("canceled")
	}
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, fixedClock(now))

	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := w.Submit(ctx, op)
	if err == nil {
		t.Fatalf("expected error from canceled ctx")
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePending && after.State != StateInFlight {
		t.Fatalf("state: got %s, want PENDING or IN_FLIGHT", after.State)
	}
}

// TestDrainQueueProcessesAll: DrainQueue returns a report with the right
// counters after processing a mix of commits, stale and perm outcomes.
func TestDrainQueueProcessesAll(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	type outcome string
	cases := []struct {
		want outcome
		resp func(call int) (int, []byte, error)
	}{
		{"committed", func(call int) (int, []byte, error) {
			return 201, []byte(`{"request_id":"r1","data":{"reservation_id":"R","stay_id":"S"}}`), nil
		}},
		{"stale", func(call int) (int, []byte, error) {
			return 409, []byte(`{"errors":[{"code":"STALE_VERSION"}]}`), nil
		}},
		{"perm", func(call int) (int, []byte, error) {
			return 409, []byte(`{"errors":[{"code":"CAPACITY_CONFLICT"}]}`), nil
		}},
	}
	d := newRecordingDispatcher(t)
	var counter atomic.Int32
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		idx := int(counter.Add(1) - 1)
		return cases[idx%len(cases)].resp(idx / len(cases))
	}

	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, fixedClock(now))

	for i, tc := range cases {
		op := validOp(now)
		op.ID = fmt.Sprintf("OP-%d", i)
		op.IdempotencyKey = fmt.Sprintf("idem-%d", i)
		if err := s.Enqueue(context.Background(), op); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		_ = tc
	}
	rep, err := w.DrainQueue(context.Background())
	if err != nil {
		t.Fatalf("DrainQueue: %v", err)
	}
	if rep.Processed != 3 {
		t.Fatalf("processed: got %d, want 3", rep.Processed)
	}
	if rep.Committed != 1 || rep.Stale != 1 || rep.Failed != 1 {
		t.Fatalf("counters: %+v", rep)
	}
}

// TestRestartRecoversInflightAndContinues: simulate a worker crash by leaving
// IN_FLIGHT entries behind; on restart ResetStuckInFlight + DrainQueue picks
// them up and either commits or fails them.
func TestRestartRecoversInflightAndContinues(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	dir := t.TempDir()
	s, err := NewStore(dir, fixedClock(now))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	w, err := NewReplayWorker(s, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, fixedClock(now))
	if err != nil {
		t.Fatalf("NewReplayWorker: %v", err)
	}
	w.SetSleep(noSleep)

	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	cur, _ := s.Get(context.Background(), op.ID)
	if err := cur.SetInFlight(); err != nil {
		t.Fatalf("SetInFlight: %v", err)
	}
	if err := s.UpdateState(context.Background(), cur); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	// Simulate worker crash + restart: a new store, then ResetStuckInFlight.
	s2, err := NewStore(dir, fixedClock(now))
	if err != nil {
		t.Fatalf("restart NewStore: %v", err)
	}
	w2, err := NewReplayWorker(s2, d, tokens, ReplayConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: time.Second}, fixedClock(now))
	if err != nil {
		t.Fatalf("restart NewReplayWorker: %v", err)
	}
	w2.SetSleep(noSleep)
	reset, err := s2.ResetStuckInFlight(context.Background())
	if err != nil {
		t.Fatalf("ResetStuckInFlight: %v", err)
	}
	if reset != 1 {
		t.Fatalf("reset: got %d, want 1", reset)
	}
	rep, err := w2.DrainQueue(context.Background())
	if err != nil {
		t.Fatalf("DrainQueue: %v", err)
	}
	if rep.Committed != 1 {
		t.Fatalf("committed: got %d, want 1", rep.Committed)
	}
	after, _ := s2.Get(context.Background(), op.ID)
	if after.State != StateCommitted {
		t.Fatalf("state: got %s, want COMMITTED", after.State)
	}
	if after.RetryCount != 1 {
		t.Fatalf("retry count after reset: got %d, want 1", after.RetryCount)
	}
}

// TestPayloadImmutableAcrossRestart: write an op, restart the store, verify
// the payload bytes are identical (no drift from JSON re-marshal).
func TestPayloadImmutableAcrossRestart(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	s, err := NewStore(dir, fixedClock(now))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Inspect the on-disk file directly to prove the payload is durable.
	raw, err := readFile(t, filepath.Join(dir, op.ID+".json"))
	if err != nil {
		t.Fatalf("read on-disk: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("on-disk file is not valid JSON")
	}
	var decoded PendingOperation
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode on-disk: %v", err)
	}
	if !jsonObjectsEqual(decoded.Payload, op.Payload) {
		t.Fatalf("payload drift: %s vs %s", decoded.Payload, op.Payload)
	}
	// Restart.
	s2, err := NewStore(dir, fixedClock(now))
	if err != nil {
		t.Fatalf("restart NewStore: %v", err)
	}
	got, err := s2.Get(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("restart Get: %v", err)
	}
	if !jsonObjectsEqual(got.Payload, op.Payload) {
		t.Fatalf("payload drift after restart")
	}
	if got.IdempotencyKey != op.IdempotencyKey {
		t.Fatalf("idempotency key drift")
	}
	if got.TokenRef != op.TokenRef {
		t.Fatalf("token ref drift")
	}
}

func readFile(t *testing.T, path string) ([]byte, error) {
	t.Helper()
	// Local helper to avoid an extra import in queue_test.go.
	return readFileOS(path)
}

// TestUncertainOutcome_ReconciliationReplay: when a network drop occurs during
// submission, the operation transitions to StatePendingReconciliation.
// When subsequently replayed after local selection has expired, the worker
// does NOT fail stale locally; it contacts the server with the identical
// idempotency key and commits upon 2xx replay.
func TestUncertainOutcome_ReconciliationReplay(t *testing.T) {
	enqueueAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	clock := enqueueAt
	nowFn := func() time.Time { return clock }

	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, nowFn)

	op := validOp(enqueueAt)
	op.SelectionExpiry = enqueueAt.Add(15 * time.Minute)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// 1. First attempt fails due to connection drop / network error
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 0, nil, errors.New("connection reset by peer")
	}

	_, _ = w.Submit(context.Background(), op)

	// Verify the operation transitioned to StatePendingReconciliation
	afterDrop, err := s.Get(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if afterDrop.State != StatePendingReconciliation {
		t.Fatalf("state after drop: got %s, want %s", afterDrop.State, StatePendingReconciliation)
	}

	// 2. Advance clock by 1 hour (past SelectionExpiry of 15 min)
	clock = enqueueAt.Add(1 * time.Hour)

	// Server is now reachable and confirms the reservation (200 OK with a
	// valid /api/v3 success envelope so the worker can validate and commit).
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 200, []byte(`{"request_id":"r1","data":{"reservation_id":"RES-RECONCILED","stay_id":"STAY-RECONCILED"}}`), nil
	}

	outcome, err := w.Submit(context.Background(), afterDrop)
	if err != nil {
		t.Fatalf("Submit reconciliation: %v", err)
	}
	if outcome != outcomeCommitted {
		t.Fatalf("outcome: got %v, want outcomeCommitted", outcome)
	}

	afterReconcile, err := s.Get(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if afterReconcile.State != StateCommitted {
		t.Fatalf("state after reconciliation: got %s, want %s", afterReconcile.State, StateCommitted)
	}
}

// TestReconciliationSnapshotDriftFailsStale: the previous dispatch's outcome
// is unknown, the local selection has expired, but the server is reachable.
// The server returns 409 STALE_VERSION. The queue transitions the uncertain
// entry to FAILED_STALE — the queue must NOT fail-closed locally without
// asking the server, even though the local clock says the selection has
// expired.
func TestReconciliationSnapshotDriftFailsStale(t *testing.T) {
	enqueueAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	clock := enqueueAt
	nowFn := func() time.Time { return clock }

	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, nowFn)

	op := validOp(enqueueAt)
	op.SelectionExpiry = enqueueAt.Add(15 * time.Minute)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// 1. First Submit: transport failure → PENDING_RECONCILIATION.
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 0, nil, errors.New("connection reset by peer")
	}
	if _, err := w.Submit(context.Background(), op); err != nil {
		t.Fatalf("first Submit: %v", err)
	}

	// 2. Advance the clock past SelectionExpiry (15 min) by 1 hour.
	clock = enqueueAt.Add(1 * time.Hour)

	// 3. Server now reachable, returns 409 STALE_VERSION.
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 409, []byte(`{"errors":[{"code":"STALE_VERSION","message":"package expired"}]}`), nil
	}
	outcome, err := w.Submit(context.Background(), op)
	if err != nil {
		t.Fatalf("second Submit: %v", err)
	}
	if outcome != outcomeStale {
		t.Fatalf("outcome: got %v, want outcomeStale", outcome)
	}

	after, err := s.Get(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.State != StateFailedStale {
		t.Fatalf("state: got %s, want FAILED_STALE", after.State)
	}
	if after.LastStatusCode != 409 {
		t.Fatalf("status: got %d, want 409", after.LastStatusCode)
	}
	// The server must have been contacted even though the local selection
	// window had expired; an outcomeStale driven by a local clock signal
	// would have skipped the call.
	if got := len(d.callsSnapshot()); got != 2 {
		t.Fatalf("dispatch count: got %d, want 2 (first drop, second stale confirmation)", got)
	}
}

// TestReconciliationPermanentRejection: the previous dispatch's outcome is
// unknown; on replay the server definitively rejects the same idempotency
// key (e.g. an admin-issued IDEMPOTENCY_CONFLICT because the payload no
// longer matches what the server holds). The queue transitions the
// uncertain entry to FAILED_PERM.
func TestReconciliationPermanentRejection(t *testing.T) {
	enqueueAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	clock := enqueueAt
	nowFn := func() time.Time { return clock }

	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, nowFn)

	op := validOp(enqueueAt)
	op.SelectionExpiry = enqueueAt.Add(15 * time.Minute)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// 1. First Submit: connection drop → PENDING_RECONCILIATION.
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 0, nil, errors.New("connection reset by peer")
	}
	if _, err := w.Submit(context.Background(), op); err != nil {
		t.Fatalf("first Submit: %v", err)
	}

	// 2. Advance clock past SelectionExpiry.
	clock = enqueueAt.Add(1 * time.Hour)

	// 3. Server now returns 409 IDEMPOTENCY_CONFLICT.
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 409, []byte(`{"errors":[{"code":"IDEMPOTENCY_CONFLICT","message":"payload mismatch"}]}`), nil
	}
	outcome, err := w.Submit(context.Background(), op)
	if err != nil {
		t.Fatalf("second Submit: %v", err)
	}
	if outcome != outcomePerm {
		t.Fatalf("outcome: got %v, want outcomePerm", outcome)
	}

	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StateFailedPerm {
		t.Fatalf("state: got %s, want FAILED_PERM", after.State)
	}
	if after.LastStatusCode != 409 {
		t.Fatalf("status: got %d, want 409", after.LastStatusCode)
	}
}

// TestReconciliationRepeatedDrops: when reconciliation replays and the
// network is still down, the entry stays in PENDING_RECONCILIATION. The
// queue does NOT exhaust retries trying to "find out" — each retry would
// either hit the same outage or duplicate-commit on a previously-committed
// server-side hold.
func TestReconciliationRepeatedDrops(t *testing.T) {
	enqueueAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	clock := enqueueAt
	nowFn := func() time.Time { return clock }

	d := newRecordingDispatcher(t)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", "tok-A")
	w, s := newTestWorker(t, d, tokens, nowFn)

	op := validOp(enqueueAt)
	op.SelectionExpiry = enqueueAt.Add(15 * time.Minute)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Dispatcher always returns connection drop.
	d.respond = func(call int, status int, body string, err error) (int, []byte, error) {
		return 0, nil, errors.New("connection reset by peer")
	}

	// First Submit: PENDING_RECONCILIATION.
	if _, err := w.Submit(context.Background(), op); err != nil {
		t.Fatalf("first Submit: %v", err)
	}
	after1, _ := s.Get(context.Background(), op.ID)
	if after1.State != StatePendingReconciliation {
		t.Fatalf("after first drop: got %s, want PENDING_RECONCILIATION", after1.State)
	}
	firstCalls := len(d.callsSnapshot())

	// Second Submit (still down): must remain PENDING_RECONCILIATION,
	// must NOT have exhausted MaxAttempts retries (the queue should not
	// have made 3 dispatches for the same dropped call).
	if _, err := w.Submit(context.Background(), op); err != nil {
		t.Fatalf("second Submit: %v", err)
	}
	after2, _ := s.Get(context.Background(), op.ID)
	if after2.State != StatePendingReconciliation {
		t.Fatalf("after second drop: got %s, want PENDING_RECONCILIATION", after2.State)
	}
	if got := len(d.callsSnapshot()); got != firstCalls+1 {
		t.Fatalf("calls: got %d, want %d (one dispatch per Submit, no in-Submit retry storm)",
			got, firstCalls+1)
	}
	if after2.RetryCount <= after1.RetryCount {
		t.Fatalf("retry count must increase: got %d, was %d", after2.RetryCount, after1.RetryCount)
	}
}

// TestEnqueueRefusesReplacementWhileUncertain: while an entry is in
// PENDING_RECONCILIATION (or any non-terminal state), the queue MUST
// refuse to create a replacement operation under the same idempotency key,
// even after the local selection has expired. The user-visible rule: "do
// not create a replacement operation until the old outcome is resolved or
// handled through an explicit safe user workflow". The queue enforces this
// at the storage layer.
func TestEnqueueRefusesReplacementWhileUncertain(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	current, _ := s.Get(context.Background(), op.ID)
	if err := current.SetInFlight(); err != nil {
		t.Fatalf("SetInFlight: %v", err)
	}
	if err := current.SetPendingReconciliation(now, 0, "simulated drop"); err != nil {
		t.Fatalf("SetPendingReconciliation: %v", err)
	}
	if err := s.UpdateState(context.Background(), current); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	// Attempt to create a "replacement" operation with the same key while
	// the old one is still uncertain. The queue must refuse.
	replacement := validOp(now)
	replacement.ID = "OP-REPLACEMENT"
	if err := s.Enqueue(context.Background(), replacement); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("replacement enqueue: got %v, want ErrOperationConflict (uncertain prior submission is not terminal)", err)
	}
	// The on-disk entry is the original; no replacement was silently created.
	got, _ := s.Get(context.Background(), "OP-1")
	if got.State != StatePendingReconciliation {
		t.Fatalf("original entry state mutated: got %s", got.State)
	}
	list, _ := s.List(context.Background())
	if len(list) != 1 {
		t.Fatalf("entry count: got %d, want 1 (no silent replacement)", len(list))
	}
}
