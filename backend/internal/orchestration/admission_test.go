package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBoundedQueue_AcquireReturnsImmediatelyWhenSlotFree(t *testing.T) {
	q, err := NewBoundedQueue("test", 2, 4)
	if err != nil {
		t.Fatalf("NewBoundedQueue: %v", err)
	}
	rel, err := q.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if rel == nil {
		t.Fatalf("Acquire returned a nil release")
	}
	rel()
	stats := q.Stats()
	if stats.InFlight != 0 {
		t.Errorf("InFlight after release = %d, want 0", stats.InFlight)
	}
}

func TestBoundedQueue_FillsToMaxThenBlocks(t *testing.T) {
	q, err := NewBoundedQueue("test", 1, 1)
	if err != nil {
		t.Fatalf("NewBoundedQueue: %v", err)
	}
	r1, err := q.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer r1()
	// Second Acquire should wait (queue depth 1) until r1 releases.
	released := make(chan struct{})
	gotRelease := make(chan func(), 1)
	go func() {
		rel, err := q.Acquire(context.Background())
		if err != nil {
			t.Errorf("second Acquire: %v", err)
			return
		}
		gotRelease <- rel
		<-released
		rel()
	}()
	// Give the goroutine a moment to enqueue.
	time.Sleep(20 * time.Millisecond)
	stats := q.Stats()
	if stats.Waiters != 1 {
		t.Errorf("Waiters = %d, want 1 (one enqueued)", stats.Waiters)
	}
	r1()
	rel := <-gotRelease
	close(released)
	rel()
}

func TestBoundedQueue_RejectsWhenQueueFull(t *testing.T) {
	q, err := NewBoundedQueue("test", 1, 0) // no queue depth
	if err != nil {
		t.Fatalf("NewBoundedQueue: %v", err)
	}
	r1, err := q.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer r1()
	// Second Acquire should fail with ErrQueueSaturated because
	// MaxInflight is reached and QueueDepth is 0.
	_, err = q.Acquire(context.Background())
	if !errors.Is(err, ErrQueueSaturated) {
		t.Errorf("expected ErrQueueSaturated, got %v", err)
	}
	stats := q.Stats()
	if stats.Rejections != 1 {
		t.Errorf("Rejections = %d, want 1", stats.Rejections)
	}
}

func TestBoundedQueue_CancelDuringWait(t *testing.T) {
	q, err := NewBoundedQueue("test", 1, 1)
	if err != nil {
		t.Fatalf("NewBoundedQueue: %v", err)
	}
	r1, err := q.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer r1()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := q.Acquire(ctx)
		errCh <- err
	}()
	// Cancel the context after a short delay.
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire did not return after cancel")
	}
}

func TestBoundedQueue_RejectsInvalidConfig(t *testing.T) {
	if _, err := NewBoundedQueue("test", 0, 0); err == nil {
		t.Errorf("expected error for max=0")
	}
	if _, err := NewBoundedQueue("test", 1, -1); err == nil {
		t.Errorf("expected error for queue depth < 0")
	}
}

func TestBoundedQueue_ReleaseUnblocksOneWaiter(t *testing.T) {
	q, err := NewBoundedQueue("test", 1, 4)
	if err != nil {
		t.Fatalf("NewBoundedQueue: %v", err)
	}
	r1, err := q.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	// Enqueue 3 waiters; only one will get the slot after r1
	// releases, the others stay parked.
	gotReleases := make(chan func(), 3)
	for i := 0; i < 3; i++ {
		go func() {
			rel, err := q.Acquire(context.Background())
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			gotReleases <- rel
		}()
	}
	time.Sleep(30 * time.Millisecond)
	r1()
	// Wait for at least one release to be delivered.
	select {
	case rel := <-gotReleases:
		rel()
	case <-time.After(time.Second):
		t.Fatal("no waiter was unblocked after release")
	}
	// The other two should still be waiting — drain them in series
	// to confirm the wake-one semantics.
	for i := 0; i < 2; i++ {
		select {
		case rel := <-gotReleases:
			rel()
		case <-time.After(time.Second):
			t.Fatalf("waiter %d did not unblock", i)
		}
	}
}
