package httpserver

import (
	"context"
	"net/http"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/store"
)

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
