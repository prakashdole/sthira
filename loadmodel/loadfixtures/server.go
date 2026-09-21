// Package loadfixtures implements an isolated, self-contained synthetic HTTP
// fixture server used only by the P7 load harness. It mirrors the route shape
// of the real /api/v3 boundary so the k6 scenarios can drive the same URLs,
// but every backend is in-memory and switchable via query/header knobs:
//
//   /health/live, /health/ready
//   /api/v3/regions/{id}/manifest                   — public cached read
//   /api/v3/packages/{id}/versions/{v}              — public cached read
//   /api/v3/resources/{id}                          — public cached read
//   /api/v3/sessions                                — citizen session issuance
//   /api/v3/places/resolve                          — place lookup
//   /api/v3/guidance/query                          — eligible destinations
//   /api/v3/reservations                            — write (capacity mutation)
//   /api/v3/reservations/{id}                       — owner read
//   /api/v3/reservations/{id}/events                — write (arrive/cancel/...)
//   /api/v3/voice/transcriptions                    — ASR-only
//   /api/v3/voice/process                           — full pipeline
//   /api/v3/voice/speech                            — TTS-only
//
// This server is NOT the production backend and NEVER shares its database. It
// exists solely so the load harness can drive a target with bounded,
// reproducible behaviour. The real backend must be benchmarked separately,
// against representative hardware and with real workers (O04).
package loadfixtures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sthira/backend/loadmodel/loadfixtures/internal"
)

// Config configures the synthetic fixture server. All knobs default to a
// neutral state and can be flipped per run via env vars or per-request
// query parameters (the latter is for the harness's outage/cold scenarios).
type Config struct {
	// Addr is the bind address (e.g. "127.0.0.1:0" for an ephemeral port).
	Addr string
	// CacheHitRate is the warm-cache hit probability for /regions/* /packages/*
	// /resources/*. The harness overrides this per scenario.
	CacheHitRate float64
	// ManifestBytes, CardBytes, ResourceBytes are synthetic body sizes for the
	// cached public delivery paths.
	ManifestBytes int
	CardBytes     int
	ResourceBytes int
	// ServiceASR, ServiceMiddle, ServiceTTS are the dummy-worker service
	// times used by /voice/process. They are sleeping only; they do not
	// model real GPU work.
	ServiceASR     time.Duration
	ServiceMiddle  time.Duration
	ServiceTTS     time.Duration
	// QueueDepth bounds how many voice requests can sit in the in-process
	// admission queue; further arrivals fail-fast with 503 QUEUE_SATURATED.
	QueueDepth int
	// OutageRate is the probability that a /packages/* fetch returns 503 to
	// simulate a source outage. The harness sets it to 1.0 for the
	// source-outage scenario and 0.0 otherwise.
	OutageRate float64
	// WriteContention is the number of concurrent reservations per
	// (facility_id, service_date). The harness sets it to 1 to exercise
	// last-space contention.
	WriteContention int
	// Logger receives structured timing lines (one per request). The bench
	// runs the server with the logger pointed at /dev/null; the harness
	// captures HTTP-level latency via k6.
	Logger *slog.Logger
}

// Server is the isolated synthetic fixture server.
type Server struct {
	cfg Config

	httpSrv *http.Server
	addr    atomic.Value // string

	// request counters (used by /health/ready, /metrics, scenario sanity).
	mu           sync.Mutex
	requests     uint64
	cacheHits    uint64
	cacheMisses  uint64
	queueRejects uint64
	voiceOK      uint64
	voiceErr     uint64
	writesOK     uint64
	writesErr    uint64

	// in-process voice admission queue (BoundedQueue semantic).
	vqMu  sync.Mutex
	vqNow int
	vqMax int

	// synthetic package + region store.
	store *syntheticStore
}

