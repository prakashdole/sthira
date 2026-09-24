// Package httpserver provides the bounded /api/v3 HTTP boundary: method and
// content-type enforcement, size/depth-limited strict JSON, cancellation via
// request context, timeouts, graceful shutdown, and separate liveness and
// dependency readiness. Missing dependencies never report operational READY.
package httpserver

import "time"

// Config holds the server bounds. All values are explicit; there are no
// hidden defaults that would weaken the boundary.
type Config struct {
	// Addr is the listen address, e.g. "127.0.0.1:8080".
	Addr string
	// ReadHeaderTimeout bounds reading request headers (slowloris guard).
	ReadHeaderTimeout time.Duration
	// ReadTimeout bounds reading the whole request including body.
	ReadTimeout time.Duration
	// WriteTimeout bounds writing the response.
	WriteTimeout time.Duration
	// IdleTimeout bounds keep-alive between requests.
	IdleTimeout time.Duration
	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
	// MaxBodyBytes bounds any JSON request body.
	MaxBodyBytes int64
	// MaxJSONDepth bounds JSON nesting depth.
	MaxJSONDepth int
	// EnablePprof activates guarded /debug/pprof/* diagnostic endpoints.
	EnablePprof bool
	// PprofToken is the required secret token for accessing /debug/pprof/*.
	PprofToken string
	// EnableAccessLog enables structured request completion logging.
	EnableAccessLog bool
	// EnableRateLimit activates the per-remote-IP token-bucket limiter. The
	// limiter is fail-closed OFF when nil; once set, the configured rps/burst
	// shape applies to every non-bypass route.
	EnableRateLimit bool
	// RateLimitRPS is the steady-state refill rate in tokens per second.
	RateLimitRPS float64
	// RateLimitBurst is the maximum bucket size (and the maximum number of
	// requests allowed in a single instant from one IP).
	RateLimitBurst float64
	// RateLimitBypass lists URL paths that skip rate-limit checks
	// (e.g. /health/live, /debug/pprof/, /api/v3/observability/metrics).
	RateLimitBypass []string
	// TrustForwardedFor controls whether X-Forwarded-For is used as the
	// client IP. Only enable behind a known reverse proxy that strips
	// client-supplied headers; otherwise this is a header-injection bypass.
	TrustForwardedFor bool
}

// DefaultConfig returns conservative boundary defaults.
func DefaultConfig(addr string) Config {
	return Config{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		MaxBodyBytes:      1 << 20, // 1 MiB
		MaxJSONDepth:      32,
	}
}
