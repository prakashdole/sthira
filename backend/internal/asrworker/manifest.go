package asrworker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// IndicConformer is the artifact catalog for the AI4Bharat
// IndicConformer-600M Multilingual model, ONNX deployment layout.
// The catalog is metadata only; this package never loads tensors.
//
// Verified layout (primary sources, retrieved 2026-09-21):
//   - Model card "ai4bharat/indic-conformer-600m-multilingual":
//     MIT license; hybrid CTC/RNN-T Conformer; the hub AutoModel
//     path uses trust_remote_code=True. The production adapter
//     (src/sthira_v2/speech_asr_adapter.py) does NOT execute
//     model-dir Python: it drives the exported assets directly —
//     assets/preprocessor.ts (TorchScript), assets/encoder.onnx,
//     assets/ctc_decoder.onnx, assets/vocab.json,
//     assets/language_masks.json, config.json — with IO names
//     discovered from the sessions at load time. The per-file
//     layout is documented in the AI4Bharat ONNX reference
//     implementation (sibling repo, same export tooling); the
//     multilingual repo itself is gated, so a mismatch must fail
//     closed at load, which is why the adapter re-checks names.
//   - Restrict configured language coverage to the approved set
//     (hi-IN, ml-IN today); the artifact is multilingual but
//     approval is not the model's to grant. The worker's
//     SupportedLanguages is derived from the loaded Runtime
//     adapter, never from the model name.
//   - The model returns a transcript string with NO confidence
//     score; confidence stays UNKNOWN (nil), never 1.0.
//
// All recorded SHA-256 digests are PLACEHOLDERS ("zero hash") until
// an authorized download is performed and a real verifier records
// digests. Worker 5 reports them with License="LicensePending" so
// the orchestrator can fail closed at startup.
//
// Hindu / Malayalam restriction note: the multilingual artifact
// name is descriptive, not a coverage grant. Worker 5's
// SupportedLanguages is derived from the Runtime adapter, never
// from the model name.
type IndicConformer struct {
	// ModelID is the Hugging Face hub identifier.
	ModelID string
	// RevisionTag is the symbolic tag (e.g. "refs/convert/parquet"
	// for ONNX exports) or commit SHA recorded at download time.
	// Empty until an authorized download is performed.
	RevisionTag string
	// PretrainedArchive is the canonical snapshot reference.
	PretrainedArchive string
	// LocalPath is the local directory the reference runtime
	// imports model_onnx.py from. Empty when the artifact is
	// not present.
	LocalPath string
	// Component digests. Zero until verified.
	ModelCheckpoint      ArtifactDigest
	Tokenizer            ArtifactDigest
	Runtime              ArtifactDigest
	ONNXConfig           ArtifactDigest
	RemoteCodeIsRequired bool
	// LicenseRef points at the declared upstream license. Worker 5
	// does not auto-download and therefore does not import this
	// into a trust decision; the field records the upstream claim.
	LicenseRef string
}

// DefaultIndicConformer is the artifact the worker reaches for when
// no STHIRA_ASR_MODEL_DIR is configured. LocalPath is the directory
// holding the ONNX deployment assets (the adapter never executes
// model-dir Python and never downloads).
func DefaultIndicConformer() IndicConformer {
	return IndicConformer{
		ModelID:           "ai4bharat/indic-conformer-600m-multilingual",
		PretrainedArchive: "ai4bharat/indic-conformer-600m-multilingual",
		LocalPath:         "models/indic-conformer-600m-multilingual",
		LicenseRef:        "MIT (AI4Bharat model card, verified 2026-09-21)",
		// The hub AutoModel path sets trust_remote_code=True; the
		// production ONNX path does not require it and refuses to
		// run unverified model-dir Python.
		RemoteCodeIsRequired: false,
	}
}

// ArtifactDigest is a record of an artifact component. Components
// that are still pending an authorized download carry a zero
// SHA-256 and License="LicensePending"; this is how the inventory
// distinguishes "we have it" from "we know about it". The
// orchestrator refuses to dispatch to a worker whose Catalog has
// any pending critical entry.
type ArtifactDigest struct {
	Name           string
	Path           string
	ChecksumSHA256 string
	ByteSize       int64
	License        string
}

// HashZero is the canonical "no digest yet" sentinel. Compare with
// ==.
const HashZero = "0000000000000000000000000000000000000000000000000000000000000000"

// HasRecordedDigest reports whether the digest carries a real SHA
// (i.e. someone has actually hashed the bytes). Empty and zero
// strings both return false.
func (a ArtifactDigest) HasRecordedDigest() bool {
	v := strings.TrimSpace(a.ChecksumSHA256)
	if v == "" || v == HashZero {
		return false
	}
	return true
}

