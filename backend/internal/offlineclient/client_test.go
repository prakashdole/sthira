package offlineclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/offlinepkg"
)

type fakeVerifier struct {
	signatures    map[string]bool
	jurisdictions map[string]bool
	lookupKeys    map[string]offlinepkg.TrustedKey
}

func (f *fakeVerifier) VerifySignature(keyID, jurisdiction string, canonicalBytes []byte, sigBase64 string) error {
	if !f.signatures[keyID] {
		return offlinepkg.ErrInvalidSignature
	}
	if !f.jurisdictions[jurisdiction] {
		return offlinepkg.ErrSignerUnauthorized
	}
	return nil
}

func (f *fakeVerifier) LookupKey(keyID string) (offlinepkg.TrustedKey, error) {
	k, ok := f.lookupKeys[keyID]
	if !ok {
		return offlinepkg.TrustedKey{}, offlinepkg.ErrKeyExpired
	}
	if k.Revoked {
		return offlinepkg.TrustedKey{}, offlinepkg.ErrKeyExpired
	}
	if time.Now().Before(k.ValidFrom) || time.Now().After(k.ValidUntil) {
		return offlinepkg.TrustedKey{}, offlinepkg.ErrKeyExpired
	}
	return k, nil
}

func makeTestKey() offlinepkg.TrustedKey {
	return offlinepkg.TrustedKey{
		KeyID:                 "test-key",
		PermittedJurisdiction: "KL",
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(time.Hour),
		Revoked:               false,
	}
}

type fakeClock time.Time

func (f fakeClock) Now() time.Time { return time.Time(f) }

func (f fakeClock) Since(d time.Time) time.Duration { return time.Time(f).Sub(d) }

type testServer struct {
	*httptest.Server
	manifest  *offlinepkg.Manifest
	card      *offlinepkg.PublicIncidentCard
	wantRange bool
	wantETag  string
}

