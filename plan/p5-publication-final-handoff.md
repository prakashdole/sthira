# P5 publication and delivery — withdrawal propagation bound

**Owner:** Worker 2 (`p5-publication-final`)
**Base commit:** `2fe0fee` (coordinator freeze)
**Worktree:** `/private/tmp/mz-worktrees/p5-publication-final`

This document records the consistency/propagation guarantee the trusted publication/delivery surface provides, and the integration deltas the coordinator needs to apply. It is intentionally simple: a speculative distributed invalidation service is out of scope.

## What is the signed withdrawal channel

There are **two** authoritative channels for revoking/withdrawing public guidance:

1. **Manifest-level supersession.** The `Manifest` carries a `Revocations` block (`RevocationBlock` in `offlinepkg/types.go`):
   `revoked_packages []string`, `cancelled_routes []string`, `superseded_versions [{package_id, version}]`.
   An operator publishes a NEW manifest revision that lists older packages/routes as revoked. The new manifest is signed by an authorized key; clients who trust the authority read the revocation block on every fresh sync and learn that older packages are no longer authoritative. The signature on the new manifest authenticates the revocation; the manifest_id + revision sequence gives monotonic ordering.

2. **Row-level withdrawal.** `Store.SetManifestStatus(ctx, jurisdiction, revision, "WITHDRAWN")` and `Store.QuarantineManifest(ctx, jurisdiction, revision, true)` mark an existing publication row as non-current. The `CachedSource`/`MemoryCache` filter `SourceStatus != CURRENT` before serving from the cache and refuse to repopulate the cache from a stale inflight fetch (see `IsLiveStatus` in `cache.go`). The `storePublicationAdapter` translates `Quarantined = true` to `offlinedelivery.ErrQuarantined`, which the HTTP handler maps to **410 Gone** with no payload body.

## Propagation bound (single-instance, in-memory cache only)

| Trigger | Effect | Max latency |
|---|---|---|
| Operator calls `QuarantineManifest` / `SetManifestStatus` → DB row update | Same process: cache entry is removed when the operator or its caller invokes `CachedSource.InvalidateManifest`/`InvalidateCard` (wired via the publication lifecycle operator; integration delta below). | Immediate on `Invalidate*`; up to `ManifestCacheTTL` (10 s default) for callers that haven't asked yet |
| Client fetches a manifest after a withdrawal | Cached source: cache miss → DB lookup → status is `WITHDRAWN` → adapter returns `ErrQuarantined` → handler returns 410 Gone | Within one upstream call |
| Inflight stale fetch completes after `Invalidate*` | `MemoryCache.PutManifest` / `PutCard` refuse to cache a record whose `SourceStatus` is not `CURRENT` (or empty legacy); record is dropped instead | Immediate; no repollution possible |
| Reader was holding a cached `CURRENT` manifest when the operator withdraws it | The next request gets the stale entry until `ManifestCacheTTL` (default 10 s) expires, OR until the reader fetches again (304 with `If-None-Match` returns from cache; a stale `CURRENT` lives up to TTL) | Up to 10 s |
| Card row quarantined | `CachedSource.InvalidateCard` purges it; subsequent requests return 410 Gone | Immediate on invalidation; up to `CardCacheTTL` (10 min) if no invalidation was issued |

**Documented bound:** within one process, withdrawal is observed within `ManifestCacheTTL` (10 s) for already-cached readers, immediately for fresh readers after invalidation. This bound is enforced; it does not depend on third-party cache infrastructure.

## Propagation bound (multi-instance / external CDN)

When more than one server instance is in scope, OR an external CDN/edge caches the response, the bound widens:

- Each instance has its own `MemoryCache`; an invalidation on instance A does not propagate to instance B until B's next upstream call (which the cache miss triggers). Each instance independently honors the withdrawal after its own cache TTL expires, or earlier if the operator's publication lifecycle calls `Invalidate*` on every instance.
- The HTTP handler emits `Cache-Control: public, max-age=60, must-revalidate` on manifest responses. A CDN holding a copy revalidates within 60 s; until then it may serve the stale `CURRENT` payload. Card responses are `public, max-age=31536000, immutable` so CDNs hold them for a year; a card marked `WITHDRAWN` at the row level still returns 410 Gone from the authoritative server, but a CDN copy may return 200 until it revalidates.
- The robust enforcement pattern at the application layer is: clients fetch the LATEST manifest revision on every fresh sync. The new manifest's `Revocations` block lists withdrawn/superseded packages/routes, signed by the authority. Clients honor the revocation block regardless of the row-level status on older cached cards.

**Documented bound for external caches:** row-level withdrawal reaches every external cache no faster than the cache's `max-age`. Manifest-level supersession reaches every external cache no faster than `max-age=60` (must-revalidate forces revalidation). For deployments that need same-instance propagation across many replicas, the application MUST emit the invalidation on every replica's `CachedSource` — this is the operator's responsibility, not a service feature.

## What we deliberately did not add

