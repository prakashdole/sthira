package offlineresources

import (
	"fmt"
	"mime"
	"strings"
)

// ValidateDescriptor runs all structural checks on a single ResourceDescriptor
// and returns the first failure as a *ValidatorError.
//
// The check order matters: identity → digest → size → media type →
// attribution → license/redistribution. Tests assert that an early failure
// is reported before later checks run, so the message is stable across
// runs.
func ValidateDescriptor(d ResourceDescriptor) error {
	if d.ResourceID == "" {
		return &ValidatorError{Reason: ReasonEmptyResourceID, Field: "resource_id"}
	}
	if d.URI == "" {
		return &ValidatorError{Reason: ReasonEmptyURI, Field: "uri"}
	}
	if !isKnownType(d.Type) {
		return &ValidatorError{
			Reason: ReasonUnknownResourceType,
			Field:  "type",
		}
	}
	if !digestRegex.MatchString(d.ChecksumSHA256) {
		return &ValidatorError{
			Reason: ReasonInvalidDigest,
			Field:  "checksum_sha256",
		}
	}
	if d.ByteSize <= 0 {
		return &ValidatorError{
			Reason: ReasonNonPositiveSize,
			Field:  "byte_size",
		}
	}
	if d.ContentType == "" {
		return &ValidatorError{
			Reason: ReasonEmptyContentType,
			Field:  "content_type",
		}
	}
	// mime.ParseMediaType rejects malformed media types (missing slash,
	// stray whitespace, garbage parameters). Run it before classify so a
	// malformed type surfaces as ReasonInvalidContentType, not the
	// less-specific ReasonUnsupportedFormat.
	if _, _, err := mime.ParseMediaType(d.ContentType); err != nil {
		return &ValidatorError{
			Reason: ReasonInvalidContentType,
			Field:  "content_type",
		}
	}
	if _, ok := classify(d.Type, d.ContentType); !ok {
		return &ValidatorError{
			Reason: fmt.Sprintf("%s (content_type=%s for type=%s)",
				ReasonUnsupportedFormat, d.ContentType, d.Type),
			Field: "content_type",
		}
	}
	if strings.TrimSpace(d.Attribution) == "" {
		return &ValidatorError{
			Reason: ReasonEmptyAttribution,
			Field:  "attribution",
		}
	}
	if d.MinZoom != nil || d.MaxZoom != nil {
		if d.MinZoom == nil || d.MaxZoom == nil {
			return &ValidatorError{
				Reason: ReasonInvalidZoom,
				Field:  "min_zoom/max_zoom",
			}
		}
		if *d.MinZoom < 0 || *d.MaxZoom > 22 || *d.MinZoom > *d.MaxZoom {
			return &ValidatorError{
				Reason: ReasonInvalidZoom,
				Field:  "min_zoom/max_zoom",
			}
		}
	}
	if d.BBox != nil {
		if d.BBox.MinLon > d.BBox.MaxLon || d.BBox.MinLat > d.BBox.MaxLat {
			return &ValidatorError{
				Reason: ReasonInvalidBBox,
				Field:  "bbox",
			}
		}
		if d.BBox.MinLon < -180 || d.BBox.MaxLon > 180 ||
			d.BBox.MinLat < -90 || d.BBox.MaxLat > 90 {
			return &ValidatorError{Reason: ReasonInvalidBBox, Field: "bbox"}
		}
	}
	for _, lang := range d.Languages {
		if !languageTagRegex.MatchString(lang) {
			return &ValidatorError{
				Reason: ReasonInvalidLanguageTag,
				Field:  "languages",
			}
		}
	}
	if err := validateLicense(d); err != nil {
		return err
	}
	if ceiling := bytesForType(d.Type); ceiling > 0 && d.ByteSize > ceiling {
		return &ValidatorError{
			Reason: fmt.Sprintf("%s (declared=%d, ceiling=%d for type=%s)",
				ReasonCriticalExceedsBudget, d.ByteSize, ceiling, d.Type),
			Field: "byte_size",
		}
	}
	return nil
}

