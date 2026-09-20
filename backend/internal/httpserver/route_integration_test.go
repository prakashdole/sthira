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

	"sthira/backend/internal/contracts"
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

// newExerciseStayServer builds a server configured with the explicit server-side
// synthetic exercise dependency for isolated test flows through real HTTP.
func newExerciseStayServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st := httpDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithSyntheticExercise(StaticSyntheticExercise(true)))
	return s, st
}

// seedSyntheticHTTPPackageFacilityPolicy seeds a package and source artifact with
// SYNTHETIC_DEMO evidence class to test synthetic boundary isolation.
func seedSyntheticHTTPPackageFacilityPolicy(t *testing.T, st *store.Store, capacity int, start, end time.Time, body string) (string, string, int) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	srcID, artID, pkgID, facID := "SRC-SYN-"+suffix, "ART-SYN-"+suffix, "PKG-SYN-"+suffix, "FAC-SYN-"+suffix
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sources := store.NewSourceStore(store.ChainAuditor{})
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			VALUES ($1,'gov','gov.example','OPERATIONAL',1,$2,$2)`, srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,1,$3,$4,'SYNTHETIC_DEMO','mem://t')`, artID, srcID, hash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			VALUES ($1,'ALT',$2,$3,1,'JTEST','SYNTHETIC_DEMO',$4,$5,$6,$7)`, pkgID, srcID, artID, now, now.Add(24*time.Hour), hash, body); err != nil {
			return err
		}
		if err := sources.RecordAuthorization(t.Context(), tx, store.Authorization{
			AuthorizationID: "AUTH-" + srcID,
			SourceID:        srcID,
			GrantedBy:       "gov",
			EvidenceRef:     "doc-1",
			Jurisdiction:    "JTEST",
			GrantedAt:       now,
		}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1,$2,'SZ','Asia/Kolkata',1,$3)`, facID, pkgID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			VALUES ('SZ',$1,'SAFE',NULL,'OPEN',NULL,1,$2)
			ON CONFLICT (zone_id, package_id) DO NOTHING`, pkgID, now); err != nil {
			return err
		}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(t.Context(), `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1,$2,$3,0,0,0,1,$4)`, facID, d, capacity, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seedSyntheticHTTPPackageFacilityPolicy: %v", err)
	}
	return pkgID, facID, 1
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
	s, st := newExerciseStayServer(t)
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
	s, st := newExerciseStayServer(t)
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
	s, st := newExerciseStayServer(t)
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

// TestRoutePolicyAbsentFailsClosed: absent route_required in policy returns 409
// (contracts.ErrValidation, "authoritative stay policy is missing a required field"),
// capacity held = 0, reservations = 0.
func TestRoutePolicyAbsentFailsClosed(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true}}`
	pkgID, facID, snap := seedHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "absent-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("absent route_required: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != string(contracts.ErrValidation) {
		t.Fatalf("error code=%v, want %s; body=%s", env.Errors, contracts.ErrValidation, rec.body)
	}
	if !strings.Contains(env.Errors[0].Message, "authoritative stay policy is missing a required field") {
		t.Fatalf("error message=%q, want 'authoritative stay policy is missing a required field'", env.Errors[0].Message)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
	var count int
	if err := st.DB().QueryRowContext(t.Context(), `SELECT count(*) FROM reservations WHERE facility_id=$1`, facID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("reservations=%d, want 0", count)
	}
}

