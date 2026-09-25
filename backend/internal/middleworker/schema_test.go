package middleworker

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestScopedModelOutputSchema_EmptyContext(t *testing.T) {
	req := RequestEnvelope{
		RequestID: "REQ-EMPTY",
		ScopedContext: ScopedContext{
			DataVersion: "v-empty",
		},
	}
	schemaBytes, err := scopedModelOutputSchema(req)
	if err != nil {
		t.Fatalf("scopedModelOutputSchema error: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal generated schema: %v", err)
	}

	p := schema["properties"].(map[string]any)
	reqID := p["request_id"].(map[string]any)
	if reqID["const"] != "REQ-EMPTY" {
		t.Errorf("request_id const = %v, want REQ-EMPTY", reqID["const"])
	}
	dataVer := p["data_version"].(map[string]any)
	if dataVer["const"] != "v-empty" {
		t.Errorf("data_version const = %v, want v-empty", dataVer["const"])
	}

	// With no known features, evidence_ids should have maxItems = 0
	evidence := p["evidence_ids"].(map[string]any)
	if evidence["maxItems"] != float64(0) {
		t.Errorf("evidence_ids maxItems = %v, want 0", evidence["maxItems"])
	}

	// With no templates, speech_key enum should be [nil]
	speechKey := p["speech_key"].(map[string]any)
	keys := speechKey["enum"].([]any)
	if len(keys) != 1 || keys[0] != nil {
		t.Errorf("speech_key enum = %v, want [nil]", keys)
	}

	// With no eligible facilities, SHOW_CHOICES and FIT_FEATURES target_ids should be false
	defs := schema["$defs"].(map[string]any)
	actionDef := defs["action"].(map[string]any)
	oneOf := actionDef["oneOf"].([]any)
	for _, raw := range oneOf {
		variant := raw.(map[string]any)
		props := variant["properties"].(map[string]any)
		typeProp := props["type"].(map[string]any)
		actionType := typeProp["const"].(string)
		if actionType == "SHOW_CHOICES" || actionType == "FIT_FEATURES" {
			if props["target_ids"] != false {
				t.Errorf("%s target_ids = %v, want false when no choices", actionType, props["target_ids"])
			}
		}
	}
}

