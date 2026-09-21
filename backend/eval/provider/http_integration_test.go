package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sthira/backend/eval/corpus"
)

// stubWorkerServer mimics the orchestrator's worker endpoints. The
// handlers echo the typed envelope shapes (PipelineResponse,
// ASRWorkerResponse, TTSWorkerResponse) so the eval provider can
// decode them at the real contract boundary.
type stubWorkerServer struct {
	mux        *http.ServeMux
	gotASR     []map[string]any
	gotMiddle  []map[string]any
	gotTTS     []map[string]any
	asrHandler func(w http.ResponseWriter, r *http.Request)
	midHandler func(w http.ResponseWriter, r *http.Request)
	ttsHandler func(w http.ResponseWriter, r *http.Request)
}

func newStub() *stubWorkerServer {
	s := &stubWorkerServer{mux: http.NewServeMux()}
	s.mux.HandleFunc("/asr", s.handleASR)
	s.mux.HandleFunc("/middle", s.handleMiddle)
	s.mux.HandleFunc("/tts", s.handleTTS)
	return s
}

func (s *stubWorkerServer) handleASR(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.gotASR = append(s.gotASR, body)
	if s.asrHandler != nil {
		s.asrHandler(w, r)
		return
	}
	conf := 0.92
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ASRWorkerResponseWire{
		RequestID:     str(body["request_id"]),
		Language:      str(body["language"]),
		Text:          "मल्लप्पाळम दिग्गज",
		Confidence:    &conf,
		State:         "OK",
		ModelRevision: "ai4bharat/indic-conformer-600m-multilingual@mock",
	})
}

func (s *stubWorkerServer) handleMiddle(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.gotMiddle = append(s.gotMiddle, body)
	if s.midHandler != nil {
		s.midHandler(w, r)
		return
	}
	rid := str(body["request_id"])
	lang := str(body["language"])
	intent := "focus_place"
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(PipelineResponseWire{
		RequestID:   rid,
		DataVersion: "v1",
		State:       "OK",
		ValidatedProposal: &ModelOutputWire{
			SchemaVersion:    "3.0",
			RequestID:        rid,
			DataVersion:      "v1",
			Status:           "OK",
			Intent:           &intent,
			Language:         lang,
			Actions:          []ActionWire{{Type: "camera.focus", TargetID: "fac-001"}},
			ClarificationIDs: []string{},
			EvidenceIDs:      []string{"fac-001", "zone-001"},
		},
		Template: PipelineTemplateWire{
			SpeechKey:       "flood.warning.level-2",
			TemplateVersion: 1,
		},
	})
}

