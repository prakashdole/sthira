package store

import (
	"context"
	"time"
)

// ExpiryWorker expires RESERVED stays whose hold expiry has passed. It is a
// real bounded, retry-safe worker: each tick claims a bounded batch of due
// stays and expires each in its own transaction, so a crash mid-tick leaves at
// most one stay to re-expire on the next tick (expiry is idempotent — Expire on
// an already-transitioned stay returns ErrInvalidTransition and is skipped).
//
// Expiry races safely with arrival/transfer: both use the same version-guarded
// conditional update and inventory conservation guard, so exactly one wins and
// the loser gets a conflict rather than a double-free of capacity.
type ExpiryWorker struct {
	store *Store
	stays *StayStore
	// BatchSize bounds how many due stays one tick processes.
	BatchSize int
	// Interval is how often the worker ticks.
	Interval time.Duration
}

// NewExpiryWorker builds a worker over the store.
func NewExpiryWorker(st *Store, stays *StayStore) *ExpiryWorker {
	return &ExpiryWorker{store: st, stays: stays, BatchSize: 100, Interval: 5 * time.Second}
}

// Run ticks until ctx is cancelled. Each tick expires a bounded batch of due
// holds. A tick error is logged by the caller's supervision; the worker keeps
// running (retry-safe).
func (w *ExpiryWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = w.Tick(ctx)
		}
	}
}

// Tick expires up to BatchSize due RESERVED holds. Returns the number expired.
// Each stay is expired in its own transaction so one failure does not block the
// batch and a crash mid-tick is safe to retry.
func (w *ExpiryWorker) Tick(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	// Claim a bounded batch of due stays.
	rows, err := w.store.db.QueryContext(ctx, `
		SELECT stay_id FROM stays
		WHERE state = 'RESERVED' AND expires_at IS NOT NULL AND expires_at <= $1
		ORDER BY stay_id
		LIMIT $2`, now, w.BatchSize)
	if err != nil {
		return 0, err
	}
	var ids []string
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	expired := 0
	for _, id := range ids {
		err := w.store.InTx(ctx, func(tx DBTX) error {
			return w.stays.Expire(ctx, tx, id, now, "")
		})
		if err == nil {
			expired++
			continue
		}
		// ErrInvalidTransition means a concurrent arrival/transfer won the race;
		// that is safe and not retried. Other errors are transient and the stay
		// remains due for the next tick.
		if err == ErrInvalidTransition {
			continue
		}
	}
	return expired, nil
}
