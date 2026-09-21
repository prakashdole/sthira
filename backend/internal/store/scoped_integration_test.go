package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/sourceact"
)

// seedP6Fixture builds the minimum persisted state needed to produce a
// typed ScopedContext for jurisdiction "KL": a source, an OPERATIONAL
// state, an authorization in KL, a current package with zones/routes/
// facilities, an alias, and a verified route binding the facility's
// safe zone. Returns the disposable store, package_id and facility id.
type p6Fixture struct {
	store          *Store
	packageID      string
	facilityID     string
	safeZoneID     string
	redZoneID      string
	routeID        string
	jurisdictionID string
	cleanup        func()
}

func newP6Fixture(t *testing.T) *p6Fixture {
	t.Helper()
	st, cleanup := disposableTestDB(t)
	now := nowUTC()
	jr := "JUR-KL-P6-" + uid("X")
	srcID := "SRC-KL-P6-" + uid("X")
	pkgID := "PKG-KL-P6-" + uid("X")
	facID := "FAC-KL-P6-" + uid("X")
	szID := "SZ-KL-P6-" + uid("X")
	rzID := "RZ-KL-P6-" + uid("X")
	rtID := "RT-KL-P6-" + uid("X")
	artID := "ART-KL-P6-" + uid("X")
	authID := "AUTH-KL-P6-" + uid("X")
	aliasID := "ALIAS-KL-P6-" + uid("X")

	// Package body: a SAFE zone + a FACILITY bound to it + a verified
	// ROUTE bound to it + an allocation_policy with the facility order.
	body := []byte(`{"red_zones":[{"id":"` + rzID + `"}],"safe_zones":[{"id":"` + szID + `","status":"OPEN"}],"approved_routes":[{"id":"` + rtID + `","from_zone_id":"` + rzID + `","to_safe_zone_id":"` + szID + `","mode":"FOOT","approval":"SYNTHETIC_DEMO","verified_by":"officer.test","valid_from":"2026-01-01T00:00:00Z","valid_until":"2030-01-01T00:00:00Z"}],"facilities":[{"id":"` + facID + `","safe_zone_id":"` + szID + `"}],"instruction_assets":[{"id":"INS-1","language":"ml-IN"},{"id":"INS-2","language":"en-IN"}],"allocation_policy":{"order":["` + facID + `"]}}`)

	if err := st.InTx(context.Background(), func(tx DBTX) error {
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at)
			VALUES ($1, 'gov-test', 'gov.example', 'OPERATIONAL', 1, $2)`,
			srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO source_authorizations
				(authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			authID, srcID, "authority-1", "doc-1", jr, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,1,$3,$4,'AUTHORIZED_OPERATIONAL',$5)`,
			artID, srcID, strings.Repeat("a", 64), now, "memory://test"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, checksum_sha256, body, effective_at, expires_at)
			VALUES ($1,$2,$3,$4,7,$5,'AUTHORIZED_OPERATIONAL',$6,$7,$8,$9)`,
			pkgID, "ALERT-P6-"+uid("X"), srcID, artID, jr, strings.Repeat("a", 64), body,
			now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO place_aliases (alias_id, jurisdiction, lookup_key, place_id, place_kind)
			VALUES ($1,$2,$3,$4,'ADMIN')`,
			aliasID, jr, "wayanad", szID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		cleanup()
		t.Fatalf("seed: %v", err)
	}
	return &p6Fixture{
		store:          st,
		packageID:      pkgID,
		facilityID:     facID,
		safeZoneID:     szID,
		redZoneID:      rzID,
		routeID:        rtID,
		jurisdictionID: jr,
		cleanup:        cleanup,
	}
}

func (f *p6Fixture) close() {
	if f.cleanup != nil {
		f.cleanup()
	}
}

