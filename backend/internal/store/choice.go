package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/sourceact"
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

// MaxChoiceDays is the maximum allowed temporary-stay duration for eligibility queries (30 days).
const MaxChoiceDays = 30

// ErrDateRangeExceeded is returned when the requested date range exceeds the policy maximum.
var ErrDateRangeExceeded = errors.New("store: date range exceeds policy maximum of 30 days")

func choiceDateRange(start, end time.Time) ([]time.Time, error) {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return nil, nil
	}
	if end.After(start.AddDate(0, 0, MaxChoiceDays)) {
		return nil, ErrDateRangeExceeded
	}
	var out []time.Time
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out, nil
}

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
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, &routeID, nil
}

// readPolicyOrderAndFacilityMap decodes the package body enough to drive
// allocation-policy ordering, validating that the package belongs to the
// requested jurisdiction, is current/effective/unexpired/not superseded,
// and has an operational source with live authorization for that scope.
// Quarantined, suspended, retired, or revoked source data cannot become
// current eligible destinations.
func readPolicyOrderAndFacilityMap(ctx context.Context, db DBTX, q ChoiceQuery, now time.Time) ([]string, map[string]string, contracts.FreshnessState, error) {
	var (
		pkgJurisdiction string
		body            []byte
		effectiveAt     time.Time
		expiresAt       time.Time
		supersededBy    sql.NullString
		sourceID        string
		pkgEvidence     string
		sourceState     sql.NullString
		authorized      bool
	)
	err := db.QueryRowContext(ctx, `
		SELECT p.jurisdiction, p.body, p.effective_at, p.expires_at, p.superseded_by,
		       p.source_id, p.evidence_class, s.state,
		       EXISTS (
		           SELECT 1 FROM source_authorizations sauth
		           WHERE sauth.source_id = p.source_id AND sauth.jurisdiction = p.jurisdiction
		             AND (sauth.expires_at IS NULL OR sauth.expires_at > $2)
		       ) AS authorized
		FROM packages p
		LEFT JOIN sources s ON s.source_id = p.source_id
		WHERE p.package_id = $1`, q.PackageID, now).Scan(
		&pkgJurisdiction, &body, &effectiveAt, &expiresAt, &supersededBy,
		&sourceID, &pkgEvidence, &sourceState, &authorized,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, contracts.FreshnessUnknown, fmt.Errorf("store: read package body: %w", ErrNoScopedContext)
		}
		return nil, nil, contracts.FreshnessUnknown, fmt.Errorf("store: read package body: %w", err)
	}

	// 1. Jurisdiction check: package must belong to requested jurisdiction.
	if q.Jurisdiction != "" && pkgJurisdiction != q.Jurisdiction {
		return nil, nil, contracts.FreshnessUnavailable, nil
	}

	// 2. Evidence class check: ordinary production requires AUTHORIZED_OPERATIONAL;
	// synthetic demo is permitted only when route gate / synthetic mode is open.
	if !q.RouteGateOpen && (pkgEvidence == "SYNTHETIC_DEMO" || pkgEvidence == "SYNTHETIC" || pkgEvidence != "AUTHORIZED_OPERATIONAL") {
		return nil, nil, contracts.FreshnessUnavailable, nil
	}

	// 3. Package currency & supersession check.
	if supersededBy.Valid && supersededBy.String != "" {
		return nil, nil, contracts.FreshnessStale, nil
	}
	if now.Before(effectiveAt) || !now.Before(expiresAt) {
		return nil, nil, contracts.FreshnessExpired, nil
	}

	// 4. Source operational state: source must be OPERATIONAL (not QUARANTINED, SUSPENDED, RETIRED, etc.).
	if !sourceState.Valid || sourceact.State(sourceState.String) != sourceact.Operational {
		return nil, nil, contracts.FreshnessUnavailable, nil
	}

	// 5. Source authorization check: active unexpired authorization in this jurisdiction.
	if !authorized {
		return nil, nil, contracts.FreshnessExpired, nil
	}

	var pb struct {
		Facilities []struct {
			ID         string `json:"id"`
			SafeZoneID string `json:"safe_zone_id"`
		} `json:"facilities"`
		AllocationPolicy struct {
			Order []string `json:"order"`
		} `json:"allocation_policy"`
	}
	if err := json.Unmarshal(body, &pb); err != nil {
		return nil, nil, contracts.FreshnessUnknown, fmt.Errorf("store: parse package body: %w", err)
	}
	facZone := make(map[string]string, len(pb.Facilities))
	for _, f := range pb.Facilities {
		if f.ID == "" {
			continue
		}
		facZone[f.ID] = f.SafeZoneID
	}
	return pb.AllocationPolicy.Order, facZone, contracts.FreshnessCurrent, nil
}

// ChoiceQuerier computes eligible destinations.
type ChoiceQuerier struct{}

// Eligible returns the destinations that satisfy every gate for the query,
// ordered by the package's authoritative allocation_policy.order (safe-zone
// IDs). All facilities in a zone share the same ordering rank (the zone's
// rank); within a zone, facilities are deterministically ordered by ID.
// A destination with unknown capacity is included with CapacityKnown=false
// (informational) but cannot be reserved. No destination is invented, ranked
// by guessed safety, or substituted.
func (q ChoiceQuerier) Eligible(ctx context.Context, db DBTX, query ChoiceQuery, now time.Time) ([]Destination, error) {
	dests, _, err := q.EligibleWithStatus(ctx, db, query, now)
	return dests, err
}

