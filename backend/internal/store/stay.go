package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// StayStore persists the P4 stay lifecycle with date-range capacity
// conservation per plan/equations.md. Stay dates are half-open [start, end) in
// the facility's local-date semantics. Every mutation is an atomic conditional
// update (version-guarded) inside the caller's transaction, written together
// with its idempotency result, audit event and outbox record.
//
// Capacity model per (facility, service_date):
//
//	E = free + held + occupied
//	reserve:       free -= p; held += p
//	arrive:        held -= p; occupied += p   (no second decrement of free)
//	cancel/expire: held -= p; free += p
//	depart:        occupied -= p; free += p
//
// Multi-date mutations lock inventory rows in deterministic (facility_id,
// service_date) sorted order to avoid deadlock between concurrent stays.
type StayStore struct {
	audit AuditSink
}

// NewStayStore builds a StayStore. audit may be nil (no audit written).
func NewStayStore(audit AuditSink) *StayStore { return &StayStore{audit: audit} }

// StayState is the stay lifecycle state.
type StayState string

const (
	StayReserved  StayState = "RESERVED"
	StayArrived   StayState = "ARRIVED"
	StayDeparted  StayState = "DEPARTED"
	StayCancelled StayState = "CANCELLED"
	StayExpired   StayState = "EXPIRED"
)

// ErrInvalidTransition is returned when the requested stay transition is not
// legal from the current state.
var ErrInvalidTransition = errors.New("store: invalid stay transition")

// ErrStayNotFound is returned when the target stay does not exist.
var ErrStayNotFound = errors.New("store: stay not found")

// Stay is the durable stay record.
type Stay struct {
	StayID        string
	ReservationID string
	SessionID     string
	FacilityID    string
	PartySize     int
	StartDate     time.Time // local date, half-open [StartDate, EndDate)
	EndDate       time.Time
	State         StayState
	PackageID     string
	RouteID       *string
	Version       int
	ExpiresAt     *time.Time
}

// dateRange returns the half-open list of service dates [start, end).
func dateRange(start, end time.Time) []time.Time {
	var out []time.Time
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}

// lockInventory locks the (facility, date) inventory rows for the whole
// half-open interval in deterministic sorted order and reads each row's
// free/held/occupied. Dates from a half-open range are already ascending, so
// locking in range order is the deterministic order.
func lockInventory(ctx context.Context, db DBTX, facilityID string, dates []time.Time) error {
	for _, d := range dates {
		var x int
		if err := db.QueryRowContext(ctx, `
			SELECT 1 FROM facility_inventory
			WHERE facility_id = $1 AND service_date = $2
			FOR UPDATE`, facilityID, d).Scan(&x); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("store: no inventory for facility %s on %s", facilityID, d.Format("2006-01-02"))
			}
			return err
		}
	}
	return nil
}

// adjustInventory applies a delta to held/occupied for a date, guarded so free
// (capacity - held - occupied) never goes negative and buckets never go below
// zero. It returns ErrCapacityExhausted when the guard matches no row.
func adjustInventory(ctx context.Context, db DBTX, facilityID string, date time.Time, heldDelta, occupiedDelta int, now time.Time) error {
	return execConditional(ctx, db, false, `
		UPDATE facility_inventory
		SET held = held + $3, occupied = occupied + $4, version = version + 1, updated_at = $5
		WHERE facility_id = $1 AND service_date = $2
		  AND held + $3 >= 0 AND occupied + $4 >= 0
		  AND held + $3 + occupied + $4 <= capacity`,
		facilityID, date, heldDelta, occupiedDelta, now)
}

