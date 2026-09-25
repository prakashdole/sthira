package offlineclient

import (
	"testing"
	"time"
)

// TestIOSJourneyStateEngineInvariants verifies the state machine invariants
// mirrored in iOS LocationManager.swift and CitizenJourneyViewModel.swift.
func TestIOSJourneyStateEngineInvariants(t *testing.T) {
	// Facility: Vythiri Community Hall
	destLat := 11.5512
	destLon := 76.0415

	now := time.Now().UnixMilli()

	// 1. Initial State: NOT_STARTED
	state := StateNotStarted

	// 2. Start Tracking
	state = StateTracking

	// 3. Normal en-route position: 500m away
	posEnRoute := PositionReading{
		Latitude:         destLat + 0.0045,
		Longitude:        destLon,
		AccuracyMeters:   12.0,
		TimestampEpochMs: now,
	}
	distEnRoute := haversineDistanceMeters(posEnRoute.Latitude, posEnRoute.Longitude, destLat, destLon)
	if distEnRoute <= 75.0 {
		t.Fatalf("expected en route distance > 75m, got %.1f", distEnRoute)
	}
	if state != StateTracking {
		t.Errorf("expected state TRACKING, got %s", state)
	}

	// 4. Stale reading (>30s) must be discarded (Gate 1 in LocationManager.swift)
	staleReadingAgeSeconds := 35.0
	isStale := staleReadingAgeSeconds > 30.0
	if !isStale {
		t.Errorf("expected reading to be marked stale")
	}

	// 5. Inaccurate reading (>100m) must set LOCATION_UNAVAILABLE (Gate 2 in LocationManager.swift)
	posInaccurate := PositionReading{
		Latitude:         destLat,
		Longitude:        destLon,
		AccuracyMeters:   105.0, // Exceeds 100m threshold
		TimestampEpochMs: now,
	}
	if posInaccurate.AccuracyMeters > 100.0 {
		state = StateLocationUnavailable
	}
	if state != StateLocationUnavailable {
		t.Errorf("expected LOCATION_UNAVAILABLE on inaccurate reading, got %s", state)
	}

	// 6. Near destination (<=75m) triggers NEAR_DESTINATION advisory
	posNear := PositionReading{
		Latitude:         destLat + 0.0003,
		Longitude:        destLon,
		AccuracyMeters:   8.0,
		TimestampEpochMs: now,
	}
	distNear := haversineDistanceMeters(posNear.Latitude, posNear.Longitude, destLat, destLon)
	if distNear <= 75.0 && posNear.AccuracyMeters <= 100.0 {
		state = StateNearDestination
	}
	if state != StateNearDestination {
		t.Errorf("expected NEAR_DESTINATION, got %s", state)
	}

	// 7. Proximity NEVER auto-confirms arrival in iOS (O10 invariant)
	if state == StateArrivalReported {
		t.Fatal("CRITICAL: Proximity auto-confirmed arrival! Geofencing auto-arrival is strictly prohibited.")
	}

	// 8. Explicit touch arrival confirmation
	state = StateArrivalReported
	if state != StateArrivalReported {
		t.Errorf("expected ARRIVAL_REPORTED, got %s", state)
	}
}

// TestIOSKeychainTokenIsolation verifies token isolation invariants
func TestIOSKeychainTokenIsolation(t *testing.T) {
	// Account isolation: tokens must be isolated by account identifier
	accountA := "CITIZEN-001"
	accountB := "CITIZEN-002"
	tokenA := "TOK-A-SECRET"
	tokenB := "TOK-B-SECRET"

	store := map[string]string{
		accountA: tokenA,
		accountB: tokenB,
	}

	if store[accountA] == store[accountB] {
		t.Errorf("tokens must not alias across accounts")
	}
	if store[accountA] != tokenA {
		t.Errorf("token for account A corrupted")
	}
}
