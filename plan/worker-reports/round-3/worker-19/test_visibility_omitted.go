// Regression: omitted visibility field differs from explicit false
//
// This test proves the semantic oracle distinguishes three visibility states
// that the wire *bool type preserves:
//
//   omitted (nil)  ≠  explicit false (*false)  ≠  explicit true (*true)
//
// The model failure mode this catches: the model emits
//   {"type":"SET_LAYER_VISIBILITY","layer":"RED_ZONES"}
// without the "visible" field, against a corpus case that expects
//   {"type":"SET_LAYER_VISIBILITY","layer":"RED_ZONES","visible":false}
//
// The wire decoder leaves Visible=nil when the JSON field is absent.
// The semantic oracle must reject nil-vs-false as semantically distinct,
// not silently accept the omission as-false.
//
// Source: backend/internal/middleworker/eval/main_test.go
// Base SHA-256: 1b688e2803d80222304cbddcc14114040872d3b3c01f50aff81f1bf12d56a673
// Status: NOT_RUN — deferred by user
//
// Run after integration:
//   cd backend/internal/middleworker/eval && go test -v -run TestOracle_RejectsOmittedVisibility ./...

package main

import (
	"testing"

	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sthira/backend/internal/middleworker"
)

// TestOracle_RejectsOmittedVisibility proves that the semantic oracle
// rejects a model proposal where the "visible" field is absent (nil),
// when the corpus case expects explicit visible=false.
//
// Three-state distinction:
//   want.Visible=nil  (corpus omitted)       — not tested here
//   want.Visible=&false (corpus explicit)  ← SYN-020 base case
//   got.Visible=nil   (model omitted)        ← THIS TEST
//
// The semantic check at semanticExpectationCheck line 562-563 rejects:
//   "want.Visible != nil && got.Visible == nil"
//   → errSemantic("action %d visible=nil want %v", i, *want.Visible)
func TestOracle_RejectsOmittedVisibility(t *testing.T) {
	// Load the corpus to get SYN-020
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	corpusPath := filepath.Join(wd, "corpus", "synthetic_v2.jsonl")

	var SYN020 CaseV2
	{
		data, err := os.ReadFile(corpusPath)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", corpusPath, err)
		}
		scan := bufio.NewScanner(bytes.NewReader(data))
		for scan.Scan() {
			line := bytes.TrimSpace(scan.Bytes())
			if len(line) == 0 || line[0] == '#' {
				continue
			}
			var c CaseV2
			if err := json.Unmarshal(line, &c); err != nil {
				continue // skip malformed
			}
			if c.ID == "SYN-020" {
				SYN020 = c
				break
			}
		}
	}
	if SYN020.ID == "" {
		t.Fatalf("SYN-020 not found in corpus")
	}
	// Verify SYN-020 baseline: expected Visible=false
	if len(SYN020.ExpectedActions) == 0 {
		t.Fatalf("SYN-020 has no ExpectedActions")
	}
	if SYN020.ExpectedActions[0].Type != "SET_LAYER_VISIBILITY" {
		t.Fatalf("SYN-020 action 0 type = %q, want SET_LAYER_VISIBILITY", SYN020.ExpectedActions[0].Type)
	}
	if SYN020.ExpectedActions[0].Visible == nil {
		t.Fatalf("SYN-020 Visible is nil — test requires non-nil Visible")
	}
	if *SYN020.ExpectedActions[0].Visible != false {
		t.Fatalf("SYN-020 Visible = %v, want false", *SYN020.ExpectedActions[0].Visible)
	}

	// Synthesize the correct proposal (visible=false) then mutate to nil (omitted)
	fixture := synthesizeProposal(SYN020)
	var correct middleworker.Proposal
	if err := json.Unmarshal([]byte(fixture), &correct); err != nil {
		t.Fatalf("synthesize JSON: %v", err)
	}
	if len(correct.Actions) == 0 {
		t.Fatalf("SYN-020 synthesized proposal has no actions")
	}

	// Mutate: set Visible=nil to simulate the model omitting the field
	correct.Actions[0].Visible = nil // ← the failure mode: omitted field

	// The oracle must reject: want.Visible=&false vs got.Visible=nil
	err = semanticExpectationCheck(SYN020, correct)
	if err == nil {
		t.Fatalf("semantic oracle accepted omitted visible (nil) against visible=false expectation;\nwant rejection")
	}
	// The error message must reference visibility
	if !bytes.Contains([]byte(err.Error()), []byte("visible")) {
		t.Fatalf("error %q does not mention 'visible'; got wrong rejection reason", err.Error())
	}
}

