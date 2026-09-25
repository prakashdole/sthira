// Tests for the strict JSON-constrained decoder. Each test exercises
// ONE failure mode so a regression in one path is easy to localize.

package middleworker

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeStrictProposal_ValidMinimal(t *testing.T) {
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`
	p, err := decodeStrictProposal([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.SchemaVersion != "3.0" {
		t.Errorf("schema_version=%q", p.SchemaVersion)
	}
	if p.RequestID != "r1" {
		t.Errorf("request_id=%q", p.RequestID)
	}
	if p.Status != "OK" {
		t.Errorf("status=%q", p.Status)
	}
	if p.Intent == nil || *p.Intent != "FOCUS_PLACE" {
		t.Errorf("intent=%v", p.Intent)
	}
	if len(p.Actions) != 1 || p.Actions[0].Type != "FOCUS_FEATURE" || p.Actions[0].TargetID != "P1" {
		t.Errorf("actions=%+v", p.Actions)
	}
}

func TestDecodeStrictProposal_ValidNonOK(t *testing.T) {
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"DATA_UNAVAILABLE","intent":null,"language":"en-IN","actions":[],"speech_key":"x","clarification_ids":[],"evidence_ids":[]}`
	p, err := decodeStrictProposal([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Intent != nil {
		t.Errorf("intent must be null for non-OK: %v", p.Intent)
	}
	if len(p.Actions) != 0 {
		t.Errorf("actions must be empty for non-OK: %+v", p.Actions)
	}
}

func TestDecodeStrictProposal_EmptyRejected(t *testing.T) {
	_, err := decodeStrictProposal(nil)
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("expected ErrMalformed, got %v", err)
	}
}

func TestDecodeStrictProposal_ExtraTextRejected(t *testing.T) {
	body := "Here is the answer:\n" + `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`
	_, err := decodeStrictProposal([]byte(body))
	if !errors.Is(err, ErrExtraText) {
		t.Fatalf("expected ErrExtraText, got %v", err)
	}
}

func TestDecodeStrictProposal_TrailingTextRejected(t *testing.T) {
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}` + " and that's all"
	_, err := decodeStrictProposal([]byte(body))
	if !errors.Is(err, ErrExtraText) {
		t.Fatalf("expected ErrExtraText, got %v", err)
	}
}

func TestDecodeStrictProposal_ExcessiveLeadingWhitespaceRejected(t *testing.T) {
	body := "\n\n  \t \n  " + `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`
	_, err := decodeStrictProposal([]byte(body))
	if !errors.Is(err, ErrExtraText) {
		t.Fatalf("expected ErrExtraText, got %v", err)
	}
}

func TestDecodeStrictProposal_MarkdownFenceRejected(t *testing.T) {
	body := "```json\n" + `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}` + "\n```"
	_, err := decodeStrictProposal([]byte(body))
	if !errors.Is(err, ErrExtraText) {
		t.Fatalf("expected ErrExtraText, got %v", err)
	}
}

func TestDecodeStrictProposal_UnknownFieldRejected(t *testing.T) {
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[],"unexpected":"x"}`
	_, err := decodeStrictProposal([]byte(body))
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("expected ErrMalformed, got %v", err)
	}
}

func TestDecodeStrictProposal_MalformedJSONRejected(t *testing.T) {
	body := `{"schema_version":"3.0","request_id":"r1"` // truncated
	_, err := decodeStrictProposal([]byte(body))
	if err == nil || !errors.Is(err, ErrMalformed) {
		t.Fatalf("expected ErrMalformed, got %v", err)
	}
}

func TestDecodeStrictProposal_NoObjectRejected(t *testing.T) {
	body := `[1, 2, 3]`
	_, err := decodeStrictProposal([]byte(body))
	if !errors.Is(err, ErrExtraText) {
		t.Fatalf("expected ErrExtraText, got %v", err)
	}
}

