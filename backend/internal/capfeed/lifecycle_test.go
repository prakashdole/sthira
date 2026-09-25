package capfeed

import (
	"testing"
	"time"
)

// mutableClock is a controllable injected clock.
type mutableClock struct{ now time.Time }

func (m *mutableClock) Clock() time.Time { return m.now }
func (m *mutableClock) Advance(d time.Duration) {
	m.now = m.now.Add(d)
}

func newTestLifecycle(start time.Time) (*Lifecycle, *mutableClock) {
	mc := &mutableClock{now: start}
	return NewLifecycle(mc.Clock), mc
}

func mustParse(t *testing.T, f capFields, retrievedAt time.Time) *Parsed {
	t.Helper()
	p, err := Parse([]byte(f.xml()), ParseOptions{
		SourceURI:   "fixture:cap",
		RetrievedAt: retrievedAt,
	})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return p
}

func TestLifecycleIngestAlertActive(t *testing.T) {
	lc, _ := newTestLifecycle(fixedNow)
	res, err := lc.Ingest(mustParse(t, baseFields(), fixedNow))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if !res.Applied || res.State != StateActive {
		t.Errorf("res = %+v", res)
	}
	if got := lc.Active(); len(got) != 1 || got[0] != "ALERT-1" {
		t.Errorf("Active = %v", got)
	}
}

func TestLifecycleDuplicateIsNoOp(t *testing.T) {
	lc, _ := newTestLifecycle(fixedNow)
	p := mustParse(t, baseFields(), fixedNow)
	if _, err := lc.Ingest(p); err != nil {
		t.Fatal(err)
	}
	res, err := lc.Ingest(p) // same identifier, same sent
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied {
		t.Error("duplicate should not be applied")
	}
	if got := lc.Active(); len(got) != 1 {
		t.Errorf("Active = %v, want 1", got)
	}
}

func TestLifecycleUpdateSupersedes(t *testing.T) {
	lc, _ := newTestLifecycle(fixedNow)
	base := baseFields()
	if _, err := lc.Ingest(mustParse(t, base, fixedNow)); err != nil {
		t.Fatal(err)
	}

	upd := base
	upd.Identifier = "ALERT-2"
	upd.MsgType = "Update"
	upd.Sent = "2026-09-12T05:00:00Z"
	upd.References = "synthetic.ndma.example,ALERT-1,2026-09-12T04:00:00Z"
	res, err := lc.Ingest(mustParse(t, upd, fixedNow))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied || res.State != StateSuperseded {
		t.Errorf("res = %+v, want SUPERSEDED", res)
	}
	if st, _ := lc.State("ALERT-1"); st != StateSuperseded {
		t.Errorf("ALERT-1 state = %q", st)
	}
	if got := lc.Active(); len(got) != 0 {
		t.Errorf("Active = %v, want empty after supersede", got)
	}
}

func TestLifecycleCancelCancels(t *testing.T) {
	lc, _ := newTestLifecycle(fixedNow)
	base := baseFields()
	if _, err := lc.Ingest(mustParse(t, base, fixedNow)); err != nil {
		t.Fatal(err)
	}
	cancel := base
	cancel.Identifier = "ALERT-C"
	cancel.MsgType = "Cancel"
	cancel.Sent = "2026-09-12T06:00:00Z"
	cancel.References = "ALERT-1"
	res, err := lc.Ingest(mustParse(t, cancel, fixedNow))
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateCancelled {
		t.Errorf("state = %q, want CANCELLED", res.State)
	}
	if got := lc.Active(); len(got) != 0 {
		t.Errorf("Active = %v", got)
	}
}

func TestLifecycleUnknownReferenceQuarantined(t *testing.T) {
	lc, _ := newTestLifecycle(fixedNow)
	upd := baseFields()
	upd.Identifier = "ALERT-9"
	upd.MsgType = "Update"
	upd.References = "sender,NOPE-1,2026-09-12T04:00:00Z"
	res, err := lc.Ingest(mustParse(t, upd, fixedNow))
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied {
		t.Error("unknown reference must not apply")
	}
	q := lc.Quarantine()
	if len(q) != 1 || q[0].Reason != QuarantineUnknownReference {
		t.Errorf("quarantine = %+v", q)
	}
}

func TestLifecycleOutOfOrderStaleQuarantined(t *testing.T) {
	lc, _ := newTestLifecycle(fixedNow)
	base := baseFields()
	if _, err := lc.Ingest(mustParse(t, base, fixedNow)); err != nil {
		t.Fatal(err)
	}
	// A newer revision arrives first.
	newer := base
	newer.Identifier = "ALERT-1"
	newer.Sent = "2026-09-12T07:00:00Z"
	if _, err := lc.Ingest(mustParse(t, newer, fixedNow)); err != nil {
		t.Fatal(err)
	}
	// An older revision arrives late (out of order) -> stale, not applied.
	older := base
	older.Sent = "2026-09-12T05:00:00Z"
	res, err := lc.Ingest(mustParse(t, older, fixedNow))
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied {
		t.Error("stale revision must not apply")
	}
	// Latest must remain the 07:00 revision.
	latest, ok := lc.Latest("ALERT-1")
	if !ok {
		t.Fatal("no latest")
	}
	if latest.Alert.Sent.Format(time.RFC3339) != "2026-09-12T07:00:00Z" {
		t.Errorf("latest sent = %v", latest.Alert.Sent)
	}
	q := lc.Quarantine()
	if len(q) == 0 || q[len(q)-1].Reason != QuarantineStaleUpdate {
		t.Errorf("quarantine = %+v", q)
	}
}

func TestLifecycleExpireExcludesExpired(t *testing.T) {
	start := fixedNow
	lc, mc := newTestLifecycle(start)
	// Expires 2026-09-13T04:00:00Z.
	if _, err := lc.Ingest(mustParse(t, baseFields(), start)); err != nil {
		t.Fatal(err)
	}
	if got := lc.Active(); len(got) != 1 {
		t.Fatalf("Active before expiry = %v", got)
	}
	// Advance the injected clock past expiry.
	mc.Advance(17 * time.Hour) // now 2026-09-13T05:00:00Z
	expired := lc.Expire()
	if len(expired) != 1 || expired[0] != "ALERT-1" {
		t.Errorf("expired = %v", expired)
	}
	// Expired alerts are excluded from Active.
	if got := lc.Active(); len(got) != 0 {
		t.Errorf("Active after expiry = %v, want empty", got)
	}
	if st, _ := lc.State("ALERT-1"); st != StateExpired {
		t.Errorf("state = %q, want EXPIRED", st)
	}
}

func TestLifecycleNotExpiredBeforeTime(t *testing.T) {
	lc, _ := newTestLifecycle(fixedNow)
	if _, err := lc.Ingest(mustParse(t, baseFields(), fixedNow)); err != nil {
		t.Fatal(err)
	}
	// Clock still before expiry: nothing expires.
	if expired := lc.Expire(); len(expired) != 0 {
		t.Errorf("expired = %v, want empty", expired)
	}
	if got := lc.Active(); len(got) != 1 {
		t.Errorf("Active = %v", got)
	}
}

func TestLifecycleNilClockPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("want panic on nil clock")
		}
	}()
	NewLifecycle(nil)
}
