package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"sthira/backend/internal/offlinepkg"
)

func TestPublishManifest_BoundaryValidation(t *testing.T) {
	st := &Store{} // no DB needed for boundary validation
	ctx := context.Background()

	// 1. Nil manifest
	if err := st.PublishManifest(ctx, nil); err == nil {
		t.Fatal("expected error for nil manifest")
	}

	// 2. Exceeds size ceiling
	huge := make([]byte, 262145)
	if err := st.PublishManifest(ctx, &PublishedManifest{
		ManifestID:   "m1",
		Jurisdiction: "KL",
		Revision:     1,
		PackageID:    "p1",
		RawJSON:      huge,
	}); err == nil {
		t.Fatal("expected error for manifest exceeding 256 KiB")
	}

	// 3. Malformed JSON
	if err := st.PublishManifest(ctx, &PublishedManifest{
		ManifestID:     "m1",
		Jurisdiction:   "KL",
		Revision:       1,
		PackageID:      "p1",
		RawJSON:        []byte("not-json"),
		ChecksumSHA256: strings.Repeat("a", 64),
	}); err == nil {
		t.Fatal("expected error for malformed JSON")
	}

	// 4. Invalid checksum length
	validJSON := []byte(`{"schema_version":"3.0","manifest_id":"m1","jurisdiction":"KL","revision":1,"package_id":"p1","created_at_utc":"2026-09-20T00:00:00Z","expires_at_utc":"2026-09-21T00:00:00Z","critical_card":{"package_id":"p1","version":1,"uri":"/card","checksum_sha256":"` + strings.Repeat("c", 64) + `","uncompressed_bytes":100},"checksum_sha256":"` + strings.Repeat("a", 64) + `","signature":{"key_id":"k1","value":"sig"}}`)
	if err := st.PublishManifest(ctx, &PublishedManifest{
		ManifestID:     "m1",
		Jurisdiction:   "KL",
		Revision:       1,
		PackageID:      "p1",
		RawJSON:        validJSON,
		ChecksumSHA256: "short",
	}); err == nil {
		t.Fatal("expected error for short checksum")
	}
}

func TestPublishCard_BoundaryValidation(t *testing.T) {
	st := &Store{}
	ctx := context.Background()

	// 1. Nil card
	if err := st.PublishCard(ctx, nil); err == nil {
		t.Fatal("expected error for nil card")
	}

	// 2. Exceeds 64 KiB ceiling
	huge := make([]byte, 65537)
	if err := st.PublishCard(ctx, &PublishedCard{
		PackageID: "p1",
		Version:   1,
		RawJSON:   huge,
	}); err == nil {
		t.Fatal("expected error for card exceeding 64 KiB")
	}

	// 3. Malformed JSON
	if err := st.PublishCard(ctx, &PublishedCard{
		PackageID:      "p1",
		Version:        1,
		RawJSON:        []byte("not-json"),
		ChecksumSHA256: strings.Repeat("a", 64),
	}); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestPublishResource_BoundaryValidation(t *testing.T) {
	st := &Store{}
	ctx := context.Background()

	// 1. Nil resource
	if err := st.PublishResource(ctx, nil); err == nil {
		t.Fatal("expected error for nil resource")
	}

	// 2. Checksum mismatch
	data := []byte("resource-content")
	if err := st.PublishResource(ctx, &PublishedResource{
		ResourceID:     "res-1",
		ContentType:    "application/octet-stream",
		ContentLength:  int64(len(data)),
		ChecksumSHA256: strings.Repeat("0", 64),
		Content:        data,
	}); err == nil {
		t.Fatal("expected error for resource checksum mismatch")
	}
}

// disposableTestDB creates a disposable database using STHIRA_TEST_ADMIN_DSN.
func disposableTestDB(t *testing.T) (*Store, func()) {
	t.Helper()
	admin := os.Getenv("STHIRA_TEST_ADMIN_DSN")
	if admin == "" {
		t.Skip("STHIRA_TEST_ADMIN_DSN unset; skipping disposable-DB test")
	}
	if _, err := exec.LookPath("psql"); err != nil {
		t.Skip("psql not on PATH; skipping disposable-DB test")
	}
	dbName := fmt.Sprintf("sthira_pubtest_%d_%d", time.Now().UnixNano(), os.Getpid())
	runCmd(t, "psql", admin, "-c", fmt.Sprintf(`CREATE DATABASE %s`, dbName))
	cleanup := func() {
		cmd := exec.Command("psql", admin, "-c", fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName))
		_ = cmd.Run()
	}
	dsn := strings.Replace(admin, "/postgres", "/"+dbName, 1)
	migDir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(migDir)
	if err != nil {
		cleanup()
		t.Fatalf("ReadDir(%s): %v", migDir, err)
	}
	var migFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migFiles = append(migFiles, entry.Name())
		}
	}
	sort.Strings(migFiles)
	for _, f := range migFiles {
		migPath := filepath.Join(migDir, f)
		runCmd(t, "psql", dsn, "-v", "ON_ERROR_STOP=1", "-q", "-f", migPath)
	}
	st, err := Open(dsn)
	if err != nil {
		cleanup()
		t.Fatalf("Open: %v", err)
	}
	return st, func() {
		_ = st.db.Close()
		cleanup()
	}
}

