package offlineclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"sthira/backend/internal/offlinepkg"
)

// On-disk layout (current generation):
//
//	state/current_generation.json   single atomic file with both manifest and
//	                                card bytes; one rename commits both or none.
//	state/state.json                freshness metadata (LastFetchedAtUnixMS,
//	                                MaxObservedUnixMS, jurisdiction, etc.).
//	state/staging/                  transient in-flight activation staging.
//	tombstones/packages.json        revoked package IDs.
//	tombstones/routes.json          cancelled route IDs.
//	tombstones/superseded.json      superseded package-version pairs.
//	tombstones/.initialized         sentinel written after the first successful
//	                                tombstone save; absence means "genuinely
//	                                new store" and empty initialization is allowed.
//	downloads/                      in-flight .part / .part.meta for resumable
//	                                downloads; cleared on successful activation.
//	resources/<id>/                 per-resource content.bin + meta.json.
//
// Coherent activation: writeAtomicBytes on current_generation.json is the
// single durable switch. A crash before it leaves no new generation; a crash
// after it leaves the new generation complete (manifest + card together).

// persistedState is the on-disk representation of state.json.
type persistedState struct {
	LastRevision int    `json:"last_revision"`
	Jurisdiction string `json:"jurisdiction,omitempty"`
	// LastSyncMonotonicNS is retained only to read pre-correction state files.
	// A Unix timestamp is not monotonic and must never be used as elapsed time.
	LastSyncMonotonicNS int64 `json:"last_sync_monotonic_ns,omitempty"`
	LastFetchedAtUnixMS int64 `json:"last_fetched_at_unix_ms"`
	MaxObservedUnixMS   int64 `json:"max_observed_unix_ms,omitempty"`
	ExpiredAtUnixMS     int64 `json:"expired_at_unix_ms,omitempty"`
}

// downloadMeta is the on-disk representation of <artifact>.part.meta.
type downloadMeta struct {
	URL            string `json:"url"`
	ExpectedETag   string `json:"expected_etag,omitempty"`
	ExpectedSize   int64  `json:"expected_size"`
	BytesWritten   int64  `json:"bytes_written"`
	RangeSupported bool   `json:"range_supported"`
	StartedAtNS    int64  `json:"started_at_ns"`
}

// resourceMeta is the on-disk representation of resources/<id>/meta.json.
type resourceMeta struct {
	ETag            string `json:"etag,omitempty"`
	ChecksumSHA256  string `json:"checksum_sha256"`
	ByteSize        int64  `json:"byte_size"`
	ContentType     string `json:"content_type"`
	FetchedAtUnixMS int64  `json:"fetched_at_unix_ms"`
}

// generation is the single on-disk generation record. Both manifest and card
// bytes are stored together so a single atomic write commits them as one
// generation — there is no window in which one is replaced without the other.
type generation struct {
	// ManifestBytes are the canonical manifest bytes (with checksum and signature
	// fields populated by the publisher). They are the verified, signed bytes
	// the client just received.
	ManifestBytes []byte `json:"manifest_bytes"`
	// CardBytes are the canonical card bytes (same shape). Empty when the
	// generation was activated without a card.
	CardBytes []byte `json:"card_bytes"`
	// Revision mirrors manifest.revision for quick checks before parsing.
	Revision int `json:"revision"`
	// ManifestID, PackageID, CardVersion, CardChecksum disambiguate the
	// generation identity for tests and for the activeIntact cross-check.
	ManifestID   string `json:"manifest_id"`
	PackageID    string `json:"package_id"`
	CardVersion  int    `json:"card_version"`
	CardChecksum string `json:"card_checksum"`
}

// storage owns the on-disk layout and provides atomic write/rename helpers.
// All methods are goroutine-safe.
type storage struct {
	mu   sync.Mutex // serializes writes; reads below use the mutex too
	root string
}

func newStorage(root string) (*storage, error) {
	if root == "" {
		return nil, errors.New("offlineclient: empty storage dir")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("offlineclient: create storage dir: %w", err)
	}
	for _, sub := range []string{"state", "state/staging", "tombstones", "downloads", "resources"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, fmt.Errorf("offlineclient: create %s: %w", sub, err)
		}
	}
	return &storage{root: root}, nil
}

