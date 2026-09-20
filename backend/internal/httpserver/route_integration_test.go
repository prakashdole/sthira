package httpserver

// P4 synthetic-route and route-bypass verification: confirms the public
// request-header cannot enable the synthetic route gate, route validation is
// consistent at commit and at transfer, closed routes are rejected, and the
// isolated synthetic route gate (RouteGateOpen=true) is reachable only via
// the store API, never via HTTP. Gated on STHIRA_TEST_DSN; skipped when unset.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/store"
)

// httpDB opens a real store or skips.
func httpDB(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" {
		t.Skip("STHIRA_TEST_DSN unset; skipping real-DB route verification")
	}
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// seedRoute inserts a route_versions row with a valid LineString, verified_by
// set, and the given approval. Routes must include a valid geography to be
// inserted; we use a minimal 2-point LineString in EPSG:4326.
func seedRoute(t *testing.T, st *store.Store, pkgID, routeID, fromZone, toZone, approval string) {
	t.Helper()
	now := time.Now().UTC()
	err := st.InTx(context.Background(), func(tx store.DBTX) error {
		_, err := tx.ExecContext(context.Background(), `
			INSERT INTO route_versions
				(route_id, package_id, from_zone_id, to_safe_zone_id, approval, mode,
				 verified_by, verified_at, valid_from, valid_until, geometry, version, updated_at)
			VALUES ($1,$2,$3,$4,$5,'FOOT','test-verifier',$6,$6,$7,
				ST_GeogFromText('SRID=4326;LINESTRING(76.0 11.0, 76.1 11.1)'),1,$6)`,
			routeID, pkgID, fromZone, toZone, approval, now, now.Add(24*time.Hour))
		return err
	})
	if err != nil {
		t.Fatalf("seedRoute: %v", err)
	}
}

// closeRoute inserts a closure row that hasn't been reopened.
func closeRoute(t *testing.T, st *store.Store, routeID string) {
	t.Helper()
	now := time.Now().UTC()
	err := st.InTx(context.Background(), func(tx store.DBTX) error {
		_, err := tx.ExecContext(context.Background(), `
			INSERT INTO route_closures (closure_id, route_id, closed_at, reason, reopened_at)
			VALUES ($1, $2, $3, 'test', NULL)`,
			fmt.Sprintf("CL-%s-%d", routeID, time.Now().UnixNano()), routeID, now)
		return err
	})
	if err != nil {
		t.Fatalf("closeRoute: %v", err)
	}
}

// TestHeaderCannotEnableSyntheticRoute: a public POST to /api/v3/guidance/query
// with the X-Sthira-Synthetic-Route-Gate header set returns the same result
// as without the header — no destination is reported as route-verified, even
// when SYNTHETIC_DEMO routes exist in the database bound to the safe zone.
// The handler hardcodes RouteGateOpen=false; the header is not read.
func TestHeaderCannotEnableSyntheticRoute(t *testing.T) {
	st := httpDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	pkgID, facID, _ := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_ = facID

	// Seed a SYNTHETIC_DEMO route to the facility's safe zone.
	seedRoute(t, st, pkgID, "RT-SYN", "RZA", "SZ", "SYNTHETIC_DEMO")

	body := `{"jurisdiction":"JTEST","package_id":"` + pkgID + `","party_size":1,"start_date":"` + httpDay(1) + `","end_date":"` + httpDay(2) + `"}`

	// Without the header.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/guidance/query",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("post no header: %v", err)
	}
	defer resp.Body.Close()
	var noHeader struct {
		Data struct {
			Destinations []map[string]any `json:"destinations"`
			RouteGate    bool             `json:"route_gate"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&noHeader)

	// With the header set.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/guidance/query",
		strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Sthira-Synthetic-Route-Gate", "open")
	resp2, err := srv.Client().Do(req2)
	if err != nil {
		t.Fatalf("post with header: %v", err)
	}
	defer resp2.Body.Close()
	var withHeader struct {
		Data struct {
			Destinations []map[string]any `json:"destinations"`
			RouteGate    bool             `json:"route_gate"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&withHeader)

	if noHeader.Data.RouteGate || withHeader.Data.RouteGate {
		t.Fatalf("route_gate=%v/%v, want false/false in both cases", noHeader.Data.RouteGate, withHeader.Data.RouteGate)
	}
	for _, d := range noHeader.Data.Destinations {
		if v, _ := d["route_verified"].(bool); v {
			t.Fatalf("no-header: route_verified=true, want false; dest=%v", d)
		}
	}
	for _, d := range withHeader.Data.Destinations {
		if v, _ := d["route_verified"].(bool); v {
			t.Fatalf("with-header: route_verified=true, want false; dest=%v", d)
		}
	}
}

