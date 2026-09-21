package offlineclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sthira/backend/internal/offlinepkg"
)

// persistedState is the on-disk representation of state.json.
type persistedState struct {
	LastRevision int `json:"last_revision"`
	// LastSyncMonotonicNS is retained only to read pre-correction state files.
	// A Unix timestamp is not monotonic and must never be used as elapsed time.
	LastSyncMonotonicNS int64 `json:"last_sync_monotonic_ns,omitempty"`
	LastFetchedAtUnixMS int64 `json:"last_fetched_at_unix_ms"`
	MaxObservedUnixMS   int64 `json:"max_observed_unix_ms,omitempty"`
	ExpiredAtUnixMS     int64 `json:"expired_at_unix_ms,omitempty"`
}

// downloadMeta is the on-disk representation of <artifact>.part.meta.
// It records enough information to resume an interrupted download safely
// (expected size, expected ETag, current bytes on disk).
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
	for _, sub := range []string{"state", "tombstones", "downloads", "resources"} {
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
		// Best-effort cleanup if rename was not reached.
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
	// fsync the directory so the rename is durable.
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

// appendBytes is like writeAtomicBytes but appends to an existing file
// (or creates it). Used to extend a .part file with resumed bytes.
func (s *storage) appendBytes(partPath string, data []byte) (int64, error) {
	f, err := os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	before, _ := f.Seek(0, io.SeekEnd)
	if _, err := f.Write(data); err != nil {
		return before, err
	}
	if err := f.Sync(); err != nil {
		return before, err
	}
	return before, nil
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

// readActiveManifest returns the canonical bytes of the active manifest
// plus the parsed struct. Missing or unreadable files return ErrNoActiveState.
func (s *storage) readActiveManifest() ([]byte, *manifestRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	binPath := filepath.Join(s.root, "state", "current_manifest.bin")
	bin, err := os.ReadFile(binPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNoActiveState
		}
		return nil, nil, err
	}
	var rec manifestRecord
	if err := json.Unmarshal(bin, &rec.Manifest); err != nil {
		return nil, nil, fmt.Errorf("offlineclient: parse manifest bytes: %w", err)
	}
	rec.RawBytes = bin
	return bin, &rec, nil
}

// manifestRecord pairs the parsed manifest with its canonical bytes.
type manifestRecord struct {
	Manifest offlinepkg.Manifest `json:"-"`
	RawBytes []byte              `json:"-"`
}

// writeActiveManifest atomically writes the canonical manifest bytes to
// the active location. The caller has already verified the bytes.
func (s *storage) writeActiveManifest(canonical []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeAtomicBytes(filepath.Join(s.root, "state", "current_manifest.bin"), canonical)
}

// readActiveCard returns the canonical bytes and parsed card.
func (s *storage) readActiveCard() ([]byte, *offlinepkg.PublicIncidentCard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	binPath := filepath.Join(s.root, "state", "current_card.bin")
	bin, err := os.ReadFile(binPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNoActiveState
		}
		return nil, nil, err
	}
	var card offlinepkg.PublicIncidentCard
	if err := json.Unmarshal(bin, &card); err != nil {
		return nil, nil, fmt.Errorf("offlineclient: parse card bytes: %w", err)
	}
	return bin, &card, nil
}

// writeActiveCard atomically writes the canonical card bytes.
func (s *storage) writeActiveCard(canonical []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeAtomicBytes(filepath.Join(s.root, "state", "current_card.bin"), canonical)
}

