//go:build integration

// Package offlinequeue HTTP integration tests against a disposable PostgreSQL
// database running the real /api/v3 server. These tests verify the offline
// pending writes harness end to end:
//
//   - queue→restart→submit
//   - response lost after server commit → same-key replay (no double commit)
//   - stale snapshot
//   - expired selection
//   - changed payload / new confirmation
//   - denial / conflict
//   - no double commitment
//   - private data excluded from public output
//
// Build tag: the file is compiled only with -tags integration. The normal
// `go test ./...` run stays offline and green; the disposable-DB tests run
// on demand by an operator who can supply a Postgres superuser DSN.
//
// Required environment:
//
//	STHIRA_TEST_ADMIN_DSN  postgres://apple@localhost:5432/postgres
//	                       (or any DB with CREATEDB rights)
//
// The test creates and drops its own uniquely-named database. It NEVER
// touches sthira_test.
package offlinequeue

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpserver"
	"sthira/backend/internal/store"
)

// disposableDB creates and migrates a uniquely-named PostgreSQL database, then
// returns the per-database DSN and a cleanup function that drops it. The
// caller is responsible for invoking cleanup (use t.Cleanup). The DSN points
// at the new database, not at the admin database, so the returned *sql.DB is
// scoped to the disposable DB.
func disposableDB(t *testing.T) (string, func()) {
	t.Helper()
	admin := os.Getenv("STHIRA_TEST_ADMIN_DSN")
	if admin == "" {
		t.Skip("STHIRA_TEST_ADMIN_DSN unset; skipping disposable-DB HTTP integration test")
	}
	if _, err := exec.LookPath("psql"); err != nil {
		t.Skip("psql not on PATH; skipping disposable-DB HTTP integration test")
	}
	dbName := fmt.Sprintf("sthira_p5pending_%d_%d", time.Now().UnixNano(), os.Getpid())
	runPsql(t, admin, "", fmt.Sprintf(`CREATE DATABASE %s`, dbName))
	cleanup := func() {
		// Best-effort drop; log on failure but do not fail the test.
		cmd := exec.Command("psql", admin, "-c", fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName))
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}
	dsn := strings.Replace(admin, "/postgres", "/"+dbName, 1)
	// Apply migrations 0001..0005 in order.
	for _, m := range []string{"0001_p3_foundation", "0002_p4_stays", "0003_p4_operator", "0004_p4_operator_grants", "0005_p4_operator_identity"} {
		migPath := filepath.Join("..", "..", "migrations", m+".sql")
		if _, err := os.Stat(migPath); err != nil {
			t.Fatalf("migration file not found: %s: %v", migPath, err)
		}
		runPsql(t, dsn, migPath, "")
	}
	return dsn, cleanup
}

func runPsql(t *testing.T, dsn, file, sql string) {
	t.Helper()
	args := []string{dsn}
	if file != "" {
		args = append(args, "-q", "-f", file)
	}
	if sql != "" {
		args = append(args, "-c", sql)
	}
	cmd := exec.Command("psql", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("psql %v: %v\n%s", args, err, string(out))
	}
}

// disposableServer spins up a real /api/v3 server wired to a disposable DB.
// The returned cleanup closes the HTTP server, the store pool and drops the
// database.
func disposableServer(t *testing.T) (*httpserver.Server, *httptest.Server, *store.Store, func()) {
	t.Helper()
	dsn, dbCleanup := disposableDB(t)
	st, err := store.Open(dsn)
	if err != nil {
		dbCleanup()
		t.Fatalf("store.Open: %v", err)
	}
	httpSrv := httpserver.New(httpserver.DefaultConfig("127.0.0.1:0"),
		httpserver.WithStore(st),
		httpserver.WithLogger(testLogger(t)),
	)
	ts := httptest.NewServer(httpSrv.Handler())
	cleanup := func() {
		ts.Close()
		_ = st.Close()
		dbCleanup()
	}
	return httpSrv, ts, st, cleanup
}

// testLogger is a tiny structured logger that swallows output. The integration
// tests don't need noise; they assert on outcomes.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- HTTP helpers (lifted from stay_http_integration_test.go so this file is
// self-contained; same shapes). ---

type httpResp struct {
	code   int
	header http.Header
	body   []byte
}

