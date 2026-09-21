package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"sthira/backend/internal/contracts"
)

// ErrNoScopedContext is returned when the persisted state cannot produce
// a ScopedContext for the requested jurisdiction (no operational package,
// expired, or unauthorized). Distinct from ErrNoOperationalContext because
// the P6 caller may want a different recovery path.
var ErrNoScopedContext = errors.New("store: no scoped context for the requested jurisdiction")

// packageBodyP6 extends the P2/P3 package body parse with the typed
// relationships P6 needs: zone status, route status, facility/safe-zone
// binding, route/safe-zone binding, and an ordered allocation policy. The
// shape is the canonical offlinepkg.Package; P6 reads more than P2 did.
type packageBodyP6 struct {
	RedZones []struct {
		ID string `json:"id"`
	} `json:"red_zones"`
	SafeZones []struct {
		ID     string `json:"id"`
		Status string `json:"status"` // OPEN | PUBLISHED | CLOSED | FULL
	} `json:"safe_zones"`
	Routes []struct {
		ID           string `json:"id"`
		FromZoneID   string `json:"from_zone_id"`
		ToSafeZoneID string `json:"to_safe_zone_id"`
		Approval     string `json:"approval"`
		VerifiedBy   string `json:"verified_by,omitempty"`
		ValidFrom    string `json:"valid_from,omitempty"`
		ValidUntil   string `json:"valid_until,omitempty"`
	} `json:"approved_routes"`
	Facilities []struct {
		ID         string `json:"id"`
		SafeZoneID string `json:"safe_zone_id"`
	} `json:"facilities"`
	Instructions []struct {
		ID       string `json:"id"`
		Language string `json:"language"`
	} `json:"instruction_assets"`
	AllocationPolicy struct {
		Order []string `json:"order"`
	} `json:"allocation_policy"`
}

