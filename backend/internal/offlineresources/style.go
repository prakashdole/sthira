package offlineresources

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MapStyle is the minimal subset of a MapLibre style document the validator
// inspects. The full style JSON is many kilobytes; we unmarshal only the
// fields that carry dependency references so the validator stays small and
// robust against schema drift in unrelated fields.
type MapStyle struct {
	Version  int8              `json:"version"`
	Sources  map[string]Source `json:"sources"`
	Sprite   string            `json:"sprite"`
	GlyphURL string            `json:"glyphs"`
}

// Source is the subset of a MapLibre source we care about for dep matching.
// Vector sources declare either a single URL or a list of tiles. Both shapes
// are accepted.
type Source struct {
	Type    string   `json:"type"`
	URL     string   `json:"url"`
	Tiles   []string `json:"tiles"`
	TileURL string   `json:"tileUrl,omitempty"`
}

// StyleValidationResult mirrors offlinepkg.StyleValidationResult from the
// P5 contract §7.4. Valid is computed by the validator; the three
// Missing* slices contain the unresolved references sorted by ResourceID.
type StyleValidationResult struct {
	Valid          bool         `json:"valid"`
	MissingSprites []MissingDep `json:"missing_sprites,omitempty"`
	MissingGlyphs  []MissingDep `json:"missing_glyphs,omitempty"`
	MissingSources []MissingDep `json:"missing_sources,omitempty"`
}

// ValidateMapStyle parses the supplied style JSON and cross-checks its
// sprite, glyph and source references against the available resources.
//
// Behaviour:
//
//   - The JSON must be a syntactically valid MapLibre-style document with a
//     numeric "version" and a "sources" object. The decoder rejects
//     trailing data and unknown top-level fields so a future schema drift
//     surfaces here rather than silently passing.
//   - A style with no sprite, no glyph and no sources is valid: the
//     critical path uses the incident card only and the map is optional.
//   - Each unresolved reference is reported individually; the result is
//     Valid=true only when no required reference is missing.
//   - The style itself is treated as optional: a missing required
//     reference does NOT make the critical card invalid, only the map.
//
// The optional flag controls whether missing required references flip
// Valid to false. When optional=true, missing entries are still reported
// but Valid remains true so the incident card stays usable without the
// map background.
func ValidateMapStyle(styleJSON []byte, available []ResourceDescriptor, optional bool) (*StyleValidationResult, error) {
	if len(styleJSON) == 0 {
		return nil, &ValidatorError{Reason: "style JSON is empty", Field: ""}
	}
	var s MapStyle
	dec := json.NewDecoder(bytes.NewReader(styleJSON))
	// The validator inspects only the dependency-relevant subset of a
	// MapLibre style document; the full style schema is large and
	// version-sensitive. Unknown top-level fields are tolerated so a
	// new MapLibre release does not break the validator.
	if err := dec.Decode(&s); err != nil {
		return nil, &ValidatorError{
			Reason: fmt.Sprintf("style JSON parse failed: %v", err),
			Field:  "",
		}
	}
	if dec.More() {
		return nil, &ValidatorError{
			Reason: "style JSON has trailing data",
			Field:  "",
		}
	}
	if s.Sources == nil && s.Sprite == "" && s.GlyphURL == "" {
		return &StyleValidationResult{Valid: true}, nil
	}
	refs := StyleReferences{
		SpriteBase: s.Sprite,
		GlyphURL:   s.GlyphURL,
	}
	for name, src := range s.Sources {
		if src.URL != "" {
			refs.Sources = append(refs.Sources, src.URL)
		}
		for _, tile := range src.Tiles {
			if tile != "" {
				refs.Sources = append(refs.Sources, tile)
			}
		}
		if src.TileURL != "" {
			refs.Sources = append(refs.Sources, src.TileURL)
		}
		_ = name
	}
	g := newDepGraph(available)
	match := g.CheckStyleReferences(refs, optional)
	res := &StyleValidationResult{
		MissingSprites: match.MissingSprites,
		MissingGlyphs:  match.MissingGlyphs,
		MissingSources: match.MissingSources,
	}
	res.Valid = !hasRequiredMissing(match)
	if optional {
		for i := range res.MissingSprites {
			res.MissingSprites[i].Optional = true
		}
		for i := range res.MissingGlyphs {
			res.MissingGlyphs[i].Optional = true
		}
		for i := range res.MissingSources {
			res.MissingSources[i].Optional = true
		}
		res.Valid = true
	}
	return res, nil
}

// hasRequiredMissing returns true when any of the three slices contains at
// least one non-optional entry.
func hasRequiredMissing(m ReferenceMatch) bool {
	return anyRequired(m.MissingSprites) ||
		anyRequired(m.MissingGlyphs) ||
		anyRequired(m.MissingSources)
}

func anyRequired(ds []MissingDep) bool {
	for _, d := range ds {
		if !d.Optional {
			return true
		}
	}
	return false
}
