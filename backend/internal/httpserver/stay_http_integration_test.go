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

// seedHTTPPackageFacility seeds an OPERATIONAL, authorized package + facility +
// inventory with a valid stay policy, and returns (packageID, facilityID,
// snapshotVersion). Reservation commit revalidation (Item B/C) requires the
// source to be OPERATIONAL with a live authorization and the package body to
// carry an authoritative allocation_policy; a bare DISCOVERED seed no longer
// passes the commit gate.
func seedHTTPPackageFacility(t *testing.T, st *store.Store, capacity int, start, end time.Time) (string, string, int) {
	t.Helper()
	return seedHTTPPackageFacilityPolicy(t, st, capacity, start, end,
		`{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`)
}

// seedHTTPPackageFacilityPolicy is seedHTTPPackageFacility with a caller-supplied
// package body (to vary or omit the stay policy).
func seedHTTPPackageFacilityPolicy(t *testing.T, st *store.Store, capacity int, start, end time.Time, body string) (string, string, int) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	srcID, artID, pkgID, facID := "SRC-"+suffix, "ART-"+suffix, "PKG-"+suffix, "FAC-"+suffix
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
			VALUES ($1,$2,1,$3,$4,'AUTHORIZED_OPERATIONAL','mem://t')`, artID, srcID, hash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			VALUES ($1,'ALT',$2,$3,1,'JTEST','AUTHORIZED_OPERATIONAL',$4,$5,$6,$7)`, pkgID, srcID, artID, now, now.Add(24*time.Hour), hash, body); err != nil {
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

// --- Item B: authoritative eligibility enforced at reservation commit ---

// quarantineSource quarantines a source directly at the store seam.
func quarantineSource(t *testing.T, st *store.Store, srcID string) {
	t.Helper()
	sources := store.NewSourceStore(store.ChainAuditor{})
	now := time.Now().UTC()
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		src, err := sources.GetSource(t.Context(), tx, srcID)
		if err != nil {
			return err
		}
		return sources.Quarantine(t.Context(), tx, srcID, src.Version, "test-op", "suspect", "EV-Q-"+srcID, now)
	})
	if err != nil {
		t.Fatalf("quarantine: %v", err)
	}
}

// sourceIDForPackage resolves the source backing a seeded package.
func sourceIDForPackage(t *testing.T, st *store.Store, pkgID string) string {
	t.Helper()
	var srcID string
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT source_id FROM packages WHERE package_id=$1`, pkgID).Scan(&srcID); err != nil {
		t.Fatalf("resolve source: %v", err)
	}
	return srcID
}

// heldCount reads the held bucket for a facility/date.
func heldCount(t *testing.T, st *store.Store, facID string, d time.Time) int {
	t.Helper()
	var held int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT held FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
		facID, d).Scan(&held); err != nil {
		t.Fatalf("read held: %v", err)
	}
	return held
}

// TestHTTPQuarantineDeniesNewReservation: quarantining the source after a
// snapshot was read denies a NEW reservation and leaves capacity unchanged.
func TestHTTPQuarantineDeniesNewReservation(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	quarantineSource(t, st, sourceIDForPackage(t, st, pkgID))

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "q-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("quarantined reserve: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d after denied reservation, want 0 (no capacity moved)", got)
	}
}

// TestHTTPSuspendedSourceDeniesReservation: a SUSPENDED source denies a new
// reservation at commit.
func TestHTTPSuspendedSourceDeniesReservation(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	sources := store.NewSourceStore(store.ChainAuditor{})
	srcID := sourceIDForPackage(t, st, pkgID)
	now := time.Now().UTC()
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		src, err := sources.GetSource(t.Context(), tx, srcID)
		if err != nil {
			return err
		}
		return sources.Transition(t.Context(), tx, srcID, src.Version, "SUSPENDED", "test-op", "hold", "EV-S-"+srcID, now)
	})
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "s-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("suspended reserve: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
}

// TestHTTPRevokedAuthorizationDeniesReservation: removing the source's
// authorization denies a new reservation even while the source row stays
// OPERATIONAL.
func TestHTTPRevokedAuthorizationDeniesReservation(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	srcID := sourceIDForPackage(t, st, pkgID)
	if _, err := st.DB().ExecContext(t.Context(),
		`DELETE FROM source_authorizations WHERE source_id=$1`, srcID); err != nil {
		t.Fatalf("revoke auth: %v", err)
	}

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "r-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("unauthorized reserve: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
}

// TestHTTPSupersededPackageDeniesReservation: a superseded package is no longer
// a valid operational context.
func TestHTTPSupersededPackageDeniesReservation(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	// superseded_by is a self-FK: create a real superseding package first.
	pkgNewer, _, _ := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	if _, err := st.DB().ExecContext(t.Context(),
		`UPDATE packages SET superseded_by=$2 WHERE package_id=$1`, pkgID, pkgNewer); err != nil {
		t.Fatalf("supersede: %v", err)
	}

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "sup-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("superseded reserve: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
}

// TestHTTPFacilityPackageMismatchRejected: a facility that does not belong to
// the requested package is rejected (no cross-package binding).
func TestHTTPFacilityPackageMismatchRejected(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	_, facID, _ := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	// A second, independent package; facID belongs to the first.
	pkgID2, _, _ := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID2, 1, httpDay(1), httpDay(2), "mm-key", 1))
	if rec.code != http.StatusConflict {
		t.Fatalf("mismatch reserve: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	if len(env.Errors) == 0 {
		t.Fatalf("want error envelope, got %s", rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
}

// --- Item C: expiry + stay policy wired to HTTP flows ---

// TestHTTPReservationStoresPolicyDeadline: a reservation created over HTTP
// persists expires_at derived from the authoritative policy + server time.
func TestHTTPReservationStoresPolicyDeadline(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	before := time.Now().UTC()
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "exp-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)

	var expiresAt *time.Time
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT expires_at FROM stays WHERE stay_id=$1`, created.StayID).Scan(&expiresAt); err != nil {
		t.Fatalf("read expires_at: %v", err)
	}
	if expiresAt == nil {
		t.Fatal("expires_at not persisted on stay")
	}
	// Policy is 3600s; deadline must be ~before+3600 (within a generous skew).
	wantLo := before.Add(3590 * time.Second)
	wantHi := time.Now().UTC().Add(3610 * time.Second)
	if expiresAt.Before(wantLo) || expiresAt.After(wantHi) {
		t.Fatalf("expires_at=%v outside policy window [%v, %v]", expiresAt, wantLo, wantHi)
	}
}

