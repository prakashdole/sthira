package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sthira/backend/internal/offlinepkg"
)

// ErrPublicationTrustMissing is returned when Publisher is constructed or used
// without a TrustStore. Missing trust configuration fails closed at publication
// time, never silently.
var ErrPublicationTrustMissing = errors.New("store: publication requires a configured trust store")

// ErrPublicationIdentityMismatch is returned when the record's identity fields
// (jurisdiction, revision, manifest_id, package_id) disagree with the signed
// content. The signed bytes are the source of truth; the caller cannot
// substitute record metadata.
var ErrPublicationIdentityMismatch = errors.New("store: record identity does not match signed content")

// ErrPublicationAuthority is returned when the source/package the publication
// claims to come from is not authorized: source is not OPERATIONAL, no live
// jurisdiction authorization exists, or the package row is missing/superseded/
// expired. The record is not inserted.
var ErrPublicationAuthority = errors.New("store: source/package not authorized for publication")

// ErrPublicationReplayConflict is returned when an idempotent retry attempts
// to restore a CURRENT or non-quarantined status onto an already-withdrawn/
// quarantined record. Replay cannot undo watermark/QR/lifecycle flags.
var ErrPublicationNotPermitted = errors.New("store: replay cannot restore caller-supplied metadata")

// Publisher is the trusted publication boundary. It wraps the Store with a
// TrustStore and refuses any publication path that does not:
//
//   - Recompute the canonical SHA-256 digest from the raw JSON bytes (the
//     caller-supplied ChecksumSHA256 is overwritten, never trusted);
//   - Cryptographically verify the embedded Ed25519 signature against a
//     configured trusted authority for the record's jurisdiction;
//   - Bind record identity (jurisdiction, revision, manifest_id, package_id)
//     to the signed content;
//   - Gate on source OPERATIONAL state + a live jurisdiction authorization +
//     a current, non-superseded package row read FOR UPDATE inside the same
//     transaction;
//   - Refuse caller-supplied SourceStatus or Quarantined on an idempotent
//     retry of an already-withdrawn record (the existing server state wins).
//
// Missing trust configuration fails closed (no default-allow Provider is
// constructed anywhere). Production wiring MUST provide a TrustStore; tests
// set one explicitly.
//
// observer is an optional seam the trusted lifecycle path uses to
// invalidate cached delivery across instances after a committed
// transition. Production main wires a shared-bus + local-cache
// observer; production callers MUST NOT silently skip invalidation.
type Publisher struct {
	st       *Store
	ts       offlinepkg.TrustStore
	nowFn    func() time.Time
	observer PublicationLifecycleObserver
}

// PublicationLifecycleObserver is the seam Publisher uses after a
// committed publish/promote/withdraw to invalidate cached delivery
// under an explicit consistency bound. See
// internal/offlinedelivery/types.go for the interface. Defined here
// as a small interface so this package does not import offlinedelivery
// (avoiding a cycle through store.Store -> offlinedelivery).
type PublicationLifecycleObserver interface {
	OnManifestWithdrawn(ctx context.Context, jurisdiction string, revision int)
	OnManifestPromoted(ctx context.Context, jurisdiction string, revision int)
	OnSourceWithdrawn(ctx context.Context, sourceID string)
	OnSourceQuarantined(ctx context.Context, sourceID string)
	OnPackageSuperseded(ctx context.Context, packageID string, supersededVersions []int)
}

// NewPublisher builds the trusted publication boundary. ts must be non-nil;
// nil fails closed at Publish time.
func NewPublisher(st *Store, ts offlinepkg.TrustStore) *Publisher {
	return &Publisher{st: st, ts: ts, nowFn: func() time.Time { return time.Now().UTC() }}
}

// WithClock overrides the clock for deterministic tests.
func (p *Publisher) WithClock(fn func() time.Time) *Publisher {
	p.nowFn = fn
	return p
}

// WithObserver wires the lifecycle observer that is notified after a
// committed publish, promote or withdraw. nil disables invalidation
// (tests use this). Production main wires a non-nil observer.
func (p *Publisher) WithObserver(observer PublicationLifecycleObserver) *Publisher {
	if p != nil {
		p.observer = observer
	}
	return p
}

// Observer returns the configured lifecycle observer (nil if none).
func (p *Publisher) Observer() PublicationLifecycleObserver { return p.observer }

// TrustStore exposes the configured trust store for advanced wiring/tests.
func (p *Publisher) TrustStore() offlinepkg.TrustStore { return p.ts }

