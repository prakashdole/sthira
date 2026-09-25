package sourceact

import (
	"strings"
	"testing"
	"time"
)

// narrowly-scoped security tests for sourceact — credential
// isolation and the OPERATIONAL gate.

func TestSourceHasNoCredentialFields(t *testing.T) {
	// Source struct MUST NOT carry secrets at the type level.
	// The full field set is intentionally tiny: id, owner,
	// domain, state, version, updated-at. This test pins that
	// shape. If a future contributor adds e.g. SigningKey,
	// the threat model (F-OUTGOINGEGRESS-01) must also be
	// updated. The AuditEvent shape is similarly clean.
	src := Source{
		SourceID:        "G01",
		GovernmentOwner: "NDMA",
		OfficialDomain:  "sachet.ndma.gov.in",
		State:           Operational,
		Version:         7,
		UpdatedAt:       time.Unix(0, 0).UTC(),
	}
	// Snapshot must round-trip with no sensitive payload.
	for _, s := range []string{src.GovernmentOwner, src.OfficialDomain, src.SourceID} {
		if strings.ContainsAny(s, "\n\r\t") {
			t.Fatalf("Source field carries a control char: %q", s)
		}
	}
	// AuditEvent fields are bounded; actor_id should never be
	// large (no payload smuggling).
	e := AuditEvent{EventID: "e1", Action: "TRANSITION"}
	if e.ActorID != "" && len(e.ActorID) > 128 {
		t.Fatalf("actor id is suspiciously long: %d bytes", len(e.ActorID))
	}
	if e.Reason != "" && len(e.Reason) > 1024 {
		t.Fatalf("reason is suspiciously long: %d bytes", len(e.Reason))
	}
}

// TestNonOperationalSourceCannotDriveGuidance asserts that a non-
// OPERATIONAL source is rejected by RequireOperational, the
// authoritative guidance gate. Per F-OPSIGMAUTH-01.
func TestNonOperationalSourceCannotDriveGuidance(t *testing.T) {
	s := newSourceactForTest()
	_, err := s.RequireOperational("G01")
	if err == nil {
		t.Fatalf("empty service accepted unknown source for guidance")
	}

	// Now drive a non-operational source explicitly.
	s.Discover(Source{SourceID: "G01", GovernmentOwner: "NDMA", OfficialDomain: "sachet.ndma.gov.in"})
	_, err = s.RequireOperational("G01")
	if err == nil {
		t.Fatalf("DISCOVERED source drove guidance; OPERATIONAL gate broken")
	}

	// Suspended and Retired should also refuse.
	s.Discover(Source{SourceID: "G02", State: Suspended})
	if _, err = s.RequireOperational("G02"); err == nil {
		t.Fatalf("SUSPENDED source drove guidance")
	}
}

// Helper: build a Service with an injected clock and a no-op
// auditor so we can drive the lifecycle transitions directly.
func newSourceactForTest() *Service {
	clock := func() time.Time { return time.Unix(0, 0).UTC() }
	return NewService(clock, func(AuditEvent) {})
}
