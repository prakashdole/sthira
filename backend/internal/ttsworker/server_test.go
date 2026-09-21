package ttsworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/ttsworker/templates"
)

// makeServer spins up a Server bound to 127.0.0.1:0 with a chosen
// bearer token (or "" for no auth).
func makeServer(t *testing.T, token string) (*Server, string) {
	t.Helper()
	catalog := buildCatalog(t)
	renderer := templates.NewRenderer(catalog)
	clock := NewStandaloneSourceVersionClock(7)
	codec := NewCodec(8*1024*1024, time.Hour, clock)
	inv := Inventory{
		Parler: ParlerTTS{
			ModelID:  "ai4bharat/indic-parler-tts",
			License:  "Apache-2.0",
			Runtime:  "transformers-4.x",
			Hardware: "cpu",
			Revision: "rev-1",
			Voices:   []string{"ml-IN-female-1"},
		},
		SupportedLanguages: []string{"hi-IN", "ml-IN", "en-IN"},
	}
	w, err := New(Config{
		Inventory:   inv,
		Runtime:     newTaggedRuntime("ml-IN", "ml-IN-female-1", "rev-1"),
		Catalog:     catalog,
		Renderer:    renderer,
		Cache:       codec,
		Clock:       clock,
		QueueDepth:  4,
		MaxInFlight: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(ServerConfig{
		Address:         "127.0.0.1:0",
		BearerToken:     token,
		Worker:          w,
		MaxRequestBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = s.Start(context.Background(), "") }()
	deadline := time.Now().Add(2 * time.Second)
	for s.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.Addr() == "" {
		t.Fatal("server failed to bind")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = s.Cancel(ctx)
	})
	return s, s.Addr()
}

func httpDo(t *testing.T, method, url, token string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// TestServerHealthEndpoint: GET /health returns 200 with the
// snapshot.
func TestServerHealthEndpoint(t *testing.T) {
	_, addr := makeServer(t, "")
	code, body := httpDo(t, http.MethodGet, "http://"+addr+"/health", "", nil)
	if code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", code, body)
	}
}

// TestServerHealthRejectsBadMethod: only GET is allowed.
func TestServerHealthRejectsBadMethod(t *testing.T) {
	_, addr := makeServer(t, "")
	code, _ := httpDo(t, http.MethodPost, "http://"+addr+"/health", "", nil)
	if code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want 405", code)
	}
}

// TestServerRequiresBearer: when a token is configured, missing
// bearer is rejected.
func TestServerRequiresBearer(t *testing.T) {
	_, addr := makeServer(t, "secret")
	code, _ := httpDo(t, http.MethodGet, "http://"+addr+"/health", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", code)
	}
	code, _ = httpDo(t, http.MethodGet, "http://"+addr+"/health", "secret", nil)
	if code != http.StatusOK {
		t.Fatalf("status: got %d want 200", code)
	}
}

// TestServerSynthesizeBadJSON: malformed bodies are rejected.
func TestServerSynthesizeBadJSON(t *testing.T) {
	_, addr := makeServer(t, "")
	code, _ := httpDo(t, http.MethodPost, "http://"+addr+"/synthesize", "", []byte("not json"))
	if code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", code)
	}
}

// TestServerSynthesizeEmptyLanguageRejected.
func TestServerSynthesizeEmptyLanguageRejected(t *testing.T) {
	_, addr := makeServer(t, "")
	body := SynthesizeRequest{
		RequestID:       "REQ-1",
		SpeechKey:       "destination_options",
		Language:        "",
		Text:            "hello",
		SourceVersion:   7,
		TemplateVersion: 1,
		Settings:        SynthesisSettings{SampleRate: 22050, BitDepth: 16, Channels: 1},
		DeadlineMillis:  1000,
	}
	bb, _ := json.Marshal(body)
	code, _ := httpDo(t, http.MethodPost, "http://"+addr+"/synthesize", "", bb)
	if code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", code)
	}
}

// TestServerSynthesizeTooLarge: oversized bodies are rejected with 413.
func TestServerSynthesizeTooLarge(t *testing.T) {
	_, addr := makeServer(t, "")
	body := []byte(`{"x":"` + strings.Repeat("a", 300*1024) + `"}`)
	code, _ := httpDo(t, http.MethodPost, "http://"+addr+"/synthesize", "", body)
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status: got %d want 413", code)
	}
}

