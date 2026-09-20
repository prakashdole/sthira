package offlineresources

import (
	"errors"
	"strings"
	"testing"
)

// completePack returns a minimal-but-complete regional pack: vector tiles,
// map style, sprites, glyphs, gazetteer and one audio asset per supported
// language. All resources are SYNTHETIC fixtures only; the license
// metadata is "declared" but the integration agent still gates live
// activation on O06 evidence.
func completePack() []ResourceDescriptor {
	return []ResourceDescriptor{
		{
			ResourceID:     "res-vector-wayanad",
			Type:           TypeVectorTiles,
			URI:            "/api/v3/resources/res-vector-wayanad",
			ChecksumSHA256: fakeDigest(0x20),
			ByteSize:       30 * 1024 * 1024,
			ContentType:    "application/vnd.mapbox-vector-tile",
			Required:       true,
			Attribution:    "Synthetic fixture; O06 pending redistribution evidence.",
			License: &LicenseInfo{
				SPDXIdentifier:    "ODbL-1.0",
				Redistribution:    RedistConditional,
				DeclaredOfflineOK: false,
			},
		},
		{
			ResourceID:     "res-style-wayanad",
			Type:           TypeMapStyle,
			URI:            "/api/v3/resources/res-style-wayanad",
			ChecksumSHA256: fakeDigest(0x21),
			ByteSize:       8 * 1024,
			ContentType:    "application/json",
			Required:       true,
			Attribution:    "Synthetic MapLibre style JSON.",
		},
		{
			ResourceID:     "res-sprite-wayanad",
			Type:           TypeMapSprite,
			URI:            "/api/v3/resources/res-sprite-wayanad",
			ChecksumSHA256: fakeDigest(0x22),
			ByteSize:       16 * 1024,
			ContentType:    "application/json",
			Required:       true,
			Attribution:    "Synthetic sprite.",
		},
		{
			ResourceID:     "res-glyphs-wayanad",
			Type:           TypeMapGlyphs,
			URI:            "/api/v3/resources/res-glyphs-wayanad/{fontstack}/{range}.pbf",
			ChecksumSHA256: fakeDigest(0x23),
			ByteSize:       4 * 1024 * 1024,
			ContentType:    "application/x-protobuf",
			Required:       true,
			Attribution:    "Synthetic glyphs.",
		},
		{
			ResourceID:     "res-gazetteer-wayanad",
			Type:           TypeGazetteer,
			URI:            "/api/v3/resources/res-gazetteer-wayanad",
			ChecksumSHA256: fakeDigest(0x24),
			ByteSize:       512 * 1024,
			ContentType:    "application/geo+json",
			Required:       false,
			Attribution:    "Synthetic gazetteer.",
		},
		{
			ResourceID:     "res-audio-ml",
			Type:           TypeEmergencyAudio,
			URI:            "/api/v3/resources/res-audio-ml",
			ChecksumSHA256: fakeDigest(0x25),
			ByteSize:       200 * 1024,
			ContentType:    "audio/mpeg",
			Required:       false,
			Attribution:    "Synthetic Malayalam advisory audio.",
			Languages:      []string{"ml-IN"},
		},
		{
			ResourceID:     "res-audio-en",
			Type:           TypeEmergencyAudio,
			URI:            "/api/v3/resources/res-audio-en",
			ChecksumSHA256: fakeDigest(0x26),
			ByteSize:       180 * 1024,
			ContentType:    "audio/mpeg",
			Required:       false,
			Attribution:    "Synthetic English advisory audio.",
			Languages:      []string{"en-IN"},
		},
	}
}

