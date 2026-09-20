package offlinepkg

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func generateTestKey(t *testing.T, keyID, jurisdiction string) (ed25519.PrivateKey, TrustedKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	tk := TrustedKey{
		KeyID:                 keyID,
		PublicKey:             pub,
		PermittedJurisdiction: jurisdiction,
		ValidFrom:             time.Now().Add(-1 * time.Hour).UTC(),
		ValidUntil:            time.Now().Add(24 * time.Hour).UTC(),
		Revoked:               false,
	}
	return priv, tk
}

func validManifestFixture(t *testing.T, keyID string, privKey ed25519.PrivateKey) *Manifest {
	t.Helper()
	m := &Manifest{
		SchemaVersion: "3.0",
		ManifestID:    "MAN-KL-2026-09-20-01",
		Jurisdiction:  "KL",
		Revision:      4,
		GeneratedAt:   "2026-09-20T07:00:00Z",
		ValidUntil:    "2026-09-20T19:00:00Z",
		SourceStatus:  "CURRENT",
		CriticalCard: CriticalCardDescriptor{
			PackageID:         "PKG-WAYANAD-2026-V4",
			Version:           4,
			URI:               "/api/v3/packages/PKG-WAYANAD-2026-V4/versions/4",
			ChecksumSHA256:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			UncompressedBytes: 48120,
			CompressedBytes:   12450,
			ContentType:       "application/json",
		},
		Resources: []ResourceDescriptor{
			{
				ResourceID:     "res-map-wayanad-vector-v1",
				Type:           TypeVectorTiles,
				URI:            "/api/v3/resources/res-map-wayanad-vector-v1",
				ChecksumSHA256: "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
				ByteSize:       38450112,
				ContentType:    "application/vnd.mapbox-vector-tile",
				Required:       false,
				Attribution:    "Synthetic test fixture for P5 offline protocol; O06 pending.",
			},
			{
				ResourceID:     "res-style-wayanad-v1",
				Type:           TypeMapStyle,
				URI:            "/api/v3/resources/res-style-wayanad-v1",
				ChecksumSHA256: "9b1e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdedc421",
				ByteSize:       65536,
				ContentType:    "application/json",
				Required:       true,
				Attribution:    "MapLibre style JSON specification; synthetic test fixture.",
			},
		},
		Revocations: RevocationBlock{
			RevokedPackages: []string{"PKG-WAYANAD-2026-V2"},
			CancelledRoutes: []string{"RT-WAYANAD-003"},
			SupersededVersions: []SupersededVersion{
				{PackageID: "PKG-WAYANAD-2026-V3", Version: 3},
			},
		},
		Provenance: ManifestProvenance{
			Authority:     "kerala.sdma.gov.in",
			DatasetID:     "DS-KL-2026-09",
			EvidenceClass: "SYNTHETIC_DEMO",
		},
	}

	can, err := CanonicalBytes(m)
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}
	m.ChecksumSHA256 = ChecksumSHA256(can)

	sig, err := SignCanonical(privKey, keyID, m)
	if err != nil {
		t.Fatalf("SignCanonical: %v", err)
	}
	m.Signature = sig
	return m
}

