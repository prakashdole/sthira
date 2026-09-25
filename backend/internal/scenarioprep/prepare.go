// Package scenarioprep is the data-preparation boundary for the Sthira
// scenario preparation CLI. It reads user-supplied catalogue + per-scenario
// operational-package inputs from the local filesystem, runs the existing
// catalogue/opkg validators, cross-checks the scenario/package mappings, and
// produces deterministic, reviewable reports and handoff bundles.
//
// Trust posture: the tool does not establish operational authority. It
// prepares data for review. Signature verification is reported as
// UNVERIFIED because the tool has no trusted verifier configured; calling a
// missing verifier "verified" would be a dummy trusted path. Catalogue
// structural validity and launch acceptance are reported separately per the
// existing catalogue package. Historical-event provenance does not
// authenticate geometry: a real past event may be linked, but its geometry
// stays synthetic unless separately evidenced. The handoff bundle is not a
// P5 signed offline-client package and never claims that compatibility.
//
// The package never imports network, database, model, or signing primitives;
// it is intentionally offline-only.
package scenarioprep

import (
	"errors"
	"fmt"
	"sort"
)

// Status is the validation outcome of a preparation run. It separates
// structural INVALID from valid-but-INCOMPLETE from READY.
type Status string

const (
	// StatusInvalid means the inputs failed structural validation and the
	// tool cannot produce a coherent report or bundle.
	StatusInvalid Status = "INVALID"
	// StatusIncomplete means the inputs validated structurally but missing
	// data (catalogue gaps, missing references, missing zones/routes/policy)
	// prevents acceptance readiness. A draft bundle may still be produced
	// when explicitly requested.
	StatusIncomplete Status = "INCOMPLETE"
	// StatusReady means the inputs validated structurally, every mapped
	// package validates, and no acceptance gap remains.
	StatusReady Status = "READY"
)

// Severity describes a per-input finding. SeverityError blocks readiness
// when the issue is structural; SeverityWarning records missing data and
// gaps; SeverityInfo records a present-but-noted item.
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// Code is a stable, machine-readable diagnostic code for a finding.
type Code string

const (
	CodeMissingCatalogue         Code = "MISSING_CATALOGUE"
	CodeMissingIndex             Code = "MISSING_INDEX"
	CodeMalformedIndex           Code = "MALFORMED_INDEX"
	CodeUnknownScenarioInIndex   Code = "UNKNOWN_SCENARIO_IN_INDEX"
	CodeDuplicateScenarioInIndex Code = "DUPLICATE_SCENARIO_IN_INDEX"
	CodeMissingPackageFile       Code = "MISSING_PACKAGE_FILE"
	CodeMalformedPackageFile     Code = "MALFORMED_PACKAGE_FILE"
	CodePackageInvalid           Code = "PACKAGE_INVALID"
	CodeJurisdictionMismatch     Code = "JURISDICTION_MISMATCH"
	CodeScenarioIDConflict       Code = "SCENARIO_ID_CONFLICT"
	CodePackageVersionConflict   Code = "PACKAGE_VERSION_CONFLICT"
	CodeMissingZoneData          Code = "MISSING_ZONE_DATA"
	CodeMissingRouteData         Code = "MISSING_ROUTE_DATA"
	CodeMissingPolicyData        Code = "MISSING_POLICY_DATA"
	CodeMissingHistoricalEvent   Code = "MISSING_HISTORICAL_EVENT"
	CodeSyntheticWithEventRef    Code = "SYNTHETIC_WITH_EVENT_REF"
	CodeUnsafeReferencePath      Code = "UNSAFE_REFERENCE_PATH"
	CodeUnsafeSymlink            Code = "UNSAFE_SYMLINK"
	CodeOutputInsideInput        Code = "OUTPUT_INSIDE_INPUT"
	CodeSignatureUnverified      Code = "SIGNATURE_UNVERIFIED"
	CodeCatalogueGap             Code = "CATALOGUE_GAP"
	CodeLaunchGap                Code = "LAUNCH_GAP"
	CodeInconsistentSupplied     Code = "INCONSISTENT_SUPPLIED_EVIDENCE"
)

