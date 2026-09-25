package orchestration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"sthira/backend/internal/contracts"
)

// CorrelationID is a server-generated request identifier. The first
// 16 hex bytes are random; the prefix makes them grep-friendly in
// logs without leaking any client-supplied value.
type CorrelationID string

func newCorrelationID() CorrelationID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure means the OS RNG is broken; fail
		// closed by returning a constant — the orchestrator will
		// still reject anything that doesn't match.
		return "req-norand"
	}
	return CorrelationID("req-" + hex.EncodeToString(b[:]))
}

// Correlation is the per-request slot the orchestrator uses to drop
// obsolete results. A stage records its in-flight key when it
// dispatches; the response is accepted only if the request still
// owns the slot. After cancellation, the slot is removed and any
// later responses are dropped (and counted as stale).
type Correlation struct {
	ID         CorrelationID
	Stage      Stage
	Generation uint64 // bumped on cancellation/withdrawal; late responses with mismatched generation are dropped
}

// CorrelationMap is keyed by request_id; each entry holds a stage →
// generation map. Lookups are O(1) and require no allocation in the
// hot path. The map is small (bounded by MaxInflight) so a
// sync.RWMutex is sufficient.
//
// Generation semantics: the counter for an (id, stage) pair only
// resets when Withdraw(id) is called. A CheckAndConsume deletes the
// in-flight slot but preserves the generation counter so the next
// Claim returns a strictly larger value.
type CorrelationMap struct {
	mu       sync.Mutex
	entries  map[CorrelationID]map[Stage]uint64
	inflight map[CorrelationID]map[Stage]uint64
}

// NewCorrelationMap returns an empty correlation map.
func NewCorrelationMap() *CorrelationMap {
	return &CorrelationMap{
		entries:  map[CorrelationID]map[Stage]uint64{},
		inflight: map[CorrelationID]map[Stage]uint64{},
	}
}

// Claim reserves the (id, stage) slot for a new generation. Returns
// the generation number assigned. If a previous generation is still
// pending, it is replaced (the older worker call will see its result
// dropped when it returns). The generation counter strictly
// increases per (id, stage) pair until Withdraw is called.
func (m *CorrelationMap) Claim(id CorrelationID, stage Stage) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	gen, ok := m.entries[id]
	if !ok {
		gen = map[Stage]uint64{}
		m.entries[id] = gen
	}
	gen[stage]++
	return gen[stage]
}

// Withdraw removes all stages for a request. Subsequent claims
// start a fresh generation (counter reset).
func (m *CorrelationMap) Withdraw(id CorrelationID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, id)
	delete(m.inflight, id)
}

// CheckAndConsume reports whether the (id, stage, generation) tuple
// is still the current owner. If yes, the in-flight slot is consumed
// (removed) so no further responses for that (id, stage) are
// accepted. Returns false when:
//   - the request_id is unknown (already withdrawn),
//   - the stage was never claimed (caller bug),
//   - the generation has been superseded.
func (m *CorrelationMap) CheckAndConsume(id CorrelationID, stage Stage, gen uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	inflight, ok := m.inflight[id]
	if !ok {
		inflight = map[Stage]uint64{}
		m.inflight[id] = inflight
	}
	// Re-claim the inflight slot if no entry exists. This avoids
	// the case where the previous slot was consumed and the next
	// worker hasn't called Claim yet; we treat that as "no
	// current call to consume".
	if g, exists := inflight[stage]; !exists || g != gen {
		return false
	}
	delete(inflight, stage)
	if len(inflight) == 0 {
		delete(m.inflight, id)
	}
	return true
}

// MarkInflight records the (id, stage, generation) tuple as the
// current in-flight call. Called by the orchestrator immediately
// after Claim so CheckAndConsume can match the worker's response.
// In production the orchestrator's wrapper does this; tests can
// call it directly.
func (m *CorrelationMap) MarkInflight(id CorrelationID, stage Stage, gen uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inflight, ok := m.inflight[id]
	if !ok {
		inflight = map[Stage]uint64{}
		m.inflight[id] = inflight
	}
	inflight[stage] = gen
}

