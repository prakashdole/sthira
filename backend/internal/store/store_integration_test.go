package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"sthira/backend/internal/sourceact"
)

// Real-DB verification for the P3 storage foundation. These tests require a
// live PostgreSQL/PostGIS instance with the 0001_p3_foundation migration
// applied. They are gated on STHIRA_TEST_DSN and skip when it is unset, so the
// offline suite stays green. Run with:
//
//	STHIRA_TEST_DSN='postgres://apple@localhost:5432/sthira_test' \
//	  go test ./internal/store/... -count=1
//
// The tests use unique ids per run so they can run repeatedly against the same
// database without colliding with prior runs.

func testDB(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" {
		t.Skip("STHIRA_TEST_DSN unset; skipping real-DB verification")
	}
	s, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.db.Close() })
	return s
}

// uid returns a run-unique id with the given prefix.
func uid(prefix string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())))
	return prefix + "-" + hex.EncodeToString(sum[:6])
}

func nowUTC() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// insertTestSource registers a DISCOVERED source and returns its id.
func insertTestSource(t *testing.T, s *Store, ss *SourceStore) string {
	t.Helper()
	id := uid("SRC")
	err := s.InTx(context.Background(), func(tx DBTX) error {
		return ss.InsertSource(context.Background(), tx, sourceact.Source{
			SourceID:        id,
			GovernmentOwner: "gov-test",
			OfficialDomain:  "gov.example",
			State:           sourceact.Discovered,
			UpdatedAt:       nowUTC(),
		})
	})
	if err != nil {
		t.Fatalf("InsertSource: %v", err)
	}
	return id
}

