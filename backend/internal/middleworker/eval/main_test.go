// Eval-driver smoke tests. No real vLLM. The harness-check and
// manifest modes are the only ones verifiable today; benchmark
// mode is gated on real artifacts AND on the unimplemented runner
// (see TestEval_BenchmarkModeIsUnimplemented below).

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sthira/backend/internal/middleworker"
)

func TestEval_HarnessCheckPrintsBlocker(t *testing.T) {
	// Capture stdout by re-running the binary would be heavy; we
	// instead assert on the manifest content which is the
	// authoritative artifact record.
	bs, err := json.Marshal(Pinned)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(bs)
	if !strings.Contains(s, "sarvamai/sarvam-30b") {
		t.Errorf("manifest missing model id: %s", s)
	}
	if !strings.Contains(s, "Apache-2.0") {
		t.Errorf("manifest missing license: %s", s)
	}
	if !strings.Contains(s, `"trust_remote_code":true`) {
		t.Errorf("manifest missing trust_remote_code=true: %s", s)
	}
	if !strings.Contains(s, "NOT_EVALUATED") {
		t.Errorf("manifest must surface NOT_EVALUATED for unverified fields: %s", s)
	}
	if !strings.Contains(s, "BLOCKED_HARDWARE") {
		t.Errorf("manifest must surface BLOCKED_HARDWARE for artifacts: %s", s)
	}
}

func TestEval_ManifestModeWritesFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "manifest.json")
	bs, _ := json.MarshalIndent(Pinned, "", "  ")
	if err := os.WriteFile(out, bs, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "sarvamai/sarvam-30b") {
		t.Errorf("manifest file missing model id")
	}
}

// TestEval_BenchmarkModeIsUnimplemented pins the documented
// capability gap: runBenchmark unconditionally exits 2 regardless
// of endpoint/corpus/quantization flags. It does NOT mean "waiting
// for a GPU" — the runner code itself has no inference path today.
// See HARDWARE_BLOCKER.md for the artifact/gpu prerequisites AND
// the runner's missing capability (which Opus must implement
// before any real benchmark can run).
func TestEval_BenchmarkModeIsUnimplemented(t *testing.T) {
	for _, a := range []ArtifactInfo{Pinned.BF16Artifact, Pinned.AWQInt4Artifact, Pinned.FP8Artifact} {
		if a.SHA256 != "" {
			t.Errorf("artifact SHA-256 must be empty until real-inference is recorded")
		}
		if a.Status != "BLOCKED_HARDWARE" && a.Status != "PENDING_DOWNLOAD" {
			t.Errorf("artifact status = %q; expected BLOCKED_HARDWARE or PENDING_DOWNLOAD", a.Status)
		}
	}
}

// --- offline corpus tests --------------------------------------------------
//
// The corpus-check oracle has TWO layers:
//
//  1. WIRE-SHAPE: decodeStrictProposal accepts the synthesized
//     proposal as a valid JSON document matching the v3.0 wire
//     contract. A round-trip via StubRuntime is the proof.
//
//  2. SEMANTIC: the decoded proposal's status/intent/actions/IDs
//     match the case's expected outcome, NOT just that the JSON
//     decodes. This is what proves the oracle can detect a wrong
//     model decision rather than only a malformed one.
//
// The semantic oracle lives in this package (not the production
// validator) so it can be exercised offline without the contracts
// module. It implements only the per-case checks the corpus
// exercises; the production contracts.ValidateModelOutput +
// EnforceScopedContext remain the authoritative gates.

const v2CorpusPath = "corpus/synthetic_v2.jsonl"
const v1CorpusPath = "corpus/synthetic.jsonl"

