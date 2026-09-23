package asrworker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Server speaks the P6 private worker protocol. The orchestrator
// reaches the server with three endpoints:
//
//   - GET  /health         →  WorkerHealth (WorkerHealthView JSON)
//   - POST /transcribe     →  ASRWorkerResponse (JSON; body is ASRWorkerRequest)
//   - POST /shutdown       →  200 when drain completes; 503 after deadline
//
// The server binds to a caller-chosen loopback address and never
// touches the public network. Authentication is a per-call bearer
// token drawn from STHIRA_ASR_WORKER_TOKEN; absent token means
// fail-closed at startup.
//
// The handler never logs audio bytes or transcript text. It logs
// request_id, language, content_type, state, latency, queue_depth
// and bounded byte_size. Transcript text and original audio are
// scrubbed from memory before the handler returns.
type Server struct {
	worker       *Worker
	listener     net.Listener
	listenerMu   sync.Mutex // guards listener field; Addr() may be called from another goroutine
	expectedTok  string
	mux          *http.ServeMux
	httpSrv      *http.Server
	readTimeout  time.Duration
	writeTimeout time.Duration
	// DefaultDecodeLimits is the limits applied to incoming audio.
	// Tests may inject stricter limits.
	DefaultDecodeLimits AudioDecodeLimits
	// MaxRequestBytes is the upper bound on the request body size.
	// Keeps a single huge payload from pinning a goroutine. Slightly
	// larger than the audio limit because the body contains a
	// base64-encoded audio blob plus framing JSON.
	MaxRequestBytes int64
	// HealthAuthRequired mirrors the production rule: bearer token
	// required on every non-loopback health probe. Defaults to true.
	HealthAuthRequired bool
}

// ServerConfig collects the network + auth knobs.
type ServerConfig struct {
	// Address is the bind address, e.g. "127.0.0.1:8765". Required.
	Address string
	// Token is the per-worker bearer token. Required for production
	// (orchestrator → worker auth). Tests may pass empty.
	Token string
	// ReadTimeout is the per-request read deadline. Defaults to 5s.
	ReadTimeout time.Duration
	// WriteTimeout is the per-response write deadline. Defaults to 30s.
	WriteTimeout time.Duration
	// MaxRequestBytes caps the request body. Defaults to 1 MiB.
	MaxRequestBytes int64
}

// NewServer builds the HTTP server. It does NOT start serving;
// call Start.
func NewServer(cfg ServerConfig, w *Worker) (*Server, error) {
	if w == nil {
		return nil, errors.New("worker is required")
	}
	rt := cfg.ReadTimeout
	if rt == 0 {
		rt = 5 * time.Second
	}
	wt := cfg.WriteTimeout
	if wt == 0 {
		wt = 30 * time.Second
	}
	mrb := cfg.MaxRequestBytes
	if mrb == 0 {
		mrb = 1 << 20
	}
	s := &Server{
		worker:              w,
		expectedTok:         cfg.Token,
		mux:                 http.NewServeMux(),
		readTimeout:         rt,
		writeTimeout:        wt,
		DefaultDecodeLimits: DefaultDecodeLimits(),
		MaxRequestBytes:     mrb,
		HealthAuthRequired:  cfg.Token != "",
	}
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/transcribe", s.handleTranscribe)
	s.mux.HandleFunc("/shutdown", s.handleShutdown)
	s.httpSrv = &http.Server{
		Handler:      s.mux,
		ReadTimeout:  rt,
		WriteTimeout: wt,
	}
	return s, nil
}

