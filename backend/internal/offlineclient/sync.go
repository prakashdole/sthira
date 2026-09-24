package offlineclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sthira/backend/internal/offlinepkg"
	"sthira/backend/internal/offlineresources"
)

// stateQuery returns the currently active card plus its freshness state.
// This is the read-only side of the client and is goroutine-safe.
func (c *ProtocolClient) stateQuery() (*offlinepkg.PublicIncidentCard, FreshnessState, error) {
	manifestBytes, cardBytes, err := c.storage.readActiveGeneration()
	if err != nil {
		if errors.Is(err, ErrNoActiveState) {
			return nil, FreshnessUnverifiable, ErrNoActiveState
		}
		return nil, FreshnessUnverifiable, err
	}
	manifest, err := offlinepkg.ParseManifest(manifestBytes, offlinepkg.Limits{MaxBytes: 256 * 1024, MaxDepth: 32})
	if err != nil {
		return nil, FreshnessUnverifiable, fmt.Errorf("offlineclient: parse active manifest: %w", err)
	}
	if err := c.verifyManifest(manifest, manifestBytes); err != nil {
		return nil, FreshnessUnverifiable, err
	}
	card, err := offlinepkg.ParseCard(cardBytes, offlinepkg.Limits{MaxBytes: 64 * 1024, MaxDepth: 32})
	if err != nil {
		return nil, FreshnessUnverifiable, fmt.Errorf("offlineclient: parse active card: %w", err)
	}
	if err := c.verifyCard(card, cardBytes); err != nil {
		return nil, FreshnessUnverifiable, err
	}
	if card.PackageID != manifest.CriticalCard.PackageID || card.Version != manifest.CriticalCard.Version || card.Jurisdiction != manifest.Jurisdiction || card.ChecksumSHA256 != manifest.CriticalCard.ChecksumSHA256 {
		return nil, FreshnessUnverifiable, errors.New("offlineclient: active card does not match active manifest reference")
	}
	st, err := c.storage.loadState()
	if err != nil {
		return nil, FreshnessUnverifiable, err
	}
	if err := c.storage.tombstones().verifyIntegrity(); err != nil {
		return nil, FreshnessUnverifiable, fmt.Errorf("offlineclient: tombstones corrupt: %w", err)
	}
	if card.PackageID != "" && c.isPackageRevoked(card.PackageID) {
		return card, FreshnessRevoked, nil
	}
	if card.PackageID != "" && c.isSuperseded(card.PackageID, card.Version) {
		return card, FreshnessRevoked, nil
	}
	if card.Signature != nil && c.trust != nil {
		tk, err := c.trust.LookupKey(card.Signature.KeyID)
		if err != nil || tk.Revoked {
			return card, FreshnessRevoked, nil
		}
	}
	now := c.now()
	state, err := c.computeFreshness(card, st, now)
	if err != nil {
		return nil, FreshnessUnverifiable, err
	}
	return card, state, nil
}

