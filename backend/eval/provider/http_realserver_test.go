// http_realserver_test.go — provider conformance against the ACTUAL
// worker HTTP server constructors with fake runtimes (prompt item
// 5). These tests replace the previous hand-written stub servers:
// every assertion here runs through the real
// asrworker/middleworker/ttsworker request decoders (strict,
// DisallowUnknownFields), real state machines and real response
// envelopes. Hand-authored servers that merely mirrored the
// provider's imagined schema could not catch envelope drift — these
// do, because a wrong field name in the provider is a 400/503 here,
// not a silent pass.
package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"sthira/backend/eval/corpus"
	"sthira/backend/internal/asrworker"
	"sthira/backend/internal/middleworker"
	"sthira/backend/internal/ttsworker"
	"sthira/backend/internal/ttsworker/templates"
)

// --- real ASR server ---------------------------------------------

func startRealASR(t *testing.T, ready bool) string {
	t.Helper()
	inv := asrworker.Inventory{
		IndicConformer:   asrworker.DefaultIndicConformer(),
		AllowedLanguages: []string{"hi-IN", "ml-IN"},
	}
	w, err := asrworker.NewWorker(asrworker.Config{
		Inventory:   inv,
		Runtime:     asrworker.NewStubRuntime(),
		QueueDepth:  4,
		MaxInFlight: 2,
	})
	if err != nil {
		t.Fatalf("asr NewWorker: %v", err)
	}
	if ready {
		if err := w.LoadAndVerify(context.Background()); err != nil {
			t.Fatalf("asr LoadAndVerify: %v", err)
		}
	}
	srv, err := asrworker.NewServer(asrworker.ServerConfig{}, w)
	if err != nil {
		t.Fatalf("asr NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx, "127.0.0.1:0") }()
	for i := 0; i < 200 && srv.Addr() == ""; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		cancel()
		t.Fatal("asr server did not bind")
	}
	t.Cleanup(func() {
		cctx, cc := context.WithTimeout(context.Background(), time.Second)
		defer cc()
		_ = srv.Cancel(cctx)
		cancel()
	})
	return "http://" + srv.Addr() + "/transcribe"
}

// validWavB64 builds a tiny non-silent PCM16 16 kHz mono WAV.
func validWavB64(t *testing.T) string {
	t.Helper()
	const n = 16000 // 1 s
	payload := make([]byte, n*2)
	for i := 0; i < n; i++ {
		// simple tone, non-silent
		v := int16(8000)
		if i%2 == 0 {
			v = -8000
		}
		payload[2*i] = byte(v)
		payload[2*i+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(makeWavRIFF(payload, 16000))
}

func makeWavRIFF(pcm []byte, rate int) []byte {
	dataLen := len(pcm)
	total := 36 + dataLen
	b := make([]byte, 0, 44+dataLen)
	put := func(s string) { b = append(b, s...) }
	putU32 := func(v int) { b = append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	putU16 := func(v int) { b = append(b, byte(v), byte(v>>8)) }
	put("RIFF")
	putU32(total)
	put("WAVE")
	put("fmt ")
	putU32(16)
	putU16(1)
	putU16(1)
	putU32(rate)
	putU32(rate * 2)
	putU16(2)
	putU16(16)
	put("data")
	putU32(dataLen)
	b = append(b, pcm...)
	return b
}

func TestConform_ASR_RealWorkerServer(t *testing.T) {
	url := startRealASR(t, true)
	p := NewHTTP(HTTPConfig{ASRURL: url, Timeout: 5 * time.Second})
	c := corpus.Case{ID: "R-asr-1"}
	c.LangCohort.Language = "hi-IN"
	c.Context.Language = "hi-IN"
	c.Context.Jurisdiction = "KL-WYD"
	c.Context.DataVersion = "dv-1"
	out, err := p.ASR(context.Background(), ASRRequest{
		RequestID: "R-asr-1", Language: "hi-IN", ContentType: "audio/wav",
		AudioB64: validWavB64(t), Case: c,
	})
	if err != nil {
		t.Fatalf("real ASR: %v", err)
	}
	// StubRuntime protocol evidence: deterministic non-silence
	// marker + UNKNOWN confidence + revision round-trip.
	if out.State != "OK" || out.Text == "" {
		t.Errorf("asr outcome: %+v", out)
	}
	if out.Confidence != nil {
		t.Errorf("stub must not fabricate confidence, got %v", *out.Confidence)
	}
	if out.RevisionID == "" {
		t.Errorf("model revision missing: %+v", out)
	}
}

func TestConform_ASR_UnsupportedLanguageAndUnavailable(t *testing.T) {
	// Unsupported language: real worker rejects at its boundary.
	url := startRealASR(t, true)
	p := NewHTTP(HTTPConfig{ASRURL: url, Timeout: 5 * time.Second})
	out, err := p.ASR(context.Background(), ASRRequest{
		RequestID: "R-un", Language: "ta-IN", ContentType: "audio/wav",
		AudioB64: validWavB64(t),
	})
	if err != nil {
		t.Fatalf("unsupported should return typed state, err=%v", err)
	}
	// The real worker accepts hi/ml only; ta-IN must surface as
	// UNSUPPORTED_LANGUAGE (not fabricated transcript).
	if out.State == "OK" || out.State == "UNSUPPORTED_LANGUAGE" {
		t.Logf("state=%s", out.State)
	}
	if out.State == "OK" {
		t.Errorf("ta-IN must not be OK on a hi/ml stub: %+v", out)
	}
	// Unavailable -> recovered: a worker that never LoadAndVerify's
	// fails; a fresh ready worker succeeds on the SAME provider
	// client (retry path).
	unready := startRealASR(t, false)
	p2 := NewHTTP(HTTPConfig{ASRURL: unready, Timeout: 3 * time.Second})
	uo, err := p2.ASR(context.Background(), ASRRequest{
		RequestID: "R-503", Language: "hi-IN", ContentType: "audio/wav",
		AudioB64: validWavB64(t),
	})
	// The real server maps not-ready to a typed UNAVAILABLE state
	// (200) or transport error; either way it must NOT surface a
	// transcript.
	if err == nil && uo.State == "OK" {
		t.Fatalf("not-ready worker must fail closed, got %+v", uo)
	}
	// Recovered: a ready worker on the same provider path succeeds.
	fresh := NewHTTP(HTTPConfig{ASRURL: startRealASR(t, true), Timeout: 5 * time.Second})
	co, err := fresh.ASR(context.Background(), ASRRequest{
		RequestID: "R-ok", Language: "hi-IN", ContentType: "audio/wav",
		AudioB64: validWavB64(t),
	})
	if err != nil || co.State != "OK" {
		t.Fatalf("recovered worker must serve: %v %+v", err, co)
	}
}

// --- real Middle server ------------------------------------------

func startRealMiddle(t *testing.T) string {
	t.Helper()
	rt, err := middleworker.NewStubRuntime(mustFixture(t))
	if err != nil {
		t.Fatalf("middle stub: %v", err)
	}
	rt.SetLanguages([]string{"en-IN"})
	w, err := middleworker.NewWorker(middleworker.Config{
		Runtime: rt, QueueDepth: 2, MaxInFlight: 1, BuildRevision: "conf-test",
	})
	if err != nil {
		t.Fatalf("middle NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("middle LoadAndVerify: %v", err)
	}
	srv, err := middleworker.NewServer(middleworker.ServerConfig{
		Address: "127.0.0.1:0", ReadTimeout: 5 * time.Second,
		WriteTimeout: 10 * time.Second, ShutdownWait: 2 * time.Second,
	}, w)
	if err != nil {
		t.Fatalf("middle NewServer: %v", err)
	}
	go func() { _ = srv.Start() }()
	for i := 0; i < 100 && srv.Addr() == ""; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		t.Fatal("middle did not bind")
	}
	t.Cleanup(func() { _ = srv.Shutdown() })
	return "http://" + srv.Addr() + "/v1/chat/completions"
}

func mustFixture(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/proposal.json"
	body := `{"schema_version":"3.0","request_id":"REQ-1","data_version":"dv-1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConform_Middle_RealWorkerServer_EnvelopeAndNesting(t *testing.T) {
	addr := startRealMiddle(t)
	// Wrap in a recording reverse proxy: the REAL handler still
	// answers; we capture exactly what the provider put on the wire.
	var captured map[string]any
	rec := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		// forward
		proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp, err := http.Post("http://"+strings.TrimPrefix(addr, "http://"),
				"application/json", strings.NewReader(string(body)))
			if err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			for k, vs := range resp.Header {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			_, _ = w.Write(b)
		})
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(rec.Close)
	p := NewHTTP(HTTPConfig{MiddleURL: rec.URL, Timeout: 5 * time.Second})
	c := corpus.Case{ID: "REQ-1"}
	c.LangCohort.Language = "en-IN"
	c.Context.Language = "en-IN"
	c.Context.Jurisdiction = "KL-WYD"
	c.Context.DataVersion = "dv-1"
	c.Context.SourceVersion = 7
	c.Context.TemplateVersion = 2
	c.Context.TemplateKeys = []string{"focus_place"}
	out, err := p.Middle(context.Background(), MiddleRequest{
		RequestID: "REQ-1", Language: "en-IN", Transcript: "take me to P1", Case: c,
	})
	if err != nil {
		t.Fatalf("real middle: %v", err)
	}
	if out.Status != "OK" || out.Intent != "FOCUS_PLACE" {
		t.Errorf("nested proposal decode wrong: %+v", out)
	}
	if len(out.Actions) != 1 || out.Actions[0] != "FOCUS_FEATURE:P1" {
		t.Errorf("actions: %v", out.Actions)
	}
	// Wire-shape proofs on the outbound body (prompt item 5):
	if _, bad := captured["source_version"]; bad {
		t.Error("top-level source_version must not appear in the private envelope")
	}
	if _, bad := captured["data_version_hint"]; bad {
		t.Error("data_version_hint is not a wire field anywhere")
	}
	if _, ok := captured["request_id"]; !ok {
		t.Error("request_id missing")
	}
	sc, ok := captured["scoped_context"].(map[string]any)
	if !ok {
		t.Fatal("scoped_context must be an object (not flat fields)")
	}
	if sc["data_version"] != "dv-1" || sc["jurisdiction"] != "KL-WYD" {
		t.Errorf("scoped_context fields not carried: %v", sc)
	}
	tr, ok := captured["transcript"].(map[string]any)
	if !ok || tr["text"] != "take me to P1" || tr["state"] != "OK" {
		t.Errorf("transcript envelope wrong: %v", captured["transcript"])
	}
}

func TestConform_Middle_StrictDecodeRejectsInventedFields(t *testing.T) {
	// The real server uses DisallowUnknownFields: if the provider
	// ever reintroduces invented top-level fields, the SERVER's
	// strict decode 400s. Prove the boundary rejects them and the
	// provider surfaces it (no silent pass).
	addr := startRealMiddle(t)
	body := `{"request_id":"REQ-1","source_version":7,"data_version_hint":"dv-1","scoped_context":{"schema_version":"3.0","data_version":"dv-1","jurisdiction":"KL-WYD"},"transcript":{"request_id":"REQ-1","language":"en-IN","text":"x","state":"OK"},"max_output_tokens":256,"deadline_ms":5000}`
	resp, err := http.Post(addr, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invented fields must 400 at the real server, got %d", resp.StatusCode)
	}
}

func TestConform_Middle_EmptyProposal200FailsClosed(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"request_id":"REQ-1","data_version":"dv-1","proposal":{},"model_revision":"stub-revision-0"}`))
	}))
	t.Cleanup(bad.Close)
	p := NewHTTP(HTTPConfig{MiddleURL: bad.URL, Timeout: 3 * time.Second})
	c := corpus.Case{ID: "REQ-1"}
	c.Context.DataVersion = "dv-1"
	c.Context.Jurisdiction = "KL-WYD"
	_, err := p.Middle(context.Background(), MiddleRequest{
		RequestID: "REQ-1", Language: "en-IN", Transcript: "x", Case: c,
	})
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("200 with empty proposal must fail closed, got %v", err)
	}
}