// New builds the server with the given config. Reasonable defaults are
// applied for any field left zero.
func New(cfg Config) *Server {
	if cfg.ManifestBytes == 0 {
		cfg.ManifestBytes = 4096
	}
	if cfg.CardBytes == 0 {
		cfg.CardBytes = 16384
	}
	if cfg.ResourceBytes == 0 {
		cfg.ResourceBytes = 8192
	}
	if cfg.ServiceASR == 0 {
		cfg.ServiceASR = 1500 * time.Millisecond
	}
	if cfg.ServiceMiddle == 0 {
		cfg.ServiceMiddle = 4 * time.Second
	}
	if cfg.ServiceTTS == 0 {
		cfg.ServiceTTS = 2 * time.Second
	}
	if cfg.QueueDepth == 0 {
		cfg.QueueDepth = 8
	}
	if cfg.WriteContention == 0 {
		cfg.WriteContention = 8
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Server{
		cfg:   cfg,
		store: newSyntheticStore(),
		vqMax: cfg.QueueDepth,
	}
}

// Listen starts the HTTP server on cfg.Addr. Pass ":0" to bind an
// ephemeral port. Use Addr() to read the resolved address.
func (s *Server) Listen() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", s.handleLive)
	mux.HandleFunc("/health/ready", s.handleReady)
	mux.HandleFunc("/metrics", s.handleMetrics)

	mux.HandleFunc("/api/v3/regions/", s.handleRegion)
	mux.HandleFunc("/api/v3/packages/", s.handlePackage)
	mux.HandleFunc("/api/v3/resources/", s.handleResource)

	mux.HandleFunc("/api/v3/sessions", s.handleCreateSession)
	mux.HandleFunc("/api/v3/places/resolve", s.handleResolvePlace)
	mux.HandleFunc("/api/v3/guidance/query", s.handleGuidanceQuery)

	mux.HandleFunc("/api/v3/reservations", s.handleReservations)
	mux.HandleFunc("/api/v3/reservations/", s.handleReservationByID)

	mux.HandleFunc("/api/v3/voice/transcriptions", s.handleVoiceTranscriptions)
	mux.HandleFunc("/api/v3/voice/process", s.handleVoiceProcess)
	mux.HandleFunc("/api/v3/voice/speech", s.handleVoiceSpeech)

	s.httpSrv = &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           withRequestID(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := internal.NewListener(s.cfg.Addr)
	if err != nil {
		return err
	}
	s.addr.Store(ln.Addr().String())

	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.cfg.Logger.Error("serve", "err", err)
		}
	}()
	return nil
}

// Addr returns the resolved listen address (only valid after Listen).
func (s *Server) Addr() string {
	if v, ok := s.addr.Load().(string); ok {
		return v
	}
	return s.cfg.Addr
}

// Shutdown gracefully stops the server with the given context.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

// Stats returns a point-in-time snapshot of internal counters for the report.
type Stats struct {
	Requests     uint64  `json:"requests"`
	CacheHits    uint64  `json:"cache_hits"`
	CacheMisses  uint64  `json:"cache_misses"`
	HitRate      float64 `json:"cache_hit_rate"`
	QueueRejects uint64  `json:"queue_rejects"`
	VoiceOK      uint64  `json:"voice_ok"`
	VoiceErr     uint64  `json:"voice_err"`
	WritesOK     uint64  `json:"writes_ok"`
	WritesErr    uint64  `json:"writes_err"`
}

// Stats returns the current counters.
func (s *Server) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Stats{
		Requests:     s.requests,
		CacheHits:    s.cacheHits,
		CacheMisses:  s.cacheMisses,
		QueueRejects: s.queueRejects,
		VoiceOK:      s.voiceOK,
		VoiceErr:     s.voiceErr,
		WritesOK:     s.writesOK,
		WritesErr:    s.writesErr,
	}
	total := out.CacheHits + out.CacheMisses
	if total > 0 {
		out.HitRate = float64(out.CacheHits) / float64(total)
	}
	return out
}

// ResetStats zeroes the counters (used between scenarios).
func (s *Server) ResetStats() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = 0
	s.cacheHits = 0
	s.cacheMisses = 0
	s.queueRejects = 0
	s.voiceOK = 0
	s.voiceErr = 0
	s.writesOK = 0
	s.writesErr = 0
}

// SetCacheHitRate flips the cache hit probability at runtime. The harness
// uses this to start a cold-cache scenario.
func (s *Server) SetCacheHitRate(r float64) {
	s.cfg.CacheHitRate = r
}

// SetOutageRate flips the source-outage probability at runtime.
func (s *Server) SetOutageRate(r float64) {
	s.cfg.OutageRate = r
}

// --- handlers ---

func (s *Server) handleLive(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, map[string]any{
		"status":     "LIVE",
		"started_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	// The fixture always answers READY; the load harness controls outages
	// per-route via knobs.
	writeData(w, http.StatusOK, map[string]any{"status": "READY"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.Stats())
}

func (s *Server) handleRegion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET/HEAD only")
		return
	}
	s.bumpRequest()
	if s.cacheShouldHit() {
		s.bumpCacheHit()
		writeCachedBytes(w, r, s.cfg.ManifestBytes, "region/manifest")
		return
	}
	s.bumpCacheMiss()
	// Simulated origin fetch — small synthetic delay.
	time.Sleep(2 * time.Millisecond)
	writeCachedBytes(w, r, s.cfg.ManifestBytes, "region/manifest")
}

