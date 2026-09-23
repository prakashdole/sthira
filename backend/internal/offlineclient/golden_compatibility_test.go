package offlineclient

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sthira/backend/internal/offlinepkg"
	"sthira/backend/internal/offlinequeue"
)

// TestMobileSharedGoldenCompatibility verifies that the Kotlin Multiplatform (KMP)
// golden fixtures created for M01 structurally conform to the Go backend P5 contracts.
func TestMobileSharedGoldenCompatibility(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "..", "mobile", "shared", "fixtures")

	// 1. Verify Good Manifest Fixture
	manifestPath := filepath.Join(fixtureDir, "good_manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", manifestPath, err)
	}

	var manifest offlinepkg.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to unmarshal manifest JSON into offlinepkg.Manifest: %v", err)
	}

	if manifest.ManifestID != "MAN-KL-2026-09-23-01" {
		t.Errorf("expected manifest ID MAN-KL-2026-09-23-01, got %s", manifest.ManifestID)
	}
	if manifest.CriticalCard.PackageID != "PKG-WAYANAD-2026-V1" {
		t.Errorf("expected critical card PKG-WAYANAD-2026-V1, got %s", manifest.CriticalCard.PackageID)
	}

	// 2. Verify Good Card Fixture
	cardPath := filepath.Join(fixtureDir, "good_card.json")
	cardData, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", cardPath, err)
	}

	var card offlinepkg.PublicIncidentCard
	if err := json.Unmarshal(cardData, &card); err != nil {
		t.Fatalf("failed to unmarshal card JSON into offlinepkg.PublicIncidentCard: %v", err)
	}

	if card.PackageID != "PKG-WAYANAD-2026-V1" {
		t.Errorf("expected card package ID PKG-WAYANAD-2026-V1, got %s", card.PackageID)
	}
	if card.Jurisdiction != "KL-WYD" {
		t.Errorf("expected jurisdiction KL-WYD, got %s", card.Jurisdiction)
	}
	if len(card.RedZones) != 1 || card.RedZones[0].ID != "RZ-MEPPADI-01" {
		t.Errorf("unexpected red zones: %+v", card.RedZones)
	}
	if len(card.SafeZones) != 1 || card.SafeZones[0].ID != "SZ-KALPETTA-01" {
		t.Errorf("unexpected safe zones: %+v", card.SafeZones)
	}
	if len(card.AllocationPolicy.Order) != 1 || card.AllocationPolicy.Order[0] != "SZ-KALPETTA-01" {
		t.Errorf("unexpected allocation policy order: %+v", card.AllocationPolicy.Order)
	}
}

// TestMobileOfflineQueueConformance verifies that the offline queue state machine
// semantics declared in Kotlin Multiplatform match Go's offlinequeue invariants.
func TestMobileOfflineQueueConformance(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	store, err := offlinequeue.NewStore(tmpDir, func() time.Time { return now })
	if err != nil {
		t.Fatalf("failed to create offlinequeue store: %v", err)
	}

	payload := []byte(`{"stay_id":"STAY-01","party_size":2}`)
	tokenRef := "TOKREF-USER-01"

	op := offlinequeue.PendingOperation{
		ID:              "OP-001",
		CreatedAt:       now,
		IdempotencyKey:  "IDEM-001",
		Endpoint:        "/api/v3/stays",
		Method:          "POST",
		TokenRef:        tokenRef,
		Payload:         payload,
		SnapshotVersion: 1,
		SelectionExpiry: now.Add(2 * time.Hour),
		State:           offlinequeue.StatePending,
	}

	// Enqueue
	if err := store.Enqueue(t.Context(), op); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	// Duplicate idempotency key with DIFFERENT payload must be rejected
	conflictOp := op
	conflictOp.ID = "OP-002"
	conflictOp.Payload = []byte(`{"stay_id":"STAY-01","party_size":5}`)
	if err := store.Enqueue(t.Context(), conflictOp); err == nil {
		t.Errorf("expected error on payload change for same key, got nil")
	}

	// Update to IN_FLIGHT
	op.State = offlinequeue.StateInFlight
	op.RetryCount = 1
	if err := store.UpdateState(t.Context(), op); err != nil {
		t.Fatalf("update to in-flight failed: %v", err)
	}

	// Transport Error -> PENDING_RECONCILIATION
	op.State = offlinequeue.StatePendingReconciliation
	op.LastError = "connection reset by peer"
	if err := store.UpdateState(t.Context(), op); err != nil {
		t.Fatalf("update to reconciliation failed: %v", err)
	}

	// Server confirms via replay -> COMMITTED
	op.State = offlinequeue.StateCommitted
	op.LastError = ""
	op.CommittedAt = now
	if err := store.UpdateState(t.Context(), op); err != nil {
		t.Fatalf("update to committed failed: %v", err)
	}

	// Verify terminal state cannot be mutated
	op.State = offlinequeue.StatePending
	if err := store.UpdateState(t.Context(), op); err == nil {
		t.Errorf("expected ErrTerminalState when mutating COMMITTED entry, got nil")
	}
}
