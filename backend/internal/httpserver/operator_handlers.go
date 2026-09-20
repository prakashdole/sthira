package httpserver

// P4 operator operations handlers. Endpoint shapes follow plan/trd.md line 58:
//   POST /api/v3/operations/sessions                        — operator session issuance (MFA-gated)
//   POST /api/v3/operations/sources/{id}/transitions        — publish/revoke/supersede (scoped)
//   POST /api/v3/operations/stays/{id}/corrections          — auditable stay correction (scoped)
//
// Operator authority comes from a verified identity plus a second factor (MFA),
// not the OPERATOR label alone (R22). Every operation is jurisdiction-scoped:
// acting on a target outside the operator's jurisdiction is FORBIDDEN. All
// writes are idempotency-keyed and commit atomically with their audit event.

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpjson"
	"sthira/backend/internal/sourceact"
	"sthira/backend/internal/store"
)

// --- operator session issuance ---

type createOperatorSessionRequest struct {
	Jurisdiction string `json:"jurisdiction"`
	// MFAVerified is true only when the caller has completed the second-factor
	// boundary. In this synthetic slice the boundary is the caller's attestation;
	// a real deployment verifies an external MFA proof before calling. Without it
	// no operator session is issued.
	MFAVerified bool `json:"mfa_verified"`
}

func (s *Server) handleCreateOperatorSession(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	if s.store == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "store unavailable", "", true)
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
	var req createOperatorSessionRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	if req.Jurisdiction == "" {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrValidation, "jurisdiction is required", "jurisdiction", false)
		return
	}
	if !req.MFAVerified {
		s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "operator MFA not verified", "mfa_verified", false)
		return
	}
	now := time.Now().UTC()
	sessionID := newID("SESS")
	token, err := store.IssueOperatorSession(r.Context(), s.store.DB(), sessionID, req.Jurisdiction, &now, 8*time.Hour, now)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "failed to issue operator session", "", false)
		return
	}
	s.writeData(w, r, http.StatusCreated, "none", contracts.FreshnessUnknown, map[string]any{
		"session_id":   sessionID,
		"token":        token, // shown once; only its hash is stored
		"principal":    "OPERATOR",
		"jurisdiction": req.Jurisdiction,
		"expires_in":   int((8 * time.Hour).Seconds()),
	})
}

// --- source publish/revoke (OPERATIONAL transitions) ---

type sourceTransitionRequest struct {
	Target         string `json:"target"` // OPERATIONAL | SUSPENDED | RETIRED
	IdempotencyKey string `json:"idempotency_key"`
	Reason         string `json:"reason,omitempty"`
}

func (s *Server) handleSourceTransition(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	op, ok := operatorFrom(r)
	if !ok {
		s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "operator session required", "", false)
		return
	}
	sourceID := r.PathValue("id")
	if !s.requireJSONContentType(w, r) {
		return
	}
	body, err := s.readBoundedBody(w, r)
	if err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	var req sourceTransitionRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	target := sourceact.State(req.Target)
	switch target {
	case sourceact.Operational, sourceact.Suspended, sourceact.Retired:
	default:
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrValidation, "target must be OPERATIONAL, SUSPENDED or RETIRED", "target", false)
		return
	}
	if req.IdempotencyKey == "" {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrValidation, "idempotency_key is required", "idempotency_key", false)
		return
	}

	now := time.Now().UTC()
	sources := store.NewSourceStore(store.ChainAuditor{})
	idem := store.IdempotencyStore{}
	payloadHash := sha256Hex(string(body))
	op_ := "source.transition"
	var resultPayload []byte
	var replay bool

	err = s.store.InTx(r.Context(), func(tx store.DBTX) error {
		res, rep, err := idem.Begin(r.Context(), tx, op.SessionID, op_, req.IdempotencyKey, payloadHash, now.Add(24*time.Hour))
		if err != nil {
			return err
		}
		if rep {
			resultPayload, replay = res, true
			return nil
		}
		src, err := sources.GetSource(r.Context(), tx, sourceID)
		if err != nil {
			return err
		}
		// Jurisdiction scope: the source must hold a currently-valid authorization
		// in the operator's own jurisdiction.
		jur, has, err := sources.AuthorizationJurisdiction(r.Context(), tx, sourceID, now)
		if err != nil {
			return err
		}
		if !has {
			return errNoAuthorization
		}
		if op.Jurisdiction == nil || jur != *op.Jurisdiction {
			return errCrossJurisdiction
		}
		if err := sources.Transition(r.Context(), tx, sourceID, src.Version, target, op.SessionID, req.Reason, newID("EV"), now); err != nil {
			return err
		}
		result := map[string]any{"source_id": sourceID, "target": string(target)}
		return idem.Complete(r.Context(), tx, op.SessionID, op_, req.IdempotencyKey, result)
	})
	if err != nil {
		s.writeOperatorError(w, r, err)
		return
	}
	if replay {
		var data any
		if err := json.Unmarshal(resultPayload, &data); err != nil {
			s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "failed to decode stored result", "", false)
			return
		}
		s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, data)
		return
	}
	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, map[string]any{
		"source_id": sourceID,
		"target":    string(target),
	})
}

