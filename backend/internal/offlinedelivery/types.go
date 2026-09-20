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
