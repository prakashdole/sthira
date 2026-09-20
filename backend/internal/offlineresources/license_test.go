package offlineresources

import "testing"

func TestValidateLicense_EmptyLicenseStructIsNotAStructuralError(t *testing.T) {
	d := validDescriptor()
	d.License = &LicenseInfo{Redistribution: RedistUndeclared}
	if err := ValidateDescriptor(d); err != nil {
		t.Fatalf("empty redistribution must surface as O06 pending, not a structural error: %v", err)
	}
}

func TestValidateLicense_AllowedWithoutIdentifierFails(t *testing.T) {
	d := validDescriptor()
	d.License = &LicenseInfo{Redistribution: RedistAllowed}
	err := ValidateDescriptor(d)
	if err == nil {
		t.Fatalf("REDISTRIBUTION_ALLOWED without SPDX or name must be rejected")
	}
}

func TestValidateLicense_AllowedWithSPDX(t *testing.T) {
	d := validDescriptor()
	d.License = &LicenseInfo{
		Redistribution:    RedistAllowed,
		SPDXIdentifier:    "ODbL-1.0",
		DeclaredOfflineOK: true,
		EvidenceReference: "O06-pending-evidence",
	}
	if err := ValidateDescriptor(d); err != nil {
		t.Fatalf("declared redistribution with SPDX must validate: %v", err)
	}
}

func TestValidateLicense_UnknownRedistributionValueRejected(t *testing.T) {
	d := validDescriptor()
	d.License = &LicenseInfo{
		Redistribution: RedistributionPermission("REDIST_FUZZY"),
		SPDXIdentifier: "ODbL-1.0",
	}
	err := ValidateDescriptor(d)
	if err == nil {
		t.Fatalf("unknown redistribution value must be rejected")
	}
}

func TestValidateLicense_DeniedIsStructuralFailure(t *testing.T) {
	d := validDescriptor()
	d.License = &LicenseInfo{Redistribution: RedistDenied}
	if err := ValidateDescriptor(d); err == nil {
		t.Fatalf("REDISTRIBUTION_DENIED must reject the descriptor outright")
	}
}

func TestLicenseStatus(t *testing.T) {
	cases := []struct {
		name string
		d    ResourceDescriptor
		want LicenseVerificationStatus
	}{
		{
			name: "no license field",
			d:    validDescriptor(),
			want: LicenseNotDeclared,
		},
		{
			name: "redistribution undeclared",
			d: func() ResourceDescriptor {
				x := validDescriptor()
				x.License = &LicenseInfo{}
				return x
			}(),
			want: LicensePending,
		},
		{
			name: "allowed without identifier",
			d: func() ResourceDescriptor {
				x := validDescriptor()
				x.License = &LicenseInfo{Redistribution: RedistAllowed}
				return x
			}(),
			want: LicensePending,
		},
		{
			name: "allowed with SPDX",
			d: func() ResourceDescriptor {
				x := validDescriptor()
				x.License = &LicenseInfo{Redistribution: RedistAllowed, SPDXIdentifier: "ODbL-1.0"}
				return x
			}(),
			want: LicenseDeclared,
		},
		{
			name: "denied",
			d: func() ResourceDescriptor {
				x := validDescriptor()
				x.License = &LicenseInfo{Redistribution: RedistDenied, SPDXIdentifier: "Proprietary"}
				return x
			}(),
			want: LicenseDenied,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LicenseStatus(c.d); got != c.want {
				t.Fatalf("status = %q, want %q", got, c.want)
			}
		})
	}
}
