package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// IdempotencyStore persists scoped idempotency keys. Scope is actor/session +
// operation + key; the payload hash binds the key to one exact request. A lost
// response after commit is retrievable with the same key; a different payload
// under the same key is a conflict (contracts.ErrIdempotencyConflict).
type IdempotencyStore struct{}

// ErrPayloadConflict is returned when a key is reused with a different payload.
var ErrPayloadConflict = errors.New("store: idempotency payload conflict")

// ErrInProgress is returned when the same key is currently being processed.
var ErrInProgress = errors.New("store: idempotency key in progress")

// Begin claims a key for (scope, operation) with the given payload hash. It
// returns replay=true with the stored result when the key already COMPLETED with
// the same payload. It returns ErrPayloadConflict for a reused key with a
// different payload, and ErrInProgress when a matching IN_PROGRESS row exists.
func (IdempotencyStore) Begin(ctx context.Context, db DBTX, scope, operation, key, payloadHash string, expiresAt time.Time) (result []byte, replay bool, err error) {
	// Try to claim the key. RowsAffected==1 means we just inserted the
	// IN_PROGRESS row, so the key is ours: return replay=false without reading
	// back our own row (which would misread as a concurrent in-progress key).
	// RowsAffected==0 means the key already exists; inspect that row.
	res, err := db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (scope, operation, idem_key, payload_hash, state, expires_at)
		VALUES ($1, $2, $3, $4, 'IN_PROGRESS', $5)
		ON CONFLICT (scope, operation, idem_key) DO NOTHING`,
		scope, operation, key, payloadHash, expiresAt)
	if err != nil {
		return nil, false, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 1 {
		return nil, false, nil
	}

	var existingHash, state string
	var stored []byte
	row := db.QueryRowContext(ctx, `
		SELECT payload_hash, state, result
		FROM idempotency_keys
		WHERE scope = $1 AND operation = $2 AND idem_key = $3`,
		scope, operation, key)
	if err := row.Scan(&existingHash, &state, &stored); err != nil {
		return nil, false, err
	}
	if existingHash != payloadHash {
		return nil, false, ErrPayloadConflict
	}
	switch state {
	case "COMPLETED":
		return stored, true, nil
	case "IN_PROGRESS":
		return nil, false, ErrInProgress
	default: // FAILED: allow retry by re-claiming below
		_, err = db.ExecContext(ctx, `
			UPDATE idempotency_keys SET state = 'IN_PROGRESS'
			WHERE scope = $1 AND operation = $2 AND idem_key = $3 AND state = 'FAILED'`,
			scope, operation, key)
		return nil, false, err
	}
}

// Complete marks a claimed key COMPLETED and stores the result for replay. It
// must run in the same transaction as the state change it protects.
func (IdempotencyStore) Complete(ctx context.Context, db DBTX, scope, operation, key string, result any) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return execConditional(ctx, db, false, `
		UPDATE idempotency_keys SET state = 'COMPLETED', result = $4
		WHERE scope = $1 AND operation = $2 AND idem_key = $3 AND state = 'IN_PROGRESS'`,
		scope, operation, key, b)
}

// Fail marks a claimed key FAILED so a later retry may re-claim it.
func (IdempotencyStore) Fail(ctx context.Context, db DBTX, scope, operation, key string) error {
	return execConditional(ctx, db, false, `
		UPDATE idempotency_keys SET state = 'FAILED'
		WHERE scope = $1 AND operation = $2 AND idem_key = $3 AND state = 'IN_PROGRESS'`,
		scope, operation, key)
}

// Sweep removes expired keys past the supported retry window.
func (IdempotencyStore) Sweep(ctx context.Context, db DBTX, now time.Time) (int64, error) {
	res, err := db.ExecContext(ctx,
		`DELETE FROM idempotency_keys WHERE expires_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// compile-time check that sql.ErrNoRows is referenced (kept for future reads).
var _ = sql.ErrNoRows
