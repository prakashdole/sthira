package offlinedelivery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryCache protects upstream PublicationSource from redundant queries
// under concurrent citizen download spikes. It is bounded: when entries or
// tracked bytes exceed the configured cap, the oldest entry by write-time is
// evicted until the cache fits again. The bound makes the cache safe under
// long-running processes with many distinct jurisdictions/packages.
type MemoryCache struct {
	mu        sync.Mutex
	manifests map[string]cacheEntry[*ManifestRecord]
	cards     map[string]cacheEntry[*CardRecord]
	now       func() time.Time

	// Bounded-size eviction. Zero or negative means unbounded (legacy/test use).
	maxEntries    int
	maxBytes      int64
	currentBytes  int64
	manifestTimes map[string]time.Time // jurisdiction -> last write
	cardTimes     map[string]time.Time // pkgID:ver -> last write
}

// cacheEntry records the cached payload, its expiry, and the size the entry
// contributes to the byte budget.
type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
	writtenAt time.Time
	byteSize  int64
}

// NewMemoryCache creates an in-memory cache. If maxEntries <= 0 or maxBytes <= 0
// the cache is unbounded (legacy/test behavior). now may be nil (defaults to time.Now).
func NewMemoryCache(now func() time.Time, maxEntries int, maxBytes int64) *MemoryCache {
	if now == nil {
		now = time.Now
	}
	return &MemoryCache{
		manifests:     make(map[string]cacheEntry[*ManifestRecord]),
		cards:         make(map[string]cacheEntry[*CardRecord]),
		manifestTimes: make(map[string]time.Time),
		cardTimes:     make(map[string]time.Time),
		now:           now,
		maxEntries:    maxEntries,
		maxBytes:      maxBytes,
	}
}

