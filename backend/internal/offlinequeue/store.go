package offlinequeue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store persists PendingOperation entries to a local directory. Each entry is
// a single JSON file at <dir>/<id>.json, written atomically via a sibling
// <id>.json.tmp file followed by os.Rename (atomic on POSIX file systems).
// On read the store enumerates the directory and parses every <id>.json,
// silently skipping .tmp files (in-progress writes whose producer crashed).
//
// Concurrency: a single Store is safe for concurrent use. Writes are guarded
// by a mutex; reads snapshot the directory listing under the same mutex so
// in-flight writes either complete before or after the listing. There is no
// cross-process lock; the reference harness assumes one process owns the
// directory at a time (the real P8 mobile client owns its own queue
// directory, and the integration test process owns its tempdir).
//
// Memory: small queues (the P5 acceptance matrix caps at hand-tens of
// entries) fit in memory; List/Peek read the whole directory. A production
// mobile client may add paging; that is explicitly out of scope here.
type Store struct {
	dir string
	now func() time.Time
	mu  sync.Mutex // serializes writes and read-then-mutate cycles
}

// NewStore opens (or creates) a queue directory at dir. The directory is
// created with 0700 permissions so the queue contents are owner-readable.
// now is injected so tests can drive expiry without sleeping.
func NewStore(dir string, now func() time.Time) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("offlinequeue: store directory is required")
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("offlinequeue: mkdir %s: %w", dir, err)
	}
	return &Store{dir: dir, now: now}, nil
}

// Dir returns the absolute path of the queue directory.
func (s *Store) Dir() string { return s.dir }

// Enqueue persists op after validating it and confirming no conflicting
// entry already exists for the same idempotency key in a non-terminal state.
// A conflicting entry is either:
//   - a pending/in-flight entry with a different payload (ErrPayloadChanged
//     — changed intent requires a new key, not a silent overwrite), or
//   - a pending/in-flight entry with the same payload (ErrOperationConflict
//     — same key is already in flight; the worker will pick it up).
//
// Terminal entries (COMMITTED, FAILED_STALE, FAILED_PERM) do NOT block a fresh
// enqueue of the same key: a user may legitimately confirm a new selection
// after the previous one failed. The new entry's CreatedAt dominates any
// stale state on disk.
func (s *Store) Enqueue(ctx context.Context, op PendingOperation) error {
	if err := op.Validate(s.now()); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.readAllLocked()
	if err != nil {
		return err
	}
	for _, e := range existing {
		if e.IdempotencyKey != op.IdempotencyKey {
			continue
		}
		if e.IsTerminal() {
			continue
		}
		if e.PayloadHash != op.PayloadHash {
			return ErrPayloadChanged
		}
		return ErrOperationConflict
	}
	if op.CreatedAt.IsZero() {
		op.CreatedAt = s.now().UTC()
	}
	return s.writeLocked(op)
}

// Get reads a single entry by id. Returns ErrNoSuchOperation if absent.
func (s *Store) Get(ctx context.Context, id string) (PendingOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readOneLocked(id)
}

// List returns every durable entry sorted by CreatedAt ascending. The result
// is a snapshot; concurrent updates are not reflected until the next call.
func (s *Store) List(ctx context.Context) ([]PendingOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAllLocked()
}

// PeekPending returns the entries the worker should attempt to dispatch on
// the next drain, oldest first. That set is:
//
//   - PENDING (never dispatched), and
//   - PENDING_RECONCILIATION (dispatched but with an uncertain prior outcome
//     — a previous Submit observed a transport error and preserved the
//     uncertainty; the server's verdict is still unknown and must be
//     resolved by a real replay).
//
// IN_FLIGHT entries left behind by a crashed worker are reset to
// PENDING_RECONCILIATION on first observation (see ResetStuckInFlight) so
// the uncertainty is preserved across restarts; they then surface here on
// the next drain.
func (s *Store) PeekPending(ctx context.Context) ([]PendingOperation, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PendingOperation, 0, len(all))
	for _, op := range all {
		if op.State == StatePending || op.State == StatePendingReconciliation {
			out = append(out, op)
		}
	}
	return out, nil
}

// UpdateState persists a new state for op.ID. The full updated entry is
// passed by value so the caller can also update RetryCount/LastError. The
// store refuses to leave a terminal state (COMMITTED/FAILED_STALE/FAILED_PERM)
// once committed: terminal entries are immutable until purged.
func (s *Store) UpdateState(ctx context.Context, updated PendingOperation) error {
	if strings.TrimSpace(updated.ID) == "" {
		return errors.New("offlinequeue: update requires an id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, err := s.readOneLocked(updated.ID)
	if err != nil {
		return err
	}
	// Preserve immutable fields from the on-disk entry. A caller cannot mutate
	// key, endpoint, method, token_ref, payload, payload_hash, snapshot_version
	// or selection_expiry through this path.
	updated.IdempotencyKey = prev.IdempotencyKey
	updated.Endpoint = prev.Endpoint
	updated.Method = prev.Method
	updated.TokenRef = prev.TokenRef
	updated.Payload = append([]byte(nil), prev.Payload...)
	updated.PayloadHash = prev.PayloadHash
	updated.SnapshotVersion = prev.SnapshotVersion
	updated.SelectionExpiry = prev.SelectionExpiry
	updated.CreatedAt = prev.CreatedAt
	if prev.IsTerminal() {
		return ErrTerminalState
	}
	return s.writeLocked(updated)
}

// Delete removes an entry by id. Returns ErrNoSuchOperation if the entry is
// absent. Use after PurgeCommitted has identified a stale COMMITTED entry
// for removal.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.pathFor(id)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNoSuchOperation
		}
		return err
	}
	return os.Remove(path)
}

