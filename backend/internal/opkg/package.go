// Package opkg is the validation boundary for authority-supplied operational
// packages. It checks relationships and safety-critical completeness only; it
// never infers missing routes, capacity, status, or allocation policy.
//
// Trust posture: a checksum proves integrity, not publisher authority. The
// canonical bytes exclude checksum_sha256 and signature, so a recomputed
// checksum matching the declared value only shows the artifact is intact.
// Signature validity is checked separately and is distinct from signer
// authorization — a valid signature from an unauthorized key is rejected by
// the caller's authorization policy, not by this validator.
package opkg

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// PackageError is a validation failure; the package is quarantined.
type PackageError struct{ Reason string }

func (e *PackageError) Error() string { return e.Reason }

func fail(format string, args ...any) *PackageError {
	return &PackageError{Reason: fmt.Sprintf(format, args...)}
}

// EvidenceClass distinguishes synthetic/exercise data from authorized data.
type EvidenceClass string

const (
	EvidenceSynthetic   EvidenceClass = "SYNTHETIC_DEMO"
	EvidenceCaptured    EvidenceClass = "CAPTURED_OFFICIAL_SAMPLE"
	EvidenceShadow      EvidenceClass = "AUTHORIZED_SHADOW"
	EvidenceOperational EvidenceClass = "AUTHORIZED_OPERATIONAL"
)

// RouteApproval marks whether a route is authorized for operational use.
type RouteApproval string

const (
	ApprovalSynthetic   RouteApproval = "SYNTHETIC_DEMO"
	ApprovalOperational RouteApproval = "AUTHORIZED_OPERATIONAL"
)

// ZoneStatus is a safe-zone operating state.
type ZoneStatus string

const (
	ZoneOpen      ZoneStatus = "OPEN"
	ZoneClosed    ZoneStatus = "CLOSED"
	ZoneFull      ZoneStatus = "FULL"
	ZonePublished ZoneStatus = "PUBLISHED"
)

// Signature carries detached signature metadata. Value validity is verified by
// an injected Verifier; authorization of the key is a separate concern.
type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Value     string `json:"value"`
}

// Provenance is the package's source identity and integrity metadata.
type Provenance struct {
	DatasetID      string        `json:"dataset_id"`
	EvidenceClass  EvidenceClass `json:"evidence_class"`
	Version        int           `json:"version"`
	Jurisdiction   string        `json:"jurisdiction"`
	Authority      string        `json:"authority"`
	EffectiveAt    string        `json:"effective_at"`
	ExpiresAt      string        `json:"expires_at"`
	ChecksumSHA256 string        `json:"checksum_sha256"`
	Signature      *Signature    `json:"signature,omitempty"`
}

// Alert identifies the alert this package serves.
type Alert struct {
	Identifier string `json:"identifier"`
}

// Zone is a red (hazard) or safe (destination) zone.
type Zone struct {
	ID       string     `json:"id"`
	Capacity *int       `json:"capacity,omitempty"`
	Location []float64  `json:"location,omitempty"`
	Status   ZoneStatus `json:"status,omitempty"`
}

// Route is an approved path from a red zone to a safe zone.
type Route struct {
	ID           string          `json:"id"`
	FromZoneID   string          `json:"from_zone_id"`
	ToSafeZoneID string          `json:"to_safe_zone_id"`
	Approval     RouteApproval   `json:"approval"`
	Geometry     json.RawMessage `json:"geometry"`
}

// InstructionAsset is a localized instruction artifact.
type InstructionAsset struct {
	ID       string `json:"id"`
	Language string `json:"language"`
}

// Facility is a shelter/facility that must reference a safe zone.
type Facility struct {
	ID       string `json:"id"`
	SafeZone string `json:"safe_zone_id"`
}

// AllocationPolicy orders safe-zone selection. It must explicitly order every
// safe zone; a missing or partial order is a validation failure, not a default.
type AllocationPolicy struct {
	Order []string `json:"order"`
}

// EmergencyContact is an official dialler target.
type EmergencyContact struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

// Package is the decoded operational package.
type Package struct {
	Provenance     Provenance         `json:"provenance"`
	Alert          Alert              `json:"alert"`
	RedZones       []Zone             `json:"red_zones"`
	SafeZones      []Zone             `json:"safe_zones"`
	ApprovedRoutes []Route            `json:"approved_routes"`
	Instructions   []InstructionAsset `json:"instruction_assets"`
	Facilities     []Facility         `json:"facilities"`
	Policy         AllocationPolicy   `json:"allocation_policy"`
	Contacts       []EmergencyContact `json:"emergency_contacts"`
}

// Validation is the validated result: the package's safety-critical identity.
type Validation struct {
	PackageID     string
	EvidenceClass EvidenceClass
	AlertID       string
	RedZoneIDs    []string
	SafeZoneIDs   []string
	RouteIDs      []string
	Languages     []string
	Version       int
	Jurisdiction  string
	EffectiveAt   time.Time
	ExpiresAt     time.Time
	Checksum      string
}

