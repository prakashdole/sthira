package store

// P4 stay-lifecycle verification against a real PostgreSQL database. Gated on
// STHIRA_TEST_DSN; skipped (not passed) when unset. Covers capacity
// conservation across every transition, concurrent last-space, expiry/arrival
// race, failed transfer retaining the original, overlapping intervals, and
// multi-date deterministic locking.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// seedPackageFacility inserts a source, package, facility and per-date
// inventory with the given capacity for the half-open range [start, end).
// Returns (packageID, facilityID).
func seedPackageFacility(t *testing.T, s *Store, capacity int, start, end time.Time) (string, string) {
	t.Helper()
	ctx := context.Background()
	srcID := insertTestSource(t, s, NewSourceStore(ChainAuditor{}))
	pkgID := uid("PKG")
	artID := uid("ART")
	facID := uid("FAC")
	now := nowUTC()
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	err := s.InTx(ctx, func(tx DBTX) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256,
				retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,1,$3,$4,'SYNTHETIC','mem://test')`, artID, srcID, hash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version,
				jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			VALUES ($1,'ALT',$2,$3,1,'JTEST','SYNTHETIC',$4,$5,$6,'{}')`,
			pkgID, srcID, artID, now, now.Add(24*time.Hour), hash); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1,$2,'SZ','Asia/Kolkata',1,$3)`, facID, pkgID, now); err != nil {
			return err
		}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(ctx, `
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
	return pkgID, facID
}

// seedSession inserts a live citizen session and returns its id.
func seedSession(t *testing.T, s *Store) string {
	t.Helper()
	id := uid("SES")
	_, err := IssueCitizenSession(context.Background(), s.db, id, time.Hour, nowUTC())
	if err != nil {
		t.Fatalf("IssueCitizenSession: %v", err)
	}
	return id
}

// buckets reads (held, occupied) for a facility+date.
func buckets(t *testing.T, s *Store, facID string, d time.Time) (held, occupied, capacity int) {
	t.Helper()
	err := s.db.QueryRowContext(context.Background(),
		`SELECT held, occupied, capacity FROM facility_inventory WHERE facility_id=$1 AND service_date=$2`,
		facID, d).Scan(&held, &occupied, &capacity)
	if err != nil {
		t.Fatalf("buckets: %v", err)
	}
	return held, occupied, capacity
}

// assertConservation verifies held+occupied <= capacity on every date in range.
func assertConservation(t *testing.T, s *Store, facID string, start, end time.Time) {
	t.Helper()
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		h, o, c := buckets(t, s, facID, d)
		if h < 0 || o < 0 || h+o > c {
			t.Fatalf("conservation violated on %s: held=%d occupied=%d capacity=%d", d.Format("2006-01-02"), h, o, c)
		}
	}
}

func mkStay(sessID, facID, pkgID string, party int, start, end time.Time) Stay {
	return Stay{
		StayID: uid("STAY"), ReservationID: uid("RES"), SessionID: sessID,
		FacilityID: facID, PartySize: party, StartDate: start, EndDate: end, PackageID: pkgID,
	}
}

// seedReservation inserts the reservation row a stay references.
func seedReservation(t *testing.T, s *Store, st Stay) {
	t.Helper()
	_, err := s.db.ExecContext(context.Background(), `
		INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, updated_at)
		VALUES ($1,$2,$3,$4,$5,'RESERVED',1,$6)`,
		st.ReservationID, st.SessionID, st.FacilityID, st.StartDate, st.PartySize, nowUTC())
	if err != nil {
		t.Fatalf("seedReservation: %v", err)
	}
}

func day(n int) time.Time { // fixed dates far in the future to avoid expiry interplay
	return time.Date(2030, 1, n, 0, 0, 0, 0, time.UTC)
}

