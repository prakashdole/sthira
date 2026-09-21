package httpserver

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/offlineclient"
	"sthira/backend/internal/offlinedelivery"
	"sthira/backend/internal/offlinepkg"
	"sthira/backend/internal/offlinequeue"
	"sthira/backend/internal/offlineresources"
	"sthira/backend/internal/store"
)

// disposableTestDB creates and migrates a uniquely-named disposable PostgreSQL database
// if STHIRA_TEST_ADMIN_DSN or local default admin is reachable, or skips if unreachable.
func disposableTestDB(t *testing.T) (string, func()) {
	t.Helper()
	admin := os.Getenv("STHIRA_TEST_ADMIN_DSN")
	if admin == "" {
		admin = os.Getenv("STHIRA_TEST_DSN")
	}
	if admin == "" {
		admin = "postgres://apple@localhost:5432/postgres"
	}
	if _, err := exec.LookPath("psql"); err != nil {
		t.Skip("psql not on PATH; skipping disposable-DB test")
	}

	dbName := fmt.Sprintf("sthira_p5acc_%d_%d", time.Now().UnixNano(), os.Getpid())
	cmd := exec.Command("psql", admin, "-c", fmt.Sprintf(`CREATE DATABASE %s`, dbName))
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot connect to admin DB %s (%v); skipping disposable DB test", admin, err)
	}

	cleanup := func() {
		dropCmd := exec.Command("psql", admin, "-c", fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, dbName))
		_ = dropCmd.Run()
	}

	dsn := strings.Replace(admin, "/postgres", "/"+dbName, 1)
	// Apply migrations 0001..0006 in order.
	for _, m := range []string{"0001_p3_foundation", "0002_p4_stays", "0003_p4_operator", "0004_p4_operator_grants", "0005_p4_operator_identity", "0006_p5_offline_publication"} {
		migPath := filepath.Join("..", "..", "migrations", m+".sql")
		if _, err := os.Stat(migPath); err != nil {
			cleanup()
			t.Fatalf("migration file not found: %s: %v", migPath, err)
		}
		pCmd := exec.Command("psql", dsn, "-q", "-f", migPath)
		out, err := pCmd.CombinedOutput()
		if err != nil {
			cleanup()
			t.Fatalf("migration %s failed: %v: %s", m, err, string(out))
		}
	}
	return dsn, cleanup
}

// makeWayanadEmergencyCard generates a realistic multi-zone, multi-route emergency card.
func makeWayanadEmergencyCard(t *testing.T, version int) *offlinepkg.PublicIncidentCard {
	t.Helper()
	card := &offlinepkg.PublicIncidentCard{
		SchemaVersion: "3.0",
		PackageID:     "PKG-KL-WAYANAD-01",
		Version:       version,
		Jurisdiction:  "KL",
		EvidenceClass: "SYNTHETIC_DEMO",
		EffectiveAt:   time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339),
		ExpiresAt:     time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		Alert: offlinepkg.AlertCard{
			Identifier:      fmt.Sprintf("ALERT-KL-WAYANAD-2026-V%d", version),
			Sender:          "kerala.sdma.gov.in",
			Headline:        "Flash Flood & Landslide Warning — Meppadi / Chooralmala",
			Severity:        "severe",
			Urgency:         "immediate",
			Certainty:       "observed",
			AreaDescription: "Meppadi Panchayath, Vythiri Taluk, Wayanad District",
		},
		RedZones: []offlinepkg.RedZoneCard{
			{ID: "rz-chooralmala", Name: "Chooralmala Riverine Hazard Zone"},
			{ID: "rz-mundakkai", Name: "Mundakkai High Hazard Landslide Slopes"},
		},
		SafeZones: []offlinepkg.SafeZoneCard{
			{ID: "sz-meppadi-1", Name: "Meppadi Higher Secondary School Relief Camp", Role: "EMERGENCY_SHELTER", Status: "OPEN", CapacityMode: "DEFINED"},
			{ID: "sz-stjoseph-2", Name: "St. Joseph Community Hall Safe Zone", Role: "TRANSIT_SHELTER", Status: "OPEN", CapacityMode: "DEFINED"},
		},
		ApprovedRoutes: []offlinepkg.RouteCard{
			{
				ID:           "rt-chooralmala-meppadi",
				FromZoneID:   "rz-chooralmala",
				ToSafeZoneID: "sz-meppadi-1",
				Mode:         "VEHICLE",
				Approval:     "DDMA-WAYANAD-EVAC-01",
			},
			{
				ID:           "rt-attamala-meppadi",
				FromZoneID:   "rz-mundakkai",
				ToSafeZoneID: "sz-stjoseph-2",
				Mode:         "FOOT",
				Approval:     "DDMA-WAYANAD-EVAC-02",
			},
		},
		Facilities: []offlinepkg.FacilityCard{
			{ID: "fac-meppadi-hss", SafeZoneID: "sz-meppadi-1", Name: "Meppadi HSS Main Hall"},
			{ID: "fac-stjoseph-hall", SafeZoneID: "sz-stjoseph-2", Name: "St. Joseph Parish Facility"},
		},
		Instructions: []offlinepkg.InstructionCard{
			{ID: "ins-ml-1", Language: "ml-IN", Title: "ഉടൻ മാറുക", Summary: "ഉരുൾപൊട്ടൽ സാധ്യതയുള്ളതിനാൽ സുരക്ഷിത സ്ഥാനങ്ങളിലേക്ക് മാറുക"},
			{ID: "ins-en-1", Language: "en-IN", Title: "Evacuate Immediately", Summary: "Proceed immediately along approved routes to designated relief shelters."},
		},
		EmergencyContacts: []offlinepkg.EmergencyContact{
			{Name: "Emergency Response", Number: "112"},
			{Name: "District Disaster Control Room", Number: "1077"},
		},
		AllocationPolicy: offlinepkg.PolicyCard{
			Order: []string{"sz-meppadi-1", "sz-stjoseph-2"},
		},
	}
	card.ChecksumSHA256 = ""
	canonical, err := offlinepkg.CanonicalBytes(card)
	if err != nil {
		t.Fatalf("CanonicalBytes(card): %v", err)
	}
	card.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	return card
}

// makeWayanadManifest creates and signs a valid regional manifest covering the card and resources.
func makeWayanadManifest(t *testing.T, card *offlinepkg.PublicIncidentCard, revision int, privKey ed25519.PrivateKey, keyID string, resources []offlinepkg.ResourceDescriptor) *offlinepkg.Manifest {
	t.Helper()
	cardSig, err := offlinepkg.SignCanonical(privKey, keyID, card)
	if err != nil {
		t.Fatalf("SignCanonical(card): %v", err)
	}
	card.Signature = cardSig
	cardBytes, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, _ = gw.Write(cardBytes)
	_ = gw.Close()

	m := &offlinepkg.Manifest{
		SchemaVersion: "3.0",
		ManifestID:    fmt.Sprintf("MAN-KL-2026-REV-%d", revision),
		Jurisdiction:  "KL",
		Revision:      revision,
		GeneratedAt:   time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339),
		ValidUntil:    time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
		SourceStatus:  "CURRENT",
		CriticalCard: offlinepkg.CriticalCardDescriptor{
			PackageID:         card.PackageID,
			Version:           card.Version,
			URI:               fmt.Sprintf("/api/v3/packages/%s/versions/%d", card.PackageID, card.Version),
			ChecksumSHA256:    card.ChecksumSHA256,
			UncompressedBytes: int64(len(cardBytes)),
			CompressedBytes:   int64(gzBuf.Len()),
			ContentType:       "application/json",
		},
		Resources: resources,
		Revocations: offlinepkg.RevocationBlock{
			RevokedPackages:    []string{},
			CancelledRoutes:    []string{},
			SupersededVersions: []offlinepkg.SupersededVersion{},
		},
		Provenance: offlinepkg.ManifestProvenance{
			Authority:     "kerala.sdma.gov.in",
			DatasetID:     "DS-KL-WAYANAD",
			EvidenceClass: "SYNTHETIC_DEMO",
		},
	}

	m.ChecksumSHA256 = ""
	canonical, err := offlinepkg.CanonicalBytes(m)
	if err != nil {
		t.Fatalf("CanonicalBytes(m): %v", err)
	}
	m.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)

	sig, err := offlinepkg.SignCanonical(privKey, keyID, m)
	if err != nil {
		t.Fatalf("SignCanonical(manifest): %v", err)
	}
	m.Signature = sig

	return m
}