// EligibleWithStatus returns the eligible destinations along with the authoritative
// freshness state of the package context.
func (ChoiceQuerier) EligibleWithStatus(ctx context.Context, db DBTX, q ChoiceQuery, now time.Time) ([]Destination, contracts.FreshnessState, error) {
	dates, err := choiceDateRange(q.StartDate, q.EndDate)
	if err != nil {
		return nil, contracts.FreshnessUnknown, err
	}
	policyOrder, facZone, freshness, err := readPolicyOrderAndFacilityMap(ctx, db, q, now)
	if err != nil {
		return nil, freshness, err
	}
	if len(policyOrder) == 0 || len(facZone) == 0 {
		// No server-permitted order or no facilities: no destinations.
		// Never fabricate an alphabetical or guessed rank.
		return nil, freshness, nil
	}
	if len(dates) == 0 {
		// Browsing with no date range: every facility is reported with
		// unknown capacity (informational); the calling handler decides
		// whether the response should set `free = null`.
		dests, err := browseEligibleByPolicyOrder(ctx, db, q.PackageID, policyOrder, facZone, now, q)
		return dests, freshness, err
	}

	// Iterate in policy order. Each safe-zone ID contributes its facilities
	// (sorted by ID) in the same group, all sharing the same rank. Zone
	// status + capacity + party-fit are computed below; failing facilities
	// are silently dropped (informational browsing was a no-op here).
	var out []Destination
	zoneFacs := make(map[string][]string)
	for facID, szID := range facZone {
		zoneFacs[szID] = append(zoneFacs[szID], facID)
	}
	for _, szID := range policyOrder {
		facs := zoneFacs[szID]
		if len(facs) == 0 {
			continue
		}
		sort.Strings(facs)
		for _, facID := range facs {
			d, ok, err := facilityDestination(ctx, db, q, facID, szID, dates, now)
			if err != nil {
				return nil, freshness, err
			}
			if !ok {
				continue
			}
			out = append(out, d)
		}
	}
	return out, freshness, nil
}

// browseEligibleByPolicyOrder returns every facility in the package that
// sits in an OPEN/PUBLISHED zone with positive zone capacity, in
// allocation_policy.order. CapacityKnown=false and Free=0 (informational).
func browseEligibleByPolicyOrder(ctx context.Context, db DBTX, packageID string, policyOrder []string, facZone map[string]string, now time.Time, q ChoiceQuery) ([]Destination, error) {
	zoneFacs := make(map[string][]string)
	for facID, szID := range facZone {
		zoneFacs[szID] = append(zoneFacs[szID], facID)
	}
	var out []Destination
	for _, szID := range policyOrder {
		facs := zoneFacs[szID]
		if len(facs) == 0 {
			continue
		}
		sort.Strings(facs)
		for _, facID := range facs {
			d, ok, err := facilityBrowseEntry(ctx, db, packageID, facID, szID, now)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			out = append(out, d)
		}
	}
	return out, nil
}

// facilityDestination is one facility evaluated against the full query
// (party + dates + route gate). Returns ok=false if the facility is not
// eligible (closed/full/zero-capacity/missing-inventory/no-fit).
func facilityDestination(ctx context.Context, db DBTX, q ChoiceQuery, facID, szID string, dates []time.Time, now time.Time) (Destination, bool, error) {
	var capNullable *int
	var status *string
	if err := db.QueryRowContext(ctx, `
		SELECT zv.capacity, zv.status
		FROM zone_versions zv
		WHERE zv.zone_id = $1 AND zv.package_id = $2`,
		szID, q.PackageID).Scan(&capNullable, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Destination{}, false, nil
		}
		return Destination{}, false, err
	}
	if status != nil && *status != "" && *status != "OPEN" && *status != "PUBLISHED" {
		return Destination{}, false, nil
	}
	d := Destination{FacilityID: facID, SafeZoneID: szID}
	if capNullable == nil {
		d.CapacityKnown = false
	} else if *capNullable <= 0 {
		return Destination{}, false, nil
	} else {
		d.CapacityKnown = true
		free, ok, err := minFreeOverInterval(ctx, db, facID, dates, q.PartySize)
		if err != nil {
			return Destination{}, false, err
		}
		if !ok {
			return Destination{}, false, nil
		}
		d.Free = free
	}
	verified, routeID, err := routeVerified(ctx, db, q.PackageID, szID, now, q.RouteGateOpen)
	if err != nil {
		return Destination{}, false, err
	}
	d.RouteVerified = verified
	d.RouteID = routeID
	return d, true, nil
}

// facilityBrowseEntry is the browsing (no party / no dates) evaluation of one
// facility. Used when ChoiceQuery.StartDate/EndDate are zero / empty.
func facilityBrowseEntry(ctx context.Context, db DBTX, packageID, facID, szID string, now time.Time) (Destination, bool, error) {
	var capNullable *int
	var status *string
	if err := db.QueryRowContext(ctx, `
		SELECT zv.capacity, zv.status
		FROM zone_versions zv
		WHERE zv.zone_id = $1 AND zv.package_id = $2`,
		szID, packageID).Scan(&capNullable, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Destination{}, false, nil
		}
		return Destination{}, false, err
	}
	if status != nil && *status != "" && *status != "OPEN" && *status != "PUBLISHED" {
		return Destination{}, false, nil
	}
	if capNullable != nil && *capNullable <= 0 {
		return Destination{}, false, nil
	}
	d := Destination{
		FacilityID:    facID,
		SafeZoneID:    szID,
		CapacityKnown: false,
	}
	verified, routeID, err := routeVerified(ctx, db, packageID, szID, now, false)
	if err != nil {
		return Destination{}, false, err
	}
	d.RouteVerified = verified
	d.RouteID = routeID
	return d, true, nil
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
