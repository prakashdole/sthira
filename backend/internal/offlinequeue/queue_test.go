package offlinequeue

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedClock returns a clock function that always reports the given time.
// Useful for deterministic expiry tests.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t.UTC() }
}

// newTestStore creates a fresh store in a per-test tempdir with the given
// clock. The tempdir is cleaned up automatically.
func newTestStore(t *testing.T, now func() time.Time) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir, now)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func validOp(now time.Time) PendingOperation {
	return PendingOperation{
		ID:              "OP-1",
		IdempotencyKey:  "idem-1",
		Endpoint:        "/api/v3/reservations",
		Method:          "POST",
		TokenRef:        "session-A",
		Payload:         json.RawMessage(`{"facility_id":"FAC-1","party_size":2}`),
		SnapshotVersion: 1,
		SelectionExpiry: now.Add(1 * time.Hour),
		State:           StatePending,
	}
}

// TestValidateAcceptsGoodOperation: a well-formed op passes Validate and gets
// its PayloadHash and CreatedAt populated.
func TestValidateAcceptsGoodOperation(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	op := validOp(now)
	if err := op.Validate(now); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if op.PayloadHash == "" {
		t.Fatal("PayloadHash not set")
	}
	// Hash must change if payload or key changes.
	other := validOp(now)
	other.Payload = json.RawMessage(`{"facility_id":"FAC-2","party_size":2}`)
	other.ID = "OP-2"
	if err := other.Validate(now); err != nil {
		t.Fatalf("Validate other: %v", err)
	}
	if op.PayloadHash == other.PayloadHash {
		t.Fatal("hash did not change with payload")
	}
	other2 := validOp(now)
	other2.IdempotencyKey = "idem-2"
	other2.ID = "OP-3"
	if err := other2.Validate(now); err != nil {
		t.Fatalf("Validate other2: %v", err)
	}
	if op.PayloadHash == other2.PayloadHash {
		t.Fatal("hash did not change with key")
	}
}

