package httpserver

// P4 operator operations HTTP verification: real HTTP -> operator auth (MFA +
// jurisdiction) -> service -> PostgreSQL. Gated on STHIRA_TEST_DSN; skipped (not
// passed) when unset. Covers the MFA/identity boundary, cross-jurisdiction scope
// denial, the auditable correction trail, and publish/revoke supersession.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sthira/backend/internal/store"
)

// createOperatorSession issues an operator session over HTTP. jurisdiction and
// mfaVerified drive the boundary. Returns (sessionID, token, response).
func createOperatorSession(t *testing.T, srv *httptest.Server, jurisdiction string, mfaVerified bool) (string, string, response) {
	t.Helper()
	body := fmt.Sprintf(`{"jurisdiction":%q,"mfa_verified":%t}`, jurisdiction, mfaVerified)
	rec := do(t, srv, http.MethodPost, "/api/v3/operations/sessions", "application/json", body)
	if rec.code != http.StatusCreated {
		return "", "", rec
	}
	env := decodeEnvelope(t, rec)
	var data struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	b, _ := json.Marshal(env.Data)
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("decode operator session: %v", err)
	}
	return data.SessionID, data.Token, rec
}

// seedOperatorSource seeds a source in AUTHORIZED with a valid authorization in
// the given jurisdiction, ready for an OPERATIONAL transition. Returns sourceID.
func seedOperatorSource(t *testing.T, st *store.Store, jurisdiction string) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	srcID := "OPSRC-" + fmt.Sprintf("%d", time.Now().UnixNano())
	sources := store.NewSourceStore(store.ChainAuditor{})
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			VALUES ($1,'gov','gov.example','AUTHORIZED',1,$2,$2)`, srcID, now); err != nil {
			return err
		}
		return sources.RecordAuthorization(t.Context(), tx, store.Authorization{
			AuthorizationID: "AUTH-" + srcID,
			SourceID:        srcID,
			GrantedBy:       "gov",
			EvidenceRef:     "doc-1",
			Jurisdiction:    jurisdiction,
			GrantedAt:       now,
		})
	})
	if err != nil {
		t.Fatalf("seed operator source: %v", err)
	}
	return srcID
}

// TestOperatorMFARequired: no operator session is issued without MFA.
func TestOperatorMFARequired(t *testing.T) {
	s, _ := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	_, _, rec := createOperatorSession(t, srv, "JTEST", false)
	if rec.code != http.StatusForbidden {
		t.Fatalf("expected 403 without MFA, got %d body=%s", rec.code, rec.body)
	}
}

// TestOperatorSessionRequiresMFAOnUse: a citizen token cannot call operator
// routes (principal-kind boundary).
func TestOperatorSessionRequiresMFAOnUse(t *testing.T) {
	s, _ := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// A citizen session must not reach operator routes.
	_, citizenToken := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/SRC-x/transitions", citizenToken,
		`{"target":"OPERATIONAL","idempotency_key":"k1"}`)
	if rec.code != http.StatusForbidden {
		t.Fatalf("expected 403 for citizen token on operator route, got %d body=%s", rec.code, rec.body)
	}
}

// TestOperatorPublishRevokeSupersession: publish (AUTHORIZED->OPERATIONAL) then
// revoke (OPERATIONAL->RETIRED), jurisdiction-scoped, with audit.
func TestOperatorPublishRevokeSupersession(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	srcID := seedOperatorSource(t, st, "JTEST")
	_, token, rec := createOperatorSession(t, srv, "JTEST", true)
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}

	// Publish: AUTHORIZED -> OPERATIONAL.
	pub := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"pub-1","reason":"go live"}`)
	if pub.code != http.StatusOK {
		t.Fatalf("publish: code=%d body=%s", pub.code, pub.body)
	}

	// Replay the same publish (same key+payload) returns the stored result.
	pubReplay := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"pub-1","reason":"go live"}`)
	if pubReplay.code != http.StatusOK {
		t.Fatalf("publish replay: code=%d body=%s", pubReplay.code, pubReplay.body)
	}

	// Revoke: OPERATIONAL -> RETIRED.
	rev := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"RETIRED","idempotency_key":"rev-1","reason":"superseded"}`)
	if rev.code != http.StatusOK {
		t.Fatalf("revoke: code=%d body=%s", rev.code, rev.body)
	}

	// Final state is RETIRED at version 3 (AUTHORIZED->OPERATIONAL->RETIRED).
	var state string
	var version int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT state, version FROM sources WHERE source_id=$1`, srcID).Scan(&state, &version); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if state != "RETIRED" || version != 3 {
		t.Fatalf("expected RETIRED v3, got %s v%d", state, version)
	}
}

// TestOperatorCrossJurisdictionDenied: an operator in jurisdiction A cannot
// publish a source authorized only in jurisdiction B.
func TestOperatorCrossJurisdictionDenied(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	srcID := seedOperatorSource(t, st, "JURISDICTION-B")
	_, token, rec := createOperatorSession(t, srv, "JURISDICTION-A", true)
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}

	pub := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"pub-cross","reason":"attempt"}`)
	if pub.code != http.StatusForbidden {
		t.Fatalf("expected 403 cross-jurisdiction, got %d body=%s", pub.code, pub.body)
	}

	// The source must remain AUTHORIZED (no transition leaked through).
	var state string
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT state FROM sources WHERE source_id=$1`, srcID).Scan(&state); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if state != "AUTHORIZED" {
		t.Fatalf("expected AUTHORIZED unchanged, got %s", state)
	}
}

// TestOperatorStayCorrectionAudit: an operator correction releases space and
// writes an audit event attributed to the operator session.
func TestOperatorStayCorrectionAudit(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// Seed a facility (jurisdiction JTEST) and a citizen reservation of party 3.
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 10, httpDayT(1), httpDayT(3))
	_, citizenToken := createSession(t, srv)
	res := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", citizenToken,
		reservationBody(facID, pkgID, 3, httpDay(1), httpDay(3), "res-corr", snap))
	if res.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", res.code, res.body)
	}
	env := decodeEnvelope(t, res)
	var rdata struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	if err := json.Unmarshal(b, &rdata); err != nil {
		t.Fatalf("decode reservation: %v", err)
	}

	// Operator in JTEST corrects party size 3 -> 1 (releases 2).
	opSessID, opToken, rec := createOperatorSession(t, srv, "JTEST", true)
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}
	corr := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/stays/"+rdata.StayID+"/corrections", opToken,
		`{"new_party_size":1,"idempotency_key":"corr-1","reason":"party shrank"}`)
	if corr.code != http.StatusOK {
		t.Fatalf("correction: code=%d body=%s", corr.code, corr.body)
	}

	// Held drops from 3 to 1 on each date.
	var held int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT held FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`, facID, httpDayT(1)).Scan(&held); err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	if held != 1 {
		t.Fatalf("expected held=1 after correction, got %d", held)
	}

	// Audit event recorded, attributed to the operator session.
	var actor, action string
	err := st.DB().QueryRowContext(t.Context(),
		`SELECT actor_id, action FROM audit_events WHERE subject_id=$1 AND action='STAY_CORRECT'`, rdata.StayID).
		Scan(&actor, &action)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if actor != opSessID {
		t.Fatalf("expected audit actor=%s, got %s", opSessID, actor)
	}
}