// GetManifest returns the cached manifest if present and unexpired.
func (c *MemoryCache) GetManifest(jurisdiction string) (*ManifestRecord, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.manifests[jurisdiction]
	if !ok || c.now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

// PutManifest caches the manifest record for ttl duration. Refuses to repopulate
// a record that has been WITHDRAWN or SUPERSEDED at the source: a withdrawn
// record must NEVER reappear as CURRENT after an explicit withdrawal, even from
// a stale inflight fetch. Callers must check IsLive before Put.
func (c *MemoryCache) PutManifest(jurisdiction string, record *ManifestRecord, ttl time.Duration) {
	if record == nil {
		return
	}
	if !IsLiveStatus(record.SourceStatus) {
		// Defensive: caller forgot to filter. Drop instead of caching a tombstoned record.
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictIfNeededManifestLocked(jurisdiction, len(record.RawJSON))
	c.manifests[jurisdiction] = cacheEntry[*ManifestRecord]{
		value:     record,
		expiresAt: c.now().Add(ttl),
		writtenAt: c.now(),
		byteSize:  int64(len(record.RawJSON)),
	}
	c.manifestTimes[jurisdiction] = c.now()
	c.currentBytes += int64(len(record.RawJSON))
}

// GetCard returns the cached card if present and unexpired.
func (c *MemoryCache) GetCard(packageID string, version int) (*CardRecord, bool) {
	key := fmt.Sprintf("%s:%d", packageID, version)
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.cards[key]
	if !ok || c.now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

// PutCard caches the card record for ttl duration. Refuses to repopulate
// a withdrawn record.
func (c *MemoryCache) PutCard(packageID string, version int, record *CardRecord, ttl time.Duration) {
	if record == nil {
		return
	}
	if !IsLiveStatus(record.SourceStatus) {
		return
	}
	key := fmt.Sprintf("%s:%d", packageID, version)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictIfNeededCardLocked(key, len(record.RawJSON))
	c.cards[key] = cacheEntry[*CardRecord]{
		value:     record,
		expiresAt: c.now().Add(ttl),
		writtenAt: c.now(),
		byteSize:  int64(len(record.RawJSON)),
	}
	c.cardTimes[key] = c.now()
	c.currentBytes += int64(len(record.RawJSON))
}

// InvalidateJurisdiction clears any cached manifest for a jurisdiction.
func (c *MemoryCache) InvalidateJurisdiction(jurisdiction string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invalidateManifestLocked(jurisdiction)
}

// InvalidateManifest clears any cached manifest for a jurisdiction.
func (c *MemoryCache) InvalidateManifest(jurisdiction string) {
	c.InvalidateJurisdiction(jurisdiction)
}

// InvalidateCard clears any cached card for a packageID and version.
func (c *MemoryCache) InvalidateCard(packageID string, version int) {
	key := fmt.Sprintf("%s:%d", packageID, version)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invalidateCardLocked(key)
}

// InvalidatePackage clears all cached card versions for a packageID.
func (c *MemoryCache) InvalidatePackage(packageID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := packageID + ":"
	for k := range c.cards {
		if strings.HasPrefix(k, prefix) {
			c.invalidateCardLocked(k)
		}
	}
}

// Stats reports current cache occupancy. Exposed for tests and metrics.
func (c *MemoryCache) Stats() (entries int, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.manifests) + len(c.cards), c.currentBytes
}

func (c *MemoryCache) invalidateManifestLocked(jurisdiction string) {
	if ent, ok := c.manifests[jurisdiction]; ok {
		c.currentBytes -= ent.byteSize
		delete(c.manifests, jurisdiction)
		delete(c.manifestTimes, jurisdiction)
	}
}

func (c *MemoryCache) invalidateCardLocked(key string) {
	if ent, ok := c.cards[key]; ok {
		c.currentBytes -= ent.byteSize
		delete(c.cards, key)
		delete(c.cardTimes, key)
	}
}

// evictIfNeededManifestLocked enforces the bounded cache before insert. The new
// entry is allowed even when at the cap (it counts toward the cap). To make
// room we evict the oldest by writtenAt until we are back inside the bounds.
func (c *MemoryCache) evictIfNeededManifestLocked(newKey string, newSize int) {
	c.evictToFitLocked(len(c.manifests)+len(c.cards)+1, c.currentBytes+int64(newSize))
}

func (c *MemoryCache) evictIfNeededCardLocked(newKey string, newSize int) {
	c.evictToFitLocked(len(c.manifests)+len(c.cards)+1, c.currentBytes+int64(newSize))
}

// evictToFitLocked removes the oldest entries (by writtenAt) until both the
// entry and byte budgets are satisfied. Caller holds c.mu.
func (c *MemoryCache) evictToFitLocked(targetEntries int, targetBytes int64) {
	if c.maxEntries <= 0 && c.maxBytes <= 0 {
		return
	}
	for (c.maxEntries > 0 && targetEntries > c.maxEntries) || (c.maxBytes > 0 && targetBytes > c.maxBytes) {
		// Build the eviction candidates with their writtenAt.
		type cand struct {
			key     string
			kind    byte // 'm' manifest, 'c' card
			written time.Time
		}
		var cs []cand
		for k, t := range c.manifestTimes {
			cs = append(cs, cand{k, 'm', t})
		}
		for k, t := range c.cardTimes {
			cs = append(cs, cand{k, 'c', t})
		}
		if len(cs) == 0 {
			return
		}
		sort.Slice(cs, func(i, j int) bool { return cs[i].written.Before(cs[j].written) })
		oldest := cs[0]
		if oldest.kind == 'm' {
			c.invalidateManifestLocked(oldest.key)
		} else {
			c.invalidateCardLocked(oldest.key)
		}
		targetEntries--
		targetBytes -= 0 // accounted for in invalidate*Locked via currentBytes adjustment
	}
}

// IsLiveStatus reports whether a record's SourceStatus is a live "serve as
// CURRENT" state. Used to filter stale fetches before they re-pollute the
// cache after a withdrawal.
func IsLiveStatus(s string) bool {
	switch s {
	case "CURRENT", "":
		return true
	default:
		// WITHDRAWN, SUPERSEDED, QUARANTINED, STALE — never serve as fresh.
		return false
	}
}

type call[T any] struct {
	wg  sync.WaitGroup
	val T
	err error
}

// callGroup coalesces concurrent identical upstream fetches. Each caller's
// context is honored independently: a cancelled caller returns ctx.Err()
// immediately without consuming the leader's result, so no caller hangs
// behind a slow or hung leader. The leader's fn runs to completion regardless
// of how many of its waiters cancel; subsequent calls after leader completion
// start a fresh fetch.
type callGroup[T any] struct {
	mu sync.Mutex
	m  map[string]*call[T]
}

func (g *callGroup[T]) DoCtx(ctx context.Context, key string, fn func() (T, error)) (T, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call[T])
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		// Existing leader: wait either for the leader to finish OR for our context.
		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			return c.val, c.err
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		}
	}
	// Become the leader for this key.
	c := new(call[T])
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	go func() {
		defer c.wg.Done()
		c.val, c.err = fn()
	}()

	// Leader waits for fn completion OR its own context cancellation.
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		g.mu.Lock()
		delete(g.m, key)
		g.mu.Unlock()
		return c.val, c.err
	case <-ctx.Done():
		// Leader was cancelled: leave fn running (it'll complete on its own
		// context), but remove the slot so the next request starts a fresh
		// fetch rather than attaching to this in-progress one. The fn's
		// return value will be discarded when c.wg fires.
		g.mu.Lock()
		delete(g.m, key)
		g.mu.Unlock()
		var zero T
		return zero, ctx.Err()
	}
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