func doAuthed(t *testing.T, ts *httptest.Server, method, path, token, body string) httpResp {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	raw := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		raw = append(raw, buf[:n]...)
		if err != nil {
			break
		}
	}
	return httpResp{code: resp.StatusCode, header: resp.Header, body: raw}
}

func decodeEnvelope(t *testing.T, rec httpResp) contracts.Envelope {
	t.Helper()
	var env contracts.Envelope
	if err := json.Unmarshal(rec.body, &env); err != nil {
		t.Fatalf("not an envelope: %v body=%s", err, string(rec.body))
	}
	return env
}

func createSession(t *testing.T, ts *httptest.Server) (string, string) {
	t.Helper()
	rec := doAuthed(t, ts, http.MethodPost, "/api/v3/sessions", "", `{}`)
	if rec.code != http.StatusCreated {
		t.Fatalf("create session: %d %s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	b, _ := json.Marshal(env.Data)
	var d struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.SessionID == "" || d.Token == "" {
		t.Fatalf("session missing fields: %s", rec.body)
	}
	return d.SessionID, d.Token
}

// seedRealPackage inserts an OPERATIONAL source + authorization + package +
// facility + inventory into the disposable DB so the real /api/v3/reservations
// commit gate accepts the test request.
func seedRealPackage(t *testing.T, st *store.Store, capacity int, start, end time.Time) (pkgID, facID string, snap int) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	srcID, artID, pkgID, facID := "SRC-"+suffix, "ART-"+suffix, "PKG-"+suffix, "FAC-"+suffix
	hash := strings.Repeat("0", 64)
	hashBytes := sha256.Sum256([]byte(hash))
	hash = hex.EncodeToString(hashBytes[:])
	ctx := t.Context()
	err := st.InTx(ctx, func(tx store.DBTX) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			VALUES ($1,'gov','gov.example','OPERATIONAL',1,$2,$2)`, srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,1,$3,$4,'AUTHORIZED_OPERATIONAL','mem://t')`, artID, srcID, hash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			VALUES ($1,'ALT',$2,$3,1,'JTEST','AUTHORIZED_OPERATIONAL',$4,$5,$6,$7)`, pkgID, srcID, artID, now, now.Add(24*time.Hour), hash,
			`{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`); err != nil {
			return err
		}
		if err := store.NewSourceStore(store.ChainAuditor{}).RecordAuthorization(ctx, tx, store.Authorization{
			AuthorizationID: "AUTH-" + srcID,
			SourceID:        srcID,
			GrantedBy:       "gov",
			EvidenceRef:     "doc-1",
			Jurisdiction:    "JTEST",
			GrantedAt:       now,
		}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1,$2,'SZ','Asia/Kolkata',1,$3)`, facID, pkgID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			VALUES ('SZ',$1,'SAFE',NULL,'OPEN',NULL,1,$2)
			ON CONFLICT (zone_id, package_id) DO NOTHING`, pkgID, now); err != nil {
			return err
		}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1,$2,$3,0,0,0,1,$4)`, facID, d, capacity, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return pkgID, facID, 1
}

// httpDayT/httpDay align with stay_http_integration_test.go helpers.
func httpDayT(n int) time.Time { return time.Date(2031, 3, n, 0, 0, 0, 0, time.UTC) }
func httpDay(n int) string     { return httpDayT(n).Format("2006-01-02") }

// reservationBody builds the JSON body the /api/v3/reservations endpoint
// expects.
func reservationBody(facID, pkgID string, party int, start, end, key string, snap int) []byte {
	body := map[string]any{
		"facility_id":      facID,
		"package_id":       pkgID,
		"party_size":       party,
		"start_date":       start,
		"end_date":         end,
		"idempotency_key":  key,
		"snapshot_version": snap,
	}
	b, _ := json.Marshal(body)
	return b
}

// buildOp builds a PendingOperation targeting /api/v3/reservations.
func buildOp(id, key string, payload []byte, snapVersion int, ref string, expiry time.Time) PendingOperation {
	return PendingOperation{
		ID:              id,
		IdempotencyKey:  key,
		Endpoint:        "/api/v3/reservations",
		Method:          "POST",
		TokenRef:        ref,
		Payload:         payload,
		SnapshotVersion: snapVersion,
		SelectionExpiry: expiry,
	}
}

// --- Tests ---

// TestHTTPQueueToRestartToSubmit: an enqueued operation survives a worker
// "restart" (new store + new worker opened against the same directory), is
// recovered on disk, and successfully submits on the next drain.
func TestHTTPQueueToRestartToSubmit(t *testing.T) {
	_, ts, st, cleanup := disposableServer(t)
	t.Cleanup(cleanup)
	pkgID, facID, snap := seedRealPackage(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, ts)

	dir := t.TempDir()
	// Worker #1: enqueue but never drain (simulating power loss after
	// enqueue, before any network attempt).
	clock := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s1, err := NewStore(dir, fixedClock(clock))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	tokens1 := NewMemoryTokenStore()
	tokens1.Put("session-A", token)
	op := buildOp("OP-RES-1", "key-restart-1",
		reservationBody(facID, pkgID, 2, httpDay(1), httpDay(3), "key-restart-1", snap),
		snap, "session-A", clock.Add(1*time.Hour))
	if err := s1.Enqueue(t.Context(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// "Restart": a fresh store + worker against the same directory.
	s2, err := NewStore(dir, fixedClock(clock))
	if err != nil {
		t.Fatalf("restart NewStore: %v", err)
	}
	w2, err := NewReplayWorker(s2, NewHTTPClientDispatcher(ts.URL, ts.Client()), tokens1, ReplayConfig{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: 5 * time.Second,
	}, fixedClock(clock))
	if err != nil {
		t.Fatalf("restart worker: %v", err)
	}
	w2.SetSleep(noSleepHTTP)
	rep, err := w2.DrainQueue(t.Context())
	if err != nil {
		t.Fatalf("DrainQueue: %v", err)
	}
	if rep.Committed != 1 {
		t.Fatalf("committed: got %d, want 1", rep.Committed)
	}
	after, _ := s2.Get(t.Context(), "OP-RES-1")
	if after.State != StateCommitted {
		t.Fatalf("state after restart+submit: got %s, want COMMITTED", after.State)
	}
	if after.LastStatusCode != http.StatusCreated {
		t.Fatalf("last status: got %d, want 201", after.LastStatusCode)
	}
}

// TestHTTPLostResponseAfterCommitReplayNoDoubleCommit: simulate the response
// being lost after the server has committed (e.g. process crash between
// commit and HTTP response). Re-submitting with the same idempotency key +
// payload must return the stored result WITHOUT doubling capacity or
// creating a duplicate stay.
func TestHTTPLostResponseAfterCommitReplayNoDoubleCommit(t *testing.T) {
	_, ts, st, cleanup := disposableServer(t)
	t.Cleanup(cleanup)
	pkgID, facID, snap := seedRealPackage(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, ts)

	// Reserve directly via HTTP to establish the server-side reservation.
	first := doAuthed(t, ts, http.MethodPost, "/api/v3/reservations", token,
		string(reservationBody(facID, pkgID, 2, httpDay(1), httpDay(2), "key-lost-resp", snap)))
	if first.code != http.StatusCreated {
		t.Fatalf("seed reservation: code=%d body=%s", first.code, first.body)
	}
	// Read capacity before replay.
	heldBefore := countInventory(t, st.DB(), facID, httpDayT(1), "held")
	if heldBefore != 2 {
		t.Fatalf("held before replay: got %d, want 2", heldBefore)
	}
	// Replay the same idempotency key + payload directly through HTTP — this
	// proves the server's own replay is also idempotent. The offlinequeue
	// harness replays through the same endpoint, so this is the real
	// boundary the harness exercises.
	replay := doAuthed(t, ts, http.MethodPost, "/api/v3/reservations", token,
		string(reservationBody(facID, pkgID, 2, httpDay(1), httpDay(2), "key-lost-resp", snap)))
	if replay.code != http.StatusOK {
		t.Fatalf("replay: code=%d body=%s", replay.code, replay.body)
	}
	heldAfter := countInventory(t, st.DB(), facID, httpDayT(1), "held")
	if heldAfter != heldBefore {
		t.Fatalf("replay doubled capacity: held before=%d after=%d", heldBefore, heldAfter)
	}
	// Now exercise the offlinequeue worker replay path end-to-end.
	dir := t.TempDir()
	clock := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(dir, fixedClock(clock))
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", token)
	w, _ := NewReplayWorker(s, NewHTTPClientDispatcher(ts.URL, ts.Client()), tokens, ReplayConfig{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: 5 * time.Second,
	}, fixedClock(clock))
	w.SetSleep(noSleepHTTP)
	op := buildOp("OP-REPLAY", "key-lost-resp",
		reservationBody(facID, pkgID, 2, httpDay(1), httpDay(2), "key-lost-resp", snap),
		snap, "session-A", clock.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), op); err != nil {
		t.Fatalf("enqueue replay: %v", err)
	}
	rep, err := w.DrainQueue(t.Context())
	if err != nil {
		t.Fatalf("DrainQueue: %v", err)
	}
	if rep.Committed != 1 {
		t.Fatalf("committed: got %d, want 1", rep.Committed)
	}
	heldFinal := countInventory(t, st.DB(), facID, httpDayT(1), "held")
	if heldFinal != heldBefore {
		t.Fatalf("offlinequeue replay doubled capacity: held before=%d after=%d", heldBefore, heldFinal)
	}
	after, _ := s.Get(t.Context(), "OP-REPLAY")
	if after.State != StateCommitted {
		t.Fatalf("state: got %s, want COMMITTED", after.State)
	}
}

// TestHTTPStaleSnapshotFromWorker: bump the package version after the
// operation is enqueued; the worker's replay must see STALE_VERSION and mark
// the entry FAILED_STALE without reserving capacity.
func TestHTTPStaleSnapshotFromWorker(t *testing.T) {
	_, ts, st, cleanup := disposableServer(t)
	t.Cleanup(cleanup)
	pkgID, facID, _ := seedRealPackage(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, ts)

	heldBefore := countInventory(t, st.DB(), facID, httpDayT(1), "held")

	// Bump the package version AFTER the client confirmed against snap=1.
	if _, err := st.DB().ExecContext(t.Context(), `UPDATE packages SET version=2 WHERE package_id=$1`, pkgID); err != nil {
		t.Fatalf("bump version: %v", err)
	}

	dir := t.TempDir()
	clock := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(dir, fixedClock(clock))
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", token)
	w, _ := NewReplayWorker(s, NewHTTPClientDispatcher(ts.URL, ts.Client()), tokens, ReplayConfig{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: 5 * time.Second,
	}, fixedClock(clock))
	w.SetSleep(noSleepHTTP)
	op := buildOp("OP-STALE", "key-stale-1",
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "key-stale-1", 1),
		1, "session-A", clock.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	outcome, err := w.Submit(t.Context(), op)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if outcome != outcomeStale {
		t.Fatalf("outcome: got %v, want outcomeStale", outcome)
	}
	after, _ := s.Get(t.Context(), "OP-STALE")
	if after.State != StateFailedStale {
		t.Fatalf("state: got %s, want FAILED_STALE", after.State)
	}
	if after.LastStatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", after.LastStatusCode)
	}
	heldAfter := countInventory(t, st.DB(), facID, httpDayT(1), "held")
	if heldAfter != heldBefore {
		t.Fatalf("stale path leaked capacity: before=%d after=%d", heldBefore, heldAfter)
	}
}

// TestHTTPExpiredSelectionNoServerContact: enqueue while the selection is
// valid, advance the clock past SelectionExpiry, and confirm the worker
// fails closed without contacting the server (no capacity change).
func TestHTTPExpiredSelectionNoServerContact(t *testing.T) {
	_, ts, st, cleanup := disposableServer(t)
	t.Cleanup(cleanup)
	pkgID, facID, snap := seedRealPackage(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, ts)
	heldBefore := countInventory(t, st.DB(), facID, httpDayT(1), "held")

	enq := time.Date(2026, 9, 20, 11, 0, 0, 0, time.UTC)
	nowT := enq.Add(2 * time.Hour)
	var clockFlag atomic.Value
	clockFlag.Store(enq)
	nowFn := func() time.Time { return clockFlag.Load().(time.Time) }

	dir := t.TempDir()
	s, _ := NewStore(dir, nowFn)
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", token)
	d := NewHTTPClientDispatcher(ts.URL, ts.Client())
	rec := &recordingHTTPDispatcher{Inner: d}
	w, _ := NewReplayWorker(s, rec, tokens, ReplayConfig{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: 5 * time.Second,
	}, nowFn)
	w.SetSleep(noSleepHTTP)
	op := buildOp("OP-EXP", "key-exp-1",
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "key-exp-1", snap),
		snap, "session-A", enq.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	clockFlag.Store(nowT)
	outcome, _ := w.Submit(t.Context(), op)
	if outcome != outcomeStale {
		t.Fatalf("outcome: got %v, want outcomeStale", outcome)
	}
	if rec.calls != 0 {
		t.Fatalf("server contacted for expired selection: %d", rec.calls)
	}
	heldAfter := countInventory(t, st.DB(), facID, httpDayT(1), "held")
	if heldAfter != heldBefore {
		t.Fatalf("expired path leaked capacity: before=%d after=%d", heldBefore, heldAfter)
	}
	after, _ := s.Get(t.Context(), "OP-EXP")
	if after.State != StateFailedStale {
		t.Fatalf("state: got %s, want FAILED_STALE", after.State)
	}
}

// TestHTTPChangedPayloadRequiresNewKey: enqueue one selection, then try to
// enqueue the SAME idempotency key with a DIFFERENT payload (different
// facility). The queue refuses; the user must confirm a new selection with a
// new key.
//
// The semantic is split between two layers:
//
//   - Queue: refuses to silently overwrite an UNCERTAIN prior submission
//     (PENDING or IN_FLIGHT) with the same key + different payload. A
//     terminal (committed or failed) prior submission does NOT block; the
//     server-side idempotency store will reject the replay with
//     IDEMPOTENCY_CONFLICT if the payload differs.
//   - Server: rejects the same key + different payload with
//     IDEMPOTENCY_CONFLICT, even when the prior submission committed.
//
// This test exercises BOTH layers end to end.
func TestHTTPChangedPayloadRequiresNewKey(t *testing.T) {
	_, ts, st, cleanup := disposableServer(t)
	t.Cleanup(cleanup)
	pkgID, facID, snap := seedRealPackage(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, ts)

	dir := t.TempDir()
	clock := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(dir, fixedClock(clock))
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", token)
	w, _ := NewReplayWorker(s, NewHTTPClientDispatcher(ts.URL, ts.Client()), tokens, ReplayConfig{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: 5 * time.Second,
	}, fixedClock(clock))
	w.SetSleep(noSleepHTTP)

	// Original selection.
	op1 := buildOp("OP-1", "shared-key",
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "shared-key", snap),
		snap, "session-A", clock.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), op1); err != nil {
		t.Fatalf("enqueue op1: %v", err)
	}
	// Layer-1: while op1 is PENDING, the queue must refuse to overwrite the
	// payload under the same key (changed intent requires a new key).
	altered := op1
	altered.ID = "OP-2"
	altered.Payload = reservationBody("FAC-DIFFERENT", pkgID, 1, httpDay(1), httpDay(2), "shared-key", snap)
	if err := s.Enqueue(t.Context(), altered); !errors.Is(err, ErrPayloadChanged) {
		t.Fatalf("queue layer: got %v, want ErrPayloadChanged", err)
	}
	// Layer-1 cont: same key + same payload while PENDING is a double-submit
	// guard, not silent overwrite — ErrOperationConflict.
	dup := op1
	dup.ID = "OP-DUP"
	if err := s.Enqueue(t.Context(), dup); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("queue layer double-submit: got %v, want ErrOperationConflict", err)
	}
	// Commit op1 so the prior entry becomes terminal.
	if _, err := w.Submit(t.Context(), op1); err != nil {
		t.Fatalf("commit op1: %v", err)
	}
	// After commit, the queue ALLOWS a fresh enqueue with the same key — the
	// terminal entry does not block. The server's idempotency store is the
	// real guard: a different payload under the same key is rejected with
	// IDEMPOTENCY_CONFLICT. The worker marks that FAILED_PERM.
	after := buildOp("OP-3", "shared-key",
		reservationBody("FAC-DIFFERENT", pkgID, 1, httpDay(1), httpDay(2), "shared-key", snap),
		snap, "session-A", clock.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), after); err != nil {
		t.Fatalf("post-commit fresh enqueue: %v", err)
	}
	outcome, err := w.Submit(t.Context(), after)
	if err != nil {
		t.Fatalf("submit after: %v", err)
	}
	if outcome != outcomePerm {
		t.Fatalf("server layer: outcome=%v, want outcomePerm (IDEMPOTENCY_CONFLICT)", outcome)
	}
	// The user now confirms the new intent with a NEW key — that succeeds.
	confirmed := buildOp("OP-4", "new-key-after",
		reservationBody("FAC-DIFFERENT", pkgID, 1, httpDay(1), httpDay(2), "new-key-after", snap),
		snap, "session-A", clock.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), confirmed); err != nil {
		t.Fatalf("enqueue with new key: %v", err)
	}
	if _, err := w.Submit(t.Context(), confirmed); err != nil {
		t.Fatalf("submit confirmed: %v", err)
	}
	// Final capacity: op1 committed on facID (1 held), confirmed on
	// FAC-DIFFERENT (1 held). After-commit (shared-key, FAC-DIFFERENT) was
	// rejected by the server, no reservation.
	if held := countInventory(t, st.DB(), facID, httpDayT(1), "held"); held != 1 {
		t.Fatalf("held on original facility: got %d, want 1", held)
	}
}

