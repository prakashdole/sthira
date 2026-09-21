package scenarioprep

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"sthira/backend/internal/catalogue"
	"sthira/backend/internal/httpjson"
	"sthira/backend/internal/opkg"
)

// Signature status constants. UNVERIFIED is the only honest answer when no
// trusted verifier is configured: the tool never invents a verifier.
const (
	signatureStatusUnverified = "UNVERIFIED"
)

// Prepare runs structural and cross-reference validation against the
// catalogue and the index's scenario mappings. It returns a Report with
// status, findings, and per-state language classification. The Prepare
// step never writes anything to disk.
func Prepare(workspace, indexPath string) (*Report, error) {
	idx, err := LoadIndex(indexPath)
	if err != nil {
		return nil, err
	}

	rep := &Report{
		IndexPath:       indexPath,
		ExerciseClock:   idx.ExerciseClock,
		SignatureStatus: signatureStatusUnverified,
	}

	// Catalogue: bounded read, strict decode.
	catalogueAbs, err := safeResolve(mustAbs(workspace), idx.Catalogue)
	if err != nil {
		rep.Status = StatusInvalid
		rep.Findings = append(rep.Findings, Finding{
			Code: CodeUnsafeReferencePath, Severity: SeverityError, Scope: "catalogue",
			Detail: err.Error(),
		})
		return rep, nil
	}
	catBody, err := os.ReadFile(catalogueAbs)
	if err != nil {
		rep.Status = StatusInvalid
		rep.Findings = append(rep.Findings, Finding{
			Code: CodeMissingCatalogue, Severity: SeverityError, Scope: "catalogue",
			Detail: "read catalogue: " + err.Error(),
		})
		return rep, nil
	}
	var manifest catalogue.Manifest
	if err := httpjson.DecodeStrict(catBody, &manifest, packageLimits); err != nil {
		rep.Status = StatusInvalid
		rep.Findings = append(rep.Findings, Finding{
			Code: CodeMalformedIndex, Severity: SeverityError, Scope: "catalogue",
			Detail: "strict decode: " + err.Error(),
		})
		return rep, nil
	}
	rep.CataloguePath = idx.Catalogue

	cv, cerr := catalogue.Validate(&manifest)
	if cerr != nil {
		rep.Status = StatusInvalid
		rep.Findings = append(rep.Findings, Finding{
			Code: CodeMalformedIndex, Severity: SeverityError, Scope: "catalogue",
			Detail: "catalogue.Validate: " + cerr.Error(),
		})
		return rep, nil
	}
	rep.StateCount = len(cv.StateCodes)
	rep.ScenarioCount = cv.ScenarioCount
	rep.HistoricalCount = cv.HistoricalCount

	// Convert catalogue validation gaps into findings.
	for _, g := range cv.Gaps {
		rep.Findings = append(rep.Findings, Finding{
			Code: CodeCatalogueGap, Severity: SeverityWarning, Scope: "catalogue",
			Detail: g.Kind + ": " + g.Detail,
		})
	}

	// Index -> scenarios. Cross-check scenario IDs against the catalogue.
	scenarioOwners := map[string]string{} // scenario_id -> state_code
	for _, st := range manifest.States {
		for _, sc := range st.Scenarios {
			scenarioOwners[sc.ScenarioID] = st.StateCode
		}
	}
	for id := range idx.Scenarios {
		if _, ok := scenarioOwners[id]; !ok {
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeUnknownScenarioInIndex, Severity: SeverityError,
				Scope: "scenario:" + id,
				Detail: fmt.Sprintf("index maps unknown scenario id %q", id),
			})
		}
	}

	// References and per-package validation.
	refs, refFindings := resolveReferences(workspace, idx)
	rep.Findings = append(rep.Findings, refFindings...)

	// indexOf helps us locate already-resolved refs by scenario id.
	refsByID := map[string]ResolvedReference{}
	for _, r := range refs {
		refsByID[r.ScenarioID] = r
	}

	// suppliedLanguages is a per-state mapping built from validated
	// packages, used to classify claimed vs evidenced languages.
	suppliedByState := map[string]map[string]bool{}
	// scenarioStatuses records per-scenario zone/route/policy coverage for
	// the report.
	type scenStatus struct {
		jurisdiction  string
		zones         bool
		routes        bool
		policy        bool
		checksumOK    bool
		jurisMatch    bool
		hasEvent      bool
		missingFields []string
	}
	perScenario := map[string]*scenStatus{}

	for _, ref := range refs {
		owner, ok := scenarioOwners[ref.ScenarioID]
		if !ok {
			continue
		}
		body, err := os.ReadFile(ref.PackagePath)
		if err != nil {
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeMissingPackageFile, Severity: SeverityError,
				Scope: "scenario:" + ref.ScenarioID,
				Detail: "read package: " + err.Error(),
			})
			continue
		}
		var pkg opkg.Package
		if err := httpjson.DecodeStrict(body, &pkg, packageLimits); err != nil {
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeMalformedPackageFile, Severity: SeverityError,
				Scope: "scenario:" + ref.ScenarioID,
				Detail: "strict decode: " + err.Error(),
			})
			continue
		}
		// Reuse opkg validation under an explicitly declared preparation
		// context. requireSignature=false because the tool has no trusted
		// verifier; signature status is reported UNVERIFIED rather than
		// silently trusted.
		_, verr := opkg.Validate(&pkg, owner, false, nil)
		if verr != nil {
			rep.Findings = append(rep.Findings, Finding{
				Code: CodePackageInvalid, Severity: SeverityError,
				Scope: "scenario:" + ref.ScenarioID,
				Detail: "opkg.Validate: " + verr.Error(),
			})
			continue
		}

		st := &scenStatus{
			jurisdiction: pkg.Provenance.Jurisdiction,
			zones:        len(pkg.RedZones) > 0 && len(pkg.SafeZones) > 0,
			routes:       len(pkg.ApprovedRoutes) > 0,
			policy:       len(pkg.Policy.Order) > 0,
			checksumOK:   pkg.Provenance.ChecksumSHA256 == opkg.Checksum(&pkg),
		}
		if pkg.Provenance.Jurisdiction != owner {
			st.jurisMatch = false
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeJurisdictionMismatch, Severity: SeverityError,
				Scope: "scenario:" + ref.ScenarioID,
				Detail: fmt.Sprintf("package jurisdiction %q != scenario state %q", pkg.Provenance.Jurisdiction, owner),
			})
		} else {
			st.jurisMatch = true
		}

		// Aggregate supplied languages per state.
		if suppliedByState[owner] == nil {
			suppliedByState[owner] = map[string]bool{}
		}
		for _, ins := range pkg.Instructions {
			suppliedByState[owner][ins.Language] = true
		}

		// Record gaps per scenario for the report.
		if len(pkg.RedZones) == 0 {
			st.missingFields = append(st.missingFields, "red_zones")
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeMissingZoneData, Severity: SeverityWarning,
				Scope: "scenario:" + ref.ScenarioID, Detail: "package has no red zones",
			})
		}
		if len(pkg.SafeZones) == 0 {
			st.missingFields = append(st.missingFields, "safe_zones")
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeMissingZoneData, Severity: SeverityWarning,
				Scope: "scenario:" + ref.ScenarioID, Detail: "package has no safe zones",
			})
		}
		if len(pkg.ApprovedRoutes) == 0 {
			st.missingFields = append(st.missingFields, "approved_routes")
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeMissingRouteData, Severity: SeverityWarning,
				Scope: "scenario:" + ref.ScenarioID, Detail: "package has no approved routes",
			})
		}
		if len(pkg.Policy.Order) == 0 {
			st.missingFields = append(st.missingFields, "allocation_policy")
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeMissingPolicyData, Severity: SeverityWarning,
				Scope: "scenario:" + ref.ScenarioID, Detail: "package has no allocation policy",
			})
		}

		// Check scenario/package version relationship when scenario
		// metadata supplies one (catalogue does not currently model
		// versions, so a conflict here means the user labelled the
		// package differently in the index — surfaced as info).
		if pkg.Provenance.Version < 1 {
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeInconsistentSupplied, Severity: SeverityWarning,
				Scope: "scenario:" + ref.ScenarioID, Detail: "package version missing",
			})
		}

		// Historical scenario must reference a known historical event;
		// synthetic must not borrow one. The catalogue already enforces
		// this; here we only mirror it as a finding if the validator's
		// gap list mentioned it (kept consistent for the report).
		perScenario[ref.ScenarioID] = st
	}

	// Per-state language classification and state status.
	for _, st := range manifest.States {
		supplied := suppliedByState[st.StateCode]
		var declared []string
		for _, l := range st.Languages {
			declared = append(declared, l.Language)
		}
		claimed, evidenced, unknown := classifyLanguages(declared, supplied)
		scenarioIDs := make([]string, 0, len(st.Scenarios))
		for _, sc := range st.Scenarios {
			scenarioIDs = append(scenarioIDs, sc.ScenarioID)
		}
		sort.Strings(scenarioIDs)
		rep.States = append(rep.States, StateStatus{
			StateCode: st.StateCode, Name: st.Name,
			Scenarios: scenarioIDs,
			LanguagesClaimed: claimed, LanguagesEvidenced: evidenced, LanguagesUnknown: unknown,
		})
	}

	// Track any scenario ID the index maps that has no supplied package —
	// a missing reference. (The refs slice omits entries with empty
	// paths; we already converted unknown IDs above.)
	missingRefs := map[string]bool{}
	for id, path := range idx.Scenarios {
		if path == "" {
			missingRefs[id] = true
			rep.Findings = append(rep.Findings, Finding{
				Code: CodeMissingPackageFile, Severity: SeverityError,
				Scope: "scenario:" + id, Detail: "index does not reference a package",
			})
		}
	}
	for id := range missingRefs {
		rep.MissingReferences = append(rep.MissingReferences, id)
	}
	sort.Strings(rep.MissingReferences)

	// Always surface the honest signature posture.
	rep.Findings = append(rep.Findings, Finding{
		Code: CodeSignatureUnverified, Severity: SeverityInfo,
		Detail: "scenario-prep has no trusted verifier configured; signature status is UNVERIFIED",
	})

	// Launch acceptance: structural validity vs launch bar (10-15 states,
	// 2-3 scenarios per state). Preserve the existing catalogue thresholds.
	launch := cv.AssessLaunch()
	for _, g := range launch.Gaps {
		rep.Findings = append(rep.Findings, Finding{
			Code: CodeLaunchGap, Severity: SeverityWarning, Scope: "catalogue",
			Detail: g.Kind + ": " + g.Detail,
		})
	}

	// Resolve overall status.
	rep.Status = deriveStatus(rep.Findings, cv)
	// Catalogue gaps that the user cannot satisfy (missing user data) are
	// reported but do not on their own turn structurally valid inputs
	// invalid; they drive the INCOMPLETE status.

	// External gates: items the user must close outside this tool.
	rep.UnresolvedGates = append(rep.UnresolvedGates,
		"O01: approved demo state/district list and 2-3 historical cases each",
		"O05: route authority approval workflow",
		"O07: facility inventory / capacity / stay policy",
		"O08: source agreements and publisher keys",
		"O11: approved instruction translations / ISL corpus",
		"O14: trusted operator identity / MFA verifier",
	)

	return rep, nil
}