func runCmd(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, string(out))
	}
}

func validManifest(jurisdiction string, revision int) (*offlinepkg.Manifest, []byte) {
	m := &offlinepkg.Manifest{
		SchemaVersion: "3.0",
		ManifestID:    fmt.Sprintf("mnf-%s-%d", jurisdiction, revision),
		Jurisdiction:  jurisdiction,
		Revision:      revision,
		GeneratedAt:   "2026-09-20T00:00:00Z",
		ValidUntil:    "2026-09-21T00:00:00Z",
		SourceStatus:  "CURRENT",
		CriticalCard: offlinepkg.CriticalCardDescriptor{
			PackageID:         "pkg-1",
			Version:           1,
			URI:               "/api/v3/packages/pkg-1/versions/1",
			ChecksumSHA256:    strings.Repeat("a", 64),
			UncompressedBytes: 500,
			CompressedBytes:   100,
			ContentType:       "application/json",
		},
		Provenance: offlinepkg.ManifestProvenance{
			Authority:     "gov-in",
			DatasetID:     "ds-1",
			EvidenceClass: "AUTHORIZED_OPERATIONAL",
		},
	}
	m.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(m)
	m.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	m.Signature = &offlinepkg.Signature{
		Algorithm: "Ed25519",
		KeyID:     "key-1",
		Value:     "sig-1",
	}
	b, _ := json.Marshal(m)
	return m, b
}

