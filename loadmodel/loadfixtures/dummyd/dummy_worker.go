// Package dummyd implements the private P6 worker protocol behind three HTTP
// endpoints. The voice-load harness uses these as fixed-latency, deterministic
// dummy workers; they are NOT real ASR / middle / TTS inference. Throughput
// measured against these dummies is not GPU throughput.
package dummyd

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
	"sync"
	"sync/atomic"
	"time"

	"sthira/backend/loadmodel/loadfixtures/internal"
)

// DummyWorkerConfig configures one dummy worker.
type DummyWorkerConfig struct {
	// Addr is ":0" for ephemeral.
	Addr string
	// Kind is "asr" | "middle" | "tts"; controls the route mounted and the
	// service time used when none is set.
	Kind string
	// ServiceTime is how long the worker sleeps before responding.
	ServiceTime time.Duration
	// ErrorRate is the probability of responding 503 MODEL_UNAVAILABLE
	// (used to exercise the orchestrator's fail-closed path).
	ErrorRate float64
	// Concurrency limits in-flight requests. The harness uses this to
	// force queue saturation from the worker side.
	Concurrency int
	// Logger receives structured timing lines.
	Logger *slog.Logger
}

// DummyWorker is the synthetic worker process. It listens on its own port.
type DummyWorker struct {
	cfg  DummyWorkerConfig
	hs   *http.Server
	addr atomic.Value

	mu        sync.Mutex
	inflight  int
	processed uint64
	rejected  uint64
	ready     atomic.Bool
}

// StartDummyWorker builds, binds and serves one worker. Use Addr() to read
// the resolved port.
func StartDummyWorker(cfg DummyWorkerConfig) (*DummyWorker, error) {
	if cfg.ServiceTime == 0 {
		switch cfg.Kind {
		case "asr":
			cfg.ServiceTime = 1500 * time.Millisecond
		case "middle":
			cfg.ServiceTime = 4 * time.Second
		case "tts":
			cfg.ServiceTime = 2 * time.Second
		default:
			cfg.ServiceTime = 1 * time.Second
		}
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 32
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	w := &DummyWorker{cfg: cfg}
	w.ready.Store(true)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", w.handleHealth)
	mux.HandleFunc("/transcribe", w.handleTranscribe)
	mux.HandleFunc("/v1/chat/completions", w.handleChat)
	mux.HandleFunc("/synthesize", w.handleSynthesize)
	w.hs = &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	ln, err := internal.NewListener(cfg.Addr)
	if err != nil {
		return nil, err
	}
	w.addr.Store(ln.Addr().String())
	go func() {
		if err := w.hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cfg.Logger.Error("dummy worker serve", "kind", cfg.Kind, "err", err)
		}
	}()
	return w, nil
}

// Addr returns the resolved listen address.
func (w *DummyWorker) Addr() string {
	if v, ok := w.addr.Load().(string); ok {
		return v
	}
	return w.cfg.Addr
}

// Shutdown gracefully stops the worker.
func (w *DummyWorker) Shutdown(ctx context.Context) error {
	if w.hs == nil {
		return nil
	}
	return w.hs.Shutdown(ctx)
}

// Stats returns request counters.
type WorkerStats struct {
	Processed uint64 `json:"processed"`
	Rejected  uint64 `json:"rejected"`
}

// Stats returns the worker's counters.
func (w *DummyWorker) Stats() WorkerStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return WorkerStats{Processed: w.processed, Rejected: w.rejected}
}

// SetReady flips the worker readiness flag.
func (w *DummyWorker) SetReady(ready bool) { w.ready.Store(ready) }

func (w *DummyWorker) handleHealth(wr http.ResponseWriter, _ *http.Request) {
	if !w.ready.Load() {
		wr.Header().Set("Content-Type", "application/json")
		wr.WriteHeader(http.StatusServiceUnavailable)
		_, _ = wr.Write([]byte(`{"ready":false}`))
		return
	}
	wr.Header().Set("Content-Type", "application/json")
	wr.WriteHeader(http.StatusOK)
	_, _ = wr.Write([]byte(`{"ready":true,"warm":true}`))
}

func (w *DummyWorker) tryAdmit() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.inflight >= w.cfg.Concurrency {
		w.rejected++
		return false
	}
	w.inflight++
	return true
}

