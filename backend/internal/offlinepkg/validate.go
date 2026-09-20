package offlinepkg

import (
	"fmt"
	"time"
)

// ValidateManifestStructure checks that a manifest meets all structural and
// semantic requirements. It does NOT check cryptographic authenticity.
func ValidateManifestStructure(m *Manifest) error {
	if m == nil {
		return fmt.Errorf("%w: manifest is nil", ErrMalformedData)
	}
	if m.SchemaVersion != "3.0" {
		return fmt.Errorf("%w: unsupported schema_version %q", ErrMalformedData, m.SchemaVersion)
	}
	if m.ManifestID == "" {
		return fmt.Errorf("%w: manifest_id is required", ErrMalformedData)
	}
	if m.Jurisdiction == "" {
		return fmt.Errorf("%w: jurisdiction is required", ErrMalformedData)
	}
	if m.Revision < 1 {
		return fmt.Errorf("%w: revision must be >= 1", ErrMalformedData)
	}
	genAt, err := time.Parse(time.RFC3339, m.GeneratedAt)
	if err != nil {
		return fmt.Errorf("%w: invalid generated_at timestamp: %v", ErrMalformedData, err)
	}
	validUntil, err := time.Parse(time.RFC3339, m.ValidUntil)
	if err != nil {
		return fmt.Errorf("%w: invalid valid_until timestamp: %v", ErrMalformedData, err)
	}
	if !validUntil.After(genAt) {
		return fmt.Errorf("%w: valid_until (%v) must be after generated_at (%v)", ErrMalformedData, validUntil, genAt)
	}
	if m.SourceStatus == "" {
		return fmt.Errorf("%w: source_status is required", ErrMalformedData)
	}

	// Critical Card Descriptor checks
	cc := m.CriticalCard
	if cc.PackageID == "" {
		return fmt.Errorf("%w: critical_card.package_id is required", ErrMalformedData)
	}
	if cc.Version < 1 {
		return fmt.Errorf("%w: critical_card.version must be >= 1", ErrMalformedData)
	}
	if cc.URI == "" {
		return fmt.Errorf("%w: critical_card.uri is required", ErrMalformedData)
	}
	if len(cc.ChecksumSHA256) != 64 {
		return fmt.Errorf("%w: critical_card.checksum_sha256 must be 64 hex chars", ErrMalformedData)
	}
	if cc.UncompressedBytes <= 0 {
		return fmt.Errorf("%w: critical_card.uncompressed_bytes must be > 0", ErrMalformedData)
	}
	if cc.UncompressedBytes > 65536 {
		return fmt.Errorf("%w: critical_card.uncompressed_bytes (%d) exceeds 64 KiB budget", ErrMalformedData, cc.UncompressedBytes)
	}
	if cc.CompressedBytes <= 0 {
		return fmt.Errorf("%w: critical_card.compressed_bytes must be > 0", ErrMalformedData)
	}
	if cc.CompressedBytes > 65536 {
		return fmt.Errorf("%w: critical_card.compressed_bytes (%d) exceeds 64 KiB budget", ErrMalformedData, cc.CompressedBytes)
	}
	if cc.ContentType == "" {
		return fmt.Errorf("%w: critical_card.content_type is required", ErrMalformedData)
	}

	// Resources validation
	for i, res := range m.Resources {
		if res.ResourceID == "" {
			return fmt.Errorf("%w: resource[%d].resource_id is required", ErrMalformedData, i)
		}
		switch res.Type {
		case TypeVectorTiles, TypeMapStyle, TypeMapSprite, TypeMapGlyphs, TypeGazetteer, TypeEmergencyAudio:
		default:
			return fmt.Errorf("%w: resource[%d] unknown type %q", ErrMalformedData, i, res.Type)
		}
		if res.URI == "" {
			return fmt.Errorf("%w: resource[%d].uri is required", ErrMalformedData, i)
		}
		if len(res.ChecksumSHA256) != 64 {
			return fmt.Errorf("%w: resource[%d].checksum_sha256 must be 64 hex chars", ErrMalformedData, i)
		}
		if res.ByteSize <= 0 {
			return fmt.Errorf("%w: resource[%d].byte_size must be > 0", ErrMalformedData, i)
		}
		if res.Attribution == "" {
			return fmt.Errorf("%w: resource[%d].attribution is required for license transparency", ErrMalformedData, i)
		}
	}

	// Provenance validation
	if m.Provenance.Authority == "" {
		return fmt.Errorf("%w: provenance.authority is required", ErrMalformedData)
	}
	if m.Provenance.DatasetID == "" {
		return fmt.Errorf("%w: provenance.dataset_id is required", ErrMalformedData)
	}
	switch m.Provenance.EvidenceClass {
	case "SYNTHETIC_DEMO", "AUTHORIZED_OPERATIONAL", "CAPTURED_OFFICIAL_SAMPLE", "AUTHORIZED_SHADOW":
	default:
		return fmt.Errorf("%w: invalid provenance.evidence_class %q", ErrMalformedData, m.Provenance.EvidenceClass)
	}

	if len(m.ChecksumSHA256) != 64 {
		return fmt.Errorf("%w: checksum_sha256 must be 64 hex characters", ErrMalformedData)
	}

	if m.Signature != nil {
		if m.Signature.Algorithm != "Ed25519" {
			return fmt.Errorf("%w: signature algorithm must be Ed25519", ErrMalformedData)
		}
		if m.Signature.KeyID == "" {
			return fmt.Errorf("%w: signature key_id is required", ErrMalformedData)
		}
		if m.Signature.Value == "" {
			return fmt.Errorf("%w: signature value is required", ErrMalformedData)
		}
	}

	return nil
}

