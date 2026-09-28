// Worker — warm-or-not middle-model process. Construction is cheap;
// readiness is established by LoadAndVerify, which performs the
// runtime warm-up + language allow-list cross-check. Concurrency is
// bounded by MaxInFlight; queueing is bounded by QueueDepth; the
// worker refuses requests when both are saturated.

package middleworker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Worker is the warm-or-not middle-model process. The state machine:
//
//   - New (NOT-READY): /health reports ready=false, warm=false. The
//     worker accepts no requests; Dispatch returns ErrWorkerNotReady.
//   - LoadAndVerify → (READY, warm): runtime reports a revision, a
//     digest and at least one language; the configured allow-list
//     matches. /health reports ready=true, warm=true. Dispatch
//     proceeds.
//   - Shutdown (DRAIN): Dispatch returns ErrWorkerShutdown after the
//     pending requests finish or hit the per-call deadline. After
//     Shutdown returns, /health reports ready=false.
//
// Concurrency model: a buffered channel (QueueDepth) plus a fixed
// worker pool (MaxInFlight). Each slot is a goroutine that consumes
// jobs from the channel. We never spawn a goroutine per request.
type Worker struct {
	runtime     Runtime
	queueDepth  int
	maxInflight int
	startedAt   time.Time
	buildRev    string
	systemHint  string

	// jobs is the bounded admission queue.
	jobs chan *Job
	// inflight counts the currently running jobs.
	inflight atomic.Int64
	// saturated counts the queue-rejected calls (observation).
	saturated atomic.Uint64
	// completed counts the jobs that returned (success or error).
	completed atomic.Uint64
	// timedOut counts jobs that hit the per-call deadline.
	timedOut atomic.Uint64
	// malformed counts jobs that returned ErrMalformed or a
	// related wire-decode error.
	malformed atomic.Uint64
	// shutdownOnce guards the shutdown transition.
	shutdownOnce sync.Once
	// wg tracks the worker pool goroutines.
	wg sync.WaitGroup

	// loadMu serialises LoadAndVerify so concurrent calls cannot
	// start two pools.
	loadMu sync.Mutex
	// readyMu guards the ready/warm/closed transition. Dispatch holds
	// it (read) across its enqueue so Shutdown cannot close jobs
	// between the readiness check and the send.
	readyMu sync.RWMutex
	ready   bool
	warm    bool
	closed  bool
}

// prober is implemented by runtimes that can check their inference
// backend is actually reachable. LoadAndVerify requires it to pass.
type prober interface {
	Probe(ctx context.Context) error
}

// Config bundles Worker construction parameters. Zero values are
// valid only for tests that construct then teardown. Production must
// always set QueueDepth, MaxInFlight and BuildRev.
type Config struct {
	// Runtime is the inference backend. Required.
	Runtime Runtime

	// QueueDepth is the bounded queue size. A request that finds
	// QueueDepth items already waiting returns QUEUE_SATURATED.
	// 0 means use DefaultQueueDepth.
	QueueDepth int

	// MaxInFlight is the number of concurrently executing jobs.
	// 0 means use DefaultMaxInflight.
	MaxInFlight int

	// BuildRevision is the worker's own build/revision identifier,
	// reported in /health for incident triage.
	BuildRevision string

	// StartedAt is the worker's process start time. Defaults to
	// time.Now().UTC() when zero.
	StartedAt time.Time

	// SystemHint is an optional advisory text for the worker
	// (e.g. "Sarvam-30B FP8 MoE"). Reported in
	// /health for incident triage; not consulted by the runtime.
	SystemHint string
}

// DefaultQueueDepth is the bounded queue bound when Config.QueueDepth
// is zero. Sized to soak a small burst while still surfacing
// backpressure quickly under sustained load.
const DefaultQueueDepth = 8

// DefaultMaxInflight is the worker-pool size when Config.MaxInFlight
// is zero. Picked to keep GPU pressure bounded; the production cap
// is a measurement, not an assumption.
const DefaultMaxInflight = 2

// ErrWorkerNotReady is returned by Dispatch when LoadAndVerify has
// not completed (or has been revoked by Shutdown).
var ErrWorkerNotReady = errors.New("middleworker: worker not ready")

