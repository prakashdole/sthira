package scenarioprep

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"sthira/backend/internal/httpjson"
)

// InputIndex is the local mapping between scenario IDs in the catalogue and
// user-supplied operational-package JSON paths. It is intentionally simple:
// the tool is a CLI for preparing data, not a database or model.
type InputIndex struct {
	// Catalogue is a workspace-relative path to the catalogue JSON.
	Catalogue string `json:"catalogue"`
	// Scenarios maps scenario_id -> workspace-relative path to the package
	// JSON. A missing entry means the scenario has no supplied package yet.
	Scenarios map[string]string `json:"scenarios"`
	// ExerciseClock is an optional RFC3339 instant the user wants to anchor
	// the preparation. It is informational only; package validity windows
	// are evaluated against their declared effective/expires values.
	ExerciseClock string `json:"exercise_clock,omitempty"`
	// Notes is free-form user notes; never interpreted as evidence.
	Notes string `json:"notes,omitempty"`
}

// indexLimits are the parsing limits applied to the index and to every
// referenced package. Both are bounded; a runaway attachment is rejected.
var indexLimits = httpjson.Limits{MaxBytes: 1 << 20, MaxDepth: 32}

// packageLimits are tighter than the public API default to keep bundle
// payloads small; operational packages rarely need megabytes.
var packageLimits = httpjson.Limits{MaxBytes: 4 << 20, MaxDepth: 32}

// LoadIndex reads and strictly decodes an input index from disk.
func LoadIndex(path string) (*InputIndex, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, wrap("load index", fmt.Errorf("%w: %v", ErrIO, err))
	}
	var idx InputIndex
	if err := httpjson.DecodeStrict(body, &idx, indexLimits); err != nil {
		return nil, wrap("parse index", err)
	}
	return &idx, nil
}

// ResolvedReference describes one scenario/package mapping after path and
// existence checks. Path is absolute (still inside workspace). RawSHA256
// is the file's content hash for the bundle.
type ResolvedReference struct {
	ScenarioID  string
	PackagePath string // absolute, inside workspace, no symlink traversal
	RawSHA256   string // hex
}

// resolveReferences returns the references declared in the index. It refuses
// paths escaping the workspace, symlinks pointing outside, and any missing
// file. It also surfaces duplicate scenario IDs as a structured finding.
func resolveReferences(workspace string, idx *InputIndex) (refs []ResolvedReference, findings []Finding) {
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		findings = append(findings, Finding{
			Code: CodeUnsafeReferencePath, Severity: SeverityError, Scope: "index",
			Detail: "could not resolve workspace absolute path: " + err.Error(),
		})
		return nil, findings
	}

	ids := make([]string, 0, len(idx.Scenarios))
	for id := range idx.Scenarios {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	seen := map[string]string{}
	for _, id := range ids {
		rel := idx.Scenarios[id]
		if rel == "" {
			continue
		}
		if prior, dup := seen[id]; dup {
			findings = append(findings, Finding{
				Code: CodeDuplicateScenarioInIndex, Severity: SeverityError,
				Scope: "scenario:" + id,
				Detail: fmt.Sprintf("scenario %q is mapped twice: %q and %q", id, prior, rel),
			})
			continue
		}
		seen[id] = rel

		abs, err := safeResolve(absWorkspace, rel)
		if err != nil {
			findings = append(findings, Finding{
				Code: CodeUnsafeReferencePath, Severity: SeverityError,
				Scope: "scenario:" + id,
				Detail: err.Error(),
			})
			continue
		}
		info, err := os.Lstat(abs)
		if err != nil {
			findings = append(findings, Finding{
				Code: CodeMissingPackageFile, Severity: SeverityError,
				Scope: "scenario:" + id,
				Detail: fmt.Sprintf("package file %q: %v", rel, err),
			})
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(abs)
			if err != nil || !pathInside(target, absWorkspace) {
				findings = append(findings, Finding{
					Code: CodeUnsafeSymlink, Severity: SeverityError,
					Scope: "scenario:" + id,
					Detail: fmt.Sprintf("package file %q is an unsafe symlink", rel),
				})
				continue
			}
		}
		body, err := os.ReadFile(abs)
		if err != nil {
			findings = append(findings, Finding{
				Code: CodeMissingPackageFile, Severity: SeverityError,
				Scope: "scenario:" + id,
				Detail: fmt.Sprintf("package file %q: %v", rel, err),
			})
			continue
		}
		refs = append(refs, ResolvedReference{
			ScenarioID:  id,
			PackagePath: abs,
			RawSHA256:   sha256Hex(body),
		})
	}
	return refs, findings
}