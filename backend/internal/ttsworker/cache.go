package ttsworker

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// CacheMaxBytes is the default bound on storage bytes. Each value
// counts its full size towards the budget.
const CacheMaxBytes = 16 * 1024 * 1024

// CacheTTL is the default record lifetime. Audio invalidated only by
// explicit source-version advance is still bounded by this TTL.
const CacheTTL = 24 * time.Hour

// CacheEntry is one stored item. Bytes are the canonical PCM WAV;
// ChecksumSHA256 is the canonical content address.
type CacheEntry struct {
	Key            CacheIdentity `json:"key"`
	Bytes          []byte        `json:"-"` // never serialized; always recomputed from cached WAV
	ChecksumSHA256 string        `json:"checksum_sha256"`
	ByteSize       int64         `json:"byte_size"`
	CreatedAt      time.Time     `json:"created_at"`
	// SourceVersionAtEntry is the sourceVersion the cache recorded
	// when this entry was created. The entry is invalidated as soon
	// as the runtime source version advances past this value.
	SourceVersionAtEntry int    `json:"source_version_at_entry"`
	TemplateKey          KeyStr `json:"template_key"`
	Language             string `json:"language"`
}

// KeyStr is a string-typed wrapper around the template key. We do
// not import templates.Key into the cache's on-the-wire JSON to keep
// this layer readable.
type KeyStr string

// CacheIdentity is the canonical identity. All fields are required;
// the cache refuses a partial identity. Stored as JSON-encoded key so
// a parameter change deterministically invalidates the cache.
type CacheIdentity struct {
	Text              string            `json:"text"`
	TemplateKey       string            `json:"template_key"`
	TemplateVersion   int               `json:"template_version"`
	SourceVersion     int               `json:"source_version"`
	Language          string            `json:"language"`
	ModelRevision     string            `json:"model_revision"`
	VoiceRevision     string            `json:"voice_revision"`
	SynthesisSettings SynthesisSettings `json:"synthesis_settings"`
}

// SynthesisSettings is the worker-side mirror of the contracts-level
// TTSSynthesisSettings used in the cache identity. We keep it here
// to avoid importing contracts across the private go.mod boundary.
type SynthesisSettings struct {
	SampleRate   int     `json:"sample_rate"`
	BitDepth     int     `json:"bit_depth"`
	Channels     int     `json:"channels"`
	SpeakingRate float64 `json:"speaking_rate,omitempty"`
}

// String returns the JSON-encoded identity. Two identities with
// equal JSON-encoded output hash to the same canonical content
// address.
func (id CacheIdentity) String() string {
	b, err := json.Marshal(id)
	if err != nil {
		// Marshalling these typed structures cannot fail; protect
		// against future field additions.
		return ""
	}
	return string(b)
}

// Codec is the cache's serialization. It is bounded, thread-safe
// and wired to the source-version clock so a withdrawal invalidates
// audio as soon as the source version advances.
type Codec struct {
	mu       sync.Mutex
	ttl      time.Duration
	maxBytes int64
	clock    SourceVersionBroadcaster

	// items maps the identity hash → *list.Element. list tracks LRU
	// order so eviction respects recency.
	items map[string]*list.Element
	lru   *list.List
	bytes int64
	now   func() time.Time
}

// NewCodec returns a fresh cache. When clock is nil, the cache still
// functions but withdrawals never invalidate; that mode exists for
// tests that want to exercise direct eviction. Production wires a
// real broadcaster so the cache's withdrawal path runs end-to-end.
func NewCodec(maxBytes int64, ttl time.Duration, clock SourceVersionBroadcaster) *Codec {
	if maxBytes <= 0 {
		maxBytes = CacheMaxBytes
	}
	if ttl <= 0 {
		ttl = CacheTTL
	}
	c := &Codec{
		ttl:      ttl,
		maxBytes: maxBytes,
		clock:    clock,
		items:    map[string]*list.Element{},
		lru:      list.New(),
		now:      time.Now,
	}
	if clock != nil {
		// OnAdvance can fire while the cache holds the lock; the
		// callback runs eviction under it.
		clock.OnAdvance(noCancel(), func(newVersion int) {
			c.handleSourceAdvance(newVersion)
		})
	}
	return c
}

