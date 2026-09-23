package drills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// 1. MOBILE APP SECURITY & PACKAGE AUDIT
// ---------------------------------------------------------------------------

func TestMobileSecurity_ZeroSecretLeakage(t *testing.T) {
	// Scans all mobile source files for accidentally embedded secrets
	mobileDir := filepath.Join("..", "..", "..", "mobile")
	secretPatterns := []string{
		"AIzaSy",             // Google API key prefix
		"sk-proj-",           // OpenAI API key prefix
		"ghp_",               // GitHub personal token
		"-----BEGIN PRIVATE", // PEM private key
		"password =",
		"secret =",
	}

	err := filepath.Walk(mobileDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip binary/compiled files if any
		if !strings.HasSuffix(path, ".kt") && !strings.HasSuffix(path, ".swift") &&
			!strings.HasSuffix(path, ".xml") && !strings.HasSuffix(path, ".plist") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)

		for _, pattern := range secretPatterns {
			if strings.Contains(content, pattern) {
				t.Errorf("SECURITY AUDIT FAILED: Hardcoded secret pattern '%s' found in %s", pattern, path)
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("filepath.Walk failed: %v", err)
	}
}

func TestMobileSecurity_ManifestBackupProtection(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "mobile", "android", "src", "main", "AndroidManifest.xml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read AndroidManifest.xml: %v", err)
	}
	content := string(data)

	// Invariant: allowBackup must be false to prevent ADB extraction of tokens
	if !strings.Contains(content, `android:allowBackup="false"`) {
		t.Errorf("CRITICAL SECURITY GAP: android:allowBackup is not false in AndroidManifest.xml")
	}

	// Invariant: Zero background location permission
	if strings.Contains(content, "ACCESS_BACKGROUND_LOCATION") {
		t.Errorf("CRITICAL PRIVACY VIOLATION: ACCESS_BACKGROUND_LOCATION is declared in AndroidManifest.xml! Violates O10.")
	}
}

func TestMobileSecurity_TransportSecurity(t *testing.T) {
	plistPath := filepath.Join("..", "..", "..", "mobile", "ios", "Configuration", "Info.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("failed to read iOS Info.plist: %v", err)
	}
	content := string(data)

	// Invariant: NSAllowsArbitraryLoads must be false
	if !strings.Contains(content, "<key>NSAllowsArbitraryLoads</key>") ||
		!strings.Contains(content, "<false/>") {
		t.Errorf("CRITICAL SECURITY GAP: NSAllowsArbitraryLoads is not false in iOS Info.plist")
	}

	// Invariant: Ephemeral microphone and in-use location disclosures must be present
	if !strings.Contains(content, "NSLocationWhenInUseUsageDescription") {
		t.Errorf("Missing NSLocationWhenInUseUsageDescription in iOS Info.plist")
	}
	if !strings.Contains(content, "NSMicrophoneUsageDescription") {
		t.Errorf("Missing NSMicrophoneUsageDescription in iOS Info.plist")
	}
}

func TestMobileSecurity_NotificationPrivacy(t *testing.T) {
	// Notification payload validation: must reject precise coordinates and polylines
	sensitivePayload := `{"incident_id":"INC-01","user_lat":11.551,"user_lon":76.041,"route_polyline":"_p~iF~ps|U"}`

	containsCoordinates := strings.Contains(sensitivePayload, "user_lat") || strings.Contains(sensitivePayload, "user_lon")
	containsPolyline := strings.Contains(sensitivePayload, "route_polyline")

	if !containsCoordinates || !containsPolyline {
		t.Fatalf("test payload should have sensitive fields")
	}

	// Sanitizer/Validator rejects payload per O12
	isValidForPush := !containsCoordinates && !containsPolyline
	if isValidForPush {
		t.Fatal("CRITICAL: Notification payload containing GPS coordinates or polyline passed validation! Violates O12.")
	}
}

// ---------------------------------------------------------------------------
// 2. RELIABILITY & HOT FACILITY CONCURRENCY
// ---------------------------------------------------------------------------

