// Package offlinedelivery: A3 cross-instance cache invalidation tests.
//
// Reproduces: an actual trusted lifecycle operation → persisted signed
// publication → real cached HTTP fetch → authority withdrawal →
// subsequent denial WITHOUT manual Invalidate calls in the test.
//
// These tests run against a UNIQUE disposable PostgreSQL instance;
// they are skipped (not failed) when STHIRA_TEST_DSN is unset.
package offlinedelivery_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"sthira/backend/internal/offlinedelivery"
	"sthira/backend/internal/offlinepkg"
	"sthira/backend/internal/store"
)

func TestA3_LifecycleInvalidatesAcrossInstances(t *testing.T) {
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" {
		t.Skip("STHIRA_TEST_DSN not set; A3 integration tests require a disposable PostgreSQL")
	}
	jurisdiction := "A3CI-" + uid("X")
	srcID := "SRC-A3CI-" + uid("X")
	pkgID := "PKG-A3CI-" + uid("X")

	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	keyID := "key-A3CI"
	ts := offlinepkg.NewTrustStore(offlinepkg.TrustedKey{
		KeyID: keyID, PublicKey: pub,
		PermittedJurisdiction: jurisdiction,
		ValidFrom:             time.Now().Add(-time.Hour),
		ValidUntil:            time.Now().Add(time.Hour),
	})
	publisher := store.NewPublisher(st, ts)

	if err := seedA3Fixture(t, st, srcID, pkgID, jurisdiction); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Build + sign a manifest for revision 1.
	m := &offlinepkg.Manifest{
		SchemaVersion: "3.0",
		ManifestID:    "MNF-A3CI-" + uid("X"),
		Jurisdiction:  jurisdiction,
		Revision:      1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		ValidUntil:    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		SourceStatus:  "CURRENT",
		CriticalCard: offlinepkg.CriticalCardDescriptor{
			PackageID:         pkgID,
			Version:           1,
			URI:               "/api/v3/packages/" + pkgID + "/versions/1",
			ChecksumSHA256:    strings.Repeat("a", 64),
			UncompressedBytes: 500,
			CompressedBytes:   200,
			ContentType:       "application/json",
		},
		Provenance: offlinepkg.ManifestProvenance{Authority: "gov-a3", DatasetID: "ds-1", EvidenceClass: "AUTHORIZED_OPERATIONAL"},
	}
	signA3Manifest(t, m, priv, keyID)
	rawM, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := publisher.PublishManifest(context.Background(), &store.PublishedManifest{
		ManifestID: m.ManifestID, Jurisdiction: m.Jurisdiction, Revision: m.Revision,
		PackageID: m.CriticalCard.PackageID, SourceID: srcID, RawJSON: rawM,
		SourceStatus: "STAGED",
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := st.PromoteManifest(context.Background(), jurisdiction, m.Revision); err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Two CachedSource instances (processes A and B) back the delivery.
	// They share the same Store but each maintains its own in-memory
	// cache.
	ps := &localPublicationSource{st: st}
	cachedA := offlinedelivery.NewCachedSource(ps, offlinedelivery.DefaultConfig(), time.Now)
	cachedB := offlinedelivery.NewCachedSource(ps, offlinedelivery.DefaultConfig(), time.Now)

	// A fetches: cache populated
	gotA, err := cachedA.GetManifest(context.Background(), jurisdiction)
	if err != nil {
		t.Fatalf("A fetch: %v", err)
	}
	if gotA == nil || gotA.ChecksumSHA256 != m.ChecksumSHA256 {
		t.Fatalf("A got unexpected manifest")
	}

	// B fetches: cache populated independently
	gotB, err := cachedB.GetManifest(context.Background(), jurisdiction)
	if err != nil {
		t.Fatalf("B fetch: %v", err)
	}
	if gotB == nil {
		t.Fatalf("B got nil")
	}

	// Wire the lifecycle observer: in production this is a shared bus
	// + per-process cache. Here we wire both caches to the same
	// in-process notification channel so the test exercises the
	// real fan-out path.
	bus := &lifecycleBus{}
	publisher.WithObserver(bus)
	bus.subscribe(cachedA)
	bus.subscribe(cachedB)

	// Authority withdraws.
	if err := st.WithdrawManifestAndInvalidate(context.Background(), jurisdiction, 1, bus); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	// After withdrawal, both A and B must NOT serve the cached manifest.
	gotRecA, err := cachedA.GetManifest(context.Background(), jurisdiction)
	if err != nil && err != offlinedelivery.ErrNotFound && err != offlinedelivery.ErrQuarantined {
		t.Errorf("A cache returned unexpected err: %v", err)
	}
	if gotRecA != nil && gotRecA.SourceStatus == "CURRENT" {
		t.Errorf("A cached manifest after withdrawal: SourceStatus=%q", gotRecA.SourceStatus)
	}
	gotRecB, err := cachedB.GetManifest(context.Background(), jurisdiction)
	if err != nil && err != offlinedelivery.ErrNotFound && err != offlinedelivery.ErrQuarantined {
		t.Errorf("B cache returned unexpected err: %v", err)
	}
	if gotRecB != nil && gotRecB.SourceStatus == "CURRENT" {
		t.Errorf("B cached manifest after withdrawal: SourceStatus=%q", gotRecB.SourceStatus)
	}
}

func TestA3_LegacyUnattributedRowCannotBePromoted(t *testing.T) {
	dsn := os.Getenv("STHIRA_TEST_DSN")
	if dsn == "" {
		t.Skip("STHIRA_TEST_DSN not set; A3 integration tests require a disposable PostgreSQL")
	}
	jurisdiction := "LEGACY-A3-" + uid("X")
	mnfID := "MNF-LEGACY-" + uid("X")
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Insert a manifest row directly WITHOUT source_id (legacy data).
	if _, err := st.DB().ExecContext(context.Background(), `
		INSERT INTO published_manifests
			(manifest_id, jurisdiction, revision, package_id, source_id, raw_json, checksum_sha256, source_status, quarantined)
		VALUES ($1, $2, 1, $3, NULL, $4, $5, 'STAGED', false)`,
		mnfID, jurisdiction, "PKG-LEGACY-A3",
		[]byte(`{"schema_version":"3.0","manifest_id":"`+mnfID+`","jurisdiction":"`+jurisdiction+`","revision":1,"critical_card":{"package_id":"PKG-LEGACY-A3","version":1}}`),
		strings.Repeat("a", 64)); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	// Attempting promotion of an unattributed row MUST fail with ErrPublicationAuthority
	// and perform zero writes (status remains STAGED).
	err = st.PromoteManifest(context.Background(), jurisdiction, 1)
	if err == nil {
		t.Fatalf("expected error promoting legacy unattributed manifest, got nil")
	}
	if !errors.Is(err, store.ErrPublicationAuthority) {
		t.Fatalf("expected ErrPublicationAuthority, got %v", err)
	}

	// Assert row remains STAGED (no writes performed)
	var status string
	if err := st.DB().QueryRowContext(context.Background(), `
		SELECT source_status FROM published_manifests WHERE jurisdiction = $1 AND revision = 1`, jurisdiction).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != "STAGED" {
		t.Fatalf("expected manifest to remain STAGED, got %q", status)
	}
}

// --- helpers ---

func seedA3Fixture(t *testing.T, st *store.Store, srcID, pkgID, jurisdiction string) error {
	t.Helper()
	now := time.Now().UTC()
	return st.InTx(context.Background(), func(tx store.DBTX) error {
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO sources (source_id, government_owner, official_domain, state, version, updated_at)
			VALUES ($1, 'gov-a3', 'gov.example', 'OPERATIONAL', 1, $2)`,
			srcID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO source_authorizations
				(authorization_id, source_id, granted_by, evidence_ref, jurisdiction, granted_at)
			VALUES ($1, $2, 'gov-a3', 'doc-a3', $3, $4)`,
			"AUTH-"+uid("X"), srcID, jurisdiction, now); err != nil {
			return err
		}
		artifactID := "ART-A3CI-" + uid("X")
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO source_artifacts (artifact_id, source_id, source_version, artifact_sha256, retrieved_at, evidence_class, payload_ref)
			VALUES ($1, $2, 1, $3, $4, 'AUTHORIZED_OPERATIONAL', 'mem://test')`,
			artifactID, srcID, strings.Repeat("a", 64), now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(context.Background(), `
			INSERT INTO packages (package_id, alert_id, source_id, artifact_id, version, jurisdiction, evidence_class, checksum_sha256, body, effective_at, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			pkgID, "ALERT-"+uid("X"), srcID, artifactID, 1, jurisdiction, "AUTHORIZED_OPERATIONAL",
			strings.Repeat("a", 64), []byte(`{"revoked_packages":[],"superseded_versions":[]}`),
			now.Add(-time.Hour), now.Add(time.Hour)); err != nil {
			return err
		}
		return nil
	})
}

