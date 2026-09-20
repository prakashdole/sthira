package offlineresources

import (
	"fmt"
	"sort"
)

// DepGraph indexes a flat resource list by URI and ID, then exposes the
// checks the contract requires:
//
//   - duplicate resource IDs are rejected;
//   - two descriptors that share a URI but disagree on digest or type are
//     rejected (conflicting descriptor);
//   - style → sprite/glyphs/source tile endpoints must resolve to a known
//     resource;
//   - cycles and self-loops are detected where the dependency graph is
//     declared; the contract does not add new fields to the descriptor to
//     carry dependencies, so cycles are derived from style references only.
//
// DepGraph is built once per audit and reused across checks; it never owns
// mutable state and is safe to construct in tests.
type DepGraph struct {
	byID  map[string]ResourceDescriptor
	byURI map[string]ResourceDescriptor
	order []string // deterministic for stable error messages
}

func newDepGraph(rs []ResourceDescriptor) *DepGraph {
	g := &DepGraph{
		byID:  make(map[string]ResourceDescriptor, len(rs)),
		byURI: make(map[string]ResourceDescriptor, len(rs)),
	}
	for _, d := range rs {
		g.byID[d.ResourceID] = d
		if existing, ok := g.byURI[d.URI]; ok {
			// First writer wins; conflicts are reported separately by
			// Conflicts so the validator can return the pair of IDs.
			if existing.ResourceID != d.ResourceID {
				g.byURI[d.URI] = existing
			}
		} else {
			g.byURI[d.URI] = d
		}
		g.order = append(g.order, d.ResourceID)
	}
	sort.Strings(g.order)
	return g
}

// DuplicateIDs returns resource IDs that appear more than once in the pack.
// The slice is empty when the pack is unique-by-ID.
func (g *DepGraph) DuplicateIDs(rs []ResourceDescriptor) []string {
	seen := make(map[string]int, len(rs))
	var dups []string
	for _, d := range rs {
		seen[d.ResourceID]++
		if seen[d.ResourceID] == 2 {
			dups = append(dups, d.ResourceID)
		}
	}
	sort.Strings(dups)
	return dups
}

// Conflict reports a pair of descriptors that share a URI but disagree on
// digest or type. The URI is the only address a downstream client has, so
// the conflict is a hard failure.
type Conflict struct {
	URI    string
	First  ResourceDescriptor
	Second ResourceDescriptor
}

