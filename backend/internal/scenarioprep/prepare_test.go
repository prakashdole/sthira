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

// TestParentSymlinkEscapePackage verifies that referencing a package through a
// parent directory symlink pointing outside the workspace is rejected.
func TestParentSymlinkEscapePackage(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, outside, "target.json", validPackage(t, "EX"))

	ws := t.TempDir()
	writeFile(t, ws, "catalogue.json", validCatalog())
	if err := os.Symlink(outside, filepath.Join(ws, "sym_parent")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}
	idx := `{
  "catalogue": "catalogue.json",
  "scenarios": {
    "SC-EX-1": "sym_parent/target.json"
  }
}`
	writeFile(t, ws, "index.json", idx)

	rep, err := Prepare(ws, filepath.Join(ws, "index.json"))
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	if rep.Status != StatusInvalid {
		t.Fatalf("status = %s, want INVALID for parent symlink escape", rep.Status)
	}
	found := false
	for _, f := range rep.Findings {
		if f.Code == CodeUnsafeSymlink {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNSAFE_SYMLINK finding in %+v", rep.Findings)
	}
}

// TestLeafSymlinkEscapePackage verifies that referencing a package that is a
// leaf symlink pointing outside the workspace is rejected.
func TestLeafSymlinkEscapePackage(t *testing.T) {
	outside := t.TempDir()
	target := writeFile(t, outside, "target.json", validPackage(t, "EX"))

	ws := t.TempDir()
	writeFile(t, ws, "catalogue.json", validCatalog())
	if err := os.Symlink(target, filepath.Join(ws, "sym_pkg.json")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}
	idx := `{
  "catalogue": "catalogue.json",
  "scenarios": {
    "SC-EX-1": "sym_pkg.json"
  }
}`
	writeFile(t, ws, "index.json", idx)

	rep, err := Prepare(ws, filepath.Join(ws, "index.json"))
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	if rep.Status != StatusInvalid {
		t.Fatalf("status = %s, want INVALID for leaf symlink escape", rep.Status)
	}
	found := false
	for _, f := range rep.Findings {
		if f.Code == CodeUnsafeSymlink {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNSAFE_SYMLINK finding in %+v", rep.Findings)
	}
}

// TestParentSymlinkEscapeCatalogue verifies that referencing a catalogue
// through a parent directory symlink pointing outside the workspace is rejected.
func TestParentSymlinkEscapeCatalogue(t *testing.T) {
	outside := t.TempDir()
	writeFile(t, outside, "cat.json", validCatalog())

	ws := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(ws, "sym_cat_dir")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}
	writeFile(t, ws, "pkg.json", validPackage(t, "EX"))
	idx := `{
  "catalogue": "sym_cat_dir/cat.json",
  "scenarios": {
    "SC-EX-1": "pkg.json"
  }
}`
	writeFile(t, ws, "index.json", idx)

	rep, err := Prepare(ws, filepath.Join(ws, "index.json"))
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	if rep.Status != StatusInvalid {
		t.Fatalf("status = %s, want INVALID for catalogue parent symlink escape", rep.Status)
	}
	found := false
	for _, f := range rep.Findings {
		if f.Code == CodeUnsafeSymlink {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UNSAFE_SYMLINK finding in %+v", rep.Findings)
	}
}

// TestInternalSymlinkAccepted verifies that an internal symlink pointing within
// the workspace is permitted.
func TestInternalSymlinkAccepted(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "catalogue.json", validCatalog())
	writeFile(t, ws, "real_pkg.json", validPackage(t, "EX"))
	if err := os.Symlink("real_pkg.json", filepath.Join(ws, "link_pkg.json")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}
	idx := `{
  "catalogue": "catalogue.json",
  "scenarios": {
    "SC-EX-1": "link_pkg.json"
  }
}`
	writeFile(t, ws, "index.json", idx)

	rep, err := Prepare(ws, filepath.Join(ws, "index.json"))
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	for _, f := range rep.Findings {
		if f.Code == CodeUnsafeSymlink {
			t.Errorf("unexpected UNSAFE_SYMLINK for internal symlink: %s", f.Detail)
		}
	}
}

// TestRejectsOversizedIndex verifies that an index exceeding indexLimits.MaxBytes
// is rejected before unbounded memory allocation.
func TestRejectsOversizedIndex(t *testing.T) {
	ws := t.TempDir()
	idxPath := filepath.Join(ws, "huge_index.json")
	// indexLimits is 1 MB. Write 1 MB + 100 bytes.
	hugeData := make([]byte, indexLimits.MaxBytes+100)
	for i := range hugeData {
		hugeData[i] = ' '
	}
	if err := os.WriteFile(idxPath, hugeData, 0o644); err != nil {
		t.Fatalf("write huge index: %v", err)
	}

	_, err := LoadIndex(idxPath)
	if err == nil {
		t.Fatal("expected LoadIndex to reject oversized index file")
	}
	if !strings.Contains(err.Error(), "exceeds size limit") && !strings.Contains(err.Error(), "exceeds limit") {
		t.Errorf("err = %v, expected limit error", err)
	}
}