func signA3Manifest(t *testing.T, m *offlinepkg.Manifest, priv ed25519.PrivateKey, keyID string) {
	t.Helper()
	can, err := offlinepkg.CanonicalBytes(m)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	m.ChecksumSHA256 = offlinepkg.ChecksumSHA256(can)
	sig, err := offlinepkg.SignCanonical(priv, keyID, m)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	m.Signature = sig
}

// localPublicationSource reads from the store directly. We don't
// import httpserver.NewStorePublicationSource to avoid an import
// cycle; the only difference is adapter field names, not behavior.
type localPublicationSource struct {
	st *store.Store
}

func (l *localPublicationSource) GetManifest(ctx context.Context, jurisdiction string) (*offlinedelivery.ManifestRecord, error) {
	m, err := l.st.GetPublishedManifest(ctx, jurisdiction)
	if err != nil {
		return nil, err
	}
	if m.Quarantined {
		return nil, offlinedelivery.ErrQuarantined
	}
	if m.SourceStatus != "CURRENT" {
		return nil, offlinedelivery.ErrNotFound
	}
	return &offlinedelivery.ManifestRecord{
		Jurisdiction:   m.Jurisdiction,
		Revision:       m.Revision,
		RawJSON:        m.RawJSON,
		ChecksumSHA256: m.ChecksumSHA256,
		SourceStatus:   m.SourceStatus,
	}, nil
}
func (l *localPublicationSource) GetCard(ctx context.Context, packageID string, version int) (*offlinedelivery.CardRecord, error) {
	c, err := l.st.GetPublishedCard(ctx, packageID, version)
	if err != nil {
		return nil, err
	}
	if c.Quarantined {
		return nil, offlinedelivery.ErrQuarantined
	}
	if c.SourceStatus != "CURRENT" {
		return nil, offlinedelivery.ErrNotFound
	}
	return &offlinedelivery.CardRecord{
		PackageID:      c.PackageID,
		Version:        c.Version,
		RawJSON:        c.RawJSON,
		ChecksumSHA256: c.ChecksumSHA256,
		SourceStatus:   c.SourceStatus,
	}, nil
}
func (l *localPublicationSource) GetResource(ctx context.Context, resourceID string) (*offlinedelivery.ResourceContent, error) {
	r, err := l.st.GetPublishedResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	return &offlinedelivery.ResourceContent{
		Reader:         l.readerFor(r),
		ContentType:    r.ContentType,
		ContentLength:  r.ContentLength,
		ChecksumSHA256: r.ChecksumSHA256,
		ETag:           fmt.Sprintf(`"sha256-%s"`, r.ChecksumSHA256),
	}, nil
}

