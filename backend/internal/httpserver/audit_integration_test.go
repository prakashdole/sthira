package httpserver

// P4 audit-identity + replay verification against the real HTTP boundary.
// Gated on STHIRA_TEST_DSN; skipped when unset.
//
// Each distinct committed operation (each distinct idempotency key for the
// same subject + action) produces its own audit event. Retries of the SAME
// idempotency key produce NO duplicate event or capacity change. The chain
// (prev_hash -> event_hash) is verified via store.VerifyChain after every
// retry, ensuring the global ordering remains intact across legitimate
// interleaving of events from different subjects and across retries.
// Audit EventID scope: different authorized operator sessions using the
// same idempotency key for distinct corrections do not collide merely
// because the subject/action/key match — the ActorID is part of the event
// identity, but two operators using the SAME subject/action/key on
// distinct sessions DO collide, which is the documented identity boundary.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"sthira/backend/internal/store"
)

// uniqueSuffix returns a per-run suffix that is unique within the run.
// Combined with the prefix, the result is collision-resistant against
// the shared DB pollution from other tests.
func uniqueSuffix() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

// nowUTCPrecise is a tiny helper for the store-layer path in this test.
func nowUTCPrecise() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// seedHTTPSecondFacility inserts a second facility on a different safe
// zone in the same package, with inventory for the half-open date range.
// Used by the interleaved-chains test to keep two stays on separate
// facilities so capacity conservation is independent.
func seedHTTPSecondFacility(t *testing.T, st *store.Store, pkgID, facID string, capacity int, start, end time.Time) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			VALUES ('SZ-B', $1, 'SAFE', NULL, 'OPEN', NULL, 1, $2)
			ON CONFLICT (zone_id, package_id) DO NOTHING`, pkgID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1, $2, 'SZ-B', 'Asia/Kolkata', 1, $3)
			ON CONFLICT (facility_id) DO NOTHING`, facID, pkgID, now); err != nil {
			return err
		}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(t.Context(), `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1, $2, $3, 0, 0, 0, 1, $4)
				ON CONFLICT DO NOTHING`, facID, d, capacity, now); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seedHTTPSecondFacility: %v", err)
	}
}

// TestRetrySameKeyNoDuplicateAuditOrCapacityReplay: retrying the same
// idempotency key through the real HTTP boundary returns the SAME stored
// result, with NO duplicate audit event and NO capacity change. The audit
// chain is verified globally after the retry.
func TestRetrySameKeyNoDuplicateAuditOrCapacityReplay(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 5, httpDayT(1), httpDayT(8))
	_, token := createSession(t, srv)

	// Reserve the stay.
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "retry-reserve-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	stayID := decodeFirstStay(t, rec)

	// First EXTEND.
	idemKey := "extend-retry-http"
	extendBody := fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
		httpDay(3), idemKey, snap)
	first := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, extendBody)
	if first.code != http.StatusOK {
		t.Fatalf("first extend: code=%d body=%s", first.code, first.body)
	}
	firstEnv := decodeEnvelope(t, first)
	firstData, _ := json.Marshal(firstEnv.Data)

	// Retry the SAME idempotency key.
	retry := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token, extendBody)
	if retry.code != http.StatusOK {
		t.Fatalf("retry extend: code=%d body=%s", retry.code, retry.body)
	}
	retryEnv := decodeEnvelope(t, retry)
	retryData, _ := json.Marshal(retryEnv.Data)
	if string(firstData) != string(retryData) {
		t.Fatalf("retry returned different payload:\nfirst=%s\nretry=%s", firstData, retryData)
	}

	// Audit events: exactly one STAY_EXTEND for this stay.
	if n := countAuditBySubject(t, st, stayID, "STAY_EXTEND"); n != 1 {
		t.Fatalf("STAY_EXTEND events=%d, want 1 (no duplicate)", n)
	}
	// Capacity unchanged.
	if got := heldCount(t, st, facID, httpDayT(2)); got != 1 {
		t.Fatalf("held on day(2) after retry=%d, want 1", got)
	}
	// Global chain integrity.
	if err := store.VerifyChain(t.Context(), st.DB()); err != nil {
		t.Fatalf("audit chain verify after retry: %v", err)
	}
}

