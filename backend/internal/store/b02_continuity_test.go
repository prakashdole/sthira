package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestB02_LegacyAndCanonicalIdempotencyCoexist tests that canonical JSON
// payload hashes and legacy pipe-delimited payload hashes coexist in the
// idempotency_keys table without collision or aliasing.
func TestB02_LegacyAndCanonicalIdempotencyCoexist(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	ctx := context.Background()
	now := nowUTC()

	is := IdempotencyStore{}
	sess1 := "SESS-CANONICAL-" + uid("X")
	sess2 := "SESS-LEGACY-" + uid("X")
	key := "IDEM-" + uid("X")

	canonicalHash := hex.EncodeToString(func() []byte {
		h := sha256.Sum256([]byte(`{"session_id":"` + sess1 + `","party_size":2}`))
		return h[:]
	}())
	legacyHash := hex.EncodeToString(func() []byte {
		h := sha256.Sum256([]byte(sess2 + "|FAC-1|PKG-1|2026-09-24|2026-09-25|" + key))
		return h[:]
	}())

	// Insert canonical row
	_, rep1, err := is.Begin(ctx, fx.store.DB(), sess1, "reservation.create", key, canonicalHash, now.Add(time.Hour))
	if err != nil || rep1 {
		t.Fatalf("insert canonical: err=%v rep=%v", err, rep1)
	}
	if err := is.Complete(ctx, fx.store.DB(), sess1, "reservation.create", key, map[string]string{"reservation_id": "RES-1"}); err != nil {
		t.Fatalf("complete canonical: %v", err)
	}

	// Insert legacy row under separate session with legacy hash format
	_, rep2, err := is.Begin(ctx, fx.store.DB(), sess2, "reservation.create", key, legacyHash, now.Add(time.Hour))
	if err != nil || rep2 {
		t.Fatalf("insert legacy: err=%v rep=%v", err, rep2)
	}
	if err := is.Complete(ctx, fx.store.DB(), sess2, "reservation.create", key, map[string]string{"reservation_id": "RES-2"}); err != nil {
		t.Fatalf("complete legacy: %v", err)
	}

	// Verify both coexist and replay their respective results
	res1, rep1, err := is.Begin(ctx, fx.store.DB(), sess1, "reservation.create", key, canonicalHash, now.Add(time.Hour))
	if err != nil || !rep1 {
		t.Fatalf("replay canonical: err=%v rep=%v", err, rep1)
	}
	var out1 map[string]string
	_ = json.Unmarshal(res1, &out1)
	if out1["reservation_id"] != "RES-1" {
		t.Errorf("canonical replay mismatch: got %v want RES-1", out1)
	}

	res2, rep2, err := is.Begin(ctx, fx.store.DB(), sess2, "reservation.create", key, legacyHash, now.Add(time.Hour))
	if err != nil || !rep2 {
		t.Fatalf("replay legacy: err=%v rep=%v", err, rep2)
	}
	var out2 map[string]string
	_ = json.Unmarshal(res2, &out2)
	if out2["reservation_id"] != "RES-2" {
		t.Errorf("legacy replay mismatch: got %v want RES-2", out2)
	}
}