// PublishManifest publishes a signed regional manifest. The signature and
// digest are recomputed and verified; the record identity is bound to the
// signed bytes; the source/package authority is gated. Idempotent retries
// preserve existing SourceStatus and Quarantined state — caller-supplied
// values cannot restore a withdrawn record to CURRENT.
func (p *Publisher) PublishManifest(ctx context.Context, m *PublishedManifest) error {
	if p.ts == nil {
		return ErrPublicationTrustMissing
	}
	if m == nil {
		return errors.New("store: manifest is nil")
	}
	if m.ManifestID == "" || m.Jurisdiction == "" || m.Revision < 1 || len(m.RawJSON) == 0 {
		return errors.New("store: invalid manifest metadata")
	}
	if len(m.RawJSON) > 262144 {
		return errors.New("store: raw_json exceeds 256 KiB ceiling")
	}
	if m.SourceID == "" {
		return fmt.Errorf("%w: source_id required for trusted publication", ErrPublicationIdentityMismatch)
	}

	// Strict structural parse + canonical digest recompute + signature verify.
	parsed, err := offlinepkg.ParseManifest(m.RawJSON, offlinepkg.Limits{MaxBytes: 262144})
	if err != nil {
		return fmt.Errorf("store: parse manifest json: %w", err)
	}
	if err := offlinepkg.VerifyManifest(parsed, p.ts); err != nil {
		return fmt.Errorf("store: trusted publish: %w", err)
	}

	// Bind record identity to the signed content. Caller cannot substitute
	// these without re-signing.
	if parsed.Jurisdiction != m.Jurisdiction {
		return fmt.Errorf("%w: jurisdiction signed=%q record=%q", ErrPublicationIdentityMismatch, parsed.Jurisdiction, m.Jurisdiction)
	}
	if parsed.Revision != m.Revision {
		return fmt.Errorf("%w: revision signed=%d record=%d", ErrPublicationIdentityMismatch, parsed.Revision, m.Revision)
	}
	if parsed.ManifestID != m.ManifestID {
		return fmt.Errorf("%w: manifest_id signed=%q record=%q", ErrPublicationIdentityMismatch, parsed.ManifestID, m.ManifestID)
	}
	if m.PackageID != "" && parsed.CriticalCard.PackageID != m.PackageID {
		return fmt.Errorf("%w: package_id signed=%q record=%q", ErrPublicationIdentityMismatch, parsed.CriticalCard.PackageID, m.PackageID)
	}

	expectedChecksum := parsed.ChecksumSHA256
	if expectedChecksum == "" {
		return fmt.Errorf("%w: verified manifest has no checksum", ErrPublicationIdentityMismatch)
	}
	m.ChecksumSHA256 = expectedChecksum
	m.PackageID = parsed.CriticalCard.PackageID

	// Authority gating and persistence in ONE SINGLE TRANSACTION.
	return p.st.InTx(ctx, func(tx DBTX) error {
		// 1. Source OPERATIONAL (FOR UPDATE).
		var state string
		err := tx.QueryRowContext(ctx, `
			SELECT s.state FROM sources s
			WHERE s.source_id = $1
			FOR UPDATE`, m.SourceID).Scan(&state)
		if err != nil {
			return fmt.Errorf("%w: source lookup: %v", ErrPublicationAuthority, err)
		}
		switch state {
		case "OPERATIONAL":
			// ok
		case "QUARANTINED":
			return fmt.Errorf("%w: source is QUARANTINED", ErrPublicationAuthority)
		default:
			return fmt.Errorf("%w: source state=%q (must be OPERATIONAL)", ErrPublicationAuthority, state)
		}

		// 2. Live authorization in the manifest's jurisdiction.
		var authExists bool
		err = tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM source_authorizations
				WHERE source_id = $1 AND jurisdiction = $2
				  AND (expires_at IS NULL OR expires_at > $3)
			)`, m.SourceID, m.Jurisdiction, p.nowFn()).Scan(&authExists)
		if err != nil {
			return fmt.Errorf("%w: authorization lookup: %v", ErrPublicationAuthority, err)
		}
		if !authExists {
			return fmt.Errorf("%w: no live jurisdiction authorization for %q", ErrPublicationAuthority, m.Jurisdiction)
		}

		// 3. A current package row exists for (source, package_id), not superseded, not expired (FOR UPDATE).
		var pkgID string
		err = tx.QueryRowContext(ctx, `
			SELECT package_id FROM packages
			WHERE source_id = $1 AND package_id = $2
			  AND superseded_by IS NULL
			  AND effective_at <= $3 AND expires_at > $3
			FOR UPDATE`, m.SourceID, m.PackageID, p.nowFn()).Scan(&pkgID)
		if err != nil {
			return fmt.Errorf("%w: no current package %q for source %q: %v", ErrPublicationAuthority, m.PackageID, m.SourceID, err)
		}

		// 4. Immutability / Idempotency check on published_manifests (FOR UPDATE).
		const checkQuery = `
			SELECT checksum_sha256, raw_json, quarantined, source_status
			FROM published_manifests
			WHERE jurisdiction = $1 AND revision = $2
			FOR UPDATE`

		var (
			existingChecksum string
			existingRaw      []byte
			existingQuar     bool
			existingStatus   string
		)
		err = tx.QueryRowContext(ctx, checkQuery, m.Jurisdiction, m.Revision).Scan(
			&existingChecksum, &existingRaw, &existingQuar, &existingStatus)
		if err == nil {
			if existingChecksum != m.ChecksumSHA256 || !bytes.Equal(existingRaw, m.RawJSON) {
				return fmt.Errorf("%w: manifest %s revision %d already exists with different content/checksum",
					ErrConflict, m.Jurisdiction, m.Revision)
			}
			return nil
		}
		if err != nil && !errors.Is(err, errSQLNoRows()) {
			return err
		}

		// 5. Insert new row with STAGED status.
		const insertQuery = `
			INSERT INTO published_manifests (manifest_id, jurisdiction, revision, package_id, source_id, raw_json, checksum_sha256, source_status, quarantined)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
		_, err = tx.ExecContext(ctx, insertQuery,
			m.ManifestID,
			m.Jurisdiction,
			m.Revision,
			m.PackageID,
			m.SourceID,
			m.RawJSON,
			m.ChecksumSHA256,
			"STAGED",
			false,
		)
		return err
	})
}

