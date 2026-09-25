package contracts

import (
	"context"
)

// P6 scoped context. The P1/P4 flat `KnownIDs map[string]bool` is insufficient
// for P6 semantics: a model proposal must reference a *typed* ID, and the
// orchestrator must revalidate that the typed relationship (route bound to a
// specific safe zone, facility bound to a permitted order) still holds against
// the persisted snapshot — not merely that the ID exists.
//
// Worker 4 (`p6-context`) owns the validator that consumes this type and the
// store extensions that produce it. The Go orchestrator (Worker 9) revalidates
// after each slow stage via SnapshotRevalidate. The middle worker (W6) and the
// ASR worker (W5) receive only a JSON-encoded snapshot.

// PlaceCandidate is a typed name/admin-ID lookup result. PlaceKind matches the
// persisted store place_aliases table taxonomy.
type PlaceCandidate struct {
	PlaceID      string `json:"place_id"`
	PlaceKind    string `json:"place_kind"` // ZONE | FACILITY | ADMIN
	Name         string `json:"name,omitempty"`
	Jurisdiction string `json:"jurisdiction,omitempty"`
}

// ZoneStatus is the lifecycle status for a zone version. CLOSED zones cannot
// host new commitments even if they appear in the snapshot.
type ZoneStatus string

const (
	ZoneStatusOpen      ZoneStatus = "OPEN"
	ZoneStatusPublished ZoneStatus = "PUBLISHED"
	ZoneStatusClosed    ZoneStatus = "CLOSED"
	ZoneStatusFull      ZoneStatus = "FULL"
)

// ZoneRef is a typed reference to a zone version. Safe zones and red zones
// share the shape; ZoneRole distinguishes them.
type ZoneRef struct {
	ZoneID        string     `json:"zone_id"`
	ZoneRole      string     `json:"zone_role"` // SAFE | RED
	Status        ZoneStatus `json:"status"`
	SourceVersion int        `json:"source_version"`
}

// FacilityRef is a typed reference to a facility.
type FacilityRef struct {
	FacilityID    string `json:"facility_id"`
	SafeZoneID    string `json:"safe_zone_id"`
	Name          string `json:"name,omitempty"`
	CapacityKnown bool   `json:"capacity_known"`
	Free          int    `json:"free,omitempty"` // meaningless when CapacityKnown=false
	SourceVersion int    `json:"source_version"`
}

// RouteStatus is the lifecycle status for a route version.
type RouteStatus string

const (
	RouteStatusVerified   RouteStatus = "VERIFIED"
	RouteStatusClosed     RouteStatus = "CLOSED"
	RouteStatusSuperseded RouteStatus = "SUPERSEDED"
	RouteStatusStale      RouteStatus = "STALE"
)

// RouteRef is a typed reference to a route version. It binds the route to its
// destination safe zone (D37). A route with a different `ToSafeZoneID` is not
// usable for a destination with a different `SafeZoneID`.
type RouteRef struct {
	RouteID      string      `json:"route_id"`
	FromZoneID   string      `json:"from_zone_id,omitempty"`
	ToSafeZoneID string      `json:"to_safe_zone_id"`
	Status       RouteStatus `json:"status"`
	// Verified reports whether the route is currently-valid and unclosed. It is
	// derived, not stored: Verified && Status==VERIFIED && ValidNow.
	Verified bool `json:"verified"`
	// SourceVersion ties the route to a specific package revision; a withdrawal
	// or supersession increments the package revision and demotes this route to
	// SUPERSEDED. Workers must re-read before re-using the ID.
	SourceVersion int `json:"source_version"`
}

// EligibleChoice is one entry in the server-permitted order for SHOW_CHOICES.
// Worker 4 enforces the order; Worker 9 never re-sorts.
type EligibleChoice struct {
	Facility FacilityRef `json:"facility"`
	// PermittedRank is the 0-based server-permitted rank; Worker 4's
	// validator rejects a SHOW_CHOICES target_ids list that does not match
	// the order and length of the eligible slice for the same request.
	PermittedRank int `json:"permitted_rank"`
}

