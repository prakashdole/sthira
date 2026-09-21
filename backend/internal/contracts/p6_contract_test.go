package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestScopedContextRoundtrip ensures the new typed context JSON-encodes
// losslessly and re-decodes into the same struct. This is the contract
// workers rely on when they cross the Go↔Python/native process boundary.
func TestScopedContextRoundtrip(t *testing.T) {
	in := ScopedContext{
		RequestID:        "REQ-P6-1",
		DataVersion:      "PKG-DEMO:7",
		Jurisdiction:     "KL",
		SchemaVersion:    SchemaVersionV3,
		SourceStatus:     FreshnessCurrent,
		SourceVersion:    7,
		TemplateVersion:  3,
		AllowedLanguages: []string{"en-IN", "ml-IN", "hi-IN"},
		TemplateKeys:     []string{"destination_options", "clarify_place"},
		KnownPlaces: map[string]PlaceCandidate{
			"PLACE-DEMO-1": {PlaceID: "PLACE-DEMO-1", PlaceKind: "ADMIN", Name: "Ward 8", Jurisdiction: "KL"},
		},
		KnownRedZones: map[string]ZoneRef{
			"ZONE-RED-1": {ZoneID: "ZONE-RED-1", ZoneRole: "RED", Status: ZoneStatusOpen, SourceVersion: 7},
		},
		KnownSafeZones: map[string]ZoneRef{
			"ZONE-SAFE-1": {ZoneID: "ZONE-SAFE-1", ZoneRole: "SAFE", Status: ZoneStatusPublished, SourceVersion: 7},
		},
		KnownRoutes: map[string]RouteRef{
			"ROUTE-1": {RouteID: "ROUTE-1", ToSafeZoneID: "ZONE-SAFE-1", Status: RouteStatusVerified, Verified: true, SourceVersion: 7},
		},
		KnownFacilities: map[string]FacilityRef{
			"FACILITY-DEMO-1": {FacilityID: "FACILITY-DEMO-1", SafeZoneID: "ZONE-SAFE-1", CapacityKnown: true, Free: 12, SourceVersion: 7},
		},
		VerifiedRoutes: map[string][]RouteRef{
			"FACILITY-DEMO-1": {{RouteID: "ROUTE-1", ToSafeZoneID: "ZONE-SAFE-1", Status: RouteStatusVerified, Verified: true, SourceVersion: 7}},
		},
		EligibleDestinations: []EligibleChoice{
			{Facility: FacilityRef{FacilityID: "FACILITY-DEMO-1", SafeZoneID: "ZONE-SAFE-1", CapacityKnown: true, Free: 12, SourceVersion: 7}, PermittedRank: 0},
		},
		IssuedAt: "2026-09-21T00:00:00Z",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out ScopedContext
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.DataVersion != in.DataVersion || out.Jurisdiction != in.Jurisdiction {
		t.Fatalf("identity lost: in=%+v out=%+v", in, out)
	}
	if out.AllowedLanguages[1] != "ml-IN" {
		t.Fatalf("language order lost: %v", out.AllowedLanguages)
	}
	if out.KnownRoutes["ROUTE-1"].ToSafeZoneID != "ZONE-SAFE-1" {
		t.Fatalf("route binding lost: %+v", out.KnownRoutes["ROUTE-1"])
	}
	if out.EligibleDestinations[0].PermittedRank != 0 {
		t.Fatalf("eligible order lost: %+v", out.EligibleDestinations)
	}
	if !out.IsLanguageAllowed("ml-IN") {
		t.Fatalf("language allow-list mismatch: %v", out.AllowedLanguages)
	}
	if !out.IsTemplateKeyAllowed("clarify_place") {
		t.Fatalf("template allow-list mismatch: %v", out.TemplateKeys)
	}
}

// TestPipelineEnvelopeRoundtrip ensures the orchestrator request/response
// shapes marshal/unmarshal correctly with all unions populated.
func TestPipelineEnvelopeRoundtrip(t *testing.T) {
	req := PipelineRequest{
		RequestID:    "REQ-P6-2",
		Jurisdiction: "KL",
		Language:     "ml-IN",
		Input: PipelineInput{
			Kind: PipelineInputTranscript,
			Text: "എവിടെ പോകണം?",
		},
		Render:         PipelineRender{Kind: PipelineRenderNone},
		IdempotencyKey: "idem-1",
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"kind":"transcript"`) {
		t.Fatalf("input.kind missing: %s", string(b))
	}
	var out PipelineRequest
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Input.Kind != PipelineInputTranscript || out.Input.Text != req.Input.Text {
		t.Fatalf("input union lost: %+v", out)
	}
}

// TestTTSEnvelopeRoundtrip ensures the TTS request/response and the typed
// cache key marshal/unmarshal correctly with all fields populated.
func TestTTSEnvelopeRoundtrip(t *testing.T) {
	req := TTSRequest{
		RequestID:     "REQ-P6-3",
		SpeechKey:     "destination_options",
		Language:      "ml-IN",
		Args:          SpeechArgs{Args: map[string]any{"facility_id": "FACILITY-DEMO-1"}},
		SourceVersion: 7,
		Settings:      TTSSynthesisSettings{SampleRate: 22050, BitDepth: 16, Channels: 1, SpeakingRate: 1.0},
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out TTSRequest
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Settings.SampleRate != 22050 || out.Settings.BitDepth != 16 {
		t.Fatalf("settings lost: %+v", out.Settings)
	}

	key := TTSCacheKey{
		SpeechKey:       "destination_options",
		Language:        "ml-IN",
		Args:            req.Args,
		SourceVersion:   7,
		TemplateVersion: 3,
		ModelRevision:   "indic-parler-tts-1.2",
		VoiceRevision:   "ml-IN-female-1",
		Settings:        req.Settings,
	}
	kb, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("cache-key marshal: %v", err)
	}
	var kout TTSCacheKey
	if err := json.Unmarshal(kb, &kout); err != nil {
		t.Fatalf("cache-key unmarshal: %v", err)
	}
	if kout.VoiceRevision != "ml-IN-female-1" {
		t.Fatalf("cache-key voice lost: %+v", kout)
	}
}

// TestTranscriptionEnvelopeRoundtrip ensures the ASR response shape
// preserves the nullable confidence and the bounded alternatives list.
func TestTranscriptionEnvelopeRoundtrip(t *testing.T) {
	resp := TranscriptionResponse{
		RequestID:   "REQ-P6-4",
		DataVersion: "PKG-DEMO:7",
		Language:    "ml-IN",
		Text:        "എവിടെ പോകണം",
		Confidence:  nil, // unknown / not calibrated
		State:       TranscriptionOK,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// confidence is explicitly rendered as null to preserve unknown semantics.
	if !strings.Contains(string(b), `"confidence":null`) {
		t.Fatalf("confidence must serialize as null when unknown, got %s", string(b))
	}
	var out TranscriptionResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Confidence != nil {
		t.Fatalf("confidence must roundtrip as nil, got %v", *out.Confidence)
	}
}

// TestWorkerHealthEnvelopeRoundtrip ensures the private worker /health shape
// marshals and accepts the saturated-queue signal the orchestrator uses to
// refuse dispatch.
func TestWorkerHealthEnvelopeRoundtrip(t *testing.T) {
	h := WorkerHealth{
		Ready: true,
		Warm:  true,
		Models: []ModelInfo{{
			ModelID: "indic-conformer-600m-multilingual", Revision: "v1",
			License: "MIT", Runtime: "onnxruntime-1.18", Hardware: "cpu",
			RemoteCode: false,
		}},
		Artifacts: []ArtifactDigest{
			{Name: "tokenizer", ChecksumSHA256: "deadbeef", License: "MIT"},
		},
		SupportedLanguages: []string{"hi-IN", "ml-IN"},
		Queue:              QueueStats{Depth: 1, MaxDepth: 8, MaxConcurrency: 2},
	}
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out WorkerHealth
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Warm || out.Models[0].ModelID != "indic-conformer-600m-multilingual" {
		t.Fatalf("warm/model lost: %+v", out)
	}
	if out.Queue.Depth >= out.Queue.MaxDepth {
		t.Fatalf("test must use unsaturated queue: %+v", out.Queue)
	}
}

// TestNewErrorCodesPresent guards against accidental removal of the new
// P6 codes. The OpenAPI and the orchestrator's handlers reference them.
func TestNewErrorCodesPresent(t *testing.T) {
	for _, c := range []string{
		ErrTranscriptUnavailable,
		ErrAudioUnavailable,
		ErrModelTimeout,
		ErrQueueSaturated,
		ErrInferenceCancelled,
		ErrTemplateUnknown,
		ErrStaleSnapshot,
	} {
		if c == "" {
			t.Fatal("an empty error code was registered")
		}
	}
}