func TestDecodeStrictProposal_DuplicateKeysRejected(t *testing.T) {
	// encoding/json keeps the last value when duplicates appear, so
	// this decodes without error there; the stricter check would be
	// an external JSON parser. We document that limitation here:
	// duplicates are NOT rejected by decodeStrictProposal alone. The
	// orchestrator's independent validator is the gate that catches
	// semantic drift; for byte-level drift, hash the response and
	// compare to the canonical digest if/when one is published.
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","request_id":"r2","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`
	_, err := decodeStrictProposal([]byte(body))
	if err != nil {
		t.Fatalf("decode allowed duplicate JSON keys (documented behavior): %v", err)
	}
}

func TestDecodeStrictProposal_StringWithBracesHandled(t *testing.T) {
	// A speech_key with embedded braces must not break brace-counting.
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":"hello { world }","clarification_ids":[],"evidence_ids":[]}`
	p, err := decodeStrictProposal([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.SpeechKey == nil || *p.SpeechKey != "hello { world }" {
		t.Errorf("speech_key=%v", p.SpeechKey)
	}
}

func TestDecodeStrictProposal_StringWithEscapedQuotesHandled(t *testing.T) {
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":"hello \"world\"","clarification_ids":[],"evidence_ids":[]}`
	p, err := decodeStrictProposal([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.SpeechKey == nil || *p.SpeechKey != `hello "world"` {
		t.Errorf("speech_key=%v", p.SpeechKey)
	}
}

func TestDecodeStrictProposal_StringWithUnmatchedBraces(t *testing.T) {
	// A string with an unmatched opening or closing brace must not alter object depth.
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":"only { an open brace","clarification_ids":[],"evidence_ids":[]}`
	p, err := decodeStrictProposal([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error with unmatched open brace: %v", err)
	}
	if p.SpeechKey == nil || *p.SpeechKey != "only { an open brace" {
		t.Errorf("speech_key=%v", p.SpeechKey)
	}

	body2 := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":"only } a close brace","clarification_ids":[],"evidence_ids":[]}`
	p2, err := decodeStrictProposal([]byte(body2))
	if err != nil {
		t.Fatalf("unexpected error with unmatched close brace: %v", err)
	}
	if p2.SpeechKey == nil || *p2.SpeechKey != "only } a close brace" {
		t.Errorf("speech_key=%v", p2.SpeechKey)
	}
}

func TestDecodeStrictProposal_EmptyActionArray(t *testing.T) {
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`
	_, err := decodeStrictProposal([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeStrictProposal_LargePayloadStillDecodes(t *testing.T) {
	// 200-char language; 5 actions; 3 clarifications (max).
	body := `{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"LIST_DESTINATIONS","language":"en-IN","actions":[{"type":"SHOW_CHOICES","target_ids":["F1","F2","F3"]}],"speech_key":"k","clarification_ids":[],"evidence_ids":["F1","F2","F3"]}`
	p, err := decodeStrictProposal([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Actions[0].TargetIDs) != 3 {
		t.Errorf("target_ids=%v", p.Actions[0].TargetIDs)
	}
}

// Document that the decoder does NOT enforce semantic limits
// (MaxModelActions, MaxClarificationIDs, etc.). The validator is the
// gate; the decoder is the wire-shape gate. This test pins that
// contract.
func TestDecodeStrictProposal_DoesNotEnforceSemanticLimits(t *testing.T) {
	// 10 actions: above the contract's MaxModelActions.
	var sb strings.Builder
	sb.WriteString(`{"schema_version":"3.0","request_id":"r1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[`)
	for i := 0; i < 10; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"type":"FOCUS_FEATURE","target_id":"P1"}`)
	}
	sb.WriteString(`],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`)
	_, err := decodeStrictProposal([]byte(sb.String()))
	if err != nil {
		t.Fatalf("decoder must not enforce semantic limits; expected nil, got %v", err)
	}
}
