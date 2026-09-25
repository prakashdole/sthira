package capfeed

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Clock supplies the current time. Injected for deterministic tests.
type Clock func() time.Time

// QuarantineReason is a safe, stable code for a rejected artifact.
type QuarantineReason string

const (
	QuarantineParseFailure     QuarantineReason = "PARSE_FAILURE"
	QuarantineUnknownReference QuarantineReason = "UNKNOWN_REFERENCE"
	QuarantineStaleUpdate      QuarantineReason = "STALE_UPDATE"
	QuarantineNonOperational   QuarantineReason = "NON_OPERATIONAL"
)

// Quarantined records a rejected artifact with a safe reason code. Raw bytes
// are retained for audit but never surfaced to guidance.
type Quarantined struct {
	Reason    QuarantineReason
	Detail    string
	SHA256    string
	SourceURI string
	At        time.Time
}

// stored is one ingested alert version.
type stored struct {
	parsed *Parsed
	// versionKey orders revisions of the same identifier by their sent time.
	sent time.Time
}

// Lifecycle applies the CAP update/cancel/expiry lifecycle against an injected
// clock. It is safe for concurrent use. It holds no durable state; durable
// publication is P3.
type Lifecycle struct {
	clock Clock

	mu sync.Mutex
	// byID maps identifier -> ordered revisions (oldest first).
	byID map[string][]stored
	// state maps identifier -> current lifecycle state.
	state map[string]LifecycleState
	// quarantine lists rejected artifacts in arrival order.
	quarantine []Quarantined
}

// NewLifecycle returns a Lifecycle using the injected clock. The clock is
// required; passing nil panics to fail fast at construction.
func NewLifecycle(clock Clock) *Lifecycle {
	if clock == nil {
		panic("capfeed: injected clock is required")
	}
	return &Lifecycle{
		clock: clock,
		byID:  map[string][]stored{},
		state: map[string]LifecycleState{},
	}
}

// IngestResult reports what happened to one parsed alert.
type IngestResult struct {
	Identifier string
	State      LifecycleState
	// Applied reports whether the alert changed current state. A duplicate or
	// out-of-order (stale) update leaves state unchanged.
	Applied bool
}

// Ingest applies one already-parsed alert. Parse failures are quarantined by
// the caller via QuarantineParse; Ingest accepts only a valid *Parsed.
//
// Semantics (ported from the Python reference):
//   - A duplicate (same identifier, same sent) is a no-op (idempotent).
//   - An Update/Cancel must reference a known identifier; otherwise it is
//     quarantined as UNKNOWN_REFERENCE and not applied.
//   - An Update supersedes the prior revision of its referenced identifier.
//   - A Cancel cancels the referenced identifier.
//   - An out-of-order revision (sent not after the current latest) is
//     quarantined as STALE_UPDATE and not applied.
func (l *Lifecycle) Ingest(p *Parsed) (IngestResult, error) {
	if p == nil {
		return IngestResult{}, fail("cannot ingest nil parsed alert")
	}
	id := p.Alert.Identifier

	l.mu.Lock()
	defer l.mu.Unlock()

	switch p.Alert.MsgType {
	case MsgUpdate, MsgCancel:
		target := l.resolveReferenceLocked(p.Alert.References)
		if target == "" {
			l.quarantineLocked(QuarantineUnknownReference, p, "no known referenced alert")
			return IngestResult{Identifier: id, State: l.state[id], Applied: false}, nil
		}
		if l.isStaleLocked(target, p.Alert.Sent) {
			l.quarantineLocked(QuarantineStaleUpdate, p, "revision is not newer than current")
			return IngestResult{Identifier: id, State: l.state[target], Applied: false}, nil
		}
		l.appendRevisionLocked(target, p)
		if p.Alert.MsgType == MsgCancel {
			l.state[target] = StateCancelled
		} else {
			l.state[target] = StateSuperseded
		}
		return IngestResult{Identifier: target, State: l.state[target], Applied: true}, nil

	default: // MsgAlert and any other message type start a new alert.
		if l.isDuplicateLocked(id, p.Alert.Sent) {
			return IngestResult{Identifier: id, State: l.state[id], Applied: false}, nil
		}
		if l.isStaleLocked(id, p.Alert.Sent) {
			l.quarantineLocked(QuarantineStaleUpdate, p, "revision is not newer than current")
			return IngestResult{Identifier: id, State: l.state[id], Applied: false}, nil
		}
		l.appendRevisionLocked(id, p)
		l.state[id] = StateActive
		return IngestResult{Identifier: id, State: StateActive, Applied: true}, nil
	}
}

