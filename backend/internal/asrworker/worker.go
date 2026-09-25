package asrworker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Worker is the warm-or-not ASR process. Construction is cheap;
// readiness is established by LoadAndVerify, which performs the
// artifact scan + runtime warm-up. Concurrency is bounded by
// MaxInFlight; queueing is bounded by QueueDepth; the worker
// refuses requests when both are saturated.
//
// Lifecycle:
//
//   - LoadAndVerify(ctx) scans the artifact directory, drives the
//     runtime through its warm-up, and sets Ready accordingly. Idempotent.
//   - Once Ready, /health returns warm=true, ready=true. Both flags
//     are gated on:
//   - inventory.Ready() == nil
//   - runtime.Revision() != "" (the model is actually loaded)
//   - runtime.Digest() != ("", "") (a digest exists)
//   - allowed languages include the configured set.
//   - On Ready, requests are dispatched via Dispatch, which enforces
//     per-call deadline, queue/concurrency bounds, and language
//     allow-listing.
//   - Shutdown drains pending requests up to a deadline. After
//     Shutdown, Dispatch returns ErrWorkerShutdown.
//
// Concurrency model: a buffered channel (QueueDepth) plus a fixed
// worker pool (MaxInFlight). Each slot is a goroutine that consumes
// jobs from the channel. We never spawn a goroutine per request.
type Worker struct {
	inv         Inventory
	runtime     Runtime
	queueDepth  int
	maxInflight int
	startedAt   time.Time
	buildRev    string

	// jobs is the bounded admission queue.
	jobs chan *job
	// inflight counts the currently running jobs.
	inflight atomic.Int64
	// saturated counts the queue-rejected calls (observation).
	saturated atomic.Uint64
	// completed counts the jobs that returned (success or error).
	completed atomic.Uint64
	// shutdownOnce guards the shutdown transition.
	shutdownOnce sync.Once
	// wg tracks the worker pool goroutines.
	wg sync.WaitGroup

	// readyMu guards the ready/warm transition.
	readyMu sync.RWMutex
	ready   bool
	warm    bool
}

// Config bundles Worker construction parameters.
//
// Zero values are valid only for tests that construct then teardown.
// Production must always set QueueDepth, MaxInFlight and BuildRev.
type Config struct {
	// Inventory is the artifact scan result (from ScanLocalInventory).
	// Worker takes a copy of the catalog at construction so the
	// /health response is stable until the next scan.
	Inventory Inventory

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
}

// DefaultQueueDepth is the bounded queue bound when Config.QueueDepth
// is zero. Sized to soak a small burst while still surfacing
// backpressure quickly under sustained load.
const DefaultQueueDepth = 8

// DefaultMaxInflight is the worker-pool size when Config.MaxInFlight
// is zero. Picked to keep GPU / subprocess pressure bounded.
const DefaultMaxInflight = 2

// NewWorker constructs a Worker in NOT-READY state. Run LoadAndVerify
// before exposing the worker to the orchestrator.
func NewWorker(cfg Config) (*Worker, error) {
	if cfg.Runtime == nil {
		return nil, errors.New("runtime is required")
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
		return nil, fmt.Errorf("MaxInFlight (%d) must be <= QueueDepth (%d)", inflight, depth)
	}
	startedAt := cfg.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	w := &Worker{
		inv:         cfg.Inventory,
		runtime:     cfg.Runtime,
		queueDepth:  depth,
		maxInflight: inflight,
		startedAt:   startedAt,
		buildRev:    cfg.BuildRevision,
		jobs:        make(chan *job, depth),
	}
	// Spawn the worker pool once at construction. The pool waits
	// for jobs; otherwise it's parked. This way we never spawn a
	// goroutine per request and the bound is fixed.
	for i := 0; i < inflight; i++ {
		w.wg.Add(1)
		go w.runWorker()
	}
	return w, nil
}

// job is one queued ASR call. Fields are immutable once the job
// is queued; results are written exactly once.
type job struct {
	ctx       context.Context
	req       TranscribeRequest
	resultCh  chan TranscribeResult
	errCh     chan error
	cancelled atomic.Bool
}

