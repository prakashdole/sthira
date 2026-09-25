package offlinepkg

import (
	"errors"
	"testing"
)

func TestVerifyManifestAndCardEndToEnd(t *testing.T) {
	priv, tk := generateTestKey(t, "key-kl-01", "KL")
	store := NewTrustStore(tk)

	m := validManifestFixture(t, "key-kl-01", priv)
	if err := VerifyManifest(m, store); err != nil {
		t.Fatalf("VerifyManifest failed: %v", err)
	}

	c := validCardFixture(t, "key-kl-01", priv)
	if err := VerifyCard(c, store); err != nil {
		t.Fatalf("VerifyCard failed: %v", err)
	}
}

func TestVerifyManifestChecksumMismatch(t *testing.T) {
	priv, tk := generateTestKey(t, "key-kl-01", "KL")
	store := NewTrustStore(tk)

	m := validManifestFixture(t, "key-kl-01", priv)
	// Tamper with data without recomputing checksum
	m.ManifestID = "TAMPERED-ID"

	err := VerifyManifest(m, store)
	if err == nil {
		t.Fatal("VerifyManifest succeeded on tampered manifest; want error")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}
}

func TestVerifyCardChecksumMismatch(t *testing.T) {
	priv, tk := generateTestKey(t, "key-kl-01", "KL")
	store := NewTrustStore(tk)

	c := validCardFixture(t, "key-kl-01", priv)
	// Tamper with card headline
	c.Alert.Headline = "TAMPERED WARNING"

	err := VerifyCard(c, store)
	if err == nil {
		t.Fatal("VerifyCard succeeded on tampered card; want error")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}
}

func TestValidateManifestStructuralErrors(t *testing.T) {
	priv, _ := generateTestKey(t, "key-1", "KL")

	tests := []struct {
		name   string
		mutate func(m *Manifest)
	}{
		{"missing schema_version", func(m *Manifest) { m.SchemaVersion = "2.0" }},
		{"missing manifest_id", func(m *Manifest) { m.ManifestID = "" }},
		{"missing jurisdiction", func(m *Manifest) { m.Jurisdiction = "" }},
		{"revision < 1", func(m *Manifest) { m.Revision = 0 }},
		{"invalid generated_at", func(m *Manifest) { m.GeneratedAt = "not-a-date" }},
		{"valid_until <= generated_at", func(m *Manifest) { m.ValidUntil = m.GeneratedAt }},
		{"missing critical_card package_id", func(m *Manifest) { m.CriticalCard.PackageID = "" }},
		{"critical_card compressed_bytes > 64 KiB", func(m *Manifest) { m.CriticalCard.CompressedBytes = 70000 }},
		{"critical_card uncompressed_bytes > 64 KiB", func(m *Manifest) { m.CriticalCard.UncompressedBytes = 70000 }},
		{"resource missing attribution", func(m *Manifest) { m.Resources[0].Attribution = "" }},
		{"missing provenance authority", func(m *Manifest) { m.Provenance.Authority = "" }},
		{"invalid evidence class", func(m *Manifest) { m.Provenance.EvidenceClass = "UNAPPROVED_DRAFT" }},
		{"invalid checksum length", func(m *Manifest) { m.ChecksumSHA256 = "tooshort" }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifestFixture(t, "key-1", priv)
			tc.mutate(m)
			err := ValidateManifestStructure(m)
			if err == nil {
				t.Fatalf("ValidateManifestStructure succeeded for %s; want ErrMalformedData", tc.name)
			}
			if !errors.Is(err, ErrMalformedData) {
				t.Fatalf("err = %v, want ErrMalformedData", err)
			}
		})
	}
}

