package httpserver

// P4 transfer recovery verification: the replacement stay ID returned by a
// successful transfer must be readable by the same session, must be
// arrvable, and a retry of the same idempotency key after a "lost response"
// must return the SAME replacement IDs (no second new stay). Opposing
// concurrent transfers (A->B and B->A on the same package) preserve
// capacity without partial writes.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"sthira/backend/internal/store"
)

// transferBody builds a TRANSFER event payload.
func transferBody(newFacID, newRouteID, idemKey string, snap int) string {
	routeField := ""
	if newRouteID != "" {
		routeField = fmt.Sprintf(`,"new_route_id":%q`, newRouteID)
	}
	return fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q%s,"idempotency_key":%q,"snapshot_version":%d}`,
		newFacID, routeField, idemKey, snap)
}

func decodeTransferResult(t *testing.T, body string) (oldID, newStayID, newResID string) {
	t.Helper()
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("decode transfer envelope: %v; body=%s", err, body)
	}
	if env.Data == nil {
		t.Fatalf("transfer: no data; body=%s", body)
	}
	oldID, _ = env.Data["stay_id"].(string)
	newStayID, _ = env.Data["new_stay_id"].(string)
	newResID, _ = env.Data["new_reservation_id"].(string)
	return
}

// seedTwoFacilities seeds a primary facility and a second facility on a
// different safe zone in the same package.
func seedTwoFacilities(t *testing.T, st *store.Store, pkgID, facA string, capacity int, start, end time.Time) (facB string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	facB = "FAC-B-" + fmt.Sprintf("%d", time.Now().UnixNano())
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, location, version, updated_at)
			VALUES ('SZ-B', $1, 'SAFE', NULL, 'OPEN', NULL, NULL, 1, $2)`,
			pkgID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1, $2, 'SZ-B', 'Asia/Kolkata', 1, $3)`, facB, pkgID, now); err != nil {
			return err
		}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(t.Context(), `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1, $2, $3, 0, 0, 0, 1, $4)`, facB, d, capacity, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seedTwoFacilities: %v", err)
	}
	return facB
}

// TestTransferReturnsReplacementIDsAndRead: a successful transfer returns
// the replacement stay ID; the same session can GET that replacement stay
// via /api/v3/reservations/{id} and arrive at it explicitly.
func TestTransferReturnsReplacementIDsAndRead(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facA, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	facB := seedTwoFacilities(t, st, pkgID, facA, 3, httpDayT(1), httpDayT(4))

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facA, pkgID, 1, httpDay(1), httpDay(2), "tr-read-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)

	tr := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", token,
		transferBody(facB, "", "transfer-replace-1", snap))
	if tr.code != http.StatusOK {
		t.Fatalf("transfer: code=%d body=%s", tr.code, tr.body)
	}
	oldID, newStayID, newResID := decodeTransferResult(t, tr.body)
	if oldID != created.StayID {
		t.Fatalf("replacement envelope: old_stay_id=%q, want %q", oldID, created.StayID)
	}
	if newStayID == "" || newStayID == created.StayID {
		t.Fatalf("replacement envelope: new_stay_id=%q (empty or same as old)", newStayID)
	}
	if newResID == "" {
		t.Fatalf("replacement envelope: new_reservation_id empty")
	}

	// The replacement stay must be readable by the same session.
	get := doAuthed(t, srv, http.MethodGet, "/api/v3/reservations/"+newStayID, token, "")
	if get.code != http.StatusOK {
		t.Fatalf("GET replacement: code=%d body=%s", get.code, get.body)
	}
	var got struct {
		FacilityID string `json:"facility_id"`
		State      string `json:"state"`
		StayID     string `json:"stay_id"`
	}
	genv := decodeEnvelope(t, get)
	bb, _ := json.Marshal(genv.Data)
	_ = json.Unmarshal(bb, &got)
	if got.StayID != newStayID {
		t.Fatalf("readback stay_id=%q, want %q", got.StayID, newStayID)
	}
	if got.FacilityID != facB {
		t.Fatalf("readback facility_id=%q, want %q (new facility)", got.FacilityID, facB)
	}
	if got.State != "RESERVED" {
		t.Fatalf("readback state=%q, want RESERVED", got.State)
	}

	// Arrive at the replacement stay explicitly using its ID.
	arr := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+newStayID+"/events", token,
		`{"type":"ARRIVE","idempotency_key":"arrive-replace-1"}`)
	if arr.code != http.StatusOK {
		t.Fatalf("ARRIVE replacement: code=%d body=%s", arr.code, arr.body)
	}
}

// TestTransferRetryReturnsSameReplacementIDs: replaying the same idempotency
// key (simulating a lost-response retry) must return the SAME replacement
// stay/reservation IDs as the first successful transfer, not create a second
// replacement.
func TestTransferRetryReturnsSameReplacementIDs(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facA, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	facB := seedTwoFacilities(t, st, pkgID, facA, 3, httpDayT(1), httpDayT(4))

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facA, pkgID, 1, httpDay(1), httpDay(2), "tr-retry-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)

	idem := "transfer-retry-1"
	tr := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", token,
		transferBody(facB, "", idem, snap))
	if tr.code != http.StatusOK {
		t.Fatalf("transfer 1: code=%d body=%s", tr.code, tr.body)
	}
	_, newStayID1, newResID1 := decodeTransferResult(t, tr.body)

	// Replay the SAME idempotency key (e.g. client retried after a lost response).
	tr2 := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", token,
		transferBody(facB, "", idem, snap))
	if tr2.code != http.StatusOK {
		t.Fatalf("transfer 2: code=%d body=%s", tr2.code, tr2.body)
	}
	_, newStayID2, newResID2 := decodeTransferResult(t, tr2.body)
	if newStayID1 != newStayID2 {
		t.Fatalf("retry produced different new_stay_id: %q vs %q", newStayID1, newStayID2)
	}
	if newResID1 != newResID2 {
		t.Fatalf("retry produced different new_reservation_id: %q vs %q", newResID1, newResID2)
	}

	// And only ONE replacement stay row exists.
	var n int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM stays WHERE stay_id = $1`, newStayID1).Scan(&n); err != nil {
		t.Fatalf("count replacement: %v", err)
	}
	if n != 1 {
		t.Fatalf("replacement stay rows=%d, want 1", n)
	}
}

