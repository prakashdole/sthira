package httpserver

import (
	"net/http"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpjson"
)

// handleLive reports process liveness only. It never inspects dependencies and
// therefore never claims operational readiness.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodGet) {
		return
	}
	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, map[string]any{
		"status":     "LIVE",
		"started_at": s.startedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// handleReady reports dependency/source readiness. Without a prober, or when
// the prober reports a blocker, it answers 503 with BLOCKED. It never reports
// READY from configuration presence alone.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.prober == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable,
			"no readiness prober configured; dependencies unverified", "", true)
		return
	}
	if err := s.prober.Probe(r.Context()); err != nil {
		// Redact internals: log the real reason, return a generic blocker.
		s.logger.Warn("readiness probe failed", "request_id", requestID(r), "reason", err.Error())
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable,
			"dependencies not ready", "", true)
		return
	}
	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, map[string]any{
		"status": "READY",
	})
}

// voiceCommandRequest is the typed chat/transcript command body. The model
// proposal it carries is validated independently against the server snapshot.
// The client-supplied request_id/data_version are NOT trusted: they are only
// correlated against the server-resolved snapshot.
type voiceCommandRequest struct {
	RequestID   string                `json:"request_id"`
	DataVersion string                `json:"data_version"`
	Proposal    contracts.ModelOutput `json:"proposal"`
}

// handleVoiceCommands validates a middle-model proposal against the
// server-resolved snapshot. It performs no write, call or capacity mutation.
//
// Trust boundary: the authoritative data version, jurisdiction and permitted
// IDs are resolved server-side via the ContextResolver. Client-echoed
// request_id/data_version are never treated as proof of a trusted snapshot.
// Until a resolver is wired (P4 binds a real source snapshot), this endpoint
// fails closed with 503 rather than validating against client-supplied values.
func (s *Server) handleVoiceCommands(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.requireJSONContentType(w, r) {
		return
	}
	body, err := s.readBoundedBody(w, r)
	if err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	var req voiceCommandRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{
		MaxBytes: s.cfg.MaxBodyBytes,
		MaxDepth: s.cfg.MaxJSONDepth,
	}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}

	if s.resolver == nil {
		// No authoritative context is available: fail closed. This preserves the
		// blocked operational seam instead of trusting client-echoed values.
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable,
			"no authoritative context resolver configured; proposal cannot be validated", "", true)
		return
	}
	snap, rerr := s.resolver.Resolve(r.Context())
	if rerr != nil {
		s.logger.Warn("context resolution failed", "request_id", requestID(r), "reason", rerr.Error())
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable,
			"context snapshot unavailable", "", true)
		return
	}

	// Correlate the client-echoed data_version against the server snapshot. A
	// mismatch means the client is stale; it is not proof of anything.
	if req.DataVersion != snap.DataVersion {
		s.writeError(w, r, http.StatusConflict, contracts.ErrStaleVersion,
			"client data_version does not match the current server snapshot", "data_version", true)
		return
	}

	// Bind validation to the server-resolved snapshot, not the echoed values.
	if verr := contracts.ValidateModelOutput(req.Proposal, req.RequestID, snap.DataVersion, snap.KnownIDs, snap.EnabledLanguages); verr != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, contracts.ErrValidation,
			"model proposal failed validation: "+verr.Error(), "proposal", false)
		return
	}

	s.writeData(w, r, http.StatusOK, snap.DataVersion, contracts.FreshnessUnknown, map[string]any{
		"validated": true,
		"status":    string(req.Proposal.Status),
	})
}