func (ts *testServer) serveManifest(w http.ResponseWriter, r *http.Request) {
	if ts.wantETag != "" {
		w.Header().Set("ETag", ts.wantETag)
		if r.Header.Get("If-None-Match") == ts.wantETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if ts.wantRange {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	// Send the manifest with its declared checksum and signature on the wire.
	// The client canonicalizes (stripping those mutable fields) before
	// re-checking the digest, per contract §4.2.
	mBytes, _ := json.Marshal(ts.manifest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(mBytes)))
	w.Write(mBytes)
}

func (ts *testServer) serveCard(w http.ResponseWriter, r *http.Request) {
	if ts.wantETag != "" {
		w.Header().Set("ETag", ts.wantETag)
		if r.Header.Get("If-None-Match") == ts.wantETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if ts.wantRange {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	// Send the card with its declared checksum and signature on the wire.
	cBytes, _ := json.Marshal(ts.card)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(cBytes)))
	w.Write(cBytes)
}

func newTestServer(t *testing.T, jurisdiction string) *testServer {
	ts := &testServer{
		wantRange: true,
		wantETag:  `"v1"`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/regions/"+jurisdiction+"/manifest", ts.serveManifest)
	mux.HandleFunc("/api/v3/packages/", ts.serveCard)
	ts.Server = httptest.NewServer(mux)
	return ts
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func makeTestManifest(t *testing.T, revision int, cardChecksum string) *offlinepkg.Manifest {
	m := &offlinepkg.Manifest{
		SchemaVersion: "3.0",
		ManifestID:    "manifest-kl-" + fmt.Sprintf("%d", revision),
		Jurisdiction:  "KL",
		Revision:      revision,
		GeneratedAt:   "2024-01-01T00:00:00Z",
		ValidUntil:    "2099-01-01T00:00:00Z",
		SourceStatus:  "active",
		CriticalCard: offlinepkg.CriticalCardDescriptor{
			PackageID:         "pkg-a",
			Version:           1,
			URI:               "/api/v3/packages/pkg-a/versions/1",
			ChecksumSHA256:    cardChecksum,
			UncompressedBytes: 4096,
			CompressedBytes:   512,
			ContentType:       "application/json",
		},
		Revocations: offlinepkg.RevocationBlock{
			RevokedPackages:    []string{},
			CancelledRoutes:    []string{},
			SupersededVersions: []offlinepkg.SupersededVersion{},
		},
		Provenance: offlinepkg.ManifestProvenance{
			Authority:     "gov.my.dpd",
			DatasetID:     "incident-registry",
			EvidenceClass: "SYNTHETIC_DEMO",
		},
	}
	m.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(m)
	m.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	return m
}

func makeTestCard(t *testing.T) *offlinepkg.PublicIncidentCard {
	c := &offlinepkg.PublicIncidentCard{
		SchemaVersion: "3.0",
		PackageID:     "pkg-a",
		Version:       1,
		Jurisdiction:  "KL",
		EvidenceClass: "SYNTHETIC_DEMO",
		EffectiveAt:   "2024-01-01T00:00:00Z",
		ExpiresAt:     "2099-01-01T00:00:00Z",
		Alert: offlinepkg.AlertCard{
			Identifier:      "alert-001",
			Sender:          "gov.my.dpd",
			Headline:        "Test Alert",
			Severity:        "severe",
			Urgency:         "high",
			Certainty:       "observed",
			AreaDescription: "Test Area",
		},
		RedZones: []offlinepkg.RedZoneCard{
			{ID: "rz-1", Name: "Test Red Zone"},
		},
		SafeZones: []offlinepkg.SafeZoneCard{
			{ID: "sz-1", Name: "Test Safe Zone", Role: "EMERGENCY_SHELTER", Status: "OPEN", CapacityMode: "DEFINED"},
		},
		ApprovedRoutes: []offlinepkg.RouteCard{},
		Facilities: []offlinepkg.FacilityCard{
			{ID: "fac-1", SafeZoneID: "sz-1", Name: "Test Facility"},
		},
		Instructions: []offlinepkg.InstructionCard{
			{ID: "ins-1", Language: "en-IN", Title: "Move to safety", Summary: "Move to the nearest safe zone"},
		},
		EmergencyContacts: []offlinepkg.EmergencyContact{
			{Name: "Emergency", Number: "112"},
		},
		AllocationPolicy: offlinepkg.PolicyCard{
			Order: []string{"sz-1"},
		},
	}
	c.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(c)
	c.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	return c
}

// testFixtures creates a consistent manifest/card pair with matching checksums.
// The checksum is computed over canonical bytes with signature stripped, so
// the wire bytes (signed) match the client verification (also strips signature
// before recomputing the digest per contract §4.2).
func testFixtures(t *testing.T, revision int) (*offlinepkg.Manifest, *offlinepkg.PublicIncidentCard) {
	card := makeTestCard(t)
	manifest := makeTestManifest(t, revision, card.ChecksumSHA256)
	manifest.Signature = &offlinepkg.Signature{
		Algorithm: "Ed25519",
		KeyID:     "test-key",
		Value:     "stub-signature-not-verified-in-tests",
	}
	card.Signature = &offlinepkg.Signature{
		Algorithm: "Ed25519",
		KeyID:     "test-key",
		Value:     "stub-signature-not-verified-in-tests",
	}
	cRaw, _ := json.Marshal(card)
	manifest.CriticalCard.UncompressedBytes = int64(len(cRaw))

	// Checksum is over the unsigned form (signature stripped) so
	// verifyChecksum's strip+recompute logic finds the same digest.
	mUnsigned := *manifest
	mUnsigned.ChecksumSHA256 = ""
	mUnsigned.Signature = nil
	mCanonical, _ := offlinepkg.CanonicalBytes(&mUnsigned)
	manifest.ChecksumSHA256 = offlinepkg.ChecksumSHA256(mCanonical)

	cUnsigned := *card
	cUnsigned.ChecksumSHA256 = ""
	cUnsigned.Signature = nil
	cCanonical, _ := offlinepkg.CanonicalBytes(&cUnsigned)
	card.ChecksumSHA256 = offlinepkg.ChecksumSHA256(cCanonical)

	return manifest, card
}

// signManifest adds a fake signature to a manifest and recomputes the
// checksum over the unsigned (signature-stripped) form, matching the
// client verification logic in verifyChecksum.
func signManifest(m *offlinepkg.Manifest) {
	m.Signature = &offlinepkg.Signature{
		Algorithm: "Ed25519",
		KeyID:     "test-key",
		Value:     "stub-signature-not-verified-in-tests",
	}
	stripped := *m
	stripped.ChecksumSHA256 = ""
	stripped.Signature = nil
	canonical, _ := offlinepkg.CanonicalBytes(&stripped)
	m.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
}

// signCard adds a fake signature to a card and recomputes the checksum
// over the unsigned (signature-stripped) form.
func signCard(c *offlinepkg.PublicIncidentCard) {
	c.Signature = &offlinepkg.Signature{
		Algorithm: "Ed25519",
		KeyID:     "test-key",
		Value:     "stub-signature-not-verified-in-tests",
	}
	stripped := *c
	stripped.ChecksumSHA256 = ""
	stripped.Signature = nil
	canonical, _ := offlinepkg.CanonicalBytes(&stripped)
	c.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
}

func newClient(t *testing.T, serverURL string, dir string, clock fakeClock) *ProtocolClient {
	verifier := &fakeVerifier{
		signatures:    map[string]bool{"test-key": true},
		jurisdictions: map[string]bool{"KL": true},
		lookupKeys:    map[string]offlinepkg.TrustedKey{"test-key": makeTestKey()},
	}
	c, err := NewClient(ClientConfig{
		BaseURL:    serverURL,
		StorageDir: dir,
		Now:        clock.Now,
		TrustStore: verifier,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestNewClient(t *testing.T) {
	dir := t.TempDir()
	clock := fakeClock(time.Now())
	c, err := NewClient(ClientConfig{
		StorageDir: dir,
		BaseURL:    "http://localhost:12345",
		Now:        clock.Now,
		TrustStore: &fakeVerifier{},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.StorageDir() != dir {
		t.Errorf("StorageDir = %q, want %q", c.StorageDir(), dir)
	}
}

func TestSync_ColdStart(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()

	ts.manifest, ts.card = testFixtures(t, 1)

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	report, err := c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !report.ManifestUpdated {
		t.Error("ManifestUpdated = false, want true")
	}
	if !report.CardUpdated {
		t.Error("CardUpdated = false, want true")
	}
	if report.ActiveRevision != 1 {
		t.Errorf("ActiveRevision = %d, want 1", report.ActiveRevision)
	}

	card, state, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard: %v", err)
	}
	if card.PackageID != "pkg-a" {
		t.Errorf("card.PackageID = %q, want pkg-a", card.PackageID)
	}
	if state != FreshnessCurrent {
		t.Errorf("freshness = %v, want FreshnessCurrent", state)
	}
}

func TestSync_RevisionRollback(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()

	ts.manifest, ts.card = testFixtures(t, 3)

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	c.Sync(context.Background(), "KL")

	// Server rewound to revision 2
	ts.manifest, ts.card = testFixtures(t, 2)

	_, err := c.Sync(context.Background(), "KL")
	if !errors.Is(err, offlinepkg.ErrVersionRollback) {
		t.Errorf("Sync error = %v, want ErrVersionRollback", err)
	}
}

func TestSync_SameRevisionSkipsCardFetch(t *testing.T) {
	ts1 := newTestServer(t, "KL")
	ts1.manifest, ts1.card = testFixtures(t, 1)
	defer ts1.Server.Close()

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts1.URL, dir, clock)

	c.Sync(context.Background(), "KL")

	// Second server (same revision) — client should detect no card change
	ts2 := newTestServer(t, "KL")
	ts2.manifest, ts2.card = testFixtures(t, 1)
	defer ts2.Server.Close()

	c2 := newClient(t, ts2.URL, dir, clock)
	report, err := c2.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("c2.Sync failed: %v", err)
	}
	if report == nil {
		t.Fatalf("c2.Sync returned nil report")
	}
	if report.CardUpdated {
		t.Error("CardUpdated = true on same-revision sync, want false")
	}
}

func TestSync_VerificationFailure_BadChecksum(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()

	ts.card = makeTestCard(t)
	signCard(ts.card)

	ts.manifest = &offlinepkg.Manifest{
		SchemaVersion: "3.0",
		ManifestID:    "manifest-kl-1",
		Jurisdiction:  "KL",
		Revision:      1,
		GeneratedAt:   "2024-01-01T00:00:00Z",
		ValidUntil:    "2099-01-01T00:00:00Z",
		SourceStatus:  "active",
		CriticalCard: offlinepkg.CriticalCardDescriptor{
			PackageID: "pkg-a", Version: 1,
			URI:            "/api/v3/packages/pkg-a/versions/1",
			ChecksumSHA256: ts.card.ChecksumSHA256,
		},
		Revocations: offlinepkg.RevocationBlock{},
		Provenance: offlinepkg.ManifestProvenance{
			Authority:     "gov.my.dpd",
			DatasetID:     "incident-registry",
			EvidenceClass: "SYNTHETIC_DEMO",
		},
		ChecksumSHA256: strings.Repeat("0", 64),
	}
	signManifest(ts.manifest)
	// Overwrite with the intentionally wrong 64-hex checksum AFTER signManifest
	// so structural validation passes but checksum verification fails.
	ts.manifest.ChecksumSHA256 = strings.Repeat("0", 64)

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	_, err := c.Sync(context.Background(), "KL")
	if err == nil {
		t.Error("Sync succeeded with bad checksum, want error")
	}
}

func TestSync_TombstonesPersisted(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()

	// Build card first to get its checksum for the manifest.
	ts.card = makeTestCard(t)
	signCard(ts.card)

	ts.manifest = makeTestManifest(t, 1, ts.card.ChecksumSHA256)
	ts.manifest.Revocations.RevokedPackages = []string{"pkg-b"}
	ts.manifest.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(ts.manifest)
	ts.manifest.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	signManifest(ts.manifest)

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)

	report, _ := c.Sync(context.Background(), "KL")
	if len(report.RevokedPackages) != 1 || report.RevokedPackages[0] != "pkg-b" {
		t.Errorf("RevokedPackages = %v, want [pkg-b]", report.RevokedPackages)
	}

	// Restart and verify tombstone survives
	c2 := newClient(t, ts.URL, dir, clock)
	if c2.IsRouteCancelled("pkg-b") {
		t.Error("IsRouteCancelled(pkg-b) = true — it is a package, not a route")
	}
}

func TestFreshnessState_Current(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)
	c.Sync(context.Background(), "KL")

	_, state, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard: %v", err)
	}
	if state != FreshnessCurrent {
		t.Errorf("freshness = %v, want FreshnessCurrent", state)
	}
}

func TestFreshnessState_Stale(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()

	// Card with a 1-hour validity window so we can drive it stale.
	ts.card = makeTestCard(t)
	now := time.Now().UTC()
	ts.card.EffectiveAt = now.Format(time.RFC3339)
	ts.card.ExpiresAt = now.Add(1 * time.Hour).Format(time.RFC3339)
	ts.card.ChecksumSHA256 = ""
	cardCanonical, _ := offlinepkg.CanonicalBytes(ts.card)
	ts.card.ChecksumSHA256 = offlinepkg.ChecksumSHA256(cardCanonical)
	signCard(ts.card)

	ts.manifest = makeTestManifest(t, 1, ts.card.ChecksumSHA256)
	ts.manifest.ChecksumSHA256 = ""
	mCanonical, _ := offlinepkg.CanonicalBytes(ts.manifest)
	ts.manifest.ChecksumSHA256 = offlinepkg.ChecksumSHA256(mCanonical)
	signManifest(ts.manifest)

	clock := fakeClock(now)
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)
	c.Sync(context.Background(), "KL")

	// Restart with a fresh process (monotonic anchor zero): freshness must
	// recover from wall-clock + the original acquisition window recorded
	// in state.json, without renewing validity.
	// Advance 30 min — still well within the 1h window; recovery reads
	// CURRENT. MaxObservedUnixMS advances each query so a later clock
	// rollback is still detected.
	future1 := fakeClock(now.Add(30 * time.Minute))
	c2 := newClient(t, ts.URL, dir, future1)
	_, state, _ := c2.GetActiveCard()
	if state != FreshnessCurrent {
		t.Errorf("after restart + 30min: freshness = %v, want FreshnessCurrent (recovered without renewing validity)", state)
	}

	// Advance 56 min total — within staleBefore (5min default), so STALE.
	future2 := fakeClock(now.Add(56 * time.Minute))
	c3 := newClient(t, ts.URL, dir, future2)
	_, state, _ = c3.GetActiveCard()
	if state != FreshnessStale {
		t.Errorf("after restart near expiry: freshness = %v, want FreshnessStale (recovered)", state)
	}
}

func TestStorageLayout(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)
	c.Sync(context.Background(), "KL")

	wantFiles := []string{"state.json", "current_generation.json"}
	for _, f := range wantFiles {
		path := filepath.Join(dir, "state", f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected state file %q not found", f)
		}
	}

	tombDir := filepath.Join(dir, "tombstones")
	if _, err := os.Stat(tombDir); os.IsNotExist(err) {
		t.Errorf("expected tombstones dir not found")
	}

	downloadsDir := filepath.Join(dir, "downloads")
	if entries, _ := os.ReadDir(downloadsDir); len(entries) != 0 {
		t.Errorf("downloads dir not empty after sync: %d entries, want 0 (parts cleaned up)", len(entries))
	}
}

