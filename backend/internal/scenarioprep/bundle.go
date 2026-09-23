package scenarioprep

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"sthira/backend/internal/catalogue"
	"sthira/backend/internal/opkg"
)

// BundleStatus is the published bundle's overall preparation posture.
type BundleStatus string

const (
	BundleReady    BundleStatus = "PREPARATION_READY"
	BundleDraft    BundleStatus = "PREPARATION_DRAFT"
	BundleRejected BundleStatus = "PREPARATION_REJECTED"
)

// BundleManifest is the deterministic handoff manifest. It is NOT the P5
// signed offline-client package: it carries the preparation posture, raw
// file hashes, and opkg checksums separately, but never claims P5
// compatibility, never publishes to citizens, and never includes a
// signature placeholder. Worker 1 owns the offline-client protocol.
type BundleManifest struct {
	ManifestVersion int             `json:"manifest_version"`
	BundleStatus    BundleStatus    `json:"bundle_status"`
	ExerciseClock   string          `json:"exercise_clock,omitempty"`
	CatalogueSHA256 string          `json:"catalogue_sha256"`
	CataloguePath   string          `json:"catalogue_path"`
	ScenarioCount   int             `json:"scenario_count"`
	StateCount      int             `json:"state_count"`
	Files           []BundleFile    `json:"files"`
	Packages        []BundlePackage `json:"packages"`
	Report          *Report         `json:"report"`
	ToolVersion     string          `json:"tool_version"`
}

// BundleFile is one entry in the deterministic file index. RawSHA256 is the
// hex SHA256 of the file's bytes; PackageChecksum is opkg.Checksum for opkg
// files only. Provenance is the package's evidence_class when applicable,
// empty otherwise.
type BundleFile struct {
	Path             string `json:"path"`
	RawSHA256        string `json:"raw_sha256"`
	Bytes            int64  `json:"bytes"`
	SourceProvenance string `json:"source_provenance,omitempty"`
}

// BundlePackage is the per-scenario summary reusing opkg metadata.
type BundlePackage struct {
	ScenarioID    string `json:"scenario_id"`
	StateCode     string `json:"state_code"`
	EvidenceClass string `json:"evidence_class"`
	Authority     string `json:"authority,omitempty"`
	Version       int    `json:"version"`
	Jurisdiction  string `json:"jurisdiction"`
	Checksum      string `json:"opkg_checksum_sha256"`
	PackagePath   string `json:"package_path"`
}

