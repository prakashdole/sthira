// Package offlineresources validates offline-resource descriptors that travel
// inside a signed manifest and the map style JSON that depends on them.
//
// It is the Agent 4 slice of the P5 contract (see plan/p5-contract.md §2.1,
// §7.4 and §8.1). It depends on offlinepkg (Agent 1) for the wire types and
// re-exports the canonical ResourceType / resource-type constants so the
// rest of this package can use a single vocabulary. The ResourceDescriptor
// type embeds offlinepkg.ResourceDescriptor and adds validation-side metadata
// (license evidence, zoom bounds, bbox, languages) that the contract leaves
// O06-pending; the JSON wire shape remains byte-identical because embedded
// fields are promoted.
//
// What this package does:
//   - Structural validation of a single ResourceDescriptor (id, type, digest,
//     byte size, media type, attribution, license/redistribution evidence).
//   - Dependency-graph validation across the resource list (duplicates,
//     conflicting metadata for one URI, missing style deps, declared cycles,
//     critical-vs-optional status).
//   - Validation of a MapLibre-style JSON document against the resource list
//     (sprite, glyphs, source tile endpoints resolve to known resources).
//   - Cumulative budget audit for the regional pack (≤ 50 MiB by default).
//
// What this package does NOT do:
//   - It does not fetch, sign, serve, store or activate any resource.
//   - It does not claim a license string is legal proof; a present
//     redistribution flag only means the descriptor declares the
//     permission — O06 evidence is still required for live activation.
//   - It does not validate the public incident card or the manifest
//     signature; those belong to Agent 1.
package offlineresources

import (
	"errors"
	"fmt"
	"regexp"

	"sthira/backend/internal/offlinepkg"
)

// ResourceType is the canonical P5 wire type from offlinepkg (§7.1).
// Aliased so existing callers can use the local name without churn.
type ResourceType = offlinepkg.ResourceType

const (
	TypeVectorTiles    = offlinepkg.TypeVectorTiles
	TypeMapStyle       = offlinepkg.TypeMapStyle
	TypeMapSprite      = offlinepkg.TypeMapSprite
	TypeMapGlyphs      = offlinepkg.TypeMapGlyphs
	TypeGazetteer      = offlinepkg.TypeGazetteer
	TypeEmergencyAudio = offlinepkg.TypeEmergencyAudio
)

// RedistributionPermission mirrors the offline redistribution declaration
// that O06 still gates. The values are deliberately a closed set so the
// validator can recognise an undeclared permission explicitly.
type RedistributionPermission string

const (
	RedistUndeclared  RedistributionPermission = ""
	RedistAllowed     RedistributionPermission = "REDISTRIBUTION_ALLOWED"
	RedistConditional RedistributionPermission = "REDISTRIBUTION_CONDITIONAL"
	RedistDenied      RedistributionPermission = "REDISTRIBUTION_DENIED"
)

// LicenseInfo is the optional license/redistribution metadata that a regional
// resource descriptor may carry. The fields are pointers so absence is
// distinct from "explicit empty value". They are not part of the v3.0
// ResourceDescriptor wire shape; descriptors without them remain valid under
// the current contract, but the validator records them as
// "verification pending" against O06. When O06 evidence arrives, this is the
// structure the integration agent promotes into offlinepkg.
//
// A present license string is metadata, not legal approval.
type LicenseInfo struct {
	SPDXIdentifier      string                   `json:"spdx_identifier,omitempty"`
	LicenseName         string                   `json:"license_name,omitempty"`
	AttributionRequired string                   `json:"attribution_required,omitempty"`
	Redistribution      RedistributionPermission `json:"redistribution,omitempty"`
	DeclaredOfflineOK   bool                     `json:"declared_offline_ok,omitempty"`
	EvidenceReference   string                   `json:"evidence_reference,omitempty"`
}

// ResourceDescriptor mirrors offlinepkg.ResourceDescriptor (§7.1) plus the
// optional LicenseInfo above. JSON tags match the contract for the base
// fields; the new fields are added with omitempty so the wire shape is
// backward-compatible.
type ResourceDescriptor struct {
	offlinepkg.ResourceDescriptor

	// License is not in the v3.0 wire shape; it is validation-side metadata
	// pending O06 evidence. The validator inspects it when present.
	License *LicenseInfo `json:"license,omitempty"`

	// Bounds apply to the optional regional map pack; safe-zone resources
	// that are part of a tileset declare their zoom and bounding box here.
	// All fields are optional; the validator treats absence as unknown and
	// never invents defaults.
	MinZoom   *int     `json:"min_zoom,omitempty"`
	MaxZoom   *int     `json:"max_zoom,omitempty"`
	BBox      *GeoBBox `json:"bbox,omitempty"`
	Languages []string `json:"languages,omitempty"`
}

// Wire returns the embedded offlinepkg.ResourceDescriptor with the same field
// values. Useful for callers that need to pass the canonical wire shape (e.g.
// when handing off to offlinepkg verification or to offlinedelivery).
func (d ResourceDescriptor) Wire() offlinepkg.ResourceDescriptor {
	return d.ResourceDescriptor
}

// GeoBBox is a four-float64 GeoJSON-style bounding box [minLon,minLat,maxLon,maxLat].
type GeoBBox struct {
	MinLon float64 `json:"min_lon"`
	MinLat float64 `json:"min_lat"`
	MaxLon float64 `json:"max_lon"`
	MaxLat float64 `json:"max_lat"`
}