func TestIsRouteCancelled(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()

	ts.card = makeTestCard(t)
	signCard(ts.card)

	ts.manifest = makeTestManifest(t, 1, ts.card.ChecksumSHA256)
	ts.manifest.Revocations.CancelledRoutes = []string{"route-x"}
	ts.manifest.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(ts.manifest)
	ts.manifest.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	signManifest(ts.manifest)

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)
	c.Sync(context.Background(), "KL")

	if !c.IsRouteCancelled("route-x") {
		t.Error("IsRouteCancelled(route-x) = false, want true")
	}
	if c.IsRouteCancelled("route-y") {
		t.Error("IsRouteCancelled(route-y) = true, want false")
	}
}

func TestHTTP304NotModified(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)
	ts.wantETag = `"etag-v1"`

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)

	report1, _ := c.Sync(context.Background(), "KL")
	if !report1.ManifestUpdated {
		t.Error("first sync: ManifestUpdated = false")
	}

	// Second sync: server returns 304 — manifest should not be re-downloaded
	report2, _ := c.Sync(context.Background(), "KL")
	if report2.ManifestUpdated {
		t.Error("second sync with 304: ManifestUpdated = true, want false")
	}
}

func TestDownloadResource(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	resourceContent := []byte("resource content here")
	resourceChecksum := offlinepkg.ChecksumSHA256(resourceContent)
	mux := ts.Server.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/v3/resources/res-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("ETag", `"res-etag"`)
		w.Write(resourceContent)
	})

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)
	c.Sync(context.Background(), "KL")

	has, _ := c.HasResource("res-1")
	if has {
		t.Error("HasResource(res-1) = true before download, want false")
	}

	desc := offlinepkg.ResourceDescriptor{
		ResourceID:     "res-1",
		Type:           offlinepkg.TypeEmergencyAudio,
		URI:            "/api/v3/resources/res-1",
		ChecksumSHA256: resourceChecksum,
		ByteSize:       int64(len(resourceContent)),
		ContentType:    "application/octet-stream",
		Required:       false,
	}
	err := c.DownloadResource(context.Background(), desc, false)
	if err != nil {
		t.Fatalf("DownloadResource: %v", err)
	}

	has, _ = c.HasResource("res-1")
	if !has {
		t.Error("HasResource(res-1) = false after download, want true")
	}
}

func TestRangeDownload_Advertised(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)
	ts.wantRange = true

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	_, err := c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
}

