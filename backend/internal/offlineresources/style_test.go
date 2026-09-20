package offlineresources

import (
	"encoding/json"
	"strings"
	"testing"
)

const styleHappyPath = `{
  "version": 8,
  "name": "wayanad-basemap",
  "sprite": "res-sprite-wayanad",
  "glyphs": "res-glyphs-wayanad/{fontstack}/{range}.pbf",
  "sources": {
    "osm": {
      "type": "vector",
      "url": "res-vector-wayanad"
    }
  },
  "layers": []
}`

const styleNoReferences = `{
  "version": 8,
  "name": "minimal",
  "sources": {},
  "layers": []
}`

func completeStyleFixture() []ResourceDescriptor {
	return []ResourceDescriptor{
		{
			ResourceID:     "res-vector-wayanad",
			Type:           TypeVectorTiles,
			URI:            "res-vector-wayanad",
			ChecksumSHA256: fakeDigest(0x10),
			ByteSize:       1024,
			ContentType:    "application/vnd.mapbox-vector-tile",
			Required:       true,
			Attribution:    "Synthetic fixture; O06 pending.",
		},
		{
			ResourceID:     "res-sprite-wayanad",
			Type:           TypeMapSprite,
			URI:            "res-sprite-wayanad",
			ChecksumSHA256: fakeDigest(0x11),
			ByteSize:       256,
			ContentType:    "application/json",
			Required:       true,
			Attribution:    "Synthetic sprite.",
		},
		{
			ResourceID:     "res-glyphs-wayanad",
			Type:           TypeMapGlyphs,
			URI:            "res-glyphs-wayanad/{fontstack}/{range}.pbf",
			ChecksumSHA256: fakeDigest(0x12),
			ByteSize:       512,
			ContentType:    "application/x-protobuf",
			Required:       true,
			Attribution:    "Synthetic glyphs.",
		},
	}
}

func TestValidateMapStyle_AcceptsCompleteDependencySet(t *testing.T) {
	res, err := ValidateMapStyle([]byte(styleHappyPath), completeStyleFixture(), false)
	if err != nil {
		t.Fatalf("complete style must validate: %v", err)
	}
	if !res.Valid {
		t.Fatalf("complete style must report Valid=true, got %+v", res)
	}
	if len(res.MissingSprites)+len(res.MissingGlyphs)+len(res.MissingSources) != 0 {
		t.Fatalf("no deps should be missing, got %+v", res)
	}
}

func TestValidateMapStyle_RejectsEmptyJSON(t *testing.T) {
	if _, err := ValidateMapStyle(nil, completeStyleFixture(), false); err == nil {
		t.Fatalf("empty JSON must be rejected")
	}
}

func TestValidateMapStyle_RejectsMalformedJSON(t *testing.T) {
	if _, err := ValidateMapStyle([]byte("{not json"), completeStyleFixture(), false); err == nil {
		t.Fatalf("malformed JSON must be rejected")
	}
}

func TestValidateMapStyle_RejectsTrailingData(t *testing.T) {
	bad := []byte(styleHappyPath + ` {"oops":true}`)
	if _, err := ValidateMapStyle(bad, completeStyleFixture(), false); err == nil {
		t.Fatalf("trailing data must be rejected")
	}
}

func TestValidateMapStyle_AcceptsNoReferences(t *testing.T) {
	res, err := ValidateMapStyle([]byte(styleNoReferences), nil, false)
	if err != nil {
		t.Fatalf("minimal style must validate: %v", err)
	}
	if !res.Valid {
		t.Fatalf("minimal style must report Valid=true, got %+v", res)
	}
}

func TestValidateMapStyle_MissingSpriteReported(t *testing.T) {
	rs := completeStyleFixture()
	// drop the sprite (index 1 in the fixture)
	rs = dropByID(rs, "res-sprite-wayanad")
	res, err := ValidateMapStyle([]byte(styleHappyPath), rs, false)
	if err != nil {
		t.Fatalf("missing sprite must not be a parse error: %v", err)
	}
	if res.Valid {
		t.Fatalf("missing sprite must flip Valid to false, got %+v", res)
	}
	if len(res.MissingSprites) != 1 || res.MissingSprites[0].Missing != "res-sprite-wayanad" {
		t.Fatalf("expected sprite reference in MissingSprites, got %+v", res.MissingSprites)
	}
}

