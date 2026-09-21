// Benchmarks for the synthetic fixture server. These run against a single
// private target — the in-memory server — and measure its own latency,
// NOT production-server latency. They exist so the harness has a stable
// local benchmark target; running them with `go test -bench` reports the
// fixture's per-route cost.
//
// Run with:
//   cd loadmodel && go test -bench=. -benchmem -benchtime=2s ./loadfixtures/...
package loadfixtures

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// newBenchServer returns a fresh server with the given bind address and
// default cache-hit-rate=0 (every request goes to origin).
func newBenchServer(b *testing.B, addr string) *Server {
	b.Helper()
	s := New(Config{
		Addr:         addr,
		CacheHitRate: 0,
	})
	if err := s.Listen(); err != nil {
		b.Fatalf("listen: %v", err)
	}
	b.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	return s
}

func benchGet(b *testing.B, url string) {
	b.Helper()
	resp, err := http.Get(url)
	if err != nil {
		b.Fatalf("get: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func benchPost(b *testing.B, url, ct, body string) {
	b.Helper()
	resp, err := http.Post(url, ct, strings.NewReader(body))
	if err != nil {
		b.Fatalf("post: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func TestBenchServerSmoke(t *testing.T) {
	// Smoke test that the benchmark server actually serves.
	s := New(Config{Addr: "127.0.0.1:0"})
	if err := s.Listen(); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	}()
	url := "http://" + s.Addr() + "/health/live"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get live: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func BenchmarkHealthLive(b *testing.B) {
	s := newBenchServer(b, "127.0.0.1:0")
	url := "http://" + s.Addr() + "/health/live"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchGet(b, url)
	}
}

func BenchmarkCachedReadManifest(b *testing.B) {
	s := newBenchServer(b, "127.0.0.1:0")
	s.cfg.CacheHitRate = 1.0 // warm cache
	url := "http://" + s.Addr() + "/api/v3/regions/JKL/manifest"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchGet(b, url)
	}
}

func BenchmarkCachedReadManifestCold(b *testing.B) {
	s := newBenchServer(b, "127.0.0.1:0")
	s.cfg.CacheHitRate = 0.0 // cold cache
	url := "http://" + s.Addr() + "/api/v3/regions/JKL/manifest"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchGet(b, url)
	}
}

func BenchmarkGuidanceQuery(b *testing.B) {
	s := newBenchServer(b, "127.0.0.1:0")
	url := "http://" + s.Addr() + "/api/v3/guidance/query"
	body := `{"jurisdiction":"KL","place_id":"PLACE-DEMO-1","language":"en-IN"}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchPost(b, url, "application/json", body)
	}
}

func BenchmarkReservationWrite(b *testing.B) {
	s := newBenchServer(b, "127.0.0.1:0")
	url := "http://" + s.Addr() + "/api/v3/reservations"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Vary the idempotency key so we don't all replay the same
		// result (which would skip the write entirely).
		benchPost(b, url, "application/json",
			fmt.Sprintf(`{"facility_id":"FACILITY-DEMO-1","service_date":"2026-09-21","party_size":1,"duration_days":7,"idempotency_key":"k%d"}`, i))
	}
}

func BenchmarkVoiceTranscriptionsNoWait(b *testing.B) {
	s := newBenchServer(b, "127.0.0.1:0")
	s.cfg.ServiceASR = 0 // no sleep, pure orchestration cost
	url := "http://" + s.Addr() + "/api/v3/voice/transcriptions"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchPost(b, url, "audio/wav", "AAAA")
	}
}

func BenchmarkQueueSaturation(b *testing.B) {
	s := newBenchServer(b, "127.0.0.1:0")
	s.cfg.ServiceASR = 100 * time.Millisecond
	s.cfg.QueueDepth = 2
	url := "http://" + s.Addr() + "/api/v3/voice/transcriptions"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Hammer the queue; the harness expects to see 503s.
		benchPost(b, url, "audio/wav", "AAAA")
	}
}

func BenchmarkSyntheticStoreReserve(b *testing.B) {
	// Pure micro-benchmark of the in-memory reservation store. This is
	// what the orchestrator hits on every write; isolating it lets the
	// report show where the time goes.
	store := newSyntheticStore()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Reserve(fmt.Sprintf("k%d", i), time.Now().UTC())
	}
}