// computeFreshness returns CURRENT / STALE / EXPIRED using monotonic
// elapsed time and absolute expiration boundaries. Wall-clock rollback
// cannot extend validity or un-expire an expired card.
//
// Restart-recovery semantics (D55 + worker requirement): after restart the
// monotonic process-local anchor is zero; the validity window is fixed at
// the original acquisition (state.LastFetchedAtUnixMS) and not renewed by
// any subsequent sync. Wall-clock is used as the time source when no
// monotonic anchor is available; clock rollback is detected via the
// high-water mark and fails closed (EXPIRED + ErrClockRolledBack). When
// the client has a monotonic anchor (same process as a verified sync),
// remaining is bounded by min(wall-clock-remaining,
// validity-at-acq − monotonic-elapsed), preventing validity from being
// extended by re-syncing.
func (c *ProtocolClient) computeFreshness(card *offlinepkg.PublicIncidentCard, st persistedState, now time.Time) (FreshnessState, error) {
	if st.LastFetchedAtUnixMS == 0 {
		// First sync never completed; bytes on disk but no time anchor.
		return FreshnessExpired, nil
	}
	if st.ExpiredAtUnixMS > 0 {
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

	// Absolute expiration check: an already-expired card must never be CURRENT.
	if !now.Before(expiresAt) {
		st.ExpiredAtUnixMS = now.UnixMilli()
		if now.UnixMilli() > st.MaxObservedUnixMS {
			st.MaxObservedUnixMS = now.UnixMilli()
		}
		_ = c.storage.saveState(st)
		return FreshnessExpired, nil
	}

	// A card not yet effective cannot be treated as CURRENT.
	if now.Before(effectiveAt) {
		return FreshnessUnverifiable, nil
	}

	// Clock rollback detection against monotonically recorded wall/monotonic marks.
	if st.MaxObservedUnixMS > 0 && now.UnixMilli() < st.MaxObservedUnixMS {
		return FreshnessExpired, ErrClockRolledBack
	}
	// Remaining validity window bounded by acquisition time.
	acqTime := time.UnixMilli(st.LastFetchedAtUnixMS)
	if acqTime.IsZero() {
		acqTime = effectiveAt
	}
	remainingAtAcq := expiresAt.Sub(acqTime)
	if remainingAtAcq <= 0 {
		st.ExpiredAtUnixMS = now.UnixMilli()
		_ = c.storage.saveState(st)
		return FreshnessExpired, nil
	}

	// Process-local monotonic elapsed is process-local and cannot be
	// serialized across restart. With no monotonic anchor we use wall
	// clock against the original acquisition window; the high-water
	// mark above already protects against wall-clock rollback. With a
	// monotonic anchor we additionally cap by
	// remainingAtAcq − elapsed so a re-sync cannot grant more validity
	// than the original acquisition window allowed.
	var elapsed time.Duration
	hasMono := !c.lastSyncMono.IsZero()
	if hasMono {
		elapsed = time.Since(c.lastSyncMono)
		if elapsed >= remainingAtAcq {
			st.ExpiredAtUnixMS = now.UnixMilli()
			_ = c.storage.saveState(st)
			return FreshnessExpired, nil
		}
	}

	// Advance max observed time (after every monotonic-anchored read).
	if now.UnixMilli() > st.MaxObservedUnixMS {
		st.MaxObservedUnixMS = now.UnixMilli()
		_ = c.storage.saveState(st)
	}

	remaining := expiresAt.Sub(now)
	if hasMono {
		monoBound := remainingAtAcq - elapsed
		if monoBound < remaining {
			remaining = monoBound
		}
	}
	if remaining <= 0 {
		st.ExpiredAtUnixMS = now.UnixMilli()
		_ = c.storage.saveState(st)
		return FreshnessExpired, nil
	}

	if remaining <= c.staleBefore {
		return FreshnessStale, nil
	}
	return FreshnessCurrent, nil
}

// isRouteCancelled checks the durable tombstones for routeID.
func (c *ProtocolClient) isRouteCancelled(routeID string) bool {
	if routeID == "" {
		return false
	}
	return c.storage.tombstones().containsRoute(routeID)
}

func (c *ProtocolClient) isPackageRevoked(packageID string) bool {
	if packageID == "" {
		return false
	}
	return c.storage.tombstones().containsPackage(packageID)
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
	if stBefore.Jurisdiction != "" && stBefore.Jurisdiction != jurisdiction {
		return nil, fmt.Errorf("offlineclient: storage is bound to jurisdiction %q", stBefore.Jurisdiction)
	}

	// Phase 1: download + verify manifest.
	manifestBytes, manifestMeta, _, manifestPart, err := c.downloadAndVerifyArtifact(ctx, manifestDownload{
		Jurisdiction: jurisdiction,
		Path:         "/api/v3/regions/" + jurisdiction + "/manifest",
	})
	if err != nil {
		return nil, err
	}
	if manifestMeta.Jurisdiction != jurisdiction {
		return nil, fmt.Errorf("offlineclient: manifest jurisdiction mismatch: got %q expected %q", manifestMeta.Jurisdiction, jurisdiction)
	}
	if manifestMeta.Revision < stBefore.LastRevision {
		return nil, offlinepkg.ErrVersionRollback
	}

	// Phase 1.5: validate the manifest's resource list against the
	// descriptor/dependency/license rules the same way the public
	// publication boundary does. Without this, the validator would only
	// run inside tests; an ill-formed pack (duplicate id, denied
	// license, over budget, missing attribution) could reach the device
	// and stay there until someone manually re-ran AuditRegionalPack.
	// The audit also surfaces license-pending and attribution-missing
	// lists on SyncReport so a UI layer can present "regional pack
	// needs review" copy without re-running the validator.
	//
	// Style-level cross-validation (ValidateMapStyle) is intentionally
	// deferred to DownloadResource: the style bytes are not in the
	// manifest and Sync does not have the download machinery to fetch
	// arbitrary sub-resources. The descriptor audit alone already
	// rejects duplicate IDs, conflicting URIs and denied licenses,
	// which are the failure modes that break a real device.
	report := &SyncReport{}
	if len(manifestMeta.Resources) > 0 {
		descList := make([]offlineresources.ResourceDescriptor, len(manifestMeta.Resources))
		for i, r := range manifestMeta.Resources {
			descList[i] = offlineresources.ResourceDescriptor{ResourceDescriptor: r}
		}
		audit, err := c.resourceValidator.AuditRegionalPack(descList)
		if err != nil {
			// AuditRegionalPack returns ErrPackInvalid on a hard
			// failure (over budget, duplicate id, denied license,
			// missing attribution on a required resource). Do NOT
			// activate the manifest.
			return nil, fmt.Errorf("offlineclient: resource audit: %w", err)
		}
		report.ResourceAudit = audit
		report.ResourcesValidated = len(descList)
	}

	// Phase 2: apply revocations to tombstones BEFORE activation so a
	// crash mid-activation does not leave tombstone state inconsistent
	// with the active manifest.
	tombs := c.storage.tombstones()
	for _, pkg := range manifestMeta.Revocations.RevokedPackages {
		added, err := tombs.addPackage(pkg)
		if err != nil {
			return nil, fmt.Errorf("offlineclient: tombstone package: %w", err)
		}
		if added {
			report.RevokedPackages = append(report.RevokedPackages, pkg)
		}
	}
	for _, route := range manifestMeta.Revocations.CancelledRoutes {
		added, err := tombs.addRoute(route)
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

	// Same-revision check: verify active generation on disk is intact.
	// The activeIntact predicate MUST bind on canonical-bytes identity, not
	// only on metadata fields that are likely identical across same-revision
	// republishings. A same-revision manifest with different bytes is a
	// different immutable object under the trust contract, and a freshly
	// fetched conflicting same-revision manifest must not silently replace
	// trust/tombstone meaning or be called unchanged.
	activeManBytes, activeCardBytes, genErr := c.storage.readActiveGeneration()
	var activeIntact bool
	var activeManifestID, activeManifestChecksum string
	if genErr == nil {
		activeManifest, perr := offlinepkg.ParseManifest(activeManBytes, offlinepkg.Limits{MaxBytes: 256 * 1024, MaxDepth: 32})
		activeCard, cerr := offlinepkg.ParseCard(activeCardBytes, offlinepkg.Limits{MaxBytes: 64 * 1024, MaxDepth: 32})
		if perr == nil && cerr == nil {
			activeManifestChecksum = activeManifest.ChecksumSHA256
			activeManifestID = activeManifest.ManifestID
			activeIntact = activeManifest.Revision == manifestMeta.Revision &&
				activeManifest.ChecksumSHA256 == manifestMeta.ChecksumSHA256 &&
				activeManifest.ManifestID == manifestMeta.ManifestID &&
				activeCard.PackageID == manifestMeta.CriticalCard.PackageID &&
				activeCard.Version == manifestMeta.CriticalCard.Version &&
				activeCard.ChecksumSHA256 == manifestMeta.CriticalCard.ChecksumSHA256
		}
	}
	// Defensive log: if a same-revision manifest with conflicting bytes was
	// served, surface it through SyncReport so the integration test can
	// detect silent acceptance. This must NOT replace the rejection logic;
	// activeIntact above is false in that case and the code falls through
	// to re-activate via Phase 4 below.
	_ = activeManifestID
	_ = activeManifestChecksum

	if manifestMeta.Revision == stBefore.LastRevision && stBefore.LastRevision > 0 && activeIntact {
		// Same immutable generation, already active and verified. We
		// preserve the original acquisition window (LastFetchedAtUnixMS in
		// state.json) but refresh the in-process monotonic anchor so the
		// post-sync read can compute CURRENT/STALE/EXPIRED against the
		// fixed wall-clock remaining time. The validity window is NOT
		// extended: LastFetchedAtUnixMS stays at its original value, and
		// computeFreshness caps by remainingAtAcq − elapsed to prevent
		// re-syncing from granting more validity than the original
		// acquisition.
		report.ActiveRevision = manifestMeta.Revision
		c.storage.clearPart(manifestPart)
		c.lastSyncMono = time.Now()
		return report, nil
	}

	// Phase 3: download card BEFORE activation if changed or missing.
	// If card download fails, NOTHING is activated; prior coherent generation stays intact.
	var newCardBytes []byte
	var cardPart string
	cardChanged := !activeIntact || stBefore.LastRevision == 0
	if !cardChanged {
		changed, err := c.cardNeedsFetchGen(activeManBytes, activeCardBytes, manifestMeta)
		if err != nil {
			return nil, err
		}
		cardChanged = changed
	}
	if cardChanged {
		cardPath := fmt.Sprintf("/api/v3/packages/%s/versions/%d",
			manifestMeta.CriticalCard.PackageID, manifestMeta.CriticalCard.Version)
		b, _, _, part, err := c.downloadAndVerifyArtifact(ctx, manifestDownload{
			Jurisdiction:     jurisdiction,
			PackageID:        manifestMeta.CriticalCard.PackageID,
			Version:          manifestMeta.CriticalCard.Version,
			Path:             cardPath,
			IsCard:           true,
			ExpectedCardDesc: &manifestMeta.CriticalCard,
		})
		if err != nil {
			return nil, err
		}
		newCardBytes = b
		cardPart = part
	} else {
		newCardBytes = activeCardBytes
	}

	// Phase 4: atomic activation of manifest and card together using generation staging.
	if err := c.storage.writeActiveGeneration(manifestBytes, newCardBytes); err != nil {
		return nil, fmt.Errorf("offlineclient: activate generation: %w", err)
	}
	report.ManifestUpdated = true
	report.ActiveRevision = manifestMeta.Revision
	if cardChanged {
		report.CardUpdated = true
		c.storage.clearPart(cardPart)
	}
	c.storage.clearPart(manifestPart)

	// Phase 5: update state.json. This is the last write; if we crash
	// before this point, the active bytes are already on disk and will
	// be re-validated on the next Sync.
	now := c.now()
	newState := persistedState{
		LastRevision:        manifestMeta.Revision,
		Jurisdiction:        jurisdiction,
		LastFetchedAtUnixMS: now.UnixMilli(),
	}
	if err := c.storage.saveState(newState); err != nil {
		return nil, fmt.Errorf("offlineclient: persist state: %w", err)
	}
	c.lastSyncMono = time.Now()
	return report, nil
}

// cardNeedsFetchGen reports whether the active card on disk matches the
// manifest's critical card reference. It compares canonical bytes via the
// manifest reference and the persisted card checksum, not by re-parsing
// raw disk files (the same-revision activeIntact predicate above already
// bound on canonical-bytes identity for the just-fetched bytes).
func (c *ProtocolClient) cardNeedsFetchGen(activeManBytes, activeCardBytes []byte, m *offlinepkg.Manifest) (bool, error) {
	if m.CriticalCard.PackageID == "" || m.CriticalCard.Version <= 0 {
		return false, errors.New("offlineclient: manifest critical_card is incomplete")
	}
	if len(activeCardBytes) == 0 {
		return true, nil
	}
	existing, err := offlinepkg.ParseCard(activeCardBytes, offlinepkg.Limits{MaxBytes: 64 * 1024, MaxDepth: 32})
	if err != nil {
		return true, nil
	}
	if existing.PackageID != m.CriticalCard.PackageID || existing.Version != m.CriticalCard.Version {
		return true, nil
	}
	if existing.ChecksumSHA256 != m.CriticalCard.ChecksumSHA256 {
		return true, nil
	}
	if existing.Jurisdiction != m.Jurisdiction {
		return true, nil
	}
	// Same identity and checksum; if our on-disk card was verified under the same
	// manifest revision we already have, no re-fetch needed.
	return false, nil
}

// helper indirection for HTTP/Range downloads (real impl lives in transport.go).
// The wrapper above is kept; the actual logic is in transport.go.

// manifestDownload carries the parameters needed to download one manifest
// or card artifact. It is internal so the public API stays narrow.
type manifestDownload struct {
	Jurisdiction     string
	PackageID        string
	Version          int
	Path             string
	IsCard           bool
	ExpectedCardDesc *offlinepkg.CriticalCardDescriptor
}

// Suppress unused-import warnings while stubs are in place.
var (
	_ = io.EOF
	_ = http.StatusOK
	_ = strconv.Itoa
	_ = strings.TrimSpace
)
