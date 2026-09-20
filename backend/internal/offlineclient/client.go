package offlineclient

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"sthira/backend/internal/offlinepkg"
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

	// Now returns the current wall-clock time. Expiry decisions use
	// time.Since on the monotonic clock of the returned time.Time value,
	// never the wall value, so wall-clock rollback cannot extend
	// validity. nil falls back to time.Now.
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
}

// ProtocolClient is the disk-backed test/reference harness for the P5
// offline protocol. It is goroutine-safe for read methods
// (GetActiveCard, IsRouteCancelled, HasResource) and not safe for
// concurrent Sync/DownloadResource calls on the same instance.
type ProtocolClient struct {
	storage          *storage
	trust            offlinepkg.TrustStore
	now              func() time.Time
	http             *http.Client
	baseURL          string
	maxRetries       int
	maxResourceBytes int64
	staleBefore      time.Duration
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

	st, err := newStorage(cfg.StorageDir)
	if err != nil {
		return nil, err
	}
	return &ProtocolClient{
		storage:          st,
		trust:            cfg.TrustStore,
		now:              now,
		http:             httpClient,
		baseURL:          cfg.BaseURL,
		maxRetries:       maxRetries,
		maxResourceBytes: maxRes,
		staleBefore:      time.Duration(staleBefore) * time.Second,
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
	_, rec, err := c.storage.readActiveManifest()
	if err != nil {
		return nil, err
	}
	return &rec.Manifest, nil
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
