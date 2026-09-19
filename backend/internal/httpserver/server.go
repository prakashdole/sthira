package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/httpjson"
)

// ReadinessProber reports whether the service's dependencies and approved
// source state allow it to serve operational traffic. It must actively probe
// real dependencies; environment-variable presence alone is not readiness.
type ReadinessProber interface {
	// Probe returns nil if ready, or an error describing the blocker. The
	// error text is redacted before it reaches the response.
	Probe(ctx context.Context) error
}

// ContextSnapshot is the authoritative server-side context a middle-model
// proposal is validated against. It is resolved per request from server state,
// never from client-echoed request/data-version values.
type ContextSnapshot struct {
	// DataVersion is the server's current snapshot version.
	DataVersion string
	// Jurisdiction is the resolved jurisdiction for the active context.
	Jurisdiction string
	// KnownIDs is the set of IDs present in the active context.
	KnownIDs map[string]bool
	// EnabledLanguages is the enabled language set for the active context.
	EnabledLanguages map[string]bool
}

// ContextResolver resolves the authoritative snapshot for one request. It is
// the seam where P4 will bind a real source snapshot; until one is wired the
// voice-commands endpoint fails closed rather than trusting client echoes.
type ContextResolver interface {
	// Resolve returns the current snapshot, or an error if no authoritative
	// context is available. The error text is redacted before the response.
	Resolve(ctx context.Context) (ContextSnapshot, error)
}

// Server is the bounded /api/v3 HTTP boundary.
type Server struct {
	cfg       Config
	prober    ReadinessProber
	resolver  ContextResolver
	logger    *slog.Logger
	httpSrv   *http.Server
	startedAt time.Time
}

// StaticContextResolver returns a ContextResolver that always serves the same
// snapshot. It is for the demo/test slice only; P4 binds a real source
// snapshot resolver.
func StaticContextResolver(snap ContextSnapshot) ContextResolver {
	return staticResolver{snap: snap}
}

type staticResolver struct{ snap ContextSnapshot }

func (s staticResolver) Resolve(ctx context.Context) (ContextSnapshot, error) {
	return s.snap, nil
}

// Option customizes a Server.
type Option func(*Server)

// WithProber sets the readiness prober. Without one, readiness reports
// BLOCKED (never READY), which is the safe default.
func WithProber(p ReadinessProber) Option { return func(s *Server) { s.prober = p } }

// WithContextResolver sets the server-side snapshot resolver for the
// voice-commands boundary. Without one, that endpoint fails closed (503) rather
// than trusting client-echoed request/data-version values.
func WithContextResolver(r ContextResolver) Option { return func(s *Server) { s.resolver = r } }

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option { return func(s *Server) { s.logger = l } }

// New builds a Server with bounded timeouts and the frozen route set.
func New(cfg Config, opts ...Option) *Server {
	s := &Server{
		cfg:       cfg,
		logger:    slog.Default(),
		startedAt: time.Now().UTC(),
	}
	for _, o := range opts {
		o(s)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", s.withRequestID(s.handleLive))
	mux.HandleFunc("/health/ready", s.withRequestID(s.handleReady))
	mux.HandleFunc("/api/v3/voice/commands", s.withRequestID(s.handleVoiceCommands))

	s.httpSrv = &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	return s
}

// Serve runs the server until ctx is cancelled, then shuts down gracefully.
func (s *Server) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("sthira backend listening", "addr", s.cfg.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		s.logger.Info("shutting down", "timeout", s.cfg.ShutdownTimeout)
		return s.httpSrv.Shutdown(shutdownCtx)
	}
}

// Handler exposes the root handler for in-process tests.
func (s *Server) Handler() http.Handler { return s.httpSrv.Handler }

// --- helpers ---

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "req-unknown"
	}
	return "req-" + hex.EncodeToString(b)
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// writeData writes a success envelope with exactly Data populated.
func (s *Server) writeData(w http.ResponseWriter, r *http.Request, status int, dataVersion string, source contracts.FreshnessState, data any) {
	env := contracts.Envelope{
		RequestID:     requestID(r),
		SchemaVersion: contracts.SchemaVersionV3,
		GeneratedAt:   nowUTC(),
		DataVersion:   dataVersion,
		SourceStatus:  source,
		Data:          data,
	}
	s.writeEnvelope(w, status, env)
}

// writeError writes an error envelope with exactly Errors populated.
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, status int, code, message, field string, retryable bool) {
	env := contracts.Envelope{
		RequestID:     requestID(r),
		SchemaVersion: contracts.SchemaVersionV3,
		GeneratedAt:   nowUTC(),
		DataVersion:   "none",
		SourceStatus:  contracts.FreshnessUnknown,
		Errors: []contracts.APIError{{
			Code:          code,
			Message:       message,
			Field:         field,
			CorrelationID: requestID(r),
			Retryable:     retryable,
		}},
	}
	s.writeEnvelope(w, status, env)
}

func (s *Server) writeEnvelope(w http.ResponseWriter, status int, env contracts.Envelope) {
	body, err := httpjson.Marshal(env)
	if err != nil {
		s.logger.Error("failed to marshal envelope", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"code":"INTERNAL","message":"failed to encode response"}]}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

type ctxKey string

const requestIDKey ctxKey = "request_id"

func requestID(r *http.Request) string {
	if v, ok := r.Context().Value(requestIDKey).(string); ok {
		return v
	}
	return "req-unknown"
}

// withRequestID assigns a request ID for correlation.
func (s *Server) withRequestID(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), requestIDKey, newRequestID())
		next(w, r.WithContext(ctx))
	}
}

// requireMethod enforces the allowed method for a route.
func (s *Server) requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		s.writeError(w, r, http.StatusMethodNotAllowed, contracts.ErrMethodNotAllowed, "method not allowed", "", false)
		return false
	}
	return true
}

// requireJSONContentType enforces an application/json request body.
func (s *Server) requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	// Allow parameters such as charset; compare the media type token.
	mediaType := ct
	for i, c := range ct {
		if c == ';' {
			mediaType = ct[:i]
			break
		}
	}
	if mediaType != "application/json" {
		s.writeError(w, r, http.StatusUnsupportedMediaType, contracts.ErrUnsupportedMedia, "Content-Type must be application/json", "", false)
		return false
	}
	return true
}

// readBoundedBody reads the request body up to the configured limit. It
// returns an error if the body exceeds the limit. MaxBytesReader is given the
// real ResponseWriter so it can signal the connection to close on overflow; an
// overflow surfaces as a read error, which we map to BODY_TOO_LARGE.
func (s *Server) readBoundedBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	limited := http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, &httpjson.FieldError{Code: contracts.ErrBodyTooLarge, Message: "request body exceeds size limit"}
	}
	return body, nil
}

// jsonDecodeError converts a strict-decode failure into an HTTP error envelope.
func (s *Server) jsonDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var fe *httpjson.FieldError
	if errors.As(err, &fe) {
		status := http.StatusBadRequest
		if fe.Code == contracts.ErrBodyTooLarge {
			status = http.StatusRequestEntityTooLarge
		}
		s.writeError(w, r, status, fe.Code, fe.Message, fe.Field, false)
		return
	}
	s.writeError(w, r, http.StatusBadRequest, contracts.ErrMalformedJSON, "invalid request body", "", false)
}
