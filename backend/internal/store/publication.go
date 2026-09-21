package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

	parsed, err := offlinepkg.ParseManifest(m.RawJSON, offlinepkg.Limits{MaxBytes: 262144})
	if err != nil {
		return fmt.Errorf("store: parse manifest json: %w", err)
	}
	if parsed.ChecksumSHA256 != m.ChecksumSHA256 {
		return fmt.Errorf("store: manifest checksum mismatch: payload has %q, record has %q", parsed.ChecksumSHA256, m.ChecksumSHA256)
	}
	if m.SourceStatus == "" {
		m.SourceStatus = "CURRENT"
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
	err = row.Scan(&existingChecksum, &existingRaw, &existingQuar, &existingStatus)
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
INSERT INTO published_manifests (manifest_id, jurisdiction, revision, package_id, source_id, raw_json, checksum_sha256, source_status, quarantined)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	var sourceID sql.NullString
	if m.SourceID != "" {
		sourceID = sql.NullString{String: m.SourceID, Valid: true}
	}
	_, err = s.db.ExecContext(ctx, insertQuery,
		m.ManifestID,
		m.Jurisdiction,
		m.Revision,
		m.PackageID,
		sourceID,
		m.RawJSON,
		m.ChecksumSHA256,
		m.SourceStatus,
		m.Quarantined,
	)
	return err
}