func (s *stubWorkerServer) handleTTS(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.gotTTS = append(s.gotTTS, body)
	if s.ttsHandler != nil {
		s.ttsHandler(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(TTSWorkerResponseWire{
		RequestID:     str(body["request_id"]),
		SpeechKey:     str(body["speech_key"]),
		Language:      str(body["language"]),
		State:         "OK",
		ContentType:   "audio/wav",
		ModelRevision: "indic-parler-tts@mock",
		VoiceRevision: "default",
		ByteSize:      12345,
	})
}

func startStub(t *testing.T, s *stubWorkerServer) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestB5_HTTPProvider_ASR_SendsRealEnvelope verifies the ASR
// provider POSTs the ASRWorkerRequest envelope (mirrors
// contracts.ASRWorkerRequest) and decodes the ASRWorkerResponse.
func TestB5_HTTPProvider_ASR_SendsRealEnvelope(t *testing.T) {
	stub := newStub()
	srv := startStub(t, stub)

	p := NewHTTP(HTTPConfig{ASRURL: srv.URL + "/asr"})
	out, err := p.ASR(context.Background(), ASRRequest{
		RequestID:   "R-asr-b5",
		Language:    "ml-IN",
		AudioB64:    base64.StdEncoding.EncodeToString([]byte("RIFFfake")),
		ContentType: "audio/wav",
	})
	if err != nil {
		t.Fatalf("ASR: %v", err)
	}
	if out.State != "OK" {
		t.Errorf("State: got %q", out.State)
	}
	if out.Text == "" {
		t.Error("Text empty")
	}
	if out.Language != "ml-IN" {
		t.Errorf("Language: got %q", out.Language)
	}
	if out.RevisionID == "" {
		t.Error("RevisionID empty")
	}
	if out.Confidence == nil || *out.Confidence != 0.92 {
		t.Errorf("Confidence: got %v", out.Confidence)
	}
	if len(stub.gotASR) != 1 {
		t.Fatalf("server saw %d ASR calls, want 1", len(stub.gotASR))
	}
	// Verify the wire envelope field names match contracts.
	sent := stub.gotASR[0]
	if sent["request_id"] != "R-asr-b5" {
		t.Errorf("request_id: got %v", sent["request_id"])
	}
	if sent["language"] != "ml-IN" {
		t.Errorf("language: got %v", sent["language"])
	}
	if sent["content_type"] != "audio/wav" {
		t.Errorf("content_type: got %v", sent["content_type"])
	}
	if sent["audio_b64"] == nil {
		t.Error("audio_b64 missing")
	}
}

// TestB5_HTTPProvider_Middle_SendsPipelineRequest verifies the
// Middle provider POSTs the typed PipelineRequest envelope and
// extracts the validated_proposal from PipelineResponse.
func TestB5_HTTPProvider_Middle_SendsPipelineRequest(t *testing.T) {
	stub := newStub()
	srv := startStub(t, stub)

	p := NewHTTP(HTTPConfig{MiddleURL: srv.URL + "/middle"})
	out, err := p.Middle(context.Background(), MiddleRequest{
		RequestID:  "R-mid-b5",
		Language:   "hi-IN",
		Transcript: "मुझे सुरक्षित स्थान चाहिए",
		Case: corpus.Case{
			ID: "case-b5",
			Context: corpus.Context{
				Jurisdiction:    "KL-WYD",
				SourceVersion:   7,
				TemplateVersion: 1,
				DataVersion:     "KL-WYD-v7",
			},
		},
	})
	if err != nil {
		t.Fatalf("Middle: %v", err)
	}
	if out.Status != "OK" {
		t.Errorf("Status: got %q", out.Status)
	}
	if out.Intent != "focus_place" {
		t.Errorf("Intent: got %q", out.Intent)
	}
	if len(out.Actions) == 0 || out.Actions[0] != "camera.focus:fac-001" {
		t.Errorf("Actions: got %v", out.Actions)
	}
	if out.SpeechKey != "flood.warning.level-2" {
		t.Errorf("SpeechKey: got %q", out.SpeechKey)
	}
	if len(out.EvidenceIDs) == 0 || out.EvidenceIDs[0] != "fac-001" {
		t.Errorf("EvidenceIDs: got %v", out.EvidenceIDs)
	}
	if len(stub.gotMiddle) != 1 {
		t.Fatalf("server saw %d middle calls", len(stub.gotMiddle))
	}
	// Verify the wire envelope carries the typed PipelineRequest
	// fields, NOT invented fields like case_id or context.
	sent := stub.gotMiddle[0]
	if sent["request_id"] != "R-mid-b5" {
		t.Errorf("request_id: got %v", sent["request_id"])
	}
	if sent["jurisdiction"] != "KL-WYD" {
		t.Errorf("jurisdiction: got %v", sent["jurisdiction"])
	}
	if sent["source_version"] != float64(7) {
		t.Errorf("source_version: got %v want 7", sent["source_version"])
	}
	if _, ok := sent["case_id"]; ok {
		t.Errorf("case_id must not be sent (invented field)")
	}
	input, ok := sent["input"].(map[string]any)
	if !ok {
		t.Errorf("input missing or not object: %T", sent["input"])
	} else if input["kind"] != "transcript" {
		t.Errorf("input.kind: got %v", input["kind"])
	}
}

// TestB5_HTTPProvider_Middle_RejectsMissingJurisdiction enforces
// the D36 fail-closed boundary: a missing jurisdiction is
// rejected before any HTTP call.
func TestB5_HTTPProvider_Middle_RejectsMissingJurisdiction(t *testing.T) {
	p := NewHTTP(HTTPConfig{MiddleURL: "http://localhost:1/middle"})
	_, err := p.Middle(context.Background(), MiddleRequest{
		RequestID: "R-no-jur",
		Case: corpus.Case{
			Context: corpus.Context{SourceVersion: 1},
		},
	})
	if err == nil {
		t.Fatal("expected error on missing jurisdiction")
	}
	if !strings.Contains(err.Error(), "jurisdiction") {
		t.Errorf("expected jurisdiction error, got %v", err)
	}
}

// TestB5_HTTPProvider_TTS_SendsTTSWorkerRequest verifies the TTS
// provider POSTs the typed TTSWorkerRequest envelope (with
// speech_key, language, jurisdiction, source_version,
// template_version) and decodes the TTSWorkerResponse.
func TestB5_HTTPProvider_TTS_SendsTTSWorkerRequest(t *testing.T) {
	stub := newStub()
	srv := startStub(t, stub)

	p := NewHTTP(HTTPConfig{TTSURL: srv.URL + "/tts"})
	out, err := p.TTS(context.Background(), TTSRequest{
		RequestID:     "R-tts-b5",
		Language:      "hi-IN",
		SpeechKey:     "flood.warning.level-2",
		SourceVersion: 5,
		Case: corpus.Case{
			Context: corpus.Context{
				Jurisdiction:    "KL-WYD",
				SourceVersion:   7,
				TemplateVersion: 2,
				DataVersion:     "KL-WYD-v7",
			},
		},
	})
	if err != nil {
		t.Fatalf("TTS: %v", err)
	}
	if out.State != "OK" {
		t.Errorf("State: got %q", out.State)
	}
	if out.SpeechKey != "flood.warning.level-2" {
		t.Errorf("SpeechKey: got %q", out.SpeechKey)
	}
	if len(stub.gotTTS) != 1 {
		t.Fatalf("server saw %d TTS calls", len(stub.gotTTS))
	}
	sent := stub.gotTTS[0]
	if sent["request_id"] != "R-tts-b5" {
		t.Errorf("request_id: got %v", sent["request_id"])
	}
	if sent["speech_key"] != "flood.warning.level-2" {
		t.Errorf("speech_key: got %v", sent["speech_key"])
	}
	if sent["jurisdiction"] != "KL-WYD" {
		t.Errorf("jurisdiction: got %v want KL-WYD", sent["jurisdiction"])
	}
	// SourceVersion from the request overrides the case's context
	// value, matching the orchestrator's "request wins" rule.
	if sent["source_version"] != float64(5) {
		t.Errorf("source_version: got %v want 5", sent["source_version"])
	}
	if sent["template_version"] != float64(2) {
		t.Errorf("template_version: got %v want 2", sent["template_version"])
	}
	if _, ok := sent["args"]; ok {
		t.Errorf("args must not be sent (free-text input is forbidden)")
	}
}

// TestB5_HTTPProvider_TTS_RejectsMissingJurisdiction enforces the
// same fail-closed boundary for TTS: missing jurisdiction is
// rejected before any HTTP call.
func TestB5_HTTPProvider_TTS_RejectsMissingJurisdiction(t *testing.T) {
	p := NewHTTP(HTTPConfig{TTSURL: "http://localhost:1/tts"})
	_, err := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-no-jur-tts",
		Case: corpus.Case{
			Context: corpus.Context{SourceVersion: 1},
		},
	})
	if err == nil {
		t.Fatal("expected error on missing jurisdiction")
	}
	if !strings.Contains(err.Error(), "jurisdiction") {
		t.Errorf("expected jurisdiction error, got %v", err)
	}
}