// ValidatorError is the typed failure returned by the validator. The Reason
// is stable enough for the integration agent to surface; Field is the JSON
// path inside the resource list.
type ValidatorError struct {
	Reason string
	Field  string
}

func (e *ValidatorError) Error() string {
	if e.Field == "" {
		return "offlineresources: " + e.Reason
	}
	return "offlineresources: " + e.Field + ": " + e.Reason
}

// Reasons used by validator errors. Listed once so tests and callers can
// match on them without depending on free-form message text.
const (
	ReasonEmptyResourceID       = "resource_id is empty"
	ReasonEmptyURI              = "uri is empty"
	ReasonUnknownResourceType   = "unknown resource_type"
	ReasonInvalidDigest         = "checksum_sha256 must be 64 lowercase hex chars"
	ReasonNonPositiveSize       = "byte_size must be > 0"
	ReasonEmptyContentType      = "content_type is empty"
	ReasonInvalidContentType    = "content_type is not a valid media type"
	ReasonEmptyAttribution      = "attribution is empty"
	ReasonUndeclaredRedist      = "redistribution permission is undeclared (O06 evidence missing)"
	ReasonDeniedRedist          = "redistribution is explicitly denied — offline pack not permitted"
	ReasonUnknownRedist         = "redistribution permission is not in the known set"
	ReasonEmptyLicense          = "license reference is empty while redistribution is declared"
	ReasonDuplicateResourceID   = "duplicate resource_id in pack"
	ReasonConflictingURI        = "two resources share a uri but differ in digest or type"
	ReasonUnsupportedFormat     = "unsupported tile or media format for resource_type"
	ReasonStyleMissingSprite    = "map style references a sprite that is not in the pack"
	ReasonStyleMissingGlyphs    = "map style references a glyph endpoint that is not in the pack"
	ReasonStyleMissingSource    = "map style references a tile source that is not in the pack"
	ReasonCycleDetected         = "resource declares a dependency cycle"
	ReasonMissingCritical       = "required resource is missing for declared dependency"
	ReasonInvalidZoom           = "min_zoom must be <= max_zoom and within [0,22]"
	ReasonInvalidBBox           = "bbox is malformed (min must be <= max on both axes)"
	ReasonInvalidLanguageTag    = "language tag does not match BCP-47 basic shape"
	ReasonCriticalExceedsBudget = "required resource alone exceeds the per-asset budget for its type"
)

// Errors returned by NewValidator consumers. Tests use errors.Is against
// these sentinels where the integration agent will surface specific reasons.
var (
	ErrPackInvalid       = errors.New("offlineresources: regional pack is invalid")
	ErrStyleInvalid      = errors.New("offlineresources: map style is invalid")
	ErrLicenseUnverified = errors.New("offlineresources: license/redistribution evidence pending O06")
)

// digestRegex matches exactly 64 lowercase hex chars.
var digestRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// languageTagRegex is a deliberately loose BCP-47 subset: 2 or 3 letter
// primary subtag, optional 2-letter region or 3-digit script subtag,
// separated by '-'. It accepts "ml", "en-IN", "zh-Hans" and rejects empty
// or malformed tags. It does NOT enforce the full IANA subtag registry.
var languageTagRegex = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// bytesForType returns the per-asset ceiling for the given resource type, in
// bytes. Critical card and emergency audio have per-asset limits; other types
// roll up into the regional pack budget. Zero means "no per-asset ceiling".
func bytesForType(t ResourceType) int64 {
	switch t {
	case TypeEmergencyAudio:
		return 512 * 1024 // 512 KiB compressed
	default:
		return 0
	}
}

// formatType enumerates the supported media types per resource kind.
// Anything outside this list is rejected with ReasonUnsupportedFormat.
type formatClass int

const (
	fmtJSON formatClass = iota
	fmtPBF
	fmtPNG
	fmtJSONFontGlyph
	fmtSpritesJSON
	fmtSpritesPNG
	fmtAudio
	fmtGazetteer
)

// classify maps a media type to its expected format class. Returns ok=false
// when the media type is missing or unknown for the resource kind.
func classify(t ResourceType, contentType string) (formatClass, bool) {
	switch t {
	case TypeMapStyle:
		if contentType == "application/json" || contentType == "application/geo+json" {
			return fmtJSON, true
		}
	case TypeVectorTiles:
		if contentType == "application/vnd.mapbox-vector-tile" || contentType == "application/x-protobuf" {
			return fmtPBF, true
		}
	case TypeMapGlyphs:
		// PBF glyphs are the standard; a JSON descriptor wrapping them is also allowed.
		if contentType == "application/x-protobuf" || contentType == "application/octet-stream" {
			return fmtJSONFontGlyph, true
		}
	case TypeMapSprite:
		if contentType == "application/json" {
			return fmtSpritesJSON, true
		}
		if contentType == "image/png" {
			return fmtSpritesPNG, true
		}
	case TypeGazetteer:
		if contentType == "application/json" || contentType == "application/geo+json" {
			return fmtGazetteer, true
		}
	case TypeEmergencyAudio:
		if contentType == "audio/mpeg" ||
			contentType == "audio/mp4" ||
			contentType == "audio/ogg" ||
			contentType == "audio/wav" ||
			contentType == "application/ogg" {
			return fmtAudio, true
		}
	}
	return 0, false
}

// labelFor is a tiny formatting helper used by tests and reason messages.
func labelFor(id string) string { return fmt.Sprintf("resource %q", id) }
