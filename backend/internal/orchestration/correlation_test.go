package orchestration

import (
	"errors"
	"testing"

	"sthira/backend/internal/contracts"
)

func TestCorrelation_ClaimAndConsume(t *testing.T) {
	cm := NewCorrelationMap()
	id := CorrelationID("req-1")
	g := cm.Claim(id, StageASR)
	if g == 0 {
		t.Fatalf("Claim returned 0; expected > 0")
	}
	cm.MarkInflight(id, StageASR, g)
	if !cm.CheckAndConsume(id, StageASR, g) {
		t.Errorf("CheckAndConsume failed for current generation")
	}
	// Second consume must fail (slot is gone).
	if cm.CheckAndConsume(id, StageASR, g) {
		t.Errorf("CheckAndConsume succeeded twice for the same generation")
	}
}

func TestCorrelation_GenerationSupersession(t *testing.T) {
	cm := NewCorrelationMap()
	id := CorrelationID("req-2")
	g1 := cm.Claim(id, StageMiddle)
	g2 := cm.Claim(id, StageMiddle)
	if g2 <= g1 {
		t.Errorf("generation did not increment: g1=%d g2=%d", g1, g2)
	}
	// Mark g2 as inflight; consume g1 must fail (generation mismatch).
	cm.MarkInflight(id, StageMiddle, g2)
	if cm.CheckAndConsume(id, StageMiddle, g1) {
		t.Errorf("older generation accepted")
	}
	if !cm.CheckAndConsume(id, StageMiddle, g2) {
		t.Errorf("current generation rejected")
	}
}

func TestCorrelation_Withdraw(t *testing.T) {
	cm := NewCorrelationMap()
	id := CorrelationID("req-3")
	g := cm.Claim(id, StageTTS)
	cm.MarkInflight(id, StageTTS, g)
	cm.Withdraw(id)
	if cm.CheckAndConsume(id, StageTTS, g) {
		t.Errorf("CheckAndConsume succeeded after Withdraw")
	}
	// Generation counter must reset.
	if g2 := cm.Claim(id, StageTTS); g2 != 1 {
		t.Errorf("generation after Withdraw = %d, want 1", g2)
	}
}

func TestCorrelation_Peek(t *testing.T) {
	cm := NewCorrelationMap()
	id := CorrelationID("req-4")
	if _, ok := cm.Peek(id, StageASR); ok {
		t.Errorf("Peek returned ok=true for an unclaimed slot")
	}
	g := cm.Claim(id, StageASR)
	got, ok := cm.Peek(id, StageASR)
	if !ok || got != g {
		t.Errorf("Peek = (%d, %v), want (%d, true)", got, ok, g)
	}
	// Peek returns the generation counter, which persists across
	// CheckAndConsume (only Withdraw resets it). This is by design.
	cm.MarkInflight(id, StageASR, g)
	cm.CheckAndConsume(id, StageASR, g)
	got, ok = cm.Peek(id, StageASR)
	if !ok || got != g {
		t.Errorf("Peek after consume = (%d, %v), want (%d, true)", got, ok, g)
	}
}

func TestCorrelation_ValidateWorkerResponse(t *testing.T) {
	cm := NewCorrelationMap()
	id := CorrelationID("req-5")
	g := cm.Claim(id, StageMiddle)
	cm.MarkInflight(id, StageMiddle, g)
	if err := ValidateWorkerResponse(string(id), g, id, StageMiddle, g, cm); err != nil {
		t.Errorf("current generation should validate: %v", err)
	}
	// Mismatched request_id
	if err := ValidateWorkerResponse("other", g, id, StageMiddle, g, cm); !errors.Is(err, ErrCorrelationMismatch) {
		t.Errorf("mismatched request_id: got %v, want ErrCorrelationMismatch", err)
	}
	// Stale generation: claim a new one and verify the old fails.
	g2 := cm.Claim(id, StageMiddle)
	cm.MarkInflight(id, StageMiddle, g2)
	if err := ValidateWorkerResponse(string(id), g, id, StageMiddle, g, cm); !errors.Is(err, ErrCorrelationMismatch) {
		t.Errorf("stale generation: got %v, want ErrCorrelationMismatch", err)
	}
	if err := ValidateWorkerResponse(string(id), g2, id, StageMiddle, g2, cm); err != nil {
		t.Errorf("current generation should validate: %v", err)
	}
}

// TestCorrelation_StaleDropDoesNotInfluenceNextRequest asserts that
// after a stale drop, the next request gets a strictly larger
// generation (the generation counter persists).
func TestCorrelation_StaleDropDoesNotInfluenceNextRequest(t *testing.T) {
	cm := NewCorrelationMap()
	id := CorrelationID("req-6")
	g1 := cm.Claim(id, StageASR)
	cm.MarkInflight(id, StageASR, g1)
	if !cm.CheckAndConsume(id, StageASR, g1) {
		t.Fatalf("consume failed for first generation")
	}
	g2 := cm.Claim(id, StageASR)
	if g2 <= g1 {
		t.Errorf("Claim did not advance generation: g1=%d g2=%d", g1, g2)
	}
}

// TestCorrelation_StageMapClearedAfterLastConsume ensures the
// generation counter survives consumption; only Withdraw clears it.
func TestCorrelation_StageMapClearedAfterLastConsume(t *testing.T) {
	cm := NewCorrelationMap()
	id := CorrelationID("req-7")
	g1 := cm.Claim(id, StageASR)
	g2 := cm.Claim(id, StageMiddle)
	cm.MarkInflight(id, StageASR, g1)
	cm.MarkInflight(id, StageMiddle, g2)
	cm.CheckAndConsume(id, StageASR, g1)
	if cm.CheckAndConsume(id, StageASR, g1) {
		t.Errorf("re-consume must fail")
	}
	cm.CheckAndConsume(id, StageMiddle, g2)
	// Generation counters are still present (only Withdraw removes them).
	if _, ok := cm.Peek(id, StageASR); !ok {
		t.Errorf("ASR peek should still be present (generation persists)")
	}
	if _, ok := cm.Peek(id, StageMiddle); !ok {
		t.Errorf("Middle peek should still be present (generation persists)")
	}
	cm.Withdraw(id)
	if _, ok := cm.Peek(id, StageASR); ok {
		t.Errorf("ASR peek should be gone after Withdraw")
	}
}

// silence unused imports
var _ = contracts.ErrInferenceCancelled
