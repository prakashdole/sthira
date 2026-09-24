package httpserver

import (
	"net/http"
	"strings"

	"sthira/backend/internal/contracts"
)

// WithAllowedOrigins sets the strict list of allowed cross-origin origins (e.g. "https://sthira.gov.in", "http://localhost:5173").
// Wildcard "*" origins are strictly forbidden and filtered out to prevent cross-origin credentials leakage.
func WithAllowedOrigins(origins ...string) Option {
	return func(s *Server) {
		var filtered []string
		for _, o := range origins {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" && trimmed != "*" {
				filtered = append(filtered, trimmed)
			}
		}
		s.allowedOrigins = filtered
		s.cfg.AllowedOrigins = filtered
	}
}

// withCORS wraps the next handler to enforce strict Cross-Origin Resource Sharing (CORS)
// rules based on the configured AllowedOrigins.
//
// Invariants:
// 1. If AllowedOrigins is empty, no CORS headers are emitted (strict same-origin isolation).
// 2. Incoming Origin headers are matched strictly against the allow-list. Wildcard origins
//    ("*") are rejected at configuration time to prevent data leakage in credentialed contexts.
// 3. Preflight OPTIONS requests for allowed origins receive 204 No Content with
//    Access-Control-Allow-Origin, Access-Control-Allow-Methods, Access-Control-Allow-Headers,
//    Access-Control-Max-Age, and Access-Control-Expose-Headers.
// 4. Preflight OPTIONS requests for disallowed origins receive HTTP 403 Forbidden.
// 5. Standard cross-origin requests from allowed origins receive Access-Control-Allow-Origin
//    and Vary: Origin. Standard requests from disallowed origins do not receive CORS headers.
// 6. Requests without an Origin header (e.g., same-origin browser navigations, mobile native clients,
//    curl) pass through unmodified.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// No Origin header: same-origin or non-browser client. Pass through.
			next.ServeHTTP(w, r)
			return
		}

		allowed := s.isOriginAllowed(origin)

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			// Preflight request
			if !allowed {
				s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "cross-origin request not allowed from this origin", "", false)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Client-Timestamp, X-Observability-Token, X-Idempotency-Key, Range, If-None-Match, X-Request-ID")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, ETag, Retry-After, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, ETag, Retry-After, X-Request-ID")
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) isOriginAllowed(origin string) bool {
	if len(s.allowedOrigins) == 0 {
		return false
	}
	cleanOrigin := strings.TrimRight(strings.ToLower(strings.TrimSpace(origin)), "/")
	for _, o := range s.allowedOrigins {
		if strings.TrimRight(strings.ToLower(strings.TrimSpace(o)), "/") == cleanOrigin {
			return true
		}
	}
	return false
}
