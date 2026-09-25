package offlinedelivery

import (
	"context"
	"errors"
	"io"
	"time"
)

// Sentinel errors returned by PublicationSource and handlers.
var (
	ErrNotFound          = errors.New("offlinedelivery: requested item not found")
	ErrQuarantined       = errors.New("offlinedelivery: requested item is quarantined")
	ErrInvalidIdentifier = errors.New("offlinedelivery: invalid or unsafe identifier")
	ErrStreamAborted     = errors.New("offlinedelivery: content stream aborted")
)

// ResourceContent provides a readable, seekable stream of an immutable resource asset.
// The caller or handler must close Reader when finished.
type ResourceContent struct {
	Reader         io.ReadSeekCloser
	ContentType    string
	ContentLength  int64
	ChecksumSHA256 string
	ETag           string
}

// ManifestRecord carries the public regional manifest payload and its integrity checksum.
type ManifestRecord struct {
	Jurisdiction   string
	Revision       int
	RawJSON        []byte
	ChecksumSHA256 string
	SourceStatus   string // e.g. "CURRENT", "STALE"
}

// CardRecord carries the public incident card payload and its integrity checksum.
type CardRecord struct {
	PackageID      string
	Version        int
	RawJSON        []byte
	ChecksumSHA256 string
	SourceStatus   string // e.g. "CURRENT", "STALE"
}

// PublicationSource is the minimal source seam defined by plan/p5-contract.md.
// It decouples HTTP delivery from the persistence layer and from Agent 1's signature engine.
type PublicationSource interface {
	// GetManifest returns the current published regional manifest for a jurisdiction.
	GetManifest(ctx context.Context, jurisdiction string) (*ManifestRecord, error)

	// GetCard returns the published public incident card for a package version.
	GetCard(ctx context.Context, packageID string, version int) (*CardRecord, error)

	// GetResource returns an open seekable stream for a content-addressed resource asset.
	GetResource(ctx context.Context, resourceID string) (*ResourceContent, error)
}

// PublicationLifecycleObserver is the seam the trusted lifecycle path
// (operator publish/promote/withdraw/quarantine/supersede handlers)
// uses to invalidate cached deliveries across instances under an
// explicit consistency bound. Implementations may fan out to a
// shared notification channel (e.g. in-process bus today; pubsub
// later) and to the local process-local cache. Production
// implementation MUST honor at-least-once semantics within the
// bounded window declared in plan/parameters.md; this seam is the
// place that bound lives, not in the handler.
type PublicationLifecycleObserver interface {
	// OnManifestWithdrawn invalidates cached manifests for the given
	// jurisdiction. Called by the trusted withdraw lifecycle path
	// after the row state is committed.
	OnManifestWithdrawn(ctx context.Context, jurisdiction string, revision int)
	// OnManifestPromoted invalidates any older-cached manifest for
	// the jurisdiction so a superseded version stops serving.
	OnManifestPromoted(ctx context.Context, jurisdiction string, revision int)
	// OnSourceWithdrawn invalidates ALL cached manifest/card for
	// every jurisdiction the source was authorized in, plus any
	// active CachedSource reads in flight.
	OnSourceWithdrawn(ctx context.Context, sourceID string)
	// OnSourceQuarantined invalidates cached manifest/card as for
	// withdrawal.
	OnSourceQuarantined(ctx context.Context, sourceID string)
	// OnPackageSuperseded invalidates every cached card for the
	// superseded versions; the new version is left cached.
	OnPackageSuperseded(ctx context.Context, packageID string, supersededVersions []int)
}

// MultiObserver fans a lifecycle event out to several observers. The
// first observer is treated as authoritative for the local process;
// later observers are best-effort within the same bounded deadline.
// Fan-out ordering is preserved; observers MUST NOT block each
// other past the deadline.
type MultiObserver struct {
	observers []PublicationLifecycleObserver
}

// NewMultiObserver constructs a multi-observer with the given fan-out
// members. nil members are skipped.
func NewMultiObserver(observers ...PublicationLifecycleObserver) *MultiObserver {
	out := make([]PublicationLifecycleObserver, 0, len(observers))
	for _, o := range observers {
		if o != nil {
			out = append(out, o)
		}
	}
	return &MultiObserver{observers: out}
}

func (m *MultiObserver) fanout(event string, fn func(o PublicationLifecycleObserver) error) {
	if m == nil {
		return
	}
	for _, o := range m.observers {
		_ = fn(o) // best effort within the lifecycle deadline
	}
}

func (m *MultiObserver) OnManifestWithdrawn(ctx context.Context, jurisdiction string, revision int) {
	m.fanout("manifest-withdrawn", func(o PublicationLifecycleObserver) error {
		o.OnManifestWithdrawn(ctx, jurisdiction, revision)
		return nil
	})
}

func (m *MultiObserver) OnManifestPromoted(ctx context.Context, jurisdiction string, revision int) {
	m.fanout("manifest-promoted", func(o PublicationLifecycleObserver) error {
		o.OnManifestPromoted(ctx, jurisdiction, revision)
		return nil
	})
}

func (m *MultiObserver) OnSourceWithdrawn(ctx context.Context, sourceID string) {
	m.fanout("source-withdrawn", func(o PublicationLifecycleObserver) error {
		o.OnSourceWithdrawn(ctx, sourceID)
		return nil
	})
}

func (m *MultiObserver) OnSourceQuarantined(ctx context.Context, sourceID string) {
	m.fanout("source-quarantined", func(o PublicationLifecycleObserver) error {
		o.OnSourceQuarantined(ctx, sourceID)
		return nil
	})
}

func (m *MultiObserver) OnPackageSuperseded(ctx context.Context, packageID string, supersededVersions []int) {
	m.fanout("package-superseded", func(o PublicationLifecycleObserver) error {
		o.OnPackageSuperseded(ctx, packageID, supersededVersions)
		return nil
	})
}

// Config tunes delivery timeouts, limits, and caching behavior.
type Config struct {
	// MaxCardBytes bounds the maximum raw card response size (default 1 MiB).
	MaxCardBytes int64

	// MaxManifestBytes bounds the maximum raw manifest response size (default 256 KiB).
	MaxManifestBytes int64

	// MaxResourceBytes bounds the maximum asset size served (default 100 MiB).
	MaxResourceBytes int64

	// StreamChunkSize is the buffer size used for streaming Range responses (default 32 KiB).
	StreamChunkSize int

	// ManifestCacheTTL is how long manifests are cached in memory to protect upstream (default 10s).
	ManifestCacheTTL time.Duration

	// CardCacheTTL is how long immutable cards are cached in memory (default 10m).
	CardCacheTTL time.Duration
}

// DefaultConfig returns safe, bounded production defaults.
func DefaultConfig() Config {
	return Config{
		MaxCardBytes:     1 << 20,   // 1 MiB
		MaxManifestBytes: 256 << 10, // 256 KiB
		MaxResourceBytes: 100 << 20, // 100 MiB
		StreamChunkSize:  32 << 10,  // 32 KiB
		ManifestCacheTTL: 10 * time.Second,
		CardCacheTTL:     10 * time.Minute,
	}
}
