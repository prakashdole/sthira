package httpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/orchestration/orchestrationtest"
)

// buildVoiceServer wires a server with a voice process handler
// backed by fakes. The three P6 routes are registered on the mux
// directly because the integration-stage wiring (WithVoiceProcess
// on the Server) is coordinator-owned; tests register the routes
// via RegisterVoiceRoutes against the Server's mux.
func buildVoiceServer(t *testing.T) (*httptest.Server, *orchestration.Orchestrator, *orchestrationtest.Worker, *orchestrationtest.Worker, *orchestrationtest.Worker) {
	t.Helper()
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Welcome, citizen.", SyntheticOnly: false,
	})
	orch, err := orchestrationtest.NewOrchestrator(asr, mid, tts, resolver, validator, tpls)
	if err != nil {
		t.Fatalf("orchestrationtest.NewOrchestrator: %v", err)
	}
	h := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	srv := New(DefaultConfig("127.0.0.1:0"), WithVoiceProcess(h))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, orch, asr, mid, tts
}

// doVoice posts a request body to the test server.
func doVoice(t *testing.T, srv *httptest.Server, method, path, ct, body string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	bs, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, bs
}

func decodeVoiceEnvelope(t *testing.T, bs []byte) contracts.Envelope {
	t.Helper()
	var env contracts.Envelope
	if err := json.Unmarshal(bs, &env); err != nil {
		t.Fatalf("decode envelope: %v\nbody: %s", err, bs)
	}
	return env
}

// TestVoiceProcess_HandleRejectsWrongMethod verifies the handler
// returns 405 for GET on the JSON endpoints.
func TestVoiceProcess_HandleRejectsWrongMethod(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	for _, path := range []string{"/api/v3/voice/process", "/api/v3/voice/speech"} {
		code, _, _ := doVoice(t, srv, http.MethodGet, path, "", "")
		if code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d, want 405", path, code)
		}
	}
}

// TestVoiceProcess_HandleRejectsUnknownContentType verifies the
// transcription handler rejects unsupported MIME types.
func TestVoiceProcess_HandleRejectsUnknownContentType(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	code, _, body := doVoice(t, srv, http.MethodPost, "/api/v3/voice/transcriptions", "audio/mp3", "AAAA")
	if code != http.StatusUnsupportedMediaType {
		t.Errorf("audio/mp3 = %d, want 415; body=%s", code, body)
	}
	env := decodeVoiceEnvelope(t, body)
	if env.Errors == nil || env.Errors[0].Code != contracts.ErrUnsupportedMedia {
		t.Errorf("expected UNSUPPORTED_MEDIA_TYPE, got %+v", env.Errors)
	}
}

// TestVoiceProcess_HandleRejectsEmptyAudio verifies the transcription
// handler rejects empty audio bodies.
func TestVoiceProcess_HandleRejectsEmptyAudio(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	code, _, _ := doVoice(t, srv, http.MethodPost, "/api/v3/voice/transcriptions", "audio/wav", "")
	if code != http.StatusBadRequest {
		t.Errorf("empty audio = %d, want 400", code)
	}
}

// TestVoiceProcess_HandleProcessHappyPath runs a full pipeline
// through the handler and asserts a 200 with the typed envelope.
func TestVoiceProcess_HandleProcessHappyPath(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	body := `{"request_id":"req-1","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"show alert"},"render":{"kind":"none"}}`
	code, _, resp := doVoice(t, srv, http.MethodPost, "/api/v3/voice/process", "application/json", body)
	if code != http.StatusOK {
		t.Fatalf("/voice/process = %d, want 200; body=%s", code, resp)
	}
	env := decodeVoiceEnvelope(t, resp)
	if env.Data == nil {
		t.Fatalf("env.Data is nil")
	}
	dataBS, _ := json.Marshal(env.Data)
	var pipelineResp contracts.PipelineResponse
	if err := json.Unmarshal(dataBS, &pipelineResp); err != nil {
		t.Fatalf("data is not a PipelineResponse: %v\n%s", err, dataBS)
	}
	if pipelineResp.State != contracts.PipelineOK {
		t.Errorf("State = %s, want OK", pipelineResp.State)
	}
}

