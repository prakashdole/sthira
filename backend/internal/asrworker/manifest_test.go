package asrworker

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScanLocalInventory_EmptyRootReportsAllPending: the inventory
// fails closed on every component when the model directory is empty
// or missing. The orchestrator sees PendingCritical and refuses to
// dispatch.
func TestScanLocalInventory_EmptyRootReportsAllPending(t *testing.T) {
	inv := ScanLocalInventory("", DefaultIndicConformer())
	if len(inv.PendingCritical) != 4 {
		t.Errorf("expected 4 pending, got %d (%v)", len(inv.PendingCritical), inv.PendingCritical)
	}
	if err := inv.Ready(); err == nil {
		t.Errorf("empty-root inventory must NOT be Ready: %+v", inv)
	}
}

// TestScanLocalInventory_NonExistentRootReportsAllPending: passing
// a root that does not exist surfaces pending-critical entries.
// Inventory and readiness fail closed.
func TestScanLocalInventory_NonExistentRootReportsAllPending(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	inv := ScanLocalInventory(dir, DefaultIndicConformer())
	if len(inv.PendingCritical) != 4 {
		t.Errorf("expected 4 pending, got %d (%v)", len(inv.PendingCritical), inv.PendingCritical)
	}
	if inv.LastScannedAt.IsZero() {
		t.Errorf("LastScannedAt must be populated even on miss")
	}
}

// TestScanLocalInventory_PartialPresenceMarksOnlyPresent: if only
// model_onnx.py exists (the gateway import of the reference
// runtime), the other three components are still pending. The
// reference runtime's failure path surfaces here.
func TestScanLocalInventory_PartialPresenceMarksOnlyPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model_onnx.py"), []byte("print()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv := ScanLocalInventory(dir, DefaultIndicConformer())
	// model_onnx.py is present → it must NOT be pending.
	for _, n := range inv.PendingCritical {
		if n == CompRuntime {
			t.Errorf("model_onnx.py is present; runtime must not be pending: %v", inv.PendingCritical)
		}
	}
	// The other three are still pending.
	if len(inv.PendingCritical) < 3 {
		t.Errorf("expected at least 3 pending (model/tokenizer/config); got %v", inv.PendingCritical)
	}
}

// TestScanLocalInventory_HashesSmallFileProbe: when a probe file is
// under 64 KiB, the scanner records its actual SHA-256. Tests
// verify the recorded digest matches a fresh hash.
func TestScanLocalInventory_HashesSmallFileProbe(t *testing.T) {
	dir := t.TempDir()
	want := []byte("print()\n")
	if err := os.WriteFile(filepath.Join(dir, "model_onnx.py"), want, 0o644); err != nil {
		t.Fatal(err)
	}
	inv := ScanLocalInventory(dir, DefaultIndicConformer())
	for _, hm := range inv.HashMatches {
		if hm.Name == CompRuntime {
			if hm.ActualSHA256 == "" {
				t.Errorf("model_onnx.py must record a digest; got: %+v", hm)
			}
			if got := SHA256Hex(want); got != hm.ActualSHA256 {
				t.Errorf("digest: got %s want %s", hm.ActualSHA256, got)
			}
		}
	}
}

// TestIndicConformer_DefaultHasLicensePendingMarkers: the default
// artifact advertises the documented upstream reference while
// keeping every component digest empty until an authorized verifier
// records it. Worker 5 fails closed without verifier-recorded
// digests.
func TestIndicConformer_DefaultHasLicensePendingMarkers(t *testing.T) {
	ic := DefaultIndicConformer()
	if ic.ModelID == "" {
		t.Errorf("ModelID must be set")
	}
	if ic.LicenseRef == "" {
		t.Errorf("LicenseRef must be set")
	}
	if ic.ModelCheckpoint.HasRecordedDigest() {
		t.Errorf("ModelCheckpoint must be empty until verifier records")
	}
	if ic.Tokenizer.HasRecordedDigest() {
		t.Errorf("Tokenizer must be empty until verifier records")
	}
	if ic.Runtime.HasRecordedDigest() {
		t.Errorf("Runtime must be empty until verifier records")
	}
	if ic.ONNXConfig.HasRecordedDigest() {
		t.Errorf("ONNXConfig must be empty until verifier records")
	}
}

// TestArtifactDigest_HasRecordedDigestDistinguishesMissingFromZero:
// HasRecordedDigest distinguishes "no digest yet" from a digest of
// empty bytes.
func TestArtifactDigest_HasRecordedDigestDistinguishesMissingFromZero(t *testing.T) {
	var empty ArtifactDigest
	if empty.HasRecordedDigest() {
		t.Errorf("zero digest must NOT be reported as recorded")
	}
	zeros := ArtifactDigest{ChecksumSHA256: HashZero}
	if zeros.HasRecordedDigest() {
		t.Errorf("zero-hash sentinel must NOT be reported as recorded")
	}
	real := ArtifactDigest{ChecksumSHA256: "abcd"}
	if !real.HasRecordedDigest() {
		t.Errorf("non-empty digest must be recorded")
	}
}

// TestInventory_ReadyRequiresNonEmptyLanguages: a ready inventory
// must include at least one language. Worker 5 refuses to surface
// Ready=true to a runtime that advertises zero languages.
func TestInventory_ReadyRequiresNonEmptyLanguages(t *testing.T) {
	inv := Inventory{IndicConformer: DefaultIndicConformer()}
	// PendingCritical is zero (no scan), HashMatches is empty.
	// AllowedLanguages is empty.
	if err := inv.Ready(); err == nil {
		t.Errorf("empty-languages inventory must NOT be Ready")
	}
	inv.AllowedLanguages = []string{"hi-IN"}
	// Still not Ready: hash digest mismatch check has nothing
	// to flag, so Ready == nil unless pending-critical is set.
	// With PendingCritical=nil this should pass.
	if err := inv.Ready(); err != nil {
		t.Errorf("with allowed languages, ready check should pass: %v", err)
	}
}

// TestComponentCritical_TableDriven: confirm only the four expected
// components are critical.
func TestComponentCritical_TableDriven(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{CompModelCheckpoint, true},
		{CompTokenizer, true},
		{CompRuntime, true},
		{CompONNXConfig, true},
		{"not_a_component", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ComponentCritical(c.name); got != c.want {
			t.Errorf("ComponentCritical(%q): got %v want %v", c.name, got, c.want)
		}
	}
}