// TestRoutePolicyNullFailsClosed: "route_required": null returns 409
// (contracts.ErrValidation, "authoritative stay policy is missing a required field"),
// capacity held = 0, reservations = 0.
func TestRoutePolicyNullFailsClosed(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":null}}`
	pkgID, facID, snap := seedHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "null-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("null route_required: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != string(contracts.ErrValidation) {
		t.Fatalf("error code=%v, want %s; body=%s", env.Errors, contracts.ErrValidation, rec.body)
	}
	if !strings.Contains(env.Errors[0].Message, "authoritative stay policy is missing a required field") {
		t.Fatalf("error message=%q, want 'authoritative stay policy is missing a required field'", env.Errors[0].Message)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
	var count int
	if err := st.DB().QueryRowContext(t.Context(), `SELECT count(*) FROM reservations WHERE facility_id=$1`, facID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("reservations=%d, want 0", count)
	}
}

// TestRoutePolicyExplicitFalsePermitsOmission: "route_required": false allows
// reservation without route (201 Created), capacity held = 1.
// Idempotent replay returns 200 with identical IDs, capacity unchanged.
func TestRoutePolicyExplicitFalsePermitsOmission(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`
	pkgID, facID, snap := seedHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)

	_, token := createSession(t, srv)
	body := reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "false-key", snap)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve without route: code=%d, want 201; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		ReservationID string `json:"reservation_id"`
		StayID        string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)
	if created.StayID == "" || created.ReservationID == "" {
		t.Fatalf("missing stay_id or reservation_id: %s", rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 1 {
		t.Fatalf("held=%d, want 1", got)
	}

	// Idempotent replay returns 200 with identical IDs, capacity unchanged.
	replay := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if replay.code != http.StatusOK {
		t.Fatalf("replay: code=%d, want 200; body=%s", replay.code, replay.body)
	}
	envReplay := decodeEnvelope(t, replay)
	var rep struct {
		ReservationID string `json:"reservation_id"`
		StayID        string `json:"stay_id"`
	}
	bRep, _ := json.Marshal(envReplay.Data)
	_ = json.Unmarshal(bRep, &rep)
	if rep.StayID != created.StayID || rep.ReservationID != created.ReservationID {
		t.Fatalf("replay mismatch: got (%s, %s), want (%s, %s)", rep.StayID, rep.ReservationID, created.StayID, created.ReservationID)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 1 {
		t.Fatalf("held after replay=%d, want 1", got)
	}
}

// TestRoutePolicyExplicitTrueRequiresRoute: "route_required": true rejects
// omission with 409 (contracts.ErrRouteUnverified), capacity held = 0.
// Supplying valid route on test server succeeds with 201 Created.
func TestRoutePolicyExplicitTrueRequiresRoute(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":true}}`
	pkgID, facID, snap := seedHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "true-no-route-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("missing required route: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != string(contracts.ErrRouteUnverified) {
		t.Fatalf("error code=%v, want %s; body=%s", env.Errors, contracts.ErrRouteUnverified, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
	var count int
	if err := st.DB().QueryRowContext(t.Context(), `SELECT count(*) FROM reservations WHERE facility_id=$1`, facID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("reservations=%d, want 0", count)
	}

	// Supplying a valid route on exercise test server succeeds with 201 Created.
	sEx, stEx := newExerciseStayServer(t)
	srvEx := httptest.NewServer(sEx.Handler())
	defer srvEx.Close()
	pkgIDEx, facIDEx, snapEx := seedHTTPPackageFacilityPolicy(t, stEx, 3, httpDayT(1), httpDayT(4), policyBody)
	seedRoute(t, stEx, pkgIDEx, "RT-EXP-TRUE", "RZA", "SZ", "SYNTHETIC_DEMO")

	_, tokenEx := createSession(t, srvEx)
	bodyWithRoute := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"route_id":"RT-EXP-TRUE","party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
		facIDEx, pkgIDEx, httpDay(1), httpDay(2), "true-with-route-key", snapEx)
	recEx := doAuthed(t, srvEx, http.MethodPost, "/api/v3/reservations", tokenEx, bodyWithRoute)
	if recEx.code != http.StatusCreated {
		t.Fatalf("valid route supplied on test server: code=%d, want 201; body=%s", recEx.code, recEx.body)
	}
	if got := heldCount(t, stEx, facIDEx, httpDayT(1)); got != 1 {
		t.Fatalf("held on test server=%d, want 1", got)
	}
}

// TestRoutePolicyExtendAndTransfer: proves absent/null fails closed on
// EXTEND/TRANSFER, explicit false permits omission, explicit true requires route.
func TestRoutePolicyExtendAndTransfer(t *testing.T) {
	s, st := newExerciseStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	policyFalse := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`
	pkgID, facA, snap := seedHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(10), policyFalse)
	now := time.Now().UTC().Truncate(time.Microsecond)
	facB := "FAC-B-" + fmt.Sprintf("%d", time.Now().UnixNano())
	facC := "FAC-C-" + fmt.Sprintf("%d", time.Now().UnixNano())

	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1, $2, 'SZ', 'Asia/Kolkata', 1, $3)`, facB, pkgID, now); err != nil {
			return err
		}
		for d := httpDayT(1); d.Before(httpDayT(10)); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(t.Context(), `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1,$2,3,0,0,0,1,$3)`, facB, d, now); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, location, version, updated_at)
			VALUES ('SZ-C', $1, 'SAFE', NULL, 'OPEN', NULL, NULL, 1, $2)`, pkgID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1, $2, 'SZ-C', 'Asia/Kolkata', 1, $3)`, facC, pkgID, now); err != nil {
			return err
		}
		for d := httpDayT(1); d.Before(httpDayT(10)); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(t.Context(), `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1,$2,3,0,0,0,1,$3)`, facC, d, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed facB/facC: %v", err)
	}
	seedRoute(t, st, pkgID, "RT-C", "RZA", "SZ-C", "SYNTHETIC_DEMO")

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facA, pkgID, 1, httpDay(1), httpDay(3), "init-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve init: code=%d body=%s", rec.code, rec.body)
	}
	stayID := decodeFirstStay(t, rec)

	// Step A: absent route_required fails closed on EXTEND and TRANSFER
	policyAbsent := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true}}`
	err = st.InTx(t.Context(), func(tx store.DBTX) error {
		_, err := tx.ExecContext(t.Context(), `UPDATE packages SET body = $1 WHERE package_id = $2`, policyAbsent, pkgID)
		return err
	})
	if err != nil {
		t.Fatalf("update to absent: %v", err)
	}

	bodyExtAbsent := fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":"ext-absent","snapshot_version":%d}`, httpDay(4), snap)
	resExtAbsent := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, bodyExtAbsent)
	if resExtAbsent.code != http.StatusConflict {
		t.Fatalf("extend absent: code=%d, want 409; body=%s", resExtAbsent.code, resExtAbsent.body)
	}
	envExtAbsent := decodeEnvelope(t, resExtAbsent)
	if len(envExtAbsent.Errors) == 0 || envExtAbsent.Errors[0].Code != string(contracts.ErrValidation) {
		t.Fatalf("extend absent error code=%v, want %s", envExtAbsent.Errors, contracts.ErrValidation)
	}

	bodyTrAbsent := fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"idempotency_key":"tr-absent","snapshot_version":%d}`, facB, snap)
	resTrAbsent := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, bodyTrAbsent)
	if resTrAbsent.code != http.StatusConflict {
		t.Fatalf("transfer absent: code=%d, want 409; body=%s", resTrAbsent.code, resTrAbsent.body)
	}
	envTrAbsent := decodeEnvelope(t, resTrAbsent)
	if len(envTrAbsent.Errors) == 0 || envTrAbsent.Errors[0].Code != string(contracts.ErrValidation) {
		t.Fatalf("transfer absent error code=%v, want %s", envTrAbsent.Errors, contracts.ErrValidation)
	}

	// Step B: null route_required fails closed on EXTEND and TRANSFER
	policyNull := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":null}}`
	err = st.InTx(t.Context(), func(tx store.DBTX) error {
		_, err := tx.ExecContext(t.Context(), `UPDATE packages SET body = $1 WHERE package_id = $2`, policyNull, pkgID)
		return err
	})
	if err != nil {
		t.Fatalf("update to null: %v", err)
	}

	bodyExtNull := fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":"ext-null","snapshot_version":%d}`, httpDay(4), snap)
	resExtNull := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, bodyExtNull)
	if resExtNull.code != http.StatusConflict {
		t.Fatalf("extend null: code=%d, want 409; body=%s", resExtNull.code, resExtNull.body)
	}

	bodyTrNull := fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"idempotency_key":"tr-null","snapshot_version":%d}`, facB, snap)
	resTrNull := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, bodyTrNull)
	if resTrNull.code != http.StatusConflict {
		t.Fatalf("transfer null: code=%d, want 409; body=%s", resTrNull.code, resTrNull.body)
	}

	// Step C: explicit false permits omission on EXTEND and TRANSFER
	err = st.InTx(t.Context(), func(tx store.DBTX) error {
		_, err := tx.ExecContext(t.Context(), `UPDATE packages SET body = $1 WHERE package_id = $2`, policyFalse, pkgID)
		return err
	})
	if err != nil {
		t.Fatalf("update to false: %v", err)
	}

	bodyExtFalse := fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":"ext-false","snapshot_version":%d}`, httpDay(4), snap)
	resExtFalse := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, bodyExtFalse)
	if resExtFalse.code != http.StatusOK {
		t.Fatalf("extend false: code=%d, want 200; body=%s", resExtFalse.code, resExtFalse.body)
	}

	bodyTrFalse := fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"idempotency_key":"tr-false","snapshot_version":%d}`, facB, snap)
	resTrFalse := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, bodyTrFalse)
	if resTrFalse.code != http.StatusOK {
		t.Fatalf("transfer false: code=%d, want 200; body=%s", resTrFalse.code, resTrFalse.body)
	}
	envTrFalse := decodeEnvelope(t, resTrFalse)
	var trResult struct {
		NewStayID string `json:"new_stay_id"`
	}
	bTr, _ := json.Marshal(envTrFalse.Data)
	_ = json.Unmarshal(bTr, &trResult)
	activeStayID := trResult.NewStayID

	// Step D: explicit true requires route
	policyTrue := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":true}}`
	err = st.InTx(t.Context(), func(tx store.DBTX) error {
		_, err := tx.ExecContext(t.Context(), `UPDATE packages SET body = $1 WHERE package_id = $2`, policyTrue, pkgID)
		return err
	})
	if err != nil {
		t.Fatalf("update to true: %v", err)
	}

	bodyExtTrueNoRoute := fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":"ext-true-noroute","snapshot_version":%d}`, httpDay(5), snap)
	resExtTrueNoRoute := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+activeStayID+"/events", token, bodyExtTrueNoRoute)
	if resExtTrueNoRoute.code != http.StatusConflict {
		t.Fatalf("extend true without route: code=%d, want 409; body=%s", resExtTrueNoRoute.code, resExtTrueNoRoute.body)
	}
	envExtTrue := decodeEnvelope(t, resExtTrueNoRoute)
	if len(envExtTrue.Errors) == 0 || envExtTrue.Errors[0].Code != string(contracts.ErrRouteUnverified) {
		t.Fatalf("extend true error code=%v, want %s", envExtTrue.Errors, contracts.ErrRouteUnverified)
	}

	bodyTrTrueNoRoute := fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"idempotency_key":"tr-true-noroute","snapshot_version":%d}`, facC, snap)
	resTrTrueNoRoute := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+activeStayID+"/events", token, bodyTrTrueNoRoute)
	if resTrTrueNoRoute.code != http.StatusConflict {
		t.Fatalf("transfer true without route: code=%d, want 409; body=%s", resTrTrueNoRoute.code, resTrTrueNoRoute.body)
	}
	envTrTrueNoRoute := decodeEnvelope(t, resTrTrueNoRoute)
	if len(envTrTrueNoRoute.Errors) == 0 || envTrTrueNoRoute.Errors[0].Code != string(contracts.ErrRouteUnverified) {
		t.Fatalf("transfer true without route error code=%v, want %s", envTrTrueNoRoute.Errors, contracts.ErrRouteUnverified)
	}

	bodyTrTrueWithRoute := fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"new_route_id":"RT-C","idempotency_key":"tr-true-withroute","snapshot_version":%d}`, facC, snap)
	resTrTrueWithRoute := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+activeStayID+"/events", token, bodyTrTrueWithRoute)
	if resTrTrueWithRoute.code != http.StatusOK {
		t.Fatalf("transfer true with route: code=%d, want 200; body=%s", resTrTrueWithRoute.code, resTrTrueWithRoute.body)
	}
	envTrTrueWithRoute := decodeEnvelope(t, resTrTrueWithRoute)
	var trResultC struct {
		NewStayID string `json:"new_stay_id"`
	}
	bTrC, _ := json.Marshal(envTrTrueWithRoute.Data)
	_ = json.Unmarshal(bTrC, &trResultC)

	bodyExtWithRoute := fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":"ext-with-route-ok","snapshot_version":%d}`, httpDay(5), snap)
	resExtWithRoute := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+trResultC.NewStayID+"/events", token, bodyExtWithRoute)
	if resExtWithRoute.code != http.StatusOK {
		t.Fatalf("extend with route: code=%d, want 200; body=%s", resExtWithRoute.code, resExtWithRoute.body)
	}
}

