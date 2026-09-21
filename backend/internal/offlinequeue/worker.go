package offlinequeue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
//	  COMMITTED           on a 2xx response whose body is a valid /api/v3
//	                      success envelope AND whose operation-specific
//	                      identifiers (e.g. reservation_id + stay_id) are
//	                      present in the data field. Anything else — empty,
//	                      malformed, error-envelope, or missing required
//	                      fields — is treated as PENDING_RECONCILIATION:
//	                      we cannot prove the server committed without a
//	                      trustworthy envelope.
//	  FAILED_STALE        on STALE_VERSION or EXPIRED. VALIDATION_FAILED is
//	                      NOT stale — it is a generic client-side validation
//	                      failure and routes to FAILED_PERM.
//	  FAILED_PERM         on IDEMPOTENCY_CONFLICT / ROUTE_UNVERIFIED /
//	                      CAPACITY_CONFLICT / NOT_FOUND / etc. — codes that
//	                      mean "the request itself is wrong; retrying with
//	                      the same key/payload will keep failing". User
//	                      must inspect and confirm a new operation.
//	  PENDING_RECONCILIATION on transport failure (network/timeout), on a 2xx
//	                      with an invalid envelope, on auth failure (401/403),
//	                      on rate-limit responses (429) within budget that
//	                      ALSO count toward the retry budget, and on
//	                      5xx/proxy responses whose retry budget is exhausted.
//	                      The next drain replays with the same idempotency
//	                      key; the server's idempotency store resolves the
//	                      duplication.
//	  PENDING (retry)     on 5xx (within MaxAttempts). The retry budget is
//	                      per-Submit; exhaustion moves to PENDING_RECONCILIATION.
//
// Auth failures (401/403) DO NOT mark FAILED_PERM: the server may have
// committed before the client's session expired, and the original key is the
// only safe way to ask the server whether the prior attempt succeeded.
// Telling the user to "submit a duplicate under a new key" would create a
// second reservation.
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
// stable, monotone counters.
//
//	Processed = Committed + Stale + Failed + Skipped + Reconciliation.
//	Committed = server-acknowledged count (incl. idempotent replays).
//	Stale     = snapshot drift / hold expired / offline-too-long (FAILED_STALE).
//	Failed    = permanent denial (FAILED_PERM).
//	Skipped   = terminal state observed, missing token ref, etc. — the worker
//	            refused to dispatch.
//	Reconciliation = a transport failure (or retry exhaustion) left the
//	                 operation in PENDING_RECONCILIATION; the next drain will
//	                 replay with the same idempotency key to learn the server's
//	                 verdict. Reconciliation is NOT a terminal outcome and is
//	                 counted separately so a run with zero network does not
//	                 appear to have made progress.
type DrainReport struct {
	Processed      int
	Committed      int
	Stale          int
	Failed         int
	Skipped        int
	Reconciliation int
}

// DrainQueue processes every dispatchable entry in arrival order until either
// the queue is empty or ctx is cancelled. It does NOT loop forever: after the
// last entry it returns. The caller decides when to invoke DrainQueue again
// (e.g. on a timer or after a network-up signal).
//
// "Dispatchable" is whatever PeekPending returns: PENDING (never dispatched)
// plus PENDING_RECONCILIATION (a previous Submit observed an uncertain
// transport failure). Both must be served by a real dispatch; otherwise an
// offline-stuck entry would never learn the server's verdict.
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
			// operation's fault. Leave the entry in its current state for
			// the next drain.
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
		case outcomePendingReconciliation:
			rep.Reconciliation++
		}
	}
	return rep, nil
}

// Submit replays one operation exactly once. It is the unit of work used by
// DrainQueue and by the integration tests. The function is safe to call
// concurrently for different operation ids, but not for the same id (the
// underlying store serializes state writes).
//
// Outcome categories:
//   - outcomeCommitted: the server returned 2xx (fresh commit, or replay of
//     a previously-unknown prior commit that the idempotency store resolves).
//   - outcomeStale: the server returned a snapshot-drift/hold-expired code,
//     the local selection window is past (and the entry was not uncertain),
//     OR the queued payload was rewritten by a stricter client (defensive).
//   - outcomePerm: the server returned a permanent denial (4xx codes other
//     than stale/expired). Retry-exhaustion is NOT a terminal failure here;
//     see outcomePendingReconciliation.
//   - outcomeSkipped: the queue refused the operation before dispatch
//     (terminal state observed, missing token ref, etc.).
//   - outcomePendingReconciliation: a transport/timeout error left the
//     operation's outcome uncertain. The entry is preserved with its
//     original key and payload so the next drain can replay it; it is NOT
//     marked failed. Retry exhaustion does not change this: a non-2xx
//     response with status code and error body still tells us "the server
//     definitely did not commit", but a callErr only tells us "we never
//     got an answer".
type SubmitOutcome int

