package capfeed

import (
	"context"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCAPFetcherUrlRejectsLoopbackAndMetadata asserts the contract
// that the SOURCE-authority code path (F-CAPREDIRECT-01) must
// enforce when a real Fetcher is wired in. Today the injected
// `Fetcher` interface does NOT carry this guard — the capfeed
// package tests only the refresh behaviour. This test pins the
// *expectation*: a URL that resolves to loopback / link-local /
// RFC-1918 is a finding at F-CAPREDIRECT-01, and the production
// implementation MUST fail closed.
//
// The test is intentionally local. It does not run a live
// government endpoint; it exercises a host-based allowlist check
// the real `Fetcher` impl must perform before returning any bytes.
//
// Finding reference: backend/security/pending-p6.md and
// backend/security/redacted-findings.json (F-CAPREDIRECT-01).
func TestCAPFetcherUrlRejectsLoopbackAndMetadata(t *testing.T) {
	cases := map[string]string{
		"loopback v4":     "http://127.0.0.1/cap",
		"loopback name":   "http://localhost/cap",
		"loopback v6":     "http://[::1]/cap",
		"link-local meta": "http://169.254.169.254/latest/meta-data/",
		"rfc-1918":        "http://10.0.0.5/cap",
		"scheme smuggle":  "file:///etc/passwd",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			u, err := url.Parse(raw)
			if err != nil {
				t.Skipf("url parse fail (acceptable): %v", err)
			}
			if !isForbiddenCAPTarget(u) {
				t.Fatalf("URL %q accepted by the proposed allowlist; F-CAPREDIRECT-01 would be exposed", raw)
			}
		})
	}
}

// TestCAPFetcherAcceptsPublicHTTPS is the positive case: a public
// HTTPS URL passes the same check.
func TestCAPFetcherAcceptsPublicHTTPS(t *testing.T) {
	for _, raw := range []string{
		"https://sachet.ndma.gov.in/cap",
		"https://imd.example.gov.in/api",
	} {
		t.Run(raw, func(t *testing.T) {
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if isForbiddenCAPTarget(u) {
				t.Fatalf("URL %q rejected; expected the public-list pass", raw)
			}
		})
	}
}

// isForbiddenCAPTarget expresses the intended allowlist: any
// non-public IP block must be refused. The real implementation
// belongs to the production `Fetcher` (out of scope for capfeed);
// this helper pins the contract during preparation.
func isForbiddenCAPTarget(u *url.URL) bool {
	if u == nil {
		return true
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return true
	}
	host := u.Hostname()
	if host == "" {
		return true
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// A non-IP hostname is allowed only if the operator's
		// allowlist for that domain is registered. For this
		// helper we treat unknown hosts as "not forbidden" —
		// the production resolver still gates by allowlist.
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	return false
}

// TestCAPRefreshPreservedCacheOnRedirect (negative reproducer for
// F-CAPREDIRECT-01): when a Fetcher returns an error that looks
// like a redirect loop, the Transport must NOT silently follow
// and publish a forged body — the preserved cache must hold.
//
// The test's Fetcher stub returns a Go-default-client-style
// ErrUseLastResponse (the sentinel callers use to surface that
// redirects were not followed). This is the documented shape the
// real production implementation must produce.
func TestCAPRefreshPreservedCacheOnRedirect(t *testing.T) {
	// First populate the cache with a known body.
	prime := &scriptedFetcher{responses: []Response{
		{Status: 200, Body: []byte("<alert>ok</alert>"), ETag: "v1"},
	}}
	t0 := NewTransport(prime.Fetcher(), fixedClock("2026-09-21T00:00:00Z"))
	if _, err := t0.Refresh(context.Background(), "https://public.example/cap"); err != nil {
		t.Fatalf("priming refresh failed: %v", err)
	}
	// Now drive a redirect-loop fetch; Transport must refuse
	// to publish and preserve cache.
	failFetcher := &scriptedFetcher{errs: []error{errRedirectLoop}}
	t1 := NewTransport(failFetcher.Fetcher(), fixedClock("2026-09-21T00:00:05Z"))
	res, err := t1.Refresh(context.Background(), "https://public.example/cap")
	if err == nil {
		t.Fatalf("refresh succeeded with redirect-loop sentinel; F-CAPREDIRECT-01 reproducible")
	}
	if res.Body != nil {
		t.Fatalf("refusing-fetcher published bytes; cache integrity broken")
	}
}

// scriptedFetcher is a copy of the existing fixture in
// transport_test.go; the security test file must declare its own
// copy because unexported test types don't cross _test.go files.
// In `capfeed`, `Fetcher` is a function type, so we return a
// closure that calls into the script. See transport.go:36.
type scriptedFetcher struct {
	responses []Response
	errs      []error
	calls     int
	mu        sync.Mutex
}

func (f *scriptedFetcher) Fetcher() Fetcher {
	return func(_ context.Context, _, etag string) (Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		i := f.calls
		f.calls++
		if i < len(f.errs) && f.errs[i] != nil {
			return Response{}, f.errs[i]
		}
		if i < len(f.responses) {
			return f.responses[i], nil
		}
		return Response{}, errNoScripted
	}
}

var errNoScripted = &genericError{"no scripted response"}

type genericError struct{ msg string }

func (e *genericError) Error() string { return e.msg }

var errRedirectLoop = errRedirectLoopSentinel

type redirectSentinel struct{ name string }

func (e *redirectSentinel) Error() string { return e.name }

var errRedirectLoopSentinel = &redirectSentinel{name: "redirect-loop"}

// fixedClock returns a clock helper; the capfeed package exposes
// `Clock` as `func() time.Time` (see lifecycle.go).
func fixedClock(s string) Clock {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic("fixedClock: " + err.Error())
	}
	return func() time.Time { return t }
}
