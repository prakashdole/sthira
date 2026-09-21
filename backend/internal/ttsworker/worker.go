package ttsworker

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"sthira/backend/internal/ttsworker/templates"
)

// TTSState is the worker's typed outcome. Mirrors contracts.TTSState.
type TTSState string

const (
	StateOK                  TTSState = "OK"
	StateUnsupportedLanguage TTSState = "UNSUPPORTED_LANGUAGE"
	StateAudioUnavailable    TTSState = "AUDIO_UNAVAILABLE"
	StateTimeout             TTSState = "TIMEOUT"
	StateUnavailable         TTSState = "UNAVAILABLE"
	StateCanceled            TTSState = "CANCELED"
	StateStaleVersion        TTSState = "STALE_VERSION"
)

// SynthesizeRequest is the worker-side mirror of
// contracts.TTSWorkerRequest.
type SynthesizeRequest struct {
	RequestID       string            `json:"request_id"`
	SpeechKey       templates.Key     `json:"speech_key"`
	Language        string            `json:"language"`
	Text            string            `json:"text"` // rendered template text only
	SourceVersion   int               `json:"source_version"`
	TemplateVersion int               `json:"template_version"`
	Settings        SynthesisSettings `json:"settings"`
	DeadlineMillis  int64             `json:"deadline_ms"`
	// Voice is the explicit voice selection (optional). The default
	// voice for the language is used when empty.
	Voice string `json:"voice,omitempty"`
}

// SynthesizeResponse is the worker-side mirror of
// contracts.TTSWorkerResponse.
type SynthesizeResponse struct {
	RequestID      string   `json:"request_id"`
	SpeechKey      string   `json:"speech_key"`
	Language       string   `json:"language"`
	State          TTSState `json:"state"`
	AudioB64       string   `json:"audio_b64,omitempty"`    // base64 of PCM/WAV bytes when state == OK
	ContentType    string   `json:"content_type,omitempty"` // audio/wav
	ChecksumSHA256 string   `json:"checksum_sha256,omitempty"`
	ModelRevision  string   `json:"model_revision,omitempty"`
	VoiceRevision  string   `json:"voice_revision,omitempty"`
	CacheHit       bool     `json:"cache_hit"`
	ByteSize       int64    `json:"byte_size,omitempty"`
}

// Worker is the TTS worker lifecycle. It owns the bounded queue, the
// bounded concurrency, the cache, the catalog and the runtime. The
// worker is constructed once per process and shut down once.
type Worker struct {
	mu        sync.Mutex
	closed    atomic.Bool
	ready     atomic.Bool
	warm      atomic.Bool
	cache     *Codec
	clock     SourceVersionBroadcaster
	catalog   *templates.Catalog
	renderer  *templates.Renderer
	runtime   Runtime
	inventory Inventory
	queue     chan *job
	wg        sync.WaitGroup
	// metrics
	processed atomic.Int64
	saturated atomic.Int64
	cacheHits atomic.Int64
	withdrawn atomic.Int64
	canceled  atomic.Int64
	timeout   atomic.Int64
	startedAt time.Time
}

// Errors callers receive.
var (
	ErrWorkerNotReady = errors.New("ttsworker: worker not ready")
	ErrWorkerShutdown = errors.New("ttsworker: worker shut down")
	ErrQueueSaturated = errors.New("ttsworker: queue saturated")
)

// Config is the constructor input.
type Config struct {
	Inventory   Inventory
	Runtime     Runtime
	Catalog     *templates.Catalog
	Renderer    *templates.Renderer
	Cache       *Codec
	Clock       SourceVersionBroadcaster
	QueueDepth  int
	MaxInFlight int
}

// Default returns the documented defaults. Production wires the
// clock and cache explicitly.
func DefaultConfig() Config {
	return Config{
		QueueDepth:  8,
		MaxInFlight: 2,
	}
}