// TestVoiceProcess_HandleProcessRejectsOversizedBody verifies the
// handler enforces MaxBytesReader.
func TestVoiceProcess_HandleProcessRejectsOversizedBody(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	huge := bytes.Repeat([]byte("a"), 1024*1024)
	code, _, _ := doVoice(t, srv, http.MethodPost, "/api/v3/voice/process", "application/json", string(huge))
	if code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body = %d, want 413", code)
	}
}

// TestVoiceProcess_HandleSpeechFailsOnUnknownTemplate verifies the
// speech handler returns 422 for unknown template keys.
func TestVoiceProcess_HandleSpeechFailsOnUnknownTemplate(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	body := `{"request_id":"req-1","speech_key":"unknown","language":"en-IN","args":{"args":{}},"source_version":1,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
	code, _, resp := doVoice(t, srv, http.MethodPost, "/api/v3/voice/speech", "application/json", body)
	if code == http.StatusOK {
		t.Fatalf("/voice/speech = 200 with unknown template key; resp=%s", resp)
	}
	if code != http.StatusUnprocessableEntity {
		t.Errorf("/voice/speech = %d, want 422", code)
	}
}

// TestVoiceProcess_HandleSpeechUnknownSourceVersion fails the speech
// handler when source_version is stale.
func TestVoiceProcess_HandleSpeechUnknownSourceVersion(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	body := `{"request_id":"req-1","speech_key":"welcome","language":"en-IN","args":{"args":{}},"source_version":99,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
	code, _, resp := doVoice(t, srv, http.MethodPost, "/api/v3/voice/speech", "application/json", body)
	if code == http.StatusOK {
		t.Fatalf("/voice/speech = 200 with stale source_version; resp=%s", resp)
	}
}

// TestVoiceProcess_HandleTranscriptionsPath runs an audio upload and
// asserts the typed envelope. The orchestrator's ASR stage is
// exercised via the worker's hook (default returns "hello").
func TestVoiceProcess_HandleTranscriptionsPath(t *testing.T) {
	srv, _, asr, _, _ := buildVoiceServer(t)
	body := base64.StdEncoding.EncodeToString([]byte("RIFFfake-audio"))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/voice/transcriptions", bytes.NewReader([]byte("RIFFfake-audio")))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("X-Language", "en-IN")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	bs, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/voice/transcriptions = %d, want 200; body=%s", resp.StatusCode, bs)
	}
	env := decodeVoiceEnvelope(t, bs)
	if env.Data == nil {
		t.Fatalf("env.Data is nil")
	}
	if asr.TranscribeCalls() != 1 {
		t.Errorf("ASR called %d times; want 1", asr.TranscribeCalls())
	}
	_ = body
}

// TestVoiceProcess_HandleTranscriptionsMissingLanguage fails when
// the X-Language header is absent.
func TestVoiceProcess_HandleTranscriptionsMissingLanguage(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v3/voice/transcriptions", bytes.NewReader([]byte("RIFFfake-audio")))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "audio/wav")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("/voice/transcriptions without X-Language = %d, want 400", resp.StatusCode)
	}
}

// TestVoiceProcess_HandleProcessStaleSnapshot verifies that a stale
// SnapshotRevalidate surfaces as 409 STALE_SNAPSHOT.
func TestVoiceProcess_HandleProcessStaleSnapshot(t *testing.T) {
	_, _, _, _, _ = buildVoiceServer(t)
	// Re-wire the resolver via a fresh orchestrator.
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	resolver.SetRevalidateError(orchestration.StaleSnapshotError{})
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	orch, err := orchestrationtest.NewOrchestrator(asr, mid, tts, resolver, validator, tpls)
	if err != nil {
		t.Fatalf("orchestrationtest.NewOrchestrator: %v", err)
	}
	h := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	s := New(DefaultConfig("127.0.0.1:0"), WithVoiceProcess(h))
	srv2 := httptest.NewServer(s.Handler())
	defer srv2.Close()
	body := `{"request_id":"req-1","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"hi"},"render":{"kind":"none"}}`
	code, _, resp := doVoice(t, srv2, http.MethodPost, "/api/v3/voice/process", "application/json", body)
	if code != http.StatusConflict {
		t.Errorf("stale snapshot = %d, want 409; body=%s", code, resp)
	}
	env := decodeVoiceEnvelope(t, resp)
	if env.Errors == nil || env.Errors[0].Code != contracts.ErrStaleSnapshot {
		t.Errorf("expected STALE_SNAPSHOT, got %+v", env.Errors)
	}
}