const (
	outcomeCommitted SubmitOutcome = iota
	outcomeStale
	outcomePerm
	outcomeSkipped
	outcomePendingReconciliation
)

func (o SubmitOutcome) String() string {
	switch o {
	case outcomeCommitted:
		return "COMMITTED"
	case outcomeStale:
		return "FAILED_STALE"
	case outcomePerm:
		return "FAILED_PERM"
	case outcomePendingReconciliation:
		return "PENDING_RECONCILIATION"
	default:
		return "SKIPPED"
	}
}

// Submit performs the dispatch loop for one operation. It honors ctx
// cancellation between attempts and inside a single call (via the
// dispatcher's PerCallTimeout).
//
// Transport failure semantics: a callErr (network error, timeout, context
// cancellation during the call) means we cannot prove the server's verdict.
// We DO NOT exhaust the retry budget within this Submit trying to find out:
// each retry within the same Submit would either (a) hit the same outage,
// wasting wall time, or (b) eventually succeed on a server that already
// committed the previous attempt, doubling the held capacity. Instead the
// first callErr transitions the entry to PENDING_RECONCILIATION and returns
// outcomePendingReconciliation; the next drain (or the next Submit call
// after a manual reconcile) replays with the SAME idempotency key and the
// server's idempotency store resolves the duplication.
//
// Server-reachable failures (4xx, 5xx with a body) are still retried within
// the retry budget — the server has answered and the answer can be acted
// on. Only the absence of an answer produces PENDING_RECONCILIATION.
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
	//
	// EXCEPTION: PENDING_RECONCILIATION entries cannot be failed closed on
	// a local clock signal alone. The previous dispatch's outcome is unknown
	// and the server is the only party that can resolve it. Skipping the
	// expiry check here means "if the server has committed, we replay and
	// confirm; if the server rejects, we mark stale".
	if current.State != StatePendingReconciliation && !current.SelectionStillValid(w.now()) {
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
		// self-conflict when current is reloaded from disk between attempts, OR
		// when ResetStuckInFlight has already moved the entry to
		// PENDING_RECONCILIATION and we are re-marking it IN_FLIGHT on a fresh
		// dispatch.
		if attempt == 0 {
			if current.State != StateInFlight {
				if err := current.SetInFlight(); err != nil {
					return outcomeSkipped, err
				}
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
			}
		}

		callCtx, cancel := context.WithTimeout(ctx, w.cfg.PerCallTimeout)
		status, body, callErr := w.dispatcher.PostJSON(callCtx, current.Method, current.Endpoint, token, current.Payload)
		cancel()

		switch {
		case callErr != nil:
			// Transport/timeout: outcome is unknowable. Preserve uncertainty.
			// We do NOT loop within this Submit to "try again" — the queue's
			// drain cadence owns retry timing, and retrying a dropped request
			// against a server that may have already committed is exactly
			// what the idempotency key is for.
			current.BumpRetry(0, transportErrorMessage(callErr))
			if err := current.SetPendingReconciliation(w.now(), 0, transportErrorMessage(callErr)); err != nil {
				return outcomePerm, err
			}
			if err := w.store.UpdateState(ctx, current); err != nil {
				return outcomePerm, err
			}
			return outcomePendingReconciliation, nil
		case status >= 200 && status < 300:
			// A 2xx status is not, by itself, proof of commitment. Validate
			// the bounded /api/v3 success envelope and the
			// operation-specific identifiers before marking COMMITTED. Any
			// defect here means we cannot prove the server committed, and
			// we must NOT tell the user "your reservation succeeded" without
			// evidence — the cost of a false-positive here is a duplicate
			// reservation on the next replay, which is the worst outcome
			// for a citizen during an emergency.
			if ok, code, reason := validateSuccessEnvelope(current.Method, current.Endpoint, status, body); !ok {
				msg := fmt.Sprintf("2xx envelope rejected: %s", reason)
				if code != "" {
					msg = fmt.Sprintf("2xx envelope rejected: %s (code=%s)", reason, code)
				}
				current.BumpRetry(status, msg)
				if err := current.SetPendingReconciliation(w.now(), status, msg); err != nil {
					return outcomePerm, err
				}
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomePendingReconciliation, nil
			}
			_ = current.SetCommitted(w.now(), status, body)
			if err := w.store.UpdateState(ctx, current); err != nil {
				return outcomePerm, err
			}
			return outcomeCommitted, nil
		default:
			code := extractServerErrorCode(body)
			if isAuthFailure(status) {
				// 401/403 (or auth-coded 4xx): losing auth access does NOT
				// prove the operation failed. The server may have committed
				// before the session expired, and the original key is the
				// only safe way to ask. Preserve PENDING_RECONCILIATION; the
				// UI must surface "please re-authenticate" without
				// inventing a new key.
				msg := fmt.Sprintf("auth failure: %s", serverErrorMessage(status, code, body))
				current.BumpRetry(status, msg)
				if err := current.SetPendingReconciliation(w.now(), status, msg); err != nil {
					return outcomePerm, err
				}
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomePendingReconciliation, nil
			}
			if isStaleCode(code) {
				_ = current.SetFailedStale(w.now(), status, serverErrorMessage(status, code, body))
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomeStale, nil
			}
			if isPermanentCode(code) {
				_ = current.SetFailedPerm(w.now(), status, serverErrorMessage(status, code, body))
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomePerm, nil
			}
			if isTransientClientCode(status, code) {
				// 429 and other "ask again later" codes: retry within budget.
				current.BumpRetry(status, serverErrorMessage(status, code, body))
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				if attempt+1 == w.cfg.MaxAttempts {
					// Retry budget exhausted: preserve uncertainty, do NOT
					// mark FAILED_PERM. The next drain keeps trying.
					_ = current.SetPendingReconciliation(w.now(), status, "retry budget exhausted on transient client code")
					if err := w.store.UpdateState(ctx, current); err != nil {
						return outcomePerm, err
					}
					return outcomePendingReconciliation, nil
				}
				if err := w.sleep(ctx, w.backoff(attempt)); err != nil {
					return outcomePerm, err
				}
				continue
			}
			// 5xx and unknown codes: server answered but is unhealthy.
			// Retry within budget (the server is reachable, just unhappy).
			current.BumpRetry(status, serverErrorMessage(status, code, body))
			if err := w.store.UpdateState(ctx, current); err != nil {
				return outcomePerm, err
			}
			if attempt+1 == w.cfg.MaxAttempts {
				// Retry exhaustion on 5xx is NOT a definitive failure: the
				// server may have committed before going unhealthy. The
				// queue keeps the entry in PENDING_RECONCILIATION so the
				// next drain (or a manual reconcile) replays with the same
				// key and the server's idempotency store resolves the
				// duplication. We do NOT mark FAILED_PERM here.
				_ = current.SetPendingReconciliation(w.now(), status, "retry budget exhausted on 5xx")
				if err := w.store.UpdateState(ctx, current); err != nil {
					return outcomePerm, err
				}
				return outcomePendingReconciliation, nil
			}
			if err := w.sleep(ctx, w.backoff(attempt)); err != nil {
				return outcomePerm, err
			}
			continue
		}
	}
	// Unreachable: the loop above either returns or falls through to the
	// MaxAttempts branch.
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
// (snapshot drift or local selection expiry). Only these are stale-by-
// construction — VALIDATION_FAILED is a generic client-side validation
// failure and routes to FAILED_PERM, not FAILED_STALE.
func isStaleCode(code string) bool {
	switch code {
	case "STALE_VERSION", "EXPIRED":
		return true
	default:
		return false
	}
}

