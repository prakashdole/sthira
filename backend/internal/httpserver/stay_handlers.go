package httpserver

// P4 citizen destination/stay handlers. Endpoint shapes follow plan/trd.md:
//   POST /api/v3/sessions                  — citizen session issuance (public)
//   POST /api/v3/places/resolve            — ambiguity-aware place lookup (public)
//   POST /api/v3/guidance/query            — eligible destinations (public read)
//   POST /api/v3/reservations              — explicit confirmed choice (session)
//   GET  /api/v3/reservations/{id}         — owner/operator read (session)
//   POST /api/v3/reservations/{id}/events  — arrive/cancel/depart/extend/transfer
//
// Preview (guidance/query) is read-only and separate from the explicit
// reservation. A reservation binds facility+route+source snapshot+party size and
// revalidates at commit; a capacity conflict is recoverable and never silently
// substitutes another destination. All writes are idempotency-keyed and commit
// atomically with their audit/outbox records.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpjson"
	"sthira/backend/internal/store"
)

// newID returns an unguessable ID with the given prefix.
func newID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return prefix + "-unknown"
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// --- sessions ---

type createSessionRequest struct {
	// No civil identity is required (R22). Optional client label only.
	Label string `json:"label,omitempty"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	if s.store == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "store unavailable", "", true)
		return
	}
	// Body is optional; decode if present.
	if r.ContentLength > 0 {
		body, err := s.readBoundedBody(w, r)
		if err != nil {
			s.jsonDecodeError(w, r, err)
			return
		}
		var req createSessionRequest
		if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
			s.jsonDecodeError(w, r, err)
			return
		}
	}
	sessionID := newID("SES")
	token, err := store.IssueCitizenSession(r.Context(), s.store.DB(), sessionID, 24*time.Hour, time.Now().UTC())
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "failed to issue session", "", true)
		return
	}
	s.writeData(w, r, http.StatusCreated, "none", contracts.FreshnessUnknown, map[string]any{
		"session_id": sessionID,
		"token":      token, // returned once; only its hash is stored
		"expires_in": int((24 * time.Hour) / time.Second),
	})
}

// --- places/resolve ---

type resolvePlaceRequest struct {
	Jurisdiction string `json:"jurisdiction"`
	Query        string `json:"query"`
}

func (s *Server) handleResolvePlace(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.requireJSONContentType(w, r) {
		return
	}
	if s.store == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "store unavailable", "", true)
		return
	}
	body, err := s.readBoundedBody(w, r)
	if err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	var req resolvePlaceRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	cand, err := store.ResolvePlace(r.Context(), s.store.DB(), req.Jurisdiction, req.Query)
	if err != nil {
		var amb *store.AmbiguousPlaceError
		if errors.As(err, &amb) {
			s.writeError(w, r, http.StatusConflict, contracts.ErrAmbiguousPlace, "multiple places match; choose a candidate", "query", true)
			return
		}
		if errors.Is(err, store.ErrPlaceNotFound) {
			s.writeError(w, r, http.StatusNotFound, contracts.ErrNotFound, "no place matches", "query", false)
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "place resolution failed", "", true)
		return
	}
	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, map[string]any{
		"place_id":   cand.PlaceID,
		"place_kind": cand.PlaceKind,
	})
}

// --- guidance/query (read-only eligible destinations) ---

type guidanceQueryRequest struct {
	Jurisdiction string `json:"jurisdiction"`
	PackageID    string `json:"package_id"`
	PartySize    int    `json:"party_size"`
	StartDate    string `json:"start_date"` // RFC 3339 date
	EndDate      string `json:"end_date"`
}

func (s *Server) handleGuidanceQuery(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.requireJSONContentType(w, r) {
		return
	}
	if s.store == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, contracts.ErrDataUnavailable, "store unavailable", "", true)
		return
	}
	body, err := s.readBoundedBody(w, r)
	if err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	var req guidanceQueryRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	start, err1 := time.Parse("2006-01-02", req.StartDate)
	end, err2 := time.Parse("2006-01-02", req.EndDate)
	if err1 != nil || err2 != nil || !end.After(start) || req.PartySize < 1 {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue, "invalid party size or date range", "start_date", false)
		return
	}
	// Route authority (O05) is open: the operational route gate is closed. Only
	// an explicit synthetic-exercise flag opens it in isolation.
	gateOpen := r.Header.Get("X-Sthira-Synthetic-Route-Gate") == "open"
	q := store.ChoiceQuery{
		Jurisdiction:  req.Jurisdiction,
		PackageID:     req.PackageID,
		PartySize:     req.PartySize,
		StartDate:     start,
		EndDate:       end,
		RouteGateOpen: gateOpen,
	}
	dests, err := store.ChoiceQuerier{}.Eligible(r.Context(), s.store.DB(), q, time.Now().UTC())
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "eligibility query failed", "", true)
		return
	}
	items := make([]map[string]any, 0, len(dests))
	for _, d := range dests {
		item := map[string]any{
			"facility_id":    d.FacilityID,
			"safe_zone_id":   d.SafeZoneID,
			"capacity_known": d.CapacityKnown,
			"route_verified": d.RouteVerified,
		}
		if d.CapacityKnown {
			item["free"] = d.Free
		} else {
			item["free"] = nil // unknown, never promised
		}
		if d.RouteID != nil {
			item["route_id"] = *d.RouteID
		}
		items = append(items, item)
	}
	s.writeData(w, r, http.StatusOK, req.PackageID, contracts.FreshnessUnknown, map[string]any{
		"destinations": items,
		"route_gate":   gateOpen,
	})
}

// --- reservations (create + read) ---

type createReservationRequest struct {
	FacilityID  string `json:"facility_id"`
	PackageID   string `json:"package_id"`
	RouteID     string `json:"route_id,omitempty"`
	PartySize   int    `json:"party_size"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	IdemKey     string `json:"idempotency_key"`
	// SnapshotVersion is the source/package snapshot the client validated its
	// choice against; revalidated at commit (stale-selection detection).
	SnapshotVersion int `json:"snapshot_version"`
}

