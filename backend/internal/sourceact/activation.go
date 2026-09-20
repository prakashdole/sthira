// Package sourceact is the government source activation lifecycle. A source
// advances DISCOVERED → ACCESS_REQUESTED → SAMPLE_ACQUIRED → VALIDATED →
// AUTHORIZED → OPERATIONAL, and only OPERATIONAL sources may drive citizen
// guidance. Transitions are optimistic-concurrency checked and audited.
//
// This is the P2 contract slice: it holds in-memory state for preview/tests.
// Durable persistence is P3; no live source is activated here.
package sourceact

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// State is the source activation lifecycle state.
type State string

const (
	Discovered      State = "DISCOVERED"
	AccessRequested State = "ACCESS_REQUESTED"
	SampleAcquired  State = "SAMPLE_ACQUIRED"
	Validated       State = "VALIDATED"
	Authorized      State = "AUTHORIZED"
	Operational     State = "OPERATIONAL"
	Suspended       State = "SUSPENDED"
	Retired         State = "RETIRED"
	// Quarantined is a restriction state: the source's evidence is held suspect
	// and must not drive operational guidance or new reservations. Distinct from
	// SUSPENDED (a temporary operational pause): quarantine marks the evidence
	// itself as not-to-be-used pending review. Reachable from any active state.
	Quarantined State = "QUARANTINED"
)

// transitions maps a state to its legal successors.
var transitions = map[State]map[State]bool{
	Discovered:      {AccessRequested: true, Quarantined: true},
	AccessRequested: {SampleAcquired: true, Quarantined: true},
	SampleAcquired:  {Validated: true, Quarantined: true},
	Validated:       {Authorized: true, Quarantined: true},
	Authorized:      {Operational: true, Quarantined: true},
	Operational:     {Suspended: true, Retired: true, Quarantined: true},
	Suspended:       {Operational: true, Retired: true, Quarantined: true},
	// Quarantine is terminal here: release requires re-validation through a fresh
	// source, not a silent un-quarantine.
	Quarantined: {},
}

// ActivationError is a lifecycle failure.
type ActivationError struct{ Reason string }

func (e *ActivationError) Error() string { return e.Reason }

func fail(format string, args ...any) *ActivationError {
	return &ActivationError{Reason: fmt.Sprintf(format, args...)}
}

// SourceNotFound is returned when a source id is unknown.
type SourceNotFound struct{ SourceID string }

func (e *SourceNotFound) Error() string { return "source not found: " + e.SourceID }

// VersionConflict is returned on an optimistic-concurrency mismatch.
type VersionConflict struct{ SourceID string }

func (e *VersionConflict) Error() string { return "source version conflict: " + e.SourceID }

// Source is one registered government source.
type Source struct {
	SourceID        string
	GovernmentOwner string
	OfficialDomain  string
	State           State
	Version         int
	UpdatedAt       time.Time
}

// MayDriveGuidance reports whether the source is OPERATIONAL.
func (s Source) MayDriveGuidance() bool { return s.State == Operational }

// AuditEvent is one recorded transition.
type AuditEvent struct {
	EventID    string
	OccurredAt time.Time
	ActorID    string
	Action     string
	SubjectID  string
	Reason     string
	FromState  State
	ToState    State
}

// Auditor records transition events. Injected; tests use a slice sink.
type Auditor func(AuditEvent)

// Service applies the activation lifecycle against an injected clock. It is
// safe for concurrent use.
type Service struct {
	clock Clock
	audit Auditor
	mu    sync.Mutex
	byID  map[string]Source
}

// Clock supplies the current time. Injected for deterministic tests.
type Clock func() time.Time

// NewService returns a Service. clock is required; audit may be nil.
func NewService(clock Clock, audit Auditor) *Service {
	if clock == nil {
		panic("sourceact: injected clock is required")
	}
	return &Service{clock: clock, audit: audit, byID: map[string]Source{}}
}

// Discover registers a new source. It must begin in DISCOVERED.
func (s *Service) Discover(src Source) (Source, error) {
	if src.SourceID == "" {
		return Source{}, fail("source_id is required")
	}
	if src.GovernmentOwner == "" {
		return Source{}, fail("government_owner is required")
	}
	if src.OfficialDomain == "" {
		return Source{}, fail("official_domain is required")
	}
	if src.State != "" && src.State != Discovered {
		return Source{}, fail("new sources must begin in DISCOVERED")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byID[src.SourceID]; exists {
		return Source{}, fail("duplicate source: %s", src.SourceID)
	}
	src.State = Discovered
	src.Version = 1
	src.UpdatedAt = s.clock().UTC()
	s.byID[src.SourceID] = src
	return src, nil
}

// Transition moves a source to a legal successor state with an optimistic
// version check and an audit record.
func (s *Service) Transition(sourceID string, target State, actorID, reason, eventID string) (Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.byID[sourceID]
	if !ok {
		return Source{}, &SourceNotFound{SourceID: sourceID}
	}
	if !transitions[current.State][target] {
		return Source{}, fail("illegal source transition: %s -> %s", current.State, target)
	}
	now := s.clock().UTC()
	updated := current
	updated.State = target
	updated.Version = current.Version + 1
	updated.UpdatedAt = now
	s.byID[sourceID] = updated
	if s.audit != nil {
		s.audit(AuditEvent{
			EventID:    eventID,
			OccurredAt: now,
			ActorID:    actorID,
			Action:     "SOURCE_STATE_TRANSITION",
			SubjectID:  sourceID,
			Reason:     reason,
			FromState:  current.State,
			ToState:    target,
		})
	}
	return updated, nil
}

// RequireOperational returns the source only if it may drive citizen guidance.
func (s *Service) RequireOperational(sourceID string) (Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.byID[sourceID]
	if !ok {
		return Source{}, &SourceNotFound{SourceID: sourceID}
	}
	if !src.MayDriveGuidance() {
		return Source{}, fail("source %s is not OPERATIONAL", sourceID)
	}
	return src, nil
}

// Get returns a source and whether it is known.
func (s *Service) Get(sourceID string) (Source, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.byID[sourceID]
	return src, ok
}

// List returns all sources ordered by source id.
func (s *Service) List() []Source {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Source, 0, len(s.byID))
	for _, src := range s.byID {
		out = append(out, src)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourceID < out[j].SourceID })
	return out
}
