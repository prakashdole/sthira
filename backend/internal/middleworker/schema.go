package middleworker

import (
	_ "embed"
	"encoding/json"
	"errors"
	"sort"
)

//go:embed schema/model_output.schema.json
var defaultModelOutputSchema []byte

// DefaultModelOutputSchema returns the pinned JSON Schema for guided generation.
func DefaultModelOutputSchema() []byte {
	return append([]byte(nil), defaultModelOutputSchema...)
}

// Narrow generation to the same server snapshot checked independently after
// inference. This constrains output; it never repairs or invents model results.
func scopedModelOutputSchema(req RequestEnvelope) ([]byte, error) {
	var schema map[string]any
	if err := json.Unmarshal(defaultModelOutputSchema, &schema); err != nil {
		return nil, err
	}
	p, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, errors.New("schema missing properties object")
	}
	p["request_id"] = map[string]any{"const": req.RequestID}
	p["data_version"] = map[string]any{"const": req.ScopedContext.DataVersion}
	ids := map[string]bool{}
	for id := range req.ScopedContext.KnownPlaces {
		ids[id] = true
	}
	for id := range req.ScopedContext.KnownSafeZones {
		ids[id] = true
	}
	for id := range req.ScopedContext.KnownRedZones {
		ids[id] = true
	}
	for id := range req.ScopedContext.KnownFacilities {
		ids[id] = true
	}
	for id := range req.ScopedContext.KnownRoutes {
		ids[id] = true
	}
	known := []string{}
	for id := range ids {
		known = append(known, id)
	}
	sort.Strings(known)
	evidence, ok := p["evidence_ids"].(map[string]any)
	if !ok {
		return nil, errors.New("schema missing evidence_ids property")
	}
	if len(known) == 0 {
		evidence["maxItems"] = 0
	} else {
		evidence["items"] = map[string]any{"enum": known}
	}
	keys := []any{nil}
	for _, key := range req.ScopedContext.TemplateKeys {
		keys = append(keys, key)
	}
	p["speech_key"] = map[string]any{"enum": keys}
	choices := []string{}
	for _, choice := range req.ScopedContext.EligibleDestinations {
		choices = append(choices, choice.Facility.FacilityID)
	}
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		return nil, errors.New("schema missing $defs")
	}
	actionDef, ok := defs["action"].(map[string]any)
	if !ok {
		return nil, errors.New("schema missing action def")
	}
	oneOf, ok := actionDef["oneOf"].([]any)
	if !ok {
		return nil, errors.New("schema missing action oneOf")
	}
	for _, raw := range oneOf {
		variant, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		props, ok := variant["properties"].(map[string]any)
		if !ok {
			continue
		}
		typeProp, ok := props["type"].(map[string]any)
		if !ok {
			continue
		}
		actionType, _ := typeProp["const"].(string)
		if actionType == "SHOW_CHOICES" {
			if len(choices) == 0 {
				props["target_ids"] = false
			} else if len(choices) <= 3 {
				props["target_ids"] = map[string]any{"const": choices}
			} else {
				prefixes := make([]any, 0, 3)
				for k := 1; k <= 3; k++ {
					prefixes = append(prefixes, choices[:k])
				}
				props["target_ids"] = map[string]any{"enum": prefixes}
			}
		} else if actionType == "FIT_FEATURES" {
			if len(known) == 0 {
				props["target_ids"] = false
			} else {
				props["target_ids"] = map[string]any{
					"type":        "array",
					"items":       map[string]any{"enum": known},
					"minItems":    1,
					"maxItems":    3,
					"uniqueItems": true,
				}
			}
		}
	}
	return json.Marshal(schema)
}
