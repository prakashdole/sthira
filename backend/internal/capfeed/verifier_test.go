package capfeed_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"sthira/backend/internal/capfeed"
	"sthira/backend/internal/contracts"
)

func sampleAlert() capfeed.Alert {
	p1, _ := contracts.NewPoint(76.105000, 11.570000)
	p2, _ := contracts.NewPoint(76.115000, 11.570000)
	p3, _ := contracts.NewPoint(76.115000, 11.580000)
	p4, _ := contracts.NewPoint(76.105000, 11.580000)

	return capfeed.Alert{
		Identifier:  "NDMA-KL-2026-001",
		Sender:      "sachet@ndma.gov.in",
		Sent:        time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC),
		Status:      capfeed.StatusActual,
		MsgType:     capfeed.MsgAlert,
		Scope:       capfeed.ScopePublic,
		References:  []string{"REF-B", "REF-A"},
		Language:    "en-IN",
		Event:       "Flash Flood Warning",
		Effective:   time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC),
		Expires:     time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
		Headline:    "Immediate Evacuation Ordered for Low-Lying Wards",
		Areas:       [][]contracts.Point{{p1, p2, p3, p4}},
		Operational: true,
	}
}

func TestCanonicalDigest_DeterminismAndWhitespaceInvariance(t *testing.T) {
	alert1 := sampleAlert()
	digest1, err := capfeed.ComputeCanonicalDigest(alert1)
	if err != nil {
		t.Fatalf("failed to compute digest 1: %v", err)
	}

	// Alert 2 with erratic whitespace in Event and Headline, and reordered References
	alert2 := sampleAlert()
	alert2.Event = "  Flash   Flood \n Warning \t"
	alert2.Headline = "  Immediate Evacuation   Ordered for Low-Lying Wards  "
	alert2.References = []string{"REF-A", "REF-B"} // different order in input

	digest2, err := capfeed.ComputeCanonicalDigest(alert2)
	if err != nil {
		t.Fatalf("failed to compute digest 2: %v", err)
	}

	if digest1 != digest2 {
		t.Fatalf("canonical digests must match despite whitespace/reordering differences: %s != %s", digest1, digest2)
	}
}

func TestVerifyAlertIntegrity_TamperDetection(t *testing.T) {
	alert := sampleAlert()
	digest, err := capfeed.ComputeCanonicalDigest(alert)
	if err != nil {
		t.Fatalf("failed to compute canonical digest: %v", err)
	}

	// Valid verification
	if err := capfeed.VerifyAlertIntegrity(alert, digest); err != nil {
		t.Fatalf("expected verification to pass: %v", err)
	}

	// 1. Tamper with Headline
	tamperedHeadline := alert
	tamperedHeadline.Headline = "Immediate Evacuation CANCELLED (FAKE)"
	if err := capfeed.VerifyAlertIntegrity(tamperedHeadline, digest); !errors.Is(err, capfeed.ErrDigestMismatch) {
		t.Fatalf("expected ErrDigestMismatch on tampered headline, got: %v", err)
	}

	// 2. Tamper with Polygon Geometry
	tamperedGeom := alert
	pModified, _ := contracts.NewPoint(76.105001, 11.570000)
	tamperedGeom.Areas = [][]contracts.Point{{pModified, alert.Areas[0][1], alert.Areas[0][2], alert.Areas[0][3]}}
	if err := capfeed.VerifyAlertIntegrity(tamperedGeom, digest); !errors.Is(err, capfeed.ErrDigestMismatch) {
		t.Fatalf("expected ErrDigestMismatch on tampered geometry, got: %v", err)
	}

	// 3. Tamper with Sender
	tamperedSender := alert
	tamperedSender.Sender = "imposter@malicious.org"
	if err := capfeed.VerifyAlertIntegrity(tamperedSender, digest); !errors.Is(err, capfeed.ErrDigestMismatch) {
		t.Fatalf("expected ErrDigestMismatch on tampered sender, got: %v", err)
	}
}

func TestAlertDeduplicator(t *testing.T) {
	dedup := capfeed.NewAlertDeduplicator(1*time.Hour, 100)
	now := time.Now()
	digest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// 1. Initial recording should succeed
	if err := dedup.CheckAndRecord(digest, now); err != nil {
		t.Fatalf("unexpected error on first record: %v", err)
	}

	// 2. Immediate duplicate within TTL should be rejected
	if err := dedup.CheckAndRecord(digest, now.Add(5*time.Minute)); !errors.Is(err, capfeed.ErrDuplicateAlert) {
		t.Fatalf("expected ErrDuplicateAlert for duplicate within TTL, got: %v", err)
	}

	// 3. After TTL has expired, re-recording should succeed
	if err := dedup.CheckAndRecord(digest, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("expected record after TTL expiry to succeed, got: %v", err)
	}
}

func TestAlertDeduplicator_Concurrent(t *testing.T) {
	dedup := capfeed.NewAlertDeduplicator(1*time.Hour, 1000)
	now := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			d := sampleAlert()
			d.Identifier = string(rune('A' + idx))
			digest, _ := capfeed.ComputeCanonicalDigest(d)
			_ = dedup.CheckAndRecord(digest, now)
		}(i)
	}

	wg.Wait()
}