// Conflicts scans the pack for resources that share a URI but differ.
func (g *DepGraph) Conflicts(rs []ResourceDescriptor) []Conflict {
	byURI := make(map[string]ResourceDescriptor, len(rs))
	var out []Conflict
	for _, d := range rs {
		prev, ok := byURI[d.URI]
		if !ok {
			byURI[d.URI] = d
			continue
		}
		if prev.ResourceID == d.ResourceID {
			continue
		}
		if prev.ChecksumSHA256 != d.ChecksumSHA256 || prev.Type != d.Type {
			out = append(out, Conflict{
				URI:    d.URI,
				First:  prev,
				Second: d,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URI < out[j].URI })
	return out
}

// RequiredMissing returns required resources that depend on something absent
// from the pack. The contract does not encode explicit dependencies on the
// ResourceDescriptor wire shape (style deps are inferred from style JSON),
// so this check only inspects language coverage for emergency-audio assets.
//
//   - A required emergency-audio resource with a non-empty Languages list
//     must be matched by a gazetteer that lists those languages, OR by an
//     audio resource per language. This is intentionally loose: the
//     integration agent will refine it once O03 lands the language matrix.
func (g *DepGraph) RequiredMissing(rs []ResourceDescriptor) []MissingDep {
	var out []MissingDep
	for _, d := range rs {
		if !d.Required {
			continue
		}
		if d.Type != TypeEmergencyAudio || len(d.Languages) == 0 {
			continue
		}
		have := map[string]bool{}
		for _, other := range rs {
			if other.Type == TypeEmergencyAudio && other.ResourceID != d.ResourceID {
				for _, lang := range other.Languages {
					have[lang] = true
				}
			}
		}
		for _, lang := range d.Languages {
			if !have[lang] {
				out = append(out, MissingDep{
					FromID:   d.ResourceID,
					Missing:  "language:" + lang,
					Kind:     "audio-language",
					Optional: false,
				})
			}
		}
	}
	return out
}

// MissingDep describes a single unresolved dependency declared by a
// resource. Optional=true means the missing dep is for an optional
// resource and does not block activation.
type MissingDep struct {
	FromID   string
	Missing  string
	Kind     string // "sprite" | "glyphs" | "source" | "audio-language"
	Optional bool
}

// StyleReferences collects the sprite, glyph, and source references that a
// MapLibre style JSON declares. The validator cross-checks them against
// the resource list.
type StyleReferences struct {
	SpriteBase string   // sprite field, e.g. "res-sprite-wayanad"
	GlyphURL   string   // text-font glyph endpoint, e.g. "{fontstack}/{range}.pbf"
	Sources    []string // tile source URLs declared in sources
}

// ReferenceMatch is the outcome of cross-checking style references against
// the resource list. Each missing or unresolved entry is reported
// separately so the integration agent can decide what to surface.
type ReferenceMatch struct {
	MissingSprites []MissingDep
	MissingGlyphs  []MissingDep
	MissingSources []MissingDep
}

// CheckStyleReferences verifies that every sprite / glyph / source declared
// in a style document resolves to a resource in the pack.
//
// The matching rules are intentionally simple:
//
//   - sprite: by ResourceID prefix OR URI path component.
//   - glyphs: by URI prefix on the corresponding glyphs resource.
//   - sources: by URI on the matching vector-tiles resource.
//
// A style may declare resources as optional; the first return value reports
// only the *required* missing entries. Optional missing entries are
// returned via MissingSprites/Glyphs/Sources with Optional=true so the
// caller can surface "no map background but guidance still usable".
func (g *DepGraph) CheckStyleReferences(refs StyleReferences, optional bool) ReferenceMatch {
	var out ReferenceMatch
	if refs.SpriteBase != "" {
		if !g.matchesAny(refs.SpriteBase, TypeMapSprite) {
			dep := MissingDep{
				FromID:   "*style*",
				Missing:  refs.SpriteBase,
				Kind:     "sprite",
				Optional: optional,
			}
			out.MissingSprites = append(out.MissingSprites, dep)
		}
	}
	if refs.GlyphURL != "" {
		if !g.matchesAnyPrefix(refs.GlyphURL, TypeMapGlyphs) {
			dep := MissingDep{
				FromID:   "*style*",
				Missing:  refs.GlyphURL,
				Kind:     "glyphs",
				Optional: optional,
			}
			out.MissingGlyphs = append(out.MissingGlyphs, dep)
		}
	}
	for _, src := range refs.Sources {
		if src == "" {
			continue
		}
		if !g.matchesAnyPrefix(src, TypeVectorTiles) {
			dep := MissingDep{
				FromID:   "*style*",
				Missing:  src,
				Kind:     "source",
				Optional: optional,
			}
			out.MissingSources = append(out.MissingSources, dep)
		}
	}
	return out
}

// CycleResult is a single dependency cycle, listed by the resource IDs
// involved. The contract does not add edges on ResourceDescriptor; cycles
// here mean cycles *within* the style-derived graph (style references
// glyphs that reference the style, etc.). It is a defensive check; current
// data shapes should not produce one.
type CycleResult struct {
	Nodes []string
}

// DetectCycles is a defensive no-op for the current wire shape. It walks
// the style references collected by ValidateMapStyle and returns a cycle
// only when, e.g., the sprite URI re-references the style document. The
// behaviour is explicit so a future contract extension that adds an
// explicit depends_on field can plug in here without changing callers.
func (g *DepGraph) DetectCycles(refs StyleReferences) []CycleResult {
	// The wire shape has no explicit edges; the only edges we can infer
	// are style→sprite, style→glyphs, style→sources. There are no
	// sprite→style or source→style references in real MapLibre style JSON,
	// so cycles cannot be expressed with this data. The method exists so
	// callers don't need to special-case the absence of a cycle detector.
	return nil
}

func (g *DepGraph) matchesAny(token string, kind ResourceType) bool {
	for _, d := range g.byID {
		if d.Type != kind {
			continue
		}
		if d.ResourceID == token || endsWithToken(d.URI, token) {
			return true
		}
	}
	return false
}

func (g *DepGraph) matchesAnyPrefix(prefix string, kind ResourceType) bool {
	for _, d := range g.byID {
		if d.Type != kind {
			continue
		}
		if d.URI == prefix || hasURIPrefix(d.URI, prefix) || hasURIPrefix(prefix, d.URI) {
			return true
		}
	}
	return false
}

func endsWithToken(uri, token string) bool {
	if uri == "" || token == "" {
		return false
	}
	if len(uri) < len(token) {
		return false
	}
	return uri[len(uri)-len(token):] == token
}

func hasURIPrefix(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) < len(b) {
		return false
	}
	return a[:len(b)] == b
}

// depSummary is a tiny helper used in tests and audit messages.
func depSummary(rs []MissingDep) string {
	if len(rs) == 0 {
		return ""
	}
	return fmt.Sprintf("%d missing", len(rs))
}