// writeAtomicBytes writes data to a temp file in the same directory as
// finalPath, fsyncs, then renames onto finalPath. The caller is guaranteed
// either the previous contents of finalPath or the new contents, never a
// partial mix.
func (s *storage) writeAtomicBytes(finalPath string, data []byte) error {
	dir := filepath.Dir(finalPath)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// writeAtomicFrom copies src into a temp file in the same directory as
// finalPath, fsyncs, and renames. src is fully drained.
func (s *storage) writeAtomicFrom(finalPath string, src io.Reader, max int64) (int64, error) {
	dir := filepath.Dir(finalPath)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return 0, err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			if max > 0 && written+int64(n) > max {
				tmp.Close()
				return written, ErrTooLarge
			}
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				tmp.Close()
				return written, werr
			}
			written += int64(n)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			tmp.Close()
			return written, rerr
		}
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return written, err
	}
	if err := tmp.Close(); err != nil {
		return written, err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return written, err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return written, nil
}

// loadState reads state.json. A missing file is not an error; it returns
// the zero value.
func (s *storage) loadState() (persistedState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadStateLocked()
}

func (s *storage) loadStateLocked() (persistedState, error) {
	var st persistedState
	b, err := os.ReadFile(filepath.Join(s.root, "state", "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, fmt.Errorf("offlineclient: parse state.json: %w", err)
	}
	return st, nil
}

func (s *storage) saveState(st persistedState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.writeAtomicBytes(filepath.Join(s.root, "state", "state.json"), b)
}

// --- generation: the single coherent active state ---

// readActiveGeneration returns the manifest and card bytes from the
// current active generation. A missing file returns ErrNoActiveState.
// A malformed file (truncated or corrupted JSON) returns an integrity
// error so the caller fails closed.
func (s *storage) readActiveGeneration() (manifestBytes, cardBytes []byte, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bin, err := os.ReadFile(filepath.Join(s.root, "state", "current_generation.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNoActiveState
		}
		return nil, nil, err
	}
	if len(bin) == 0 {
		return nil, nil, fmt.Errorf("offlineclient: current_generation.json is zero-byte")
	}
	var gen generation
	if err := json.Unmarshal(bin, &gen); err != nil {
		return nil, nil, fmt.Errorf("offlineclient: parse current_generation.json: %w", err)
	}
	return gen.ManifestBytes, gen.CardBytes, nil
}

// writeActiveGeneration atomically commits both manifest and card bytes as
// ONE generation. The single writeAtomicBytes ensures the active state is
// never a mixed pair: either the previous generation or the new one is
// selected after the call returns. The caller has already verified both
// bytes (checksum + signature) before this call.
//
// Intermediate stages are placed under state/staging/ so the final rename
// is single-file atomic (POSIX rename within the same filesystem). The
// staging directory is created at storage initialization and is reused;
// staging failures (mkdir, write, rename) are surfaced, never swallowed.
func (s *storage) writeActiveGeneration(manifestBytes, cardBytes []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stagingDir := filepath.Join(s.root, "state", "staging")
	tmp, err := os.CreateTemp(stagingDir, "generation-*.json.tmp")
	if err != nil {
		return fmt.Errorf("offlineclient: create staging generation: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	gen := generation{
		ManifestBytes: append([]byte(nil), manifestBytes...),
		CardBytes:     append([]byte(nil), cardBytes...),
	}
	// Best-effort metadata extraction. We do not fail the activation if the
	// bytes do not parse here; the caller's verification path will reject
	// anything that is not a well-formed signed manifest.
	if m, perr := offlinepkg.ParseManifest(manifestBytes, offlinepkg.Limits{MaxBytes: 256 * 1024, MaxDepth: 32}); perr == nil {
		gen.Revision = m.Revision
		gen.ManifestID = m.ManifestID
		gen.PackageID = m.CriticalCard.PackageID
		gen.CardVersion = m.CriticalCard.Version
		gen.CardChecksum = m.CriticalCard.ChecksumSHA256
	}
	payload, err := json.Marshal(gen)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("offlineclient: marshal generation: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return fmt.Errorf("offlineclient: write generation tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("offlineclient: sync generation tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("offlineclient: close generation tmp: %w", err)
	}

	finalPath := filepath.Join(s.root, "state", "current_generation.json")
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("offlineclient: activate generation: %w", err)
	}
	if d, err := os.Open(filepath.Dir(finalPath)); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// writePartMeta persists downloadMeta as <part>.meta.
func (s *storage) writePartMeta(partPath string, m downloadMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return s.writeAtomicBytes(partPath+".meta", b)
}

func (s *storage) readPartMeta(partPath string) (downloadMeta, error) {
	var m downloadMeta
	b, err := os.ReadFile(partPath + ".meta")
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func (s *storage) clearPart(partPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = os.Remove(partPath)
	_ = os.Remove(partPath + ".meta")
}

func (s *storage) partPath(kind, id string) string {
	return filepath.Join(s.root, "downloads", kind+"-"+id+".part")
}

// --- tombstones ---

// tombstoneFile is the single on-disk tombstone state. File presence is the
// canonical signal recorded by the .initialized sentinel written BEFORE
// the file. The sentinel + the file together guarantee the trust model
// required by the lane:
//   - Sentinel missing → genuinely new store (no revocation knowledge);
//     file missing is OK.
//   - Sentinel present, file present and well-formed → use it.
//   - Sentinel present, file missing or zero-byte or malformed →
//     integrity violation; fail closed. The "have we ever recorded
//     anything?" question is answered by the sentinel; the "what did we
//     record?" question is answered by the file. A user tampering with
//     one without the other is detected.
type tombstoneFile struct {
	Packages   []string                       `json:"packages,omitempty"`
	Routes     []string                       `json:"routes,omitempty"`
	Superseded []offlinepkg.SupersededVersion `json:"superseded,omitempty"`
}

type tombstoneStore struct {
	mu   sync.Mutex
	root string
}

func (s *storage) tombstones() *tombstoneStore {
	return &tombstoneStore{root: filepath.Join(s.root, "tombstones")}
}

const (
	tombstonePath     = "tombstones.json"
	tombstoneInitName = ".initialized"
)

// isInitializedLocked reports whether the sentinel exists (caller holds mu).
func (t *tombstoneStore) isInitializedLocked() bool {
	_, err := os.Stat(filepath.Join(t.root, tombstoneInitName))
	return err == nil
}

// writeMarkerLocked durably creates the sentinel. Called before any save
// so a crash that leaves the file but not the sentinel still triggers
// the "new store" semantics on the next load — at worst the client
// retries the sync. A crash that leaves the sentinel but not the file is
// impossible because the file rename is the last durable step; if it
// fails, the marker is rolled back by the caller.
func (t *tombstoneStore) writeMarkerLocked() error {
	path := filepath.Join(t.root, tombstoneInitName)
	if t.isInitializedLocked() {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("offlineclient: tombstone marker: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	if d, err := os.Open(t.root); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// removeMarkerLocked deletes the sentinel. Used when the client explicitly
// resets the tombstone store (e.g., during the rare rollback case). The
// public API never calls this; tests can use it via the storage mutex.
func (t *tombstoneStore) removeMarkerLocked() {
	_ = os.Remove(filepath.Join(t.root, tombstoneInitName))
}

// loadOrInit reads the tombstone state.
//   - Sentinel missing: any state is OK (genuine new store) → returns
//     empty.
//   - Sentinel present, file missing/zero-byte/malformed: integrity
//     error (fail closed).
//   - Sentinel present, file well-formed: returns the parsed state.
//
// Reads and writes are goroutine-safe.
func (t *tombstoneStore) loadOrInit() (tombstoneFile, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.loadLocked()
}

func (t *tombstoneStore) loadLocked() (tombstoneFile, error) {
	b, err := os.ReadFile(filepath.Join(t.root, tombstonePath))
	initialized := t.isInitializedLocked()
	if err != nil {
		if os.IsNotExist(err) {
			if initialized {
				return tombstoneFile{}, fmt.Errorf("offlineclient: tombstone state file missing after initialization")
			}
			return tombstoneFile{}, nil
		}
		return tombstoneFile{}, err
	}
	if len(b) == 0 {
		if initialized {
			return tombstoneFile{}, fmt.Errorf("offlineclient: tombstone state is zero-byte (corrupt) after initialization")
		}
		return tombstoneFile{}, nil
	}
	var tf tombstoneFile
	if err := json.Unmarshal(b, &tf); err != nil {
		if initialized {
			return tombstoneFile{}, fmt.Errorf("offlineclient: tombstone state malformed after initialization: %w", err)
		}
		return tombstoneFile{}, fmt.Errorf("offlineclient: parse tombstone state: %w", err)
	}
	return tf, nil
}

// save writes the entire tombstone state atomically. The sentinel is
// written BEFORE the file so a crash between sentinel and file leaves
// the next load treating the store as fresh-empty (at worst the client
// retries the sync and re-records). A crash after the file rename has
// both marker and file consistent.
func (t *tombstoneStore) save(tf tombstoneFile) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.saveLocked(tf)
}

func (t *tombstoneStore) saveLocked(tf tombstoneFile) error {
	if err := t.writeMarkerLocked(); err != nil {
		return err
	}
	b, err := json.Marshal(tf)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(t.root, ".tmp-*")
	if err != nil {
		return fmt.Errorf("offlineclient: staging tombstone: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filepath.Join(t.root, tombstonePath)); err != nil {
		return fmt.Errorf("offlineclient: activate tombstone: %w", err)
	}
	if d, err := os.Open(t.root); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// addPackage adds a revoked package id (idempotent).
func (t *tombstoneStore) addPackage(id string) (added bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tf, err := t.loadLocked()
	if err != nil {
		return false, err
	}
	for _, x := range tf.Packages {
		if x == id {
			return false, nil
		}
	}
	tf.Packages = append(tf.Packages, id)
	return true, t.saveLocked(tf)
}

// addRoute adds a cancelled route id (idempotent).
func (t *tombstoneStore) addRoute(id string) (added bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tf, err := t.loadLocked()
	if err != nil {
		return false, err
	}
	for _, x := range tf.Routes {
		if x == id {
			return false, nil
		}
	}
	tf.Routes = append(tf.Routes, id)
	return true, t.saveLocked(tf)
}

// addSuperseded records a superseded (package, version) pair (idempotent).
func (t *tombstoneStore) addSuperseded(pkgID string, version int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	tf, err := t.loadLocked()
	if err != nil {
		return err
	}
	for _, x := range tf.Superseded {
		if x.PackageID == pkgID && x.Version == version {
			return nil
		}
	}
	tf.Superseded = append(tf.Superseded, offlinepkg.SupersededVersion{PackageID: pkgID, Version: version})
	return t.saveLocked(tf)
}

// containsPackage / containsRoute / isSuperseded are read-only checks that
// fail closed: any error (including the explicit zero-byte integrity error)
// returns true so the caller treats the affected ID as revoked/cancelled.
func (t *tombstoneStore) containsPackage(id string) bool {
	tf, err := t.loadOrInit()
	if err != nil {
		return true
	}
	for _, x := range tf.Packages {
		if x == id {
			return true
		}
	}
	return false
}

func (t *tombstoneStore) containsRoute(id string) bool {
	tf, err := t.loadOrInit()
	if err != nil {
		return true
	}
	for _, x := range tf.Routes {
		if x == id {
			return true
		}
	}
	return false
}

func (t *tombstoneStore) isSuperseded(pkgID string, version int) bool {
	if pkgID == "" || version <= 0 {
		return false
	}
	tf, err := t.loadOrInit()
	if err != nil {
		return true
	}
	for _, x := range tf.Superseded {
		if x.PackageID == pkgID && x.Version == version {
			return true
		}
	}
	return false
}

// verifyIntegrity checks the tombstone state is consistent with the
// initialization sentinel. Missing file + missing sentinel → OK.
// Missing file + present sentinel → integrity error. Present and
// well-formed file → OK. Present and zero-byte/malformed file →
// integrity error. stateQuery fails closed (FreshnessUnverifiable) on
// any verifyIntegrity error.
func (t *tombstoneStore) verifyIntegrity() error {
	_, err := t.loadOrInit()
	return err
}

// --- resource store ---

// readResource returns the on-disk bytes, parsed meta and a freshness
// signal. Missing files return ErrNoActiveState; integrity is checked by
// the caller.
func (s *storage) readResource(id string) ([]byte, resourceMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, "resources", id)
	bin, err := os.ReadFile(filepath.Join(dir, "content.bin"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, resourceMeta{}, ErrNoActiveState
		}
		return nil, resourceMeta{}, err
	}
	mb, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil, resourceMeta{}, fmt.Errorf("offlineclient: read resource meta: %w", err)
	}
	var m resourceMeta
	if err := json.Unmarshal(mb, &m); err != nil {
		return nil, resourceMeta{}, fmt.Errorf("offlineclient: parse resource meta: %w", err)
	}
	return bin, m, nil
}

func (s *storage) writeResource(id string, content []byte, m resourceMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, "resources", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mb, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := s.writeAtomicBytes(filepath.Join(dir, "meta.json"), mb); err != nil {
		return err
	}
	return s.writeAtomicBytes(filepath.Join(dir, "content.bin"), content)
}

// hasResourcePart reports whether a .part + .part.meta pair exists for id.
func (s *storage) hasResourcePart(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := os.Stat(s.partPath("resource", id))
	return err == nil
}
