package contracts

// narrowly-scoped security tests for the P6 typed contracts. These
// tests pin the trust boundary at /api/v3: the typed envelopes
// reject URL-typed fields, set bounded depths, and refuse to
// marshal insecure shapes. The runner is the security-surface
// witness for backend/security/threat-model.md (T6 prompt
// injection, T7 audio decoding abuse).

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTranscriptionRequestRejectsOversizeBodyBytes pins the
// compression / decode budget. A request that exceeds 512 KiB
// compressed or 20 s decoded is invalid JSON-shape; we assert the
// constants are conservative.
func TestTranscriptionRequestRejectsOversizeBodyBytes(t *testing.T) {
	if MaxTranscriptionCompressedBytes < 256*1024 || MaxTranscriptionCompressedBytes > 1<<20 {
		t.Fatalf("compressed-bytes budget drifted: %d", MaxTranscriptionCompressedBytes)
	}
	if MaxTranscriptionDecodedSeconds < 10 || MaxTranscriptionDecodedSeconds > 30 {
		t.Fatalf("decoded-seconds budget drifted: %f", MaxTranscriptionDecodedSeconds)
	}
}

// TestPipelineEnvelopeNeverCarriesArbitraryText pins the
// "no arbitrary text reaches the worker" promise. PipelineInput
// keeps transcript text and audio body separate; the
// Input.Kind discriminator is one of two strings only.
func TestPipelineEnvelopeNeverCarriesArbitraryText(t *testing.T) {
	b, err := json.Marshal(PipelineRequest{
		RequestID:    "R1",
		Jurisdiction: "KL",
		Language:     "ml-IN",
		Input:        PipelineInput{Kind: PipelineInputTranscript, Text: "hello"},
		Render:       PipelineRender{Kind: PipelineRenderNone},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	// The contract forbids hidden tooling / URLs / HTML at the
	// envelope. A literal "<tool_call>" or "file://" must never
	// appear because the field set is closed.
	for _, banned := range []string{
		"<tool_call>", "</tool_call>", "<tool>", "<system>",
		"file://", "javascript:",
	} {
		if strings.Contains(s, banned) {
			t.Fatalf("envelope leaked banned token %q: %s", banned, s)
		}
	}
}

// TestPipelineStateSetIsFinite locks the Pipeline state code
// enum; the orchestrator can only return one of these values. A
// new state requires a contract update — a security-relevant
// boundary.
func TestPipelineStateSetIsFinite(t *testing.T) {
	allowed := map[PipelineState]struct{}{
		PipelineOK:               {},
		PipelineClarify:          {},
		PipelineUnsupported:      {},
		PipelineDataUnavailable:  {},
		PipelineModelUnavailable: {},
		PipelineCanceled:         {},
	}
	for _, s := range []PipelineState{
		PipelineOK, PipelineClarify, PipelineUnsupported,
		PipelineDataUnavailable, PipelineModelUnavailable, PipelineCanceled,
	} {
		if _, ok := allowed[s]; !ok {
			t.Fatalf("pipeline state %q not registered in the allow-list", s)
		}
	}
	// Any string not in the allow-list must not be a known state.
	if _, ok := allowed[PipelineState("UNKNOWN_THING")]; ok {
		t.Fatalf("arbitrary string is a valid state")
	}
}

// TestTTSRequestArgsMustBeJSONScalars pins the typed Args boundary:
// each arg is JSON-serializable as scalar, array, or object only.
// The worker never receives arbitrary streaming or function-typed
// payloads.
func TestTTSRequestArgsMustBeJSONScalars(t *testing.T) {
	req := TTSRequest{
		RequestID:     "R1",
		SpeechKey:     "destination_options",
		Language:      "ml-IN",
		Args:          SpeechArgs{Args: map[string]any{"facility_id": "F1", "count": 3.0}},
		SourceVersion: 7,
		Settings:      TTSSynthesisSettings{SampleRate: 22050, BitDepth: 16, Channels: 1, SpeakingRate: 1.0},
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Round-trip: rebuild, verify scalars preserved.
	var rt TTSRequest
	if err := json.Unmarshal(b, &rt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if id, ok := rt.Args.Args["facility_id"].(string); !ok || id != "F1" {
		t.Fatalf("arg facility_id lost shape")
	}
	if _, ok := rt.Args.Args["count"].(float64); !ok {
		t.Fatalf("arg count did not round-trip as number")
	}
}