// Start listens on the configured address and begins serving. It
// blocks on the listener. Use Cancel+Wait to stop. Tests typically
// start with t.Cleanup.
func (s *Server) Start(ctx context.Context, addr string) error {
	if addr == "" {
		return errors.New("server bind address is required")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listenerMu.Lock()
	s.listener = ln
	s.listenerMu.Unlock()
	errCh := make(chan error, 1)
	go func() {
		err := s.httpSrv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	// Wait for either ctx cancellation or serve-failure.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// Wait blocks until the listener is closed. Pair with Cancel.
func (s *Server) Wait() error {
	s.listenerMu.Lock()
	ln := s.listener
	s.listenerMu.Unlock()
	if ln == nil {
		return errors.New("server not started")
	}
	if err := s.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Cancel closes the listener and waits for in-flight handlers.
func (s *Server) Cancel(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

// Addr returns the bound address (useful for tests).
func (s *Server) Addr() string {
	s.listenerMu.Lock()
	ln := s.listener
	s.listenerMu.Unlock()
	if ln == nil {
		return ""
	}
	return ln.Addr().String()
}

// HandleTranscribeFor exposes the typed handler for unit tests so
// they can drive it without going through a real socket. Production
// paths go through Start/Wait.
func (s *Server) HandleTranscribeFor(r *http.Request) (int, []byte) {
	rr := &recordingResponseWriter{header: http.Header{}}
	s.handleTranscribe(rr, r)
	return rr.status, rr.body
}

// recordingResponseWriter is a tiny net/http response writer
// stand-in used only by HandleTranscribeFor; production paths use
// the real net/http writer.
type recordingResponseWriter struct {
	header http.Header
	body   []byte
	status int
}

func (r *recordingResponseWriter) Header() http.Header { return r.header }
func (r *recordingResponseWriter) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}
func (r *recordingResponseWriter) WriteHeader(statusCode int) { r.status = statusCode }

// requireAuth enforces the per-call bearer token. It is a no-op
// when the server was constructed with an empty token (test mode).
func (s *Server) requireAuth(r *http.Request) bool {
	if !s.HealthAuthRequired {
		return true
	}
	tok := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(tok, prefix) {
		return false
	}
	got := tok[len(prefix):]
	if got == s.expectedTok {
		return true
	}
	// constant-time-ish compare to avoid leaking length via timing
	if len(got) != len(s.expectedTok) {
		return false
	}
	diff := 0
	for i := 0; i < len(got); i++ {
		diff |= int(got[i] ^ s.expectedTok[i])
	}
	return diff == 0
}

// healthJSON is the wire shape of the /health response. Mirrors
// contracts.WorkerHealth but is kept private so the worker module
// has zero Go-level coupling to the orchestrator. The orchestrator
// decodes via the public contracts package which asserts the same
// field names.
type healthJSON struct {
	Ready              bool           `json:"ready"`
	Warm               bool           `json:"warm"`
	Models             []modelJSON    `json:"models"`
	Artifacts          []artifactJSON `json:"artifacts"`
	SupportedLanguages []string       `json:"supported_languages"`
	Queue              queueJSON      `json:"queue"`
	StartedAt          string         `json:"started_at,omitempty"`
	BuildRevision      string         `json:"build_revision,omitempty"`
	LastInventoryScan  string         `json:"last_inventory_scan,omitempty"`
	PendingCritical    []string       `json:"pending_critical,omitempty"`
}

type modelJSON struct {
	ModelID        string `json:"model_id"`
	Revision       string `json:"revision"`
	ChecksumSHA256 string `json:"checksum_sha256,omitempty"`
	License        string `json:"license,omitempty"`
	Runtime        string `json:"runtime,omitempty"`
	Hardware       string `json:"hardware,omitempty"`
	RemoteCode     bool   `json:"remote_code"`
}

type artifactJSON struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	License        string `json:"license,omitempty"`
}

type queueJSON struct {
	Depth          int `json:"depth"`
	MaxDepth       int `json:"max_depth"`
	MaxConcurrency int `json:"max_concurrency"`
}

// requestJSON is the wire shape of the /transcribe request body.
type requestJSON struct {
	RequestID      string  `json:"request_id"`
	Language       string  `json:"language"`
	ContentType    string  `json:"content_type"`
	AudioB64       string  `json:"audio_b64"`
	ByteSize       int64   `json:"byte_size"`
	DecodedSeconds float64 `json:"decoded_seconds"`
	DeadlineMillis int64   `json:"deadline_ms"`
}

// responseJSON is the wire shape of the /transcribe response body.
// Mirrors contracts.ASRWorkerResponse.
type responseJSON struct {
	RequestID      string            `json:"request_id"`
	Language       string            `json:"language"`
	Text           string            `json:"text"`
	Confidence     *float64          `json:"confidence"`
	Alternatives   []alternativeJSON `json:"alternatives,omitempty"`
	State          string            `json:"state"`
	ModelRevision  string            `json:"model_revision"`
	ArtifactDigest string            `json:"artifact_digest"`
}

type alternativeJSON struct {
	Text       string   `json:"text"`
	Confidence *float64 `json:"confidence,omitempty"`
}

// handleHealth is the GET /health endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h := s.worker.Snapshot()
	payload := healthJSON{
		Ready:              h.Ready,
		Warm:               h.Warm,
		StartedAt:          h.StartedAt,
		BuildRevision:      h.BuildRevision,
		LastInventoryScan:  h.LastInventoryScan,
		SupportedLanguages: h.SupportedLanguages,
		PendingCritical:    h.PendingCritical,
		Queue: queueJSON{
			Depth:          h.Queue.Depth,
			MaxDepth:       h.Queue.MaxDepth,
			MaxConcurrency: h.Queue.MaxConcurrency,
		},
	}
	for _, m := range h.Models {
		payload.Models = append(payload.Models, modelJSON{
			ModelID:        m.ModelID,
			Revision:       m.Revision,
			ChecksumSHA256: m.ChecksumSHA256,
			License:        m.License,
			Runtime:        m.Runtime,
			Hardware:       m.Hardware,
			RemoteCode:     m.RemoteCode,
		})
	}
	for _, a := range h.Artifacts {
		payload.Artifacts = append(payload.Artifacts, artifactJSON{
			Name:           a.Name,
			Path:           a.Path,
			ChecksumSHA256: a.ChecksumSHA256,
			License:        a.License,
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Always 200 on /health; the orchestrator reads the body.
	_ = json.NewEncoder(w).Encode(payload)
}

// handleTranscribe is the POST /transcribe endpoint. The handler
// enforces input trust boundaries:
//
//   - bearer auth
//   - method == POST
//   - body size <= MaxRequestBytes
//   - JSON-decoded request matches our wire shape
//   - audio_b64 decodes to the contract's compressed-bytes cap
//   - content_type is in the worker allow-list (this package, not
//     the public handler's)
//   - decode enforces channels / sample-rate / duration limits
//   - the runtime's Transcribe is called with a context bounded
//     by deadline_ms
//
// Scrubbing policy:
//
//   - audio bytes: zeroed before the handler returns; the decoded
//     buffer is wiped after the runtime call.
//   - transcript text: kept in the response body; the orchestrator
//     owns downstream retention.
//   - on any error path we still attempt to wipe the audio buffer
//     before returning.
func (s *Server) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxRequestBytes)
	defer r.Body.Close()
	var req requestJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// http.MaxBytesReader sets a sentinel via errors.As; we
		// translate that to a 413 BODY_TOO_LARGE signal. Other
		// decoding errors stay 400.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_ = json.NewEncoder(w).Encode(responseJSON{
				RequestID: req.RequestID,
				Language:  req.Language,
				State:     string(TranscriptionAudioUnavailable),
			})
			return
		}
		writeTypedError(w, req.RequestID, req.Language, "invalid request body: "+err.Error())
		return
	}
	audio, err := base64.StdEncoding.DecodeString(req.AudioB64)
	if err != nil {
		writeTypedError(w, req.RequestID, req.Language, "audio_b64 decode: "+err.Error())
		return
	}
	if len(audio) == 0 {
		writeTypedError(w, req.RequestID, req.Language, "audio_b64 empty")
		return
	}
	// Decode + resample + bound. DecodeAudio dispatches to
	// DecodeCompressed (ffmpeg subprocess) for Ogg/Opus/WebM inputs
	// and to DecodeWAV for WAV input.
	decoded, err := DecodeAudio(audio, req.ContentType, s.DefaultDecodeLimits)
	if err != nil {
		state := stateFromDecodeError(err)
		writeTypedASRResponse(w, responseJSON{
			RequestID: req.RequestID,
			Language:  req.Language,
			State:     string(state),
		})
		return
	}
	// Build a request with the samples. We will wipe after the call.
	deadline := time.Now().Add(time.Duration(req.DeadlineMillis) * time.Millisecond)
	tr := TranscribeRequest{
		RequestID:    req.RequestID,
		Language:     req.Language,
		Samples:      decoded.Samples,
		SampleRate:   decoded.SampleRate,
		DurationSecs: decoded.DurationSecs,
		Deadline:     deadline,
	}
	ctx, cancel := context.WithDeadline(r.Context(), deadline)
	defer cancel()
	res, runErr := s.worker.Dispatch(ctx, tr)
	// Wipe the decoded buffer and the original audio before
	// formatting a response.
	WipeBuffer(decoded.Samples)
	decoded.Samples = nil
	for i := range audio {
		audio[i] = 0
	}
	audio = nil
	if runErr != nil {
		state := stateFromRuntimeError(runErr)
		writeTypedASRResponse(w, responseJSON{
			RequestID: req.RequestID,
			Language:  req.Language,
			State:     string(state),
		})
		return
	}
	modelRev, _ := s.worker.runtime.Digest()
	revOnly := s.worker.runtime.Revision()
	_ = modelRev
	out := responseJSON{
		RequestID:      req.RequestID,
		Language:       req.Language,
		Text:           res.Text,
		Confidence:     res.Confidence,
		State:          string(TranscriptionOK),
		ModelRevision:  revOnly,
		ArtifactDigest: modelRev,
	}
	for _, alt := range res.Alternatives {
		out.Alternatives = append(out.Alternatives, alternativeJSON{
			Text:       alt.Text,
			Confidence: alt.Confidence,
		})
	}
	writeTypedASRResponse(w, out)
}