// Verifier reports whether a signature is cryptographically valid over the
// package's canonical bytes. It does NOT establish signer authorization.
type Verifier func(pkg *Package, sig *Signature) bool

// lineString is the decoded route geometry.
type lineString struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

// Validate checks a decoded package. expectedJurisdiction, when non-empty,
// must match the package jurisdiction. requireSignature demands a present,
// well-formed signature verified by verifier; it does not authorize the signer.
func Validate(pkg *Package, expectedJurisdiction string, requireSignature bool, verifier Verifier) (*Validation, error) {
	if pkg == nil {
		return nil, fail("package is required")
	}
	p := pkg.Provenance
	switch p.EvidenceClass {
	case EvidenceSynthetic, EvidenceCaptured, EvidenceShadow, EvidenceOperational:
	default:
		return nil, fail("invalid evidence class %q", p.EvidenceClass)
	}
	if p.DatasetID == "" {
		return nil, fail("package dataset_id is required")
	}
	if p.Version < 1 {
		return nil, fail("package version must be a positive integer")
	}
	if p.Jurisdiction == "" {
		return nil, fail("package jurisdiction is required")
	}
	if p.Authority == "" {
		return nil, fail("package authority is required")
	}
	effectiveAt, err := parseTime("effective_at", p.EffectiveAt)
	if err != nil {
		return nil, err
	}
	expiresAt, err := parseTime("expires_at", p.ExpiresAt)
	if err != nil {
		return nil, err
	}
	if !expiresAt.After(effectiveAt) {
		return nil, fail("package expiry must be after effective time")
	}
	if len(p.ChecksumSHA256) != 64 {
		return nil, fail("package checksum is required")
	}
	if p.ChecksumSHA256 != Checksum(pkg) {
		return nil, fail("package checksum verification failed")
	}
	if expectedJurisdiction != "" && p.Jurisdiction != expectedJurisdiction {
		return nil, fail("package jurisdiction does not match")
	}
	if requireSignature {
		if p.Signature == nil || p.Signature.Value == "" {
			return nil, fail("package signature is required")
		}
		if p.Signature.Algorithm == "" || p.Signature.KeyID == "" {
			return nil, fail("package signature metadata is required")
		}
		if verifier == nil {
			return nil, fail("package signature verifier is not configured")
		}
		if !verifier(pkg, p.Signature) {
			return nil, fail("package signature verification failed")
		}
	}

	if pkg.Alert.Identifier == "" {
		return nil, fail("package alert identifier is required")
	}
	if len(pkg.RedZones) == 0 {
		return nil, fail("red_zones are required")
	}
	if len(pkg.SafeZones) == 0 {
		return nil, fail("safe_zones are required")
	}
	if len(pkg.ApprovedRoutes) == 0 {
		return nil, fail("approved_routes are required")
	}
	if len(pkg.Instructions) == 0 {
		return nil, fail("instruction_assets are required")
	}
	if len(pkg.Facilities) == 0 {
		return nil, fail("facilities are required")
	}
	if len(pkg.Policy.Order) == 0 {
		return nil, fail("allocation policy is required")
	}
	if len(pkg.Contacts) == 0 {
		return nil, fail("emergency contacts are required")
	}

	redIDs, err := zoneIDs(pkg.RedZones, "red_zones")
	if err != nil {
		return nil, err
	}
	safeIDs, err := zoneIDs(pkg.SafeZones, "safe_zones")
	if err != nil {
		return nil, err
	}
	routeIDs, err := routeIDsOf(pkg.ApprovedRoutes)
	if err != nil {
		return nil, err
	}
	redSet := toSet(redIDs)
	safeSet := toSet(safeIDs)

	// Allocation policy must explicitly order every safe zone (same members,
	// same order), with no omissions and no extras.
	if !equalStrings(pkg.Policy.Order, safeIDs) {
		return nil, fail("allocation policy must explicitly order every safe zone")
	}
	for _, c := range pkg.Contacts {
		if c.Number == "" {
			return nil, fail("emergency contact numbers are required")
		}
	}
	for _, f := range pkg.Facilities {
		if f.ID == "" {
			return nil, fail("facility ids are required")
		}
		if !safeSet[f.SafeZone] {
			return nil, fail("facility %q must reference a safe zone", f.ID)
		}
	}

	for _, z := range pkg.SafeZones {
		if z.Capacity == nil || *z.Capacity < 0 {
			return nil, fail("safe-zone %q capacity must be explicit and non-negative", z.ID)
		}
		if err := validateLocation(z.Location); err != nil {
			return nil, fail("safe-zone %q %v", z.ID, err)
		}
		switch z.Status {
		case "", ZoneOpen, ZoneClosed, ZoneFull, ZonePublished:
		default:
			return nil, fail("safe-zone %q invalid status %q", z.ID, z.Status)
		}
	}
	for _, r := range pkg.ApprovedRoutes {
		if !redSet[r.FromZoneID] {
			return nil, fail("route %q origin must reference a red zone", r.ID)
		}
		if !safeSet[r.ToSafeZoneID] {
			return nil, fail("route %q destination must reference a safe zone", r.ID)
		}
		if r.Approval != ApprovalSynthetic && r.Approval != ApprovalOperational {
			return nil, fail("route %q approval is not authorized", r.ID)
		}
		if err := validateRouteGeometry(r.Geometry); err != nil {
			return nil, fail("route %q %v", r.ID, err)
		}
	}

	languages := make([]string, 0, len(pkg.Instructions))
	for _, a := range pkg.Instructions {
		if a.ID == "" {
			return nil, fail("instruction_asset ids are required")
		}
		if a.Language == "" {
			return nil, fail("instruction language is required")
		}
		languages = append(languages, a.Language)
	}

	return &Validation{
		PackageID:     p.DatasetID,
		EvidenceClass: p.EvidenceClass,
		AlertID:       pkg.Alert.Identifier,
		RedZoneIDs:    redIDs,
		SafeZoneIDs:   safeIDs,
		RouteIDs:      routeIDs,
		Languages:     languages,
		Version:       p.Version,
		Jurisdiction:  p.Jurisdiction,
		EffectiveAt:   effectiveAt,
		ExpiresAt:     expiresAt,
		Checksum:      p.ChecksumSHA256,
	}, nil
}