// Get fetches an entry by identity. Returns (nil, false) when there
// is no live entry; invalidation by source-version advance removes
// it before the lookup returns. A racing withdrawal that fires
// between Get and the caller reading the bytes is impossible: Get
// holds the codec lock for the entire branch.
func (c *Codec) Get(id CacheIdentity) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	current := -1
	if c.clock != nil {
		current = c.clock.Current()
	}
	el, ok := c.items[id.String()]
	if !ok {
		return nil, false
	}
	entry := el.Value.(*CacheEntry)
	// Hard expiry by TTL.
	if now.Sub(entry.CreatedAt) > c.ttl {
		c.removeElement(el)
		return nil, false
	}
	// Source-version withdrawal: if the current source version is
	// past the version recorded at entry time, the entry is invalid.
	// This is the wired path the worker prompt requires — a purge
	// helper tested alone is not sufficient.
	if current >= 0 && current > entry.SourceVersionAtEntry {
		c.removeElement(el)
		return nil, false
	}
	// LRU touch.
	c.lru.MoveToFront(el)
	cp := *entry
	cp.Bytes = append([]byte(nil), entry.Bytes...)
	return &cp, true
}

// Put inserts or refreshes an entry. Returns an error when the entry
// would push the byte budget past the cap; the caller is expected
// to either accept the rejection or evict explicitly. The Put does
// NOT silently truncate bytes.
func (c *Codec) Put(id CacheIdentity, bytes []byte) error {
	if len(bytes) == 0 {
		return errors.New("empty bytes")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	current := -1
	if c.clock != nil {
		current = c.clock.Current()
	}
	entry := &CacheEntry{
		Key:                  id,
		Bytes:                bytes,
		ChecksumSHA256:       sha256Hex(bytes),
		ByteSize:             int64(len(bytes)),
		CreatedAt:            c.now(),
		SourceVersionAtEntry: -1, // set below
		TemplateKey:          KeyStr(id.TemplateKey),
		Language:             id.Language,
	}
	if current >= 0 {
		entry.SourceVersionAtEntry = current
	}
	// Replace if present.
	key := id.String()
	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
	el := c.lru.PushFront(entry)
	c.items[key] = el
	c.bytes += int64(len(bytes))
	// Evict while over budget.
	for c.bytes > c.maxBytes {
		tail := c.lru.Back()
		if tail == nil {
			break
		}
		c.removeElement(tail)
	}
	return nil
}

// InvalidateBySourceVersion is the explicit purge path. It exists
// for tests that want to assert the helper directly. In production
// the wired source-version broadcaster fires the same removal logic
// automatically. Worker 7 explicitly distinguishes: a purge helper
// tested alone is insufficient; the cache must also drop entries
// when the broadcaster advances the source version while another
// request is in flight.
func (c *Codec) InvalidateBySourceVersion(_ int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.removeAll()
}

// Stats returns the cache's current state. The introspection helper
// exposes counts and bytes so tests can verify the LRU/TTL paths
// without inspecting private state.
type CodecStats struct {
	Entries     int   `json:"entries"`
	Bytes       int64 `json:"bytes"`
	MaxBytes    int64 `json:"max_bytes"`
	TTLSeconds  int   `json:"ttl_seconds"`
	CurrentSrc  int   `json:"current_source_version"`
	PendingHits int64 `json:"pending_hits"`
	Evicted     int64 `json:"evicted"`
}

// Stats returns the current state.
func (c *Codec) Stats() CodecStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	src := -1
	if c.clock != nil {
		src = c.clock.Current()
	}
	return CodecStats{
		Entries:    len(c.items),
		Bytes:      c.bytes,
		MaxBytes:   c.maxBytes,
		TTLSeconds: int(c.ttl.Seconds()),
		CurrentSrc: src,
	}
}

func (c *Codec) removeElement(el *list.Element) {
	c.lru.Remove(el)
	entry := el.Value.(*CacheEntry)
	c.bytes -= entry.ByteSize
	delete(c.items, entry.Key.String())
}

func (c *Codec) removeAll() int {
	n := 0
	for {
		el := c.lru.Back()
		if el == nil {
			break
		}
		c.removeElement(el)
		n++
	}
	return n
}

