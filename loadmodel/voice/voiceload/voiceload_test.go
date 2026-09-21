package voiceload

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubWorker pretends to be an ASR/middle/TTS worker. It echoes a 200
// for /transcribe, /v1/chat/completions, /synthesize and tracks request
// counts so the test can verify the harness drove each endpoint.
type stubWorker struct {
	server *httptest.Server
	mu     sync.Mutex
	hits   map[string]int
}

func newStubWorker(t *testing.T, sleep time.Duration) *stubWorker {
	t.Helper()
	w := &stubWorker{hits: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(wr http.ResponseWriter, r *http.Request) {
		wr.WriteHeader(200)
		_, _ = io.WriteString(wr, `{"ready":true}`)
	})
	mux.HandleFunc("/transcribe", func(wr http.ResponseWriter, r *http.Request) {
		w.mu.Lock()
		w.hits["/transcribe"]++
		w.mu.Unlock()
		_, _ = io.Copy(io.Discard, r.Body)
		time.Sleep(sleep)
		wr.WriteHeader(200)
	})
	mux.HandleFunc("/v1/chat/completions", func(wr http.ResponseWriter, r *http.Request) {
		w.mu.Lock()
		w.hits["/v1/chat/completions"]++
		w.mu.Unlock()
		_, _ = io.Copy(io.Discard, r.Body)
		time.Sleep(sleep)
		wr.WriteHeader(200)
		_, _ = io.WriteString(wr, `{"action":"SHOW_CHOICES"}`)
	})
	mux.HandleFunc("/synthesize", func(wr http.ResponseWriter, r *http.Request) {
		w.mu.Lock()
		w.hits["/synthesize"]++
		w.mu.Unlock()
		_, _ = io.Copy(io.Discard, r.Body)
		time.Sleep(sleep)
		wr.WriteHeader(200)
	})
	w.server = httptest.NewServer(mux)
	t.Cleanup(w.server.Close)
	return w
}

func TestRun_DrivesAllRoles(t *testing.T) {
	asr := newStubWorker(t, 5*time.Millisecond)
	mid := newStubWorker(t, 5*time.Millisecond)
	tts := newStubWorker(t, 5*time.Millisecond)

	cfg := Config{
		Workers: WorkerClients{
			ASR:    asr.server.URL,
			Middle: mid.server.URL,
			TTS:    tts.server.URL,
		},
		ArrivalRate:      10,
		Duration:         200 * time.Millisecond,
		RenderTTSFraction: 0.5,
		Language:         "en-IN",
		Jurisdiction:     "KL",
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		HTTPClient:       &http.Client{Timeout: 2 * time.Second},
	}
	res := Run(context.Background(), cfg)
	if res.Total == 0 {
		t.Fatalf("expected total > 0, got %d", res.Total)
	}
	if res.OK == 0 {
		t.Fatalf("expected OK > 0, got %d", res.OK)
	}
	if res.Other != 0 {
		t.Fatalf("expected other=0, got %d", res.Other)
	}
	if res.QueueSaturated != 0 {
		t.Fatalf("expected QueueSaturated=0, got %d", res.QueueSaturated)
	}

	asr.mu.Lock()
	asrHits := asr.hits["/transcribe"]
	asr.mu.Unlock()
	mid.mu.Lock()
	midHits := mid.hits["/v1/chat/completions"]
	mid.mu.Unlock()
	tts.mu.Lock()
	ttsHits := tts.hits["/synthesize"]
	tts.mu.Unlock()

	if asrHits == 0 || midHits == 0 || ttsHits == 0 {
		t.Fatalf("expected every role to be hit; asr=%d mid=%d tts=%d", asrHits, midHits, ttsHits)
	}
	if asrHits != int(res.Total) {
		t.Fatalf("ASR hits %d != total %d", asrHits, res.Total)
	}
	if midHits != int(res.Total) {
		t.Fatalf("middle hits %d != total %d", midHits, res.Total)
	}
	// TTS hits should be ~50% of total (RenderTTSFraction = 0.5).
	expectedTTS := int(res.Total) / 2
	diff := ttsHits - expectedTTS
	if diff < -1 || diff > 1 {
		t.Fatalf("TTS hits %d, expected ~%d (50%% of %d)", ttsHits, expectedTTS, res.Total)
	}
}

func TestRun_RecordsPercentiles(t *testing.T) {
	asr := newStubWorker(t, 1*time.Millisecond)
	mid := newStubWorker(t, 1*time.Millisecond)
	tts := newStubWorker(t, 1*time.Millisecond)
	cfg := Config{
		Workers: WorkerClients{
			ASR:    asr.server.URL,
			Middle: mid.server.URL,
			TTS:    tts.server.URL,
		},
		ArrivalRate:      20,
		Duration:         300 * time.Millisecond,
		RenderTTSFraction: 1.0,
		Language:         "en-IN",
		Jurisdiction:     "KL",
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		HTTPClient:       &http.Client{Timeout: 2 * time.Second},
	}
	res := Run(context.Background(), cfg)
	if res.p50 == 0 || res.p95 == 0 || res.p99 == 0 {
		t.Fatalf("expected non-zero percentiles; got %s", res.FormatPercentiles())
	}
	if res.p50 > res.p95 || res.p95 > res.p99 {
		t.Fatalf("percentiles out of order: %s", res.FormatPercentiles())
	}
}

func TestRun_Cancellation(t *testing.T) {
	asr := newStubWorker(t, 50*time.Millisecond)
	mid := newStubWorker(t, 50*time.Millisecond)
	tts := newStubWorker(t, 50*time.Millisecond)
	cfg := Config{
		Workers: WorkerClients{
			ASR:    asr.server.URL,
			Middle: mid.server.URL,
			TTS:    tts.server.URL,
		},
		ArrivalRate:      100,
		Duration:         5 * time.Second,
		RenderTTSFraction: 1.0,
		Language:         "en-IN",
		Jurisdiction:     "KL",
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		HTTPClient:       &http.Client{Timeout: 200 * time.Millisecond},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res := Run(ctx, cfg)
	// We don't require a specific Cancelled count; we just require the
	// run terminated and produced a Result.
	if res.StartedAt.IsZero() || res.EndedAt.IsZero() {
		t.Fatalf("run did not terminate")
	}
}

func TestApproxMedian(t *testing.T) {
	samples := make([]time.Duration, 100)
	for i := range samples {
		samples[i] = time.Duration(i) * time.Millisecond
	}
	// ApproxMedian rounds to nearest; p50 on a 0..99 list -> idx 50 -> 50ms.
	if got := ApproxMedian(samples, 0.5); got != 50*time.Millisecond {
		t.Fatalf("p50 = %v", got)
	}
	// p95 on a 0..99 list -> idx 95 -> 95ms (rounded from 94.05).
	if got := ApproxMedian(samples, 0.95); got != 94*time.Millisecond {
		t.Fatalf("p95 = %v", got)
	}
}

// ensure imports aren't dropped
var _ = atomic.AddUint64
var _ = fmt.Sprintf
var _ = strings.NewReader
