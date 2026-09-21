package loadfixtures

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// syntheticStore is a tiny in-memory reservation store used by the
// synthetic fixture server. It models the (facility_id, service_date)
// contention that D28 demands and the idempotency-keyed replay that
// R13 / D30 require.
type syntheticStore struct {
	mu     sync.Mutex
	next   int
	idem   map[string]string // idempotency_key -> reservation_id
	byID   map[string]reservation
	cap    map[string]int // facility|service_date -> remaining capacity
}

type reservation struct {
	ID         string `json:"reservation_id"`
	FacilityID string `json:"facility_id"`
	ServiceDate string `json:"service_date"`
	State      string `json:"state"`
	CreatedAt  string `json:"created_at"`
}

func newSyntheticStore() *syntheticStore {
	return &syntheticStore{
		idem: map[string]string{},
		byID: map[string]reservation{},
		cap:  map[string]int{},
	}
}

func (s *syntheticStore) Reserve(idemKey string, now time.Time) (reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idemKey != "" {
		if id, ok := s.idem[idemKey]; ok {
			return s.byID[id], nil
		}
	}
	// Round-robin across 10 facilities, one date = today, capacity=1 each.
	// This is the worst-case contention scenario: every write hits a row
	// with capacity 1, so all concurrent writers after the first one fail.
	const facs = 10
	const cap = 1
	s.next = (s.next + 1) % facs
	key := strconv.Itoa(s.next)
	if cur, ok := s.cap[key]; ok && cur <= 0 {
		return reservation{}, fmt.Errorf("capacity exhausted for slot %s", key)
	}
	s.cap[key] = cap - 1
	id := newIDLocked("RES")
	r := reservation{
		ID:          id,
		FacilityID:  fmt.Sprintf("FACILITY-DEMO-%d", s.next+1),
		ServiceDate: now.Format("2006-01-02"),
		State:       "HELD",
		CreatedAt:   now.Format(time.RFC3339Nano),
	}
	s.byID[id] = r
	if idemKey != "" {
		s.idem[idemKey] = id
	}
	return r, nil
}

func (s *syntheticStore) Get(id string) (reservation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.byID[id]
	return r, ok
}

func newIDLocked(prefix string) string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (i % 8))
	}
	// reuse hex but inline here to avoid pulling encoding/hex into this file
	const hexDigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, x := range b {
		out[i*2] = hexDigits[x>>4]
		out[i*2+1] = hexDigits[x&0x0f]
	}
	return prefix + "-" + string(out)
}
