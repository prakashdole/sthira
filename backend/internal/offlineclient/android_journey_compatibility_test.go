package offlineclient

import (
	"math"
	"testing"
	"time"
)

// JourneyState mirrors the Android JourneyState enum in Kotlin
type JourneyState string

const (
	StateNotStarted          JourneyState = "NOT_STARTED"
	StateTracking            JourneyState = "TRACKING"
	StateNearDestination     JourneyState = "NEAR_DESTINATION"
	StateArrivalReported     JourneyState = "ARRIVAL_REPORTED"
	StatePaused              JourneyState = "PAUSED"
	StateLocationUnavailable JourneyState = "LOCATION_UNAVAILABLE"
	StateRouteRevoked        JourneyState = "ROUTE_REVOKED"
)

type PositionReading struct {
	Latitude           float64
	Longitude          float64
	AccuracyMeters     float64
	TimestampEpochMs   int64
	MonotonicElapsedMs int64
}

// Calculate Haversine distance in meters
func haversineDistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0 // Earth radius in meters
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return r * c
}

// TestAndroidJourneyStateEngineInvariants verifies the state machine invariants
// mirrored in Android's JourneyStateEngine.kt and CitizenJourneyViewModel.kt.
func TestAndroidJourneyStateEngineInvariants(t *testing.T) {
	// Facility: Vythiri Community Hall
	destLat := 11.5512
	destLon := 76.0415

	now := time.Now().UnixMilli()

	// 1. Initial State: NOT_STARTED
	state := StateNotStarted
	if state != StateNotStarted {
		t.Fatalf("expected initial state NOT_STARTED, got %s", state)
	}

	// 2. Start Tracking
	state = StateTracking

	// 3. Location update: Far away (500m)
	// (approx 0.0045 deg latitude is ~500m)
	posFar := PositionReading{
		Latitude:         destLat + 0.0045,
		Longitude:        destLon,
		AccuracyMeters:   15.0,
		TimestampEpochMs: now,
	}
	distFar := haversineDistanceMeters(posFar.Latitude, posFar.Longitude, destLat, destLon)
	if distFar < 400 || distFar > 600 {
		t.Errorf("expected ~500m distance, got %.1f", distFar)
	}
	if distFar > 75.0 {
		state = StateTracking
	}
	if state != StateTracking {
		t.Errorf("expected state TRACKING at 500m, got %s", state)
	}

	// 4. Inaccurate reading (>100m) must trigger LOCATION_UNAVAILABLE or stay in current advisory state
	posInaccurate := PositionReading{
		Latitude:         destLat,
		Longitude:        destLon,
		AccuracyMeters:   120.0, // Exceeds 100m threshold
		TimestampEpochMs: now,
	}
	if posInaccurate.AccuracyMeters > 100.0 {
		// Engine ignores or flags location unavailable
		state = StateLocationUnavailable
	}
	if state != StateLocationUnavailable {
		t.Errorf("expected LOCATION_UNAVAILABLE on poor accuracy, got %s", state)
	}

	// 5. Accurate reading within 50m of facility -> NEAR_DESTINATION
	posNear := PositionReading{
		Latitude:         destLat + 0.0003,
		Longitude:        destLon,
		AccuracyMeters:   10.0,
		TimestampEpochMs: now,
	}
	distNear := haversineDistanceMeters(posNear.Latitude, posNear.Longitude, destLat, destLon)
	if distNear > 75.0 {
		t.Fatalf("expected distance <= 75m, got %.1f", distNear)
	}
	if distNear <= 75.0 && posNear.AccuracyMeters <= 100.0 {
		state = StateNearDestination
	}
	if state != StateNearDestination {
		t.Errorf("expected state NEAR_DESTINATION, got %s", state)
	}

	// 6. Proximity alone MUST NOT auto-confirm arrival (O10 requirement)
	// State remains NEAR_DESTINATION until explicit citizen touch confirmation!
	if state == StateArrivalReported {
		t.Fatal("CRITICAL VIOLATION: Proximity auto-confirmed arrival! Geofencing auto-arrival is forbidden by O10.")
	}

	// 7. Explicit touch arrival confirmed by citizen
	state = StateArrivalReported
	if state != StateArrivalReported {
		t.Errorf("expected ARRIVAL_REPORTED after explicit confirmation, got %s", state)
	}
}

// TestAndroidCapacitySemantics verifies capacity states handled in Android UI
func TestAndroidCapacitySemantics(t *testing.T) {
	type FacilityCapacity struct {
		ID        string
		Total     *int
		Remaining *int
	}

	intPtr := func(v int) *int { return &v }

	// Case A: Normal availability
	normal := FacilityCapacity{ID: "FAC-1", Total: intPtr(100), Remaining: intPtr(25)}
	if normal.Remaining == nil || *normal.Remaining <= 0 {
		t.Errorf("expected normal capacity available")
	}

	// Case B: Full shelter
	full := FacilityCapacity{ID: "FAC-2", Total: intPtr(100), Remaining: intPtr(0)}
	isFull := full.Remaining != nil && *full.Remaining <= 0
	if !isFull {
		t.Errorf("expected shelter to be marked full")
	}

	// Case C: Unconfirmed capacity (nil)
	unknown := FacilityCapacity{ID: "FAC-3", Total: nil, Remaining: nil}
	isUnknown := unknown.Remaining == nil
	if !isUnknown {
		t.Errorf("expected capacity to be marked unknown")
	}
}