// LoadAndVerify performs the warm-up scan. After it returns, the
// worker's Ready / Warm flags reflect the actual current state.
//
// Idempotent: safe to call repeatedly. Calling while jobs are
// queued is fine; the warm/ready transitions are guarded by
// readyMu and gated by both inventory and runtime.
func (w *Worker) LoadAndVerify(ctx context.Context) error {
	// Inventory gates: non-zero language list, no pending critical,
	// no digest mismatch. Compute from w.inv which is the
	// stable copy taken at construction.
	if err := w.inv.Ready(); err != nil {
		return fmt.Errorf("inventory not ready: %w", err)
	}
	// Runtime gates: actual loaded artifact (revision + digest),
	// non-empty language list.
	if w.runtime.Revision() == "" {
		return errors.New("runtime revision is empty (model not actually loaded)")
	}
	if name, digest := w.runtime.Digest(); name == "" || digest == "" {
		return errors.New("runtime artifact digest is empty (model not hashed)")
	}
	if len(w.runtime.SupportedLanguages()) == 0 {
		return errors.New("runtime adapter reports no allowed languages")
	}
	// Allow-list the runtime languages into the inventory view.
	w.inv.AllowedLanguages = append([]string(nil), w.runtime.SupportedLanguages()...)
	w.setReady(true, true)
	return nil
}

// Dispatch performs one bounded ASR call. Returns immediately with
// QUEUE_SATURATED if the queue is full; in that case the caller
// (HTTP handler) maps to ErrorCode = ErrQueueSaturated and to
// TranscriptionState = UNAVAILABLE.
//
// On success the result struct is typed. On error the error wraps
// ErrLanguageUnsupported / ErrRuntimeUnavailable /
// context.DeadlineExceeded as appropriate.
func (w *Worker) Dispatch(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	if w.isShutdown() {
		return TranscribeResult{}, ErrWorkerShutdown
	}
	if !w.readyFlag() {
		return TranscribeResult{}, ErrWorkerNotReady
	}
	// Language allow-list at the boundary: never spend decode time
	// on a language the runtime cannot handle.
	if !containsString(w.runtime.SupportedLanguages(), req.Language) {
		return TranscribeResult{}, fmt.Errorf("%w: %s", ErrLanguageUnsupported, req.Language)
	}
	// Apply the per-request deadline as both a context deadline and
	// a TranscribeRequest.Deadline (the runtime may use the latter
	// for scheduling).
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && !req.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, req.Deadline)
		defer cancel()
	}
	j := &job{
		ctx:      ctx,
		req:      req,
		resultCh: make(chan TranscribeResult, 1),
		errCh:    make(chan error, 1),
	}
	select {
	case <-ctx.Done():
		return TranscribeResult{}, ctx.Err()
	case w.jobs <- j:
	default:
		w.saturated.Add(1)
		return TranscribeResult{}, ErrQueueSaturated
	}
	select {
	case <-ctx.Done():
		// Mark the in-flight (if any) so the worker pool can drop
		// it without blocking the result channels. We don't try to
		// signal cancel mid-run here; we just unblock the caller.
		j.cancelled.Store(true)
		return TranscribeResult{}, ctx.Err()
	case res := <-j.resultCh:
		return res, nil
	case err := <-j.errCh:
		return TranscribeResult{}, err
	}
}

// runWorker is the goroutine body of the worker pool. It pulls
// jobs from the channel and dispatches them. Errors and results
// are routed back via the per-job channels. A goroutine that gets
// a job whose context is already done simply no-ops and discards
// the work without calling the runtime.
func (w *Worker) runWorker() {
	defer w.wg.Done()
	for j := range w.jobs {
		w.inflight.Add(1)
		w.handleJob(j)
		w.inflight.Add(-1)
		w.completed.Add(1)
	}
}

// handleJob runs one job through the runtime.
func (w *Worker) handleJob(j *job) {
	// Drop cancelled before entering the runtime: cheaper than
	// running it.
	if j.cancelled.Load() {
		j.errCh <- context.Canceled
		return
	}
	if err := j.ctx.Err(); err != nil {
		j.errCh <- err
		return
	}
	res, err := w.runtime.Transcribe(j.ctx, j.req)
	if err != nil {
		j.errCh <- err
		return
	}
	j.resultCh <- res
}

// Shutdown initiates a graceful drain. New Dispatch calls return
// ErrWorkerShutdown. The worker pool is closed after the drain
// completes, or after DrainDeadline, whichever comes first.
//
// Safe to call concurrently or after the worker is already shut
// down; both are no-ops.
func (w *Worker) Shutdown(ctx context.Context) error {
	var firstErr error
	w.shutdownOnce.Do(func() {
		close(w.jobs) // signals worker pool to exit when drained
		// We rely on the runtime's Close() to release subprocesses.
		if err := w.runtime.Close(); err != nil {
			firstErr = err
		}
		w.setReady(false, false)
	})
	// Wait for the pool to drain, bounded by the parent context.
	done := make(chan struct{})
	go func() { w.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return errors.Join(firstErr, ctx.Err())
	}
	return firstErr
}

