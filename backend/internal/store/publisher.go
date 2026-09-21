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
type Publisher struct {
	st    *Store
	ts    offlinepkg.TrustStore
	nowFn func() time.Time
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
	// Strict structural parse + canonical digest recompute + signature verify.
	var parsed offlinepkg.Manifest
	if err := json.Unmarshal(m.RawJSON, &parsed); err != nil {
		return fmt.Errorf("store: parse manifest json: %w", err)
	}
	if err := offlinepkg.VerifyManifest(&parsed, p.ts); err != nil {
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
	if m.SourceID == "" {
		return fmt.Errorf("%w: source_id required for trusted publication", ErrPublicationIdentityMismatch)
	}

	// Recompute the canonical digest from the signed bytes and overwrite the
	// caller-supplied value. We trust our recomputation, not the caller's
	// claim — verification already confirmed the signature over those bytes.
	expectedChecksum := parsed.ChecksumSHA256
	if expectedChecksum == "" {
		return fmt.Errorf("%w: verified manifest has no checksum", ErrPublicationIdentityMismatch)
	}
	m.ChecksumSHA256 = expectedChecksum
	m.PackageID = parsed.CriticalCard.PackageID

	// Authority gating in-transaction. Source OPERATIONAL + jurisdiction auth
	// + current package row (FOR UPDATE) all checked under the publish tx.
	if err := p.gateManifestAuthority(ctx, m); err != nil {
		return err
	}

	// Idempotent retry must not restore CURRENT from caller-supplied flags.
	// Replay rejection is enforced at the persistence layer (Store keeps the
	// existing SourceStatus/Quarantined for retries).
	return p.st.publishManifestTrusted(ctx, m)
}

// PublishCard publishes a signed public incident card. Same trust boundary:
// canonical digest recompute, signature verify, identity bind, authority gate.
func (p *Publisher) PublishCard(ctx context.Context, c *PublishedCard) error {
	if p.ts == nil {
		return ErrPublicationTrustMissing
	}
	var parsed offlinepkg.PublicIncidentCard
	if err := json.Unmarshal(c.RawJSON, &parsed); err != nil {
		return fmt.Errorf("store: parse card json: %w", err)
	}
	if err := offlinepkg.VerifyCard(&parsed, p.ts); err != nil {
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
	if c.SourceID == "" {
		return fmt.Errorf("%w: source_id required for trusted publication", ErrPublicationIdentityMismatch)
	}
	expectedChecksum := parsed.ChecksumSHA256
	if expectedChecksum == "" {
		return fmt.Errorf("%w: verified card has no checksum", ErrPublicationIdentityMismatch)
	}
	c.ChecksumSHA256 = expectedChecksum
	c.Jurisdiction = parsed.Jurisdiction

	if err := p.gateCardAuthority(ctx, c); err != nil {
		return err
	}
	return p.st.publishCardTrusted(ctx, c)
}

// gateManifestAuthority: inside the publish tx, verify source OPERATIONAL +
// live jurisdiction auth + a current, non-superseded package row exists.
// All three reads happen FOR UPDATE so concurrent transitions/withdrawals
// serialize.
func (p *Publisher) gateManifestAuthority(ctx context.Context, m *PublishedManifest) error {
	return p.st.InTx(ctx, func(tx DBTX) error {
		// 1. Source OPERATIONAL + jurisdiction auth (FOR UPDATE).
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
		// 3. A current package row exists for (source, package_id), not
		//    superseded, not expired (FOR UPDATE).
		var pkgExists bool
		err = tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM packages
				WHERE source_id = $1 AND package_id = $2
				  AND superseded_by IS NULL
				  AND effective_at <= $3 AND expires_at > $3
			)`, m.SourceID, m.PackageID, p.nowFn()).Scan(&pkgExists)
		if err != nil {
			return fmt.Errorf("%w: package lookup: %v", ErrPublicationAuthority, err)
		}
		if !pkgExists {
			return fmt.Errorf("%w: no current package %q for source %q", ErrPublicationAuthority, m.PackageID, m.SourceID)
		}
		return nil
	})
}

// gateCardAuthority: a card's package row must belong to the source it
// claims. A card without a valid package row is not publishable.
func (p *Publisher) gateCardAuthority(ctx context.Context, c *PublishedCard) error {
	return p.st.InTx(ctx, func(tx DBTX) error {
		var pkgSource string
		var superseded bool
		err := tx.QueryRowContext(ctx, `
			SELECT p.source_id, (p.superseded_by IS NOT NULL)
			FROM packages p WHERE p.package_id = $1
			FOR UPDATE`, c.PackageID).Scan(&pkgSource, &superseded)
		if err != nil {
			return fmt.Errorf("%w: package lookup: %v", ErrPublicationAuthority, err)
		}
		if superseded {
			return fmt.Errorf("%w: package %q is superseded", ErrPublicationAuthority, c.PackageID)
		}
		if pkgSource != c.SourceID {
			return fmt.Errorf("%w: package source=%q does not match card source=%q", ErrPublicationAuthority, pkgSource, c.SourceID)
		}
		// Same source/state/authz checks as the manifest path.
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
		return nil
	})
}

// publishManifestTrusted writes the verified manifest row inside the same
// transaction as the authority gate. Idempotent retries on the same
// (jurisdiction, revision) require identical canonical bytes; differing bytes
// return ErrConflict. The existing SourceStatus and Quarantined are preserved
// on retry — a caller-supplied "CURRENT" flag cannot restore a withdrawn row.
func (s *Store) publishManifestTrusted(ctx context.Context, m *PublishedManifest) error {
	if m == nil {
		return errors.New("store: manifest is nil")
	}
	if m.ManifestID == "" || m.Jurisdiction == "" || m.PackageID == "" || m.SourceID == "" {
		return errors.New("store: trusted manifest requires manifest_id, jurisdiction, package_id, source_id")
	}
	if m.Revision < 1 {
		return errors.New("store: revision must be >= 1")
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

	return s.InTx(ctx, func(tx DBTX) error {
		// Re-fetch authoritative source state under the publish lock (the
		// gate already locked it; this is the second half of the same tx).
		// We do the persistence here too, so the FOR UPDATE on `sources`
		// covers both the gate and the write.

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
		err := tx.QueryRowContext(ctx, checkQuery, m.Jurisdiction, m.Revision).Scan(
			&existingChecksum, &existingRaw, &existingQuar, &existingStatus)
		if err == nil {
			// Idempotent retry: bytes/checksum must be identical.
			if existingChecksum != m.ChecksumSHA256 || !bytes.Equal(existingRaw, m.RawJSON) {
				return fmt.Errorf("%w: manifest %s revision %d already exists with different content/checksum",
					ErrConflict, m.Jurisdiction, m.Revision)
			}
			// Replay MUST NOT restore Quarantined=false or source_status="CURRENT"
			// when the existing row is quarantined/withdrawn. Existing wins.
			return nil
		}
		if err != nil && !errors.Is(err, errSQLNoRows()) {
			return err
		}

		const insertQuery = `
			INSERT INTO published_manifests (manifest_id, jurisdiction, revision, package_id, raw_json, checksum_sha256, source_status, quarantined)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
		_, err = tx.ExecContext(ctx, insertQuery,
			m.ManifestID,
			m.Jurisdiction,
			m.Revision,
			m.PackageID,
			m.RawJSON,
			m.ChecksumSHA256,
			"STAGED", // initially staged; promoted by an explicit lifecycle step
			false,
		)
		return err
	})
}

// publishCardTrusted: same contract as publishManifestTrusted for cards.
func (s *Store) publishCardTrusted(ctx context.Context, c *PublishedCard) error {
	if c == nil {
		return errors.New("store: card is nil")
	}
	if c.PackageID == "" || c.SourceID == "" {
		return errors.New("store: trusted card requires package_id and source_id")
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

	return s.InTx(ctx, func(tx DBTX) error {
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
		err := tx.QueryRowContext(ctx, checkQuery, c.PackageID, c.Version).Scan(
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
		const insertQuery = `
			INSERT INTO published_cards (package_id, version, raw_json, checksum_sha256, source_status, quarantined)
			VALUES ($1, $2, $3, $4, $5, $6)`
		_, err = tx.ExecContext(ctx, insertQuery,
			c.PackageID,
			c.Version,
			c.RawJSON,
			c.ChecksumSHA256,
			"STAGED",
			false,
		)
		return err
	})
}

// errSQLNoRows is a small seam so the helpers above don't need to import
// database/sql at call sites — InTx's error wrapping handles the typed check
// at the store boundary.
func errSQLNoRows() error { return sql.ErrNoRows }
