# Worker 1 — p5-client-final handoff

**Branch:** `codex/p5-client-final`
**Worktree:** `/private/tmp/mz-worktrees/p5-client-final`
**Base commit:** `2fe0fee` (P6 contract freeze)
**Lane owner:** Worker 1 — `p5-client-final`. Owns `backend/internal/offlineclient/` only.

## Commits on this branch

| SHA | Subject |
| --- | --- |
| `f67ec79` | fix(offlineclient): coherent single-file generation + tombstone integrity (Areas A–G) |
| `af20983` | test(offlineclient): real-signature end-to-end + Areas A–G regression tests |

## Files touched (all owned by this lane)

```
backend/internal/offlineclient/client.go
backend/internal/offlineclient/client_test.go
backend/internal/offlineclient/client_p5_final_test.go (new)
backend/internal/offlineclient/storage.go
backend/internal/offlineclient/sync.go
```

No shared file modified. No migration touched. No `go.mod` / `go.sum` touched.
No `.txt` file touched. No production server wiring changed.

## What changed

### Layout

* `state/current_generation.json` — single atomic record holding both
  manifest and card canonical bytes plus identity metadata
  (revision, manifest_id, package_id, card_version, card_checksum). One
  `writeAtomicBytes` makes activation a single-file durable switch.
  Either the prior coherent generation or the new one is selected;
  never a mixed pair. Failure between stages leaves the old
  generation intact.
* `tombstones/tombstones.json` — single atomic file with the three
  lists (packages, routes, superseded). `tombstones/.initialized`
  sentinel is written BEFORE the file so a crash that leaves only the
  sentinel falls back to fresh-empty semantics on next load.
* `state/staging/` — transient staging dir for in-flight activation
  (created at storage init).

### Sync

* `activeIntact` predicate now binds on canonical-bytes identity
  (revision, manifest_id, critical-card checksum, manifest
  checksum_sha256). A same-revision manifest with conflicting bytes is
  no longer silently treated as unchanged; the client re-activates
  the new generation. This was the bug called out in the lane prompt.
* Same-revision early-return path now refreshes the in-process
  monotonic anchor (`c.lastSyncMono = time.Now()`) so a fresh client
  after restart can compute CURRENT/STALE/EXPIRED on read, while
  preserving `state.LastFetchedAtUnixMS` so the validity window is
  NOT renewed.
* Phase 4 activation uses a single os.Rename of the staging
  generation file onto `state/current_generation.json`. Staging
  failures (mkdir, write, rename, sync) surface; they are never
  swallowed.

### Freshness

* `computeFreshness` distinguishes two time sources:

  - no in-process monotonic anchor (post-restart): wall-clock against
    the persisted acquisition window and the high-water mark. A
    restarted client reads CURRENT / STALE / EXPIRED instead of always
    UNVERIFIABLE. Clock rollback is caught by `MaxObservedUnixMS` and
    fails closed (EXPIRED + `ErrClockRolledBack`).
  - with a monotonic anchor (same process as a verified sync):
    remaining is additionally bounded by
    `min(wall-clock-remaining, validity-at-acq − elapsed)`, preventing a
    re-sync from granting more validity than the original acquisition.

* Latched EXPIRED semantics (`ExpiredAtUnixMS`) preserved.

### Tombstone integrity (req 6)

* `loadOrInit` distinguishes "genuinely new store" (no `.initialized`
  sentinel → empty list, OK) from "existing store with tampering"
  (sentinel present + file missing/zero-byte/malformed → integrity
  error → fail closed).
* `saveLocked` writes the sentinel BEFORE the file: a crash leaving
  the file but no sentinel is detected as a fresh install and the
  client retries the sync. A crash leaving the sentinel but no file
  is impossible because the file rename is the last durable step
  and the sentinel write precedes it.

### Tests

Real Ed25519 signatures throughout `client_p5_final_test.go`. Fakes
limited to parser isolation (per the lane contract). Server fixtures
are exercised over real `httptest.Server` with real signatures
verified through `offlinepkg.MemoryTrustStore.VerifySignature`.

## Coverage matrix

| Lane requirement | Test | Result |
| --- | --- | --- |
| Restart + same-revision sync, freshness recovers without renewing validity | `TestRestartSameRevisionRecoversFreshnessWithoutRenewingValidity` (req 1) | PASS |
| Same-revision conflicting bytes rejected | `TestSameRevisionConflictingBytesRejected` (req 2) | PASS |
| Preserve observed expiry across restart + new manifest referencing the same expired card | `TestClockRollbackAfterReSyncKeepsExpired` (req 3) | PASS |
| Replace two renames with one coherent generation; interruption before/after each boundary; on reopen, never a mixed pair | `TestAtomicActivationInterruptedBoundaries` + `TestActivationNeverPresentsMixedPair` + `TestSameRevisionRejectsConflictingActiveGeneration` (reqs 4, 5) | PASS |
| Tombstone truncate/zero-byte/delete after init fail closed; only a genuinely new store initializes empty | `TestTombstoneCorruptionAfterInitFailsClosed` + `TestTombstoneGenuineNewStoreAllowsEmpty` (req 6) | PASS |
| Real-signature end-to-end + byte-level tamper detection | `TestRealSignatureEndToEndWithRestart` | PASS |

## Verification

* `gofmt -l .` clean
* `go vet ./...` clean
* `go build ./...` ok
* `go test ./internal/offlineclient ./internal/offlinepkg ./internal/offlinedelivery
       ./internal/offlineresources ./internal/offlinequeue ./internal/httpjson -count=1`
  → all ok
* `go test ./internal/offlineclient -count=2 -race` clean
* `TestServedRoutesMatchOpenAPI` (shared OpenAPI/route test) still passes

## Limits / out of scope

* No hardware, no GPU, no real-model execution — all tests use offline
  Ed25519 + offline parser + real HTTP. P6 voice pipeline is a separate
  lane.
* Worker did not edit `backend/contracts/openapi.yaml`,
  `backend/internal/httpserver/server.go`, `cmd/sthira/main.go`,
  `backend/go.mod`/`go.sum`, `backend/migrations/`, or
  `plan/prompt.md`. These remain coordinator-owned.
* Optional resource validation (style JSON cross-validation) and
  actual interrupted-resume behavior were preserved as-is from the
  Areas A–G prior work. No regression introduced.
* The `.initialized` sentinel writes only on the first successful
  tombstone save; we do not write it on `state.json` save or
  resource save. That is intentional: those do not establish
  revocation knowledge.

## Integration deltas needed

None. This lane is self-contained:
* No shared wiring edits.
* No new endpoints.
* No new error codes (reuses existing codes; nothing added to
  `contracts/errors.go`).
* No new migrations.

The coordinator can integrate this branch as-is by merging
`codex/p5-client-final` onto `CLEAN`. After integration, the P5 client
acceptance matrix in `plan/prompt.md` is expected to update Areas A–G
to PASS for the engineering half. External blockers (O01, O05, O06,
O07, O14) remain unchanged.
