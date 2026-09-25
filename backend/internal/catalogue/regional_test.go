package catalogue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegionalDrillCatalogue(t *testing.T) {
	path := filepath.Join("..", "..", "..", "plan", "drills", "catalogue.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read catalogue manifest: %v", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal catalogue manifest: %v", err)
	}

	v, err := Validate(&m)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if !v.Acceptable() {
		t.Fatalf("catalogue has unexpected gaps: %+v", v.Gaps)
	}

	launch := v.AssessLaunch()
	for _, g := range launch.Gaps {
		if g.Kind == "INSUFFICIENT_STATES" || g.Kind == "EXCESS_STATES" || g.Kind == "INSUFFICIENT_SCENARIOS" {
			t.Errorf("launch gap encountered: %s: %s", g.Kind, g.Detail)
		}
	}

	if len(v.StateCodes) < MinLaunchStates || len(v.StateCodes) > MaxLaunchStates {
		t.Errorf("expected between %d and %d states, got %d", MinLaunchStates, MaxLaunchStates, len(v.StateCodes))
	}

	for _, code := range v.StateCodes {
		scCount := v.ScenariosByState[code]
		if scCount < MinScenariosPerState || scCount > MaxScenariosPerState {
			t.Errorf("state %s has %d scenarios, want %d-%d", code, scCount, MinScenariosPerState, MaxScenariosPerState)
		}
	}

	if v.HistoricalCount < 20 {
		t.Errorf("expected at least 20 historical events for 10 states, got %d", v.HistoricalCount)
	}
}
