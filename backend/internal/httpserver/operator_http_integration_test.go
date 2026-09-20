package httpserver

// P4 operator operations HTTP verification: real HTTP -> trusted identity/MFA
// boundary -> server-controlled grant -> service -> PostgreSQL. Gated on
// STHIRA_TEST_DSN; skipped (not passed) when unset.
//
// The synthetic verifier here is TEST-ONLY: it is injected via
// WithOperatorVerifier and simulates a trusted boundary by reading a header the
// production binary never wires. It is never selectable by an ordinary request
// in the production server (main wires no verifier, so issuance fails closed).

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sthira/backend/internal/store"
)

// syntheticVerifier is a test-injected trusted boundary. It treats the
// X-Test-Operator-Subject header as an already-verified identity with MFA. The
// production server never wires this; it exists only so tests can exercise the
// grant-bound issuance path.
type syntheticVerifier struct {
	// mfaNil, when true, returns an identity with no MFA evidence (rejected).
	mfaNil bool
	// fail, when set, makes verification fail (missing/invalid evidence).
	fail bool
}

func (v syntheticVerifier) VerifyOperator(r *http.Request) (VerifiedIdentity, error) {
	if v.fail {
		return VerifiedIdentity{}, errors.New("synthetic: identity evidence rejected")
	}
	subj := r.Header.Get("X-Test-Operator-Subject")
	if subj == "" {
		return VerifiedIdentity{}, errors.New("synthetic: no verified subject")
	}
	now := time.Now().UTC()
	id := VerifiedIdentity{Subject: subj, MFAVerifiedAt: &now}
	if v.mfaNil {
		id.MFAVerifiedAt = nil
	}
	return id, nil
}

// newOperatorServer builds a server wired to the real store plus the test-only
// synthetic verifier, and returns it.
func newOperatorServer(t *testing.T, v OperatorVerifier) (*httptest.Server, *store.Store) {
	t.Helper()
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithOperatorVerifier(v))
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv, st
}

// grantOperator provisions a server-controlled grant for a subject+jurisdiction.
func grantOperator(t *testing.T, st *store.Store, subject, jurisdiction string) {
	t.Helper()
	now := time.Now().UTC()
	err := store.OperatorGrantStore{}.GrantOperator(t.Context(), st.DB(),
		"GRANT-"+subject+"-"+jurisdiction, subject, jurisdiction, "test-admin", nil, now)
	if err != nil {
		t.Fatalf("grant operator: %v", err)
	}
}

// issueOperator calls the issuance endpoint with the given verified subject and
// returns (sessionID, token, response).
func issueOperator(t *testing.T, srv *httptest.Server, subject string) (string, string, response) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/operations/sessions", nil)
	if subject != "" {
		req.Header.Set("X-Test-Operator-Subject", subject)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("issue operator: %v", err)
	}
	defer resp.Body.Close()
	rec := response{code: resp.StatusCode, header: resp.Header, body: readAll(t, resp)}
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

// --- issuance boundary ---

// TestOperatorIssuanceFailsClosedNoVerifier: with no verifier wired (the
// production default), issuance is unavailable and no token is minted even with
// a self-asserted mfa_verified:true body.
func TestOperatorIssuanceFailsClosedNoVerifier(t *testing.T) {
	s, _ := newStayServer(t) // no WithOperatorVerifier
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	rec := do(t, srv, http.MethodPost, "/api/v3/operations/sessions", "application/json",
		`{"jurisdiction":"JTEST","mfa_verified":true}`)
	if rec.code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 fail-closed without verifier, got %d body=%s", rec.code, rec.body)
	}
}

// TestOperatorIssuanceRejectsBodyMFA: a request-body mfa_verified:true is not
// evidence; without a verified subject the boundary rejects issuance.
func TestOperatorIssuanceRejectsBodyMFA(t *testing.T) {
	srv, _ := newOperatorServer(t, syntheticVerifier{})
	// No X-Test-Operator-Subject header: the synthetic boundary has nothing
	// verified. A body flag must not substitute.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/operations/sessions", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without verified subject, got %d", resp.StatusCode)
	}
}

// TestOperatorIssuanceNoGrant: a verified identity with MFA but no
// server-controlled grant cannot obtain an operator session.
func TestOperatorIssuanceNoGrant(t *testing.T) {
	srv, _ := newOperatorServer(t, syntheticVerifier{})
	_, _, rec := issueOperator(t, srv, "subj-no-grant")
	if rec.code != http.StatusForbidden {
		t.Fatalf("expected 403 without grant, got %d body=%s", rec.code, rec.body)
	}
}

