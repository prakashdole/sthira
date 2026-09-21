package orchestration

import (
	"encoding/json"
	"sort"
	"sync"

	"sthira/backend/internal/contracts"
)

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

// Add inserts or updates an approved template.
func (r *MapTemplateRegistry) Add(t contracts.ApprovedTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
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

// DefaultTemplateRegistry returns a TemplateRegistry preloaded with approved templates
// for the supported languages (en-IN, hi-IN, ml-IN).
func DefaultTemplateRegistry() *MapTemplateRegistry {
	r := NewMapTemplateRegistry()
	templates := []contracts.ApprovedTemplate{
		{
			SpeechKey:       "destination_options",
			Language:        "en-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "Destination choices are displayed on screen.",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "destination_options",
			Language:        "hi-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "गंतव्य विकल्प स्क्रीन पर प्रदर्शित हैं।",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "destination_options",
			Language:        "ml-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "ലക്ഷ്യസ്ഥാന ഓപ്ഷനുകൾ സ്ക്രീനിൽ കാണിച്ചിരിക്കുന്നു.",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "clarify_place",
			Language:        "en-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "Please clarify the location.",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "clarify_place",
			Language:        "hi-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "कृपया स्थान स्पष्ट करें।",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "clarify_place",
			Language:        "ml-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "ദയവായി സ്ഥലം വ്യക്തമാക്കുക.",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "verified_route_unavailable",
			Language:        "en-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "Verified route is currently unavailable.",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "verified_route_unavailable",
			Language:        "hi-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "सत्यापित मार्ग वर्तमान में अनुपलब्ध है।",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "verified_route_unavailable",
			Language:        "ml-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "സ്ഥിരീകരിച്ച റൂട്ട് നിലവിൽ ലഭ്യമല്ല.",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "welcome",
			Language:        "en-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "Welcome to Sthira emergency guidance.",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "welcome",
			Language:        "hi-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "स्थिरा आपातकालीन मार्गदर्शन में आपका स्वागत है।",
			SyntheticOnly:   false,
		},
		{
			SpeechKey:       "welcome",
			Language:        "ml-IN",
			TemplateVersion: 1,
			SourceVersion:   1,
			Text:            "സ്ഥിര അടിയന്തര മാർഗ്ഗനിർദ്ദേശത്തിലേക്ക് സ്വാഗതം.",
			SyntheticOnly:   false,
		},
	}
	for _, t := range templates {
		r.Add(t)
	}
	return r
}
