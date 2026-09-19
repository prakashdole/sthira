// Package catalogue owns the canonical scenario catalogue: the manifest of
// states, languages, historical exercise events, and synthetic scenarios that
// the product may present. Ownership lives here, outside frontend source; the
// existing frontend/v2/src/scenario.json remains a consumer until deliberately
// migrated.
//
// Trust posture: the catalogue separates historical event evidence (real past
// incidents supplied by the user/authority) from synthetic geometry and
// exercise time. It validates supplied records and reports what is missing; it
// never invents zones, routes, approvals, or historical evidence to fill gaps.
// A record that fails validation is rejected, not defaulted.
package catalogue

import (
	"fmt"
	"sort"
	"time"
)

// CatalogueError is a validation failure for a catalogue record.
type CatalogueError struct{ Reason string }

func (e *CatalogueError) Error() string { return e.Reason }

func fail(format string, args ...any) *CatalogueError {
	return &CatalogueError{Reason: fmt.Sprintf(format, args...)}
}

// EvidenceKind separates real historical evidence from synthetic exercise data.
type EvidenceKind string

const (
	// EvidenceHistorical is a real past event supplied by the user/authority.
	EvidenceHistorical EvidenceKind = "HISTORICAL_EVENT"
	// EvidenceSynthetic is fabricated exercise geometry/time, never real.
	EvidenceSynthetic EvidenceKind = "SYNTHETIC_EXERCISE"
)

// LanguageSupport records whether a language is supported for a state, with the
// evidence for that claim. Support is never asserted without evidence.
type LanguageSupport struct {
	Language string `json:"language"` // BCP-47-ish code, e.g. "ml-IN"
	// ASR/TTS support is per-model and gated by P6 evidence; absent means unknown.
	ASRSupported bool `json:"asr_supported"`
	TTSSupported bool `json:"tts_supported"`
}

// HistoricalEvent is one real past disaster event used as an exercise basis.
// It carries evidence provenance; the catalogue does not fabricate it.
type HistoricalEvent struct {
	EventID   string `json:"event_id"`
	State     string `json:"state"`      // state code this event belongs to
	Kind      string `json:"kind"`       // e.g. "flood", "landslide"
	Occurred  string `json:"occurred"`   // ISO date or year of the real event
	SourceRef string `json:"source_ref"` // authority/user-supplied reference
}

// Scenario is one exercise scenario for a state. Geometry and exercise time are
// synthetic unless backed by a HistoricalEvent reference.
type Scenario struct {
	ScenarioID string `json:"scenario_id"`
	State      string `json:"state"`
	// EvidenceKind marks whether this is a historical-event exercise or purely
	// synthetic. Historical scenarios must reference a known HistoricalEvent.
	Evidence EvidenceKind `json:"evidence"`
	// HistoricalEventID links a historical scenario to its evidence record.
	// Required when Evidence is EvidenceHistorical; must be empty for synthetic.
	HistoricalEventID string `json:"historical_event_id,omitempty"`
	// ExerciseTime is the injected exercise clock instant (not a real alert time).
	ExerciseTime string `json:"exercise_time"`
}

// StateEntry is one state's catalogue record.
type StateEntry struct {
	StateCode string            `json:"state_code"` // e.g. "KL"
	Name      string            `json:"name"`
	Languages []LanguageSupport `json:"languages"`
	Scenarios []Scenario        `json:"scenarios"`
}

// Manifest is the canonical catalogue: states, their languages, scenarios, and
// the historical evidence records those scenarios may reference.
type Manifest struct {
	// CatalogueVersion versions the manifest itself.
	CatalogueVersion int               `json:"catalogue_version"`
	States           []StateEntry      `json:"states"`
	HistoricalEvents []HistoricalEvent `json:"historical_events"`
}

// Gap reports a missing or blocked catalogue item without inventing it.
type Gap struct {
	Kind   string // "MISSING_STATE", "MISSING_LANGUAGE_EVIDENCE", "MISSING_HISTORICAL_EVENT", ...
	Detail string
}

