package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

// Session issuance and authentication. Citizen sessions are account-free (R22):
// issuance requires no civil identity. The bearer token is unguessable (32
// crypto/rand bytes); only its SHA-256 hash is stored in credential_ref, never
// the raw token. Sessions expire and can be revoked. Knowing a session or
// reservation ID is not authorization — the bearer token is.
//
// Operator sessions are a separate, strongly-authenticated boundary (Part C);
// this store records only the session row and its principal kind/jurisdiction.

// ErrSessionInvalid is returned when a token does not resolve to a live session.
var ErrSessionInvalid = errors.New("store: session invalid, expired or revoked")

// Session is an authenticated principal.
type Session struct {
	SessionID     string
	PrincipalKind string // CITIZEN | OPERATOR
	Jurisdiction  *string
	ExpiresAt     time.Time
	// MFAVerifiedAt is when the operator's second factor was verified. Nil for
	// citizens and for operators who have not completed MFA; operational access
	// requires it non-nil.
	MFAVerifiedAt *time.Time
	// OperatorSubject is the verified identity subject this operator session was
	// issued for. Nil for citizens and for legacy operator sessions issued before
	// trusted identity was persisted (those are revoked at migration 0005 and can
	// no longer act). Audit attribution traces event -> session -> this subject.
	OperatorSubject *string
	// OperatorGrantID is the grant this session was issued under. The current
	// validity of that grant is re-checked on every protected operation.
	OperatorGrantID *string
}

// ErrMFARequired is returned when an operator session lacks MFA verification.
var ErrMFARequired = errors.New("store: operator MFA not verified")

// hashToken hashes a bearer token for storage. The raw token is never stored.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IssueCitizenSession creates a CITIZEN session and returns the raw bearer
// token (shown to the caller once) plus the session ID. Only the token hash is
// persisted. ttl bounds the session lifetime.
func IssueCitizenSession(ctx context.Context, db DBTX, sessionID string, ttl time.Duration, now time.Time) (token string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token = hex.EncodeToString(raw)
	_, err = db.ExecContext(ctx, `
		INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, created_at, expires_at)
		VALUES ($1, 'CITIZEN', NULL, $2, $3, $4)`,
		sessionID, hashToken(token), now, now.Add(ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

// IssueOperatorSession creates a jurisdiction-scoped OPERATOR session bound to
// a verified identity and the grant that authorized it. It requires verified
// MFA evidence (mfaVerifiedAt non-nil), the verified subject, and the grant ID.
// Jurisdiction is required and must match the grant's. Only the token hash is
// persisted. Persisting subject + grant lets every protected operation re-check
// the CURRENT grant and lets audit trace event -> session -> verified identity.
func IssueOperatorSession(ctx context.Context, db DBTX, sessionID, jurisdiction, subject, grantID string, mfaVerifiedAt *time.Time, ttl time.Duration, now time.Time) (token string, err error) {
	if jurisdiction == "" {
		return "", errors.New("store: operator jurisdiction required")
	}
	if subject == "" {
		return "", errors.New("store: operator verified subject required")
	}
	if grantID == "" {
		return "", errors.New("store: operator grant id required")
	}
	if mfaVerifiedAt == nil {
		return "", ErrMFARequired
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token = hex.EncodeToString(raw)
	_, err = db.ExecContext(ctx, `
		INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, created_at, expires_at, mfa_verified_at, operator_subject, operator_grant_id)
		VALUES ($1, 'OPERATOR', $2, $3, $4, $5, $6, $7, $8)`,
		sessionID, jurisdiction, hashToken(token), now, now.Add(ttl), mfaVerifiedAt, subject, grantID)
	if err != nil {
		return "", err
	}
	return token, nil
}

// Authenticate resolves a raw bearer token to a live session. It returns
// ErrSessionInvalid for an unknown, expired or revoked token.
func Authenticate(ctx context.Context, db DBTX, token string, now time.Time) (Session, error) {
	var s Session
	var revokedAt *time.Time
	err := db.QueryRowContext(ctx, `
		SELECT session_id, principal_kind, jurisdiction, expires_at, revoked_at, mfa_verified_at,
		       operator_subject, operator_grant_id
		FROM sessions WHERE credential_ref = $1`, hashToken(token)).
		Scan(&s.SessionID, &s.PrincipalKind, &s.Jurisdiction, &s.ExpiresAt, &revokedAt, &s.MFAVerifiedAt,
			&s.OperatorSubject, &s.OperatorGrantID)
	if err != nil {
		return Session{}, ErrSessionInvalid
	}
	if revokedAt != nil || !now.Before(s.ExpiresAt) {
		return Session{}, ErrSessionInvalid
	}
	return s, nil
}

// RevokeSession marks a session revoked. Idempotent.
func RevokeSession(ctx context.Context, db DBTX, sessionID string, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = $2 WHERE session_id = $1 AND revoked_at IS NULL`,
		sessionID, now)
	return err
}
