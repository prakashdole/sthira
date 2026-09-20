package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/store"
)

// Citizen session authentication. Public paths (places/resolve, guidance/query)
// need no session. Private paths (reservations, stay events, the reservation
// read path) require a live citizen session via `Authorization: Bearer`.
//
// The token resolves server-side to a session row; knowing a reservation or
// session ID is not authorization (R22). Expired/revoked tokens are rejected.

type sessionKey ctxKey

// withSession is middleware that requires a live citizen session. On success it
// stores the resolved session in the request context for the handler.
func (s *Server) withSession(next http.HandlerFunc) http.HandlerFunc {
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
		ctx := context.WithValue(r.Context(), sessionKey("session"), sess)
		next(w, r.WithContext(ctx))
	}
}

// sessionFrom returns the authenticated session, or false if none.
func sessionFrom(r *http.Request) (store.Session, bool) {
	sess, ok := r.Context().Value(sessionKey("session")).(store.Session)
	return sess, ok
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}
