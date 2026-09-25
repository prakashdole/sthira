package offlinepkg

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"
)

func TestCriticalCardCompressedBudget(t *testing.T) {
	priv, _ := generateTestKey(t, "key-kl-01", "KL")

	// Build a comprehensive, realistic multi-zone, multi-route, bilingual emergency incident card.
	card := validCardFixture(t, "key-kl-01", priv)

	// Enrich with realistic flood scenario data:
	// - 2 red hazard zones with coordinate polygons
	// - 3 safe zones with facilities, locations, and services
	// - 3 approved routes with LineString coordinates
	// - 3 instructions (English, Malayalam, Hindi)
	// - 3 emergency contacts
	card.RedZones = append(card.RedZones, RedZoneCard{
		ID:       "RZ-CHOORALMALA-02",
		Name:     "Upper Chooralmala Hillside Hazard Zone",
		Centroid: []float64{76.1350, 11.5320},
		Geometry: map[string]any{
			"type": "Polygon",
			"coordinates": [][][]float64{
				{{76.130, 11.530}, {76.140, 11.530}, {76.140, 11.535}, {76.130, 11.535}, {76.130, 11.530}},
			},
		},
	})

	card.SafeZones = append(card.SafeZones,
		SafeZoneCard{
			ID:            "SZ-VYTHIRI-TALUK",
			Name:          "Vythiri Taluk Community Relief Centre",
			Role:          "EMERGENCY_SHELTER",
			Status:        "OPEN",
			CapacityMode:  "DEFINED",
			TotalCapacity: intPtr(500),
			Location:      []float64{76.0400, 11.5500},
			Services:      []string{"DRINKING_WATER", "MEDICAL_FIRST_AID", "COMMUNITY_KITCHEN", "SANITATION"},
		},
		SafeZoneCard{
			ID:            "SZ-CHUNDALE-GROUND",
			Name:          "Chundale Panchayat Ground Relief Camp",
			Role:          "TEMPORARY_ACCOMMODATION",
			Status:        "OPEN",
			CapacityMode:  "DEFINED",
			TotalCapacity: intPtr(250),
			Location:      []float64{76.0700, 11.5700},
			Services:      []string{"DRINKING_WATER", "BLANKETS", "SANITATION"},
		},
	)

	card.ApprovedRoutes = append(card.ApprovedRoutes,
		RouteCard{
			ID:           "RT-WAYANAD-002",
			FromZoneID:   "RZ-CHOORALMALA-02",
			ToSafeZoneID: "SZ-VYTHIRI-TALUK",
			Mode:         "VEHICLE",
			Approval:     "SYNTHETIC_DEMO",
			VerifiedBy:   "ddma.field.officer.12",
			VerifiedAt:   "2026-09-20T06:45:00Z",
			ValidFrom:    "2026-09-20T06:45:00Z",
			ValidUntil:   "2026-09-21T06:00:00Z",
			Geometry: map[string]any{
				"type": "LineString",
				"coordinates": [][]float64{
					{76.130, 11.530}, {76.100, 11.540}, {76.060, 11.545}, {76.040, 11.550},
				},
			},
		},
		RouteCard{
			ID:           "RT-WAYANAD-003",
			FromZoneID:   "RZ-MEPPADI-01",
			ToSafeZoneID: "SZ-CHUNDALE-GROUND",
			Mode:         "FOOT",
			Approval:     "SYNTHETIC_DEMO",
			VerifiedBy:   "ddma.field.officer.05",
			VerifiedAt:   "2026-09-20T07:00:00Z",
			ValidFrom:    "2026-09-20T07:00:00Z",
			ValidUntil:   "2026-09-21T06:00:00Z",
			Geometry: map[string]any{
				"type": "LineString",
				"coordinates": [][]float64{
					{76.125, 11.545}, {76.100, 11.555}, {76.070, 11.570},
				},
			},
		},
	)

	card.Facilities = append(card.Facilities,
		FacilityCard{
			ID:           "FAC-VYTHIRI-01",
			SafeZoneID:   "SZ-VYTHIRI-TALUK",
			Name:         "Taluk Hospital Annex Relief Wing",
			Address:      "Vythiri, Wayanad",
			ContactPhone: "04936255100",
		},
		FacilityCard{
			ID:           "FAC-CHUNDALE-01",
			SafeZoneID:   "SZ-CHUNDALE-GROUND",
			Name:         "Panchayat Community Hall",
			Address:      "Chundale P.O., Wayanad",
			ContactPhone: "04936277200",
		},
	)

	card.Instructions = append(card.Instructions,
		InstructionCard{
			ID:       "INS-HI-001",
			Language: "hi-IN",
			Title:    "तत्काल सुरक्षित स्थान पर जाएं",
			Summary:  "चूरलमाला नदी तट के निवासी तुरंत सेंट जोसेफ स्कूल राहत शिविर में स्थानांतरित हों।",
		},
	)

	card.AllocationPolicy.Order = []string{"SZ-MEPPADI-SCHOOL", "SZ-VYTHIRI-TALUK", "SZ-CHUNDALE-GROUND"}

	// Recompute checksum and re-sign
	can, err := CanonicalBytes(card)
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}
	card.ChecksumSHA256 = ChecksumSHA256(can)
	sig, err := SignCanonical(priv, "key-kl-01", card)
	if err != nil {
		t.Fatalf("SignCanonical: %v", err)
	}
	card.Signature = sig

	// Marshal to JSON
	cardJSON, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("Marshal card: %v", err)
	}
	uncompressedSize := len(cardJSON)

	// Gzip compress
	var gzipBuf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&gzipBuf, gzip.BestCompression)
	if err != nil {
		t.Fatalf("gzip.NewWriterLevel: %v", err)
	}
	if _, err := gw.Write(cardJSON); err != nil {
		t.Fatalf("gw.Write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gw.Close: %v", err)
	}

	compressedSize := gzipBuf.Len()
	const maxBudget = 65536 // 64 KiB

	t.Logf("Measured Critical Incident Card Size: Uncompressed = %d bytes (%.2f KiB), Compressed = %d bytes (%.2f KiB), Budget = %d bytes (64 KiB)",
		uncompressedSize, float64(uncompressedSize)/1024,
		compressedSize, float64(compressedSize)/1024,
		maxBudget)

	if compressedSize > maxBudget {
		t.Fatalf("Compressed card size %d exceeds 64 KiB budget limit (%d bytes)", compressedSize, maxBudget)
	}
}
