package asrworker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// readyStub returns a Worker configured to be Ready=true with a
// stub runtime. Tests use this to focus on the worker-level
// invariants (queue, deadline, concurrency) rather than wiring.
func readyStub(t *testing.T) *Worker {
	t.Helper()
	inv := Inventory{
		IndicConformer:   DefaultIndicConformer(),
		AllowedLanguages: []string{"hi-IN", "ml-IN"},
	}
	w, err := NewWorker(Config{
		Inventory:     inv,
		Runtime:       NewStubRuntime(),
		QueueDepth:    4,
		MaxInFlight:   2,
		BuildRevision: "test-build",
	})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	return w
}

// TestNewWorker_RequiresRuntime: a nil runtime is rejected.
func TestNewWorker_RequiresRuntime(t *testing.T) {
	if _, err := NewWorker(Config{Runtime: nil}); err == nil {
		t.Fatal("expected runtime-required error")
	}
}

// TestNewWorker_RejectsInflightGreaterThanQueue: invariant.
func TestNewWorker_RejectsInflightGreaterThanQueue(t *testing.T) {
	_, err := NewWorker(Config{
		Runtime:     NewStubRuntime(),
		QueueDepth:  1,
		MaxInFlight: 4,
	})
	if err == nil {
		t.Fatalf("expected inflight>queue rejection")
	}
}

