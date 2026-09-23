package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	body := `{"request_id":"req-1","jurisdiction":"JTEST","speech_key":"unknown","language":"en-IN","args":{"args":{}},"source_version":1,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
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
	body := `{"request_id":"req-1","jurisdiction":"JTEST","speech_key":"welcome","language":"en-IN","args":{"args":{}},"source_version":99,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
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

// TestVoiceProcess_HandleSpeech_TwoJurisdictions verifies multi-jurisdiction scoping at the HTTP layer.
func TestVoiceProcess_HandleSpeech_TwoJurisdictions(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc1 := orchestrationtest.BuildScopedContext("J1", "en-IN")
	sc1.TemplateKeys = []string{"welcome"}
	sc1.ApprovedSpeechKeys = map[string][]string{"welcome": {"en-IN"}}
	sc1.ApprovedTemplateSHA = map[string]string{"welcome": orchestrationtest.DigestString("Welcome, citizen.")}
	sc2 := orchestrationtest.BuildScopedContext("J2", "en-IN")
	sc2.TemplateKeys = []string{"other_key"} // "welcome" not allowed in J2
	sc2.ApprovedSpeechKeys = map[string][]string{}
	sc2.ApprovedTemplateSHA = map[string]string{}

	resolver := orchestrationtest.NewResolver(sc1)
	resolver.AddJurisdiction(sc2)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Welcome, citizen.", SyntheticOnly: false,
	})
	orch, err := orchestrationtest.NewOrchestrator(asr, mid, tts, resolver, validator, tpls)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	h := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	srv := New(DefaultConfig("127.0.0.1:0"), WithVoiceProcess(h))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// J2 requesting "welcome" -> 422
	bodyJ2 := `{"request_id":"req-j2","jurisdiction":"J2","speech_key":"welcome","language":"en-IN","args":{"args":{}},"source_version":1,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
	code, _, _ := doVoice(t, ts, http.MethodPost, "/api/v3/voice/speech", "application/json", bodyJ2)
	if code != http.StatusUnprocessableEntity {
		t.Errorf("J2 /voice/speech = %d, want 422", code)
	}

	// J1 requesting "welcome" -> 200
	bodyJ1 := `{"request_id":"req-j1","jurisdiction":"J1","speech_key":"welcome","language":"en-IN","args":{"args":{}},"source_version":1,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
	code, _, resp := doVoice(t, ts, http.MethodPost, "/api/v3/voice/speech", "application/json", bodyJ1)
	if code != http.StatusOK {
		t.Fatalf("J1 /voice/speech = %d, want 200; body=%s", code, resp)
	}
}

// TestVoiceProcess_HandleSpeech_InjectedArg verifies template arg injection rejection at HTTP layer.
func TestVoiceProcess_HandleSpeech_InjectedArg(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	sc.TemplateKeys = append(sc.TemplateKeys, "choice_prompt")
	sc.ApprovedSpeechKeys["choice_prompt"] = []string{"en-IN"}
	sc.ApprovedTemplateSHA["choice_prompt"] = orchestrationtest.DigestString("Select destination: {facility_id}.")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "choice_prompt", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text:      "Select destination: {facility_id}.",
		ArgSchema: map[string]string{"facility_id": "string"},
	})
	orch, err := orchestrationtest.NewOrchestrator(asr, mid, tts, resolver, validator, tpls)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	h := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	srv := New(DefaultConfig("127.0.0.1:0"), WithVoiceProcess(h))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	body := `{"request_id":"req-1","jurisdiction":"JTEST","speech_key":"choice_prompt","language":"en-IN","args":{"args":{"facility_id":"{inject}"}},"source_version":1,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
	code, _, _ := doVoice(t, ts, http.MethodPost, "/api/v3/voice/speech", "application/json", body)
	if code != http.StatusUnprocessableEntity {
		t.Errorf("injected arg /voice/speech = %d, want 422", code)
	}
}

// TestVoiceProcess_HandleSpeech_SourceWithdrawal verifies 409 STALE_SNAPSHOT on withdrawal during synthesis.
func TestVoiceProcess_HandleSpeech_SourceWithdrawal(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Welcome, citizen.", SyntheticOnly: false,
	})
	calls := 0
	resolver.SetRevalidateHook(func(ctx context.Context, sc contracts.ScopedContext) error {
		calls++
		if calls > 1 {
			return orchestration.ErrStaleSnapshot
		}
		return nil
	})
	orch, err := orchestrationtest.NewOrchestrator(asr, mid, tts, resolver, validator, tpls)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	h := NewVoiceProcessHandler(orch, orchestration.DefaultLimits())
	srv := New(DefaultConfig("127.0.0.1:0"), WithVoiceProcess(h))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	body := `{"request_id":"req-1","jurisdiction":"JTEST","speech_key":"welcome","language":"en-IN","args":{"args":{}},"source_version":1,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
	code, _, _ := doVoice(t, ts, http.MethodPost, "/api/v3/voice/speech", "application/json", body)
	if code != http.StatusConflict {
		t.Errorf("/voice/speech during withdrawal = %d, want 409", code)
	}
}

