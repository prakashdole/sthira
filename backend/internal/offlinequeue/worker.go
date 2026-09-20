package offlinequeue

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ReplayConfig bounds the worker's retry behavior. The defaults are sized
// for the P5 reference harness: a real mobile client reads these from
// operator-owned policy and may override per-deployment.
type ReplayConfig struct {
	// MaxAttempts caps how many times a single operation will be dispatched
	// before it is marked FAILED_PERM. The reference default of 5 matches
	// the planned exponential backoff over the constrained network profile.
	MaxAttempts int
	// BaseDelay is the first retry sleep; it doubles each attempt until
	// MaxDelay is reached.
	BaseDelay time.Duration
	// MaxDelay caps the per-retry sleep so a stuck endpoint cannot keep the
	// worker waiting indefinitely.
	MaxDelay time.Duration
	// PerCallTimeout bounds a single dispatch. A timeout is treated like a
	// network error: the worker increments RetryCount and re-queues.
	PerCallTimeout time.Duration
}

func (c ReplayConfig) withDefaults() ReplayConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 5
	}
	if c.BaseDelay <= 0 {
		c.BaseDelay = 250 * time.Millisecond
	}
	if c.MaxDelay <= 0 {
		c.MaxDelay = 8 * time.Second
	}
	if c.PerCallTimeout <= 0 {
		c.PerCallTimeout = 10 * time.Second
	}
	return c
}

// ReplayWorker drains the queue by replaying each PENDING operation through
// the dispatcher. It is single-threaded by design: the harness does not need
// parallelism, and parallel submission complicates the ordering invariants
// the contract documents (one operation at a time, deterministic state
// transitions). A real mobile client may add a worker pool; that is
// explicitly out of scope for the reference harness.
//
// State machine per operation (driven by Worker.Submit, used by DrainQueue):
//
//	PENDING -> IN_FLIGHT
//	IN_FLIGHT ->
//	  COMMITTED           on 2xx (incl. idempotent 2xx replay)
//	  FAILED_STALE        on STALE_VERSION/EXPIRED/4xx-selection codes
//	  FAILED_PERM         on IDEMPOTENCY_CONFLICT/ROUTE_UNVERIFIED/
//	                      CAPACITY_CONFLICT/FORBIDDEN/NOT_FOUND/other 4xx
//	  PENDING (retry)     on 5xx, network/timeout (within MaxAttempts)
//	  FAILED_PERM         on retry budget exhausted
//
// The worker NEVER mutates the operation's idempotency key, payload,
// endpoint, method, token ref, snapshot version or selection expiry.
type ReplayWorker struct {
	store      *Store
	dispatcher HTTPDispatcher
	tokens     TokenProvider
	cfg        ReplayConfig
	now        func() time.Time
	sleep      func(context.Context, time.Duration) error
}

