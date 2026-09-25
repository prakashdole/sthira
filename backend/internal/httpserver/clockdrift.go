package httpserver

import (
	"net/http"
	"time"

	"sthira/backend/internal/contracts"
)

const (
	// HeaderClientTimestamp is the optional client-reported request time in RFC 3339.
	HeaderClientTimestamp = "X-Client-Timestamp"

	// MaxFutureClockDrift bounds how far ahead a client timestamp may be (5 minutes).
	MaxFutureClockDrift = 5 * time.Minute

	// MaxStaleRequestDrift bounds how stale a transactional request may be (15 minutes).
	MaxStaleRequestDrift = 15 * time.Minute
)

// withClockDriftGuard inspects HeaderClientTimestamp when present.
// It fails closed with HTTP 400 when the timestamp is unparseable or outside
// the acceptable window around the server's authoritative clock.
func (s *Server) withClockDriftGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tsHeader := r.Header.Get(HeaderClientTimestamp)
		if tsHeader == "" {
			next(w, r)
			return
		}

		clientTime, err := time.Parse(time.RFC3339, tsHeader)
		if err != nil {
			s.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue,
				"invalid X-Client-Timestamp format; expected RFC 3339", HeaderClientTimestamp, false)
			return
		}

		now := time.Now().UTC()
		diff := clientTime.Sub(now)

		// Reject future timestamps exceeding skew tolerance
		if diff > MaxFutureClockDrift {
			s.writeError(w, r, http.StatusBadRequest, contracts.ErrClockDrift,
				"client timestamp is too far in the future", HeaderClientTimestamp, false)
			return
		}

		// Reject stale requests on modifying/stateful methods (POST, PUT, DELETE, PATCH)
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if now.Sub(clientTime) > MaxStaleRequestDrift {
				s.writeError(w, r, http.StatusBadRequest, contracts.ErrClockDrift,
					"request timestamp is too stale; exceeds maximum allowed skew window", HeaderClientTimestamp, false)
				return
			}
		}

		next(w, r)
	}
}
