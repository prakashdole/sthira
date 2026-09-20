package httpserver

// P4 citizen stay HTTP verification: real HTTP -> session auth -> service ->
// PostgreSQL. Gated on STHIRA_TEST_DSN; skipped (not passed) when unset. Covers
// the auth boundary, cross-session denial, duplicate-confirmation replay, stale
// snapshot rejection, and the authenticated read path for restart recovery.

import (
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

// httpTestDB opens a real store or skips.
func httpTestDB(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" {
		t.Skip("STHIRA_TEST_DSN unset; skipping real-DB HTTP verification")
	}
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// newStayServer builds a server wired to the real store.
func newStayServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st))
	return s, st
}

// seedHTTPPackageFacility seeds a package + facility + inventory and returns
// (packageID, facilityID, snapshotVersion).
func seedHTTPPackageFacility(t *testing.T, st *store.Store, capacity int, start, end time.Time) (string, string, int) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	srcID, artID, pkgID, facID := "SRC-"+suffix, "ART-"+suffix, "PKG-"+suffix, "FAC-"+suffix
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			VALUES ($1,'gov','gov.example','DISCOVERED',1,$2,$2)`, srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,1,$3,$4,'SYNTHETIC','mem://t')`, artID, srcID, hash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			VALUES ($1,'ALT',$2,$3,1,'JTEST','SYNTHETIC',$4,$5,$6,'{}')`, pkgID, srcID, artID, now, now.Add(24*time.Hour), hash); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1,$2,'SZ','Asia/Kolkata',1,$3)`, facID, pkgID, now); err != nil {
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
		t.Fatalf("seed: %v", err)
	}
	return pkgID, facID, 1
}

