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

// handleReady reports dependency/source readiness with structured subsystem breakdown.
// Without a prober, or when dependencies report a blocker, it answers 503 with NOT_READY.
// It never reports READY from configuration presence alone.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodGet) {
		return
	}
	report := s.CheckReadiness(r.Context())
	if report.Status != "READY" {
		s.logger.Warn("readiness check failed", "request_id", requestID(r), "summary", report.Summary())
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable,
			report.Summary(), "", true)
		return
	}
	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, report)
}

// voiceCommandRequest is the typed chat/transcript command body. The model
// proposal it carries is validated independently against the server snapshot.
// The client-supplied request_id/data_version are NOT trusted: they are only
// correlated against the server-resolved snapshot. The jurisdiction is an
// untrusted lookup input selecting WHICH jurisdiction's snapshot to resolve;
// it never proves authorization or snapshot contents.
type voiceCommandRequest struct {
	RequestID    string                `json:"request_id"`
	DataVersion  string                `json:"data_version"`
	Jurisdiction string                `json:"jurisdiction"`
	Proposal     contracts.ModelOutput `json:"proposal"`
}

// handleVoiceCommands validates a middle-model proposal against the
// server-resolved snapshot. It performs no write, call or capacity mutation.
//
// Trust boundary: the authoritative data version, jurisdiction and permitted
// IDs are resolved server-side via the ContextResolver, scoped to the requested
// jurisdiction. Client-echoed request_id/data_version are never treated as
// proof of a trusted snapshot, and the requested jurisdiction is an untrusted
// lookup input — it selects which snapshot to resolve, never its contents.
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
	// The jurisdiction is required to select which snapshot to resolve. Without
	// it the resolver would have to guess across jurisdictions; that is rejected,
	// never defaulted to the highest-version package.
	if req.Jurisdiction == "" {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrValidation,
			"jurisdiction is required to resolve the authoritative context", "jurisdiction", false)
		return
	}
	snap, rerr := s.resolver.Resolve(r.Context(), req.Jurisdiction)
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