// ValidateCardStructure checks that an incident card meets all structural and
// semantic requirements. It does NOT check cryptographic authenticity.
func ValidateCardStructure(c *PublicIncidentCard) error {
	if c == nil {
		return fmt.Errorf("%w: card is nil", ErrMalformedData)
	}
	if c.SchemaVersion != "3.0" {
		return fmt.Errorf("%w: unsupported schema_version %q", ErrMalformedData, c.SchemaVersion)
	}
	if c.PackageID == "" {
		return fmt.Errorf("%w: package_id is required", ErrMalformedData)
	}
	if c.Version < 1 {
		return fmt.Errorf("%w: version must be >= 1", ErrMalformedData)
	}
	if c.Jurisdiction == "" {
		return fmt.Errorf("%w: jurisdiction is required", ErrMalformedData)
	}
	switch c.EvidenceClass {
	case "SYNTHETIC_DEMO", "AUTHORIZED_OPERATIONAL", "CAPTURED_OFFICIAL_SAMPLE", "AUTHORIZED_SHADOW":
	default:
		return fmt.Errorf("%w: invalid evidence_class %q", ErrMalformedData, c.EvidenceClass)
	}
	effAt, err := time.Parse(time.RFC3339, c.EffectiveAt)
	if err != nil {
		return fmt.Errorf("%w: invalid effective_at timestamp: %v", ErrMalformedData, err)
	}
	expAt, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return fmt.Errorf("%w: invalid expires_at timestamp: %v", ErrMalformedData, err)
	}
	if !expAt.After(effAt) {
		return fmt.Errorf("%w: expires_at (%v) must be after effective_at (%v)", ErrMalformedData, expAt, effAt)
	}

	// Alert
	if c.Alert.Identifier == "" {
		return fmt.Errorf("%w: alert.identifier is required", ErrMalformedData)
	}
	if c.Alert.Sender == "" {
		return fmt.Errorf("%w: alert.sender is required", ErrMalformedData)
	}
	if c.Alert.Headline == "" {
		return fmt.Errorf("%w: alert.headline is required", ErrMalformedData)
	}
	if c.Alert.Severity == "" || c.Alert.Urgency == "" || c.Alert.Certainty == "" {
		return fmt.Errorf("%w: alert severity, urgency, and certainty are required", ErrMalformedData)
	}

	// Red Zones
	if len(c.RedZones) == 0 {
		return fmt.Errorf("%w: red_zones must contain at least one zone", ErrMalformedData)
	}
	for i, rz := range c.RedZones {
		if rz.ID == "" {
			return fmt.Errorf("%w: red_zones[%d].id is required", ErrMalformedData, i)
		}
	}

	// Safe Zones
	if len(c.SafeZones) == 0 {
		return fmt.Errorf("%w: safe_zones must contain at least one zone", ErrMalformedData)
	}
	safeSet := make(map[string]bool, len(c.SafeZones))
	for i, sz := range c.SafeZones {
		if sz.ID == "" {
			return fmt.Errorf("%w: safe_zones[%d].id is required", ErrMalformedData, i)
		}
		safeSet[sz.ID] = true
		if sz.Name == "" {
			return fmt.Errorf("%w: safe_zones[%d].name is required", ErrMalformedData, i)
		}
		if sz.Role == "" {
			return fmt.Errorf("%w: safe_zones[%d].role is required", ErrMalformedData, i)
		}
		if sz.Status == "" {
			return fmt.Errorf("%w: safe_zones[%d].status is required", ErrMalformedData, i)
		}
		if sz.CapacityMode == "" {
			return fmt.Errorf("%w: safe_zones[%d].capacity_mode is required", ErrMalformedData, i)
		}
		if sz.TotalCapacity != nil && *sz.TotalCapacity < 0 {
			return fmt.Errorf("%w: safe_zones[%d].total_capacity cannot be negative", ErrMalformedData, i)
		}
		if len(sz.Location) > 0 {
			if len(sz.Location) != 2 {
				return fmt.Errorf("%w: safe_zones[%d].location must be [lon, lat]", ErrMalformedData, i)
			}
			lon, lat := sz.Location[0], sz.Location[1]
			if lon < -180 || lon > 180 || lat < -90 || lat > 90 {
				return fmt.Errorf("%w: safe_zones[%d].location out of WGS84 bounds", ErrMalformedData, i)
			}
		}
	}

	// Approved Routes
	for i, rt := range c.ApprovedRoutes {
		if rt.ID == "" {
			return fmt.Errorf("%w: approved_routes[%d].id is required", ErrMalformedData, i)
		}
		if rt.FromZoneID == "" {
			return fmt.Errorf("%w: approved_routes[%d].from_zone_id is required", ErrMalformedData, i)
		}
		if rt.ToSafeZoneID == "" {
			return fmt.Errorf("%w: approved_routes[%d].to_safe_zone_id is required", ErrMalformedData, i)
		}
		if !safeSet[rt.ToSafeZoneID] {
			return fmt.Errorf("%w: route %q destination %q is not in safe_zones", ErrMalformedData, rt.ID, rt.ToSafeZoneID)
		}
		switch rt.Mode {
		case "FOOT", "VEHICLE", "AMBULANCE":
		default:
			return fmt.Errorf("%w: approved_routes[%d].mode must be FOOT, VEHICLE, or AMBULANCE", ErrMalformedData, i)
		}
		if rt.Approval == "" {
			return fmt.Errorf("%w: approved_routes[%d].approval is required", ErrMalformedData, i)
		}
		if rt.ValidFrom != "" && rt.ValidUntil != "" {
			from, err := time.Parse(time.RFC3339, rt.ValidFrom)
			if err != nil {
				return fmt.Errorf("%w: route %q valid_from parse: %v", ErrMalformedData, rt.ID, err)
			}
			until, err := time.Parse(time.RFC3339, rt.ValidUntil)
			if err != nil {
				return fmt.Errorf("%w: route %q valid_until parse: %v", ErrMalformedData, rt.ID, err)
			}
			if !until.After(from) {
				return fmt.Errorf("%w: route %q valid_until must be after valid_from", ErrMalformedData, rt.ID)
			}
		}
	}

	// Facilities
	if len(c.Facilities) == 0 {
		return fmt.Errorf("%w: facilities must contain at least one entry", ErrMalformedData)
	}
	for i, f := range c.Facilities {
		if f.ID == "" {
			return fmt.Errorf("%w: facilities[%d].id is required", ErrMalformedData, i)
		}
		if f.SafeZoneID == "" {
			return fmt.Errorf("%w: facilities[%d].safe_zone_id is required", ErrMalformedData, i)
		}
		if !safeSet[f.SafeZoneID] {
			return fmt.Errorf("%w: facility %q references unknown safe zone %q", ErrMalformedData, f.ID, f.SafeZoneID)
		}
		if f.Name == "" {
			return fmt.Errorf("%w: facilities[%d].name is required", ErrMalformedData, i)
		}
	}

	// Instructions
	if len(c.Instructions) == 0 {
		return fmt.Errorf("%w: instructions must contain at least one entry", ErrMalformedData)
	}
	for i, ins := range c.Instructions {
		if ins.ID == "" {
			return fmt.Errorf("%w: instructions[%d].id is required", ErrMalformedData, i)
		}
		if ins.Language == "" {
			return fmt.Errorf("%w: instructions[%d].language is required", ErrMalformedData, i)
		}
		if ins.Title == "" {
			return fmt.Errorf("%w: instructions[%d].title is required", ErrMalformedData, i)
		}
	}

	// Emergency Contacts
	if len(c.EmergencyContacts) == 0 {
		return fmt.Errorf("%w: emergency_contacts must contain at least one contact", ErrMalformedData)
	}
	for i, ec := range c.EmergencyContacts {
		if ec.Name == "" {
			return fmt.Errorf("%w: emergency_contacts[%d].name is required", ErrMalformedData, i)
		}
		if ec.Number == "" {
			return fmt.Errorf("%w: emergency_contacts[%d].number is required", ErrMalformedData, i)
		}
	}

	// Allocation Policy
	if len(c.AllocationPolicy.Order) == 0 {
		return fmt.Errorf("%w: allocation_policy.order is required", ErrMalformedData)
	}
	if c.AllocationPolicy.ReservationExpirySeconds != nil && *c.AllocationPolicy.ReservationExpirySeconds <= 0 {
		return fmt.Errorf("%w: reservation_expiry_seconds must be > 0", ErrMalformedData)
	}
	minDays, maxDays := c.AllocationPolicy.TemporaryStayMinDays, c.AllocationPolicy.TemporaryStayMaxDays
	if (minDays == nil) != (maxDays == nil) {
		return fmt.Errorf("%w: temporary_stay min and max days must both be specified", ErrMalformedData)
	}
	if minDays != nil && maxDays != nil {
		if *minDays < 1 || *minDays > *maxDays {
			return fmt.Errorf("%w: invalid temporary_stay day range [%d, %d]", ErrMalformedData, *minDays, *maxDays)
		}
	}

	if len(c.ChecksumSHA256) != 64 {
		return fmt.Errorf("%w: checksum_sha256 must be 64 hex characters", ErrMalformedData)
	}

	if c.Signature != nil {
		if c.Signature.Algorithm != "Ed25519" {
			return fmt.Errorf("%w: signature algorithm must be Ed25519", ErrMalformedData)
		}
		if c.Signature.KeyID == "" {
			return fmt.Errorf("%w: signature key_id is required", ErrMalformedData)
		}
		if c.Signature.Value == "" {
			return fmt.Errorf("%w: signature value is required", ErrMalformedData)
		}
	}

	return nil
}