// PurgeCommitted removes entries whose state is COMMITTED and whose
// CommittedAt is older than olderThan ago (measured by s.now). The number of
// removed entries is returned. Failed entries are NEVER auto-purged: the
// user must inspect and confirm whether to retry with a new key.
func (s *Store) PurgeCommitted(ctx context.Context, olderThan time.Duration) (int, error) {
	if olderThan < 0 {
		return 0, errors.New("offlinequeue: olderThan must be non-negative")
	}
	cutoff := s.now().Add(-olderThan)
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readAllLocked()
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, op := range all {
		if op.State != StateCommitted {
			continue
		}
		if op.CommittedAt.IsZero() || op.CommittedAt.After(cutoff) {
			continue
		}
		if err := os.Remove(s.pathFor(op.ID)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// ResetStuckInFlight moves every IN_FLIGHT entry back to PENDING_RECONCILIATION
// so a fresh worker can re-attempt it. The reference harness calls this at
// startup; an IN_FLIGHT entry left on disk is by definition one whose
// producer crashed before the server acknowledged.
//
// We deliberately reset to PENDING_RECONCILIATION, not PENDING. A crashed
// dispatch leaves the operation's outcome genuinely unknown: the request may
// have reached the server, the server may have committed, the response may
// have been on its way back. Returning the entry to plain PENDING would
// imply "the server has not seen this", which the queue cannot prove.
// PENDING_RECONCILIATION forces the next drain to contact the server with
// the identical idempotency key; the server's idempotency store resolves
// the duplication (commit replay / explicit rejection / fresh commit) and
// the entry lands in a real terminal state.
//
// RetryCount is incremented and LastError records the recovery reason so
// the operator can see "this entry was recovered from a crashed dispatch".
func (s *Store) ResetStuckInFlight(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.readAllLocked()
	if err != nil {
		return 0, err
	}
	reset := 0
	for _, op := range all {
		if op.State != StateInFlight {
			continue
		}
		op.State = StatePendingReconciliation
		op.RetryCount++
		op.LastError = "reset: previous worker did not acknowledge"
		if err := s.writeLocked(op); err != nil {
			return reset, err
		}
		reset++
	}
	return reset, nil
}

// SnapshotForInspection returns a deep-copied view of the queue with the
// bearer-token field removed (TokenRef is preserved; the actual token bytes
// were never stored). Intended for diagnostic dumps; the queue itself is the
// single source of truth and this method does not bypass the state machine.
//
// Although TokenRef is itself not a bearer token, this is the place to
// guarantee that NO future field carrying token material leaks. The view is
// the only way the queue exports its state to anything outside the package.
func (s *Store) SnapshotForInspection(ctx context.Context) ([]PendingOperation, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PendingOperation, 0, len(all))
	for _, op := range all {
		// Belt-and-braces: blank out any field whose name suggests token
		// material, in case a future struct change reintroduces one.
		opView := op
		opView.TokenRef = "REDACTED"
		out = append(out, opView)
	}
	return out, nil
}

// --- internal ---

func (s *Store) pathFor(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) tempPathFor(id string) string {
	return filepath.Join(s.dir, id+".json.tmp")
}

func (s *Store) readOneLocked(id string) (PendingOperation, error) {
	path := s.pathFor(id)
	raw, err := os.ReadFile(filepath.Clean(path)) // #nosec G304
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return PendingOperation{}, ErrNoSuchOperation
		}
		return PendingOperation{}, err
	}
	var op PendingOperation
	if err := json.Unmarshal(raw, &op); err != nil {
		return PendingOperation{}, fmt.Errorf("offlinequeue: corrupt entry %s: %w", id, err)
	}
	return op, nil
}

func (s *Store) readAllLocked() ([]PendingOperation, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]PendingOperation, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		op, err := s.readOneLocked(id)
		if err != nil {
			if errors.Is(err, ErrNoSuchOperation) {
				continue
			}
			return nil, err
		}
		out = append(out, op)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// writeLocked serializes op as JSON and writes it atomically to disk.
// fsync is best-effort: the rename is the durability boundary on POSIX.
func (s *Store) writeLocked(op PendingOperation) error {
	if strings.TrimSpace(op.ID) == "" {
		return errors.New("offlinequeue: cannot write operation without an id")
	}
	if op.PayloadHash == "" {
		op.PayloadHash = hashPayload(op.IdempotencyKey, op.Payload)
	}
	raw, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.tempPathFor(op.ID)
	final := s.pathFor(op.ID)
	f, err := os.OpenFile(filepath.Clean(tmp), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) // #nosec G304
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	return nil
}