// TestOpposingTransfersPreserveCapacity: two sessions each try to transfer
// to the other's facility; deterministic lock ordering across both facilities
// prevents deadlock and partial writes; capacity is conserved (each facility
// holds its own hold across the interval).
func TestOpposingTransfersPreserveCapacity(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facA, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	facB := seedTwoFacilities(t, st, pkgID, facA, 3, httpDayT(1), httpDayT(4))

	// Two sessions, two reservations on opposite facilities.
	_, tokenA := createSession(t, srv)
	recA := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", tokenA,
		reservationBody(facA, pkgID, 1, httpDay(1), httpDay(2), "opp-A-key", snap))
	if recA.code != http.StatusCreated {
		t.Fatalf("reserve A: code=%d body=%s", recA.code, recA.body)
	}
	stayA := decodeFirstStay(t, recA)

	_, tokenB := createSession(t, srv)
	recB := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", tokenB,
		reservationBody(facB, pkgID, 1, httpDay(1), httpDay(2), "opp-B-key", snap))
	if recB.code != http.StatusCreated {
		t.Fatalf("reserve B: code=%d body=%s", recB.code, recB.body)
	}
	stayB := decodeFirstStay(t, recB)

	// Each transfers to the other's facility.
	var wg sync.WaitGroup
	wg.Add(2)
	recAtr := new(0)
	recBtr := new(0)
	go func() {
		defer wg.Done()
		r := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayA+"/events", tokenA,
			transferBody(facB, "", "opp-A-transfer", snap))
		*recAtr = r.code
	}()
	go func() {
		defer wg.Done()
		r := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayB+"/events", tokenB,
			transferBody(facA, "", "opp-B-transfer", snap))
		*recBtr = r.code
	}()
	wg.Wait()

	// Both should succeed: each transfer frees its origin's held capacity
	// and holds the destination's. Total capacity conserved on every date.
	if *recAtr != http.StatusOK {
		t.Fatalf("transfer A: code=%d", *recAtr)
	}
	if *recBtr != http.StatusOK {
		t.Fatalf("transfer B: code=%d", *recBtr)
	}
	for d := httpDayT(1); d.Before(httpDayT(2)); d = d.AddDate(0, 0, 1) {
		hA, _, cA := heldOccupiedCapacity(t, st, facA, d)
		hB, _, cB := heldOccupiedCapacity(t, st, facB, d)
		if hA+hB > cA+cB {
			t.Fatalf("on %s: held A+B=%d > total capacity A+B=%d", d.Format("2006-01-02"), hA+hB, cA+cB)
		}
	}
}