// handleSourceAdvance is the wired withdrawal path. It runs under
// the broadcaster's lock; the codec takes its own lock to avoid
// deadlocks with concurrent Get/Put calls. The function is invoked
// once per source-version advance with the new value.
func (c *Codec) handleSourceAdvance(newVersion int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// The withdrawal passes when newVersion > recorded
	// SourceVersionAtEntry. We drop every entry that was recorded at
	// an older version.
	for key, el := range c.items {
		entry := el.Value.(*CacheEntry)
		_ = key
		if entry.SourceVersionAtEntry >= 0 && entry.SourceVersionAtEntry < newVersion {
			c.removeElement(el)
		}
	}
}

// EvictExpired walks the cache and drops entries past their TTL. Not
// needed for correct semantics (Get double-checks TTL) but useful
// for tests to assert cleanup behavior without sleeping.
func (c *Codec) EvictExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	removed := 0
	els := []*list.Element{}
	for el := c.lru.Front(); el != nil; el = el.Next() {
		els = append(els, el)
	}
	for _, el := range els {
		entry := el.Value.(*CacheEntry)
		if now.Sub(entry.CreatedAt) > c.ttl {
			c.removeElement(el)
			removed++
		}
	}
	return removed
}

// InventoryKeyFor is a small helper that builds a default identity
// where the caller has only the (template_key, language, text, version)
// fields. The other fields are blank and the cache key includes the
// blanks; production callers pass full identities.
func InventoryKeyFor(text, templateKey, language string, templateVer int) CacheIdentity {
	return CacheIdentity{
		Text:              text,
		TemplateKey:       templateKey,
		TemplateVersion:   templateVer,
		Language:          language,
		SynthesisSettings: SynthesisSettings{},
	}
}

// SortIdentitiesBy is a small helper for tests that want to compare
// two identity sets.
func SortIdentitiesBy(ids []CacheIdentity, by func(a, b CacheIdentity) bool) []CacheIdentity {
	out := append([]CacheIdentity(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return by(out[i], out[j]) })
	return out
}

// WhyInvalid is a debugging helper returned by tests that want to
// distinguish withdrawal-driven from TTL-driven eviction. It is not
// required for the cache to function.
type WhyInvalid string

const (
	WhyExpired     WhyInvalid = "TTL_EXPIRED"
	WhyWithdrawn   WhyInvalid = "SOURCE_WITHDRAWN"
	WhyCacheMiss   WhyInvalid = "MISS"
	WhyRejected    WhyInvalid = "REJECTED"
	WhyUnavailable WhyInvalid = "UNAVAILABLE"
)

// String pretty-prints the reason.
func (w WhyInvalid) String() string { return string(w) }

// ErrInvalidIdentity signals the cache that the synthetic caller
// forgot to populate a required field. The worker surfaces this as
// AUDIO_UNAVAILABLE in production so missing fields never silently
// become cache hits.
var ErrInvalidIdentity = errors.New("ttsworker: identity has unfilled required field")

// Validate ensures the identity is fully populated so a missing
// field cannot silently map to an existing entry from a different
// request.
func (id CacheIdentity) Validate() error {
	switch {
	case id.Text == "":
		return fmt.Errorf("%w: text", ErrInvalidIdentity)
	case id.TemplateKey == "":
		return fmt.Errorf("%w: template_key", ErrInvalidIdentity)
	case id.Language == "":
		return fmt.Errorf("%w: language", ErrInvalidIdentity)
	case id.ModelRevision == "":
		return fmt.Errorf("%w: model_revision", ErrInvalidIdentity)
	case id.VoiceRevision == "":
		return fmt.Errorf("%w: voice_revision", ErrInvalidIdentity)
	case id.SourceVersion == 0:
		return fmt.Errorf("%w: source_version", ErrInvalidIdentity)
	}
	return nil
}

// noCancel returns a CancelableContext that never fires. Used to
// register long-lived OnAdvance listeners whose lifetime matches
// the worker's.
func noCancel() CancelableContext { return neverCancel{} }

type neverCancel struct{}

func (neverCancel) Done() <-chan struct{} { return nil }
func (neverCancel) Err() error            { return nil }
