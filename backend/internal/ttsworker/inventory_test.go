package ttsworker

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScanLocalInventory_EmptyRootReportsAllPending: missing root
// means every critical component is PendingCritical.
func TestScanLocalInventory_EmptyRootReportsAllPending(t *testing.T) {
	inv := ScanLocalInventory("", DefaultParlerTTS())
	if len(inv.PendingCritical) != 5 {
		t.Errorf("expected 5 pending critical, got %d (%v)", len(inv.PendingCritical), inv.PendingCritical)
	}
	if err := inv.Ready(); err == nil {
		t.Errorf("empty-root inventory must NOT be Ready")
	}
}

// TestScanLocalInventory_PartialPresence: a partial artifact set
// keeps the missing components pending.
func TestScanLocalInventory_PartialPresence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "parler_tts_runtime.py"), []byte("print()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model.safetensors"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "voices.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	inv := ScanLocalInventory(dir, DefaultParlerTTS())
	for _, n := range inv.PendingCritical {
		if n == CompRuntime {
			t.Errorf("runtime must not be pending: %v", inv.PendingCritical)
		}
		if n == CompModelCheckpoint {
			t.Errorf("model checkpoint present: %v", inv.PendingCritical)
		}
		if n == CompVoiceRegistry {
			t.Errorf("voices.json present: %v", inv.PendingCritical)
		}
	}
}

// TestInventoryReadyRequiresAuthorizedLicense: a default ParlerTTS
// with LicensePending must not pass Ready.
func TestInventoryReadyRequiresAuthorizedLicense(t *testing.T) {
	inv := Inventory{Parler: DefaultParlerTTS()}
	if err := inv.Ready(); err == nil {
		t.Errorf("DefaultParlerTTS inventory must NOT be Ready (license pending)")
	}
}

// TestParlerRemoteCodeMustBeFalse: production must keep
// RemoteCode false; the inventory refuses Ready=true if true.
func TestParlerRemoteCodeMustBeFalse(t *testing.T) {
	inv := Inventory{
		Parler: ParlerTTS{
			ModelID:            "ai4bharat/indic-parler-tts",
			License:            "Apache-2.0",
			Runtime:            "transformers-4.x",
			Hardware:           "cpu",
			Revision:           "rev-1",
			Voices:             []string{"ml-IN-female-1"},
			SupportedLanguages: []string{"ml-IN"},
		},
	}
	if err := inv.Ready(); err == nil {
		t.Errorf("inventory with default parlertts (RemoteCode false) MUST still require explicit SupportedLanguages from adapter")
	}
	inv.RemoteCodeTrust = true
	if err := inv.Ready(); err == nil {
		t.Errorf("RemoteCodeTrust true must reject Ready")
	}
}

// TestParlerComponentCriticalTableDriven: confirm only the five
// canonical components are critical.
func TestParlerComponentCriticalTableDriven(t *testing.T) {
	for _, c := range []string{CompModelCheckpoint, CompTokenizer, CompVoiceRegistry, CompRuntime, CompPromptPrefix} {
		if !ComponentCritical(c) {
			t.Errorf("%q must be critical", c)
		}
	}
	for _, c := range []string{"", "not_a_component"} {
		if ComponentCritical(c) {
			t.Errorf("%q must NOT be critical", c)
		}
	}
}

// TestHashZeroIsRecognized: HashZero is the sentinel used to
// distinguish "no digest recorded" from an empty digest.
func TestHashZeroIsRecognized(t *testing.T) {
	if HashZero == "" {
		t.Fatal("HashZero must be a non-empty sentinel")
	}
	if len(HashZero) != 64 {
		t.Fatalf("HashZero must be 64 hex chars, got %d", len(HashZero))
	}
}
