// Package offlinepkg implements public offline card and manifest wire formats,
// deterministic canonical serialization, SHA-256 digests, Ed25519 digital
// signatures, and trust-store verification for Sthira v2 P5.
//
// All types defined herein are strictly public: they NEVER contain private
// citizen session tokens, civilian identifiers, private reservations, or
// uncommitted capacity claims.
package offlinepkg

import (
	"crypto/ed25519"
	"errors"
	"time"
)

// Sentinel errors defined by the P5 contract.
var (
	ErrInvalidSignature   = errors.New("offlinepkg: invalid digital signature")
	ErrSignerUnauthorized = errors.New("offlinepkg: signing key not authorized for jurisdiction")
	ErrKeyExpired         = errors.New("offlinepkg: signing key is expired or not yet valid")
	ErrChecksumMismatch   = errors.New("offlinepkg: checksum verification failed")
	ErrVersionRollback    = errors.New("offlinepkg: manifest revision is older than active revision")
	ErrMalformedData      = errors.New("offlinepkg: data failed structural validation")
	ErrUnknownKey         = errors.New("offlinepkg: unknown signing key")
	ErrKeyRevoked         = errors.New("offlinepkg: signing key has been revoked")
)

// ResourceType identifies the offline auxiliary asset type.
type ResourceType string

const (
	TypeVectorTiles    ResourceType = "VECTOR_TILES"
	TypeMapStyle       ResourceType = "MAP_STYLE"
	TypeMapSprite      ResourceType = "MAP_SPRITE"
	TypeMapGlyphs      ResourceType = "MAP_GLYPHS"
	TypeGazetteer      ResourceType = "GAZETTEER"
	TypeEmergencyAudio ResourceType = "EMERGENCY_AUDIO"
)

// Signature carries detached cryptographic signature metadata.
type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Value     string `json:"value"` // Base64 RFC 4648
}

// CriticalCardDescriptor is the manifest pointer to the region's active critical card.
type CriticalCardDescriptor struct {
	PackageID         string `json:"package_id"`
	Version           int    `json:"version"`
	URI               string `json:"uri"`
	ChecksumSHA256    string `json:"checksum_sha256"`
	UncompressedBytes int64  `json:"uncompressed_bytes"`
	CompressedBytes   int64  `json:"compressed_bytes"`
	ContentType       string `json:"content_type"`
}

// ResourceDescriptor describes an auxiliary offline asset (tiles, styles, fonts, audio).
type ResourceDescriptor struct {
	ResourceID     string       `json:"resource_id"`
	Type           ResourceType `json:"type"`
	URI            string       `json:"uri"`
	ChecksumSHA256 string       `json:"checksum_sha256"`
	ByteSize       int64        `json:"byte_size"`
	ContentType    string       `json:"content_type"`
	Required       bool         `json:"required"`
	Attribution    string       `json:"attribution"`
}

// RevocationBlock enumerates revoked and superseded operational entities.
type RevocationBlock struct {
	RevokedPackages    []string            `json:"revoked_packages,omitempty"`
	CancelledRoutes    []string            `json:"cancelled_routes,omitempty"`
	SupersededVersions []SupersededVersion `json:"superseded_versions,omitempty"`
}

// SupersededVersion pairs a package ID with an older superseded version.
type SupersededVersion struct {
	PackageID string `json:"package_id"`
	Version   int    `json:"version"`
}

// ManifestProvenance identifies the authority issuing the manifest.
type ManifestProvenance struct {
	Authority     string `json:"authority"`
	DatasetID     string `json:"dataset_id"`
	EvidenceClass string `json:"evidence_class"`
}

// Manifest is the root public discovery document for a jurisdiction/region.
type Manifest struct {
	SchemaVersion  string                 `json:"schema_version"`
	ManifestID     string                 `json:"manifest_id"`
	Jurisdiction   string                 `json:"jurisdiction"`
	Revision       int                    `json:"revision"`
	GeneratedAt    string                 `json:"generated_at"`
	ValidUntil     string                 `json:"valid_until"`
	SourceStatus   string                 `json:"source_status"`
	CriticalCard   CriticalCardDescriptor `json:"critical_card"`
	Resources      []ResourceDescriptor   `json:"resources,omitempty"`
	Revocations    RevocationBlock        `json:"revocations"`
	Provenance     ManifestProvenance     `json:"provenance"`
	ChecksumSHA256 string                 `json:"checksum_sha256"`
	Signature      *Signature             `json:"signature,omitempty"`
}