func TestValidateCardStructuralErrors(t *testing.T) {
	priv, _ := generateTestKey(t, "key-1", "KL")

	tests := []struct {
		name   string
		mutate func(c *PublicIncidentCard)
	}{
		{"missing schema_version", func(c *PublicIncidentCard) { c.SchemaVersion = "1.0" }},
		{"missing package_id", func(c *PublicIncidentCard) { c.PackageID = "" }},
		{"version < 1", func(c *PublicIncidentCard) { c.Version = 0 }},
		{"expires_at before effective_at", func(c *PublicIncidentCard) { c.ExpiresAt = c.EffectiveAt }},
		{"missing alert identifier", func(c *PublicIncidentCard) { c.Alert.Identifier = "" }},
		{"empty red_zones", func(c *PublicIncidentCard) { c.RedZones = nil }},
		{"empty safe_zones", func(c *PublicIncidentCard) { c.SafeZones = nil }},
		{"negative capacity", func(c *PublicIncidentCard) { c.SafeZones[0].TotalCapacity = intPtr(-10) }},
		{"invalid location longitude", func(c *PublicIncidentCard) { c.SafeZones[0].Location = []float64{200.0, 11.5} }},
		{"route to unknown destination", func(c *PublicIncidentCard) { c.ApprovedRoutes[0].ToSafeZoneID = "SZ-NONEXISTENT" }},
		{"empty facilities", func(c *PublicIncidentCard) { c.Facilities = nil }},
		{"empty instructions", func(c *PublicIncidentCard) { c.Instructions = nil }},
		{"empty emergency_contacts", func(c *PublicIncidentCard) { c.EmergencyContacts = nil }},
		{"empty policy order", func(c *PublicIncidentCard) { c.AllocationPolicy.Order = nil }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validCardFixture(t, "key-1", priv)
			tc.mutate(c)
			err := ValidateCardStructure(c)
			if err == nil {
				t.Fatalf("ValidateCardStructure succeeded for %s; want ErrMalformedData", tc.name)
			}
			if !errors.Is(err, ErrMalformedData) {
				t.Fatalf("err = %v, want ErrMalformedData", err)
			}
		})
	}
}

func TestCheckRevisionRollback(t *testing.T) {
	// Active revision is 4
	activeRev := 4

	// Older revision 3 must be rejected
	if err := CheckRevisionRollback(3, activeRev); !errors.Is(err, ErrVersionRollback) {
		t.Fatalf("CheckRevisionRollback(3, 4) = %v, want ErrVersionRollback", err)
	}

	// Same revision 4 is allowed (idempotent re-validation)
	if err := CheckRevisionRollback(4, activeRev); err != nil {
		t.Fatalf("CheckRevisionRollback(4, 4) = %v, want nil", err)
	}

	// Newer revision 5 is allowed
	if err := CheckRevisionRollback(5, activeRev); err != nil {
		t.Fatalf("CheckRevisionRollback(5, 4) = %v, want nil", err)
	}
}

func TestRevocationAndTombstones(t *testing.T) {
	rev := &RevocationBlock{
		RevokedPackages: []string{"PKG-REVOKED-01"},
		CancelledRoutes: []string{"RT-WASHOUT-01"},
		SupersededVersions: []SupersededVersion{
			{PackageID: "PKG-OLD", Version: 2},
		},
	}

	if !IsPackageRevoked("PKG-REVOKED-01", 1, rev) {
		t.Errorf("IsPackageRevoked(PKG-REVOKED-01) should be true")
	}
	if !IsPackageRevoked("PKG-OLD", 2, rev) {
		t.Errorf("IsPackageRevoked(PKG-OLD, 2) should be true")
	}
	if IsPackageRevoked("PKG-OLD", 3, rev) {
		t.Errorf("IsPackageRevoked(PKG-OLD, 3) should be false (version 3 not superseded)")
	}
	if IsPackageRevoked("PKG-ACTIVE", 1, rev) {
		t.Errorf("IsPackageRevoked(PKG-ACTIVE) should be false")
	}

	if !IsRouteCancelled("RT-WASHOUT-01", rev) {
		t.Errorf("IsRouteCancelled(RT-WASHOUT-01) should be true")
	}
	if IsRouteCancelled("RT-SAFE-01", rev) {
		t.Errorf("IsRouteCancelled(RT-SAFE-01) should be false")
	}
}