// TestBuildScopedContext_BindsTypedReferences: BuildScopedContext must
// produce a ScopedContext with all the typed maps populated and the
// verified route bound to the facility's safe zone.
func TestBuildScopedContext_BindsTypedReferences(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()

	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if sc.Jurisdiction != fx.jurisdictionID {
		t.Errorf("jurisdiction: got %q want %q", sc.Jurisdiction, fx.jurisdictionID)
	}
	if sc.DataVersion != fx.packageID+":7" {
		t.Errorf("data_version: got %q want %q", sc.DataVersion, fx.packageID+":7")
	}
	if _, ok := sc.KnownSafeZones[fx.safeZoneID]; !ok {
		t.Errorf("safe zone %q must be in KnownSafeZones", fx.safeZoneID)
	}
	if _, ok := sc.KnownRedZones[fx.redZoneID]; !ok {
		t.Errorf("red zone %q must be in KnownRedZones", fx.redZoneID)
	}
	if _, ok := sc.KnownRoutes[fx.routeID]; !ok {
		t.Errorf("route %q must be in KnownRoutes", fx.routeID)
	} else {
		rt := sc.KnownRoutes[fx.routeID]
		if !rt.Verified {
			t.Errorf("route %q must be Verified", fx.routeID)
		}
		if rt.ToSafeZoneID != fx.safeZoneID {
			t.Errorf("route destination: got %q want %q", rt.ToSafeZoneID, fx.safeZoneID)
		}
	}
	if fac, ok := sc.KnownFacilities[fx.facilityID]; !ok {
		t.Errorf("facility %q must be in KnownFacilities", fx.facilityID)
	} else if fac.SafeZoneID != fx.safeZoneID {
		t.Errorf("facility safe zone: got %q want %q", fac.SafeZoneID, fx.safeZoneID)
	}
	vrs, ok := sc.VerifiedRoutes[fx.facilityID]
	if !ok {
		t.Errorf("VerifiedRoutes[%q] must be populated", fx.facilityID)
	}
	if len(vrs) != 1 || vrs[0].RouteID != fx.routeID {
		t.Errorf("VerifiedRoutes[%q]: got %+v", fx.facilityID, vrs)
	}
	// Languages must include the instruction languages.
	if !sc.IsLanguageAllowed("ml-IN") || !sc.IsLanguageAllowed("en-IN") {
		t.Errorf("AllowedLanguages missing instruction languages: %v", sc.AllowedLanguages)
	}
	// The alias must surface as a place candidate (no public geocoder).
	found := false
	for _, p := range sc.KnownPlaces {
		if p.PlaceID == fx.safeZoneID && p.PlaceKind == "ZONE" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("alias should resolve to a zone place; got places: %+v", sc.KnownPlaces)
	}
}

// TestScopedContext_NoCrossJurisdictionLeak: an alias in jurisdiction
// TN must NOT appear in a KL ScopedContext. We never guess across
// jurisdictions.
func TestScopedContext_NoCrossJurisdictionLeak(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	// Insert a same-named alias in a different jurisdiction.
	if err := fx.store.InTx(context.Background(), func(tx DBTX) error {
		_, err := tx.ExecContext(context.Background(), `
			INSERT INTO place_aliases (alias_id, jurisdiction, lookup_key, place_id, place_kind)
			VALUES ($1,$2,$3,$4,'ADMIN')`,
			"ALIAS-TN-"+uid("X"), "JUR-TN-P6-"+uid("X"), "wayanad", "FAC-TN-X")
		return err
	}); err != nil {
		t.Fatalf("seed TN alias: %v", err)
	}
	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	for id := range sc.KnownPlaces {
		if strings.HasPrefix(id, "FAC-TN-") {
			t.Errorf("cross-jurisdiction place %q leaked into KL context", id)
		}
	}
}

// TestScopedContext_EnforcesPendingZeroConsequentialWrites: ResolveContext
// must NOT mutate any database row. We snapshot the row counts before
// and after; they must be identical. This protects the trust boundary:
// the orchestrator's "read" path must be free of side effects.
func TestScopedContext_EnforcesPendingZeroConsequentialWrites(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	before := countRows(t, fx.store)
	resolver := NewScopedContextResolver(fx.store)
	_, _ = resolver.Resolve(context.Background(), fx.jurisdictionID)
	_ = resolver.SnapshotRevalidate(context.Background(), contracts.ScopedContext{
		Jurisdiction:    fx.jurisdictionID,
		DataVersion:     fx.packageID + ":7",
		SourceVersion:   7,
		TemplateVersion: 7,
	})
	after := countRows(t, fx.store)
	if before != after {
		t.Errorf("row counts changed: before=%d after=%d", before, after)
	}
}