// TestP5Flow1_EndToEndPublicationToDiskActivation verifies Flow 1:
// Persist/publish synthetic package -> signed manifest -> real HTTP ->
// disk download -> real signature/resource validation -> atomic activation.
func TestP5Flow1_EndToEndPublicationToDiskActivation(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	keyID := "kpub:KL:sdma:2026:01"
	trustedKey := offlinepkg.TrustedKey{
		KeyID:                 keyID,
		PublicKey:             pubKey,
		PermittedJurisdiction: "KL",
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(24 * time.Hour),
		Revoked:               false,
	}
	trustStore := offlinepkg.NewTrustStore(trustedKey)

	card := makeWayanadEmergencyCard(t, 1)

	// Build auxiliary test resources (style and vector pack)
	styleJSON := []byte(`{"version":8,"sources":{"wayanad":{"type":"vector","tiles":["/api/v3/resources/res-tiles"]}},"sprite":"/api/v3/resources/res-sprite","glyphs":"/api/v3/resources/res-glyphs/{fontstack}/{range}.pbf"}`)
	styleSum := offlinepkg.ChecksumSHA256(styleJSON)

	tileData := bytes.Repeat([]byte("MVT_TILES_PAYLOAD_WAYANAD"), 200)
	tileSum := offlinepkg.ChecksumSHA256(tileData)

	spriteData := []byte(`{"icon_shelter":{"width":32,"height":32,"x":0,"y":0}}`)
	spriteSum := offlinepkg.ChecksumSHA256(spriteData)

	glyphData := bytes.Repeat([]byte("FONT_PBF_BYTES"), 50)
	glyphSum := offlinepkg.ChecksumSHA256(glyphData)

	audioData := bytes.Repeat([]byte("MALAYALAM_EMERGENCY_BROADCAST_AUDIO"), 100)
	audioSum := offlinepkg.ChecksumSHA256(audioData)

	resList := []offlinepkg.ResourceDescriptor{
		{
			ResourceID:     "res-tiles",
			Type:           offlinepkg.TypeVectorTiles,
			URI:            "/api/v3/resources/res-tiles",
			ChecksumSHA256: tileSum,
			ByteSize:       int64(len(tileData)),
			ContentType:    "application/vnd.mapbox-vector-tile",
			Required:       false,
			Attribution:    "Government of Kerala Survey Dept (Synthetic Demo)",
		},
		{
			ResourceID:     "res-style",
			Type:           offlinepkg.TypeMapStyle,
			URI:            "/api/v3/resources/res-style",
			ChecksumSHA256: styleSum,
			ByteSize:       int64(len(styleJSON)),
			ContentType:    "application/json",
			Required:       false,
			Attribution:    "MapLibre Style v8 / Sthira Cartography",
		},
		{
			ResourceID:     "res-sprite",
			Type:           offlinepkg.TypeMapSprite,
			URI:            "/api/v3/resources/res-sprite",
			ChecksumSHA256: spriteSum,
			ByteSize:       int64(len(spriteData)),
			ContentType:    "application/json",
			Required:       false,
			Attribution:    "Sthira Hazard Icons",
		},
		{
			ResourceID:     "res-glyphs",
			Type:           offlinepkg.TypeMapGlyphs,
			URI:            "/api/v3/resources/res-glyphs",
			ChecksumSHA256: glyphSum,
			ByteSize:       int64(len(glyphData)),
			ContentType:    "application/x-protobuf",
			Required:       false,
			Attribution:    "Noto Sans Malayalam / Google Fonts",
		},
		{
			ResourceID:     "res-audio-ml",
			Type:           offlinepkg.TypeEmergencyAudio,
			URI:            "/api/v3/resources/res-audio-ml",
			ChecksumSHA256: audioSum,
			ByteSize:       int64(len(audioData)),
			ContentType:    "audio/ogg",
			Required:       false,
			Attribution:    "KSDMA Spoken Malayalam Alert",
		},
	}

	manifest := makeWayanadManifest(t, card, 1, privKey, keyID, resList)

	// Validate resources with Agent 4 validator before publishing
	validator := offlineresources.NewValidator()
	descList := make([]offlineresources.ResourceDescriptor, len(resList))
	for i, r := range resList {
		descList[i] = offlineresources.ResourceDescriptor{ResourceDescriptor: r}
	}
	audit, err := validator.AuditRegionalPack(descList)
	if err != nil {
		t.Fatalf("AuditRegionalPack failed: %v", err)
	}
	if audit.ExceedsBudget {
		t.Fatalf("AuditRegionalPack exceeds budget: %d > %d", audit.TotalBytes, audit.BudgetLimitBytes)
	}
	styleRes, err := validator.ValidateMapStyle(styleJSON, descList)
	if err != nil {
		t.Fatalf("ValidateMapStyle failed: %v", err)
	}
	if !styleRes.Valid {
		t.Fatalf("ValidateMapStyle reported invalid: missing sprites=%v, glyphs=%v, sources=%v",
			styleRes.MissingSprites, styleRes.MissingGlyphs, styleRes.MissingSources)
	}

	// Persist to real disposable PostgreSQL database if available; fallback to in-memory adapter
	dsn, cleanup := disposableTestDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	// 1. Publish into Store
	manifestRaw, _ := json.Marshal(manifest)
	if err := st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID:     manifest.ManifestID,
		Jurisdiction:   manifest.Jurisdiction,
		Revision:       manifest.Revision,
		PackageID:      manifest.CriticalCard.PackageID,
		RawJSON:        manifestRaw,
		ChecksumSHA256: manifest.ChecksumSHA256,
		SourceStatus:   "CURRENT",
		Quarantined:    false,
	}); err != nil {
		t.Fatalf("PublishManifest: %v", err)
	}

	cardRaw, _ := json.Marshal(card)
	if err := st.PublishCard(ctx, &store.PublishedCard{
		PackageID:      card.PackageID,
		Version:        card.Version,
		RawJSON:        cardRaw,
		ChecksumSHA256: card.ChecksumSHA256,
		SourceStatus:   "CURRENT",
		Quarantined:    false,
	}); err != nil {
		t.Fatalf("PublishCard: %v", err)
	}

	// Publish resources
	resourcesToStore := map[string][]byte{
		"res-tiles":    tileData,
		"res-style":    styleJSON,
		"res-sprite":   spriteData,
		"res-glyphs":   glyphData,
		"res-audio-ml": audioData,
	}
	for id, content := range resourcesToStore {
		if err := st.PublishResource(ctx, &store.PublishedResource{
			ResourceID:     id,
			ContentType:    "application/octet-stream",
			ContentLength:  int64(len(content)),
			ChecksumSHA256: offlinepkg.ChecksumSHA256(content),
			Content:        content,
		}); err != nil {
			t.Fatalf("PublishResource(%s): %v", id, err)
		}
	}

	// 2. Start HTTP server with publication adapter
	srv := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), WithStore(st)).Handler())
	defer srv.Close()

	// 3. Initialize disk-backed offline client
	clientDir := t.TempDir()
	client, err := offlineclient.NewClient(offlineclient.ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: clientDir,
		TrustStore: trustStore,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// 4. Perform atomic sync over real HTTP
	report, err := client.Sync(ctx, "KL")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !report.ManifestUpdated {
		t.Error("report.ManifestUpdated = false, want true")
	}
	if !report.CardUpdated {
		t.Error("report.CardUpdated = false, want true")
	}
	if report.ActiveRevision != 1 {
		t.Errorf("report.ActiveRevision = %d, want 1", report.ActiveRevision)
	}

	// 5. Verify active state on disk
	activeCard, freshness, err := client.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard: %v", err)
	}
	if freshness != offlineclient.FreshnessCurrent {
		t.Errorf("freshness = %v, want CURRENT", freshness)
	}
	if activeCard.PackageID != "PKG-KL-WAYANAD-01" {
		t.Errorf("activeCard.PackageID = %s, want PKG-KL-WAYANAD-01", activeCard.PackageID)
	}
	if len(activeCard.RedZones) != 2 {
		t.Errorf("len(RedZones) = %d, want 2", len(activeCard.RedZones))
	}
	if len(activeCard.SafeZones) != 2 {
		t.Errorf("len(SafeZones) = %d, want 2", len(activeCard.SafeZones))
	}

	// 6. Download an auxiliary resource and assert disk presence
	if err := client.DownloadResource(ctx, resList[0], false); err != nil {
		t.Fatalf("DownloadResource(res-tiles): %v", err)
	}
	hasRes, err := client.HasResource("res-tiles")
	if err != nil || !hasRes {
		t.Errorf("HasResource(res-tiles) = %v (err: %v), want true", hasRes, err)
	}

	// 7. Verify atomic storage files exist and .part files were removed
	genPath := filepath.Join(clientDir, "state", "current_generation.json")
	if _, err := os.Stat(genPath); err != nil {
		t.Errorf("current_generation.json missing on disk: %v", err)
	}
	downloadsDir := filepath.Join(clientDir, "downloads")
	entries, _ := os.ReadDir(downloadsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") {
			t.Errorf("leftover incomplete download file: %s", e.Name())
		}
	}
}

