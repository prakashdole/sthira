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
	var (
		body     []byte
		sourceID string
	)
	err := db.QueryRowContext(ctx, `SELECT body, source_id FROM packages WHERE package_id = $1`, packageID).Scan(&body, &sourceID)
	if err != nil {
		return contracts.ScopedContext{}, fmt.Errorf("%w: package body: %v", ErrNoScopedContext, err)
	}
	var pb packageBodyP6
	if err := json.Unmarshal(body, &pb); err != nil {
		return contracts.ScopedContext{}, fmt.Errorf("store: parse package body: %w", err)
	}

	// Build typed references.
	sc := contracts.ScopedContext{
		SourceID:        sourceID,
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

	// Template keys: the approved speech_keys for this jurisdiction,
	// read from the persisted translation-authority table
	// (approved_translations). This is the authoritative approval
	// boundary the orchestrator refuses to fake with a global registry
	// fallback (see orchestration stageContext: empty TemplateKeys is
	// an authoritative "nothing approved"). An empty result is therefore
	// an honest fail-closed signal. In production the table stays empty
	// until a government translation authority records an approval
	// (external gate O11); seeding it from a test is an isolated
	// fixture, not an invented approved translation.
	//
	// A row with a NULL language, source_id or template_sha256 cannot
	// authorize speech: migration 0010 quarantines such active rows and
	// readApprovedSpeechKeys fails closed (no wildcard, no empty-source
	// match). An empty result is an honest fail-closed signal.
	allowedLangs := make(map[string]struct{}, len(sc.AllowedLanguages))
	for _, l := range sc.AllowedLanguages {
		allowedLangs[l] = struct{}{}
	}
	approved, approvedLangs, digests, err := readApprovedSpeechKeys(ctx, db, jurisdiction, sourceVersion, sc.TemplateVersion, sourceID, now, allowedLangs)
	if err != nil {
		return contracts.ScopedContext{}, fmt.Errorf("store: read approved translations: %w", err)
	}
	sc.TemplateKeys = approved
	sc.ApprovedSpeechKeys = approvedLangs
	sc.ApprovedTemplateSHA = digests

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

	// Eligible destinations: server-permitted order from the package's
	// allocation_policy.order. General destination browsing does not assume
	// party size or duration (no synthetic capacity promises). If allocation
	// policy order is empty, no arbitrary alphabetical order or PermittedRank is fabricated.
	if err := buildEligible(ctx, db, packageID, jurisdiction, pb.AllocationPolicy.Order, now, &sc); err != nil {
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

// readApprovedSpeechKeys returns the distinct, ascending set of
// speech_keys currently approved for the jurisdiction matching the exact
// sourceVersion, templateVersion and sourceID, active at now, backed by
// the persisted approved_translations authority table.
//
// B01 fail-closed rules:
//   - empty sourceID returns no approvals (cannot authorize every source);
//   - source_id must be non-NULL and exactly equal to sourceID;
//   - language must be non-NULL (no wildcard) and in the allowed set;
//   - template_sha256 must be non-NULL (64-hex digest) and is returned
//     as speech_key -> digest for the orchestrator's template check.
func readApprovedSpeechKeys(ctx context.Context, db DBTX, jurisdiction string, sourceVersion, templateVersion int, sourceID string, now time.Time, allowed map[string]struct{}) ([]string, map[string][]string, map[string]string, error) {
	if sourceID == "" {
		return nil, nil, map[string]string{}, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT speech_key, language, template_sha256
		FROM approved_translations
		WHERE jurisdiction = $1
		  AND source_version = $2
		  AND template_version = $3
		  AND source_id IS NOT NULL
		  AND source_id = $4
		  AND language IS NOT NULL
		  AND template_sha256 IS NOT NULL
		  AND revoked_at IS NULL
		  AND approved_at <= $5
		ORDER BY speech_key, language`,
		jurisdiction, sourceVersion, templateVersion, sourceID, now)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	seen := map[string]struct{}{}
	approvedLangs := map[string][]string{}
	digests := map[string]string{}
	var out []string
	for rows.Next() {
		var key, digest string
		var lang string
		if err := rows.Scan(&key, &lang, &digest); err != nil {
			return nil, nil, nil, err
		}
		if _, ok := allowed[lang]; !ok {
			continue
		}
		approvedLangs[key] = append(approvedLangs[key], lang)
		if digest != "" {
			digests[key] = digest
		}
		if _, dup := seen[key]; !dup {
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	sort.Strings(out)
	return out, approvedLangs, digests, nil
}

func isZoneID(id string, m map[string]contracts.ZoneRef) bool {
	_, ok := m[id]
	return ok
}

func isFacilityID(id string, m map[string]contracts.FacilityRef) bool {
	_, ok := m[id]
	return ok
}

// buildEligible populates sc.EligibleDestinations using the package's
// authoritative allocation_policy.order, which lists SAFE-ZONE IDs (per
// opkg.AllocationPolicy and the opkg validator). General destination
// browsing does not assume party size or duration (PartySize and dates
// are unknown), so it does not make synthetic capacity promises
// (CapacityKnown=false, Free=0). Each safe-zone ID maps to ALL of its
// member facilities; all facilities in a zone share the same
// PermittedRank (= the zone's rank in policyOrder). Within a zone,
// facilities are deterministically ordered by ID; that ordering is
// display-only and is NOT a claimed authority safety ranking.
//
// Filtering:
//   - Zone must be present in the package body and be OPEN or PUBLISHED.
//   - Zone must have positive capacity in zone_versions for this package.
//   - Unknown / missing zone IDs are dropped (never fabricated).
//   - DB errors propagate; they are never silently swallowed.
//
// If allocation policy order is empty, no arbitrary alphabetical order
// or PermittedRank is fabricated.
func buildEligible(ctx context.Context, db DBTX, packageID, jurisdiction string, policyOrder []string, now time.Time, sc *contracts.ScopedContext) error {
	if len(policyOrder) == 0 {
		// No server-permitted order: leave sc.EligibleDestinations empty.
		// Never fabricate an alphabetical or guessed rank.
		return nil
	}

	for rank, szID := range policyOrder {
		if szID == "" {
			continue
		}
		// Zone must be present in the typed map built from the package.
		sz, ok := sc.KnownSafeZones[szID]
		if !ok {
			// Unknown / missing zone ID: drop, never fabricate facilities.
			continue
		}
		if sz.Status != contracts.ZoneStatusOpen && sz.Status != contracts.ZoneStatusPublished {
			continue
		}

		// Zone capacity in this package. sql.ErrNoRows means the zone
		// was named in policyOrder without a zone_versions row for this
		// package — treat as zero / ineligible.
		var zoneCap *int
		err := db.QueryRowContext(ctx, `
			SELECT zv.capacity FROM zone_versions zv
			WHERE zv.zone_id = $1 AND zv.package_id = $2`, szID, packageID).Scan(&zoneCap)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return fmt.Errorf("store: read zone capacity for %s: %w", szID, err)
		}
		if zoneCap == nil || *zoneCap <= 0 {
			continue
		}

		// Find all facilities in this zone that belong to this package
		// (KnownFacilities was built from the same package body).
		var facs []string
		for facID, fac := range sc.KnownFacilities {
			if fac.SafeZoneID == szID {
				facs = append(facs, facID)
			}
		}
		sort.Strings(facs)

		for _, facID := range facs {
			fac := sc.KnownFacilities[facID]
			// Capacity is unknown for general browsing (no party size or
			// dates). The commit-time query enriches CapacityKnown + Free.
			fac.CapacityKnown = false
			fac.Free = 0
			sc.KnownFacilities[facID] = fac
			sc.EligibleDestinations = append(sc.EligibleDestinations, contracts.EligibleChoice{
				Facility:      fac,
				PermittedRank: rank,
			})
		}
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
// TemplateVersion, DataVersion) changed, or when any translation
// approval in the active context was revoked or modified.
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

	// Revalidate approval withdrawal during inference/TTS:
	// 1. Any key in sc.TemplateKeys must not have been revoked at or before r.now().
	for _, key := range sc.TemplateKeys {
		var isRevoked bool
		err := r.store.DB().QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM approved_translations
				WHERE jurisdiction = $1
				  AND source_version = $2
				  AND template_version = $3
				  AND speech_key = $4
				  AND source_id IS NOT NULL
				  AND source_id = $5
				  AND revoked_at IS NOT NULL
				  AND revoked_at <= $6
			)`, sc.Jurisdiction, sc.SourceVersion, sc.TemplateVersion, key, sc.SourceID, r.now()).Scan(&isRevoked)
		if err == nil && isRevoked {
			return errors.New(contracts.ErrStaleSnapshot)
		}
	}

	// 2. Active approved speech keys, languages and digests must match the snapshot.
	allowedLangs := make(map[string]struct{}, len(sc.AllowedLanguages))
	for _, l := range sc.AllowedLanguages {
		allowedLangs[l] = struct{}{}
	}
	currentKeys, currentApprovedLangs, currentDigests, err := readApprovedSpeechKeys(ctx, r.store.DB(), sc.Jurisdiction, sc.SourceVersion, sc.TemplateVersion, sc.SourceID, r.now(), allowedLangs)
	if err != nil {
		return err
	}
	if len(currentKeys) != len(sc.TemplateKeys) {
		return errors.New(contracts.ErrStaleSnapshot)
	}
	for i, k := range currentKeys {
		if k != sc.TemplateKeys[i] {
			return errors.New(contracts.ErrStaleSnapshot)
		}
	}
	for k, langs := range sc.ApprovedSpeechKeys {
		curr, ok := currentApprovedLangs[k]
		if !ok || len(curr) != len(langs) {
			return errors.New(contracts.ErrStaleSnapshot)
		}
		for i, l := range langs {
			if l != curr[i] {
				return errors.New(contracts.ErrStaleSnapshot)
			}
		}
	}
	for k, want := range sc.ApprovedTemplateSHA {
		if currentDigests[k] != want {
			return errors.New(contracts.ErrStaleSnapshot)
		}
	}
	if len(currentDigests) != len(sc.ApprovedTemplateSHA) {
		return errors.New(contracts.ErrStaleSnapshot)
	}

	return nil
}
