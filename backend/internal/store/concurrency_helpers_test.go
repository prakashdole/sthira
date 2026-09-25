package store

// Deterministic lock-wait observation helpers for the P4 concurrency tests.
// These connect to an auxiliary observer pool so the test can observe
// PostgreSQL's lock manager state while two test transactions are racing.
// Rather than relying on arbitrary sleeps to establish race ordering,
// the tests inspect pg_stat_activity and pg_blocking_pids to verify that the
// specific intended waiter backend has entered lock-wait state (wait_event_type = 'Lock')
// and is blocked specifically by the expected competing transaction.
// Short polling intervals (e.g. 2ms) only rate-limit the inspection loop;
// they do not determine or establish who wins the race.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// openObserverPool opens a small auxiliary pool used only for lock
// observation. It is separate from the racing transactions so observation is
// independent of their connection lifecycle.
func openObserverPool(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" {
		t.Skip("STHIRA_TEST_DSN unset; skipping observer setup")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("observer Open: %v", err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxIdleTime(time.Minute)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// backendPID returns the PostgreSQL server process ID for the exact connection
// bound to the current transaction.
func backendPID(ctx context.Context, tx DBTX) (int, error) {
	var pid int
	err := tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid)
	return pid, err
}

// isBackendBlockedBy reports whether the specific waiter backend is actively
// waiting on a lock held by expectedBlockerPID. If expectedBlockerPID <= 0,
// it checks whether waiterPID is waiting on any lock.
func isBackendBlockedBy(ctx context.Context, db *sql.DB, waiterPID, expectedBlockerPID int) (bool, error) {
	var blocked bool
	var err error
	if expectedBlockerPID > 0 {
		err = db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE pid = $1
				  AND wait_event_type = 'Lock'
				  AND state = 'active'
				  AND $2 = ANY(pg_blocking_pids($1))
			)`, waiterPID, expectedBlockerPID).Scan(&blocked)
	} else {
		err = db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE pid = $1
				  AND wait_event_type = 'Lock'
				  AND state = 'active'
			)`, waiterPID).Scan(&blocked)
	}
	return blocked, err
}

// pollUntilBlocked polls until the specific waiter backend is demonstrably
// blocked by blockerPID (or any lock holder if blockerPID <= 0). It polls
// with a short ticker up to budget. Sleeps in this loop only rate-limit
// polling; they do not determine race ordering. If budget expires or ctx is
// cancelled, it returns an error.
func pollUntilBlocked(ctx context.Context, db *sql.DB, waiterPID, blockerPID int, budget time.Duration) error {
	deadline := time.Now().Add(budget)
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()

	for {
		blocked, err := isBackendBlockedBy(ctx, db, waiterPID, blockerPID)
		if err == nil && blocked {
			return nil
		}
		if time.Now().After(deadline) {
			if blockerPID > 0 {
				return fmt.Errorf("timeout waiting for backend %d to be blocked by %d", waiterPID, blockerPID)
			}
			return fmt.Errorf("timeout waiting for backend %d to be blocked", waiterPID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// waitingBackends reports the count of all backends whose wait_event_type is
// 'Lock' (excluding the observer's own connection). Retained for diagnostic
// verification and negative tests demonstrating that global contention does not
// satisfy the targeted observer.
func waitingBackends(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM pg_stat_activity
		WHERE wait_event_type = 'Lock'
		  AND state = 'active'
		  AND pid <> pg_backend_pid()`).Scan(&n)
	return n, err
}