func TestScopedModelOutputSchema_WithChoicesUnderLimit(t *testing.T) {
	req := RequestEnvelope{
		RequestID: "REQ-CHOICES",
		ScopedContext: ScopedContext{
			DataVersion:  "v1",
			TemplateKeys: []string{"welcome_msg", "evac_instructions"},
			KnownPlaces: map[string]PlaceCandidate{
				"PLACE-1": {PlaceID: "PLACE-1"},
			},
			KnownFacilities: map[string]FacilityRef{
				"FAC-1": {FacilityID: "FAC-1"},
				"FAC-2": {FacilityID: "FAC-2"},
			},
			EligibleDestinations: []EligibleChoice{
				{Facility: FacilityRef{FacilityID: "FAC-1"}},
				{Facility: FacilityRef{FacilityID: "FAC-2"}},
			},
		},
	}
	schemaBytes, err := scopedModelOutputSchema(req)
	if err != nil {
		t.Fatalf("scopedModelOutputSchema error: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal generated schema: %v", err)
	}

	p := schema["properties"].(map[string]any)
	evidence := p["evidence_ids"].(map[string]any)
	items := evidence["items"].(map[string]any)
	enumItems := items["enum"].([]any)
	wantKnown := []any{"FAC-1", "FAC-2", "PLACE-1"}
	if !reflect.DeepEqual(enumItems, wantKnown) {
		t.Errorf("evidence_ids enum = %v, want %v", enumItems, wantKnown)
	}

	speechKey := p["speech_key"].(map[string]any)
	keys := speechKey["enum"].([]any)
	wantKeys := []any{nil, "welcome_msg", "evac_instructions"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Errorf("speech_key enum = %v, want %v", keys, wantKeys)
	}

	defs := schema["$defs"].(map[string]any)
	actionDef := defs["action"].(map[string]any)
	oneOf := actionDef["oneOf"].([]any)
	for _, raw := range oneOf {
		variant := raw.(map[string]any)
		props := variant["properties"].(map[string]any)
		typeProp := props["type"].(map[string]any)
		actionType := typeProp["const"].(string)
		if actionType == "SHOW_CHOICES" {
			targetIDs, ok := props["target_ids"].(map[string]any)
			if !ok {
				t.Fatalf("SHOW_CHOICES target_ids not a map: %v", props["target_ids"])
			}
			constVal, ok := targetIDs["const"].([]any)
			if !ok {
				t.Fatalf("SHOW_CHOICES target_ids const not a slice: %v", targetIDs)
			}
			wantConst := []any{"FAC-1", "FAC-2"}
			if !reflect.DeepEqual(constVal, wantConst) {
				t.Errorf("SHOW_CHOICES target_ids const = %v, want %v", constVal, wantConst)
			}
		} else if actionType == "FIT_FEATURES" {
			targetIDs, ok := props["target_ids"].(map[string]any)
			if !ok {
				t.Fatalf("FIT_FEATURES target_ids not a map: %v", props["target_ids"])
			}
			items, ok := targetIDs["items"].(map[string]any)
			if !ok {
				t.Fatalf("FIT_FEATURES target_ids items not a map: %v", targetIDs)
			}
			enumVal, ok := items["enum"].([]any)
			if !ok {
				t.Fatalf("FIT_FEATURES items enum not a slice: %v", items)
			}
			wantEnum := []any{"FAC-1", "FAC-2", "PLACE-1"}
			if !reflect.DeepEqual(enumVal, wantEnum) {
				t.Errorf("FIT_FEATURES enum = %v, want %v", enumVal, wantEnum)
			}
			if targetIDs["minItems"] != float64(1) || targetIDs["maxItems"] != float64(3) || targetIDs["uniqueItems"] != true {
				t.Errorf("FIT_FEATURES bounds/unique mismatch: min=%v, max=%v, unique=%v", targetIDs["minItems"], targetIDs["maxItems"], targetIDs["uniqueItems"])
			}
		}
	}
}

func TestScopedModelOutputSchema_WithChoicesOverLimit(t *testing.T) {
	req := RequestEnvelope{
		RequestID: "REQ-OVERLIMIT",
		ScopedContext: ScopedContext{
			DataVersion: "v1",
			EligibleDestinations: []EligibleChoice{
				{Facility: FacilityRef{FacilityID: "FAC-1"}},
				{Facility: FacilityRef{FacilityID: "FAC-2"}},
				{Facility: FacilityRef{FacilityID: "FAC-3"}},
				{Facility: FacilityRef{FacilityID: "FAC-4"}},
				{Facility: FacilityRef{FacilityID: "FAC-5"}},
			},
		},
	}
	schemaBytes, err := scopedModelOutputSchema(req)
	if err != nil {
		t.Fatalf("scopedModelOutputSchema error: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal generated schema: %v", err)
	}

	defs := schema["$defs"].(map[string]any)
	actionDef := defs["action"].(map[string]any)
	oneOf := actionDef["oneOf"].([]any)
	for _, raw := range oneOf {
		variant := raw.(map[string]any)
		props := variant["properties"].(map[string]any)
		typeProp := props["type"].(map[string]any)
		actionType := typeProp["const"].(string)
		if actionType == "SHOW_CHOICES" {
			targetIDs, ok := props["target_ids"].(map[string]any)
			if !ok {
				t.Fatalf("SHOW_CHOICES target_ids not a map: %v", props["target_ids"])
			}
			enumVal, ok := targetIDs["enum"].([]any)
			if !ok {
				t.Fatalf("SHOW_CHOICES target_ids enum not a slice: %v", targetIDs)
			}
			if len(enumVal) != 3 {
				t.Fatalf("SHOW_CHOICES target_ids enum length = %d, want 3 prefixes", len(enumVal))
			}
			// Each prefix must have length <= 3
			for i, pfx := range enumVal {
				pSlice := pfx.([]any)
				if len(pSlice) != i+1 {
					t.Errorf("prefix %d len = %d, want %d", i, len(pSlice), i+1)
				}
			}
		} else if actionType == "FIT_FEATURES" {
			if props["target_ids"] != false {
				t.Errorf("FIT_FEATURES target_ids = %v, want false when known is empty", props["target_ids"])
			}
		}
	}
}