// PublicIncidentCard is the standalone, offline-executable guidance for citizens.
type PublicIncidentCard struct {
	SchemaVersion     string             `json:"schema_version"`
	PackageID         string             `json:"package_id"`
	Version           int                `json:"version"`
	Jurisdiction      string             `json:"jurisdiction"`
	EvidenceClass     string             `json:"evidence_class"`
	EffectiveAt       string             `json:"effective_at"`
	ExpiresAt         string             `json:"expires_at"`
	Alert             AlertCard          `json:"alert"`
	RedZones          []RedZoneCard      `json:"red_zones"`
	SafeZones         []SafeZoneCard     `json:"safe_zones"`
	ApprovedRoutes    []RouteCard        `json:"approved_routes"`
	Facilities        []FacilityCard     `json:"facilities"`
	Instructions      []InstructionCard  `json:"instructions"`
	EmergencyContacts []EmergencyContact `json:"emergency_contacts"`
	AllocationPolicy  PolicyCard         `json:"allocation_policy"`
	ChecksumSHA256    string             `json:"checksum_sha256"`
	Signature         *Signature         `json:"signature,omitempty"`
}

// AlertCard conveys emergency alert severity, urgency, and description.
type AlertCard struct {
	Identifier      string `json:"identifier"`
	Sender          string `json:"sender"`
	Headline        string `json:"headline"`
	Severity        string `json:"severity"`
	Urgency         string `json:"urgency"`
	Certainty       string `json:"certainty"`
	AreaDescription string `json:"area_description"`
}

// RedZoneCard describes a hazard or evacuation zone.
type RedZoneCard struct {
	ID       string    `json:"id"`
	Name     string    `json:"name,omitempty"`
	Centroid []float64 `json:"centroid,omitempty"`
	Geometry any       `json:"geometry"`
}

// SafeZoneCard describes an evacuation destination or relief camp.
type SafeZoneCard struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	Status        string    `json:"status"`
	CapacityMode  string    `json:"capacity_mode"`
	TotalCapacity *int      `json:"total_capacity,omitempty"`
	Location      []float64 `json:"location,omitempty"`
	Services      []string  `json:"services,omitempty"`
}

// RouteCard defines an authorized evacuation path.
type RouteCard struct {
	ID           string `json:"id"`
	FromZoneID   string `json:"from_zone_id"`
	ToSafeZoneID string `json:"to_safe_zone_id"`
	Mode         string `json:"mode"`
	Approval     string `json:"approval"`
	VerifiedBy   string `json:"verified_by,omitempty"`
	VerifiedAt   string `json:"verified_at,omitempty"`
	ValidFrom    string `json:"valid_from,omitempty"`
	ValidUntil   string `json:"valid_until,omitempty"`
	Geometry     any    `json:"geometry"`
}

// FacilityCard defines a physical structure within a safe zone.
type FacilityCard struct {
	ID           string `json:"id"`
	SafeZoneID   string `json:"safe_zone_id"`
	Name         string `json:"name"`
	Address      string `json:"address,omitempty"`
	ContactPhone string `json:"contact_phone,omitempty"`
}

// InstructionCard contains localized advisory text for citizens.
type InstructionCard struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
}

// EmergencyContact represents an official phone number for device dialler handoff.
type EmergencyContact struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

// PolicyCard defines government-owned stay and routing rules.
type PolicyCard struct {
	Order                    []string `json:"order"`
	ReservationExpirySeconds *int     `json:"reservation_expiry_seconds,omitempty"`
	TemporaryStayMinDays     *int     `json:"temporary_stay_min_days,omitempty"`
	TemporaryStayMaxDays     *int     `json:"temporary_stay_max_days,omitempty"`
	AllowWalkIns             *bool    `json:"allow_walk_ins,omitempty"`
	AllowTransfers           *bool    `json:"allow_transfers,omitempty"`
	RouteRequired            *bool    `json:"route_required,omitempty"`
}

// TrustedKey represents a pinned authority public key.
type TrustedKey struct {
	KeyID                 string            `json:"key_id"`
	PublicKey             ed25519.PublicKey `json:"-"`
	PermittedJurisdiction string            `json:"permitted_jurisdiction"`
	ValidFrom             time.Time         `json:"valid_from"`
	ValidUntil            time.Time         `json:"valid_until"`
	Revoked               bool              `json:"revoked"`
}

// TrustStore defines the signature verification and key lookup contract.
type TrustStore interface {
	VerifySignature(keyID, jurisdiction string, canonicalBytes []byte, sigBase64 string) error
	LookupKey(keyID string) (TrustedKey, error)
}