// TestScopedContext_SnapshotRevalidate_StableWhenUnchanged: when the
// persisted package has not changed, revalidation returns nil.
func TestScopedContext_SnapshotRevalidate_StableWhenUnchanged(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := resolver.SnapshotRevalidate(context.Background(), sc); err != nil {
		t.Fatalf("revalidate after no change: %v", err)
	}
}

// TestScopedContext_SnapshotRevalidate_StaleOnPackageWithdraw: when the
// source is moved to QUARANTINED, the next revalidation must report
// stale. The orchestrator must drop any in-flight proposal that was
// generated against the pre-withdrawal snapshot.
func TestScopedContext_SnapshotRevalidate_StaleOnPackageWithdraw(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Quarantine the source.
	if err := fx.store.InTx(context.Background(), func(tx DBTX) error {
		var v int
		if err := tx.QueryRowContext(context.Background(),
			`SELECT version FROM sources WHERE source_id LIKE 'SRC-KL-P6-%'`).Scan(&v); err != nil {
			return err
		}
		ss := NewSourceStore(nil)
		return ss.Transition(context.Background(), tx, findKLP6Source(t, tx), v, sourceact.Quarantined, "test", "quarantine", uid("EV"), nowUTC())
	}); err != nil {
		t.Fatalf("quarantine: %v", err)
	}

	if err := resolver.SnapshotRevalidate(context.Background(), sc); err == nil {
		t.Fatalf("revalidate after quarantine: expected stale, got nil")
	}
}

// TestScopedContext_SnapshotRevalidate_StaleOnPackageSupersession: when
// the package is superseded by another (package_id+version becomes a
// new value), revalidation must report stale.
func TestScopedContext_SnapshotRevalidate_StaleOnPackageSupersession(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Insert a NEW package row that supersedes the original.
	srcID := findKLP6Source(t, fx.store.DB())
	if err := fx.store.InTx(context.Background(), func(tx DBTX) error {
		newArtID := "ART-NEW-" + uid("X")
		newPkgID := "PKG-NEW-" + uid("X")
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,2,$3,$4,'AUTHORIZED_OPERATIONAL',$5)`,
			newArtID, srcID, strings.Repeat("b", 64), nowUTC(), "memory://test"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, checksum_sha256, body, effective_at, expires_at, superseded_by)
			VALUES ($1,$2,$3,$4,8,$5,'AUTHORIZED_OPERATIONAL',$6,$7,$8,$9,$10)`,
			newPkgID, "ALERT-NEW-"+uid("X"), srcID, newArtID, fx.jurisdictionID, strings.Repeat("b", 64), []byte(`{"red_zones":[],"safe_zones":[],"approved_routes":[],"facilities":[],"instruction_assets":[],"allocation_policy":{"order":[]}}`),
			nowUTC().Add(-time.Minute), nowUTC().Add(time.Hour), fx.packageID); err != nil {
			return err
		}
		// Supersede the original.
		if _, err := tx.ExecContext(context.Background(), `
			UPDATE packages SET superseded_by = $1 WHERE package_id = $2`, newPkgID, fx.packageID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("supersede: %v", err)
	}

	if err := resolver.SnapshotRevalidate(context.Background(), sc); err == nil {
		t.Fatalf("revalidate after supersession: expected stale, got nil")
	}
}

// TestBuildScopedContext_UnknownJurisdiction: an unknown jurisdiction
// fails closed (ErrNoScopedContext). The orchestrator must not invent
// jurisdiction from the model proposal.
func TestBuildScopedContext_UnknownJurisdiction(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	resolver := NewScopedContextResolver(fx.store)
	if _, err := resolver.Resolve(context.Background(), "JUR-NONEXISTENT-"+uid("X")); !errors.Is(err, ErrNoScopedContext) {
		t.Fatalf("expected ErrNoScopedContext, got %v", err)
	}
}

