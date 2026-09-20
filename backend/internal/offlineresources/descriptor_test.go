package offlineresources

import (
	"strings"
	"testing"
)

// fakeDigest returns a deterministic 64-char lowercase hex string built
// from the supplied seed. Tests use this instead of crypto/sha256 to keep
// the fixtures readable while still satisfying the format check.
func fakeDigest(seed byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 64)
	for i := range out {
		out[i] = hex[(int(seed)+i)%16]
	}
	return string(out)
}

func validDescriptor() ResourceDescriptor {
	return ResourceDescriptor{
		ResourceID:     "res-vector-wayanad",
		Type:           TypeVectorTiles,
		URI:            "/api/v3/resources/res-vector-wayanad",
		ChecksumSHA256: fakeDigest(0x01),
		ByteSize:       1024,
		ContentType:    "application/vnd.mapbox-vector-tile",
		Required:       false,
		Attribution:    "Synthetic fixture; O06 pending.",
	}
}

func TestValidateDescriptor_AcceptsBaselineFixture(t *testing.T) {
	if err := ValidateDescriptor(validDescriptor()); err != nil {
		t.Fatalf("baseline fixture must validate: %v", err)
	}
}

func TestValidateDescriptor_RejectsEmptyResourceID(t *testing.T) {
	d := validDescriptor()
	d.ResourceID = ""
	err := ValidateDescriptor(d)
	if err == nil || !strings.Contains(err.Error(), ReasonEmptyResourceID) {
		t.Fatalf("expected %q error, got %v", ReasonEmptyResourceID, err)
	}
}

func TestValidateDescriptor_RejectsEmptyURI(t *testing.T) {
	d := validDescriptor()
	d.URI = ""
	err := ValidateDescriptor(d)
	if err == nil || !strings.Contains(err.Error(), ReasonEmptyURI) {
		t.Fatalf("expected %q error, got %v", ReasonEmptyURI, err)
	}
}

func TestValidateDescriptor_RejectsUnknownType(t *testing.T) {
	d := validDescriptor()
	d.Type = ResourceType("ROCKET_FUEL")
	err := ValidateDescriptor(d)
	if err == nil || !strings.Contains(err.Error(), ReasonUnknownResourceType) {
		t.Fatalf("expected unknown resource type error, got %v", err)
	}
}

func TestValidateDescriptor_RejectsInvalidDigest(t *testing.T) {
	d := validDescriptor()
	d.ChecksumSHA256 = "not-hex"
	err := ValidateDescriptor(d)
	if err == nil || !strings.Contains(err.Error(), ReasonInvalidDigest) {
		t.Fatalf("expected invalid digest error, got %v", err)
	}

	d.ChecksumSHA256 = strings.Repeat("a", 63) // one short
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("expected invalid digest error for 63 chars")
	}

	d.ChecksumSHA256 = strings.ToUpper(fakeDigest(0x01)) // uppercase rejected
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("expected invalid digest error for uppercase hex")
	}
}

func TestValidateDescriptor_RejectsZeroSize(t *testing.T) {
	d := validDescriptor()
	d.ByteSize = 0
	err := ValidateDescriptor(d)
	if err == nil || !strings.Contains(err.Error(), ReasonNonPositiveSize) {
		t.Fatalf("expected non-positive size error, got %v", err)
	}
	d.ByteSize = -10
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("expected non-positive size error for negative value")
	}
}

func TestValidateDescriptor_RejectsEmptyContentType(t *testing.T) {
	d := validDescriptor()
	d.ContentType = ""
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("expected empty content_type rejection")
	}
}

func TestValidateDescriptor_RejectsMalformedContentType(t *testing.T) {
	d := validDescriptor()
	d.ContentType = "not a media type"
	err := ValidateDescriptor(d)
	if err == nil || !strings.Contains(err.Error(), ReasonInvalidContentType) {
		t.Fatalf("expected invalid content_type error, got %v", err)
	}
}