func (s *Server) handlePackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET/HEAD only")
		return
	}
	s.bumpRequest()
	if s.cfg.OutageRate > 0 && internal.RandFloat() < s.cfg.OutageRate {
		writeError(w, http.StatusServiceUnavailable, "DATA_UNAVAILABLE", "source outage (synthetic)")
		return
	}
	if s.cacheShouldHit() {
		s.bumpCacheHit()
		writeCachedBytes(w, r, s.cfg.CardBytes, "package/card")
		return
	}
	s.bumpCacheMiss()
	time.Sleep(5 * time.Millisecond)
	writeCachedBytes(w, r, s.cfg.CardBytes, "package/card")
}

func (s *Server) handleResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET/HEAD only")
		return
	}
	s.bumpRequest()
	if s.cacheShouldHit() {
		s.bumpCacheHit()
		writeCachedBytes(w, r, s.cfg.ResourceBytes, "aux/resource")
		return
	}
	s.bumpCacheMiss()
	time.Sleep(1 * time.Millisecond)
	writeCachedBytes(w, r, s.cfg.ResourceBytes, "aux/resource")
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	s.bumpRequest()
	// Always succeeds; bounded work.
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1024))
	_ = body
	id := newID("SES")
	tok := newID("TOK")
	writeData(w, http.StatusCreated, map[string]any{
		"session_id": id,
		"token":      tok,
		"expires_in": 86400,
	})
}

func (s *Server) handleResolvePlace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	s.bumpRequest()
	// Synthetic deterministic response keyed on the query string.
	body, _ := io.ReadAll(io.LimitReader(r.Body, 8192))
	_ = body
	time.Sleep(2 * time.Millisecond)
	writeData(w, http.StatusOK, map[string]any{
		"matches": []map[string]any{
			{"id": "PLACE-DEMO-1", "name": "Meppadi", "jurisdiction": "KL", "score": 0.91},
		},
	})
}

func (s *Server) handleGuidanceQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	s.bumpRequest()
	body, _ := io.ReadAll(io.LimitReader(r.Body, 8192))
	_ = body
	// Synthetic eligible destinations — deterministic, synthetic.
	time.Sleep(3 * time.Millisecond)
	writeData(w, http.StatusOK, map[string]any{
		"eligible_destinations": []map[string]any{
			{"facility_id": "FACILITY-DEMO-1", "safe_zone_id": "SZ-DEMO-1", "route_id": "ROUTE-DEMO-1", "order": 0},
			{"facility_id": "FACILITY-DEMO-2", "safe_zone_id": "SZ-DEMO-2", "route_id": "ROUTE-DEMO-2", "order": 1},
		},
		"data_version": "SYN-1",
	})
}

func (s *Server) handleReservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	s.bumpRequest()
	body, _ := io.ReadAll(io.LimitReader(r.Body, 16384))
	_ = body
	// Real reservation would lock + validate + commit; the synthetic store
	// applies the per-(facility_id, service_date) serialisation that the
	// real backend requires (D28).
	idem := r.Header.Get("Idempotency-Key")
	res, err := s.store.Reserve(idem, time.Now().UTC())
	if err != nil {
		s.bumpWriteErr()
		writeError(w, http.StatusConflict, "CAPACITY_CONFLICT", err.Error())
		return
	}
	s.bumpWriteOK()
	writeData(w, http.StatusCreated, res)
}

func (s *Server) handleReservationByID(w http.ResponseWriter, r *http.Request) {
	// Synthetic: handle both GET /reservations/{id} and POST /reservations/{id}/events
	path := strings.TrimPrefix(r.URL.Path, "/api/v3/reservations/")
	if strings.HasSuffix(path, "/events") {
		s.handleStayEvent(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET only")
		return
	}
	s.bumpRequest()
	id := strings.SplitN(path, "/", 2)[0]
	res, ok := s.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "reservation not found")
		return
	}
	writeData(w, http.StatusOK, res)
}

func (s *Server) handleStayEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	s.bumpRequest()
	body, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	_ = body
	// Synthetic arrive/cancel/depart: ack only, deterministic.
	writeData(w, http.StatusOK, map[string]any{"event": "ACK", "received_at": time.Now().UTC().Format(time.RFC3339)})
}

// --- voice routes ---

func (s *Server) admitVoice() bool {
	s.vqMu.Lock()
	defer s.vqMu.Unlock()
	if s.vqNow >= s.vqMax {
		return false
	}
	s.vqNow++
	return true
}

func (s *Server) releaseVoice() {
	s.vqMu.Lock()
	s.vqNow--
	s.vqMu.Unlock()
}

func (s *Server) handleVoiceTranscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	s.bumpRequest()
	if !s.admitVoice() {
		s.bumpQueueReject()
		writeError(w, http.StatusServiceUnavailable, "QUEUE_SATURATED", "voice queue full")
		return
	}
	defer s.releaseVoice()
	// Bounded audio: MaxBytes is enforced by Go's net/http default; we
	// just sleep for the ASR service time.
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<19))
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_VALUE", "empty body")
		return
	}
	time.Sleep(s.cfg.ServiceASR)
	writeData(w, http.StatusOK, map[string]any{
		"request_id": requestIDFromContext(r),
		"language":   r.Header.Get("X-Language"),
		"text":       "synthetic transcript",
		"confidence": nil,
		"state":      "OK",
	})
}