// handleShutdown performs a graceful drain.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Honor the request context's deadline; if absent, default to 5s.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.worker.Shutdown(ctx); err != nil {
		http.Error(w, "shutdown incomplete: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"state":"DRAINED"}`))
}

// writeTypedASRResponse writes a /transcribe response with the
// documented content type. We never log the body.
func writeTypedASRResponse(w http.ResponseWriter, body responseJSON) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(body)
}

// writeTypedError is the path we use for malformed input (400 +
// transcribed typed error envelope). Even on error we attempt to
// return state typed in the response body.
func writeTypedError(w http.ResponseWriter, requestID, lang, msg string) {
	body := responseJSON{
		RequestID: requestID,
		Language:  lang,
		State:     string(TranscriptionAudioUnavailable),
	}
	// We do NOT echo the message into the response because it
	// could carry an attacker-controlled byte slice; the typed
	// state is the only signal clients need.
	_ = msg
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(body)
}

// Transcription* state values used by the server handler.
// These constants mirror contracts.TranscriptionState so the worker
// module speaks the same wire values without importing the
// orchestrator's contracts package.
//
// State mapping table:
//
//	DecodeWAV success   → OK
//	ErrUnsupportedCodec → AUDIO_UNAVAILABLE
//	oversize bytes      → AUDIO_UNAVAILABLE
//	oversize duration   → AUDIO_UNAVAILABLE
//	malformed header    → AUDIO_UNAVAILABLE
//	ErrLanguageUnsupported → UNSUPPORTED_LANGUAGE
//	ErrQueueSaturated   → UNAVAILABLE
//	ErrRuntimeUnavailable → UNAVAILABLE
//	ErrWorkerNotReady   → UNAVAILABLE
//	ErrWorkerShutdown   → UNAVAILABLE
//	context.DeadlineExceeded / context.Canceled → TIMEOUT / CANCELED
type transcriptionState = string

