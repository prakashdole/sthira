package scenarioprep

import (
	"encoding/json"
	"testing"

	"sthira/backend/internal/opkg"
)

// canonicalJSONOf marshals an opkg.Package via opkg's canonical encoding
// path so the embedded checksum validates.
func canonicalJSONOf(t *testing.T, v any) string {
	t.Helper()
	pkg, ok := v.(*opkg.Package)
	if !ok {
		t.Fatalf("canonicalJSONOf: want *opkg.Package, got %T", v)
	}
	pkg.Provenance.ChecksumSHA256 = opkg.Checksum(pkg)
	body, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(body) + "\n"
}

// minimalPackageValue builds a minimal opkg.Package the validator accepts.
func minimalPackageValue(jurisdiction string) any {
	cap1 := 10
	walkIns := true
	expiry := 300
	return &opkg.Package{
		Provenance: opkg.Provenance{
			DatasetID:     "PKG-TEST-1",
			EvidenceClass: opkg.EvidenceSynthetic,
			Version:       1,
			Jurisdiction:  jurisdiction,
			Authority:     "test.example",
			EffectiveAt:   "2026-09-12T04:00:00Z",
			ExpiresAt:     "2026-09-13T04:00:00Z",
		},
		Alert: opkg.Alert{Identifier: "ALERT-TEST-1"},
		RedZones: []opkg.Zone{
			{ID: "RZ-1"},
		},
		SafeZones: []opkg.Zone{
			{ID: "SZ-1", Capacity: &cap1, Location: []float64{76.5, 11.5}, Status: opkg.ZoneOpen},
		},
		ApprovedRoutes: []opkg.Route{
			{
				ID:           "RT-1",
				FromZoneID:   "RZ-1",
				ToSafeZoneID: "SZ-1",
				Approval:     opkg.ApprovalSynthetic,
				Mode:         opkg.ModeFoot,
				Geometry:     []byte(`{"type":"LineString","coordinates":[[76.0,11.4],[76.5,11.5]]}`),
			},
		},
		Instructions: []opkg.InstructionAsset{
			{ID: "INS-1", Language: "en-IN"},
		},
		Facilities: []opkg.Facility{
			{ID: "FAC-1", SafeZone: "SZ-1"},
		},
		Policy: opkg.AllocationPolicy{
			Order:                   []string{"SZ-1"},
			ReservationExpirySeconds: &expiry,
			AllowWalkIns:             &walkIns,
		},
		Contacts: []opkg.EmergencyContact{
			{Name: "Test Control", Number: "112"},
		},
	}
}