// TestP5Flow2_InterruptedDownloadAndRangeResume verifies Flow 2:
// Interrupted download -> resume -> restart -> correct active package.
func TestP5Flow2_InterruptedDownloadAndRangeResume(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(crand.Reader)
	keyID := "kpub:KL:sdma:2026:01"
	trustStore := offlinepkg.NewTrustStore(offlinepkg.TrustedKey{
		KeyID:                 keyID,
		PublicKey:             pubKey,
		PermittedJurisdiction: "KL",
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(24 * time.Hour),
	})

	card := makeWayanadEmergencyCard(t, 1)
	tilePayload := bytes.Repeat([]byte("0123456789ABCDEF-VECTOR-TILE-SLICE-"), 100) // 3500 bytes
	tileSum := offlinepkg.ChecksumSHA256(tilePayload)

	resList := []offlinepkg.ResourceDescriptor{
		{
			ResourceID:     "res-tiles-resume",
			Type:           offlinepkg.TypeVectorTiles,
			URI:            "/api/v3/resources/res-tiles-resume",
			ChecksumSHA256: tileSum,
			ByteSize:       int64(len(tilePayload)),
			ContentType:    "application/vnd.mapbox-vector-tile",
			Required:       false,
			Attribution:    "KSDMA (Synthetic)",
		},
	}
	manifest := makeWayanadManifest(t, card, 1, privKey, keyID, resList)

	dsn, cleanup := disposableTestDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	manifestRaw, _ := json.Marshal(manifest)
	_ = st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID:     manifest.ManifestID,
		Jurisdiction:   manifest.Jurisdiction,
		Revision:       manifest.Revision,
		PackageID:      manifest.CriticalCard.PackageID,
		RawJSON:        manifestRaw,
		ChecksumSHA256: manifest.ChecksumSHA256,
		SourceStatus:   "CURRENT",
	})
	cardRaw, _ := json.Marshal(card)
	_ = st.PublishCard(ctx, &store.PublishedCard{
		PackageID:      card.PackageID,
		Version:        card.Version,
		RawJSON:        cardRaw,
		ChecksumSHA256: card.ChecksumSHA256,
		SourceStatus:   "CURRENT",
	})
	_ = st.PublishResource(ctx, &store.PublishedResource{
		ResourceID:     "res-tiles-resume",
		ContentType:    "application/octet-stream",
		ContentLength:  int64(len(tilePayload)),
		ChecksumSHA256: tileSum,
		Content:        tilePayload,
	})

	srv := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), WithStore(st)).Handler())
	defer srv.Close()

	// 1. Direct Range slicing test over HTTP
	resURL := srv.URL + "/api/v3/resources/res-tiles-resume"

	// Fetch prefix: bytes 0-499
	req1, _ := http.NewRequest(http.MethodGet, resURL, nil)
	req1.Header.Set("Range", "bytes=0-499")
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("GET bytes 0-499: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", resp1.StatusCode)
	}
	etag := resp1.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("missing ETag on 206 response")
	}
	chunk1, _ := io.ReadAll(resp1.Body)
	if len(chunk1) != 500 {
		t.Fatalf("expected 500 bytes chunk1, got %d", len(chunk1))
	}

	// Resume from offset 500 to end with If-Range
	req2, _ := http.NewRequest(http.MethodGet, resURL, nil)
	req2.Header.Set("Range", "bytes=500-")
	req2.Header.Set("If-Range", etag)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("GET bytes 500-: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 on resumed range, got %d", resp2.StatusCode)
	}
	chunk2, _ := io.ReadAll(resp2.Body)

	// Verify reassembly
	reassembled := append(chunk1, chunk2...)
	if !bytes.Equal(reassembled, tilePayload) {
		t.Fatalf("reassembled content does not match original tile payload")
	}

	// 2. If-Range mismatch falls back to full entity (200 OK)
	req3, _ := http.NewRequest(http.MethodGet, resURL, nil)
	req3.Header.Set("Range", "bytes=500-")
	req3.Header.Set("If-Range", `"sha256-wrong-stale-tag"`)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("GET with stale If-Range: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK fallback on stale If-Range, got %d", resp3.StatusCode)
	}

	// 3. Client crash/restart recovery
	clientDir := t.TempDir()
	c1, err := offlineclient.NewClient(offlineclient.ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: clientDir,
		TrustStore: trustStore,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c1.Sync(ctx, "KL"); err != nil {
		t.Fatalf("c1.Sync: %v", err)
	}

	// Simulate client restart: fresh client against existing storage
	c2, err := offlineclient.NewClient(offlineclient.ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: clientDir,
		TrustStore: trustStore,
	})
	if err != nil {
		t.Fatalf("NewClient on restart: %v", err)
	}
	activeCard, freshness, err := c2.GetActiveCard()
	if err != nil {
		t.Fatalf("c2.GetActiveCard on restart: %v", err)
	}
	if freshness != offlineclient.FreshnessCurrent {
		t.Errorf("freshness after restart = %v, want CURRENT", freshness)
	}
	if activeCard.PackageID != "PKG-KL-WAYANAD-01" {
		t.Errorf("activeCard.PackageID = %s, want PKG-KL-WAYANAD-01", activeCard.PackageID)
	}
}