// TestStayLifecycleConservation: reserve -> arrive -> depart conserves capacity
// on every date, with arrival converting held->occupied (no second decrement).
func TestStayLifecycleConservation(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	start, end := day(10), day(13) // 3 dates
	pkgID, facID := seedPackageFacility(t, s, 5, start, end)
	sessID := seedSession(t, s)

	st := mkStay(sessID, facID, pkgID, 2, start, end)
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, st, nowUTC()) }); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	h, o, c := buckets(t, s, facID, start)
	if h != 2 || o != 0 || c != 5 {
		t.Fatalf("after reserve: held=%d occupied=%d capacity=%d, want 2/0/5", h, o, c)
	}

	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Arrive(ctx, tx, st.StayID, nowUTC()) }); err != nil {
		t.Fatalf("Arrive: %v", err)
	}
	h, o, c = buckets(t, s, facID, start)
	if h != 0 || o != 2 || c != 5 {
		t.Fatalf("after arrive: held=%d occupied=%d, want 0/2 (no second decrement)", h, o)
	}

	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Depart(ctx, tx, st.StayID, nowUTC()) }); err != nil {
		t.Fatalf("Depart: %v", err)
	}
	h, o, c = buckets(t, s, facID, start)
	if h != 0 || o != 0 || c != 5 {
		t.Fatalf("after depart: held=%d occupied=%d, want 0/0", h, o)
	}
	assertConservation(t, s, facID, start, end)
}

// TestConcurrentLastSpaceStay: N concurrent reserves for one remaining space;
// exactly one wins, conservation holds.
func TestConcurrentLastSpaceStay(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	start, end := day(20), day(21)
	pkgID, facID := seedPackageFacility(t, s, 1, start, end)

	const n = 8
	var wg sync.WaitGroup
	wins := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessID := seedSession(t, s)
			st := mkStay(sessID, facID, pkgID, 1, start, end)
			seedReservation(t, s, st)
			err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, st, nowUTC()) })
			wins <- err
		}()
	}
	wg.Wait()
	close(wins)
	succeeded := 0
	for err := range wins {
		if err == nil {
			succeeded++
		} else if !errors.Is(err, ErrCapacityExhausted) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent last space: %d succeeded, want exactly 1", succeeded)
	}
	h, o, c := buckets(t, s, facID, start)
	if h+o > c {
		t.Fatalf("over-reserved: held=%d occupied=%d capacity=%d", h, o, c)
	}
}

// TestExpiryArrivalRace: a RESERVED hold past expiry cannot arrive; the expiry
// worker frees it. Expire then Arrive -> Arrive fails; conservation holds.
func TestExpiryArrivalRace(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	start, end := day(30), day(31)
	pkgID, facID := seedPackageFacility(t, s, 2, start, end)
	sessID := seedSession(t, s)

	st := mkStay(sessID, facID, pkgID, 1, start, end)
	past := nowUTC().Add(-time.Minute)
	st.ExpiresAt = &past
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, st, nowUTC()) }); err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	// Expiry wins.
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Expire(ctx, tx, st.StayID, nowUTC()) }); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	// Arrival after expiry must fail.
	err := s.InTx(ctx, func(tx DBTX) error { return stays.Arrive(ctx, tx, st.StayID, nowUTC()) })
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("arrive after expire: got %v, want ErrInvalidTransition", err)
	}
	h, o, _ := buckets(t, s, facID, start)
	if h != 0 || o != 0 {
		t.Fatalf("after expire: held=%d occupied=%d, want 0/0 (freed)", h, o)
	}
}

// TestExpiryWorker: the running worker expires a due hold against the DB.
func TestExpiryWorker(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	start, end := day(5), day(6)
	pkgID, facID := seedPackageFacility(t, s, 2, start, end)
	sessID := seedSession(t, s)

	st := mkStay(sessID, facID, pkgID, 1, start, end)
	past := nowUTC().Add(-time.Minute)
	st.ExpiresAt = &past
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, st, nowUTC()) }); err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	w := NewExpiryWorker(s, stays)
	n, err := w.Tick(ctx)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if n != 1 {
		t.Fatalf("worker expired %d, want 1", n)
	}
	var state string
	if err := s.db.QueryRowContext(ctx, `SELECT state FROM stays WHERE stay_id=$1`, st.StayID).Scan(&state); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != string(StayExpired) {
		t.Fatalf("state=%s, want EXPIRED", state)
	}
}

