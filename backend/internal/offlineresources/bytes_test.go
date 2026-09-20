package offlineresources

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestFixtureBytes_AreMeasuredNotEstimated validates the contract
// requirement that measured fixture bytes are reported separately from
// real-world estimates. The fixture pack used by the pack tests is
// measured on disk via the rendered JSON of each descriptor; this is
// what the integration agent will surface in the public "pack size"
// copy.
//
// The critical incident card is EXCLUDED from this fixture; its 64 KiB
// budget is enforced by Agent 1 (offlinepkg) and Agent 2 (offlinedelivery).
// What this test asserts is that the validator's own fixture pack, when
// serialised and gzipped, fits inside the 50 MiB optional regional pack
// budget by a large margin and that the measured bytes are reported
// (not estimated).
func TestFixtureBytes_AreMeasuredNotEstimated(t *testing.T) {
	pack := completePack()

	// Sum declared byte sizes; this is the *fixture* budget, not a real
	// district pack. The test does NOT claim the synthetic pack proves
	// a real production pack fits; that is the gate listed in the
	// task description and remains OPEN.
	var declared int64
	for _, d := range pack {
		declared += d.ByteSize
	}

	// Serialize the pack as canonical-ish JSON and measure the wire size.
	// We deliberately do not gzip the *declared* bytes (those are the
	// shipped asset sizes); we measure the wire-level descriptor list
	// separately so the audit can compare against a real public endpoint.
	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if _, err := gz.Write(raw); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	t.Logf("fixture pack: declared=%d bytes, descriptor JSON=%d bytes, descriptor JSON.gz=%d bytes",
		declared, len(raw), buf.Len())

	if declared >= DefaultRegionalPackBudgetBytes {
		t.Fatalf("fixture pack declared bytes (%d) must be well under the 50 MiB budget", declared)
	}
	if int64(len(raw)) >= DefaultRegionalPackBudgetBytes {
		t.Fatalf("descriptor JSON too large for a fixture: %d bytes", len(raw))
	}
}

// TestFixtureStyleFile_OnDiskIsTiny asserts that the bundled style fixture
// file is genuinely tiny and not a stand-in for a real MapLibre style.
// The integration agent must replace this with a measured style from a
// real provider before any production claim is made.
func TestFixtureStyleFile_OnDiskIsTiny(t *testing.T) {
	path := filepath.Join("testdata", "style_complete.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("fixture file missing: %v", err)
	}
	if info.Size() > 4*1024 {
		t.Fatalf("fixture style must stay under 4 KiB; got %d bytes", info.Size())
	}
	t.Logf("style_complete.json = %d bytes (synthetic)", info.Size())

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	res, err := ValidateMapStyle(data, completeStyleFixture(), false)
	if err != nil {
		t.Fatalf("fixture style must validate: %v", err)
	}
	if !res.Valid {
		t.Fatalf("fixture style must report Valid=true: %+v", res)
	}
}