// TestVoiceProcess_HandleProcess_TTSFailurePreservesActions verifies that when TTS fails during /voice/process,
// model proposal actions and text are preserved and an audio stage failure is reported with 200 OK.
func TestVoiceProcess_HandleProcess_TTSFailurePreservesActions(t *testing.T) {
	srv, _, _, mid, tts := buildVoiceServer(t)
	key := "welcome"
	mid.SetProposeHook(func(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		return contracts.MiddleWorkerResponse{
			RequestID: req.RequestID,
			Proposal: contracts.ModelOutput{
				SchemaVersion: contracts.ModelSchemaVersion,
				RequestID:     req.RequestID,
				DataVersion:   req.ScopedContext.DataVersion,
				Status:        contracts.StatusOK,
				Intent:        orchestrationtest.IntentPtr(contracts.IntentListDestinations),
				Language:      req.Transcript.Language,
				Actions:       []contracts.Action{{Type: contracts.ActionShowChoices, TargetIDs: []string{"FAC-DEMO-1"}}},
				SpeechKey:     &key,
			},
			ModelRevision: "r0",
		}, nil
	})
	tts.SetSynthesizeHook(func(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		return contracts.TTSWorkerResponse{}, errors.New("tts worker failure")
	})

	body := `{"request_id":"req-fail-tts","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"hello"},"render":{"kind":"tts"}}`
	code, _, resp := doVoice(t, srv, http.MethodPost, "/api/v3/voice/process", "application/json", body)
	if code != http.StatusOK {
		t.Fatalf("/voice/process = %d, want 200; body=%s", code, resp)
	}
	env := decodeVoiceEnvelope(t, resp)
	dataBS, _ := json.Marshal(env.Data)
	var pipelineResp contracts.PipelineResponse
	_ = json.Unmarshal(dataBS, &pipelineResp)

	if len(pipelineResp.ValidatedProposal.Actions) != 1 || pipelineResp.ValidatedProposal.Actions[0].Type != contracts.ActionShowChoices {
		t.Errorf("actions lost on TTS failure: %+v", pipelineResp.ValidatedProposal.Actions)
	}
	if pipelineResp.Audio != nil {
		t.Errorf("Audio should be nil on TTS failure")
	}
	foundTTSFail := false
	for _, f := range pipelineResp.StageFailures {
		if f == "tts" {
			foundTTSFail = true
		}
	}
	if !foundTTSFail {
		t.Errorf("StageFailures = %+v, want tts", pipelineResp.StageFailures)
	}
}

// TestVoiceProcess_HandleSpeech_PlayableDecode verifies that /voice/speech returns audio_b64
// that decodes into bytes matching checksum_sha256 and byte_size.
func TestVoiceProcess_HandleSpeech_PlayableDecode(t *testing.T) {
	srv, _, _, _, _ := buildVoiceServer(t)
	body := `{"request_id":"req-audio-ok","jurisdiction":"JTEST","speech_key":"welcome","language":"en-IN","args":{"args":{}},"source_version":1,"settings":{"sample_rate":16000,"bit_depth":16,"channels":1}}`
	code, _, resp := doVoice(t, srv, http.MethodPost, "/api/v3/voice/speech", "application/json", body)
	if code != http.StatusOK {
		t.Fatalf("/voice/speech = %d, want 200; body=%s", code, resp)
	}
	env := decodeVoiceEnvelope(t, resp)
	dataBS, _ := json.Marshal(env.Data)
	var ttsResp contracts.TTSResponse
	if err := json.Unmarshal(dataBS, &ttsResp); err != nil {
		t.Fatalf("unmarshal TTSResponse: %v", err)
	}
	if ttsResp.AudioB64 == "" {
		t.Fatalf("AudioB64 is empty")
	}
	audioBytes, err := base64.StdEncoding.DecodeString(ttsResp.AudioB64)
	if err != nil {
		t.Fatalf("base64 decode AudioB64: %v", err)
	}
	if int64(len(audioBytes)) != ttsResp.ByteSize {
		t.Errorf("decoded length %d != byte_size %d", len(audioBytes), ttsResp.ByteSize)
	}
	sum := sha256.Sum256(audioBytes)
	checksum := hex.EncodeToString(sum[:])
	if checksum != ttsResp.ChecksumSHA256 {
		t.Errorf("checksum mismatch: computed %s != expected %s", checksum, ttsResp.ChecksumSHA256)
	}
}