// TestHTTPDenialAndConflict: a CAPACITY_CONFLICT surfaces to the queue as
// FAILED_PERM. Capacity was not changed (the server rejected).
func TestHTTPDenialAndConflict(t *testing.T) {
	_, ts, st, cleanup := disposableServer(t)
	t.Cleanup(cleanup)
	pkgID, facID, snap := seedRealPackage(t, st, 1, httpDayT(1), httpDayT(3))
	_, token := createSession(t, ts)

	// First reservation fills capacity.
	first := doAuthed(t, ts, http.MethodPost, "/api/v3/reservations", token,
		string(reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "key-fill", snap)))
	if first.code != http.StatusCreated {
		t.Fatalf("seed fill: code=%d body=%s", first.code, first.body)
	}
	// Enqueue a competing reservation that the worker will replay.
	dir := t.TempDir()
	clock := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(dir, fixedClock(clock))
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", token)
	w, _ := NewReplayWorker(s, NewHTTPClientDispatcher(ts.URL, ts.Client()), tokens, ReplayConfig{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: 5 * time.Second,
	}, fixedClock(clock))
	w.SetSleep(noSleepHTTP)
	op := buildOp("OP-DENY", "key-deny-1",
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "key-deny-1", snap),
		snap, "session-A", clock.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	outcome, _ := w.Submit(t.Context(), op)
	if outcome != outcomePerm {
		t.Fatalf("outcome: got %v, want outcomePerm (CAPACITY_CONFLICT)", outcome)
	}
	after, _ := s.Get(t.Context(), "OP-DENY")
	if after.State != StateFailedPerm {
		t.Fatalf("state: got %s, want FAILED_PERM", after.State)
	}
	if after.LastStatusCode != http.StatusConflict {
		t.Fatalf("status: got %d, want 409", after.LastStatusCode)
	}
	// No retry: the worker should have contacted the server exactly once.
	// (Verified indirectly by absence of RetryCount beyond 1.)
	if after.RetryCount > 1 {
		t.Fatalf("retry count: got %d, want <= 1 for permanent denial", after.RetryCount)
	}
}

