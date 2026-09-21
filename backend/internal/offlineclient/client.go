package offlineclient

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"sthira/backend/internal/offlinepkg"
	"sthira/backend/internal/offlineresources"
)

// FreshnessState describes the client's view of an artifact's currentness.
// It is the explicit answer to "what can I trust right now?" — UNVERIFIABLE
// is a first-class value distinct from CURRENT and from EXPIRED.
type FreshnessState string

const (
	FreshnessCurrent      FreshnessState = "CURRENT"
	FreshnessStale        FreshnessState = "STALE"
	FreshnessExpired      FreshnessState = "EXPIRED"
	FreshnessRevoked      FreshnessState = "REVOKED"
	FreshnessUnverifiable FreshnessState = "UNVERIFIABLE"
)

// ResourceFreshnessState is the per-resource view of currentness. The
// critical card has its own freshness (FreshnessState above); a regional
// map pack has its own clock keyed on FetchedAtUnixMS. The two can drift:
// a CURRENT card may sit on top of a STALE map pack after the user has
// kept the device offline for a long time, and the device must NOT trust
// a STALE map pack for live routing without re-downloading.
type ResourceFreshnessState string

const (
	ResourceCurrent ResourceFreshnessState = "CURRENT"
	ResourceStale   ResourceFreshnessState = "STALE"
	ResourceMissing ResourceFreshnessState = "MISSING"
)

// ResourceLicenseStatus surfaces the coarse license/redistribution state of
// a downloaded resource. See ResourceLicenseStatus() for the rules.
type ResourceLicenseStatus string

const (
	ResourceLicensePending ResourceLicenseStatus = "LICENSE_PENDING"
	ResourceLicenseAllowed ResourceLicenseStatus = "LICENSE_ALLOWED"
	ResourceLicenseDenied  ResourceLicenseStatus = "LICENSE_DENIED"
)

// resourceMaxAgeMS is the per-resource freshness window. 24 hours is a
// deliberately conservative stand-in until operator policy supplies a
// different value: regional map data is expected to be refreshed at
// least daily. A STALE resource MUST NOT be silently treated as CURRENT
// for live routing.
const resourceMaxAgeMS int64 = 24 * 60 * 60 * 1000

// ClientConfig is the constructor input. All fields are required unless
// documented otherwise; bounds are enforced at construction time so the
// methods can assume valid configuration.
type ClientConfig struct {
	// BaseURL is the origin that serves the public delivery endpoints
	// (e.g. "https://example.test"). The client appends path components;
	// trailing slash is normalized.
	BaseURL string

	// StorageDir is the directory the client owns. It is created if
	// absent. The client never writes outside this directory.
	StorageDir string

	// HTTPClient is the transport used for all fetches. Tests inject a
	// client whose Transport points at an httptest.Server. nil falls
	// back to http.DefaultClient with a bounded timeout.
	HTTPClient *http.Client

	// TrustStore is the verification seam. Production wires Agent 1's
	// real offlinepkg.TrustStore; tests inject a labelled fake.
	TrustStore offlinepkg.TrustStore

	// Now returns the current wall-clock time for absolute validity checks.
	// Process-local elapsed time is taken separately from time.Since; it is
	// never serialized. nil falls back to time.Now.
	Now func() time.Time

	// MaxRetries bounds transient transport failures per request
	// (default 3). Verification failures are not retried.
	MaxRetries int

	// MaxResourceBytes is the per-artifact byte ceiling for resources
	// (default 100 MiB). Manifest and card have separate, smaller
	// ceilings enforced internally.
	MaxResourceBytes int64

	// StaleBeforeSeconds is how close to expiry (in seconds) a card is
	// still considered CURRENT rather than STALE. Default 300s (5 min)
	// gives the UI a chance to refresh before the artifact actually
	// expires.
	StaleBeforeSeconds int

	// ResourceValidator is the seam declared in plan/p5-contract.md §7.4
	// (see offlineresources.Validator). Sync invokes it on every
	// manifest's resource list so descriptor/dependency/license checks
	// run against the SAME descriptor set the client is about to
	// activate, not as a separate step only tests exercise. nil falls
	// back to offlineresources.NewValidator() with the default 50 MiB
	// regional-pack budget.
	//
	// The default validator is stateless and safe for concurrent use;
	// production wires a single instance across all client sessions.
	ResourceValidator offlineresources.ResourceValidator
}

// ProtocolClient is the disk-backed test/reference harness for the P5
// offline protocol. It is goroutine-safe for read methods
// (GetActiveCard, IsRouteCancelled, HasResource) and not safe for
// concurrent Sync/DownloadResource calls on the same instance.
type ProtocolClient struct {
	storage           *storage
	trust             offlinepkg.TrustStore
	now               func() time.Time
	http              *http.Client
	baseURL           string
	maxRetries        int
	maxResourceBytes  int64
	staleBefore       time.Duration
	resourceValidator offlineresources.ResourceValidator
	lastSyncMono      time.Time
}

