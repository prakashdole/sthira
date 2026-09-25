package offlineresources

// Validator implements the ResourceValidator interface declared in the
// P5 contract §7.4. It is the public seam between Agent 4 and the
// integration agent; everything else in this package is unexported.
//
// The implementation is stateless and safe for concurrent use. Callers
// pass the slice of descriptors (and, where relevant, the style JSON)
// each time; the validator returns either a typed result or a
// *ValidatorError wrapped in ErrPackInvalid / ErrStyleInvalid.
type Validator struct {
	// budgetBytes is the regional pack ceiling. Zero means use
	// DefaultRegionalPackBudgetBytes (50 MiB). It is set once at
	// construction and never mutated; no shared mutable global.
	budgetBytes int64
}

// ResourceValidator is the seam declared by the P5 contract §7.4. It is
// re-exported here because the contract spec lists it as the return type
// of NewValidator; the concrete *Validator satisfies it.
type ResourceValidator interface {
	ValidateMapStyle(styleJSON []byte, availableResources []ResourceDescriptor) (*StyleValidationResult, error)
	AuditRegionalPack(resources []ResourceDescriptor) (*RegionalPackAudit, error)
}

// NewValidator is the contract §7.4 factory. It returns a Validator
// configured with the default regional-pack budget (50 MiB). The returned
// value satisfies ResourceValidator.
//
// The contract's signature is `func NewValidator() ResourceValidator`; the
// concrete return type is *Validator so callers that need the budget knob
// can type-assert or use NewValidatorWithBudget below.
func NewValidator() ResourceValidator {
	return NewValidatorWithBudget(0)
}

// NewValidatorWithBudget returns a Validator with a custom budget; the
// zero value uses DefaultRegionalPackBudgetBytes.
func NewValidatorWithBudget(budgetBytes int64) *Validator {
	if budgetBytes == 0 {
		budgetBytes = DefaultRegionalPackBudgetBytes
	}
	return &Validator{budgetBytes: budgetBytes}
}

// ValidateMapStyle delegates to the package-level helper. The contract
// signature does not carry an "optional" flag, so by default a missing
// required reference flips Valid to false. Tests and the integration agent
// that need the optional semantics can call the package-level
// ValidateMapStyle directly with optional=true.
func (v *Validator) ValidateMapStyle(styleJSON []byte, availableResources []ResourceDescriptor) (*StyleValidationResult, error) {
	return ValidateMapStyle(styleJSON, availableResources, false)
}

// AuditRegionalPack delegates to the package-level helper.
func (v *Validator) AuditRegionalPack(resources []ResourceDescriptor) (*RegionalPackAudit, error) {
	return AuditRegionalPack(resources, v.budgetBytes)
}

// WithBudget returns a shallow copy with a different budget. Used by tests
// when measuring against a tighter target without mutating the parent
// validator.
func (v *Validator) WithBudget(b int64) *Validator {
	if b == 0 {
		b = DefaultRegionalPackBudgetBytes
	}
	cp := *v
	cp.budgetBytes = b
	return &cp
}

// Budget returns the current budget in bytes.
func (v *Validator) Budget() int64 {
	return v.budgetBytes
}
