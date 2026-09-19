package catalogue

import (
	"strings"
	"testing"
)

func validManifest() *Manifest {
	return &Manifest{
		CatalogueVersion: 1,
		HistoricalEvents: []HistoricalEvent{
			{EventID: "EV-KL-2018", State: "KL", Kind: "flood", Occurred: "2018-08", SourceRef: "user:ksdma-2018-report"},
		},
		States: []StateEntry{
			{
				StateCode: "KL",
				Name:      "Kerala",
				Languages: []LanguageSupport{
					{Language: "ml-IN"},
					{Language: "en-IN"},
				},
				Scenarios: []Scenario{
					{
						ScenarioID:        "SC-KL-2018-FLOOD",
						State:             "KL",
						Evidence:          EvidenceHistorical,
						HistoricalEventID: "EV-KL-2018",
						ExerciseTime:      "2026-09-12T04:00:00Z",
					},
					{
						ScenarioID:   "SC-KL-SYNTH-1",
						State:        "KL",
						Evidence:     EvidenceSynthetic,
						ExerciseTime: "2026-09-12T04:00:00Z",
					},
				},
			},
		},
	}
}

func TestValidateValidManifest(t *testing.T) {
	v, err := Validate(validManifest())
	if err != nil {
		t.Fatal(err)
	}
	if !v.Acceptable() {
		t.Errorf("gaps = %+v", v.Gaps)
	}
	if v.ScenarioCount != 2 || v.HistoricalCount != 1 {
		t.Errorf("counts = %+v", v)
	}
	if len(v.StateCodes) != 1 || v.StateCodes[0] != "KL" {
		t.Errorf("states = %v", v.StateCodes)
	}
}

func TestValidateEmptyManifestIsGapNotError(t *testing.T) {
	v, err := Validate(&Manifest{CatalogueVersion: 1})
	if err != nil {
		t.Fatalf("empty manifest should validate structurally, got %v", err)
	}
	if v.Acceptable() {
		t.Error("empty manifest must not be acceptable")
	}
	// Missing states is a gap, not a structural error.
	found := false
	for _, g := range v.Gaps {
		if g.Kind == "MISSING_STATE" {
			found = true
		}
	}
	if !found {
		t.Errorf("gaps = %+v, want MISSING_STATE", v.Gaps)
	}
}

func TestValidateHistoricalScenarioRequiresEventRef(t *testing.T) {
	m := validManifest()
	m.States[0].Scenarios[0].HistoricalEventID = ""
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "historical_event_id") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateHistoricalScenarioRejectsUnknownEvent(t *testing.T) {
	m := validManifest()
	m.States[0].Scenarios[0].HistoricalEventID = "EV-NOPE"
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "unknown historical event") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateSyntheticMustNotReferenceEvent(t *testing.T) {
	m := validManifest()
	m.States[0].Scenarios[1].HistoricalEventID = "EV-KL-2018"
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "must not reference") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateScenarioStateMustMatchOwner(t *testing.T) {
	m := validManifest()
	m.States[0].Scenarios[0].State = "TN"
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "does not match owning state") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateDuplicateStateRejected(t *testing.T) {
	m := validManifest()
	m.States = append(m.States, m.States[0])
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "duplicate state") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateDuplicateEventRejected(t *testing.T) {
	m := validManifest()
	m.HistoricalEvents = append(m.HistoricalEvents, m.HistoricalEvents[0])
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "duplicate historical event") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateHistoricalEventRequiresSourceRef(t *testing.T) {
	m := validManifest()
	m.HistoricalEvents[0].SourceRef = ""
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "source_ref") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateInvalidEvidenceKind(t *testing.T) {
	m := validManifest()
	m.States[0].Scenarios[0].Evidence = "GUESSED"
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "invalid evidence kind") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateInvalidExerciseTime(t *testing.T) {
	m := validManifest()
	m.States[0].Scenarios[0].ExerciseTime = "not-a-time"
	_, err := Validate(m)
	if err == nil || !strings.Contains(err.Error(), "RFC 3339") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateStateWithNoLanguagesIsGap(t *testing.T) {
	m := validManifest()
	m.States[0].Languages = nil
	v, err := Validate(m)
	if err != nil {
		t.Fatal(err)
	}
	if v.Acceptable() {
		t.Error("state with no languages must not be acceptable")
	}
}
