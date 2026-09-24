package offlineresources

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"sthira/backend/internal/offlinepkg"
)

var (
	// ErrMissingPackageID indicates card is missing required package ID.
	ErrMissingPackageID = errors.New("offlineresources: card missing package_id")
	// ErrMissingJurisdiction indicates card is missing jurisdiction code.
	ErrMissingJurisdiction = errors.New("offlineresources: card missing jurisdiction")
	// ErrMissingValidity indicates card is missing effective_at or expires_at.
	ErrMissingValidity = errors.New("offlineresources: card missing validity timeframe")
	// ErrNoSafeZones indicates card does not define any evacuation safe zones.
	ErrNoSafeZones = errors.New("offlineresources: card must contain at least one safe zone")
	// ErrCardChecksumMismatch indicates card text does not match expected SHA-256 digest.
	ErrCardChecksumMismatch = errors.New("offlineresources: card content checksum mismatch")
)

// ValidateCardCompleteness verifies that an incident card contains all mandatory
// operational disaster guidance fields before generating citizen cards.
func ValidateCardCompleteness(card offlinepkg.PublicIncidentCard) error {
	if strings.TrimSpace(card.PackageID) == "" {
		return ErrMissingPackageID
	}
	if strings.TrimSpace(card.Jurisdiction) == "" {
		return ErrMissingJurisdiction
	}
	if strings.TrimSpace(card.EffectiveAt) == "" || strings.TrimSpace(card.ExpiresAt) == "" {
		return ErrMissingValidity
	}
	if len(card.SafeZones) == 0 {
		return ErrNoSafeZones
	}
	return nil
}

// ComputeCardTextDigest computes the SHA-256 hex digest of the generated card text.
func ComputeCardTextDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// VerifyCardTextIntegrity verifies generated card text against an expected SHA-256 digest using constant-time comparison.
func VerifyCardTextIntegrity(text, expectedDigest string) error {
	actual := ComputeCardTextDigest(text)
	cleanExpected := strings.ToLower(strings.TrimSpace(expectedDigest))
	if subtle.ConstantTimeCompare([]byte(actual), []byte(cleanExpected)) != 1 {
		return ErrCardChecksumMismatch
	}
	return nil
}