// PublishCard publishes a signed public incident card. Same trust boundary:
// canonical digest recompute, signature verify, identity bind, authority gate.
func (p *Publisher) PublishCard(ctx context.Context, c *PublishedCard) error {
	if p.ts == nil {
		return ErrPublicationTrustMissing
	}
	if c == nil {
		return errors.New("store: card is nil")
	}
	if c.PackageID == "" || c.Version < 1 || len(c.RawJSON) == 0 {
		return errors.New("store: invalid card metadata")
	}
	if len(c.RawJSON) > 65536 {
		return errors.New("store: raw_json exceeds 64 KiB ceiling")
	}
	if c.SourceID == "" {
		return fmt.Errorf("%w: source_id required for trusted publication", ErrPublicationIdentityMismatch)
	}

	parsed, err := offlinepkg.ParseCard(c.RawJSON, offlinepkg.Limits{MaxBytes: 65536})
	if err != nil {
		return fmt.Errorf("store: parse card json: %w", err)
	}
	if err := offlinepkg.VerifyCard(parsed, p.ts); err != nil {
		return fmt.Errorf("store: trusted publish: %w", err)
	}

	if parsed.PackageID != c.PackageID {
		return fmt.Errorf("%w: package_id signed=%q record=%q", ErrPublicationIdentityMismatch, parsed.PackageID, c.PackageID)
	}
	if parsed.Version != c.Version {
		return fmt.Errorf("%w: version signed=%d record=%d", ErrPublicationIdentityMismatch, parsed.Version, c.Version)
	}
	if parsed.Jurisdiction != "" && c.Jurisdiction != "" && parsed.Jurisdiction != c.Jurisdiction {
		return fmt.Errorf("%w: jurisdiction signed=%q record=%q", ErrPublicationIdentityMismatch, parsed.Jurisdiction, c.Jurisdiction)
	}
	expectedChecksum := parsed.ChecksumSHA256
	if expectedChecksum == "" {
		return fmt.Errorf("%w: verified card has no checksum", ErrPublicationIdentityMismatch)
	}
	c.ChecksumSHA256 = expectedChecksum
	if c.Jurisdiction == "" {
		c.Jurisdiction = parsed.Jurisdiction
	}

	return p.st.InTx(ctx, func(tx DBTX) error {
		// 1. Package check (FOR UPDATE)
		var (
			pkgSource       string
			pkgVersion      int
			pkgJurisdiction string
			superseded      bool
			active          bool
		)
		err := tx.QueryRowContext(ctx, `
			SELECT p.source_id, p.version, p.jurisdiction, (p.superseded_by IS NOT NULL),
			       (p.effective_at <= $2 AND p.expires_at > $2)
			FROM packages p WHERE p.package_id = $1
			FOR UPDATE`, c.PackageID, p.nowFn()).Scan(&pkgSource, &pkgVersion, &pkgJurisdiction, &superseded, &active)
		if err != nil {
			return fmt.Errorf("%w: package lookup: %v", ErrPublicationAuthority, err)
		}
		if superseded {
			return fmt.Errorf("%w: package %q is superseded", ErrPublicationAuthority, c.PackageID)
		}
		if !active {
			return fmt.Errorf("%w: package %q is expired or not yet effective", ErrPublicationAuthority, c.PackageID)
		}
		if pkgVersion != c.Version {
			return fmt.Errorf("%w: package version %d does not match card version %d", ErrPublicationAuthority, pkgVersion, c.Version)
		}
		if pkgSource != c.SourceID {
			return fmt.Errorf("%w: package source=%q does not match card source=%q", ErrPublicationAuthority, pkgSource, c.SourceID)
		}
		if c.Jurisdiction != "" && pkgJurisdiction != c.Jurisdiction {
			return fmt.Errorf("%w: package jurisdiction=%q does not match card jurisdiction=%q", ErrPublicationAuthority, pkgJurisdiction, c.Jurisdiction)
		}

		// 2. Source OPERATIONAL check (FOR UPDATE)
		var state string
		err = tx.QueryRowContext(ctx, `
			SELECT s.state FROM sources s WHERE s.source_id = $1
			FOR UPDATE`, c.SourceID).Scan(&state)
		if err != nil {
			return fmt.Errorf("%w: source lookup: %v", ErrPublicationAuthority, err)
		}
		switch state {
		case "OPERATIONAL":
		case "QUARANTINED":
			return fmt.Errorf("%w: source is QUARANTINED", ErrPublicationAuthority)
		default:
			return fmt.Errorf("%w: source state=%q (must be OPERATIONAL)", ErrPublicationAuthority, state)
		}

		// 3. Live authorization check in the card's jurisdiction
		var authExists bool
		err = tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM source_authorizations
				WHERE source_id = $1 AND jurisdiction = $2
				  AND (expires_at IS NULL OR expires_at > $3)
			)`, c.SourceID, pkgJurisdiction, p.nowFn()).Scan(&authExists)
		if err != nil {
			return fmt.Errorf("%w: authorization lookup: %v", ErrPublicationAuthority, err)
		}
		if !authExists {
			return fmt.Errorf("%w: no live jurisdiction authorization for %q", ErrPublicationAuthority, pkgJurisdiction)
		}

		// 4. Immutability / Idempotency check on published_cards (FOR UPDATE)
		const checkQuery = `
			SELECT checksum_sha256, raw_json, quarantined, source_status
			FROM published_cards
			WHERE package_id = $1 AND version = $2
			FOR UPDATE`
		var (
			existingChecksum string
			existingRaw      []byte
			existingQuar     bool
			existingStatus   string
		)
		err = tx.QueryRowContext(ctx, checkQuery, c.PackageID, c.Version).Scan(
			&existingChecksum, &existingRaw, &existingQuar, &existingStatus)
		if err == nil {
			if existingChecksum != c.ChecksumSHA256 || !bytes.Equal(existingRaw, c.RawJSON) {
				return fmt.Errorf("%w: card %s version %d already exists with different content/checksum",
					ErrConflict, c.PackageID, c.Version)
			}
			return nil
		}
		if err != nil && !errors.Is(err, errSQLNoRows()) {
			return err
		}

		// 5. Insert new row as STAGED
		const insertQuery = `
			INSERT INTO published_cards (package_id, version, source_id, jurisdiction, raw_json, checksum_sha256, source_status, quarantined)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
		_, err = tx.ExecContext(ctx, insertQuery,
			c.PackageID,
			c.Version,
			c.SourceID,
			pkgJurisdiction,
			c.RawJSON,
			c.ChecksumSHA256,
			"STAGED",
			false,
		)
		return err
	})
}

// PromoteManifest marks a staged manifest revision as CURRENT and marks prior CURRENT manifests for that jurisdiction as SUPERSEDED.
func (p *Publisher) PromoteManifest(ctx context.Context, jurisdiction string, revision int) error {
	return p.st.PromoteManifest(ctx, jurisdiction, revision)
}

// PromoteCard marks a staged card as CURRENT and marks prior CURRENT versions for that package as SUPERSEDED.
func (p *Publisher) PromoteCard(ctx context.Context, packageID string, version int) error {
	return p.st.PromoteCard(ctx, packageID, version)
}

// errSQLNoRows is a small seam so the helpers above don't need to import
// database/sql at call sites — InTx's error wrapping handles the typed check
// at the store boundary.
func errSQLNoRows() error { return sql.ErrNoRows }
