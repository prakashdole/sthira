package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/store"
)

// VerifiedIdentity is the output of a trusted identity/MFA boundary. The server
// never derives it from request-body flags: it comes only from an
// OperatorVerifier the operator wired server-side.
type VerifiedIdentity struct {
	// Subject is the stable, verified principal identifier (e.g. an IdP subject
	// claim). Operator grants bind to this, never to a caller-chosen label.
	Subject string
	// MFAVerifiedAt is when the second factor was verified. Nil means MFA was
	// not completed; operator issuance requires it non-nil.
	MFAVerifiedAt *time.Time
}

// OperatorVerifier is the seam to a trusted identity + MFA boundary (e.g. an
// OIDC/mTLS identity provider). It verifies the request's authentication
// evidence and returns the verified identity. The production binary wires a real
// verifier; a synthetic verifier exists only in tests and is never selectable by
// an ordinary request. When no verifier is configured, operator issuance fails
// closed.
type OperatorVerifier interface {
	// VerifyOperator authenticates the request and returns the verified identity
	// with MFA evidence, or an error when evidence is missing, invalid or expired.
	VerifyOperator(r *http.Request) (VerifiedIdentity, error)
}

// ErrNoOperatorVerifier is returned when issuance is attempted with no trusted
// verifier configured (fail closed).
var ErrNoOperatorVerifier = errors.New("no trusted operator identity verifier configured")

// Operator authentication and authorization. An OPERATOR session is not
// stronger authentication by label alone (R22): operational access requires a
// verified identity and a second factor, recorded as mfa_verified_at on the
// session. withOperator rejects non-operator sessions, operators without MFA,
// and (in the handlers) any operation outside the operator's jurisdiction.

type operatorKey ctxKey

// withOperator is middleware that requires a live OPERATOR session with
// verified MFA. On success it stores the resolved session in the request
// context. Cross-jurisdiction scoping is enforced per-handler, where the
// target's jurisdiction is known.
func (s *Server) withOperator(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "store unavailable", "", true)
			return
		}
		token := bearerToken(r)
		if token == "" {
			s.writeError(w, r, http.StatusUnauthorized, contracts.ErrForbidden, "missing bearer token", "", false)
			return
		}
		sess, err := store.Authenticate(r.Context(), s.store.DB(), token, time.Now().UTC())
		if err != nil {
			s.writeError(w, r, http.StatusUnauthorized, contracts.ErrForbidden, "session invalid, expired or revoked", "", false)
			return
		}
		if sess.PrincipalKind != "OPERATOR" {
			s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "operator session required", "", false)
			return
		}
		if sess.MFAVerifiedAt == nil {
			s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "operator MFA not verified", "", false)
			return
		}
		ctx := context.WithValue(r.Context(), operatorKey("operator"), sess)
		next(w, r.WithContext(ctx))
	}
}

// operatorFrom returns the authenticated operator session, or false if none.
func operatorFrom(r *http.Request) (store.Session, bool) {
	sess, ok := r.Context().Value(operatorKey("operator")).(store.Session)
	return sess, ok
}