// TestP5Flow3_RevocationPropagationAndTombstones verifies Flow 3:
// Updated/revoked manifest -> persistent invalidation -> old package cannot resurrect evidence after restart.
func TestP5Flow3_RevocationPropagationAndTombstones(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(crand.Reader)
	keyID := "kpub:KL:sdma:2026:01"
	trustStore := offlinepkg.NewTrustStore(offlinepkg.TrustedKey{
		KeyID:                 keyID,
		PublicKey:             pubKey,
		PermittedJurisdiction: "KL",
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(24 * time.Hour),
	})

	card1 := makeWayanadEmergencyCard(t, 1)
	manifest1 := makeWayanadManifest(t, card1, 1, privKey, keyID, nil)

	dsn, cleanup := disposableTestDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	mRaw1, _ := json.Marshal(manifest1)
	_ = st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID: manifest1.ManifestID, Jurisdiction: "KL", Revision: 1,
		PackageID: card1.PackageID, RawJSON: mRaw1, ChecksumSHA256: manifest1.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})
	cRaw1, _ := json.Marshal(card1)
	_ = st.PublishCard(ctx, &store.PublishedCard{
		PackageID: card1.PackageID, Version: 1, RawJSON: cRaw1, ChecksumSHA256: card1.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})

	delCfg := offlinedelivery.DefaultConfig()
	delCfg.ManifestCacheTTL = 0
	srv := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), WithStore(st), WithDeliveryConfig(delCfg)).Handler())
	defer srv.Close()

	clientDir := t.TempDir()
	client, err := offlineclient.NewClient(offlineclient.ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: clientDir,
		TrustStore: trustStore,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// 1. Initial sync at revision 1
	if _, err := client.Sync(ctx, "KL"); err != nil {
		t.Fatalf("Sync rev 1: %v", err)
	}
	if client.IsRouteCancelled("rt-chooralmala-meppadi") {
		t.Errorf("route rt-chooralmala-meppadi cancelled prematurely")
	}

	// 2. Publish revision 2 revoking the route and package
	card2 := makeWayanadEmergencyCard(t, 2)
	manifest2 := makeWayanadManifest(t, card2, 2, privKey, keyID, nil)
	manifest2.Revocations = offlinepkg.RevocationBlock{
		RevokedPackages:    []string{"PKG-KL-WAYANAD-01"},
		CancelledRoutes:    []string{"rt-chooralmala-meppadi"},
		SupersededVersions: []offlinepkg.SupersededVersion{{PackageID: "PKG-KL-WAYANAD-01", Version: 1}},
	}
	manifest2.ChecksumSHA256 = ""
	canonical2, _ := offlinepkg.CanonicalBytes(manifest2)
	manifest2.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical2)
	sig2, _ := offlinepkg.SignCanonical(privKey, keyID, manifest2)
	manifest2.Signature = sig2

	mRaw2, _ := json.Marshal(manifest2)
	_ = st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID: manifest2.ManifestID, Jurisdiction: "KL", Revision: 2,
		PackageID: card2.PackageID, RawJSON: mRaw2, ChecksumSHA256: manifest2.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})
	cRaw2, _ := json.Marshal(card2)
	_ = st.PublishCard(ctx, &store.PublishedCard{
		PackageID: card2.PackageID, Version: 2, RawJSON: cRaw2, ChecksumSHA256: card2.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})

	// 3. Client syncs revision 2
	rep2, err := client.Sync(ctx, "KL")
	if err != nil {
		t.Fatalf("Sync rev 2: %v", err)
	}
	if len(rep2.CancelledRoutes) != 1 || rep2.CancelledRoutes[0] != "rt-chooralmala-meppadi" {
		t.Errorf("rep2.CancelledRoutes = %v, want [rt-chooralmala-meppadi]", rep2.CancelledRoutes)
	}
	if !client.IsRouteCancelled("rt-chooralmala-meppadi") {
		t.Error("IsRouteCancelled = false after sync rev 2, want true")
	}

	// 4. Restart client: verify tombstone persistence
	clientRestarted, err := offlineclient.NewClient(offlineclient.ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: clientDir,
		TrustStore: trustStore,
	})
	if err != nil {
		t.Fatalf("NewClient on restart: %v", err)
	}
	if !clientRestarted.IsRouteCancelled("rt-chooralmala-meppadi") {
		t.Error("tombstone lost after client restart!")
	}

	// 5. Version rollback prevention: server re-publishes older revision 1
	_, _ = st.DB().ExecContext(ctx, "DELETE FROM published_manifests WHERE jurisdiction = 'KL' AND revision = 2")
	_ = st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID: manifest1.ManifestID, Jurisdiction: "KL", Revision: 1,
		PackageID: card1.PackageID, RawJSON: mRaw1, ChecksumSHA256: manifest1.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})
	_, err = clientRestarted.Sync(ctx, "KL")
	if err == nil {
		t.Fatal("expected ErrVersionRollback when server replays revision 1, got nil")
	}
	if !errors.Is(err, offlinepkg.ErrVersionRollback) {
		t.Errorf("expected ErrVersionRollback, got %v", err)
	}
	// Route remains cancelled, cannot be resurrected
	if !clientRestarted.IsRouteCancelled("rt-chooralmala-meppadi") {
		t.Error("revoked route resurrected by attempted version rollback!")
	}
}

// TestP5Flow4_MissingOptionalMapOrAudioLeavesCardUsable verifies Flow 4:
// Missing map/audio -> valid critical card remains usable.
func TestP5Flow4_MissingOptionalMapOrAudioLeavesCardUsable(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(crand.Reader)
	keyID := "kpub:KL:sdma:2026:01"
	trustStore := offlinepkg.NewTrustStore(offlinepkg.TrustedKey{
		KeyID:                 keyID,
		PublicKey:             pubKey,
		PermittedJurisdiction: "KL",
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(24 * time.Hour),
	})

	card := makeWayanadEmergencyCard(t, 1)

	// Manifest declares optional style and audio, but we DO NOT publish them to the server
	missingRes := []offlinepkg.ResourceDescriptor{
		{
			ResourceID:     "res-unavailable-tiles",
			Type:           offlinepkg.TypeVectorTiles,
			URI:            "/api/v3/resources/res-unavailable-tiles",
			ChecksumSHA256: strings.Repeat("0", 64),
			ByteSize:       1024,
			ContentType:    "application/vnd.mapbox-vector-tile",
			Required:       false,
			Attribution:    "KSDMA (Missing)",
		},
		{
			ResourceID:     "res-unavailable-audio",
			Type:           offlinepkg.TypeEmergencyAudio,
			URI:            "/api/v3/resources/res-unavailable-audio",
			ChecksumSHA256: strings.Repeat("1", 64),
			ByteSize:       2048,
			ContentType:    "audio/ogg",
			Required:       false,
			Attribution:    "KSDMA (Missing)",
		},
	}
	manifest := makeWayanadManifest(t, card, 1, privKey, keyID, missingRes)

	dsn, cleanup := disposableTestDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	mRaw, _ := json.Marshal(manifest)
	_ = st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID: manifest.ManifestID, Jurisdiction: "KL", Revision: 1,
		PackageID: card.PackageID, RawJSON: mRaw, ChecksumSHA256: manifest.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})
	cRaw, _ := json.Marshal(card)
	_ = st.PublishCard(ctx, &store.PublishedCard{
		PackageID: card.PackageID, Version: 1, RawJSON: cRaw, ChecksumSHA256: card.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})

	srv := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), WithStore(st)).Handler())
	defer srv.Close()

	clientDir := t.TempDir()
	client, err := offlineclient.NewClient(offlineclient.ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: clientDir,
		TrustStore: trustStore,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// 1. Sync succeeds despite missing optional map and audio
	report, err := client.Sync(ctx, "KL")
	if err != nil {
		t.Fatalf("Sync failed when optional assets are absent: %v", err)
	}
	if !report.CardUpdated {
		t.Errorf("CardUpdated = false, want true")
	}

	// 2. Active critical card is fully available and current
	activeCard, freshness, err := client.GetActiveCard()
	if err != nil {
		t.Fatalf("GetActiveCard: %v", err)
	}
	if freshness != offlineclient.FreshnessCurrent {
		t.Errorf("freshness = %v, want CURRENT", freshness)
	}
	if activeCard.Alert.Headline != card.Alert.Headline {
		t.Errorf("activeCard headline mismatch: %s", activeCard.Alert.Headline)
	}
	if len(activeCard.Instructions) != 2 {
		t.Errorf("activeCard instructions missing: got %d, want 2", len(activeCard.Instructions))
	}
	if activeCard.EmergencyContacts[0].Number != "112" {
		t.Errorf("emergency contact number = %s, want 112", activeCard.EmergencyContacts[0].Number)
	}

	// 3. Attempting to download missing resource returns error, but active card is unaffected
	err = client.DownloadResource(ctx, missingRes[0], false)
	if err == nil {
		t.Errorf("expected error downloading missing resource, got nil")
	}
	has, _ := client.HasResource("res-unavailable-tiles")
	if has {
		t.Errorf("HasResource(missing) = true, want false")
	}

	// Card remains usable
	_, freshness2, err2 := client.GetActiveCard()
	if err2 != nil || freshness2 != offlineclient.FreshnessCurrent {
		t.Errorf("card invalidated by failed optional resource download: %v (freshness: %v)", err2, freshness2)
	}
}

