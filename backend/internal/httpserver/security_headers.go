package httpserver

import (
	"net/http"
	"strings"
)

// withSecurityHeaders wraps an http.Handler with defensive HTTP response headers.
// These headers apply across all status codes (including errors, 404, and 429):
//
//   - X-Frame-Options: DENY (anti-clickjacking)
//   - X-Content-Type-Options: nosniff (anti-MIME-confusion)
//   - Content-Security-Policy: default-src 'none'; frame-ancestors 'none' (API lockdown)
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Permissions-Policy: geolocation=(), microphone=(), camera=() (device isolation)
//   - X-XSS-Protection: 0 (OWASP modern standard)
//
// Additionally, sensitive transactional routes (sessions, reservations, operator operations)
// receive "Cache-Control: no-store, no-cache, must-revalidate, private" and "Pragma: no-cache".
// Public delivery channels (/manifest, /packages, /resources) retain their declared
// public cacheability headers.
func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("X-XSS-Protection", "0")

		path := r.URL.Path
		if isSensitiveRoute(path) {
			h.Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			h.Set("Pragma", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
}

// isSensitiveRoute identifies routes that must never be cached by intermediaries or browsers.
func isSensitiveRoute(path string) bool {
	return strings.HasPrefix(path, "/api/v3/sessions") ||
		strings.HasPrefix(path, "/api/v3/reservations") ||
		strings.HasPrefix(path, "/api/v3/operations") ||
		path == observabilityRoute
}