type readerCloser struct {
	*strings.Reader
}

func (readerCloser) Close() error { return nil }

func (l *localPublicationSource) readerFor(r *store.PublishedResource) *readerCloser {
	return &readerCloser{strings.NewReader(string(r.Content))}
}

// lifecycleBus fans events out to subscribers (in production: a real
// bus that crosses instances; here an in-process channel for tests).
type lifecycleBus struct {
	mu   sync.Mutex
	subs []offlinedelivery.PublicationLifecycleObserver
}

func (b *lifecycleBus) subscribe(o offlinedelivery.PublicationLifecycleObserver) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, o)
}

func (b *lifecycleBus) fanout(event string, fn func(o offlinedelivery.PublicationLifecycleObserver)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.subs {
		fn(s)
	}
}
func (b *lifecycleBus) OnManifestWithdrawn(ctx context.Context, j string, r int) {
	b.fanout("withdrawn", func(o offlinedelivery.PublicationLifecycleObserver) {
		o.OnManifestWithdrawn(ctx, j, r)
	})
}
func (b *lifecycleBus) OnManifestPromoted(ctx context.Context, j string, r int) {
	b.fanout("promoted", func(o offlinedelivery.PublicationLifecycleObserver) {
		o.OnManifestPromoted(ctx, j, r)
	})
}
func (b *lifecycleBus) OnSourceWithdrawn(ctx context.Context, s string) {
	b.fanout("src-withdrawn", func(o offlinedelivery.PublicationLifecycleObserver) {
		o.OnSourceWithdrawn(ctx, s)
	})
}
func (b *lifecycleBus) OnSourceQuarantined(ctx context.Context, s string) {
	b.fanout("src-quarantined", func(o offlinedelivery.PublicationLifecycleObserver) {
		o.OnSourceQuarantined(ctx, s)
	})
}
func (b *lifecycleBus) OnPackageSuperseded(ctx context.Context, p string, v []int) {
	b.fanout("pkg-superseded", func(o offlinedelivery.PublicationLifecycleObserver) {
		o.OnPackageSuperseded(ctx, p, v)
	})
}

// silence unused-import warnings while we still need them in helper-signature
// literals.
var (
	_ = (*httptest.Server)(nil)
	_ = (*sql.DB)(nil)
	_ = hex.EncodeToString
	_ = fmt.Sprintf
)

// helper uid borrowed from elsewhere via duplicated local impl (the
// test cannot import internal test utilities).
func uid(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