func reservationPayloadHash(sessID string, req createReservationRequest) string {
	h := sha256.Sum256([]byte(sessID + "|" + req.FacilityID + "|" + req.PackageID + "|" + req.RouteID + "|" + req.StartDate + "|" + req.EndDate + "|" + req.IdemKey))
	return hex.EncodeToString(h[:])
}

func (s *Server) handleCreateReservation(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.requireJSONContentType(w, r) {
		return
	}
	sess, ok := sessionFrom(r)
	if !ok {
		s.writeError(w, r, http.StatusUnauthorized, contracts.ErrForbidden, "session required", "", false)
		return
	}
	body, err := s.readBoundedBody(w, r)
	if err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	var req createReservationRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	start, err1 := time.Parse("2006-01-02", req.StartDate)
	end, err2 := time.Parse("2006-01-02", req.EndDate)
	if err1 != nil || err2 != nil || !end.After(start) || req.PartySize < 1 || req.IdemKey == "" {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue, "invalid reservation request", "", false)
		return
	}

	ctx := r.Context()
	is := store.IdempotencyStore{}
	stays := store.NewStayStore(store.ChainAuditor{})
	payloadHash := reservationPayloadHash(sess.SessionID, req)
	now := time.Now().UTC()
	stayID := newID("STAY")
	resID := newID("RES")

	var result []byte
	var replay bool
	err = s.store.InTx(ctx, func(tx store.DBTX) error {
		// Idempotency: replay a completed key, conflict on a changed payload.
		res, rep, err := is.Begin(ctx, tx, sess.SessionID, "reservation.create", req.IdemKey, payloadHash, now.Add(time.Hour))
		if err != nil {
			return err
		}
		if rep {
			result, replay = res, true
			return nil
		}
		// Revalidate the source snapshot version at commit (stale selection).
		var curVersion int
		if err := tx.QueryRowContext(ctx, `SELECT version FROM packages WHERE package_id = $1`, req.PackageID).Scan(&curVersion); err != nil {
			return err
		}
		if curVersion != req.SnapshotVersion {
			return &snapshotStaleError{want: req.SnapshotVersion, got: curVersion}
		}
		// Persist the reservation record then the stay (held capacity).
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,'RESERVED',1,$6,$6)`,
			resID, sess.SessionID, req.FacilityID, start, req.PartySize, now); err != nil {
			return err
		}
		var routeID *string
		if req.RouteID != "" {
			routeID = &req.RouteID
		}
		if err := stays.Reserve(ctx, tx, store.Stay{
			StayID: stayID, ReservationID: resID, SessionID: sess.SessionID,
			FacilityID: req.FacilityID, PartySize: req.PartySize,
			StartDate: start, EndDate: end, PackageID: req.PackageID, RouteID: routeID,
		}, now); err != nil {
			return err
		}
		return is.Complete(ctx, tx, sess.SessionID, "reservation.create", req.IdemKey, map[string]string{
			"reservation_id": resID, "stay_id": stayID,
		})
	})

	if replay {
		var out map[string]string
		_ = json.Unmarshal(result, &out)
		s.writeData(w, r, http.StatusOK, req.PackageID, contracts.FreshnessUnknown, out)
		return
	}
	if err != nil {
		s.writeReservationError(w, r, err)
		return
	}
	s.writeData(w, r, http.StatusCreated, req.PackageID, contracts.FreshnessUnknown, map[string]string{
		"reservation_id": resID, "stay_id": stayID,
	})
}

type snapshotStaleError struct{ want, got int }

func (e *snapshotStaleError) Error() string { return "snapshot version stale" }

// writeReservationError maps store errors to stable contract codes.
func (s *Server) writeReservationError(w http.ResponseWriter, r *http.Request, err error) {
	var stale *snapshotStaleError
	switch {
	case errors.As(err, &stale):
		s.writeError(w, r, http.StatusConflict, contracts.ErrStaleVersion, "source snapshot changed; re-read choices before confirming", "snapshot_version", true)
	case errors.Is(err, store.ErrPayloadConflict):
		s.writeError(w, r, http.StatusConflict, contracts.ErrIdempotencyConflict, "idempotency key reused with a different payload", "idempotency_key", false)
	case errors.Is(err, store.ErrInProgress):
		s.writeError(w, r, http.StatusConflict, contracts.ErrIdempotencyConflict, "request with this key is in progress", "idempotency_key", true)
	case errors.Is(err, store.ErrCapacityExhausted):
		// Recoverable conflict; never silently substitute another destination.
		s.writeError(w, r, http.StatusConflict, contracts.ErrCapacityConflict, "capacity exhausted for the requested dates", "facility_id", true)
	case errors.Is(err, store.ErrInvalidTransition):
		s.writeError(w, r, http.StatusConflict, contracts.ErrValidation, "invalid stay transition", "", false)
	case errors.Is(err, store.ErrStayNotFound):
		s.writeError(w, r, http.StatusNotFound, contracts.ErrNotFound, "stay not found", "", false)
	default:
		s.writeError(w, r, http.StatusInternalServerError, contracts.ErrInternal, "reservation failed", "", true)
	}
}

// handleGetReservation is the authenticated owner read path for restart
// recovery. Only the owning session may read it (private, no-store).
func (s *Server) handleGetReservation(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodGet) {
		return
	}
	sess, ok := sessionFrom(r)
	if !ok {
		s.writeError(w, r, http.StatusUnauthorized, contracts.ErrForbidden, "session required", "", false)
		return
	}
	stayID := r.PathValue("id")
	var st struct {
		StayID, FacilityID, State, PackageID string
		PartySize                            int
		StartDate, EndDate                   time.Time
		SessionID                            string
	}
	err := s.store.DB().QueryRowContext(r.Context(), `
		SELECT stay_id, facility_id, state, package_id, party_size, start_date, end_date, session_id
		FROM stays WHERE stay_id = $1 OR reservation_id = $1`, stayID).
		Scan(&st.StayID, &st.FacilityID, &st.State, &st.PackageID, &st.PartySize, &st.StartDate, &st.EndDate, &st.SessionID)
	if err != nil {
		s.writeError(w, r, http.StatusNotFound, contracts.ErrNotFound, "reservation not found", "", false)
		return
	}
	if st.SessionID != sess.SessionID {
		// Owner-only; knowing the ID is not authorization (R22).
		s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "not the owning session", "", false)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.writeData(w, r, http.StatusOK, st.PackageID, contracts.FreshnessUnknown, map[string]any{
		"stay_id": st.StayID, "facility_id": st.FacilityID, "state": st.State,
		"package_id": st.PackageID, "party_size": st.PartySize,
		"start_date": st.StartDate.Format("2006-01-02"), "end_date": st.EndDate.Format("2006-01-02"),
	})
}

// --- reservation events (arrive/cancel/depart/extend/transfer) ---

type stayEventRequest struct {
	Type       string `json:"type"` // ARRIVE|CANCEL|DEPART|EXTEND|TRANSFER
	IdemKey    string `json:"idempotency_key"`
	NewEndDate string `json:"new_end_date,omitempty"`  // EXTEND
	NewFacilityID string `json:"new_facility_id,omitempty"` // TRANSFER
}

func (s *Server) handleStayEvent(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.requireJSONContentType(w, r) {
		return
	}
	sess, ok := sessionFrom(r)
	if !ok {
		s.writeError(w, r, http.StatusUnauthorized, contracts.ErrForbidden, "session required", "", false)
		return
	}
	stayID := r.PathValue("id")
	body, err := s.readBoundedBody(w, r)
	if err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	var req stayEventRequest
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return
	}
	if req.IdemKey == "" {
		s.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue, "idempotency_key is required", "idempotency_key", false)
		return
	}

	ctx := r.Context()
	is := store.IdempotencyStore{}
	stays := store.NewStayStore(store.ChainAuditor{})
	now := time.Now().UTC()
	op := "stay." + strings.ToLower(req.Type)
	payloadHash := sha256Hex(sess.SessionID + "|" + stayID + "|" + req.Type + "|" + req.NewEndDate + "|" + req.NewFacilityID + "|" + req.IdemKey)

	var result []byte
	var replay bool
	err = s.store.InTx(ctx, func(tx store.DBTX) error {
		res, rep, err := is.Begin(ctx, tx, sess.SessionID, op, req.IdemKey, payloadHash, now.Add(time.Hour))
		if err != nil {
			return err
		}
		if rep {
			result, replay = res, true
			return nil
		}
		// Authorize: the session must own the stay.
		var owner string
		if err := tx.QueryRowContext(ctx, `SELECT session_id FROM stays WHERE stay_id = $1`, stayID).Scan(&owner); err != nil {
			return store.ErrStayNotFound
		}
		if owner != sess.SessionID {
			return errNotOwner
		}
		var opErr error
		switch strings.ToUpper(req.Type) {
		case "ARRIVE":
			opErr = stays.Arrive(ctx, tx, stayID, now)
		case "CANCEL":
			opErr = stays.Cancel(ctx, tx, stayID, now)
		case "DEPART":
			opErr = stays.Depart(ctx, tx, stayID, now)
		case "EXTEND":
			nd, perr := time.Parse("2006-01-02", req.NewEndDate)
			if perr != nil {
				return errInvalidEvent
			}
			opErr = stays.Extend(ctx, tx, stayID, nd, now)
		case "TRANSFER":
			if req.NewFacilityID == "" {
				return errInvalidEvent
			}
			opErr = stays.Transfer(ctx, tx, stayID, newID("STAY"), newID("RES"), req.NewFacilityID, now)
		default:
			return errInvalidEvent
		}
		if opErr != nil {
			return opErr
		}
		return is.Complete(ctx, tx, sess.SessionID, op, req.IdemKey, map[string]string{"stay_id": stayID, "type": req.Type})
	})

	if replay {
		var out map[string]string
		_ = json.Unmarshal(result, &out)
		s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, out)
		return
	}
	if err != nil {
		if errors.Is(err, errNotOwner) {
			s.writeError(w, r, http.StatusForbidden, contracts.ErrForbidden, "not the owning session", "", false)
			return
		}
		if errors.Is(err, errInvalidEvent) {
			s.writeError(w, r, http.StatusBadRequest, contracts.ErrInvalidValue, "invalid stay event", "type", false)
			return
		}
		s.writeReservationError(w, r, err)
		return
	}
	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, map[string]string{
		"stay_id": stayID, "type": req.Type,
	})
}

var errNotOwner = errors.New("not the owning session")
var errInvalidEvent = errors.New("invalid stay event")

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
