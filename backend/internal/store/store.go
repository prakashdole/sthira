// Package store is the P3 durable persistence layer for the Go backend. It
// owns SQL and transaction boundaries for sources/artifacts, versioned facts,
// packages, sessions, reservations/inventory, idempotency and audit/outbox.
//
// Driver posture: the layer is written against database/sql via the small DBTX
// seam so the SQL, transaction scope and optimistic-concurrency logic are
// concrete and reviewable without a live database. The PostgreSQL driver
// (pgx stdlib) is wired in Open(); fetching that driver requires network access
// the build sandbox denies, so real-DB verification is BLOCKED_EXTERNAL. No
// in-memory substitute is provided here: an in-memory green cannot satisfy the
// PostgreSQL/PostGIS acceptance bar (trd.md T05, prompt.md P3).
//
// Optimistic concurrency: every mutable row carries a version. Mutations use an
// atomic conditional UPDATE ... WHERE id = $1 AND version = $2 and inspect
// RowsAffected; zero rows means a stale expected version (conflict) or a missing
// row, never a silent success. Incrementing under an in-process mutex alone is
// not acceptable; the conditional update is the real guard.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// ErrVersionConflict is returned when an atomic conditional update matched no
// row because the caller's expected version was stale.
var ErrVersionConflict = errors.New("store: version conflict")

// ErrConflict is returned when an operation conflicts with an existing immutable record.
var ErrConflict = errors.New("store: conflict")

// ErrNotFound is returned when the target row does not exist.
var ErrNotFound = errors.New("store: not found")

// SchemaRevision is the highest migration revision this binary expects. It is
// the single source of truth the readiness prober checks schema_migrations
// against; bump it when a new migration is added (0001 -> 1, 0002 -> 2, ...).
// Readiness fails on an outdated database rather than assuming revision 1.
//
// Current value matches backend/migrations/0009_p6_template_binding.sql; the
// binary's queries depend on the source_id and template_sha256 columns added
// by migrations 0008/0009. A DB at revision 7 or 8 must not pass readiness.
const SchemaRevision = 9

// DBTX is the minimal database/sql surface the layer needs, satisfied by both
// *sql.DB and *sql.Tx. This keeps every repository method runnable inside or
// outside an explicit transaction without duplicating SQL.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store is the durable persistence root. It wraps a *sql.DB and exposes
// transaction-scoped work via InTx.
type Store struct {
	db *sql.DB
}

// New wraps an already-open *sql.DB. The driver must be registered by the
// caller (see Open for the intended pgx wiring).
func New(db *sql.DB) *Store { return &Store{db: db} }

// DB exposes the underlying pool for wiring (readiness prober, repositories).
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the underlying pool. Called on graceful shutdown.
func (s *Store) Close() error { return s.db.Close() }

// Open registers the pgx stdlib driver (imported above) and opens a connection
// pool against the given PostgreSQL DSN. The pool is tuned conservatively and a
// startup Ping verifies connectivity before the store is handed out, so a bad
// DSN fails fast at boot rather than on first use.
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open pgx: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{db: db}, nil
}

// InTx runs fn inside a single database transaction. State change, audit event
// and outbox record are written by the repositories using the same *sql.Tx, so
// they commit or roll back atomically (T05). fn receives the Tx as a DBTX.
func (s *Store) InTx(ctx context.Context, fn func(tx DBTX) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// execConditional runs an atomic conditional UPDATE/DELETE and translates zero
// affected rows into ErrVersionConflict (or ErrNotFound when notFound is true).
// This is the single guard every optimistic mutation routes through.
func execConditional(ctx context.Context, db DBTX, notFound bool, query string, args ...any) error {
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if notFound {
			return ErrNotFound
		}
		return ErrVersionConflict
	}
	return nil
}