// isAuthFailure reports whether the response status indicates an auth-
// scoped failure (401/403). Auth failures preserve the original
// idempotency key in PENDING_RECONCILIATION so the user can re-authenticate
// and replay; the server's idempotency store resolves the duplicate. The
// worker never tells the user to submit under a new key on auth failure —
// that would create a second reservation.
func isAuthFailure(status int) bool {
	return status == 401 || status == 403
}

// isTransientClientCode reports whether the response is a "ask again later"
// code that should retry within budget. 429 is transient: rate limits
// typically clear in seconds, and the operation itself is correct.
func isTransientClientCode(status int, code string) bool {
	if status == 429 {
		return true
	}
	switch code {
	case "RATE_LIMITED":
		return true
	default:
		return false
	}
}

// isPermanentCode classifies codes that mean the request itself is wrong
// and retrying with the same key/payload will keep failing. IDEMPOTENCY_CONFLICT
// is permanent with the current key/payload; the worker marks FAILED_PERM and
// the user investigates. Capacity conflicts are permanent: the user must
// inspect and re-confirm with a new selection (new key) — auto-retrying the
// same payload against the same committed hold cannot succeed.
func isPermanentCode(code string) bool {
	switch code {
	case "IDEMPOTENCY_CONFLICT", "CAPACITY_CONFLICT",
		"NOT_FOUND",
		"ROUTE_UNVERIFIED", "ROUTE_UNAVAILABLE",
		"AMBIGUOUS_PLACE", "DATA_UNAVAILABLE",
		"MODEL_UNAVAILABLE", "LANGUAGE_UNSUPPORTED",
		"MALFORMED_JSON", "VALIDATION_FAILED",
		"DUPLICATE_KEY", "TRAILING_DATA",
		"BODY_TOO_LARGE", "DEPTH_EXCEEDED",
		"UNKNOWN_FIELD", "INVALID_VALUE",
		"METHOD_NOT_ALLOWED", "UNSUPPORTED_MEDIA_TYPE":
		return true
	default:
		return false
	}
}

