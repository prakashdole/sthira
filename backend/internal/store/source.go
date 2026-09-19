package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"sthira/backend/internal/sourceact"
)

// SourceStore persists the sourceact lifecycle durably. The lifecycle rules
// (legal transitions, OPERATIONAL gating) stay in sourceact; this type owns the
// SQL and the atomic version guard. A state change, its audit event and its
// outbox record are written in one transaction via InTx.
type SourceStore struct {
	audit AuditSink
}

// NewSourceStore builds a SourceStore. audit may be nil (no audit written).
func NewSourceStore(audit AuditSink) *SourceStore { return &SourceStore{audit: audit} }

// InsertSource registers a newly DISCOVERED source at version 1.
func (s *SourceStore) InsertSource(ctx context.Context, db DBTX, src sourceact.Source) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5)`,
		src.SourceID, src.GovernmentOwner, src.OfficialDomain, string(src.State), src.UpdatedAt)
	return err
}

// GetSource loads a source by id.
func (s *SourceStore) GetSource(ctx context.Context, db DBTX, sourceID string) (sourceact.Source, error) {
	var src sourceact.Source
	var state string
	err := db.QueryRowContext(ctx, `
		SELECT source_id, government_owner, official_domain, state, version, updated_at
		FROM sources WHERE source_id = $1`, sourceID).
		Scan(&src.SourceID, &src.GovernmentOwner, &src.OfficialDomain, &state, &src.Version, &src.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return sourceact.Source{}, ErrNotFound
	}
	if err != nil {
		return sourceact.Source{}, err
	}
	src.State = sourceact.State(state)
	return src, nil
}

// Transition atomically moves a source to target, requiring the caller's
// expected version. The conditional UPDATE matches no row when the version is
// stale (conflict) or the source is missing; the two are distinguished by a
// follow-up existence check so callers get ErrNotFound vs ErrVersionConflict.
// The audit event and outbox record are written in the same transaction.
func (s *SourceStore) Transition(ctx context.Context, db DBTX, sourceID string, expectedVersion int, target sourceact.State, actorID, reason, eventID string, now time.Time) error {
	err := execConditional(ctx, db, false, `
		UPDATE sources
		SET state = $3, version = version + 1, updated_at = $4
		WHERE source_id = $1 AND version = $2`,
		sourceID, expectedVersion, string(target), now)
	if errors.Is(err, ErrVersionConflict) {
		// Distinguish stale version from a missing source.
		if _, gerr := s.GetSource(ctx, db, sourceID); errors.Is(gerr, ErrNotFound) {
			return ErrNotFound
		}
		return ErrVersionConflict
	}
	if err != nil {
		return err
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, db, AuditEvent{
			EventID:   eventID,
			OccuredAt: now,
			ActorID:   actorID,
			Action:    "SOURCE_STATE_TRANSITION",
			SubjectID: sourceID,
			Outcome:   "OK",
			Reason:    reason,
			ToState:   string(target),
		}); err != nil {
			return err
		}
	}
	return nil
}

// RecordAuthorization stores the authorization evidence for a source. Source
// activation requires recorded evidence, not merely advancing an enum: the
// AUTHORIZED -> OPERATIONAL transition must reference an authorization row.
func (s *SourceStore) RecordAuthorization(ctx context.Context, db DBTX, a Authorization) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO source_authorizations
			(authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		a.AuthorizationID, a.SourceID, a.GrantedBy, a.EvidenceRef, a.Jurisdiction, a.GrantedAt, a.ExpiresAt)
	return err
}

// HasAuthorization reports whether a source has a currently-valid authorization
// record for the jurisdiction. Used to gate the OPERATIONAL transition.
func (s *SourceStore) HasAuthorization(ctx context.Context, db DBTX, sourceID, jurisdiction string, now time.Time) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM source_authorizations
			WHERE source_id = $1 AND jurisdiction = $2
			  AND (expires_at IS NULL OR expires_at > $3)
		)`, sourceID, jurisdiction, now).Scan(&exists)
	return exists, err
}

// Authorization is recorded source-activation authorization evidence.
type Authorization struct {
	AuthorizationID string
	SourceID        string
	GrantedBy       string
	EvidenceRef     string
	Jurisdiction    string
	GrantedAt       time.Time
	ExpiresAt       *time.Time
}