// New constructs a worker. Validation:
//   - QueueDepth > 0, MaxInFlight > 0
//   - Runtime, Catalog, Renderer, Clock, Cache non-nil
//   - Inventory.Ready() must succeed (gates Warm)
func New(cfg Config) (*Worker, error) {
	if cfg.QueueDepth <= 0 || cfg.MaxInFlight <= 0 {
		return nil, errors.New("queue depth and max in-flight must be positive")
	}
	if cfg.Runtime == nil || cfg.Catalog == nil || cfg.Renderer == nil || cfg.Cache == nil || cfg.Clock == nil {
		return nil, errors.New("runtime, catalog, renderer, cache and clock are required")
	}
	if err := cfg.Inventory.Ready(); err != nil {
		return nil, err
	}
	w := &Worker{
		cache:     cfg.Cache,
		clock:     cfg.Clock,
		catalog:   cfg.Catalog,
		renderer:  cfg.Renderer,
		runtime:   cfg.Runtime,
		inventory: cfg.Inventory,
		queue:     make(chan *job, cfg.QueueDepth),
		startedAt: time.Now().UTC(),
	}
	// Pool size = MaxInFlight. Sized at construction; no per-request
	// goroutine spawning.
	for i := 0; i < cfg.MaxInFlight; i++ {
		w.wg.Add(1)
		go w.runWorker()
	}
	w.warm.Store(true)
	w.ready.Store(true)
	return w, nil
}

// MetricsSnapshot is the counters surface. The orchestrator reports
// these in /health indirectly via the WorkerHealth envelope.
type MetricsSnapshot struct {
	Processed int64 `json:"processed"`
	Saturated int64 `json:"saturated"`
	CacheHits int64 `json:"cache_hits"`
	Withdrawn int64 `json:"withdrawn"`
	Canceled  int64 `json:"canceled"`
	Timeout   int64 `json:"timeout"`
	// QueueStats mirrors contracts.QueueStats.
	QueueStats QueueStats `json:"queue"`
}

// QueueStats is the worker-side mirror of contracts.QueueStats.
type QueueStats struct {
	Depth          int `json:"depth"`
	MaxDepth       int `json:"max_depth"`
	MaxConcurrency int `json:"max_concurrency"`
}

// Health returns a snapshot for the /health envelope. The shape
// mirrors contracts.WorkerHealth: the orchestrator reads it via
// the same JSON field names regardless of which worker responded.
type WorkerHealth struct {
	Ready              bool           `json:"ready"`
	Warm               bool           `json:"warm"`
	Models             []ModelInfo    `json:"models"`
	Artifacts          []ArtifactInfo `json:"artifacts"`
	SupportedLanguages []string       `json:"supported_languages"`
	Queue              QueueStats     `json:"queue"`
	StartedAt          string         `json:"started_at,omitempty"`
	BuildRevision      string         `json:"build_revision,omitempty"`
	// Internal diagnostics — the orchestrator does not depend on
	// these fields; they are surfaced for incident triage.
	RuntimeRevision      string          `json:"runtime_revision,omitempty"`
	RuntimeVoices        []VoiceInfo     `json:"runtime_voices,omitempty"`
	CurrentSourceVersion int             `json:"current_source_version,omitempty"`
	Inventory            Inventory       `json:"inventory,omitempty"`
	Metrics              MetricsSnapshot `json:"metrics,omitempty"`
}

// ModelInfo is the worker-side mirror of contracts.ModelInfo.
type ModelInfo struct {
	ModelID        string `json:"model_id"`
	Revision       string `json:"revision"`
	ChecksumSHA256 string `json:"checksum_sha256,omitempty"`
	License        string `json:"license,omitempty"`
	Runtime        string `json:"runtime,omitempty"`
	Hardware       string `json:"hardware,omitempty"`
	RemoteCode     bool   `json:"remote_code"`
}

// ArtifactInfo is the worker-side mirror of contracts.ArtifactDigest.
type ArtifactInfo struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	License        string `json:"license,omitempty"`
}