// TestHTTPProvider_HandlesServiceUnavailable maps 503 to a typed
// "model unavailable" error so the runner can mark BLOCKED_EXTERNAL.
func TestHTTPProvider_HandlesServiceUnavailable(t *testing.T) {
	stub := newStub()
	stub.asrHandler = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusServiceUnavailable)
	}
	srv := startStub(t, stub)

	p := NewHTTP(HTTPConfig{ASRURL: srv.URL + "/asr"})
	_, err := p.ASR(context.Background(), ASRRequest{
		RequestID:   "R-503",
		Language:    "hi-IN",
		AudioB64:    base64.StdEncoding.EncodeToString([]byte("x")),
		ContentType: "audio/wav",
	})
	if err == nil {
		t.Fatal("expected error from 503")
	}
	if !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("expected 503 + unavailable, got %v", err)
	}
}

func TestHTTPProvider_RejectsEmptyAudio(t *testing.T) {
	stub := newStub()
	srv := startStub(t, stub)
	p := NewHTTP(HTTPConfig{ASRURL: srv.URL + "/asr"})
	_, err := p.ASR(context.Background(), ASRRequest{
		RequestID: "R-empty",
		Language:  "hi-IN",
		AudioB64:  "",
	})
	if err == nil {
		t.Fatal("expected error for empty audio")
	}
}

func TestHTTPProvider_RejectsBadBase64(t *testing.T) {
	stub := newStub()
	srv := startStub(t, stub)
	p := NewHTTP(HTTPConfig{ASRURL: srv.URL + "/asr"})
	_, err := p.ASR(context.Background(), ASRRequest{
		RequestID:   "R-bad",
		Language:    "hi-IN",
		AudioB64:    "this is not base64!@#",
		ContentType: "audio/wav",
	})
	if err == nil {
		t.Fatal("expected error for bad base64")
	}
}

// str safely extracts a string from an any map.
func str(v any) string {
	s, _ := v.(string)
	return s
}