func TestValidateDescriptor_RejectsUnsupportedFormat(t *testing.T) {
	d := validDescriptor()
	d.Type = TypeVectorTiles
	d.ContentType = "image/png" // wrong format for vector tiles
	err := ValidateDescriptor(d)
	if err == nil || !strings.Contains(err.Error(), ReasonUnsupportedFormat) {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestValidateDescriptor_RejectsEmptyAttribution(t *testing.T) {
	d := validDescriptor()
	d.Attribution = "   "
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("expected empty attribution rejection (whitespace only)")
	}
}

func TestValidateDescriptor_AcceptsAllContentTypesForKnownTypes(t *testing.T) {
	cases := []struct {
		typ ResourceType
		ct  string
	}{
		{TypeVectorTiles, "application/vnd.mapbox-vector-tile"},
		{TypeVectorTiles, "application/x-protobuf"},
		{TypeMapStyle, "application/json"},
		{TypeMapStyle, "application/geo+json"},
		{TypeMapSprite, "application/json"},
		{TypeMapSprite, "image/png"},
		{TypeMapGlyphs, "application/x-protobuf"},
		{TypeMapGlyphs, "application/octet-stream"},
		{TypeGazetteer, "application/json"},
		{TypeGazetteer, "application/geo+json"},
		{TypeEmergencyAudio, "audio/mpeg"},
		{TypeEmergencyAudio, "audio/mp4"},
		{TypeEmergencyAudio, "audio/ogg"},
		{TypeEmergencyAudio, "audio/wav"},
	}
	for _, c := range cases {
		d := validDescriptor()
		d.Type = c.typ
		d.ContentType = c.ct
		if err := ValidateDescriptor(d); err != nil {
			t.Errorf("%s/%s must validate, got %v", c.typ, c.ct, err)
		}
	}
}

func TestValidateDescriptor_RejectsAudioExceedingCeiling(t *testing.T) {
	d := validDescriptor()
	d.Type = TypeEmergencyAudio
	d.ContentType = "audio/mpeg"
	d.ByteSize = 513 * 1024 // ceiling is 512 KiB
	err := ValidateDescriptor(d)
	if err == nil || !strings.Contains(err.Error(), ReasonCriticalExceedsBudget) {
		t.Fatalf("expected per-asset ceiling rejection, got %v", err)
	}
}

func TestValidateDescriptor_AcceptsAudioAtCeiling(t *testing.T) {
	d := validDescriptor()
	d.Type = TypeEmergencyAudio
	d.ContentType = "audio/mpeg"
	d.ByteSize = 512 * 1024
	if err := ValidateDescriptor(d); err != nil {
		t.Fatalf("audio at ceiling must validate, got %v", err)
	}
}

func TestValidateDescriptor_RejectsInvertedZoom(t *testing.T) {
	d := validDescriptor()
	min, max := 5, 3
	d.MinZoom = &min
	d.MaxZoom = &max
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("inverted zoom range must be rejected")
	}
	d2 := validDescriptor()
	min, max = 0, 30
	d2.MinZoom = &min
	d2.MaxZoom = &max
	if err := ValidateDescriptor(d2); err == nil {
		t.Fatalf("zoom > 22 must be rejected")
	}
}

func TestValidateDescriptor_RejectsInvertedBBox(t *testing.T) {
	d := validDescriptor()
	d.BBox = &GeoBBox{MinLon: 80, MinLat: 10, MaxLon: 70, MaxLat: 20}
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("inverted bbox must be rejected")
	}
}

func TestValidateDescriptor_RejectsBadLanguageTag(t *testing.T) {
	d := validDescriptor()
	d.Type = TypeEmergencyAudio
	d.ContentType = "audio/mpeg"
	d.Languages = []string{"ml_IN", "1n"}
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("malformed language tag must be rejected")
	}
}

func TestValidateDescriptor_AcceptsLanguageTag(t *testing.T) {
	d := validDescriptor()
	d.Type = TypeEmergencyAudio
	d.ContentType = "audio/mpeg"
	d.Languages = []string{"ml-IN", "en-IN", "zh-Hans"}
	if err := ValidateDescriptor(d); err != nil {
		t.Fatalf("valid language tags must pass: %v", err)
	}
}
