package offlinedelivery_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"sthira/backend/internal/offlinedelivery"
	"sthira/backend/internal/offlinepkg"
)

func makeManifest(jurisdiction string, revision int, checksum string, activeCard string, revokedPkgs, cancelledRoutes []string, validUntil time.Time) *offlinepkg.Manifest {
	return &offlinepkg.Manifest{
		SchemaVersion:  "3.0",
		ManifestID:     "mnf-test-01",
		Jurisdiction:   jurisdiction,
		Revision:       revision,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		ValidUntil:     validUntil.UTC().Format(time.RFC3339),
		SourceStatus:   "CURRENT",
		ChecksumSHA256: checksum,
		CriticalCard: offlinepkg.CriticalCardDescriptor{
			PackageID: activeCard,
			Version:   1,
		},
		Revocations: offlinepkg.RevocationBlock{
			RevokedPackages: revokedPkgs,
			CancelledRoutes: cancelledRoutes,
		},
	}
}

func TestManifestRevisionTracker_MonotonicProgression(t *testing.T) {
	tracker := offlinedelivery.NewManifestRevisionTracker()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	validUntil := now.Add(24 * time.Hour)

	// 1. Commit Revision 1
	m1 := makeManifest("KL-WYD", 1, "sha-rev-1", "PKG-ACTIVE-01", []string{"PKG-REVOKED-00"}, []string{"RT-CLOSED-01"}, validUntil)
	if err := tracker.Commit(m1, now); err != nil {
		t.Fatalf("unexpected error committing rev 1: %v", err)
	}

	state, ok := tracker.GetState("KL-WYD")
	if !ok || state.Revision != 1 {
		t.Fatalf("expected state revision 1, got %+v", state)
	}

	// 2. Commit Revision 2 with proper tombstone retention
	m2 := makeManifest("KL-WYD", 2, "sha-rev-2", "PKG-ACTIVE-02", []string{"PKG-REVOKED-00", "PKG-REVOKED-01"}, []string{"RT-CLOSED-01"}, validUntil)
	if err := tracker.Commit(m2, now.Add(1*time.Hour)); err != nil {
		t.Fatalf("unexpected error committing rev 2: %v", err)
	}

	state, _ = tracker.GetState("KL-WYD")
	if state.Revision != 2 {
		t.Fatalf("expected state revision 2, got %d", state.Revision)
	}

	// 3. Rollback attempt: Revision 1 submitted when Revision 2 is active
	err := tracker.ValidateNext(m1, now.Add(2*time.Hour))
	if !errors.Is(err, offlinedelivery.ErrRevisionRollback) {
		t.Fatalf("expected ErrRevisionRollback, got: %v", err)
	}

	// 4. Identical revision resubmission with same checksum -> succeeds
	if err := tracker.ValidateNext(m2, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("expected idempotent resubmission to succeed, got: %v", err)
	}

	// 5. Identical revision resubmission with differing checksum -> rejected
	m2Tampered := makeManifest("KL-WYD", 2, "sha-tampered-2", "PKG-ACTIVE-02", []string{"PKG-REVOKED-00", "PKG-REVOKED-01"}, []string{"RT-CLOSED-01"}, validUntil)
	err = tracker.ValidateNext(m2Tampered, now.Add(2*time.Hour))
	if !errors.Is(err, offlinedelivery.ErrIdenticalRevisionModified) {
		t.Fatalf("expected ErrIdenticalRevisionModified, got: %v", err)
	}
}

func TestManifestRevisionTracker_TombstoneIntegrity(t *testing.T) {
	tracker := offlinedelivery.NewManifestRevisionTracker()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	validUntil := now.Add(24 * time.Hour)

	// Commit initial revision with revoked package PKG-TOMB-01 and cancelled route RT-TOMB-01
	m1 := makeManifest("KL-WYD", 1, "sha-1", "PKG-OK", []string{"PKG-TOMB-01"}, []string{"RT-TOMB-01"}, validUntil)
	if err := tracker.Commit(m1, now); err != nil {
		t.Fatalf("unexpected error committing rev 1: %v", err)
	}

	// Case A: Successor attempts to resurrect tombstoned package as active card
	m2Resurrect := makeManifest("KL-WYD", 2, "sha-2a", "PKG-TOMB-01", []string{"PKG-TOMB-01"}, []string{"RT-TOMB-01"}, validUntil)
	err := tracker.ValidateNext(m2Resurrect, now.Add(1*time.Hour))
	if !errors.Is(err, offlinedelivery.ErrTombstoneViolated) {
		t.Fatalf("expected ErrTombstoneViolated when active card uses revoked package, got: %v", err)
	}

	// Case B: Successor drops tombstoned cancelled route
	m2DropRoute := makeManifest("KL-WYD", 2, "sha-2b", "PKG-OK-2", []string{"PKG-TOMB-01"}, []string{}, validUntil)
	err = tracker.ValidateNext(m2DropRoute, now.Add(1*time.Hour))
	if !errors.Is(err, offlinedelivery.ErrTombstoneDropped) {
		t.Fatalf("expected ErrTombstoneDropped when cancelled route is dropped, got: %v", err)
	}

	// Case C: Successor drops tombstoned revoked package
	m2DropPkg := makeManifest("KL-WYD", 2, "sha-2c", "PKG-OK-2", []string{}, []string{"RT-TOMB-01"}, validUntil)
	err = tracker.ValidateNext(m2DropPkg, now.Add(1*time.Hour))
	if !errors.Is(err, offlinedelivery.ErrTombstoneDropped) {
		t.Fatalf("expected ErrTombstoneDropped when revoked package is dropped, got: %v", err)
	}
}

func TestManifestRevisionTracker_ExpiryValidation(t *testing.T) {
	tracker := offlinedelivery.NewManifestRevisionTracker()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	expiredUntil := now.Add(-1 * time.Hour) // in the past

	mExpired := makeManifest("KL-WYD", 1, "sha-exp", "PKG-OK", nil, nil, expiredUntil)
	err := tracker.ValidateNext(mExpired, now)
	if !errors.Is(err, offlinedelivery.ErrManifestExpired) {
		t.Fatalf("expected ErrManifestExpired, got: %v", err)
	}
}

func TestManifestRevisionTracker_ConcurrentAccess(t *testing.T) {
	tracker := offlinedelivery.NewManifestRevisionTracker()
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	validUntil := now.Add(24 * time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m := makeManifest("KL-WYD", idx+1, "sha", "PKG-ACTIVE", nil, nil, validUntil)
			_ = tracker.Commit(m, now)
			_, _ = tracker.GetState("KL-WYD")
		}(i)
	}
	wg.Wait()
}
