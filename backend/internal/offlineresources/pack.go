package offlineresources

import (
	"fmt"
	"sort"
)

// DefaultRegionalPackBudgetBytes is the default ceiling for the optional
// regional map pack: 50 MiB per plan/parameters.md.
const DefaultRegionalPackBudgetBytes int64 = 50 * 1024 * 1024

// RegionalPackAudit mirrors offlinepkg.RegionalPackAudit from the P5
// contract §7.4. TotalBytes is the cumulative sum of ByteSize across the
// resource list; ExceedsBudget is true when that sum crosses
// BudgetLimitBytes. AttributionMissing lists the resource IDs whose
// attribution string is empty (a separate, per-descriptor check) so the
// integration agent can attach evidence before live activation.
type RegionalPackAudit struct {
	TotalBytes         int64    `json:"total_bytes"`
	BudgetLimitBytes   int64    `json:"budget_limit_bytes"`
	ExceedsBudget      bool     `json:"exceeds_budget"`
	ResourceCount      int      `json:"resource_count"`
	AttributionMissing []string `json:"attribution_missing,omitempty"`
	// OptionalBytes is the sum of ByteSize over resources with
	// Required=false. The integration agent can use this to surface
	// "download the optional pack?" copy.
	OptionalBytes int64 `json:"optional_bytes"`
	// LicensePending lists the resource IDs whose license metadata is
	// O06-pending. They contribute to the budget audit but are flagged
	// for the integration agent.
	LicensePending []string `json:"license_pending,omitempty"`
	// LicenseDenied lists resources whose declared redistribution is
	// DENIED. They MUST NOT enter an offline pack.
	LicenseDenied []string `json:"license_denied,omitempty"`
}

// AuditRegionalPack sums byte sizes across the supplied resource list and
// checks the total against the supplied budget. It does NOT enforce the
// per-resource structural rules; those run in ValidateDescriptor.
//
// Budget policy:
//   - The critical incident card is excluded from this audit; it has its
//     own ceiling enforced elsewhere (plan/parameters.md, 64 KiB
//     compressed).
//   - The 50 MiB default is for *optional* map packs the device may or
//     may not download; the contract calls this out separately so a
//     regional product claim is never inferred from a synthetic fixture.
//
// AttributionMissing is computed here (rather than inside
// ValidateDescriptor) because the integration agent audits the full pack
// in one pass to surface evidence work.
func AuditRegionalPack(resources []ResourceDescriptor, budgetBytes int64) (*RegionalPackAudit, error) {
	if budgetBytes <= 0 {
		budgetBytes = DefaultRegionalPackBudgetBytes
	}
	a := &RegionalPackAudit{
		BudgetLimitBytes: budgetBytes,
		ResourceCount:    len(resources),
	}
	// Per-asset structural checks run first; the audit does not retry
	// errors but does skip further aggregation if a per-descriptor
	// failure would corrupt the sum (e.g. negative size).
	for _, d := range resources {
		if err := ValidateDescriptor(d); err != nil {
			return nil, fmt.Errorf("%w: %s: %s", ErrPackInvalid, d.ResourceID, err.Error())
		}
	}
	// Cross-descriptor checks: duplicate IDs and conflicting URI metadata
	// are pack-level errors, not per-descriptor errors.
	if dups := newDepGraph(resources).DuplicateIDs(resources); len(dups) > 0 {
		return nil, fmt.Errorf("%w: %s: %s (%v)", ErrPackInvalid, "<pack>",
			ReasonDuplicateResourceID, dups)
	}
	if confs := newDepGraph(resources).Conflicts(resources); len(confs) > 0 {
		first := confs[0]
		return nil, fmt.Errorf("%w: %s: %s (uri=%s ids=%s,%s)",
			ErrPackInvalid, "<pack>", ReasonConflictingURI,
			first.URI, first.First.ResourceID, first.Second.ResourceID)
	}
	for _, d := range resources {
		a.TotalBytes += d.ByteSize
		if !d.Required {
			a.OptionalBytes += d.ByteSize
		}
		if d.Attribution == "" {
			a.AttributionMissing = append(a.AttributionMissing, d.ResourceID)
		}
		switch LicenseStatus(d) {
		case LicensePending:
			a.LicensePending = append(a.LicensePending, d.ResourceID)
		case LicenseDenied:
			a.LicenseDenied = append(a.LicenseDenied, d.ResourceID)
		}
	}
	a.ExceedsBudget = a.TotalBytes > budgetBytes
	sort.Strings(a.AttributionMissing)
	sort.Strings(a.LicensePending)
	sort.Strings(a.LicenseDenied)
	return a, nil
}
