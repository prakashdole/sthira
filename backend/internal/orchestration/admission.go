package orchestration

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// BoundedQueue is the small primitive the orchestrator uses to bound
// per-stage admission. It is NOT a scheduling framework: it tracks
// in-flight tokens (MaxInflight) and a small queue of waiters
// (QueueDepth) and refuses new arrivals when the queue is full.
//
//   - Acquire returns immediately when a slot is free.
//   - When MaxInflight is reached, Acquire blocks until a slot frees
//     or the context is canceled.
//   - When QueueDepth waiters are already pending, Acquire returns
//     ErrQueueSaturated immediately (fail fast under overload).
//
// Release is paired with Acquire; the waiter's per-stage deadline
// cancels the wait. The primitive has zero external dependencies and
// uses only sync.Mutex + sync.Cond, no unbounded goroutines.
type BoundedQueue struct {
	name     string
	max      int
	queueMax int
	mu       sync.Mutex
	cond     *sync.Cond
	inFlight int
	waiters  int
	// rejectionCount is the cumulative number of times Acquire
	// returned ErrQueueSaturated. Exposed for metrics.
	rejectionCount atomic.Uint64
	// admissionCount is the cumulative number of successful Acquire.
	admissionCount atomic.Uint64
	// currentWaiters tracks per-waiter "is the slot available" signals
	// so each waiter can wake on either signal or cancellation. The
	// map is keyed by a pointer that the waiter's acquire function
	// allocates locally; the watcher goroutine writes to the channel
	// embedded in that pointer when ctx is canceled.
	currentWaiters map[*waiter]struct{}
}

type waiter struct {
	ready chan struct{} // closed when slot is available
	done  chan struct{} // closed when ctx is canceled
}

// NewBoundedQueue returns a queue with MaxInflight >= 1 and
// QueueDepth >= 0. Returns an error if MaxInflight <= 0 or
// QueueDepth < 0.
func NewBoundedQueue(name string, maxInflight, queueDepth int) (*BoundedQueue, error) {
	if maxInflight <= 0 {
		return nil, errors.New("orchestration: max inflight must be > 0")
	}
	if queueDepth < 0 {
		return nil, errors.New("orchestration: queue depth must be >= 0")
	}
	q := &BoundedQueue{
		name:           name,
		max:            maxInflight,
		queueMax:       queueDepth,
		currentWaiters: map[*waiter]struct{}{},
	}
	q.cond = sync.NewCond(&q.mu)
	return q, nil
}

// Acquire returns when a slot is available. On a saturated queue it
// returns ErrQueueSaturated immediately. On context cancellation it
// returns ctx.Err(). The returned release function MUST be called
// exactly once, typically via defer.
func (q *BoundedQueue) Acquire(ctx context.Context) (release func(), err error) {
	// Fail fast when the caller's ctx is already canceled.
	if err := ctx.Err(); err != nil {
		return func() {}, err
	}
	// Fast path: a slot is free. Avoid the lock when possible.
	q.mu.Lock()
	if q.inFlight < q.max {
		q.inFlight++
		q.admissionCount.Add(1)
		q.mu.Unlock()
		return q.makeRelease(), nil
	}
	if q.waiters >= q.queueMax {
		q.rejectionCount.Add(1)
		q.mu.Unlock()
		return func() {}, ErrQueueSaturated
	}
	w := &waiter{ready: make(chan struct{}), done: make(chan struct{})}
	q.waiters++
	q.currentWaiters[w] = struct{}{}
	q.mu.Unlock()

	// Watcher goroutine closes w.done and broadcasts the cond
	// when ctx fires. The watcher never leaks: it terminates
	// either on ctx.Done or on the waiter's release path
	// (which closes a stop channel).
	stopWatcher := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			q.mu.Lock()
			if _, ok := q.currentWaiters[w]; ok {
				delete(q.currentWaiters, w)
				if q.waiters > 0 {
					q.waiters--
				}
			}
			close(w.done)
			// Wake the parked waiter so it can re-check state.
			q.cond.Broadcast()
			q.mu.Unlock()
		case <-stopWatcher:
		}
	}()

	defer func() {
		select {
		case <-stopWatcher:
			// already stopped
		default:
			close(stopWatcher)
		}
	}()

	for {
		// First, check if our watcher goroutine already canceled us.
		select {
		case <-ctx.Done():
			return func() {}, ctx.Err()
		default:
		}
		select {
		case <-w.done:
			if err := ctx.Err(); err != nil {
				return func() {}, err
			}
			// spurious; loop again
		default:
		}

		// Re-check state under the lock.
		q.mu.Lock()
		if _, stillWaiting := q.currentWaiters[w]; !stillWaiting {
			// We've been canceled (ctx path). Return.
			q.mu.Unlock()
			if err := ctx.Err(); err != nil {
				return func() {}, err
			}
			continue
		}
		if q.inFlight < q.max {
			delete(q.currentWaiters, w)
			if q.waiters > 0 {
				q.waiters--
			}
			q.inFlight++
			q.admissionCount.Add(1)
			q.mu.Unlock()
			return q.makeRelease(), nil
		}
		// Re-check the saturation cap. If the queue grew past
		// queueMax while we waited (shouldn't happen, but
		// defensive), fail closed rather than pile on.
		if q.waiters > q.queueMax {
			delete(q.currentWaiters, w)
			if q.waiters > 0 {
				q.waiters--
			}
			q.rejectionCount.Add(1)
			q.mu.Unlock()
			return func() {}, ErrQueueSaturated
		}
		// Park on cond. The cond is shared by all waiters; the
		// Release function calls Signal so one slot at a time
		// wakes. We re-check state after waking.
		q.cond.Wait()
		q.mu.Unlock()
	}
}

func (q *BoundedQueue) makeRelease() func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			q.mu.Lock()
			if q.inFlight > 0 {
				q.inFlight--
			}
			q.cond.Signal()
			q.mu.Unlock()
		})
	}
}

// Stats returns a low-cardinality snapshot of the queue's state.
// Safe to call concurrently; values are best-effort.
func (q *BoundedQueue) Stats() QueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()
	return QueueStats{
		Name:          q.name,
		InFlight:      q.inFlight,
		Waiters:       q.waiters,
		MaxInflight:   q.max,
		MaxQueueDepth: q.queueMax,
		Rejections:    q.rejectionCount.Load(),
	}
}

// QueueStats is the metrics-facing snapshot of one BoundedQueue.
type QueueStats struct {
	Name          string
	InFlight      int
	Waiters       int
	MaxInflight   int
	MaxQueueDepth int
	Rejections    uint64
}

// WaitWithDeadline wraps Acquire with a deadline that closes over the
// queue's wait channel. Useful when the parent context has no
// deadline of its own.
func WaitWithDeadline(ctx context.Context, q *BoundedQueue, d time.Duration) (release func(), err error) {
	subCtx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return q.Acquire(subCtx)
}
