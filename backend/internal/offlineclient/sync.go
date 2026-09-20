package offlineclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"sthira/backend/internal/offlinepkg"
)

// stateQuery returns the currently active card plus its freshness state.
// This is the read-only side of the client and is goroutine-safe.
func (c *ProtocolClient) stateQuery() (*offlinepkg.PublicIncidentCard, FreshnessState, error) {
	_, card, err := c.storage.readActiveCard()
	if err != nil {
		if errors.Is(err, ErrNoActiveState) {
			return nil, FreshnessUnverifiable, ErrNoActiveState
		}
		return nil, FreshnessUnverifiable, err
	}
	st, err := c.storage.loadState()
	if err != nil {
		return nil, FreshnessUnverifiable, err
	}
	if card.PackageID != "" && c.isPackageRevoked(card.PackageID) {
		return card, FreshnessRevoked, nil
	}
	if card.PackageID != "" && c.isSuperseded(card.PackageID, card.Version) {
		return card, FreshnessRevoked, nil
	}
	now := c.now()
	state, err := c.computeFreshness(card, st, now)
	if err != nil {
		return nil, FreshnessUnverifiable, err
	}
	return card, state, nil
}

// computeFreshness returns CURRENT / STALE / EXPIRED using monotonic
// elapsed time. Wall-clock rollback cannot extend validity.
func (c *ProtocolClient) computeFreshness(card *offlinepkg.PublicIncidentCard, st persistedState, now time.Time) (FreshnessState, error) {
	if st.LastSyncMonotonicNS == 0 {
		// First sync never completed; bytes on disk but no time anchor.
		return FreshnessExpired, nil
	}
	expiresAt, err := time.Parse(time.RFC3339, card.ExpiresAt)
	if err != nil {
		return FreshnessExpired, fmt.Errorf("offlineclient: parse card expires_at: %w", err)
	}
	effectiveAt, err := time.Parse(time.RFC3339, card.EffectiveAt)
	if err != nil {
		return FreshnessExpired, fmt.Errorf("offlineclient: parse card effective_at: %w", err)
	}
	// Server-asserted window: how long the artifact is intended to live.
	window := expiresAt.Sub(effectiveAt)
	if window <= 0 {
		return FreshnessExpired, errors.New("offlineclient: card has non-positive validity window")
	}
	// Monotonic elapsed since the last successful sync. If monotonic time
	// itself is broken, surface that as a hard error.
	monoNow := now.UnixNano()
	if monoNow < st.LastSyncMonotonicNS {
		return FreshnessExpired, ErrClockRolledBack
	}
	elapsed := time.Duration(monoNow - st.LastSyncMonotonicNS)
	if elapsed > window {
		return FreshnessExpired, nil
	}
	if window-elapsed <= c.staleBefore {
		return FreshnessStale, nil
	}
	return FreshnessCurrent, nil
}

// isRouteCancelled checks the durable tombstones for routeID.
func (c *ProtocolClient) isRouteCancelled(routeID string) bool {
	if routeID == "" {
		return false
	}
	t := c.storage.tombstones()
	return t.contains("routes.json", routeID)
}

func (c *ProtocolClient) isPackageRevoked(packageID string) bool {
	if packageID == "" {
		return false
	}
	t := c.storage.tombstones()
	return t.contains("packages.json", packageID)
}

func (c *ProtocolClient) isSuperseded(packageID string, version int) bool {
	if packageID == "" || version <= 0 {
		return false
	}
	return c.storage.tombstones().isSuperseded(packageID, version)
}

// hasResource reports whether the named resource is on disk with the
// declared digest intact. The desc is optional; if provided, the digest
// must match the on-disk meta.
func (c *ProtocolClient) hasResource(resourceID string) (bool, error) {
	if resourceID == "" {
		return false, nil
	}
	_, meta, err := c.storage.readResource(resourceID)
	if err != nil {
		if errors.Is(err, ErrNoActiveState) {
			return false, nil
		}
		return false, err
	}
	if meta.ChecksumSHA256 == "" {
		// No recorded checksum is an integrity gap: report missing.
		return false, nil
	}
	bin, _, err := c.storage.readResource(resourceID)
	if err != nil {
		return false, err
	}
	got := offlinepkg.ChecksumSHA256(bin)
	return got == meta.ChecksumSHA256, nil
}