// TestRequestBodyCannotEnableSyntheticRoute: a caller cannot enable the
// synthetic route gate by smuggling a field into the request body. Only the
// ChoiceQuery struct's RouteGateOpen matters, and the handler hardcodes
// false regardless of payload. The route_gate response field must report
// false; the destinations must not be route_verified.
func TestRequestBodyCannotEnableSyntheticRoute(t *testing.T) {
	st := httpDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	pkgID, _, _ := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	seedRoute(t, st, pkgID, "RT-SYN2", "RZA", "SZ", "SYNTHETIC_DEMO")

	// Try to set route_gate_open / route_gate / synthetic_route_gate in the body.
	body := `{"jurisdiction":"JTEST","package_id":"` + pkgID + `","party_size":1,"start_date":"` + httpDay(1) + `","end_date":"` + httpDay(2) + `","route_gate_open":true,"route_gate":true,"synthetic_route_gate":true}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/guidance/query",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sthira-Synthetic-Route-Gate", "open")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	var env struct {
		Data struct {
			Destinations []map[string]any `json:"destinations"`
			RouteGate    bool             `json:"route_gate"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if env.Data.RouteGate {
		t.Fatalf("route_gate=true, want false (handler is server-controlled)")
	}
	for _, d := range env.Data.Destinations {
		if v, _ := d["route_verified"].(bool); v {
			t.Fatalf("smuggled payload: route_verified=true, want false; dest=%v", d)
		}
	}
}

// TestValidIsolatedSyntheticFlowSucceeds: when the gate is opened at the
// store level (RouteGateOpen=true, the isolated test seam), a SYNTHETIC_DEMO
// route is returned and the destination is reported as RouteVerified=true.
// This confirms the gate is reachable only via the direct store API, never
// via the HTTP request header.
func TestValidIsolatedSyntheticFlowSucceeds(t *testing.T) {
	st := httpDB(t)
	pkgID, _, _ := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	seedRoute(t, st, pkgID, "RT-ISO", "RZA", "SZ", "SYNTHETIC_DEMO")

	q := store.ChoiceQuery{
		Jurisdiction:  "JTEST",
		PackageID:     pkgID,
		PartySize:     1,
		StartDate:     httpDayT(1),
		EndDate:       httpDayT(2),
		RouteGateOpen: true, // isolated test seam
	}
	dests, err := (store.ChoiceQuerier{}).Eligible(context.Background(), st.DB(), q, time.Now().UTC())
	if err != nil {
		t.Fatalf("Eligible: %v", err)
	}
	if len(dests) == 0 {
		t.Fatalf("no destinations under isolated gate")
	}
	var found bool
	for _, d := range dests {
		if d.RouteVerified && d.RouteID != nil && *d.RouteID == "RT-ISO" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no RouteVerified RT-ISO destination under isolated gate: %+v", dests)
	}
}

// TestReservationRouteToWrongDestinationRejected: a reservation with a route
// bound to a DIFFERENT safe zone than the facility's is rejected at commit.
// RevalidateReservationContext accepts a route only when its to_safe_zone_id
// matches the facility's safe_zone_id (the route must be to that destination).
func TestReservationRouteToWrongDestinationRejected(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	// Route to a DIFFERENT safe zone (WZS != SZ).
	seedRoute(t, st, pkgID, "RT-WRONG", "RZA", "WZS", "SYNTHETIC_DEMO")
	_, token := createSession(t, srv)

	body := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d,"route_id":"RT-WRONG"}`,
		facID, pkgID, httpDay(1), httpDay(2), "wrong-route-key", snap)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if rec.code != http.StatusConflict {
		t.Fatalf("reserve wrong-route: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0 (wrong-route rejected)", got)
	}
}

// TestReservationClosedRouteRejected: a reservation whose route has an
// un-reopened closure is rejected at commit.
func TestReservationClosedRouteRejected(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	seedRoute(t, st, pkgID, "RT-CLOSE", "RZA", "SZ", "SYNTHETIC_DEMO")
	closeRoute(t, st, "RT-CLOSE")
	_, token := createSession(t, srv)

	body := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d,"route_id":"RT-CLOSE"}`,
		facID, pkgID, httpDay(1), httpDay(2), "closed-route-key", snap)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if rec.code != http.StatusConflict {
		t.Fatalf("reserve closed-route: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0 (closed-route rejected)", got)
	}
}