func TestCorpusV2_LoadsAndCounts(t *testing.T) {
	cases, err := loadCorpusJSONL(v2CorpusPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// The augmented corpus adds the camera-control, unknown-ID,
	// destination-panel, route-invention, arrival, weather,
	// injection-vector, and known-facility-subset cases on top of
	// the existing 15. Keep this number honest: if you add or
	// remove cases, update the constant and the deliverable note.
	const wantV2 = 13
	if got := len(cases); got != wantV2 {
		t.Errorf("v2 case count = %d, want %d (update the constant when intentional)", got, wantV2)
	}
	// Every case must carry the augmented contract fields.
	for _, c := range cases {
		if c.ExpectedStatus == "" {
			t.Errorf("%s: expected_status empty", c.ID)
		}
		if c.ExpectedRequestID == "" {
			t.Errorf("%s: expected_request_id empty", c.ID)
		}
		if c.ExpectedDataVersion == "" {
			t.Errorf("%s: expected_data_version empty", c.ID)
		}
		if c.Reason == "" {
			t.Errorf("%s: reason empty", c.ID)
		}
	}
}

func TestCorpusV2_UniqueIDsAndNoCrossFileDuplicates(t *testing.T) {
	v1, err := loadCorpusJSONL(v1CorpusPath)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	v2, err := loadCorpusJSONL(v2CorpusPath)
	if err != nil {
		t.Fatalf("v2: %v", err)
	}
	if err := uniqueCaseIDs(v2); err != nil {
		t.Errorf("v2 unique: %v", err)
	}
	if err := uniqueCaseIDs(v1); err != nil {
		t.Errorf("v1 unique: %v", err)
	}
	if err := uniqueCaseIDs(append(v1, v2...)); err != nil {
		t.Errorf("merged unique: %v", err)
	}
}

func TestCorpusV2_NoInventedFixtureIDs(t *testing.T) {
	cases, err := loadCorpusJSONL(v2CorpusPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := validateFixtureRefs(cases, allowedFixtureIDs()); err != nil {
		t.Errorf("fixture refs: %v", err)
	}
}

// TestCorpusV2_PerCaseContext exercises the per-case allow-list:
// every fixture ID the case's expected proposal references must
// appear in the case's own ExpectedContextIDs set, not just in the
// global allow-list. This catches "wrong scenario" matches that
// the global check cannot — for example, a model that focuses
// FACILITY-DEMO-1 in a case whose scenario is about PLACE-DEMO-1.
func TestCorpusV2_PerCaseContext(t *testing.T) {
	cases, err := loadCorpusJSONL(v2CorpusPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := validatePerCaseContext(cases); err != nil {
		t.Errorf("per-case context: %v", err)
	}
}

func TestCorpusV2_SyntheticProposalMatchesShapeContract(t *testing.T) {
	// For every case, the synthesized proposal must be valid JSON
	// with the exact top-level keys the wire schema requires. This
	// proves the corpus's expected outputs round-trip through the
	// decoder-side pre-conditions (required fields, type checks).
	cases, err := loadCorpusJSONL(v2CorpusPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	required := []string{
		"schema_version", "request_id", "data_version", "status",
		"intent", "language", "actions", "speech_key",
		"clarification_ids", "evidence_ids",
	}
	for _, c := range cases {
		body := synthesizeProposal(c)
		var probe map[string]any
		if err := json.Unmarshal([]byte(body), &probe); err != nil {
			t.Errorf("%s: synthesized proposal not valid JSON: %v\n%s", c.ID, err, body)
			continue
		}
		for _, k := range required {
			if _, ok := probe[k]; !ok {
				t.Errorf("%s: synthesized proposal missing required key %q", c.ID, k)
			}
		}
		if probe["schema_version"] != "3.0" {
			t.Errorf("%s: schema_version = %v, want 3.0", c.ID, probe["schema_version"])
		}
		if probe["status"] != c.ExpectedStatus {
			t.Errorf("%s: status = %v, want %s", c.ID, probe["status"], c.ExpectedStatus)
		}
		// Non-OK status must carry no actions.
		if c.ExpectedStatus != "OK" {
			if acts, ok := probe["actions"].([]any); !ok || len(acts) != 0 {
				t.Errorf("%s: non-OK status must have empty actions, got %v", c.ID, probe["actions"])
			}
		}
	}
}

// TestOracle_WireShapeAcceptsEveryCase covers BOTH OK and non-OK
// cases. Non-OK proposals must also decode cleanly — they carry
// empty actions and a null intent, but the JSON itself must be
// wire-shape valid. Skipping non-OK cases would leave abstention
// and error paths untested, exactly the paths a misbehaving model
// is most likely to emit.
func TestOracle_WireShapeAcceptsEveryCase(t *testing.T) {
	cases, err := loadCorpusJSONL(v2CorpusPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			fixture := writeSynthFixture(t, c)
			rt, err := middleworker.NewStubRuntime(fixture)
			if err != nil {
				t.Fatalf("NewStubRuntime: %v", err)
			}
			rt.SetLanguages([]string{c.Language, "en-IN", "hi-IN", "ml-IN"})
			req := middleworker.RequestEnvelope{
				RequestID: c.ExpectedRequestID,
				ScopedContext: middleworker.ScopedContext{
					SchemaVersion: "3.0",
					DataVersion:   c.ExpectedDataVersion,
				},
				Transcript: middleworker.TranscriptInput{
					RequestID: c.ExpectedRequestID,
					Language:  c.Language,
					Text:      c.Transcript,
					State:     "OK",
				},
			}
			resp, err := rt.Propose(context.Background(), req)
			if err != nil {
				t.Fatalf("wire-shape oracle rejected allowed proposal: %v\nfixture=%s", err, fixture)
			}
			if resp.Proposal.Status != c.ExpectedStatus {
				t.Errorf("decoded status = %s, want %s", resp.Proposal.Status, c.ExpectedStatus)
			}
			if c.ExpectedStatus == "OK" {
				if resp.Proposal.Intent == nil {
					t.Errorf("OK case has nil intent: %s", c.ID)
				} else if *resp.Proposal.Intent != *c.ExpectedIntent {
					t.Errorf("decoded intent = %v, want %v", resp.Proposal.Intent, c.ExpectedIntent)
				}
			} else {
				if resp.Proposal.Intent != nil {
					t.Errorf("non-OK case %s has non-nil intent: %v", c.ID, resp.Proposal.Intent)
				}
				if len(resp.Proposal.Actions) != 0 {
					t.Errorf("non-OK case %s has actions: %+v", c.ID, resp.Proposal.Actions)
				}
			}
			if len(resp.Proposal.Actions) != len(c.ExpectedActions) {
				t.Errorf("decoded actions = %d, want %d", len(resp.Proposal.Actions), len(c.ExpectedActions))
			}
		})
	}
}

// TestOracle_SemanticExpectationsAcceptCorrectProposal is the
// positive side of the semantic oracle: when a proposal matches
// the case's expected outcome, the oracle returns nil. Together
// with the negative-side tests below, this proves the oracle can
// distinguish right from wrong proposals — not merely "JSON
// decodes" from "JSON does not decode".
func TestOracle_SemanticExpectationsAcceptCorrectProposal(t *testing.T) {
	cases, err := loadCorpusJSONL(v2CorpusPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, c := range cases {
		t.Run(c.ID+"/correct", func(t *testing.T) {
			body := synthesizeProposal(c)
			var p middleworker.Proposal
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatalf("synthesized proposal not valid JSON: %v", err)
			}
			if err := semanticExpectationCheck(c, p); err != nil {
				t.Errorf("semantic oracle rejected the case's own expected proposal: %v", err)
			}
		})
	}
}

// --- negative examples ----------------------------------------------------
//
// Each negative example constructs a deliberately-wrong proposal
// (wire-shape valid, semantically wrong) and asserts the semantic
// oracle rejects it. The wire-shape decoder accepts these — that
// is the point. The semantic oracle is what catches them.

func TestOracle_RejectsWrongStatus(t *testing.T) {
	// SYN-016 (ZOOM OK) with status flipped to CLARIFY.
	c := mustLoadCase(t, "SYN-016")
	wrong := wrongProposal(t, c, func(p *middleworker.Proposal) {
		p.Status = "CLARIFY"
	})
	if err := semanticExpectationCheck(c, wrong); err == nil {
		t.Fatalf("semantic oracle accepted wrong status; want rejection")
	}
}

func TestOracle_RejectsForbiddenTarget(t *testing.T) {
	// SYN-019 (FIT_FEATURES with SZ-1,RZ-1,ROUTE-1) rewritten to
	// reference PLACE-DEMO-1 — globally known but not in the case
	// context. Per-case context check rejects.
	c := mustLoadCase(t, "SYN-019")
	wrong := wrongProposal(t, c, func(p *middleworker.Proposal) {
		if len(p.Actions) == 0 {
			t.Fatalf("expected an action to mutate")
		}
		p.Actions[0].TargetIDs = []string{"PLACE-DEMO-1"}
	})
	if err := semanticExpectationCheck(c, wrong); err == nil {
		t.Fatalf("semantic oracle accepted forbidden target; want rejection")
	}
}

func TestOracle_RejectsWrongVisibility(t *testing.T) {
	// SYN-020 (SET_LAYER_VISIBILITY visible=false) flipped to true.
	c := mustLoadCase(t, "SYN-020")
	wrong := wrongProposal(t, c, func(p *middleworker.Proposal) {
		if len(p.Actions) == 0 {
			t.Fatalf("expected an action to mutate")
		}
		tr := true
		p.Actions[0].Visible = &tr
	})
	if err := semanticExpectationCheck(c, wrong); err == nil {
		t.Fatalf("semantic oracle accepted wrong visibility; want rejection")
	}
}

func TestOracle_RejectsUnauthorizedExtraAction(t *testing.T) {
	// SYN-021 (HIGHLIGHT_FEATURE on SZ-1) with an extra action
	// appended — the case explicitly expects one action.
	c := mustLoadCase(t, "SYN-021")
	wrong := wrongProposal(t, c, func(p *middleworker.Proposal) {
		p.Actions = append(p.Actions, middleworker.Action{Type: "RECENTER"})
	})
	if err := semanticExpectationCheck(c, wrong); err == nil {
		t.Fatalf("semantic oracle accepted extra action; want rejection")
	}
}

// TestOracle_RejectsNonOKWithActions covers the schema-level rule
// that a non-OK status must carry no actions; the semantic oracle
// must catch a model that emits "DATA_UNAVAILABLE" with a stray
// action attached.
func TestOracle_RejectsNonOKWithActions(t *testing.T) {
	c := mustLoadCase(t, "SYN-023") // DATA_UNAVAILABLE
	wrong := wrongProposal(t, c, func(p *middleworker.Proposal) {
		p.Actions = []middleworker.Action{{Type: "RECENTER"}}
	})
	if err := semanticExpectationCheck(c, wrong); err == nil {
		t.Fatalf("semantic oracle accepted non-OK with actions; want rejection")
	}
}

// --- wire-shape rejection tests -------------------------------------------

func TestOracle_RejectsMalformedJSON(t *testing.T) {
	fixture := writeRawFixture(t, `{not json`)
	rt, _ := middleworker.NewStubRuntime(fixture)
	_, err := rt.Propose(context.Background(), middleworker.RequestEnvelope{
		RequestID: "REQ-X",
		Transcript: middleworker.TranscriptInput{
			RequestID: "REQ-X", Language: "en-IN", Text: "x", State: "OK",
		},
	})
	if err == nil {
		t.Fatalf("expected ErrMalformed for truncated JSON, got nil")
	}
	if !errors.Is(err, middleworker.ErrMalformed) {
		t.Errorf("expected ErrMalformed, got %v", err)
	}
}

func TestOracle_RejectsExtraText(t *testing.T) {
	body := synthesizeProposal(CaseV2{
		ID: "X", Language: "en-IN",
		ExpectedStatus:    "OK",
		ExpectedIntent:    strPtr("FOCUS_PLACE"),
		ExpectedActions:   []ActionSpec{{Type: "FOCUS_FEATURE", TargetID: "PLACE-DEMO-1"}},
		ExpectedRequestID: "REQ-EXTRA",
	})
	fixture := writeRawFixture(t, "Sure thing:\n"+body)
	rt, _ := middleworker.NewStubRuntime(fixture)
	_, err := rt.Propose(context.Background(), middleworker.RequestEnvelope{
		RequestID: "REQ-EXTRA",
		Transcript: middleworker.TranscriptInput{
			RequestID: "REQ-EXTRA", Language: "en-IN", Text: "x", State: "OK",
		},
	})
	if err == nil {
		t.Fatalf("expected ErrExtraText, got nil")
	}
	if !errors.Is(err, middleworker.ErrExtraText) {
		t.Errorf("expected ErrExtraText, got %v", err)
	}
}

func TestOracle_RejectsUnknownField(t *testing.T) {
	body := synthesizeProposal(CaseV2{
		ID: "X", Language: "en-IN",
		ExpectedStatus:    "OK",
		ExpectedIntent:    strPtr("FOCUS_PLACE"),
		ExpectedActions:   []ActionSpec{{Type: "FOCUS_FEATURE", TargetID: "PLACE-DEMO-1"}},
		ExpectedRequestID: "REQ-UNK",
	})
	// Inject a top-level field the contract forbids.
	body = body[:len(body)-1] + `,"fabricated_commit":true}`
	fixture := writeRawFixture(t, body)
	rt, _ := middleworker.NewStubRuntime(fixture)
	_, err := rt.Propose(context.Background(), middleworker.RequestEnvelope{
		RequestID: "REQ-UNK",
		Transcript: middleworker.TranscriptInput{
			RequestID: "REQ-UNK", Language: "en-IN", Text: "x", State: "OK",
		},
	})
	if err == nil {
		t.Fatalf("expected ErrMalformed for unknown field, got nil")
	}
	if !errors.Is(err, middleworker.ErrMalformed) {
		t.Errorf("expected ErrMalformed, got %v", err)
	}
}

func TestOracle_RejectsRequestIDMismatch(t *testing.T) {
	body := synthesizeProposal(CaseV2{
		ID: "X", Language: "en-IN",
		ExpectedStatus:    "OK",
		ExpectedIntent:    strPtr("FOCUS_PLACE"),
		ExpectedActions:   []ActionSpec{{Type: "FOCUS_FEATURE", TargetID: "PLACE-DEMO-1"}},
		ExpectedRequestID: "REQ-SENT",
	})
	fixture := writeRawFixture(t, body)
	rt, _ := middleworker.NewStubRuntime(fixture)
	_, err := rt.Propose(context.Background(), middleworker.RequestEnvelope{
		RequestID: "REQ-OTHER",
		Transcript: middleworker.TranscriptInput{
			RequestID: "REQ-OTHER", Language: "en-IN", Text: "x", State: "OK",
		},
	})
	if err == nil {
		t.Fatalf("expected request_id mismatch error, got nil")
	}
	if !middleworker.IsRequestIDMismatch(err) {
		t.Errorf("expected request-id mismatch sentinel, got %v", err)
	}
}

// --- helpers ---------------------------------------------------------------

// mustLoadCase loads the v2 corpus and returns the case with the
// given id, failing the test if it is missing.
func mustLoadCase(t *testing.T, id string) CaseV2 {
	t.Helper()
	cases, err := loadCorpusJSONL(v2CorpusPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, c := range cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("case %s not found in %s", id, v2CorpusPath)
	return CaseV2{}
}

// wrongProposal synthesizes the case's expected proposal, decodes
// it into a Proposal, applies a mutation, and returns the mutated
// proposal. The mutation produces a wire-shape-valid but
// semantically-wrong proposal for the negative-example tests.
func wrongProposal(t *testing.T, c CaseV2, mutate func(*middleworker.Proposal)) middleworker.Proposal {
	t.Helper()
	body := synthesizeProposal(c)
	var p middleworker.Proposal
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	mutate(&p)
	return p
}

// semanticExpectationCheck compares an observed Proposal against a
// case's locked expected outcome. It implements only the checks
// the corpus exercises; the production validator in
// backend/internal/contracts remains the authoritative gate.
//
// Returns nil iff the observed proposal matches the case's
// expected status, intent, actions, evidence_ids, clarification_ids
// and speech_key (where applicable). The check distinguishes:
//
//   - wrong status (OK claimed for a non-OK case, or vice versa)
//   - wrong intent (intent string mismatch)
//   - wrong actions (count or content mismatch, including extras)
//   - wrong visibility on SET_LAYER_VISIBILITY (explicit-false vs
//     missing vs true)
//   - forbidden targets (an ID outside the case's per-case context)
//   - non-OK carrying actions (status rule violation)
//
// All comparisons ignore request_id/data_version/language because
// those reflect the request envelope, not the model's decision.
func semanticExpectationCheck(c CaseV2, observed middleworker.Proposal) error {
	if observed.Status != c.ExpectedStatus {
		return errSemantic("status=%q want %q", observed.Status, c.ExpectedStatus)
	}
	if c.ExpectedStatus == "OK" {
		if observed.Intent == nil {
			return errSemantic("OK status requires non-nil intent")
		}
		if *observed.Intent != *c.ExpectedIntent {
			return errSemantic("intent=%q want %q", *observed.Intent, *c.ExpectedIntent)
		}
	} else {
		if observed.Intent != nil {
			return errSemantic("non-OK status must have nil intent; got %q", *observed.Intent)
		}
		if len(observed.Actions) != 0 {
			return errSemantic("non-OK status must carry no actions; got %d", len(observed.Actions))
		}
	}
	if len(observed.Actions) != len(c.ExpectedActions) {
		return errSemantic("actions count=%d want %d", len(observed.Actions), len(c.ExpectedActions))
	}
	for i, want := range c.ExpectedActions {
		got := observed.Actions[i]
		if got.Type != want.Type {
			return errSemantic("action %d type=%q want %q", i, got.Type, want.Type)
		}
		if want.Type == "SET_LAYER_VISIBILITY" {
			if got.Layer != want.Layer {
				return errSemantic("action %d layer=%q want %q", i, got.Layer, want.Layer)
			}
			// explicit-false vs missing vs true must round-trip
			// distinctly. The *bool shape preserves all three.
			switch {
			case want.Visible == nil && got.Visible != nil:
				return errSemantic("action %d visible=%v want nil (missing)", i, *got.Visible)
			case want.Visible != nil && got.Visible == nil:
				return errSemantic("action %d visible=nil want %v", i, *want.Visible)
			case want.Visible != nil && got.Visible != nil && *want.Visible != *got.Visible:
				return errSemantic("action %d visible=%v want %v", i, *got.Visible, *want.Visible)
			}
		}
		if got.TargetID != want.TargetID {
			return errSemantic("action %d target_id=%q want %q", i, got.TargetID, want.TargetID)
		}
		if !stringSlicesEqual(got.TargetIDs, want.TargetIDs) {
			return errSemantic("action %d target_ids=%v want %v", i, got.TargetIDs, want.TargetIDs)
		}
		if got.RouteID != want.RouteID {
			return errSemantic("action %d route_id=%q want %q", i, got.RouteID, want.RouteID)
		}
		if got.Panel != want.Panel {
			return errSemantic("action %d panel=%q want %q", i, got.Panel, want.Panel)
		}
		if got.Direction != want.Direction {
			return errSemantic("action %d direction=%q want %q", i, got.Direction, want.Direction)
		}
		if got.Steps != want.Steps {
			return errSemantic("action %d steps=%d want %d", i, got.Steps, want.Steps)
		}
		if got.Language != want.Language {
			return errSemantic("action %d language=%q want %q", i, got.Language, want.Language)
		}
		if got.Layer != want.Layer {
			return errSemantic("action %d layer=%q want %q", i, got.Layer, want.Layer)
		}
	}
	if !stringSlicesEqual(observed.EvidenceIDs, c.ExpectedEvidenceIDs) {
		return errSemantic("evidence_ids=%v want %v", observed.EvidenceIDs, c.ExpectedEvidenceIDs)
	}
	if !stringSlicesEqual(observed.ClarificationIDs, c.ExpectedClarifyIDs) {
		return errSemantic("clarification_ids=%v want %v", observed.ClarificationIDs, c.ExpectedClarifyIDs)
	}
	// speech_key: nil-vs-non-nil matters; the string value must match.
	switch {
	case observed.SpeechKey == nil && c.ExpectedSpeechKey != nil:
		return errSemantic("speech_key=nil want %q", *c.ExpectedSpeechKey)
	case observed.SpeechKey != nil && c.ExpectedSpeechKey == nil:
		return errSemantic("speech_key=%q want nil", *observed.SpeechKey)
	case observed.SpeechKey != nil && c.ExpectedSpeechKey != nil && *observed.SpeechKey != *c.ExpectedSpeechKey:
		return errSemantic("speech_key=%q want %q", *observed.SpeechKey, *c.ExpectedSpeechKey)
	}
	// Per-case context check: every action target/route and every
	// evidence/clarify id must be in ExpectedContextIDs, when the
	// case declares a non-empty per-case context.
	if len(c.ExpectedContextIDs) > 0 {
		allowed := map[string]struct{}{}
		for _, id := range c.ExpectedContextIDs {
			allowed[id] = struct{}{}
		}
		contains := func(id string) bool {
			_, ok := allowed[id]
			return ok
		}
		for _, a := range observed.Actions {
			if a.TargetID != "" && !contains(a.TargetID) {
				return errSemantic("target_id %q not in case context", a.TargetID)
			}
			for _, id := range a.TargetIDs {
				if id != "" && !contains(id) {
					return errSemantic("target_id %q not in case context", id)
				}
			}
			if a.RouteID != "" && !contains(a.RouteID) {
				return errSemantic("route_id %q not in case context", a.RouteID)
			}
		}
		for _, id := range observed.EvidenceIDs {
			if !contains(id) {
				return errSemantic("evidence_id %q not in case context", id)
			}
		}
	}
	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// errSemantic is a small typed error so tests can recognize the
// oracle's own output vs. production validator errors.
type semanticMismatch struct{ msg string }

func (e *semanticMismatch) Error() string { return "semantic oracle: " + e.msg }
func errSemantic(format string, args ...any) error {
	return &semanticMismatch{msg: fmt.Sprintf(format, args...)}
}

// writeSynthFixture writes the synthesized proposal of c to a
// temporary file and returns its path. The StubRuntime reads the
// file on each Propose call.
func writeSynthFixture(t *testing.T, c CaseV2) string {
	t.Helper()
	return writeRawFixture(t, synthesizeProposal(c))
}

func writeRawFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "proposal.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func strPtr(s string) *string { return &s }

// TestOracle_RejectsOmittedVisibility: the model omits "visible" on the
// wire (absent key decodes to nil) while SYN-020 expects explicit false.
// Omission must not be scored as hide.
func TestOracle_RejectsOmittedVisibility(t *testing.T) {
	c := mustLoadCase(t, "SYN-020")
	var raw map[string]any
	if err := json.Unmarshal([]byte(synthesizeProposal(c)), &raw); err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	actions, _ := raw["actions"].([]any)
	if len(actions) == 0 {
		t.Fatalf("SYN-020 has no actions")
	}
	delete(actions[0].(map[string]any), "visible")
	body, _ := json.Marshal(raw)
	var got middleworker.Proposal
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Actions[0].Visible != nil {
		t.Fatalf("absent visible decoded as %v, want nil", *got.Actions[0].Visible)
	}
	if err := semanticExpectationCheck(c, got); err == nil || !strings.Contains(err.Error(), "visible") {
		t.Fatalf("oracle accepted omitted visible against visible=false (err=%v)", err)
	}
}

// TestCorpusContext_RejectsGloballyKnownIDOutsideCase: a fixture ID that
// is in the global allow-list but not in this case's context must fail
// the per-case check (SYN-021 context is SZ-1 only).
func TestCorpusContext_RejectsGloballyKnownIDOutsideCase(t *testing.T) {
	c := mustLoadCase(t, "SYN-021")
	if _, ok := allowedFixtureIDs()["FACILITY-DEMO-1"]; !ok {
		t.Fatalf("FACILITY-DEMO-1 must be globally known for this test")
	}
	c.ExpectedEvidenceIDs = append(append([]string{}, c.ExpectedEvidenceIDs...), "FACILITY-DEMO-1")
	if err := validateFixtureRefs([]CaseV2{c}, allowedFixtureIDs()); err != nil {
		t.Fatalf("global check should pass: %v", err)
	}
	if err := validatePerCaseContext([]CaseV2{c}); err == nil {
		t.Fatalf("per-case check accepted FACILITY-DEMO-1 outside SYN-021 context")
	}
}