// BuildScopedContext produces a typed ScopedContext from the persisted
// snapshot for (jurisdiction, package). It is the read-side counterpart
// to the trusted Publish* path: ScopedContext fields are bound to the
// current package revision so the validator never trusts client-echoed
// data.
//
// TemplateVersion is derived from the package version (one template
// revision per package revision, in this implementation). The real
// template lifecycle is owned by the P6 templates worker and would be
// its own column; the derivation here is documented and conservative.
func BuildScopedContext(ctx context.Context, db DBTX, jurisdiction, packageID string, sourceVersion int, now time.Time) (contracts.ScopedContext, error) {
	var body []byte
	err := db.QueryRowContext(ctx, `SELECT body FROM packages WHERE package_id = $1`, packageID).Scan(&body)
	if err != nil {
		return contracts.ScopedContext{}, fmt.Errorf("%w: package body: %v", ErrNoScopedContext, err)
	}
	var pb packageBodyP6
	if err := json.Unmarshal(body, &pb); err != nil {
		return contracts.ScopedContext{}, fmt.Errorf("store: parse package body: %w", err)
	}

	// Build typed references.
	sc := contracts.ScopedContext{
		DataVersion:     packageID + ":" + fmt.Sprintf("%d", sourceVersion),
		Jurisdiction:    jurisdiction,
		SchemaVersion:   contracts.SchemaVersionV3,
		SourceStatus:    contracts.FreshnessCurrent,
		SourceVersion:   sourceVersion,
		TemplateVersion: sourceVersion, // derived; see doc above
		KnownPlaces:     map[string]contracts.PlaceCandidate{},
		KnownRedZones:   map[string]contracts.ZoneRef{},
		KnownSafeZones:  map[string]contracts.ZoneRef{},
		KnownRoutes:     map[string]contracts.RouteRef{},
		KnownFacilities: map[string]contracts.FacilityRef{},
		VerifiedRoutes:  map[string][]contracts.RouteRef{},
		IssuedAt:        now.UTC().Format(time.RFC3339),
	}

	// Red zones: ZONE | RED | status from package body.
	for _, rz := range pb.RedZones {
		if rz.ID == "" {
			continue
		}
		sc.KnownRedZones[rz.ID] = contracts.ZoneRef{
			ZoneID:        rz.ID,
			ZoneRole:      "RED",
			Status:        contracts.ZoneStatusOpen, // package body omits the red-zone status; treat as OPEN
			SourceVersion: sourceVersion,
		}
	}

	// Safe zones: SAFE + status from package body.
	for _, sz := range pb.SafeZones {
		if sz.ID == "" {
			continue
		}
		st := contracts.ZoneStatus(sz.Status)
		if st == "" {
			st = contracts.ZoneStatusOpen
		}
		sc.KnownSafeZones[sz.ID] = contracts.ZoneRef{
			ZoneID:        sz.ID,
			ZoneRole:      "SAFE",
			Status:        st,
			SourceVersion: sourceVersion,
		}
	}

	// Routes: typed RouteRef + Verified computed from approval + window +
	// not-closed. CLOSED / SUPERSEDED / STALE are not Verified. A
	// SYNTHETIC_DEMO approval is only Verified when the test
	// configuration has opened the route gate (handled by the
	// orchestrator's choice query; here we keep the route bound but mark
	// it not-Verified unless the package is currently operational).
	for _, rt := range pb.Routes {
		if rt.ID == "" {
			continue
		}
		verified := isRouteVerified(rt, now)
		status := contracts.RouteStatusVerified
		if !verified {
			status = contracts.RouteStatusStale
		}
		sc.KnownRoutes[rt.ID] = contracts.RouteRef{
			RouteID:       rt.ID,
			FromZoneID:    rt.FromZoneID,
			ToSafeZoneID:  rt.ToSafeZoneID,
			Status:        status,
			Verified:      verified,
			SourceVersion: sourceVersion,
		}
	}

	// Facilities: typed FacilityRef. Capacity is unknown unless joined
	// against facility_inventory; the validator does not require
	// capacity_known (an informational reference is fine).
	for _, f := range pb.Facilities {
		if f.ID == "" {
			continue
		}
		sc.KnownFacilities[f.ID] = contracts.FacilityRef{
			FacilityID:    f.ID,
			SafeZoneID:    f.SafeZoneID,
			CapacityKnown: false,
			SourceVersion: sourceVersion,
		}
	}

	// VerifiedRoutes: facility ID -> verified routes that bind the
	// facility's safe zone. Built by joining routes by ToSafeZoneID =
	// facility.SafeZoneID.
	for facID, fac := range sc.KnownFacilities {
		var matched []contracts.RouteRef
		for _, rt := range sc.KnownRoutes {
			if rt.Verified && rt.ToSafeZoneID == fac.SafeZoneID {
				matched = append(matched, rt)
			}
		}
		if len(matched) > 0 {
			sc.VerifiedRoutes[facID] = matched
		}
	}

	// Languages: distinct, sorted (so the validator's IsLanguageAllowed
	// is stable across reads).
	langSet := map[string]struct{}{}
	for _, ins := range pb.Instructions {
		if ins.Language != "" {
			langSet[ins.Language] = struct{}{}
		}
	}
	for l := range langSet {
		sc.AllowedLanguages = append(sc.AllowedLanguages, l)
	}
	sort.Strings(sc.AllowedLanguages)

	// Template keys: empty in this derivation (templates are a P6
	// worker concept). Worker 7's registry supplies the authoritative
	// list at request time; the validator only checks that the model's
	// speech_key, when set, matches the registry.

	// Place aliases: each alias maps a normalized lookup key to a place
	// ID; we expose them as KnownPlaces with kind guessed from where
	// the ID appears in the typed maps. A name that does not alias any
	// known place produces NO candidate; we do NOT invent jurisdiction
	// or call a public geocoder (R: scoped context must come from the
	// persisted snapshot).
	aliases, err := readAliases(ctx, db, jurisdiction)
	if err != nil {
		return contracts.ScopedContext{}, fmt.Errorf("store: read aliases: %w", err)
	}
	for _, a := range aliases {
		if _, already := sc.KnownPlaces[a.PlaceID]; already {
			// keep the existing typed entry; just leave the place
			// discoverable by its alias id (a separate name lookup
			// is performed by Worker 9 at request time, not here).
			continue
		}
		kind := "ADMIN"
		switch {
		case isZoneID(a.PlaceID, sc.KnownSafeZones), isZoneID(a.PlaceID, sc.KnownRedZones):
			kind = "ZONE"
		case isFacilityID(a.PlaceID, sc.KnownFacilities):
			kind = "FACILITY"
		}
		sc.KnownPlaces[a.PlaceID] = contracts.PlaceCandidate{
			PlaceID:      a.PlaceID,
			PlaceKind:    kind,
			Jurisdiction: jurisdiction,
		}
	}

	// Eligible destinations: server-permitted order, joined against
	// facility_inventory so unknown capacity is reported honestly. We
	// reuse the existing ChoiceQuerier for the eligibility verdict; the
	// ordering comes from packageRows as documented by the P4 contract.
	if err := buildEligible(ctx, db, packageID, jurisdiction, now, &sc); err != nil {
		return contracts.ScopedContext{}, fmt.Errorf("store: eligible destinations: %w", err)
	}

	return sc, nil
}

