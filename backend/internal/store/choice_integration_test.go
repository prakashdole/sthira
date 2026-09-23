package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestChoiceQuerier_OrderingMatchesPolicyOrder: ChoiceQuerier.Eligible
// must return destinations in allocation_policy.order (safe-zone IDs),
// with all facilities in a zone sharing the same rank and within-zone
// facilities sorted by ID. It must NOT return them in alphabetical order
// or some other guessed ranking.
func TestChoiceQuerier_OrderingMatchesPolicyOrder(t *testing.T) {
	st, cleanup := disposableTestDB(t)
	defer cleanup()
	now := nowUTC()
	jr := "JUR-CHQ-ORDER-" + uid("X")
	srcID := "SRC-CHQ-ORDER-" + uid("X")
	pkgID := "PKG-CHQ-ORDER-" + uid("X")
	artID := "ART-CHQ-ORDER-" + uid("X")
	authID := "AUTH-CHQ-ORDER-" + uid("X")

	// Two zones, three facilities. Zone order is Z-second FIRST, Z-first
	// SECOND (i.e. policy deliberately puts the alphabetically-second zone
	// at rank 0). All zones are OPEN with capacity.
	szFirst := "SZ-FIRST-" + uid("X") // alphabetically earlier
	szSecond := "SZ-SECOND-" + uid("X")
	facA := "FAC-A-" + uid("X") // in szFirst
	facB := "FAC-B-" + uid("X") // in szSecond
	facC := "FAC-C-" + uid("X") // in szSecond (2 facilities per zone)

	body := []byte(`{
		"safe_zones":[
			{"id":"` + szFirst + `","status":"OPEN"},
			{"id":"` + szSecond + `","status":"OPEN"}
		],
		"facilities":[
			{"id":"` + facA + `","safe_zone_id":"` + szFirst + `"},
			{"id":"` + facB + `","safe_zone_id":"` + szSecond + `"},
			{"id":"` + facC + `","safe_zone_id":"` + szSecond + `"}
		],
		"instruction_assets":[{"id":"INS-1","language":"en-IN"}],
		"allocation_policy":{"order":["` + szSecond + `","` + szFirst + `"]}
	}`)

	if err := st.InTx(context.Background(), func(tx DBTX) error {
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at) VALUES ($1, 'gov-test', 'gov.example', 'OPERATIONAL', 1, $2)`,
			srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO source_authorizations (authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			authID, srcID, "authority-1", "doc-1", jr, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref) VALUES ($1,$2,1,$3,$4,'AUTHORIZED_OPERATIONAL',$5)`,
			artID, srcID, strings.Repeat("a", 64), now, "memory://test"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, checksum_sha256, body, effective_at, expires_at) VALUES ($1,$2,$3,$4,1,$5,'AUTHORIZED_OPERATIONAL',$6,$7,$8,$9)`,
			pkgID, "ALERT-"+uid("X"), srcID, artID, jr, strings.Repeat("a", 64), body, now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at) VALUES ($1, $2, 'SAFE', 'SAFE', 'OPEN', 100, 1, $4), ($3, $2, 'SAFE', 'SAFE', 'OPEN', 100, 1, $4)`,
			szFirst, pkgID, szSecond, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	// Browsing (no dates): informational list, capacity_known=false.
	q := ChoiceQuery{Jurisdiction: jr, PackageID: pkgID}
	dests, err := (ChoiceQuerier{}).Eligible(context.Background(), st.DB(), q, time.Now().UTC())
	if err != nil {
		t.Fatalf("Eligible: %v", err)
	}
	if len(dests) != 3 {
		t.Fatalf("expected 3 destinations, got %d", len(dests))
	}
	// szSecond rank 0: facB, facC (sorted by ID)
	// szFirst rank 1: facA
	wantOrder := []string{facB, facC, facA}
	for i, d := range dests {
		if d.FacilityID != wantOrder[i] {
			t.Errorf("dest %d FacilityID = %s, want %s (alphabetical within policy order)", i, d.FacilityID, wantOrder[i])
		}
	}
	if dests[0].SafeZoneID != szSecond || dests[1].SafeZoneID != szSecond || dests[2].SafeZoneID != szFirst {
		t.Errorf("safe zones not in policy order: %s, %s, %s (want %s,%s,%s)",
			dests[0].SafeZoneID, dests[1].SafeZoneID, dests[2].SafeZoneID,
			szSecond, szSecond, szFirst)
	}
	for _, d := range dests {
		if d.CapacityKnown {
			t.Errorf("browsing must report unknown capacity, got CapacityKnown=true for %s", d.FacilityID)
		}
	}
}

// TestChoiceQuerier_UnknownZoneDropped: an unknown safe-zone ID in
// allocation_policy.order contributes nothing. No destination is fabricated.
func TestChoiceQuerier_UnknownZoneDropped(t *testing.T) {
	st, cleanup := disposableTestDB(t)
	defer cleanup()
	now := nowUTC()
	jr := "JUR-CHQ-UNZ-" + uid("X")
	srcID := "SRC-CHQ-UNZ-" + uid("X")
	pkgID := "PKG-CHQ-UNZ-" + uid("X")
	artID := "ART-CHQ-UNZ-" + uid("X")
	authID := "AUTH-CHQ-UNZ-" + uid("X")
	szReal := "SZ-REAL-" + uid("X")
	szFake := "SZ-FAKE-" + uid("X")
	facID := "FAC-REAL-" + uid("X")

	body := []byte(`{
		"safe_zones":[{"id":"` + szReal + `","status":"OPEN"}],
		"facilities":[{"id":"` + facID + `","safe_zone_id":"` + szReal + `"}],
		"instruction_assets":[{"id":"INS-1","language":"en-IN"}],
		"allocation_policy":{"order":["` + szFake + `","` + szReal + `"]}
	}`)

	if err := st.InTx(context.Background(), func(tx DBTX) error {
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at) VALUES ($1, 'gov-test', 'gov.example', 'OPERATIONAL', 1, $2)`,
			srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO source_authorizations (authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			authID, srcID, "authority-1", "doc-1", jr, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref) VALUES ($1,$2,1,$3,$4,'AUTHORIZED_OPERATIONAL',$5)`,
			artID, srcID, strings.Repeat("a", 64), now, "memory://test"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, checksum_sha256, body, effective_at, expires_at) VALUES ($1,$2,$3,$4,1,$5,'AUTHORIZED_OPERATIONAL',$6,$7,$8,$9)`,
			pkgID, "ALERT-"+uid("X"), srcID, artID, jr, strings.Repeat("a", 64), body, now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at) VALUES ($1, $2, 'SAFE', 'SAFE', 'OPEN', 100, 1, $3)`,
			szReal, pkgID, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seed fixture: %v", err)
	}

	dests, err := (ChoiceQuerier{}).Eligible(context.Background(), st.DB(),
		ChoiceQuery{Jurisdiction: jr, PackageID: pkgID}, time.Now().UTC())
	if err != nil {
		t.Fatalf("Eligible: %v", err)
	}
	if len(dests) != 1 || dests[0].FacilityID != facID {
		var got []string
		for _, d := range dests {
			got = append(got, d.FacilityID)
		}
		t.Fatalf("expected only the real facility %s, got %v", facID, got)
	}
}