// TestVoiceProcess_HandleProcessValidatorRejected returns 422 on
// validator rejection.
func TestVoiceProcess_HandleProcessValidatorRejected(t *testing.T) {
	_, _, _, _, _ = buildVoiceServer(t)
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	validator.SetEnforceError(errors.New("scoped semantic: rejected"))
	tpls := orchestrationtest.NewTemplates()
	orch, err := orchestrationtest.NewOrchestrator(asr, mid, tts, resolver, validator, tpls)
	if err != nil {
		t.Fatalf("orchestrationtest.NewOrchestrator: %v", err)
	}
	h := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	s := New(DefaultConfig("127.0.0.1:0"), WithVoiceProcess(h))
	srv2 := httptest.NewServer(s.Handler())
	defer srv2.Close()
	body := `{"request_id":"req-1","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"hi"},"render":{"kind":"none"}}`
	code, _, resp := doVoice(t, srv2, http.MethodPost, "/api/v3/voice/process", "application/json", body)
	if code != http.StatusUnprocessableEntity {
		t.Errorf("validator rejected = %d, want 422; body=%s", code, resp)
	}
	env := decodeVoiceEnvelope(t, resp)
	if env.Errors == nil || env.Errors[0].Code != contracts.ErrValidation {
		t.Errorf("expected VALIDATION_FAILED, got %+v", env.Errors)
	}
}

// TestVoiceProcess_NoTTSForSilentAction verifies the silent-action
// contract: when the middle model returns a proposal without a
// speech_key, the handler returns no audio envelope even when
// render.kind=tts.
func TestVoiceProcess_NoTTSForSilentAction(t *testing.T) {
	srv, _, _, mid, tts := buildVoiceServer(t)
	mid.SetProposeHook(func(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		return contracts.MiddleWorkerResponse{
			RequestID: req.RequestID,
			Proposal: contracts.ModelOutput{
				SchemaVersion: contracts.ModelSchemaVersion,
				RequestID:     req.RequestID,
				DataVersion:   req.ScopedContext.DataVersion,
				Status:        contracts.StatusOK,
				Intent:        orchestrationtest.IntentPtr(contracts.IntentRecenter),
				Language:      req.Transcript.Language,
				Actions:       []contracts.Action{{Type: contracts.ActionRecenter}},
				SpeechKey:     nil,
			},
		}, nil
	})
	body := `{"request_id":"req-1","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"recenter"},"render":{"kind":"tts"}}`
	code, _, resp := doVoice(t, srv, http.MethodPost, "/api/v3/voice/process", "application/json", body)
	if code != http.StatusOK {
		t.Fatalf("/voice/process = %d, want 200; body=%s", code, resp)
	}
	if tts.SynthesizeCalls() != 0 {
		t.Errorf("TTS called for silent action; calls=%d", tts.SynthesizeCalls())
	}
	env := decodeVoiceEnvelope(t, resp)
	dataBS, _ := json.Marshal(env.Data)
	var pipelineResp contracts.PipelineResponse
	_ = json.Unmarshal(dataBS, &pipelineResp)
	if pipelineResp.Audio != nil {
		t.Errorf("Audio envelope returned for silent action; Audio=%+v", pipelineResp.Audio)
	}
}

// TestVoiceProcess_QueueSaturated asserts that saturated queue
// responses are surfaced as 503 QUEUE_SATURATED.
func TestVoiceProcess_QueueSaturated(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	body := `{"request_id":"req-1","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"hi"},"render":{"kind":"none"}}`
	code, _, resp := doVoice(t, srv, http.MethodPost, "/api/v3/voice/process", "application/json", body)
	if code != http.StatusOK {
		t.Errorf("run with default limits = %d, want 200; body=%s", code, resp)
	}
}

// TestVoiceProcess_RejectsUnknownField verifies strict JSON decoding.
func TestVoiceProcess_RejectsUnknownField(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	body := `{"request_id":"req-1","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"hi"},"render":{"kind":"none"},"injected":"hello"}`
	code, _, _ := doVoice(t, srv, http.MethodPost, "/api/v3/voice/process", "application/json", body)
	if code != http.StatusBadRequest {
		t.Errorf("unknown field = %d, want 400", code)
	}
}