// NewClient validates the configuration, creates the storage layout, and
// returns a ready client. It does not perform any network I/O.
func NewClient(cfg ClientConfig) (*ProtocolClient, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("offlineclient: BaseURL required")
	}
	if cfg.StorageDir == "" {
		return nil, errors.New("offlineclient: StorageDir required")
	}
	if cfg.TrustStore == nil {
		return nil, errors.New("offlineclient: TrustStore required")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	maxRes := cfg.MaxResourceBytes
	if maxRes <= 0 {
		maxRes = 100 * 1024 * 1024 // 100 MiB
	}
	staleBefore := cfg.StaleBeforeSeconds
	if staleBefore <= 0 {
		staleBefore = 300
	}

	resVal := cfg.ResourceValidator
	if resVal == nil {
		resVal = offlineresources.NewValidator()
	}

	st, err := newStorage(cfg.StorageDir)
	if err != nil {
		return nil, err
	}
	return &ProtocolClient{
		storage:           st,
		trust:             cfg.TrustStore,
		now:               now,
		http:              httpClient,
		baseURL:           cfg.BaseURL,
		maxRetries:        maxRetries,
		maxResourceBytes:  maxRes,
		staleBefore:       time.Duration(staleBefore) * time.Second,
		resourceValidator: resVal,
	}, nil
}

// SyncReport summarizes the result of a Sync call.
type SyncReport struct {
	// ManifestUpdated is true if a new manifest was verified and activated.
	ManifestUpdated bool
	// ActiveRevision is the now-active manifest revision.
	ActiveRevision int
	// CardUpdated is true if a new critical card was verified and activated.
	CardUpdated bool
	// BytesDownloaded is the total number of bytes pulled from the network
	// during this Sync (resumed bytes from .part are NOT counted).
	BytesDownloaded int64
	// RevokedPackages are package IDs newly tombstoned by this sync.
	RevokedPackages []string
	// CancelledRoutes are route IDs newly tombstoned by this sync.
	CancelledRoutes []string
	// Superseded records package IDs whose version was newly tombstoned.
	Superseded []offlinepkg.SupersededVersion
	// ResourceAudit is the result of AuditRegionalPack on the just-
	// verified manifest's resource list. Always populated when the
	// manifest declared at least one resource; nil for a manifest with
	// no optional resources. Sync fails-closed (does not activate the
	// manifest) when audit returns ErrPackInvalid. LicensePending and
	// AttributionMissing lists are surfaced here so a UI layer can
	// present "regional pack needs review" copy without re-running the
	// validator. LicenseDenied resources block activation entirely.
	//
	// Style-level cross-validation (ValidateMapStyle) is deferred to
	// DownloadResource: Sync does not fetch arbitrary sub-resources, so
	// the style bytes are unavailable at activation time. The
	// descriptor audit alone already rejects the structural failure
	// modes (duplicate id, conflicting URI, denied license, missing
	// attribution) that would corrupt a device.
	ResourceAudit *offlineresources.RegionalPackAudit
	// ResourcesValidated is the count of resources the validator
	// inspected on this Sync. Stable across the same revision so a
	// caller can detect a manifest that silently shrank.
	ResourcesValidated int
}

// Sync downloads, verifies, and atomically activates the latest manifest
// for the given jurisdiction. If a critical card is changed, it is fetched,
// verified and activated too. Revocations are applied to the durable
// tombstone store before activation.
//
// If a previous Sync was interrupted, .part files persist on disk and this
// call resumes them when the server still returns the same ETag, or
// restarts them when the ETag has drifted.
//
// Sync is not safe to call concurrently with DownloadResource on the same
// ProtocolClient instance.
func (c *ProtocolClient) Sync(ctx context.Context, jurisdiction string) (*SyncReport, error) {
	return c.sync(ctx, jurisdiction)
}

// GetActiveCard returns the currently active public incident card and its
// freshness state. ErrNoActiveState is returned when there is no verified
// active state (cold start, lost storage, interrupted activation). The
// returned freshness is REVOKED if the card's package is in the durable
// tombstone store.
func (c *ProtocolClient) GetActiveCard() (*offlinepkg.PublicIncidentCard, FreshnessState, error) {
	return c.stateQuery()
}

// GetActiveManifest returns the currently active manifest.
func (c *ProtocolClient) GetActiveManifest() (*offlinepkg.Manifest, error) {
	manifestBytes, _, err := c.storage.readActiveGeneration()
	if err != nil {
		return nil, err
	}
	return offlinepkg.ParseManifest(manifestBytes, offlinepkg.Limits{MaxBytes: 256 * 1024, MaxDepth: 32})
}