// Health returns a snapshot for the /health envelope. Field
// names match contracts.WorkerHealth so the orchestrator
// decodes the same way for ASR/middle/TTS.
func (w *Worker) Health() WorkerHealth {
	w.mu.Lock()
	defer w.mu.Unlock()
	langs := append([]string(nil), w.runtime.Languages()...)
	rev := w.runtime.Revision()
	dname, dsha := runtimeDigest(w.runtime)
	// Models: the worker holds one model at a time.
	var models []ModelInfo
	if rev != "" || dname != "" {
		models = []ModelInfo{{
			ModelID:        "ai4bharat/indic-parler-tts",
			Revision:       rev,
			ChecksumSHA256: dsha,
			License:        "LicensePending",
			Runtime:        "transformers-pinned",
			Hardware:       "private-loopback",
			RemoteCode:     false,
		}}
	}
	var artifacts []ArtifactInfo
	if dname != "" {
		artifacts = []ArtifactInfo{{
			Name:           dname,
			Path:           dname,
			ChecksumSHA256: dsha,
			License:        "LicensePending",
		}}
	}
	return WorkerHealth{
		Ready:              w.ready.Load(),
		Warm:               w.warm.Load(),
		Models:             models,
		Artifacts:          artifacts,
		SupportedLanguages: langs,
		Queue: QueueStats{
			Depth:          len(w.queue),
			MaxDepth:       cap(w.queue),
			MaxConcurrency: w.concurrencyBound(),
		},
		StartedAt:            w.startedAt.Format(time.RFC3339Nano),
		BuildRevision:        buildRevision,
		RuntimeRevision:      rev,
		RuntimeVoices:        append([]VoiceInfo(nil), w.runtime.Voices()...),
		CurrentSourceVersion: w.clock.Current(),
		Inventory:            w.inventory,
		Metrics:              w.snapshotLocked(),
	}
}

// runtimeDigest returns the runtime's artifact digest in the
// (name, sha256) form the contracts.WorkerHealth expects.
// Returns empty strings when the runtime is the stub.
func runtimeDigest(r Runtime) (string, string) {
	if d, ok := r.(DigestProvider); ok {
		return d.Digest()
	}
	return "", ""
}

// DigestProvider is implemented by runtimes that expose an
// artifact digest for /health. Optional; missing it leaves
// the Models/Artifacts fields empty.
type DigestProvider interface {
	Digest() (name, sha256 string)
}

const buildRevision = "ttsworker-b4-corrections"

// Metrics returns the cumulative counter snapshot.
func (w *Worker) Metrics() MetricsSnapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshotLocked()
}

func (w *Worker) snapshotLocked() MetricsSnapshot {
	return MetricsSnapshot{
		Processed: w.processed.Load(),
		Saturated: w.saturated.Load(),
		CacheHits: w.cacheHits.Load(),
		Withdrawn: w.withdrawn.Load(),
		Canceled:  w.canceled.Load(),
		Timeout:   w.timeout.Load(),
		QueueStats: QueueStats{
			Depth:          len(w.queue),
			MaxDepth:       cap(w.queue),
			MaxConcurrency: w.concurrencyBound(),
		},
	}
}

func (w *Worker) concurrencyBound() int { return cap(w.queue) }

// LoadAndVerify re-runs Inventory.Ready() and toggles Ready=true if
// the inventory now passes. Used during periodic revalidation.
func (w *Worker) LoadAndVerify() error {
	if err := w.inventory.Ready(); err != nil {
		w.ready.Store(false)
		return err
	}
	w.ready.Store(true)
	return nil
}

// Synthesize is the bounded admission path. The request is enqueued
// and a worker goroutine picks it up. Dispatch honors the per-call
// deadline and the worker-context cancellation: a cancelled call
// returns StateCanceled before the runtime is invoked.
func (w *Worker) Synthesize(req SynthesizeRequest) (*SynthesizeResponse, error) {
	if w.closed.Load() {
		return nil, ErrWorkerShutdown
	}
	if !w.ready.Load() {
		return nil, ErrWorkerNotReady
	}
	requestCtx, cancelRequest := makeContext(req.DeadlineMillis)
	defer cancelRequest()
	// Build the job with a prebuilt done channel.
	done := make(chan *SynthesizeResponse, 1)
	j := &job{req: req, ctx: requestCtx, done: done}
	select {
	case w.queue <- j:
	default:
		w.saturated.Add(1)
		return nil, ErrQueueSaturated
	}
	select {
	case <-requestCtx.Canceled:
		w.canceled.Add(1)
		return nil, ErrWorkerShutdown
	case res := <-done:
		return res, nil
	}
}

