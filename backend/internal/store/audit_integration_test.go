package store

// P4 audit-identity verification: each distinct committed operation (each
// distinct idempotency key for the same subject + action) must produce its
// own audit event. Retries of the SAME idempotency key produce NO duplicate
// event or capacity change. The chain (prev_hash -> event_hash) remains
// valid across multiple legitimate operations and across retries.

import (
	"context"
	"testing"
)

// countAudit returns the number of audit events for a given subject+action.
func countAudit(t *testing.T, s *Store, subjectID, action string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM audit_events WHERE subject_id=$1 AND action=$2`,
		subjectID, action).Scan(&n); err != nil {
		t.Fatalf("countAudit: %v", err)
	}
	return n
}

// TestTwoExtensionsDifferentKeysBothSucceed: a single stay is extended
// twice with two different idempotency keys. Both succeed; both produce
// distinct audit events (different EventIDs because the idem keys differ).
func TestTwoExtensionsDifferentKeysBothSucceed(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	pkgID, facID, srcID, _ := seedOperationalPackage(t, s, 5, day(2), day(8))
	_ = srcID
	sessID := seedSession(t, s)

	st := mkStay(sessID, facID, pkgID, 1, day(2), day(4))
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Reserve(ctx, tx, st, nowUTC(), "reserve-1")
	}); err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	// First extension: day(4) -> day(6).
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Extend(ctx, tx, st.StayID, day(6), nowUTC(), "extend-A")
	}); err != nil {
		t.Fatalf("Extend A: %v", err)
	}
	// Second extension: day(6) -> day(8). Different idem key.
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Extend(ctx, tx, st.StayID, day(8), nowUTC(), "extend-B")
	}); err != nil {
		t.Fatalf("Extend B: %v", err)
	}

	// Two distinct STAY_EXTEND audit events.
	if n := countAudit(t, s, st.StayID, "STAY_EXTEND"); n != 2 {
		t.Fatalf("STAY_EXTEND events=%d, want 2", n)
	}

	// And the EventIDs differ (different idem keys -> different ids).
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id FROM audit_events WHERE subject_id=$1 AND action='STAY_EXTEND' ORDER BY occurred_at`,
		st.StayID)
	if err != nil {
		t.Fatalf("query extend events: %v", err)
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
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("extend event IDs not distinct: %v", ids)
	}
}

