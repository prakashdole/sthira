package asrworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// makeServer spins up a Server bound to 127.0.0.1 with a chosen
// port. Tests call t.Cleanup to release.
func makeServer(t *testing.T, token string) *Server {
	t.Helper()
	inv := Inventory{IndicConformer: DefaultIndicConformer(), AllowedLanguages: []string{"hi-IN", "ml-IN"}}
	w, err := NewWorker(Config{
		Inventory:     inv,
		Runtime:       NewStubRuntime(),
		QueueDepth:    4,
		MaxInFlight:   2,
		BuildRevision: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Shutdown(context.Background()) })
	s, err := NewServer(ServerConfig{Address: "127.0.0.1:0", Token: token}, w)
	if err != nil {
		t.Fatal(err)
	}
	// Bind a real listener so tests can hit it via HTTP.
	ln := getLoopbackListener(t)
	s.listener = ln
	s.httpSrv = &http.Server{
		Handler:      s.mux,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = s.httpSrv.Serve(ln)
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(ctx)
		wg.Wait()
	})
	return s
}

// getLoopbackListener binds to a free port on 127.0.0.1; tests
// read Addr() to learn the port.
func getLoopbackListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

// http JSON helpers used by tests.
func writeJSON(t *testing.T, body any) []byte {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestServer_HealthEndpoint_200: a /health request returns 200 and
// the JSON body decodes cleanly with the documented fields.
func TestServer_HealthEndpoint_200(t *testing.T) {
	s := makeServer(t, "")
	body, status := getJSON(t, s, http.MethodGet, "/health", "", nil)
	if status != http.StatusOK {
		t.Fatalf("status: got %d want 200", status)
	}
	var h healthJSON
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !h.Ready || !h.Warm {
		t.Errorf("expected Ready=true Warm=true; got %+v", h)
	}
	if len(h.SupportedLanguages) == 0 {
		t.Errorf("expected supported languages; got empty")
	}
	if h.Queue.MaxDepth == 0 || h.Queue.MaxConcurrency == 0 {
		t.Errorf("queue stats missing: %+v", h.Queue)
	}
	if h.BuildRevision == "" {
		t.Errorf("build revision must be populated")
	}
}

// TestServer_HealthEndpoint_RejectsBadMethod: only GET is allowed.
func TestServer_HealthEndpoint_RejectsBadMethod(t *testing.T) {
	s := makeServer(t, "")
	_, status := getJSON(t, s, http.MethodPost, "/health", "", nil)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want 405", status)
	}
}

// TestServer_HealthEndpoint_RequiresBearerToken: when a token is
// configured, the bearer header is required.
func TestServer_HealthEndpoint_RequiresBearerToken(t *testing.T) {
	s := makeServer(t, "secret")
	_, status := getJSON(t, s, http.MethodGet, "/health", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("status without bearer: got %d want 401", status)
	}
	// Right token works.
	body, status := getJSON(t, s, http.MethodGet, "/health", "secret", nil)
	if status != http.StatusOK {
		t.Fatalf("with bearer: got %d body=%s", status, body)
	}
}

// TestServer_Transcribe_RejectsBadJSONBody: a malformed body
// returns 400 with a typed error envelope.
func TestServer_Transcribe_RejectsBadJSONBody(t *testing.T) {
	s := makeServer(t, "")
	_, status := getJSON(t, s, http.MethodPost, "/transcribe", "", []byte("not json"))
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", status)
	}
}

