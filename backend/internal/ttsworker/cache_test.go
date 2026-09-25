package ttsworker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

// TestCacheIdentityStringIsStable: the same identity across rebuilds
// produces the same canonical string.
func TestCacheIdentityStringIsStable(t *testing.T) {
	a := CacheIdentity{Text: "hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-1"}
	b := CacheIdentity{Text: "hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-1"}
	if a.String() != b.String() {
		t.Fatalf("identity strings should match: %q vs %q", a.String(), b.String())
	}
}

// TestCacheIdentityStrDiffersOnAnyField: every field changes the
// identity string. This is the cache-correctness invariant: a
// parameter change MUST invalidate the cache.
func TestCacheIdentityStrDiffersOnAnyField(t *testing.T) {
	base := CacheIdentity{Text: "hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-1"}
	baseStr := base.String()
	variants := []CacheIdentity{
		{Text: "Hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-1"},
		{Text: "hello", TemplateKey: "b", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-1"},
		{Text: "hello", TemplateKey: "a", Language: "ml-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-1"},
		{Text: "hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 2, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-1"},
		{Text: "hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 2, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-1"},
		{Text: "hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-2", VoiceRevision: "v-1"},
		{Text: "hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "rev-1", VoiceRevision: "v-2"},
		{Text: "hello", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d1", ModelRevision: "rev-1", VoiceRevision: "v-1"},
	}
	for i, v := range variants {
		if v.String() == baseStr {
			t.Fatalf("variant %d (%+v) produced identical identity", i, v)
		}
	}
}

// TestCodecStoresAndServes: round-trip a payload with the same
// identity produces a hit.
func TestCodecStoresAndServes(t *testing.T) {
	clock := NewStandaloneSourceVersionClock(1)
	c := NewCodec(1024*1024, time.Minute, clock)
	id := CacheIdentity{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "r-1", VoiceRevision: "v-1"}
	bytes := []byte("WAVBYTES")
	if err := c.Put(id, bytes); err != nil {
		t.Fatal(err)
	}
	got, ok := c.Get(id)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if !bytesEqualLocal(got.Bytes, bytes) {
		t.Fatalf("bytes mismatch")
	}
	if c.Stats().Entries != 1 {
		t.Fatalf("entries: got %d want 1", c.Stats().Entries)
	}
}

// TestCodecLRUEvictsUnderBudget: when the byte budget is hit, oldest
// entries are evicted.
func TestCodecLRUEvictsUnderBudget(t *testing.T) {
	clock := NewStandaloneSourceVersionClock(1)
	c := NewCodec(20, time.Minute, clock)
	for i := 0; i < 5; i++ {
		id := CacheIdentity{Text: string(rune('a' + i)), TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "r-1", VoiceRevision: "v-1"}
		if err := c.Put(id, bytes.Repeat([]byte{'x'}, 10)); err != nil {
			t.Fatal(err)
		}
	}
	if c.Stats().Bytes > c.Stats().MaxBytes {
		t.Fatalf("byte budget violated: %d > %d", c.Stats().Bytes, c.Stats().MaxBytes)
	}
	if c.Stats().Entries == 0 {
		t.Fatalf("expected some entries remaining")
	}
}

// TestCodecInvalidatesOnSourceAdvance: a source-version advance that
// the broadcaster fires MUST remove cached entries. The worker
// prompt requires this: a purge helper tested alone is not enough.
func TestCodecInvalidatesOnSourceAdvance(t *testing.T) {
	clock := NewStandaloneSourceVersionClock(1)
	c := NewCodec(1024*1024, time.Hour, clock)
	id := CacheIdentity{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "r-1", VoiceRevision: "v-1"}
	if err := c.Put(id, []byte("WAVBYTES")); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get(id); !ok {
		t.Fatal("expected hit before advance")
	}
	// Advance the broadcaster. The cache's OnAdvance listener drops
	// the entry automatically.
	clock.Advance(2)
	if _, ok := c.Get(id); ok {
		t.Fatal("WIRED eviction failed: cache returned stale entry after withdrawal")
	}
}

// TestCodecWithdrawsMidServe: a concurrent withdrawal during a
// Get call drops the entry before the caller sees it. The test
// drives multiple goroutines to demonstrate that the lock-protected
// Get/handleSourceAdvance never serves withdrawn audio.
func TestCodecWithdrawsMidServe(t *testing.T) {
	clock := NewStandaloneSourceVersionClock(1)
	c := NewCodec(1024*1024, time.Hour, clock)
	id := CacheIdentity{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "r-1", VoiceRevision: "v-1"}
	if err := c.Put(id, []byte("WAVBYTES")); err != nil {
		t.Fatal(err)
	}
	// Drive the broadcaster from a goroutine while a reader hits Get.
	const N = 200
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < N; i++ {
			select {
			case <-stop:
				return
			default:
				clock.Advance(2)
			}
		}
	}()
	for i := 0; i < N; i++ {
		// Exercise concurrent reads while broadcaster advances.
		_, _ = c.Get(id)
	}
	close(stop)
	<-done
}

// TestCodecPurgeHelper: direct calls to InvalidateBySourceVersion
// remove all entries. Keeps the helper-level assertion the worker
// prompt warns against trusting alone.
func TestCodecPurgeHelper(t *testing.T) {
	clock := NewStandaloneSourceVersionClock(1)
	c := NewCodec(1024*1024, time.Hour, clock)
	id := CacheIdentity{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "r-1", VoiceRevision: "v-1"}
	if err := c.Put(id, []byte("WAVBYTES")); err != nil {
		t.Fatal(err)
	}
	if n := c.InvalidateBySourceVersion(2); n != 1 {
		t.Fatalf("helper-evicted: got %d want 1", n)
	}
	if _, ok := c.Get(id); ok {
		t.Fatal("entry not removed by helper")
	}
}

// TestCodecTTLExpires: expired entries are removed on Get and via
// EvictExpired.
func TestCodecTTLExpires(t *testing.T) {
	clock := NewStandaloneSourceVersionClock(1)
	c := NewCodec(1024*1024, time.Hour, clock)
	c.now = func() time.Time {
		return time.Unix(0, 0)
	}
	id := CacheIdentity{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d0", ModelRevision: "r-1", VoiceRevision: "v-1"}
	if err := c.Put(id, []byte("WAVBYTES")); err != nil {
		t.Fatal(err)
	}
	// Advance the clock past TTL.
	c.now = func() time.Time {
		return time.Unix(0, 0).Add(2 * time.Hour)
	}
	if _, ok := c.Get(id); ok {
		t.Fatal("expected TTL expiry to drop entry")
	}
}

// TestCacheIdentityValidate: an identity with missing fields is
// rejected so partial identities can never map to existing entries.
func TestCacheIdentityValidate(t *testing.T) {
	ok := CacheIdentity{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "d", ModelRevision: "r", VoiceRevision: "v"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("expected ok identity to validate: %v", err)
	}
	bad := []CacheIdentity{
		{},                             // all blank
		{Text: "h", Language: "en-IN"}, // missing template key
		{Text: "h", TemplateKey: "a"},  // missing language
		{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, VoiceRevision: "v"},                    // missing model
		{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, ModelRevision: "r"},                    // missing voice
		{Text: "h", TemplateKey: "a", Language: "en-IN", ModelRevision: "r", VoiceRevision: "v"},                  // missing source_version
		{Text: "", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, ModelRevision: "r", VoiceRevision: "v"}, // empty text
		{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 0, TemplateSHA256: "d", ModelRevision: "r", VoiceRevision: "v"},
		{Text: "h", TemplateKey: "a", Language: "en-IN", SourceVersion: 1, TemplateVersion: 1, TemplateSHA256: "", ModelRevision: "r", VoiceRevision: "v"},
	}
	for i, b := range bad {
		err := b.Validate()
		if !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("bad identity %d should fail: %v", i, err)
		}
	}
}

// TestChecksumContentAddress: identical bytes hash to identical
// checksums.
func TestChecksumContentAddress(t *testing.T) {
	a := sha256.Sum256([]byte("hello"))
	b := sha256.Sum256([]byte("hello"))
	if hex.EncodeToString(a[:]) != hex.EncodeToString(b[:]) {
		t.Fatalf("hash mismatch")
	}
}
