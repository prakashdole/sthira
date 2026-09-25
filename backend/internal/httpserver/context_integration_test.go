package httpserver

// P4 persisted context resolver verification: the voice-commands boundary
// validates against the current authorized OPERATIONAL package snapshot, failing
// closed on absent/expired/unauthorized/quarantined evidence. Real DB; gated on
// STHIRA_TEST_DSN (skipped, not passed, when unset).

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sthira/backend/internal/store"
)

// uniqueJTEST returns a per-test-run unique jurisdiction identifier so the
// persisted context resolver cannot collide with packages left in the
// shared DB by earlier runs. The persisted resolver is jurisdiction-scoped
// (D36); giving each test its own jurisdiction is the smallest isolation
// seam that works against the shared-DB fixture state.
func uniqueJTEST() string {
	return "JTEST-" + fmt.Sprintf("%d", time.Now().UnixNano())
}

// seedOperationalPackage seeds a package whose source is OPERATIONAL with a
// valid authorization, so the persisted resolver can resolve it. The body
// carries known IDs and languages. Returns (packageID, sourceID).
func seedOperationalPackage(t *testing.T, st *store.Store, jurisdiction string) (string, string) {
	t.Helper()
	return seedOperationalPackageBody(t, st, jurisdiction, `{
		"red_zones":[{"id":"RZ-1"}],
		"safe_zones":[{"id":"SZ-1"}],
		"approved_routes":[{"id":"RT-1"}],
		"facilities":[{"id":"FAC-1"}],
		"instruction_assets":[{"id":"IA-1","language":"en-IN"},{"id":"IA-2","language":"hi-IN"}]
	}`)
}