// ScopedContext is the richer trusted context the P6 orchestrator and the
// independent semantic validator consume. Every field is derived from ONE
// consistent persisted snapshot; mixing fields across revisions is rejected.
//
// AllowedLanguages and TemplateKeys are ordered slices; the validator accepts
// the union of these slices and rejects unknown values.
type ScopedContext struct {
	RequestID       string         `json:"request_id"`
	DataVersion     string         `json:"data_version"`
	Jurisdiction    string         `json:"jurisdiction"`
	SchemaVersion   string         `json:"schema_version"` // always "3.0"
	SourceID        string         `json:"source_id,omitempty"`
	SourceStatus    FreshnessState `json:"source_status"`
	SourceVersion   int            `json:"source_version"`
	TemplateVersion int            `json:"template_version"`

	// AllowedLanguages lists the enabled context languages. Order is not
	// semantic; the model may pick any one. Unknown languages are rejected.
	AllowedLanguages []string `json:"allowed_languages"`
	// TemplateKeys lists the approved speech_key values. Worker 7 freezes
	// the rendered text for each key.
	TemplateKeys []string `json:"template_keys"`
	// ApprovedSpeechKeys maps speech_key to the exact list of approved
	// languages. Wildcard approval is not representable: B01 fails closed
	// when language or digests are missing.
	ApprovedSpeechKeys map[string][]string `json:"approved_speech_keys,omitempty"`
	// ApprovedTemplateSHA maps speech_key/language (e.g. "welcome/en-IN") to
	// the approved template_sha256 digest (hex). The orchestrator rejects a
	// registered template whose Text digest does not match this value.
	ApprovedTemplateSHA map[string]string `json:"approved_template_sha256,omitempty"`

	// KnownPlaces maps a place_id to its typed candidate. Distinct from the
	// flat KnownIDs map; it carries kind and jurisdiction for validation.
	KnownPlaces map[string]PlaceCandidate `json:"known_places"`
	// KnownRedZones maps a red-zone ID to its typed reference.
	KnownRedZones map[string]ZoneRef `json:"known_red_zones"`
	// KnownSafeZones maps a safe-zone ID to its typed reference.
	KnownSafeZones map[string]ZoneRef `json:"known_safe_zones"`
	// KnownRoutes maps a route ID to its typed reference.
	KnownRoutes map[string]RouteRef `json:"known_routes"`
	// KnownFacilities maps a facility ID to its typed reference.
	KnownFacilities map[string]FacilityRef `json:"known_facilities"`

	// VerifiedRoutes maps a facility ID to its verified routes. The validator
	// requires that any route referenced by an action targeting that facility
	// appears in this slice.
	VerifiedRoutes map[string][]RouteRef `json:"verified_routes"`

	// EligibleDestinations is the server-permitted order for SHOW_CHOICES.
	// Worker 4 enforces the order; the model never re-sorts.
	EligibleDestinations []EligibleChoice `json:"eligible_destinations"`

	// IssuedAt is the server-side timestamp the snapshot was produced; the
	// orchestrator can use it for freshness reporting only.
	IssuedAt string `json:"issued_at"`
}

// SnapshotRevalidator is the seam Worker 4 implements so Worker 9 can
// revalidate the same context after a slow stage. Re-read the persisted
// snapshot for (jurisdiction, source_id). Return ErrStaleSnapshot if the
// (SourceVersion, TemplateVersion, DataVersion) tuple changed since the
// current ScopedContext was issued. Cancellation is honored via ctx.
type SnapshotRevalidator interface {
	SnapshotRevalidate(ctx context.Context, sc ScopedContext) error
}

// ErrStaleSnapshot (defined in errors.go) is returned when the persisted
// snapshot has changed since the ScopedContext was issued. Mapped to HTTP 409.

// AllowedIntent checks whether intent is one of the strict middleware intents.
// Reused from the P1 model contract.
func (sc ScopedContext) AllowedIntent(intent Intent) bool {
	_, ok := validIntents[intent]
	return ok
}

// IsLanguageAllowed reports whether the language is in the allowed set.
func (sc ScopedContext) IsLanguageAllowed(language string) bool {
	for _, l := range sc.AllowedLanguages {
		if l == language {
			return true
		}
	}
	return false
}

// IsTemplateKeyAllowed reports whether the speech_key is in the allowed set.
func (sc ScopedContext) IsTemplateKeyAllowed(key string) bool {
	for _, k := range sc.TemplateKeys {
		if k == key {
			return true
		}
	}
	return false
}

// IsSpeechKeyApprovedForLanguage reports whether the speech_key is approved
// for the given language. B01: fail closed — empty ApprovedSpeechKeys does
// not fall back to TemplateKeys, and "*" is not an accepted language.
func (sc ScopedContext) IsSpeechKeyApprovedForLanguage(key, language string) bool {
	if key == "" || language == "" {
		return false
	}
	langs, ok := sc.ApprovedSpeechKeys[key]
	if !ok || len(langs) == 0 {
		return false
	}
	for _, l := range langs {
		if l != "" && l != "*" && l == language {
			return true
		}
	}
	return false
}

// TemplateDigestKey formats the composite map key binding speech_key and language.
func TemplateDigestKey(speechKey, language string) string {
	return speechKey + "/" + language
}

// TemplateDigest returns the approved template digest for the given speech_key
// and language. It requires the exact language-bound tuple key "speech_key/language".
// Missing tuple or wrong language rejects (fails closed).
// B01/C03: If not approved or missing, returns ("", false).
func (sc ScopedContext) TemplateDigest(key, language string) (string, bool) {
	if key == "" || language == "" || sc.ApprovedTemplateSHA == nil {
		return "", false
	}
	if !sc.IsSpeechKeyApprovedForLanguage(key, language) {
		return "", false
	}
	tupleKey := TemplateDigestKey(key, language)
	if sha, ok := sc.ApprovedTemplateSHA[tupleKey]; ok && sha != "" {
		return sha, true
	}
	return "", false
}

// IsKnownID reports whether id appears in known places, safe zones, red zones, routes or facilities.
func (sc ScopedContext) IsKnownID(id string) bool {
	if _, ok := sc.KnownPlaces[id]; ok {
		return true
	}
	if _, ok := sc.KnownSafeZones[id]; ok {
		return true
	}
	if _, ok := sc.KnownRedZones[id]; ok {
		return true
	}
	if _, ok := sc.KnownRoutes[id]; ok {
		return true
	}
	if _, ok := sc.KnownFacilities[id]; ok {
		return true
	}
	return false
}