func TestDefaultModelOutputSchema_HasAllActionVariants(t *testing.T) {
	schemaBytes := DefaultModelOutputSchema()
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("DefaultModelOutputSchema invalid JSON: %v", err)
	}
	defs := schema["$defs"].(map[string]any)
	actionDef := defs["action"].(map[string]any)
	oneOf := actionDef["oneOf"].([]any)

	actionTypes := map[string]bool{}
	for _, raw := range oneOf {
		variant := raw.(map[string]any)
		props := variant["properties"].(map[string]any)
		typeProp := props["type"].(map[string]any)
		actionTypes[typeProp["const"].(string)] = true
	}

	expectedTypes := []string{
		"FOCUS_FEATURE",
		"HIGHLIGHT_FEATURE",
		"SHOW_CHOICES",
		"FIT_FEATURES",
		"SHOW_ROUTE",
		"OPEN_PANEL",
		"ZOOM",
		"PAN",
		"RECENTER",
		"SET_LANGUAGE",
		"SET_LAYER_VISIBILITY",
	}

	for _, exp := range expectedTypes {
		if !actionTypes[exp] {
			t.Errorf("schema missing action variant %s", exp)
		}
	}
}

func TestScopedModelOutputSchema_FitFeaturesWithoutDestinations(t *testing.T) {
	req := RequestEnvelope{
		RequestID: "REQ-NODEST",
		ScopedContext: ScopedContext{
			DataVersion: "v1",
			KnownSafeZones: map[string]ZoneRef{
				"SZ-1": {ZoneID: "SZ-1"},
			},
			KnownRedZones: map[string]ZoneRef{
				"RZ-1": {ZoneID: "RZ-1"},
			},
			KnownRoutes: map[string]RouteRef{
				"ROUTE-1": {RouteID: "ROUTE-1"},
			},
			// EligibleDestinations is EMPTY!
			EligibleDestinations: nil,
		},
	}
	schemaBytes, err := scopedModelOutputSchema(req)
	if err != nil {
		t.Fatalf("scopedModelOutputSchema error: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal generated schema: %v", err)
	}

	defs := schema["$defs"].(map[string]any)
	actionDef := defs["action"].(map[string]any)
	oneOf := actionDef["oneOf"].([]any)
	for _, raw := range oneOf {
		variant := raw.(map[string]any)
		props := variant["properties"].(map[string]any)
		typeProp := props["type"].(map[string]any)
		actionType := typeProp["const"].(string)
		if actionType == "SHOW_CHOICES" {
			if props["target_ids"] != false {
				t.Errorf("SHOW_CHOICES target_ids = %v, want false when no eligible destinations", props["target_ids"])
			}
		} else if actionType == "FIT_FEATURES" {
			targetIDs, ok := props["target_ids"].(map[string]any)
			if !ok {
				t.Fatalf("FIT_FEATURES target_ids not a map: %v", props["target_ids"])
			}
			items, ok := targetIDs["items"].(map[string]any)
			if !ok {
				t.Fatalf("FIT_FEATURES target_ids items not a map: %v", targetIDs)
			}
			enumVal, ok := items["enum"].([]any)
			if !ok {
				t.Fatalf("FIT_FEATURES items enum not a slice: %v", items)
			}
			wantEnum := []any{"ROUTE-1", "RZ-1", "SZ-1"}
			if !reflect.DeepEqual(enumVal, wantEnum) {
				t.Errorf("FIT_FEATURES enum = %v, want %v", enumVal, wantEnum)
			}
			if targetIDs["minItems"] != float64(1) || targetIDs["maxItems"] != float64(3) || targetIDs["uniqueItems"] != true {
				t.Errorf("FIT_FEATURES bounds/unique mismatch: min=%v, max=%v, unique=%v", targetIDs["minItems"], targetIDs["maxItems"], targetIDs["uniqueItems"])
			}
		}
	}
}