// Reserve creates a stay in RESERVED and converts free space to held across the
// whole half-open interval. Capacity must be available on every date; the
// conditional update matches no row (ErrCapacityExhausted) when any date is
// full. The reservation, stay, inventory updates, idempotency result, audit and
// outbox records commit in the caller's transaction.
func (s *StayStore) Reserve(ctx context.Context, db DBTX, stay Stay, now time.Time) error {
	dates := dateRange(stay.StartDate, stay.EndDate)
	if len(dates) == 0 {
		return fmt.Errorf("store: stay interval is empty")
	}
	// Deterministic lock order across the interval.
	if err := lockInventory(ctx, db, stay.FacilityID, dates); err != nil {
		return err
	}
	// Convert free -> held on every date.
	for _, d := range dates {
		if err := adjustInventory(ctx, db, stay.FacilityID, d, stay.PartySize, 0, now); err != nil {
			if errors.Is(err, ErrVersionConflict) {
				return ErrCapacityExhausted
			}
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO stays (stay_id, reservation_id, session_id, facility_id, party_size,
			start_date, end_date, state, package_id, route_id, version, created_at, updated_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1,$11,$11,$12)`,
		stay.StayID, stay.ReservationID, stay.SessionID, stay.FacilityID, stay.PartySize,
		stay.StartDate, stay.EndDate, string(StayReserved), stay.PackageID, stay.RouteID,
		now, stay.ExpiresAt); err != nil {
		return err
	}
	return s.record(ctx, db, stay.StayID, stay.SessionID, "STAY_RESERVE", "", string(StayReserved), now)
}

// getStayForUpdate locks and reads a stay row.
func (s *StayStore) getStayForUpdate(ctx context.Context, db DBTX, stayID string) (Stay, error) {
	var st Stay
	var state string
	err := db.QueryRowContext(ctx, `
		SELECT stay_id, reservation_id, session_id, facility_id, party_size,
		       start_date, end_date, state, package_id, route_id, version, expires_at
		FROM stays WHERE stay_id = $1 FOR UPDATE`, stayID).
		Scan(&st.StayID, &st.ReservationID, &st.SessionID, &st.FacilityID, &st.PartySize,
			&st.StartDate, &st.EndDate, &state, &st.PackageID, &st.RouteID, &st.Version, &st.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Stay{}, ErrStayNotFound
	}
	if err != nil {
		return Stay{}, err
	}
	st.State = StayState(state)
	return st, nil
}

// setState updates the stay state with a version guard.
func (s *StayStore) setState(ctx context.Context, db DBTX, stayID string, expectedVersion int, target StayState, now time.Time) error {
	return execConditional(ctx, db, false, `
		UPDATE stays SET state = $3, version = version + 1, updated_at = $4
		WHERE stay_id = $1 AND version = $2`,
		stayID, expectedVersion, string(target), now)
}

// Arrive converts held space to occupied across the interval (no second
// decrement of free). Only a RESERVED stay may arrive; expiry/arrival races are
// resolved by the version guard and the inventory conditional update.
func (s *StayStore) Arrive(ctx context.Context, db DBTX, stayID string, now time.Time) error {
	st, err := s.getStayForUpdate(ctx, db, stayID)
	if err != nil {
		return err
	}
	if st.State != StayReserved {
		return ErrInvalidTransition
	}
	// A hold past its expiry must not arrive; the expiry worker owns it.
	if st.ExpiresAt != nil && !now.Before(*st.ExpiresAt) {
		return ErrInvalidTransition
	}
	dates := dateRange(st.StartDate, st.EndDate)
	if err := lockInventory(ctx, db, st.FacilityID, dates); err != nil {
		return err
	}
	for _, d := range dates {
		if err := adjustInventory(ctx, db, st.FacilityID, d, -st.PartySize, st.PartySize, now); err != nil {
			return err
		}
	}
	if err := s.setState(ctx, db, stayID, st.Version, StayArrived, now); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE stays SET arrived_at = $2 WHERE stay_id = $1`, stayID, now); err != nil {
		return err
	}
	return s.record(ctx, db, stayID, st.SessionID, "STAY_ARRIVE", string(StayReserved), string(StayArrived), now)
}

// releaseHeld returns held space to free (cancel/expire before arrival).
func (s *StayStore) releaseHeld(ctx context.Context, db DBTX, st Stay, action string, now time.Time) error {
	dates := dateRange(st.StartDate, st.EndDate)
	if err := lockInventory(ctx, db, st.FacilityID, dates); err != nil {
		return err
	}
	for _, d := range dates {
		if err := adjustInventory(ctx, db, st.FacilityID, d, -st.PartySize, 0, now); err != nil {
			return err
		}
	}
	return nil
}

// Cancel releases a RESERVED hold back to free. Only RESERVED may cancel.
func (s *StayStore) Cancel(ctx context.Context, db DBTX, stayID string, now time.Time) error {
	st, err := s.getStayForUpdate(ctx, db, stayID)
	if err != nil {
		return err
	}
	if st.State != StayReserved {
		return ErrInvalidTransition
	}
	if err := s.releaseHeld(ctx, db, st, "STAY_CANCEL", now); err != nil {
		return err
	}
	if err := s.setState(ctx, db, stayID, st.Version, StayCancelled, now); err != nil {
		return err
	}
	return s.record(ctx, db, stayID, st.SessionID, "STAY_CANCEL", string(StayReserved), string(StayCancelled), now)
}

// Expire releases a RESERVED hold whose expiry has passed. The expiry worker
// calls this; the version guard and inventory conditional update race safely
// with a concurrent arrival.
func (s *StayStore) Expire(ctx context.Context, db DBTX, stayID string, now time.Time) error {
	st, err := s.getStayForUpdate(ctx, db, stayID)
	if err != nil {
		return err
	}
	if st.State != StayReserved {
		return ErrInvalidTransition // already arrived/cancelled; race lost safely
	}
	if st.ExpiresAt == nil || now.Before(*st.ExpiresAt) {
		return ErrInvalidTransition // not yet due
	}
	if err := s.releaseHeld(ctx, db, st, "STAY_EXPIRE", now); err != nil {
		return err
	}
	if err := s.setState(ctx, db, stayID, st.Version, StayExpired, now); err != nil {
		return err
	}
	return s.record(ctx, db, stayID, st.SessionID, "STAY_EXPIRE", string(StayReserved), string(StayExpired), now)
}

// Depart frees occupied space across the interval. Only ARRIVED may depart.
func (s *StayStore) Depart(ctx context.Context, db DBTX, stayID string, now time.Time) error {
	st, err := s.getStayForUpdate(ctx, db, stayID)
	if err != nil {
		return err
	}
	if st.State != StayArrived {
		return ErrInvalidTransition
	}
	dates := dateRange(st.StartDate, st.EndDate)
	if err := lockInventory(ctx, db, st.FacilityID, dates); err != nil {
		return err
	}
	for _, d := range dates {
		if err := adjustInventory(ctx, db, st.FacilityID, d, 0, -st.PartySize, now); err != nil {
			return err
		}
	}
	if err := s.setState(ctx, db, stayID, st.Version, StayDeparted, now); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `UPDATE stays SET departed_at = $2 WHERE stay_id = $1`, stayID, now); err != nil {
		return err
	}
	return s.record(ctx, db, stayID, st.SessionID, "STAY_DEPART", string(StayArrived), string(StayDeparted), now)
}