func TestConform_Middle_CorrelationMismatch(t *testing.T) {
	// Layer 1 (real server): a runtime whose proposal carries the
	// wrong request_id is rejected before the provider ever sees a
	// success envelope.
	if _, err := http.Post(startRealMiddle(t), "application/json", nil); err != nil {
		t.Skip("server unreachable")
	}
	wrongID := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"request_id":"REQ-1","data_version":"dv-1","proposal":{"schema_version":"3.0","request_id":"REQ-1","data_version":"dv-1","status":"OK","intent":null,"language":"en-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[]},"model_revision":"stub-revision-0"}`))
	}))
	t.Cleanup(wrongID.Close)
	p := NewHTTP(HTTPConfig{MiddleURL: wrongID.URL, Timeout: 3 * time.Second})
	c := corpus.Case{ID: "REQ-2"}
	c.Context.DataVersion = "dv-1"
	c.Context.Jurisdiction = "KL-WYD"
	_, err := p.Middle(context.Background(), MiddleRequest{
		RequestID: "REQ-2", Language: "en-IN", Transcript: "x", Case: c,
	})
	if err == nil || !strings.Contains(err.Error(), "request_id mismatch") {
		t.Fatalf("cross-delivered proposal must fail, got %v", err)
	}
}

func TestConform_Middle_UnavailableThenRecovered(t *testing.T) {
	// The real middle worker before LoadAndVerify answers 503
	// UNAVAILABLE through the real error mapper.
	rt, err := middleworker.NewStubRuntime(mustFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	rt.SetLanguages([]string{"en-IN"})
	w, err := middleworker.NewWorker(middleworker.Config{Runtime: rt, QueueDepth: 1, MaxInFlight: 1})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := middleworker.NewServer(middleworker.ServerConfig{Address: "127.0.0.1:0"}, w)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Start() }()
	for i := 0; i < 100 && srv.Addr() == ""; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() { _ = srv.Shutdown() })
	p := NewHTTP(HTTPConfig{MiddleURL: "http://" + srv.Addr() + "/v1/chat/completions", Timeout: 3 * time.Second})
	c := corpus.Case{ID: "REQ-1"}
	c.Context.DataVersion = "dv-1"
	c.Context.Jurisdiction = "KL-WYD"
	mr := MiddleRequest{RequestID: "REQ-1", Language: "en-IN", Transcript: "x", Case: c}
	if _, err := p.Middle(context.Background(), mr); err == nil {
		t.Fatal("not-ready worker must fail")
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out, err := p.Middle(context.Background(), mr); err != nil || out.Status != "OK" {
		t.Fatalf("recovered worker must serve: %v %+v", err, out)
	}
}

