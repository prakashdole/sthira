package orchestration

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitBreakerState represents the operational state of the circuit breaker.
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "CLOSED"
	StateOpen     CircuitBreakerState = "OPEN"
	StateHalfOpen CircuitBreakerState = "HALF_OPEN"
)

var (
	// ErrCircuitOpen is returned immediately when the circuit breaker is open or saturated in half-open.
	ErrCircuitOpen = errors.New("circuit breaker is open: upstream worker unavailable")
)

// CircuitBreakerConfig defines threshold and timing parameters for worker protection.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive worker failures before the breaker trips open.
	FailureThreshold int
	// SuccessThreshold is the number of consecutive successful trial probes before resetting to closed.
	SuccessThreshold int
	// ResetTimeout is the cooldown duration before an open breaker permits trial requests in half-open.
	ResetTimeout time.Duration
	// Clock provides authoritative time, defaulting to time.Now if nil.
	Clock func() time.Time
}

// CircuitBreakerStats provides an instantaneous snapshot of breaker metrics.
type CircuitBreakerStats struct {
	Name                 string              `json:"name"`
	State                CircuitBreakerState `json:"state"`
	ConsecutiveFailures  int                 `json:"consecutive_failures"`
	ConsecutiveSuccesses int                 `json:"consecutive_successes"`
	TrippedCount         uint64              `json:"tripped_count"`
	LastStateChange      time.Time           `json:"last_state_change"`
}

// CircuitBreaker protects downstream callers and upstream ML workers from cascade failures
// and connection saturation during GPU surges, model crashes, or network partitions.
type CircuitBreaker struct {
	name                 string
	cfg                  CircuitBreakerConfig
	mu                   sync.Mutex
	state                CircuitBreakerState
	consecutiveFailures  int
	consecutiveSuccesses int
	trippedCount         uint64
	lastStateChange      time.Time
	halfOpenInFlight     bool
	clock                func() time.Time
}

// NewCircuitBreaker creates a circuit breaker with conservative defaults.
func NewCircuitBreaker(name string, cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = 10 * time.Second
	}
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}

	return &CircuitBreaker{
		name:            name,
		cfg:             cfg,
		state:           StateClosed,
		lastStateChange: clock(),
		clock:           clock,
	}
}

// Execute wraps an upstream invocation with circuit protection.
//
// Invariants:
//  1. In CLOSED: calls proceed normally. Consecutive failures >= FailureThreshold trip to OPEN.
//  2. In OPEN: calls fail fast immediately with ErrCircuitOpen.
//  3. After ResetTimeout expires: transitions to HALF_OPEN.
//  4. In HALF_OPEN: admits a single concurrent probe. If the probe fails, trips immediately back to OPEN.
//     If consecutive probes >= SuccessThreshold succeed, resets to CLOSED.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// 1. Pre-execution gate check
	cb.mu.Lock()
	now := cb.clock()

	if cb.state == StateOpen {
		if now.Sub(cb.lastStateChange) >= cb.cfg.ResetTimeout {
			cb.state = StateHalfOpen
			cb.consecutiveSuccesses = 0
			cb.halfOpenInFlight = false
			cb.lastStateChange = now
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
	}

	if cb.state == StateHalfOpen {
		if cb.halfOpenInFlight {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
		cb.halfOpenInFlight = true
	}
	cb.mu.Unlock()

	// 2. Execute upstream call outside lock
	callErr := fn()

	// 3. Post-execution state reconciliation
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now = cb.clock()

	if cb.state == StateHalfOpen {
		cb.halfOpenInFlight = false
		if callErr != nil {
			// Probe failed: trip back to OPEN
			cb.state = StateOpen
			cb.consecutiveFailures++
			cb.consecutiveSuccesses = 0
			cb.trippedCount++
			cb.lastStateChange = now
			return callErr
		}

		cb.consecutiveSuccesses++
		if cb.consecutiveSuccesses >= cb.cfg.SuccessThreshold {
			// Probes passed: reset to CLOSED
			cb.state = StateClosed
			cb.consecutiveFailures = 0
			cb.consecutiveSuccesses = 0
			cb.lastStateChange = now
		}
		return nil
	}

	if cb.state == StateClosed {
		if callErr != nil {
			cb.consecutiveFailures++
			if cb.consecutiveFailures >= cb.cfg.FailureThreshold {
				cb.state = StateOpen
				cb.trippedCount++
				cb.lastStateChange = now
			}
			return callErr
		}
		cb.consecutiveFailures = 0
		return nil
	}

	return callErr
}

// State returns the current operational state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateOpen && cb.clock().Sub(cb.lastStateChange) >= cb.cfg.ResetTimeout {
		return StateHalfOpen
	}
	return cb.state
}

// Stats returns an instantaneous snapshot of breaker metrics.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	currentState := cb.state
	if currentState == StateOpen && cb.clock().Sub(cb.lastStateChange) >= cb.cfg.ResetTimeout {
		currentState = StateHalfOpen
	}

	return CircuitBreakerStats{
		Name:                 cb.name,
		State:                currentState,
		ConsecutiveFailures:  cb.consecutiveFailures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
		TrippedCount:         cb.trippedCount,
		LastStateChange:      cb.lastStateChange,
	}
}

// Reset resets the breaker to StateClosed with zero failures.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.halfOpenInFlight = false
	cb.lastStateChange = cb.clock()
}
