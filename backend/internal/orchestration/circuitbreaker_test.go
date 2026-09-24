package orchestration_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/orchestration"
)

func TestCircuitBreaker_ClosedSuccess(t *testing.T) {
	cb := orchestration.NewCircuitBreaker("test-asr", orchestration.CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		ResetTimeout:     5 * time.Second,
	})

	if cb.State() != orchestration.StateClosed {
		t.Fatalf("expected StateClosed, got %v", cb.State())
	}

	callCount := 0
	err := cb.Execute(context.Background(), func() error {
		callCount++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected callCount 1, got %d", callCount)
	}

	stats := cb.Stats()
	if stats.ConsecutiveFailures != 0 || stats.TrippedCount != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestCircuitBreaker_TripToOpenAndFailFast(t *testing.T) {
	mockTime := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	var timeMu sync.Mutex
	clock := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return mockTime
	}

	cb := orchestration.NewCircuitBreaker("test-worker", orchestration.CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		ResetTimeout:     10 * time.Second,
		Clock:            clock,
	})

	workerErr := errors.New("upstream worker crash")

	// 1. Fail twice (below threshold 3)
	for i := 0; i < 2; i++ {
		err := cb.Execute(context.Background(), func() error {
			return workerErr
		})
		if !errors.Is(err, workerErr) {
			t.Fatalf("expected workerErr, got %v", err)
		}
		if cb.State() != orchestration.StateClosed {
			t.Fatalf("expected still closed after %d failures", i+1)
		}
	}

	// 2. 3rd failure trips the breaker
	err := cb.Execute(context.Background(), func() error {
		return workerErr
	})
	if !errors.Is(err, workerErr) {
		t.Fatalf("expected workerErr, got %v", err)
	}
	if cb.State() != orchestration.StateOpen {
		t.Fatalf("expected StateOpen after 3 failures, got %v", cb.State())
	}

	// 3. Subsequent call fails fast with ErrCircuitOpen without executing fn
	fnExecuted := false
	err = cb.Execute(context.Background(), func() error {
		fnExecuted = true
		return nil
	})
	if !errors.Is(err, orchestration.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if fnExecuted {
		t.Fatalf("fn must not be executed when circuit is open")
	}

	stats := cb.Stats()
	if stats.TrippedCount != 1 {
		t.Fatalf("expected TrippedCount 1, got %d", stats.TrippedCount)
	}
}

func TestCircuitBreaker_HalfOpenRecoveryAndReTrip(t *testing.T) {
	mockTime := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	var timeMu sync.Mutex
	advanceTime := func(d time.Duration) {
		timeMu.Lock()
		mockTime = mockTime.Add(d)
		timeMu.Unlock()
	}
	clock := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return mockTime
	}

	cb := orchestration.NewCircuitBreaker("test-recovery", orchestration.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		ResetTimeout:     10 * time.Second,
		Clock:            clock,
	})

	workerErr := errors.New("timeout")

	// Trip to open
	cb.Execute(context.Background(), func() error { return workerErr })
	cb.Execute(context.Background(), func() error { return workerErr })

	if cb.State() != orchestration.StateOpen {
		t.Fatalf("expected StateOpen, got %v", cb.State())
	}

	// Advance time past ResetTimeout (10s)
	advanceTime(11 * time.Second)

	if cb.State() != orchestration.StateHalfOpen {
		t.Fatalf("expected StateHalfOpen after timeout, got %v", cb.State())
	}

	// Trial probe 1 succeeds
	err := cb.Execute(context.Background(), func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected probe 1 error: %v", err)
	}

	// Still in HalfOpen after 1 success (SuccessThreshold = 2)
	if cb.State() != orchestration.StateHalfOpen {
		t.Fatalf("expected StateHalfOpen after 1 success, got %v", cb.State())
	}

	// Trial probe 2 succeeds -> resets to StateClosed
	err = cb.Execute(context.Background(), func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected probe 2 error: %v", err)
	}

	if cb.State() != orchestration.StateClosed {
		t.Fatalf("expected StateClosed after 2 successes, got %v", cb.State())
	}

	// Now trip it again
	cb.Execute(context.Background(), func() error { return workerErr })
	cb.Execute(context.Background(), func() error { return workerErr })
	if cb.State() != orchestration.StateOpen {
		t.Fatalf("expected StateOpen, got %v", cb.State())
	}

	// Advance time past ResetTimeout again
	advanceTime(11 * time.Second)

	// Trial probe fails -> immediately re-trips to StateOpen
	err = cb.Execute(context.Background(), func() error { return workerErr })
	if !errors.Is(err, workerErr) {
		t.Fatalf("expected workerErr, got %v", err)
	}
	if cb.State() != orchestration.StateOpen {
		t.Fatalf("expected StateOpen after failed half-open probe, got %v", cb.State())
	}
	if cb.Stats().TrippedCount != 3 {
		t.Fatalf("expected TrippedCount 3, got %d", cb.Stats().TrippedCount)
	}
}

func TestCircuitBreaker_ConcurrentExecution(t *testing.T) {
	cb := orchestration.NewCircuitBreaker("test-concurrent", orchestration.CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		ResetTimeout:     50 * time.Millisecond,
	})

	var wg sync.WaitGroup
	var completed atomic.Int64
	var workerErr = errors.New("intermittent fail")

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = cb.Execute(context.Background(), func() error {
				if idx%5 == 0 {
					return workerErr
				}
				return nil
			})
			completed.Add(1)
		}(i)
	}

	wg.Wait()
	if completed.Load() != 50 {
		t.Fatalf("expected 50 completed executions, got %d", completed.Load())
	}
}