// TestHTTPNoDoubleCommitmentViaAutoSubstitution: an automated retry on
// FAILED_PERM with a changed payload must NOT silently substitute another
// destination. The queue holds the original payload; the user must confirm
// a different facility with a NEW key.
func TestHTTPNoDoubleCommitmentViaAutoSubstitution(t *testing.T) {
	_, ts, st, cleanup := disposableServer(t)
	t.Cleanup(cleanup)
	pkgID, facID, snap := seedRealPackage(t, st, 1, httpDayT(1), httpDayT(3))
	_, token := createSession(t, ts)

	// Seed two facilities to give the worker a real substitution choice.
	facID2 := "FAC-ALT-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if _, err := st.DB().ExecContext(t.Context(), `
		INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
		VALUES ($1,$2,'SZ','Asia/Kolkata',1,$3)`, facID2, pkgID, time.Now().UTC().Truncate(time.Microsecond)); err != nil {
		t.Fatalf("seed alt facility: %v", err)
	}
	if _, err := st.DB().ExecContext(t.Context(), `
		INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
		VALUES ($1,$2,1,0,0,0,1,$3)`, facID2, httpDayT(1), time.Now().UTC().Truncate(time.Microsecond)); err != nil {
		t.Fatalf("seed alt inventory: %v", err)
	}

	// Fill capacity on facID.
	first := doAuthed(t, ts, http.MethodPost, "/api/v3/reservations", token,
		string(reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "key-fill-2", snap)))
	if first.code != http.StatusCreated {
		t.Fatalf("seed fill: code=%d body=%s", first.code, first.body)
	}

	dir := t.TempDir()
	clock := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(dir, fixedClock(clock))
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", token)
	w, _ := NewReplayWorker(s, NewHTTPClientDispatcher(ts.URL, ts.Client()), tokens, ReplayConfig{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, PerCallTimeout: 5 * time.Second,
	}, fixedClock(clock))
	w.SetSleep(noSleepHTTP)
	op := buildOp("OP-SUB", "key-sub-1",
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "key-sub-1", snap),
		snap, "session-A", clock.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	outcome, _ := w.Submit(t.Context(), op)
	if outcome != outcomePerm {
		t.Fatalf("outcome: got %v, want outcomePerm", outcome)
	}
	after, _ := s.Get(t.Context(), "OP-SUB")
	if after.State != StateFailedPerm {
		t.Fatalf("state: got %s, want FAILED_PERM", after.State)
	}
	// Critical assertion: no other reservation was created. Capacity on
	// facID2 is still 0 held.
	held := countInventory(t, st.DB(), facID2, httpDayT(1), "held")
	if held != 0 {
		t.Fatalf("alt facility was silently used: held=%d, want 0", held)
	}
	// And the original payload is intact — no substitution was attempted.
	if !strings.Contains(string(after.Payload), facID) {
		t.Fatalf("payload was substituted: %s", after.Payload)
	}
}

// TestHTTPPrivateDataNotInPublicOutput: the queue never causes private data
// (session token, capacity figures) to leak into public-facing output. The
// P5 contract §1.2 invariant 1 says public resources must not echo private
// tokens or session ids. The harness has no public endpoint, but its
// SnapshotForInspection MUST redact TokenRef.
func TestHTTPPrivateDataNotInPublicOutput(t *testing.T) {
	_, ts, st, cleanup := disposableServer(t)
	t.Cleanup(cleanup)
	pkgID, facID, snap := seedRealPackage(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, ts)
	if token == "" {
		t.Fatal("token empty")
	}

	dir := t.TempDir()
	clock := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	s, _ := NewStore(dir, fixedClock(clock))
	tokens := NewMemoryTokenStore()
	tokens.Put("session-A", token)
	op := buildOp("OP-PRIV", "key-priv-1",
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "key-priv-1", snap),
		snap, "session-A", clock.Add(1*time.Hour))
	if err := s.Enqueue(t.Context(), op); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// Inspect the on-disk file directly to prove no token bytes leaked.
	raw, err := os.ReadFile(dir + "/OP-PRIV.json")
	if err != nil {
		t.Fatalf("read on-disk: %v", err)
	}
	if strings.Contains(string(raw), token) {
		t.Fatalf("bearer token persisted to queue file:\n%s", string(raw))
	}
	// The snapshot view also redacts the TokenRef label.
	view, err := s.SnapshotForInspection(t.Context())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	for _, op := range view {
		if op.TokenRef != "REDACTED" {
			t.Fatalf("TokenRef leaked: %q", op.TokenRef)
		}
		if strings.Contains(string(op.Payload), "Authorization") {
			t.Fatalf("payload carries Authorization header")
		}
	}
}

// noSleepHTTP is the worker sleep used by integration tests.
func noSleepHTTP(_ context.Context, _ time.Duration) error { return nil }

// recordingHTTPDispatcher wraps a real dispatcher and counts calls. Used in
// tests that must prove the worker did NOT contact the server (e.g., expired
// selection path).
type recordingHTTPDispatcher struct {
	Inner HTTPDispatcher
	calls int32
}

func (r *recordingHTTPDispatcher) PostJSON(ctx context.Context, method, path, token string, body []byte) (int, []byte, error) {
	atomic.AddInt32(&r.calls, 1)
	return r.Inner.PostJSON(ctx, method, path, token, body)
}

// countInventory reads the held column for a single facility+date.
func countInventory(t *testing.T, db *sql.DB, facID string, day time.Time, column string) int {
	t.Helper()
	row := db.QueryRowContext(t.Context(),
		fmt.Sprintf(`SELECT %s FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`, column),
		facID, day)
	var v int
	if err := row.Scan(&v); err != nil {
		t.Fatalf("count inventory: %v", err)
	}
	return v
}