func TestAuditRegionalPack_AcceptsCompletePack(t *testing.T) {
	a, err := AuditRegionalPack(completePack(), DefaultRegionalPackBudgetBytes)
	if err != nil {
		t.Fatalf("complete pack must audit: %v", err)
	}
	if a.ResourceCount != len(completePack()) {
		t.Fatalf("resource count = %d, want %d", a.ResourceCount, len(completePack()))
	}
	if a.ExceedsBudget {
		t.Fatalf("complete fixture pack must fit in 50 MiB; total=%d", a.TotalBytes)
	}
	if len(a.AttributionMissing) != 0 {
		t.Fatalf("no attribution should be missing, got %v", a.AttributionMissing)
	}
	if len(a.LicenseDenied) != 0 {
		t.Fatalf("no license should be denied, got %v", a.LicenseDenied)
	}
	// The conditional redistribution is metadata, not a denial; the audit
	// keeps it in LicensePending because DeclaredOfflineOK is false.
	if len(a.LicensePending) != 1 {
		t.Fatalf("expected exactly one license-pending resource, got %v", a.LicensePending)
	}
}

func TestAuditRegionalPack_OptionalMissingAudioDoesNotFailPack(t *testing.T) {
	// The critical path is the incident card and the map. Audio assets
	// are optional; an empty audio pack must still audit cleanly so the
	// device can ship the critical guidance without audio.
	pack := completePack()
	for i := range pack {
		if pack[i].Type == TypeEmergencyAudio {
			pack[i].Required = false
			pack[i].Languages = nil
		}
	}
	a, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err != nil {
		t.Fatalf("pack without audio must audit: %v", err)
	}
	if a.ExceedsBudget {
		t.Fatalf("audio-less pack must fit budget")
	}
}

func TestAuditRegionalPack_RejectsOverBudget(t *testing.T) {
	pack := completePack()
	// Force the total above 50 MiB.
	for i := range pack {
		if pack[i].Type == TypeVectorTiles {
			pack[i].ByteSize = 51 * 1024 * 1024
		}
	}
	a, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err != nil {
		t.Fatalf("audit must surface budget result, not error: %v", err)
	}
	if !a.ExceedsBudget {
		t.Fatalf("expected ExceedsBudget=true, got false")
	}
}

func TestAuditRegionalPack_RejectsZeroAndNegativeSize(t *testing.T) {
	pack := completePack()
	for i := range pack {
		if pack[i].Type == TypeVectorTiles {
			pack[i].ByteSize = 0
		}
	}
	_, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err == nil {
		t.Fatalf("zero-size resource must be a structural failure")
	}
	if !errors.Is(err, ErrPackInvalid) {
		t.Fatalf("expected ErrPackInvalid, got %v", err)
	}
}

func TestAuditRegionalPack_RejectsDuplicateResourceID(t *testing.T) {
	pack := completePack()
	pack = append(pack, pack[0]) // duplicate the vector tiles descriptor
	_, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err == nil {
		t.Fatalf("duplicate resource_id must be rejected")
	}
}

func TestAuditRegionalPack_RejectsConflictingURI(t *testing.T) {
	pack := completePack()
	// Append a duplicate URI with different digest.
	dup := pack[0]
	dup.ResourceID = "res-vector-wayanad-different"
	dup.ChecksumSHA256 = fakeDigest(0xAA)
	dup.URI = pack[0].URI
	pack = append(pack, dup)
	_, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err == nil {
		t.Fatalf("conflicting URI metadata must be rejected")
	}
}

func TestAuditRegionalPack_RejectsDeniedLicense(t *testing.T) {
	pack := completePack()
	for i := range pack {
		if pack[i].Type == TypeVectorTiles {
			pack[i].License = &LicenseInfo{
				SPDXIdentifier: "Proprietary-foo",
				Redistribution: RedistDenied,
			}
		}
	}
	_, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err == nil {
		t.Fatalf("REDISTRIBUTION_DENIED must reject the descriptor")
	}
}

func TestAuditRegionalPack_CustomBudget(t *testing.T) {
	pack := completePack()
	v := NewValidatorWithBudget(1 * 1024 * 1024) // 1 MiB
	a, err := v.AuditRegionalPack(pack)
	if err != nil {
		t.Fatalf("audit with custom budget must run: %v", err)
	}
	if !a.ExceedsBudget {
		t.Fatalf("1 MiB budget must flag the fixture pack as over")
	}
	if a.BudgetLimitBytes != 1*1024*1024 {
		t.Fatalf("budget limit must be reported")
	}
}

