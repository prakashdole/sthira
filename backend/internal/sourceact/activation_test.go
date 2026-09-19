package sourceact

import (
	"strings"
	"testing"
	"time"
)

var start = time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

func newService() (*Service, *[]AuditEvent) {
	events := &[]AuditEvent{}
	svc := NewService(func() time.Time { return start }, func(e AuditEvent) { *events = append(*events, e) })
	return svc, events
}

func validSource() Source {
	return Source{SourceID: "imd", GovernmentOwner: "India Meteorological Department", OfficialDomain: "mausam.imd.gov.in"}
}

// advanceToOperational walks a source through the legal chain.
func advanceToOperational(t *testing.T, svc *Service, id string) {
	t.Helper()
	chain := []State{AccessRequested, SampleAcquired, Validated, Authorized, Operational}
	for _, target := range chain {
		if _, err := svc.Transition(id, target, "tester", "advance", "evt-"+string(target)); err != nil {
			t.Fatalf("transition to %s: %v", target, err)
		}
	}
}

func TestDiscoverBeginsDiscovered(t *testing.T) {
	svc, _ := newService()
	src, err := svc.Discover(validSource())
	if err != nil {
		t.Fatal(err)
	}
	if src.State != Discovered || src.Version != 1 {
		t.Errorf("src = %+v", src)
	}
}

func TestDiscoverRejectsNonDiscoveredStart(t *testing.T) {
	svc, _ := newService()
	s := validSource()
	s.State = Operational
	if _, err := svc.Discover(s); err == nil {
		t.Error("must reject non-DISCOVERED start")
	}
}

func TestDiscoverRejectsDuplicate(t *testing.T) {
	svc, _ := newService()
	if _, err := svc.Discover(validSource()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Discover(validSource()); err == nil {
		t.Error("must reject duplicate source")
	}
}

func TestDiscoverRequiresFields(t *testing.T) {
	svc, _ := newService()
	if _, err := svc.Discover(Source{}); err == nil {
		t.Error("must reject empty source")
	}
}

func TestLegalChainToOperational(t *testing.T) {
	svc, events := newService()
	if _, err := svc.Discover(validSource()); err != nil {
		t.Fatal(err)
	}
	advanceToOperational(t, svc, "imd")
	src, err := svc.RequireOperational("imd")
	if err != nil {
		t.Fatal(err)
	}
	if !src.MayDriveGuidance() {
		t.Error("OPERATIONAL source must drive guidance")
	}
	// Each transition audited.
	if len(*events) != 5 {
		t.Errorf("audit events = %d, want 5", len(*events))
	}
}

func TestIllegalTransitionRejected(t *testing.T) {
	svc, _ := newService()
	if _, err := svc.Discover(validSource()); err != nil {
		t.Fatal(err)
	}
	// DISCOVERED -> OPERATIONAL skips the chain.
	if _, err := svc.Transition("imd", Operational, "tester", "skip", "e1"); err == nil {
		t.Error("must reject DISCOVERED -> OPERATIONAL")
	} else if !strings.Contains(err.Error(), "illegal source transition") {
		t.Errorf("err = %v", err)
	}
}

func TestRequireOperationalBlocksNonOperational(t *testing.T) {
	svc, _ := newService()
	if _, err := svc.Discover(validSource()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequireOperational("imd"); err == nil {
		t.Error("DISCOVERED source must not drive guidance")
	}
}

func TestOperationalToSuspendedAndBack(t *testing.T) {
	svc, _ := newService()
	if _, err := svc.Discover(validSource()); err != nil {
		t.Fatal(err)
	}
	advanceToOperational(t, svc, "imd")
	if _, err := svc.Transition("imd", Suspended, "tester", "outage", "e6"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequireOperational("imd"); err == nil {
		t.Error("SUSPENDED must not drive guidance")
	}
	if _, err := svc.Transition("imd", Operational, "tester", "restored", "e7"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequireOperational("imd"); err != nil {
		t.Error("restored source must drive guidance")
	}
}

func TestRetiredIsTerminal(t *testing.T) {
	svc, _ := newService()
	if _, err := svc.Discover(validSource()); err != nil {
		t.Fatal(err)
	}
	advanceToOperational(t, svc, "imd")
	if _, err := svc.Transition("imd", Retired, "tester", "decommission", "e6"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Transition("imd", Operational, "tester", "revive", "e7"); err == nil {
		t.Error("RETIRED must be terminal")
	}
}

func TestTransitionUnknownSource(t *testing.T) {
	svc, _ := newService()
	if _, err := svc.Transition("nope", AccessRequested, "t", "r", "e"); err == nil {
		t.Error("want error for unknown source")
	} else if _, ok := err.(*SourceNotFound); !ok {
		t.Errorf("err type = %T", err)
	}
}

func TestVersionIncrementsOnTransition(t *testing.T) {
	svc, _ := newService()
	if _, err := svc.Discover(validSource()); err != nil {
		t.Fatal(err)
	}
	src, err := svc.Transition("imd", AccessRequested, "t", "r", "e")
	if err != nil {
		t.Fatal(err)
	}
	if src.Version != 2 {
		t.Errorf("version = %d, want 2", src.Version)
	}
}

func TestNilClockPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want panic on nil clock")
		}
	}()
	NewService(nil, nil)
}
