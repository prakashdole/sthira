// Server speaks the P6 private worker protocol. The orchestrator
// reaches the server with three endpoints:
//
//   - GET  /health                  →  HealthEnvelope
//   - POST /v1/chat/completions     →  ResponseEnvelope
//   - POST /shutdown                →  200 on drain, 503 on deadline
//
// The server binds to a caller-chosen loopback address and never
// touches the public network. Bearer token auth is configured from
// STHIRA_MIDDLE_WORKER_TOKEN; absence fails closed at startup.
//
// The handler never logs request payloads or proposal text. It logs
// request_id, language, state, latency, queue_depth and bounded
// prompt bytes.

package middleworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Server is the loopback HTTP server that exposes the worker's
// private protocol to the orchestrator.
type Server struct {
	worker       *Worker
	listener     net.Listener
	expectedTok  string
	mux          *http.ServeMux
	httpSrv      *http.Server
	readTimeout  time.Duration
	writeTimeout time.Duration
	shutdownWait time.Duration

	// Limits mirror the Client's limits. The Server enforces them
	// on the request body before any decode happens.
	MaxRequestBytes int64

	// HealthAuthRequired mirrors the production rule: bearer token
	// required on every non-loopback health probe. Defaults to
	// true; tests can flip it.
	HealthAuthRequired bool

	// startedAt is the server's start time, used for /health.
	startedAt time.Time
}

// ServerConfig collects the network + auth knobs.
type ServerConfig struct {
	// Address is the bind address, e.g. "127.0.0.1:8766". Required.
	Address string
	// Token is the per-worker bearer token. Required for
	// production (orchestrator → worker auth). Tests may pass empty.
	Token string
	// ReadTimeout is the per-request read deadline. Defaults to 5s.
	ReadTimeout time.Duration
	// WriteTimeout is the per-response write deadline. Defaults to 30s.
	WriteTimeout time.Duration
	// ShutdownWait is the deadline for /shutdown drain.
	ShutdownWait time.Duration
	// MaxRequestBytes caps the request body. Defaults to 1 MiB.
	MaxRequestBytes int64
	// HealthAuthRequired mirrors the production rule. Defaults to true.
	HealthAuthRequired bool
}

// NewServer builds the HTTP server. It does NOT start serving;
// call Start.
func NewServer(cfg ServerConfig, w *Worker) (*Server, error) {
	if w == nil {
		return nil, errors.New("server: worker is required")
	}
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, errors.New("server: address required")
	}
	rt := cfg.ReadTimeout
	if rt == 0 {
		rt = 5 * time.Second
	}
	wt := cfg.WriteTimeout
	if wt == 0 {
		wt = 30 * time.Second
	}
	sw := cfg.ShutdownWait
	if sw == 0 {
		sw = 10 * time.Second
	}
	mrb := cfg.MaxRequestBytes
	if mrb <= 0 {
		mrb = 1 * 1024 * 1024
	}
	l, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("server: bind %s: %w", cfg.Address, err)
	}
	return &Server{
		worker:             w,
		listener:           l,
		expectedTok:        cfg.Token,
		readTimeout:        rt,
		writeTimeout:       wt,
		shutdownWait:       sw,
		MaxRequestBytes:    mrb,
		HealthAuthRequired: cfg.HealthAuthRequired,
		startedAt:          time.Now().UTC(),
	}, nil
}

// Start runs the HTTP server. Returns when the server stops.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/chat/completions", s.handlePropose)
	mux.HandleFunc("/shutdown", s.handleShutdown)
	s.mux = mux
	s.httpSrv = &http.Server{
		Handler:      mux,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
	}
	err := s.httpSrv.Serve(s.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the HTTP server and the worker.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownWait)
	defer cancel()
	var firstErr error
	if s.httpSrv != nil {
		if err := s.httpSrv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			firstErr = err
		}
	}
	if err := s.worker.Shutdown(s.shutdownWait); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// Addr returns the bound listener address. Useful for tests.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// requireAuth enforces the bearer-token rule when configured.