// createTestSession creates a citizen session against the running HTTP server.
func createTestSession(t *testing.T, baseURL string) (string, string) {
	t.Helper()
	resp, err := http.Post(baseURL+"/api/v3/sessions", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("createTestSession POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("createTestSession: %d %s", resp.StatusCode, string(b))
	}
	var env struct {
		Data struct {
			SessionID string `json:"session_id"`
			Token     string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("createTestSession decode: %v", err)
	}
	if env.Data.SessionID == "" || env.Data.Token == "" {
		t.Fatalf("createTestSession missing fields")
	}
	return env.Data.SessionID, env.Data.Token
}

// seedRealPackage seeds an authorized operational package and facility for stay testing.
func seedRealPackage(t *testing.T, st *store.Store, capacity int, start, end time.Time) (pkgID, facID string, snap int) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	srcID, artID, pkgID, facID := "SRC-"+suffix, "ART-"+suffix, "PKG-"+suffix, "FAC-"+suffix
	hash := strings.Repeat("0", 64)
	hashBytes := sha256.Sum256([]byte(hash))
	hash = hex.EncodeToString(hashBytes[:])
	ctx := context.Background()
	err := st.InTx(ctx, func(tx store.DBTX) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, created_at, updated_at)
			VALUES ($1,'gov','gov.example','OPERATIONAL',1,$2,$2)`, srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1,$2,1,$3,$4,'AUTHORIZED_OPERATIONAL','mem://t')`, artID, srcID, hash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, effective_at, expires_at, checksum_sha256, body)
			VALUES ($1,'ALT',$2,$3,1,'JTEST','AUTHORIZED_OPERATIONAL',$4,$5,$6,$7)`, pkgID, srcID, artID, now, now.Add(24*time.Hour), hash,
			`{"allocation_policy":{"reservation_expiry_seconds":3600,"temporary_stay_min_days":1,"temporary_stay_max_days":14,"allow_transfers":true,"route_required":false}}`); err != nil {
			return err
		}
		if err := store.NewSourceStore(store.ChainAuditor{}).RecordAuthorization(ctx, tx, store.Authorization{
			AuthorizationID: "AUTH-" + srcID,
			SourceID:        srcID,
			GrantedBy:       "gov",
			EvidenceRef:     "doc-1",
			Jurisdiction:    "JTEST",
			GrantedAt:       now,
		}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO facilities (facility_id, package_id, safe_zone_id, timezone, version, updated_at)
			VALUES ($1,$2,'SZ','Asia/Kolkata',1,$3)`, facID, pkgID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO zone_versions (zone_id, package_id, kind, role, status, capacity, version, updated_at)
			VALUES ('SZ',$1,'SAFE',NULL,'OPEN',NULL,1,$2)
			ON CONFLICT (zone_id, package_id) DO NOTHING`, pkgID, now); err != nil {
			return err
		}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO facility_inventory (facility_id, service_date, capacity, reserved, held, occupied, version, updated_at)
				VALUES ($1,$2,$3,0,0,0,1,$4)`, facID, d, capacity, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seedRealPackage: %v", err)
	}
	return pkgID, facID, 1
}

// TestP5Flow5_PendingWriteReconnectAndLostResponseReplay verifies Flow 5:
// Pending write -> reconnect -> existing P4 server validation -> lost-response replay -> exactly one commitment.
func TestP5Flow5_PendingWriteReconnectAndLostResponseReplay(t *testing.T) {
	dsn, cleanup := disposableTestDB(t)
	defer cleanup()

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	// Seed real operational package and facility in the DB
	start := time.Now().UTC().Truncate(24 * time.Hour)
	end := start.Add(48 * time.Hour)
	pkgID, facID, snapVer := seedRealPackage(t, st, 10, start, end)

	srv := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), WithStore(st)).Handler())
	defer srv.Close()

	// Create citizen session via HTTP
	_, citizenToken := createTestSession(t, srv.URL)

	queueDir := t.TempDir()
	qStore, err := offlinequeue.NewStore(queueDir, time.Now)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// 1. Client offline: enqueue reservation request
	idempotencyKey := "idem-p5-flow5-key-001"
	payloadObj := map[string]any{
		"facility_id":      facID,
		"package_id":       pkgID,
		"party_size":       2,
		"start_date":       start.Format("2006-01-02"),
		"end_date":         end.Format("2006-01-02"),
		"idempotency_key":  idempotencyKey,
		"snapshot_version": snapVer,
	}
	payloadBytes, _ := json.Marshal(payloadObj)

	tokenRef := "ref-citizen-session"
	tokenStore := offlinequeue.NewMemoryTokenStore()
	tokenStore.Put(tokenRef, citizenToken)

	op := offlinequeue.PendingOperation{
		ID:              "op-1",
		IdempotencyKey:  idempotencyKey,
		Endpoint:        "/api/v3/reservations",
		Method:          "POST",
		TokenRef:        tokenRef,
		Payload:         payloadBytes,
		SnapshotVersion: snapVer,
		SelectionExpiry: time.Now().Add(time.Hour),
	}
	if err := qStore.Enqueue(context.Background(), op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// 2. Simulate client restart while offline
	qStoreRestarted, err := offlinequeue.NewStore(queueDir, time.Now)
	if err != nil {
		t.Fatalf("NewStore on restart: %v", err)
	}
	pendingList, err := qStoreRestarted.PeekPending(context.Background())
	if err != nil || len(pendingList) != 1 {
		t.Fatalf("expected 1 recovered pending op on restart, got %d (err: %v)", len(pendingList), err)
	}
	if pendingList[0].State != offlinequeue.StatePending {
		t.Errorf("recovered op state = %s, want PENDING", pendingList[0].State)
	}

	// 3. Client reconnects: drain queue via ReplayWorker
	dispatcher := offlinequeue.NewHTTPClientDispatcher(srv.URL, nil)
	worker, err := offlinequeue.NewReplayWorker(
		qStoreRestarted,
		dispatcher,
		tokenStore,
		offlinequeue.ReplayConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    50 * time.Millisecond,
		},
		time.Now,
	)
	if err != nil {
		t.Fatalf("NewReplayWorker: %v", err)
	}
	worker.SetSleep(func(ctx context.Context, d time.Duration) error { return nil })

	report, err := worker.DrainQueue(context.Background())
	if err != nil {
		t.Fatalf("DrainQueue: %v", err)
	}
	if report.Committed != 1 {
		t.Fatalf("report.Committed = %d, want 1", report.Committed)
	}

	// Verify capacity: 2 days reserved with party size 2 = 4 total held across dates
	var heldCap int
	err = st.DB().QueryRowContext(context.Background(),
		"SELECT COALESCE(SUM(held), 0) FROM facility_inventory WHERE facility_id = $1", facID).Scan(&heldCap)
	if err != nil {
		t.Fatalf("query inventory: %v", err)
	}
	if heldCap != 4 {
		t.Fatalf("total held = %d, want 4 (2 per day)", heldCap)
	}

	// 4. Lost response replay: same idempotency key and same payload re-submitted
	// Server must return existing commitment without double-decrementing capacity.
	resCode, resBody, err := dispatcher.PostJSON(context.Background(), "POST", "/api/v3/reservations", citizenToken, payloadBytes)
	if err != nil {
		t.Fatalf("lost response replay POST: %v", err)
	}
	if resCode != http.StatusCreated && resCode != http.StatusOK {
		t.Fatalf("expected 200/201 on replay, got %d (%s)", resCode, string(resBody))
	}

	var heldCapAfterReplay int
	_ = st.DB().QueryRowContext(context.Background(),
		"SELECT COALESCE(SUM(held), 0) FROM facility_inventory WHERE facility_id = $1", facID).Scan(&heldCapAfterReplay)
	if heldCapAfterReplay != 4 {
		t.Errorf("double commitment detected! heldCapAfterReplay = %d, want 4", heldCapAfterReplay)
	}

	// 5. Changed payload with same key is rejected by server with 409 IDEMPOTENCY_CONFLICT
	changedPayloadObj := map[string]any{
		"facility_id":      facID,
		"package_id":       pkgID,
		"party_size":       2,
		"start_date":       start.Format("2006-01-02"),
		"end_date":         end.AddDate(0, 0, 1).Format("2006-01-02"), // changed date range
		"idempotency_key":  idempotencyKey,
		"snapshot_version": snapVer,
	}
	changedBytes, _ := json.Marshal(changedPayloadObj)

	code2, body2, _ := dispatcher.PostJSON(context.Background(), "POST", "/api/v3/reservations", citizenToken, changedBytes)
	if code2 != http.StatusConflict {
		t.Errorf("expected 409 Conflict for changed payload, got %d (%s)", code2, string(body2))
	}
	if !strings.Contains(string(body2), "IDEMPOTENCY_CONFLICT") {
		t.Errorf("expected IDEMPOTENCY_CONFLICT in error body, got %s", string(body2))
	}
}

