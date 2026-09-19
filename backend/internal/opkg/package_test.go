package opkg

import (
	"strings"
	"testing"
)

// validPackage returns a complete, internally consistent package with the
// checksum computed and injected. Tests mutate it then recompute or corrupt.
func validPackage() *Package {
	p := &Package{
		Provenance: Provenance{
			DatasetID:     "PKG-1",
			EvidenceClass: EvidenceSynthetic,
			Version:       1,
			Jurisdiction:  "KL",
			Authority:     "synthetic.ddma.example",
			EffectiveAt:   "2026-09-12T04:00:00Z",
			ExpiresAt:     "2026-09-13T04:00:00Z",
		},
		Alert: Alert{Identifier: "ALERT-1"},
		RedZones: []Zone{
			{ID: "RZ-1"},
		},
		SafeZones: []Zone{
			{ID: "SZ-1", Capacity: intPtr(100), Location: []float64{76.5, 11.5}, Status: ZoneOpen},
			{ID: "SZ-2", Capacity: intPtr(50), Location: []float64{76.6, 11.6}, Status: ZoneOpen},
		},
		ApprovedRoutes: []Route{
			{
				ID:           "RT-1",
				FromZoneID:   "RZ-1",
				ToSafeZoneID: "SZ-1",
				Approval:     ApprovalSynthetic,
				Geometry:     []byte(`{"type":"LineString","coordinates":[[76.0,11.4],[76.5,11.5]]}`),
			},
		},
		Instructions: []InstructionAsset{
			{ID: "INS-1", Language: "en-IN"},
			{ID: "INS-2", Language: "ml-IN"},
		},
		Facilities: []Facility{
			{ID: "FAC-1", SafeZone: "SZ-1"},
		},
		Policy: AllocationPolicy{Order: []string{"SZ-1", "SZ-2"}},
		Contacts: []EmergencyContact{
			{Name: "Control", Number: "112"},
		},
	}
	p.Provenance.ChecksumSHA256 = Checksum(p)
	return p
}

func intPtr(v int) *int { return &v }

func validateOK(t *testing.T, p *Package) *Validation {
	t.Helper()
	v, err := Validate(p, "", false, nil)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	return v
}

func validateErr(t *testing.T, p *Package, want string) error {
	t.Helper()
	_, err := Validate(p, "", false, nil)
	if err == nil {
		t.Fatalf("Validate succeeded, want error containing %q", want)
	}
	if want != "" && !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want substring %q", err, want)
	}
	return err
}

// recompute refreshes the checksum after a legitimate mutation.
func recompute(p *Package) { p.Provenance.ChecksumSHA256 = Checksum(p) }

func TestValidateValidPackage(t *testing.T) {
	v := validateOK(t, validPackage())
	if v.PackageID != "PKG-1" {
		t.Errorf("package id = %q", v.PackageID)
	}
	if len(v.SafeZoneIDs) != 2 || len(v.RedZoneIDs) != 1 || len(v.RouteIDs) != 1 {
		t.Errorf("ids = %+v", v)
	}
	if len(v.Languages) != 2 {
		t.Errorf("languages = %v", v.Languages)
	}
}

func TestChecksumIntegrityNotAuthority(t *testing.T) {
	p := validPackage()
	// Tamper with content without recomputing -> checksum mismatch.
	p.Alert.Identifier = "TAMPERED"
	validateErr(t, p, "checksum verification failed")
}

func TestChecksumExcludesSignature(t *testing.T) {
	p := validPackage()
	sumBefore := Checksum(p)
	// Adding/changing a signature must not change the canonical checksum.
	p.Provenance.Signature = &Signature{Algorithm: "Ed25519", KeyID: "k1", Value: "abc"}
	if Checksum(p) != sumBefore {
		t.Error("signature must be excluded from canonical checksum")
	}
}

func TestValidateJurisdictionMismatch(t *testing.T) {
	p := validPackage()
	if _, err := Validate(p, "TN", false, nil); err == nil || !strings.Contains(err.Error(), "jurisdiction") {
		t.Errorf("err = %v, want jurisdiction mismatch", err)
	}
}

func TestValidateJurisdictionMatch(t *testing.T) {
	p := validPackage()
	if _, err := Validate(p, "KL", false, nil); err != nil {
		t.Errorf("Validate with matching jurisdiction: %v", err)
	}
}

func TestValidateSignatureRequired(t *testing.T) {
	p := validPackage()
	// requireSignature but no signature present.
	if _, err := Validate(p, "", true, nil); err == nil || !strings.Contains(err.Error(), "signature is required") {
		t.Errorf("err = %v, want signature required", err)
	}
}