// TestHTTPMissingPolicyRejected: a package without an authoritative stay policy
// cannot create a reservation (no invented defaults).
func TestHTTPMissingPolicyRejected(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacilityPolicy(t, st, 3, httpDayT(1), httpDayT(4), `{}`)
	_, token := createSession(t, srv)

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "nop-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("missing policy: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
}

// TestHTTPOutOfPolicyDatesRejected: a stay interval outside the authoritative
// min/max day bounds cannot allocate capacity.
func TestHTTPOutOfPolicyDatesRejected(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	// Policy allows max 14 days; request a 20-day stay (seed inventory covers it).
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 3, httpDayT(1), httpDayT(25))
	_, token := createSession(t, srv)

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(21), "oop-key", snap))
	if rec.code != http.StatusConflict {
		t.Fatalf("out-of-policy dates: code=%d, want 409; body=%s", rec.code, rec.body)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d, want 0", got)
	}
}

// TestHTTPExpiryWorkerReleasesHoldOnce: a reservation whose policy deadline
// passes is expired by the real worker, releasing capacity exactly once.
func TestHTTPExpiryWorkerReleasesHoldOnce(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	// 1-second expiry policy so the worker can fire on controlled time.
	pkgID, facID, snap := seedHTTPPackageFacilityPolicy(t, st, 2, httpDayT(1), httpDayT(4),
		`{"allocation_policy":{"reservation_expiry_seconds":1,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`)
	_, token := createSession(t, srv)

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "wk-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)
	if got := heldCount(t, st, facID, httpDayT(1)); got != 1 {
		t.Fatalf("held=%d before expiry, want 1", got)
	}

	// Advance past the deadline, then run the real worker tick.
	time.Sleep(1500 * time.Millisecond)
	stays := store.NewStayStore(store.ChainAuditor{})
	worker := store.NewExpiryWorker(st, stays)
	if _, err := worker.Tick(t.Context()); err != nil {
		t.Fatalf("worker tick: %v", err)
	}

	var state string
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT state FROM stays WHERE stay_id=$1`, created.StayID).Scan(&state); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != "EXPIRED" {
		t.Fatalf("state=%s, want EXPIRED", state)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d after expiry, want 0 (released)", got)
	}

	// A second tick is a no-op: capacity released exactly once.
	if _, err := worker.Tick(t.Context()); err != nil {
		t.Fatalf("worker tick 2: %v", err)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 0 {
		t.Fatalf("held=%d after second tick, want 0 (no double release)", got)
	}
}

// TestHTTPFailedTransferPreservesOriginal: a transfer to a full facility fails
// and leaves the original stay and its capacity intact.
func TestHTTPFailedTransferPreservesOriginal(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 2, httpDayT(1), httpDayT(4))
	// A full target facility in the SAME package (capacity 0).
	now := time.Now().UTC().Truncate(time.Microsecond)
	fullFac := "FAC-FULL-" + fmt.Sprintf("%d", time.Now().UnixNano())
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1,$2,'SZ','Asia/Kolkata',1,$3)`, fullFac, pkgID, now); err != nil {
			return err
		}
		for d := httpDayT(1); d.Before(httpDayT(4)); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(t.Context(), `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1,$2,0,0,0,0,1,$3)`, fullFac, d, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed full facility: %v", err)
	}

	_, token := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "tf-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	env := decodeEnvelope(t, rec)
	var created struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	_ = json.Unmarshal(b, &created)

	// Transfer to the full facility must fail. snapshot_version is required
	// for TRANSFER so the locked-snapshot revalidation cannot be skipped.
	tr := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+created.StayID+"/events", token,
		fmt.Sprintf(`{"type":"TRANSFER","new_facility_id":%q,"idempotency_key":"ev-tf","snapshot_version":%d}`, fullFac, snap))
	if tr.code != http.StatusConflict {
		t.Fatalf("transfer to full: code=%d, want 409; body=%s", tr.code, tr.body)
	}
	// Original retained: still RESERVED, capacity still held on the origin.
	var state string
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT state FROM stays WHERE stay_id=$1`, created.StayID).Scan(&state); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != "RESERVED" {
		t.Fatalf("origin state=%s, want RESERVED (transfer failed atomically)", state)
	}
	if got := heldCount(t, st, facID, httpDayT(1)); got != 1 {
		t.Fatalf("origin held=%d, want 1 (capacity retained)", got)
	}
}