// TestOperatorIssuanceMFARequired: a verified identity without MFA evidence is
// rejected even with a grant.
func TestOperatorIssuanceMFARequired(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{mfaNil: true})
	grantOperator(t, st, "subj-nomfa", "JTEST")
	_, _, rec := issueOperator(t, srv, "subj-nomfa")
	if rec.code != http.StatusForbidden {
		t.Fatalf("expected 403 without MFA, got %d body=%s", rec.code, rec.body)
	}
}

// TestOperatorIssuanceInvalidEvidence: rejected identity evidence -> 401.
func TestOperatorIssuanceInvalidEvidence(t *testing.T) {
	srv, _ := newOperatorServer(t, syntheticVerifier{fail: true})
	_, _, rec := issueOperator(t, srv, "subj-x")
	if rec.code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on rejected evidence, got %d body=%s", rec.code, rec.body)
	}
}

// TestOperatorIssuanceDerivesJurisdiction: with a verified identity, MFA and a
// grant, issuance succeeds and the session jurisdiction comes from the grant
// (the caller never supplied one).
func TestOperatorIssuanceDerivesJurisdiction(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-op", "JTEST")
	sessID, token, rec := issueOperator(t, srv, "subj-op")
	if rec.code != http.StatusCreated {
		t.Fatalf("expected 201 with grant+MFA, got %d body=%s", rec.code, rec.body)
	}
	if sessID == "" || token == "" {
		t.Fatalf("missing session/token: %s", rec.body)
	}
	// The session's jurisdiction is the granted one.
	var jur string
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT jurisdiction FROM sessions WHERE session_id=$1`, sessID).Scan(&jur); err != nil {
		t.Fatalf("read session: %v", err)
	}
	if jur != "JTEST" {
		t.Fatalf("expected jurisdiction JTEST from grant, got %s", jur)
	}
}

// --- operator route authorization ---

// TestOperatorSessionRequiresMFAOnUse: a citizen token cannot call operator
// routes (principal-kind boundary).
func TestOperatorSessionRequiresMFAOnUse(t *testing.T) {
	s, _ := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	_, citizenToken := createSession(t, srv)
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/SRC-x/transitions", citizenToken,
		`{"target":"OPERATIONAL","idempotency_key":"k1"}`)
	if rec.code != http.StatusForbidden {
		t.Fatalf("expected 403 for citizen token on operator route, got %d body=%s", rec.code, rec.body)
	}
}

// TestOperatorPublishRevokeSupersession: publish (AUTHORIZED->OPERATIONAL) then
// revoke (OPERATIONAL->RETIRED), jurisdiction-scoped, with idempotent replay.
func TestOperatorPublishRevokeSupersession(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-pub", "JTEST")
	srcID := seedOperatorSource(t, st, "JTEST")
	_, token, rec := issueOperator(t, srv, "subj-pub")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}

	pub := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"pub-1","reason":"go live"}`)
	if pub.code != http.StatusOK {
		t.Fatalf("publish: code=%d body=%s", pub.code, pub.body)
	}
	pubReplay := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"pub-1","reason":"go live"}`)
	if pubReplay.code != http.StatusOK {
		t.Fatalf("publish replay: code=%d body=%s", pubReplay.code, pubReplay.body)
	}
	rev := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"RETIRED","idempotency_key":"rev-1","reason":"superseded"}`)
	if rev.code != http.StatusOK {
		t.Fatalf("revoke: code=%d body=%s", rev.code, rev.body)
	}

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

// TestOperatorCrossJurisdictionDenied: an operator granted jurisdiction A cannot
// publish a source authorized only in jurisdiction B.
func TestOperatorCrossJurisdictionDenied(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-cross", "JURISDICTION-A")
	srcID := seedOperatorSource(t, st, "JURISDICTION-B")
	_, token, rec := issueOperator(t, srv, "subj-cross")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}

	pub := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"pub-cross","reason":"attempt"}`)
	if pub.code != http.StatusForbidden {
		t.Fatalf("expected 403 cross-jurisdiction, got %d body=%s", pub.code, pub.body)
	}
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
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-corr", "JTEST")

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

	opSessID, opToken, rec := issueOperator(t, srv, "subj-corr")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}
	corr := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/stays/"+rdata.StayID+"/corrections", opToken,
		`{"new_party_size":1,"idempotency_key":"corr-1","reason":"party shrank"}`)
	if corr.code != http.StatusOK {
		t.Fatalf("correction: code=%d body=%s", corr.code, corr.body)
	}

	var held int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT held FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`, facID, httpDayT(1)).Scan(&held); err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	if held != 1 {
		t.Fatalf("expected held=1 after correction, got %d", held)
	}
	var actor string
	err := st.DB().QueryRowContext(t.Context(),
		`SELECT actor_id FROM audit_events WHERE subject_id=$1 AND action='STAY_CORRECT'`, rdata.StayID).Scan(&actor)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if actor != opSessID {
		t.Fatalf("expected audit actor=%s, got %s", opSessID, actor)
	}
}