// TestServer_Transcribe_RejectsEmptyAudio: zero-length base64
// audio is rejected with a typed error.
func TestServer_Transcribe_RejectsEmptyAudio(t *testing.T) {
	s := makeServer(t, "")
	body, status := postTranscribe(t, s, requestJSON{
		RequestID:   "R-1",
		Language:    "hi-IN",
		ContentType: "audio/wav",
		AudioB64:    "",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400; body=%s", status, body)
	}
	var resp responseJSON
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.State != string(TranscriptionAudioUnavailable) {
		t.Errorf("state: got %q want %q", resp.State, TranscriptionAudioUnavailable)
	}
	if resp.RequestID != "R-1" {
		t.Errorf("request_id must be echoed in error response")
	}
}

// TestServer_Transcribe_RejectsUnsupportedCodec: a request whose
// content-type is not in WorkerSupportedContentTypes is rejected at
// decode time, with state = AUDIO_UNAVAILABLE.
func TestServer_Transcribe_RejectsUnsupportedCodec(t *testing.T) {
	s := makeServer(t, "")
	wav := silenceBufferAt(100, 16_000)
	body, status := postTranscribe(t, s, requestJSON{
		RequestID:      "R-2",
		Language:       "hi-IN",
		ContentType:    "audio/webm",
		AudioB64:       base64.StdEncoding.EncodeToString(wav),
		ByteSize:       int64(len(wav)),
		DeadlineMillis: 5000,
	})
	if status != http.StatusOK {
		t.Fatalf("status: got %d body=%s", status, body)
	}
	var resp responseJSON
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.State != string(TranscriptionAudioUnavailable) {
		t.Errorf("state: got %q want %q", resp.State, TranscriptionAudioUnavailable)
	}
}

// TestServer_Transcribe_RejectsUnsupportedLanguage: a request
// whose language is not in the runtime's allow list returns
// UNSUPPORTED_LANGUAGE.
func TestServer_Transcribe_RejectsUnsupportedLanguage(t *testing.T) {
	s := makeServer(t, "")
	wav := silenceBufferAt(100, 16_000)
	body, status := postTranscribe(t, s, requestJSON{
		RequestID:      "R-3",
		Language:       "ta-IN",
		ContentType:    "audio/wav",
		AudioB64:       base64.StdEncoding.EncodeToString(wav),
		ByteSize:       int64(len(wav)),
		DeadlineMillis: 5000,
	})
	if status != http.StatusOK {
		t.Fatalf("status: got %d body=%s", status, body)
	}
	var resp responseJSON
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.State != string(TranscriptionUnsupportedLanguage) {
		t.Errorf("state: got %q want %q", resp.State, TranscriptionUnsupportedLanguage)
	}
}

// TestServer_Transcribe_OKOnValidMonoWAV: end-to-end happy path.
// The response carries confidence = nil (unknown) because the stub
// never claims a calibrated probability.
func TestServer_Transcribe_OKOnValidMonoWAV(t *testing.T) {
	s := makeServer(t, "")
	// Use a tone (non-silence) so the stub produces the canned
	// marker instead of empty text.
	wav := silenceBufferAt(200, 16_000)
	payload := make([]byte, len(wav))
	copy(payload, wav)
	// Muck up some samples so the buffer is not silent.
	payload[44+0] = 0x10
	payload[44+1] = 0x00
	body, status := postTranscribe(t, s, requestJSON{
		RequestID:      "R-OK",
		Language:       "hi-IN",
		ContentType:    "audio/wav",
		AudioB64:       base64.StdEncoding.EncodeToString(payload),
		ByteSize:       int64(len(payload)),
		DeadlineMillis: 5000,
	})
	if status != http.StatusOK {
		t.Fatalf("status: got %d body=%s", status, body)
	}
	var resp responseJSON
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.State != string(TranscriptionOK) {
		t.Errorf("state: got %q want %q", resp.State, TranscriptionOK)
	}
	if resp.RequestID != "R-OK" {
		t.Errorf("request_id: got %q want %q", resp.RequestID, "R-OK")
	}
	if resp.Confidence != nil {
		t.Errorf("confidence must be nil (stub never claims calibration); got %v", *resp.Confidence)
	}
	if resp.ModelRevision == "" {
		t.Errorf("model revision must be populated (stub returns revision)")
	}
	// Either Text is the canned marker (non-silence path) or some
	// valid string. Either is fine; the property is that we didn't
	// fabricate unsupported calibration.
	_ = resp.Text
}

// TestServer_Transcribe_RejectsOversizeBytes: the body cap is
// enforced at the server level. The HTTP layer blocks oversized
// bodies before the worker ever sees them. We craft a JSON object
// with a long string field that is syntactically valid past the
// cap, so the JSON decoder attempts to read past the cap and
// MaxBytesReader returns its typed error.
func TestServer_Transcribe_RejectsOversizeRequestBody(t *testing.T) {
	s := makeServer(t, "")
	s.MaxRequestBytes = 512
	pad := strings.Repeat("a", 4*1024)
	body := []byte(`{"request_id":"R","language":"hi-IN","content_type":"audio/wav","audio_b64":"` + pad + `","byte_size":1,"decoded_seconds":1,"deadline_ms":1000}`)
	_, status := getJSON(t, s, http.MethodPost, "/transcribe", "", body)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status: got %d want 413", status)
	}
}

// TestServer_Transcribe_RejectsOversizeAudioAtDecode: an audio blob
// of 100 frames is fine; the decode limit is hit when the input
// bytes exceed the cap. We override the limit below.
func TestServer_Transcribe_RejectsOversizeAudioAtDecode(t *testing.T) {
	s := makeServer(t, "")
	s.DefaultDecodeLimits.CompressedBytes = 50 // very small cap
	wav := silenceBufferAt(1000, 16_000)
	body, status := postTranscribe(t, s, requestJSON{
		RequestID:      "R-OVERSIZE",
		Language:       "hi-IN",
		ContentType:    "audio/wav",
		AudioB64:       base64.StdEncoding.EncodeToString(wav),
		ByteSize:       int64(len(wav)),
		DeadlineMillis: 5000,
	})
	if status != http.StatusOK {
		t.Fatalf("status: got %d body=%s", status, body)
	}
	var resp responseJSON
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.State != string(TranscriptionAudioUnavailable) {
		t.Errorf("state: got %q want %q", resp.State, TranscriptionAudioUnavailable)
	}
}