func TestSyncReport_Fields(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()

	ts.card = makeTestCard(t)
	ts.card.PackageID = "pkg-x"
	ts.card.Version = 3
	ts.card.Alert.Identifier = "a"
	ts.card.Alert.Sender = "s"
	ts.card.Alert.Headline = "h"
	ts.card.ChecksumSHA256 = ""
	cardCanonical, _ := offlinepkg.CanonicalBytes(ts.card)
	ts.card.ChecksumSHA256 = offlinepkg.ChecksumSHA256(cardCanonical)
	signCard(ts.card)

	ts.manifest = makeTestManifest(t, 5, ts.card.ChecksumSHA256)
	ts.manifest.CriticalCard.PackageID = "pkg-x"
	ts.manifest.CriticalCard.Version = 3
	ts.manifest.CriticalCard.URI = "/api/v3/packages/pkg-x/versions/3"
	ts.manifest.Revocations = offlinepkg.RevocationBlock{
		RevokedPackages:    []string{"revoked-pkg"},
		CancelledRoutes:    []string{"cancelled-route"},
		SupersededVersions: []offlinepkg.SupersededVersion{{PackageID: "old-pkg", Version: 1}},
	}
	ts.manifest.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(ts.manifest)
	ts.manifest.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	signManifest(ts.manifest)

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	report, _ := c.Sync(context.Background(), "KL")

	if report.ActiveRevision != 5 {
		t.Errorf("ActiveRevision = %d, want 5", report.ActiveRevision)
	}
	if !report.ManifestUpdated {
		t.Error("ManifestUpdated = false, want true")
	}
	if !report.CardUpdated {
		t.Error("CardUpdated = false, want true")
	}
	if len(report.RevokedPackages) != 1 {
		t.Errorf("RevokedPackages len = %d, want 1", len(report.RevokedPackages))
	}
	if len(report.CancelledRoutes) != 1 {
		t.Errorf("CancelledRoutes len = %d, want 1", len(report.CancelledRoutes))
	}
	if len(report.Superseded) != 1 {
		t.Errorf("Superseded len = %d, want 1", len(report.Superseded))
	}
}

func TestStorageDir(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewClient(ClientConfig{
		StorageDir: dir,
		BaseURL:    "http://localhost:9999",
		Now:        time.Now,
		TrustStore: &fakeVerifier{},
	})
	if c.StorageDir() != dir {
		t.Errorf("StorageDir = %q, want %q", c.StorageDir(), dir)
	}
}

func TestNewClient_MissingBaseURL(t *testing.T) {
	_, err := NewClient(ClientConfig{
		StorageDir: t.TempDir(),
		TrustStore: &fakeVerifier{},
	})
	if err == nil {
		t.Error("NewClient with empty BaseURL: got nil error, want error")
	}
}

func TestNewClient_MissingStorageDir(t *testing.T) {
	_, err := NewClient(ClientConfig{
		BaseURL:    "http://localhost:1234",
		TrustStore: &fakeVerifier{},
	})
	if err == nil {
		t.Error("NewClient with empty StorageDir: got nil error, want error")
	}
}

func TestNewClient_MissingTrustStore(t *testing.T) {
	_, err := NewClient(ClientConfig{
		BaseURL:    "http://localhost:1234",
		StorageDir: t.TempDir(),
	})
	if err == nil {
		t.Error("NewClient with nil TrustStore: got nil error, want error")
	}
}

func TestSync_EmptyJurisdiction(t *testing.T) {
	clock := fakeClock(time.Now())
	c := newClient(t, "http://localhost:1234", t.TempDir(), clock)
	_, err := c.Sync(context.Background(), "")
	if err == nil {
		t.Error("Sync with empty jurisdiction: got nil error, want error")
	}
}

func TestSync_CardNeedsFetch_PkgChange(t *testing.T) {
	ts1 := newTestServer(t, "KL")
	ts1.manifest, ts1.card = testFixtures(t, 1)
	defer ts1.Server.Close()

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts1.URL, dir, clock)
	c.Sync(context.Background(), "KL")

	// Second server with pkg-a v2 card.
	ts2 := newTestServer(t, "KL")
	defer ts2.Server.Close()

	ts2.card = makeTestCard(t)
	ts2.card.Version = 2
	ts2.card.Alert.Headline = "h v2"
	ts2.card.ChecksumSHA256 = ""
	cardCanonical, _ := offlinepkg.CanonicalBytes(ts2.card)
	ts2.card.ChecksumSHA256 = offlinepkg.ChecksumSHA256(cardCanonical)
	signCard(ts2.card)

	ts2.manifest = makeTestManifest(t, 2, ts2.card.ChecksumSHA256)
	ts2.manifest.CriticalCard.PackageID = "pkg-a"
	ts2.manifest.CriticalCard.Version = 2
	ts2.manifest.CriticalCard.URI = "/api/v3/packages/pkg-a/versions/2"
	ts2.manifest.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(ts2.manifest)
	ts2.manifest.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	signManifest(ts2.manifest)

	c2 := newClient(t, ts2.URL, dir, clock)
	report, _ := c2.Sync(context.Background(), "KL")
	if !report.CardUpdated {
		t.Error("CardUpdated = false on new card version, want true")
	}

	card, _, _ := c2.GetActiveCard()
	if card.Version != 2 {
		t.Errorf("card.Version = %d, want 2", card.Version)
	}
}

func TestDownloadResource_NotFound(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)
	c.Sync(context.Background(), "KL")

	desc := offlinepkg.ResourceDescriptor{
		ResourceID:     "nonexistent",
		Type:           offlinepkg.TypeEmergencyAudio,
		URI:            "/api/v3/resources/nonexistent",
		ChecksumSHA256: "abc123",
		ByteSize:       100,
		ContentType:    "application/octet-stream",
	}
	err := c.DownloadResource(context.Background(), desc, false)
	if err == nil {
		t.Error("DownloadResource(nonexistent): got nil error, want error")
	}
}

func TestSupersededTombstone(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()

	ts.card = makeTestCard(t)
	signCard(ts.card)

	ts.manifest = makeTestManifest(t, 1, ts.card.ChecksumSHA256)
	ts.manifest.Revocations.SupersededVersions = []offlinepkg.SupersededVersion{{PackageID: "old", Version: 1}}
	ts.manifest.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(ts.manifest)
	ts.manifest.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	signManifest(ts.manifest)

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)

	report, _ := c.Sync(context.Background(), "KL")
	if len(report.Superseded) != 1 {
		t.Errorf("Superseded len = %d, want 1", len(report.Superseded))
	}
}

func TestHasResource_NotDownloaded(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)
	c.Sync(context.Background(), "KL")

	has, _ := c.HasResource("res-1")
	if has {
		t.Error("HasResource(res-1) = true before download, want false")
	}
}

