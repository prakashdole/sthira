package offlineresources_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"sthira/backend/internal/offlinepkg"
	"sthira/backend/internal/offlineresources"
)

func sampleIncidentCard() offlinepkg.PublicIncidentCard {
	cap100 := 100
	return offlinepkg.PublicIncidentCard{
		SchemaVersion: "3.0",
		PackageID:     "PKG-WYD-FLOOD-01",
		Version:       1,
		Jurisdiction:  "KL-WYD",
		EvidenceClass: "AUTHORIZED_OPERATIONAL",
		EffectiveAt:   "2026-09-24T12:00:00Z",
		ExpiresAt:     "2026-09-25T12:00:00Z",
		Alert: offlinepkg.AlertCard{
			Identifier:      "NDMA-KL-2026-001",
			Sender:          "sachet@ndma.gov.in",
			Headline:        "Severe Flash Flood Warning in Meppadi",
			Severity:        "Extreme",
			Urgency:         "Immediate",
			Certainty:       "Observed",
			AreaDescription: "Wayanad District, Meppadi and Chooralmala Wards",
		},
		RedZones: []offlinepkg.RedZoneCard{
			{ID: "RZ-01", Name: "Chooralmala Riverbank"},
		},
		SafeZones: []offlinepkg.SafeZoneCard{
			{
				ID:            "SZ-01",
				Name:          "Meppadi Higher Secondary School",
				Role:          "PRIMARY_RELIEF_CAMP",
				Status:        "OPERATIONAL",
				CapacityMode:  "FIXED",
				TotalCapacity: &cap100,
				Services:      []string{"DRINKING_WATER", "FIRST_AID", "SANITATION"},
			},
		},
		Facilities: []offlinepkg.FacilityCard{
			{
				ID:           "FAC-01",
				SafeZoneID:   "SZ-01",
				Name:         "Main Auditorium",
				Address:      "Meppadi Town Center",
				ContactPhone: "04936-202112",
			},
		},
		ApprovedRoutes: []offlinepkg.RouteCard{
			{
				ID:           "RT-01",
				FromZoneID:   "RZ-01",
				ToSafeZoneID: "SZ-01",
				Mode:         "FOOT_AND_VEHICLE",
				Approval:     "GOVERNMENT_VERIFIED",
			},
		},
		Instructions: []offlinepkg.InstructionCard{
			{
				ID:       "INS-01",
				Language: "en-IN",
				Title:    "Evacuate Immediately",
				Summary:  "Follow marked relief routes to Meppadi HSS. Do not cross swollen culverts.",
			},
		},
		EmergencyContacts: []offlinepkg.EmergencyContact{
			{Name: "Disaster Control Room", Number: "1077"},
		},
	}
}

func TestValidateCardCompleteness(t *testing.T) {
	card := sampleIncidentCard()
	if err := offlineresources.ValidateCardCompleteness(card); err != nil {
		t.Fatalf("expected complete card to pass validation, got: %v", err)
	}

	// Missing PackageID
	noPkg := card
	noPkg.PackageID = ""
	if err := offlineresources.ValidateCardCompleteness(noPkg); !errors.Is(err, offlineresources.ErrMissingPackageID) {
		t.Fatalf("expected ErrMissingPackageID, got: %v", err)
	}

	// Missing Jurisdiction
	noJur := card
	noJur.Jurisdiction = ""
	if err := offlineresources.ValidateCardCompleteness(noJur); !errors.Is(err, offlineresources.ErrMissingJurisdiction) {
		t.Fatalf("expected ErrMissingJurisdiction, got: %v", err)
	}

	// Missing Validity
	noVal := card
	noVal.ExpiresAt = ""
	if err := offlineresources.ValidateCardCompleteness(noVal); !errors.Is(err, offlineresources.ErrMissingValidity) {
		t.Fatalf("expected ErrMissingValidity, got: %v", err)
	}

	// No Safe Zones
	noSZ := card
	noSZ.SafeZones = nil
	if err := offlineresources.ValidateCardCompleteness(noSZ); !errors.Is(err, offlineresources.ErrNoSafeZones) {
		t.Fatalf("expected ErrNoSafeZones, got: %v", err)
	}
}

func TestFormatPlaintextCard_StructureAndDeterminism(t *testing.T) {
	card := sampleIncidentCard()
	txt1, err := offlineresources.FormatPlaintextCard(card)
	if err != nil {
		t.Fatalf("failed to format plaintext card: %v", err)
	}

	txt2, err := offlineresources.FormatPlaintextCard(card)
	if err != nil {
		t.Fatalf("failed to format plaintext card second time: %v", err)
	}

	if txt1 != txt2 {
		t.Fatalf("card formatting must be strictly deterministic")
	}

	// Required content checks
	requiredStrings := []string{
		"OFFICIAL DISASTER EVACUATION GUIDANCE CARD",
		"KL-WYD",
		"PKG-WYD-FLOOD-01",
		"Severe Flash Flood Warning in Meppadi",
		"112 - Unified Emergency Response Support System (ERSS)",
		"1077 - Disaster Control Room",
		"Chooralmala Riverbank",
		"Meppadi Higher Secondary School",
		"100 beds",
		"Main Auditorium",
		"Route RT-01",
		"Evacuate Immediately",
		"Sthira never places automated calls",
	}

	for _, req := range requiredStrings {
		if !strings.Contains(txt1, req) {
			t.Errorf("plaintext card missing required string: %q", req)
		}
	}
}

func TestFormatMarkdownCard_Structure(t *testing.T) {
	card := sampleIncidentCard()
	md, err := offlineresources.FormatMarkdownCard(card)
	if err != nil {
		t.Fatalf("failed to format markdown card: %v", err)
	}

	if !strings.Contains(md, "# Official Disaster Evacuation Guidance") {
		t.Errorf("missing markdown title")
	}
	if !strings.Contains(md, "[112](tel:112)") {
		t.Errorf("missing markdown 112 dialler link")
	}
	if !strings.Contains(md, "Meppadi Higher Secondary School") {
		t.Errorf("missing safe zone name")
	}
}

func TestCardDigestAndTamperVerification(t *testing.T) {
	card := sampleIncidentCard()
	txt, err := offlineresources.FormatPlaintextCard(card)
	if err != nil {
		t.Fatalf("failed to format card: %v", err)
	}

	digest := offlineresources.ComputeCardTextDigest(txt)
	if len(digest) != 64 {
		t.Fatalf("expected 64-character SHA-256 hex digest, got %q", digest)
	}

	// Successful verification
	if err := offlineresources.VerifyCardTextIntegrity(txt, digest); err != nil {
		t.Fatalf("expected checksum verification to pass, got: %v", err)
	}

	// Tampered content fails
	tamperedTxt := strings.Replace(txt, "100 beds", "0 beds", 1)
	if err := offlineresources.VerifyCardTextIntegrity(tamperedTxt, digest); !errors.Is(err, offlineresources.ErrCardChecksumMismatch) {
		t.Fatalf("expected ErrCardChecksumMismatch on tampered text, got: %v", err)
	}
}

func TestCardGen_Concurrent(t *testing.T) {
	card := sampleIncidentCard()
	var wg sync.WaitGroup

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			txt, err := offlineresources.FormatPlaintextCard(card)
			if err != nil {
				return
			}
			digest := offlineresources.ComputeCardTextDigest(txt)
			_ = offlineresources.VerifyCardTextIntegrity(txt, digest)
		}()
	}

	wg.Wait()
}
