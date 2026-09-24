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
	"sthira/backend/internal/offlinedelivery"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/store"
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
// the seam where P4 binds a real source snapshot; until one is wired the
// voice-commands endpoint fails closed rather than trusting client echoes.
type ContextResolver interface {
	// Resolve returns the current snapshot for the requested jurisdiction, or an
	// error if no authoritative context is available for it. The jurisdiction is
	// an untrusted lookup input: it selects WHICH snapshot to resolve, never
	// proves authorization or snapshot contents. An empty/unknown jurisdiction
	// yields no snapshot (fail closed); the resolver never substitutes another
	// jurisdiction's package. The error text is redacted before the response.
	Resolve(ctx context.Context, jurisdiction string) (ContextSnapshot, error)
}

// Server is the bounded /api/v3 HTTP boundary.
type Server struct {
	cfg       Config
	prober    ReadinessProber
	resolver  ContextResolver
	logger    *slog.Logger
	httpSrv   *http.Server
	startedAt time.Time
	// store is the durable persistence root for the P4 destination/stay routes.
	// Nil in foundation mode; those routes then fail closed (503).
	store *store.Store
	// operatorVerifier is the trusted identity/MFA boundary for operator session
	// issuance. Nil means issuance fails closed (no self-granted operator tokens).
	operatorVerifier OperatorVerifier
	// syntheticExercise is the explicit server-side test/exercise dependency.
	// Nil in production; synthetic evidence is then rejected for commitments.
	syntheticExercise SyntheticExercise
	// pubSource is the source for P5 offline delivery (manifests, cards, resources).
	pubSource   offlinedelivery.PublicationSource
	deliveryCfg *offlinedelivery.Config
	// voiceProcess is the handler for P6 voice pipeline routes.
	// When nil, routes are registered in unavailable mode (fail closed 503).
	voiceProcess *VoiceProcessHandler
	// metrics exposes a low-cardinality pipeline snapshot for the
	// observability endpoint. Nil means pipeline metrics are absent.
	metrics PipelineMetricsSnapshotter
	// workersHealthFn returns per-stage worker health summaries when wired.
	// Nil means workers are absent from the observability snapshot.
	workersHealthFn func() []WorkerHealthSummary
}

// WithVoiceProcess wires the voice process handler for the /api/v3/voice/{transcriptions,process,speech} routes.
func WithVoiceProcess(h *VoiceProcessHandler) Option {
	return func(s *Server) { s.voiceProcess = h }
}

// WithMetricsSnapshotter wires the pipeline metrics source for the
// /api/v3/observability/metrics endpoint. Production passes the orchestrator's
// *orchestration.Metrics; nil means the field stays absent from the snapshot.
func WithMetricsSnapshotter(m PipelineMetricsSnapshotter) Option {
	return func(s *Server) { s.metrics = m }
}

// WithWorkersHealth wires the function that returns per-stage worker health
// summaries for the /api/v3/observability/metrics endpoint. Nil means the
// workers field stays absent from the snapshot.
func WithWorkersHealth(fn func() []WorkerHealthSummary) Option {
	return func(s *Server) { s.workersHealthFn = fn }
}

// WithPprof enables diagnostic pprof endpoints guarded by the provided secret token.
func WithPprof(token string) Option {
	return func(s *Server) {
		s.cfg.EnablePprof = true
		s.cfg.PprofToken = token
	}
}

// WithAccessLog enables structured, privacy-preserving request completion logging.
func WithAccessLog(enable bool) Option {
	return func(s *Server) {
		s.cfg.EnableAccessLog = enable
	}
}

// persistedResolver adapts the store's persisted context resolution to the
// ContextResolver seam. It resolves the snapshot for the REQUESTED jurisdiction
// only — never the highest-version package across unrelated jurisdictions — and
// fails closed when that jurisdiction has no current authorized OPERATIONAL
// package.
type persistedResolver struct{ st *store.Store }