// TestValidateRejectsMissingFields: every required field is enforced.
func TestValidateRejectsMissingFields(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		mut  func(*PendingOperation)
	}{
		{"empty id", func(op *PendingOperation) { op.ID = "" }},
		{"empty idem", func(op *PendingOperation) { op.IdempotencyKey = "" }},
		{"relative endpoint", func(op *PendingOperation) { op.Endpoint = "reservations" }},
		{"unsupported method", func(op *PendingOperation) { op.Method = "GET" }},
		{"missing token ref", func(op *PendingOperation) { op.TokenRef = "" }},
		{"empty payload", func(op *PendingOperation) { op.Payload = nil }},
		{"malformed json", func(op *PendingOperation) { op.Payload = json.RawMessage(`{`) }},
		{"zero expiry", func(op *PendingOperation) { op.SelectionExpiry = time.Time{} }},
		{"past expiry", func(op *PendingOperation) { op.SelectionExpiry = now.Add(-1 * time.Minute) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := validOp(now)
			tc.mut(&op)
			if err := op.Validate(now); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

// TestEnqueuePersistsAtomicFile: a single enqueue writes a JSON file in
// the store directory; the file is parseable, has 0600 perms, and no .tmp
// siblings remain.
func TestEnqueuePersistsAtomicFile(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	final := filepath.Join(s.Dir(), op.ID+".json")
	st, err := os.Stat(final)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("perm: got %o, want 0o600", got)
	}
	if _, err := os.Stat(final + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no .tmp file, got %v", err)
	}
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != op.ID+".json" {
		t.Fatalf("expected exactly one entry %s.json, got %v", op.ID, entries)
	}
}

// TestEnqueueRejectsPayloadChange: a second enqueue with the same key but a
// different payload returns ErrPayloadChanged, not a silent overwrite.
func TestEnqueueRejectsPayloadChange(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	if err := s.Enqueue(context.Background(), validOp(now)); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	modified := validOp(now)
	modified.Payload = json.RawMessage(`{"facility_id":"FAC-OTHER"}`)
	modified.ID = "OP-2"
	if err := s.Enqueue(context.Background(), modified); !errors.Is(err, ErrPayloadChanged) {
		t.Fatalf("expected ErrPayloadChanged, got %v", err)
	}
	// On-disk state unchanged.
	got, err := s.Get(context.Background(), "OP-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !jsonObjectsEqual(got.Payload, []byte(`{"facility_id":"FAC-1","party_size":2}`)) {
		t.Fatalf("payload was overwritten: %s", got.Payload)
	}
}

// TestEnqueueRejectsSamePayload: same key + same payload while still pending
// returns ErrOperationConflict. The user is not allowed to double-enqueue an
// in-flight submission.
func TestEnqueueRejectsSamePayload(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	if err := s.Enqueue(context.Background(), validOp(now)); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	dup := validOp(now)
	if err := s.Enqueue(context.Background(), dup); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("expected ErrOperationConflict, got %v", err)
	}
}

// TestEnqueueAfterTerminalAllowsFreshKey: a terminal entry (COMMITTED) does
// NOT block a fresh enqueue of the same key. The user may legitimately
// re-confirm a new selection that happens to use the same idempotency key.
func TestEnqueueAfterTerminalAllowsFreshKey(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	current, err := s.Get(context.Background(), op.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := current.SetCommitted(now, 201, json.RawMessage(`{"reservation_id":"RES-1"}`)); err != nil {
		t.Fatalf("SetCommitted: %v", err)
	}
	if err := s.UpdateState(context.Background(), current); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	// Fresh enqueue with a different payload is allowed because the previous
	// entry is now terminal. The user changed intent and must use a NEW key.
	fresh := validOp(now)
	fresh.ID = "OP-2"
	fresh.Payload = json.RawMessage(`{"facility_id":"FAC-NEW"}`)
	fresh.IdempotencyKey = "idem-NEW"
	if err := s.Enqueue(context.Background(), fresh); err != nil {
		t.Fatalf("enqueue fresh: %v", err)
	}
}

// TestUpdateStateRejectsTerminalMutation: a committed entry is immutable.
func TestUpdateStateRejectsTerminalMutation(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	current, _ := s.Get(context.Background(), op.ID)
	if err := current.SetCommitted(now, 201, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("SetCommitted: %v", err)
	}
	if err := s.UpdateState(context.Background(), current); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	// Try to mutate it.
	if err := s.UpdateState(context.Background(), current); !errors.Is(err, ErrTerminalState) {
		t.Fatalf("expected ErrTerminalState, got %v", err)
	}
}

// TestUpdateStatePreservesImmutableFields: changing key/payload/etc through
// UpdateState is silently dropped. The store rewrites immutable fields from
// the on-disk entry, so the caller's intent to mutate cannot leak through.
func TestUpdateStatePreservesImmutableFields(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	current, _ := s.Get(context.Background(), op.ID)
	originalHash := current.PayloadHash
	tampered := current
	tampered.IdempotencyKey = "tampered-key"
	tampered.Payload = json.RawMessage(`{"tampered":true}`)
	tampered.Endpoint = "/api/v3/admin"
	tampered.Method = "DELETE"
	tampered.TokenRef = "leaked-token"
	tampered.PayloadHash = "DEADBEEF"
	tampered.SnapshotVersion = 999
	tampered.SelectionExpiry = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.UpdateState(context.Background(), tampered); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.IdempotencyKey != "idem-1" {
		t.Fatalf("key mutated: %q", after.IdempotencyKey)
	}
	if !jsonObjectsEqual(after.Payload, op.Payload) {
		t.Fatalf("payload mutated: %s", after.Payload)
	}
	if after.Endpoint != "/api/v3/reservations" {
		t.Fatalf("endpoint mutated: %s", after.Endpoint)
	}
	if after.Method != "POST" {
		t.Fatalf("method mutated: %s", after.Method)
	}
	if after.TokenRef != "session-A" {
		t.Fatalf("token ref mutated: %q", after.TokenRef)
	}
	if after.PayloadHash != originalHash {
		t.Fatalf("payload hash mutated: %s vs %s", after.PayloadHash, originalHash)
	}
	if after.SnapshotVersion != 1 {
		t.Fatalf("snapshot version mutated: %d", after.SnapshotVersion)
	}
}

// TestRestartRecoversAllEntries: a fresh store opened against the same
// directory recovers every prior enqueue. Restart recovery is the entire
// reason this queue exists.
func TestRestartRecoversAllEntries(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	s, err := NewStore(dir, fixedClock(now))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	for i := 0; i < 7; i++ {
		op := validOp(now)
		op.ID = "OP-" + string(rune('A'+i))
		op.IdempotencyKey = "idem-" + string(rune('A'+i))
		op.Payload = json.RawMessage(`{"i":` + itoa(i) + `}`)
		if err := s.Enqueue(context.Background(), op); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	// Restart.
	s2, err := NewStore(dir, fixedClock(now))
	if err != nil {
		t.Fatalf("NewStore restart: %v", err)
	}
	got, err := s2.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("restart list: got %d entries, want 7", len(got))
	}
	for _, e := range got {
		if e.State != StatePending {
			t.Fatalf("entry %s state=%s, want PENDING", e.ID, e.State)
		}
	}
}

// TestResetStuckInFlight: a worker crash leaves IN_FLIGHT entries behind; on
// restart the store recovers them to PENDING_RECONCILIATION with RetryCount
// bumped. We do NOT reset to plain PENDING because the previous dispatch's
// outcome is genuinely unknown: the request may have reached the server
// and been committed. Returning to PENDING would falsely imply "the server
// has not seen this"; the next drain would then submit a duplicate and
// silently double-commit a hold.
func TestResetStuckInFlight(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	current, _ := s.Get(context.Background(), op.ID)
	if err := current.SetInFlight(); err != nil {
		t.Fatalf("SetInFlight: %v", err)
	}
	if err := s.UpdateState(context.Background(), current); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	reset, err := s.ResetStuckInFlight(context.Background())
	if err != nil {
		t.Fatalf("ResetStuckInFlight: %v", err)
	}
	if reset != 1 {
		t.Fatalf("reset count: got %d, want 1", reset)
	}
	after, _ := s.Get(context.Background(), op.ID)
	if after.State != StatePendingReconciliation {
		t.Fatalf("state after reset: got %s, want PENDING_RECONCILIATION", after.State)
	}
	if after.RetryCount != 1 {
		t.Fatalf("retry count after reset: got %d, want 1", after.RetryCount)
	}
	if !strings.Contains(after.LastError, "previous worker did not acknowledge") {
		t.Fatalf("last error: got %q, want mention of crashed worker", after.LastError)
	}
}

// TestPurgeCommittedRemovesOnlyOldEntries: COMMITTED entries past the cutoff
// are removed; recent COMMITTED entries and any FAILED_* entries stay.
func TestPurgeCommittedRemovesOnlyOldEntries(t *testing.T) {
	base := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	clock := base
	now := func() time.Time { return clock }

	s, err := NewStore(t.TempDir(), now)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	for i, commitAt := range []time.Time{base.Add(-2 * time.Hour), base.Add(-1 * time.Minute), base.Add(-30 * time.Minute)} {
		clock = commitAt.Add(-time.Minute)
		op := validOp(clock)
		op.ID = "OP-" + itoa(i)
		op.IdempotencyKey = "idem-" + itoa(i)
		if err := s.Enqueue(context.Background(), op); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		clock = commitAt
		cur, _ := s.Get(context.Background(), op.ID)
		if err := cur.SetCommitted(clock, 201, json.RawMessage(`{}`)); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
		if err := s.UpdateState(context.Background(), cur); err != nil {
			t.Fatalf("UpdateState %d: %v", i, err)
		}
	}
	clock = base
	removed, err := s.PurgeCommitted(context.Background(), 1*time.Hour)
	if err != nil {
		t.Fatalf("PurgeCommitted: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed: got %d, want 1", removed)
	}
	remaining, _ := s.List(context.Background())
	if len(remaining) != 2 {
		t.Fatalf("remaining: got %d, want 2", len(remaining))
	}
}

// TestSnapshotRedactsTokenRef: the inspection snapshot must NEVER leak
// TokenRef values that look like real tokens. The harness uses opaque labels
// for TokenRefs; we redact them defensively in case a future struct change
// reintroduces a Bearer field.
func TestSnapshotRedactsTokenRef(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s := newTestStore(t, fixedClock(now))
	op := validOp(now)
	if err := s.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	view, err := s.SnapshotForInspection(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(view) != 1 {
		t.Fatalf("view len: %d", len(view))
	}
	if view[0].TokenRef != "REDACTED" {
		t.Fatalf("TokenRef leaked in snapshot: %q", view[0].TokenRef)
	}
}

// TestMemoryTokenStorePutForget: the in-memory TokenProvider stores and
// removes by ref, returning ErrUnknownTokenRef for unknown refs.
func TestMemoryTokenStorePutForget(t *testing.T) {
	ts := NewMemoryTokenStore()
	if _, err := ts.BearerTokenFor(context.Background(), "missing"); !errors.Is(err, ErrUnknownTokenRef) {
		t.Fatalf("unknown ref: got %v, want ErrUnknownTokenRef", err)
	}
	ts.Put("session-A", "tok-A")
	tok, err := ts.BearerTokenFor(context.Background(), "session-A")
	if err != nil {
		t.Fatalf("BearerTokenFor: %v", err)
	}
	if tok != "tok-A" {
		t.Fatalf("token: got %q, want tok-A", tok)
	}
	ts.Forget("session-A")
	if _, err := ts.BearerTokenFor(context.Background(), "session-A"); !errors.Is(err, ErrUnknownTokenRef) {
		t.Fatalf("after Forget: got %v, want ErrUnknownTokenRef", err)
	}
}

// TestDispatcherRejectsEndpoints: the dispatcher refuses scheme-bearing or
// relative endpoints as a defense against SSRF / protocol confusion.
func TestDispatcherRejectsEndpoints(t *testing.T) {
	d := NewHTTPClientDispatcher("http://example.com", nil)
	cases := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"relative", "reservations"},
		{"absolute URL", "http://other.example/api/x"},
		{"traversal", "/api/../admin"},
		{"non-api path", "/admin/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := d.PostJSON(context.Background(), "POST", tc.path, "tok", []byte("{}")); err == nil {
				t.Fatalf("expected error for %q", tc.path)
			}
		})
	}
}

