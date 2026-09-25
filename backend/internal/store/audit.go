package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// AuditEvent is one append-only audit record. The hash chain links each event
// to its predecessor for tamper evidence; per trd.md the chain alone is not
// proof against a privileged rewrite, so the DB role also revokes UPDATE/DELETE.
type AuditEvent struct {
	EventID   string
	OccuredAt time.Time
	ActorID   string
	Action    string
	SubjectID string
	Outcome   string
	Reason    string
	FromState string
	ToState   string
}

// AuditSink appends audit events with a hash chain. It runs inside the caller's
// transaction so the change and its audit commit atomically.
type AuditSink interface {
	Record(ctx context.Context, db DBTX, ev AuditEvent) error
}

// ChainAuditor is an AuditSink that maintains the prev_hash -> event_hash chain
// by reading the latest event inside the same transaction.
type ChainAuditor struct{}

// genesisHash is the prev_hash of the first event in the chain.
const genesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// Record appends one event. It reads the current chain head inside the
// transaction (serializing concurrent appends via the identity PK) and writes
// the new event with its computed hash.
func (ChainAuditor) Record(ctx context.Context, db DBTX, ev AuditEvent) error {
	// Serialize appends: the head read and the insert must be atomic, else two
	// concurrent transactions read the same head and write divergent successors,
	// breaking the chain. A transaction-scoped advisory lock held to commit gives
	// that serialization without a table lock. (The identity PK alone does not:
	// both txs can read head H before either commits.)
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_xact_lock(7301)`); err != nil {
		return fmt.Errorf("store: lock audit chain: %w", err)
	}
	prev := genesisHash
	var h string
	err := db.QueryRowContext(ctx,
		`SELECT event_hash FROM audit_events ORDER BY event_seq DESC LIMIT 1`).Scan(&h)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: read audit head: %w", err)
	}
	if h != "" {
		prev = h
	}
	sum := computeEventHash(prev, ev)
	_, err = db.ExecContext(ctx, `
		INSERT INTO audit_events
			(event_id, occurred_at, actor_id, action, subject_id, outcome, reason,
			 from_state, to_state, prev_hash, event_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		ev.EventID, ev.OccuredAt, ev.ActorID, ev.Action, ev.SubjectID, ev.Outcome,
		nullIfEmpty(ev.Reason), nullIfEmpty(ev.FromState), nullIfEmpty(ev.ToState),
		prev, sum)
	return err
}

// computeEventHash hashes prev_hash with the canonical event fields.
func computeEventHash(prev string, ev AuditEvent) string {
	h := sha256.New()
	h.Write([]byte(prev))
	h.Write([]byte{0})
	for _, f := range []string{
		ev.EventID, ev.OccuredAt.UTC().Format(time.RFC3339Nano), ev.ActorID,
		ev.Action, ev.SubjectID, ev.Outcome, ev.Reason, ev.FromState, ev.ToState,
	} {
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyChain recomputes the chain over all events and reports the first
// mismatch, or nil if the chain is intact. Used by restore/consistency checks.
func VerifyChain(ctx context.Context, db DBTX) error {
	rows, err := db.QueryContext(ctx, `
		SELECT event_id, occurred_at, actor_id, action, subject_id, outcome,
		       COALESCE(reason,''), COALESCE(from_state,''), COALESCE(to_state,''),
		       prev_hash, event_hash
		FROM audit_events ORDER BY event_seq`)
	if err != nil {
		return err
	}
	defer rows.Close()
	prev := genesisHash
	for rows.Next() {
		var ev AuditEvent
		var prevHash, eventHash string
		if err := rows.Scan(&ev.EventID, &ev.OccuredAt, &ev.ActorID, &ev.Action,
			&ev.SubjectID, &ev.Outcome, &ev.Reason, &ev.FromState, &ev.ToState,
			&prevHash, &eventHash); err != nil {
			return err
		}
		if prevHash != prev {
			return fmt.Errorf("store: audit chain break before event %s", ev.EventID)
		}
		if computeEventHash(prevHash, ev) != eventHash {
			return fmt.Errorf("store: audit event %s hash mismatch", ev.EventID)
		}
		prev = eventHash
	}
	return rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
