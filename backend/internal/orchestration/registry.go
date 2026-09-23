package orchestration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"

	"sthira/backend/internal/contracts"
)

// sha256HexOfString digests canonical template bytes (tpl.Text), never
// the rendered substitution output.
func sha256HexOfString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ProductionValidator implements VoiceValidator using the contracts package's
// shape and scoped semantic validators.
type ProductionValidator struct{}

// NewProductionValidator constructs a ProductionValidator.
func NewProductionValidator() *ProductionValidator {
	return &ProductionValidator{}
}

// ValidateShape validates that raw is well-formed JSON.
func (ProductionValidator) ValidateShape(raw []byte) error {
	if len(raw) == 0 {
		return nil
	}
	var probe map[string]any
	return json.Unmarshal(raw, &probe)
}

// Enforce runs contracts.EnforceScopedContext.
func (ProductionValidator) Enforce(out contracts.ModelOutput, sc contracts.ScopedContext) error {
	return contracts.EnforceScopedContext(out, sc)
}

// FlatValidate runs contracts.ValidateModelOutput.
func (ProductionValidator) FlatValidate(out contracts.ModelOutput, requestID, dataVersion string, known map[string]bool, langs map[string]bool) error {
	return contracts.ValidateModelOutput(out, requestID, dataVersion, known, langs)
}

// MapTemplateRegistry is a concurrent-safe implementation of TemplateRegistry.
type MapTemplateRegistry struct {
	mu   sync.RWMutex
	tpls map[string]contracts.ApprovedTemplate
}

// NewMapTemplateRegistry creates an empty MapTemplateRegistry.
func NewMapTemplateRegistry() *MapTemplateRegistry {
	return &MapTemplateRegistry{
		tpls: make(map[string]contracts.ApprovedTemplate),
	}
}

// Add inserts or updates an approved template. The template's
// TemplateSHA256 is computed over Text when empty so the orchestrator
// can compare against the DB-approved digest.
func (r *MapTemplateRegistry) Add(t contracts.ApprovedTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.TemplateSHA256 == "" {
		t.TemplateSHA256 = sha256HexOfString(t.Text)
	}
	key := t.SpeechKey + "/" + t.Language
	r.tpls[key] = t
}

// Lookup finds an approved template by speech key and language.
func (r *MapTemplateRegistry) Lookup(speechKey, language string) (contracts.ApprovedTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := speechKey + "/" + language
	t, ok := r.tpls[key]
	return t, ok
}

// Keys returns all unique speech keys in sorted order.
func (r *MapTemplateRegistry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	var keys []string
	for _, t := range r.tpls {
		if !seen[t.SpeechKey] {
			seen[t.SpeechKey] = true
			keys = append(keys, t.SpeechKey)
		}
	}
	sort.Strings(keys)
	return keys
}

// DefaultTemplateRegistry returns a TemplateRegistry preloaded with
// synthetic-only translations for the supported languages
// (en-IN, hi-IN, ml-IN). Real approval must be loaded from recorded
// evidence (e.g. offline translations loaded into a MapTemplateRegistry
// at startup) — the built-in default MUST NOT silently serve as
// operational approval. TemplateVersion/SourceVersion are intentionally
// 0 here to flag "not approved"; the production wired registry must
// override these with recorded source/version evidence before a
// jurisdiction can return speech_key text.
func DefaultTemplateRegistry() *MapTemplateRegistry {
	r := NewMapTemplateRegistry()
	templates := []contracts.ApprovedTemplate{
		{
			SpeechKey:       "destination_options",
			Language:        "en-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "Destination choices are displayed on screen.",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "destination_options",
			Language:        "hi-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "गंतव्य विकल्प स्क्रीन पर प्रदर्शित हैं।",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "destination_options",
			Language:        "ml-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "ലക്ഷ്യസ്ഥാന ഓപ്ഷനുകൾ സ്ക്രീനിൽ കാണിച്ചിരിക്കുന്നു.",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "clarify_place",
			Language:        "en-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "Please clarify the location.",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "clarify_place",
			Language:        "hi-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "कृपया स्थान स्पष्ट करें।",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "clarify_place",
			Language:        "ml-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "ദയവായി സ്ഥലം വ്യക്തമാക്കുക.",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "verified_route_unavailable",
			Language:        "en-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "Verified route is currently unavailable.",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "verified_route_unavailable",
			Language:        "hi-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "सत्यापित मार्ग वर्तमान में अनुपलब्ध है।",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "verified_route_unavailable",
			Language:        "ml-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "സ്ഥിരീകരിച്ച റൂട്ട് നിലവിൽ ലഭ്യമല്ല.",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "welcome",
			Language:        "en-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "Welcome to Sthira emergency guidance.",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "welcome",
			Language:        "hi-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "स्थिरा आपातकालीन मार्गदर्शन में आपका स्वागत है।",
			SyntheticOnly:   true,
		},
		{
			SpeechKey:       "welcome",
			Language:        "ml-IN",
			TemplateVersion: 0,
			SourceVersion:   0,
			Text:            "സ്ഥിര അടിയന്തര മാർഗ്ഗനിർദ്ദേശത്തിലേക്ക് സ്വാഗതം.",
			SyntheticOnly:   true,
		},
	}
	for _, t := range templates {
		r.Add(t)
	}
	return r
}