// NewReplayWorker constructs a worker with the given dependencies. The store
// must be non-nil; the dispatcher, tokens and now function are mandatory
// because the worker is useless without them. The constructor does not touch
// disk; call DrainQueue (or ResetStuckInFlight) to begin.
func NewReplayWorker(store *Store, dispatcher HTTPDispatcher, tokens TokenProvider, cfg ReplayConfig, now func() time.Time) (*ReplayWorker, error) {
	if store == nil {
		return nil, errors.New("offlinequeue: worker requires a store")
	}
	if dispatcher == nil {
		return nil, errors.New("offlinequeue: worker requires a dispatcher")
	}
	if tokens == nil {
		return nil, errors.New("offlinequeue: worker requires a TokenProvider (tokens must NEVER live in queue entries)")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ReplayWorker{
		store:      store,
		dispatcher: dispatcher,
		tokens:     tokens,
		cfg:        cfg.withDefaults(),
		now:        now,
		sleep:      defaultSleep,
	}, nil
}

// SetSleep replaces the worker's sleep function. Tests inject a no-op sleep
// to avoid real-time waits; production uses defaultSleep (context-aware).
func (w *ReplayWorker) SetSleep(sleep func(context.Context, time.Duration) error) {
	if sleep != nil {
		w.sleep = sleep
	}
}

// DrainReport summarizes a single DrainQueue invocation. The numbers are
// stable, monotone counters; "processed" is the total (committed + stale +
// failed + skipped), "committed" is the server-acknowledged count.
type DrainReport struct {
	Processed int
	Committed int
	Stale     int
	Failed    int
	Skipped   int // expired at read time (worker chose FAILED_STALE without a call)
}

// DrainQueue processes every PENDING entry in arrival order until either the
// queue is empty or ctx is cancelled. It does NOT loop forever: after the
// last entry it returns. The caller decides when to invoke DrainQueue again
// (e.g. on a timer or after a network-up signal).
func (w *ReplayWorker) DrainQueue(ctx context.Context) (*DrainReport, error) {
	pending, err := w.store.PeekPending(ctx)
	if err != nil {
		return nil, err
	}
	rep := &DrainReport{}
	for _, op := range pending {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		outcome, err := w.Submit(ctx, op)
		if err != nil {
			// A worker-level failure (e.g. token provider error) is not the
			// operation's fault. Leave the entry PENDING for the next drain.
			return rep, err
		}
		rep.Processed++
		switch outcome {
		case outcomeCommitted:
			rep.Committed++
		case outcomeStale:
			rep.Stale++
		case outcomePerm:
			rep.Failed++
		case outcomeSkipped:
			rep.Skipped++
		}
	}
	return rep, nil
}

// Submit replays one operation exactly once. It is the unit of work used by
// DrainQueue and by the integration tests. The function is safe to call
// concurrently for different operation ids, but not for the same id (the
// underlying store serializes state writes).
//
// Failure categories:
//   - outcomeCommitted: the server returned 2xx (fresh commit or replay).
//   - outcomeStale: the server returned a snapshot-drift/hold-expired code,
//     the local selection window is past, OR the queued payload was
//     rewritten by a stricter client (defensive).
//   - outcomePerm: the server returned a permanent denial (4xx codes other
//     than stale/expired) or the retry budget is exhausted.
//   - outcomeSkipped: the queue refused the operation before dispatch
//     (terminal state observed, missing token ref, etc.).
type SubmitOutcome int

const (
	outcomeCommitted SubmitOutcome = iota
	outcomeStale
	outcomePerm
	outcomeSkipped
)

func (o SubmitOutcome) String() string {
	switch o {
	case outcomeCommitted:
		return "COMMITTED"
	case outcomeStale:
		return "FAILED_STALE"
	case outcomePerm:
		return "FAILED_PERM"
	default:
		return "SKIPPED"
	}
}

// Submit performs the dispatch loop for one operation. It honors ctx
// cancellation between attempts and inside a single call (via the
// dispatcher's PerCallTimeout).
func (w *ReplayWorker) Submit(ctx context.Context, op PendingOperation) (SubmitOutcome, error) {
	// Re-read the on-disk entry: the caller may have stale state.
	current, err := w.store.Get(ctx, op.ID)
	if err != nil {
		if errors.Is(err, ErrNoSuchOperation) {
			return outcomeSkipped, nil
		}
		return outcomeSkipped, err
	}
	if current.IsTerminal() {
		return outcomeSkipped, nil
	}
	// Selection window: an offline-too-long entry is FAILED_STALE without
	// contacting the server. The user must confirm a new selection.
	if !current.SelectionStillValid(w.now()) {
		_ = current.SetFailedStale(w.now(), 0, ErrSelectionExpired.Error())
		if err := w.store.UpdateState(ctx, current); err != nil {
			return outcomePerm, err
		}
		return outcomeStale, nil
	}

	token, err := w.tokens.BearerTokenFor(ctx, current.TokenRef)
	if err != nil {
		return outcomeSkipped, fmt.Errorf("offlinequeue: token lookup for %q: %w", current.TokenRef, err)
	}

	for attempt := 0; attempt < w.cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return outcomePerm, err
		}

		// Mark IN_FLIGHT before dispatch so a worker crash leaves a recoverable
		// marker (ResetStuckInFlight). The marker is only written on the first
		// attempt of a Submit; subsequent retries within the same Submit do NOT
		// re-mark IN_FLIGHT (the on-disk state stays IN_FLIGHT until the loop
		// either reaches a terminal outcome or rewrites it via BumpRetry). This
		// avoids the SetInFlight("cannot move to IN_FLIGHT from IN_FLIGHT")
		// self-conflict when current is reloaded from disk between attempts.
		if attempt == 0 {
			if err := current.SetInFlight(); err != nil {
				return outcomeSkipped, err
			}
			if err := w.store.UpdateState(ctx, current); err != nil {
				return outcomePerm, err
			}
		}

		callCtx, cancel := context.WithTimeout(ctx, w.cfg.PerCallTimeout)
		status, body, callErr := w.dispatcher.PostJSON(callCtx, current.Method, current.Endpoint, token, current.Payload)
		cancel()

		switch {
		case callErr != nil:
			current.BumpRetry(0, transportErrorMessage(callErr))
			if err := w.store.UpdateState(ctx, current); err != nil {
				return outcomePerm, err
			}
			if attempt+1 == w.cfg.MaxAttempts {
				_ = current.SetFailedPerm(w.now(), 0, "max attempts reached on transient error")
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomePerm, nil
			}
			if err := w.sleep(ctx, w.backoff(attempt)); err != nil {
				return outcomePerm, err
			}
			continue
		case status >= 200 && status < 300:
			_ = current.SetCommitted(w.now(), status, body)
			if err := w.store.UpdateState(ctx, current); err != nil {
				return outcomePerm, err
			}
			return outcomeCommitted, nil
		default:
			code := extractServerErrorCode(body)
			if isStaleCode(code) || isCapacityExpiredCode(code) {
				_ = current.SetFailedStale(w.now(), status, serverErrorMessage(status, code, body))
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomeStale, nil
			}
			if isPermanentCode(code) || (status >= 400 && status < 500) {
				_ = current.SetFailedPerm(w.now(), status, serverErrorMessage(status, code, body))
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomePerm, nil
			}
			// 5xx and unknown codes: retry within budget.
			current.BumpRetry(status, serverErrorMessage(status, code, body))
			if err := w.store.UpdateState(ctx, current); err != nil {
				return outcomePerm, err
			}
			if attempt+1 == w.cfg.MaxAttempts {
				_ = current.SetFailedPerm(w.now(), status, "max attempts reached")
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomePerm, nil
			}
			if err := w.sleep(ctx, w.backoff(attempt)); err != nil {
				return outcomePerm, err
			}
			continue
		}
	}
	// Unreachable: loop returns or falls through to MaxAttempts branch above.
	return outcomePerm, nil
}

