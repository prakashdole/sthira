package store

import (
	"context"
	"errors"
	"time"
)

// ReservationStore persists the reservation/capacity foundation. P3 stores the
// reservation record and the facility/date inventory with conservation; the
// full stay workflow (arrival/depart/transfer/extend) is P4. Conservation is
// enforced by an atomic conditional UPDATE on the inventory row so concurrent
// reservations for the last spaces race safely without a process-local lock.
type ReservationStore struct {
	audit AuditSink
}

// NewReservationStore builds a ReservationStore. audit may be nil.
func NewReservationStore(audit AuditSink) *ReservationStore {
	return &ReservationStore{audit: audit}
}

// ErrCapacityExhausted is returned when the inventory row has no room for the
// requested party size (the conditional UPDATE matched no row).
var ErrCapacityExhausted = errors.New("store: capacity exhausted")

// Reserve atomically increments reserved on the facility/date inventory row and
// inserts the reservation, in the caller's transaction. The conditional UPDATE
// (reserved + party <= capacity) matches no row when capacity is exhausted or
// the inventory row is absent, so no-row/first-insert contention is handled by
// the caller creating the inventory row first (see EnsureInventory).
func (s *ReservationStore) Reserve(ctx context.Context, db DBTX, r Reservation, expectedInventoryVersion int) error {
	err := execConditional(ctx, db, false, `
		UPDATE facility_inventory
		SET reserved = reserved + $3, version = version + 1, updated_at = $4
		WHERE facility_id = $1 AND service_date = $2
		  AND version = $5
		  AND reserved + $3 <= capacity`,
		r.FacilityID, r.ServiceDate, r.PartySize, r.CreatedAt, expectedInventoryVersion)
	if errors.Is(err, ErrVersionConflict) {
		// No row matched: either capacity exhausted, stale version, or no
		// inventory row. Surface capacity/missing distinctly from a stale read.
		var exists bool
		qerr := db.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM facility_inventory
				WHERE facility_id = $1 AND service_date = $2 AND reserved + $3 <= capacity)`,
			r.FacilityID, r.ServiceDate, r.PartySize).Scan(&exists)
		if qerr != nil {
			return qerr
		}
		if !exists {
			return ErrCapacityExhausted
		}
		return ErrVersionConflict
	}
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO reservations
			(reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,'RESERVED',1,$6,$6,$7)`,
		r.ReservationID, r.SessionID, r.FacilityID, r.ServiceDate, r.PartySize,
		r.CreatedAt, r.ExpiresAt); err != nil {
		return err
	}

	if s.audit != nil {
		return s.audit.Record(ctx, db, AuditEvent{
			EventID:   r.ReservationID,
			OccuredAt: r.CreatedAt,
			ActorID:   r.SessionID,
			Action:    "RESERVATION_CREATE",
			SubjectID: r.ReservationID,
			Outcome:   "OK",
			ToState:   "RESERVED",
		})
	}
	return nil
}

// EnsureInventory inserts the facility/date inventory row if absent. It is the
// first-insert path for the no-row contention case: concurrent creators race on
// the PRIMARY KEY and the loser re-reads.
func (s *ReservationStore) EnsureInventory(ctx context.Context, db DBTX, facilityID string, serviceDate time.Time, capacity int, now time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, version, updated_at)
		VALUES ($1, $2, $3, 0, 1, $4)
		ON CONFLICT (facility_id, service_date) DO NOTHING`,
		facilityID, serviceDate, capacity, now)
	return err
}

// Reservation is the durable reservation record P3 persists.
type Reservation struct {
	ReservationID string
	SessionID     string
	FacilityID    string
	ServiceDate   time.Time
	PartySize     int
	CreatedAt     time.Time
	ExpiresAt     *time.Time
}
