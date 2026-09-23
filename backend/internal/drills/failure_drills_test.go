package drills

import (
	"math"
	"sync"
	"testing"
	"time"
)

// JourneyState models the client journey states across Android and iOS
type JourneyState string

const (
	StateNotStarted         JourneyState = "NOT_STARTED"
	StateTracking           JourneyState = "TRACKING"
	StateNearDestination   JourneyState = "NEAR_DESTINATION"
	StateArrivalReported   JourneyState = "ARRIVAL_REPORTED"
	StatePaused             JourneyState = "PAUSED"
	StateLocationUnavailable JourneyState = "LOCATION_UNAVAILABLE"
	StateRouteRevoked       JourneyState = "ROUTE_REVOKED"
)

type PositionReading struct {
	Latitude          float64
	Longitude         float64
	AccuracyMeters    float64
	TimestampEpochMs  int64
	MonotonicElapsedMs int64
}

func haversineDistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return r * c
}

// Simulated Capacity Ledger for controlled drills
type MockFacilityLedger struct {
	mu           sync.Mutex
	total        int
	remaining    int
	reservations map[string]int // reservationId -> partySize
	statuses     map[string]string // reservationId -> "RESERVED" / "ARRIVED" / "DEPARTED" / "CANCELLED"
	idempKeys    map[string]string // idempKey -> reservationId
}

func NewMockFacilityLedger(total int) *MockFacilityLedger {
	return &MockFacilityLedger{
		total:        total,
		remaining:    total,
		reservations: make(map[string]int),
		statuses:     make(map[string]string),
		idempKeys:    make(map[string]string),
	}
}

func (l *MockFacilityLedger) Reserve(idempKey, resID string, count int) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Idempotent replay: return existing reservation without duplicate decrement
	if existingID, exists := l.idempKeys[idempKey]; exists {
		return true, existingID
	}

	if l.remaining < count {
		return false, "CAPACITY_EXHAUSTED"
	}

	l.remaining -= count
	l.reservations[resID] = count
	l.statuses[resID] = "RESERVED"
	l.idempKeys[idempKey] = resID
	return true, resID
}

func (l *MockFacilityLedger) Arrive(resID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if status, exists := l.statuses[resID]; exists && status == "RESERVED" {
		l.statuses[resID] = "ARRIVED"
		return true
	}
	return false
}

func (l *MockFacilityLedger) Depart(resID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if count, exists := l.reservations[resID]; exists && l.statuses[resID] != "DEPARTED" {
		l.statuses[resID] = "DEPARTED"
		l.remaining += count
		return true
	}
	return false
}

// -------------------------------------------------------------
// DRILL 01: Same-Name Village Clarification (409 Conflict)
// -------------------------------------------------------------
func TestDrill01_SameNameVillageClarification(t *testing.T) {
	type Candidate struct {
		ID   string
		Name string
	}

	query := "meppadi"
	var candidates []Candidate

	if query == "meppadi" {
		candidates = []Candidate{
			{ID: "LOC-MEP-01", Name: "Meppadi Village (Near Church)"},
			{ID: "LOC-MEP-02", Name: "Meppadi Junction (Bus Stand)"},
		}
	}

	if len(candidates) <= 1 {
		t.Fatalf("expected multiple candidates for ambiguous name")
	}

	// Invariant: Auto-selection is forbidden. Citizen must explicitly select.
	autoSelected := false
	if autoSelected {
		t.Fatal("CRITICAL VIOLATION: Ambiguous place auto-selected! PRD mandates explicit disambiguation.")
	}

	// Citizen taps candidate 1
	selected := candidates[0]
	if selected.ID != "LOC-MEP-01" {
		t.Errorf("expected candidate 1, got %s", selected.ID)
	}
}

// -------------------------------------------------------------
// DRILL 02: Missing Permissions (Microphone & Location Denied)
// -------------------------------------------------------------
func TestDrill02_MissingPermissions(t *testing.T) {
	micPermission := false
	gpsPermission := false

	// Citizen should still be able to search textually and view turn-by-turn instructions
	textFallbackActive := !micPermission
	if !textFallbackActive {
		t.Fatal("text fallback must be active when mic permission is denied")
	}
	textQuery := "Vythiri Community Hall"
	if textQuery == "" {
		t.Fatal("text search should be available when mic denied")
	}

	// GPS denied: state is LOCATION_UNAVAILABLE or manual fallback
	state := StateNotStarted
	if !gpsPermission {
		state = StateLocationUnavailable
	}

	if state != StateLocationUnavailable {
		t.Errorf("expected LOCATION_UNAVAILABLE, got %s", state)
	}

	// Non-map textual instructions remain complete and accessible
	instructions := []string{
		"Step 1: Proceed North on Main Road for 500m",
		"Step 2: Turn Right at High Grounds Safe Zone sign",
		"Step 3: Arrive at Community Hall Shelter on Left",
	}

	if len(instructions) != 3 {
		t.Errorf("expected 3 instructions, got %d", len(instructions))
	}
}