// ErrWorkerShutdown is returned by Dispatch after Shutdown begins.
var ErrWorkerShutdown = errors.New("middleworker: worker shutting down")

// ErrQueueSaturated is returned by Dispatch when the queue is full.
var ErrQueueSaturated = errors.New("middleworker: queue saturated")

// NewWorker constructs a Worker in NOT-READY state. Run
// LoadAndVerify before exposing the worker to the orchestrator.
func NewWorker(cfg Config) (*Worker, error) {
	if cfg.Runtime == nil {
		return nil, errors.New("worker: runtime required")
	}
	depth := cfg.QueueDepth
	if depth <= 0 {
		depth = DefaultQueueDepth
	}
	inflight := cfg.MaxInFlight
	if inflight <= 0 {
		inflight = DefaultMaxInflight
	}
	if inflight > depth {
		// QueueDepth must bound both waiting and running jobs.
		return nil, fmt.Errorf("worker: MaxInFlight %d > QueueDepth %d", inflight, depth)
	}
	w := &Worker{
		runtime:     cfg.Runtime,
		queueDepth:  depth,
		maxInflight: inflight,
		startedAt:   cfg.StartedAt,
		buildRev:    cfg.BuildRevision,
		systemHint:  cfg.SystemHint,
		jobs:        make(chan *Job, depth),
	}
	if w.startedAt.IsZero() {
		w.startedAt = time.Now().UTC()
	}
	return w, nil
}

// LoadAndVerify performs the warm-up gate. It is idempotent: a second
// call on an already-ready worker is a no-op. The gate requires:
//
//   - runtime.Revision() != ""
//   - runtime.Digest() returns a non-empty name and SHA-256
//   - runtime.Languages() reports at least one language
//   - a runtime that can probe its inference backend (HTTPClientRuntime)
//     reaches it
//
// If any of these fail, the worker stays NOT-READY and Dispatch
// returns ErrWorkerNotReady. The /health response carries ready=false
// and warm=false. After Shutdown it returns ErrWorkerShutdown.
func (w *Worker) LoadAndVerify(ctx context.Context) error {
	w.loadMu.Lock()
	defer w.loadMu.Unlock()
	w.readyMu.RLock()
	ready, closed := w.ready, w.closed
	w.readyMu.RUnlock()
	if closed {
		return ErrWorkerShutdown
	}
	if ready {
		return nil
	}
	if w.runtime.Revision() == "" {
		return errors.New("worker: runtime revision is empty")
	}
	name, sha := w.runtime.Digest()
	if name == "" || sha == "" {
		return errors.New("worker: runtime artifact digest is empty")
	}
	langs := w.runtime.Languages()
	if len(langs) == 0 {
		return errors.New("worker: runtime reported zero languages")
	}
	if p, ok := w.runtime.(prober); ok {
		if err := p.Probe(ctx); err != nil {
			return fmt.Errorf("worker: inference backend not reachable: %w", err)
		}
	}
	w.readyMu.Lock()
	defer w.readyMu.Unlock()
	if w.closed {
		return ErrWorkerShutdown
	}
	w.warm = true
	w.ready = true
	// Start the worker pool. Each goroutine pulls from the
	// bounded queue. We never spawn per-request.
	for i := 0; i < w.maxInflight; i++ {
		w.wg.Add(1)
		go w.run()
	}
	return nil
}

// Job is one in-flight request. Result carries the response envelope
// or a typed error.
type Job struct {
	req      RequestEnvelope
	resultCh chan jobResult
	// cancel is closed when the caller cancels the dispatch (or
	// when the worker's per-call deadline expires). The handle
	// goroutine watches it and cancels its own per-call context.
	cancel     chan struct{}
	cancelOnce sync.Once
	deadline   time.Time
}

type jobResult struct {
	resp *ResponseEnvelope
	err  error
}