// seedOperationalPackageBody is seedOperationalPackage with a caller-supplied
// package body, so tests can give each jurisdiction distinct known IDs.
func seedOperationalPackageBody(t *testing.T, st *store.Store, jurisdiction, body string) (string, string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	srcID, artID, pkgID := "CSRC-"+suffix, "CART-"+suffix, "CPKG-"+suffix
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sources := store.NewSourceStore(store.ChainAuditor{})
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			VALUES ($1,'gov','gov.example','OPERATIONAL',1,$2,$2)`, srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,1,$3,$4,'SYNTHETIC','mem://t')`, artID, srcID, hash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			VALUES ($1,'ALT',$2,$3,1,$4,'SYNTHETIC',$5,$6,$7,$8)`,
			pkgID, srcID, artID, jurisdiction, now.Add(-time.Hour), now.Add(24*time.Hour), hash, body); err != nil {
			return err
		}
		return sources.RecordAuthorization(t.Context(), tx, store.Authorization{
			AuthorizationID: "AUTH-" + srcID,
			SourceID:        srcID,
			GrantedBy:       "gov",
			EvidenceRef:     "doc-1",
			Jurisdiction:    jurisdiction,
			GrantedAt:       now,
		})
	})
	if err != nil {
		t.Fatalf("seed operational package: %v", err)
	}
	return pkgID, srcID
}

// voiceProposal builds a minimal valid ModelOutput proposal referencing a known
// ID via a FOCUS_FEATURE action, in the enabled language en-IN, scoped to a
// jurisdiction.
func voiceProposal(requestID, dataVersion, jurisdiction, knownID string) string {
	return fmt.Sprintf(`{
		"request_id":%q,"data_version":%q,"jurisdiction":%q,
		"proposal":{
			"schema_version":"3.0","request_id":%q,"data_version":%q,
			"status":"OK","intent":"PREVIEW_DESTINATION","language":"en-IN",
			"actions":[{"type":"FOCUS_FEATURE","target_id":%q}],
			"speech_key":null,"clarification_ids":[],"evidence_ids":[]
		}
	}`, requestID, dataVersion, jurisdiction, requestID, dataVersion, knownID)
}

// resolveDataVersion reads the snapshot the persisted resolver resolves for a
// jurisdiction, so a test validates against the actual current package rather
// than assuming its own seed wins in the shared DB.
func resolveDataVersion(t *testing.T, st *store.Store, jurisdiction string) store.ContextSnapshot {
	t.Helper()
	snap, err := store.ResolveContext(t.Context(), st.DB(), jurisdiction, time.Now().UTC())
	if err != nil {
		t.Fatalf("resolve context: %v", err)
	}
	return snap
}

// TestPersistedContextResolves: a valid persisted snapshot resolves and a valid
// proposal validates against it. The test uses a per-run unique jurisdiction
// so it does not depend on the (shared) DB's state for the canonical JTEST
// jurisdiction; the persisted resolver is jurisdiction-scoped.
func TestPersistedContextResolves(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	jur := uniqueJTEST()
	seedOperationalPackage(t, st, jur)
	snap := resolveDataVersion(t, st, jur)
	// Use a known ID from the resolved snapshot.
	var knownID string
	for id := range snap.KnownIDs {
		knownID = id
		break
	}
	if knownID == "" {
		t.Fatal("resolved snapshot has no known IDs")
	}

	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-1", snap.DataVersion, jur, knownID))
	if rec.code != http.StatusOK {
		t.Fatalf("expected 200 valid proposal, got %d body=%s", rec.code, rec.body)
	}
}

// TestPersistedContextFailsClosedNoPackage: a jurisdiction with no operational
// package resolves nothing (fail closed at the store seam).
func TestPersistedContextFailsClosedNoPackage(t *testing.T) {
	st := httpTestDB(t)
	_, err := store.ResolveContext(t.Context(), st.DB(), "JURISDICTION-NONE-"+fmt.Sprintf("%d", time.Now().UnixNano()), time.Now().UTC())
	if !errors.Is(err, store.ErrNoOperationalContext) {
		t.Fatalf("expected ErrNoOperationalContext, got %v", err)
	}
}

// TestPersistedContextStaleClientVersion: a client-echoed data_version that does
// not match the resolved snapshot is rejected as stale (409).
func TestPersistedContextStaleClientVersion(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	jur := uniqueJTEST()
	seedOperationalPackage(t, st, jur)
	snap := resolveDataVersion(t, st, jur)
	var knownID string
	for id := range snap.KnownIDs {
		knownID = id
		break
	}
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-1", "stale-version:99", jur, knownID))
	if rec.code != http.StatusConflict {
		t.Fatalf("expected 409 stale version, got %d body=%s", rec.code, rec.body)
	}
}

// TestPersistedContextQuarantineInvalidates: quarantining the source removes the
// resolved context for that jurisdiction (subsequent resolution fails closed).
func TestPersistedContextQuarantineInvalidates(t *testing.T) {
	st := httpTestDB(t)
	jur := "JQ-" + fmt.Sprintf("%d", time.Now().UnixNano())
	_, srcID := seedOperationalPackage(t, st, jur)

	// Resolves before quarantine (jurisdiction-scoped).
	if _, err := store.ResolveContext(t.Context(), st.DB(), jur, time.Now().UTC()); err != nil {
		t.Fatalf("pre-quarantine resolve: %v", err)
	}

	// Quarantine the source directly (store-level).
	now := time.Now().UTC()
	sources := store.NewSourceStore(store.ChainAuditor{})
	err := st.InTx(t.Context(), func(tx store.DBTX) error {
		src, err := sources.GetSource(t.Context(), tx, srcID)
		if err != nil {
			return err
		}
		return sources.Quarantine(t.Context(), tx, srcID, src.Version, "test-op", "suspect", "EV-Q-"+srcID, now)
	})
	if err != nil {
		t.Fatalf("quarantine: %v", err)
	}

	// Resolution for that jurisdiction now fails closed.
	if _, err := store.ResolveContext(t.Context(), st.DB(), jur, time.Now().UTC()); !errors.Is(err, store.ErrNoOperationalContext) {
		t.Fatalf("post-quarantine: expected ErrNoOperationalContext, got %v", err)
	}
}

// TestPersistedContextUnknownIDRejected: a proposal referencing an ID not in the
// resolved snapshot is rejected (422).
func TestPersistedContextUnknownIDRejected(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	jur := uniqueJTEST()
	seedOperationalPackage(t, st, jur)
	snap := resolveDataVersion(t, st, jur)
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-1", snap.DataVersion, jur, "FAC-DOES-NOT-EXIST"))
	if rec.code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 unknown ID, got %d body=%s", rec.code, rec.body)
	}
}

// --- Item D: voice context scoped to the requested jurisdiction ---

// TestVoiceContextScopedPerJurisdiction: two jurisdictions with different
// package versions each resolve their OWN snapshot; neither is resolved from
// the other's package.
func TestVoiceContextScopedPerJurisdiction(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	jurA := "JA-" + fmt.Sprintf("%d", time.Now().UnixNano())
	jurB := "JB-" + fmt.Sprintf("%d", time.Now().UnixNano())
	seedOperationalPackageBody(t, st, jurA, `{"facilities":[{"id":"FAC-A"}],"instruction_assets":[{"id":"IA","language":"en-IN"}]}`)
	seedOperationalPackageBody(t, st, jurB, `{"facilities":[{"id":"FAC-B"}],"instruction_assets":[{"id":"IA","language":"en-IN"}]}`)

	snapA := resolveDataVersion(t, st, jurA)
	snapB := resolveDataVersion(t, st, jurB)
	if snapA.DataVersion == snapB.DataVersion {
		t.Fatalf("jurisdictions resolved the same snapshot %q", snapA.DataVersion)
	}
	if !snapA.KnownIDs["FAC-A"] || snapA.KnownIDs["FAC-B"] {
		t.Fatalf("jurA snapshot has wrong IDs: %+v", snapA.KnownIDs)
	}
	if !snapB.KnownIDs["FAC-B"] || snapB.KnownIDs["FAC-A"] {
		t.Fatalf("jurB snapshot has wrong IDs: %+v", snapB.KnownIDs)
	}

	// A proposal scoped to jurA validates against jurA's snapshot.
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-a", snapA.DataVersion, jurA, "FAC-A"))
	if rec.code != http.StatusOK {
		t.Fatalf("jurA proposal: code=%d body=%s", rec.code, rec.body)
	}
	// A proposal scoped to jurB validates against jurB's snapshot.
	rec = do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-b", snapB.DataVersion, jurB, "FAC-B"))
	if rec.code != http.StatusOK {
		t.Fatalf("jurB proposal: code=%d body=%s", rec.code, rec.body)
	}
}

// TestVoiceContextMissingJurisdictionRejected: an empty jurisdiction is
// rejected (400), never defaulted to another jurisdiction's package.
func TestVoiceContextMissingJurisdictionRejected(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	jur := "JM-" + fmt.Sprintf("%d", time.Now().UnixNano())
	seedOperationalPackage(t, st, jur)
	snap := resolveDataVersion(t, st, jur)

	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-m", snap.DataVersion, "", "FAC-1"))
	if rec.code != http.StatusBadRequest {
		t.Fatalf("missing jurisdiction: code=%d, want 400; body=%s", rec.code, rec.body)
	}
}

// TestVoiceContextUnknownJurisdictionFailsClosed: a jurisdiction with no
// operational package resolves nothing (503), never another jurisdiction's.
func TestVoiceContextUnknownJurisdictionFailsClosed(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	// A real jurisdiction with a package exists, but we query a different one.
	seedOperationalPackage(t, st, "JREAL-"+fmt.Sprintf("%d", time.Now().UnixNano()))

	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-u", "any:1", "JURISDICTION-NONE-"+fmt.Sprintf("%d", time.Now().UnixNano()), "FAC-1"))
	if rec.code != http.StatusServiceUnavailable {
		t.Fatalf("unknown jurisdiction: code=%d, want 503; body=%s", rec.code, rec.body)
	}
}

// TestVoiceContextCrossJurisdictionIDRejected: an ID known in jurisdiction A is
// rejected when the request is scoped to jurisdiction B (wrong-jurisdiction IDs
// fail; no cross-jurisdiction leakage).
func TestVoiceContextCrossJurisdictionIDRejected(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	jurA := "JCA-" + fmt.Sprintf("%d", time.Now().UnixNano())
	jurB := "JCB-" + fmt.Sprintf("%d", time.Now().UnixNano())
	seedOperationalPackageBody(t, st, jurA, `{"facilities":[{"id":"FAC-A"}],"instruction_assets":[{"id":"IA","language":"en-IN"}]}`)
	seedOperationalPackageBody(t, st, jurB, `{"facilities":[{"id":"FAC-B"}],"instruction_assets":[{"id":"IA","language":"en-IN"}]}`)

	snapB := resolveDataVersion(t, st, jurB)
	// FAC-A is known in jurA but the request is scoped to jurB: must be 422.
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-x", snapB.DataVersion, jurB, "FAC-A"))
	if rec.code != http.StatusUnprocessableEntity {
		t.Fatalf("cross-jurisdiction ID: code=%d, want 422; body=%s", rec.code, rec.body)
	}
}

// TestVoiceContextRevocationInvalidatesSubsequent: quarantining a
// jurisdiction's source invalidates subsequent requests scoped to it.
func TestVoiceContextRevocationInvalidatesSubsequent(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	jur := "JR-" + fmt.Sprintf("%d", time.Now().UnixNano())
	_, srcID := seedOperationalPackage(t, st, jur)
	snap := resolveDataVersion(t, st, jur)

	// Resolves and validates before quarantine.
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-pre", snap.DataVersion, jur, "FAC-1"))
	if rec.code != http.StatusOK {
		t.Fatalf("pre-quarantine: code=%d body=%s", rec.code, rec.body)
	}

	quarantineSource(t, st, srcID)

	// Same request now fails closed (503): the jurisdiction has no operational
	// context, and no other jurisdiction is substituted.
	rec = do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-post", snap.DataVersion, jur, "FAC-1"))
	if rec.code != http.StatusServiceUnavailable {
		t.Fatalf("post-quarantine: code=%d, want 503; body=%s", rec.code, rec.body)
	}
}