// TestTwoCorrectionsDifferentKeysBothSucceed: two downward corrections with
// different idem keys both produce distinct audit events and both succeed.
func TestTwoCorrectionsDifferentKeysBothSucceed(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	pkgID, facID, srcID, _ := seedOperationalPackage(t, s, 5, day(2), day(8))
	_ = srcID
	sessID := seedSession(t, s)

	st := mkStay(sessID, facID, pkgID, 4, day(2), day(4))
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Reserve(ctx, tx, st, nowUTC(), "reserve-corr")
	}); err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	// First correction: 4 -> 3.
	if err := s.InTx(ctx, func(tx DBTX) error {
		return stays.Correct(ctx, tx, st.StayID, 3, sessID, "less-than-expected", nowUTC(), "correct-A")
	}); err != nil {
		t.Fatalf("Correct A: %v", err)
	}
	// Second correction: 3 -> 2. Different idem key.
	if err := s.InTx(ctx, func(tx DBTX) error {
		return stays.Correct(ctx, tx, st.StayID, 2, sessID, "re-counted", nowUTC(), "correct-B")
	}); err != nil {
		t.Fatalf("Correct B: %v", err)
	}

	if n := countAudit(t, s, st.StayID, "STAY_CORRECT"); n != 2 {
		t.Fatalf("STAY_CORRECT events=%d, want 2", n)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id FROM audit_events WHERE subject_id=$1 AND action='STAY_CORRECT' ORDER BY occurred_at`,
		st.StayID)
	if err != nil {
		t.Fatalf("query correct events: %v", err)
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
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("correct event IDs not distinct: %v", ids)
	}
}

// TestRetrySameKeyNoDuplicateAuditOrCapacity: replaying an EXTEND with the
// same idempotency key produces NO new audit event and NO capacity change.
func TestRetrySameKeyNoDuplicateAuditOrCapacity(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	pkgID, facID, _, _ := seedOperationalPackage(t, s, 5, day(2), day(8))
	sessID := seedSession(t, s)

	st := mkStay(sessID, facID, pkgID, 1, day(2), day(4))
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Reserve(ctx, tx, st, nowUTC(), "reserve-retry")
	}); err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	// First extend.
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Extend(ctx, tx, st.StayID, day(6), nowUTC(), "extend-retry")
	}); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if n := countAudit(t, s, st.StayID, "STAY_EXTEND"); n != 1 {
		t.Fatalf("after first extend: events=%d, want 1", n)
	}
	heldBefore, _, _ := buckets(t, s, facID, day(5))
	if heldBefore != 1 {
		t.Fatalf("held on day(5)=%d, want 1", heldBefore)
	}

	// Retry the SAME idem key — must not change anything.
	// (The HTTP layer short-circuits via idempotency, but at the store level
	// Extend with the same call must still not silently double-apply if called
	// directly. We expect either a no-op or a no-dup-audit outcome.)
	_ = s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		_ = stays.Extend(ctx, tx, st.StayID, day(6), nowUTC(), "extend-retry")
		return nil
	})
	// Audit events unchanged.
	if n := countAudit(t, s, st.StayID, "STAY_EXTEND"); n != 1 {
		t.Fatalf("after retry: events=%d, want 1 (no duplicate)", n)
	}
	heldAfter, _, _ := buckets(t, s, facID, day(5))
	if heldAfter != heldBefore {
		t.Fatalf("held on day(5) changed: before=%d after=%d", heldBefore, heldAfter)
	}
}

// TestAuditChainRemainsValid: the prev_hash -> event_hash chain is intact
// after multiple distinct operations on the same stay.
func TestAuditChainRemainsValid(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	pkgID, facID, _, _ := seedOperationalPackage(t, s, 5, day(2), day(8))
	sessID := seedSession(t, s)

	st := mkStay(sessID, facID, pkgID, 2, day(2), day(4))
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Reserve(ctx, tx, st, nowUTC(), "chain-reserve")
	}); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := s.InTx(ctx, func(tx DBTX) error {
		_, _, rerr := RevalidateReservationContext(ctx, tx, facID, pkgID, nil, nowUTC())
		if rerr != nil {
			return rerr
		}
		return stays.Extend(ctx, tx, st.StayID, day(6), nowUTC(), "chain-extend")
	}); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if err := s.InTx(ctx, func(tx DBTX) error {
		return stays.Correct(ctx, tx, st.StayID, 1, sessID, "chain-correct", nowUTC(), "chain-correct-key")
	}); err != nil {
		t.Fatalf("Correct: %v", err)
	}

	// Walk the chain: each event's prev_hash equals the previous event's event_hash.
	rows, err := s.db.QueryContext(ctx, `
		SELECT prev_hash, event_hash FROM audit_events
		WHERE subject_id = $1
		ORDER BY occurred_at, event_id`, st.StayID)
	if err != nil {
		t.Fatalf("query chain: %v", err)
	}
	defer rows.Close()
	var chain []struct{ prev, hash string }
	for rows.Next() {
		var p, h string
		if err := rows.Scan(&p, &h); err != nil {
			t.Fatalf("scan chain: %v", err)
		}
		chain = append(chain, struct{ prev, hash string }{p, h})
	}
	if len(chain) < 3 {
		t.Fatalf("chain length=%d, want >= 3 (reserve, extend, correct)", len(chain))
	}
	for i := 1; i < len(chain); i++ {
		if chain[i].prev != chain[i-1].hash {
			t.Fatalf("chain[%d].prev=%q != chain[%d].hash=%q", i, chain[i].prev, i-1, chain[i-1].hash)
		}
	}
}

// TestAuditEventIDIsDistinctAcrossKeys: directly assert that the
// auditEventID helper produces different IDs for (same subject, same action,
// different keys) and same IDs for (same subject, same action, same key).
func TestAuditEventIDIsDistinctAcrossKeys(t *testing.T) {
	a := auditEventID("S1", "STAY_EXTEND", "key-A")
	b := auditEventID("S1", "STAY_EXTEND", "key-B")
	if a == b {
		t.Fatalf("auditEventID: a=%q == b=%q with different keys", a, b)
	}
	a2 := auditEventID("S1", "STAY_EXTEND", "key-A")
	if a != a2 {
		t.Fatalf("auditEventID not stable: a=%q a2=%q", a, a2)
	}
	// Different subjects with same key/action are distinct.
	c := auditEventID("S2", "STAY_EXTEND", "key-A")
	if c == a {
		t.Fatalf("auditEventID: c=%q == a=%q across subjects", c, a)
	}
	// Empty key falls back to subject+action only (legacy compatibility).
	noKey1 := auditEventID("S1", "STAY_EXTEND", "")
	noKey2 := auditEventID("S1", "STAY_EXTEND", "")
	if noKey1 != noKey2 {
		t.Fatalf("auditEventID empty key not stable: %q vs %q", noKey1, noKey2)
	}
}