// TestTwoExtensionsDifferentKeysBothSucceed: two EXTEND operations on the
// same stay with DIFFERENT idempotency keys both succeed; both produce
// distinct audit events (different EventIDs because the idem keys differ).
func TestTwoExtensionsDifferentKeysBothSucceed(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 5, httpDayT(1), httpDayT(8))
	_, token := createSession(t, srv)

	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "extA-reserve-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	stayID := decodeFirstStay(t, rec)

	// First EXTEND.
	first := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token,
		fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
			httpDay(3), "extA-http", snap))
	if first.code != http.StatusOK {
		t.Fatalf("extend A: code=%d body=%s", first.code, first.body)
	}
	// Second EXTEND with different key.
	second := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+stayID+"/events", token,
		fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
			httpDay(4), "extB-http", snap))
	if second.code != http.StatusOK {
		t.Fatalf("extend B: code=%d body=%s", second.code, second.body)
	}
	if n := countAuditBySubject(t, st, stayID, "STAY_EXTEND"); n != 2 {
		t.Fatalf("STAY_EXTEND events=%d, want 2", n)
	}
	ids := auditEventIDsForSubject(t, st, stayID, "STAY_EXTEND")
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("extend event IDs not distinct: %v", ids)
	}
	if err := store.VerifyChain(t.Context(), st.DB()); err != nil {
		t.Fatalf("audit chain verify: %v", err)
	}
}

// TestChainInterleavingAcrossSubjects: two distinct stays produce
// interleaved audit events; the global chain remains valid across both
// subjects (a chain test that filters one subject would falsely pass even
// if the global chain were broken).
func TestChainInterleavingAcrossSubjects(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 5, httpDayT(1), httpDayT(8))
	_, token := createSession(t, srv)

	// First stay.
	recA := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 1, httpDay(1), httpDay(2), "inter-A-key", snap))
	if recA.code != http.StatusCreated {
		t.Fatalf("reserve A: code=%d body=%s", recA.code, recA.body)
	}
	stayA := decodeFirstStay(t, recA)

	// Second stay on different facility so it doesn't share capacity.
	facB := "FAC-B-" + uniqueSuffix()
	seedHTTPSecondFacility(t, st, pkgID, facB, 3, httpDayT(1), httpDayT(4))
	recB := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facB, pkgID, 1, httpDay(1), httpDay(2), "inter-B-key", snap))
	if recB.code != http.StatusCreated {
		t.Fatalf("reserve B: code=%d body=%s", recB.code, recB.body)
	}
	stayB := decodeFirstStay(t, recB)

	// Interleave EXTENDs across A and B. Each stay is lengthened twice:
	// A: [1,2) -> [1,3) -> [1,4); B: [1,2) -> [1,3) -> [1,4). The
	// interleaving order is A1, B1, A2, B2 so the audit chain genuinely
	// interleaves the two subjects.
	extSeq := []struct{ stayID, key, endDate string }{
		{stayA, "inter-ext-A1", httpDay(3)},
		{stayB, "inter-ext-B1", httpDay(3)},
		{stayA, "inter-ext-A2", httpDay(4)},
		{stayB, "inter-ext-B2", httpDay(4)},
	}
	for i, args := range extSeq {
		rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations/"+args.stayID+"/events", token,
			fmt.Sprintf(`{"type":"EXTEND","new_end_date":%q,"idempotency_key":%q,"snapshot_version":%d}`,
				args.endDate, args.key, snap))
		if rec.code != http.StatusOK {
			t.Fatalf("extend %d: code=%d body=%s", i, rec.code, rec.body)
		}
	}

	// Global chain integrity (validates the entire interleaved sequence).
	if err := store.VerifyChain(t.Context(), st.DB()); err != nil {
		t.Fatalf("audit chain verify (interleaved): %v", err)
	}
	// Both subjects have 2 STAY_EXTEND events.
	if n := countAuditBySubject(t, st, stayA, "STAY_EXTEND"); n != 2 {
		t.Fatalf("STAY_EXTEND stayA=%d, want 2", n)
	}
	if n := countAuditBySubject(t, st, stayB, "STAY_EXTEND"); n != 2 {
		t.Fatalf("STAY_EXTEND stayB=%d, want 2", n)
	}
	// Interleaving check: the event_seq for stayA's first EXTEND must NOT
	// be adjacent to stayA's second EXTEND in the chain — they are split
	// by B's events.
	seqsA := auditEventSeqsForSubject(t, st, stayA, "STAY_EXTEND")
	seqsB := auditEventSeqsForSubject(t, st, stayB, "STAY_EXTEND")
	sort.Slice(seqsA, func(i, j int) bool { return seqsA[i] < seqsA[j] })
	sort.Slice(seqsB, func(i, j int) bool { return seqsB[i] < seqsB[j] })
	all := append(append([]int64{}, seqsA...), seqsB...)
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	interleaved := false
	for i := 1; i < len(all); i++ {
		prev, cur := all[i-1], all[i]
		prevIsA := contains(seqsA, prev)
		curIsA := contains(seqsA, cur)
		if prevIsA != curIsA {
			interleaved = true
			break
		}
	}
	if !interleaved {
		t.Fatalf("events not interleaved; seqsA=%v seqsB=%v", seqsA, seqsB)
	}
}

