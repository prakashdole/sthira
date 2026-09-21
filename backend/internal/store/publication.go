package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"sthira/backend/internal/offlinepkg"
)

// PublishedManifest represents an active or historical signed manifest in the store.
type PublishedManifest struct {
	ManifestID     string
	Jurisdiction   string
	Revision       int
	PackageID      string
	SourceID       string // trusted publish path: binds record to its source
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
	Jurisdiction   string // populated by trusted publish; lower-level callers may leave blank
	SourceID       string // trusted publish path: binds record to its source
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

// PublishManifest stores a regional manifest immutably. Boundary checks validate
// schema, checksum, and payload length before persistence. Duplicate publish with
// identical content and checksum is idempotent; any content mutation under an
// existing (jurisdiction, revision) returns ErrConflict.
func (s *Store) PublishManifest(ctx context.Context, m *PublishedManifest) error {
	if m == nil {
		return errors.New("store: manifest is nil")
	}
	if m.ManifestID == "" {
		return errors.New("store: manifest_id required")
	}
	if m.Jurisdiction == "" {
		return errors.New("store: jurisdiction required")
	}
	if m.Revision < 1 {
		return errors.New("store: revision must be >= 1")
	}
	if m.PackageID == "" {
		return errors.New("store: package_id required")
	}
	if len(m.RawJSON) == 0 {
		return errors.New("store: raw_json required")
	}
	if len(m.RawJSON) > 262144 {
		return errors.New("store: raw_json exceeds 256 KiB ceiling")
	}
	if len(m.ChecksumSHA256) != 64 {
		return errors.New("store: invalid checksum length")
	}

	var parsed offlinepkg.Manifest
	if err := json.Unmarshal(m.RawJSON, &parsed); err != nil {
		return fmt.Errorf("store: parse manifest json: %w", err)
	}
	if err := offlinepkg.ValidateManifestStructure(&parsed); err != nil {
		return fmt.Errorf("store: validate manifest structure: %w", err)
	}
	if parsed.ChecksumSHA256 != m.ChecksumSHA256 {
		return fmt.Errorf("store: manifest checksum mismatch: payload has %q, record has %q", parsed.ChecksumSHA256, m.ChecksumSHA256)
	}

	// Check existing row for immutability
	const checkQuery = `
SELECT checksum_sha256, raw_json, quarantined, source_status
FROM published_manifests
WHERE jurisdiction = $1 AND revision = $2`

	var (
		existingChecksum string
		existingRaw      []byte
		existingQuar     bool
		existingStatus   string
	)
	row := s.db.QueryRowContext(ctx, checkQuery, m.Jurisdiction, m.Revision)
	err := row.Scan(&existingChecksum, &existingRaw, &existingQuar, &existingStatus)
	if err == nil {
		// Row exists: check immutability
		if existingChecksum != m.ChecksumSHA256 || !bytes.Equal(existingRaw, m.RawJSON) {
			return fmt.Errorf("%w: manifest %s revision %d already exists with different content/checksum", ErrConflict, m.Jurisdiction, m.Revision)
		}
		// Idempotent retry: update metadata if changed
		if existingQuar != m.Quarantined || existingStatus != m.SourceStatus {
			const updateMeta = `
UPDATE published_manifests
SET quarantined = $3, source_status = $4
WHERE jurisdiction = $1 AND revision = $2`
			_, uerr := s.db.ExecContext(ctx, updateMeta, m.Jurisdiction, m.Revision, m.Quarantined, m.SourceStatus)
			return uerr
		}
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// Insert new row
	const insertQuery = `
INSERT INTO published_manifests (manifest_id, jurisdiction, revision, package_id, raw_json, checksum_sha256, source_status, quarantined)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err = s.db.ExecContext(ctx, insertQuery,
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

// QuarantineManifest marks a manifest revision as quarantined or unquarantined.
func (s *Store) QuarantineManifest(ctx context.Context, jurisdiction string, revision int, quarantined bool) error {
	const query = `
UPDATE published_manifests
SET quarantined = $3
WHERE jurisdiction = $1 AND revision = $2`
	res, err := s.db.ExecContext(ctx, query, jurisdiction, revision, quarantined)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetManifestStatus updates the source_status of a published manifest (e.g. 'CURRENT', 'WITHDRAWN', 'SUPERSEDED').
func (s *Store) SetManifestStatus(ctx context.Context, jurisdiction string, revision int, status string) error {
	const query = `
UPDATE published_manifests
SET source_status = $3
WHERE jurisdiction = $1 AND revision = $2`
	res, err := s.db.ExecContext(ctx, query, jurisdiction, revision, status)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// PublishCard stores an immutable public incident card. Boundary checks validate
// schema, checksum, and payload length before persistence. Duplicate publish with
// identical content and checksum is idempotent; any content mutation under an
// existing (package_id, version) returns ErrConflict.
func (s *Store) PublishCard(ctx context.Context, c *PublishedCard) error {
	if c == nil {
		return errors.New("store: card is nil")
	}
	if c.PackageID == "" {
		return errors.New("store: package_id required")
	}
	if c.Version < 1 {
		return errors.New("store: version must be >= 1")
	}
	if len(c.RawJSON) == 0 {
		return errors.New("store: raw_json required")
	}
	if len(c.RawJSON) > 65536 {
		return errors.New("store: raw_json exceeds 64 KiB ceiling")
	}
	if len(c.ChecksumSHA256) != 64 {
		return errors.New("store: invalid checksum length")
	}

	var parsed offlinepkg.PublicIncidentCard
	if err := json.Unmarshal(c.RawJSON, &parsed); err != nil {
		return fmt.Errorf("store: parse card json: %w", err)
	}
	if err := offlinepkg.ValidateCardStructure(&parsed); err != nil {
		return fmt.Errorf("store: validate card structure: %w", err)
	}
	if parsed.ChecksumSHA256 != c.ChecksumSHA256 {
		return fmt.Errorf("store: card checksum mismatch: payload has %q, record has %q", parsed.ChecksumSHA256, c.ChecksumSHA256)
	}

	// Check existing row for immutability
	const checkQuery = `
SELECT checksum_sha256, raw_json, quarantined, source_status
FROM published_cards
WHERE package_id = $1 AND version = $2`

	var (
		existingChecksum string
		existingRaw      []byte
		existingQuar     bool
		existingStatus   string
	)
	row := s.db.QueryRowContext(ctx, checkQuery, c.PackageID, c.Version)
	err := row.Scan(&existingChecksum, &existingRaw, &existingQuar, &existingStatus)
	if err == nil {
		// Row exists: check immutability
		if existingChecksum != c.ChecksumSHA256 || !bytes.Equal(existingRaw, c.RawJSON) {
			return fmt.Errorf("%w: card %s version %d already exists with different content/checksum", ErrConflict, c.PackageID, c.Version)
		}
		// Idempotent retry: update metadata if changed
		if existingQuar != c.Quarantined || existingStatus != c.SourceStatus {
			const updateMeta = `
UPDATE published_cards
SET quarantined = $3, source_status = $4
WHERE package_id = $1 AND version = $2`
			_, uerr := s.db.ExecContext(ctx, updateMeta, c.PackageID, c.Version, c.Quarantined, c.SourceStatus)
			return uerr
		}
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// Insert new row
	const insertQuery = `
INSERT INTO published_cards (package_id, version, raw_json, checksum_sha256, source_status, quarantined)
VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = s.db.ExecContext(ctx, insertQuery,
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

// QuarantineCard marks an incident card version as quarantined or unquarantined.
func (s *Store) QuarantineCard(ctx context.Context, packageID string, version int, quarantined bool) error {
	const query = `
UPDATE published_cards
SET quarantined = $3
WHERE package_id = $1 AND version = $2`
	res, err := s.db.ExecContext(ctx, query, packageID, version, quarantined)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetCardStatus updates the source_status of a published card (e.g. 'CURRENT', 'WITHDRAWN', 'SUPERSEDED').
func (s *Store) SetCardStatus(ctx context.Context, packageID string, version int, status string) error {
	const query = `
UPDATE published_cards
SET source_status = $3
WHERE package_id = $1 AND version = $2`
	res, err := s.db.ExecContext(ctx, query, packageID, version, status)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// PublishResource stores a content-addressed auxiliary asset. Boundary checks validate
// digest and length before persistence. Duplicate publish with identical content
// and checksum is idempotent; mutation under the same resource_id returns ErrConflict.
func (s *Store) PublishResource(ctx context.Context, r *PublishedResource) error {
	if r == nil {
		return errors.New("store: resource is nil")
	}
	if r.ResourceID == "" {
		return errors.New("store: resource_id required")
	}
	if r.ContentType == "" {
		return errors.New("store: content_type required")
	}
	if len(r.Content) == 0 {
		return errors.New("store: content required")
	}
	if r.ContentLength != int64(len(r.Content)) {
		return errors.New("store: content_length mismatch")
	}
	if len(r.ChecksumSHA256) != 64 {
		return errors.New("store: invalid checksum length")
	}
	if got := offlinepkg.ChecksumSHA256(r.Content); got != r.ChecksumSHA256 {
		return fmt.Errorf("store: resource checksum mismatch: computed %q, declared %q", got, r.ChecksumSHA256)
	}

	// Check existing row for immutability
	const checkQuery = `
SELECT checksum_sha256, content
FROM published_resources
WHERE resource_id = $1`

	var (
		existingChecksum string
		existingContent  []byte
	)
	row := s.db.QueryRowContext(ctx, checkQuery, r.ResourceID)
	err := row.Scan(&existingChecksum, &existingContent)
	if err == nil {
		if existingChecksum != r.ChecksumSHA256 || !bytes.Equal(existingContent, r.Content) {
			return fmt.Errorf("%w: resource %s already exists with different content/checksum", ErrConflict, r.ResourceID)
		}
		return nil // Idempotent
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// Insert new row
	const insertQuery = `
INSERT INTO published_resources (resource_id, content_type, content_length, checksum_sha256, content)
VALUES ($1, $2, $3, $4, $5)`
	_, err = s.db.ExecContext(ctx, insertQuery,
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