// backoff returns BaseDelay * 2^attempt, capped at MaxDelay. attempt is the
// 0-based index of the just-finished attempt.
func (w *ReplayWorker) backoff(attempt int) time.Duration {
	d := w.cfg.BaseDelay
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= w.cfg.MaxDelay {
			return w.cfg.MaxDelay
		}
	}
	if d > w.cfg.MaxDelay {
		return w.cfg.MaxDelay
	}
	return d
}

// isStaleCode classifies the response codes the worker treats as stale
// (snapshot drift or selection expiry). The names come from the /api/v3
// envelope code set in backend/internal/contracts/errors.go and the
// reservation handler mapping.
func isStaleCode(code string) bool {
	switch code {
	case "STALE_VERSION", "VALIDATION_FAILED", "EXPIRED":
		return true
	default:
		return false
	}
}

// isCapacityExpiredCode treats an out-of-policy denial (e.g. stay dates
// outside the authoritative temporary-stay bounds) as a STALE outcome: the
// user's offline selection is no longer valid against current policy.
func isCapacityExpiredCode(code string) bool {
	return code == "VALIDATION_FAILED" || code == "STALE_VERSION" || code == "EXPIRED"
}

// isPermanentCode classifies codes that mean the request itself is wrong and
// retrying with the same key/payload will keep failing. Capacity conflicts
// are NOT permanent here: the user can re-confirm a different facility with a
// new key after seeing the conflict. IDEMPOTENCY_CONFLICT is permanent with
// the current key/payload; the worker marks FAILED_PERM and the user
// investigates.
func isPermanentCode(code string) bool {
	switch code {
	case "IDEMPOTENCY_CONFLICT",
		"FORBIDDEN", "NOT_FOUND",
		"ROUTE_UNVERIFIED", "ROUTE_UNAVAILABLE",
		"AMBIGUOUS_PLACE", "DATA_UNAVAILABLE",
		"MODEL_UNAVAILABLE", "LANGUAGE_UNSUPPORTED",
		"RATE_LIMITED", "MALFORMED_JSON",
		"DUPLICATE_KEY", "TRAILING_DATA",
		"BODY_TOO_LARGE", "DEPTH_EXCEEDED",
		"UNKNOWN_FIELD", "INVALID_VALUE",
		"METHOD_NOT_ALLOWED", "UNSUPPORTED_MEDIA_TYPE":
		return true
	default:
		return false
	}
}

// serverErrorMessage and transportErrorMessage build redacted diagnostic
// strings. They NEVER include the bearer token, payload bytes or full body
// verbatim; only the status code and stable error token.
func serverErrorMessage(status int, code string, body []byte) string {
	if code != "" {
		return fmt.Sprintf("server returned %d %s", status, code)
	}
	return fmt.Sprintf("server returned %d", status)
}

func transportErrorMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "transport timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "transport canceled"
	}
	// net/http wraps these; surface only the type, not the message (URLs and
	// headers can leak through net.OpError text on some platforms).
	return "transport error"
}

// defaultSleep is the worker's default cancellable sleep. The queue uses an
// exponential backoff; honoring context cancellation here is the difference
// between a snappy shutdown and one that waits for the longest backoff.
func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// _ ensures the http package import is exercised at compile time: the worker
// is built and tested, and the dispatcher returns an http.Client-shaped
// transport. Keep this even when no symbol from http is directly referenced
// in worker.go so a future refactor does not silently drop the stdlib HTTP
// surface the dispatcher depends on.
var _ = http.MethodPost