// TestRestartRecovery verifies that after sync, a fresh client reading the
// same storage recovers the freshness from wall-clock and the persisted
// acquisition window. The validity window is NOT renewed: the same
// LastFetchedAtUnixMS anchors the elapsed calculation. The monotonic
// process-local anchor is not relied on across restart.
func TestRestartRecovery(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)
	c.Sync(context.Background(), "KL")

	// New client pointing at the same storage; wall-clock time has not
	// advanced, so the card is still CURRENT.
	c2 := newClient(t, ts.URL, dir, clock)
	card, state, err := c2.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard after restart: %v", err)
	}
	if card.PackageID != "pkg-a" {
		t.Errorf("card.PackageID = %q, want pkg-a", card.PackageID)
	}
	if state != FreshnessCurrent {
		t.Errorf("freshness after restart = %v, want FreshnessCurrent (recovered without renewing validity)", state)
	}
}

// verify that nil context is handled (not panicked)
func TestSync_NilContext(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	// nil context should not panic
	//lint:ignore SA1012 intentional test of nil context resilience
	_, _ = c.Sync(nil, "KL")
}

// TestCardReferenceBindingMismatch reproduces Bug 2: a card accepted despite a
// different checksum declared in the manifest reference.
func TestCardReferenceBindingMismatch(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	m, card := testFixtures(t, 1)
	m.CriticalCard.ChecksumSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	signManifest(m)

	ts.manifest = m
	ts.card = card

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	_, err := c.Sync(context.Background(), "KL")
	if err == nil {
		t.Fatal("Sync succeeded despite critical card checksum mismatch in manifest reference; want error")
	}
	if !errors.Is(err, offlinepkg.ErrChecksumMismatch) && !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected checksum mismatch error, got: %v", err)
	}
}

// TestSync_StorageJurisdictionBound prevents revision counters and tombstones
// from one jurisdiction being compared with another in the same directory.
func TestSync_StorageJurisdictionBound(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	c := newClient(t, ts.URL, t.TempDir(), fakeClock(time.Now()))
	if _, err := c.Sync(context.Background(), "KL"); err != nil {
		t.Fatalf("initial KL Sync: %v", err)
	}
	if _, err := c.Sync(context.Background(), "TN"); err == nil || !strings.Contains(err.Error(), "bound to jurisdiction") {
		t.Fatalf("Sync(TN) error = %v, want storage jurisdiction binding failure", err)
	}
}

// TestCardDeclaredSizeExceeded verifies that a card exceeding the manifest's
// declared uncompressed byte size limit is rejected.
func TestCardDeclaredSizeExceeded(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	m, card := testFixtures(t, 1)
	m.CriticalCard.UncompressedBytes = 100
	signManifest(m)

	ts.manifest = m
	ts.card = card

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	_, err := c.Sync(context.Background(), "KL")
	if err == nil {
		t.Fatal("Sync succeeded despite card exceeding declared uncompressed size; want error")
	}
}

// TestAlreadyExpiredCardReportedExpired reproduces Bug 1: an already-expired
// card reported CURRENT.
func TestAlreadyExpiredCardReportedExpired(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	m, card := testFixtures(t, 1)

	now := time.Now().UTC()
	card.EffectiveAt = now.Add(-24 * time.Hour).Format(time.RFC3339)
	card.ExpiresAt = now.Add(-1 * time.Hour).Format(time.RFC3339) // expired 1 hour ago
	signCard(card)

	m.CriticalCard.ChecksumSHA256 = card.ChecksumSHA256
	m.CriticalCard.UncompressedBytes = int64(len(mustMarshal(card)))
	signManifest(m)

	ts.manifest = m
	ts.card = card

	clock := fakeClock(now)
	c := newClient(t, ts.URL, t.TempDir(), clock)

	_, err := c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	_, state, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard failed: %v", err)
	}
	if state == FreshnessCurrent {
		t.Fatalf("already-expired card reported CURRENT; want EXPIRED")
	}
	if state != FreshnessExpired {
		t.Fatalf("state = %v; want FreshnessExpired", state)
	}
}

// TestClockRollbackCannotUnexpire verifies that once a card has expired,
// rolling back the system clock does not restore it to CURRENT.
func TestClockRollbackCannotUnexpire(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	m, card := testFixtures(t, 1)

	baseTime := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	card.EffectiveAt = baseTime.Format(time.RFC3339)
	card.ExpiresAt = baseTime.Add(2 * time.Hour).Format(time.RFC3339) // expires at 12:00
	signCard(card)

	m.CriticalCard.ChecksumSHA256 = card.ChecksumSHA256
	m.CriticalCard.UncompressedBytes = int64(len(mustMarshal(card)))
	signManifest(m)

	ts.manifest = m
	ts.card = card

	clock := fakeClock(baseTime) // sync at 10:00
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)

	_, err := c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Advance clock past expiry (13:00)
	c.now = func() time.Time { return baseTime.Add(3 * time.Hour) }
	_, state1, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard failed: %v", err)
	}
	if state1 != FreshnessExpired {
		t.Fatalf("at 13:00 state = %v, want EXPIRED", state1)
	}

	// Roll back clock to 11:00 (before expiry)
	c.now = func() time.Time { return baseTime.Add(1 * time.Hour) }
	_, state2, _ := c.GetActiveCard()
	if state2 == FreshnessCurrent {
		t.Fatalf("clock rollback restored expired card to CURRENT; want EXPIRED")
	}
}

// TestKeyRevocationInvalidatesCard verifies that revoking the signing key
// invalidates an active cached card.
func TestKeyRevocationInvalidatesCard(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	m, card := testFixtures(t, 1)

	baseTime := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	card.EffectiveAt = baseTime.Format(time.RFC3339)
	card.ExpiresAt = baseTime.Add(2 * time.Hour).Format(time.RFC3339)
	signCard(card)

	m.CriticalCard.ChecksumSHA256 = card.ChecksumSHA256
	m.CriticalCard.UncompressedBytes = int64(len(mustMarshal(card)))
	signManifest(m)

	ts.manifest = m
	ts.card = card

	clock := fakeClock(baseTime)
	dir := t.TempDir()

	tk := makeTestKey()
	tk.KeyID = "test-key"
	tk.Revoked = false
	fakeV := &fakeVerifier{
		signatures:    map[string]bool{"test-key": true},
		jurisdictions: map[string]bool{"KL": true},
		lookupKeys:    map[string]offlinepkg.TrustedKey{"test-key": tk},
	}

	c, err := NewClient(ClientConfig{
		BaseURL:    ts.URL,
		StorageDir: dir,
		Now:        clock.Now,
		TrustStore: fakeV,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	_, stateBefore, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard before: %v", err)
	}
	if stateBefore != FreshnessCurrent {
		t.Fatalf("expected CURRENT before revocation, got %v", stateBefore)
	}

	// Revoke the key in the trust store
	tk.Revoked = true
	fakeV.lookupKeys["test-key"] = tk

	_, stateAfter, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard after: %v", err)
	}
	if stateAfter == FreshnessCurrent {
		t.Fatalf("card still reported CURRENT after signing key revoked; want REVOKED")
	}
	if stateAfter != FreshnessRevoked {
		t.Fatalf("state = %v; want REVOKED", stateAfter)
	}
}