// validateSuccessEnvelope decodes a 2xx response body and confirms it is a
// valid /api/v3 success envelope with operation-specific identifiers. It
// returns ok=true when the body is a real success response; ok=false (with a
// short reason) otherwise. The worker treats ok=false as PENDING_RECONCILIATION:
// we cannot prove the server committed without a trustworthy envelope, and a
// false-positive "committed" here would produce a duplicate reservation
// when the next drain replays.
//
// Validation rules:
//   - body must be valid JSON (any non-2xx-ish proxy output is rejected);
//   - body must NOT contain an errors[].code field (a 2xx that carries an
//     error envelope is not a success);
//   - body must contain a `data` field that is a non-null JSON object;
//   - POST /api/v3/reservations: data must include reservation_id AND stay_id;
//   - POST /api/v3/reservations/{id}/events: data must include stay_id AND a
//     recognized type (ARRIVE|CANCEL|DEPART|EXTEND|TRANSFER).
func validateSuccessEnvelope(method, path string, status int, body []byte) (ok bool, code, reason string) {
	if len(body) == 0 {
		return false, "", "empty body"
	}
	var env struct {
		RequestID string          `json:"request_id"`
		Data      json.RawMessage `json:"data"`
		Errors    []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, "", fmt.Sprintf("malformed JSON: %v", err)
	}
	if len(env.Errors) > 0 && env.Errors[0].Code != "" {
		return false, env.Errors[0].Code, "2xx response contains errors[].code"
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return false, "", "missing or null data field"
	}
	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return false, "", fmt.Sprintf("data not a JSON object: %v", err)
	}
	if strings.ToUpper(method) != "POST" {
		// Defensive: the queue only enqueues POST. Anything else would be a
		// config bug; reject the envelope so a misconfigured client does
		// not mark a phantom success.
		return false, "", fmt.Sprintf("unsupported method %q", method)
	}
	switch {
	case path == "/api/v3/reservations":
		rid, _ := data["reservation_id"].(string)
		sid, _ := data["stay_id"].(string)
		if rid == "" || sid == "" {
			return false, "", "missing reservation_id or stay_id"
		}
		return true, "", ""
	case strings.HasPrefix(path, "/api/v3/reservations/") && strings.HasSuffix(path, "/events"):
		sid, _ := data["stay_id"].(string)
		typ, _ := data["type"].(string)
		if sid == "" {
			return false, "", "missing stay_id"
		}
		switch strings.ToUpper(typ) {
		case "ARRIVE", "CANCEL", "DEPART", "EXTEND", "TRANSFER":
			return true, "", ""
		default:
			return false, "", fmt.Sprintf("unknown stay event type %q", typ)
		}
	default:
		// Other /api/v3 endpoints: accept the envelope if it has data; the
		// queue currently only carries reservation.create and stay.* but
		// a future endpoint (e.g. /api/v3/sessions) gets the same
		// envelope check without an endpoint-specific identifier rule.
		return true, "", ""
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