// GetPublishedManifest retrieves the current published manifest for a jurisdiction.
// Staging a newer publication must not hide the valid CURRENT one.
// STAGED manifests are excluded from active selection.
func (s *Store) GetPublishedManifest(ctx context.Context, jurisdiction string) (*PublishedManifest, error) {
	const query = `
SELECT manifest_id, jurisdiction, revision, package_id, COALESCE(source_id, ''), raw_json, checksum_sha256, source_status, quarantined, created_at
FROM published_manifests
WHERE jurisdiction = $1 AND source_status != 'STAGED'
ORDER BY (CASE WHEN source_status = 'CURRENT' THEN 1 ELSE 0 END) DESC, revision DESC
LIMIT 1`
	row := s.db.QueryRowContext(ctx, query, jurisdiction)
	var m PublishedManifest
	err := row.Scan(
		&m.ManifestID,
		&m.Jurisdiction,
		&m.Revision,
		&m.PackageID,
		&m.SourceID,
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

// PromoteManifest marks a staged manifest revision as CURRENT and marks prior CURRENT manifests for that jurisdiction as SUPERSEDED.
// It transactionally binds promotion to attributed source, package, jurisdiction, live authorization,
// active unexpired non-superseded package, and manifest-card version relationship.
// Rejects legacy unattributed promotion with no writes.
func (s *Store) PromoteManifest(ctx context.Context, jurisdiction string, revision int) error {
	now := time.Now().UTC()
	return s.InTx(ctx, func(tx DBTX) error {
		var (
			manifestID   string
			packageID    string
			sourceIDNull sql.NullString
			status       string
			quarantined  bool
		)
		err := tx.QueryRowContext(ctx, `
			SELECT manifest_id, package_id, source_id, source_status, quarantined
			FROM published_manifests
			WHERE jurisdiction = $1 AND revision = $2`, jurisdiction, revision).
			Scan(&manifestID, &packageID, &sourceIDNull, &status, &quarantined)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if quarantined {
			return errors.New("store: cannot promote quarantined manifest")
		}
		if status == "WITHDRAWN" {
			return errors.New("store: cannot promote withdrawn manifest")
		}
		if status == "CURRENT" {
			return nil // idempotent
		}
		// Reject legacy unattributed row (source_id is NULL or empty) with no writes
		if !sourceIDNull.Valid || strings.TrimSpace(sourceIDNull.String) == "" {
			return fmt.Errorf("%w: cannot promote unattributed manifest: source_id required", ErrPublicationAuthority)
		}
		sourceID := sourceIDNull.String

		// Strict lock order across transactions:
		// 1. sources (FOR UPDATE)
		// 2. packages (FOR UPDATE)
		// 3. published_manifests (FOR UPDATE)
		var srcState string
		err = tx.QueryRowContext(ctx, `
			SELECT state FROM sources WHERE source_id = $1 FOR UPDATE`, sourceID).Scan(&srcState)
		if err != nil {
			return fmt.Errorf("%w: source lookup: %v", ErrPublicationAuthority, err)
		}
		if srcState != "OPERATIONAL" {
			return fmt.Errorf("%w: source state is %q (must be OPERATIONAL)", ErrPublicationAuthority, srcState)
		}

		// Check live authorization in jurisdiction
		var authExists bool
		err = tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM source_authorizations
				WHERE source_id = $1 AND jurisdiction = $2
				  AND (expires_at IS NULL OR expires_at > $3)
			)`, sourceID, jurisdiction, now).Scan(&authExists)
		if err != nil {
			return fmt.Errorf("%w: authorization lookup: %v", ErrPublicationAuthority, err)
		}
		if !authExists {
			return fmt.Errorf("%w: no live authorization for %q", ErrPublicationAuthority, jurisdiction)
		}

		// 2. Lock packages FOR UPDATE
		var (
			pkgVersion    int
			pkgSuperseded bool
			pkgActive     bool
		)
		err = tx.QueryRowContext(ctx, `
			SELECT version, (superseded_by IS NOT NULL), (effective_at <= $3 AND expires_at > $3)
			FROM packages
			WHERE source_id = $1 AND package_id = $2
			FOR UPDATE`, sourceID, packageID, now).Scan(&pkgVersion, &pkgSuperseded, &pkgActive)
		if err != nil {
			return fmt.Errorf("%w: package lookup: %v", ErrPublicationAuthority, err)
		}
		if pkgSuperseded {
			return fmt.Errorf("%w: package %q is superseded", ErrPublicationAuthority, packageID)
		}
		if !pkgActive {
			return fmt.Errorf("%w: package %q is expired or not yet effective", ErrPublicationAuthority, packageID)
		}

		// Check manifest CriticalCard package and version
		var rawJSON []byte
		err = tx.QueryRowContext(ctx, `
			SELECT raw_json FROM published_manifests
			WHERE jurisdiction = $1 AND revision = $2`, jurisdiction, revision).Scan(&rawJSON)
		if err == nil && len(rawJSON) > 0 {
			parsed, perr := offlinepkg.ParseManifest(rawJSON, offlinepkg.Limits{MaxBytes: 262144})
			if perr == nil {
				if parsed.CriticalCard.PackageID != packageID {
					return fmt.Errorf("%w: manifest critical_card package_id %q does not match manifest package_id %q", ErrPublicationAuthority, parsed.CriticalCard.PackageID, packageID)
				}
				if parsed.CriticalCard.Version != pkgVersion {
					return fmt.Errorf("%w: manifest critical_card version %d does not match package version %d", ErrPublicationAuthority, parsed.CriticalCard.Version, pkgVersion)
				}
			}
		}

		// 3. Lock published_manifests FOR UPDATE and execute promotion
		var currentStatus string
		var currentQuar bool
		err = tx.QueryRowContext(ctx, `
			SELECT source_status, quarantined
			FROM published_manifests
			WHERE jurisdiction = $1 AND revision = $2
			FOR UPDATE`, jurisdiction, revision).Scan(&currentStatus, &currentQuar)
		if err != nil {
			return err
		}
		if currentQuar || currentStatus == "WITHDRAWN" {
			return fmt.Errorf("%w: manifest state changed concurrently", ErrPublicationAuthority)
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE published_manifests
			SET source_status = 'SUPERSEDED'
			WHERE jurisdiction = $1 AND revision < $2 AND source_status = 'CURRENT'`,
			jurisdiction, revision)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE published_manifests
			SET source_status = 'CURRENT'
			WHERE jurisdiction = $1 AND revision = $2`,
			jurisdiction, revision)
		return err
	})
}

// PromoteManifestWithObserver atomically promotes the manifest and,
// on success, fires the observer so cached delivery is purged under
// the documented consistency bound. Tests use this to verify that
// promotions surface through to delivery; production wire calls
// the underlying transaction.
func (s *Store) PromoteManifestWithObserver(ctx context.Context, jurisdiction string, revision int, observer PublicationLifecycleObserver) error {
	if err := s.PromoteManifest(ctx, jurisdiction, revision); err != nil {
		return err
	}
	if observer != nil {
		observer.OnManifestPromoted(ctx, jurisdiction, revision)
	}
	return nil
}

// WithdrawManifestAndInvalidate sets the manifest status to WITHDRAWN
// in one transaction and notifies the observer outside the tx so
// cached delivery stops serving immediately. The InTx is bounded;
// failure to invalidate does NOT roll back the withdrawal.
func (s *Store) WithdrawManifestAndInvalidate(ctx context.Context, jurisdiction string, revision int, observer PublicationLifecycleObserver) error {
	if err := s.SetManifestStatus(ctx, jurisdiction, revision, "WITHDRAWN"); err != nil {
		return err
	}
	if observer != nil {
		observer.OnManifestWithdrawn(ctx, jurisdiction, revision)
	}
	return nil
}

// QuarantineManifestAndInvalidate quarantines the manifest and
// notifies the observer. Same semantics as WithdrawManifestAndInvalidate.
func (s *Store) QuarantineManifestAndInvalidate(ctx context.Context, jurisdiction string, revision int, observer PublicationLifecycleObserver) error {
	if err := s.QuarantineManifest(ctx, jurisdiction, revision, true); err != nil {
		return err
	}
	if observer != nil {
		observer.OnSourceQuarantined(ctx, "manifest:"+jurisdiction)
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

	parsed, err := offlinepkg.ParseCard(c.RawJSON, offlinepkg.Limits{MaxBytes: 65536})
	if err != nil {
		return fmt.Errorf("store: parse card json: %w", err)
	}
	if parsed.ChecksumSHA256 != c.ChecksumSHA256 {
		return fmt.Errorf("store: card checksum mismatch: payload has %q, record has %q", parsed.ChecksumSHA256, c.ChecksumSHA256)
	}
	if c.SourceStatus == "" {
		c.SourceStatus = "CURRENT"
	}
	if c.Jurisdiction == "" && parsed.Jurisdiction != "" {
		c.Jurisdiction = parsed.Jurisdiction
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
	err = row.Scan(&existingChecksum, &existingRaw, &existingQuar, &existingStatus)
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
INSERT INTO published_cards (package_id, version, source_id, jurisdiction, raw_json, checksum_sha256, source_status, quarantined)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	var sourceID, jurisdiction sql.NullString
	if c.SourceID != "" {
		sourceID = sql.NullString{String: c.SourceID, Valid: true}
	}
	if c.Jurisdiction != "" {
		jurisdiction = sql.NullString{String: c.Jurisdiction, Valid: true}
	}
	_, err = s.db.ExecContext(ctx, insertQuery,
		c.PackageID,
		c.Version,
		sourceID,
		jurisdiction,
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
SELECT package_id, version, COALESCE(source_id, ''), COALESCE(jurisdiction, ''), raw_json, checksum_sha256, source_status, quarantined, created_at
FROM published_cards
WHERE package_id = $1 AND version = $2`
	row := s.db.QueryRowContext(ctx, query, packageID, version)
	var c PublishedCard
	err := row.Scan(
		&c.PackageID,
		&c.Version,
		&c.SourceID,
		&c.Jurisdiction,
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

// PromoteCard marks a staged card as CURRENT and marks prior CURRENT versions for that package as SUPERSEDED.
// It transactionally binds promotion to attributed source, package, jurisdiction, live authorization,
// active unexpired non-superseded package, and version relationship.
// Rejects legacy unattributed promotion with no writes.
func (s *Store) PromoteCard(ctx context.Context, packageID string, version int) error {
	now := time.Now().UTC()
	return s.InTx(ctx, func(tx DBTX) error {
		var (
			sourceIDNull sql.NullString
			jurisdiction sql.NullString
			status       string
			quarantined  bool
		)
		err := tx.QueryRowContext(ctx, `
			SELECT source_id, jurisdiction, source_status, quarantined
			FROM published_cards
			WHERE package_id = $1 AND version = $2`, packageID, version).
			Scan(&sourceIDNull, &jurisdiction, &status, &quarantined)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if quarantined {
			return errors.New("store: cannot promote quarantined card")
		}
		if status == "WITHDRAWN" {
			return errors.New("store: cannot promote withdrawn card")
		}
		if status == "CURRENT" {
			return nil // idempotent
		}
		// Reject legacy unattributed row (source_id is NULL or empty) with no writes
		if !sourceIDNull.Valid || strings.TrimSpace(sourceIDNull.String) == "" {
			return fmt.Errorf("%w: cannot promote unattributed card: source_id required", ErrPublicationAuthority)
		}
		sourceID := sourceIDNull.String
		jur := jurisdiction.String

		// Strict lock order across transactions:
		// 1. sources (FOR UPDATE)
		// 2. packages (FOR UPDATE)
		// 3. published_cards (FOR UPDATE)
		var srcState string
		err = tx.QueryRowContext(ctx, `
			SELECT state FROM sources WHERE source_id = $1 FOR UPDATE`, sourceID).Scan(&srcState)
		if err != nil {
			return fmt.Errorf("%w: source lookup: %v", ErrPublicationAuthority, err)
		}
		if srcState != "OPERATIONAL" {
			return fmt.Errorf("%w: source state is %q (must be OPERATIONAL)", ErrPublicationAuthority, srcState)
		}

		// Check live authorization in jurisdiction (if jurisdiction present)
		if jur != "" {
			var authExists bool
			err = tx.QueryRowContext(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM source_authorizations
					WHERE source_id = $1 AND jurisdiction = $2
					  AND (expires_at IS NULL OR expires_at > $3)
				)`, sourceID, jur, now).Scan(&authExists)
			if err != nil {
				return fmt.Errorf("%w: authorization lookup: %v", ErrPublicationAuthority, err)
			}
			if !authExists {
				return fmt.Errorf("%w: no live authorization for %q", ErrPublicationAuthority, jur)
			}
		}

		// 2. Lock packages FOR UPDATE
		var (
			pkgVersion      int
			pkgJurisdiction string
			pkgSuperseded   bool
			pkgActive       bool
		)
		err = tx.QueryRowContext(ctx, `
			SELECT version, jurisdiction, (superseded_by IS NOT NULL), (effective_at <= $3 AND expires_at > $3)
			FROM packages
			WHERE source_id = $1 AND package_id = $2
			FOR UPDATE`, sourceID, packageID, now).Scan(&pkgVersion, &pkgJurisdiction, &pkgSuperseded, &pkgActive)
		if err != nil {
			return fmt.Errorf("%w: package lookup: %v", ErrPublicationAuthority, err)
		}
		if pkgVersion != version {
			return fmt.Errorf("%w: card version %d does not match package version %d", ErrPublicationAuthority, version, pkgVersion)
		}
		if jur != "" && pkgJurisdiction != jur {
			return fmt.Errorf("%w: package jurisdiction %q does not match card jurisdiction %q", ErrPublicationAuthority, pkgJurisdiction, jur)
		}
		if pkgSuperseded {
			return fmt.Errorf("%w: package %q is superseded", ErrPublicationAuthority, packageID)
		}
		if !pkgActive {
			return fmt.Errorf("%w: package %q is expired or not yet effective", ErrPublicationAuthority, packageID)
		}

		// 3. Lock published_cards FOR UPDATE and execute promotion
		var currentStatus string
		var currentQuar bool
		err = tx.QueryRowContext(ctx, `
			SELECT source_status, quarantined
			FROM published_cards
			WHERE package_id = $1 AND version = $2
			FOR UPDATE`, packageID, version).Scan(&currentStatus, &currentQuar)
		if err != nil {
			return err
		}
		if currentQuar || currentStatus == "WITHDRAWN" {
			return fmt.Errorf("%w: card state changed concurrently", ErrPublicationAuthority)
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE published_cards
			SET source_status = 'SUPERSEDED'
			WHERE package_id = $1 AND version < $2 AND source_status = 'CURRENT'`,
			packageID, version)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE published_cards
			SET source_status = 'CURRENT'
			WHERE package_id = $1 AND version = $2`,
			packageID, version)
		return err
	})
}

// PromoteCardWithObserver atomically promotes the card and,
// on success, fires the observer so cached delivery is purged under
// the documented consistency bound.
func (s *Store) PromoteCardWithObserver(ctx context.Context, packageID string, version int, observer PublicationLifecycleObserver) error {
	if err := s.PromoteCard(ctx, packageID, version); err != nil {
		return err
	}
	if observer != nil {
		observer.OnPackageSuperseded(ctx, packageID, []int{version})
	}
	return nil
}

// WithdrawCardAndInvalidate sets the card status to WITHDRAWN
// in one transaction and notifies the observer outside the tx.
func (s *Store) WithdrawCardAndInvalidate(ctx context.Context, packageID string, version int, observer PublicationLifecycleObserver) error {
	if err := s.SetCardStatus(ctx, packageID, version, "WITHDRAWN"); err != nil {
		return err
	}
	if observer != nil {
		observer.OnPackageSuperseded(ctx, packageID, []int{version})
	}
	return nil
}

// QuarantineCardAndInvalidate quarantines the card and
// notifies the observer. Same semantics as WithdrawCardAndInvalidate.
func (s *Store) QuarantineCardAndInvalidate(ctx context.Context, packageID string, version int, observer PublicationLifecycleObserver) error {
	if err := s.QuarantineCard(ctx, packageID, version, true); err != nil {
		return err
	}
	if observer != nil {
		observer.OnSourceQuarantined(ctx, "card:"+packageID)
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