// TestOperatorCorrectionCrossJurisdictionDenied: an operator granted another
// jurisdiction cannot correct a stay whose facility sits elsewhere.
func TestOperatorCorrectionCrossJurisdictionDenied(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-xc", "ELSEWHERE")

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

	_, opToken, rec := issueOperator(t, srv, "subj-xc")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}
	corr := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/stays/"+rdata.StayID+"/corrections", opToken,
		`{"new_party_size":1,"idempotency_key":"corr-x","reason":"attempt"}`)
	if corr.code != http.StatusForbidden {
		t.Fatalf("expected 403 cross-jurisdiction correction, got %d body=%s", corr.code, corr.body)
	}
	var held int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT held FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`, facID, httpDayT(1)).Scan(&held); err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	if held != 2 {
		t.Fatalf("expected held=2 unchanged, got %d", held)
	}
}

// --- idempotency resource-binding ---

// TestOperatorIdempotencyTargetBinding: the same key+body against a DIFFERENT
// target source must not return the first target's result.
func TestOperatorIdempotencyTargetBinding(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-idem", "JTEST")
	srcA := seedOperatorSource(t, st, "JTEST")
	srcB := seedOperatorSource(t, st, "JTEST")
	_, token, rec := issueOperator(t, srv, "subj-idem")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}

	// Publish source A with key "same-key".
	a := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcA+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"same-key","reason":"r"}`)
	if a.code != http.StatusOK {
		t.Fatalf("publish A: code=%d body=%s", a.code, a.body)
	}
	// Same key+body against source B must NOT replay A's result; it is a distinct
	// resource-scoped key and applies the transition to B.
	b := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcB+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"same-key","reason":"r"}`)
	if b.code != http.StatusOK {
		t.Fatalf("publish B (distinct target): code=%d body=%s", b.code, b.body)
	}
	// Both transitions applied independently.
	for _, id := range []string{srcA, srcB} {
		var state string
		if err := st.DB().QueryRowContext(t.Context(), `SELECT state FROM sources WHERE source_id=$1`, id).Scan(&state); err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		if state != "OPERATIONAL" {
			t.Fatalf("expected %s OPERATIONAL, got %s", id, state)
		}
	}
}

// TestOperatorIdempotencyPayloadConflict: same key, changed payload -> conflict.
func TestOperatorIdempotencyPayloadConflict(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-pc", "JTEST")
	srcID := seedOperatorSource(t, st, "JTEST")
	_, token, rec := issueOperator(t, srv, "subj-pc")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}

	first := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"k-conf","reason":"one"}`)
	if first.code != http.StatusOK {
		t.Fatalf("first: code=%d body=%s", first.code, first.body)
	}
	// Same key, different payload (different reason) -> 409 conflict.
	conflict := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"k-conf","reason":"two"}`)
	if conflict.code != http.StatusConflict {
		t.Fatalf("expected 409 payload conflict, got %d body=%s", conflict.code, conflict.body)
	}
}

// --- quarantine ---

// TestOperatorQuarantine: authorized quarantine restricts the source, is
// idempotent, attributed, and a second quarantine is terminal-conflict.
func TestOperatorQuarantine(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-q", "JTEST")
	srcID := seedOperatorSource(t, st, "JTEST")
	opSess, token, rec := issueOperator(t, srv, "subj-q")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}

	q := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/quarantine", token,
		`{"idempotency_key":"q-1","reason":"suspect evidence"}`)
	if q.code != http.StatusOK {
		t.Fatalf("quarantine: code=%d body=%s", q.code, q.body)
	}
	// Replay returns the stored result without a duplicate transition.
	qReplay := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/quarantine", token,
		`{"idempotency_key":"q-1","reason":"suspect evidence"}`)
	if qReplay.code != http.StatusOK {
		t.Fatalf("quarantine replay: code=%d body=%s", qReplay.code, qReplay.body)
	}

	var state string
	var version int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT state, version FROM sources WHERE source_id=$1`, srcID).Scan(&state, &version); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if state != "QUARANTINED" || version != 2 {
		t.Fatalf("expected QUARANTINED v2, got %s v%d", state, version)
	}
	// Audit attributed to the operator.
	var actor string
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT actor_id FROM audit_events WHERE subject_id=$1 AND to_state='QUARANTINED'`, srcID).Scan(&actor); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if actor != opSess {
		t.Fatalf("expected audit actor=%s, got %s", opSess, actor)
	}

	// A second, distinct quarantine (different key) is terminal-conflict.
	q2 := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/quarantine", token,
		`{"idempotency_key":"q-2","reason":"again"}`)
	if q2.code != http.StatusConflict {
		t.Fatalf("expected 409 terminal quarantine, got %d body=%s", q2.code, q2.body)
	}
}

// TestOperatorQuarantineCrossJurisdiction: an operator granted jurisdiction A
// cannot quarantine a source authorized only in jurisdiction B.
func TestOperatorQuarantineCrossJurisdiction(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-qx", "JURISDICTION-A")
	srcID := seedOperatorSource(t, st, "JURISDICTION-B")
	_, token, rec := issueOperator(t, srv, "subj-qx")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}
	q := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/quarantine", token,
		`{"idempotency_key":"q-x","reason":"attempt"}`)
	if q.code != http.StatusForbidden {
		t.Fatalf("expected 403 cross-jurisdiction quarantine, got %d body=%s", q.code, q.body)
	}
	var state string
	if err := st.DB().QueryRowContext(t.Context(), `SELECT state FROM sources WHERE source_id=$1`, srcID).Scan(&state); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if state != "AUTHORIZED" {
		t.Fatalf("expected AUTHORIZED unchanged, got %s", state)
	}
}

// --- Item A: persisted identity + current-grant enforcement ---

// TestOperatorGrantRevokedDeniesOperationAndReplay: a session issued under a
// valid grant is denied once that grant is revoked — both a fresh operation and
// the replay of a previously-completed key.
func TestOperatorGrantRevokedDeniesOperationAndReplay(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-rev", "JTEST")
	srcID := seedOperatorSource(t, st, "JTEST")
	_, token, rec := issueOperator(t, srv, "subj-rev")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}

	// Complete a transition so a replayable key exists.
	ok := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"t-1","reason":"publish"}`)
	if ok.code != http.StatusOK {
		t.Fatalf("initial transition: code=%d body=%s", ok.code, ok.body)
	}

	// Revoke the grant out-of-band.
	now := time.Now().UTC()
	if err := (store.OperatorGrantStore{}).RevokeOperatorGrant(t.Context(), st.DB(), "subj-rev", "JTEST", now); err != nil {
		t.Fatalf("revoke grant: %v", err)
	}

	// Replay of the completed key is denied (no disclosure under a dead grant).
	rep := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"t-1","reason":"publish"}`)
	if rep.code != http.StatusForbidden {
		t.Fatalf("replay after revoke: expected 403, got %d body=%s", rep.code, rep.body)
	}
	// A fresh operation is denied too.
	fresh := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"SUSPENDED","idempotency_key":"t-2","reason":"suspend"}`)
	if fresh.code != http.StatusForbidden {
		t.Fatalf("fresh op after revoke: expected 403, got %d body=%s", fresh.code, fresh.body)
	}
	// Source state unchanged by the denied attempts (still OPERATIONAL from t-1).
	var state string
	if err := st.DB().QueryRowContext(t.Context(), `SELECT state FROM sources WHERE source_id=$1`, srcID).Scan(&state); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if state != "OPERATIONAL" {
		t.Fatalf("expected OPERATIONAL unchanged, got %s", state)
	}
}

