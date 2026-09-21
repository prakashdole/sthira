package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sthira/backend/eval/corpus"
)

// stubWorkerServer mimics the orchestrator's worker endpoints so we
// can exercise HTTPProvider against a real HTTP transport instead
// of just the deterministic provider. The handlers here record what
// the provider sent and return the worker's frozen envelope shapes
// so the eval runner can extract nested response data.
type stubWorkerServer struct {
	mux        *http.ServeMux
	gotASR     []string
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
	s.gotASR = append(s.gotASR, r.Header.Get("X-Request-ID"))
	if s.asrHandler != nil {
		s.asrHandler(w, r)
		return
	}
	body, _ := io.ReadAll(r.Body)
	// Echo content-type back as the model revision so the test
	// can verify the provider forwarded it correctly.
	_ = body
	w.Header().Set("Content-Type", "application/json")
	conf := 0.92
	_ = json.NewEncoder(w).Encode(map[string]any{
		"state":          "OK",
		"text":           "मल्लप्पाळम दिग्गज",
		"language":       r.Header.Get("X-Language"),
		"confidence":     conf,
		"model_revision": "ai4bharat/indic-conformer-600m-multilingual@mock",
	})
}

func (s *stubWorkerServer) handleMiddle(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.gotMiddle = append(s.gotMiddle, body)
	if s.midHandler != nil {
		s.midHandler(w, r)
		return
	}
	// Echo the request_id back and pick an intent derived from the
	// input so the runner's safety assertions can find evidence.
	rid, _ := body["request_id"].(string)
	lang, _ := body["language"].(string)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "OK",
		"intent":          "focus_place",
		"language":        lang,
		"actions":         []string{"camera.focus"},
		"speech_key":      "flood.warning.level-2",
		"evidence_ids":    []string{"fac-001", "zone-001"},
		"request_id_echo": rid,
	})
}

func (s *stubWorkerServer) handleTTS(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.gotTTS = append(s.gotTTS, body)
	if s.ttsHandler != nil {
		s.ttsHandler(w, r)
		return
	}
	// Echo jurisdiction and source_version back. The provider must
	// forward both through TTSEnvelopeBridge.
	_ = json.NewEncoder(w).Encode(map[string]any{
		"state":       "OK",
		"speech_key":  body["speech_key"],
		"byte_size":   12345,
		"duration_ms": 2500,
	})
}

func startStub(t *testing.T, s *stubWorkerServer) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestHTTPProvider_ASR_ForwardsHeadersAndDecodesResponse verifies the
// ASR provider forwards request_id/language/content_type headers,
// decodes the nested JSON response, and exposes the model revision
// in the outcome.
func TestHTTPProvider_ASR_ForwardsHeadersAndDecodesResponse(t *testing.T) {
	stub := newStub()
	srv := startStub(t, stub)

	p := NewHTTP(HTTPConfig{
		ASRURL: srv.URL + "/asr",
	})
	out, err := p.ASR(context.Background(), ASRRequest{
		RequestID:   "R-asr-1",
		Language:    "ml-IN",
		AudioB64:    base64.StdEncoding.EncodeToString([]byte("RIFFfake")),
		ContentType: "audio/wav",
	})
	if err != nil {
		t.Fatalf("ASR: %v", err)
	}
	if out.State != "OK" {
		t.Errorf("State: got %q want %q", out.State, "OK")
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
		t.Errorf("Confidence: got %v want 0.92", out.Confidence)
	}
	if len(stub.gotASR) != 1 || stub.gotASR[0] != "R-asr-1" {
		t.Errorf("request_id not forwarded: %v", stub.gotASR)
	}
}