// TestB02_LegacyReplayPreservesSemanticsAndPreventsAliasing verifies that
// legacy replay works for the authentic actor and original semantics, but
// strictly rejects party size or facility aliasing attempts with ErrPayloadConflict.
func TestB02_LegacyReplayPreservesSemanticsAndPreventsAliasing(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	ctx := context.Background()
	now := nowUTC()

	is := IdempotencyStore{}
	sessID := "SESS-COMPAT-" + uid("X")
	key := "IDEM-" + uid("X")
	facID := fx.facilityID
	pkgID := fx.packageID
	routeID := fx.routeID
	resID := "RES-" + uid("X")
	stayID := "STAY-" + uid("X")
	startDate := "2026-10-01"
	endDate := "2026-10-05"
	originalPartySize := 2

	// Seed session and actual reservation in the database with originalPartySize
	startT, _ := time.Parse("2006-01-02", startDate)
	expT := now.Add(48 * time.Hour)
	if _, err := fx.store.DB().ExecContext(ctx, `
		INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, expires_at)
		VALUES ($1, 'CITIZEN', $2, 'cred-test', $3)`,
		sessID, fx.jurisdictionID, expT); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	if _, err := fx.store.DB().ExecContext(ctx, `
		INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'RESERVED', 1, $6, $6, $7)`,
		resID, sessID, facID, startT, originalPartySize, now, expT); err != nil {
		t.Fatalf("seed reservation: %v", err)
	}

	// Legacy hash format (omitted party_size and snapshot_version)
	legacyHash := hex.EncodeToString(func() []byte {
		h := sha256.Sum256([]byte(sessID + "|" + facID + "|" + pkgID + "|" + routeID + "|" + startDate + "|" + endDate + "|" + key))
		return h[:]
	}())

	// Store legacy completed idempotency record
	if _, err := fx.store.DB().ExecContext(ctx, `
		INSERT INTO idempotency_keys (scope, operation, idem_key, payload_hash, state, result, expires_at)
		VALUES ($1, 'reservation.create', $2, $3, 'COMPLETED', $4, $5)`,
		sessID, key, legacyHash, []byte(fmt.Sprintf(`{"reservation_id":"%s","stay_id":"%s"}`, resID, stayID)), expT); err != nil {
		t.Fatalf("seed legacy idempotency: %v", err)
	}

	// Modern canonical hash for identical semantics
	canonicalHashIdentical := hex.EncodeToString(func() []byte {
		h := sha256.Sum256([]byte(`{"session_id":"` + sessID + `","party_size":2}`))
		return h[:]
	}())

	// Compatibility checker function matching stay_handlers logic
	compatChecker := func(attemptedPartySize int, attemptedFacID string) LegacyChecker {
		return func(c context.Context, db DBTX, existingHash string, storedResult []byte) (bool, error) {
			if existingHash != legacyHash {
				return false, nil
			}
			var out map[string]string
			if err := json.Unmarshal(storedResult, &out); err != nil {
				return false, ErrPayloadConflict
			}
			rID := out["reservation_id"]
			if rID == "" {
				return false, ErrPayloadConflict
			}
			var dbSess, dbFac string
			var dbParty int
			err := db.QueryRowContext(c, `
				SELECT session_id, facility_id, party_size
				FROM reservations WHERE reservation_id = $1`, rID).Scan(&dbSess, &dbFac, &dbParty)
			if err != nil {
				return false, ErrPayloadConflict
			}
			if dbSess != sessID || dbFac != attemptedFacID || dbParty != attemptedPartySize {
				return false, ErrPayloadConflict
			}
			return true, nil
		}
	}

	// 1. Replay with identical semantics (party_size=2, facility=facID) -> MUST SUCCEED
	res, rep, err := is.BeginWithCompat(ctx, fx.store.DB(), sessID, "reservation.create", key, canonicalHashIdentical, compatChecker(originalPartySize, facID), expT)
	if err != nil || !rep {
		t.Fatalf("expected successful legacy replay: err=%v, rep=%v", err, rep)
	}
	var out map[string]string
	_ = json.Unmarshal(res, &out)
	if out["reservation_id"] != resID || out["stay_id"] != stayID {
		t.Fatalf("unexpected replay result: %v", out)
	}

	// 2. Replay with aliasing attempt (party_size=5 under same key) -> MUST REJECT WITH CONFLICT
	canonicalHashAliasedParty := hex.EncodeToString(func() []byte {
		h := sha256.Sum256([]byte(`{"session_id":"` + sessID + `","party_size":5}`))
		return h[:]
	}())
	_, _, err = is.BeginWithCompat(ctx, fx.store.DB(), sessID, "reservation.create", key, canonicalHashAliasedParty, compatChecker(5, facID), expT)
	if err != ErrPayloadConflict {
		t.Fatalf("expected ErrPayloadConflict for party size aliasing, got %v", err)
	}

	// 3. Replay with different facility -> MUST REJECT WITH CONFLICT
	canonicalHashAliasedFac := hex.EncodeToString(func() []byte {
		h := sha256.Sum256([]byte(`{"session_id":"` + sessID + `","party_size":2,"fac":"OTHER"}`))
		return h[:]
	}())
	_, _, err = is.BeginWithCompat(ctx, fx.store.DB(), sessID, "reservation.create", key, canonicalHashAliasedFac, compatChecker(originalPartySize, "FAC-OTHER"), expT)
	if err != ErrPayloadConflict {
		t.Fatalf("expected ErrPayloadConflict for facility mismatch, got %v", err)
	}
}

