package capfeed

import (
	"context"
	"fmt"
	"time"
)

// FetchState is the outcome of one conditional fetch cycle.
type FetchState string

const (
	// FetchUpdated means new bytes were retrieved (HTTP 200).
	FetchUpdated FetchState = "UPDATED"
	// FetchNotModified means the source confirmed the cached copy (HTTP 304).
	FetchNotModified FetchState = "CACHED_NOT_MODIFIED"
	// FetchStale means the fetch failed but a prior cached copy is preserved.
	FetchStale FetchState = "STALE_CACHE"
	// FetchUnavailable means the fetch failed and no cached copy exists.
	FetchUnavailable FetchState = "UNAVAILABLE"
)

// Response is one transport reply. Status uses HTTP semantics: 200 carries
// Body, 304 carries none. ETag is the validator for the next conditional
// request.
type Response struct {
	Status int
	Body   []byte
	ETag   string
}

// Fetcher performs one conditional GET. It is injected so tests use a fixture
// transport and production uses a bounded HTTP client (P3+). The etag argument
// is the previously stored validator; empty means no conditional. The ctx
// carries the per-attempt timeout.
type Fetcher func(ctx context.Context, uri, etag string) (Response, error)

// Transport performs conditional CAP retrieval with bounded retries and
// stale-cache preservation. It never fabricates bytes: a failed fetch returns
// the preserved cache or UNAVAILABLE.
type Transport struct {
	fetch Fetcher
	clock Clock
	// MaxAttempts bounds total attempts per Refresh (>=1).
	MaxAttempts int
	// BaseDelay is the first backoff delay; it doubles each retry.
	BaseDelay time.Duration
	// AttemptTimeout bounds a single fetch attempt.
	AttemptTimeout time.Duration

	// cached holds the last successfully retrieved bytes and validator.
	cachedBody []byte
	cachedETag string
	haveCache  bool
}

// NewTransport returns a Transport. fetch and clock are required.
func NewTransport(fetch Fetcher, clock Clock) *Transport {
	if fetch == nil {
		panic("capfeed: fetcher is required")
	}
	if clock == nil {
		panic("capfeed: injected clock is required")
	}
	return &Transport{
		fetch:          fetch,
		clock:          clock,
		MaxAttempts:    3,
		BaseDelay:      50 * time.Millisecond,
		AttemptTimeout: 5 * time.Second,
	}
}

// RefreshResult reports the cycle outcome and any fresh bytes.
type RefreshResult struct {
	State FetchState
	// Body is set only when State is FetchUpdated.
	Body []byte
	// ETag is the validator to store for the next cycle.
	ETag string
	// RetrievedAt is the injected-clock instant of the successful update.
	RetrievedAt time.Time
	// Attempts is how many fetch calls were made.
	Attempts int
}

// Refresh performs one conditional retrieval cycle with bounded retries. On
// 200 it updates the cache and returns the new bytes. On 304 it returns the
// preserved cache state. On transport failure it retries with exponential
// backoff, then falls back to the preserved cache (STALE_CACHE) or reports
// UNAVAILABLE when no cache exists.
func (t *Transport) Refresh(ctx context.Context, uri string) (RefreshResult, error) {
	if uri == "" {
		return RefreshResult{}, fail("source URI is required")
	}
	attempts := t.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if t.AttemptTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, t.AttemptTimeout)
		}
		resp, err := t.fetch(attemptCtx, uri, t.cachedETag)
		cancel()

		if err == nil {
			switch resp.Status {
			case 200:
				if len(resp.Body) == 0 {
					lastErr = fail("empty 200 body")
					break
				}
				t.cachedBody = append([]byte(nil), resp.Body...)
				t.cachedETag = resp.ETag
				t.haveCache = true
				return RefreshResult{
					State:       FetchUpdated,
					Body:        append([]byte(nil), resp.Body...),
					ETag:        resp.ETag,
					RetrievedAt: t.clock().UTC(),
					Attempts:    attempt,
				}, nil
			case 304:
				if !t.haveCache {
					// A 304 with no cache is a source/protocol error; treat as failure.
					lastErr = fail("304 received with no cached copy")
					break
				}
				return RefreshResult{State: FetchNotModified, ETag: t.cachedETag, Attempts: attempt}, nil
			default:
				lastErr = fail("unexpected source status %d", resp.Status)
			}
		} else {
			lastErr = err
		}

		// Backoff before the next attempt (skip after the final attempt).
		if attempt < attempts {
			delay := t.BaseDelay << (attempt - 1)
			select {
			case <-ctx.Done():
				return t.fallback(uri, attempt, ctx.Err())
			case <-time.After(delay):
			}
		}
	}
	return t.fallback(uri, attempts, lastErr)
}

// fallback preserves the stale cache when present, else reports UNAVAILABLE.
func (t *Transport) fallback(uri string, attempts int, cause error) (RefreshResult, error) {
	if t.haveCache {
		return RefreshResult{State: FetchStale, ETag: t.cachedETag, Attempts: attempts}, nil
	}
	if cause == nil {
		cause = fmt.Errorf("no attempts made")
	}
	return RefreshResult{State: FetchUnavailable, Attempts: attempts}, fail("CAP source %q unavailable: %v", uri, cause)
}