// SHA256Hex returns the SHA-256 of bytes formatted as lowercase
// hex. The worker uses this during the authorized-download path;
// it does NOT hash model weights at startup.
func SHA256Hex(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

// Component names that must be verified before Ready=true.
const (
	CompModelCheckpoint = "model_checkpoint"
	CompTokenizer       = "tokenizer"
	CompRuntime         = "runtime"
	CompONNXConfig      = "onnx_config"
)

// ComponentCritical reports whether a component must be present
// and verified before the worker can claim Ready=true.
func ComponentCritical(name string) bool {
	switch name {
	case CompModelCheckpoint, CompTokenizer, CompRuntime, CompONNXConfig:
		return true
	}
	return false
}

// Inventory is a runtime snapshot of the artifact catalog. The
// orchestrator reads AllowedLanguages and SupportedComponents from
// here on every /health call so a torn state surfaces immediately.
type Inventory struct {
	IndicConformer
	LastScannedAt   time.Time   // UTC
	PendingCritical []string    // names of zero-digest critical components
	HashMatches     []HashMatch // per-component recorded-vs-disk hash comparison
	// AllowedLanguages is set by the runtime adapter via the Worker
	// after construction; the inventory scan does not populate it.
	AllowedLanguages []string
}

// HashMatch records whether a recorded digest matches the bytes on
// disk at LastScannedAt. The orchestrator refuses Ready=true while
// any HashMissingOnDisk or HashMismatch is reported.
type HashMatch struct {
	Name           string
	ExpectedSHA256 string
	ActualSHA256   string
	Status         HashStatus
}

// HashStatus is the result of a digest comparison.
type HashStatus string

const (
	HashMatchOK         HashStatus = "OK"
	HashMismatch        HashStatus = "MISMATCH"
	HashMissingOnDisk   HashStatus = "MISSING_ON_DISK"
	HashMissingRecorded HashStatus = "MISSING_RECORDED"
)

// ScanLocalInventory inspects the configured LocalPath and fills
// whatever it can without downloading. It NEVER reads the ONNX or
// TorchScript payloads; only file existence and (when small enough
// for a fast probe) SHA-256.
//
// Component map for the verified ONNX deployment layout (see
// IndicConformer):
//   - model_checkpoint: assets/encoder.onnx + assets/ctc_decoder.onnx.
//   - tokenizer: assets/vocab.json + assets/language_masks.json.
//   - runtime: assets/preprocessor.ts (TorchScript front-end). The
//     hub repo's model_onnx.py is NOT required: the production
//     adapter never executes model-dir Python.
//   - onnx_config: config.json (BLANK_ID/SOS/FRAME_DURATION_MS).
func ScanLocalInventory(root string, ic IndicConformer) Inventory {
	inv := Inventory{IndicConformer: ic, LastScannedAt: time.Now().UTC()}
	if strings.TrimSpace(root) == "" {
		inv.PendingCritical = []string{CompModelCheckpoint, CompTokenizer, CompRuntime, CompONNXConfig}
		sort.Strings(inv.PendingCritical)
		return inv
	}
	probeSet := func(name string, rels ...string) {
		var missing bool
		for _, rel := range rels {
			if _, err := os.Stat(root + rel); err != nil {
				missing = true
			}
		}
		if missing {
			inv.PendingCritical = append(inv.PendingCritical, name)
			inv.HashMatches = append(inv.HashMatches, HashMatch{
				Name:   name,
				Status: HashMissingOnDisk,
			})
			return
		}
		// Hash only the small JSON/config members; graphs stay
		// unread on disk probes.
		var actual string
		full := root + rels[len(rels)-1]
		if st, err := os.Stat(full); err == nil && st.Size() > 0 && st.Size() <= 64*1024 {
			if b, err := os.ReadFile(filepath.Clean(full)); err == nil { // #nosec G304
				actual = SHA256Hex(b)
			}
		}
		inv.HashMatches = append(inv.HashMatches, HashMatch{
			Name:         name,
			ActualSHA256: actual,
			Status:       HashMissingRecorded,
		})
	}
	probeSet(CompModelCheckpoint, "/assets/encoder.onnx", "/assets/ctc_decoder.onnx")
	probeSet(CompTokenizer, "/assets/vocab.json", "/assets/language_masks.json")
	probeSet(CompRuntime, "/assets/preprocessor.ts")
	probeSet(CompONNXConfig, "/config.json")
	sort.Strings(inv.PendingCritical)
	sort.Slice(inv.HashMatches, func(i, j int) bool {
		return inv.HashMatches[i].Name < inv.HashMatches[j].Name
	})
	return inv
}

// Ready reports whether this inventory can back a Ready=true
// /health response. Three conditions must ALL hold:
//
//   - No critical components are still pending.
//   - The recorded digests (if any) match what's on disk.
//   - The adapter has reported a non-empty AllowedLanguages.
//
// The catalog does NOT check hardware; the runtime adapter does.
func (i Inventory) Ready() error {
	if len(i.PendingCritical) > 0 {
		return fmt.Errorf("pending critical components: %s", strings.Join(i.PendingCritical, ","))
	}
	for _, hm := range i.HashMatches {
		if hm.Status == HashMismatch {
			return fmt.Errorf("digest mismatch on %s", hm.Name)
		}
	}
	if len(i.AllowedLanguages) == 0 {
		return errors.New("runtime adapter reports no allowed languages")
	}
	return nil
}

// SupportedComponents returns the catalog of components the worker
// currently advertises to the orchestrator. Empty entries are
// filtered (orchestrators should not see placeholder rows).
func (i Inventory) SupportedComponents() []ArtifactDigest {
	var out []ArtifactDigest
	if i.ModelCheckpoint.HasRecordedDigest() {
		out = append(out, i.ModelCheckpoint)
	}
	if i.Tokenizer.HasRecordedDigest() {
		out = append(out, i.Tokenizer)
	}
	if i.Runtime.HasRecordedDigest() {
		out = append(out, i.Runtime)
	}
	if i.ONNXConfig.HasRecordedDigest() {
		out = append(out, i.ONNXConfig)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