// TestB02_LastSpaceCrashAfterCommit verifies that when 1 capacity unit remains:
// - A reservation commits and decrements capacity to 0.
// - Replay after a simulated client crash returns replay=true without double allocation.
// - A competing reservation for another session is denied (no overbooking / capacity never negative).
func TestB02_LastSpaceCrashAfterCommit(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	ctx := context.Background()
	now := nowUTC()

	servDate, _ := time.Parse("2006-01-02", "2026-10-10")
	expT := now.Add(time.Hour)
	startDate, endDate := servDate, servDate.Add(24*time.Hour)

	is := IdempotencyStore{}
	stays := NewStayStore(ChainAuditor{})
	sess1 := "SESS-WINNER-" + uid("X")
	sess2 := "SESS-LOSER-" + uid("X")
	key1 := "IDEM-1-" + uid("X")
	key2 := "IDEM-2-" + uid("X")
	resID1 := "RES-1-" + uid("X")
	stayID1 := "STAY-1-" + uid("X")
	hash1 := hex.EncodeToString(func() []byte { h := sha256.Sum256([]byte(key1)); return h[:] }())
	hash2 := hex.EncodeToString(func() []byte { h := sha256.Sum256([]byte(key2)); return h[:] }())

	// Seed sessions
	if _, err := fx.store.DB().ExecContext(ctx, `
		INSERT INTO sessions (session_id, principal_kind, jurisdiction, credential_ref, expires_at)
		VALUES ($1, 'CITIZEN', $3, 'cred-1', $4),
		       ($2, 'CITIZEN', $3, 'cred-2', $4)`,
		sess1, sess2, fx.jurisdictionID, expT); err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	// Seed facility row
	if _, err := fx.store.DB().ExecContext(ctx, `
		INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
		VALUES ($1, $2, $3, 'Asia/Kolkata', 1, $4)
		ON CONFLICT (facility_id) DO NOTHING`,
		fx.facilityID, fx.packageID, fx.safeZoneID, now); err != nil {
		t.Fatalf("seed facility: %v", err)
	}

	// Seed inventory with capacity = 1
	if _, err := fx.store.DB().ExecContext(ctx, `
		INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
		VALUES ($1, $2, 1, 0, 0, 0, 1, $3)
		ON CONFLICT (facility_id, service_date) DO UPDATE
		SET capacity = 1, reserved = 0, held = 0, occupied = 0`,
		fx.facilityID, servDate, now); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	// Session 1 commits the reservation
	err := fx.store.InTx(ctx, func(tx DBTX) error {
		_, rep, err := is.Begin(ctx, tx, sess1, "reservation.create", key1, hash1, expT)
		if err != nil || rep {
			return fmt.Errorf("begin err=%v rep=%v", err, rep)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at, expires_at)
			VALUES ($1, $2, $3, $4, 1, 'RESERVED', 1, $5, $5, $6)`,
			resID1, sess1, fx.facilityID, servDate, now, expT); err != nil {
			return err
		}
		if err := stays.Reserve(ctx, tx, Stay{
			StayID: stayID1, ReservationID: resID1, SessionID: sess1,
			FacilityID: fx.facilityID, PartySize: 1,
			StartDate: startDate, EndDate: endDate, PackageID: fx.packageID, RouteID: &fx.routeID,
			ExpiresAt: &expT,
		}, now, key1); err != nil {
			return err
		}
		return is.Complete(ctx, tx, sess1, "reservation.create", key1, map[string]string{
			"reservation_id": resID1, "stay_id": stayID1,
		})
	})
	if err != nil {
		t.Fatalf("session 1 reservation failed: %v", err)
	}

	// Verify inventory remaining = 0 (held=1, capacity=1)
	var held, capVal int
	if err := fx.store.DB().QueryRowContext(ctx, `
		SELECT held, capacity
		FROM facility_inventory WHERE facility_id = $1 AND service_date = $2`,
		fx.facilityID, servDate).Scan(&held, &capVal); err != nil {
		t.Fatalf("query inventory: %v", err)
	}
	if held != 1 || capVal != 1 {
		t.Fatalf("inventory state mismatch: held=%d cap=%d", held, capVal)
	}

	// Simulate crash-after-commit: client replays with same key -> MUST RETURN STORED RESULT
	var replayResult []byte
	var replayed bool
	err = fx.store.InTx(ctx, func(tx DBTX) error {
		res, rep, err := is.Begin(ctx, tx, sess1, "reservation.create", key1, hash1, expT)
		if err != nil {
			return err
		}
		replayResult, replayed = res, rep
		return nil
	})
	if err != nil || !replayed {
		t.Fatalf("crash replay failed: err=%v replayed=%v", err, replayed)
	}
	var repOut map[string]string
	_ = json.Unmarshal(replayResult, &repOut)
	if repOut["reservation_id"] != resID1 {
		t.Fatalf("replay returned wrong reservation ID: %v", repOut)
	}

	// Verify capacity was NOT decremented again on replay (still held=1, cap=1)
	if err := fx.store.DB().QueryRowContext(ctx, `
		SELECT held, capacity
		FROM facility_inventory WHERE facility_id = $1 AND service_date = $2`,
		fx.facilityID, servDate).Scan(&held, &capVal); err != nil {
		t.Fatalf("query inventory post replay: %v", err)
	}
	if held != 1 || capVal != 1 {
		t.Fatalf("capacity leaked on replay: held=%d cap=%d", held, capVal)
	}

	// Session 2 attempts reservation for the last space -> MUST FAIL WITH CAPACITY CONFLICT
	err2 := fx.store.InTx(ctx, func(tx DBTX) error {
		_, rep, err := is.Begin(ctx, tx, sess2, "reservation.create", key2, hash2, expT)
		if err != nil || rep {
			return fmt.Errorf("begin session 2 err=%v rep=%v", err, rep)
		}
		return stays.Reserve(ctx, tx, Stay{
			StayID: "STAY-2-" + uid("X"), ReservationID: "RES-2-" + uid("X"), SessionID: sess2,
			FacilityID: fx.facilityID, PartySize: 1,
			StartDate: startDate, EndDate: endDate, PackageID: fx.packageID, RouteID: &fx.routeID,
			ExpiresAt: &expT,
		}, now, key2)
	})
	if err2 == nil {
		t.Fatalf("expected capacity conflict for session 2 on full facility, but succeeded")
	}

	// Capacity must never be negative
	var postHeld int
	_ = fx.store.DB().QueryRowContext(ctx, `
		SELECT held FROM facility_inventory WHERE facility_id = $1 AND service_date = $2`,
		fx.facilityID, servDate).Scan(&postHeld)
	if postHeld > 1 || postHeld < 0 {
		t.Fatalf("illegal capacity value: %d", postHeld)
	}
}

// TestB02_LiveAuthorityWithdrawalFlowsIntoDelivery verifies that when an authority
// is suspended/quarantined or revoked, subsequent context revalidation immediately
// fails closed, preventing stale delivery across instances.
func TestB02_LiveAuthorityWithdrawalFlowsIntoDelivery(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	ctx := context.Background()

	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(ctx, fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := resolver.SnapshotRevalidate(ctx, sc); err != nil {
		t.Fatalf("Initial SnapshotRevalidate: %v", err)
	}

	// Withdraw source authority via quarantine/suspension
	srcID := findKLP6Source(t, fx.store.DB())
	if _, err := fx.store.DB().ExecContext(ctx, `
		UPDATE sources SET state = 'SUSPENDED' WHERE source_id = $1`, srcID); err != nil {
		t.Fatalf("suspend source: %v", err)
	}

	// SnapshotRevalidate must fail immediately on the next call
	if err := resolver.SnapshotRevalidate(ctx, sc); err == nil {
		t.Fatalf("SnapshotRevalidate must fail closed when source is SUSPENDED")
	}
}