func validCard(pkgID string, version int) (*offlinepkg.PublicIncidentCard, []byte) {
	c := &offlinepkg.PublicIncidentCard{
		SchemaVersion: "3.0",
		PackageID:     pkgID,
		Version:       version,
		Jurisdiction:  "KL",
		EvidenceClass: "AUTHORIZED_OPERATIONAL",
		EffectiveAt:   "2026-09-20T00:00:00Z",
		ExpiresAt:     "2026-09-21T00:00:00Z",
		Alert: offlinepkg.AlertCard{
			Identifier: "alt-1",
			Sender:     "snd-1",
			Headline:   "Emergency Warning",
			Severity:   "Extreme",
			Urgency:    "Immediate",
			Certainty:  "Observed",
		},
		RedZones: []offlinepkg.RedZoneCard{
			{ID: "rz-1", Name: "Red Zone 1"},
		},
		SafeZones: []offlinepkg.SafeZoneCard{
			{ID: "sz-1", Name: "Safe Zone 1", Role: "EMERGENCY_SHELTER", Status: "OPEN", CapacityMode: "DEFINED"},
		},
		ApprovedRoutes: []offlinepkg.RouteCard{},
		Facilities: []offlinepkg.FacilityCard{
			{ID: "fac-1", SafeZoneID: "sz-1", Name: "Facility 1"},
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
	c.Signature = &offlinepkg.Signature{
		Algorithm: "Ed25519",
		KeyID:     "key-1",
		Value:     "sig-1",
	}
	b, _ := json.Marshal(c)
	return c, b
}

func TestPublicationImmutabilityAndQuarantine(t *testing.T) {
	st, cleanup := disposableTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// --- 1. Manifest Immutability ---
	mObj1, mJSON1 := validManifest("KL", 1)
	m1 := &PublishedManifest{
		ManifestID:     mObj1.ManifestID,
		Jurisdiction:   mObj1.Jurisdiction,
		Revision:       mObj1.Revision,
		PackageID:      mObj1.CriticalCard.PackageID,
		RawJSON:        mJSON1,
		ChecksumSHA256: mObj1.ChecksumSHA256,
		SourceStatus:   "CURRENT",
	}
	if err := st.PublishManifest(ctx, m1); err != nil {
		t.Fatalf("first PublishManifest failed: %v", err)
	}

	// Idempotent retry with identical bytes and checksum must succeed
	if err := st.PublishManifest(ctx, m1); err != nil {
		t.Fatalf("idempotent PublishManifest failed: %v", err)
	}

	// Mutated content under same (jurisdiction, revision) MUST return ErrConflict
	mObj1Mutated, _ := validManifest("KL", 1)
	mObj1Mutated.ValidUntil = "2026-09-22T00:00:00Z"
	mObj1Mutated.ChecksumSHA256 = ""
	canMut, _ := offlinepkg.CanonicalBytes(mObj1Mutated)
	mObj1Mutated.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canMut)
	mJSON1Mutated, _ := json.Marshal(mObj1Mutated)

	m1Mutated := &PublishedManifest{
		ManifestID:     mObj1Mutated.ManifestID,
		Jurisdiction:   mObj1Mutated.Jurisdiction,
		Revision:       mObj1Mutated.Revision,
		PackageID:      mObj1Mutated.CriticalCard.PackageID,
		RawJSON:        mJSON1Mutated,
		ChecksumSHA256: mObj1Mutated.ChecksumSHA256,
		SourceStatus:   "CURRENT",
	}
	err := st.PublishManifest(ctx, m1Mutated)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on manifest mutation, got %v", err)
	}

	// Quarantine and status updates
	if err := st.QuarantineManifest(ctx, "KL", 1, true); err != nil {
		t.Fatalf("QuarantineManifest failed: %v", err)
	}
	gotM, err := st.GetPublishedManifest(ctx, "KL")
	if err != nil {
		t.Fatalf("GetPublishedManifest failed: %v", err)
	}
	if !gotM.Quarantined {
		t.Fatal("expected manifest to be quarantined")
	}

	// --- 2. Card Immutability ---
	cObj1, cJSON1 := validCard("pkg-wayanad", 1)
	c1 := &PublishedCard{
		PackageID:      cObj1.PackageID,
		Version:        cObj1.Version,
		RawJSON:        cJSON1,
		ChecksumSHA256: cObj1.ChecksumSHA256,
		SourceStatus:   "CURRENT",
	}
	if err := st.PublishCard(ctx, c1); err != nil {
		t.Fatalf("first PublishCard failed: %v", err)
	}

	// Idempotent retry
	if err := st.PublishCard(ctx, c1); err != nil {
		t.Fatalf("idempotent PublishCard failed: %v", err)
	}

	// Mutated content under same (package_id, version) MUST return ErrConflict
	cObj1Mutated, _ := validCard("pkg-wayanad", 1)
	cObj1Mutated.Alert.Headline = "Updated Headline"
	cObj1Mutated.ChecksumSHA256 = ""
	canCardMut, _ := offlinepkg.CanonicalBytes(cObj1Mutated)
	cObj1Mutated.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canCardMut)
	cJSON1Mutated, _ := json.Marshal(cObj1Mutated)

	c1Mutated := &PublishedCard{
		PackageID:      cObj1Mutated.PackageID,
		Version:        cObj1Mutated.Version,
		RawJSON:        cJSON1Mutated,
		ChecksumSHA256: cObj1Mutated.ChecksumSHA256,
		SourceStatus:   "CURRENT",
	}
	err = st.PublishCard(ctx, c1Mutated)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on card mutation, got %v", err)
	}

	// Quarantine card
	if err := st.QuarantineCard(ctx, "pkg-wayanad", 1, true); err != nil {
		t.Fatalf("QuarantineCard failed: %v", err)
	}
	gotC, err := st.GetPublishedCard(ctx, "pkg-wayanad", 1)
	if err != nil {
		t.Fatalf("GetPublishedCard failed: %v", err)
	}
	if !gotC.Quarantined {
		t.Fatal("expected card to be quarantined")
	}

	// --- 3. Resource Immutability ---
	resData := []byte("binary-asset-data")
	resHash := offlinepkg.ChecksumSHA256(resData)
	r1 := &PublishedResource{
		ResourceID:     "res-map-1",
		ContentType:    "application/octet-stream",
		ContentLength:  int64(len(resData)),
		ChecksumSHA256: resHash,
		Content:        resData,
	}
	if err := st.PublishResource(ctx, r1); err != nil {
		t.Fatalf("first PublishResource failed: %v", err)
	}

	// Idempotent retry
	if err := st.PublishResource(ctx, r1); err != nil {
		t.Fatalf("idempotent PublishResource failed: %v", err)
	}

	// Mutated content under same resource_id MUST return ErrConflict
	resDataMutated := []byte("binary-asset-data-MUTATED")
	r1Mutated := &PublishedResource{
		ResourceID:     "res-map-1",
		ContentType:    "application/octet-stream",
		ContentLength:  int64(len(resDataMutated)),
		ChecksumSHA256: offlinepkg.ChecksumSHA256(resDataMutated),
		Content:        resDataMutated,
	}
	err = st.PublishResource(ctx, r1Mutated)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on resource mutation, got %v", err)
	}
}