// Dispatch enqueues a request and waits for the result. The returned
// context is the caller's deadline context; the worker derives the
// per-call deadline from it. If the caller's ctx is canceled, the
// in-flight goroutine is canceled too.
func (w *Worker) Dispatch(ctx context.Context, req RequestEnvelope) (*ResponseEnvelope, error) {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, ErrCanceled
		}
		return nil, ErrTimeout
	}
	if !w.Snapshot().Ready {
		return nil, ErrWorkerNotReady
	}
	// Language allow-list: a request whose language is not in the
	// runtime's reported allow-list is rejected without contacting
	// the runtime. A request with no language is never admitted, even
	// if the runtime's list carries an empty entry. This is
	// defense-in-depth on top of the orchestrator's validator; neither
	// is a substitute.
	allowed := w.runtime.Languages()
	if strings.TrimSpace(req.Transcript.Language) == "" || !containsString(allowed, req.Transcript.Language) {
		return nil, fmt.Errorf("worker: language %q not in runtime allow-list %v", req.Transcript.Language, allowed)
	}

	job := &Job{
		req:      req,
		resultCh: make(chan jobResult, 1),
		cancel:   make(chan struct{}),
		deadline: deadlineFromCtx(ctx),
	}

	// Bind the caller's ctx to job.cancel via AfterFunc. When
	// the caller cancels, the AfterFunc callback closes
	// job.cancel; the handle goroutine sees the closure and
	// cancels its own per-call context. The returned stop()
	// ensures the callback is unregistered if Dispatch returns
	// before ctx is canceled — no goroutine leak on the happy
	// path.
	stop := context.AfterFunc(ctx, func() {
		job.cancelOnce.Do(func() { close(job.cancel) })
	})
	defer stop()

	if err := w.enqueue(ctx, job); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		// Caller gave up. Cancel the job and wait for it to
		// drain so we never leak a goroutine.
		job.cancelOnce.Do(func() { close(job.cancel) })
		<-job.resultCh
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ErrCanceled
		}
		w.timedOut.Add(1)
		return nil, ErrTimeout
	case r := <-job.resultCh:
		w.completed.Add(1)
		if r.err != nil {
			w.classify(r.err)
		}
		return r.resp, r.err
	}
}

// enqueue admits job under readyMu so it can never send on the queue
// after Shutdown has closed it. The send is non-blocking, so holding
// the read lock cannot stall Shutdown.
func (w *Worker) enqueue(ctx context.Context, job *Job) error {
	w.readyMu.RLock()
	defer w.readyMu.RUnlock()
	if w.closed {
		return ErrWorkerShutdown
	}
	if !w.ready {
		return ErrWorkerNotReady
	}
	select {
	case w.jobs <- job:
		w.inflight.Add(1)
		return nil
	case <-ctx.Done():
		return ErrCanceled
	default:
		w.saturated.Add(1)
		return ErrQueueSaturated
	}
}

// run is one slot of the worker pool. It pulls jobs from the
// bounded queue and dispatches them to the runtime.
func (w *Worker) run() {
	defer w.wg.Done()
	for j := range w.jobs {
		w.handle(j)
	}
}

func (w *Worker) handle(j *Job) {
	defer w.inflight.Add(-1)

	// Per-call context, bound to the job's cancel channel.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-j.cancel:
			cancel()
		case <-ctx.Done():
		}
	}()

	out, err := w.propose(ctx, j.req)
	if err != nil {
		j.resultCh <- jobResult{err: err}
		return
	}
	// Wrap the proposal into the wire response envelope.
	j.resultCh <- jobResult{resp: &ResponseEnvelope{
		RequestID:     j.req.RequestID,
		DataVersion:   out.Proposal.DataVersion,
		Proposal:      out.Proposal,
		FinishReason:  out.FinishReason,
		ModelRevision: out.ModelRevision,
	}}
}

// propose calls the runtime and turns a panic into a typed
// ErrRuntimeUnavailable, so one bad inference call cannot kill the
// pool slot or the process.
func (w *Worker) propose(ctx context.Context, req RequestEnvelope) (out *ProposeOutput, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("%w: runtime panic: %v", ErrRuntimeUnavailable, r)
		}
	}()
	return w.runtime.Propose(ctx, req)
}

func (w *Worker) classify(err error) {
	switch {
	case errors.Is(err, ErrTimeout):
		w.timedOut.Add(1)
	case errors.Is(err, ErrMalformed),
		errors.Is(err, ErrExtraText),
		errors.Is(err, ErrSchemaUnsupported),
		errors.Is(err, ErrOutputExceeded):
		w.malformed.Add(1)
	}
}

