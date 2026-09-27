// corpus.go — offline helpers for the synthetic JSONL evaluation
// corpus. NO model calls, NO network. The loader is intentionally
// permissive on JSON shape (it does not DisallowUnknownFields): the
// corpus is our own artifact, so we want it readable by future
// schema bumps without breaking the offline check. Wire-shape
// strictness lives in the decoder (decodeStrictProposal), which the
// oracle tests exercise via StubRuntime fixtures.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// CaseV2 is the JSONL schema for the new corpus. It is a superset
// of the v1 fields; missing v2 fields default to empty so the
// loader can also re-read older cases without rewriting them.
type CaseV2 struct {
	ID                  string       `json:"id"`
	Language            string       `json:"language"`
	Scenario            string       `json:"scenario"`
	Transcript          string       `json:"transcript"`
	ReviewStatus        string       `json:"review_status"`
	ExpectedStatus      string       `json:"expected_status"`
	ExpectedIntent      *string      `json:"expected_intent"`
	ExpectedActions     []ActionSpec `json:"expected_actions"`
	ExpectedSpeechKey   *string      `json:"expected_speech_key"`
	ExpectedClarifyIDs  []string     `json:"expected_clarification_ids"`
	ExpectedEvidenceIDs []string     `json:"expected_evidence_ids"`
	ExpectedRequestID   string       `json:"expected_request_id"`
	ExpectedDataVersion string       `json:"expected_data_version"`
	Reason              string       `json:"reason"`
	Notes               string       `json:"notes,omitempty"`
}

// ActionSpec is the per-action expected shape. It mirrors the wire
// Action variant set in middleworker/wire.go so that synthesizing a
// fixture from a CaseV2 is a straight pass through.
type ActionSpec struct {
	Type      string   `json:"type"`
	TargetID  string   `json:"target_id,omitempty"`
	TargetIDs []string `json:"target_ids,omitempty"`
	RouteID   string   `json:"route_id,omitempty"`
	Panel     string   `json:"panel,omitempty"`
	Direction string   `json:"direction,omitempty"`
	Steps     int      `json:"steps,omitempty"`
	Language  string   `json:"language,omitempty"`
	Layer     string   `json:"layer,omitempty"`
	Visible   *bool    `json:"visible,omitempty"`
}

// loadCorpusJSONL reads one or more JSONL files and returns the
// parsed cases in input order. Blank lines are skipped. A missing
// field is non-fatal: the loader only fails on bad JSON, missing
// id, or unreadable file.
func loadCorpusJSONL(paths ...string) ([]CaseV2, error) {
	var out []CaseV2
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", p, err)
		}
		scanner := bufio.NewScanner(f)
		// Allow long lines: a single case with many actions can
		// exceed the default 64 KiB scanner buffer.
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		line := 0
		for scanner.Scan() {
			line++
			raw := strings.TrimSpace(scanner.Text())
			if raw == "" {
				continue
			}
			var c CaseV2
			if err := json.Unmarshal([]byte(raw), &c); err != nil {
				f.Close()
				return nil, fmt.Errorf("parse %s line %d: %w", p, line, err)
			}
			if c.ID == "" {
				f.Close()
				return nil, fmt.Errorf("missing id in %s line %d", p, line)
			}
			out = append(out, c)
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return nil, fmt.Errorf("scan %s: %w", p, err)
		}
		f.Close()
	}
	return out, nil
}

// uniqueCaseIDs returns an error listing every id that appears
// more than once across the input slice. Used to enforce no
// duplicates within or across corpus files.
func uniqueCaseIDs(cases []CaseV2) error {
	seen := map[string]int{}
	for _, c := range cases {
		seen[c.ID]++
	}
	var dupes []string
	for id, n := range seen {
		if n > 1 {
			dupes = append(dupes, fmt.Sprintf("%s x%d", id, n))
		}
	}
	if len(dupes) > 0 {
		sort.Strings(dupes)
		return fmt.Errorf("duplicate ids: %s", strings.Join(dupes, ", "))
	}
	return nil
}