// TestFailedTransferRetainsOriginal: transfer to a full facility fails and the
// original stay is unchanged (still RESERVED, capacity still held).
func TestFailedTransferRetainsOriginal(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	start, end := day(15), day(16)
	pkgID, facID := seedPackageFacility(t, s, 2, start, end)
	// Full target facility (capacity 1, already occupied by another stay).
	_, fullFac := seedPackageFacility(t, s, 1, start, end)
	sessID := seedSession(t, s)

	// Fill the target.
	fill := mkStay(sessID, fullFac, pkgID, 1, start, end)
	seedReservation(t, s, fill)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, fill, nowUTC()) }); err != nil {
		t.Fatalf("fill target: %v", err)
	}

	// Original stay on facID.
	orig := mkStay(sessID, facID, pkgID, 2, start, end)
	seedReservation(t, s, orig)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, orig, nowUTC()) }); err != nil {
		t.Fatalf("Reserve orig: %v", err)
	}

	// Transfer to the full facility must fail.
	err := s.InTx(ctx, func(tx DBTX) error {
		return stays.Transfer(ctx, tx, orig.StayID, uid("STAY"), uid("RES"), fullFac, nowUTC())
	})
	if !errors.Is(err, ErrCapacityExhausted) {
		t.Fatalf("transfer to full: got %v, want ErrCapacityExhausted", err)
	}
	// Original retained: still RESERVED, capacity still held on facID.
	var state string
	if err := s.db.QueryRowContext(ctx, `SELECT state FROM stays WHERE stay_id=$1`, orig.StayID).Scan(&state); err != nil {
		t.Fatalf("read orig: %v", err)
	}
	if state != string(StayReserved) {
		t.Fatalf("orig state=%s, want RESERVED (retained)", state)
	}
	h, _, _ := buckets(t, s, facID, start)
	if h != 2 {
		t.Fatalf("orig held=%d, want 2 (still held)", h)
	}
}

// TestSuccessfulTransfer: transfer to an available facility moves the hold.
func TestSuccessfulTransfer(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	start, end := day(25), day(26)
	pkgID, facA := seedPackageFacility(t, s, 2, start, end)
	_, facB := seedPackageFacility(t, s, 3, start, end)
	sessID := seedSession(t, s)

	orig := mkStay(sessID, facA, pkgID, 1, start, end)
	seedReservation(t, s, orig)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, orig, nowUTC()) }); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	newStay := uid("STAY")
	if err := s.InTx(ctx, func(tx DBTX) error {
		return stays.Transfer(ctx, tx, orig.StayID, newStay, uid("RES"), facB, nowUTC())
	}); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	// Old freed, new held.
	ha, _, _ := buckets(t, s, facA, start)
	hb, _, _ := buckets(t, s, facB, start)
	if ha != 0 {
		t.Fatalf("origin held=%d, want 0 (freed)", ha)
	}
	if hb != 1 {
		t.Fatalf("dest held=%d, want 1", hb)
	}
	var state string
	if err := s.db.QueryRowContext(ctx, `SELECT state FROM stays WHERE stay_id=$1`, orig.StayID).Scan(&state); err != nil {
		t.Fatalf("read orig: %v", err)
	}
	if state != string(StayDeparted) {
		t.Fatalf("orig state=%s, want DEPARTED", state)
	}
	assertConservation(t, s, facA, start, end)
	assertConservation(t, s, facB, start, end)
}