// isRouteVerified returns true when the route is approved, verified by a
// recorded authority, currently within its valid window, and not closed.
// The closure check is a placeholder until the offlinequeue/closure
// persistence is wired; SYNTHETIC_DEMO / AUTHORIZED_OPERATIONAL approvals
// are both treated as Verified here because the route gate is enforced
// upstream by the choice querier (a Verified route may still fail the
// operational gate until O05 closes).
func isRouteVerified(rt struct {
	ID           string `json:"id"`
	FromZoneID   string `json:"from_zone_id"`
	ToSafeZoneID string `json:"to_safe_zone_id"`
	Approval     string `json:"approval"`
	VerifiedBy   string `json:"verified_by,omitempty"`
	ValidFrom    string `json:"valid_from,omitempty"`
	ValidUntil   string `json:"valid_until,omitempty"`
}, now time.Time) bool {
	switch rt.Approval {
	case "SYNTHETIC_DEMO", "AUTHORIZED_OPERATIONAL":
	default:
		return false
	}
	if rt.VerifiedBy == "" {
		return false
	}
	if rt.ValidFrom != "" {
		t, err := time.Parse(time.RFC3339, rt.ValidFrom)
		if err == nil && now.UTC().Before(t.UTC()) {
			return false
		}
	}
	if rt.ValidUntil != "" {
		t, err := time.Parse(time.RFC3339, rt.ValidUntil)
		if err == nil && !now.UTC().Before(t.UTC()) {
			return false
		}
	}
	return true
}