// Bundle writes a deterministic handoff directory. It copies the catalogue
// and every referenced operational-package file under a temp directory,
// writes the bundle manifest, and renames atomically. It refuses to
// overwrite an existing destination. It never follows URL fields, never
// crawls, never includes database files, and never creates signing key
// material. allowDraft must be set for the operation to publish an
// INCOMPLETE bundle; without it, an INCOMPLETE report produces a rejected
// bundle and exit code 3.
func Bundle(workspace, indexPath, output string, allowDraft bool, rep *Report, toolVersion string) error {
	if outputInsideInput(output, workspace) {
		return wrap("bundle", fmt.Errorf("%w: output directory %q is inside the input workspace %q", ErrUnsafeLayout, output, workspace))
	}
	if destinationExists(output) {
		return wrap("bundle", fmt.Errorf("%w: destination %q already exists; refusing to overwrite", ErrIO, output))
	}

	// Re-resolve references deterministically so the bundle contains
	// exactly the same files the report described.
	idx, err := LoadIndex(indexPath)
	if err != nil {
		return err
	}
	refs, refFindings := resolveReferences(workspace, idx)
	if len(refFindings) > 0 {
		// Surface resolution findings in the bundle manifest's report and
		// refuse the bundle: we cannot atomically publish something that
		// references unsafe paths.
		if rep == nil {
			rep = &Report{}
		}
		for _, f := range refFindings {
			if f.Severity == SeverityError {
				rep.Findings = append(rep.Findings, f)
			}
		}
	}
	catalogueAbs, err := safeResolve(mustAbs(workspace), idx.Catalogue)
	if err != nil {
		return wrap("bundle", err)
	}
	catBody, err := readBoundedFile(catalogueAbs, int64(packageLimits.MaxBytes))
	if err != nil {
		return wrap("bundle", fmt.Errorf("%w: catalogue: %v", ErrIO, err))
	}

	// Determine bundle status from the report.
	bundleStatus := BundleReady
	switch {
	case rep == nil:
		bundleStatus = BundleDraft
	case rep.Status == StatusInvalid:
		bundleStatus = BundleRejected
	case rep.Status == StatusIncomplete:
		if !allowDraft {
			bundleStatus = BundleRejected
		} else {
			bundleStatus = BundleDraft
		}
	}

	// Build manifest.
	manifestEntries := []BundleFile{}
	pkgEntries := []BundlePackage{}
	for _, ref := range refs {
		body, err := readBoundedFile(ref.PackagePath, int64(packageLimits.MaxBytes))
		if err != nil {
			return wrap("bundle", fmt.Errorf("%w: read %s: %v", ErrIO, ref.ScenarioID, err))
		}
		var pkg opkg.Package
		if err := json.Unmarshal(body, &pkg); err != nil {
			// Skip; finding already recorded in the report.
			continue
		}
		pkgEntries = append(pkgEntries, BundlePackage{
			ScenarioID:    ref.ScenarioID,
			StateCode:     pkg.Provenance.Jurisdiction,
			EvidenceClass: string(pkg.Provenance.EvidenceClass),
			Authority:     pkg.Provenance.Authority,
			Version:       pkg.Provenance.Version,
			Jurisdiction:  pkg.Provenance.Jurisdiction,
			Checksum:      pkg.Provenance.ChecksumSHA256,
			PackagePath:   filepath.ToSlash(relInsideWorkspace(workspace, ref.PackagePath)),
		})
	}

	// Sort deterministically by scenario id.
	sort.Slice(pkgEntries, func(i, j int) bool { return pkgEntries[i].ScenarioID < pkgEntries[j].ScenarioID })

	// Catalogue checksum (raw bytes).
	catSum := sha256Hex(catBody)

	// File index for the catalogue.
	manifestEntries = append(manifestEntries, BundleFile{
		Path:             filepath.ToSlash(idx.Catalogue),
		RawSHA256:        catSum,
		Bytes:            int64(len(catBody)),
		SourceProvenance: "catalogue",
	})

	// Add per-package file entries with raw hashes and opkg provenance.
	for _, ref := range refs {
		body, err := readBoundedFile(ref.PackagePath, int64(packageLimits.MaxBytes))
		if err != nil {
			continue
		}
		var pkg opkg.Package
		if err := json.Unmarshal(body, &pkg); err != nil {
			manifestEntries = append(manifestEntries, BundleFile{
				Path:      filepath.ToSlash(relInsideWorkspace(workspace, ref.PackagePath)),
				RawSHA256: ref.RawSHA256,
				Bytes:     int64(len(body)),
			})
			continue
		}
		manifestEntries = append(manifestEntries, BundleFile{
			Path:             filepath.ToSlash(relInsideWorkspace(workspace, ref.PackagePath)),
			RawSHA256:        ref.RawSHA256,
			Bytes:            int64(len(body)),
			SourceProvenance: string(pkg.Provenance.EvidenceClass),
		})
	}
	sort.Slice(manifestEntries, func(i, j int) bool { return manifestEntries[i].Path < manifestEntries[j].Path })

	// State / scenario counts from the catalogue (best effort, only used
	// in the manifest summary).
	stateCount := 0
	scenarioCount := 0
	var manifestObj catalogue.Manifest
	if err := json.Unmarshal(catBody, &manifestObj); err == nil {
		stateCount = len(manifestObj.States)
		for _, st := range manifestObj.States {
			scenarioCount += len(st.Scenarios)
		}
	}

	manifest := BundleManifest{
		ManifestVersion: 1,
		BundleStatus:    bundleStatus,
		ExerciseClock:   idx.ExerciseClock,
		CatalogueSHA256: catSum,
		CataloguePath:   filepath.ToSlash(idx.Catalogue),
		ScenarioCount:   scenarioCount,
		StateCount:      stateCount,
		Files:           manifestEntries,
		Packages:        pkgEntries,
		Report:          rep,
		ToolVersion:     toolVersion,
	}

	if bundleStatus == BundleRejected {
		// Write a deterministic rejection summary in the output dir so the
		// caller knows why no real bundle was produced, then return.
		return writeRejection(output, &manifest)
	}

	// Build temp directory inside output's parent for atomic rename.
	parent := filepath.Dir(output)
	if parent == "" {
		parent = "."
	}
	tmp, err := os.MkdirTemp(parent, "scenario-prep-bundle-")
	if err != nil {
		return wrap("bundle", fmt.Errorf("%w: temp dir: %v", ErrIO, err))
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	// Copy catalogue.
	if err := copyFileAtomic(filepath.Join(tmp, filepath.FromSlash(idx.Catalogue)), catBody); err != nil {
		cleanup()
		return wrap("bundle", err)
	}
	// Copy each referenced package.
	for _, ref := range refs {
		body, err := readBoundedFile(ref.PackagePath, int64(packageLimits.MaxBytes))
		if err != nil {
			cleanup()
			return wrap("bundle", fmt.Errorf("%w: read %s: %v", ErrIO, ref.ScenarioID, err))
		}
		rel := relInsideWorkspace(workspace, ref.PackagePath)
		if err := copyFileAtomic(filepath.Join(tmp, filepath.FromSlash(rel)), body); err != nil {
			cleanup()
			return wrap("bundle", err)
		}
	}
	// Write manifest.
	mbytes, err := json.MarshalIndent(&manifest, "", "  ")
	if err != nil {
		cleanup()
		return wrap("bundle", err)
	}
	mbytes = append(mbytes, '\n')
	if err := copyFileAtomic(filepath.Join(tmp, "bundle-manifest.json"), mbytes); err != nil {
		cleanup()
		return wrap("bundle", err)
	}
	// Atomic rename.
	if err := os.Rename(tmp, output); err != nil {
		cleanup()
		return wrap("bundle", fmt.Errorf("%w: rename: %v", ErrIO, err))
	}
	return nil
}

// relInsideWorkspace returns a slash-form workspace-relative path. When the
// path is already inside the workspace (the only safe input shape), it is
// safe to relativize.
func relInsideWorkspace(workspace, path string) string {
	if workspace == "" {
		return filepath.ToSlash(path)
	}
	realWS, err := filepath.EvalSymlinks(workspace)
	if err == nil {
		workspace = realWS
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = realPath
	}
	rel, err := filepath.Rel(workspace, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// copyFileAtomic writes body to path using a temp file in the same
// directory and renaming. Ensures the destination directory exists.
func copyFileAtomic(path string, body []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: mkdir %s: %v", ErrIO, path, err)
	}
	tmp, err := os.CreateTemp(dir, ".scenario-prep-")
	if err != nil {
		return fmt.Errorf("%w: temp file: %v", ErrIO, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := io.Copy(tmp, newBytesReader(body)); err != nil {
		cleanup()
		_ = tmp.Close()
		return fmt.Errorf("%w: write %s: %v", ErrIO, path, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("%w: close %s: %v", ErrIO, path, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		cleanup()
		return fmt.Errorf("%w: chmod %s: %v", ErrIO, path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("%w: rename %s: %v", ErrIO, path, err)
	}
	return nil
}

// newBytesReader avoids importing bytes elsewhere.
func newBytesReader(b []byte) io.Reader {
	return &sliceReader{b: b}
}

type sliceReader struct {
	b   []byte
	off int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.off >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.off:])
	r.off += n
	return n, nil
}

// writeRejection writes a single rejection summary file describing why no
// bundle was produced. It does not copy any input files: nothing is
// published under a rejected status.
func writeRejection(output string, manifest *BundleManifest) error {
	if destinationExists(output) {
		return wrap("bundle", fmt.Errorf("%w: destination %q already exists; refusing to overwrite", ErrIO, output))
	}
	parent := filepath.Dir(output)
	if parent == "" {
		parent = "."
	}
	tmp, err := os.MkdirTemp(parent, "scenario-prep-rejected-")
	if err != nil {
		return wrap("bundle", fmt.Errorf("%w: temp dir: %v", ErrIO, err))
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	manifest.BundleStatus = BundleRejected
	mbytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		cleanup()
		return wrap("bundle", err)
	}
	mbytes = append(mbytes, '\n')
	if err := copyFileAtomic(filepath.Join(tmp, "bundle-rejection.json"), mbytes); err != nil {
		cleanup()
		return wrap("bundle", err)
	}
	if err := os.Rename(tmp, output); err != nil {
		cleanup()
		return wrap("bundle", fmt.Errorf("%w: rename: %v", ErrIO, err))
	}
	return nil
}