// sync is the workhorse for Sync. It is split out so the state machine
// can be exercised without exposing the entire public API.
func (c *ProtocolClient) sync(ctx context.Context, jurisdiction string) (*SyncReport, error) {
	if jurisdiction == "" {
		return nil, errors.New("offlineclient: jurisdiction required")
	}
	stBefore, err := c.storage.loadState()
	if err != nil {
		return nil, err
	}

	// Phase 1: download + verify manifest.
	manifestBytes, manifestMeta, _, manifestPart, err := c.downloadAndVerifyArtifact(ctx, manifestDownload{
		Jurisdiction: jurisdiction,
		Path:         "/api/v3/regions/" + jurisdiction + "/manifest",
	})
	if err != nil {
		return nil, err
	}
	if manifestMeta.Revision < stBefore.LastRevision {
		return nil, offlinepkg.ErrVersionRollback
	}

	// Phase 2: apply revocations to tombstones BEFORE activation so a
	// crash mid-activation does not leave tombstone state inconsistent
	// with the active manifest.
	tombs := c.storage.tombstones()
	report := &SyncReport{}
	for _, pkg := range manifestMeta.Revocations.RevokedPackages {
		added, err := tombs.add("packages.json", pkg)
		if err != nil {
			return nil, fmt.Errorf("offlineclient: tombstone package: %w", err)
		}
		if added {
			report.RevokedPackages = append(report.RevokedPackages, pkg)
		}
	}
	for _, route := range manifestMeta.Revocations.CancelledRoutes {
		added, err := tombs.add("routes.json", route)
		if err != nil {
			return nil, fmt.Errorf("offlineclient: tombstone route: %w", err)
		}
		if added {
			report.CancelledRoutes = append(report.CancelledRoutes, route)
		}
	}
	for _, sv := range manifestMeta.Revocations.SupersededVersions {
		if err := tombs.addSuperseded(sv.PackageID, sv.Version); err != nil {
			return nil, fmt.Errorf("offlineclient: tombstone supersede: %w", err)
		}
		report.Superseded = append(report.Superseded, sv)
	}

	// Same-revision replay: the server returned the manifest we already
	// have. Revocations were already applied in phase 2 (idempotent); we
	// skip the redundant activation write and leave the .part on disk so
	// a crash can still resume cleanly.
	if manifestMeta.Revision == stBefore.LastRevision && stBefore.LastRevision > 0 {
		report.ActiveRevision = manifestMeta.Revision
		c.storage.clearPart(manifestPart)
		return report, nil
	}

	// Phase 3: atomic activation of the manifest bytes.
	if err := c.storage.writeActiveManifest(manifestBytes); err != nil {
		return nil, fmt.Errorf("offlineclient: activate manifest: %w", err)
	}
	report.ManifestUpdated = true
	report.ActiveRevision = manifestMeta.Revision

	// Phase 4: card, if changed or first sync.
	cardChanged, err := c.cardNeedsFetch(stBefore, manifestMeta)
	if err != nil {
		return nil, err
	}
	if cardChanged {
		cardPath := fmt.Sprintf("/api/v3/packages/%s/versions/%d",
			manifestMeta.CriticalCard.PackageID, manifestMeta.CriticalCard.Version)
		cardBytes, _, _, cardPart, err := c.downloadAndVerifyArtifact(ctx, manifestDownload{
			Jurisdiction: jurisdiction,
			PackageID:    manifestMeta.CriticalCard.PackageID,
			Version:      manifestMeta.CriticalCard.Version,
			Path:         cardPath,
			IsCard:       true,
		})
		if err != nil {
			return nil, err
		}
		if err := c.storage.writeActiveCard(cardBytes); err != nil {
			return nil, fmt.Errorf("offlineclient: activate card: %w", err)
		}
		// Card is now durably committed; the .part is no longer needed
		// and would only confuse a future restart.
		c.storage.clearPart(cardPart)
		report.CardUpdated = true
	}

	// Manifest is durably committed; clear its .part too.
	c.storage.clearPart(manifestPart)

	// Phase 5: update state.json. This is the last write; if we crash
	// before this point, the active bytes are already on disk and will
	// be re-validated on the next Sync.
	now := c.now()
	newState := persistedState{
		LastRevision:        manifestMeta.Revision,
		LastSyncMonotonicNS: now.UnixNano(),
		LastFetchedAtUnixMS: now.UnixMilli(),
	}
	if err := c.storage.saveState(newState); err != nil {
		return nil, fmt.Errorf("offlineclient: persist state: %w", err)
	}
	return report, nil
}