// FormatPlaintextCard generates an 80-column accessible plaintext evacuation card
// for citizens without smartphones, feature phones, SMS relays, and local printing.
func FormatPlaintextCard(card offlinepkg.PublicIncidentCard) (string, error) {
	if err := ValidateCardCompleteness(card); err != nil {
		return "", err
	}

	var sb strings.Builder
	sep80 := strings.Repeat("=", 80)
	sub80 := strings.Repeat("-", 80)

	sb.WriteString(sep80 + "\n")
	sb.WriteString("OFFICIAL DISASTER EVACUATION GUIDANCE CARD\n")
	sb.WriteString(fmt.Sprintf("Jurisdiction: %s | Authority: %s\n", card.Jurisdiction, card.EvidenceClass))
	sb.WriteString(fmt.Sprintf("Package: %s v%d | Valid: %s to %s\n", card.PackageID, card.Version, card.EffectiveAt, card.ExpiresAt))
	sb.WriteString(sep80 + "\n\n")

	sb.WriteString(fmt.Sprintf("EMERGENCY ALERT: %s\n", card.Alert.Headline))
	sb.WriteString(fmt.Sprintf("Severity: %s | Urgency: %s | Certainty: %s\n", card.Alert.Severity, card.Alert.Urgency, card.Alert.Certainty))
	if card.Alert.AreaDescription != "" {
		sb.WriteString(fmt.Sprintf("Affected Area: %s\n", card.Alert.AreaDescription))
	}
	sb.WriteString("\n")

	// Emergency Helplines
	sb.WriteString(sub80 + "\n")
	sb.WriteString("EMERGENCY DISPATCH & HELPLINES (DIAL DIRECTLY FROM DEVICE)\n")
	sb.WriteString(sub80 + "\n")
	sb.WriteString("* 112 - Unified Emergency Response Support System (ERSS)\n")
	for _, c := range card.EmergencyContacts {
		if c.Number != "112" {
			sb.WriteString(fmt.Sprintf("* %s - %s\n", c.Number, c.Name))
		}
	}
	sb.WriteString("\n")

	// Danger / Red Zones
	if len(card.RedZones) > 0 {
		sb.WriteString(sub80 + "\n")
		sb.WriteString("HAZARD & RED ZONES (STRICT EVACUATION REQUIRED)\n")
		sb.WriteString(sub80 + "\n")
		for _, rz := range card.RedZones {
			name := rz.Name
			if name == "" {
				name = rz.ID
			}
			sb.WriteString(fmt.Sprintf("* %s (%s)\n", name, rz.ID))
		}
		sb.WriteString("\n")
	}

	// Safe Zones & Facilities
	sb.WriteString(sub80 + "\n")
	sb.WriteString("APPROVED SAFE ZONES & RELIEF SHELTERS\n")
	sb.WriteString(sub80 + "\n")
	for _, sz := range card.SafeZones {
		capStr := "Variable"
		if sz.TotalCapacity != nil {
			capStr = fmt.Sprintf("%d beds", *sz.TotalCapacity)
		}
		sb.WriteString(fmt.Sprintf("* %s [%s]\n", sz.Name, sz.ID))
		sb.WriteString(fmt.Sprintf("  Status: %s | Role: %s | Capacity: %s\n", sz.Status, sz.Role, capStr))
		if len(sz.Services) > 0 {
			sb.WriteString(fmt.Sprintf("  Available Services: %s\n", strings.Join(sz.Services, ", ")))
		}

		// Facilities inside this safe zone
		for _, fac := range card.Facilities {
			if fac.SafeZoneID == sz.ID {
				addr := fac.Address
				if addr == "" {
					addr = "On-site shelter"
				}
				sb.WriteString(fmt.Sprintf("  - Facility: %s | Address: %s\n", fac.Name, addr))
				if fac.ContactPhone != "" {
					sb.WriteString(fmt.Sprintf("    Phone: %s\n", fac.ContactPhone))
				}
			}
		}
	}
	sb.WriteString("\n")

	// Approved Routes
	if len(card.ApprovedRoutes) > 0 {
		sb.WriteString(sub80 + "\n")
		sb.WriteString("AUTHORITATIVE EVACUATION ROUTES\n")
		sb.WriteString(sub80 + "\n")
		for _, rt := range card.ApprovedRoutes {
			sb.WriteString(fmt.Sprintf("* Route %s: From %s -> To Safe Zone %s\n", rt.ID, rt.FromZoneID, rt.ToSafeZoneID))
			sb.WriteString(fmt.Sprintf("  Mode: %s | Status: %s\n", rt.Mode, rt.Approval))
		}
		sb.WriteString("\n")
	}

	// Instructions
	if len(card.Instructions) > 0 {
		sb.WriteString(sub80 + "\n")
		sb.WriteString("GOVERNMENT SAFETY INSTRUCTIONS\n")
		sb.WriteString(sub80 + "\n")
		for _, inst := range card.Instructions {
			sb.WriteString(fmt.Sprintf("* %s: %s\n", inst.Title, inst.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(sep80 + "\n")
	sb.WriteString("STATUTORY NOTICE: Sthira is an interface over official emergency systems.\n")
	sb.WriteString("Sthira never places automated calls or silent dispatch requests.\n")
	sb.WriteString("Citizens in imminent danger must dial 112 directly through their phone dialler.\n")
	sb.WriteString(sep80 + "\n")

	return sb.String(), nil
}

// FormatMarkdownCard generates accessible markdown for mobile offline viewer or bulletin printing.
func FormatMarkdownCard(card offlinepkg.PublicIncidentCard) (string, error) {
	if err := ValidateCardCompleteness(card); err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Official Disaster Evacuation Guidance: %s\n\n", card.Alert.Headline))
	sb.WriteString(fmt.Sprintf("**Jurisdiction:** `%s` | **Authority:** `%s` | **Package:** `%s v%d`\n\n",
		card.Jurisdiction, card.EvidenceClass, card.PackageID, card.Version))
	sb.WriteString(fmt.Sprintf("**Valid Window:** `%s` to `%s`\n\n", card.EffectiveAt, card.ExpiresAt))

	sb.WriteString("## Emergency Alert Overview\n\n")
	sb.WriteString(fmt.Sprintf("- **Severity:** %s\n", card.Alert.Severity))
	sb.WriteString(fmt.Sprintf("- **Urgency:** %s\n", card.Alert.Urgency))
	sb.WriteString(fmt.Sprintf("- **Certainty:** %s\n", card.Alert.Certainty))
	if card.Alert.AreaDescription != "" {
		sb.WriteString(fmt.Sprintf("- **Affected Area:** %s\n", card.Alert.AreaDescription))
	}
	sb.WriteString("\n")

	// Helplines
	sb.WriteString("## Emergency Helplines\n\n")
	sb.WriteString("- **[112](tel:112)**: Unified Emergency Response Support System (ERSS)\n")
	for _, c := range card.EmergencyContacts {
		if c.Number != "112" {
			sb.WriteString(fmt.Sprintf("- **[%s](tel:%s)**: %s\n", c.Number, c.Number, c.Name))
		}
	}
	sb.WriteString("\n")

	// Red Zones
	if len(card.RedZones) > 0 {
		sb.WriteString("## Hazard & Red Zones (Evacuate Immediately)\n\n")
		for _, rz := range card.RedZones {
			name := rz.Name
			if name == "" {
				name = rz.ID
			}
			sb.WriteString(fmt.Sprintf("- **%s** (`%s`)\n", name, rz.ID))
		}
		sb.WriteString("\n")
	}

	// Safe Zones
	sb.WriteString("## Approved Safe Zones & Shelters\n\n")
	for _, sz := range card.SafeZones {
		capStr := "Variable"
		if sz.TotalCapacity != nil {
			capStr = fmt.Sprintf("%d beds", *sz.TotalCapacity)
		}
		sb.WriteString(fmt.Sprintf("### %s (`%s`)\n\n", sz.Name, sz.ID))
		sb.WriteString(fmt.Sprintf("- **Status:** %s\n- **Role:** %s\n- **Capacity:** %s\n", sz.Status, sz.Role, capStr))
		if len(sz.Services) > 0 {
			sb.WriteString(fmt.Sprintf("- **Services:** %s\n", strings.Join(sz.Services, ", ")))
		}
		for _, fac := range card.Facilities {
			if fac.SafeZoneID == sz.ID {
				sb.WriteString(fmt.Sprintf("- **Facility:** %s (Address: %s, Phone: %s)\n", fac.Name, fac.Address, fac.ContactPhone))
			}
		}
		sb.WriteString("\n")
	}

	// Approved Routes
	if len(card.ApprovedRoutes) > 0 {
		sb.WriteString("## Approved Evacuation Routes\n\n")
		for _, rt := range card.ApprovedRoutes {
			sb.WriteString(fmt.Sprintf("- **Route `%s`**: `%s` &rarr; Safe Zone `%s` (Mode: %s, Approval: %s)\n",
				rt.ID, rt.FromZoneID, rt.ToSafeZoneID, rt.Mode, rt.Approval))
		}
		sb.WriteString("\n")
	}

	// Instructions
	if len(card.Instructions) > 0 {
		sb.WriteString("## Official Instructions\n\n")
		for _, inst := range card.Instructions {
			sb.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", inst.Title, inst.Summary))
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("> **Statutory Notice:** Sthira is an interface over official emergency systems.\n")
	sb.WriteString("> Sthira never automatically calls 112 or dispatches responders. In danger, call [112](tel:112) immediately.\n")

	return sb.String(), nil
}