func (p persistedResolver) Resolve(ctx context.Context, jurisdiction string) (ContextSnapshot, error) {
	snap, err := store.ResolveContext(ctx, p.st.DB(), jurisdiction, time.Now().UTC())
	if err != nil {
		return ContextSnapshot{}, err
	}
	return ContextSnapshot{
		DataVersion:      snap.DataVersion,
		Jurisdiction:     snap.Jurisdiction,
		KnownIDs:         snap.KnownIDs,
		EnabledLanguages: snap.EnabledLanguages,
	}, nil
}

// WithPersistedContextResolver wires the database-backed context resolver for
// the voice-commands boundary. The snapshot is derived from the current
// authorized OPERATIONAL package; absent/expired/unauthorized/quarantined
// evidence fails closed. Distinct from the static demo resolver, which is
// explicitly non-operational.
func WithPersistedContextResolver(st *store.Store) Option {
	return func(s *Server) { s.resolver = persistedResolver{st: st} }
}

// StaticContextResolver returns a ContextResolver that always serves the same
// snapshot regardless of jurisdiction. It is for the demo/test slice only; P4
// binds a real source snapshot resolver.
func StaticContextResolver(snap ContextSnapshot) ContextResolver {
	return staticResolver{snap: snap}
}

type staticResolver struct{ snap ContextSnapshot }