// Extend lengthens a stay's end_date, requiring availability on the added dates
// only. The original dates are already held/occupied; only the extension dates
// convert free -> held (RESERVED) or free -> occupied (ARRIVED). Bounded by the
// facility/policy temporary-stay limits, enforced by the caller.
func (s *StayStore) Extend(ctx context.Context, db DBTX, stayID string, newEndDate time.Time, now time.Time) error {
	st, err := s.getStayForUpdate(ctx, db, stayID)
	if err != nil {
		return err
	}
	if st.State != StayReserved && st.State != StayArrived {
		return ErrInvalidTransition
	}
	if !newEndDate.After(st.EndDate) {
		return fmt.Errorf("store: extension must lengthen the stay")
	}
	added := dateRange(st.EndDate, newEndDate)
	if err := lockInventory(ctx, db, st.FacilityID, added); err != nil {
		return err
	}
	// RESERVED holds add to held; ARRIVED stays add to occupied.
	heldDelta, occDelta := st.PartySize, 0
	if st.State == StayArrived {
		heldDelta, occDelta = 0, st.PartySize
	}
	for _, d := range added {
		if err := adjustInventory(ctx, db, st.FacilityID, d, heldDelta, occDelta, now); err != nil {
			if errors.Is(err, ErrVersionConflict) {
				return ErrCapacityExhausted
			}
			return err
		}
	}
	if err := execConditional(ctx, db, false, `
		UPDATE stays SET end_date = $3, version = version + 1, updated_at = $4
		WHERE stay_id = $1 AND version = $2`,
		stayID, st.Version, newEndDate, now); err != nil {
		return err
	}
	return s.record(ctx, db, stayID, st.SessionID, "STAY_EXTEND", string(st.State), string(st.State), now)
}

