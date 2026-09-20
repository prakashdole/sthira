//go:build crashtest

package httpserver

// Test-only crash fault-injection. This file is compiled ONLY under the
// `crashtest` build tag; production builds never include it. It installs an
// HTTP endpoint that performs a real committed write and then blocks the
// response, so a parent process can confirm the commit landed in the database
// and then SIGKILL the server mid-response — demonstrating
// crash-after-commit/before-response recovery.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpjson"
	"sthira/backend/internal/store"
)

// crashCommitRequest is the body for the test-only crash endpoint.
type crashCommitRequest struct {
	ReservationID string    `json:"reservation_id"`
	SessionID     string    `json:"session_id"`
	FacilityID    string    `json:"facility_id"`
	ServiceDate   time.Time `json:"service_date"`
	PartySize     int       `json:"party_size"`
	IdemKey       string    `json:"idem_key"`
}

func crashPayloadHash(req crashCommitRequest) string {
	h := sha256.Sum256([]byte(req.ReservationID + "|" + req.SessionID + "|" + req.FacilityID + "|" + req.IdemKey))
	return hex.EncodeToString(h[:])
}

// registerCrashHook installs the test-only endpoints. Both run a real
// reservation write (idempotency Begin+Complete, reservation insert, inventory
// conditional update, audit) inside one InTx:
//   - POST /crashtest/commit-and-hang commits then blocks the response, so the
//     caller can kill the process before any bytes return (crash boundary).
//   - POST /crashtest/commit commits then responds, for cross-process
//     concurrency tests that need the write to complete.
func (s *Server) registerCrashHook(mux *http.ServeMux) {
	mux.HandleFunc("/crashtest/commit-and-hang", s.withRequestID(s.handleCrashCommitAndHang))
	mux.HandleFunc("/crashtest/commit", s.withRequestID(s.handleCrashCommit))
}

// doCrashCommit performs the committed reservation write in one transaction.
func (s *Server) doCrashCommit(ctx context.Context, req crashCommitRequest) error {
	rs := store.NewReservationStore(store.ChainAuditor{})
	is := store.IdempotencyStore{}
	payloadHash := crashPayloadHash(req)
	return s.store.InTx(ctx, func(tx store.DBTX) error {
		if _, _, err := is.Begin(ctx, tx, req.SessionID, "reservation.create", req.IdemKey, payloadHash, time.Now().UTC().Add(time.Hour)); err != nil {
			return err
		}
		if err := rs.EnsureInventory(ctx, tx, req.FacilityID, req.ServiceDate, 1, time.Now().UTC()); err != nil {
			return err
		}
		var ver int
		if err := tx.QueryRowContext(ctx,
			`SELECT version FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
			req.FacilityID, req.ServiceDate).Scan(&ver); err != nil {
			return err
		}
		if err := rs.Reserve(ctx, tx, store.Reservation{
			ReservationID: req.ReservationID,
			SessionID:     req.SessionID,
			FacilityID:    req.FacilityID,
			ServiceDate:   req.ServiceDate,
			PartySize:     req.PartySize,
			CreatedAt:     time.Now().UTC(),
		}, ver); err != nil {
			return err
		}
		return is.Complete(ctx, tx, req.SessionID, "reservation.create", req.IdemKey, map[string]string{"reservation_id": req.ReservationID})
	})
}

// decodeCrashCommit reads and validates the crash-commit request body.
func (s *Server) decodeCrashCommit(w http.ResponseWriter, r *http.Request) (crashCommitRequest, bool) {
	var req crashCommitRequest
	if !s.requireMethod(w, r, http.MethodPost) {
		return req, false
	}
	if s.store == nil {
		s.writeError(w, r, http.StatusServiceUnavailable, "DATA_UNAVAILABLE", "no store", "", false)
		return req, false
	}
	body, err := s.readBoundedBody(w, r)
	if err != nil {
		s.jsonDecodeError(w, r, err)
		return req, false
	}
	if err := httpjson.DecodeStrict(body, &req, httpjson.Limits{MaxBytes: s.cfg.MaxBodyBytes, MaxDepth: s.cfg.MaxJSONDepth}); err != nil {
		s.jsonDecodeError(w, r, err)
		return req, false
	}
	return req, true
}

func (s *Server) handleCrashCommitAndHang(w http.ResponseWriter, r *http.Request) {
	req, ok := s.decodeCrashCommit(w, r)
	if !ok {
		return
	}
	if err := s.doCrashCommit(r.Context(), req); err != nil {
		s.writeError(w, r, http.StatusConflict, "CAPACITY_CONFLICT", err.Error(), "", true)
		return
	}
	// The transaction committed. Block the response: the parent confirms the
	// commit in the database and SIGKILLs this process before any response bytes
	// are written. Block on the request context so a clean shutdown cancels it.
	<-r.Context().Done()
}

// handleCrashCommit commits and responds, for cross-process concurrency tests.
func (s *Server) handleCrashCommit(w http.ResponseWriter, r *http.Request) {
	req, ok := s.decodeCrashCommit(w, r)
	if !ok {
		return
	}
	if err := s.doCrashCommit(r.Context(), req); err != nil {
		s.writeError(w, r, http.StatusConflict, "CAPACITY_CONFLICT", err.Error(), "", true)
		return
	}
	s.writeData(w, r, http.StatusOK, "none", contracts.FreshnessUnknown, map[string]string{"reservation_id": req.ReservationID})
}
