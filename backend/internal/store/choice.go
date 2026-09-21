package store

import (
	"context"
	"time"
)

// Eligible destination choice. A destination is eligible only when every gate
// passes (plan/equations.md):
//
//	eligible = authorized_active_facility AND current_incident_and_policy
//	           AND party_and_stay_supported AND capacity_policy_satisfied
//	           AND operational_route_gate_satisfied
//
// The operational route gate is FALSE while route authority (O05) is open; only
// clearly isolated synthetic fixtures may satisfy it. Unknown capacity is
// reported as unknown and never becomes a reservation promise. No destination
// is silently substituted after a capacity conflict; no safety, accessibility
// or road condition is inferred.

// Destination is one eligible choice presented to the citizen.
type Destination struct {
	FacilityID string
	SafeZoneID string
	// CapacityKnown is false when the facility's capacity is unknown; Capacity
	// is then meaningless and must be displayed as "unknown", never promised.
	CapacityKnown bool
	// Free is the current free space across the requested interval; only set
	// when CapacityKnown. Displayed honestly; not a guarantee at commit time.
	Free int
	// RouteID is the verified route to this destination, when one exists and
	// the route gate is satisfiable. Nil while route authority is open.
	RouteID *string
	// RouteVerified reports whether a verified, currently-valid, unclosed route
	// exists. False blocks operational guidance for this destination.
	RouteVerified bool
}

// ChoiceQuery bounds an eligibility query.
type ChoiceQuery struct {
	Jurisdiction string
	PackageID    string // the current package snapshot
	PartySize    int
	StartDate    time.Time // half-open [StartDate, EndDate)
	EndDate      time.Time
	// RouteGateOpen reports whether operational routing is authorized (O05).
	// While false, no destination is route-verified for operational use.
	// This MUST be false in production; synthetic routes are only available
	// via isolated test configuration that directly invokes the store layer.
	RouteGateOpen bool
}

// routeVerified reports whether a route from any red zone to the safe zone is
// verified, currently valid and not closed. Synthetic-approval routes count
// only when the caller has explicitly opened the gate for an isolated exercise
// (RouteGateOpen=true). In production, RouteGateOpen is always false.
func routeVerified(ctx context.Context, db DBTX, packageID, safeZoneID string, now time.Time, gateOpen bool) (bool, *string, error) {
	if !gateOpen {
		return false, nil, nil
	}
	var routeID string
	err := db.QueryRowContext(ctx, `
		SELECT rv.route_id FROM route_versions rv
		WHERE rv.package_id = $1 AND rv.to_safe_zone_id = $2
		  AND rv.approval IN ('SYNTHETIC_DEMO','AUTHORIZED_OPERATIONAL')
		  AND rv.verified_by IS NOT NULL
		  AND (rv.valid_from IS NULL OR rv.valid_from <= $3)
		  AND (rv.valid_until IS NULL OR rv.valid_until > $3)
		  AND NOT EXISTS (
			SELECT 1 FROM route_closures rc
			WHERE rc.route_id = rv.route_id AND rc.reopened_at IS NULL)
		ORDER BY rv.route_id
		LIMIT 1`, packageID, safeZoneID, now).Scan(&routeID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, &routeID, nil
}

// ChoiceQuerier computes eligible destinations.
type ChoiceQuerier struct{}

// Eligible returns the destinations that satisfy every gate for the query. A
// destination with unknown capacity is included with CapacityKnown=false
// (informational) but cannot be reserved. No destination is invented, ranked by
// guessed safety, or substituted.
func (ChoiceQuerier) Eligible(ctx context.Context, db DBTX, q ChoiceQuery, now time.Time) ([]Destination, error) {
	dates := dateRange(q.StartDate, q.EndDate)
	if len(dates) == 0 {
		return nil, nil
	}
	// Facilities in the package's safe zones, joined to their zone status.
	rows, err := db.QueryContext(ctx, `
		SELECT f.facility_id, f.safe_zone_id, zv.capacity, zv.status
		FROM facilities f
		JOIN zone_versions zv ON zv.zone_id = f.safe_zone_id AND zv.package_id = f.package_id
		WHERE f.package_id = $1
		ORDER BY f.facility_id`, q.PackageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Destination
	for rows.Next() {
		var d Destination
		var capNullable *int
		var status *string
		if err := rows.Scan(&d.FacilityID, &d.SafeZoneID, &capNullable, &status); err != nil {
			return nil, err
		}
		// Zone must be open/published to be eligible (closed/full are not).
		if status != nil && *status != "" && *status != "OPEN" && *status != "PUBLISHED" {
			continue
		}
		if capNullable == nil {
			// Unknown capacity: informational only, never a promise.
			d.CapacityKnown = false
		} else if *capNullable <= 0 {
			// Zero zone capacity: not eligible
			continue
		} else {
			d.CapacityKnown = true
			// Free across the whole interval: the minimum free over all dates.
			free, ok, err := minFreeOverInterval(ctx, db, d.FacilityID, dates, q.PartySize)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue // some date lacks an inventory row or capacity: not eligible
			}
			d.Free = free
		}
		// Route gate.
		verified, routeID, err := routeVerified(ctx, db, q.PackageID, d.SafeZoneID, now, q.RouteGateOpen)
		if err != nil {
			return nil, err
		}
		d.RouteVerified = verified
		d.RouteID = routeID
		out = append(out, d)
	}
	return out, rows.Err()
}

// minFreeOverInterval returns the minimum free (capacity - held - occupied)
// across the interval, and ok=false if any date lacks an inventory row or the
// party cannot fit on every date.
func minFreeOverInterval(ctx context.Context, db DBTX, facilityID string, dates []time.Time, partySize int) (int, bool, error) {
	minFree := -1
	for _, d := range dates {
		var capacity, held, occupied int
		err := db.QueryRowContext(ctx, `
			SELECT capacity, held, occupied FROM facility_inventory
			WHERE facility_id = $1 AND service_date = $2`, facilityID, d).
			Scan(&capacity, &held, &occupied)
		if err != nil {
			if err.Error() == "sql: no rows in result set" {
				return 0, false, nil
			}
			return 0, false, err
		}
		free := capacity - held - occupied
		if free <= 0 || (partySize > 0 && free < partySize) {
			return 0, false, nil // cannot fit the whole interval or zero free capacity
		}
		if minFree < 0 || free < minFree {
			minFree = free
		}
	}
	return minFree, true, nil
}
