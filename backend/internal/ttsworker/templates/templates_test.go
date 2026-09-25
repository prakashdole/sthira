package templates

import (
	"errors"
	"strings"
	"testing"
)

// TestRegisterAndLookup: a registered template is round-tripped.
func TestRegisterAndLookup(t *testing.T) {
	c := NewCatalog()
	tpl := &Template{
		Key:         "destination_options",
		Status:      StatusApproved,
		Version:     1,
		Description: "Destination options for the assigned safe zone",
		Languages:   []string{"en-IN", "hi-IN", "ml-IN"},
		ArgSchema: map[string]ArgType{
			"facility_id": ArgString,
			"rank":        ArgInt,
		},
		Translations: map[string]string{
			"en-IN": "Showing your assigned safe zone options.",
			"hi-IN": "आपके निर्धारित सुरक्षित क्षेत्र के विकल्प दिखाए जा रहे हैं।",
			"ml-IN": "നിങ്ങളുടെ നിയോഗിച്ച സുരക്ഷിത മേഖലയുടെ ഓപ്ഷനുകൾ കാണിക്കുന്നു.",
		},
	}
	if err := c.Register(tpl); err != nil {
		t.Fatal(err)
	}
	got, ok := c.Lookup("destination_options")
	if !ok {
		t.Fatal("expected to find")
	}
	if got.Version != 1 || got.Status != StatusApproved {
		t.Fatalf("status/version lost: %+v", got)
	}
}

// TestRegisterRefusesUndeclaredTranslation: a template whose
// translation references an undeclared language is rejected.
func TestRegisterRefusesUndeclaredTranslation(t *testing.T) {
	c := NewCatalog()
	tpl := &Template{
		Key:       "destination_options",
		Status:    StatusApproved,
		Version:   1,
		Languages: []string{"en-IN"},
		Translations: map[string]string{
			"tl-PH": "Placeholder",
		},
	}
	if err := c.Register(tpl); err == nil {
		t.Fatal("expected rejection for undeclared translation language")
	}
}