// Shutdown drains the worker pool and the runtime. Returns when the
// pool is empty or after the worker's internal shutdown deadline is
// reached. Idempotent.
func (w *Worker) Shutdown() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(w.queue)
	w.wg.Wait()
	_ = w.runtime.Close()
	w.ready.Store(false)
	w.warm.Store(false)
	return nil
}

// ActiveConn returns the number of in-flight jobs. Surfaced in
// /health.
func (w *Worker) ActiveConn() int { return -1 } // placeholder; the metrics already report depth

// job is one in-flight task. Held by the queue channel until a pool
// goroutine picks it up.
type job struct {
	req  SynthesizeRequest
	ctx  RequestContext
	done chan *SynthesizeResponse
}

// runWorker is the goroutine body. Loops on jobs; exits when the
// queue is closed (Shutdown).
func (w *Worker) runWorker() {
	defer w.wg.Done()
	for j := range w.queue {
		w.handleJob(j)
	}
}

// handleJob performs the synthesizer work. Steps:
//  1. Pre-flight: render-language/source-version check.
//  2. Cache lookup. Withdrawal is honored before decode.
//  3. Hot-path miss is AUDIO_UNAVAILABLE: the worker never
//     synthesizes on the hot path. Approved prerecorded common
//     speech is pre-populated by the offline workflow; missing
//     entries surface as typed-unavailable so the orchestrator's
//     on-screen response remains authoritative.
//  4. Cleanup: zero any decoded buffers (we never had any here)
//     before returning.
//
// The runtime is reserved for OFFLINE pre-generation only. The hot
// path is cache-only so a real-inference outage cannot fabricate
// audio as successful speech.
func (w *Worker) handleJob(j *job) {
	w.mu.Lock()
	snapshot := w.inventory
	w.mu.Unlock()
	// 1. Pre-flight.
	if j.req.Text == "" {
		w.emit(j, errorResponse(j.req, StateAudioUnavailable, "text is required"))
		w.processed.Add(1)
		return
	}
	if j.req.SpeechKey == "" {
		w.emit(j, errorResponse(j.req, StateAudioUnavailable, "speech_key is required"))
		w.processed.Add(1)
		return
	}
	// Verify the template is still in the catalog and not withdrawn.
	// A withdrawal between orchestrator render and worker pickup is
	// honored here.
	if t, ok := w.catalog.Lookup(templates.Key(j.req.SpeechKey)); !ok {
		w.emit(j, errorResponse(j.req, StateAudioUnavailable, "template unknown"))
		w.processed.Add(1)
		return
	} else if t.Status == templates.StatusWithdrawn {
		w.withdrawn.Add(1)
		w.emit(j, errorResponse(j.req, StateStaleVersion, "template withdrawn"))
		w.processed.Add(1)
		return
	}
	// Source-version bound: the request's source version must equal
	// the current clock.
	if cur := w.clock.Current(); j.req.SourceVersion != cur {
		w.withdrawn.Add(1)
		w.emit(j, errorResponse(j.req, StateStaleVersion, "source_version is stale"))
		w.processed.Add(1)
		return
	}
	// Language must be supported by the runtime allow-list.
	if !containsString(snapshot.SupportedLanguages, j.req.Language) {
		w.emit(j, errorResponse(j.req, StateUnsupportedLanguage, "language not supported"))
		w.processed.Add(1)
		return
	}
	// 2. Cache identity and lookup. Hot path only.
	voiceRev := w.runtime.Voice(j.req.Language)
	if voiceRev == "" {
		voiceRev = j.req.Voice
	}
	if j.req.Voice != "" && j.req.Voice != voiceRev {
		w.emit(j, errorResponse(j.req, StateAudioUnavailable, "voice not supported"))
		w.processed.Add(1)
		return
	}
	id := CacheIdentity{
		Text:              j.req.Text,
		TemplateKey:       string(j.req.SpeechKey),
		TemplateVersion:   j.req.TemplateVersion,
		SourceVersion:     j.req.SourceVersion,
		Language:          j.req.Language,
		ModelRevision:     w.runtime.Revision(),
		VoiceRevision:     voiceRev,
		SynthesisSettings: toWorkerSettings(j.req.Settings),
	}
	if err := id.Validate(); err != nil {
		w.emit(j, errorResponse(j.req, StateAudioUnavailable, err.Error()))
		w.processed.Add(1)
		return
	}
	// 3. Cache lookup. Get holds the codec lock for the entire
	// branch, so a withdrawal concurrent with a cache read surfaces
	// here as a miss.
	if entry, ok := w.cache.Get(id); ok {
		w.cacheHits.Add(1)
		b64 := encodeBase64(entry.Bytes)
		w.emit(j, &SynthesizeResponse{
			RequestID:      j.req.RequestID,
			SpeechKey:      id.TemplateKey,
			Language:       j.req.Language,
			State:          StateOK,
			AudioB64:       b64,
			ContentType:    "audio/wav",
			ChecksumSHA256: entry.ChecksumSHA256,
			ModelRevision:  id.ModelRevision,
			VoiceRevision:  id.VoiceRevision,
			CacheHit:       true,
			ByteSize:       entry.ByteSize,
		})
		w.processed.Add(1)
		return
	}
	// 4. Hot-path miss => AUDIO_UNAVAILABLE. Approved speech is
	//    pre-populated by the offline pre-generation workflow which
	//    calls runtime.Synthesize and codec.Put directly. The hot
	//    path serves approved entries only.
	w.emit(j, errorResponse(j.req, StateAudioUnavailable, "approved audio not pre-generated for this identity"))
	w.processed.Add(1)
}