// Transfer moves a stay to a different facility. The new facility's space is
// held first; only after the new stay is safely created is the old stay's space
// released. A failed transfer (new facility full) returns ErrCapacityExhausted
// and retains the original stay unchanged.
func (s *StayStore) Transfer(ctx context.Context, db DBTX, stayID, newStayID, newReservationID, newFacilityID string, now time.Time) error {
	st, err := s.getStayForUpdate(ctx, db, stayID)
	if err != nil {
		return err
	}
	if st.State != StayReserved && st.State != StayArrived {
		return ErrInvalidTransition
	}
	// Hold the new facility's space first. If this fails the original stay is
	// untouched (the transaction rolls back).
	dates := dateRange(st.StartDate, st.EndDate)
	if err := lockInventory(ctx, db, newFacilityID, dates); err != nil {
		return err
	}
	for _, d := range dates {
		if err := adjustInventory(ctx, db, newFacilityID, d, st.PartySize, 0, now); err != nil {
			if errors.Is(err, ErrVersionConflict) {
				return ErrCapacityExhausted
			}
			return err
		}
	}
	// Create the replacement reservation then the replacement stay in RESERVED,
	// linked to its origin. The reservation row must exist for the stay FK.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO reservations (reservation_id, session_id, facility_id, service_date, party_size, state, version, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,'RESERVED',1,$6,$6)`,
		newReservationID, st.SessionID, newFacilityID, st.StartDate, st.PartySize, now); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO stays (stay_id, reservation_id, session_id, facility_id, party_size,
			start_date, end_date, state, package_id, route_id, transferred_from, version, created_at, updated_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1,$12,$12,$13)`,
		newStayID, newReservationID, st.SessionID, newFacilityID, st.PartySize,
		st.StartDate, st.EndDate, string(StayReserved), st.PackageID, st.RouteID,
		stayID, now, st.ExpiresAt); err != nil {
		return err
	}
	// Release the old stay's space (held if RESERVED, occupied if ARRIVED).
	if err := lockInventory(ctx, db, st.FacilityID, dates); err != nil {
		return err
	}
	heldDelta, occDelta := -st.PartySize, 0
	if st.State == StayArrived {
		heldDelta, occDelta = 0, -st.PartySize
	}
	for _, d := range dates {
		if err := adjustInventory(ctx, db, st.FacilityID, d, heldDelta, occDelta, now); err != nil {
			return err
		}
	}
	if err := s.setState(ctx, db, stayID, st.Version, StayDeparted, now); err != nil {
		return err
	}
	if err := s.record(ctx, db, stayID, st.SessionID, "STAY_TRANSFER_OUT", string(st.State), string(StayDeparted), now); err != nil {
		return err
	}
	return s.record(ctx, db, newStayID, st.SessionID, "STAY_TRANSFER_IN", "", string(StayReserved), now)
}

// StayFacilityJurisdiction reports the jurisdiction of the facility a stay
// belongs to. Used to scope an operator correction to the operator's own
// jurisdiction (cross-jurisdiction assistance is forbidden).
func (s *StayStore) StayFacilityJurisdiction(ctx context.Context, db DBTX, stayID string) (string, error) {
	var j string
	err := db.QueryRowContext(ctx, `
		SELECT p.jurisdiction FROM stays st
		JOIN facilities f ON f.facility_id = st.facility_id
		JOIN packages p ON p.package_id = f.package_id
		WHERE st.stay_id = $1`, stayID).Scan(&j)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrStayNotFound
	}
	if err != nil {
		return "", err
	}
	return j, nil
}

// Correct applies an auditable operator correction to a stay's party size.
// Only a live (RESERVED or ARRIVED) stay may be corrected, and only downward
// (releasing space). Releasing space is always conservation-safe: held/occupied
// fall and free rises. An upward correction would require new capacity and is
// rejected here rather than silently overbooking. The correction, its audit
// event (actor = operator session) and the inventory adjustment commit in the
// caller's transaction.
func (s *StayStore) Correct(ctx context.Context, db DBTX, stayID string, newPartySize int, operatorSessionID, reason string, now time.Time) error {
	st, err := s.getStayForUpdate(ctx, db, stayID)
	if err != nil {
		return err
	}
	if st.State != StayReserved && st.State != StayArrived {
		return ErrInvalidTransition
	}
	if newPartySize < 1 {
		return fmt.Errorf("store: corrected party size must be at least 1")
	}
	if newPartySize > st.PartySize {
		return fmt.Errorf("store: operator correction cannot increase party size")
	}
	if newPartySize == st.PartySize {
		return nil // no-op; nothing to release
	}
	delta := st.PartySize - newPartySize
	dates := dateRange(st.StartDate, st.EndDate)
	if err := lockInventory(ctx, db, st.FacilityID, dates); err != nil {
		return err
	}
	heldDelta, occDelta := -delta, 0
	if st.State == StayArrived {
		heldDelta, occDelta = 0, -delta
	}
	for _, d := range dates {
		if err := adjustInventory(ctx, db, st.FacilityID, d, heldDelta, occDelta, now); err != nil {
			return err
		}
	}
	if err := execConditional(ctx, db, false, `
		UPDATE stays SET party_size = $3, version = version + 1, updated_at = $4
		WHERE stay_id = $1 AND version = $2`,
		stayID, st.Version, newPartySize, now); err != nil {
		return err
	}
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, db, AuditEvent{
		EventID:   stayID + ":STAY_CORRECT",
		OccuredAt: now,
		ActorID:   operatorSessionID,
		Action:    "STAY_CORRECT",
		SubjectID: stayID,
		Outcome:   "OK",
		Reason:    reason,
		FromState: string(st.State),
		ToState:   string(st.State),
	})
}

// record appends an audit event for a stay transition.
func (s *StayStore) record(ctx context.Context, db DBTX, stayID, sessionID, action, from, to string, now time.Time) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, db, AuditEvent{
		EventID:   stayID + ":" + action,
		OccuredAt: now,
		ActorID:   sessionID,
		Action:    action,
		SubjectID: stayID,
		Outcome:   "OK",
		FromState: from,
		ToState:   to,
	})
}