func (s *Server) handleVoiceProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	s.bumpRequest()
	if !s.admitVoice() {
		s.bumpQueueReject()
		writeError(w, http.StatusServiceUnavailable, "QUEUE_SATURATED", "voice queue full")
		return
	}
	defer s.releaseVoice()
	// ASR + middle + (optional TTS).
	time.Sleep(s.cfg.ServiceASR)
	time.Sleep(s.cfg.ServiceMiddle)
	// Decode request to see if render=tts.
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<19))
	var req map[string]any
	_ = json.Unmarshal(body, &req)
	renderTTS := false
	if r, ok := req["render"].(map[string]any); ok {
		if r["kind"] == "tts" {
			renderTTS = true
		}
	}
	if renderTTS {
		time.Sleep(s.cfg.ServiceTTS)
	}
	s.bumpVoiceOK()
	writeData(w, http.StatusOK, map[string]any{
		"request_id": requestIDFromContext(r),
		"state":      "OK",
		"validated_proposal": map[string]any{
			"action": "SHOW_CHOICES",
		},
		"data_version": "SYN-1",
	})
}

func (s *Server) handleVoiceSpeech(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST only")
		return
	}
	s.bumpRequest()
	if !s.admitVoice() {
		s.bumpQueueReject()
		writeError(w, http.StatusServiceUnavailable, "QUEUE_SATURATED", "voice queue full")
		return
	}
	defer s.releaseVoice()
	body, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	_ = body
	time.Sleep(s.cfg.ServiceTTS)
	writeData(w, http.StatusOK, map[string]any{
		"request_id":       requestIDFromContext(r),
		"audio_id":         newID("AUD"),
		"content_type":     "audio/wav",
		"byte_size":        s.cfg.ResourceBytes,
		"checksum_sha256":  newID("CHK"),
		"cache_hit":        false,
		"language":         r.Header.Get("X-Language"),
		"voice_version":    "v1",
		"template_version": 1,
		"source_version":   1,
		"model_revision":   "synthetic-1",
	})
}

// --- helpers ---

func (s *Server) cacheShouldHit() bool {
	return internal.RandFloat() < s.cfg.CacheHitRate
}

func (s *Server) bumpRequest() {
	s.mu.Lock()
	s.requests++
	s.mu.Unlock()
}

func (s *Server) bumpCacheHit() {
	s.mu.Lock()
	s.cacheHits++
	s.mu.Unlock()
}

func (s *Server) bumpCacheMiss() {
	s.mu.Lock()
	s.cacheMisses++
	s.mu.Unlock()
}

func (s *Server) bumpQueueReject() {
	s.mu.Lock()
	s.queueRejects++
	s.mu.Unlock()
}

func (s *Server) bumpVoiceOK() {
	s.mu.Lock()
	s.voiceOK++
	s.mu.Unlock()
}

func (s *Server) bumpVoiceErr() {
	s.mu.Lock()
	s.voiceErr++
	s.mu.Unlock()
}

func (s *Server) bumpWriteOK() {
	s.mu.Lock()
	s.writesOK++
	s.mu.Unlock()
}

func (s *Server) bumpWriteErr() {
	s.mu.Lock()
	s.writesErr++
	s.mu.Unlock()
}

func writeData(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":            data,
		"errors":          nil,
		"data_version":    "SYN-1",
		"freshness_class": "SYNTHETIC_DEMO",
	})
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":            nil,
		"data_version":    "SYN-1",
		"freshness_class": "SYNTHETIC_DEMO",
		"errors":          []map[string]any{{"code": errCode, "message": msg, "retryable": code >= 500}},
	})
}

func writeCachedBytes(w http.ResponseWriter, r *http.Request, n int, kind string) {
	h := sha256.Sum256([]byte(kind + ":" + strconv.Itoa(n)))
	etag := `"` + hex.EncodeToString(h[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=300, must-revalidate")
	w.Header().Set("Content-Length", strconv.Itoa(n))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	body := make([]byte, n)
	for i := range body {
		body[i] = byte(h[i%len(h)])
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func newID(prefix string) string {
	b := make([]byte, 8)
	// Deterministic length; we don't need cryptographic strength here.
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (i % 8))
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newID("REQ")
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(withReqID(r.Context(), id)))
	})
}

type reqIDKey struct{}

func withReqID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, reqIDKey{}, id)
}

func requestIDFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(reqIDKey{}).(string); ok {
		return v
	}
	return r.Header.Get("X-Request-ID")
}