// validateLicense enforces the small set of rules that the optional License
// field must obey when present. It never claims the declared permission is
// legally valid — O06 evidence is required for live activation.
func validateLicense(d ResourceDescriptor) error {
	if d.License == nil {
		return nil
	}
	switch d.License.Redistribution {
	case RedistUndeclared:
		// A license struct was provided but the redistribution field is
		// empty. Treat as O06-pending rather than a structural failure so
		// the integration agent can surface "verification pending" instead
		// of rejecting valid descriptors.
		return nil
	case RedistDenied:
		// An explicitly denied redistribution means the descriptor must
		// never enter an offline pack. The audit catches it via
		// LicenseDenied; the descriptor validator also rejects it
		// outright so individual calls fail fast.
		return &ValidatorError{
			Reason: ReasonDeniedRedist,
			Field:  "license.redistribution",
		}
	case RedistAllowed, RedistConditional:
		// Recognised value; SPDX/name presence is checked below.
	default:
		return &ValidatorError{
			Reason: ReasonUnknownRedist,
			Field:  "license.redistribution",
		}
	}
	if d.License.Redistribution == RedistAllowed || d.License.Redistribution == RedistConditional {
		// A redistributable descriptor must point at something. The
		// presence of either an SPDX id or a license name is sufficient;
		// the integration agent promotes this into evidence.
		if d.License.SPDXIdentifier == "" && d.License.LicenseName == "" {
			return &ValidatorError{
				Reason: ReasonEmptyLicense,
				Field:  "license",
			}
		}
	}
	return nil
}

// LicenseVerificationStatus describes the result of inspecting the optional
// license/redistribution metadata. It is reported by the audit so the
// integration agent can gate live activation on O06 evidence.
type LicenseVerificationStatus string

const (
	// LicenseNotDeclared means the descriptor carries no License field.
	// The validator does not reject it; the integration agent may still
	// activate under the O06-pending default.
	LicenseNotDeclared LicenseVerificationStatus = "NOT_DECLARED"
	// LicensePending means a License field is present but redistribution
	// is undeclared or the SPDX/name is empty. O06 evidence is required
	// before this descriptor can be promoted to live.
	LicensePending LicenseVerificationStatus = "VERIFICATION_PENDING"
	// LicenseDeclared means a License field is present with a known
	// redistribution permission and an SPDX identifier or name. It is
	// metadata only; legal validity still depends on O06 evidence.
	LicenseDeclared LicenseVerificationStatus = "DECLARED"
	// LicenseDenied means redistribution is explicitly denied. This
	// descriptor must never enter an offline pack.
	LicenseDenied LicenseVerificationStatus = "REDISTRIBUTION_DENIED"
)

// LicenseStatus inspects the License field of a descriptor and returns one
// of the LicenseVerificationStatus constants above.
func LicenseStatus(d ResourceDescriptor) LicenseVerificationStatus {
	if d.License == nil {
		return LicenseNotDeclared
	}
	if d.License.Redistribution == RedistDenied {
		return LicenseDenied
	}
	if d.License.Redistribution == RedistUndeclared {
		return LicensePending
	}
	// Conditional redistribution is treated as Pending unless the
	// descriptor explicitly declares offline redistribution is allowed
	// (DeclaredOfflineOK=true). The integration agent surfaces
	// Pending in the audit under LicensePending so O06 evidence can
	// upgrade it to LicenseDeclared.
	if d.License.Redistribution == RedistConditional && !d.License.DeclaredOfflineOK {
		return LicensePending
	}
	if d.License.SPDXIdentifier == "" && d.License.LicenseName == "" {
		return LicensePending
	}
	return LicenseDeclared
}

func isKnownType(t ResourceType) bool {
	switch t {
	case TypeVectorTiles, TypeMapStyle, TypeMapSprite, TypeMapGlyphs,
		TypeGazetteer, TypeEmergencyAudio:
		return true
	}
	return false
}