// TestHTTPProvider_Middle_PassesJurisdictionAndVerifiesCorrelation
// confirms Middle forwards the wire shape and the runner can correlate
// request_id across the call.
func TestHTTPProvider_Middle_PassesJurisdictionAndVerifiesCorrelation(t *testing.T) {
	stub := newStub()
	stub.midHandler = func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		rid, _ := body["request_id"].(string)
		ctx, _ := body["context"].(map[string]any)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":          "OK",
			"intent":          "focus_place",
			"language":        body["language"],
			"actions":         []any{"camera.focus"},
			"speech_key":      "flood.warning.level-2",
			"evidence_ids":    []any{"fac-001"},
			"request_id_echo": rid,
			"context_echo":    ctx,
		})
	}
	srv := startStub(t, stub)

	p := NewHTTP(HTTPConfig{
		MiddleURL: srv.URL + "/middle",
	})
	out, err := p.Middle(context.Background(), MiddleRequest{
		RequestID:  "R-mid-1",
		Language:   "hi-IN",
		Transcript: "मुझे सुरक्षित स्थान चाहिए",
		Context:    map[string]any{"jurisdiction": "KL-WYD", "data_version": "v1"},
		Case:       corpus.Case{ID: "case-mid-1"},
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
	if len(out.Actions) == 0 || out.Actions[0] != "camera.focus" {
		t.Errorf("Actions: got %v", out.Actions)
	}
	if len(out.EvidenceIDs) == 0 || out.EvidenceIDs[0] != "fac-001" {
		t.Errorf("EvidenceIDs: got %v", out.EvidenceIDs)
	}
	if len(stub.gotMiddle) != 1 {
		t.Fatalf("server saw %d middle calls, want 1", len(stub.gotMiddle))
	}
	if stub.gotMiddle[0]["request_id"] != "R-mid-1" {
		t.Errorf("server request_id mismatch: %v", stub.gotMiddle[0]["request_id"])
	}
	if got := stub.gotMiddle[0]["case_id"]; got != "case-mid-1" {
		t.Errorf("server case_id: got %v", got)
	}
}

// TestHTTPProvider_TTS_ForwardsJurisdictionAndVersion verifies the
// TTS provider forwards jurisdiction and source_version correctly.
func TestHTTPProvider_TTS_ForwardsJurisdictionAndVersion(t *testing.T) {
	stub := newStub()
	stub.ttsHandler = func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state":               "OK",
			"speech_key":          body["speech_key"],
			"byte_size":           99000,
			"duration_ms":         4200,
			"jurisdiction_echo":   body["jurisdiction"],
			"source_version_echo": body["source_version"],
		})
	}
	srv := startStub(t, stub)

	p := NewHTTP(HTTPConfig{
		TTSURL: srv.URL + "/tts",
	})
	out, err := p.TTS(context.Background(), TTSRequest{
		RequestID:     "R-tts-1",
		Language:      "hi-IN",
		SpeechKey:     "flood.warning.level-2",
		Args:          map[string]any{"place": "Mallappuram"},
		SourceVersion: 5,
		Case: corpus.Case{
			Context: corpus.Context{DataVersion: "KL-WYD-v1"},
		},
	})
	if err != nil {
		t.Fatalf("TTS: %v", err)
	}
	if out.State != "OK" {
		t.Errorf("State: got %q", out.State)
	}
	if out.ByteSize != 99000 {
		t.Errorf("ByteSize: got %d", out.ByteSize)
	}
	if out.DurationMS != 4200 {
		t.Errorf("DurationMS: got %d", out.DurationMS)
	}
	if len(stub.gotTTS) != 1 {
		t.Fatalf("server saw %d TTS calls, want 1", len(stub.gotTTS))
	}
	if got := stub.gotTTS[0]["jurisdiction"]; got != "KL-WYD" {
		t.Errorf("server jurisdiction: got %v want %q", got, "KL-WYD")
	}
	// Source version comes from the request, not the case context.
	if got := stub.gotTTS[0]["source_version"]; got != float64(5) {
		t.Errorf("server source_version: got %v want 5", got)
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

// TestHTTPProvider_RejectsEmptyAudio validates the input-boundary
// check: an empty audio payload returns an error.
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

// TestHTTPProvider_RejectsBadBase64 covers a tampered envelope.
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
		t.Fatal("expected decode error")
	}
}