// Validation is the result of validating a manifest.
type Validation struct {
	// StateCodes are the validated state codes, sorted.
	StateCodes []string
	// ScenarioCount is the number of validated scenarios.
	ScenarioCount int
	// HistoricalCount is the number of validated historical events.
	HistoricalCount int
	// Gaps lists missing/blocked items the catalogue cannot satisfy. A non-empty
	// Gaps means acceptance is blocked even though structure validated.
	Gaps []Gap
}

// Validate checks a manifest's structure and cross-references. It returns a
// Validation describing what is present and what is missing. A structural
// failure returns a *CatalogueError; missing user data is reported as a Gap,
// not an error, so the caller can distinguish "invalid" from "incomplete".
func Validate(m *Manifest) (*Validation, error) {
	if m == nil {
		return nil, fail("manifest is required")
	}
	if m.CatalogueVersion < 1 {
		return nil, fail("catalogue_version must be a positive integer")
	}

	v := &Validation{}
	eventIndex := map[string]HistoricalEvent{}
	for _, ev := range m.HistoricalEvents {
		if ev.EventID == "" || ev.State == "" || ev.Kind == "" || ev.SourceRef == "" {
			return nil, fail("historical event requires event_id, state, kind and source_ref")
		}
		if _, dup := eventIndex[ev.EventID]; dup {
			return nil, fail("duplicate historical event id %q", ev.EventID)
		}
		eventIndex[ev.EventID] = ev
	}
	v.HistoricalCount = len(m.HistoricalEvents)

	stateSeen := map[string]bool{}
	for _, st := range m.States {
		if st.StateCode == "" || st.Name == "" {
			return nil, fail("state requires state_code and name")
		}
		if stateSeen[st.StateCode] {
			return nil, fail("duplicate state code %q", st.StateCode)
		}
		stateSeen[st.StateCode] = true
		v.StateCodes = append(v.StateCodes, st.StateCode)

		if len(st.Languages) == 0 {
			v.Gaps = append(v.Gaps, Gap{Kind: "MISSING_LANGUAGE_EVIDENCE", Detail: "state " + st.StateCode + " declares no languages"})
		}
		for _, lang := range st.Languages {
			if lang.Language == "" {
				return nil, fail("state %q has a language with an empty code", st.StateCode)
			}
		}

		for _, sc := range st.Scenarios {
			if sc.ScenarioID == "" {
				return nil, fail("state %q has a scenario with an empty scenario_id", st.StateCode)
			}
			if sc.State != st.StateCode {
				return nil, fail("scenario %q state %q does not match owning state %q", sc.ScenarioID, sc.State, st.StateCode)
			}
			switch sc.Evidence {
			case EvidenceHistorical:
				if sc.HistoricalEventID == "" {
					return nil, fail("historical scenario %q must reference a historical_event_id", sc.ScenarioID)
				}
				if _, ok := eventIndex[sc.HistoricalEventID]; !ok {
					return nil, fail("historical scenario %q references unknown historical event %q", sc.ScenarioID, sc.HistoricalEventID)
				}
			case EvidenceSynthetic:
				if sc.HistoricalEventID != "" {
					return nil, fail("synthetic scenario %q must not reference a historical event", sc.ScenarioID)
				}
			default:
				return nil, fail("scenario %q has invalid evidence kind %q", sc.ScenarioID, sc.Evidence)
			}
			if _, err := time.Parse(time.RFC3339, sc.ExerciseTime); err != nil {
				return nil, fail("scenario %q exercise_time must be RFC 3339", sc.ScenarioID)
			}
			v.ScenarioCount++
		}
	}
	sort.Strings(v.StateCodes)

	if len(m.States) == 0 {
		v.Gaps = append(v.Gaps, Gap{Kind: "MISSING_STATE", Detail: "no states supplied"})
	}
	return v, nil
}

// Acceptable reports whether the catalogue is complete enough to accept: it
// validated structurally and has no blocking gaps.
func (v *Validation) Acceptable() bool {
	return v != nil && len(v.Gaps) == 0
}