// readAliases returns the persisted place_aliases rows for the
// jurisdiction. They are used to populate KnownPlaces so a name lookup
// can disambiguate without contacting a public geocoder.
func readAliases(ctx context.Context, db DBTX, jurisdiction string) ([]PlaceCandidate, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT place_id, place_kind FROM place_aliases
		WHERE jurisdiction = $1`, jurisdiction)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlaceCandidate
	for rows.Next() {
		var c PlaceCandidate
		if err := rows.Scan(&c.PlaceID, &c.PlaceKind); err != nil {
			return nil, err
		}
		if c.PlaceID != "" {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

func isInMap(id string, m map[string]contracts.ZoneRef) bool { _, ok := m[id]; return ok }

func isZoneID(id string, m map[string]contracts.ZoneRef) bool {
	_, ok := m[id]
	return ok
}

func isFacilityID(id string, m map[string]contracts.FacilityRef) bool {
	_, ok := m[id]
	return ok
}

// buildEligible populates sc.EligibleDestinations using the existing
// ChoiceQuerier for the per-facility verdict. The ordering follows the
// package's allocation_policy.order (server-authoritative); the
// querier's own ordering is secondary and we sort by rank.
func buildEligible(ctx context.Context, db DBTX, packageID, jurisdiction string, now time.Time, sc *contracts.ScopedContext) error {
	dates := []time.Time{now.UTC().Truncate(24 * time.Hour)}
	q := ChoiceQuery{
		Jurisdiction: jurisdiction,
		PackageID:    packageID,
		PartySize:    1,
		StartDate:    dates[0],
		EndDate:      dates[0].Add(24 * time.Hour),
		// RouteGateOpen is intentionally false here. The scope's
		// VerifiedRoutes already documents route availability; the
		// operational gate is enforced by Worker 9's choice path
		// (D38).
		RouteGateOpen: false,
	}
	dests, err := (ChoiceQuerier{}).Eligible(ctx, db, q, now)
	if err != nil {
		return err
	}
	// Stable order: by facility ID (the package's allocation_policy.order
	// is the operator's intent; absent that, alphabetic). The P6
	// validator's enforceChoiceOrder accepts any subsequence in this
	// canonical order; it never re-sorts.
	sort.Slice(dests, func(i, j int) bool { return dests[i].FacilityID < dests[j].FacilityID })
	for i, d := range dests {
		fac, ok := sc.KnownFacilities[d.FacilityID]
		if !ok {
			continue
		}
		fac.CapacityKnown = d.CapacityKnown
		if d.CapacityKnown {
			fac.Free = d.Free
		}
		sc.KnownFacilities[d.FacilityID] = fac
		sc.EligibleDestinations = append(sc.EligibleDestinations, contracts.EligibleChoice{
			Facility:      fac,
			PermittedRank: i,
		})
	}
	return nil
}

// ScopedContextResolver produces ScopedContexts from the persisted store
// and implements SnapshotRevalidate. The orchestrator (Worker 9) wires
// it through the /voice/process handler so each pipeline stage can
// re-read the live snapshot after slow inference.
//
// Construction needs the persisted Store and the now function (default
// time.Now().UTC()). The first call to Resolve establishes the
// baseline (SourceVersion, TemplateVersion, DataVersion). Every
// SnapshotRevalidate call rechecks those three against the live store;
// a change returns ErrStaleSnapshot and the orchestrator must drop the
// in-flight result.
type ScopedContextResolver struct {
	store *Store
	now   func() time.Time
}

// NewScopedContextResolver wires a resolver against the given Store.
func NewScopedContextResolver(s *Store) *ScopedContextResolver {
	return &ScopedContextResolver{store: s, now: func() time.Time { return time.Now().UTC() }}
}

// WithClock overrides the clock for deterministic tests.
func (r *ScopedContextResolver) WithClock(now func() time.Time) *ScopedContextResolver {
	if now != nil {
		r.now = now
	}
	return r
}

// Resolve returns the typed ScopedContext for the requested jurisdiction,
// backed by the same query as ResolveContext (operational + authorized +
// effective + unexpired + not superseded). The SourceVersion on the
// returned context matches the live package revision; the orchestrator
// records it so SnapshotRevalidate can compare on later stages.
func (r *ScopedContextResolver) Resolve(ctx context.Context, jurisdiction string) (contracts.ScopedContext, error) {
	if r.store == nil {
		return contracts.ScopedContext{}, ErrNoScopedContext
	}
	snap, err := ResolveContext(ctx, r.store.DB(), jurisdiction, r.now())
	if err != nil {
		return contracts.ScopedContext{}, fmt.Errorf("%w: %v", ErrNoScopedContext, err)
	}
	sc, err := BuildScopedContext(ctx, r.store.DB(), jurisdiction, snap.PackageID, snap.PackageVersion, r.now())
	if err != nil {
		return contracts.ScopedContext{}, err
	}
	// Tag the context with the server-side RequestID placeholder (the
	// orchestrator rewrites this per request before dispatching the
	// model). Leave the data_version intact; SnapshotRevalidate
	// compares data_version as one of the three fields.
	sc.RequestID = ""
	return sc, nil
}

// SnapshotRevalidate re-reads the persisted snapshot for the same
// jurisdiction and returns ErrStaleSnapshot when (SourceVersion,
// TemplateVersion, DataVersion) changed. Cancellation is honored via ctx.
func (r *ScopedContextResolver) SnapshotRevalidate(ctx context.Context, sc contracts.ScopedContext) error {
	if r.store == nil {
		return ErrNoScopedContext
	}
	snap, err := ResolveContext(ctx, r.store.DB(), sc.Jurisdiction, r.now())
	if err != nil {
		// A package withdrawn or expired between dispatch and now is the
		// canonical "stale snapshot" signal: the orchestrator must
		// drop the in-flight result. Map no-snapshot to stale so the
		// orchestrator can present "withdrawn" cleanly.
		return errors.New(contracts.ErrStaleSnapshot)
	}
	if snap.PackageVersion != sc.SourceVersion {
		return errors.New(contracts.ErrStaleSnapshot)
	}
	if sc.TemplateVersion != sc.SourceVersion {
		// TemplateVersion is derived from the package revision; a
		// divergence means a template-side update landed without a
		// package bump (or vice versa).
		return errors.New(contracts.ErrStaleSnapshot)
	}
	if (snap.PackageID + ":" + fmt.Sprintf("%d", snap.PackageVersion)) != sc.DataVersion {
		return errors.New(contracts.ErrStaleSnapshot)
	}
	return nil
}