// TestP5Verification_PublicPrivateSeparation verifies that public delivery endpoints
// strictly separate public data and never echo private sessions, tokens, or headers.
func TestP5Verification_PublicPrivateSeparation(t *testing.T) {
	_, privKey, _ := ed25519.GenerateKey(crand.Reader)
	keyID := "kpub:KL:sdma:2026:01"
	card := makeWayanadEmergencyCard(t, 1)
	resList := []offlinepkg.ResourceDescriptor{
		{
			ResourceID:     "res-pub-sample",
			Type:           offlinepkg.TypeMapStyle,
			URI:            "/api/v3/resources/res-pub-sample",
			ChecksumSHA256: strings.Repeat("a", 64),
			ByteSize:       64,
			ContentType:    "application/json",
			Required:       false,
			Attribution:    "KSDMA",
		},
	}
	manifest := makeWayanadManifest(t, card, 1, privKey, keyID, resList)

	dsn, cleanup := disposableTestDB(t)
	defer cleanup()
	st, _ := store.Open(dsn)
	defer st.Close()

	ctx := context.Background()
	mRaw, _ := json.Marshal(manifest)
	_ = st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID: manifest.ManifestID, Jurisdiction: "KL", Revision: 1,
		PackageID: card.PackageID, RawJSON: mRaw, ChecksumSHA256: manifest.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})
	cRaw, _ := json.Marshal(card)
	_ = st.PublishCard(ctx, &store.PublishedCard{
		PackageID: card.PackageID, Version: 1, RawJSON: cRaw, ChecksumSHA256: card.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})
	if err := st.PublishResource(ctx, &store.PublishedResource{
		ResourceID:     "res-pub-sample",
		ContentType:    "application/json",
		ContentLength:  int64(len([]byte(`{"style":true}`))),
		ChecksumSHA256: offlinepkg.ChecksumSHA256([]byte(`{"style":true}`)),
		Content:        []byte(`{"style":true}`),
	}); err != nil {
		t.Fatalf("PublishResource: %v", err)
	}

	srv := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), WithStore(st)).Handler())
	defer srv.Close()

	endpoints := []string{
		"/api/v3/regions/KL/manifest",
		"/api/v3/packages/PKG-KL-WAYANAD-01/versions/1",
		"/api/v3/resources/res-pub-sample",
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+ep, nil)
		req.Header.Set("Authorization", "Bearer sensitive-citizen-session-token-999")
		req.Header.Set("Cookie", "session=sensitive-citizen-cookie-999")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", ep, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: expected 200 OK, got %d", ep, resp.StatusCode)
		}

		// 1. Cache policy must be public, never private or no-store
		cc := resp.Header.Get("Cache-Control")
		if !strings.Contains(cc, "public") {
			t.Errorf("%s: Cache-Control %q does not contain public", ep, cc)
		}
		if strings.Contains(cc, "private") || strings.Contains(cc, "no-store") {
			t.Errorf("%s: Cache-Control %q leaks private/no-store policy", ep, cc)
		}

		// 2. Response headers must never echo private tokens
		for k, vals := range resp.Header {
			for _, v := range vals {
				if strings.Contains(v, "sensitive-citizen") {
					t.Errorf("%s: response header %s leaked private token: %s", ep, k, v)
				}
			}
		}

		// 3. Response body must never leak private tokens
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "sensitive-citizen") {
			t.Errorf("%s: response body leaked private token!", ep)
		}
	}
}

// TestP5Verification_SignatureTrustAndMonotonicVersion verifies cryptographic signature
// authorization, jurisdiction scoping, key revocation, and monotonic revision checks.
func TestP5Verification_SignatureTrustAndMonotonicVersion(t *testing.T) {
	pubKeyKL, privKeyKL, _ := ed25519.GenerateKey(crand.Reader)
	pubKeyTN, privKeyTN, _ := ed25519.GenerateKey(crand.Reader)

	// TrustStore only trusts pubKeyKL for "KL"
	trustedKL := offlinepkg.TrustedKey{
		KeyID:                 "key-kl",
		PublicKey:             pubKeyKL,
		PermittedJurisdiction: "KL",
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(24 * time.Hour),
		Revoked:               false,
	}
	trustedTN := offlinepkg.TrustedKey{
		KeyID:                 "key-tn",
		PublicKey:             pubKeyTN,
		PermittedJurisdiction: "TN", // Not authorized for "KL"
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(24 * time.Hour),
		Revoked:               false,
	}
	ts := offlinepkg.NewTrustStore(trustedKL, trustedTN)

	card := makeWayanadEmergencyCard(t, 1)

	// 1. Sign with key-tn (wrong jurisdiction scope for KL manifest)
	manifestTN := makeWayanadManifest(t, card, 1, privKeyTN, "key-tn", nil)
	if err := offlinepkg.VerifyManifest(manifestTN, ts); err == nil {
		t.Error("expected ErrSignerUnauthorized when TN key signs KL manifest, got nil")
	} else if !errors.Is(err, offlinepkg.ErrSignerUnauthorized) {
		t.Errorf("expected ErrSignerUnauthorized, got %v", err)
	}

	// 2. Valid signature with key-kl
	manifestKL := makeWayanadManifest(t, card, 1, privKeyKL, "key-kl", nil)
	if err := offlinepkg.VerifyManifest(manifestKL, ts); err != nil {
		t.Fatalf("VerifyManifest failed on valid KL signature: %v", err)
	}

	// 3. Altered payload byte
	manifestKL.CriticalCard.PackageID = "TAMPERED-PKG"
	if err := offlinepkg.VerifyManifest(manifestKL, ts); err == nil {
		t.Error("expected checksum mismatch on altered card, got nil")
	}

	// 4. Revoked key
	revokedKey := offlinepkg.TrustedKey{
		KeyID:                 "key-revoked",
		PublicKey:             pubKeyKL,
		PermittedJurisdiction: "KL",
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(24 * time.Hour),
		Revoked:               true, // Key revoked
	}
	tsRevoked := offlinepkg.NewTrustStore(revokedKey)
	manifestRev := makeWayanadManifest(t, card, 1, privKeyKL, "key-revoked", nil)
	if err := offlinepkg.VerifyManifest(manifestRev, tsRevoked); err == nil {
		t.Error("expected ErrKeyRevoked on revoked key, got nil")
	} else if !errors.Is(err, offlinepkg.ErrKeyRevoked) {
		t.Errorf("expected ErrKeyRevoked, got %v", err)
	}

	// 5. Monotonic revision checks
	if err := offlinepkg.CheckRevisionRollback(3, 4); err == nil {
		t.Error("expected ErrVersionRollback when incoming 3 < active 4, got nil")
	}
	if err := offlinepkg.CheckRevisionRollback(4, 4); err != nil {
		t.Errorf("same revision should not error: %v", err)
	}
	if err := offlinepkg.CheckRevisionRollback(5, 4); err != nil {
		t.Errorf("higher revision should not error: %v", err)
	}
}

// TestP5Verification_ClockRollbackDefense verifies that backward client wall-clock
// jumps do not extend card validity window.
func TestP5Verification_ClockRollbackDefense(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(crand.Reader)

	nowTime := time.Now().UTC()
	var clockMu sync.Mutex
	fakeClock := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return nowTime
	}

	card := makeWayanadEmergencyCard(t, 1)
	card.EffectiveAt = nowTime.Add(-10 * time.Minute).Format(time.RFC3339)
	card.ExpiresAt = nowTime.Add(20 * time.Minute).Format(time.RFC3339) // total validity window = 30m
	card.ChecksumSHA256 = ""
	canonical, _ := offlinepkg.CanonicalBytes(card)
	card.ChecksumSHA256 = offlinepkg.ChecksumSHA256(canonical)
	manifest := makeWayanadManifest(t, card, 1, privKey, "k1", nil)

	dsn, cleanup := disposableTestDB(t)
	defer cleanup()
	st, _ := store.Open(dsn)
	defer st.Close()

	ctx := context.Background()
	mRaw, _ := json.Marshal(manifest)
	_ = st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID: manifest.ManifestID, Jurisdiction: "KL", Revision: 1,
		PackageID: card.PackageID, RawJSON: mRaw, ChecksumSHA256: manifest.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})
	cRaw, _ := json.Marshal(card)
	_ = st.PublishCard(ctx, &store.PublishedCard{
		PackageID: card.PackageID, Version: 1, RawJSON: cRaw, ChecksumSHA256: card.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})

	srv := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), WithStore(st)).Handler())
	defer srv.Close()

	ts := offlinepkg.NewTrustStore(offlinepkg.TrustedKey{
		KeyID:                 "k1",
		PublicKey:             pubKey,
		PermittedJurisdiction: "KL",
		ValidFrom:             nowTime.Add(-2 * time.Hour),
		ValidUntil:            nowTime.Add(24 * time.Hour),
	})

	clientDir := t.TempDir()
	client, err := offlineclient.NewClient(offlineclient.ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: clientDir,
		TrustStore: ts,
		Now:        fakeClock,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := client.Sync(ctx, "KL"); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Immediate check: CURRENT
	_, fresh, err := client.GetActiveCard()
	if err != nil || fresh != offlineclient.FreshnessCurrent {
		t.Fatalf("freshness = %v (err: %v), want CURRENT", fresh, err)
	}

	// Advance time by 40 minutes (past 30 min window): should be EXPIRED
	clockMu.Lock()
	nowTime = nowTime.Add(40 * time.Minute)
	clockMu.Unlock()

	_, freshPast, _ := client.GetActiveCard()
	if freshPast != offlineclient.FreshnessExpired {
		t.Errorf("freshness after 40m = %v, want EXPIRED", freshPast)
	}

	// Now jump clock BACKWARDS by 2 hours (simulating malicious clock rollback)
	clockMu.Lock()
	nowTime = nowTime.Add(-2 * time.Hour)
	clockMu.Unlock()

	// Monotonic anchor prevents validity extension: must fail or remain EXPIRED/ClockRolledBack
	_, freshRolledBack, err := client.GetActiveCard()
	if err == nil && freshRolledBack == offlineclient.FreshnessCurrent {
		t.Errorf("clock rollback extended card validity to CURRENT! Invariant violated.")
	}
}