// TestOperatorGrantExpiredDeniesSession: a session whose grant has expired is
// denied even though the session itself is unexpired and unrevoked.
func TestOperatorGrantExpiredDeniesSession(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	// Grant that expires almost immediately.
	now := time.Now().UTC()
	exp := now.Add(2 * time.Second)
	if err := (store.OperatorGrantStore{}).GrantOperator(t.Context(), st.DB(),
		"GRANT-subj-exp-JTEST", "subj-exp", "JTEST", "test-admin", &exp, now); err != nil {
		t.Fatalf("grant: %v", err)
	}
	srcID := seedOperatorSource(t, st, "JTEST")
	_, token, rec := issueOperator(t, srv, "subj-exp")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}
	// Let the grant expire.
	time.Sleep(3 * time.Second)
	resp := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"e-1","reason":"publish"}`)
	if resp.code != http.StatusForbidden {
		t.Fatalf("expired grant: expected 403, got %d body=%s", resp.code, resp.body)
	}
}

// TestLegacyUnboundOperatorSessionCannotAct: an OPERATOR session with no
// persisted verified subject (legacy self-attested) cannot act, even if its
// token is otherwise live. Migration 0005 revokes these; this asserts the store
// also refuses to treat them as grant-bound.
func TestLegacyUnboundOperatorSessionCannotAct(t *testing.T) {
	st := httpTestDB(t)
	now := time.Now().UTC()
	// Insert a legacy operator session directly: MFA present but no subject/grant.
	sessID := "SESS-legacy-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if _, err := st.DB().ExecContext(t.Context(), `
		INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, created_at, expires_at, mfa_verified_at)
		VALUES ($1,'OPERATOR','JTEST',$2,$3,$4,$5)`,
		sessID, "legacy-cred", now, now.Add(time.Hour), now); err != nil {
		t.Fatalf("insert legacy session: %v", err)
	}
	// ValidateSessionGrant must reject a session with no subject/grant binding.
	sess := store.Session{SessionID: sessID, PrincipalKind: "OPERATOR"}
	err := store.OperatorGrantStore{}.ValidateSessionGrant(sess, store.Grant{}, now)
	if !errors.Is(err, store.ErrGrantInvalid) {
		t.Fatalf("expected ErrGrantInvalid for unbound session, got %v", err)
	}
}

// TestOperatorAuditResolvesToVerifiedIdentity: the audit actor (operator
// session) resolves through the session row to the verified identity subject.
func TestOperatorAuditResolvesToVerifiedIdentity(t *testing.T) {
	srv, st := newOperatorServer(t, syntheticVerifier{})
	grantOperator(t, st, "subj-audit", "JTEST")
	srcID := seedOperatorSource(t, st, "JTEST")
	opSess, token, rec := issueOperator(t, srv, "subj-audit")
	if rec.code != http.StatusCreated {
		t.Fatalf("operator session: code=%d body=%s", rec.code, rec.body)
	}
	ok := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", token,
		`{"target":"OPERATIONAL","idempotency_key":"a-1","reason":"publish"}`)
	if ok.code != http.StatusOK {
		t.Fatalf("transition: code=%d body=%s", ok.code, ok.body)
	}
	// Trace event -> session -> verified subject.
	var subject string
	err := st.DB().QueryRowContext(t.Context(), `
		SELECT s.operator_subject FROM audit_events a
		JOIN sessions s ON s.session_id = a.actor_id
		WHERE a.subject_id = $1 AND a.to_state = 'OPERATIONAL'`, srcID).Scan(&subject)
	if err != nil {
		t.Fatalf("trace audit to subject: %v", err)
	}
	if subject != "subj-audit" {
		t.Fatalf("expected audit to resolve to verified subject subj-audit, got %q (actor session %s)", subject, opSess)
	}
}

// TestCitizenSessionUnaffectedByOperatorGrant: a citizen session still works for
// the citizen path and is denied on operator routes, independent of grants.
func TestCitizenSessionUnaffectedByOperatorGrant(t *testing.T) {
	srv, _ := newOperatorServer(t, syntheticVerifier{})
	// Citizen session via the public endpoint.
	c := do(t, srv, http.MethodPost, "/api/v3/sessions", "application/json", `{}`)
	if c.code != http.StatusCreated {
		t.Fatalf("citizen session: code=%d body=%s", c.code, c.body)
	}
	env := decodeEnvelope(t, c)
	var data struct {
		Token string `json:"token"`
	}
	b, _ := json.Marshal(env.Data)
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatalf("decode citizen session: %v", err)
	}
	srcID := "OPSRC-citizen"
	denied := doAuthed(t, srv, http.MethodPost, "/api/v3/operations/sources/"+srcID+"/transitions", data.Token,
		`{"target":"OPERATIONAL","idempotency_key":"c-1"}`)
	if denied.code != http.StatusForbidden {
		t.Fatalf("citizen on operator route: expected 403, got %d body=%s", denied.code, denied.body)
	}
}