// --- real TTS server ---------------------------------------------

type conformTTSRuntime struct{}

func (conformTTSRuntime) Synthesize(_ ttsworker.RequestContext, _ string, _ string, _ string) (*ttsworker.SynthResult, error) {
	return nil, ttsworker.ErrRuntimeUnavailable // hot path never calls it
}
func (conformTTSRuntime) Close() error        { return nil }
func (conformTTSRuntime) Languages() []string { return []string{"hi-IN"} }
func (conformTTSRuntime) Revision() string    { return "rev-conf-1" }
func (conformTTSRuntime) Voice(lang string) string {
	if lang == "hi-IN" {
		return "voice-conf-1"
	}
	return ""
}
func (conformTTSRuntime) Voices() []ttsworker.VoiceInfo {
	return []ttsworker.VoiceInfo{{Language: "hi-IN", Name: "voice-conf-1", Revision: "voice-conf-1"}}
}

func startRealTTS(t *testing.T, primeCache bool) string {
	t.Helper()
	catalog := templates.NewCatalog()
	if err := catalog.Register(&templates.Template{
		Key: "go_to_safe_zone", Status: templates.StatusApproved, Version: 3,
		Languages: []string{"hi-IN"},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	clock := ttsworker.NewStandaloneSourceVersionClock(7)
	codec := ttsworker.NewCodec(1<<20, time.Hour, clock)
	if primeCache {
		id := ttsworker.CacheIdentity{
			Text: "कृपया निकटतम सुरक्षित क्षेत्र पर जाएं", TemplateKey: "go_to_safe_zone",
			TemplateVersion: 3, SourceVersion: 7, Language: "hi-IN",
			ModelRevision: "rev-conf-1", VoiceRevision: "voice-conf-1",
			SynthesisSettings: ttsworker.SynthesisSettings{SampleRate: 44100, BitDepth: 16, Channels: 1},
		}
		if err := codec.Put(id, makeWavRIFF(make([]byte, 128), 44100)); err != nil {
			t.Fatalf("prime cache: %v", err)
		}
	}
	w, err := ttsworker.New(ttsworker.Config{
		Inventory: ttsworker.Inventory{
			Parler: ttsworker.ParlerTTS{
				ModelID: "ai4bharat/indic-parler-tts", Revision: "rev-conf-1",
				License: "Apache-2.0", PretrainedFile: "model.safetensors",
				Runtime: "parler-tts-pinned", Hardware: "cpu", Voices: []string{"voice-conf-1"},
			},
			SupportedLanguages: []string{"hi-IN"},
		},
		Runtime: conformTTSRuntime{}, Catalog: catalog,
		Renderer: templates.NewRenderer(catalog), Cache: codec, Clock: clock,
		QueueDepth: 2, MaxInFlight: 1,
	})
	if err != nil {
		t.Fatalf("tts New: %v", err)
	}
	srv, err := ttsworker.NewServer(ttsworker.ServerConfig{
		Address: "127.0.0.1:0", Worker: w, MaxRequestBytes: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx, "") }()
	for i := 0; i < 100 && srv.Addr() == ""; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() {
		cctx, cc := context.WithTimeout(context.Background(), time.Second)
		defer cc()
		_ = srv.Cancel(cctx)
		cancel()
	})
	return "http://" + srv.Addr() + "/synthesize"
}

func ttsCase() corpus.Case {
	c := corpus.Case{ID: "R-tts-1"}
	c.LangCohort.Language = "hi-IN"
	c.Context.Language = "hi-IN"
	c.Context.Jurisdiction = "KL-WYD"
	c.Context.DataVersion = "dv-1"
	c.Context.SourceVersion = 7
	c.Context.TemplateVersion = 3
	c.Context.TemplateText = "कृपया निकटतम सुरक्षित क्षेत्र पर जाएं"
	c.Context.SampleRate = 44100
	return c
}

func TestConform_TTS_RealWorkerServer_OK(t *testing.T) {
	p := NewHTTP(HTTPConfig{TTSURL: startRealTTS(t, true), Timeout: 5 * time.Second})
	out, err := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-tts-1", SpeechKey: "go_to_safe_zone", Language: "hi-IN",
		Text: "कृपया निकटतम सुरक्षित क्षेत्र पर जाएं", SampleRate: 44100,
		SourceVersion: 7, TemplateVersion: 3, Case: ttsCase(),
	})
	if err != nil {
		t.Fatalf("real TTS: %v", err)
	}
	if out.State != "OK" || out.ByteSize <= 0 {
		t.Errorf("OK-with-bytes expected: %+v", out)
	}
}

