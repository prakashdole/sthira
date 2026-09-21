package ttsworker

import (
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ParlerTTS is the frozen identifier set for the Indic Parler-TTS
// artifact. ModelID follows the source-register reference; the worker
// records it as a candidate without claiming authorization — the
// License field stays as LicensePending until an authorized verifier
// records the SPDX expression.
type ParlerTTS struct {
	ModelID            string   `json:"model_id"`            // ai4bharat/indic-parler-tts
	Revision           string   `json:"revision"`            // claimed pinned revision; "" until authorized verifier records
	License            string   `json:"license"`             // SPDX identifier OR "LicensePending"
	PretrainedFile     string   `json:"pretrained_file"`     // filename expected at root, e.g. "pytorch_model.bin" or "model.safetensors"
	Runtime            string   `json:"runtime"`             // e.g. "transformers-4.x" OR "LicensePending"
	Hardware           string   `json:"hardware"`            // e.g. "cpu" OR LicensePending until authorized verifier records
	RemoteCode         bool     `json:"remote_code"`         // trust-this must be false in production
	Voices             []string `json:"voices"`              // populated from the artifact description; empty until inventory scan
	SupportedLanguages []string `json:"supported_languages"` // kept narrow; configured languages only
}

// DefaultParlerTTS returns the candidate inventory used by the worker
// before authorized artifacts land. LicensePending everywhere signals
// to the orchestrator that Ready=true cannot be advertised yet.
func DefaultParlerTTS() ParlerTTS {
	return ParlerTTS{
		ModelID:        "ai4bharat/indic-parler-tts",
		Revision:       "",
		License:        "LicensePending",
		PretrainedFile: "model.safetensors",
		Runtime:        "LicensePending",
		Hardware:       "LicensePending",
		RemoteCode:     false,
		Voices:         nil,
		// Languages are derived from the runtime adapter, NOT from
		// the model name. The reference Python projection does not
		// exist; the stub runtime advertises nothing.
		SupportedLanguages: nil,
	}
}

// ArtifactDigest records the SHA-256 of an auxiliary Parler-TTS
// artifact (tokenizer, voice embedding, prompt prefix, etc.). All
// digests are recorded as the HashZero placeholder until an
// authorized verifier populates them.
type ArtifactDigest struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	ChecksumSHA256 string `json:"checksum_sha256"` // sha256 hex, or HashZero placeholder
	License        string `json:"license,omitempty"`
}

// HashStatus enumerates the lifecycle of a hash match in the inventory
// scan. Production reports PendingCritical whenever any non-optional
// component is HashMissingOnDisk or HashMissingRecorded.
type HashStatus string

const (
	HashOK              HashStatus = "OK"
	HashMissingOnDisk   HashStatus = "MISSING_ON_DISK"
	HashMissingRecorded HashStatus = "MISSING_RECORDED"
	HashMismatch        HashStatus = "MISMATCH"
	HashLicensePending  HashStatus = "LICENSE_PENDING"
)

// HashMatch is one entry in the inventory scan result.
type HashMatch struct {
	Name           string     `json:"name"`
	ExpectedSHA256 string     `json:"expected_sha256,omitempty"` // populated only when a verifier recorded one
	ActualSHA256   string     `json:"actual_sha256,omitempty"`   // populated when the file is ≤ 64 KiB and probe-able
	Status         HashStatus `json:"status"`
}

// ComponentCritical returns true if the named component is essential
// for production synthesis. Worker 5/7 treat PendingCritical as a
// blocker on Ready=true.
func ComponentCritical(name string) bool {
	switch name {
	case CompModelCheckpoint, CompTokenizer, CompVoiceRegistry, CompRuntime, CompPromptPrefix:
		return true
	}
	return false
}

// Component constants for Parler-TTS. Renamed/lost files end up as
// MISSING_ON_DISK; placeholder digests end up as MISSING_RECORDED.
const (
	CompModelCheckpoint = "model_checkpoint"
	CompTokenizer       = "tokenizer"
	CompVoiceRegistry   = "voice_registry"
	CompRuntime         = "runtime"
	CompPromptPrefix    = "prompt_prefix"
)

// HashZero is the placeholder sentry for an empty digest. We never
// store an actual zero-byte artifact's digest as if it were an
// authoritative match.
const HashZero = "0000000000000000000000000000000000000000000000000000000000000000"

