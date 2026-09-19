package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func known() map[string]bool {
	return map[string]bool{
		"PLACE-DEMO-1": true, "PLACE-DEMO-2": true,
		"FACILITY-DEMO-1": true, "FACILITY-DEMO-2": true,
	}
}

func langs() map[string]bool {
	return map[string]bool{"en-IN": true, "hi-IN": true, "ml-IN": true}
}

func loadModel(t *testing.T, name string) ModelOutput {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../testdata/model", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var out ModelOutput
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("golden fixture %s is not valid JSON: %v", name, err)
	}
	return out
}

// The golden examples must validate against a matching request/snapshot.
func TestGoldenExamplesValidate(t *testing.T) {
	cases := []struct {
		fixture   string
		requestID string
	}{
		{"silent_focus.json", "REQ-DEMO-1"},
		{"list_destinations.json", "REQ-DEMO-2"},
		{"clarify_place.json", "REQ-DEMO-3"},
		{"route_unavailable.json", "REQ-DEMO-4"},
	}
	for _, c := range cases {
		out := loadModel(t, c.fixture)
		if err := ValidateModelOutput(out, c.requestID, "EXERCISE-7", known(), langs()); err != nil {
			t.Errorf("%s: golden example rejected: %v", c.fixture, err)
		}
	}
}

func TestRejectsUnknownTargetID(t *testing.T) {
	out := loadModel(t, "invalid_unknown_id.json")
	err := ValidateModelOutput(out, "REQ-DEMO-1", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "not in active context") {
		t.Fatalf("expected unknown-ID rejection, got %v", err)
	}
}

func TestRejectsNonOKWithActions(t *testing.T) {
	out := loadModel(t, "invalid_nonok_with_actions.json")
	err := ValidateModelOutput(out, "REQ-DEMO-4", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "no actions") {
		t.Fatalf("expected non-OK-with-actions rejection, got %v", err)
	}
}

func TestRejectsFabricatedLanguage(t *testing.T) {
	out := loadModel(t, "invalid_fabricated_language.json")
	err := ValidateModelOutput(out, "REQ-DEMO-1", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "not an enabled context language") {
		t.Fatalf("expected language rejection, got %v", err)
	}
}

func TestRejectsProhibitedActionType(t *testing.T) {
	out := loadModel(t, "invalid_prohibited_action.json")
	err := ValidateModelOutput(out, "REQ-DEMO-1", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "unknown action type") {
		t.Fatalf("expected prohibited-action rejection, got %v", err)
	}
}

func TestRejectsStaleDataVersion(t *testing.T) {
	out := loadModel(t, "silent_focus.json")
	err := ValidateModelOutput(out, "REQ-DEMO-1", "EXERCISE-8", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "data_version") {
		t.Fatalf("expected stale-version rejection, got %v", err)
	}
}

func TestRejectsWrongRequestID(t *testing.T) {
	out := loadModel(t, "silent_focus.json")
	err := ValidateModelOutput(out, "REQ-OTHER", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "request_id") {
		t.Fatalf("expected request_id rejection, got %v", err)
	}
}

func TestRejectsTooManyActions(t *testing.T) {
	out := loadModel(t, "silent_focus.json")
	out.Actions = []Action{
		{Type: ActionRecenter}, {Type: ActionRecenter}, {Type: ActionRecenter},
		{Type: ActionRecenter}, {Type: ActionRecenter}, {Type: ActionRecenter},
	}
	err := ValidateModelOutput(out, "REQ-DEMO-1", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "exceed max") {
		t.Fatalf("expected action-count rejection, got %v", err)
	}
}

func TestRejectsOKWithoutIntent(t *testing.T) {
	out := loadModel(t, "silent_focus.json")
	out.Intent = nil
	err := ValidateModelOutput(out, "REQ-DEMO-1", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "requires an intent") {
		t.Fatalf("expected missing-intent rejection, got %v", err)
	}
}

func TestZoomRequiresExactlyOneStep(t *testing.T) {
	out := loadModel(t, "silent_focus.json")
	zoom := IntentZoom
	out.Intent = &zoom
	out.Actions = []Action{{Type: ActionZoom, Direction: "IN", Steps: 3}}
	err := ValidateModelOutput(out, "REQ-DEMO-1", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "steps must be exactly 1") {
		t.Fatalf("expected steps rejection, got %v", err)
	}
}

func TestClarifyRequiresCandidateIDs(t *testing.T) {
	out := loadModel(t, "clarify_place.json")
	out.ClarificationIDs = nil
	err := ValidateModelOutput(out, "REQ-DEMO-3", "EXERCISE-7", known(), langs())
	if err == nil || !strings.Contains(err.Error(), "CLARIFY requires") {
		t.Fatalf("expected clarify rejection, got %v", err)
	}
}