// Finding is one diagnostic produced by a preparation run. Code is stable;
// Detail is human-readable and may include raw error text from validators.
type Finding struct {
	Code     Code     `json:"code"`
	Severity Severity `json:"severity"`
	Scope    string   `json:"scope,omitempty"` // e.g. "scenario:SC-KL-2018-FLOOD" or "catalogue"
	Detail   string   `json:"detail"`
}

// Report is the machine-readable outcome of a prepare run. It is a data-
// preparation report, not a completion certificate: StatusReady means the
// inputs validate against existing schemas and the cross-checks pass. It
// does not authorize publication.
type Report struct {
	Status            Status        `json:"status"`
	CataloguePath     string        `json:"catalogue_path,omitempty"`
	IndexPath         string        `json:"index_path,omitempty"`
	ExerciseClock     string        `json:"exercise_clock,omitempty"`
	StateCount        int           `json:"state_count"`
	ScenarioCount     int           `json:"scenario_count"`
	HistoricalCount   int           `json:"historical_count"`
	States            []StateStatus `json:"states"`
	MissingReferences []string      `json:"missing_references,omitempty"`
	SignatureStatus   string        `json:"signature_status"`
	UnresolvedGates   []string      `json:"unresolved_gates,omitempty"`
	Findings          []Finding     `json:"findings"`
}

// StateStatus describes one state in the catalogue: whether it is supplied,
// its declared languages, scenario counts, and any per-state gaps.
type StateStatus struct {
	StateCode          string   `json:"state_code"`
	Name               string   `json:"name,omitempty"`
	Scenarios          []string `json:"scenarios,omitempty"`
	LanguagesClaimed   []string `json:"languages_claimed,omitempty"`
	LanguagesEvidenced []string `json:"languages_evidenced,omitempty"`
	LanguagesUnknown   []string `json:"languages_unknown,omitempty"`
}

// LanguageStatus classifies a language per the existing catalogue and opkg
// evidence: claimed in catalogue languages list, evidenced via opkg
// instruction_assets for any mapped package, or unknown when declared but
// no supplied package supplies a matching instruction asset.
type LanguageStatus string

const (
	LanguageClaimed   LanguageStatus = "claimed"
	LanguageEvidenced LanguageStatus = "evidenced"
	LanguageUnknown   LanguageStatus = "unknown"
)

// classifyLanguages partitions the state-declared languages into claimed
// (declared only), evidenced (a mapped package supplies an instruction in
// that language), and unknown (declared but not evidenced; this is the
// honest gap, not an approval claim).
func classifyLanguages(declared []string, suppliedLanguages map[string]bool) (claimed, evidenced, unknown []string) {
	declaredSet := map[string]bool{}
	for _, l := range declared {
		declaredSet[l] = true
	}
	for _, l := range declared {
		if suppliedLanguages[l] {
			evidenced = append(evidenced, l)
		} else {
			unknown = append(unknown, l)
		}
	}
	for lang := range suppliedLanguages {
		if lang == "" {
			continue
		}
		if !declaredSet[lang] {
			claimed = append(claimed, lang)
		}
	}
	sort.Strings(evidenced)
	sort.Strings(unknown)
	sort.Strings(claimed)
	return claimed, evidenced, unknown
}

// ExitCode translates a status into a documented exit code. Codes are
// stable for automation; see HANDOFF.md.
func ExitCode(status Status) int {
	switch status {
	case StatusReady:
		return 0
	case StatusIncomplete:
		return 3
	case StatusInvalid:
		return 2
	}
	return 1
}

// Sentinel errors for IO/permission failures so callers can map to exit 4.
var (
	ErrIO            = errors.New("scenario-prep IO failure")
	ErrUsage         = errors.New("scenario-prep usage error")
	ErrUnsafeLayout  = errors.New("scenario-prep unsafe workspace layout")
	ErrUnsafeSymlink = errors.New("scenario-prep unsafe symlink")
	ErrBoundedRead   = errors.New("scenario-prep file exceeds maximum allowed size")
)

// PrintableError formats an error with stable code so logs are greppable.
type PrintableError struct {
	Op  string
	Err error
}

func (e *PrintableError) Error() string {
	return fmt.Sprintf("scenario-prep %s: %v", e.Op, e.Err)
}

func (e *PrintableError) Unwrap() error { return e.Err }

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return &PrintableError{Op: op, Err: err}
}
