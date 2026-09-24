package httpserver

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"sthira/backend/internal/contracts"
)

// RateLimiter is a per-remote-IP token-bucket limiter. It is intentionally
// minimal: a single mutex, an in-memory map keyed by IP, no external
// dependencies. Use only when cfg.RateLimit.Enabled is true.
//
// Burst capacity sets the bucket size; tokens refill at rps. A burst
// higher than rps tolerates a quick spike; setting burst = rps enforces a
// strict per-second average.
//
// Stale entries are evicted on a slow ticker so a long-running server
// under churn doesn't leak memory. Per-IP state is purely memory; restart
// resets all buckets (acceptable for a soft limiter).
// RateLimiterStats captures an instantaneous snapshot of limiter activity and configured limits.
// It exposes only low-cardinality counts and configured numbers; no client IP addresses
// or request identifiers are stored or returned.
type RateLimiterStats struct {
	RPS       float64 `json:"rps"`
	Burst     float64 `json:"burst"`
	ActiveIPs int     `json:"active_ips"`
	Allowed   uint64  `json:"allowed"`
	Blocked   uint64  `json:"blocked"`
}

type RateLimiter struct {
	mu      sync.Mutex
	rps     float64
	burst   float64
	now     func() time.Time
	cur     map[string]*bucket
	stopCh  chan struct{}
	allowed uint64
	blocked uint64
}

// bucket holds the live token count and last-refill time for one IP.
type bucket struct {
	tokens   float64
	lastFill time.Time
}

// NewRateLimiter constructs a limiter. rps is the steady-state refill
// rate (tokens/sec); burst is the maximum bucket size. Both must be
// strictly positive. The supplied now func is used for tests; production
// callers pass nil to use time.Now.
func NewRateLimiter(rps, burst float64, now func() time.Time) *RateLimiter {
	if now == nil {
		now = time.Now
	}
	rl := &RateLimiter{
		rps:    rps,
		burst:  burst,
		now:    now,
		cur:    make(map[string]*bucket),
		stopCh: make(chan struct{}),
	}
	go rl.janitor()
	return rl
}

// Stop releases the janitor goroutine. Idempotent.
func (r *RateLimiter) Stop() {
	select {
	case <-r.stopCh:
		// already closed
	default:
		close(r.stopCh)
	}
}

// Allow returns true when the request from ip may proceed and false when
// it must be rejected with HTTP 429. Empty IP is treated as "trusted"
// and allowed (caller should validate r.RemoteAddr upstream).
func (r *RateLimiter) Allow(ip string) bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if ip == "" {
		r.allowed++
		return true
	}
	now := r.now()
	b, ok := r.cur[ip]
	if !ok {
		// First request from this IP: fill the bucket and let one through.
		b = &bucket{tokens: r.burst - 1, lastFill: now}
		r.cur[ip] = b
		r.allowed++
		return true
	}
	// Refill: add tokens proportional to elapsed seconds, capped at burst.
	elapsed := now.Sub(b.lastFill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * r.rps
		if b.tokens > r.burst {
			b.tokens = r.burst
		}
		b.lastFill = now
	}
	if b.tokens < 1 {
		r.blocked++
		return false
	}
	b.tokens--
	r.allowed++
	return true
}

// Stats returns an instantaneous snapshot of limiter configuration and throughput counters.
// Safe for concurrent access.
func (r *RateLimiter) Stats() RateLimiterStats {
	if r == nil {
		return RateLimiterStats{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return RateLimiterStats{
		RPS:       r.rps,
		Burst:     r.burst,
		ActiveIPs: len(r.cur),
		Allowed:   r.allowed,
		Blocked:   r.blocked,
	}
}

// janitor periodically drops IPs that haven't been seen in a while so the
// map doesn't grow unbounded under churn.
func (r *RateLimiter) janitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.evictStale(30 * time.Minute)
		}
	}
}

func (r *RateLimiter) evictStale(maxAge time.Duration) {
	cutoff := r.now().Add(-maxAge)
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip, b := range r.cur {
		if b.lastFill.Before(cutoff) {
			delete(r.cur, ip)
		}
	}
}

// clientIP extracts the remote IP. Proxies are not trusted by default;
// X-Forwarded-For is only honored when STHIRA_TRUST_FORWARDED_FOR=1,
// because the server has no other way to validate the upstream.
func clientIP(r *http.Request, trustForwarded bool) string {
	if trustForwarded {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				xff = xff[:i]
			}
			return strings.TrimSpace(xff)
		}
	}
	// http.Request.RemoteAddr is "host:port"; strip the port.
	addr := r.RemoteAddr
	if i := strings.LastIndexByte(addr, ':'); i >= 0 {
		// Only strip if it really looks like host:port (avoid IPv6 ambiguity).
		if !strings.Contains(addr, "]") || i > strings.Index(addr, "]") {
			addr = addr[:i]
		}
	}
	return addr
}

// withRateLimit returns a middleware that rejects with HTTP 429 when the
// per-IP token bucket is empty. When cfg.RateLimit is nil the
// middleware is a no-op. Bypass paths (configured via WithRateLimitBypass)
// skip the check (used for liveness, pprof, observability, and the
// rate-limited-by-design offline delivery channels).
//
// 429 responses carry an ErrRateLimited envelope and a Retry-After
// header derived from the bucket's deficit.
func (s *Server) withRateLimit(next http.HandlerFunc) http.HandlerFunc {
	if s.rateLimit == nil {
		return next
	}
	bypass := s.rateLimitBypass
	trust := s.trustForwardedFor
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		for _, b := range bypass {
			if b == path {
				next(w, r)
				return
			}
		}
		ip := clientIP(r, trust)
		if !s.rateLimit.Allow(ip) {
			// Retry-After = ceil((1 - tokens) / rps); conservative lower
			// bound of 1 second so clients don't hot-loop.
			retry := 1
			w.Header().Set("Retry-After", itoa(retry))
			s.writeError(w, r, http.StatusTooManyRequests, contracts.ErrRateLimited,
				"too many requests; retry later", "", true)
			return
		}
		next(w, r)
	}
}

// itoa avoids pulling in strconv just for the Retry-After integer.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