// TestOracle_RejectsOmittedVisibilityVsTrue is the complementary case:
// corpus expects visible=true, model emits visible=nil (omitted).
func TestOracle_RejectsOmittedVisibilityVsTrue(t *testing.T) {
	// Build a synthetic case with visible=true
	c := CaseV2{
		ID:                 "REGR-OMIT-VIS",
		Language:           "en-IN",
		Scenario:           "regression omitted visibility vs true",
		ReviewStatus:       "REVIEWED",
		ExpectedStatus:     "OK",
		ExpectedIntent:     strPtr("FOCUS_PLACE"),
		ExpectedDataVersion: "EXERCISE-7",
		ExpectedSpeechKey:  nil,
		ExpectedClarifyIDs: []string{},
		ExpectedEvidenceIDs: []string{},
		ExpectedRequestID:  "REQ-REGR-OMIT-VIS",
		ExpectedContextIDs: []string{},
		ExpectedActions: []ActionSpec{{
			Type:    "SET_LAYER_VISIBILITY",
			Layer:   "RED_ZONES",
			Visible: ptrBool(true), // explicit true in corpus
		}},
	}
	// Synthesize + decode → Visible=&true
	fixture := synthesizeProposal(c)
	var correct middleworker.Proposal
	if err := json.Unmarshal([]byte(fixture), &correct); err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if correct.Actions[0].Visible == nil || *correct.Actions[0].Visible != true {
		t.Fatalf("synthesized proposal Visible != true")
	}

	// Mutate: set Visible=nil — the model omitted the field
	correct.Actions[0].Visible = nil

	err := semanticExpectationCheck(c, correct)
	if err == nil {
		t.Fatalf("semantic oracle accepted omitted visible (nil) against visible=true expectation; want rejection")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("visible")) {
		t.Fatalf("error %q does not mention 'visible'", err.Error())
	}
}

// TestOracle_AcceptsExplicitFalse is the positive control:
// corpus expects visible=false, model emits visible=false (explicit).
// Both nil and &false must be distinct; this proves explicit=false passes.
func TestOracle_AcceptsExplicitFalse(t *testing.T) {
	c := CaseV2{
		ID:                 "REGR-VIS-FALSE",
		Language:           "en-IN",
		Scenario:           "regression explicit false",
		ReviewStatus:       "REVIEWED",
		ExpectedStatus:     "OK",
		ExpectedIntent:     strPtr("FOCUS_PLACE"),
		ExpectedDataVersion: "EXERCISE-7",
		ExpectedSpeechKey:  nil,
		ExpectedClarifyIDs: []string{},
		ExpectedEvidenceIDs: []string{},
		ExpectedRequestID:  "REQ-REGR-VIS-FALSE",
		ExpectedContextIDs: []string{},
		ExpectedActions: []ActionSpec{{
			Type:    "SET_LAYER_VISIBILITY",
			Layer:   "RED_ZONES",
			Visible: ptrBool(false), // explicit false
		}},
	}
	fixture := synthesizeProposal(c)
	var proposal middleworker.Proposal
	if err := json.Unmarshal([]byte(fixture), &proposal); err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	// Visible must be &false, not nil
	if proposal.Actions[0].Visible == nil {
		t.Fatalf("synthesized Visible is nil, want &false")
	}
	if *proposal.Actions[0].Visible != false {
		t.Fatalf("synthesized Visible = %v, want false", *proposal.Actions[0].Visible)
	}
	// Oracle must accept exact match
	err := semanticExpectationCheck(c, proposal)
	if err != nil {
		t.Fatalf("semantic oracle rejected exact match visible=false: %v", err)
	}
}

// ptrBool is a test helper to avoid &true/&false literals.
func ptrBool(b bool) *bool { return &b }
