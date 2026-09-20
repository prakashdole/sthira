package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// OperatorGrantStore persists server-controlled operator authorization grants.
// A grant binds a verified identity subject to a jurisdiction. Grants are
// provisioned out-of-band (never via a public request); operator session
// issuance checks them and fails closed when none matches. This is the
// server-controlled half of operator authority: the trusted boundary verifies
// WHO the principal is (identity + MFA); the grant records WHAT they may act on.
type OperatorGrantStore struct{}

// ErrNoOperatorGrant is returned when no live grant binds the verified subject
// to the jurisdiction.
var ErrNoOperatorGrant = errors.New("store: no operator grant for subject and jurisdiction")

// GrantOperator provisions a grant. Out-of-band administrative path; not exposed
// by any public handler. Idempotent on (subject, jurisdiction).
func (OperatorGrantStore) GrantOperator(ctx context.Context, db DBTX, grantID, subject, jurisdiction, grantedBy string, expiresAt *time.Time, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO operator_grants (grant_id, subject, jurisdiction, granted_by, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (subject, jurisdiction) DO NOTHING`,
		grantID, subject, jurisdiction, grantedBy, now, expiresAt)
	return err
}

// RevokeOperatorGrant withdraws a grant. Idempotent.
func (OperatorGrantStore) RevokeOperatorGrant(ctx context.Context, db DBTX, subject, jurisdiction string, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		UPDATE operator_grants SET revoked_at = $3
		WHERE subject = $1 AND jurisdiction = $2 AND revoked_at IS NULL`,
		subject, jurisdiction, now)
	return err
}

// HasOperatorGrant reports whether a live (unexpired, unrevoked) grant binds the
// verified subject to the jurisdiction.
func (OperatorGrantStore) HasOperatorGrant(ctx context.Context, db DBTX, subject, jurisdiction string, now time.Time) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM operator_grants
			WHERE subject = $1 AND jurisdiction = $2
			  AND revoked_at IS NULL
			  AND (expires_at IS NULL OR expires_at > $3)
		)`, subject, jurisdiction, now).Scan(&exists)
	return exists, err
}

// FirstJurisdiction returns the jurisdiction of the subject's live grant, or
// ErrNoOperatorGrant when none. The verified subject does not choose a
// jurisdiction; issuance derives it from the server-controlled grant. When a
// subject holds multiple live grants this returns the earliest-created one; the
// single-grant case is the supported operator model here.
func (OperatorGrantStore) FirstJurisdiction(ctx context.Context, db DBTX, subject string, now time.Time) (string, error) {
	var j string
	err := db.QueryRowContext(ctx, `
		SELECT jurisdiction FROM operator_grants
		WHERE subject = $1
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $2)
		ORDER BY created_at ASC LIMIT 1`, subject, now).Scan(&j)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoOperatorGrant
	}
	if err != nil {
		return "", err
	}
	return j, nil
}