// TestOperatorCorrectionCrossJurisdictionDenied: an operator in another
// jurisdiction cannot correct a stay whose facility sits elsewhere.
func TestOperatorCorrectionCrossJurisdictionDenied(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 10, httpDayT(1), httpDayT(3))
	_, citizenToken := createSession(t, srv)
	res := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", citizenToken,
		reservationBody(facID, pkgID, 2, httpDay(1), httpDay(3), "res-xjur", snap))
	if res.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", res.code, res.body)
	}
	env := decodeEnvelope(t, res)
	var rdata struct {
		StayID string `json:"stay_id"`
	}
	b, _ := json.Marshal(env.Data)
	if err := json.Unmarshal(b, &rdata); err != nil {
		t.Fatalf("decode reservation: %v", err)
	}

	// Operator in a DIFFERENT jurisdiction attempts the correction.
	_, opToken, rec := createOperatorSession(t, srv, "ELSEWHERE", true)
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}
	corr := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/stays/"+rdata.StayID+"/corrections", opToken,
		`{"new_party_size":1,"idempotency_key":"corr-x","reason":"attempt"}`)
	if corr.code != http.StatusForbidden {
		t.Fatalf("expected 403 cross-jurisdiction correction, got %d body=%s", corr.code, corr.body)
	}

	// Held unchanged (still 2).
	var held int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT held FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`, facID, httpDayT(1)).Scan(&held); err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	if held != 2 {
		t.Fatalf("expected held=2 unchanged, got %d", held)
	}
}