// allowedFixtureIDs is the closed set of IDs the corpus may
// reference. The list mirrors the demo fixture used by the existing
// testdata golden cases and the scoped-validator tests; adding a
// new ID here requires also widening the production validator's
// allowed set, which is intentionally out of lane.
func allowedFixtureIDs() map[string]struct{} {
	return map[string]struct{}{
		// places (from synthetic.jsonl)
		"PLACE-DEMO-1": {},
		"PLACE-DEMO-2": {},
		// facilities
		"FACILITY-DEMO-1": {},
		"FACILITY-DEMO-2": {},
		// safe / red / route (from scoped_test.go and schema_test.go)
		"SZ-1":    {},
		"RZ-1":    {},
		"ROUTE-1": {},
		"FAC-1":   {},
		"FAC-2":   {},
		"FAC-3":   {},
		"FAC-4":   {},
		"FAC-5":   {},
		"PLACE-1": {},
	}
}

// validateFixtureRefs returns an error listing every fixture ID
// the corpus references that is NOT in the allowed set. This is
// the structural guard against invented route/zone/facility IDs
// in the corpus itself.
func validateFixtureRefs(cases []CaseV2, allowed map[string]struct{}) error {
	check := func(bad *[]string, caseID, id string) {
		if id == "" {
			return
		}
		if _, ok := allowed[id]; !ok {
			*bad = append(*bad, fmt.Sprintf("%s/%s", caseID, id))
		}
	}
	var bad []string
	for _, c := range cases {
		for _, a := range c.ExpectedActions {
			check(&bad, c.ID, a.TargetID)
			for _, id := range a.TargetIDs {
				check(&bad, c.ID, id)
			}
			check(&bad, c.ID, a.RouteID)
		}
		for _, id := range c.ExpectedClarifyIDs {
			check(&bad, c.ID, id)
		}
		for _, id := range c.ExpectedEvidenceIDs {
			check(&bad, c.ID, id)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("invented or unknown fixture IDs: %s", strings.Join(bad, ", "))
	}
	return nil
}

// synthesizeProposal builds the wire-format Proposal that the
// oracle (production validator) is expected to accept for a case.
// Used by offline tests to prove the corpus's expected outputs
// round-trip through the decoder; it does NOT exercise the
// orchestrator's contracts.ValidateModelOutput, which lives in a
// different module and is intentionally out of lane.
func synthesizeProposal(c CaseV2) string {
	requestID := c.ExpectedRequestID
	if requestID == "" {
		requestID = "REQ-" + c.ID
	}
	dataVersion := c.ExpectedDataVersion
	if dataVersion == "" {
		dataVersion = "EXERCISE-7"
	}
	intent := "null"
	if c.ExpectedIntent != nil {
		intent = jsonString(*c.ExpectedIntent)
	}
	speech := "null"
	if c.ExpectedSpeechKey != nil {
		speech = jsonString(*c.ExpectedSpeechKey)
	}
	actionsJSON := "[]"
	if len(c.ExpectedActions) > 0 {
		bs, err := json.Marshal(c.ExpectedActions)
		if err != nil {
			// An un-marshallable expected action is a corpus bug.
			// Surface it loudly so the loader rejects the suite.
			panic(fmt.Sprintf("synthesizeProposal %s: marshal actions: %v", c.ID, err))
		}
		actionsJSON = string(bs)
	}
	clarifyJSON, _ := json.Marshal(c.ExpectedClarifyIDs)
	evidenceJSON, _ := json.Marshal(c.ExpectedEvidenceIDs)
	langJSON, _ := json.Marshal(c.Language)
	return fmt.Sprintf(
		`{"schema_version":"3.0","request_id":%s,"data_version":%s,"status":%s,"intent":%s,"language":%s,"actions":%s,"speech_key":%s,"clarification_ids":%s,"evidence_ids":%s}`,
		jsonString(requestID),
		jsonString(dataVersion),
		jsonString(c.ExpectedStatus),
		intent,
		langJSON,
		actionsJSON,
		speech,
		clarifyJSON,
		evidenceJSON,
	)
}

// jsonString returns a JSON string literal for s.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