func TestValidateMapStyle_MissingGlyphsReported(t *testing.T) {
	rs := completeStyleFixture()
	// drop the glyphs resource
	rs = dropByID(rs, "res-glyphs-wayanad")
	res, err := ValidateMapStyle([]byte(styleHappyPath), rs, false)
	if err != nil {
		t.Fatalf("missing glyphs must not be a parse error: %v", err)
	}
	if res.Valid {
		t.Fatalf("missing glyphs must flip Valid to false")
	}
	if len(res.MissingGlyphs) != 1 {
		t.Fatalf("expected one missing glyph dep, got %+v", res.MissingGlyphs)
	}
}

func TestValidateMapStyle_MissingSourcesReported(t *testing.T) {
	rs := completeStyleFixture()
	// drop the vector tiles
	rs = dropByID(rs, "res-vector-wayanad")
	res, err := ValidateMapStyle([]byte(styleHappyPath), rs, false)
	if err != nil {
		t.Fatalf("missing source must not be a parse error: %v", err)
	}
	if res.Valid {
		t.Fatalf("missing source must flip Valid to false")
	}
	if len(res.MissingSources) != 1 {
		t.Fatalf("expected one missing source dep, got %+v", res.MissingSources)
	}
}

func TestValidateMapStyle_OptionalMapDoesNotInvalidate(t *testing.T) {
	// The critical incident card stays valid even when the map background
	// is unavailable. The optional=true path must report the missing deps
	// but keep Valid=true so the integration agent can ship the card
	// without the map.
	rs := completeStyleFixture()
	// Drop the sprite and re-run; the result should still be valid.
	rs2 := dropByID(rs, "res-sprite-wayanad")
	res2, err := ValidateMapStyle([]byte(styleHappyPath), rs2, true)
	if err != nil {
		t.Fatalf("optional style parse must succeed: %v", err)
	}
	if !res2.Valid {
		t.Fatalf("optional flag must keep Valid=true when sprite missing, got %+v", res2)
	}
	if len(res2.MissingSprites) != 1 || !res2.MissingSprites[0].Optional {
		t.Fatalf("missing sprite must be reported as optional, got %+v", res2.MissingSprites)
	}
}

func TestValidateMapStyle_AcceptsPBFSourceWithTilesArray(t *testing.T) {
	style := `{
		"version": 8,
		"sources": {
			"composite": {
				"type": "vector",
				"tiles": ["res-vector-wayanad/{z}/{x}/{y}.pbf"],
				"tileSize": 256
			}
		}
	}`
	res, err := ValidateMapStyle([]byte(style), completeStyleFixture(), false)
	if err != nil {
		t.Fatalf("tile-array style must validate: %v", err)
	}
	if !res.Valid {
		t.Fatalf("tile-array style must report Valid=true, got %+v", res)
	}
}

func TestValidateMapStyle_AcceptsFullRoundTrip(t *testing.T) {
	// Marshal/unmarshal round-trip on the same content must yield the
	// same validation outcome.
	var raw map[string]any
	if err := json.Unmarshal([]byte(styleHappyPath), &raw); err != nil {
		t.Fatalf("must parse JSON: %v", err)
	}
	round, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("must marshal JSON: %v", err)
	}
	res, err := ValidateMapStyle(round, completeStyleFixture(), false)
	if err != nil {
		t.Fatalf("round-trip style must validate: %v", err)
	}
	if !res.Valid {
		t.Fatalf("round-trip style must report Valid=true")
	}
	if !strings.Contains(string(round), "sprite") {
		t.Fatalf("round-trip must preserve sprite field")
	}
}

// dropByID returns a new slice with the resource whose ResourceID matches
// id removed. Used by tests that need to delete one descriptor without
// depending on slice index order.
func dropByID(rs []ResourceDescriptor, id string) []ResourceDescriptor {
	out := make([]ResourceDescriptor, 0, len(rs))
	for _, d := range rs {
		if d.ResourceID == id {
			continue
		}
		out = append(out, d)
	}
	return out
}