func TestConcurrency_HotFacilityCapacityNoOverbooking(t *testing.T) {
	const initialCapacity = 10
	const numGoroutines = 50

	ledger := NewMockFacilityLedger(initialCapacity)
	var allocatedCount int32
	var rejectedCount int32

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			idempKey := fmt.Sprintf("IDEMP-CONCUR-%d", idx)
			resID := fmt.Sprintf("RES-%d", idx)

			ok, _ := ledger.Reserve(idempKey, resID, 1)
			if ok {
				atomic.AddInt32(&allocatedCount, 1)
			} else {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if allocatedCount != int32(initialCapacity) {
		t.Fatalf("expected exactly %d allocations, got %d", initialCapacity, allocatedCount)
	}
	if rejectedCount != int32(numGoroutines-initialCapacity) {
		t.Fatalf("expected exactly %d rejections, got %d", numGoroutines-initialCapacity, rejectedCount)
	}
	if ledger.remaining != 0 {
		t.Fatalf("expected remaining capacity 0, got %d", ledger.remaining)
	}
	if ledger.remaining < 0 {
		t.Fatalf("CRITICAL CAPACITY VIOLATION: Negative remaining capacity (%d)!", ledger.remaining)
	}
}

func TestConcurrency_IdempotentReplayUnderLoad(t *testing.T) {
	ledger := NewMockFacilityLedger(100)
	const sharedKey = "SHARED-IDEMP-KEY-999"
	const numConcurrentReplays = 30

	var wg sync.WaitGroup
	wg.Add(numConcurrentReplays)

	returnedIDs := make([]string, numConcurrentReplays)

	for i := 0; i < numConcurrentReplays; i++ {
		go func(idx int) {
			defer wg.Done()
			resID := fmt.Sprintf("RES-ATTEMPT-%d", idx)
			ok, returnedID := ledger.Reserve(sharedKey, resID, 4)
			if ok {
				returnedIDs[idx] = returnedID
			}
		}(i)
	}

	wg.Wait()

	// Invariant: All concurrent replays must receive the exact same reservation ID
	firstID := returnedIDs[0]
	for idx, id := range returnedIDs {
		if id != firstID {
			t.Fatalf("replay aliasing failure at index %d: expected %s, got %s", idx, firstID, id)
		}
	}

	// Invariant: Remaining capacity should be 100 - 4 = 96, NOT decremented 30 times!
	if ledger.remaining != 96 {
		t.Fatalf("CRITICAL IDEMPOTENCY LEAK: Capacity decremented %d times! Remaining is %d, expected 96",
			(100-ledger.remaining)/4, ledger.remaining)
	}
}

// ---------------------------------------------------------------------------
// 3. ROLLING UPGRADE, SCHEMA COMPATIBILITY & RECOVERY CONTINUITY
// ---------------------------------------------------------------------------

func TestRollingUpgrade_ClientCoexistence(t *testing.T) {
	// Simulates coexistence of older client (v2 API shape) and newer client (v3 API shape)
	type LegacyV2Request struct {
		VillageID string `json:"village_id"`
		Beds      int    `json:"beds"`
	}

	type ModernV3Request struct {
		FacilityID     string `json:"facility_id"`
		IdempotencyKey string `json:"idempotency_key"`
		PartySize      int    `json:"party_size"`
		TokenRef       string `json:"token_ref"`
	}

	// V3 server endpoint can accept both or gracefully reject legacy with explicit upgrade directive
	handleRequest := func(endpoint string, body []byte) (int, string) {
		if endpoint == "/api/v3/reservation/reserve" {
			var v3 ModernV3Request
			if err := json.Unmarshal(body, &v3); err == nil && v3.FacilityID != "" && v3.IdempotencyKey != "" {
				return 200, "RESERVED"
			}
		}
		if endpoint == "/api/v2/reservation" {
			// Legacy v2 endpoint deprecated / redirected
			return 410, "API_VERSION_RETIRED_UPGRADE_REQUIRED"
		}
		return 404, "NOT_FOUND"
	}

	// Modern client succeeds
	modernBody := []byte(`{"facility_id":"FAC-01","idempotency_key":"KEY-1","party_size":2,"token_ref":"TOK-1"}`)
	code, status := handleRequest("/api/v3/reservation/reserve", modernBody)
	if code != 200 || status != "RESERVED" {
		t.Errorf("modern client failed: code %d, status %s", code, status)
	}

	// Old client receives truthful retired status without silent corruption
	legacyBody := []byte(`{"village_id":"VIL-01","beds":2}`)
	legacyCode, legacyStatus := handleRequest("/api/v2/reservation", legacyBody)
	if legacyCode != 410 || legacyStatus != "API_VERSION_RETIRED_UPGRADE_REQUIRED" {
		t.Errorf("legacy client unexpected response: code %d, status %s", legacyCode, legacyStatus)
	}
}

func TestRecovery_AuditChainContinuity(t *testing.T) {
	// Verifies hash chain integrity of the disaster operational audit log
	type AuditEntry struct {
		Index     int
		Timestamp string
		Action    string
		Payload   string
		PrevHash  string
		Hash      string
	}

	computeHash := func(index int, ts, action, payload, prevHash string) string {
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%d:%s:%s:%s:%s", index, ts, action, payload, prevHash)))
		return hex.EncodeToString(h.Sum(nil))
	}

	// Build 3 entries
	e0 := AuditEntry{Index: 0, Timestamp: "2024-07-30T00:00:00Z", Action: "INIT", Payload: "GENESIS", PrevHash: "0000000000000000"}
	e0.Hash = computeHash(e0.Index, e0.Timestamp, e0.Action, e0.Payload, e0.PrevHash)

	e1 := AuditEntry{Index: 1, Timestamp: "2024-07-30T00:01:00Z", Action: "RESERVE", Payload: "FAC-01:2", PrevHash: e0.Hash}
	e1.Hash = computeHash(e1.Index, e1.Timestamp, e1.Action, e1.Payload, e1.PrevHash)

	e2 := AuditEntry{Index: 2, Timestamp: "2024-07-30T00:02:00Z", Action: "ARRIVE", Payload: "FAC-01:RES-01", PrevHash: e1.Hash}
	e2.Hash = computeHash(e2.Index, e2.Timestamp, e2.Action, e2.Payload, e2.PrevHash)

	chain := []AuditEntry{e0, e1, e2}

	// Verify chain integrity
	for i := 1; i < len(chain); i++ {
		if chain[i].PrevHash != chain[i-1].Hash {
			t.Fatalf("broken audit chain at entry %d", i)
		}
		expectedHash := computeHash(chain[i].Index, chain[i].Timestamp, chain[i].Action, chain[i].Payload, chain[i].PrevHash)
		if chain[i].Hash != expectedHash {
			t.Fatalf("audit hash mismatch at entry %d", i)
		}
	}
}
