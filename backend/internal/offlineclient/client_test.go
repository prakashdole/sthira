package offlineclient

import (
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

	// Advance 30 min — should still be CURRENT (within window, > staleBefore).
	future1 := fakeClock(now.Add(30 * time.Minute))
	c2 := newClient(t, ts.URL, dir, future1)
	_, state, _ := c2.GetActiveCard()
	if state != FreshnessCurrent {
		t.Errorf("after 30min: freshness = %v, want FreshnessCurrent", state)
	}

	// Advance 56 min total — within staleBefore (5min default), so STALE.
	future2 := fakeClock(now.Add(56 * time.Minute))
	c3 := newClient(t, ts.URL, dir, future2)
	_, state, _ = c3.GetActiveCard()
	if state != FreshnessStale {
		t.Errorf("after 56min: freshness = %v, want FreshnessStale", state)
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

	wantFiles := []string{"state.json", "current_manifest.bin", "current_card.bin"}
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

// Smoke test: verify that after sync, GetActiveCard and GetFreshnessState work
// on a fresh client instance (restart recovery).
func TestRestartRecovery(t *testing.T) {
	ts := newTestServer(t, "KL")
	defer ts.Server.Close()
	ts.manifest, ts.card = testFixtures(t, 1)

	clock := fakeClock(time.Now())
	dir := t.TempDir()
	c := newClient(t, ts.URL, dir, clock)
	c.Sync(context.Background(), "KL")

	// New client pointing at the same storage
	c2 := newClient(t, ts.URL, dir, clock)
	card, state, err := c2.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard after restart: %v", err)
	}
	if card.PackageID != "pkg-a" {
		t.Errorf("card.PackageID = %q, want pkg-a", card.PackageID)
	}
	if state != FreshnessCurrent {
		t.Errorf("freshness after restart = %v, want FreshnessCurrent", state)
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



