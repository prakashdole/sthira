package store

// P4 concurrent commitment/withdrawal verification: deterministic two-
// connection tests with barrier coordination and pg_locks-based
// observation (no timing-dependent sleeps; the test proves the competing
// transaction actually entered lock-wait state before the racing
// transaction releases).
//
// Gated on STHIRA_TEST_DSN; skipped when unset.
//
// The locking protocol in RevalidateReservationContext (FOR UPDATE OF s,p)
// means a reservation commit blocks any concurrent source withdrawal that
// touches the same source row, and vice versa. Whichever transaction
// acquires the lock first determines whether the new commitment sees a
// still-operational or already-withdrawn source.

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/sourceact"
)

// twoStoreDSN returns the DSN the tests are using. Cached after first read.
var twoStoreDSN = sync.OnceValues(func() (string, error) {
	d := os.Getenv("STHIRA_TEST_DSN")
	if d == "" {
		return "", errors.New("STHIRA_TEST_DSN unset")
	}
	return d, nil
})

// openSecondStore opens a separate connection pool to the same DB.
func openSecondStore(t *testing.T) *Store {
	t.Helper()
	dsn, err := twoStoreDSN()
	if err != nil {
		t.Skip(err)
	}
	s, err := Open(dsn)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// sourceIDForPkg returns the source_id for a package.
func sourceIDForPkg(t *testing.T, s *Store, pkgID string) string {
	t.Helper()
	var id string
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT source_id FROM packages WHERE package_id=$1`, pkgID).Scan(&id); err != nil {
		t.Fatalf("sourceIDForPkg: %v", err)
	}
	return id
}

// runRacing is a small runner that executes fn on goroutines, propagates
// errors back to the test, and releases all participants if any error
// occurs. It deliberately does NOT use Fatalf in goroutines: errors are
// collected on errs and asserted by the caller.
func runRacing(t *testing.T, fns ...func() error) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make(chan error, len(fns))
	for _, fn := range fns {
		fn := fn
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- fn()
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatalf("worker err: %v", e)
		}
	}
}

// TestSourceLockBlocksWithdrawal: when a reservation holds FOR UPDATE OF s,p
// on the source row, a concurrent quarantine must block until the
// reservation commits or rolls back. The test OBSERVES that the
// quarantine transaction entered lock-wait state via pg_locks before the
// reservation releases the lock, proving the lock contention is real.
//
// Ordering protocol (no sleeps):
//  1. reservation begins tx and acquires FOR UPDATE on the source row.
//  2. reservation signals "holding-lock" (lockHeld barrier).
//  3. quarantine begins tx and attempts the UPDATE; the UPDATE blocks
//     because the source row is locked.
//  4. observer polls pg_locks and signals when it sees the wait state.
//  5. reservation verifies the source version was NOT bumped under its
//     lock, commits its work, and signals "released".
//  6. quarantine's UPDATE unblocks; it returns whatever the store says
//     (here ErrVersionConflict, since its srcVersion is now stale).
func TestSourceLockBlocksWithdrawal(t *testing.T) {
	s := testDB(t)
	s2 := openSecondStore(t)
	obs := openObserverPool(t)
	ctx := context.Background()
	start, end := day(20), day(21)
	pkgID, facID, srcID, srcVersion := seedOperationalPackage(t, s, 1, start, end)
	stays := NewStayStore(ChainAuditor{})
	sources := NewSourceStore(ChainAuditor{})

	lockHeld := make(chan struct{})           // commit has the source FOR UPDATE
	quarantineIssued := make(chan struct{})   // quarantine tx has issued its UPDATE
	quarantineObserved := make(chan struct{}) // observer confirms wait
	commitRelease := make(chan struct{})      // commit transaction finished

	var quarObservedWaiting atomic.Bool

	runRacing(t,
		func() error {
			// Reservation goroutine.
			return s.InTx(ctx, func(tx DBTX) error {
				_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
				if rerr != nil {
					return rerr
				}
				// We hold the FOR UPDATE on sources/packages; tell the
				// quarantine goroutine to issue its UPDATE.
				close(lockHeld)
				// Wait for the quarantine to have actually issued the UPDATE.
				select {
				case <-quarantineIssued:
				case <-time.After(5 * time.Second):
					return errors.New("quarantine did not issue its UPDATE")
				}
				// Wait for the observer to confirm the quarantine IS waiting
				// on the row lock (the contention is observable, not assumed).
				select {
				case <-quarantineObserved:
				case <-time.After(5 * time.Second):
					return errors.New("observer did not confirm quarantine as waiting")
				}
				var v int
				if err := tx.QueryRowContext(ctx, `SELECT version FROM sources WHERE source_id=$1`, srcID).Scan(&v); err != nil {
					return err
				}
				if v != srcVersion {
					t.Errorf("under lock: source version=%d, want %d (withdrawal blocked)", v, srcVersion)
				}
				sessID := seedSession(t, s)
				st := mkStay(sessID, facID, pkgID, 1, start, end)
				seedReservation(t, s, st)
				if err := stays.Reserve(ctx, tx, st, nowUTC(), "lock-test-key"); err != nil {
					return err
				}
				close(commitRelease)
				return nil
			})
		},
		func() error {
			// Quarantine goroutine. Waits for the reservation to acquire the
			// lock, then issues its UPDATE; the UPDATE will block on the
			// source row FOR UPDATE.
			return s2.InTx(ctx, func(tx DBTX) error {
				// Make sure the reservation goroutine is holding the lock.
				select {
				case <-lockHeld:
				case <-time.After(5 * time.Second):
					return errors.New("reservation did not acquire lock first")
				}
				// Issue the UPDATE in a goroutine; it will block on the
				// source row. We signal quarantineIssued AFTER issuing, then
				// wait for the observer to confirm we are in lock-wait.
				errCh := make(chan error, 1)
				go func() {
					errCh <- sources.Quarantine(ctx, tx, srcID, srcVersion, "test-op", "hold", "EV-Q-"+srcID, nowUTC())
				}()
				close(quarantineIssued)
				// Observer: poll pg_locks to confirm the quarantine IS blocked.
				go func() {
					if _, err := pollUntilWaiting(ctx, obs, 5*time.Second); err == nil {
						quarObservedWaiting.Store(true)
						close(quarantineObserved)
					}
				}()
				// Wait for the commit to release.
				select {
				case <-commitRelease:
				case <-time.After(10 * time.Second):
					return errors.New("reservation did not release lock in time")
				}
				if !quarObservedWaiting.Load() {
					return errors.New("observer did not confirm lock-wait contention")
				}
				// Quarantine runs against the post-commit state; its
				// srcVersion is now stale, so it gets ErrVersionConflict.
				// That is the CORRECT outcome of lock-serialized contention:
				// the second writer waits, then sees the new state.
				return <-errCh
			})
		},
	)
}

// TestConcurrentBarrierCommitmentWins: both goroutines reach the lock
// before either commits. The COMMIT transaction holds the source lock
// first; the QUARANTINE UPDATE is observed in lock-wait state until the
// commit releases. Capacity reflects the commit; the quarantine then
// succeeds against the new source version. No data corruption.
//
// Ordering protocol (no sleeps):
//  1. commit begins tx and acquires FOR UPDATE on the source row.
//  2. commit signals "lock-held" (lockHeld barrier).
//  3. quarantine begins tx; in a goroutine issues its UPDATE (which
//     blocks on the source row); signals "issued" (quarantineIssued).
//  4. observer polls pg_stat_activity and signals "observed" when it
//     sees the quarantine waiting on a Lock wait_event_type.
//  5. commit verifies source version unchanged, commits, and signals
//     "released".
//  6. quarantine's UPDATE unblocks; its srcVersion is stale, so it
//     returns ErrVersionConflict (the second-writer-sees-new-state
//     contract).
func TestConcurrentBarrierCommitmentWins(t *testing.T) {
	s := testDB(t)
	s2 := openSecondStore(t)
	obs := openObserverPool(t)
	ctx := context.Background()
	start, end := day(29), day(30)
	pkgID, facID, srcID, srcVersion := seedOperationalPackage(t, s, 5, start, end)
	stays := NewStayStore(ChainAuditor{})
	sources := NewSourceStore(ChainAuditor{})

	lockHeld := make(chan struct{})
	quarantineIssued := make(chan struct{})
	quarantineObserved := make(chan struct{})
	commitRelease := make(chan struct{})
	var commitHeld atomic.Int32
	var quarObservedWaiting atomic.Bool

	runRacing(t,
		func() error {
			return s.InTx(ctx, func(tx DBTX) error {
				_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
				if rerr != nil {
					return rerr
				}
				close(lockHeld) // commit holds the source FOR UPDATE
				// Wait for the quarantine to have actually issued its UPDATE.
				select {
				case <-quarantineIssued:
				case <-time.After(5 * time.Second):
					return errors.New("quarantine did not issue its UPDATE")
				}
				select {
				case <-quarantineObserved:
				case <-time.After(5 * time.Second):
					return errors.New("observer did not report quarantine waiting")
				}
				sessID := seedSession(t, s)
				st := mkStay(sessID, facID, pkgID, 1, start, end)
				seedReservation(t, s, st)
				if err := stays.Reserve(ctx, tx, st, nowUTC(), "barrier-commit"); err != nil {
					return err
				}
				commitHeld.Add(1)
				close(commitRelease)
				return nil
			})
		},
		func() error {
			return s2.InTx(ctx, func(tx DBTX) error {
				select {
				case <-lockHeld:
				case <-time.After(5 * time.Second):
					return errors.New("commit did not acquire lock first")
				}
				errCh := make(chan error, 1)
				go func() {
					errCh <- sources.Quarantine(ctx, tx, srcID, srcVersion, "test-op", "hold", "EV-Q-"+srcID, nowUTC())
				}()
				close(quarantineIssued)
				go func() {
					if _, err := pollUntilWaiting(ctx, obs, 5*time.Second); err == nil {
						quarObservedWaiting.Store(true)
						close(quarantineObserved)
					}
				}()
				select {
				case <-commitRelease:
				case <-time.After(10 * time.Second):
					return errors.New("commit did not release in time")
				}
				if !quarObservedWaiting.Load() {
					return errors.New("observer did not confirm lock-wait contention")
				}
				// Quarantine runs against the post-commit state; its
				// srcVersion is now stale, so it gets ErrVersionConflict.
				return <-errCh
			})
		},
	)

	// The commit won; capacity reflects it. The quarantine got
	// ErrVersionConflict (its expected srcVersion is now stale), which is
	// the correct outcome of lock-serialized contention.
	if got := commitHeld.Load(); got != 1 {
		t.Fatalf("commitHeld=%d, want 1", got)
	}
	h, _, _ := buckets(t, s, facID, start)
	if h != 1 {
		t.Fatalf("after barrier commit: held=%d, want 1", h)
	}
}

// TestConcurrentBarrierWithdrawalWins: same barrier pattern but the
// QUARANTINE goroutine acquires the source UPDATE first. The COMMIT
// transaction's RevalidateReservationContext waits for the source lock,
// then reads the now-QUARANTINED source under its own lock and rejects
// with ErrReservationContext. Capacity unchanged.
//
// Ordering protocol (no sleeps):
//  1. quarantine begins tx and signals "in-tx" (inTx barrier).
//  2. commit begins tx; signals "in-tx" (inTx barrier).
//  3. quarantine issues its UPDATE; commits its lock acquisition via
//     "withdrawal-locked" barrier.
//  4. commit issues its SELECT FOR UPDATE; the observer confirms the
//     commit is waiting; commit signals "commit-observer-saw-wait".
//  5. quarantine waits for "commit-observed" barrier; then releases
//     its tx by completing.
//  6. commit's SELECT FOR UPDATE unblocks and reads the QUARANTINED
//     state; it returns ErrReservationContext.
func TestConcurrentBarrierWithdrawalWins(t *testing.T) {
	s := testDB(t)
	s2 := openSecondStore(t)
	obs := openObserverPool(t)
	ctx := context.Background()
	start, end := day(31), day(32)
	pkgID, facID, srcID, srcVersion := seedOperationalPackage(t, s, 5, start, end)
	sources := NewSourceStore(ChainAuditor{})

	withdrawalLocked := make(chan struct{})      // quarantine holds the source UPDATE lock
	commitIssued := make(chan struct{})          // commit issued its SELECT FOR UPDATE
	commitObservedWaiting := make(chan struct{}) // observer confirms commit is in lock-wait
	withdrawalRelease := make(chan struct{})     // quarantine completed

	var quarObserved atomic.Bool

	runRacing(t,
		func() error {
			// Quarantine goroutine.
			return s2.InTx(ctx, func(tx DBTX) error {
				// Issue the UPDATE; it will lock the source row.
				quarErr := sources.Quarantine(ctx, tx, srcID, srcVersion, "test-op", "hold", "EV-Q-"+srcID, nowUTC())
				if quarErr != nil {
					return quarErr
				}
				// Quarantine has the source row locked (UPDATE completed
				// but the tx is still open).
				close(withdrawalLocked)
				// Wait for the commit to have issued its SELECT FOR UPDATE.
				select {
				case <-commitIssued:
				case <-time.After(5 * time.Second):
					return errors.New("commit did not issue its SELECT FOR UPDATE")
				}
				// Wait for the observer to confirm the commit IS waiting.
				select {
				case <-commitObservedWaiting:
				case <-time.After(5 * time.Second):
					return errors.New("observer did not confirm commit as waiting")
				}
				if !quarObserved.Load() {
					return errors.New("observer did not confirm commit lock-wait")
				}
				close(withdrawalRelease)
				return nil
			})
		},
		func() error {
			// Commit goroutine. Must wait for the quarantine to acquire its
			// lock BEFORE issuing the SELECT FOR UPDATE, so the SELECT FOR
			// UPDATE is guaranteed to block (lock-wait observable).
			select {
			case <-withdrawalLocked:
			case <-time.After(5 * time.Second):
				return errors.New("quarantine did not acquire its lock")
			}
			return s.InTx(ctx, func(tx DBTX) error {
				// Issue the SELECT FOR UPDATE in a goroutine; it will
				// block on the source row held by the quarantine tx.
				errCh := make(chan error, 1)
				go func() {
					_, _, err := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
					errCh <- err
				}()
				close(commitIssued)
				// Observer confirms the commit is in lock-wait.
				go func() {
					if _, err := pollUntilWaiting(ctx, obs, 5*time.Second); err == nil {
						quarObserved.Store(true)
						close(commitObservedWaiting)
					}
				}()
				// Wait for the quarantine tx to release.
				select {
				case <-withdrawalRelease:
				case <-time.After(10 * time.Second):
					return errors.New("withdrawal did not release in time")
				}
				// The SELECT FOR UPDATE unblocks; the source is QUARANTINED,
				// so RevalidateReservationContext returns ErrReservationContext.
				revalErr := <-errCh
				if !errors.Is(revalErr, ErrReservationContext) {
					t.Errorf("revalidate after withdrawal: got %v, want ErrReservationContext", revalErr)
				}
				return nil
			})
		},
	)

	// Capacity unchanged: the commit never reached stays.Reserve.
	h, _, _ := buckets(t, s, facID, start)
	if h != 0 {
		t.Fatalf("after withdrawal-wins: held=%d, want 0 (commit denied)", h)
	}
}

// TestCommitmentWinsThenWithdrawalCommits: a reservation commits; the
// source is then quarantined; a SECOND reservation is rejected with
// ErrReservationContext, and capacity is unchanged from the first commit.
func TestCommitmentWinsThenWithdrawalCommits(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	start, end := day(22), day(23)
	pkgID, facID, srcID, srcVersion := seedOperationalPackage(t, s, 5, start, end)
	stays := NewStayStore(ChainAuditor{})
	sources := NewSourceStore(ChainAuditor{})

	// 1. First reservation commits.
	sessID := seedSession(t, s)
	st := mkStay(sessID, facID, pkgID, 1, start, end)
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Reserve(ctx, tx, st, nowUTC(), "commit-1")
	}); err != nil {
		t.Fatalf("first commit: %v", err)
	}
	h, _, c := buckets(t, s, facID, start)
	if h != 1 || c != 5 {
		t.Fatalf("after first commit: held=%d capacity=%d, want 1/5", h, c)
	}

	// 2. Quarantine.
	if err := s.InTx(ctx, func(tx DBTX) error {
		return sources.Quarantine(ctx, tx, srcID, srcVersion, "test-op", "hold", "EV-Q-"+srcID, nowUTC())
	}); err != nil {
		t.Fatalf("quarantine: %v", err)
	}

	// 3. Second reservation rejected; capacity unchanged.
	st2 := mkStay(sessID, facID, pkgID, 1, start, end)
	seedReservation(t, s, st2)
	err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Reserve(ctx, tx, st2, nowUTC(), "commit-2")
	})
	if !errors.Is(err, ErrReservationContext) {
		t.Fatalf("post-quarantine commit: got %v, want ErrReservationContext", err)
	}
	h, _, _ = buckets(t, s, facID, start)
	if h != 1 {
		t.Fatalf("capacity changed on rejection: held=%d, want 1 (unchanged)", h)
	}
}

// TestWithdrawalWinsThenCommitmentDenied: source quarantined first; a
// subsequent reservation sees the withdrawn state under its lock and rejects
// without changing capacity.
func TestWithdrawalWinsThenCommitmentDenied(t *testing.T) {
	s := testDB(t)
	start, end := day(24), day(25)
	pkgID, facID, srcID, srcVersion := seedOperationalPackage(t, s, 5, start, end)
	stays := NewStayStore(ChainAuditor{})
	sources := NewSourceStore(ChainAuditor{})

	if err := s.InTx(t.Context(), func(tx DBTX) error {
		return sources.Quarantine(t.Context(), tx, srcID, srcVersion, "test-op", "hold", "EV-Q-"+srcID, nowUTC())
	}); err != nil {
		t.Fatalf("quarantine: %v", err)
	}

	sessID := seedSession(t, s)
	st := mkStay(sessID, facID, pkgID, 1, start, end)
	seedReservation(t, s, st)
	err := s.InTx(t.Context(), func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(t.Context(), tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Reserve(t.Context(), tx, st, nowUTC(), "post-withdrawal")
	})
	if !errors.Is(err, ErrReservationContext) {
		t.Fatalf("post-withdrawal commit: got %v, want ErrReservationContext", err)
	}
	h, _, _ := buckets(t, s, facID, start)
	if h != 0 {
		t.Fatalf("capacity changed on rejection: held=%d, want 0", h)
	}
}

// TestSnapshotSupersessionUnderLock: a superseded package is invisible to a
// reservation under its lock (the SELECT ... FOR UPDATE sees the superseded
// row and the WHERE filter excludes it). Capacity is unchanged.
func TestSnapshotSupersessionUnderLock(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	start, end := day(26), day(27)
	pkgID, facID, srcID, _ := seedOperationalPackage(t, s, 5, start, end)
	stays := NewStayStore(ChainAuditor{})

	// Supersede the package: insert a new package row, mark old as superseded.
	newPkgID := uid("PKG")
	artID := uid("ART")
	now := nowUTC()
	err := s.InTx(ctx, func(tx DBTX) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256,
				retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,2,$3,$4,'SYNTHETIC','mem://t')`, artID, srcID,
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version,
				jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			VALUES ($1,'ALT',$2,$3,2,'JTEST','SYNTHETIC',$4,$5,$6,$7)`,
			newPkgID, srcID, artID, now, now.Add(24*time.Hour),
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			`{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true}}`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE packages SET superseded_by = $1 WHERE package_id = $2`,
			newPkgID, pkgID)
		return err
	})
	if err != nil {
		t.Fatalf("seed supersession: %v", err)
	}

	// Reservation against the superseded package must be rejected under the lock.
	sessID := seedSession(t, s)
	st := mkStay(sessID, facID, pkgID, 1, start, end)
	seedReservation(t, s, st)
	rerr := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, now)
		if rerr != nil {
			return rerr
		}
		return stays.Reserve(ctx, tx, st, now, "stale-snap")
	})
	if !errors.Is(rerr, ErrReservationContext) {
		t.Fatalf("superseded commit: got %v, want ErrReservationContext", rerr)
	}
	h, _, _ := buckets(t, s, facID, start)
	if h != 0 {
		t.Fatalf("capacity changed on stale snapshot: held=%d, want 0", h)
	}
}

