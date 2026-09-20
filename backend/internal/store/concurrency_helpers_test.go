package store

// Deterministic lock-wait observation helpers for the P4 concurrency tests.
// These connect to a third pool so the test can observe the system's own
// lock table while two test transactions are racing on a row. Polling
// pg_stat_activity.wait_event_type='Lock' from the test process is a
// reliable, no-sleep substitute for "wait until the competing transaction
// is in lock-wait state"; the test uses it to prove the FOR UPDATE in
// RevalidateReservationContext actually blocks the competing source
// UPDATE before releasing.

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// openObserverPool opens a small auxiliary pool used only for pg_locks
// polling. It is separate from the racing transactions so observation is
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

// waitingBackends reports the count of backends whose wait_event_type is
// 'Lock' (i.e. currently waiting on a row/relation lock). Excludes the
// observer's own session so it does not observe itself.
func waitingBackends(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM pg_stat_activity
		WHERE wait_event_type = 'Lock'
		  AND state = 'active'
		  AND pid <> pg_backend_pid()`).Scan(&n)
	return n, err
}

// pollUntilWaiting waits up to budget for at least one non-observer
// backend to enter lock-wait state. Returns the count observed on
// success or an error if the wait never sees contention.
func pollUntilWaiting(ctx context.Context, db *sql.DB, budget time.Duration) (int, error) {
	deadline := time.Now().Add(budget)
	for {
		w, err := waitingBackends(ctx, db)
		if err == nil && w > 0 {
			return w, nil
		}
		if time.Now().After(deadline) {
			return w, errors.New("never observed lock-wait contention")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