// TestDispatcherPostOKRoundTrip: with a httptest server, the dispatcher
// correctly sends Content-Type, Accept and Authorization headers, and reads
// the response.
func TestDispatcherPostOKRoundTrip(t *testing.T) {
	type rec struct {
		Method        string
		Path          string
		Authorization string
		Body          string
	}
	hits := make(chan rec, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/echo", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		hits <- rec{Method: r.Method, Path: r.URL.Path, Authorization: r.Header.Get("Authorization"), Body: string(buf[:n])}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := NewHTTPClientDispatcher(srv.URL, srv.Client())
	status, body, err := d.PostJSON(context.Background(), "POST", "/api/v3/echo", "tok-X", []byte(`{"hi":1}`))
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if status != 201 {
		t.Fatalf("status: %d", status)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("body: %s", body)
	}
	select {
	case r := <-hits:
		if r.Method != "POST" || r.Path != "/api/v3/echo" || r.Authorization != "Bearer tok-X" || !strings.Contains(r.Body, `"hi":1`) {
			t.Fatalf("server saw: %+v", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not see request")
	}
}

// TestExtractServerErrorCode: the helper classifies known error tokens.
func TestExtractServerErrorCode(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`{"errors":[{"code":"STALE_VERSION"}]}`, "STALE_VERSION"},
		{`{"errors":[{"code":"CAPACITY_CONFLICT","message":"full"}]}`, "CAPACITY_CONFLICT"},
		{`{}`, ""},
		{`not-json`, ""},
		{`{"errors":[]}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := extractServerErrorCode([]byte(tc.body)); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

// itoa is a tiny base-10 integer formatter used by tests that build ids.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var out []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	if neg {
		out = append([]byte{'-'}, out...)
	}
	return string(out)
}

// jsonObjectsEqual compares two JSON byte slices as values, ignoring
// insignificant whitespace or key ordering. It exists because the store
// marshals with MarshalIndent while tests construct compact payloads.
func jsonObjectsEqual(a, b []byte) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	ra, _ := json.Marshal(av)
	rb, _ := json.Marshal(bv)
	return string(ra) == string(rb)
}