// IsRouteCancelled reports whether the named route ID has been tombstoned
// by a verified manifest's revocations block.
func (c *ProtocolClient) IsRouteCancelled(routeID string) bool {
	return c.isRouteCancelled(routeID)
}

// HasResource reports whether the named resource is on disk with a
// matching content digest.
func (c *ProtocolClient) HasResource(resourceID string) (bool, error) {
	return c.hasResource(resourceID)
}

// DownloadResource fetches the resource described by desc, verifies its
// declared checksum, and atomically swaps it into resources/<id>/content.bin.
// If resume is true and a matching .part exists, the download is resumed
// from the last written byte. On verification failure the .part is removed
// and the previous resource (if any) is untouched.
func (c *ProtocolClient) DownloadResource(ctx context.Context, desc offlinepkg.ResourceDescriptor, resume bool) error {
	return c.downloadResource(ctx, desc, resume)
}

// ResourceFreshness reports the freshness state of a downloaded resource
// based on its on-disk meta:
//
//   - RESOURCE_MISSING: the resource is not on disk or has no recorded
//     checksum.
//   - RESOURCE_CURRENT: the resource is on disk with a matching digest
//     and was fetched within ResourceMaxAge.
//   - RESOURCE_STALE: the resource is on disk with a matching digest but
//     is older than ResourceMaxAge. A re-download with a fresh Sync will
//     refresh it; the device MUST NOT trust a STALE map pack for live
//     routing decisions without explicit confirmation.
//
// Resources have independent freshness from the critical card: a card can
// be CURRENT while the regional map pack is STALE, and vice versa. This
// is the "enforce independent resource freshness" half of the Area G
// acceptance matrix.
func (c *ProtocolClient) ResourceFreshness(resourceID string) (ResourceFreshnessState, error) {
	if resourceID == "" {
		return ResourceMissing, errors.New("offlineclient: resource id required")
	}
	bin, meta, err := c.storage.readResource(resourceID)
	if err != nil {
		if errors.Is(err, ErrNoActiveState) {
			return ResourceMissing, nil
		}
		return ResourceMissing, err
	}
	if meta.ChecksumSHA256 == "" {
		return ResourceMissing, nil
	}
	if got := offlinepkg.ChecksumSHA256(bin); got != meta.ChecksumSHA256 {
		return ResourceMissing, nil
	}
	if meta.FetchedAtUnixMS == 0 {
		return ResourceStale, nil
	}
	ageMS := c.now().UnixMilli() - meta.FetchedAtUnixMS
	if ageMS < 0 || ageMS > resourceMaxAgeMS {
		return ResourceStale, nil
	}
	return ResourceCurrent, nil
}

// ResourceLicenseStatus inspects the on-disk resource meta and returns a
// coarse license/redistribution status:
//
//   - RESOURCE_LICENSE_PENDING: no explicit license metadata on the
//     downloaded resource. O06 is open; the operator must supply
//     redistribution evidence before any production deployment.
//   - RESOURCE_LICENSE_ALLOWED: the descriptor carried
//     LicenseInfo.Redistribution == REDISTRIBUTION_ALLOWED with
//     DeclaredOfflineOK. The device may store and surface the resource
//     offline.
//   - RESOURCE_LICENSE_DENIED: the descriptor carried
//     LicenseInfo.Redistribution == REDISTRIBUTION_DENIED. The audit
//     refuses to publish such a resource, so it should never reach the
//     device; a DENIED status here indicates the local on-disk cache
//     predates the publication boundary check.
//
// License evidence is not yet carried by the signed v3 wire descriptor.
// O06 therefore remains pending even when bytes are locally intact; never
// infer permission merely from successful download.
func (c *ProtocolClient) ResourceLicenseStatus(resourceID string) ResourceLicenseStatus {
	if resourceID == "" {
		return ResourceLicensePending
	}
	if _, _, err := c.storage.readResource(resourceID); err != nil {
		return ResourceLicensePending
	}
	return ResourceLicensePending
}

// StorageDir returns the configured storage directory path. Tests use
// this to inspect on-disk state.
func (c *ProtocolClient) StorageDir() string {
	if c == nil || c.storage == nil {
		return ""
	}
	return c.storage.root
}

// filesExists is a small helper for tests that need to assert "this file
// is on disk now." Not part of the public API.
func filesExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// joinPath is filepath.Join wrapped so the rest of the package does not
// import path/filepath everywhere.
func joinPath(parts ...string) string {
	return filepath.Join(parts...)
}

// errors is re-exported for the package. We deliberately shadow nothing.
var _ = errors.New
