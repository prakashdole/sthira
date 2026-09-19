package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ReadinessProber actively probes the database and migration state. It reports
// database/application readiness only; it never claims operational source
// readiness (that requires an OPERATIONAL authorized source, checked separately
// by sourceact.RequireOperational). It satisfies httpserver.ReadinessProber.
//
// The probe pings the DB and verifies the applied migration revision matches
// the expected one. Errors are returned for the handler to redact; no secret or
// DSN detail is included in the returned text.
type ReadinessProber struct {
	db *sql.DB
	// expectedRevision is the highest migration revision the binary expects.
	expectedRevision int
}

// NewReadinessProber builds a prober over an open DB. expectedRevision is the
// migration revision the schema_migrations table must have reached.
func NewReadinessProber(db *sql.DB, expectedRevision int) *ReadinessProber {
	return &ReadinessProber{db: db, expectedRevision: expectedRevision}
}

// Probe returns nil when the DB is reachable and migrations are current. The
// returned error text is safe to log; the handler redacts it before responding.
func (p *ReadinessProber) Probe(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("database handle not configured")
	}
	if err := p.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %s", redact(err))
	}
	var rev int
	err := p.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(revision), 0) FROM schema_migrations`).Scan(&rev)
	if err != nil {
		return fmt.Errorf("migration state unreadable: %s", redact(err))
	}
	if rev < p.expectedRevision {
		return fmt.Errorf("schema at revision %d, expected %d", rev, p.expectedRevision)
	}
	return nil
}

// redact strips connection detail that could carry DSN/user material, keeping
// the failure class without leaking secrets.
func redact(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, "password"); i >= 0 {
		return msg[:i] + "[redacted]"
	}
	return msg
}