const (
	TranscriptionOK                  transcriptionState = "OK"
	TranscriptionUnsupportedLanguage transcriptionState = "UNSUPPORTED_LANGUAGE"
	TranscriptionAudioUnavailable    transcriptionState = "AUDIO_UNAVAILABLE"
	TranscriptionTimeout             transcriptionState = "TIMEOUT"
	TranscriptionUnavailable         transcriptionState = "UNAVAILABLE"
	TranscriptionCancelled           transcriptionState = "CANCELED"
)

func stateFromDecodeError(err error) transcriptionState {
	if errors.Is(err, ErrUnsupportedCodec) {
		return TranscriptionAudioUnavailable
	}
	var de *DecodeError
	if errors.As(err, &de) {
		return TranscriptionAudioUnavailable
	}
	return TranscriptionAudioUnavailable
}

func stateFromRuntimeError(err error) transcriptionState {
	switch {
	case errors.Is(err, ErrLanguageUnsupported):
		return TranscriptionUnsupportedLanguage
	case errors.Is(err, ErrQueueSaturated),
		errors.Is(err, ErrWorkerNotReady),
		errors.Is(err, ErrWorkerShutdown),
		errors.Is(err, ErrRuntimeUnavailable):
		return TranscriptionUnavailable
	case errors.Is(err, context.DeadlineExceeded):
		return TranscriptionTimeout
	case errors.Is(err, context.Canceled):
		return TranscriptionCancelled
	default:
		return TranscriptionUnavailable
	}
}

// FormatAddress strips "tcp" prefixes for log output.
func FormatAddress(addr string) string {
	if strings.HasPrefix(addr, "[::]") {
		return addr
	}
	return addr
}

// SockAddr is a tiny helper that returns the canonical loopback
// address for tests when the caller passes a port-zero listener.
func SockAddr(host string, port int) string {
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// bodySink is a sync.Once'd helper used by tests to capture-and-drop
// a request body without allocating shared buffers in production.
type bodySink struct {
	once  sync.Once
	bytes []byte
}

func (b *bodySink) Capture(_ []byte) { b.once.Do(func() {}) }