func TestValidateSignatureVerified(t *testing.T) {
	p := validPackage()
	p.Provenance.Signature = &Signature{Algorithm: "Ed25519", KeyID: "k1", Value: "sig"}
	// Verifier that accepts.
	ok, err := Validate(p, "", true, func(pkg *Package, sig *Signature) bool { return true })
	if err != nil || ok == nil {
		t.Errorf("valid signature rejected: %v", err)
	}
	// Verifier that rejects (invalid signature).
	if _, err := Validate(p, "", true, func(pkg *Package, sig *Signature) bool { return false }); err == nil {
		t.Error("invalid signature must be rejected")
	}
}

func TestValidatePolicyMustOrderEverySafeZone(t *testing.T) {
	p := validPackage()
	p.Policy.Order = []string{"SZ-1"} // omits SZ-2
	recompute(p)
	validateErr(t, p, "order every safe zone")
}

func TestValidatePolicyRejectsExtraZone(t *testing.T) {
	p := validPackage()
	p.Policy.Order = []string{"SZ-1", "SZ-2", "SZ-3"} // extra
	recompute(p)
	validateErr(t, p, "order every safe zone")
}

func TestValidateFacilityMustReferenceSafeZone(t *testing.T) {
	p := validPackage()
	p.Facilities[0].SafeZone = "NOPE"
	recompute(p)
	validateErr(t, p, "must reference a safe zone")
}

func TestValidateRouteOriginMustBeRedZone(t *testing.T) {
	p := validPackage()
	p.ApprovedRoutes[0].FromZoneID = "SZ-1" // not a red zone
	recompute(p)
	validateErr(t, p, "origin must reference a red zone")
}

func TestValidateRouteApprovalAuthorized(t *testing.T) {
	p := validPackage()
	p.ApprovedRoutes[0].Approval = "UNVERIFIED"
	recompute(p)
	validateErr(t, p, "approval is not authorized")
}

func TestValidateRouteGeometryLineString(t *testing.T) {
	p := validPackage()
	p.ApprovedRoutes[0].Geometry = []byte(`{"type":"Point","coordinates":[76.0,11.4]}`)
	recompute(p)
	validateErr(t, p, "LineString")
}

func TestValidateRouteGeometryMinPositions(t *testing.T) {
	p := validPackage()
	p.ApprovedRoutes[0].Geometry = []byte(`{"type":"LineString","coordinates":[[76.0,11.4]]}`)
	recompute(p)
	validateErr(t, p, "at least two positions")
}

func TestValidateRouteGeometryBounds(t *testing.T) {
	p := validPackage()
	p.ApprovedRoutes[0].Geometry = []byte(`{"type":"LineString","coordinates":[[200.0,11.4],[76.5,11.5]]}`)
	recompute(p)
	validateErr(t, p, "outside CRS bounds")
}

func TestValidateSafeZoneLocationBounds(t *testing.T) {
	p := validPackage()
	p.SafeZones[0].Location = []float64{76.5, 95.0} // lat 95 invalid
	recompute(p)
	validateErr(t, p, "outside CRS bounds")
}

func TestValidateSafeZoneCapacityExplicit(t *testing.T) {
	p := validPackage()
	p.SafeZones[0].Capacity = nil // missing
	recompute(p)
	validateErr(t, p, "capacity must be explicit")
}

func TestValidateSafeZoneCapacityNonNegative(t *testing.T) {
	p := validPackage()
	p.SafeZones[0].Capacity = intPtr(-5)
	recompute(p)
	validateErr(t, p, "non-negative")
}

func TestValidateDuplicateZoneIDs(t *testing.T) {
	p := validPackage()
	p.SafeZones[1].ID = "SZ-1" // duplicate
	recompute(p)
	validateErr(t, p, "unique")
}

func TestValidateInvalidEvidenceClass(t *testing.T) {
	p := validPackage()
	p.Provenance.EvidenceClass = "BOGUS"
	recompute(p)
	validateErr(t, p, "evidence class")
}

func TestValidateExpiryAfterEffective(t *testing.T) {
	p := validPackage()
	p.Provenance.ExpiresAt = "2026-09-11T04:00:00Z" // before effective
	recompute(p)
	validateErr(t, p, "expiry must be after effective")
}

func TestValidateMissingContacts(t *testing.T) {
	p := validPackage()
	p.Contacts = nil
	recompute(p)
	validateErr(t, p, "emergency contacts are required")
}

func TestValidateInstructionLanguageRequired(t *testing.T) {
	p := validPackage()
	p.Instructions[0].Language = ""
	recompute(p)
	validateErr(t, p, "instruction language is required")
}