// TestServerSynthesizeInvalidJSONRejected.
func TestServerSynthesizeInvalidJSONRejected(t *testing.T) {
	_, addr := makeServer(t, "")
	body := []byte("this is not json")
	code, _ := httpDo(t, http.MethodPost, "http://"+addr+"/synthesize", "", body)
	if code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", code)
	}
}

// TestServerShutdownDrainsExisting: /shutdown returns 200 after
// draining the pool.
func TestServerShutdownDrainsExisting(t *testing.T) {
	_, addr := makeServer(t, "")
	code, _ := httpDo(t, http.MethodPost, "http://"+addr+"/shutdown", "", nil)
	if code != http.StatusOK {
		t.Fatalf("status: got %d want 200", code)
	}
}

// TestServerShutdownRejectsBadMethod.
func TestServerShutdownRejectsBadMethod(t *testing.T) {
	_, addr := makeServer(t, "")
	code, _ := httpDo(t, http.MethodGet, "http://"+addr+"/shutdown", "", nil)
	if code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want 405", code)
	}
}

// TestServerSynthesizeOKOnCacheHit: pre-populate the cache, send a
// request, and verify a 200 with audio_b64.
func TestServerSynthesizeOKOnCacheHit(t *testing.T) {
	catalog := buildCatalog(t)
	renderer := templates.NewRenderer(catalog)
	clock := NewStandaloneSourceVersionClock(7)
	codec := NewCodec(8*1024*1024, time.Hour, clock)
	inv := Inventory{
		Parler: ParlerTTS{
			ModelID:  "ai4bharat/indic-parler-tts",
			License:  "Apache-2.0",
			Runtime:  "transformers-4.x",
			Hardware: "cpu",
			Revision: "rev-1",
			Voices:   []string{"ml-IN-female-1"},
		},
		SupportedLanguages: []string{"hi-IN", "ml-IN", "en-IN"},
	}
	w, err := New(Config{
		Inventory: inv, Runtime: newTaggedRuntime("ml-IN", "ml-IN-female-1", "rev-1"),
		Catalog: catalog, Renderer: renderer, Cache: codec, Clock: clock,
		QueueDepth: 4, MaxInFlight: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(ServerConfig{
		Address:         "127.0.0.1:0",
		Worker:          w,
		MaxRequestBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = s.Start(context.Background(), "") }()
	deadline := time.Now().Add(2 * time.Second)
	for s.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	addr := s.Addr()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Cancel(ctx)
	})
	// Pre-populate cache.
	args, err := renderer.Cat().ValidateArgs(templates.Key("destination_options"), map[string]any{
		"facility_id": "FACILITY-A",
		"rank":        1,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderer.Render(templates.Key("destination_options"), "ml-IN", args)
	if err != nil {
		t.Fatal(err)
	}
	id := CacheIdentity{
		Text:              out.Text,
		TemplateKey:       "destination_options",
		TemplateVersion:   1,
		SourceVersion:     clock.Current(),
		Language:          "ml-IN",
		ModelRevision:     "rev-1",
		VoiceRevision:     "ml-IN-female-1",
		SynthesisSettings: SynthesisSettings{SampleRate: 22050, BitDepth: 16, Channels: 1, SpeakingRate: 1.0},
	}
	silence, err := EncodeSilenceWav(22050, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if err := codec.Put(id, silence.Bytes); err != nil {
		t.Fatal(err)
	}
	body := SynthesizeRequest{
		RequestID:       "REQ-cache-hit",
		SpeechKey:       "destination_options",
		Language:        "ml-IN",
		Text:            out.Text,
		SourceVersion:   clock.Current(),
		TemplateVersion: 1,
		Settings:        id.SynthesisSettings,
		DeadlineMillis:  5000,
	}
	bb, _ := json.Marshal(body)
	code, resp := httpDo(t, http.MethodPost, "http://"+addr+"/synthesize", "", bb)
	if code != http.StatusOK {
		t.Fatalf("status: got %d want 200; body=%s", code, resp)
	}
	var got SynthesizeResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("response decode: %v body=%s", err, resp)
	}
	if got.State != StateOK {
		t.Fatalf("state: got %s want OK", got.State)
	}
	if !got.CacheHit {
		t.Fatalf("expected cache hit")
	}
	_, err = base64.StdEncoding.DecodeString(got.AudioB64)
	if err != nil {
		t.Fatalf("audio_b64 decode: %v", err)
	}
}

// silence unused imports.
var _ atomic.Int64
var _ = errors.Is