// Inventory is one moment in time of the Parler-TTS artifact set.
// PendingCritical is the canonical "we are NOT ready" signal — every
// component the catalog names as essential has a non-empty entry
// here until an authorized scan passes.
type Inventory struct {
	Parler          ParlerTTS `json:"parler"`
	LastScannedAt   time.Time `json:"last_scanned_at"`
	LastScannedRoot string    `json:"last_scanned_root"`

	// HashMatches enumerates every component the scanner saw. Includes
	// missing-disk and missing-recorded entries.
	HashMatches []HashMatch `json:"hash_matches"`

	// PendingCritical lists the components that blocked Ready=true.
	// When non-empty, the worker refuses to advertise Warm=true /
	// Ready=true. Order is sorted for stable output.
	PendingCritical []string `json:"pending_critical"`

	// SupportedLanguages is the runtime-derived allow list, not the
	// model-name-derived list. An inventory with no adapter may have
	// empty supported languages; that's also a Ready=false signal.
	SupportedLanguages []string `json:"supported_languages"`

	// RemoteCodeTrust surfaces the worker's trust-this setting.
	// Production must keep this false.
	RemoteCodeTrust bool `json:"remote_code_trust"`
}

// Ready reports whether the inventory is sufficient to advertise
// Ready=true on /health. Required invariants:
//
//   - PendingCritical must be empty.
//   - Every HashMatch entry must be HashOK or HashLicensePending.
//   - Parler.License must not be LicensePending.
//   - Parler.Runtime and Hardware must not be LicensePending.
//   - SupportedLanguages must be non-empty.
//   - Parler.Revision must be non-empty.
//   - RemoteCodeTrust must be false.
func (inv Inventory) Ready() error {
	if len(inv.PendingCritical) > 0 {
		return errors.New("inventory has pending critical components")
	}
	if inv.Parler.License == "LicensePending" || inv.Parler.License == "" {
		return errors.New("license is not authorized")
	}
	if inv.Parler.Runtime == "LicensePending" || inv.Parler.Runtime == "" {
		return errors.New("runtime is not authorized")
	}
	if inv.Parler.Hardware == "LicensePending" || inv.Parler.Hardware == "" {
		return errors.New("hardware is not authorized")
	}
	if inv.Parler.Revision == "" {
		return errors.New("model revision is not recorded")
	}
	if len(inv.SupportedLanguages) == 0 {
		return errors.New("supported languages is empty")
	}
	for _, hm := range inv.HashMatches {
		switch hm.Status {
		case HashOK, HashLicensePending:
		default:
			return errors.New("hash match status is not OK: " + string(hm.Status))
		}
	}
	if inv.RemoteCodeTrust {
		return errors.New("remote_code trust must be false")
	}
	return nil
}

// ScanLocalInventory inspects LocalPath and fills whatever it can
// without downloading. It NEVER reads model.safetensors or
// pytorch_model.bin payloads; only file existence and (when small
// enough for a fast probe) SHA-256.
//
// The scan is metadata-only. When authorized artifacts land, the
// inventory is filled by an external verifier that reads the
// canonical digest per component. The placeholder digests in this
// worker are NEVER trusted for production.
func ScanLocalInventory(root string, par ParlerTTS) Inventory {
	inv := Inventory{Parler: par, LastScannedAt: time.Now().UTC(), LastScannedRoot: root, RemoteCodeTrust: par.RemoteCode}
	if strings.TrimSpace(root) == "" {
		inv.PendingCritical = []string{CompModelCheckpoint, CompTokenizer, CompVoiceRegistry, CompRuntime, CompPromptPrefix}
		sort.Strings(inv.PendingCritical)
		inv.SupportedLanguages = nil
		return inv
	}
	checkpoints := []string{root + "/model.safetensors", root + "/pytorch_model.bin"}
	probes := map[string]string{
		CompModelCheckpoint: "", // resolved below
		CompTokenizer:       "/tokenizer",
		CompVoiceRegistry:   "/voices.json",
		CompRuntime:         "/parler_tts_runtime.py",
		CompPromptPrefix:    "/prompt_prefix.json",
	}
	// Model checkpoint: probe any canonical filename at the root.
	hasCheckpoint := false
	for _, p := range checkpoints {
		if _, err := os.Stat(p); err == nil {
			hasCheckpoint = true
			break
		}
	}
	if hasCheckpoint {
		inv.HashMatches = append(inv.HashMatches, HashMatch{Name: CompModelCheckpoint, Status: HashMissingRecorded})
	} else {
		inv.PendingCritical = append(inv.PendingCritical, CompModelCheckpoint)
		inv.HashMatches = append(inv.HashMatches, HashMatch{Name: CompModelCheckpoint, Status: HashMissingOnDisk})
	}
	for name, rel := range probes {
		if rel == "" {
			continue
		}
		full := root + rel
		st, err := os.Stat(full)
		if errors.Is(err, os.ErrNotExist) || err != nil {
			inv.PendingCritical = append(inv.PendingCritical, name)
			inv.HashMatches = append(inv.HashMatches, HashMatch{Name: name, Status: HashMissingOnDisk})
			continue
		}
		var actual string
		if st.Size() > 0 && st.Size() <= 64*1024 {
			if b, err := os.ReadFile(full); err == nil {
				actual = SHA256Hex(b)
			}
		}
		inv.HashMatches = append(inv.HashMatches, HashMatch{Name: name, ActualSHA256: actual, Status: HashMissingRecorded})
	}
	sort.Strings(inv.PendingCritical)
	sort.Slice(inv.HashMatches, func(i, j int) bool { return inv.HashMatches[i].Name < inv.HashMatches[j].Name })
	return inv
}

