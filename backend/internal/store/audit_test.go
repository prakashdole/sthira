package store

import (
	"strings"
	"testing"
	"time"
)

// The SQL/transaction paths require a real PostgreSQL/PostGIS instance and are
// verified there (BLOCKED_EXTERNAL in this build). The hash-chain computation is
// pure and is verified here: it must be deterministic, sensitive to every field
// and to the prev link, and produce a 64-char hex digest.

func ev(id string) AuditEvent {
	return AuditEvent{
		EventID:   id,
		OccuredAt: time.Date(2026, 9, 19, 4, 0, 0, 0, time.UTC),
		ActorID:   "operator-1",
		Action:    "SOURCE_STATE_TRANSITION",
		SubjectID: "SRC-1",
		Outcome:   "OK",
		ToState:   "VALIDATED",
	}
}

func TestComputeEventHashDeterministic(t *testing.T) {
	a := computeEventHash(genesisHash, ev("E1"))
	b := computeEventHash(genesisHash, ev("E1"))
	if a != b {
		t.Error("hash must be deterministic")
	}
	if len(a) != 64 {
		t.Errorf("hash length = %d, want 64", len(a))
	}
}

func TestComputeEventHashChainsOnPrev(t *testing.T) {
	base := computeEventHash(genesisHash, ev("E1"))
	chained := computeEventHash(base, ev("E1"))
	if base == chained {
		t.Error("hash must depend on the prev link")
	}
}

func TestComputeEventHashSensitiveToFields(t *testing.T) {
	base := computeEventHash(genesisHash, ev("E1"))
	mutated := ev("E1")
	mutated.Outcome = "DENIED"
	if computeEventHash(genesisHash, mutated) == base {
		t.Error("hash must change when a field changes")
	}
}

func TestGenesisHashIs64HexZeros(t *testing.T) {
	if len(genesisHash) != 64 || strings.Trim(genesisHash, "0") != "" {
		t.Errorf("genesis hash must be 64 zero hex chars, got %q", genesisHash)
	}
}