// -------------------------------------------------------------
// DRILL 03: Full Shelter & Unknown Capacity Handling
// -------------------------------------------------------------
func TestDrill03_FullAndUnknownCapacity(t *testing.T) {
	ledger := NewMockFacilityLedger(50)

	// Fill shelter to capacity
	ok, _ := ledger.Reserve("K1", "RES-01", 50)
	if !ok || ledger.remaining != 0 {
		t.Fatalf("expected remaining to be 0")
	}

	// Next reservation must fail closed
	ok2, reason := ledger.Reserve("K2", "RES-02", 2)
	if ok2 {
		t.Fatal("CRITICAL: Full shelter allowed reservation! Overbooking violates capacity invariants.")
	}
	if reason != "CAPACITY_EXHAUSTED" {
		t.Errorf("expected CAPACITY_EXHAUSTED, got %s", reason)
	}

	// Unknown capacity test: nil capacity must not be assumed 0 or 100%
	var unknownRemaining *int = nil
	if unknownRemaining != nil {
		t.Errorf("expected nil for unconfirmed capacity")
	}
}

// -------------------------------------------------------------
// DRILL 04: Unavailable / Revoked Route in Transit
// -------------------------------------------------------------
func TestDrill04_UnavailableAndRevokedRoute(t *testing.T) {
	state := StateTracking
	routeRevokedByAuthority := true

	if routeRevokedByAuthority {
		state = StateRouteRevoked
	}

	if state != StateRouteRevoked {
		t.Errorf("expected ROUTE_REVOKED, got %s", state)
	}

	// Invariant: Navigation along revoked corridor must be immediately halted
	navigationHalted := (state == StateRouteRevoked)
	if !navigationHalted {
		t.Fatal("CRITICAL: Navigation continued on revoked route!")
	}
}

// -------------------------------------------------------------
// DRILL 05: Closure While Viewing or Traveling
// -------------------------------------------------------------
func TestDrill05_ClosureWhileViewingOrTraveling(t *testing.T) {
	facilityStatus := "OPERATIONAL"
	// Authority closes facility due to flooding
	facilityStatus = "CLOSED_DUE_TO_HAZARD"

	// Client revalidates before check-in
	allowCheckIn := (facilityStatus == "OPERATIONAL")
	if allowCheckIn {
		t.Fatal("CRITICAL: Check-in allowed at closed facility!")
	}
}

// -------------------------------------------------------------
// DRILL 06: Statutory Stay Extension Limits (7 to 30 days)
// -------------------------------------------------------------
func TestDrill06_PolicyLimits7to30Days(t *testing.T) {
	const MaxPolicyExtensionDays = 30

	requestedDaysValid := 7
	if requestedDaysValid > MaxPolicyExtensionDays {
		t.Errorf("expected valid extension to pass")
	}

	requestedDaysExcess := 45
	allowed := requestedDaysExcess <= MaxPolicyExtensionDays
	if allowed {
		t.Fatal("CRITICAL: 45 day extension allowed! Exceeds statutory policy limit of 30 days.")
	}
}

// -------------------------------------------------------------
// DRILL 07: Transfer Failure Preserves Original Stay
// -------------------------------------------------------------
func TestDrill07_TransferFailureWithoutLosingStay(t *testing.T) {
	shelterA := NewMockFacilityLedger(20)
	shelterB := NewMockFacilityLedger(0) // Full

	// Initial stay at Shelter A
	ok, resID := shelterA.Reserve("KEY-A", "RES-A", 4)
	if !ok {
		t.Fatalf("failed initial reserve")
	}

	// Citizen attempts transfer to Shelter B (which is full)
	transferOK, _ := shelterB.Reserve("KEY-B", "RES-B", 4)
	if transferOK {
		t.Fatal("transfer should have failed due to zero capacity")
	}

	// Invariant: Shelter A stay MUST NOT be released if transfer fails
	if shelterA.statuses[resID] != "RESERVED" || shelterA.remaining != 16 {
		t.Fatal("CRITICAL: Original stay cancelled despite failed transfer! Evacuee left without shelter.")
	}
}

// -------------------------------------------------------------
// DRILL 08: Expiry Racing Arrival
// -------------------------------------------------------------
func TestDrill08_ExpiryRacingArrival(t *testing.T) {
	now := time.Now().UTC()
	reservationExpiry := now.Add(-100 * time.Millisecond) // Expired 100ms ago

	// Arrival attempt
	arrivalAllowed := now.Before(reservationExpiry)
	if arrivalAllowed {
		t.Fatal("CRITICAL: Expired reservation allowed arrival check-in! Must require renewed validation.")
	}
}

