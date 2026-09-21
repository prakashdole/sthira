package corpus

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SourceLoc points to a corpus file with its provenance class. Only
// ClassSynthetic is git-trackable; ClassConsentManifest and
// ClassRecordedAudio are external paths. The runner refuses to load a
// file that mislabels itself.
type SourceLoc struct {
	Path  string
	Class SourceClass
}

// SourceClass discriminates file use.
type SourceClass string

const (
	SourceClassSynthetic       SourceClass = "SYNTHETIC_FIXTURE"
	SourceClassConsentManifest SourceClass = "CONSENT_MANIFEST"
	SourceClassRecordedAudio   SourceClass = "RECORDED_AUDIO"
)

// LoadOptions controls how the loader treats external content.
type LoadOptions struct {
	// ReadFile is the file reader; tests inject a sandboxed reader.
	// Default: os.ReadFile.
	ReadFile func(path string) ([]byte, error)
	// AuditTrail is invoked once per loaded file with its path, byte
	// count and sha256. The runner prints a stable identifier the
	// evaluator can paste into evidence records.
	AuditTrail func(path string, bytes int, sha256hex string)
}

// Load reads every source, parses JSON, validates probes, and audits.
// All files must share the same SchemaVersion.
func Load(locs []SourceLoc, opts LoadOptions) (Suite, error) {
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	merged := Suite{SchemaVersion: SchemaVersion}
	seenFile := map[string]struct{}{}
	for _, loc := range locs {
		if _, dup := seenFile[loc.Path]; dup {
			return Suite{}, fmt.Errorf("duplicate source: %s", loc.Path)
		}
		seenFile[loc.Path] = struct{}{}
		switch loc.Class {
		case SourceClassSynthetic, SourceClassConsentManifest:
			// ok
		case SourceClassRecordedAudio:
			// recorded audio paths are referenced by manifest; we
			// never load them as JSON.
			if opts.AuditTrail != nil {
				f, err := opts.ReadFile(loc.Path)
				if err != nil {
					return Suite{}, fmt.Errorf("audio not readable at %s: %w", loc.Path, err)
				}
				opts.AuditTrail(loc.Path, len(f), contentDigest(f))
			}
			continue
		default:
			return Suite{}, fmt.Errorf("unknown source class: %s", loc.Class)
		}
		data, err := opts.ReadFile(loc.Path)
		if err != nil {
			return Suite{}, fmt.Errorf("read %s: %w", loc.Path, err)
		}
		s, err := Parse(data)
		if err != nil {
			return Suite{}, fmt.Errorf("parse %s: %w", loc.Path, err)
		}
		if opts.AuditTrail != nil {
			opts.AuditTrail(loc.Path, len(data), contentDigest(data))
		}
		merged.Cases = append(merged.Cases, s.Cases...)
	}
	if err := ValidateSuite(merged); err != nil {
		return Suite{}, fmt.Errorf("merged suite: %w", err)
	}
	return merged, nil
}

// Discover walks a directory and returns every *.json as a synthetic
// fixture loc. Real-consent manifests are NEVER discovered by walking;
// they are explicit.
func Discover(root string) ([]SourceLoc, error) {
	var locs []SourceLoc
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		locs = append(locs, SourceLoc{Path: path, Class: SourceClassSynthetic})
		return nil
	})
	return locs, err
}

// Copy ensures each source's bytes pass through a stable wrapper so
// streaming readers can register an audit hook.
func Copy(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