// TestRejectsOversizedPackage verifies that a package exceeding packageLimits.MaxBytes
// is rejected before unbounded decode.
func TestRejectsOversizedPackage(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "catalogue.json", validCatalog())
	hugePkg := filepath.Join(ws, "huge_pkg.json")
	// packageLimits is 4 MB. Write 4 MB + 100 bytes.
	hugeData := make([]byte, packageLimits.MaxBytes+100)
	for i := range hugeData {
		hugeData[i] = ' '
	}
	if err := os.WriteFile(hugePkg, hugeData, 0o644); err != nil {
		t.Fatalf("write huge package: %v", err)
	}
	idx := `{
  "catalogue": "catalogue.json",
  "scenarios": {
    "SC-EX-1": "huge_pkg.json"
  }
}`
	writeFile(t, ws, "index.json", idx)

	rep, err := Prepare(ws, filepath.Join(ws, "index.json"))
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	if rep.Status != StatusInvalid {
		t.Fatalf("status = %s, want INVALID for oversized package", rep.Status)
	}
	found := false
	for _, f := range rep.Findings {
		if f.Code == CodeMalformedPackageFile && strings.Contains(f.Detail, "exceeds size limit") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected MALFORMED_PACKAGE_FILE size limit finding in %+v", rep.Findings)
	}
}

// TestBundleRefusesExistingDirectory verifies that Bundle refuses to overwrite
// an existing destination directory, preventing unexpected destruction.
func TestBundleRefusesExistingDirectory(t *testing.T) {
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
	out := filepath.Join(root, "pre_existing_dir")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a sentinel file inside pre_existing_dir to verify it is untouched
	sentinel := filepath.Join(out, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("preserve me"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	err = Bundle(dir, filepath.Join(dir, "index.json"), out, true, rep, "test")
	if err == nil {
		t.Fatal("expected Bundle to refuse overwrite on existing directory")
	}
	// Verify sentinel file is still preserved
	content, err := os.ReadFile(sentinel)
	if err != nil || string(content) != "preserve me" {
		t.Fatalf("sentinel file altered or missing: %v", err)
	}
}

// TestBundleRefusesOutputInsideWorkspace verifies that bundle refuses to write
// inside the input workspace to avoid recursion loops.
func TestBundleRefusesOutputInsideWorkspace(t *testing.T) {
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
	outInside := filepath.Join(dir, "bundle_output")
	err = Bundle(dir, filepath.Join(dir, "index.json"), outInside, true, rep, "test")
	if err == nil {
		t.Fatal("expected Bundle to refuse output inside input workspace")
	}
}

// TestIncompleteHandling verifies that a catalogue with valid structure but
// below launch minimums produces INCOMPLETE status, produces a rejection when
// draft is not allowed, and produces PREPARATION_DRAFT when allowDraft is set.
func TestIncompleteHandling(t *testing.T) {
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
	if rep.Status != StatusIncomplete {
		t.Fatalf("status = %s, want INCOMPLETE (1 state vs 10 launch min)", rep.Status)
	}

	// Without allowDraft: produces rejected bundle
	outRejected := filepath.Join(root, "out-rejected")
	if err := Bundle(dir, filepath.Join(dir, "index.json"), outRejected, false, rep, "test"); err != nil {
		t.Fatalf("bundle without allowDraft failed: %v", err)
	}
	rejectionBytes, err := os.ReadFile(filepath.Join(outRejected, "bundle-rejection.json"))
	if err != nil {
		t.Fatalf("missing bundle-rejection.json: %v", err)
	}
	if !strings.Contains(string(rejectionBytes), "PREPARATION_REJECTED") {
		t.Errorf("expected PREPARATION_REJECTED in rejection summary, got %s", string(rejectionBytes))
	}

	// With allowDraft: produces DRAFT bundle
	outDraft := filepath.Join(root, "out-draft")
	if err := Bundle(dir, filepath.Join(dir, "index.json"), outDraft, true, rep, "test"); err != nil {
		t.Fatalf("bundle with allowDraft failed: %v", err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(outDraft, "bundle-manifest.json"))
	if err != nil {
		t.Fatalf("missing bundle-manifest.json: %v", err)
	}
	if !strings.Contains(string(manifestBytes), "PREPARATION_DRAFT") {
		t.Errorf("expected PREPARATION_DRAFT in manifest, got %s", string(manifestBytes))
	}
}