func (w *DummyWorker) release() {
	w.mu.Lock()
	w.inflight--
	w.mu.Unlock()
}

func (w *DummyWorker) bumpProcessed() {
	w.mu.Lock()
	w.processed++
	w.mu.Unlock()
}

func (w *DummyWorker) handleTranscribe(wr http.ResponseWriter, r *http.Request) {
	if !w.tryAdmit() {
		wr.Header().Set("Content-Type", "application/json")
		wr.WriteHeader(http.StatusServiceUnavailable)
		_, _ = wr.Write([]byte(`{"code":"QUEUE_SATURATED"}`))
		return
	}
	defer w.release()
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<19))
	if internal.RandFloat() < w.cfg.ErrorRate {
		wr.Header().Set("Content-Type", "application/json")
		wr.WriteHeader(http.StatusServiceUnavailable)
		_, _ = wr.Write([]byte(`{"code":"MODEL_UNAVAILABLE"}`))
		return
	}
	time.Sleep(w.cfg.ServiceTime)
	w.bumpProcessed()
	wr.Header().Set("Content-Type", "application/json")
	wr.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(wr).Encode(map[string]any{
		"text":       "synthetic transcript",
		"language":   r.Header.Get("X-Language"),
		"confidence": nil,
		"state":      "OK",
	})
}

func (w *DummyWorker) handleChat(wr http.ResponseWriter, r *http.Request) {
	if !w.tryAdmit() {
		wr.Header().Set("Content-Type", "application/json")
		wr.WriteHeader(http.StatusServiceUnavailable)
		_, _ = wr.Write([]byte(`{"code":"QUEUE_SATURATED"}`))
		return
	}
	defer w.release()
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<15))
	if internal.RandFloat() < w.cfg.ErrorRate {
		wr.Header().Set("Content-Type", "application/json")
		wr.WriteHeader(http.StatusServiceUnavailable)
		_, _ = wr.Write([]byte(`{"code":"MODEL_UNAVAILABLE"}`))
		return
	}
	time.Sleep(w.cfg.ServiceTime)
	w.bumpProcessed()
	// Always return the canonical SHOW_CHOICES action shape so the
	// synthetic middle output is valid against the frozen schema.
	wr.Header().Set("Content-Type", "application/json")
	wr.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(wr).Encode(map[string]any{
		"action":       "SHOW_CHOICES",
		"data_version": 1,
	})
}

func (w *DummyWorker) handleSynthesize(wr http.ResponseWriter, r *http.Request) {
	if !w.tryAdmit() {
		wr.Header().Set("Content-Type", "application/json")
		wr.WriteHeader(http.StatusServiceUnavailable)
		_, _ = wr.Write([]byte(`{"code":"QUEUE_SATURATED"}`))
		return
	}
	defer w.release()
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 4096))
	if internal.RandFloat() < w.cfg.ErrorRate {
		wr.Header().Set("Content-Type", "application/json")
		wr.WriteHeader(http.StatusServiceUnavailable)
		_, _ = wr.Write([]byte(`{"code":"MODEL_UNAVAILABLE"}`))
		return
	}
	time.Sleep(w.cfg.ServiceTime)
	w.bumpProcessed()
	// Synthesize a fake audio envelope: a 1-second silent WAV header is
	// NOT generated — the orchestrator is supposed to track
	// byte_size+checksum but the harness only checks response codes and
	// content-type; the audio bytes are not exercised on this path.
	body := make([]byte, 1024)
	h := sha256.Sum256([]byte("synthetic-audio"))
	wr.Header().Set("Content-Type", "audio/wav")
	wr.Header().Set("X-Checksum-SHA256", hex.EncodeToString(h[:]))
	wr.Header().Set("X-Audio-ID", "AUD-"+hex.EncodeToString(h[:4]))
	wr.Header().Set("X-Model-Revision", "synthetic-1")
	wr.WriteHeader(http.StatusOK)
	_, _ = wr.Write(body)
}

// String returns a one-line summary of the worker.
func (w *DummyWorker) String() string {
	return fmt.Sprintf("dummy-%s@%s", w.cfg.Kind, w.Addr())
}

// URL returns the worker's base URL.
func (w *DummyWorker) URL() string { return "http://" + w.Addr() }