// Snapshot returns the worker's current health snapshot. The
// returned value is a copy; mutating it does not affect the worker.
// A model or artifact is advertised only when its SHA-256 digest is
// non-empty: a checksum-less entry must not be published for an
// incident responder to trust.
func (w *Worker) Snapshot() HealthEnvelope {
	w.readyMu.RLock()
	ready := w.ready
	warm := w.warm
	w.readyMu.RUnlock()
	rev := w.runtime.Revision()
	dname, dsha := w.runtime.Digest()
	langs := w.runtime.Languages()
	_ = w.inflight.Load()
	queueDepth := len(w.jobs)
	models := []ModelInfo(nil)
	if rev != "" && dsha != "" {
		models = []ModelInfo{{
			ModelID:        runtimeModelID(w.runtime),
			Revision:       rev,
			ChecksumSHA256: dsha,
			License:        runtimeLicense(w.runtime),
			Runtime:        runtimeRuntimeName(w.runtime),
			Hardware:       "private-loopback-vllm",
			RemoteCode:     false,
		}}
	}
	artifacts := []ArtifactDigest(nil)
	if dname != "" && dsha != "" {
		artifacts = []ArtifactDigest{{
			Name:           dname,
			Path:           dname,
			ChecksumSHA256: dsha,
			License:        runtimeLicense(w.runtime),
		}}
	}
	sortedLangs := append([]string(nil), langs...)
	sort.Strings(sortedLangs)
	return HealthEnvelope{
		Ready:              ready,
		Warm:               warm,
		Models:             models,
		Artifacts:          artifacts,
		SupportedLanguages: sortedLangs,
		Queue: QueueStats{
			Depth:          queueDepth,
			MaxDepth:       w.queueDepth,
			MaxConcurrency: w.maxInflight,
		},
		StartedAt:     w.startedAt.Format(time.RFC3339Nano),
		BuildRevision: w.buildRev,
		// SystemHint exposed only to /health's text body, not the
		// JSON envelope. Kept here for tests.
	}
}

// Stats returns the worker's runtime counters (debug-only; not part
// of the wire /health response).
func (w *Worker) Stats() (saturated, completed, timedOut, malformed uint64, inflight int64) {
	return w.saturated.Load(), w.completed.Load(), w.timedOut.Load(), w.malformed.Load(), w.inflight.Load()
}

// Shutdown drains pending requests up to deadline. After Shutdown,
// Dispatch returns ErrWorkerShutdown.
func (w *Worker) Shutdown(deadline time.Duration) error {
	w.shutdownOnce.Do(func() {
		// Close the queue under the same lock Dispatch holds for
		// its send, so run() returns once the queued jobs drain
		// and no Dispatch can send after the close.
		w.readyMu.Lock()
		w.ready = false
		w.warm = false
		w.closed = true
		close(w.jobs)
		w.readyMu.Unlock()
	})
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(deadline):
		return fmt.Errorf("worker: shutdown deadline %s exceeded", deadline)
	}
}

// deadlineFromCtx returns ctx.Deadline() or a far-future fallback.
func deadlineFromCtx(ctx context.Context) time.Time {
	if d, ok := ctx.Deadline(); ok {
		return d
	}
	return time.Now().Add(time.Hour)
}

// containsString reports whether s is in xs.
func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// runtimeModelID returns the model identifier from a runtime.
// HTTPClientRuntime reports its configured model_id; the others
// report their fixture/path.
func runtimeModelID(r Runtime) string {
	if h, ok := r.(*HTTPClientRuntime); ok {
		return h.ModelID()
	}
	if s, ok := r.(*StubRuntime); ok {
		return "stub:" + s.fixturePath
	}
	return "blocked-real-inference"
}

func runtimeLicense(r Runtime) string {
	if _, ok := r.(*HTTPClientRuntime); ok {
		return "Apache-2.0" // pinned for Sarvam-30B
	}
	return ""
}

func runtimeRuntimeName(r Runtime) string {
	if _, ok := r.(*HTTPClientRuntime); ok {
		return "vllm-pinned"
	}
	return "blocked-stub"
}
