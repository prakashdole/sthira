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
	"context"
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

// handleCreateOperatorSession issues an OPERATOR session only after a trusted
// boundary verifies the caller's identity + MFA AND a server-controlled grant
// binds that verified subject to a jurisdiction. The request carries NO
// jurisdiction or MFA flag: both are derived server-side. A caller cannot
// self-grant operator authority. With no verifier configured, issuance fails
// closed.
func (s *Server) handleCreateOperatorSession(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	if s.store == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "store unavailable", "", true)
		return
	}
	if s.operatorVerifier == nil {
		// Fail closed: no trusted boundary means no operator issuance.
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable,
			"operator issuance unavailable: no trusted identity verifier configured", "", true)
		return
	}
	// Verify identity + MFA at the trusted boundary. Missing/invalid/expired
	// evidence is rejected here; no request-body flag is consulted.
	id, err := s.operatorVerifier.VerifyOperator(r)
	if err != nil {
		s.writeError(w, r, http.StatusUnauthorized, contracts.ErrForbidden, "operator identity or MFA not verified", "", false)
		return
	}
	if id.MFAVerifiedAt == nil {
		s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "operator MFA not verified", "", false)
		return
	}
	// Derive the permitted jurisdiction and authorizing grant from a
	// server-controlled grant bound to the verified subject. The caller does not
	// choose it.
	now := time.Now().UTC()
	grant, err := store.OperatorGrantStore{}.FirstLiveGrant(r.Context(), s.store.DB(), id.Subject, now)
	if err != nil {
		if errors.Is(err, store.ErrNoOperatorGrant) {
			s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "no operator grant for verified identity", "", false)
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "grant lookup failed", "", false)
		return
	}
	sessionID := newID("SESS")
	token, err := store.IssueOperatorSession(r.Context(), s.store.DB(), sessionID, grant.Jurisdiction, id.Subject, grant.GrantID, id.MFAVerifiedAt, 8*time.Hour, now)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "failed to issue operator session", "", false)
		return
	}
	s.writeData(w, r, http.StatusCreated, "none", contracts.FreshnessUnknown, map[string]any{
		"session_id":   sessionID,
		"token":        token, // shown once; only its hash is stored
		"principal":    "OPERATOR",
		"jurisdiction": grant.Jurisdiction,
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
	// Bind idempotency to the target resource: the operation name carries the
	// source id, so the same key against a different source is a distinct key
	// (never the first target's replay). Scope is the authenticated operator.
	opName := "source.transition:" + sourceID
	var resultPayload []byte
	var replay bool

	err = s.store.InTx(r.Context(), func(tx store.DBTX) error {
		res, rep, err := idem.Begin(r.Context(), tx, op.SessionID, opName, req.IdempotencyKey, payloadHash, now.Add(24*time.Hour))
		if err != nil {
			return err
		}
		// Revalidate the session's CURRENT grant (expiry/revocation/jurisdiction)
		// before any replay disclosure or mutation; a withdrawn grant denies both.
		if err := revalidateOperatorGrant(r.Context(), tx, op, now); err != nil {
			return err
		}
		// Revalidate current authorization for THIS target before disclosing any
		// replay result or applying a transition. A revoked/out-of-jurisdiction
		// grant must not disclose or act even on a previously-completed key.
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
		if rep {
			resultPayload, replay = res, true
			return nil
		}
		src, err := sources.GetSource(r.Context(), tx, sourceID)
		if err != nil {
			return err
		}
		if err := sources.Transition(r.Context(), tx, sourceID, src.Version, target, op.SessionID, req.Reason, newID("EV"), now); err != nil {
			return err
		}
		result := map[string]any{"source_id": sourceID, "target": string(target)}
		return idem.Complete(r.Context(), tx, op.SessionID, opName, req.IdempotencyKey, result)
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
	// Bind idempotency to the target stay: the operation name carries the stay
	// id, so the same key against a different stay is a distinct key (never the
	// first stay's replay). Scope is the authenticated operator.
	opName := "stay.correction:" + stayID
	var resultPayload []byte
	var replay bool

	err = s.store.InTx(r.Context(), func(tx store.DBTX) error {
		res, rep, err := idem.Begin(r.Context(), tx, op.SessionID, opName, req.IdempotencyKey, payloadHash, now.Add(24*time.Hour))
		if err != nil {
			return err
		}
		// Revalidate the session's CURRENT grant before any replay disclosure or
		// mutation; a withdrawn grant denies both.
		if err := revalidateOperatorGrant(r.Context(), tx, op, now); err != nil {
			return err
		}
		// Revalidate jurisdiction for THIS stay before disclosing any replay
		// result or applying a correction.
		jur, err := stays.StayFacilityJurisdiction(r.Context(), tx, stayID)
		if err != nil {
			return err
		}
		if op.Jurisdiction == nil || jur != *op.Jurisdiction {
			return errCrossJurisdiction
		}
		if rep {
			resultPayload, replay = res, true
			return nil
		}
		if err := stays.Correct(r.Context(), tx, stayID, req.NewPartySize, op.SessionID, req.Reason, now, req.IdempotencyKey); err != nil {
			return err
		}
		result := map[string]any{"stay_id": stayID, "new_party_size": req.NewPartySize}
		return idem.Complete(r.Context(), tx, op.SessionID, opName, req.IdempotencyKey, result)
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

// --- source quarantine ---

type quarantineRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Reason         string `json:"reason"`
}

// handleSourceQuarantine restricts a source's evidence (any state ->
// QUARANTINED). Jurisdiction-scoped, idempotency-keyed to the target source,
// version-guarded, and audit-attributed to the operator. Quarantined evidence is
// excluded from operational use by the context resolver and by reservation
// commit revalidation; existing stays are NOT cancelled or released.
func (s *Server) handleSourceQuarantine(w http.ResponseWriter, r *http.Request) {
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
	var req quarantineRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	if req.IdempotencyKey == "" {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrValidation, "idempotency_key is required", "idempotency_key", false)
		return
	}
	if req.Reason == "" {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrValidation, "reason is required", "reason", false)
		return
	}

	now := time.Now().UTC()
	sources := store.NewSourceStore(store.ChainAuditor{})
	idem := store.IdempotencyStore{}
	payloadHash := sha256Hex(string(body))
	opName := "source.quarantine:" + sourceID
	var resultPayload []byte
	var replay bool

	err = s.store.InTx(r.Context(), func(tx store.DBTX) error {
		res, rep, err := idem.Begin(r.Context(), tx, op.SessionID, opName, req.IdempotencyKey, payloadHash, now.Add(24*time.Hour))
		if err != nil {
			return err
		}
		// Revalidate the session's CURRENT grant before any replay disclosure or
		// mutation; a withdrawn grant denies both.
		if err := revalidateOperatorGrant(r.Context(), tx, op, now); err != nil {
			return err
		}
		// Revalidate jurisdiction for THIS source before disclosing a replay or
		// acting. Scope via the source's current authorization jurisdiction.
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
		if rep {
			resultPayload, replay = res, true
			return nil
		}
		src, err := sources.GetSource(r.Context(), tx, sourceID)
		if err != nil {
			return err
		}
		if err := sources.Quarantine(r.Context(), tx, sourceID, src.Version, op.SessionID, req.Reason, newID("EV"), now); err != nil {
			return err
		}
		result := map[string]any{"source_id": sourceID, "state": string(sourceact.Quarantined)}
		return idem.Complete(r.Context(), tx, op.SessionID, opName, req.IdempotencyKey, result)
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
		"state":     string(sourceact.Quarantined),
	})
}

// --- errors ---

var (
	errCrossJurisdiction = errors.New("target outside operator jurisdiction")
	errNoAuthorization   = errors.New("source has no valid authorization")
)

// revalidateOperatorGrant re-checks, inside the operation's transaction, that
// the session's authorizing grant is still live (unexpired, unrevoked) and still
// binds the session's verified subject and jurisdiction. The grant row is locked
// FOR UPDATE so a concurrent revocation commits only after this operation
// commits — a withdrawn grant cannot be bypassed by a stale authorization check.
// Runs before any replay disclosure or mutation.
func revalidateOperatorGrant(ctx context.Context, tx store.DBTX, op store.Session, now time.Time) error {
	if op.OperatorGrantID == nil {
		return store.ErrGrantInvalid
	}
	g, err := store.OperatorGrantStore{}.LiveGrantForUpdate(ctx, tx, *op.OperatorGrantID)
	if err != nil {
		return err
	}
	return store.OperatorGrantStore{}.ValidateSessionGrant(op, g, now)
}

// writeOperatorError maps store/scope errors onto the operator error envelope.
func (s *Server) writeOperatorError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errCrossJurisdiction):
		s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "target outside operator jurisdiction", "", false)
	case errors.Is(err, store.ErrGrantInvalid):
		s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "operator grant expired, revoked or no longer matches", "", false)
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
	case errors.Is(err, store.ErrQuarantineTerminal):
		s.writeError(w, r, http.StatusConflict, contracts.ErrValidation, "source already quarantined", "", false)
	default:
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "operation failed", "", false)
	}
}
