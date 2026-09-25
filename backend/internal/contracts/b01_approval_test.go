package contracts

import "testing"

// TestIsSpeechKeyApprovedForLanguage_FailClosed: B01 — no TemplateKeys
// fallback, no wildcard language, no empty keys.
func TestIsSpeechKeyApprovedForLanguage_FailClosed(t *testing.T) {
	sc := mkScoped()

	if !sc.IsSpeechKeyApprovedForLanguage("destination_options", "en-IN") {
		t.Fatalf("expected exact language approval")
	}
	if sc.IsSpeechKeyApprovedForLanguage("destination_options", "ta-IN") {
		t.Fatalf("unapproved language must fail closed")
	}
	if sc.IsSpeechKeyApprovedForLanguage("unknown_key", "en-IN") {
		t.Fatalf("key absent from ApprovedSpeechKeys must fail closed (no TemplateKeys fallback)")
	}
	if sc.IsSpeechKeyApprovedForLanguage("", "en-IN") {
		t.Fatalf("empty key must fail closed")
	}
	if sc.IsSpeechKeyApprovedForLanguage("destination_options", "") {
		t.Fatalf("empty language must fail closed")
	}

	// Empty ApprovedSpeechKeys must not fall back to TemplateKeys.
	sc2 := mkScoped()
	sc2.ApprovedSpeechKeys = nil
	sc2.ApprovedTemplateSHA = nil
	if sc2.IsSpeechKeyApprovedForLanguage("destination_options", "en-IN") {
		t.Fatalf("nil ApprovedSpeechKeys must fail closed despite TemplateKeys")
	}

	// "*" is not an accepted language.
	sc3 := mkScoped()
	sc3.ApprovedSpeechKeys = map[string][]string{"destination_options": {"*"}}
	if sc3.IsSpeechKeyApprovedForLanguage("destination_options", "en-IN") {
		t.Fatalf("wildcard * language must fail closed")
	}
}