func (s staticResolver) Resolve(ctx context.Context, _ string) (ContextSnapshot, error) {
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

// WithStore wires the durable store for the P4 destination/stay routes. Without
// it those routes fail closed (503) rather than fabricating success.
func WithStore(st *store.Store) Option { return func(s *Server) { s.store = st } }

// WithOperatorVerifier wires the trusted identity/MFA boundary for operator
// session issuance. Without it operator issuance fails closed (503); a
// request-body MFA flag is never accepted as evidence.
func WithOperatorVerifier(v OperatorVerifier) Option {
	return func(s *Server) { s.operatorVerifier = v }
}

// SyntheticExercise defines the server-controlled boundary for running
// synthetic/demo exercise flows through real HTTP. In production this is
// nil (disabled), which fails closed: synthetic evidence (packages, source
// artifacts, routes) is rejected for operational commitments. It cannot
// be enabled by request headers, body fields, query parameters, or an ordinary
// production request.
type SyntheticExercise interface {
	AllowSynthetic() bool
}

// StaticSyntheticExercise returns a SyntheticExercise dependency with a fixed
// setting. Used only for isolated test configuration.
func StaticSyntheticExercise(allow bool) SyntheticExercise {
	return staticSyntheticExercise{allow: allow}
}

type staticSyntheticExercise struct{ allow bool }

func (s staticSyntheticExercise) AllowSynthetic() bool { return s.allow }

// WithSyntheticExercise wires the explicit server-side test/exercise dependency
// needed to run synthetic flows through real HTTP. Without it (or when nil),
// synthetic evidence is rejected for operational commitments.
func WithSyntheticExercise(ex SyntheticExercise) Option {
	return func(s *Server) { s.syntheticExercise = ex }
}

func (s *Server) allowSynthetic() bool {
	return s.syntheticExercise != nil && s.syntheticExercise.AllowSynthetic()
}

// WithPublicationSource wires the publication source for P5 offline delivery.
func WithPublicationSource(src offlinedelivery.PublicationSource) Option {
	return func(s *Server) { s.pubSource = src }
}

// WithDeliveryConfig configures P5 offline delivery options such as cache TTLs.
func WithDeliveryConfig(cfg offlinedelivery.Config) Option {
	return func(s *Server) { s.deliveryCfg = &cfg }
}

type emptyPublicationSource struct{}

func (emptyPublicationSource) GetManifest(ctx context.Context, jurisdiction string) (*offlinedelivery.ManifestRecord, error) {
	return nil, offlinedelivery.ErrNotFound
}
func (emptyPublicationSource) GetCard(ctx context.Context, packageID string, version int) (*offlinedelivery.CardRecord, error) {
	return nil, offlinedelivery.ErrNotFound
}
func (emptyPublicationSource) GetResource(ctx context.Context, resourceID string) (*offlinedelivery.ResourceContent, error) {
	return nil, offlinedelivery.ErrNotFound
}

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

	// P4 citizen destination/stay routes. Public reads need no session; writes
	// and the private read path require a live citizen session (Bearer token).
	mux.HandleFunc("/api/v3/sessions", s.withRequestID(s.handleCreateSession))
	mux.HandleFunc("/api/v3/places/resolve", s.withRequestID(s.handleResolvePlace))
	mux.HandleFunc("/api/v3/guidance/query", s.withRequestID(s.handleGuidanceQuery))
	mux.HandleFunc("/api/v3/reservations", s.withRequestID(s.withSession(s.handleCreateReservation)))
	mux.HandleFunc("/api/v3/reservations/{id}", s.withRequestID(s.withSession(s.handleGetReservation)))
	mux.HandleFunc("/api/v3/reservations/{id}/events", s.withRequestID(s.withSession(s.handleStayEvent)))

	// P4 operator operations routes. Issuance is MFA-gated at the boundary;
	// operational routes require a live OPERATOR session with verified MFA and
	// are jurisdiction-scoped per handler.
	mux.HandleFunc("/api/v3/operations/sessions", s.withRequestID(s.handleCreateOperatorSession))
	mux.HandleFunc("/api/v3/operations/sources/{id}/transitions", s.withRequestID(s.withOperator(s.handleSourceTransition)))
	mux.HandleFunc("/api/v3/operations/sources/{id}/quarantine", s.withRequestID(s.withOperator(s.handleSourceQuarantine)))
	mux.HandleFunc("/api/v3/operations/stays/{id}/corrections", s.withRequestID(s.withOperator(s.handleStayCorrection)))

	// P5 public offline delivery routes (manifest, card, and auxiliary resources).
	pubSrc := s.pubSource
	if pubSrc == nil && s.store != nil {
		pubSrc = NewStorePublicationSource(s.store)
	}
	if pubSrc == nil {
		pubSrc = emptyPublicationSource{}
	}
	delCfg := offlinedelivery.DefaultConfig()
	if s.deliveryCfg != nil {
		delCfg = *s.deliveryCfg
	}
	deliveryHandler := offlinedelivery.NewHandler(delCfg, pubSrc, s.logger)
	deliveryHandler.RegisterRoutes(mux, s.withRequestID)

	// Test-only crash fault-injection endpoint; a no-op unless built with the
	// `crashtest` tag. Never present in production builds.
	s.registerCrashHook(mux)

	// Diagnostic pprof endpoints behind authentication token (when enabled).
	s.registerPprofRoutes(mux)

	// P6 voice pipeline routes (transcriptions, full pipeline process, speech synthesis).
	// When no voice process handler is configured, incomplete model configuration
	// stays unavailable: the routes are registered and enforce HTTP methods (returning 405
	// on non-POST), while returning 503 (MODEL_UNAVAILABLE) on requests.
	voiceHandler := s.voiceProcess
	if voiceHandler == nil {
		voiceHandler = NewVoiceProcessHandler(nil, orchestration.DefaultLimits())
	}
	voiceHandler.RegisterVoiceRoutes(mux, s.withRequestID)

	// Low-cardinality operational metrics endpoint (token-guarded via
	// cfg.PprofToken). Without a configured token the handler fails closed.
	mux.HandleFunc(observabilityRoute, s.withRequestID(s.handleObservability))

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

// withRequestID assigns a request ID for correlation and logs access metrics when enabled.
func (s *Server) withRequestID(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqID := newRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		start := time.Now()
		tw := newStatusTrackingResponseWriter(w)
		next(tw, r.WithContext(ctx))
		if s.cfg.EnableAccessLog {
			s.logAccess(reqID, r.Method, r.URL.Path, tw.statusCode, time.Since(start), tw.bytesWritten)
		}
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