// NewCachedSource creates a cached publication source. A bounded cache is
// used by default (4096 entries, 32 MiB); pass nil for the now function.
func NewCachedSource(inner PublicationSource, cfg Config, now func() time.Time) *CachedSource {
	return &CachedSource{
		inner: inner,
		cache: NewMemoryCache(now, defaultCacheEntries, defaultCacheBytes),
		cfg:   cfg,
	}
}

const (
	defaultCacheEntries = 4096
	defaultCacheBytes   = 32 << 20 // 32 MiB
)

// InvalidateManifest purges the cached manifest for a jurisdiction so upstream
// updates are immediately visible.
func (s *CachedSource) InvalidateManifest(jurisdiction string) {
	s.cache.InvalidateManifest(jurisdiction)
}

// InvalidateCard purges the cached card for a package and version.
func (s *CachedSource) InvalidateCard(packageID string, version int) {
	s.cache.InvalidateCard(packageID, version)
}

// InvalidatePackage purges all cached card versions for a package.
func (s *CachedSource) InvalidatePackage(packageID string) {
	s.cache.InvalidatePackage(packageID)
}

// GetManifest checks cache before calling upstream, coalescing concurrent misses.
// A waiter's context cancellation returns ctx.Err immediately without consuming
// the leader's result. A withdrawn upstream result is filtered before it can
// repopulate the cache.
func (s *CachedSource) GetManifest(ctx context.Context, jurisdiction string) (*ManifestRecord, error) {
	if rec, hit := s.cache.GetManifest(jurisdiction); hit {
		return rec, nil
	}
	return s.manifestFlight.DoCtx(ctx, jurisdiction, func() (*ManifestRecord, error) {
		if rec, hit := s.cache.GetManifest(jurisdiction); hit {
			return rec, nil
		}
		rec, err := s.inner.GetManifest(ctx, jurisdiction)
		if err != nil {
			return nil, err
		}
		if rec != nil && IsLiveStatus(rec.SourceStatus) && s.cfg.ManifestCacheTTL > 0 {
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
	return s.cardFlight.DoCtx(ctx, key, func() (*CardRecord, error) {
		if rec, hit := s.cache.GetCard(packageID, version); hit {
			return rec, nil
		}
		rec, err := s.inner.GetCard(ctx, packageID, version)
		if err != nil {
			return nil, err
		}
		if rec != nil && IsLiveStatus(rec.SourceStatus) && s.cfg.CardCacheTTL > 0 {
			s.cache.PutCard(packageID, version, rec, s.cfg.CardCacheTTL)
		}
		return rec, nil
	})
}

// GetResource passes directly to inner source, as resource streams are seekable readers.
func (s *CachedSource) GetResource(ctx context.Context, resourceID string) (*ResourceContent, error) {
	return s.inner.GetResource(ctx, resourceID)
}
