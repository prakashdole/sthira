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
type voiceCommandRequest struct {
	RequestID   string                `json:"request_id"`
	DataVersion string                `json:"data_version"`
	Proposal    contracts.ModelOutput `json:"proposal"`
}

// handleVoiceCommands validates a middle-model proposal against the current
// server snapshot. It performs no write, call or capacity mutation; it only
// confirms whether the proposal is valid against the active context. This is
// the P1 contract slice; real context resolution arrives with P2/P4.
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

	// Bind validation to the server's current snapshot, not the echoed values.
	if verr := contracts.ValidateModelOutput(req.Proposal, req.RequestID, req.DataVersion, s.knownIDs, s.enabledLanguages); verr != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, contracts.ErrValidation,
			"model proposal failed validation: "+verr.Error(), "proposal", false)
		return
	}

	s.writeData(w, r, http.StatusOK, req.DataVersion, contracts.FreshnessUnknown, map[string]any{
		"validated": true,
		"status":    string(req.Proposal.Status),
	})
}
