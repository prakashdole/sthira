package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// PublishedManifest represents an active or historical signed manifest in the store.
type PublishedManifest struct {
	ManifestID     string
	Jurisdiction   string
	Revision       int
	PackageID      string
	RawJSON        []byte
	ChecksumSHA256 string
	SourceStatus   string
	Quarantined    bool
	CreatedAt      time.Time
}

// PublishedCard represents an immutable public incident card in the store.
type PublishedCard struct {
	PackageID      string
	Version        int
	RawJSON        []byte
	ChecksumSHA256 string
	SourceStatus   string
	Quarantined    bool
	CreatedAt      time.Time
}

// PublishedResource represents a content-addressed auxiliary asset in the store.
type PublishedResource struct {
	ResourceID     string
	ContentType    string
	ContentLength  int64
	ChecksumSHA256 string
	Content        []byte
	CreatedAt      time.Time
}

// PublishManifest stores or updates a regional manifest.
func (s *Store) PublishManifest(ctx context.Context, m *PublishedManifest) error {
	const query = `
INSERT INTO published_manifests (manifest_id, jurisdiction, revision, package_id, raw_json, checksum_sha256, source_status, quarantined)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (jurisdiction, revision) DO UPDATE
SET raw_json = EXCLUDED.raw_json,
    checksum_sha256 = EXCLUDED.checksum_sha256,
    source_status = EXCLUDED.source_status,
    quarantined = EXCLUDED.quarantined`
	_, err := s.db.ExecContext(ctx, query,
		m.ManifestID,
		m.Jurisdiction,
		m.Revision,
		m.PackageID,
		m.RawJSON,
		m.ChecksumSHA256,
		m.SourceStatus,
		m.Quarantined,
	)
	return err
}

// GetPublishedManifest retrieves the highest-revision published manifest for a jurisdiction.
func (s *Store) GetPublishedManifest(ctx context.Context, jurisdiction string) (*PublishedManifest, error) {
	const query = `
SELECT manifest_id, jurisdiction, revision, package_id, raw_json, checksum_sha256, source_status, quarantined, created_at
FROM published_manifests
WHERE jurisdiction = $1
ORDER BY revision DESC
LIMIT 1`
	row := s.db.QueryRowContext(ctx, query, jurisdiction)
	var m PublishedManifest
	err := row.Scan(
		&m.ManifestID,
		&m.Jurisdiction,
		&m.Revision,
		&m.PackageID,
		&m.RawJSON,
		&m.ChecksumSHA256,
		&m.SourceStatus,
		&m.Quarantined,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// PublishCard stores or updates an immutable public incident card.
func (s *Store) PublishCard(ctx context.Context, c *PublishedCard) error {
	const query = `
INSERT INTO published_cards (package_id, version, raw_json, checksum_sha256, source_status, quarantined)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (package_id, version) DO UPDATE
SET raw_json = EXCLUDED.raw_json,
    checksum_sha256 = EXCLUDED.checksum_sha256,
    source_status = EXCLUDED.source_status,
    quarantined = EXCLUDED.quarantined`
	_, err := s.db.ExecContext(ctx, query,
		c.PackageID,
		c.Version,
		c.RawJSON,
		c.ChecksumSHA256,
		c.SourceStatus,
		c.Quarantined,
	)
	return err
}

// GetPublishedCard retrieves the published incident card for a package and version.
func (s *Store) GetPublishedCard(ctx context.Context, packageID string, version int) (*PublishedCard, error) {
	const query = `
SELECT package_id, version, raw_json, checksum_sha256, source_status, quarantined, created_at
FROM published_cards
WHERE package_id = $1 AND version = $2`
	row := s.db.QueryRowContext(ctx, query, packageID, version)
	var c PublishedCard
	err := row.Scan(
		&c.PackageID,
		&c.Version,
		&c.RawJSON,
		&c.ChecksumSHA256,
		&c.SourceStatus,
		&c.Quarantined,
		&c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// PublishResource stores or updates a content-addressed auxiliary asset.
func (s *Store) PublishResource(ctx context.Context, r *PublishedResource) error {
	const query = `
INSERT INTO published_resources (resource_id, content_type, content_length, checksum_sha256, content)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (resource_id) DO UPDATE
SET content_type = EXCLUDED.content_type,
    content_length = EXCLUDED.content_length,
    checksum_sha256 = EXCLUDED.checksum_sha256,
    content = EXCLUDED.content`
	_, err := s.db.ExecContext(ctx, query,
		r.ResourceID,
		r.ContentType,
		r.ContentLength,
		r.ChecksumSHA256,
		r.Content,
	)
	return err
}

// GetPublishedResource retrieves a content-addressed auxiliary asset.
func (s *Store) GetPublishedResource(ctx context.Context, resourceID string) (*PublishedResource, error) {
	const query = `
SELECT resource_id, content_type, content_length, checksum_sha256, content, created_at
FROM published_resources
WHERE resource_id = $1`
	row := s.db.QueryRowContext(ctx, query, resourceID)
	var r PublishedResource
	err := row.Scan(
		&r.ResourceID,
		&r.ContentType,
		&r.ContentLength,
		&r.ChecksumSHA256,
		&r.Content,
		&r.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}
