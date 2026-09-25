package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrAuditChainBroken indicates a cryptographic hash chain verification failure.
	ErrAuditChainBroken = errors.New("audit log chain broken: hash mismatch")
	// ErrAuditSeqMismatch indicates non-monotonic event sequencing.
	ErrAuditSeqMismatch = errors.New("audit log sequence mismatch")
	// ErrInvalidAuditEvent indicates required audit fields are missing.
	ErrInvalidAuditEvent = errors.New("invalid audit event: missing required fields")
)

// SecurityAuditEvent represents one immutable security-relevant action recorded
// at the HTTP boundary. The hash chain links each event to its predecessor,
// guaranteeing tamper-evidence.
type SecurityAuditEvent struct {
	Seq          uint64    `json:"seq"`
	Timestamp    time.Time `json:"timestamp"`
	EventType    string    `json:"event_type"`
	RequestID    string    `json:"request_id"`
	Actor        string    `json:"actor"`        // e.g. "anonymous", "operator", or truncated session ref
	Jurisdiction string    `json:"jurisdiction"` // e.g. "IN-KL" or "none"
	Action       string    `json:"action"`
	Status       int       `json:"status"`
	PrevHash     string    `json:"prev_hash"`
	EventHash    string    `json:"event_hash"`
}

// SecurityAuditor maintains an in-memory, thread-safe, cryptographically chained
// security audit trail of sensitive HTTP boundary events.
type SecurityAuditor struct {
	mu     sync.RWMutex
	events []SecurityAuditEvent
	tail   string
}

// NewSecurityAuditor initializes a new auditor with a genesis hash.
func NewSecurityAuditor() *SecurityAuditor {
	return &SecurityAuditor{
		tail: "genesis",
	}
}

// Record appends a new audit event, automatically assigning monotonic sequence
// and calculating the SHA-256 event hash linked to the previous event.
func (a *SecurityAuditor) Record(eventType, reqID, actor, jurisdiction, action string, status int) (SecurityAuditEvent, error) {
	if eventType == "" || action == "" {
		return SecurityAuditEvent{}, ErrInvalidAuditEvent
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	seq := uint64(len(a.events) + 1)
	now := time.Now().UTC()
	prev := a.tail

	ev := SecurityAuditEvent{
		Seq:          seq,
		Timestamp:    now,
		EventType:    eventType,
		RequestID:    reqID,
		Actor:        actor,
		Jurisdiction: jurisdiction,
		Action:       action,
		Status:       status,
		PrevHash:     prev,
	}

	ev.EventHash = computeSecurityHash(prev, ev)
	a.tail = ev.EventHash
	a.events = append(a.events, ev)

	return ev, nil
}

// Verify validates the entire cryptographic hash chain from genesis to tail.
func (a *SecurityAuditor) Verify() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	expectedPrev := "genesis"
	for i, ev := range a.events {
		if ev.Seq != uint64(i+1) {
			return fmt.Errorf("%w: at index %d expected seq %d, got %d", ErrAuditSeqMismatch, i, i+1, ev.Seq)
		}
		if ev.PrevHash != expectedPrev {
			return fmt.Errorf("%w: at seq %d expected prev %s, got %s", ErrAuditChainBroken, ev.Seq, expectedPrev, ev.PrevHash)
		}
		computed := computeSecurityHash(ev.PrevHash, ev)
		if ev.EventHash != computed {
			return fmt.Errorf("%w: at seq %d expected hash %s, got %s", ErrAuditChainBroken, ev.Seq, computed, ev.EventHash)
		}
		expectedPrev = ev.EventHash
	}
	return nil
}

// Events returns a copy of the recorded events.
func (a *SecurityAuditor) Events() []SecurityAuditEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()
	res := make([]SecurityAuditEvent, len(a.events))
	copy(res, a.events)
	return res
}

// Len returns the number of recorded events.
func (a *SecurityAuditor) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.events)
}

// LastHash returns the current tail hash.
func (a *SecurityAuditor) LastHash() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tail
}

func computeSecurityHash(prev string, ev SecurityAuditEvent) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%d|%d|%s|%s|%s|%s|%s|%d",
		prev,
		ev.Seq,
		ev.Timestamp.UnixNano(),
		ev.EventType,
		ev.RequestID,
		ev.Actor,
		ev.Jurisdiction,
		ev.Action,
		ev.Status,
	)
	return hex.EncodeToString(h.Sum(nil))
}
