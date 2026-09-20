package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"sthira/backend/internal/opkg"
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
// outbox records commit in the caller's transaction. idemKey is the idempotency
// key for the operation; when provided it is included in the audit EventID to
// give each distinct committed operation a unique identity.
func (s *StayStore) Reserve(ctx context.Context, db DBTX, stay Stay, now time.Time, idemKey string) error {
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
	return s.record(ctx, db, stay.StayID, stay.SessionID, "STAY_RESERVE", "", string(StayReserved), now, idemKey)
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
func (s *StayStore) Arrive(ctx context.Context, db DBTX, stayID string, now time.Time, idemKey string) error {
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
	return s.record(ctx, db, stayID, st.SessionID, "STAY_ARRIVE", string(StayReserved), string(StayArrived), now, idemKey)
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
func (s *StayStore) Cancel(ctx context.Context, db DBTX, stayID string, now time.Time, idemKey string) error {
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
	return s.record(ctx, db, stayID, st.SessionID, "STAY_CANCEL", string(StayReserved), string(StayCancelled), now, idemKey)
}

// Expire releases a RESERVED hold whose expiry has passed. The expiry worker
// calls this; the version guard and inventory conditional update race safely
// with a concurrent arrival.
func (s *StayStore) Expire(ctx context.Context, db DBTX, stayID string, now time.Time, idemKey string) error {
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
	return s.record(ctx, db, stayID, st.SessionID, "STAY_EXPIRE", string(StayReserved), string(StayExpired), now, idemKey)
}

// Depart frees occupied space across the interval. Only ARRIVED may depart.
func (s *StayStore) Depart(ctx context.Context, db DBTX, stayID string, now time.Time, idemKey string) error {
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
	return s.record(ctx, db, stayID, st.SessionID, "STAY_DEPART", string(StayArrived), string(StayDeparted), now, idemKey)
}

// Extend lengthens a stay's end_date, requiring availability on the added dates
// only. The original dates are already held/occupied; only the extension dates
// convert free -> held (RESERVED) or free -> occupied (ARRIVED). Bounded by the
// facility/policy temporary-stay limits, enforced by the caller. idemKey is the
// idempotency key for the operation; when provided it is included in the audit
// EventID to give each distinct committed operation a unique identity.
func (s *StayStore) Extend(ctx context.Context, db DBTX, stayID string, newEndDate time.Time, now time.Time, idemKey string) error {
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
	return s.record(ctx, db, stayID, st.SessionID, "STAY_EXTEND", string(st.State), string(st.State), now, idemKey)
}

// Transfer moves a stay to a different facility. The new facility's space is
// held first; only after the new stay is safely created is the old stay's space
// released. A failed transfer (new facility full) returns ErrCapacityExhausted
// and retains the original stay unchanged. Inventory rows for BOTH facilities
// are locked in deterministic (facility_id, service_date) order to avoid
// deadlock between opposing transfers. The caller passes newRouteID as the
// route validated against the NEW facility's safe zone (never the old stay's
// route); it is persisted on the replacement stay so an authenticated read
// after a lost response sees the actual route to the destination. idemKey is
// the idempotency key for the operation; when provided it is included in the
// audit EventID to give each distinct committed operation a unique identity.
func (s *StayStore) Transfer(ctx context.Context, db DBTX, stayID, newStayID, newReservationID, newFacilityID string, newRouteID *string, newExpiresAt *time.Time, now time.Time, idemKey string) error {
	st, err := s.getStayForUpdate(ctx, db, stayID)
	if err != nil {
		return err
	}
	if st.State != StayReserved && st.State != StayArrived {
		return ErrInvalidTransition
	}
	dates := dateRange(st.StartDate, st.EndDate)
	// Deterministic lock order across both facilities to prevent deadlock.
	lockOrder := []string{st.FacilityID, newFacilityID}
	if lockOrder[0] > lockOrder[1] {
		lockOrder[0], lockOrder[1] = lockOrder[1], lockOrder[0]
	}
	for _, f := range lockOrder {
		if err := lockInventory(ctx, db, f, dates); err != nil {
			return err
		}
	}
	// Hold the new facility's space first. If this fails the original stay is
	// untouched (the transaction rolls back).
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
	// The replacement stay is persisted with the VALIDATED replacement route
	// (newRouteID), not the old stay's route. The old stay's route bound the
	// origin facility's safe zone; it does not satisfy the new facility's safe
	// zone binding. The caller passes the route that RevalidateReservationContext
	// accepted against the destination. The replacement stay gets a FRESH hold
	// deadline, not the old one: copying a stale/expired deadline into the new
	// stay would let an already-due hold persist. The caller derives the new
	// deadline from authoritative policy.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO stays (stay_id, reservation_id, session_id, facility_id, party_size,
			start_date, end_date, state, package_id, route_id, transferred_from, version, created_at, updated_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1,$12,$12,$13)`,
		newStayID, newReservationID, st.SessionID, newFacilityID, st.PartySize,
		st.StartDate, st.EndDate, string(StayReserved), st.PackageID, newRouteID,
		stayID, now, newExpiresAt); err != nil {
		return err
	}
	// Release the old stay's space (held if RESERVED, occupied if ARRIVED).
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
	if err := s.record(ctx, db, stayID, st.SessionID, "STAY_TRANSFER_OUT", string(st.State), string(StayDeparted), now, idemKey); err != nil {
		return err
	}
	return s.record(ctx, db, newStayID, st.SessionID, "STAY_TRANSFER_IN", "", string(StayReserved), now, idemKey)
}

// ErrReservationContext is returned when the reservation's authoritative
// context (source state/authorization, package validity/supersession,
// jurisdiction, facility membership) is not currently operational. A direct
// reservation request is denied just as the resolved voice context is.
var ErrReservationContext = errors.New("store: reservation context is not currently operational")

// ErrFacilityPackageMismatch is returned when the facility does not belong to
// the requested package.
var ErrFacilityPackageMismatch = errors.New("store: facility is not in the requested package")

// ErrRouteUnavailable is returned when a requested route is not verified,
// currently valid and unclosed in the package. Operational routing stays
// disabled while O05 is open; a route that cannot be verified blocks the
// commitment that depends on it.
var ErrRouteUnavailable = errors.New("store: route is not verified and currently valid")

// StayPolicy is the authoritative government-owned stay/reservation policy read
// from the package body. Fields are absent (nil) when the source does not supply
// them; they are never inferred or defaulted.
type StayPolicy struct {
	ReservationExpirySeconds *int
	TemporaryStayMinDays     *int
	TemporaryStayMaxDays     *int
	AllowTransfers           *bool
	// RouteRequired means every operational commitment (reservation,
	// extension, transfer) must reference a verified, currently-valid,
	// unclosed route bound to the destination's safe zone. The store fails
	// closed when RouteRequired=true and no route is provided: the commitment
	// is rejected with ErrRouteRequired and no capacity change occurs.
	RouteRequired *bool
}

// RoutePolicyRequired reports whether the policy requires a route on every
// operational commitment. It fails closed: if route_required is absent or null
// in the authoritative policy, it returns ErrNoStayPolicy rather than inferring
// a government default.
func (p StayPolicy) RoutePolicyRequired() (bool, error) {
	if p.RouteRequired == nil {
		return false, ErrNoStayPolicy
	}
	return *p.RouteRequired, nil
}

// ErrNoStayPolicy is returned when a required policy field is absent from the
// authoritative package. The caller must reject rather than invent a default.
var ErrNoStayPolicy = errors.New("store: authoritative stay policy is missing the required field")

// ErrRouteRequired is returned when the authoritative policy requires a route
// (allocation_policy.route_required = true) and the commitment supplied no
// route_id, or the supplied route is invalid. Capacity is unchanged.
var ErrRouteRequired = errors.New("store: route required by authoritative policy and none provided")

// ErrStayOutOfPolicy is returned when the requested stay dates fall outside the
// authoritative temporary-stay bounds.
var ErrStayOutOfPolicy = errors.New("store: stay dates outside the authoritative temporary-stay bounds")

// ReservationContextOption configures RevalidateReservationContext.
type ReservationContextOption func(*reservationContextConfig)

type reservationContextConfig struct {
	allowSynthetic bool
}

// WithAllowSynthetic controls whether synthetic evidence (SYNTHETIC_DEMO
// packages, source artifacts, and routes) is accepted by
// RevalidateReservationContext for isolated test/exercise execution.
// Defaults to false (ordinary production configuration fails closed).
func WithAllowSynthetic(allow bool) ReservationContextOption {
	return func(c *reservationContextConfig) {
		c.allowSynthetic = allow
	}
}

// policyBody is the minimal package-body shape for the allocation policy.
type policyBody struct {
	AllocationPolicy struct {
		ReservationExpirySeconds *int  `json:"reservation_expiry_seconds"`
		TemporaryStayMinDays     *int  `json:"temporary_stay_min_days"`
		TemporaryStayMaxDays     *int  `json:"temporary_stay_max_days"`
		AllowTransfers           *bool `json:"allow_transfers"`
		RouteRequired            *bool `json:"route_required"`
	} `json:"allocation_policy"`
}

// RevalidateReservationContext enforces the authoritative eligibility gates at
// commit time, inside the reservation transaction. It binds the facility,
// optional route, package and requested stay to ONE currently-operational
// context: the package's source must be OPERATIONAL with a currently-valid
// authorization in the package's jurisdiction; the package must be effective,
// unexpired and not superseded; the facility must belong to the package. A
// requested route must be verified, currently valid and unclosed. Both the
// source and package rows are locked FOR UPDATE (source first, then package)
// so a concurrent quarantine/suspension/revocation/supersession serializes
// against this reservation: whichever commits first determines whether the
// reservation sees a still-operational or already-withdrawn context. The
// source state is rechecked under the lock. Returns the package jurisdiction
// and the authoritative stay policy for the caller's use.
func RevalidateReservationContext(ctx context.Context, db DBTX, facilityID, packageID string, routeID *string, now time.Time, opts ...ReservationContextOption) (string, StayPolicy, error) {
	var cfg reservationContextConfig
	for _, o := range opts {
		o(&cfg)
	}

	var (
		jurisdiction string
		body         []byte
		sourceID     string
		pkgEvidence  string
		artEvidence  string
	)
	// Lock source first (consistent order with source state transitions), then package.
	err := db.QueryRowContext(ctx, `
		SELECT p.jurisdiction, p.body, p.source_id, p.evidence_class, sa.evidence_class
		FROM packages p
		JOIN sources s ON s.source_id = p.source_id
		JOIN source_artifacts sa ON sa.artifact_id = p.artifact_id
		WHERE p.package_id = $1
		  AND s.state = 'OPERATIONAL'
		  AND p.effective_at <= $2 AND p.expires_at > $2
		  AND p.superseded_by IS NULL
		  AND EXISTS (
			SELECT 1 FROM source_authorizations sauth
			WHERE sauth.source_id = p.source_id AND sauth.jurisdiction = p.jurisdiction
			  AND (sauth.expires_at IS NULL OR sauth.expires_at > $2)
		  )
		FOR UPDATE OF s, p`, packageID, now).Scan(&jurisdiction, &body, &sourceID, &pkgEvidence, &artEvidence)
	if errors.Is(err, sql.ErrNoRows) {
		return "", StayPolicy{}, ErrReservationContext
	}
	if err != nil {
		return "", StayPolicy{}, err
	}

	// Ordinary production configuration must reject synthetic evidence for
	// operational commitments. Check persisted source and package evidence consistently.
	isSynthetic := pkgEvidence == string(opkg.EvidenceSynthetic) ||
		pkgEvidence == "SYNTHETIC" ||
		artEvidence == string(opkg.EvidenceSynthetic) ||
		artEvidence == "SYNTHETIC"

	if !cfg.allowSynthetic {
		if isSynthetic || pkgEvidence != string(opkg.EvidenceOperational) || artEvidence != string(opkg.EvidenceOperational) {
			return "", StayPolicy{}, ErrReservationContext
		}
	}

	// Facility must belong to the requested package.
	var fcount int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM facilities WHERE facility_id = $1 AND package_id = $2`,
		facilityID, packageID).Scan(&fcount); err != nil {
		return "", StayPolicy{}, err
	}
	if fcount == 0 {
		return "", StayPolicy{}, ErrFacilityPackageMismatch
	}
	// The facility's safe zone must be currently available. CLOSED, FULL or
	// other non-operational zone status blocks the commitment; OPEN and
	// PUBLISHED are accepted. This is the destination-availability boundary
	// enforced at commit (not only at eligibility preview).
	var zoneStatus string
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(zv.status, '') FROM facilities f
		JOIN zone_versions zv ON zv.zone_id = f.safe_zone_id AND zv.package_id = f.package_id
		WHERE f.facility_id = $1 AND f.package_id = $2`,
		facilityID, packageID).Scan(&zoneStatus); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return "", StayPolicy{}, err
		}
		// No zone_versions row: zone status is unknown; reject (fail closed).
		return "", StayPolicy{}, ErrReservationContext
	}
	switch zoneStatus {
	case "", "OPEN", "PUBLISHED":
		// accepted
	default:
		return "", StayPolicy{}, ErrReservationContext
	}
	// A requested route must be verified, currently valid, unclosed, AND bound
	// to the facility's safe zone (to_safe_zone_id matches). The same gate
	// applies at reserve, extend and transfer: the route must lead to the
	// selected destination's safe zone.
	// In ordinary production configuration, synthetic routes are rejected and
	// operational routing is disabled while O05 is open (R08).
	hasRoute := routeID != nil && *routeID != ""
	if hasRoute {
		if !cfg.allowSynthetic {
			return "", StayPolicy{}, ErrRouteUnavailable
		}
		var rcount int
		if err := db.QueryRowContext(ctx, `
			SELECT count(*) FROM route_versions rv
			JOIN facilities f ON f.facility_id = $4 AND f.package_id = $1
			WHERE rv.package_id = $1 AND rv.route_id = $2
			  AND rv.to_safe_zone_id = f.safe_zone_id
			  AND rv.approval IN ('SYNTHETIC_DEMO','AUTHORIZED_OPERATIONAL')
			  AND rv.verified_by IS NOT NULL
			  AND (rv.valid_from IS NULL OR rv.valid_from <= $3)
			  AND (rv.valid_until IS NULL OR rv.valid_until > $3)
			  AND NOT EXISTS (
				SELECT 1 FROM route_closures rc
				WHERE rc.route_id = rv.route_id AND rc.reopened_at IS NULL)`,
			packageID, *routeID, now, facilityID).Scan(&rcount); err != nil {
			return "", StayPolicy{}, err
		}
		if rcount == 0 {
			return "", StayPolicy{}, ErrRouteUnavailable
		}
	}
	// Read the authoritative stay policy from the same locked package row.
	var pb policyBody
	if err := json.Unmarshal(body, &pb); err != nil {
		return "", StayPolicy{}, err
	}
	pol := StayPolicy{
		ReservationExpirySeconds: pb.AllocationPolicy.ReservationExpirySeconds,
		TemporaryStayMinDays:     pb.AllocationPolicy.TemporaryStayMinDays,
		TemporaryStayMaxDays:     pb.AllocationPolicy.TemporaryStayMaxDays,
		AllowTransfers:           pb.AllocationPolicy.AllowTransfers,
		RouteRequired:            pb.AllocationPolicy.RouteRequired,
	}
	// Missing route-policy authority must fail closed: missing or null
	// route_required rejects a new commitment that needs this policy with
	// ErrNoStayPolicy. Explicit true requires a validated route (ErrRouteRequired
	// if omitted). Explicit false permits route omission if all other gates pass.
	routeReq, err := pol.RoutePolicyRequired()
	if err != nil {
		return "", StayPolicy{}, err
	}
	if routeReq && !hasRoute {
		return "", StayPolicy{}, ErrRouteRequired
	}
	return jurisdiction, pol, nil
}

// CheckStayBounds enforces the authoritative temporary-stay bounds on a
// half-open [start, end) interval. A bound that is absent means the policy does
// not authorize stays; that is rejected, not defaulted. The interval length in
// whole days must satisfy min <= days <= max.
func CheckStayBounds(pol StayPolicy, start, end time.Time) error {
	if pol.TemporaryStayMinDays == nil || pol.TemporaryStayMaxDays == nil {
		return ErrNoStayPolicy
	}
	days := int(end.Sub(start).Hours() / 24)
	if days < *pol.TemporaryStayMinDays || days > *pol.TemporaryStayMaxDays {
		return ErrStayOutOfPolicy
	}
	return nil
}

// HoldExpiry derives the reservation hold deadline from the authoritative
// policy and server time. A missing expiry policy is rejected, never defaulted.
func HoldExpiry(pol StayPolicy, now time.Time) (time.Time, error) {
	if pol.ReservationExpirySeconds == nil || *pol.ReservationExpirySeconds <= 0 {
		return time.Time{}, ErrNoStayPolicy
	}
	return now.Add(time.Duration(*pol.ReservationExpirySeconds) * time.Second), nil
}

// StayContext returns the stay's package, facility and route for authoritative
// revalidation of a new commitment (extension/transfer).
func (s *StayStore) StayContext(ctx context.Context, db DBTX, stayID string) (packageID, facilityID string, routeID *string, err error) {
	err = db.QueryRowContext(ctx, `
		SELECT package_id, facility_id, route_id FROM stays WHERE stay_id = $1`, stayID).
		Scan(&packageID, &facilityID, &routeID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil, ErrStayNotFound
	}
	if err != nil {
		return "", "", nil, err
	}
	return packageID, facilityID, routeID, nil
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
// caller's transaction. idemKey is the idempotency key for the operation; when
// provided it is included in the audit EventID to give each distinct committed
// operation a unique identity.
func (s *StayStore) Correct(ctx context.Context, db DBTX, stayID string, newPartySize int, operatorSessionID, reason string, now time.Time, idemKey string) error {
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
		EventID:   auditEventID(stayID, "STAY_CORRECT", operatorSessionID, idemKey),
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

// auditEventID builds a unique audit event ID by combining the subject ID,
// action, actor and optional idempotency key. When idemKey is provided, it
// is included to give each distinct committed operation a unique identity.
// This prevents collisions on repeated legitimate operations (e.g., two
// extensions with different keys) while preserving exactly-once behavior
// for retries of the same operation (same key + same actor produces same
// EventID). Different authorized operator sessions using the same key for
// distinct corrections do NOT collide because their ActorID is part of the
// identity.
func auditEventID(subjectID, action, actorID, idemKey string) string {
	parts := []string{subjectID, action, actorID}
	if idemKey != "" {
		parts = append(parts, idemKey)
	}
	return strings.Join(parts, ":")
}

// record appends an audit event for a stay transition. idemKey is the
// idempotency key for the operation; when provided it is included in the
// audit EventID to give each distinct committed operation a unique identity.
func (s *StayStore) record(ctx context.Context, db DBTX, stayID, sessionID, action, from, to string, now time.Time, idemKey string) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, db, AuditEvent{
		EventID:   auditEventID(stayID, action, sessionID, idemKey),
		OccuredAt: now,
		ActorID:   sessionID,
		Action:    action,
		SubjectID: stayID,
		Outcome:   "OK",
		FromState: from,
		ToState:   to,
	})
}