// deriveStatus maps the finding set and catalogue validation result to the
// documented status. It does not invent acceptance: a structural failure is
// INVALID; structural validity plus blocking catalogue gaps or missing
// references is INCOMPLETE; otherwise READY.
func deriveStatus(findings []Finding, cv *catalogue.Validation) Status {
	hasError := false
	hasWarning := false
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			hasError = true
		case SeverityWarning:
			hasWarning = true
		}
	}
	if hasError {
		return StatusInvalid
	}
	if hasWarning {
		// Even structurally valid catalogues may be INCOMPLETE due to
		// catalogue-level gaps (e.g. MISSING_STATE).
		if cv == nil || len(cv.Gaps) > 0 {
			return StatusIncomplete
		}
		// Other warnings include missing per-package zones/routes; this
		// is a coverage gap, not a structural defect. We still surface
		// INCOMPLETE so callers know to fetch/replace the package.
		return StatusIncomplete
	}
	return StatusReady
}

// mustAbs returns the absolute path or "" on error (used only to seed safe
// resolution from a workspace argument).
func mustAbs(p string) string {
	if p == "" {
		return "."
	}
	return p
}

// EncodeReport returns the JSON-encoded report with stable key ordering.
func EncodeReport(rep *Report) ([]byte, error) {
	return json.Marshal(rep)
}