// TestTransferWithOldDestinationRouteRejected: a transfer that sends the
// OLD stay's route ID (bound to the OLD facility's safe zone) is rejected
// because the route must be to the NEW facility's safe zone.
func TestTransferWithOldDestinationRouteRejected(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facA, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	// Second facility on a different safe zone in the same package.
	now := time.Now().UTC().Truncate(time.Microsecond)
	facB := "FAC-B-" + fmt.Sprintf("%d", time.Now().UnixNano())
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		// SZ-B is a different safe zone. The seedHTTPPackageFacility uses 'SZ'.
		// We add a zone_versions row for SZ-B so the facility is eligible.
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, location, version, updated_at)
			VALUES ('SZ-B', $1, 'SAFE', NULL, 'OPEN', NULL, NULL, 1, $2)`,
			pkgID, now); err != nil {
			return err
		}
		_, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1, $2, 'SZ-B', 'Asia/Kolkata', 1, $3)`, facB, pkgID, now)
		return err
	})
	if err != nil {
		t.Fatalf("seed facB: %v", err)
	}
	// Route to OLD facility's safe zone (SZ). The transfer tries to reuse this
	// for the NEW facility, whose safe zone is SZ-B. RevalidateReservationContext
	// checks rv.to_safe_zone_id = facility's safe_zone_id; the route's to_safe_zone_id
	// is 'SZ' (the OLD facility's), so it must NOT match facB's 'SZ-B'.
	seedRoute(t, st, pkgID, "RT-OLD-ZONE", "RZA", "SZ", "SYNTHETIC_DEMO")

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facA, pkgID, 1, httpDay(1), httpDay(2), "old-route-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)

	// Transfer to facB with the OLD-zone route. This must be rejected because
	// the route's to_safe_zone_id = 'SZ' != facB's safe_zone_id = 'SZ-B'.
	body := fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"new_route_id":"RT-OLD-ZONE","idempotency_key":"old-route-transfer","snapshot_version":%d}`, facB, snap)
	tr := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", token, body)
	if tr.code != http.StatusConflict {
		t.Fatalf("transfer old-zone: code=%d, want 409; body=%s", tr.code, tr.body)
	}
	// Original stays unchanged.
	var state string
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT state FROM stays WHERE stay_id=$1`, created.StayID).Scan(&state); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != "RESERVED" {
		t.Fatalf("origin state=%s, want RESERVED", state)
	}
	// Capacity unchanged on the original.
	if got := heldCount(t, st, facA, httpDayT(1)); got != 1 {
		t.Fatalf("origin held=%d, want 1", got)
	}
}

// TestReservationMissingRouteOnSeededSucceeds: a reservation WITHOUT a route
// still succeeds when the policy doesn't require one. Documents the current
// contract: routes are optional when policy.route_required is unset.
func TestReservationMissingRouteOnSeededSucceeds(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "no-route-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve without route: code=%d, want 201; body=%s", rec.code, rec.body)
	}
}

// TestRouteRequiredPolicyRejectsMissingRoute: when the authoritative
// allocation_policy.route_required=true and the caller omits route_id (or
// supplies an empty one), the reservation is rejected with
// ROUTE_UNVERIFIED and NO capacity change occurs. The route gate is a
// POLICY boundary, not a caller-controlled one.
func TestRouteRequiredPolicyRejectsMissingRoute(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":true}}`
	pkgID, facID, snap := seedHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)
	// Seed a valid route anyway so the only failure is the missing one.
	seedRoute(t, st, pkgID, "RT-REQ", "RZA", "SZ", "SYNTHETIC_DEMO")

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "req-route-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("missing required route: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	// No capacity change.
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0 (policy-required route missing rejected)", got)
	}
	// No reservation row.
	var n int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM reservations WHERE facility_id=$1`, facID).Scan(&n); err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if n != 0 {
		t.Fatalf("reservations=%d, want 0 (no writes on policy failure)", n)
	}
}

// TestRouteRequiredPolicyAcceptsProvidedRoute: the same policy, but with a
// valid route_id supplied, succeeds. Confirms route_required only constrains
// omission, not the existence of a valid route.
func TestRouteRequiredPolicyAcceptsProvidedRoute(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":true}}`
	pkgID, facID, snap := seedHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)
	seedRoute(t, st, pkgID, "RT-OK", "RZA", "SZ", "SYNTHETIC_DEMO")

	_, token := createSession(t, srv)
	body := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"route_id":"RT-OK","party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
		facID, pkgID, httpDay(1), httpDay(2), "ok-route-key", snap)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if rec.code != http.StatusCreated {
		t.Fatalf("with required route: code=%d, want 201; body=%s", rec.code, rec.body)
	}
}

// TestUnavailableDestinationRejected: a destination whose safe zone is closed
// (zone_versions.status = 'CLOSED') cannot create a new commitment. Capacity
// unchanged. Confirms the eligibility boundary (closed zones) is enforced at
// reservation commit, not only at guidance preview.
func TestUnavailableDestinationRejected(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	// Close the safe zone.
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := st.InTx(t.Context(), func(tx store.DBTX) error {
		// Insert a zone_versions row for 'SZ' and mark it CLOSED. The
		// RevalidateReservationContext path does not read zone status, but the
		// capacity-eligible path does; we close the safe zone so the inventory
		// join does not resolve. The eligibility boundary is enforced here.
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			VALUES ('SZ', $1, 'SAFE', NULL, 'CLOSED', NULL, 1, $2)
			ON CONFLICT (zone_id, package_id) DO UPDATE SET status='CLOSED', version=zone_versions.version+1, updated_at=$2`,
			pkgID, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("close zone: %v", err)
	}

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "closed-zone-key", snap))
	if rec.code == http.StatusCreated {
		t.Fatalf("closed-zone reservation: code=201, want non-201; body=%s", rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("closed-zone held=%d, want 0", got)
	}
}