// Health is the typed snapshot the /health endpoint returns. It is
// computed from current state; there is no locking beyond the
// readyMu read-lock and a brief atomic read for the counters.
type Health struct {
	Ready              bool
	Warm               bool
	Models             []RuntimeModel
	Artifacts          []ArtifactDigest
	SupportedLanguages []string
	Queue              QueueStats
	StartedAt          string
	BuildRevision      string
	LastInventoryScan  string
	PendingCritical    []string
}

// RuntimeModel is the runtime-reported descriptor. Matches
// contracts.ModelInfo's wire shape but is kept private to this
// module so we don't couple to the orchestrator's contracts.
type RuntimeModel struct {
	ModelID        string
	Revision       string
	ChecksumSHA256 string
	License        string
	Runtime        string
	Hardware       string
	RemoteCode     bool
}

// QueueStats mirrors contracts.QueueStats' wire shape.
type QueueStats struct {
	Depth          int
	MaxDepth       int
	MaxConcurrency int
}

// Snapshot returns a Health view of the current worker state.
func (w *Worker) Snapshot() Health {
	w.readyMu.RLock()
	ready, warm := w.ready, w.warm
	w.readyMu.RUnlock()
	pending := append([]string(nil), w.inv.PendingCritical...)
	sort.Strings(pending)
	supported := append([]string(nil), w.runtime.SupportedLanguages()...)
	sort.Strings(supported)
	models := []RuntimeModel{}
	if rev := w.runtime.Revision(); rev != "" {
		name, digest := w.runtime.Digest()
		models = append(models, RuntimeModel{
			ModelID:        "stub-asr", // ponY-tail: runtime does not yet expose an HF ID
			Revision:       rev,
			ChecksumSHA256: digest,
			License:        "LicensePending",
			Runtime:        "go-stub-1",
			Hardware:       "none",
			RemoteCode:     false,
		})
		_ = name
	}
	return Health{
		Ready:              ready,
		Warm:               warm,
		Models:             models,
		Artifacts:          w.inv.SupportedComponents(),
		SupportedLanguages: supported,
		Queue: QueueStats{
			Depth:          len(w.jobs),
			MaxDepth:       w.queueDepth,
			MaxConcurrency: w.maxInflight,
		},
		StartedAt:         w.startedAt.Format(time.RFC3339),
		BuildRevision:     w.buildRev,
		LastInventoryScan: w.inv.LastScannedAt.Format(time.RFC3339),
		PendingCritical:   pending,
	}
}

// QueueMetricsSnapshot is the per-stage counter view exposed to
// the orchestrator for low-cardinality metrics.
type QueueMetricsSnapshot struct {
	Inflight  int64
	Saturated uint64
	Completed uint64
	QueueLen  int
	QueueCap  int
}

func (w *Worker) MetricsSnapshot() QueueMetricsSnapshot {
	return QueueMetricsSnapshot{
		Inflight:  w.inflight.Load(),
		Saturated: w.saturated.Load(),
		Completed: w.completed.Load(),
		QueueLen:  len(w.jobs),
		QueueCap:  cap(w.jobs),
	}
}

// Errors exposed to the HTTP handler.
//
//   - ErrWorkerNotReady   — worker is between construction and
//     LoadAndVerify (or LoadAndVerify returned an error).
//   - ErrWorkerShutdown   — Shutdown was called.
//   - ErrQueueSaturated   — queue depth == max, request rejected.
var (
	ErrWorkerNotReady = errors.New("worker not ready")
	ErrWorkerShutdown = errors.New("worker is shutting down")
	ErrQueueSaturated = errors.New("worker queue is saturated")
)

// readyFlag / isShutdown are small read helpers.
func (w *Worker) readyFlag() bool {
	w.readyMu.RLock()
	defer w.readyMu.RUnlock()
	return w.ready
}

func (w *Worker) isShutdown() bool {
	w.readyMu.RLock()
	defer w.readyMu.RUnlock()
	// We use the warm flag as a proxy: once we tear down warm goes
	// false permanently. A more explicit field is unnecessary; the
	// lifecycle invariant is single-shot.
	return !w.ready && !w.warm && w.completed.Load() > 0 && w.inv.LastScannedAt.IsZero()
}

func (w *Worker) setReady(ready, warm bool) {
	w.readyMu.Lock()
	defer w.readyMu.Unlock()
	w.ready = ready
	w.warm = warm
}

func containsString(ls []string, want string) bool {
	for _, l := range ls {
		if l == want {
			return true
		}
	}
	return false
}