// QuarantineParse records a parse failure with a safe reason code. The raw
// digest (not content) is retained for audit correlation.
func (l *Lifecycle) QuarantineParse(sourceURI, sha256Hex, detail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.quarantine = append(l.quarantine, Quarantined{
		Reason:    QuarantineParseFailure,
		Detail:    detail,
		SHA256:    sha256Hex,
		SourceURI: sourceURI,
		At:        l.clock().UTC(),
	})
}

// Expire marks alerts whose expiry has passed (relative to the injected clock)
// as EXPIRED. It returns the identifiers that transitioned. Expired alerts are
// excluded from Active; the transition is recorded, not deleted.
func (l *Lifecycle) Expire() []string {
	now := l.clock().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	var expired []string
	for id, revs := range l.byID {
		if l.state[id] != StateActive {
			continue
		}
		latest := revs[len(revs)-1].parsed
		if !latest.Alert.Expires.After(now) {
			l.state[id] = StateExpired
			expired = append(expired, id)
		}
	}
	sort.Strings(expired)
	return expired
}

// Active returns the identifiers currently in ACTIVE state, sorted. Expired,
// superseded and cancelled alerts are excluded.
func (l *Lifecycle) Active() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.state))
	for id, st := range l.state {
		if st == StateActive {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// State returns the lifecycle state for an identifier and whether it is known.
func (l *Lifecycle) State(id string) (LifecycleState, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.state[id]
	return st, ok
}

// Latest returns the newest applied revision for an identifier.
func (l *Lifecycle) Latest(id string) (*Parsed, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	revs, ok := l.byID[id]
	if !ok || len(revs) == 0 {
		return nil, false
	}
	return revs[len(revs)-1].parsed, true
}

// Quarantine returns a copy of the quarantine list in arrival order.
func (l *Lifecycle) Quarantine() []Quarantined {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Quarantined(nil), l.quarantine...)
}

// resolveReferenceLocked maps an update/cancel references list to a known
// identifier. CAP references are "sender,identifier,sent" triples; we match on
// the identifier component, falling back to a bare identifier.
func (l *Lifecycle) resolveReferenceLocked(refs []string) string {
	for _, ref := range refs {
		candidate := ref
		if parts := strings.Split(ref, ","); len(parts) >= 2 {
			candidate = strings.TrimSpace(parts[1])
		}
		candidate = strings.TrimSpace(candidate)
		if _, ok := l.byID[candidate]; ok {
			return candidate
		}
	}
	return ""
}

func (l *Lifecycle) isDuplicateLocked(id string, sent time.Time) bool {
	for _, r := range l.byID[id] {
		if r.sent.Equal(sent) {
			return true
		}
	}
	return false
}

// isStaleLocked reports whether a revision with the given sent time is not
// newer than the current latest revision for id. Unknown ids are not stale.
func (l *Lifecycle) isStaleLocked(id string, sent time.Time) bool {
	revs, ok := l.byID[id]
	if !ok || len(revs) == 0 {
		return false
	}
	return !sent.After(revs[len(revs)-1].sent)
}

func (l *Lifecycle) appendRevisionLocked(id string, p *Parsed) {
	l.byID[id] = append(l.byID[id], stored{parsed: p, sent: p.Alert.Sent})
	// Keep revisions ordered by sent so Latest is the newest.
	revs := l.byID[id]
	sort.SliceStable(revs, func(i, j int) bool { return revs[i].sent.Before(revs[j].sent) })
}

func (l *Lifecycle) quarantineLocked(reason QuarantineReason, p *Parsed, detail string) {
	l.quarantine = append(l.quarantine, Quarantined{
		Reason: reason,
		Detail: fmt.Sprintf("%s (identifier %q)", detail, p.Alert.Identifier),
		SHA256: p.SHA256,
		At:     l.clock().UTC(),
	})
}