// TestNormalServerRejectsSyntheticDemoCommitment: normal server rejects
// reservation against SYNTHETIC_DEMO package and route (409 Conflict,
// contracts.ErrValidation). Zero held capacity, zero reservations.
func TestNormalServerRejectsSyntheticDemoCommitment(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`
	pkgID, facID, snap := seedSyntheticHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)
	seedRoute(t, st, pkgID, "RT-SYN-DEMO", "RZA", "SZ", "SYNTHETIC_DEMO")

	_, token := createSession(t, srv)
	// Without route
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "syn-no-route", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("normal server synthetic reservation without route: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != string(contracts.ErrValidation) {
		t.Fatalf("error code=%v, want %s; body=%s", env.Errors, contracts.ErrValidation, rec.body)
	}
	if !strings.Contains(env.Errors[0].Message, "reservation context is not") {
		t.Fatalf("expected error message for non-operational context, got: %q", env.Errors[0].Message)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}

	// With route
	bodyWithRoute := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"route_id":"RT-SYN-DEMO","party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
		facID, pkgID, httpDay(1), httpDay(2), "syn-with-route", snap)
	rec2 := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, bodyWithRoute)
	if rec2.code != http.StatusConflict {
		t.Fatalf("normal server synthetic reservation with route: code=%d, want 409; body=%s", rec2.code, rec2.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
	var count int
	if err := st.DB().QueryRowContext(t.Context(), `SELECT count(*) FROM reservations WHERE facility_id=$1`, facID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("reservations=%d, want 0", count)
	}
}

// TestNormalServerHeadersAndBodyCannotEnableSynthetic: normal server rejects
// synthetic reservation even when header X-Sthira-Synthetic-Route-Gate: open or
// body fields (route_gate_open: true) are passed. Zero held capacity.
func TestNormalServerHeadersAndBodyCannotEnableSynthetic(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`
	pkgID, facID, snap := seedSyntheticHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)
	seedRoute(t, st, pkgID, "RT-SYN-SMUGGLE", "RZA", "SZ", "SYNTHETIC_DEMO")

	_, token := createSession(t, srv)

	// 1. With X-Sthira-Synthetic-Route-Gate header set on standard reservation body.
	bodyStandard := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"route_id":"RT-SYN-SMUGGLE","party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
		facID, pkgID, httpDay(1), httpDay(2), "smuggle-header-key", snap)
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/reservations", strings.NewReader(bodyStandard))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("X-Sthira-Synthetic-Route-Gate", "open")
	resp1, err := srv.Client().Do(req1)
	if err != nil {
		t.Fatalf("request 1: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusConflict {
		t.Fatalf("header smuggling code=%d, want 409", resp1.StatusCode)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}

	// 2. With body fields smuggled (route_gate_open: true).
	bodySmuggle := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"route_id":"RT-SYN-SMUGGLE","party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d,"route_gate_open":true}`,
		facID, pkgID, httpDay(1), httpDay(2), "smuggle-body-key", snap)
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/reservations", strings.NewReader(bodySmuggle))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := srv.Client().Do(req2)
	if err != nil {
		t.Fatalf("request 2: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest && resp2.StatusCode != http.StatusConflict {
		t.Fatalf("body smuggling code=%d, want 400 or 409", resp2.StatusCode)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
	var count int
	if err := st.DB().QueryRowContext(t.Context(), `SELECT count(*) FROM reservations WHERE facility_id=$1`, facID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("reservations=%d, want 0", count)
	}
}

// TestIsolatedTestConfiguredServerCompletesSyntheticReservation: isolated
// test-configured server (WithSyntheticExercise(StaticSyntheticExercise(true)))
// successfully completes synthetic reservation through real HTTP and
// PostgreSQL (201 Created), capacity held = 1.
func TestIsolatedTestConfiguredServerCompletesSyntheticReservation(t *testing.T) {
	s, st := newExerciseStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":true}}`
	pkgID, facID, snap := seedSyntheticHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)
	seedRoute(t, st, pkgID, "RT-SYN-EX-OK", "RZA", "SZ", "SYNTHETIC_DEMO")

	_, token := createSession(t, srv)
	body := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"route_id":"RT-SYN-EX-OK","party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
		facID, pkgID, httpDay(1), httpDay(2), "ex-ok-key", snap)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if rec.code != http.StatusCreated {
		t.Fatalf("test server synthetic reservation: code=%d, want 201; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID        string `json:"stay_id"`
		ReservationID string `json:"reservation_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)
	if created.StayID == "" || created.ReservationID == "" {
		t.Fatalf("missing stay_id or reservation_id: %s", rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 1 {
		t.Fatalf("held=%d, want 1", got)
	}
}

// TestIsolatedTestConfiguredServerRejectsRequiredRouteOmission: omission of
// required route on test-configured server returns 409.
func TestIsolatedTestConfiguredServerRejectsRequiredRouteOmission(t *testing.T) {
	s, st := newExerciseStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":true}}`
	pkgID, facID, snap := seedSyntheticHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), policyBody)
	seedRoute(t, st, pkgID, "RT-SYN-EX-REQ", "RZA", "SZ", "SYNTHETIC_DEMO")

	_, token := createSession(t, srv)
	body := reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "ex-missing-route-key", snap)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if rec.code != http.StatusConflict {
		t.Fatalf("missing required route on test server: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != string(contracts.ErrRouteUnverified) {
		t.Fatalf("error code=%v, want %s; body=%s", env.Errors, contracts.ErrRouteUnverified, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
}

// TestExtensionAndTransferSyntheticBoundaryIsolation: normal server rejects
// synthetic extension and transfer; test server permits them when valid, and
// rejects wrong destination or closed route.
func TestExtensionAndTransferSyntheticBoundaryIsolation(t *testing.T) {
	sEx, st := newExerciseStayServer(t)
	srvEx := httptest.NewServer(sEx.Handler())
	defer srvEx.Close()

	policyBody := `{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`
	pkgID, facA, snap := seedSyntheticHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(10), policyBody)
	now := time.Now().UTC().Truncate(time.Microsecond)
	facB := "FAC-SYN-B-" + fmt.Sprintf("%d", time.Now().UnixNano())

	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1, $2, 'SZ', 'Asia/Kolkata', 1, $3)`, facB, pkgID, now); err != nil {
			return err
		}
		for d := httpDayT(1); d.Before(httpDayT(10)); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(t.Context(), `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1,$2,3,0,0,0,1,$3)`, facB, d, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed facB: %v", err)
	}

	_, token := createSession(t, srvEx)
	rec := doAuthed(t, srvEx, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facA, pkgID, 1, httpDay(1), httpDay(3), "syn-stay-init", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve on test server: code=%d body=%s", rec.code, rec.body)
	}
	stayID := decodeFirstStay(t, rec)

	// Normal server must fail closed with 409 Conflict (ErrValidation - "reservation context is not operational")
	sNorm := New(DefaultConfig("127.0.0.1:0"), WithStore(st))
	srvNorm := httptest.NewServer(sNorm.Handler())
	defer srvNorm.Close()

	extNorm := doAuthed(t, srvNorm, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token,
		fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":"norm-ext-fail","snapshot_version":%d}`, httpDay(4), snap))
	if extNorm.code != http.StatusConflict {
		t.Fatalf("normal server extend synthetic: code=%d, want 409; body=%s", extNorm.code, extNorm.body)
	}
	envExtNorm := decodeEnvelope(t, extNorm)
	if len(envExtNorm.Errors) == 0 || envExtNorm.Errors[0].Code != string(contracts.ErrValidation) {
		t.Fatalf("normal extend error code=%v, want %s", envExtNorm.Errors, contracts.ErrValidation)
	}

	trNorm := doAuthed(t, srvNorm, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token,
		fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"idempotency_key":"norm-tr-fail","snapshot_version":%d}`, facB, snap))
	if trNorm.code != http.StatusConflict {
		t.Fatalf("normal server transfer synthetic: code=%d, want 409; body=%s", trNorm.code, trNorm.body)
	}
	envTrNorm := decodeEnvelope(t, trNorm)
	if len(envTrNorm.Errors) == 0 || envTrNorm.Errors[0].Code != string(contracts.ErrValidation) {
		t.Fatalf("normal transfer error code=%v, want %s", envTrNorm.Errors, contracts.ErrValidation)
	}

	// On TEST server, EXTEND and TRANSFER succeed.
	extEx := doAuthed(t, srvEx, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token,
		fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":"ex-ext-ok","snapshot_version":%d}`, httpDay(4), snap))
	if extEx.code != http.StatusOK {
		t.Fatalf("test server extend: code=%d, want 200; body=%s", extEx.code, extEx.body)
	}

	trEx := doAuthed(t, srvEx, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token,
		fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"idempotency_key":"ex-tr-ok","snapshot_version":%d}`, facB, snap))
	if trEx.code != http.StatusOK {
		t.Fatalf("test server transfer: code=%d, want 200; body=%s", trEx.code, trEx.body)
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
	s, st := newExerciseStayServer(t)
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

// TestReservationRejectsZeroSnapshotVersion: a reservation with
// snapshot_version=0 (or absent in JSON) is rejected at the boundary with
// 400 INVALID_VALUE; the revalidation path is never reached. This locks
// down the "mandatory where required" defect at the handler boundary
// rather than relying on the inner snapshotStaleError branch.
func TestReservationRejectsZeroSnapshotVersion(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, _ := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	// snapshot_version=0 explicitly.
	body := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":0}`,
		facID, pkgID, httpDay(1), httpDay(2), "zero-snap-key")
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if rec.code != http.StatusBadRequest {
		t.Fatalf("zero snapshot_version: code=%d, want 400; body=%s", rec.code, rec.body)
	}
	// snapshot_version absent.
	body2 := fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"party_size":1,"start_date":%q,"end_date":%q,"idempotency_key":%q}`,
		facID, pkgID, httpDay(1), httpDay(2), "absent-snap-key")
	rec2 := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body2)
	if rec2.code != http.StatusBadRequest {
		t.Fatalf("absent snapshot_version: code=%d, want 400; body=%s", rec2.code, rec2.body)
	}
	// No writes.
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("zero/absent snapshot_version held=%d, want 0", got)
	}
}

// TestExtendRejectsZeroSnapshotVersion: an EXTEND event with snapshot_version
// missing or 0 is rejected at the boundary with 400 INVALID_VALUE.
func TestExtendRejectsZeroSnapshotVersion(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)
	// Reserve first.
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "ext-zero-snap-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	stayID := decodeFirstStay(t, rec)

	// EXTEND with snapshot_version=0.
	body := fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":%q,"snapshot_version":0}`,
		httpDay(3), "ext-zero-key")
	ex := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, body)
	if ex.code != http.StatusBadRequest {
		t.Fatalf("zero EXTEND snapshot_version: code=%d, want 400; body=%s", ex.code, ex.body)
	}
	// EXTEND with snapshot_version missing.
	body2 := fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":%q}`,
		httpDay(3), "ext-absent-key")
	ex2 := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, body2)
	if ex2.code != http.StatusBadRequest {
		t.Fatalf("absent EXTEND snapshot_version: code=%d, want 400; body=%s", ex2.code, ex2.body)
	}
}