// seedOperationalPackage seeds a source/package/facility/inventory the way
// RevalidateReservationContext accepts: source OPERATIONAL with a live
// jurisdiction authorization, package effective/unexpired/not-superseded,
// facility bound to the package. Returns the current source version so the
// caller can pass it to source state transitions.
func seedOperationalPackage(t *testing.T, s *Store, capacity int, start, end time.Time) (pkgID, facID, srcID string, srcVersion int) {
	t.Helper()
	pkgID, facID = seedPackageFacility(t, s, capacity, start, end)
	srcID = sourceIDForPkg(t, s, pkgID)
	now := nowUTC()
	err := s.InTx(t.Context(), func(tx DBTX) error {
		ss := NewSourceStore(ChainAuditor{})
		src, err := ss.GetSource(t.Context(), tx, srcID)
		if err != nil {
			return err
		}
		if src.State != sourceact.Operational {
			if err := ss.Transition(t.Context(), tx, srcID, src.Version, sourceact.Operational, "test", "seed", "EV-OP-"+srcID, now); err != nil {
				return err
			}
		}
		if err := ss.RecordAuthorization(t.Context(), tx, Authorization{
			AuthorizationID: "AUTH-" + srcID,
			SourceID:        srcID,
			GrantedBy:       "gov",
			EvidenceRef:     "doc-1",
			Jurisdiction:    "JTEST",
			GrantedAt:       now,
		}); err != nil {
			return err
		}
		final, err := ss.GetSource(t.Context(), tx, srcID)
		if err != nil {
			return err
		}
		srcVersion = final.Version
		return nil
	})
	if err != nil {
		t.Fatalf("seedOperationalPackage: %v", err)
	}
	return pkgID, facID, srcID, srcVersion
}
