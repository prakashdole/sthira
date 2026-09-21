package middleworker

import (
	"reflect"
	"testing"
)

// TestSarvamModelID_IsCanonical asserts the model identifier is the
// HF repo path, not a friendly alias.
func TestSarvamModelID_IsCanonical(t *testing.T) {
	if SarvamModelID != "sarvamai/sarvam-30b" {
		t.Errorf("SarvamModelID: got %q want canonical %q",
			SarvamModelID, "sarvamai/sarvam-30b")
	}
}

// TestSarvamVLLMStartupArgs_UsesFP8 enforces that vLLM is configured
// with --quantization fp8, NOT bf16/fp16. Decision D59 (Sarvam-30B FP8).
func TestSarvamVLLMStartupArgs_UsesFP8(t *testing.T) {
	args := SarvamVLLMStartupArgs()
	found := false
	for i, a := range args {
		if a == "--quantization" {
			if i+1 >= len(args) {
				t.Fatal("--quantization flag without value")
			}
			if args[i+1] != "fp8" {
				t.Errorf("vLLM quantization: got %q want %q",
					args[i+1], "fp8")
			}
			found = true
		}
		if a == "fp16" || a == "bf16" {
			t.Errorf("must NOT use %q — D59 selects FP8", a)
		}
	}
	if !found {
		t.Error("--quantization fp8 not in vLLM startup args")
	}
}

// TestSarvamVLLMStartupArgs_BindsLoopback enforces the
// fail-closed network boundary: vLLM listens on loopback only so
// the worker cannot be exposed to citizen ingress.
func TestSarvamVLLMStartupArgs_BindsLoopback(t *testing.T) {
	args := SarvamVLLMStartupArgs()
	host, port := "", ""
	for i, a := range args {
		if a == "--host" && i+1 < len(args) {
			host = args[i+1]
		}
		if a == "--port" && i+1 < len(args) {
			port = args[i+1]
		}
	}
	if host != "127.0.0.1" {
		t.Errorf("vLLM host: got %q want %q (loopback only)", host, "127.0.0.1")
	}
	if port != "8000" {
		t.Errorf("vLLM port: got %q want %q", port, "8000")
	}
}

// TestSarvamVLLMStartupArgs_NoRemovedGuidedFlag enforces that the
// startup reference never resurrects the --guided-decoding-backend
// flag, which vLLM >=0.12 removed (verified against the pinned
// structured-outputs docs, fetched 2026-09-21). Structured output is
// selected per-request via response_format instead.
func TestSarvamVLLMStartupArgs_NoRemovedGuidedFlag(t *testing.T) {
	for _, a := range SarvamVLLMStartupArgs() {
		if a == "--guided-decoding-backend" {
			t.Error("--guided-decoding-backend was removed in vLLM >=0.12; do not pass it")
		}
	}
}

// TestSarvamLimits_Bounded ensures output, context, and timeout
// are bounded per the action contract.
func TestSarvamLimits_Bounded(t *testing.T) {
	limits := SarvamLimits()
	if limits.MaxOutputTokens > 512 {
		t.Errorf("MaxOutputTokens too large for action contract: %d", limits.MaxOutputTokens)
	}
	if limits.MaxContextTokens <= 0 {
		t.Errorf("MaxContextTokens must be positive: %d", limits.MaxContextTokens)
	}
	if limits.PerCallTimeout <= 0 {
		t.Errorf("PerCallTimeout must be positive: %v", limits.PerCallTimeout)
	}
	if limits.MaxResponseBytes <= 0 {
		t.Errorf("MaxResponseBytes must be positive: %d", limits.MaxResponseBytes)
	}
}

// TestSarvamChatTemplateKwargs_DisablesThinking ensures the client
// sends enable_thinking=false as a Jinja variable to the
// server-loaded official Sarvam template (whose raw chat_template.
// jinja, fetched 2026-09-21, reads exactly this variable and emits
// the model's <|nothink|> token). The previous "template override"
// constant set the variable in an invented template that never
// referenced it — a dead switch, now removed.
func TestSarvamChatTemplateKwargs_DisablesThinking(t *testing.T) {
	kw := SarvamChatTemplateKwargs()
	if v, ok := kw["enable_thinking"]; !ok || v != false {
		t.Errorf("SarvamChatTemplateKwargs must set enable_thinking=false; got %v", kw)
	}
}

// TestSarvamSupportedLanguages_Intersection returns a non-empty
// list of the languages the worker advertises; this is the
// intersection with verified evaluation evidence.
func TestSarvamSupportedLanguages_Intersection(t *testing.T) {
	got := SarvamSupportedLanguages()
	want := []string{"en-IN", "hi-IN", "ml-IN"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SupportedLanguages: got %v want %v", got, want)
	}
}

// TestSarvamConfig_PopulatesFields checks that the constructed
// config binds the canonical model id and the supported languages.
// Uses a minimal stub Client (no network) — SarvamConfig only
// stores references; it does not contact vLLM.
func TestSarvamConfig_PopulatesFields(t *testing.T) {
	cfg := SarvamConfig(&Client{}, "test-system")
	if cfg.ModelID != SarvamModelID {
		t.Errorf("ModelID: got %q want %q", cfg.ModelID, SarvamModelID)
	}
	if cfg.DigestName != SarvamModelID {
		t.Errorf("DigestName: got %q want %q", cfg.DigestName, SarvamModelID)
	}
	if cfg.System != "test-system" {
		t.Errorf("System prompt not propagated: %q", cfg.System)
	}
	if len(cfg.Languages) == 0 {
		t.Error("Languages empty")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