// Peek returns the current generation for (id, stage) without
// consuming. Useful for diagnostics and tests.
func (m *CorrelationMap) Peek(id CorrelationID, stage Stage) (uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	gen, ok := m.entries[id]
	if !ok {
		return 0, false
	}
	g, ok := gen[stage]
	return g, ok
}

// CorrelationObserver is the interface the orchestrator uses to count
// stale drops. Real implementations write to the metrics package.
type CorrelationObserver interface {
	ObserveStaleDrop(stage Stage)
}

// TrackWorkerDispatch records the in-flight correlation and returns
// the generation the worker call should echo in its response. If the
// caller cancels before the response arrives, the orchestrator calls
// Withdraw on the same ID and the stale response is dropped.
//
// The generation is independent per stage: a fresh middle-stage call
// after a stale middle-stage drop is a new generation.
func TrackWorkerDispatch(id CorrelationID, stage Stage, cm *CorrelationMap) uint64 {
	return cm.Claim(id, stage)
}

// WithdrawOnCancel calls Withdraw when the parent context is canceled.
// Safe to call from any goroutine; idempotent under repeated cancel.
func WithdrawOnCancel(ctx context.Context, id CorrelationID, cm *CorrelationMap) {
	go func() {
		<-ctx.Done()
		cm.Withdraw(id)
	}()
}

// ErrCorrelationMismatch is returned when a worker's response carries
// a request_id / generation that doesn't match the in-flight slot.
// The orchestrator never delivers such a response to the citizen.
var ErrCorrelationMismatch = errors.New("orchestration: correlation mismatch (stale or wrong worker)")

// ValidateWorkerResponse enforces (request_id, generation) correlation.
// The orchestrator builds the typed request envelope with the
// correlation's ID; the worker response must echo it.
func ValidateWorkerResponse(workerRequestID string, workerGeneration uint64, want CorrelationID, stage Stage, expectedGen uint64, cm *CorrelationMap) error {
	if workerRequestID != string(want) {
		return ErrCorrelationMismatch
	}
	if workerGeneration != expectedGen {
		return ErrCorrelationMismatch
	}
	if !cm.CheckAndConsume(want, stage, expectedGen) {
		return ErrCorrelationMismatch
	}
	return nil
}

// StageDeadline returns a child context whose deadline is min(parent
// deadline, now+stageDeadline). When stageDeadline is zero, the
// parent deadline is preserved.
func StageDeadline(parent context.Context, stageDeadline time.Duration) (context.Context, context.CancelFunc) {
	if stageDeadline <= 0 {
		return context.WithCancel(parent)
	}
	d := time.Now().Add(stageDeadline)
	if dl, ok := parent.Deadline(); ok && dl.Before(d) {
		d = dl
	}
	return context.WithDeadline(parent, d)
}

// ScopedContextEnvelope is the orchestrator's view of the resolved
// ScopedContext plus its source/template versions. It exists so the
// revalidation step has the version tuple available without re-parsing
// the JSON.
type ScopedContextEnvelope struct {
	Context         contracts.ScopedContext
	ResolvedAt      time.Time
	SourceVersion   int
	TemplateVersion int
}

// IsStale reports whether the (source_version, template_version) of
// the envelope differ from the (SourceVersion, TemplateVersion) of
// the embedded ScopedContext. Used by tests and the validator chain.
func (e ScopedContextEnvelope) IsStale() bool {
	if e.SourceVersion != 0 && e.Context.SourceVersion != 0 && e.SourceVersion != e.Context.SourceVersion {
		return true
	}
	if e.TemplateVersion != 0 && e.Context.TemplateVersion != 0 && e.TemplateVersion != e.Context.TemplateVersion {
		return true
	}
	return false
}