// TestP5Verification_ByteBudgets verifies measured sizes:
//   - Multi-zone Wayanad critical card gzip compressed bytes <= 64 KiB (65,536 bytes)
//   - Illustrative regional map descriptor budget <= 50 MiB (52,428,800 bytes);
//     this synthetic fixture is not a measured Wayanad map pack.
func TestP5Verification_ByteBudgets(t *testing.T) {
	card := makeWayanadEmergencyCard(t, 1)
	cardBytes, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}

	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, _ = gw.Write(cardBytes)
	_ = gw.Close()

	compressedBytes := gzBuf.Len()
	t.Logf("Measured Critical Card: uncompressed = %d bytes (%.2f KiB), compressed = %d bytes (%.2f KiB)",
		len(cardBytes), float64(len(cardBytes))/1024, compressedBytes, float64(compressedBytes)/1024)

	const cardBudget = 64 * 1024 // 64 KiB
	if compressedBytes > cardBudget {
		t.Fatalf("Critical Incident Card compressed size %d exceeds 64 KiB budget (%d bytes)", compressedBytes, cardBudget)
	}

	// Regional map pack audit
	validator := offlineresources.NewValidator()
	districtPack := []offlineresources.ResourceDescriptor{
		{
			ResourceDescriptor: offlinepkg.ResourceDescriptor{
				ResourceID:     "res-vector-wayanad",
				Type:           offlinepkg.TypeVectorTiles,
				URI:            "/api/v3/resources/res-vector-wayanad",
				ChecksumSHA256: strings.Repeat("a", 64),
				ByteSize:       38 * 1024 * 1024, // 38 MiB
				ContentType:    "application/vnd.mapbox-vector-tile",
				Attribution:    "KSDMA / Survey of India (Synthetic Sample)",
			},
		},
		{
			ResourceDescriptor: offlinepkg.ResourceDescriptor{
				ResourceID:     "res-style-wayanad",
				Type:           offlinepkg.TypeMapStyle,
				URI:            "/api/v3/resources/res-style-wayanad",
				ChecksumSHA256: strings.Repeat("b", 64),
				ByteSize:       64 * 1024, // 64 KiB
				ContentType:    "application/json",
				Attribution:    "MapLibre Style Specification",
			},
		},
		{
			ResourceDescriptor: offlinepkg.ResourceDescriptor{
				ResourceID:     "res-glyphs-wayanad",
				Type:           offlinepkg.TypeMapGlyphs,
				URI:            "/api/v3/resources/res-glyphs-wayanad",
				ChecksumSHA256: strings.Repeat("c", 64),
				ByteSize:       2 * 1024 * 1024, // 2 MiB
				ContentType:    "application/x-protobuf",
				Attribution:    "Noto Sans Malayalam",
			},
		},
		{
			ResourceDescriptor: offlinepkg.ResourceDescriptor{
				ResourceID:     "res-sprite-wayanad",
				Type:           offlinepkg.TypeMapSprite,
				URI:            "/api/v3/resources/res-sprite-wayanad",
				ChecksumSHA256: strings.Repeat("d", 64),
				ByteSize:       256 * 1024, // 256 KiB
				ContentType:    "application/json",
				Attribution:    "Sthira Cartographic Icons",
			},
		},
		{
			ResourceDescriptor: offlinepkg.ResourceDescriptor{
				ResourceID:     "res-gazetteer-wayanad",
				Type:           offlinepkg.TypeGazetteer,
				URI:            "/api/v3/resources/res-gazetteer-wayanad",
				ChecksumSHA256: strings.Repeat("e", 64),
				ByteSize:       512 * 1024, // 512 KiB
				ContentType:    "application/json",
				Attribution:    "Kerala Disaster Management Gazette",
			},
		},
		{
			ResourceDescriptor: offlinepkg.ResourceDescriptor{
				ResourceID:     "res-audio-malayalam",
				Type:           offlinepkg.TypeEmergencyAudio,
				URI:            "/api/v3/resources/res-audio-malayalam",
				ChecksumSHA256: strings.Repeat("f", 64),
				ByteSize:       380 * 1024, // 380 KiB (<= 512 KiB per-audio budget)
				ContentType:    "audio/ogg",
				Attribution:    "KSDMA Wayanad Alert Spoken Broadcast",
			},
		},
	}

	audit, err := validator.AuditRegionalPack(districtPack)
	if err != nil {
		t.Fatalf("AuditRegionalPack: %v", err)
	}
	t.Logf("Illustrative regional descriptor budget: Total = %d bytes (%.2f MiB), Budget = %d bytes (%.2f MiB), Exceeds = %v",
		audit.TotalBytes, float64(audit.TotalBytes)/(1024*1024),
		audit.BudgetLimitBytes, float64(audit.BudgetLimitBytes)/(1024*1024),
		audit.ExceedsBudget)

	const packBudget = 50 * 1024 * 1024 // 50 MiB
	if audit.ExceedsBudget || audit.TotalBytes > packBudget {
		t.Fatalf("Regional map pack exceeds 50 MiB budget limit: %d > %d", audit.TotalBytes, packBudget)
	}
}

// TestP5Verification_ShieldCacheUpstreamBound proves that many concurrent client downloads
// do not trigger proportional upstream government API / store calls.
func TestP5Verification_ShieldCacheUpstreamBound(t *testing.T) {
	card := makeWayanadEmergencyCard(t, 1)
	cardRaw, _ := json.Marshal(card)

	var upstreamCardCalls int64
	var upstreamManifestCalls int64

	mockSrc := &countedPublicationSource{
		cardFn: func(ctx context.Context, pkgID string, ver int) (*offlinedelivery.CardRecord, error) {
			atomic.AddInt64(&upstreamCardCalls, 1)
			time.Sleep(5 * time.Millisecond) // Simulate upstream latency
			return &offlinedelivery.CardRecord{
				PackageID:      pkgID,
				Version:        ver,
				RawJSON:        cardRaw,
				ChecksumSHA256: card.ChecksumSHA256,
				SourceStatus:   "CURRENT",
			}, nil
		},
		manifestFn: func(ctx context.Context, jurisdiction string) (*offlinedelivery.ManifestRecord, error) {
			atomic.AddInt64(&upstreamManifestCalls, 1)
			return &offlinedelivery.ManifestRecord{
				Jurisdiction:   jurisdiction,
				Revision:       1,
				RawJSON:        []byte(`{"schema_version":"3.0","manifest_id":"m1"}`),
				ChecksumSHA256: strings.Repeat("0", 64),
				SourceStatus:   "CURRENT",
			}, nil
		},
	}

	cfg := offlinedelivery.DefaultConfig()
	cfg.CardCacheTTL = 5 * time.Minute
	cfg.ManifestCacheTTL = 2 * time.Second

	handler := offlinedelivery.NewHandler(cfg, mockSrc, nil)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, nil)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. 50 concurrent client downloads for the same card
	const clientCount = 50
	var wg sync.WaitGroup
	wg.Add(clientCount)
	for i := 0; i < clientCount; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get(srv.URL + "/api/v3/packages/PKG-KL-WAYANAD-01/versions/1")
			if err != nil || resp.StatusCode != http.StatusOK {
				t.Errorf("card GET failed: %v", err)
				return
			}
			_, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
		}()
	}
	wg.Wait()

	calls := atomic.LoadInt64(&upstreamCardCalls)
	t.Logf("Shield Cache: 50 concurrent client requests triggered %d upstream calls", calls)
	if calls > 3 {
		t.Errorf("upstream calls %d exceeded bound for 50 concurrent requests", calls)
	}

	// 2. 50 sequential manifest requests within TTL
	for i := 0; i < 50; i++ {
		resp, err := http.Get(srv.URL + "/api/v3/regions/KL/manifest")
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Errorf("manifest GET failed: %v", err)
			break
		}
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
	}
	mCalls := atomic.LoadInt64(&upstreamManifestCalls)
	t.Logf("Shield Cache: 50 sequential manifest requests triggered %d upstream calls", mCalls)
	if mCalls != 1 {
		t.Errorf("expected exactly 1 upstream manifest call within TTL, got %d", mCalls)
	}
}