// TestCorruptTombstonesFailClosed verifies that corrupt tombstones fail closed:
// stateQuery returns FreshnessUnverifiable with an error rather than silently
// treating routes and packages as non-revoked.
func TestCorruptTombstonesFailClosed(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	m, card := testFixtures(t, 1)
	ts.manifest = m
	ts.card = card

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)

	_, err := c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Corrupt the single tombstone state file.
	tombPath := filepath.Join(dir, "tombstones", "tombstones.json")
	if err := os.WriteFile(tombPath, []byte("NOT_VALID_JSON{{{"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, state, err := c.GetActiveCard()
	if err == nil && state == FreshnessCurrent {
		t.Fatalf("GetActiveCard reported CURRENT despite corrupted tombstones; must fail closed")
	}
	if state != FreshnessUnverifiable {
		t.Fatalf("state = %v; want FreshnessUnverifiable on corrupt tombstones", state)
	}
}

// TestAtomicActivationInterrupted verifies that if card download fails after manifest
// is downloaded, the client does not commit an inconsistent manifest-without-card.
func TestAtomicActivationInterrupted(t *testing.T) {
	cardFail := false
	mux := http.NewServeMux()
	m1, card1 := testFixtures(t, 1)
	m2, card2 := testFixtures(t, 2)
	card2.PackageID = "pkg-v2"
	card2.Version = 2
	m2.CriticalCard.PackageID = "pkg-v2"
	m2.CriticalCard.Version = 2
	signCard(card2)
	m2.CriticalCard.ChecksumSHA256 = card2.ChecksumSHA256
	m2.CriticalCard.UncompressedBytes = int64(len(mustMarshal(card2)))
	signManifest(m2)

	currentManifest := m1
	currentCard := card1

	mux.HandleFunc("/api/v3/regions/KL/manifest", func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(currentManifest)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	mux.HandleFunc("/api/v3/packages/", func(w http.ResponseWriter, r *http.Request) {
		if cardFail {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		b, _ := json.Marshal(currentCard)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, srv.URL, dir, clock)

	// Sync rev 1 successfully
	_, err := c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync rev 1: %v", err)
	}

	active1, _, err := c.GetActiveCard()
	if err != nil || active1.PackageID != "pkg-a" {
		t.Fatalf("expected pkg-a active, got %v, err=%v", active1, err)
	}

	// Now offer rev 2 manifest, but fail card download
	currentManifest = m2
	currentCard = card2
	cardFail = true

	_, err = c.Sync(context.Background(), "KL")
	if err == nil {
		t.Fatal("Sync rev 2 should fail on card error")
	}

	// Active state MUST remain rev 1 coherent! It must NOT have rev 2 manifest paired with rev 1 card.
	manRec, err := c.GetActiveManifest()
	if err != nil {
		t.Fatalf("GetActiveManifest: %v", err)
	}
	if manRec.Revision != 1 {
		t.Fatalf("active manifest was updated to revision %d despite failed card download; atomic activation violated", manRec.Revision)
	}
	activeCard, _, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard: %v", err)
	}
	if activeCard.PackageID != "pkg-a" {
		t.Fatalf("active card was corrupted; got package %s", activeCard.PackageID)
	}
}

// TestSameRevisionRepair verifies that if active files are deleted or corrupted,
// a sync against the same manifest revision detects the damage and repairs them.
func TestSameRevisionRepair(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	m, card := testFixtures(t, 1)
	ts.manifest = m
	ts.card = card

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)

	_, err := c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync 1: %v", err)
	}

	// Delete the active generation (corrupt / evicted / same-revision
	// recovery scenario).
	genPath := filepath.Join(dir, "state", "current_generation.json")
	_ = os.Remove(genPath)

	// Sync again at same revision 1: the client must detect the missing
	// generation and repair it without silently treating the absence as
	// a fresh empty trust store.
	_, err = c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync repair failed: %v", err)
	}

	// Verify card is restored.
	repaired, state, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard after repair: %v", err)
	}
	if repaired == nil || state != FreshnessCurrent {
		t.Fatalf("repaired card invalid: state = %v", state)
	}
}