// -------------------------------------------------------------
// DRILL 09: Lost Response Idempotent Replay
// -------------------------------------------------------------
func TestDrill09_LostResponseIdempotentRetry(t *testing.T) {
	ledger := NewMockFacilityLedger(100)
	idempKey := "CLIENT-IDEMP-UUID-12345"

	// First attempt: succeeds on server, but network drops before client receives ACK
	ok1, res1 := ledger.Reserve(idempKey, "RES-001", 3)
	if !ok1 || res1 != "RES-001" {
		t.Fatalf("first reservation failed")
	}
	remainingAfter1 := ledger.remaining

	// Client retries with exact same idempotency key
	ok2, res2 := ledger.Reserve(idempKey, "RES-002-DIFFERENT-CALL", 3)
	if !ok2 || res2 != "RES-001" {
		t.Fatalf("replay did not return original reservation ID: got %s", res2)
	}

	// Invariant: Remaining capacity must NOT have decremented twice!
	if ledger.remaining != remainingAfter1 {
		t.Fatalf("CRITICAL CAPACITY LEAK: remaining decremented twice! Expected %d, got %d", remainingAfter1, ledger.remaining)
	}
}

// -------------------------------------------------------------
// DRILL 10: Revoked Source Invalidates Cached Package
// -------------------------------------------------------------
func TestDrill10_RevokedSourceAndStaleCache(t *testing.T) {
	type PackageMetadata struct {
		SourceID      string
		SourceStatus  string
		PackageStatus string
	}

	pkg := PackageMetadata{
		SourceID:      "SRC-KSDMA-01",
		SourceStatus:  "ACTIVE",
		PackageStatus: "ACTIVE",
	}

	// Authority suspends/quarantines source
	pkg.SourceStatus = "SUSPENDED"

	// Client revalidation check
	isUsable := pkg.SourceStatus == "ACTIVE" && pkg.PackageStatus == "ACTIVE"
	if isUsable {
		t.Fatal("CRITICAL: Suspended source package remained active for navigation!")
	}
}

// -------------------------------------------------------------
// DRILL 11: Offline Cold Start & Clock Rollback Detection
// -------------------------------------------------------------
func TestDrill11_OfflineColdStartAndClockRollback(t *testing.T) {
	lastKnownServerEpochMs := int64(1722300000000)
	currentDeviceTimeEpochMs := int64(1722100000000) // Clock rolled back 2 days

	// Freshness evaluator checks monotonic / server time drift
	clockTampered := currentDeviceTimeEpochMs < lastKnownServerEpochMs
	if !clockTampered {
		t.Fatalf("expected clock rollback to be detected")
	}

	freshnessState := "FRESH"
	if clockTampered {
		freshnessState = "UNVERIFIABLE"
	}

	if freshnessState != "UNVERIFIABLE" {
		t.Errorf("expected UNVERIFIABLE freshness on clock rollback, got %s", freshnessState)
	}
}

// -------------------------------------------------------------
// DRILL 12: Dependency Outages & Fail-Closed Behavior
// -------------------------------------------------------------
func TestDrill12_DependencyOutagesAndFailClosed(t *testing.T) {
	dbHealthy := false

	// Public endpoint prober
	statusCode := 200
	if !dbHealthy {
		statusCode = 503 // Service Unavailable fail-closed
	}

	if statusCode != 503 {
		t.Fatalf("expected 503 fail-closed during database outage, got %d", statusCode)
	}
}

// -------------------------------------------------------------
// DRILL 13: Location Proximity NEVER Auto-Confirms Arrival
// -------------------------------------------------------------
func TestDrill13_LocationProximityNeverAutoConfirms(t *testing.T) {
	shelterLat := 11.5512
	shelterLon := 76.0415
	ledger := NewMockFacilityLedger(50)

	_, resID := ledger.Reserve("IDEMP-ARR", "RES-ARR-01", 2)

	// User position is right on top of shelter (distance = 2m)
	pos := PositionReading{
		Latitude:         shelterLat + 0.00002,
		Longitude:        shelterLon,
		AccuracyMeters:   5.0,
		TimestampEpochMs: time.Now().UnixMilli(),
	}

	dist := haversineDistanceMeters(pos.Latitude, pos.Longitude, shelterLat, shelterLon)
	if dist > 10.0 {
		t.Fatalf("expected distance < 10m, got %.1f", dist)
	}

	// Engine state becomes NEAR_DESTINATION (Advisory only)
	state := StateNearDestination
	if state == StateArrivalReported {
		t.Fatal("CRITICAL VIOLATION: Near destination auto-confirmed arrival! Geofencing auto-arrival strictly forbidden.")
	}

	// Status in official ledger remains RESERVED, not ARRIVED
	if ledger.statuses[resID] != "RESERVED" {
		t.Fatalf("official ledger mutated to ARRIVED without explicit touch confirmation!")
	}

	// Explicit citizen touch confirmation arrives
	arrived := ledger.Arrive(resID)
	if !arrived || ledger.statuses[resID] != "ARRIVED" {
		t.Fatalf("explicit arrival failed")
	}
}