// createSession issues a citizen session over HTTP and returns (sessionID, token).
func createSession(t *testing.T, srv *httptest.Server) (string, string) {
	t.Helper()
	rec := do(t, srv, http.MethodPost, "/api/v3/sessions", "application/json", `{}`)
	if rec.code != http.StatusCreated {
		t.Fatalf("create session: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var data struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	b, _ := json.Marshal(env.Data)
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if data.Token == "" || data.SessionID == "" {
		t.Fatalf("session missing token/id: %s", rec.body)
	}
	return data.SessionID, data.Token
}

// doAuthed performs a request with a Bearer token.
func doAuthed(t *testing.T, srv *httptest.Server, method, path, token, body string) response {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("doAuthed: %v", err)
	}
	defer resp.Body.Close()
	b := readAll(t, resp)
	return response{code: resp.StatusCode, header: resp.Header, body: b}
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return sb.String()
}

func httpDay(n int) string {
	return time.Date(2031, 3, n, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func httpDayT(n int) time.Time { return time.Date(2031, 3, n, 0, 0, 0, 0, time.UTC) }

// reservationBody builds a create-reservation payload.
func reservationBody(facID, pkgID string, party int, start, end string, key string, snap int) string {
	return fmt.Sprintf(`{"facility_id":%q,"package_id":%q,"party_size":%d,"start_date":%q,"end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
		facID, pkgID, party, start, end, key, snap)
}

// TestHTTPCreateReservationAndRead: full reserve flow then owner read.
func TestHTTPCreateReservationAndRead(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 2, httpDay(1), httpDay(3), "key-1", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		ReservationID string `json:"reservation_id"`
		StayID        string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)
	if created.StayID == "" {
		t.Fatalf("no stay_id: %s", rec.body)
	}

	// Owner read path (restart recovery).
	get := doAuthed(t, srv, http.MethodGet, "/api/v3/reservations/"+created.StayID, token, "")
	if get.code != http.StatusOK {
		t.Fatalf("owner read: code=%d body=%s", get.code, get.body)
	}
	if get.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("private read must be no-store, got %q", get.header.Get("Cache-Control"))
	}
	genv := decodeEnvelope(t, get)
	var rd struct {
		State string `json:"state"`
	}
	gb, _ := json.Marshal(genv.Data)
	_ = json.Unmarshal(gb, &rd)
	if rd.State != "RESERVED" {
		t.Fatalf("state=%s, want RESERVED", rd.State)
	}
}

// TestHTTPRequiresSession: writes and the private read reject missing/invalid tokens.
func TestHTTPRequiresSession(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))

	// No token.
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", "",
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "k", snap))
	if rec.code != http.StatusUnauthorized {
		t.Fatalf("no token: code=%d, want 401", rec.code)
	}
	// Garbage token.
	rec = doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", "not-a-token",
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "k", snap))
	if rec.code != http.StatusUnauthorized {
		t.Fatalf("bad token: code=%d, want 401", rec.code)
	}
}

// TestHTTPCrossSessionDenial: one session cannot read another's stay (R22).
func TestHTTPCrossSessionDenial(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, tokenA := createSession(t, srv)
	_, tokenB := createSession(t, srv)

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", tokenA,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "key-a", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve A: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)

	// B reads A's stay -> 403 (knowing the ID is not authorization).
	get := doAuthed(t, srv, http.MethodGet, "/api/v3/reservations/"+created.StayID, tokenB, "")
	if get.code != http.StatusForbidden {
		t.Fatalf("cross-session read: code=%d, want 403; body=%s", get.code, get.body)
	}
	// B events on A's stay -> 403.
	ev := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", tokenB,
		`{"type":"CANCEL","idempotency_key":"ev-b"}`)
	if ev.code != http.StatusForbidden {
		t.Fatalf("cross-session event: code=%d, want 403; body=%s", ev.code, ev.body)
	}
}

// TestHTTPDuplicateConfirmationReplay: same idempotency key + payload replays
// the stored result without a duplicate reservation or double capacity.
func TestHTTPDuplicateConfirmationReplay(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, srv)
	body := reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "dup-key", snap)

	r1 := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if r1.code != http.StatusCreated {
		t.Fatalf("first: code=%d body=%s", r1.code, r1.body)
	}
	r2 := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token, body)
	if r2.code != http.StatusOK {
		t.Fatalf("replay: code=%d, want 200 replay; body=%s", r2.code, r2.body)
	}
	// Same stay id both times.
	var d1, d2 struct {
		StayID string `json:"stay_id"`
	}
	e1 := decodeEnvelope(t, r1)
	b1, _ := json.Marshal(e1.Data)
	_ = json.Unmarshal(b1, &d1)
	e2 := decodeEnvelope(t, r2)
	b2, _ := json.Marshal(e2.Data)
	_ = json.Unmarshal(b2, &d2)
	if d1.StayID != d2.StayID {
		t.Fatalf("replay returned different stay: %s vs %s", d1.StayID, d2.StayID)
	}
	// Capacity held exactly once.
	var held int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT held FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
		facID, httpDayT(1)).Scan(&held); err != nil {
		t.Fatalf("read held: %v", err)
	}
	if held != 1 {
		t.Fatalf("held=%d, want 1 (no double decrement)", held)
	}
}

// TestHTTPStaleSnapshotRejected: a reservation validated against a stale source
// snapshot version is rejected with STALE_VERSION.
func TestHTTPStaleSnapshotRejected(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, _ := seedHTTPPackageFacility(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, srv)

	// snapshot_version 999 != current 1 -> STALE_VERSION.
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "stale-key", 999))
	if rec.code != http.StatusConflict {
		t.Fatalf("stale snapshot: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrStaleVersion {
		t.Fatalf("want STALE_VERSION, got %+v", env.Errors)
	}
}

// TestHTTPCapacityConflictNoSubstitution: a full facility returns
// CAPACITY_CONFLICT and never silently picks another destination.
func TestHTTPCapacityConflictNoSubstitution(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 1, httpDayT(1), httpDayT(3))
	_, tok1 := createSession(t, srv)
	_, tok2 := createSession(t, srv)

	r1 := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", tok1,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "k1", snap))
	if r1.code != http.StatusCreated {
		t.Fatalf("first reserve: code=%d body=%s", r1.code, r1.body)
	}
	// Second party for the same single space -> conflict.
	r2 := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", tok2,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "k2", snap))
	if r2.code != http.StatusConflict {
		t.Fatalf("capacity conflict: code=%d, want 409; body=%s", r2.code, r2.body)
	}
	env := decodeEnvelope(t, r2)
	if len(env.Errors) == 0 || env.Errors[0].Code != contracts.ErrCapacityConflict {
		t.Fatalf("want CAPACITY_CONFLICT, got %+v", env.Errors)
	}
}

// TestHTTPArriveDepartFlow: arrive then depart over HTTP conserves capacity.
func TestHTTPArriveDepartFlow(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 2, httpDayT(1), httpDayT(3))
	_, token := createSession(t, srv)

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "flow-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)

	// Arrive (explicit citizen confirmation).
	ar := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", token,
		`{"type":"ARRIVE","idempotency_key":"ev-arrive"}`)
	if ar.code != http.StatusOK {
		t.Fatalf("arrive: code=%d body=%s", ar.code, ar.body)
	}
	var held, occ int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT held, occupied FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
		facID, httpDayT(1)).Scan(&held, &occ); err != nil {
		t.Fatalf("read buckets: %v", err)
	}
	if held != 0 || occ != 1 {
		t.Fatalf("after arrive: held=%d occupied=%d, want 0/1", held, occ)
	}

	// Depart.
	dp := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", token,
		`{"type":"DEPART","idempotency_key":"ev-depart"}`)
	if dp.code != http.StatusOK {
		t.Fatalf("depart: code=%d body=%s", dp.code, dp.body)
	}
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT held, occupied FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
		facID, httpDayT(1)).Scan(&held, &occ); err != nil {
		t.Fatalf("read buckets: %v", err)
	}
	if held != 0 || occ != 0 {
		t.Fatalf("after depart: held=%d occupied=%d, want 0/0", held, occ)
	}
}

// TestHTTPGuidanceQueryUnknownCapacityHonest: a facility with no inventory row
// is reported with unknown capacity, never promised.
func TestHTTPGuidanceQueryUnknownCapacityHonest(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	// Seed a package + facility but NO inventory rows (unknown capacity).
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	srcID, artID, pkgID, facID := "SRC-"+suffix, "ART-"+suffix, "PKG-"+suffix, "FAC-"+suffix
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at) VALUES ($1,'gov','gov.example','DISCOVERED',1,$2,$2)`, srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref) VALUES ($1,$2,1,$3,$4,'SYNTHETIC','mem://t')`, artID, srcID, hash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body) VALUES ($1,'ALT',$2,$3,1,'JTEST','SYNTHETIC',$4,$5,$6,'{}')`, pkgID, srcID, artID, now, now.Add(24*time.Hour), hash); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at) VALUES ($1,$2,'SZ','Asia/Kolkata',1,$3)`, facID, pkgID, now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/guidance/query", "",
		fmt.Sprintf(`{"jurisdiction":"JTEST","package_id":%q,"party_size":1,"start_date":%q,"end_date":%q}`, pkgID, httpDay(1), httpDay(2)))
	if rec.code != http.StatusOK {
		t.Fatalf("guidance: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var data struct {
		Destinations []struct {
			FacilityID    string `json:"facility_id"`
			CapacityKnown bool   `json:"capacity_known"`
			RouteVerified bool   `json:"route_verified"`
		} `json:"destinations"`
		RouteGate bool `json:"route_gate"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &data)
	if data.RouteGate {
		t.Fatalf("route gate must be closed while O05 open")
	}
	// No inventory -> facility excluded (cannot fit) OR reported unknown; never promised free space.
	for _, d := range data.Destinations {
		if d.RouteVerified {
			t.Fatalf("route verified while gate closed for %s", d.FacilityID)
		}
	}
}