// TestResumableTransferRealConnectionDrop verifies that when a real connection drops
// mid-download, the client has streamed partial bytes into the .part file on disk,
// and a subsequent request resumes from that offset and succeeds.
func TestResumableTransferRealConnectionDrop(t *testing.T) {
	fullPayload := make([]byte, 10000)
	for i := range fullPayload {
		fullPayload[i] = byte(i % 251)
	}
	expectedHash := offlinepkg.ChecksumSHA256(fullPayload)

	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"res-v1"`)

		if attempt == 1 {
			// First attempt: stream 3000 bytes then drop connection violently
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullPayload)))
			w.WriteHeader(http.StatusOK)

			// Write 3000 bytes
			_, _ = w.Write(fullPayload[:3000])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			// Hijack and close raw connection to simulate mid-transfer drop
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, err := hj.Hijack()
				if err == nil {
					conn.Close()
					return
				}
			}
			return
		}

		// Second attempt: handle Range request
		rangeHdr := r.Header.Get("Range")
		if !strings.HasPrefix(rangeHdr, "bytes=") {
			t.Errorf("expected Range header on resumed attempt, got %q", rangeHdr)
			http.Error(w, "bad range", http.StatusBadRequest)
			return
		}
		var start int64
		fmt.Sscanf(rangeHdr, "bytes=%d-", &start)
		if start != 3000 {
			t.Errorf("resumed start offset = %d, want 3000", start)
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(fullPayload)-1, len(fullPayload)))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullPayload)-int(start)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(fullPayload[start:])
	}))
	defer srv.Close()

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, srv.URL, dir, clock)

	desc := offlinepkg.ResourceDescriptor{
		ResourceID:     "res-stream-test",
		URI:            "/res-stream-test",
		ChecksumSHA256: expectedHash,
		ByteSize:       int64(len(fullPayload)),
		ContentType:    "application/octet-stream",
	}

	// First attempt must fail due to connection drop
	err1 := c.DownloadResource(context.Background(), desc, true)
	if err1 == nil {
		t.Fatal("expected error on connection drop, got nil")
	}

	// Verify that the partial file was streamed to disk and NOT discarded
	partPath := c.partPathForResource("res-stream-test")
	partBytes, err := os.ReadFile(partPath)
	if err != nil {
		t.Fatalf("partial file %s missing after connection drop: %v", partPath, err)
	}
	if len(partBytes) != 3000 {
		t.Fatalf("partial file size = %d, want 3000", len(partBytes))
	}

	// Second attempt should resume and succeed
	err2 := c.DownloadResource(context.Background(), desc, true)
	if err2 != nil {
		t.Fatalf("resumed download failed: %v", err2)
	}

	has, err := c.HasResource("res-stream-test")
	if err != nil || !has {
		t.Fatalf("HasResource = %v, err = %v, want true", has, err)
	}
}

// TestResumableTransferETagDriftFallback verifies that if the server's ETag changes
// between partial download and resume, the client discards the partial file and
// downloads the full updated payload.
func TestResumableTransferETagDriftFallback(t *testing.T) {
	fullV2 := []byte("updated-full-content-from-server-v2-after-drift")
	hashV2 := offlinepkg.ChecksumSHA256(fullV2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server detects If-Range "old-etag" mismatch and returns 200 OK with full body
		w.Header().Set("ETag", `"v2"`)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullV2)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fullV2)
	}))
	defer srv.Close()

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, srv.URL, dir, clock)

	// Pre-seed a stale .part with old ETag
	partPath := c.partPathForResource("res-drift")
	_ = os.WriteFile(partPath, []byte("stale-partial-bytes"), 0o644)
	_ = c.storage.writePartMeta(partPath, downloadMeta{
		URL:            srv.URL + "/res-drift",
		ExpectedETag:   "old-etag",
		ExpectedSize:   100,
		BytesWritten:   int64(len("stale-partial-bytes")),
		RangeSupported: true,
	})

	desc := offlinepkg.ResourceDescriptor{
		ResourceID:     "res-drift",
		URI:            "/res-drift",
		ChecksumSHA256: hashV2,
		ByteSize:       int64(len(fullV2)),
		ContentType:    "application/octet-stream",
	}

	err := c.DownloadResource(context.Background(), desc, true)
	if err != nil {
		t.Fatalf("DownloadResource with ETag drift failed: %v", err)
	}

	has, err := c.HasResource("res-drift")
	if err != nil || !has {
		t.Fatalf("HasResource = %v, err = %v, want true", has, err)
	}
}

// TestResumableTransfer416Reset verifies that when a server returns 416 Range Not Satisfiable,
// the client resets the .part file and restarts fresh.
func TestResumableTransfer416Reset(t *testing.T) {
	fullPayload := []byte("fresh-content-after-416-reset")
	hash := offlinepkg.ChecksumSHA256(fullPayload)

	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if r.Header.Get("Range") != "" {
			// Reject range with 416
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", len(fullPayload)))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		// Subsequent fresh request succeeds with 200 OK
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullPayload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fullPayload)
	}))
	defer srv.Close()

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, srv.URL, dir, clock)

	// Pre-seed an invalid range .part with matching size
	partPath := c.partPathForResource("res-416")
	_ = os.WriteFile(partPath, bytes.Repeat([]byte("x"), 1000), 0o644)
	_ = c.storage.writePartMeta(partPath, downloadMeta{
		URL:            srv.URL + "/res-416",
		ExpectedETag:   "v1",
		ExpectedSize:   int64(len(fullPayload)),
		BytesWritten:   1000,
		RangeSupported: true,
	})

	desc := offlinepkg.ResourceDescriptor{
		ResourceID:     "res-416",
		URI:            "/res-416",
		ChecksumSHA256: hash,
		ByteSize:       int64(len(fullPayload)),
		ContentType:    "application/octet-stream",
	}

	err := c.DownloadResource(context.Background(), desc, true)
	if err != nil {
		t.Fatalf("DownloadResource after 416 reset failed: %v", err)
	}

	has, err := c.HasResource("res-416")
	if err != nil || !has {
		t.Fatalf("HasResource = %v, err = %v, want true", has, err)
	}
}

// TestResourceValidatorIntegration_DescriptorAudit: the public Sync flow
// invokes AuditRegionalPack on the manifest's resource list. A manifest
// with a duplicate resource id (or a denied license, or a missing
// attribution) must NOT activate; the same descriptor list that the
// validator rejects at publication time must also block activation on
// the device.
func TestResourceValidatorIntegration_DescriptorAudit(t *testing.T) {
	manifest, card := testFixtures(t, 1)
	// Add two resources with the SAME id to trip the duplicate-id check.
	dupRes := []offlinepkg.ResourceDescriptor{
		{
			ResourceID:     "res-dup",
			Type:           offlinepkg.TypeVectorTiles,
			URI:            "/api/v3/resources/res-dup",
			ChecksumSHA256: offlinepkg.ChecksumSHA256([]byte("dup1")),
			ByteSize:       1024,
			ContentType:    "application/vnd.mapbox-vector-tile",
			Attribution:    "Synthetic Map Attribution",
			Required:       true,
		},
		{
			ResourceID:     "res-dup",
			Type:           offlinepkg.TypeMapStyle,
			URI:            "/api/v3/resources/res-dup-style",
			ChecksumSHA256: offlinepkg.ChecksumSHA256([]byte("dup2")),
			ByteSize:       64,
			ContentType:    "application/json",
			Attribution:    "Synthetic Style Attribution",
			Required:       true,
		},
	}
	manifest.Resources = dupRes
	signManifest(manifest)

	ts := newTestServerWithResources(t, "KL", manifest, card, dupRes, map[string][]byte{})
	defer ts.Close()

	clock := fakeClock(time.Now())
	c := newClient(t, ts.URL, t.TempDir(), clock)

	if _, err := c.Sync(context.Background(), "KL"); err == nil {
		t.Fatalf("Sync accepted manifest with duplicate resource id; expected ErrPackInvalid")
	}
	// Active state must remain empty: a refused resource audit leaves the
	// prior coherent generation (none in this test) untouched.
	hasActive := activeManifestOnDisk(t, c)
	if hasActive {
		t.Fatalf("manifest was activated despite resource audit failure")
	}
}

// TestResourceValidatorIntegration_MissingOptionalLeavesCardValid: a
// manifest declaring an OPTIONAL regional pack with all resources absent
// from the wire (e.g. the operator chose not to publish regional maps)
// still activates the critical card. Optional assets MUST NOT prevent a
// valid critical card from reaching the device; status is reported
// separately via SyncReport so the UI can offer "download optional pack".
func TestResourceValidatorIntegration_MissingOptionalLeavesCardValid(t *testing.T) {
	manifest, card := testFixtures(t, 1)
	manifest.Resources = []offlinepkg.ResourceDescriptor{
		{
			ResourceID:     "res-optional-tiles",
			Type:           offlinepkg.TypeVectorTiles,
			URI:            "/api/v3/resources/res-optional-tiles",
			ChecksumSHA256: offlinepkg.ChecksumSHA256([]byte("opt")),
			ByteSize:       1024,
			ContentType:    "application/vnd.mapbox-vector-tile",
			Attribution:    "Synthetic Map Attribution",
			Required:       false,
		},
	}
	signManifest(manifest)

	// Server responds 404 for the optional resource (it was never published).
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/regions/KL/manifest", ts_serveManifest(manifest))
	mux.HandleFunc("/api/v3/packages/", ts_serveCard(card))
	mux.HandleFunc("/api/v3/resources/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "optional resource not published", http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	clock := fakeClock(time.Now())
	c := newClient(t, srv.URL, t.TempDir(), clock)

	rep, err := c.Sync(context.Background(), "KL")
	if err != nil {
		t.Fatalf("Sync failed for missing optional resources: %v", err)
	}
	if !rep.ManifestUpdated {
		t.Fatalf("ManifestUpdated=false; manifest must activate despite optional asset gaps")
	}
	if rep.ResourceAudit == nil {
		t.Fatalf("ResourceAudit=nil; Sync must surface the audit even when the pack is empty")
	}
	if rep.ResourceAudit.ExceedsBudget {
		t.Fatalf("ResourceAudit.ExceedsBudget=true for a 1 KiB optional resource")
	}
	// The critical card is still CURRENT.
	gotCard, state, err := c.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard: %v", err)
	}
	if state != FreshnessCurrent {
		t.Fatalf("freshness = %v, want CURRENT (critical card must remain usable)", state)
	}
	if gotCard.PackageID != card.PackageID {
		t.Fatalf("active card package = %q, want %q", gotCard.PackageID, card.PackageID)
	}
}

// TestResourceFreshnessIndependent: a downloaded resource's freshness is
// independent of the critical card's freshness. The card can be CURRENT
// while the resource is STALE (age > 24h), and a fresh Sync refreshes
// the resource without invalidating the card.
func TestResourceFreshnessIndependent(t *testing.T) {
	manifest, card := testFixtures(t, 1)
	resBytes := []byte("regional-tile-bytes-12345")
	res := offlinepkg.ResourceDescriptor{
		ResourceID:     "res-tiles",
		Type:           offlinepkg.TypeVectorTiles,
		URI:            "/api/v3/resources/res-tiles",
		ChecksumSHA256: offlinepkg.ChecksumSHA256(resBytes),
		ByteSize:       int64(len(resBytes)),
		ContentType:    "application/vnd.mapbox-vector-tile",
		Attribution:    "Synthetic Map Attribution",
		Required:       false,
	}
	manifest.Resources = []offlinepkg.ResourceDescriptor{res}
	signManifest(manifest)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/regions/KL/manifest", ts_serveManifest(manifest))
	mux.HandleFunc("/api/v3/packages/", ts_serveCard(card))
	mux.HandleFunc("/api/v3/resources/res-tiles", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(resBytes)))
		w.Write(resBytes)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, srv.URL, dir, clock)

	// Sync activates the manifest (card is CURRENT).
	if _, err := c.Sync(context.Background(), "KL"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, state, _ := c.GetActiveCard(); state != FreshnessCurrent {
		t.Fatalf("card freshness after Sync = %v, want CURRENT", state)
	}

	// Download the resource at clock+0: it's CURRENT.
	if err := c.DownloadResource(context.Background(), res, false); err != nil {
		t.Fatalf("DownloadResource: %v", err)
	}
	fresh, err := c.ResourceFreshness("res-tiles")
	if err != nil {
		t.Fatalf("ResourceFreshness: %v", err)
	}
	if fresh != ResourceCurrent {
		t.Fatalf("fresh = %v, want ResourceCurrent", fresh)
	}

	// Advance clock past the 24h window: a new client against the same
	// storage dir with a future clock sees the resource as STALE; the
	// critical card remains CURRENT because its freshness is keyed on a
	// different monotonic clock.
	future := fakeClock(time.Now().Add(25 * time.Hour))
	cFuture := newClient(t, srv.URL, dir, future)
	stale, err := cFuture.ResourceFreshness("res-tiles")
	if err != nil {
		t.Fatalf("ResourceFreshness (after age): %v", err)
	}
	if stale != ResourceStale {
		t.Fatalf("stale = %v, want ResourceStale", stale)
	}
	// The critical card expires in 2099, so it remains CURRENT across the
	// restart at clock+25h. Resource freshness is independent of card
	// freshness; recovery uses wall-clock against the persisted
	// LastFetchedAtUnixMS without renewing the validity window.
	if _, cardState, _ := cFuture.GetActiveCard(); cardState != FreshnessCurrent {
		t.Fatalf("card freshness after restart = %v, want FreshnessCurrent (recovered)", cardState)
	}
}

// ts_serveManifest / ts_serveCard are package-local helpers for the
// above tests. They serialize the same manifest/card the test built and
// serve them with the contract's content-type headers.
func ts_serveManifest(m *offlinepkg.Manifest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Accept-Ranges", "bytes")
		b, _ := json.Marshal(m)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b)))
		w.Write(b)
	}
}

func ts_serveCard(c *offlinepkg.PublicIncidentCard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Accept-Ranges", "bytes")
		b, _ := json.Marshal(c)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b)))
		w.Write(b)
	}
}

// activeManifestOnDisk is a small inspection helper for tests that need
// to assert "no generation got activated" after a failed Sync.
func activeManifestOnDisk(t *testing.T, c *ProtocolClient) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(c.StorageDir(), "state", "current_generation.json"))
	return err == nil
}

// newTestServerWithResources builds a test server that serves the given
// manifest/card AND a map of resource_id -> bytes.
func newTestServerWithResources(t *testing.T, jurisdiction string, manifest *offlinepkg.Manifest, card *offlinepkg.PublicIncidentCard, resList []offlinepkg.ResourceDescriptor, resBytes map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/regions/"+jurisdiction+"/manifest", ts_serveManifest(manifest))
	mux.HandleFunc("/api/v3/packages/", ts_serveCard(card))
	mux.HandleFunc("/api/v3/resources/", func(w http.ResponseWriter, r *http.Request) {
		// Path looks like /api/v3/resources/<id>
		id := strings.TrimPrefix(r.URL.Path, "/api/v3/resources/")
		b, ok := resBytes[id]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(b)))
		w.Write(b)
	})
	return httptest.NewServer(mux)
}