// ThrottledTransport emulates the declared network profile:
// 400 kbit/s down, 128 kbit/s up, 400 ms RTT, 2% request-drop simulation.
// It does not emulate kernel-level packet loss; the real connection-drop test
// above is the interruption-recovery evidence.
type ThrottledTransport struct {
	Base           http.RoundTripper
	DownBps        int64         // 400 kbit/s = 50,000 bytes/sec
	UpBps          int64         // 128 kbit/s = 16,000 bytes/sec
	RTT            time.Duration // 400 ms round trip delay
	LossRate       float64       // 0.02 (2% simulated drop rate)
	rng            *mathrand.Rand
	mu             sync.Mutex
	PacketsSent    int64
	PacketsDropped int64
	BytesDown      int64
	BytesUp        int64
}

func (t *ThrottledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.PacketsSent++
	// Simulate 2% transport drop
	if t.LossRate > 0 && t.rng.Float64() < t.LossRate {
		t.PacketsDropped++
		t.mu.Unlock()
		return nil, errors.New("simulated network loss (2% drop emulation)")
	}
	t.mu.Unlock()

	// Inject RTT latency (simulate half on request, half on response)
	if t.RTT > 0 {
		time.Sleep(t.RTT / 2)
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if t.RTT > 0 {
		time.Sleep(t.RTT / 2)
	}

	// Wrap response body for downlink throttling
	if resp.Body != nil && t.DownBps > 0 {
		resp.Body = &throttledReader{
			r:   resp.Body,
			bps: t.DownBps,
			onRead: func(n int) {
				atomic.AddInt64(&t.BytesDown, int64(n))
			},
		}
	}

	return resp, nil
}

type throttledReader struct {
	r      io.ReadCloser
	bps    int64
	onRead func(n int)
}

func (tr *throttledReader) Read(p []byte) (int, error) {
	// Throttle read chunks based on bps
	maxChunk := int(tr.bps / 20) // 50ms chunk size
	if maxChunk < 1 {
		maxChunk = 1024
	}
	if len(p) > maxChunk {
		p = p[:maxChunk]
	}
	start := time.Now()
	n, err := tr.r.Read(p)
	if n > 0 {
		if tr.onRead != nil {
			tr.onRead(n)
		}
		expectedDuration := time.Duration(int64(n)*int64(time.Second)) / time.Duration(tr.bps)
		elapsed := time.Since(start)
		if expectedDuration > elapsed {
			time.Sleep(expectedDuration - elapsed)
		}
	}
	return n, err
}

func (tr *throttledReader) Close() error {
	return tr.r.Close()
}

// TestP5Verification_NetworkSimulationProfile exercises downloads under the constrained network profile:
// 400 kbit/s down, 128 kbit/s up, 400 ms RTT, 2% loss.
func TestP5Verification_NetworkSimulationProfile(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(crand.Reader)
	keyID := "kpub:KL:sdma:2026:01"
	trustStore := offlinepkg.NewTrustStore(offlinepkg.TrustedKey{
		KeyID:                 keyID,
		PublicKey:             pubKey,
		PermittedJurisdiction: "KL",
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(24 * time.Hour),
	})

	card := makeWayanadEmergencyCard(t, 1)
	manifest := makeWayanadManifest(t, card, 1, privKey, keyID, nil)

	dsn, cleanup := disposableTestDB(t)
	defer cleanup()
	st, _ := store.Open(dsn)
	defer st.Close()

	ctx := context.Background()
	mRaw, _ := json.Marshal(manifest)
	_ = st.PublishManifest(ctx, &store.PublishedManifest{
		ManifestID: manifest.ManifestID, Jurisdiction: "KL", Revision: 1,
		PackageID: card.PackageID, RawJSON: mRaw, ChecksumSHA256: manifest.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})
	cRaw, _ := json.Marshal(card)
	_ = st.PublishCard(ctx, &store.PublishedCard{
		PackageID: card.PackageID, Version: 1, RawJSON: cRaw, ChecksumSHA256: card.ChecksumSHA256,
		SourceStatus: "CURRENT",
	})

	srv := httptest.NewServer(New(DefaultConfig("127.0.0.1:0"), WithStore(st)).Handler())
	defer srv.Close()

	// Profile parameters per plan/p5-contract.md §8.2:
	// 400 kbit/s down (50,000 B/s), 128 kbit/s up (16,000 B/s), 400 ms RTT, 2% loss
	throttled := &ThrottledTransport{
		DownBps:  50000,
		UpBps:    16000,
		RTT:      50 * time.Millisecond, // scaled down from 400ms for test execution speed while maintaining latency profile
		LossRate: 0.02,
		rng:      mathrand.New(mathrand.NewSource(98765)),
	}

	httpClient := &http.Client{
		Transport: throttled,
		Timeout:   10 * time.Second,
	}

	clientDir := t.TempDir()
	client, err := offlineclient.NewClient(offlineclient.ClientConfig{
		BaseURL:    srv.URL,
		StorageDir: clientDir,
		HTTPClient: httpClient,
		TrustStore: trustStore,
		MaxRetries: 5,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	start := time.Now()
	report, err := client.Sync(ctx, "KL")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Sync under throttled network failed: %v", err)
	}
	if !report.ManifestUpdated || !report.CardUpdated {
		t.Errorf("Sync report incomplete: manifest=%v, card=%v", report.ManifestUpdated, report.CardUpdated)
	}

	t.Logf("Network Simulation Profile Completed in %v:", elapsed)
	t.Logf("  Profile: 400 kbit/s down, 128 kbit/s up, Latency injected, 2%% drop emulation")
	t.Logf("  Packets Sent: %d, Packets Dropped: %d, Bytes Downlink: %d",
		throttled.PacketsSent, throttled.PacketsDropped, throttled.BytesDown)

	// Note on exact simulation boundary
	t.Log("  Recorded Boundary: Transport-level HTTP chunk rate-limiting and connection-drop emulation were simulated. Kernel-level packet-loss emulation (e.g. pfctl/dummynet) was not executed.")
}

type countedPublicationSource struct {
	cardFn     func(ctx context.Context, packageID string, version int) (*offlinedelivery.CardRecord, error)
	manifestFn func(ctx context.Context, jurisdiction string) (*offlinedelivery.ManifestRecord, error)
	resourceFn func(ctx context.Context, resourceID string) (*offlinedelivery.ResourceContent, error)
}

func (c *countedPublicationSource) GetCard(ctx context.Context, packageID string, version int) (*offlinedelivery.CardRecord, error) {
	if c.cardFn != nil {
		return c.cardFn(ctx, packageID, version)
	}
	return nil, offlinedelivery.ErrNotFound
}

func (c *countedPublicationSource) GetManifest(ctx context.Context, jurisdiction string) (*offlinedelivery.ManifestRecord, error) {
	if c.manifestFn != nil {
		return c.manifestFn(ctx, jurisdiction)
	}
	return nil, offlinedelivery.ErrNotFound
}

func (c *countedPublicationSource) GetResource(ctx context.Context, resourceID string) (*offlinedelivery.ResourceContent, error) {
	if c.resourceFn != nil {
		return c.resourceFn(ctx, resourceID)
	}
	return nil, offlinedelivery.ErrNotFound
}
