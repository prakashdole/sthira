package httpserver

import (
	"sync"
	"testing"
)

func TestSecurityAuditor_RecordAndVerify(t *testing.T) {
	auditor := NewSecurityAuditor()

	ev1, err := auditor.Record("AUTH_LOGIN", "req-1", "operator-1", "IN-KL", "issue_session", 200)
	if err != nil {
		t.Fatalf("Record ev1: %v", err)
	}
	if ev1.Seq != 1 {
		t.Fatalf("expected seq 1, got %d", ev1.Seq)
	}
	if ev1.PrevHash != "genesis" {
		t.Fatalf("expected prev_hash genesis, got %s", ev1.PrevHash)
	}

	ev2, err := auditor.Record("SOURCE_QUARANTINE", "req-2", "operator-1", "IN-KL", "quarantine_source", 200)
	if err != nil {
		t.Fatalf("Record ev2: %v", err)
	}
	if ev2.Seq != 2 {
		t.Fatalf("expected seq 2, got %d", ev2.Seq)
	}
	if ev2.PrevHash != ev1.EventHash {
		t.Fatalf("expected prev_hash %s, got %s", ev1.EventHash, ev2.PrevHash)
	}

	if err := auditor.Verify(); err != nil {
		t.Fatalf("expected valid chain, got: %v", err)
	}
	if auditor.Len() != 2 {
		t.Fatalf("expected len 2, got %d", auditor.Len())
	}
}

func TestSecurityAuditor_TamperDetection(t *testing.T) {
	auditor := NewSecurityAuditor()

	_, _ = auditor.Record("EVENT_1", "r1", "act1", "J1", "action1", 200)
	_, _ = auditor.Record("EVENT_2", "r2", "act2", "J1", "action2", 200)
	_, _ = auditor.Record("EVENT_3", "r3", "act3", "J1", "action3", 200)

	if err := auditor.Verify(); err != nil {
		t.Fatalf("expected clean chain, got: %v", err)
	}

	// Tamper with event 2 in place
	auditor.mu.Lock()
	auditor.events[1].Action = "tampered_action"
	auditor.mu.Unlock()

	if err := auditor.Verify(); err == nil {
		t.Fatalf("expected chain verification to fail after payload tampering")
	}
}

func TestSecurityAuditor_EmptyOrInvalidEventRejected(t *testing.T) {
	auditor := NewSecurityAuditor()

	if _, err := auditor.Record("", "r1", "act", "J1", "act", 200); err != ErrInvalidAuditEvent {
		t.Fatalf("expected ErrInvalidAuditEvent for empty eventType, got %v", err)
	}
	if _, err := auditor.Record("TYPE", "r1", "act", "J1", "", 200); err != ErrInvalidAuditEvent {
		t.Fatalf("expected ErrInvalidAuditEvent for empty action, got %v", err)
	}
}

func TestSecurityAuditor_ConcurrentRecording(t *testing.T) {
	auditor := NewSecurityAuditor()
	const count = 100
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = auditor.Record("EVENT", "req", "actor", "J1", "action", 200)
		}(i)
	}
	wg.Wait()

	if auditor.Len() != count {
		t.Fatalf("expected %d events, got %d", count, auditor.Len())
	}
	if err := auditor.Verify(); err != nil {
		t.Fatalf("expected intact chain under concurrency, got: %v", err)
	}
}