// TestRenderRespectsArgSchema: extra args are rejected; missing args
// rejected; valid args render exactly.
func TestRenderRespectsArgSchema(t *testing.T) {
	c := buildOKCatalog(t)
	r := NewRenderer(c)
	// valid
	out, err := r.Render("safe_zone_options", "en-IN", []Argument{
		{Name: "facility_id", Type: ArgString, Value: "FACILITY-1"},
		{Name: "rank", Type: ArgInt, Value: "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "FACILITY-1") {
		t.Fatalf("expected substituted text: %q", out.Text)
	}
	// missing arg
	if _, err := r.Render("safe_zone_options", "en-IN", []Argument{
		{Name: "facility_id", Type: ArgString, Value: "FACILITY-1"},
	}); !errors.Is(err, ErrArgMismatch) {
		t.Fatalf("expected ErrArgMismatch, got %v", err)
	}
	// extra arg via renderer path: validation rejects it.
	if _, err := c.ValidateArgs("safe_zone_options", map[string]any{
		"facility_id":  "FACILITY-1",
		"rank":         1,
		"unsanctioned": "nope",
	}); !errors.Is(err, ErrUnknownArg) {
		t.Fatalf("expected ErrUnknownArg, got %v", err)
	}
}

// TestRenderRejectsTemplateInjection: substitution placeholders can
// never smuggle braces or special characters — the validator rejects
// forbidden chars and the renderer writes only validated values.
func TestRenderRejectsTemplateInjection(t *testing.T) {
	c := NewCatalog()
	tpl := &Template{
		Key:          "advisory",
		Status:       StatusApproved,
		Version:      1,
		Languages:    []string{"en-IN"},
		ArgSchema:    map[string]ArgType{"facility_id": ArgString},
		Translations: map[string]string{"en-IN": "Advisory for {facility_id}."},
	}
	if err := c.Register(tpl); err != nil {
		t.Fatal(err)
	}
	_ = NewRenderer(c)
	// injection-style strings
	for _, payload := range []string{
		"{facility_id}",             // placeholder collision
		"<script>alert(1)</script>", // bytes forbidden by validIDChar
		"foo;bar",                   // semicolon forbidden
		"DROP 'table'",              // quote forbidden
		"a\\b",                      // backslash forbidden
	} {
		_, err := c.ValidateArgs("advisory", map[string]any{"facility_id": payload})
		if err == nil {
			t.Fatalf("payload %q accepted: expected injection rejection", payload)
		}
	}
}

// TestRenderHonorsWithdrawnTemplate: a withdrawn template cannot be
// synthesized, even if its translation table is intact.
func TestRenderHonorsWithdrawnTemplate(t *testing.T) {
	c := buildOKCatalog(t)
	if err := c.Withdraw("safe_zone_options", 2); err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(c)
	if _, err := r.Render("safe_zone_options", "en-IN", []Argument{
		{Name: "facility_id", Type: ArgString, Value: "FACILITY-1"},
		{Name: "rank", Type: ArgInt, Value: "1"},
	}); !errors.Is(err, ErrWithdrawn) {
		t.Fatalf("expected ErrWithdrawn, got %v", err)
	}
}

// TestRenderRejectsSyntheticOnlyByDefault: synthetic-only templates
// must require an explicit opt-in.
func TestRenderRejectsSyntheticOnlyByDefault(t *testing.T) {
	c := NewCatalog()
	tpl := &Template{
		Key:          "demo_filler",
		Status:       StatusSyntheticOnly,
		Version:      1,
		Languages:    []string{"en-IN"},
		ArgSchema:    nil,
		Translations: map[string]string{"en-IN": "Demo filler."},
	}
	if err := c.Register(tpl); err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(c) // no AllowSynthetic
	_, err := r.Render("demo_filler", "en-IN", nil)
	if !errors.Is(err, ErrSyntheticOnly) {
		t.Fatalf("expected ErrSyntheticOnly, got %v", err)
	}
	// AllowSynthetic toggles it on for tests.
	r.AllowSynthetic(true)
	if _, err := r.Render("demo_filler", "en-IN", nil); err != nil {
		t.Fatalf("allow-toggle should bypass synthetic lock: %v", err)
	}
}

// TestRenderRejectsPendingReview: pending-review templates render
// only after approval.
func TestRenderRejectsPendingReview(t *testing.T) {
	c := NewCatalog()
	tpl := &Template{
		Key:          "untouched",
		Status:       StatusPendingReview,
		Version:      1,
		Languages:    []string{"en-IN"},
		Translations: map[string]string{"en-IN": "irrelevant"},
	}
	if err := c.Register(tpl); err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(c)
	if _, err := r.Render("untouched", "en-IN", nil); !errors.Is(err, ErrPendingReview) {
		t.Fatalf("expected ErrPendingReview, got %v", err)
	}
}

// TestRenderRejectsUnknownLanguage: a language not in the template's
// declared list returns ErrLanguageUnsupported (NEVER an invented
// translation).
func TestRenderRejectsUnknownLanguage(t *testing.T) {
	c := buildOKCatalog(t)
	r := NewRenderer(c)
	if _, err := r.Render("safe_zone_options", "tl-PH", []Argument{
		{Name: "facility_id", Type: ArgString, Value: "FACILITY-1"},
		{Name: "rank", Type: ArgInt, Value: "1"},
	}); !errors.Is(err, ErrLanguageUnsupported) {
		t.Fatalf("expected ErrLanguageUnsupported, got %v", err)
	}
}

// TestRenderRejectsUnknownKey: arbitrary speech_key values are rejected.
func TestRenderRejectsUnknownKey(t *testing.T) {
	c := buildOKCatalog(t)
	r := NewRenderer(c)
	if _, err := r.Render("not-in-catalog", "en-IN", nil); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("expected ErrUnknownKey, got %v", err)
	}
}

// TestRenderRefusesTemplateWithUndeclaredPlaceholder: catalog
// registration refuses bodies that reference undeclared args.
func TestRenderRefusesTemplateWithUndeclaredPlaceholder(t *testing.T) {
	c := NewCatalog()
	tpl := &Template{
		Key:          "broken",
		Status:       StatusApproved,
		Version:      1,
		Languages:    []string{"en-IN"},
		ArgSchema:    map[string]ArgType{"facility_id": ArgString},
		Translations: map[string]string{"en-IN": "Hello {nope}"},
	}
	if err := c.Register(tpl); err == nil {
		t.Fatal("expected registration rejection for undeclared placeholder")
	}
}

// TestValidateArgsRejectsEmptyString: a validated string arg cannot
// be empty.
func TestValidateArgsRejectsEmptyString(t *testing.T) {
	c := buildOKCatalog(t)
	if _, err := c.ValidateArgs("safe_zone_options", map[string]any{
		"facility_id": "",
		"rank":        1,
	}); !errors.Is(err, ErrArgMismatch) {
		t.Fatalf("expected ErrArgMismatch for empty string, got %v", err)
	}
}

// TestValidateArgsCastsFloats: floats with integer values are
// accepted; non-integer floats rejected.
func TestValidateArgsCastsFloats(t *testing.T) {
	c := buildOKCatalog(t)
	_, err := c.ValidateArgs("safe_zone_options", map[string]any{
		"facility_id": "FACILITY-1",
		"rank":        float64(2),
	})
	if err != nil {
		t.Fatalf("float integer cast failed: %v", err)
	}
	_, err = c.ValidateArgs("safe_zone_options", map[string]any{
		"facility_id": "FACILITY-1",
		"rank":        float64(2.5),
	})
	if err == nil {
		t.Fatalf("non-integer float must be rejected")
	}
}

// TestKeysLists: Keys returns the registered template keys sorted.
func TestKeysLists(t *testing.T) {
	c := NewCatalog()
	if err := c.Register(&Template{
		Key: "alpha", Status: StatusApproved, Languages: []string{"en-IN"},
		Translations: map[string]string{"en-IN": "a"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Register(&Template{
		Key: "zeta", Status: StatusApproved, Languages: []string{"en-IN"},
		Translations: map[string]string{"en-IN": "z"},
	}); err != nil {
		t.Fatal(err)
	}
	keys := c.Keys()
	if len(keys) != 2 || keys[0] != "alpha" || keys[1] != "zeta" {
		t.Fatalf("expected sorted [alpha, zeta], got %v", keys)
	}
}

// TestSnapshotIsolatesAfterMutation: mutating the original catalog
// after Snapshot must NOT affect the snapshot (deep copy).
func TestSnapshotIsolatesAfterMutation(t *testing.T) {
	c := buildOKCatalog(t)
	snap := c.Snapshot()
	if err := c.Withdraw("safe_zone_options", 9); err != nil {
		t.Fatal(err)
	}
	got, ok := snap.Lookup("safe_zone_options")
	if !ok || got.Status == StatusWithdrawn {
		t.Fatalf("snapshot mutated by catalog Withdraw: %+v", got)
	}
}

func buildOKCatalog(t *testing.T) *Catalog {
	t.Helper()
	c := NewCatalog()
	tpl := &Template{
		Key:         "safe_zone_options",
		Status:      StatusApproved,
		Version:     1,
		Description: "Spoken menu for destination options",
		Languages:   []string{"en-IN", "hi-IN", "ml-IN"},
		ArgSchema: map[string]ArgType{
			"facility_id": ArgString,
			"rank":        ArgInt,
		},
		Translations: map[string]string{
			"en-IN": "Showing your assigned safe zone options. First: {facility_id}, rank {rank}.",
			"hi-IN": "आपके निर्धारित सुरक्षित क्षेत्र के विकल्प दिखाए जा रहे हैं। पहला: {facility_id}, क्रम {rank}.",
			"ml-IN": "നിങ്ങളുടെ നിയോഗിച്ച സുരക്ഷിത മേഖലയുടെ ഓപ്ഷനുകൾ കാണിക്കുന്നു. ആദ്യം: {facility_id}, റാങ്ക് {rank}.",
		},
	}
	if err := c.Register(tpl); err != nil {
		t.Fatal(err)
	}
	return c
}