- A speculative distributed invalidation service. The bounded cache + signed-manifest-revocation-block pair gives the same operational guarantee for the single-instance case without a new broker.
- A new `revocation_records` table. The signed revocation block on the manifest is the canonical channel; row-level status changes are server-controlled and observable through the adapter.
- Tamper-proof client-side revocation propagation. Clients are trusted to honor what the manifest's revocation block says; clients that don't are clients, and offline-client integrity is out of scope here.

## Integration deltas for the coordinator

These belong to `cmd/sthira/main.go` (the entrypoint), which is **not** owned by Worker 2. The coordinator should:

1. **Construct a `Publisher`** once at startup from the configured `offlinepkg.TrustStore`. The trust store is loaded from a configuration file (e.g. `STHIRA_TRUST_KEYS_FILE` JSON list of `{key_id, public_key, permitted_jurisdiction, valid_from, valid_until}`). Construction is:
   ```go
   pub := store.NewPublisher(st, ts)
   ```
   And expose it through a `WithPublisher(pub)` server option so the operator routes (`handleSourceTransition`, future publish/correction) use the trusted path. **No production operator route currently publishes**; when one is added it MUST use `pub.PublishManifest` / `pub.PublishCard`, NOT `Store.PublishManifest` directly.

2. **Wire `CachedSource.InvalidateManifest` / `InvalidateCard` into the publication lifecycle.** The `store.PublicationSource` returned by `NewStorePublicationSource(st)` is currently a read-only adapter; the write side does not currently invalidate. Add a small hook so that:
   - `SetManifestStatus(ctx, jurisdiction, revision, ...)` → call `pubSource.InvalidateManifest(jurisdiction)` after the row UPDATE.
   - `SetCardStatus(ctx, packageID, version, ...)` → call `pubSource.InvalidateCard(packageID, version)`.
   - `QuarantineManifest` / `QuarantineCard` → the same invalidate call (they're a status change).
   - A new manifest revision that supersedes an older one → `InvalidateManifest(jurisdiction)` so fresh readers don't see the older `CURRENT` until their cache TTL.

   The `CachedSource` already exposes these as public methods; the only wiring is calling them from the store-layer write paths.

3. **Document the propagation bound** in the operator runbook (see above) so operators understand that external CDN caching delays are bounded by `max-age`, not by the application.

4. **Add a startup-time trust-store sanity check.** If no TrustStore is wired into `Publisher`, every publish fails closed with `ErrPublicationTrustMissing`. The startup log should explicitly state which key_ids are loaded and which jurisdictions each is permitted to sign for, so a misconfigured deployment is loud at boot rather than silent at first publish.

5. **No new migration** in this stage. The two source columns required by the trusted path (`source_id`, `jurisdiction` on the published_card row) are stored via the existing schema (with `source_id` as a new column added to `published_manifests` and `published_cards`; this requires a new migration `0007_p5_trusted_publication.sql` that adds `source_id text NOT NULL` to both tables). The coordinator should add this migration on the integration branch; see `backend/migrations/0006_p5_offline_publication.sql` for the existing pattern.

## Tests exercised in this stage

Run with `STHIRA_TEST_ADMIN_DSN='postgres://apple@localhost:5432/postgres'` against a local PostgreSQL 16+PostGIS instance:

- `TestPublisher_TamperRejected` — single-byte mutation of signed JSON is rejected before any persistence.
- `TestPublisher_WrongJurisdictionRejected` — KL-signed manifest claiming TN jurisdiction is rejected.
- `TestPublisher_RevokedKeyRejected` — key revoked at verify-time is rejected.
- `TestPublisher_MissingTrustFailsClosed` — Publisher without a TrustStore rejects every publish.
- `TestPublisher_IdentityBindRejectsCallerOverride` — caller cannot substitute `package_id`/`revision`/`manifest_id`.
- `TestPublisher_AuthorityGateSuspendedSource` — non-OPERATIONAL source cannot publish.
- `TestPublisher_AuthorityGateMissingPackage` — orphan package_id rejected.
- `TestPublisher_ConcurrentIdenticalSucceed` — 8 goroutines, distinct revisions, all succeed.
- `TestPublisher_ConcurrentConflicting` — 16 goroutines, half A / half B, only `ErrConflict` for non-matching.
- `TestPublisher_ReplayCannotRestoreCurrentAfterWithdraw` — replay cannot undo a WITHDRAWN.
- `TestMemoryCache_BoundedByEntries` / `TestMemoryCache_BoundedByBytes` — bounded cache evicts oldest.
- `TestMemoryCache_StaleRepublishRejected` — withdrawn record cannot repopulate the cache.
- `TestCachedSource_CancelableWaiters` — waiters cancel independently; leader coalesces; no caller hangs.
- Existing `TestCachedSource_QuarantineAndWithdrawalInvalidation`, `TestMemoryCache_Invalidation`, `TestPublicationImmutabilityAndQuarantine` continue to pass.