// TestOverlappingIntervals: two stays sharing only some dates conserve capacity
// independently per date.
func TestOverlappingIntervals(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	// Facility capacity 3 across days 1..4.
	pkgID, facID := seedPackageFacility(t, s, 3, day(1), day(5))
	sessID := seedSession(t, s)

	// Stay A: days 1-2 (party 2). Stay B: days 2-3 (party 2). They overlap day 2.
	a := mkStay(sessID, facID, pkgID, 2, day(1), day(3))
	seedReservation(t, s, a)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, a, nowUTC()) }); err != nil {
		t.Fatalf("Reserve A: %v", err)
	}
	b := mkStay(sessID, facID, pkgID, 2, day(2), day(4))
	seedReservation(t, s, b)
	err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, b, nowUTC()) })
	if !errors.Is(err, ErrCapacityExhausted) {
		t.Fatalf("overlapping over-capacity reserve: got %v, want ErrCapacityExhausted (day2: 2+2>3)", err)
	}
	// Day 1 held by A only (2); day 2 still held by A only (B rolled back).
	h1, _, _ := buckets(t, s, facID, day(1))
	h2, _, _ := buckets(t, s, facID, day(2))
	if h1 != 2 || h2 != 2 {
		t.Fatalf("after failed overlap: day1 held=%d day2 held=%d, want 2/2 (B rolled back)", h1, h2)
	}
	assertConservation(t, s, facID, day(1), day(5))
}

// TestMultiDateDeadlockFree: two multi-date reserves over the same dates in
// opposite creation order complete without deadlock (deterministic lock order).
func TestMultiDateDeadlockFree(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	start, end := day(8), day(12) // 4 dates
	pkgID, facID := seedPackageFacility(t, s, 4, start, end)
	sessID := seedSession(t, s)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st := mkStay(sessID, facID, pkgID, 2, start, end)
			seedReservation(t, s, st)
			errs <- s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, st, nowUTC()) })
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("multi-date reserve: %v", err)
		}
	}
	// Both fit exactly (2+2=4=capacity).
	h, o, c := buckets(t, s, facID, start)
	if h != 4 || o != 0 || c != 4 {
		t.Fatalf("after both: held=%d occupied=%d capacity=%d, want 4/0/4", h, o, c)
	}
	assertConservation(t, s, facID, start, end)
}

// TestExtendAddsOnlyNewDates: extending a stay holds only the added dates.
func TestExtendAddsOnlyNewDates(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	pkgID, facID := seedPackageFacility(t, s, 3, day(1), day(6))
	sessID := seedSession(t, s)

	st := mkStay(sessID, facID, pkgID, 1, day(1), day(3)) // days 1-2
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, st, nowUTC()) }); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Extend(ctx, tx, st.StayID, day(5), nowUTC()) }); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	// Days 1-4 each held=1 now.
	for d := day(1); d.Before(day(5)); d = d.AddDate(0, 0, 1) {
		h, _, _ := buckets(t, s, facID, d)
		if h != 1 {
			t.Fatalf("after extend %s: held=%d, want 1", d.Format("2006-01-02"), h)
		}
	}
	assertConservation(t, s, facID, day(1), day(6))
}

// TestCancelReleasesHold: cancel returns held to free.
func TestCancelReleasesHold(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	stays := NewStayStore(ChainAuditor{})
	start, end := day(18), day(19)
	pkgID, facID := seedPackageFacility(t, s, 2, start, end)
	sessID := seedSession(t, s)
	st := mkStay(sessID, facID, pkgID, 2, start, end)
	seedReservation(t, s, st)
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Reserve(ctx, tx, st, nowUTC()) }); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := s.InTx(ctx, func(tx DBTX) error { return stays.Cancel(ctx, tx, st.StayID, nowUTC()) }); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	h, o, _ := buckets(t, s, facID, start)
	if h != 0 || o != 0 {
		t.Fatalf("after cancel: held=%d occupied=%d, want 0/0", h, o)
	}
	// Double cancel must fail.
	err := s.InTx(ctx, func(tx DBTX) error { return stays.Cancel(ctx, tx, st.StayID, nowUTC()) })
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("double cancel: got %v, want ErrInvalidTransition", err)
	}
}