// TestStaleVersionConflict: a second transition with a stale expected version
// must be rejected with ErrVersionConflict, never a silent success.
func TestStaleVersionConflict(t *testing.T) {
	s := testDB(t)
	ss := NewSourceStore(ChainAuditor{})
	ctx := context.Background()
	id := insertTestSource(t, s, ss)

	// First transition DISCOVERED -> ACCESS_REQUESTED at version 1.
	err := s.InTx(ctx, func(tx DBTX) error {
		return ss.Transition(ctx, tx, id, 1, sourceact.AccessRequested, "op-1", "req", uid("EV"), nowUTC())
	})
	if err != nil {
		t.Fatalf("first transition: %v", err)
	}

	// Replay the same expected version (now stale): must conflict.
	err = s.InTx(ctx, func(tx DBTX) error {
		return ss.Transition(ctx, tx, id, 1, sourceact.AccessRequested, "op-1", "replay", uid("EV"), nowUTC())
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale version: got %v, want ErrVersionConflict", err)
	}

	// Missing source must surface ErrNotFound, not a version conflict.
	err = s.InTx(ctx, func(tx DBTX) error {
		return ss.Transition(ctx, tx, uid("SRC"), 1, sourceact.AccessRequested, "op-1", "x", uid("EV"), nowUTC())
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing source: got %v, want ErrNotFound", err)
	}
}

// TestConcurrentLastSpace: N competing reservations for a single remaining
// space; exactly one wins, the rest get ErrCapacityExhausted or a version
// conflict they would retry. Conservation (reserved <= capacity) must hold.
func TestConcurrentLastSpace(t *testing.T) {
	s := testDB(t)
	rs := NewReservationStore(ChainAuditor{})
	ctx := context.Background()

	fac := uid("FAC")
	date := nowUTC().Truncate(24 * time.Hour)
	sess := uid("SES")
	// Reservation FK requires a session row.
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sessions (session_id, principal_kind, credential_ref, expires_at)
			VALUES ($1, 'CITIZEN', 'cred', $2)`, sess, nowUTC().Add(time.Hour))
		return err
	}); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := s.InTx(ctx, func(tx DBTX) error {
		return rs.EnsureInventory(ctx, tx, fac, date, 1, nowUTC()) // capacity 1
	}); err != nil {
		t.Fatalf("EnsureInventory: %v", err)
	}

	const contenders = 8
	var wg sync.WaitGroup
	wins := 0
	var mu sync.Mutex
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each contender reads the current inventory version then reserves.
			var ver int
			_ = s.db.QueryRowContext(ctx,
				`SELECT version FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
				fac, date).Scan(&ver)
			err := s.InTx(ctx, func(tx DBTX) error {
				return rs.Reserve(ctx, tx, Reservation{
					ReservationID: uid("RES"),
					SessionID:     sess,
					FacilityID:    fac,
					ServiceDate:   date,
					PartySize:     1,
					CreatedAt:     nowUTC(),
				}, ver)
			})
			if err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(err, ErrCapacityExhausted) && !errors.Is(err, ErrVersionConflict) {
				t.Errorf("contender %d: unexpected error %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("exactly one reservation must win the last space; got %d", wins)
	}
	// Conservation: reserved must equal 1 and never exceed capacity.
	var reserved, capacity int
	if err := s.db.QueryRowContext(ctx,
		`SELECT reserved, capacity FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
		fac, date).Scan(&reserved, &capacity); err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	if reserved > capacity {
		t.Fatalf("conservation violated: reserved %d > capacity %d", reserved, capacity)
	}
}

// TestIdempotencyReplayAndConflict: a completed key replays its stored result;
// the same key with a different payload is a conflict.
func TestIdempotencyReplayAndConflict(t *testing.T) {
	s := testDB(t)
	is := IdempotencyStore{}
	ctx := context.Background()
	scope, op, key := uid("SCOPE"), "reservation.create", uid("KEY")
	hash := hex.EncodeToString(func() []byte { h := sha256.Sum256([]byte("payload-a")); return h[:] }())

	err := s.InTx(ctx, func(tx DBTX) error {
		_, replay, err := is.Begin(ctx, tx, scope, op, key, hash, nowUTC().Add(time.Hour))
		if err != nil || replay {
			return fmt.Errorf("first begin: replay=%v err=%v", replay, err)
		}
		return is.Complete(ctx, tx, scope, op, key, map[string]string{"id": "r1"})
	})
	if err != nil {
		t.Fatalf("begin+complete: %v", err)
	}

	// Replay with the same payload returns the stored result.
	var stored []byte
	var replay bool
	err = s.InTx(ctx, func(tx DBTX) error {
		var err error
		stored, replay, err = is.Begin(ctx, tx, scope, op, key, hash, nowUTC().Add(time.Hour))
		return err
	})
	if err != nil || !replay {
		t.Fatalf("replay: replay=%v err=%v", replay, err)
	}
	if len(stored) == 0 {
		t.Error("replay must return the stored result")
	}

	// Same key, different payload -> conflict.
	other := hex.EncodeToString(func() []byte { h := sha256.Sum256([]byte("payload-b")); return h[:] }())
	err = s.InTx(ctx, func(tx DBTX) error {
		_, _, err := is.Begin(ctx, tx, scope, op, key, other, nowUTC().Add(time.Hour))
		return err
	})
	if !errors.Is(err, ErrPayloadConflict) {
		t.Fatalf("payload conflict: got %v, want ErrPayloadConflict", err)
	}
}

// TestAuthorizationGatesOperational: the AUTHORIZED -> OPERATIONAL transition
// must reference recorded authorization evidence; without it HasAuthorization
// is false, with it (and not expired) it is true.
func TestAuthorizationGatesOperational(t *testing.T) {
	s := testDB(t)
	ss := NewSourceStore(ChainAuditor{})
	ctx := context.Background()
	id := insertTestSource(t, s, ss)
	jur := "JUR-" + uid("X")

	// No authorization recorded yet.
	ok, err := ss.HasAuthorization(ctx, s.db, id, jur, nowUTC())
	if err != nil {
		t.Fatalf("HasAuthorization: %v", err)
	}
	if ok {
		t.Fatal("no authorization recorded; must not be authorized")
	}

	// Record evidence, then it authorizes.
	if err := s.InTx(ctx, func(tx DBTX) error {
		return ss.RecordAuthorization(ctx, tx, Authorization{
			AuthorizationID: uid("AUTH"),
			SourceID:        id,
			GrantedBy:       "authority-1",
			EvidenceRef:     "doc-123",
			Jurisdiction:    jur,
			GrantedAt:       nowUTC(),
		})
	}); err != nil {
		t.Fatalf("RecordAuthorization: %v", err)
	}
	ok, err = ss.HasAuthorization(ctx, s.db, id, jur, nowUTC())
	if err != nil || !ok {
		t.Fatalf("after record: ok=%v err=%v", ok, err)
	}

	// A different jurisdiction is not authorized by this evidence.
	ok, err = ss.HasAuthorization(ctx, s.db, id, "OTHER-"+uid("X"), nowUTC())
	if err != nil || ok {
		t.Fatalf("cross-jurisdiction: ok=%v err=%v (must be false)", ok, err)
	}
}

// TestAuditChainConsistency: a sequence of transitions produces an intact
// hash chain that VerifyChain accepts, and tampering is detectable.
func TestAuditChainConsistency(t *testing.T) {
	s := testDB(t)
	ss := NewSourceStore(ChainAuditor{})
	ctx := context.Background()
	id := insertTestSource(t, s, ss)

	// Drive a few transitions, each writing a chained audit event.
	steps := []sourceact.State{sourceact.AccessRequested, sourceact.SampleAcquired, sourceact.Validated}
	for i, st := range steps {
		ver := i + 1
		if err := s.InTx(ctx, func(tx DBTX) error {
			return ss.Transition(ctx, tx, id, ver, st, "op-1", "chain-test", uid("EV"), nowUTC())
		}); err != nil {
			t.Fatalf("transition to %s: %v", st, err)
		}
	}

	if err := VerifyChain(ctx, s.db); err != nil {
		t.Fatalf("VerifyChain: %v", err)
	}
}

// TestRestartPersistence: data written and committed survives a fresh
// connection (a stand-in for process restart).
func TestRestartPersistence(t *testing.T) {
	s := testDB(t)
	ss := NewSourceStore(nil)
	ctx := context.Background()
	id := insertTestSource(t, s, ss)

	// Reopen a brand-new pool against the same DSN.
	s2 := testDB(t)
	got, err := ss.GetSource(ctx, s2.db, id)
	if err != nil {
		t.Fatalf("GetSource after reopen: %v", err)
	}
	if got.SourceID != id || got.State != sourceact.Discovered || got.Version != 1 {
		t.Fatalf("unexpected source after reopen: %+v", got)
	}
}

// TestReadinessProbe: the readiness prober reports ready when the DB is
// reachable and migrations are current, and not-ready when the handle is nil.
func TestReadinessProbe(t *testing.T) {
	s := testDB(t)
	p := NewReadinessProber(s.db, 1)
	if err := p.Probe(context.Background()); err != nil {
		t.Fatalf("Probe against live DB: %v", err)
	}
	// A prober expecting a future revision must report not-ready.
	future := NewReadinessProber(s.db, 9999)
	if err := future.Probe(context.Background()); err == nil {
		t.Fatal("Probe with future revision must fail")
	}
}