// Checksum returns the package's integrity digest over canonical bytes that
// exclude checksum_sha256 and signature. It proves integrity only.
func Checksum(pkg *Package) string {
	sum := sha256.Sum256(canonicalBytes(pkg))
	return hex.EncodeToString(sum[:])
}

// canonicalBytes is the signed/checksummed representation: the package with
// mutable integrity metadata removed, keys sorted, no insignificant whitespace.
func canonicalBytes(pkg *Package) []byte {
	// Copy and strip mutable provenance fields before canonical encoding.
	cpy := *pkg
	cpy.Provenance.ChecksumSHA256 = ""
	cpy.Provenance.Signature = nil
	return canonicalJSON(cpy)
}

// canonicalJSON marshals v with deterministic key order and no extra
// whitespace, matching the Python reference's sort_keys + compact separators.
// encoding/json already sorts map keys and emits no padding; struct field
// order is fixed by declaration, so we route through a map to guarantee
// canonical ordering independent of struct layout.
func canonicalJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var generic any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return nil
	}
	var buf bytes.Buffer
	writeCanonical(&buf, generic)
	return buf.Bytes()
}

func writeCanonical(buf *bytes.Buffer, v any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteByte(':')
			writeCanonical(buf, t[k])
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonical(buf, e)
		}
		buf.WriteByte(']')
	default:
		b, _ := json.Marshal(t)
		buf.Write(b)
	}
}

func validateLocation(loc []float64) error {
	if len(loc) != 2 {
		return fail("location is required")
	}
	lon, lat := loc[0], loc[1]
	if lon < -180 || lon > 180 || lat < -90 || lat > 90 {
		return fail("location is outside CRS bounds")
	}
	return nil
}

func validateRouteGeometry(raw json.RawMessage) error {
	var g lineString
	if err := json.Unmarshal(raw, &g); err != nil {
		return fail("route geometry must be a LineString")
	}
	if g.Type != "LineString" {
		return fail("route geometry must be a LineString")
	}
	if len(g.Coordinates) < 2 {
		return fail("route geometry requires at least two positions")
	}
	for _, pt := range g.Coordinates {
		if len(pt) != 2 {
			return fail("route geometry positions are invalid")
		}
		lon, lat := pt[0], pt[1]
		if lon < -180 || lon > 180 || lat < -90 || lat > 90 {
			return fail("route geometry position outside CRS bounds")
		}
	}
	return nil
}

func zoneIDs(zones []Zone, label string) ([]string, error) {
	out := make([]string, 0, len(zones))
	seen := map[string]bool{}
	for _, z := range zones {
		if z.ID == "" {
			return nil, fail("%s ids are required", label)
		}
		if seen[z.ID] {
			return nil, fail("%s ids must be unique", label)
		}
		seen[z.ID] = true
		out = append(out, z.ID)
	}
	return out, nil
}

func routeIDsOf(routes []Route) ([]string, error) {
	out := make([]string, 0, len(routes))
	seen := map[string]bool{}
	for _, r := range routes {
		if r.ID == "" {
			return nil, fail("approved_routes ids are required")
		}
		if seen[r.ID] {
			return nil, fail("approved_routes ids must be unique")
		}
		seen[r.ID] = true
		out = append(out, r.ID)
	}
	return out, nil
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func parseTime(label, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fail("%s is required", label)
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fail("%s must be RFC 3339 with timezone", label)
	}
	return t.UTC(), nil
}
