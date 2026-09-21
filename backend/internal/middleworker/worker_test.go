// Tests for the Worker (warm lifecycle, queue, concurrency,
// language allow-list) and the StubRuntime / SubprocessRuntime
// adapters. The Server is exercised separately.

package middleworker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "proposal.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

const stubProposalOK = `{"schema_version":"3.0","request_id":"REQ-1","data_version":"v1","status":"OK","intent":"FOCUS_PLACE","language":"en-IN","actions":[{"type":"FOCUS_FEATURE","target_id":"P1"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}`

func newReadyStubWorker(t *testing.T) *Worker {
	t.Helper()
	path := writeFixture(t, stubProposalOK)
	rt, err := NewStubRuntime(path)
	if err != nil {
		t.Fatalf("NewStubRuntime: %v", err)
	}
	rt.SetLanguages([]string{"en-IN", "hi-IN"})
	w, err := NewWorker(Config{
		Runtime:       rt,
		QueueDepth:    4,
		MaxInFlight:   1,
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

func TestWorker_LoadAndVerifyRequiresRevision(t *testing.T) {
	w, err := NewWorker(Config{Runtime: NewSubprocessRuntime(), QueueDepth: 1, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	if err := w.LoadAndVerify(context.Background()); err == nil {
		t.Fatalf("expected LoadAndVerify to fail when runtime reports empty revision")
	}
	if w.Snapshot().Ready {
		t.Fatalf("worker must not be ready after failed LoadAndVerify")
	}
}

func TestWorker_DispatchBeforeReadyRejected(t *testing.T) {
	w, err := NewWorker(Config{Runtime: NewSubprocessRuntime(), QueueDepth: 1, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	// No LoadAndVerify.
	_, err = w.Dispatch(context.Background(), RequestEnvelope{RequestID: "r1"})
	if !errors.Is(err, ErrWorkerNotReady) {
		t.Fatalf("expected ErrWorkerNotReady, got %v", err)
	}
}

func TestWorker_DispatchHappyPath(t *testing.T) {
	w := newReadyStubWorker(t)
	resp, err := w.Dispatch(context.Background(), RequestEnvelope{
		RequestID: "REQ-1",
		ScopedContext: ScopedContext{
			SchemaVersion:    "3.0",
			DataVersion:      "v1",
			AllowedLanguages: []string{"en-IN"},
			SourceVersion:    1,
			TemplateVersion:  1,
		},
		Transcript: TranscriptInput{
			RequestID: "REQ-1",
			Language:  "en-IN",
			Text:      "show me shelter",
			State:     "OK",
		},
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if resp.Proposal.RequestID != "REQ-1" {
		t.Errorf("proposal.request_id=%q", resp.Proposal.RequestID)
	}
	if resp.Proposal.Status != "OK" {
		t.Errorf("status=%q", resp.Proposal.Status)
	}
}

func TestWorker_LanguageAllowList(t *testing.T) {
	w := newReadyStubWorker(t)
	_, err := w.Dispatch(context.Background(), RequestEnvelope{
		RequestID: "REQ-1",
		Transcript: TranscriptInput{
			RequestID: "REQ-1",
			Language:  "fr-FR",
			Text:      "bonjour",
			State:     "OK",
		},
	})
	if err == nil {
		t.Fatalf("expected language rejection")
	}
	if !strings.Contains(err.Error(), "not in runtime allow-list") {
		t.Fatalf("expected allow-list rejection, got %v", err)
	}
}

func TestWorker_QueueSaturation(t *testing.T) {
	rt := newSlowStubRuntime()
	w, err := NewWorker(Config{Runtime: rt, QueueDepth: 1, MaxInFlight: 1})
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	rt.SetLanguages([]string{"en-IN"})
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	// Saturate the queue: 1 in-flight + 1 queued = 2. The 3rd
	// must be rejected with ErrQueueSaturated.
	rt.holdOn()

	enqueue := func() error {
		_, err := w.Dispatch(context.Background(), RequestEnvelope{
			RequestID: "REQ",
			Transcript: TranscriptInput{
				RequestID: "REQ", Language: "en-IN", Text: "x", State: "OK",
			},
		})
		return err
	}
	// In-flight: runs on the worker goroutine; it'll block on hold.
	g1err := make(chan error, 1)
	go func() { g1err <- enqueue() }()
	time.Sleep(20 * time.Millisecond) // ensure g1 is running

	// Queued: enqueue one more (won't block because queue depth = 1).
	g2err := make(chan error, 1)
	go func() { g2err <- enqueue() }()
	time.Sleep(20 * time.Millisecond)

	// Third: must saturate immediately.
	err = enqueue()
	if !errors.Is(err, ErrQueueSaturated) {
		t.Fatalf("expected ErrQueueSaturated, got %v", err)
	}
	// Release g1.
	rt.release()
	if err := <-g1err; err != nil {
		t.Fatalf("g1: %v", err)
	}
	if err := <-g2err; err != nil {
		t.Fatalf("g2: %v", err)
	}
}

func TestWorker_Cancellation(t *testing.T) {
	rt := newSlowStubRuntime()
	w, _ := NewWorker(Config{Runtime: rt, QueueDepth: 1, MaxInFlight: 1})
	rt.SetLanguages([]string{"en-IN"})
	w.LoadAndVerify(context.Background())
	rt.holdOn()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := w.Dispatch(ctx, RequestEnvelope{
			RequestID: "REQ",
			Transcript: TranscriptInput{
				RequestID: "REQ", Language: "en-IN", Text: "x", State: "OK",
			},
		})
		errCh <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, ErrCanceled) {
			t.Fatalf("expected ErrCanceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("Dispatch did not return after cancel")
	}
	rt.release()
}

func TestWorker_SubprocessRuntimeStaysNotReady(t *testing.T) {
	// The SubprocessRuntime reports empty revision/digest/languages
	// by design. LoadAndVerify refuses to mark it ready. The worker
	// never dispatches; the orchestrator surfaces MODEL_UNAVAILABLE
	// before reaching the runtime.
	w, _ := NewWorker(Config{Runtime: NewSubprocessRuntime(), QueueDepth: 1, MaxInFlight: 1})
	err := w.LoadAndVerify(context.Background())
	if err == nil {
		t.Fatalf("expected LoadAndVerify to fail on SubprocessRuntime")
	}
	if w.Snapshot().Ready {
		t.Fatalf("SubprocessRuntime must keep the worker NOT-READY")
	}
	_, err = w.Dispatch(context.Background(), RequestEnvelope{
		RequestID: "REQ",
		Transcript: TranscriptInput{
			RequestID: "REQ", Language: "en-IN", Text: "x", State: "OK",
		},
	})
	if !errors.Is(err, ErrWorkerNotReady) {
		t.Fatalf("expected ErrWorkerNotReady, got %v", err)
	}
}

func TestWorker_HealthEnvelopeShape(t *testing.T) {
	w := newReadyStubWorker(t)
	h := w.Snapshot()
	if !h.Ready || !h.Warm {
		t.Errorf("health: ready=%v warm=%v", h.Ready, h.Warm)
	}
	if h.Queue.MaxDepth != 4 || h.Queue.MaxConcurrency != 1 {
		t.Errorf("queue: %+v", h.Queue)
	}
	if len(h.SupportedLanguages) == 0 {
		t.Errorf("supported_languages empty")
	}
}

func TestWorker_ShutdownDrains(t *testing.T) {
	w := newReadyStubWorker(t)
	if err := w.Shutdown(time.Second); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if w.Snapshot().Ready {
		t.Errorf("ready must be false after Shutdown")
	}
}

// slowStubRuntime is a stub that holds each Propose call on the
// `hold` channel until the test releases it. Used to exercise
// concurrency, queueing and cancellation.
type slowStubRuntime struct {
	mu       sync.Mutex
	revision string
	hold     chan struct{}
	closed   bool
	calls    atomic.Int32
	langs    []string
}

func newSlowStubRuntime() *slowStubRuntime {
	return &slowStubRuntime{
		revision: "slow-stub-0",
	}
}

// holdOn arms the gate; subsequent Propose calls block until
// release() or ctx.Done.
func (s *slowStubRuntime) holdOn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hold == nil {
		s.hold = make(chan struct{})
	}
}

// release closes the gate exactly once; idempotent.
func (s *slowStubRuntime) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.hold != nil {
		close(s.hold)
	}
	s.closed = true
}

func (s *slowStubRuntime) SetLanguages(langs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.langs = append([]string(nil), langs...)
}

func (s *slowStubRuntime) Propose(ctx context.Context, req RequestEnvelope) (*ProposeOutput, error) {
	s.calls.Add(1)
	s.mu.Lock()
	hold := s.hold
	s.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return nil, ErrCanceled
		}
	}
	proposal, err := decodeStrictProposal([]byte(stubProposalOK))
	if err != nil {
		return nil, err
	}
	proposal.RequestID = req.RequestID
	return &ProposeOutput{
		Proposal:      proposal,
		ModelRevision: s.revision,
		FinishReason:  "stop",
	}, nil
}

func (s *slowStubRuntime) Revision() string { return s.revision }
func (s *slowStubRuntime) Digest() (string, string) {
	return "slow-stub", "1111111111111111111111111111111111111111111111111111111111111111"
}
func (s *slowStubRuntime) Languages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.langs...)
}

// stubWithLanguages is the minimum stub that reports a non-empty
// revision so LoadAndVerify passes; used by tests that want a
// ready worker but want to drive Propose's failure path
// independently.
type stubWithLanguages struct{}

func (s *stubWithLanguages) Propose(_ context.Context, _ RequestEnvelope) (*ProposeOutput, error) {
	return nil, ErrRuntimeUnavailable
}
func (s *stubWithLanguages) Revision() string {
	return "test-revision"
}
func (s *stubWithLanguages) Digest() (string, string) {
	return "test", "2222222222222222222222222222222222222222222222222222222222222222"
}
func (s *stubWithLanguages) Languages() []string { return []string{"en-IN"} }

// TestRuntimeUnavailabilitySurfacesAsTypedError verifies that
// ErrRuntimeUnavailable from any Runtime propagates through
// Dispatch as the same typed sentinel.
func TestRuntimeUnavailabilitySurfacesAsTypedError(t *testing.T) {
	w, _ := NewWorker(Config{Runtime: &stubWithLanguages{}, QueueDepth: 1, MaxInFlight: 1})
	if err := w.LoadAndVerify(context.Background()); err != nil {
		t.Fatalf("LoadAndVerify: %v", err)
	}
	_, err := w.Dispatch(context.Background(), RequestEnvelope{
		RequestID: "REQ",
		Transcript: TranscriptInput{
			RequestID: "REQ", Language: "en-IN", Text: "x", State: "OK",
		},
	})
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Fatalf("expected ErrRuntimeUnavailable, got %v", err)
	}
}
