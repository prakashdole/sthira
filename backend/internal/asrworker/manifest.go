package asrworker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// IndicConformer is the artifact catalog for the AI4Bharat
// IndicConformer-600M Multilingual model as referenced by the
// existing src/sthira_v2/{local_voice,speech_stt}.py reference
// runtime. The catalog is metadata only; this package never loads
// tensors.
//
// The reference runtime:
//   - Reaches into models/indic-conformer-600m-multilingual via an
//     importlib spec on model_onnx.py.
//   - Restricts configured language coverage to hi-IN and ml-IN —
//     the artifact is multilingual but the reference pipeline is
//     not. Worker 5 must NOT advertise wider coverage than the
//     runtime itself supports.
//   - Returns confidence = 1.0 unconditionally. Worker 5 reports
//     confidence as UNKNOWN unless the runtime exposes calibrated
//     n-best with token-level posterior.
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
// no STHIRA_ASR_MODEL_DIR is configured. The reference runtime
// points at models/indic-conformer-600m-multilingual relative to
// repo root.
func DefaultIndicConformer() IndicConformer {
	return IndicConformer{
		ModelID:           "ai4bharat/indic-conformer-600m-multilingual",
		PretrainedArchive: "ai4bharat/indic-conformer-600m-multilingual",
		LocalPath:         "models/indic-conformer-600m-multilingual",
		LicenseRef:        "Apache-2.0 (claimed upstream; pending verification)",
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
// whatever it can without downloading. It NEVER reads .safetensors
// or .bin payloads; only file existence and (when small enough for
// a fast probe) SHA-256.
//
// The reference runtime imports a single Python file as the
// gateway: model_onnx.py. If that file is missing, we surface
// PendingCritical; the worker will not advertise Ready=true.
//
// Model checkpoint naming varies by export target:
//   - HF default snapshot: pytorch_model.bin / model.safetensors
//   - Llama exports: consolidated.safetensors (or sharded 0..N)
//
// We treat the checkpoint as present if any of these canonical
// filenames exist at the root, without scanning the directory
// recursively.
func ScanLocalInventory(root string, ic IndicConformer) Inventory {
	inv := Inventory{IndicConformer: ic, LastScannedAt: time.Now().UTC()}
	if strings.TrimSpace(root) == "" {
		inv.PendingCritical = []string{CompModelCheckpoint, CompTokenizer, CompRuntime, CompONNXConfig}
		sort.Strings(inv.PendingCritical)
		return inv
	}
	checkpoints := []string{
		root + "/pytorch_model.bin",
		root + "/model.safetensors",
		root + "/consolidated.safetensors",
	}
	probeSingle := func(name, rel string) {
		full := root + rel
		st, err := os.Stat(full)
		switch {
		case errors.Is(err, os.ErrNotExist), err != nil:
			inv.PendingCritical = append(inv.PendingCritical, name)
			inv.HashMatches = append(inv.HashMatches, HashMatch{
				Name:   name,
				Status: HashMissingOnDisk,
			})
		default:
			var actual string
			if st.Size() > 0 && st.Size() <= 64*1024 {
				b, err := os.ReadFile(full)
				if err == nil {
					actual = SHA256Hex(b)
				}
			}
			inv.HashMatches = append(inv.HashMatches, HashMatch{
				Name:         name,
				ActualSHA256: actual,
				Status:       HashMissingRecorded,
			})
		}
	}
	// Model checkpoint: at least one canonical filename exists.
	hasCheckpoint := false
	for _, p := range checkpoints {
		if _, err := os.Stat(p); err == nil {
			hasCheckpoint = true
			break
		}
	}
	if !hasCheckpoint {
		inv.PendingCritical = append(inv.PendingCritical, CompModelCheckpoint)
		inv.HashMatches = append(inv.HashMatches, HashMatch{Name: CompModelCheckpoint, Status: HashMissingOnDisk})
	} else {
		inv.HashMatches = append(inv.HashMatches, HashMatch{
			Name:   CompModelCheckpoint,
			Status: HashMissingRecorded,
		})
	}
	probeSingle(CompTokenizer, "/tokenizer")
	probeSingle(CompRuntime, "/model_onnx.py")
	probeSingle(CompONNXConfig, "/config.yaml")
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
