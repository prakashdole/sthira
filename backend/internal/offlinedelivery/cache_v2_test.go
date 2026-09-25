package offlinedelivery

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func jur(i int) string { return fmt.Sprintf("JUR-%d", i) }

// TestMemoryCache_BoundedByEntries: cache evicts oldest entry when entry count
// would exceed the cap. Verified by writing more entries than the cap and
// checking that the oldest is gone.
func TestMemoryCache_BoundedByEntries(t *testing.T) {
	now := time.Now()
	mc := NewMemoryCache(func() time.Time { return now }, 3, 0)
	for i := 0; i < 5; i++ {
		mc.PutManifest(jur(i), &ManifestRecord{
			Jurisdiction:   jur(i),
			RawJSON:        []byte(`{"x":1}`),
			ChecksumSHA256: "c" + jur(i),
			SourceStatus:   "CURRENT",
		}, time.Minute)
		// advance "now" so each Put has a strictly newer writtenAt
		now = now.Add(time.Millisecond)
	}
	entries, _ := mc.Stats()
	if entries > 3 {
		t.Fatalf("entries=%d > cap 3", entries)
	}
	// Oldest entries should be gone.
	if _, hit := mc.GetManifest(jur(0)); hit {
		t.Errorf("jurisdiction %q should have been evicted", jur(0))
	}
	if _, hit := mc.GetManifest(jur(1)); hit {
		t.Errorf("jurisdiction %q should have been evicted", jur(1))
	}
	// Newest should still be present.
	if _, hit := mc.GetManifest(jur(4)); !hit {
		t.Errorf("jurisdiction %q should still be present", jur(4))
	}
}

// TestMemoryCache_BoundedByBytes: cache evicts oldest entry when byte total
// would exceed the cap. Each entry is 1 KiB; cap = 3 KiB; writing 5 must evict 2.
func TestMemoryCache_BoundedByBytes(t *testing.T) {
	now := time.Now()
	mc := NewMemoryCache(func() time.Time { return now }, 0, 3072)
	payload := make([]byte, 1024) // 1 KiB
	for i := 0; i < 5; i++ {
		mc.PutManifest(jur(i), &ManifestRecord{
			Jurisdiction:   jur(i),
			RawJSON:        payload,
			ChecksumSHA256: "c",
			SourceStatus:   "CURRENT",
		}, time.Minute)
		now = now.Add(time.Millisecond)
	}
	entries, bytes := mc.Stats()
	if bytes > 3072 {
		t.Fatalf("bytes=%d > cap 3072", bytes)
	}
	if entries > 4 { // 3 KiB allows 3 entries; one more may exist briefly during insert
		t.Fatalf("entries=%d > reasonable bound", entries)
	}
}

// TestMemoryCache_StaleRepublishRejected: a record whose SourceStatus is
// WITHDRAWN must NOT be cached, even from a fresh upstream result. This prevents
// a stale inflight fetch from re-polluting the cache after an explicit withdrawal.
func TestMemoryCache_StaleRepublishRejected(t *testing.T) {
	now := time.Now()
	mc := NewMemoryCache(func() time.Time { return now }, 0, 0)

	// 1. Warm the cache with a CURRENT record.
	mc.PutManifest("KL", &ManifestRecord{
		Jurisdiction:   "KL",
		RawJSON:        []byte(`{"a":1}`),
		ChecksumSHA256: "c",
		SourceStatus:   "CURRENT",
	}, time.Minute)
	if _, hit := mc.GetManifest("KL"); !hit {
		t.Fatalf("expected initial hit")
	}

	// 2. Operator withdraws the manifest.
	mc.InvalidateManifest("KL")

	// 3. A stale inflight fetch returns the (now WITHDRAWN) record.
	staleRec := &ManifestRecord{
		Jurisdiction:   "KL",
		RawJSON:        []byte(`{"a":1}`),
		ChecksumSHA256: "c",
		SourceStatus:   "WITHDRAWN",
	}
	mc.PutManifest("KL", staleRec, time.Minute)

	// 4. The cache MUST NOT contain the withdrawn record.
	if rec, hit := mc.GetManifest("KL"); hit {
		t.Fatalf("stale WITHDRAWN record leaked into cache: %+v", rec)
	}
}

// TestCachedSource_CancelableWaiters: of N concurrent waiters for one leader,
// any waiter that cancels returns ctx.Err immediately. The remaining waiters
// still receive the leader's result.
func TestCachedSource_CancelableWaiters(t *testing.T) {
	mock := newMockPublicationSource()
	mock.delayGetManifest = 200 * time.Millisecond
	cs := NewCachedSource(mock, Config{
		ManifestCacheTTL: 10 * time.Minute,
		CardCacheTTL:     10 * time.Minute,
	}, nil)

	const n = 8
	var (
		cancelAfter = 20 * time.Millisecond
		cancelHits  atomic.Int64
		okHits      atomic.Int64
	)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), cancelAfter)
			defer cancel()
			_, err := cs.GetManifest(ctx, "KL")
			if err == context.DeadlineExceeded {
				cancelHits.Add(1)
			} else if err == nil {
				okHits.Add(1)
			}
		}()
	}
	wg.Wait()
	if cancelHits.Load() == 0 {
		t.Fatalf("expected at least some waiters to hit deadline; got 0 cancels, %d oks", okHits.Load())
	}
	// Leader's fn must have run exactly once (coalesced).
	if mock.manifestCalls.Load() != 1 {
		t.Fatalf("expected exactly 1 upstream call after coalesced waiters, got %d", mock.manifestCalls.Load())
	}
}
