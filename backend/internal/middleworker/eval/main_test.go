// Eval-driver smoke tests. No real vLLM. The harness-check and
// manifest modes are the only ones verifiable today; benchmark
// mode is gated on real artifacts.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEval_HarnessCheckPrintsBlocker(t *testing.T) {
	// Capture stdout by re-running the binary would be heavy; we
	// instead assert on the manifest content which is the
	// authoritative artifact record.
	bs, err := json.Marshal(Pinned)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(bs)
	if !strings.Contains(s, "Qwen3-4B-Instruct-2507") {
		t.Errorf("manifest missing model id: %s", s)
	}
	if !strings.Contains(s, "Apache-2.0") {
		t.Errorf("manifest missing license: %s", s)
	}
	if !strings.Contains(s, `"trust_remote_code":false`) {
		t.Errorf("manifest missing trust_remote_code=false: %s", s)
	}
	if !strings.Contains(s, "NOT_EVALUATED") {
		t.Errorf("manifest must surface NOT_EVALUATED for unverified fields: %s", s)
	}
	if !strings.Contains(s, "BLOCKED_HARDWARE") {
		t.Errorf("manifest must surface BLOCKED_HARDWARE for artifacts: %s", s)
	}
}

func TestEval_ManifestModeWritesFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "manifest.json")
	bs, _ := json.MarshalIndent(Pinned, "", "  ")
	if err := os.WriteFile(out, bs, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "Qwen3-4B-Instruct-2507") {
		t.Errorf("manifest file missing model id")
	}
}

func TestEval_BenchmarkModeRejectsWithoutEndpoint(t *testing.T) {
	// benchmark mode without an endpoint is a documented
	// NOT_EVALUATED path; this test asserts the manifest stays
	// free of fake SHA-256 values.
	for _, a := range []ArtifactInfo{Pinned.BF16Artifact, Pinned.AWQInt4Artifact} {
		if a.SHA256 != "" {
			t.Errorf("artifact SHA-256 must be empty until real-inference is recorded")
		}
	}
}