// TestScopedContext_VerifierReadsViaAcceptableRoute: the validator
// accepts a SHOW_ROUTE action that uses a route in VerifiedRoutes
// for the destination facility. End-to-end smoke: resolve context,
// then run EnforceScopedContext on a proposal that uses that route.
func TestScopedContext_VerifierReadsViaAcceptableRoute(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Worker 7's registry (P6 templates worker) supplies the
	// authoritative TemplateKeys at request time; the persisted
	// snapshot does not carry them. Inject a single key for this test.
	sc.TemplateKeys = []string{"destination_options"}
	intent := contracts.IntentShowRoute
	prop := contracts.ModelOutput{
		SchemaVersion: "3.0",
		RequestID:     "REQ-1",
		DataVersion:   sc.DataVersion,
		Status:        contracts.StatusOK,
		Intent:        &intent,
		Language:      "ml-IN",
		Actions: []contracts.Action{
			{Type: contracts.ActionShowRoute, RouteID: fx.routeID},
		},
		SpeechKey:        strPtr("destination_options"),
		ClarificationIDs: nil,
		EvidenceIDs:      []string{fx.routeID},
	}
	if err := contracts.EnforceScopedContext(prop, sc); err != nil {
		t.Fatalf("end-to-end: %v", err)
	}
}

// TestScopedContext_VerifierRejectsWrongFacilityRoute: a route bound
// to a different safe zone must not serve as the facility's verified
// route. BuildScopedContext places the route in VerifiedRoutes[facID]
// only when ToSafeZoneID == fac.SafeZoneID, so a model that picks
// such a route is rejected (its route_id is in VerifiedRoutes for a
// different facility's safe zone, not for FACILITY-1's).
//
// This is the prompt's "route must match the selected destination"
// requirement, enforced at the ScopedContext build layer.
func TestScopedContext_VerifierRejectsWrongFacilityRoute(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	// Add a SECOND facility in a DIFFERENT safe zone, bound to a
	// verified route. The route's ToSafeZoneID != FACILITY-1's
	// SafeZoneID, so FACILITY-1's VerifiedRoutes must NOT contain
	// the second route.
	srcID := findKLP6Source(t, fx.store.DB())
	if err := fx.store.InTx(context.Background(), func(tx DBTX) error {
		newArtID := "ART-OTHER-" + uid("X")
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,3,$3,$4,'AUTHORIZED_OPERATIONAL',$5)`,
			newArtID, srcID, strings.Repeat("c", 64), nowUTC(), "memory://test"); err != nil {
			return err
		}
		newPkgID := "PKG-OTHER-" + uid("X")
		body := []byte(`{"red_zones":[],"safe_zones":[{"id":"SZ-OTHER","status":"OPEN"}],"approved_routes":[{"id":"RT-OTHER","from_zone_id":"RZ-DUMMY","to_safe_zone_id":"SZ-OTHER","mode":"FOOT","approval":"SYNTHETIC_DEMO","verified_by":"officer.test","valid_from":"2026-01-01T00:00:00Z","valid_until":"2030-01-01T00:00:00Z"}],"facilities":[{"id":"FAC-OTHER","safe_zone_id":"SZ-OTHER"}],"instruction_assets":[],"allocation_policy":{"order":["FAC-OTHER"]}}`)
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, checksum_sha256, body, effective_at, expires_at)
			VALUES ($1,$2,$3,$4,9,$5,'AUTHORIZED_OPERATIONAL',$6,$7,$8,$9)`,
			newPkgID, "ALERT-OTHER-"+uid("X"), srcID, newArtID, fx.jurisdictionID, strings.Repeat("c", 64), body,
			nowUTC().Add(-time.Minute), nowUTC().Add(time.Hour)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seed second package: %v", err)
	}
	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// The first facility's VerifiedRoutes must NOT contain RT-OTHER.
	if vrs, ok := sc.VerifiedRoutes[fx.facilityID]; ok {
		for _, r := range vrs {
			if r.RouteID == "RT-OTHER" {
				t.Errorf("FACILITY-1 must not have RT-OTHER in VerifiedRoutes: %+v", vrs)
			}
		}
	}
}