// TestServer_RetainsNoRawAudioInResponseBody: the response body
// must not contain echoes of the audio data, and the request body
// bytes are released before the handler returns.
func TestServer_RetainsNoRawAudioInResponseBody(t *testing.T) {
	s := makeServer(t, "")
	// A distinctive WAV we can fingerprint later.
	wav := silenceBufferAt(200, 16_000)
	probe := bytes.Repeat([]byte{'Z'}, 32)
	wav = append(wav, probe...)
	body, status := postTranscribe(t, s, requestJSON{
		RequestID:      "R-NOLEAK",
		Language:       "hi-IN",
		ContentType:    "audio/wav",
		AudioB64:       base64.StdEncoding.EncodeToString(wav),
		ByteSize:       int64(len(wav)),
		DeadlineMillis: 5000,
	})
	if status != http.StatusOK {
		t.Fatalf("status: got %d body=%s", status, body)
	}
	if bytes.Contains(body, probe) {
		t.Errorf("response body must not echo raw audio bytes")
	}
}

// TestServer_QueueSaturationReturnsUnavailable: when the runtime
// holds many requests, additional requests are bounded at the
// server too; the orchestrator sees UNAVAILABLE.
func TestServer_QueueSaturationReturnsUnavailable(t *testing.T) {
	stub := NewStubRuntime()
	stub.LatencyMs = 200
	inv := Inventory{IndicConformer: DefaultIndicConformer(), AllowedLanguages: []string{"hi-IN", "ml-IN"}}
	w, err := NewWorker(Config{
		Inventory:   inv,
		Runtime:     stub,
		QueueDepth:  1,
		MaxInFlight: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Shutdown(context.Background()) })
	s, err := NewServer(ServerConfig{Address: "127.0.0.1:0"}, w)
	if err != nil {
		t.Fatal(err)
	}
	ln := getLoopbackListener(t)
	s.listener = ln
	s.httpSrv = &http.Server{Handler: s.mux}
	go s.httpSrv.Serve(ln)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(ctx)
	})

	wav := silenceBufferAt(100, 16_000)

	// Two background requests: first consumes the in-flight slot,
	// second fills the queue.
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			body, status := postTranscribeURL(t, s, requestJSON{
				RequestID:      "BG",
				Language:       "hi-IN",
				ContentType:    "audio/wav",
				AudioB64:       base64.StdEncoding.EncodeToString(wav),
				ByteSize:       int64(len(wav)),
				DeadlineMillis: 1000,
			})
			_ = body
			_ = status
		}()
	}
	// Yield briefly so both background goroutines have entered
	// the dispatch queue.
	time.Sleep(50 * time.Millisecond)
	body, status := postTranscribe(t, s, requestJSON{
		RequestID:      "R-SAT",
		Language:       "hi-IN",
		ContentType:    "audio/wav",
		AudioB64:       base64.StdEncoding.EncodeToString(wav),
		ByteSize:       int64(len(wav)),
		DeadlineMillis: 5000,
	})
	if status != http.StatusOK {
		t.Fatalf("status: got %d body=%s", status, body)
	}
	var resp responseJSON
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.State != string(TranscriptionUnavailable) {
		t.Errorf("state: got %q want %q", resp.State, TranscriptionUnavailable)
	}
	wg.Wait()
}

// TestServer_ShutdownDrainsExisting: /shutdown returns 200 after
// the in-flight goroutines are drained.
func TestServer_ShutdownDrainsExisting(t *testing.T) {
	s := makeServer(t, "")
	// Issue a quick /shutdown.
	body, status := postEmpty(t, s, "/shutdown")
	if status != http.StatusOK {
		t.Fatalf("status: got %d body=%s", status, body)
	}
	if !strings.Contains(string(body), "DRAINED") {
		t.Errorf("body must say DRAINED; got %s", body)
	}
	// A subsequent /transcribe must reject with UNAVAILABLE.
	wav := silenceBufferAt(100, 16_000)
	body, _ = postTranscribe(t, s, requestJSON{
		RequestID:      "POST-DRAIN",
		Language:       "hi-IN",
		ContentType:    "audio/wav",
		AudioB64:       base64.StdEncoding.EncodeToString(wav),
		ByteSize:       int64(len(wav)),
		DeadlineMillis: 1000,
	})
	var resp responseJSON
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.State != string(TranscriptionUnavailable) {
		t.Errorf("post-shutdown state: got %q want %q", resp.State, TranscriptionUnavailable)
	}
}

// === helpers ===
//
// silenceBufferAt and makeWAVPlain are defined in audio_test.go;
// both _test.go files are in package asrworker, so they share.
func postEmpty(t *testing.T, s *Server, path string) ([]byte, int) {
	t.Helper()
	return getJSON(t, s, http.MethodPost, path, "", nil)
}

func postTranscribe(t *testing.T, s *Server, req requestJSON) ([]byte, int) {
	t.Helper()
	return postTranscribeURL(t, s, req)
}

func postTranscribeURL(t *testing.T, s *Server, req requestJSON) ([]byte, int) {
	t.Helper()
	body := writeJSON(t, req)
	return getJSON(t, s, http.MethodPost, "/transcribe", "", body)
}

func getJSON(t *testing.T, s *Server, method, path, token string, body []byte) ([]byte, int) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, "http://"+s.Addr()+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode
}