// SHA256Hex returns the hex-encoded SHA-256 of b. Local import of the
// standard crypto keeps the worker self-contained.
func SHA256Hex(b []byte) string {
	// Avoid pulling in crypto/sha256 import here — exported from
	// runtime.go for tests. Kept as a thin wrapper to keep a single
	// import location.
	return sha256Hex(b)
}

// SourceVersionBroadcaster is the seam that surfaces authority-side
// source-version changes to the worker. In production this is wired
// to the same publication channel that drives P5 card withdrawal;
// the worker treats any new value as a global cache-invalidation
// trigger for the prior version. Until wired, the worker uses the
// StandaloneSourceVersionClock below.
type SourceVersionBroadcaster interface {
	// Current returns the most recently observed source version.
	Current() int
	// OnAdvance registers a non-blocking callback invoked when the
	// observed source version advances. The callback MUST honor the
	// context cancellation; the broadcaster drops the callback after
	// the context is done.
	OnAdvance(ctx CancelableContext, fn func(newVersion int))
}

// CancelableContext is a tiny alias to avoid pulling context into
// this isolated module's API surface.
type CancelableContext interface {
	Done() <-chan struct{}
	Err() error
}

// StandaloneSourceVersionClock is the in-process default. Production
// replaces it with one wired to the publication channel. The clock is
// the audit trail the worker reads; advancing the clock is what
// invalidates the audio cache.
type StandaloneSourceVersionClock struct {
	mu        sync.Mutex
	version   int
	listeners []func(int)
}

// NewStandaloneSourceVersionClock returns a fresh clock at version v.
func NewStandaloneSourceVersionClock(v int) *StandaloneSourceVersionClock {
	return &StandaloneSourceVersionClock{version: v}
}

// Current returns the latest observed source version.
func (c *StandaloneSourceVersionClock) Current() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.version
}

// Advance bumps the version (monotonic). Returns the new version.
// Not allowed to regress; a smaller argument leaves the value
// untouched. Listeners are invoked WITHOUT holding the clock mutex
// so they can acquire downstream mutexes (e.g. the cache codec's
// mutex) without deadlocking against a Get racing with Advance.
func (c *StandaloneSourceVersionClock) Advance(to int) int {
	c.mu.Lock()
	if to <= c.version {
		c.mu.Unlock()
		return c.version
	}
	c.version = to
	fns := append([]func(int){}, c.listeners...)
	c.mu.Unlock()
	for _, fn := range fns {
		fn(c.version)
	}
	return c.version
}

// OnAdvance registers a synchronous listener that is invoked under
// the clock's lock when Advance increments the version. Listeners
// must not block; for production use the cache subscribes with a
// non-blocking channel hop.
func (c *StandaloneSourceVersionClock) OnAdvance(_ CancelableContext, fn func(newVersion int)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listeners = append(c.listeners, fn)
}