func validCardFixture(t *testing.T, keyID string, privKey ed25519.PrivateKey) *PublicIncidentCard {
	t.Helper()
	c := &PublicIncidentCard{
		SchemaVersion: "3.0",
		PackageID:     "PKG-WAYANAD-2026-V4",
		Version:       4,
		Jurisdiction:  "KL",
		EvidenceClass: "SYNTHETIC_DEMO",
		EffectiveAt:   "2026-09-20T06:00:00Z",
		ExpiresAt:     "2026-09-21T06:00:00Z",
		Alert: AlertCard{
			Identifier:      "ALERT-KL-2026-09-20-FLOOD-01",
			Sender:          "kerala.sdma.gov.in",
			Headline:        "Flash Flood & Landslide Warning — Meppadi / Chooralmala",
			Severity:        "Severe",
			Urgency:         "Immediate",
			Certainty:       "Observed",
			AreaDescription: "Meppadi Panchayath, Vythiri Taluk, Wayanad",
		},
		RedZones: []RedZoneCard{
			{
				ID:       "RZ-MEPPADI-01",
				Name:     "Chooralmala Riverine Hazard Zone",
				Centroid: []float64{76.1234, 11.5432},
				Geometry: map[string]any{
					"type": "Polygon",
					"coordinates": [][][]float64{
						{{76.12, 11.54}, {76.13, 11.54}, {76.13, 11.55}, {76.12, 11.55}, {76.12, 11.54}},
					},
				},
			},
		},
		SafeZones: []SafeZoneCard{
			{
				ID:            "SZ-MEPPADI-SCHOOL",
				Name:          "St. Joseph GHSS Relief Centre",
				Role:          "EMERGENCY_SHELTER",
				Status:        "OPEN",
				CapacityMode:  "DEFINED",
				TotalCapacity: intPtr(350),
				Location:      []float64{76.1450, 11.5600},
				Services:      []string{"DRINKING_WATER", "MEDICAL_FIRST_AID", "SANITATION"},
			},
		},
		ApprovedRoutes: []RouteCard{
			{
				ID:           "RT-WAYANAD-001",
				FromZoneID:   "RZ-MEPPADI-01",
				ToSafeZoneID: "SZ-MEPPADI-SCHOOL",
				Mode:         "FOOT",
				Approval:     "SYNTHETIC_DEMO",
				VerifiedBy:   "officer.sdma.kl",
				VerifiedAt:   "2026-09-20T06:30:00Z",
				ValidFrom:    "2026-09-20T06:30:00Z",
				ValidUntil:   "2026-09-21T06:00:00Z",
				Geometry: map[string]any{
					"type": "LineString",
					"coordinates": [][]float64{
						{76.125, 11.545}, {76.135, 11.552}, {76.145, 11.560},
					},
				},
			},
		},
		Facilities: []FacilityCard{
			{
				ID:           "FAC-SZ-01",
				SafeZoneID:   "SZ-MEPPADI-SCHOOL",
				Name:         "Main Auditorium Shelter",
				Address:      "Meppadi P.O., Wayanad",
				ContactPhone: "04936202201",
			},
		},
		Instructions: []InstructionCard{
			{
				ID:       "INS-KL-001",
				Language: "ml-IN",
				Title:    "ഉടൻ സുരക്ഷിത സ്ഥാനത്തേക്ക് മാറുക",
				Summary:  "ചൂരൽമല പുഴയോരത്ത് താമസിക്കുന്നവർ ഉടൻ സെന്റ് ജോസഫ് സ്കൂളിലെ ദുരിതാശ്വാസ ക്യാമ്പിലേക്ക് മാറുക.",
			},
			{
				ID:       "INS-EN-001",
				Language: "en-IN",
				Title:    "Immediate Evacuation Advisory",
				Summary:  "Residents near Chooralmala river basin must relocate immediately to St. Joseph School Shelter.",
			},
		},
		EmergencyContacts: []EmergencyContact{
			{Name: "District Disaster Control Room Wayanad", Number: "1077"},
			{Name: "Emergency Police / Fire / Ambulance", Number: "112"},
		},
		AllocationPolicy: PolicyCard{
			Order:                    []string{"SZ-MEPPADI-SCHOOL"},
			ReservationExpirySeconds: intPtr(3600),
			TemporaryStayMinDays:     intPtr(7),
			TemporaryStayMaxDays:     intPtr(30),
			AllowWalkIns:             boolPtr(true),
			AllowTransfers:           boolPtr(false),
			RouteRequired:            boolPtr(true),
		},
	}

	can, err := CanonicalBytes(c)
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}
	c.ChecksumSHA256 = ChecksumSHA256(can)

	sig, err := SignCanonical(privKey, keyID, c)
	if err != nil {
		t.Fatalf("SignCanonical: %v", err)
	}
	c.Signature = sig
	return c
}

func intPtr(i int) *int    { return &i }
func boolPtr(b bool) *bool { return &b }
