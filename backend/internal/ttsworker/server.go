package ttsworker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Server speaks the P6 private TTS worker protocol. The orchestrator
// is its only expected client; endpoints are bound to 127.0.0.1 in
// foundation mode. Bearer-token authentication is enforced when a
// token is configured.
type Server struct {
	worker       *Worker
	bearerToken  string
	listener     net.Listener
	mux          *http.ServeMux
	httpSrv      *http.Server
	shuttingDown atomic.Bool
	maxBody      int64
}

// ServerConfig collects the network + auth knobs.
type ServerConfig struct {
	// Address is the bind address, e.g. "127.0.0.1:8765". Required.
	Address string
	// BearerToken, when non-empty, requires an Authorization: Bearer
	//<token> header on every endpoint. Endpoints for /health and
	// /synthesize both apply.
	BearerToken string
	// MaxRequestBytes caps the body for /synthesize and /shutdown.
	// 1 MiB by default; smaller caps are honored.
	MaxRequestBytes int64
	// Worker is the worker this server fronts. Required.
	Worker *Worker
}

// NewServer builds the HTTP server. It does NOT start serving; call
// Start. Returns an error if the worker is missing.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Worker == nil {
		return nil, errors.New("worker is required")
	}
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = 1 * 1024 * 1024
	}
	s := &Server{
		worker:      cfg.Worker,
		bearerToken: cfg.BearerToken,
		mux:         http.NewServeMux(),
		maxBody:     cfg.MaxRequestBytes,
	}
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/synthesize", s.handleSynthesize)
	s.mux.HandleFunc("/shutdown", s.handleShutdown)
	s.httpSrv = &http.Server{
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if cfg.Address != "" {
		ln, err := net.Listen("tcp", cfg.Address)
		if err != nil {
			return nil, err
		}
		s.listener = ln
		s.httpSrv.Addr = ln.Addr().String()
	}
	return s, nil
}

// Start binds addr (if not already bound) and serves until Cancel.
func (s *Server) Start(ctx context.Context, addr string) error {
	if addr != "" && s.listener == nil {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		s.listener = ln
	}
	if s.listener == nil {
		return errors.New("no listener; bind address is required")
	}
	err := s.httpSrv.Serve(s.listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Wait blocks until Start returns.
func (s *Server) Wait() {}

// Cancel drains and closes.
func (s *Server) Cancel(ctx context.Context) error {
	s.shuttingDown.Store(true)
	if err := s.httpSrv.Shutdown(ctx); err != nil {
		return err
	}
	if err := s.worker.Shutdown(); err != nil && !errors.Is(err, ErrWorkerShutdown) {
		return err
	}
	return nil
}

// Addr returns the bound address (useful for tests).
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// handleHealth returns the typed WorkerHealth snapshot.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(s.worker.Health())
}

// handleSynthesize decodes a typed SynthesizeRequest and dispatches.
func (s *Server) handleSynthesize(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if s.shuttingDown.Load() {
		http.Error(w, "worker draining", http.StatusServiceUnavailable)
		return
	}
	if r.Body == nil {
		http.Error(w, "missing body", http.StatusBadRequest)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}
	var req SynthesizeRequest
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "could not decode request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateSynthRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.worker.Synthesize(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrQueueSaturated):
			http.Error(w, "queue saturated", http.StatusServiceUnavailable)
		case errors.Is(err, ErrWorkerNotReady):
			http.Error(w, "worker not ready", http.StatusServiceUnavailable)
		case errors.Is(err, ErrWorkerShutdown):
			http.Error(w, "worker shutdown", http.StatusServiceUnavailable)
		default:
			http.Error(w, "unavailable: "+err.Error(), http.StatusServiceUnavailable)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleShutdown drains the worker. The HTTP server itself is shut
// down from Cancel; calling Server.Shutdown from inside the handler
// is racy because the active POST connection would block Shutdown.
// Instead, the handler signals shutdown to the worker and replies
// 200; the orchestrator's owner code calls Cancel(ctx) afterwards.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	_ = s.worker.Shutdown()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"state":"OK","shutdown":"drained"}`))
}

// requireAuth enforces the bearer token when configured.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.bearerToken == "" {
		return true
	}
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		http.Error(w, "missing bearer", http.StatusUnauthorized)
		return false
	}
	got := authz[len(prefix):]
	if !constantTimeEq(got, s.bearerToken) {
		http.Error(w, "invalid bearer", http.StatusUnauthorized)
		return false
	}
	return true
}

func constantTimeEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var r byte
	for i := 0; i < len(a); i++ {
		r |= a[i] ^ b[i]
	}
	return r == 0
}

func validateSynthRequest(r *SynthesizeRequest) error {
	if r == nil {
		return errors.New("nil request")
	}
	if strings.TrimSpace(r.RequestID) == "" {
		return errors.New("request_id is required")
	}
	if r.SpeechKey == "" {
		return errors.New("speech_key is required")
	}
	if r.Language == "" {
		return errors.New("language is required")
	}
	if r.Text == "" {
		return errors.New("text is required")
	}
	if r.SourceVersion <= 0 {
		return errors.New("source_version is required")
	}
	if r.TemplateVersion <= 0 {
		return errors.New("template_version is required")
	}
	return nil
}