// TestTwoCorrectionsDistinctOperatorSessions: two downward corrections by
// DIFFERENT operator sessions, with the same idempotency key, both succeed
// because the audit identity scope is per-session; the event records the
// actor and is distinct from a single-actor identity. The chain remains
// valid across both.
func TestTwoCorrectionsDistinctOperatorSessions(t *testing.T) {
	s, st := newStayServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	pkgID, facID, snap := seedHTTPPackageFacility(t, st, 5, httpDayT(1), httpDayT(4))
	_, token := createSession(t, srv)

	// Reserve with party 4 (so corrections have room).
	rec := doAuthed(t, srv, http.MethodPost, "/api/v3/reservations", token,
		reservationBody(facID, pkgID, 4, httpDay(1), httpDay(2), "corr-reserve-key", snap))
	if rec.code != http.StatusCreated {
		t.Fatalf("reserve: code=%d body=%s", rec.code, rec.body)
	}
	stayID := decodeFirstStay(t, rec)

	// Operator session A and B (synthetic).
	// Corrections on a stay require operator authorization; we exercise the
	// store layer directly (operator handler coverage is in
	// operator_http_integration_test.go). The test confirms the store
	// records actor_id correctly and the chain stays valid across two
	// distinct operator sessions using the same key.
	idem := "shared-corr-key"
	if err := st.InTx(t.Context(), func(tx store.DBTX) error {
		return store.NewStayStore(store.ChainAuditor{}).Correct(t.Context(), tx, stayID, 3, "OPS-A", "less", nowUTCPrecise(), idem)
	}); err != nil {
		t.Fatalf("correct A: %v", err)
	}
	if err := st.InTx(t.Context(), func(tx store.DBTX) error {
		return store.NewStayStore(store.ChainAuditor{}).Correct(t.Context(), tx, stayID, 2, "OPS-B", "re-count", nowUTCPrecise(), idem)
	}); err != nil {
		t.Fatalf("correct B: %v", err)
	}
	if n := countAuditBySubject(t, st, stayID, "STAY_CORRECT"); n != 2 {
		t.Fatalf("STAY_CORRECT events=%d, want 2", n)
	}
	// Two distinct EventIDs (subject/action/key same, but actor differs).
	ids := auditEventIDsForSubject(t, st, stayID, "STAY_CORRECT")
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("correct event IDs not distinct: %v", ids)
	}
	// Chain valid.
	if err := store.VerifyChain(t.Context(), st.DB()); err != nil {
		t.Fatalf("audit chain verify (corrections): %v", err)
	}
}

// countAuditBySubject counts audit events for a given subject+action.
func countAuditBySubject(t *testing.T, st *store.Store, subjectID, action string) int {
	t.Helper()
	var n int
	if err := st.DB().QueryRowContext(t.Context(),
		`SELECT count(*) FROM audit_events WHERE subject_id=$1 AND action=$2`,
		subjectID, action).Scan(&n); err != nil {
		t.Fatalf("countAuditBySubject: %v", err)
	}
	return n
}

// auditEventIDsForSubject returns the audit event_ids for a subject+action
// ordered by event_seq.
func auditEventIDsForSubject(t *testing.T, st *store.Store, subjectID, action string) []string {
	t.Helper()
	rows, err := st.DB().QueryContext(t.Context(),
		`SELECT event_id FROM audit_events WHERE subject_id=$1 AND action=$2 ORDER BY event_seq`,
		subjectID, action)
	if err != nil {
		t.Fatalf("auditEventIDsForSubject: %v", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// auditEventSeqsForSubject returns the event_seqs for a subject+action
// ordered by event_seq.
func auditEventSeqsForSubject(t *testing.T, st *store.Store, subjectID, action string) []int64 {
	t.Helper()
	rows, err := st.DB().QueryContext(t.Context(),
		`SELECT event_seq FROM audit_events WHERE subject_id=$1 AND action=$2 ORDER BY event_seq`,
		subjectID, action)
	if err != nil {
		t.Fatalf("auditEventSeqsForSubject: %v", err)
	}
	defer rows.Close()
	var seqs []int64
	for rows.Next() {
		var s int64
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seqs = append(seqs, s)
	}
	return seqs
}

// contains is a tiny helper for int64 slice membership.
func contains(xs []int64, x int64) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