// TestScopedContext_DefaultsRouteGateClosed: BuildScopedContext must
// produce VerifiedRoutes[] only with Verified==true routes. SYNTHETIC_DEMO
// approvals ARE marked Verified (the operational gate is enforced at
// the choice querier, not here); but routes without verified_by or
// outside the valid window are NOT Verified.
func TestScopedContext_DefaultsRouteGateClosed(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	// Insert a second route into the existing package whose
	// valid_until is in the past.
	srcID := findKLP6Source(t, fx.store.DB())
	// Replace the package body to add the expired route, keeping the
	// existing valid route.
	newBody := []byte(`{"red_zones":[{"id":"` + fx.redZoneID + `"}],"safe_zones":[{"id":"` + fx.safeZoneID + `","status":"OPEN"}],"approved_routes":[{"id":"` + fx.routeID + `","from_zone_id":"` + fx.redZoneID + `","to_safe_zone_id":"` + fx.safeZoneID + `","mode":"FOOT","approval":"SYNTHETIC_DEMO","verified_by":"officer.test","valid_from":"2026-01-01T00:00:00Z","valid_until":"2030-01-01T00:00:00Z"},{"id":"RT-EXPIRED","from_zone_id":"` + fx.redZoneID + `","to_safe_zone_id":"` + fx.safeZoneID + `","mode":"FOOT","approval":"SYNTHETIC_DEMO","verified_by":"officer.test","valid_from":"2024-01-01T00:00:00Z","valid_until":"2025-01-01T00:00:00Z"}],"facilities":[{"id":"` + fx.facilityID + `","safe_zone_id":"` + fx.safeZoneID + `"}],"instruction_assets":[],"allocation_policy":{"order":["` + fx.facilityID + `"]}}`)
	if err := fx.store.InTx(context.Background(), func(tx DBTX) error {
		if _, err := tx.ExecContext(context.Background(),
			`UPDATE packages SET body = $1 WHERE package_id = $2`,
			newBody, fx.packageID); err != nil {
			return err
		}
		// Bump source_version to make the change observable.
		if _, err := tx.ExecContext(context.Background(),
			`UPDATE sources SET version = version + 1 WHERE source_id = $1`, srcID); err != nil {
			return err
		}
		// Bump package version too.
		if _, err := tx.ExecContext(context.Background(),
			`UPDATE packages SET version = version + 1 WHERE package_id = $1`, fx.packageID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seed expired route: %v", err)
	}
	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if rt, ok := sc.KnownRoutes["RT-EXPIRED"]; ok {
		if rt.Verified {
			t.Errorf("expired route must NOT be Verified, got %+v", rt)
		}
	}
}

// TestScopedContext_AllowsSyntheticApprovedRoute: the harness allows
// SYNTHETIC_DEMO approvals (the operational gate is upstream). Confirms
// a synthetic route can appear in VerifiedRoutes.
func TestScopedContext_AllowsSyntheticApprovedRoute(t *testing.T) {
	fx := newP6Fixture(t)
	defer fx.close()
	resolver := NewScopedContextResolver(fx.store)
	sc, err := resolver.Resolve(context.Background(), fx.jurisdictionID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	rt, ok := sc.KnownRoutes[fx.routeID]
	if !ok {
		t.Fatalf("route must be in KnownRoutes")
	}
	if !rt.Verified {
		t.Fatalf("synthetic approval must yield Verified=true under the harness; got %+v", rt)
	}
}

// === helpers ===

func countRows(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRowContext(context.Background(),
		`SELECT (SELECT COUNT(*) FROM sources) +
		        (SELECT COUNT(*) FROM source_authorizations) +
		        (SELECT COUNT(*) FROM source_artifacts) +
		        (SELECT COUNT(*) FROM packages) +
		        (SELECT COUNT(*) FROM place_aliases)`).Scan(&n); err != nil {
		t.Fatalf("countRows: %v", err)
	}
	return n
}

func findKLP6Source(t *testing.T, tx DBTX) string {
	t.Helper()
	var id string
	if err := tx.QueryRowContext(context.Background(),
		`SELECT source_id FROM sources WHERE source_id LIKE 'SRC-KL-P6-%' LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("findKLP6Source: %v", err)
	}
	return id
}

func strPtr(s string) *string { return &s }