// TestLoadAndVerify_EmptyInventoryIsNotReady: an inventory that
// reports PendingCritical fails LoadAndVerify.
func TestLoadAndVerify_EmptyInventoryIsNotReady(t *testing.T) {
	w, err := NewWorker(Config{
		Inventory:  Inventory{IndicConformer: DefaultIndicConformer()},
		Runtime:    NewStubRuntime(),
		QueueDepth: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.LoadAndVerify(context.Background()); err == nil {
		t.Fatal("expected inventory-not-ready error")
	}
}

// TestLoadAndVerify_SubprocessStubFails: the subprocess stub does
// not advertise a revision or digest, so LoadAndVerify rejects.
func TestLoadAndVerify_SubprocessStubFails(t *testing.T) {
	inv := Inventory{
		IndicConformer:   DefaultIndicConformer(),
		AllowedLanguages: []string{"hi-IN"},
	}
	w, err := NewWorker(Config{
		Inventory:  inv,
		Runtime:    NewSubprocessRuntime(DefaultSubprocessRuntimeConfig()),
		QueueDepth: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.LoadAndVerify(context.Background()); err == nil {
		t.Fatal("expected empty-metadata error")
	}
}

// TestDispatch_BeforeReadyReturnsNotReady: a Worker that hasn't
// completed LoadAndVerify rejects with ErrWorkerNotReady.
func TestDispatch_BeforeReadyReturnsNotReady(t *testing.T) {
	w, _ := NewWorker(Config{
		Inventory:  Inventory{IndicConformer: DefaultIndicConformer(), AllowedLanguages: []string{"hi-IN"}},
		Runtime:    NewStubRuntime(),
		QueueDepth: 2,
	})
	_, err := w.Dispatch(context.Background(), TranscribeRequest{Language: "hi-IN", Samples: []float32{0.1}})
	if !errors.Is(err, ErrWorkerNotReady) {
		t.Fatalf("expected ErrWorkerNotReady, got %v", err)
	}
}

// TestDispatch_RejectsUnsupportedLanguage: Worker-level language
// filter rejects before queueing.
func TestDispatch_RejectsUnsupportedLanguage(t *testing.T) {
	w := readyStub(t)
	defer w.Shutdown(context.Background())
	_, err := w.Dispatch(context.Background(), TranscribeRequest{
		Language: "ta-IN",
		Samples:  []float32{0.1},
	})
	if !errors.Is(err, ErrLanguageUnsupported) {
		t.Fatalf("expected ErrLanguageUnsupported, got %v", err)
	}
}

// TestDispatch_QueueSaturationReturnsTypedError: when the queue
// is full, Dispatch returns ErrQueueSaturated immediately. The
// orchestrator's /transcribe handler maps this to TranscriptionState
// = UNAVAILABLE.
func TestDispatch_QueueSaturationReturnsTypedError(t *testing.T) {
	rt := NewStubRuntime()
	rt.LatencyMs = 200 // holds the in-flight slot
	inv := Inventory{IndicConformer: DefaultIndicConformer(), AllowedLanguages: []string{"hi-IN", "ml-IN"}}
	w, err := NewWorker(Config{
		Inventory:   inv,
		Runtime:     rt,
		QueueDepth:  1,
		MaxInFlight: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Bounded number of goroutines; we coordinate completion via
	// wg to keep -race clean.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = w.Dispatch(context.Background(), TranscribeRequest{
			Language: "hi-IN",
			Samples:  []float32{0.1, -0.1},
		})
	}()
	go func() {
		defer wg.Done()
		_, _ = w.Dispatch(context.Background(), TranscribeRequest{
			Language: "hi-IN",
			Samples:  []float32{0.1, -0.1},
		})
	}()
	// Yield so both background goroutines have entered the channel
	// or are blocked in select.
	time.Sleep(50 * time.Millisecond)
	_, err = w.Dispatch(context.Background(), TranscribeRequest{
		Language: "hi-IN",
		Samples:  []float32{0.1, -0.1},
	})
	if !errors.Is(err, ErrQueueSaturated) {
		t.Fatalf("expected ErrQueueSaturated, got %v", err)
	}
	if w.MetricsSnapshot().Saturated == 0 {
		t.Errorf("expected saturated counter to increment")
	}
	// Wait for the saturated-rejected third call to leave the
	// state, then let the queued items drain on their own, then
	// shut down. Closing jobs before the goroutines finish would
	// race with the writers.
	wg.Wait()
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestDispatch_CancellationDoesNotInvokeRuntime: a request whose
// context is already cancelled never reaches the runtime.
//
// We prove this by counting runtime invocations before and after
// the cancellation; the count must not change.
func TestDispatch_CancellationDoesNotInvokeRuntime(t *testing.T) {
	counter := &countingRuntime{base: NewStubRuntime(), calls: &atomic.Int64{}}
	inv := Inventory{IndicConformer: DefaultIndicConformer(), AllowedLanguages: []string{"hi-IN"}}
	w, err := NewWorker(Config{
		Inventory:   inv,
		Runtime:     counter,
		QueueDepth:  2,
		MaxInFlight: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Shutdown(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = w.Dispatch(ctx, TranscribeRequest{
		Language: "hi-IN",
		Samples:  []float32{0.1, 0.2},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Give the worker pool a chance to (incorrectly) invoke the
	// runtime. It must NOT.
	time.Sleep(50 * time.Millisecond)
	if counter.calls.Load() > 0 {
		t.Errorf("runtime must NOT be invoked when caller context is cancelled; got %d calls", counter.calls.Load())
	}
}

// countingRuntime wraps another Runtime and counts Transcribe calls.
type countingRuntime struct {
	base  Runtime
	calls *atomic.Int64
}

func (c *countingRuntime) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	c.calls.Add(1)
	return c.base.Transcribe(ctx, req)
}

func (c *countingRuntime) SupportedLanguages() []string { return c.base.SupportedLanguages() }
func (c *countingRuntime) Revision() string             { return c.base.Revision() }
func (c *countingRuntime) Digest() (string, string)     { return c.base.Digest() }
func (c *countingRuntime) Close() error                 { return c.base.Close() }

// TestDispatch_PerRequestDeadline: a request with a deadline is
// short-circuited if the deadline is already past.
func TestDispatch_PerRequestDeadline(t *testing.T) {
	w := readyStub(t)
	defer w.Shutdown(context.Background())
	req := TranscribeRequest{
		Language: "hi-IN",
		Samples:  []float32{0.1, 0.2},
		Deadline: time.Now().Add(-1 * time.Second),
	}
	_, err := w.Dispatch(context.Background(), req)
	if err == nil {
		t.Fatalf("expected deadline-exceeded error")
	}
}

// TestShutdown_RejectsNewDispatch: after Shutdown, Dispatch
// returns ErrWorkerShutdown.
func TestShutdown_RejectsNewDispatch(t *testing.T) {
	w := readyStub(t)
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := w.Dispatch(context.Background(), TranscribeRequest{
		Language: "hi-IN",
		Samples:  []float32{0.1},
	})
	if err == nil {
		t.Fatalf("expected ErrWorkerShutdown")
	}
}

// TestShutdown_IsIdempotent: calling Shutdown twice does not
// panic and does not error.
func TestShutdown_IsIdempotent(t *testing.T) {
	w := readyStub(t)
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown must be a no-op; got %v", err)
	}
}

// TestSnapshot_ReportsReadyAndWarm: after LoadAndVerify, the
// snapshot reports Ready=true and Warm=true. The orchestrator
// reads this in /health.
func TestSnapshot_ReportsReadyAndWarm(t *testing.T) {
	w := readyStub(t)
	defer w.Shutdown(context.Background())
	h := w.Snapshot()
	if !h.Ready || !h.Warm {
		t.Errorf("expected Ready and Warm; got Ready=%v Warm=%v", h.Ready, h.Warm)
	}
	if len(h.SupportedLanguages) == 0 {
		t.Errorf("SupportedLanguages must be populated from runtime")
	}
	if h.Queue.MaxConcurrency != 2 {
		t.Errorf("MaxConcurrency: got %d want 2", h.Queue.MaxConcurrency)
	}
	if h.Queue.MaxDepth != 4 {
		t.Errorf("MaxDepth: got %d want 4", h.Queue.MaxDepth)
	}
}

// TestDispatch_ConcurrencyCappedAtMaxInflight: with MaxInFlight=1
// and a stub latency, two concurrent requests serialize. We prove
// this by counting OVERLAPPING Transcribe invocations inside the
// runtime; only one may be in flight at a time.
func TestDispatch_ConcurrencyCappedAtMaxInflight(t *testing.T) {
	stub := NewStubRuntime()
	stub.LatencyMs = 80
	tracker := &concurrencyTrackingRuntime{base: stub}
	inv := Inventory{IndicConformer: DefaultIndicConformer(), AllowedLanguages: []string{"hi-IN"}}
	w, err := NewWorker(Config{
		Inventory:   inv,
		Runtime:     tracker,
		QueueDepth:  4,
		MaxInFlight: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer w.Shutdown(context.Background())
	var wg sync.WaitGroup
	wg.Add(4)
	for i := 0; i < 4; i++ {
		go func() {
			defer wg.Done()
			_, _ = w.Dispatch(context.Background(), TranscribeRequest{
				Language: "hi-IN",
				Samples:  []float32{0.1, 0.2},
			})
		}()
	}
	wg.Wait()
	if tracker.max.Load() > 1 {
		t.Errorf("MaxInFlight must be 1 at the runtime; observed %d concurrent", tracker.max.Load())
	}
}

// concurrencyTrackingRuntime records the maximum number of
// overlapping Transcribe calls observed by an inner Runtime.
type concurrencyTrackingRuntime struct {
	base    *StubRuntime
	current atomic.Int64
	max     atomic.Int64
}

func (c *concurrencyTrackingRuntime) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error) {
	now := c.current.Add(1)
	defer c.current.Add(-1)
	for {
		m := c.max.Load()
		if now <= m || c.max.CompareAndSwap(m, now) {
			break
		}
	}
	return c.base.Transcribe(ctx, req)
}
func (c *concurrencyTrackingRuntime) SupportedLanguages() []string {
	return c.base.SupportedLanguages()
}
func (c *concurrencyTrackingRuntime) Revision() string { return c.base.Revision() }
func (c *concurrencyTrackingRuntime) Digest() (string, string) {
	return c.base.Digest()
}
func (c *concurrencyTrackingRuntime) Close() error { return c.base.Close() }