// VerifyManifest performs full structural validation, verifies that the
// checksum matches canonical bytes, and cryptographically checks the signature
// against the provided TrustStore.
func VerifyManifest(m *Manifest, ts TrustStore) error {
	if err := ValidateManifestStructure(m); err != nil {
		return err
	}
	if m.Signature == nil {
		return fmt.Errorf("%w: manifest has no signature", ErrInvalidSignature)
	}

	canonical, err := CanonicalBytes(m)
	if err != nil {
		return err
	}

	expectedChecksum := ChecksumSHA256(canonical)
	if m.ChecksumSHA256 != expectedChecksum {
		return fmt.Errorf("%w: manifest checksum mismatch (declared %q != computed %q)",
			ErrChecksumMismatch, m.ChecksumSHA256, expectedChecksum)
	}

	return ts.VerifySignature(m.Signature.KeyID, m.Jurisdiction, canonical, m.Signature.Value)
}

// VerifyCard performs full structural validation, verifies that the
// checksum matches canonical bytes, and cryptographically checks the signature
// against the provided TrustStore.
func VerifyCard(c *PublicIncidentCard, ts TrustStore) error {
	if err := ValidateCardStructure(c); err != nil {
		return err
	}
	if c.Signature == nil {
		return fmt.Errorf("%w: card has no signature", ErrInvalidSignature)
	}

	canonical, err := CanonicalBytes(c)
	if err != nil {
		return err
	}

	expectedChecksum := ChecksumSHA256(canonical)
	if c.ChecksumSHA256 != expectedChecksum {
		return fmt.Errorf("%w: card checksum mismatch (declared %q != computed %q)",
			ErrChecksumMismatch, c.ChecksumSHA256, expectedChecksum)
	}

	return ts.VerifySignature(c.Signature.KeyID, c.Jurisdiction, canonical, c.Signature.Value)
}

// CheckRevisionRollback enforces monotonic manifest revisions. If incomingRevision < activeRevision,
// it returns ErrVersionRollback.
func CheckRevisionRollback(incomingRevision, activeRevision int) error {
	if incomingRevision < activeRevision {
		return fmt.Errorf("%w: incoming revision %d < active revision %d",
			ErrVersionRollback, incomingRevision, activeRevision)
	}
	return nil
}

// IsPackageRevoked checks if packageID (or specific version) is tombstoned.
func IsPackageRevoked(packageID string, version int, rev *RevocationBlock) bool {
	if rev == nil {
		return false
	}
	for _, id := range rev.RevokedPackages {
		if id == packageID {
			return true
		}
	}
	for _, sv := range rev.SupersededVersions {
		if sv.PackageID == packageID && sv.Version == version {
			return true
		}
	}
	return false
}

// IsRouteCancelled checks if routeID is tombstoned in the manifest.
func IsRouteCancelled(routeID string, rev *RevocationBlock) bool {
	if rev == nil {
		return false
	}
	for _, id := range rev.CancelledRoutes {
		if id == routeID {
			return true
		}
	}
	return false
}