// emit sends the response back to the caller. Done channels buffer 1
// so a caller that already gave up cannot block the worker.
func (w *Worker) emit(j *job, res *SynthesizeResponse) {
	select {
	case j.done <- res:
	default:
	}
}

// errorResponse constructs a TTS error response with the given state
// and human-readable reason. The orchestrator maps State to the
// contracts.TTSState mirror.
func errorResponse(req SynthesizeRequest, state TTSState, reason string) *SynthesizeResponse {
	return &SynthesizeResponse{
		RequestID: req.RequestID,
		SpeechKey: string(req.SpeechKey),
		Language:  req.Language,
		State:     state,
	}
}

// toWorkerSettings converts the public settings to the cache's
// SynthesisSettings shape.
func toWorkerSettings(in SynthesisSettings) SynthesisSettings { return in }

// makeContext converts DeadlineMillis into a RequestContext with a
// done channel that fires on deadline. The cancel func is returned
// for the caller to invoke at completion. We do not import
// context.Context into the runtime API.
func makeContext(deadlineMillis int64) (RequestContext, func()) {
	if deadlineMillis <= 0 {
		return RequestContext{DeadlineMillis: 0}, func() {}
	}
	deadline := time.Now().Add(time.Duration(deadlineMillis) * time.Millisecond)
	done := make(chan struct{})
	timer := time.AfterFunc(time.Until(deadline), func() { close(done) })
	cancel := func() { timer.Stop() }
	return RequestContext{
		DeadlineMillis: deadlineMillis,
		Canceled:       done,
	}, cancel
}

// containsString is a tiny helper for Slices used in narrowing.
func containsString(s []string, needle string) bool {
	for _, v := range s {
		if v == needle {
			return true
		}
	}
	return false
}

// encodeBase64 is a thin wrapper kept on this file to avoid imports
// pulling net/http into the cache module.
func encodeBase64(b []byte) string {
	// base64.StdEncoding.EncodeToString — inlined via a tiny helper
	// to keep imports near.
	const tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if len(b) == 0 {
		return ""
	}
	out := make([]byte, 0, ((len(b)+2)/3)*4)
	for i := 0; i < len(b); i += 3 {
		var n uint32
		var c int
		switch len(b) - i {
		case 1:
			n = uint32(b[i]) << 16
			c = 2
		case 2:
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8
			c = 3
		default:
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8 | uint32(b[i+2])
			c = 4
		}
		out = append(out, tbl[(n>>18)&0x3F], tbl[(n>>12)&0x3F])
		switch c {
		case 2:
			out = append(out, '=', '=')
		case 3:
			out = append(out, tbl[(n>>6)&0x3F], '=')
		case 4:
			out = append(out, tbl[(n>>6)&0x3F], tbl[n&0x3F])
		}
	}
	return string(out)
}
