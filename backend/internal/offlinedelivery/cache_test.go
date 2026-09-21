package offlinedelivery

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_Invalidation(t *testing.T) {
	now := time.Now()
	mc := NewMemoryCache(func() time.Time { return now }, 0, 0) // unbounded for this test

	// Manifest caching & invalidation
	mRec := &ManifestRecord{
		Jurisdiction:   "KL",
		Revision:       1,
		RawJSON:        []byte(`{"test":true}`),
		ChecksumSHA256: "abc",
		SourceStatus:   "CURRENT",
	}
	mc.PutManifest("KL", mRec, time.Minute)
	if _, hit := mc.GetManifest("KL"); !hit {
		t.Fatal("expected manifest cache hit")
	}

	mc.InvalidateManifest("KL")
	if _, hit := mc.GetManifest("KL"); hit {
		t.Fatal("expected manifest cache miss after InvalidateManifest")
	}

	// Card caching & granular invalidation
	cRec1 := &CardRecord{PackageID: "pkg-1", Version: 1, SourceStatus: "CURRENT"}
	cRec2 := &CardRecord{PackageID: "pkg-1", Version: 2, SourceStatus: "CURRENT"}
	cRec3 := &CardRecord{PackageID: "pkg-2", Version: 1, SourceStatus: "CURRENT"}

	mc.PutCard("pkg-1", 1, cRec1, time.Minute)
	mc.PutCard("pkg-1", 2, cRec2, time.Minute)
	mc.PutCard("pkg-2", 1, cRec3, time.Minute)

	if _, hit := mc.GetCard("pkg-1", 1); !hit {
		t.Fatal("expected card pkg-1:1 hit")
	}
	if _, hit := mc.GetCard("pkg-1", 2); !hit {
		t.Fatal("expected card pkg-1:2 hit")
	}
	if _, hit := mc.GetCard("pkg-2", 1); !hit {
		t.Fatal("expected card pkg-2:1 hit")
	}

	// Invalidate single card version
	mc.InvalidateCard("pkg-1", 1)
	if _, hit := mc.GetCard("pkg-1", 1); hit {
		t.Fatal("expected card pkg-1:1 miss after InvalidateCard")
	}
	if _, hit := mc.GetCard("pkg-1", 2); !hit {
		t.Fatal("expected card pkg-1:2 to remain cached")
	}

	// Invalidate whole package
	mc.InvalidatePackage("pkg-1")
	if _, hit := mc.GetCard("pkg-1", 2); hit {
		t.Fatal("expected card pkg-1:2 miss after InvalidatePackage")
	}
	if _, hit := mc.GetCard("pkg-2", 1); !hit {
		t.Fatal("expected card pkg-2:1 to remain cached")
	}
}

func TestCachedSource_QuarantineAndWithdrawalInvalidation(t *testing.T) {
	now := time.Now()
	mock := newMockPublicationSource()
	cs := NewCachedSource(mock, Config{
		ManifestCacheTTL: 10 * time.Minute,
		CardCacheTTL:     10 * time.Minute,
	}, func() time.Time { return now })

	ctx := context.Background()

	// 1. Initial fetch warms the cache
	m, err := cs.GetManifest(ctx, "KL")
	if err != nil {
		t.Fatalf("GetManifest failed: %v", err)
	}
	if m.SourceStatus != "CURRENT" {
		t.Fatalf("expected CURRENT, got %s", m.SourceStatus)
	}

	// 2. Upstream quarantines the manifest
	mock.manifestErr = ErrQuarantined

	// Without invalidation, cache still returns the stale cached record
	staleM, err := cs.GetManifest(ctx, "KL")
	if err != nil || staleM.SourceStatus != "CURRENT" {
		t.Fatalf("expected cache to return stale record before invalidation, got rec=%v, err=%v", staleM, err)
	}

	// Invalidate manifest cache
	cs.InvalidateManifest("KL")

	// Now fetch must hit upstream and return ErrQuarantined
	_, err = cs.GetManifest(ctx, "KL")
	if err != ErrQuarantined {
		t.Fatalf("expected ErrQuarantined after InvalidateManifest, got %v", err)
	}

	// 3. Card quarantine test
	card, err := cs.GetCard(ctx, "PKG-WAYANAD-01", 1)
	if err != nil {
		t.Fatalf("GetCard failed: %v", err)
	}
	if card.PackageID != "PKG-WAYANAD-01" {
		t.Fatalf("unexpected card: %v", card)
	}

	// Upstream withdraws the card
	mock.cardErr = ErrQuarantined

	// Invalidate single card
	cs.InvalidateCard("PKG-WAYANAD-01", 1)

	_, err = cs.GetCard(ctx, "PKG-WAYANAD-01", 1)
	if err != ErrQuarantined {
		t.Fatalf("expected ErrQuarantined after InvalidateCard, got %v", err)
	}
}
