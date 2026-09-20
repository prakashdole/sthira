package offlinedelivery

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type cachedItem[T any] struct {
	value     T
	expiresAt time.Time
}

// MemoryCache protects upstream PublicationSource from redundant queries
// under concurrent citizen download spikes.
type MemoryCache struct {
	mu        sync.RWMutex
	manifests map[string]cachedItem[*ManifestRecord]
	cards     map[string]cachedItem[*CardRecord]
	now       func() time.Time
}

// NewMemoryCache creates an in-memory cache.
func NewMemoryCache(now func() time.Time) *MemoryCache {
	if now == nil {
		now = time.Now
	}
	return &MemoryCache{
		manifests: make(map[string]cachedItem[*ManifestRecord]),
		cards:     make(map[string]cachedItem[*CardRecord]),
		now:       now,
	}
}

// GetManifest returns the cached manifest if present and unexpired.
func (c *MemoryCache) GetManifest(jurisdiction string) (*ManifestRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.manifests[jurisdiction]
	if !ok || c.now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

// PutManifest caches the manifest record for ttl duration.
func (c *MemoryCache) PutManifest(jurisdiction string, record *ManifestRecord, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.manifests[jurisdiction] = cachedItem[*ManifestRecord]{
		value:     record,
		expiresAt: c.now().Add(ttl),
	}
}

// GetCard returns the cached card if present and unexpired.
func (c *MemoryCache) GetCard(packageID string, version int) (*CardRecord, bool) {
	key := fmt.Sprintf("%s:%d", packageID, version)
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.cards[key]
	if !ok || c.now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

// PutCard caches the card record for ttl duration.
func (c *MemoryCache) PutCard(packageID string, version int, record *CardRecord, ttl time.Duration) {
	key := fmt.Sprintf("%s:%d", packageID, version)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cards[key] = cachedItem[*CardRecord]{
		value:     record,
		expiresAt: c.now().Add(ttl),
	}
}

// InvalidateJurisdiction clears any cached manifest for a jurisdiction.
func (c *MemoryCache) InvalidateJurisdiction(jurisdiction string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.manifests, jurisdiction)
}

type call[T any] struct {
	wg  sync.WaitGroup
	val T
	err error
}

type callGroup[T any] struct {
	mu sync.Mutex
	m  map[string]*call[T]
}

func (g *callGroup[T]) Do(key string, fn func() (T, error)) (T, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call[T])
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(call[T])
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}

// CachedSource wraps any PublicationSource with in-memory caching and
// request coalescing to shield upstream sources from concurrent bursts.
type CachedSource struct {
	inner          PublicationSource
	cache          *MemoryCache
	cfg            Config
	manifestFlight callGroup[*ManifestRecord]
	cardFlight     callGroup[*CardRecord]
}

// NewCachedSource creates a cached publication source.
func NewCachedSource(inner PublicationSource, cfg Config, now func() time.Time) *CachedSource {
	return &CachedSource{
		inner: inner,
		cache: NewMemoryCache(now),
		cfg:   cfg,
	}
}

// GetManifest checks cache before calling upstream, coalescing concurrent misses.
func (s *CachedSource) GetManifest(ctx context.Context, jurisdiction string) (*ManifestRecord, error) {
	if rec, hit := s.cache.GetManifest(jurisdiction); hit {
		return rec, nil
	}
	return s.manifestFlight.Do(jurisdiction, func() (*ManifestRecord, error) {
		if rec, hit := s.cache.GetManifest(jurisdiction); hit {
			return rec, nil
		}
		rec, err := s.inner.GetManifest(ctx, jurisdiction)
		if err != nil {
			return nil, err
		}
		if rec != nil && s.cfg.ManifestCacheTTL > 0 {
			s.cache.PutManifest(jurisdiction, rec, s.cfg.ManifestCacheTTL)
		}
		return rec, nil
	})
}

// GetCard checks cache before calling upstream, coalescing concurrent misses.
func (s *CachedSource) GetCard(ctx context.Context, packageID string, version int) (*CardRecord, error) {
	if rec, hit := s.cache.GetCard(packageID, version); hit {
		return rec, nil
	}
	key := fmt.Sprintf("%s:%d", packageID, version)
	return s.cardFlight.Do(key, func() (*CardRecord, error) {
		if rec, hit := s.cache.GetCard(packageID, version); hit {
			return rec, nil
		}
		rec, err := s.inner.GetCard(ctx, packageID, version)
		if err != nil {
			return nil, err
		}
		if rec != nil && s.cfg.CardCacheTTL > 0 {
			s.cache.PutCard(packageID, version, rec, s.cfg.CardCacheTTL)
		}
		return rec, nil
	})
}

// GetResource passes directly to inner source, as resource streams are seekable readers.
func (s *CachedSource) GetResource(ctx context.Context, resourceID string) (*ResourceContent, error) {
	return s.inner.GetResource(ctx, resourceID)
}