func heldOccupiedCapacity(t *testing.T, st *store.Store, facID string, d time.Time) (held, occupied, capacity int) {
	t.Helper()
	err := st.DB().QueryRowContext(t.Context(),
		`SELECT held, occupied, capacity FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
		facID, d).Scan(&held, &occupied, &capacity)
	if err != nil {
		t.Fatalf("heldOccupiedCapacity: %v", err)
	}
	return
}

// decodeFirstStay extracts the first stay_id from a 201 reserve response.
func decodeFirstStay(t *testing.T, rec response) string {
	t.Helper()
	env := decodeEnvelope(t, rec)
	var d struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &d)
	if d.StayID == "" {
		t.Fatalf("no stay_id in 201; body=%s", rec.body)
	}
	return d.StayID
}

// new is a tiny pointer constructor for ints.
func new(v int) *int { return &v }

// TestFailedTransferPreservesOriginal: a transfer that fails (target full)
// keeps the original stay intact. Documents the existing contract; ensures
// the new code paths (replacement IDs, lock ordering) didn't regress it.
func TestFailedTransferPreservesOriginal(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facA, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	// Full target facility in same package (capacity 0).
	now := time.Now().UTC().Truncate(time.Microsecond)
	fullFac := "FAC-FULL-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, location, version, updated_at)
			VALUES ('SZ-F', $1, 'SAFE', NULL, 'OPEN', NULL, NULL, 1, $2)`, pkgID, now); err != nil {
			return err
		}
		_, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1, $2, 'SZ-F', 'Asia/Kolkata', 1, $3)`, fullFac, pkgID, now)
		return err
	}); err != nil {
		t.Fatalf("seed full target: %v", err)
	}
	for d := httpDayT(1); d.Before(httpDayT(2)); d = d.AddDate(0, 0, 1) {
		if _, err := st.DB().ExecContext(t.Context(), `
			INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
			VALUES ($1, $2, 0, 0, 0, 0, 1, $3)`, fullFac, d, now); err != nil {
			t.Fatalf("seed full inv: %v", err)
		}
	}
	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facA, pkgID, 1, httpDay(1), httpDay(2), "fail-tr-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	stayID := decodeFirstStay(t, rec)
	tr := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token,
		transferBody(fullFac, "", "fail-tr-event", snap))
	if tr.code != http.StatusConflict {
		t.Fatalf("transfer to full: code=%d, want 409; body=%s", tr.code, tr.body)
	}
	var state string
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT state FROM stays WHERE stay_id=$1`, stayID).Scan(&state); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != "RESERVED" {
		t.Fatalf("origin state=%s, want RESERVED (transfer failed atomically)", state)
	}
	if got := heldCount(t, st, facA, httpDayT(1)); got != 1 {
		t.Fatalf("origin held=%d, want 1 (capacity retained)", got)
	}
}