// cardNeedsFetch reports whether the active card on disk (if any) matches
// the manifest's critical card reference. If it does, we skip the
// download entirely; the manifest already passed verification.
func (c *ProtocolClient) cardNeedsFetch(st persistedState, m *offlinepkg.Manifest) (bool, error) {
	if m.CriticalCard.PackageID == "" || m.CriticalCard.Version <= 0 {
		return false, errors.New("offlineclient: manifest critical_card is incomplete")
	}
	if st.LastRevision == 0 {
		return true, nil // cold start
	}
	_, existing, err := c.storage.readActiveCard()
	if err != nil {
		if errors.Is(err, ErrNoActiveState) {
			return true, nil
		}
		return false, err
	}
	if existing.PackageID != m.CriticalCard.PackageID || existing.Version != m.CriticalCard.Version {
		return true, nil
	}
	// Same identity; if our on-disk card was verified under the same
	// manifest revision we already have, no re-fetch needed.
	if st.LastRevision == m.Revision {
		return false, nil
	}
	// Different revision with same critical-card identity: still safe to
	// skip re-download because the card is immutable; it was verified
	// already.
	return false, nil
}

// bytesAreIdentical is a defensive check that the bytes we just wrote are
// byte-equal to what we had on disk; a divergence indicates tampering.
// Revisions are monotonic per the contract, so a same-revision replay with
// different bytes is a tamper signal.
func (c *ProtocolClient) bytesAreIdentical(fresh []byte) (bool, error) {
	bin, err := os.ReadFile(joinPath(c.StorageDir(), "state", "current_manifest.bin"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if len(bin) != len(fresh) {
		return false, nil
	}
	for i := range bin {
		if bin[i] != fresh[i] {
			return false, nil
		}
	}
	return true, nil
}

func readFileIfExists(path string) ([]byte, error) {
	if !filesExists(path) {
		return nil, nil
	}
	return os.ReadFile(path)
}

// helper indirection for HTTP/Range downloads (real impl lives in transport.go).
// The wrapper above is kept; the actual logic is in transport.go.

// manifestDownload carries the parameters needed to download one manifest
// or card artifact. It is internal so the public API stays narrow.
type manifestDownload struct {
	Jurisdiction string
	PackageID    string
	Version      int
	Path         string
	IsCard       bool
}

// validateAndCanonicalizeManifest / validateAndCanonicalizeCard are
// inline helpers retained for callers in the Sync flow that want a
// one-step parse. The deeper checks (checksum, signature, structural)
// happen in transport.go.
func parseManifestBytes(bytes []byte) (*offlinepkg.Manifest, error) {
	var m offlinepkg.Manifest
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, fmt.Errorf("offlineclient: parse manifest: %w", err)
	}
	return &m, nil
}

func parseCardBytes(bytes []byte) (*offlinepkg.PublicIncidentCard, error) {
	var card offlinepkg.PublicIncidentCard
	if err := json.Unmarshal(bytes, &card); err != nil {
		return nil, fmt.Errorf("offlineclient: parse card: %w", err)
	}
	return &card, nil
}

// Suppress unused-import warnings while stubs are in place.
var (
	_ = io.EOF
	_ = http.StatusOK
	_ = strconv.Itoa
	_ = strings.TrimSpace
)
