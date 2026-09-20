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

// seedOperationalPackage seeds a package whose source is OPERATIONAL with a
// valid authorization, so the persisted resolver can resolve it. The body
// carries known IDs and languages. Returns (packageID, sourceID).
func seedOperationalPackage(t *testing.T, st *store.Store, jurisdiction string) (string, string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	srcID, artID, pkgID := "CSRC-"+suffix, "CART-"+suffix, "CPKG-"+suffix
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	body := `{
		"red_zones":[{"id":"RZ-1"}],
		"safe_zones":[{"id":"SZ-1"}],
		"approved_routes":[{"id":"RT-1"}],
		"facilities":[{"id":"FAC-1"}],
		"instruction_assets":[{"id":"IA-1","language":"en-IN"},{"id":"IA-2","language":"hi-IN"}]
	}`
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
// ID via a FOCUS_FEATURE action, in the enabled language en-IN.
func voiceProposal(requestID, dataVersion, knownID string) string {
	return fmt.Sprintf(`{
		"request_id":%q,"data_version":%q,
		"proposal":{
			"schema_version":"3.0","request_id":%q,"data_version":%q,
			"status":"OK","intent":"PREVIEW_DESTINATION","language":"en-IN",
			"actions":[{"type":"FOCUS_FEATURE","target_id":%q}],
			"speech_key":null,"clarification_ids":[],"evidence_ids":[]
		}
	}`, requestID, dataVersion, requestID, dataVersion, knownID)
}

// resolveDataVersion reads the snapshot the persisted resolver currently
// resolves, so a test validates against the actual current package rather than
// assuming its own seed wins in the shared DB.
func resolveDataVersion(t *testing.T, st *store.Store) store.ContextSnapshot {
	t.Helper()
	snap, err := store.ResolveAnyOperationalContext(t.Context(), st.DB(), time.Now().UTC())
	if err != nil {
		t.Fatalf("resolve context: %v", err)
	}
	return snap
}

// TestPersistedContextResolves: a valid persisted snapshot resolves and a valid
// proposal validates against it.
func TestPersistedContextResolves(t *testing.T) {
	st := httpTestDB(t)
	s := New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithPersistedContextResolver(st))
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	seedOperationalPackage(t, st, "JTEST")
	snap := resolveDataVersion(t, st)
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
		voiceProposal("req-1", snap.DataVersion, knownID))
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

	seedOperationalPackage(t, st, "JTEST")
	snap := resolveDataVersion(t, st)
	var knownID string
	for id := range snap.KnownIDs {
		knownID = id
		break
	}
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-1", "stale-version:99", knownID))
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

	seedOperationalPackage(t, st, "JTEST")
	snap := resolveDataVersion(t, st)
	rec := do(t, srv, http.MethodPost, "/api/v3/voice/commands", "application/json",
		voiceProposal("req-1", snap.DataVersion, "FAC-DOES-NOT-EXIST"))
	if rec.code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 unknown ID, got %d body=%s", rec.code, rec.body)
	}
}