// --- auditable stay correction ---

type stayCorrectionRequest struct {
	NewPartySize   int    `json:"new_party_size"`
	IdempotencyKey string `json:"idempotency_key"`
	Reason         string `json:"reason"`
}

func (s *Server) handleStayCorrection(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	op, ok := operatorFrom(r)
	if !ok {
		s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "operator session required", "", false)
		return
	}
	stayID := r.PathValue("id")
	if !s.requireJSONContentType(w, r) {
		return
	}
	body, err := s.readBoundedBody(w, r)
	if err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	var req stayCorrectionRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	if req.NewPartySize < 1 {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrValidation, "new_party_size must be at least 1", "new_party_size", false)
		return
	}
	if req.IdempotencyKey == "" {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrValidation, "idempotency_key is required", "idempotency_key", false)
		return
	}

	now := time.Now().UTC()
	stays := store.NewStayStore(store.ChainAuditor{})
	idem := store.IdempotencyStore{}
	payloadHash := sha256Hex(string(body))
	op_ := "stay.correction"
	var resultPayload []byte
	var replay bool

	err = s.store.InTx(r.Context(), func(tx store.DBTX) error {
		res, rep, err := idem.Begin(r.Context(), tx, op.SessionID, op_, req.IdempotencyKey, payloadHash, now.Add(24*time.Hour))
		if err != nil {
			return err
		}
		if rep {
			resultPayload, replay = res, true
			return nil
		}
		// Jurisdiction scope: the stay's facility must sit in the operator's
		// jurisdiction before any correction is applied.
		jur, err := stays.StayFacilityJurisdiction(r.Context(), tx, stayID)
		if err != nil {
			return err
		}
		if op.Jurisdiction == nil || jur != *op.Jurisdiction {
			return errCrossJurisdiction
		}
		if err := stays.Correct(r.Context(), tx, stayID, req.NewPartySize, op.SessionID, req.Reason, now); err != nil {
			return err
		}
		result := map[string]any{"stay_id": stayID, "new_party_size": req.NewPartySize}
		return idem.Complete(r.Context(), tx, op.SessionID, op_, req.IdempotencyKey, result)
	})
	if err != nil {
		s.writeOperatorError(w, r, err)
		return
	}
	if replay {
		var data any
		if err := json.Unmarshal(resultPayload, &data); err != nil {
			s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "failed to decode stored result", "", false)
			return
		}
		s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, data)
		return
	}
	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, map[string]any{
		"stay_id":        stayID,
		"new_party_size": req.NewPartySize,
	})
}

// --- errors ---

var (
	errCrossJurisdiction = errors.New("target outside operator jurisdiction")
	errNoAuthorization   = errors.New("source has no valid authorization")
)

// writeOperatorError maps store/scope errors onto the operator error envelope.
func (s *Server) writeOperatorError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errCrossJurisdiction):
		s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "target outside operator jurisdiction", "", false)
	case errors.Is(err, errNoAuthorization):
		s.writeError(w, r, http.StatusConflict, contracts.ErrValidation, "source has no valid authorization to operate", "", false)
	case errors.Is(err, store.ErrPayloadConflict), errors.Is(err, store.ErrInProgress):
		s.writeError(w, r, http.StatusConflict, contracts.ErrIdempotencyConflict, "idempotency key conflict", "", false)
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrStayNotFound):
		s.writeError(w, r, http.StatusNotFound, contracts.ErrNotFound, "target not found", "", false)
	case errors.Is(err, store.ErrVersionConflict):
		s.writeError(w, r, http.StatusConflict, contracts.ErrStaleVersion, "stale version", "", false)
	case errors.Is(err, store.ErrInvalidTransition):
		s.writeError(w, r, http.StatusConflict, contracts.ErrValidation, "invalid transition for current state", "", false)
	default:
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "operation failed", "", false)
	}
}