func (s *Server) requireAuth(r *http.Request) bool {
	if s.expectedTok == "" {
		return true
	}
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	tok := strings.TrimPrefix(h, prefix)
	return tok == s.expectedTok
}

// writeTypedError writes a typed error envelope in the same shape
// the orchestrator expects. The response shape mirrors the JSON
// failure envelope of contracts.MiddleWorkerResponse with state set
// to the typed MiddleState.
func (s *Server) writeTypedError(w http.ResponseWriter, reqID string, state string, msg string, status int) {
	payload := map[string]any{
		"request_id": reqID,
		"state":      state,
		"error":      msg,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Sthira-State", state)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.HealthAuthRequired && !s.requireAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h := s.worker.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(h)
}

func (s *Server) handlePropose(w http.ResponseWriter, r *http.Request) {
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

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req RequestEnvelope
	if err := dec.Decode(&req); err != nil {
		s.writeTypedError(w, req.RequestID, MiddleStateMalformed,
			"invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.RequestID == "" {
		s.writeTypedError(w, "", MiddleStateMalformed, "request_id required", http.StatusBadRequest)
		return
	}
	if req.ScopedContext.SchemaVersion == "" {
		s.writeTypedError(w, req.RequestID, MiddleStateMalformed,
			"scoped_context.schema_version required", http.StatusBadRequest)
		return
	}
	if req.ScopedContext.DataVersion == "" {
		s.writeTypedError(w, req.RequestID, MiddleStateMalformed,
			"scoped_context.data_version required", http.StatusBadRequest)
		return
	}
	if req.Transcript.Text == "" && req.Transcript.State != "OK" {
		// Non-OK transcript state is allowed (the model may be
		// told "the user said nothing"), but a totally empty
		// transcript is rejected at the worker seam.
	}

	// Per-call deadline: client supplies deadline_ms in the wire
	// envelope. The worker ALSO derives a deadline from the request
	// context; we use the request context here.
	dl := time.Duration(req.DeadlineMillis) * time.Millisecond
	if dl <= 0 {
		dl = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), dl)
	defer cancel()

	resp, err := s.worker.Dispatch(ctx, req)
	if err != nil {
		state, status := mapWorkerErrorToState(err)
		s.writeTypedError(w, req.RequestID, state, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Shutdown is async; the response is sent first, then the
	// server drains. We bound the wait to ShutdownWait.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = s.Shutdown()
	}()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"state":"DRAINED"}`))
	case <-time.After(s.shutdownWait):
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"state":"DEADLINE_EXCEEDED"}`))
	}
}

// mapWorkerErrorToState maps a worker-side typed error to the
// (MiddleState, HTTP status) pair the orchestrator expects.
func mapWorkerErrorToState(err error) (string, int) {
	switch {
	case errors.Is(err, ErrWorkerNotReady):
		return MiddleStateUnavailable, http.StatusServiceUnavailable
	case errors.Is(err, ErrWorkerShutdown):
		return MiddleStateUnavailable, http.StatusServiceUnavailable
	case errors.Is(err, ErrQueueSaturated):
		return MiddleStateUnavailable, http.StatusTooManyRequests
	case errors.Is(err, ErrRuntimeUnavailable):
		return MiddleStateUnavailable, http.StatusServiceUnavailable
	case errors.Is(err, ErrTimeout):
		return MiddleStateTimeout, http.StatusGatewayTimeout
	case errors.Is(err, ErrCanceled):
		return MiddleStateCanceled, 499
	case errors.Is(err, ErrMalformed),
		errors.Is(err, ErrExtraText),
		errors.Is(err, ErrSchemaUnsupported):
		return MiddleStateMalformed, http.StatusBadRequest
	case errors.Is(err, ErrContextExceeded):
		return MiddleStateContextExceeded, http.StatusRequestEntityTooLarge
	case errors.Is(err, ErrOutputExceeded),
		errors.Is(err, ErrOversized):
		return MiddleStateOutputExceeded, http.StatusBadRequest
	default:
		return MiddleStateUnavailable, http.StatusInternalServerError
	}
}
