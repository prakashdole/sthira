# Worker 4 — P6 Context Handoff

Worker 4 (p6-context) shipped the typed `ScopedContext` builder, the
independent semantic validator, and the `SnapshotRevalidator` seam that
Worker 9 (orchestrator) wires into the voice-pipeline request lifecycle.
This handoff lists the public surface, the contract, and how to call it.

## Files Owned

**New:**

- `backend/internal/contracts/scoped.go` — semantic validator + JSON shape gate
- `backend/internal/contracts/scoped_test.go` — unit regressions (~36 cases)
- `backend/internal/store/scoped.go` — `BuildScopedContext`, `ScopedContextResolver`
- `backend/internal/store/scoped_integration_test.go` — real-DB regressions (11 cases)

**Extended (allowed by freeze):**

- `backend/internal/store/context.go` — `ContextSnapshot.PackageID` and
  `ContextSnapshot.PackageVersion` populated by `ResolveContext`.

## Public Surface

### `contracts.EnforceScopedContext(out ModelOutput, sc ScopedContext) error`

Returns `*ErrScopedSemantic` (a typed error carrying `Reason`) when the
proposal violates one of the typed rules:

- `sc.Jurisdiction` is empty.
- `out.DataVersion` is set and does not match `sc.DataVersion` (stale
  proposal against a current snapshot).
- `out.Language` is empty or not in `sc.IsLanguageAllowed(...)`.
- Any `out.Actions` entry references an ID outside the typed maps, has
  the wrong type for its action kind, or mismatches its jurisdiction.
- `out.SpeechKey` is set but not in `sc.IsTemplateKeyAllowed(...)`.
- `out.ClarificationIDs[i]` does not exist in `sc.AllowedClarifications`.
- Actions exceed `contracts.MaxModelActions`.
- Choices exceed `contracts.MaxShowChoices`.
- `out.Panels` references an unknown panel enum.

Out-of-scope IDs (places, zones, routes, facilities) fail closed: the
orchestrator must NEVER send the model the unresolved proposal.

### `contracts.ValidateModelOutputShape(raw json.RawMessage) error`

Called before `EnforceScopedContext`. Rejects:

- Unknown JSON fields at the top level.
- `status` outside the `Status` enum.
- `intent` outside the `Intent` enum.
- `actions[i].type` outside the `Action` enum.
- Reordered `clarification_ids` or `evidence_ids`.
- Prompt-injection-style strings in `notes`/`instruction`/`template_args`.

The two gates are independent on purpose: shape may pass and semantics
fail (model fabricates a zone ID), or shape may fail (model invented a
field) — Worker 9 must call both and surface both kinds of failure
differently.

### `store.NewScopedContextResolver(s *Store) *ScopedContextResolver`

The resolver builds a typed `ScopedContext` for a jurisdiction. It
queries only the current persisted package (zero consequential writes;
verified by `TestScopedContext_EnforcesPendingZeroConsequentialWrites`)
and partitions the result into the typed maps the validator consumes:

- `KnownPlaces`, `KnownSafeZones`, `KnownRedZones`, `KnownRoutes`,
  `KnownFacilities`.
- `VerifiedRoutes[facilityID]` — only routes whose `ToSafeZoneID ==
  fac.SafeZoneID` AND who pass `isRouteVerified` (verified_by set, now
  within `valid_from`/`valid_until`).
- `AllowedLanguages` — union of instruction-asset languages plus
  `sc.AllowedLanguages` (Worker 7 template registry).
- `TemplateKeys` — empty here; Worker 7 populates from the registry at
  request time.
- `AllowedClarifications` — empty here; Worker 7 supplies the registry
  list at request time.
- `DataVersion = "<packageID>:<version>"`. Worker 9 must store the
  proposal's `out.DataVersion` alongside the request so the validator
  can detect stale proposals.

### `ScopedContextResolver.SnapshotRevalidate(ctx, sc) error`

The seam Worker 9 calls:

1. After `EnforceScopedContext` re-validates a previously-resolved
   proposal (replay check).
2. After a long-running middle-worker call returns (the model might
   have been invoked against a snapshot that has since changed).

Return contract:

- `nil` — snapshot unchanged. Safe to apply the proposal.
- `errors.New(contracts.ErrStaleSnapshot)` — package superseded, source
  withdrawn, or `DataVersion` no longer resolves. The orchestrator must
  reject the proposal and re-resolve from scratch.

The error wraps the constant string so callers can `==`-compare or use
`errors.Is` against `errors.New(contracts.ErrStaleSnapshot)`.

## Integration Order (Worker 9)

```
1. ctx := BuildScopedContext(jurisdiction)
2. raw := middleWorker.Call(ctx)
3. if err := ValidateModelOutputShape(raw); err != nil { refuse(raw); return }
4. var out contracts.ModelOutput
   decode(raw) into out
5. if err := EnforceScopedContext(out, ctx); err != nil { refuse(reason); return }
6. if err := resolver.SnapshotRevalidate(ctx); err != nil { refuse(stale); return }
7. apply(out)
```

Steps 3 and 6 are the new gates. Step 6 catches the gap where a model
proposal was generated against a snapshot that the operator withdrew
during inference.

## Verification

```
go test ./internal/contracts  -count=1                # 36 unit cases, all pass
export STHIRA_TEST_ADMIN_DSN='postgres://apple@localhost:5432/postgres'
go test ./internal/store       -count=1                # 11 real-DB cases
go test ./...                  -count=1 -timeout 180s  # full backend, all pass
gofmt -l ... ; go vet ./internal/contracts ./internal/store  # clean
```

Commit: see `git log --oneline` on `codex/p6-context`.

## Trust Boundary Invariants

These are the rules Worker 4 enforces and Worker 9 must not weaken:

1. A jurisdiction ID is the only thing that opens the door. Unknown
   jurisdiction → `ErrNoScopedContext`. Never guess.
2. Cross-jurisdiction place-alias lookups are forbidden (see
   `TestScopedContext_NoCrossJurisdictionLeak`). The store layer
   filters by `place_aliases.jurisdiction`.
3. `VerifiedRoutes[facility]` is built ONLY from routes whose
   `ToSafeZoneID == fac.SafeZoneID`. The validator rejects proposals
   that cite an unverified route for the chosen destination.
4. Routes outside their `valid_from`/`valid_until` window are NOT
   `Verified` (SYNTHETIC_DEMO approvals are still considered verified
   for the harness; the operational gate is upstream at the choice
   querier).
5. `ResolveContext` and `BuildScopedContext` perform ZERO writes
   (verified by row-count snapshot before/after).
6. The validator runs in two independent passes. Both must succeed for
   a proposal to be passed downstream.

## Known Refactors Worth Removing Later

These are deliberate simplifications; Worker 4 left them with
`ponytail:` comments inline:

- `isRouteVerified` accepts SYNTHETIC_DEMO approvals because the
  operational gate is upstream. Production deployment must replace
  the in-harness "approval=SYNTHETIC_DEMO ⇒ verified" with a real
  officer signature check (P7 hardening).
- `BuildScopedContext` reads the package JSON inline rather than via a
  typed struct because the package body is a forwarded-government
  payload and Worker 4 owns no down-stream code that consumes each
  field. When Worker 5/6 begin typing the package body, migrate to a
  shared `packagebody` package — open this as `ponytail:` debt if you
  are the next person to touch it.
- The alias resolver inflates every alias into a synthetic place
  candidate. Withdrawals of an alias row do not propagate; the
  revalidation path catches this via the package row, not the alias.