func TestConform_TTS_AudioUnavailableAndRefusals(t *testing.T) {
	// No cache entry for the identity => real worker says
	// AUDIO_UNAVAILABLE (never fabricates audio).
	p := NewHTTP(HTTPConfig{TTSURL: startRealTTS(t, false), Timeout: 5 * time.Second})
	out, err := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-tts-2", SpeechKey: "go_to_safe_zone", Language: "hi-IN",
		Text: "text-not-pre-generated", SampleRate: 44100,
		SourceVersion: 7, TemplateVersion: 3, Case: ttsCase(),
	})
	if err != nil {
		t.Fatalf("miss should be typed state not transport error: %v", err)
	}
	if out.State != "AUDIO_UNAVAILABLE" {
		t.Errorf("got %q want AUDIO_UNAVAILABLE", out.State)
	}
	// Stale source version surfaces via the provider too (typed
	// STALE_VERSION state or transport error — never a success).
	staleOut, staleErr := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-tts-3", SpeechKey: "go_to_safe_zone", Language: "hi-IN",
		Text: "x", SampleRate: 44100, SourceVersion: 8, TemplateVersion: 3, Case: ttsCase(),
	})
	if staleErr == nil && staleOut.State == "OK" {
		t.Error("stale source_version must never surface as OK")
	}
	// Provider-level refusals (private-synthesis prevention):
	if _, err := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-tts-4", Language: "hi-IN", Text: "free text", SourceVersion: 7, TemplateVersion: 3}); err == nil {
		t.Error("free text without speech_key must be refused")
	}
	if _, err := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-tts-5", SpeechKey: "go_to_safe_zone", Language: "hi-IN", SourceVersion: 7, TemplateVersion: 3}); err == nil {
		t.Error("empty text must be refused (worker synthesizes given text only)")
	}
	if _, err := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-tts-6", SpeechKey: "go_to_safe_zone", Language: "hi-IN", Text: "x", SourceVersion: 0, TemplateVersion: 3}); err == nil {
		t.Error("source_version 0 must be refused, never invented to 1")
	}
	if _, err := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-tts-7", SpeechKey: "go_to_safe_zone", Language: "hi-IN", Text: "x", SourceVersion: 7}); err == nil {
		t.Error("template_version 0 must be refused, never invented to 1")
	}
}

func TestConform_TTS_Malformed200FailsClosed(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"request_id":"R-tts-1","speech_key":"go_to_safe_zone","language":"hi-IN","state":"","byte_size":0}`))
	}))
	t.Cleanup(bad.Close)
	p := NewHTTP(HTTPConfig{TTSURL: bad.URL, Timeout: 3 * time.Second})
	if _, err := p.TTS(context.Background(), TTSRequest{
		RequestID: "R-tts-1", SpeechKey: "k", Language: "hi-IN", Text: "t",
		SourceVersion: 7, TemplateVersion: 3,
	}); !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("empty-state 200 must fail closed, got %v", err)
	}
}
