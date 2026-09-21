package scenarioprep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a tiny test helper.
func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// validCatalog returns a structurally valid catalogue.
func validCatalog() string {
	return `{
  "catalogue_version": 1,
  "historical_events": [
    {"event_id": "EV-1", "state": "EX", "kind": "flood", "occurred": "2018-08", "source_ref": "user:test"}
  ],
  "states": [
    {
      "state_code": "EX",
      "name": "Example",
      "languages": [{"language": "en-IN"}],
      "scenarios": [
        {"scenario_id": "SC-EX-1", "state": "EX", "evidence": "SYNTHETIC_EXERCISE", "exercise_time": "2026-09-12T04:00:00Z"},
        {"scenario_id": "SC-EX-2", "state": "EX", "evidence": "HISTORICAL_EVENT", "historical_event_id": "EV-1", "exercise_time": "2026-09-12T04:00:00Z"}
      ]
    }
  ]
}`
}

// validPackage returns a minimal opkg JSON that opkg.Validate accepts.
// It takes a *testing.T so it can plumb through to the opkg-aware helper.
func validPackage(t *testing.T, jurisdiction string) string {
	t.Helper()
	return canonicalJSONOf(t, minimalPackageValue(jurisdiction))
}

func minimalPackage(jurisdiction string) any {
	// Constructed in canonical_test_helper.go to avoid forcing this file
	// to import the opkg package types (which would cause an import
	// cycle when internal/scenarioprep tests want to call Prepare).
	return minimalPackageValue(jurisdiction)
}

// TestRejectsUnknownScenario covers the cross-check that the index must
// not reference scenario IDs not in the catalogue.
func TestRejectsUnknownScenario(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "catalogue.json", validCatalog())
	idx := `{
  "catalogue": "catalogue.json",
  "scenarios": {
    "SC-NOT-REAL": "pkg.json"
  }
}`
	writeFile(t, dir, "index.json", idx)
	writeFile(t, dir, "pkg.json", validPackage(t, "EX"))

	rep, err := Prepare(dir, filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if rep.Status != StatusInvalid {
		t.Fatalf("status = %s, want INVALID", rep.Status)
	}
	found := false
	for _, f := range rep.Findings {
		if f.Code == CodeUnknownScenarioInIndex {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNKNOWN_SCENARIO_IN_INDEX finding in %+v", rep.Findings)
	}
}

// TestRejectsPathEscape rejects paths that escape the workspace via "..".
func TestRejectsPathEscape(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "catalogue.json", validCatalog())
	idx := `{
  "catalogue": "catalogue.json",
  "scenarios": {
    "SC-EX-1": "../escape.json"
  }
}`
	writeFile(t, dir, "index.json", idx)

	rep, err := Prepare(dir, filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if rep.Status != StatusInvalid {
		t.Fatalf("status = %s, want INVALID", rep.Status)
	}
	found := false
	for _, f := range rep.Findings {
		if f.Code == CodeUnsafeReferencePath && strings.Contains(f.Detail, "escapes workspace") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNSAFE_REFERENCE_PATH escape finding in %+v", rep.Findings)
	}
}

// TestRejectsDuplicateIndexKeys verifies that the strict JSON decoder
// rejects the index with a duplicate key, surfaced as INVALID.
func TestRejectsDuplicateIndexKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "catalogue.json", validCatalog())
	// Duplicate scenario key.
	body := `{"catalogue": "catalogue.json", "scenarios": {"SC-EX-1": "a.json", "SC-EX-1": "b.json"}}`
	writeFile(t, dir, "index.json", body)

	_, err := Prepare(dir, filepath.Join(dir, "index.json"))
	if err == nil {
		t.Fatal("expected Prepare to fail on duplicate index keys")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("err = %v, want substring 'duplicate'", err)
	}
}

// TestBundleRefusesOverwrite verifies that running bundle twice on the
// same output directory is rejected.
func TestBundleRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "ws")
	writeFile(t, dir, "catalogue.json", validCatalog())
	writeFile(t, dir, "pkg.json", validPackage(t, "EX"))
	idx := `{
  "catalogue": "catalogue.json",
  "scenarios": {
    "SC-EX-1": "pkg.json"
  }
}`
	writeFile(t, dir, "index.json", idx)

	rep, err := Prepare(dir, filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	out := filepath.Join(root, "out")
	if err := Bundle(dir, filepath.Join(dir, "index.json"), out, true, rep, "test"); err != nil {
		t.Fatalf("first bundle: %v", err)
	}
	if err := Bundle(dir, filepath.Join(dir, "index.json"), out, true, rep, "test"); err == nil {
		t.Fatal("second bundle must refuse overwrite")
	}
}

// TestBundleDeterministicRepeat verifies that two bundle runs into
// different output directories produce byte-identical manifests and
// identical raw-sha256 entries.
func TestBundleDeterministicRepeat(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "ws")
	writeFile(t, dir, "catalogue.json", validCatalog())
	writeFile(t, dir, "pkg.json", validPackage(t, "EX"))
	idx := `{
  "catalogue": "catalogue.json",
  "scenarios": {
    "SC-EX-1": "pkg.json"
  }
}`
	writeFile(t, dir, "index.json", idx)
	rep, err := Prepare(dir, filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	outA := filepath.Join(root, "out-a")
	outB := filepath.Join(root, "out-b")
	if err := Bundle(dir, filepath.Join(dir, "index.json"), outA, true, rep, "test"); err != nil {
		t.Fatalf("bundle A: %v", err)
	}
	if err := Bundle(dir, filepath.Join(dir, "index.json"), outB, true, rep, "test"); err != nil {
		t.Fatalf("bundle B: %v", err)
	}
	aBytes, _ := os.ReadFile(filepath.Join(outA, "bundle-manifest.json"))
	bBytes, _ := os.ReadFile(filepath.Join(outB, "bundle-manifest.json"))
	if string(aBytes) != string(bBytes) {
		t.Fatalf("manifests differ:\nA=%s\nB=%s", aBytes, bBytes)
	}
}