// writeActiveGeneration atomically stages and commits manifest and card together.
func (s *storage) writeActiveGeneration(manifestBytes []byte, cardBytes []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stateDir := filepath.Join(s.root, "state")
	tmpDir, err := os.MkdirTemp(stateDir, ".staging-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	if err := os.WriteFile(filepath.Join(tmpDir, "current_manifest.bin"), manifestBytes, 0o644); err != nil {
		return err
	}
	if len(cardBytes) > 0 {
		if err := os.WriteFile(filepath.Join(tmpDir, "current_card.bin"), cardBytes, 0o644); err != nil {
			return err
		}
	}

	if err := os.Rename(filepath.Join(tmpDir, "current_manifest.bin"), filepath.Join(stateDir, "current_manifest.bin")); err != nil {
		return err
	}
	if len(cardBytes) > 0 {
		if err := os.Rename(filepath.Join(tmpDir, "current_card.bin"), filepath.Join(stateDir, "current_card.bin")); err != nil {
			return err
		}
	}

	if d, err := os.Open(stateDir); err == nil {
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

type tombstoneStore struct {
	mu   sync.Mutex
	root string
}

func (s *storage) tombstones() *tombstoneStore {
	return &tombstoneStore{root: filepath.Join(s.root, "tombstones")}
}

// loadOrInit reads a tombstone list, creating the file if absent.
func (t *tombstoneStore) loadOrInit(name string) ([]string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	path := filepath.Join(t.root, name)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("offlineclient: parse %s: %w", name, err)
	}
	return out, nil
}

func (t *tombstoneStore) save(name string, list []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(t.root, ".tmp-*")
	if err != nil {
		return err
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
	return os.Rename(tmpPath, filepath.Join(t.root, name))
}

func (t *tombstoneStore) add(name, id string) (added bool, err error) {
	cur, err := t.loadOrInit(name)
	if err != nil {
		return false, err
	}
	for _, x := range cur {
		if x == id {
			return false, nil
		}
	}
	cur = append(cur, id)
	if err := t.save(name, cur); err != nil {
		return false, err
	}
	return true, nil
}

func (t *tombstoneStore) checkContains(name, id string) (bool, error) {
	cur, err := t.loadOrInit(name)
	if err != nil {
		return false, err
	}
	for _, x := range cur {
		if x == id {
			return true, nil
		}
	}
	return false, nil
}

func (t *tombstoneStore) contains(name, id string) bool {
	cur, err := t.loadOrInit(name)
	if err != nil {
		return true // fail closed
	}
	for _, x := range cur {
		if x == id {
			return true
		}
	}
	return false
}

// supersededKey composes the file name for superseded-version records.
const supersededFile = "superseded.json"

// addSuperseded records a (package_id, version) tombstone. Multiple
// supersessions of the same pair are idempotent.
func (t *tombstoneStore) addSuperseded(pkgID string, version int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	path := filepath.Join(t.root, supersededFile)
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var list []offlinepkg.SupersededVersion
	if len(b) > 0 {
		if err := json.Unmarshal(b, &list); err != nil {
			return err
		}
	}
	for _, x := range list {
		if x.PackageID == pkgID && x.Version == version {
			return nil
		}
	}
	list = append(list, offlinepkg.SupersededVersion{PackageID: pkgID, Version: version})
	out, err := json.Marshal(list)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(t.root, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(out); err != nil {
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
	return os.Rename(tmpPath, path)
}

// loadSuperseded returns the current superseded list.
func (t *tombstoneStore) loadSuperseded() ([]offlinepkg.SupersededVersion, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	b, err := os.ReadFile(filepath.Join(t.root, supersededFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []offlinepkg.SupersededVersion
	if len(b) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// isSuperseded reports whether (pkgID, version) is in the superseded list.
func (t *tombstoneStore) isSuperseded(pkgID string, version int) bool {
	list, err := t.loadSuperseded()
	if err != nil {
		return true // fail closed
	}
	for _, x := range list {
		if x.PackageID == pkgID && x.Version == version {
			return true
		}
	}
	return false
}

// verifyIntegrity checks that all tombstone files exist and are well-formed.
func (t *tombstoneStore) verifyIntegrity() error {
	for _, name := range []string{"packages.json", "routes.json"} {
		if _, err := t.loadOrInit(name); err != nil {
			return err
		}
	}
	if _, err := t.loadSuperseded(); err != nil {
		return err
	}
	return nil
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

// monotonicNow returns the monotonic clock reading from the injected Now
// function. It is the only time source used for expiry decisions.
func monotonicNow(now func() time.Time) time.Time {
	t := now()
	return t
}