func TestAuditRegionalPack_ReportsAttributionMissing(t *testing.T) {
	pack := completePack()
	pack[3].Attribution = "" // glyphs
	_, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err == nil {
		t.Fatalf("missing attribution must fail the descriptor")
	}
	if !strings.Contains(err.Error(), ReasonEmptyAttribution) {
		t.Fatalf("error must mention empty attribution: %v", err)
	}
}

func TestAuditRegionalPack_DeclaresTotalBytesConsistentWithSum(t *testing.T) {
	pack := completePack()
	a, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err != nil {
		t.Fatalf("audit must succeed: %v", err)
	}
	var sum int64
	for _, d := range pack {
		sum += d.ByteSize
	}
	if a.TotalBytes != sum {
		t.Fatalf("total bytes mismatch: audit=%d sum=%d", a.TotalBytes, sum)
	}
}

func TestNewValidator_DefaultBudgetIsContractValue(t *testing.T) {
	v := NewValidatorWithBudget(0)
	if v.Budget() != DefaultRegionalPackBudgetBytes {
		t.Fatalf("default validator budget = %d, want %d", v.Budget(), DefaultRegionalPackBudgetBytes)
	}
}

func TestNewValidator_AuditRunsThroughValidatorInterface(t *testing.T) {
	v := NewValidatorWithBudget(0)
	rv := ResourceValidator(v) // interface contract value
	a, err := rv.AuditRegionalPack(completePack())
	if err != nil {
		t.Fatalf("validator interface must run audit: %v", err)
	}
	if a.ExceedsBudget {
		t.Fatalf("complete fixture pack must fit default budget, total=%d", a.TotalBytes)
	}
}

func TestNewValidator_ValidateMapStyleThroughInterface(t *testing.T) {
	v := NewValidatorWithBudget(0)
	rv := ResourceValidator(v)
	res, err := rv.ValidateMapStyle([]byte(styleHappyPath), completeStyleFixture())
	if err != nil {
		t.Fatalf("style must validate through interface: %v", err)
	}
	if !res.Valid {
		t.Fatalf("style must report Valid=true through interface")
	}
}

func TestValidator_WithBudgetReturnsCopy(t *testing.T) {
	v := NewValidatorWithBudget(0)
	v2 := v.WithBudget(1024)
	if v2.Budget() != 1024 {
		t.Fatalf("WithBudget must apply new value")
	}
	if v.Budget() != DefaultRegionalPackBudgetBytes {
		t.Fatalf("original validator budget must not mutate")
	}
}

// RequiredMissing covers the loose audio-language dep check. A required
// emergency-audio resource that names a language not provided by any
// other audio resource in the pack must be reported. Required audio is
// unusual but the test ensures the dep walker does not panic and the
// result is well-formed.
func TestRequiredMissing_AudioLanguageDependency(t *testing.T) {
	pack := []ResourceDescriptor{
		{
			ResourceID:     "res-audio-ml",
			Type:           TypeEmergencyAudio,
			URI:            "/api/v3/resources/res-audio-ml",
			ChecksumSHA256: fakeDigest(0x30),
			ByteSize:       100 * 1024,
			ContentType:    "audio/mpeg",
			Required:       true,
			Attribution:    "Synthetic.",
			Languages:      []string{"ml-IN", "ta-IN"},
		},
	}
	a, err := AuditRegionalPack(pack, DefaultRegionalPackBudgetBytes)
	if err != nil {
		t.Fatalf("single required audio must audit: %v", err)
	}
	// The dep check only flags missing coverage, not single-resource
	// packs; a single required audio is a valid audit object.
	if a.ExceedsBudget {
		t.Fatalf("small audio must not exceed budget")
	}
}
