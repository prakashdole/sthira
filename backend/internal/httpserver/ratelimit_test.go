package httpserver

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
)

func TestRateLimiter_Allow_FirstRequestFromIP(t *testing.T) {
	rl := NewRateLimiter(10, 10, nil)
	defer rl.Stop()
	if !rl.Allow("1.2.3.4") {
		t.Fatalf("first request from a fresh IP must be allowed")
	}
}

func TestRateLimiter_Allow_BurstThenBlock(t *testing.T) {
	rl := NewRateLimiter(10, 3, nil)
	defer rl.Stop()
	// 3 allowed, 4th rejected.
	for i := 0; i < 3; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("burst slot %d must be allowed", i)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatalf("4th request from a depleted bucket must be blocked")
	}
}

func TestRateLimiter_Allow_RefillsOverTime(t *testing.T) {
	now := time.Unix(1700000000, 0)
	rl := NewRateLimiter(10, 1, func() time.Time { return now })
	defer rl.Stop()
	if !rl.Allow("1.2.3.4") {
		t.Fatalf("first request must pass")
	}
	if rl.Allow("1.2.3.4") {
		t.Fatalf("immediate second request must be blocked (burst=1, no time elapsed)")
	}
	now = now.Add(200 * time.Millisecond) // 200ms × 10 rps = 2 tokens
	if !rl.Allow("1.2.3.4") {
		t.Fatalf("after refill window the request must be allowed")
	}
}

func TestRateLimiter_Allow_PerIPIsolation(t *testing.T) {
	rl := NewRateLimiter(10, 1, nil)
	defer rl.Stop()
	if !rl.Allow("a") {
		t.Fatalf("a: first must pass")
	}
	if rl.Allow("a") {
		t.Fatalf("a: second must block")
	}
	if !rl.Allow("b") {
		t.Fatalf("b: must pass independently of a's bucket")
	}
}

func TestRateLimiter_Allow_EmptyIPIsTrusted(t *testing.T) {
	rl := NewRateLimiter(10, 1, nil)
	defer rl.Stop()
	for i := 0; i < 100; i++ {
		if !rl.Allow("") {
			t.Fatalf("empty IP must be treated as trusted and allowed")
		}
	}
}

func TestRateLimiter_Stop_Idempotent(t *testing.T) {
	rl := NewRateLimiter(1, 1, nil)
	rl.Stop()
	rl.Stop() // must not panic
}

func TestRateLimiter_EvictStale(t *testing.T) {
	now := time.Unix(1700000000, 0)
	rl := NewRateLimiter(10, 10, func() time.Time { return now })
	defer rl.Stop()
	rl.Allow("old")
	now = now.Add(45 * time.Minute)
	rl.Allow("new")
	rl.evictStale(30 * time.Minute)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if _, ok := rl.cur["old"]; ok {
		t.Fatalf("stale IP must have been evicted")
	}
	if _, ok := rl.cur["new"]; !ok {
		t.Fatalf("fresh IP must be retained")
	}
}

func TestRateLimiter_Allow_ConcurrentSameIP(t *testing.T) {
	rl := NewRateLimiter(1000, 100, nil)
	defer rl.Stop()
	var allowed, blocked int64
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow("c") {
				atomic.AddInt64(&allowed, 1)
			} else {
				atomic.AddInt64(&blocked, 1)
			}
		}()
	}
	wg.Wait()
	if allowed == 0 || allowed > 100 {
		t.Fatalf("allowed=%d must be in (0, 100]", allowed)
	}
	if allowed+blocked != 200 {
		t.Fatalf("expected exactly 200 outcomes, got allowed=%d blocked=%d", allowed, blocked)
	}
}

func TestRateLimit_HTTP_NoLimiter_NoEffect(t *testing.T) {
	// When the limiter is nil, the middleware is a transparent passthrough.
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	for i := 0; i < 50; i++ {
		resp, err := http.Get(ts.URL + "/health/live")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	}
}

func TestRateLimit_HTTP_Blocks429(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg, WithRateLimit(1, 2))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	// First 2 succeed; 3rd is 429.
	for i := 0; i < 2; i++ {
		resp, err := http.Get(ts.URL + "/health/live")
		if err != nil {
			t.Fatalf("burst %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("burst %d expected 200, got %d", i, resp.StatusCode)
		}
	}
	resp, err := http.Get(ts.URL + "/health/live")
	if err != nil {
		t.Fatalf("over-limit GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 over-limit, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatalf("429 response must include a Retry-After header")
	}
}

func TestRateLimit_HTTP_BypassPathsSkipCheck(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg,
		WithRateLimit(1, 1),
		WithRateLimitBypass("/health/live"),
	)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	// /health/live is in bypass → 10 sequential requests all pass.
	for i := 0; i < 10; i++ {
		resp, err := http.Get(ts.URL + "/health/live")
		if err != nil {
			t.Fatalf("burst %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("bypass path burst %d expected 200, got %d", i, resp.StatusCode)
		}
	}
}

func TestRateLimit_HTTP_429EnvelopeIsStable(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg, WithRateLimit(1, 1))
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	_, _ = http.Get(ts.URL + "/health/live") // consume burst
	resp, err := http.Get(ts.URL + "/health/live")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json envelope, got %q", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options: nosniff, got %q", got)
	}
	// Error code in body must be the stable RATE_LIMITED constant.
	body := readBody(t, resp)
	if !stringContains(body, contracts.ErrRateLimited) {
		t.Fatalf("response body must contain %q: %s", contracts.ErrRateLimited, body)
	}
}

func TestRateLimit_DisabledByDefault(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	if cfg.EnableRateLimit {
		t.Fatalf("DefaultConfig.EnableRateLimit must be false (fail-closed default)")
	}
}

func TestRateLimit_ZeroArgsIgnored(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:0")
	srv := New(cfg, WithRateLimit(0, 10))
	if srv.rateLimit != nil {
		t.Fatalf("WithRateLimit(0, 10) must NOT wire a limiter (zero rps)")
	}
	srv2 := New(cfg, WithRateLimit(10, 0))
	if srv2.rateLimit != nil {
		t.Fatalf("WithRateLimit(10, 0) must NOT wire a limiter (zero burst)")
	}
}

func TestClientIP_StripsPort(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.1:54321"
	if got := clientIP(r, false); got != "203.0.113.1" {
		t.Fatalf("expected 203.0.113.1, got %q", got)
	}
}

func TestClientIP_TrustsForwardedWhenConfigured(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:54321"
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.99")
	if got := clientIP(r, false); got != "10.0.0.1" {
		t.Fatalf("untrusted mode must use RemoteAddr, got %q", got)
	}
	if got := clientIP(r, true); got != "198.51.100.7" {
		t.Fatalf("trusted mode must use first XFF entry, got %q", got)
	}
}

func readBody(t *testing.T, r *http.Response) string {
	t.Helper()
	defer r.Body.Close()
	b := make([]byte, 4096)
	n, _ := r.Body.Read(b)
	return string(b[:n])
}

func stringContains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (stringIndex(haystack, needle) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
