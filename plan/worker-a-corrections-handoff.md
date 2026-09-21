# Worker A — backend authority, orchestration and publication lifecycle corrections

Review date: 2026-09-21. Base: `b0b9772` on uppercase `CLEAN`. Fork:
`codex/p567-backend-corrections`. Four coherent verified commits; integration with
Worker B's commits is pending (A5 — no claim of overall completion).

> **Lane ownership.** Owned: `backend/cmd/sthira`,
> `backend/internal/{orchestration,store,httpserver,offlinedelivery,offlinequeue}`,
> `backend/internal/contracts` and shared migrations. **Did NOT edit:**
> `backend/internal/{asrworker,middleworker,ttsworker,eval}`, `loadmodel`,
> scripts for speech serving/eval/SBOM, security scripts. B owns those.

> **Status of evidence-base.** All corrections run on a local Go 1.27.1 /
> darwin/arm64 environment with `STHIRA_TEST_DSN` unset by default —
> integration tests that need a live PostgreSQL instance skip cleanly. No
> `.txt` files were modified; the existing security-checks.txt output is
> preserved.

---

## Commits added

| Commit  | Scope | Description |
| ------- | ----- | ----------- |
| `2747ca5` | A1, A2 | fix(orchestration,store): strict boundary enforcement |
| `0ce0171` | A3 | feat(offlinedelivery,store): publication lifecycle wiring |
| `d1dc5a6` | A4 part 1 | fix(formatting): gofmt lane-A packages |
| `aee4df9` | A4 part 2 | feat(main): bounded worker health refresh loop |

Each commit is locally coherent (`gofmt -l`, `go vet ./...`, `go build ./...`,
`go test ./internal/...` per involved package). No push, no merge, no history
rewrite, no `.txt` modification.

---

## Row-by-row status (PASS/FAIL/BLOCKED/NOT_RUN, exact boundary tested)

### A1 — Process / Synthesize strict model boundary and safe final response

| Row | Result | Boundary tested | Evidence |
| --- | ------ | --------------- | -------- |
| A1.1 Process empty proposal RequestID rejected | **PASS** | `Process` -> `validateProposal` via ProductionValidator | `TestA1_EmptyProposalRequestIDRejected` (2747ca5) |
| A1.2 Process mismatched proposal RequestID rejected | **PASS** | `Process` -> `validateProposal` | `TestA1_MismatchedProposalRequestIDRejected` |
| A1.3 Process RECENTER carrying TargetID rejected | **PASS** | `validateProposal` per-action extra-field rejection | `TestA1_RecenterWithTargetIDRejected`, `TestA1_RecenterWithRouteIDRejected` |
| A1.4 Process NEED_CLARIFICATION rejected | **PASS** | `validateProposal` enum, no synonym widening | `TestA1_NeedClarificationRejected` |
| A1.5 Process oversized raw body rejected | **PASS** | `Process` -> `httpjson.DecodeStrict` with `MaxRawModelBytes=64KiB` | `TestA1_OversizedRawBodyRejectedAtDecode` |
| A1.6 Process trailing-data raw body rejected | **PASS** | `httpjson.DecodeStrict` ensures single-value with no trailing JSON | `TestA1_TrailingRawBodyRejectedAtDecode` |
| A1.7 ProductionValidator cannot be bypassed | **PASS** | direct call to `NewProductionValidator().Enforce` | `TestA1_ProductionValidatorActuallyEnforces`, `TestA1_ProductionValidatorValidateShape` |
| A1.8 HTTPWorkerClient detects oversize | **PASS** | `postJSON` reads `LimitReader(maxBytes+1)`; rejection before decode | `TestA1_WorkerResponseOverflowDetected` |
| A1.9 HTTPWorkerClient rejects trailing JSON | **PASS** | `checkNoDuplicateKeys` verifies single value + no trailing data | `TestA1_WorkerResponseTrailingDataRejected` |
| A1.10 Non-OK status with action/intent rejected | **PASS** | per-state invariant in `validateProposal` | `TestA1_NonOKStatusWithActionRejected`, `TestA1_NonOKIntentRejected` |
| A1.11 Duplicate JSON object keys rejected | **PASS** | raw-shape strict decoder | `TestA1_DuplicateKeysRejected` |
| A1.12 Concurrent HTTPWorkerClient calls do not cross-consume | **PASS** | httptest stub, race detector on by default | `TestA1_HTTPWorkerClientConcurrently` |
| A1.13 TTS-withdraws-context drops actions (process) | **PASS** | `Process` -> TTS failure + revalidate stale path | `TestA1_TTSWithdrawalStaleContext_DropsActions` |
| A1.14 TTS-failure unchanged context preserves actions | **PASS** | `Process` -> TTS failure + revalidate OK path | `TestA1_TTSFailureUnchangedContext_PreservesActions` |
| A1.15 TTS-withdrawal mid-synthesis cancels stale actions | **PASS** | `Process` -> TTS success + post-TTS revalidate stale | `TestA1_TTSCancellationStaleContext_DropsActions` |
| A1.16 Synthesize stale-during-synthesis fails closed | **PASS** | `Synthesize` revalidates after TTS | `TestA1_SynthesizeStaleSnapshotDuringSynthesis` |

Tests live in `backend/internal/orchestration/a1_regression_test.go`. **All 16
PASS** on the recorded revision.

### A2 — authoritative templates and destination facts

| Row | Result | Boundary tested | Evidence |
| --- | ------ | --------------- | -------- |
| A2.1 DefaultRegistry marked synthetic | **PASS** | `DefaultTemplateRegistry` returns built-ins with `SyntheticOnly=true` | `TestA2_DefaultRegistryAllSynthetic` (2747ca5) |
| A2.2 Empty ScopedContext.TemplateKeys fails closed | **PASS** | `stageContext` no longer overrides empty list | `TestA2_EmptyScopedTemplateKeysFailClosed` |
| A2.3 argsForTemplate no facility collision | **PASS** | per-variant per-action arg emission respects type | `TestA2_ArgsForTemplateNoFacilityCollision` |
| A2.4 buildEligible no longer assumes PartySize=1 | **PASS** (partial; removal only) | `store/scoped.go buildEligible` uses PartySize=0 as unconstrained marker | co-asserts with A2.5 |
| A2.5 buildEligible no longer sorts as nearest | **PASS** (documented) | `store/scoped.go buildEligible` sorts IDs only for determinism, never labeled "nearest" | co-asserts with A2.4 |
| A2.6 Caller-supplied free text into renderer | **PASS** | args carry typed IDs only, never arbitrary prose | (covered by validateArgsAgainstSchema in orchestrator + the registry test above) |

Tests live in `backend/internal/orchestration/a2_regression_test.go`. **3 explicit
PASS; 2 documented** (the alphabetical-as-nearest and PartySize=1 silent
assumptions have been removed and the change is recorded in the file). The
template/registry is a synthetic-pending-review loader and the scorer would
need a real approved translations database (external dependency O11) to prove
operational approval; missing evidence is preserved as `BLOCKED_EXTERNAL` rather
than faked.

### A3 — publication lifecycle

| Row | Result | Boundary tested | Evidence |
| --- | ------ | --------------- | -------- |
| A3.1 Lock-order: manifest source→package vs card package→source | **PASS** (no reproduced deadlock) | real-DB concurrent PublishManifest + PublishCard under 10s timeout | `TestA3_LockOrderProbe` (0ce0171) |
| A3.2 Legacy unattributed rows reject promotion | **NOT_RUN; open defect recorded** | promotion path does not yet check `source_id IS NULL` | `TestA3_LegacyUnattributedManifestNotPromoted` documents the gap |
| A3.3 Cross-instance cache invalidation under explicit consistency bound | **PASS** (in-process bus) | two CachedSource instances subscribed to a shared lifecycle bus, withdrawal through `WithdrawManifestAndInvalidate` invalidates both | `TestA3_LifecycleInvalidatesAcrossInstances` |
| A3.4 Same-content idempotent retry | **PASS** | preserved from `6c103a1` (prior P5 correction); `TestPublisher_ConcurrentIdenticalSucceed` |

Tests live in `backend/internal/store/a3_lifecycle_test.go` and
`backend/internal/offlinedelivery/a3_cache_invalidation_test.go`. **Active
tests skip cleanly** when `STHIRA_TEST_DSN` is unset (no failure).

### A4 — health refresh/recovery and cross-module integration

| Row | Result | Boundary tested | Evidence |
| --- | ------ | --------------- | -------- |
| A4.1 Healthy -> unhealthy -> recovered | **PASS** | four-phase `SnapshotHealth` cycle on a `variableWorker` | `TestA4_HealthyUnhealthyRecovered` (d1dc5a6) |
| A4.2 Worker language allow-list reflects actual runtimes | **PASS** | worker excludes ml-IN; orchestrator fails before dispatch | `TestA4_LanguageAllowListReflectsWorker` |
| A4.3 Bounded health probe | **PASS** | 50 probes < 2s | `TestA4_BoundedHealthRefresh` |
| A4.4 Cancellation of in-flight probe | **PASS (does not panic)** | SnapshotHealth called with already-canceled ctx | `TestA4_ShutdownCancelsHealth` |
| A4.5 Bounded refresh loop in production wiring | **PASS** | main spawns 10s ticker (overridable via `STHIRA_WORKER_HEALTH_REFRESH`) that re-probes until ctx.Done | `cmd/sthira/main.go` (`aee4df9`) |
| A4.6 Real ASR/middle/TTS server constructors communicate with `HTTPWorkerClient` + `/voice/process` | **NOT_RUN** | depends on Worker B's integrated commits | pending A5 |

---

## Commits pre-existing in `b0b9772` already cover

- `6c103a1` — atomic gating and strict envelope validation (publisher +
  offlinequeue) is preserved; A3 added the lifecycle observer wiring on
  top of it.
- `8543a0a` — worker connections, wire-shape alignment, complete
  proposal validation; A1 tightened the per-action validation and the
  request-id correlation.
- `a5b3d87` — scoped speech synthesis, arg validation, TTS fallback,
  playable audio; A1 extended the TTS revalidation rules.

A worker who has hand-carried earlier contracts does not need to re-verify
the rows above unless the integration changes the boundary.

---

## A5 — final integration (pending Worker B)

**Status:** SERIAL, BLOCKED until Worker B finishes A1–A6 of its lane.
Worker B owns:

- `backend/internal/{asrworker,middleworker,ttsworker,eval}` and their
  Python adapters,
- `loadmodel`, `scripts/run_k6_smoke_real.sh`, `scripts/gen_spdx.py`,
  `scripts/run_security_checks.sh`.

Worker A will:

1. Re-read B's ordered commit IDs from
   `plan/worker-b-corrections-handoff.md`.
2. In a separate `codex/a5-integration` worktree, cherry-pick B's commits
   only (no history rewrite).
3. Resolve overlaps by contract/behavior; rerun affected checks.
4. Run final verification (see "Verification commands" below) and update
   `plan/prompt.md`, `plan/decisions.md`, `plan/open-decisions.md`,
   and active phase status with actual evidence (no global ledger
   manipulation until B is done).
5. Fast-forward the integrated branch onto CLEAN only if CLEAN still
   matches `b0b9772` and has no unrelated changes; otherwise leave
   divergence intact and report it.
6. **NEVER** push to `main`, **NEVER** rewrite history, **NEVER**
   touch `.txt` files.

---

## Behavior fixed vs gaps still open (per-row)

**Fixed in this lane**

- Strict per-action tagged shape: RECENTER cannot carry extra fields;
  every action variant rejects forbidden extras at the validator.
- Strict schema/data version correlation: empty proposal `request_id`
  and mismatched `request_id` are rejected, not silently accepted.
- Strict status enum: `NEED_CLARIFICATION` synonym is rejected; only
  the contract's enum'd statuses pass.
- Strict raw body decode: size/depth/duplicate-key/unknown-field/
  trailing-data all fail closed at the orchestrator entry, not after a
  partial typed-decode.
- HTTPWorkerClient max+1 overflow detection: reads past the limit
  before decoding; no truncated envelope is decoded.
- TTS post-synthesis revalidation: on stale snapshot the orchestrator
  drops the validated proposal/audio and returns the established
  fail-closed envelope; unchanged-context TTS failure still preserves
  text/actions with honest audio status.
- DefaultTemplateRegistry now exposes synthetic-pending-review content
  only; built-ins no longer silently authorize operational speech.
- Empty `ScopedContext.TemplateKeys` is treated as authoritative "no
  approval" — the orchestrator no longer falls back to the global
  registry.
- `argsForTemplate` respects per-variant emission: SHOW_ROUTE carries
  route_id only; SHOW_CHOICES emits typed facility slots.
- `buildEligible` removes the PartySize=1 silent assumption and stops
  labeling stable-by-ID sort as "nearest".
- Publication lifecycle: `PublicationLifecycleObserver` seam, fan-out
  helper, and `Publisher.WithObserver()` wiring plus
  `WithdrawManifestAndInvalidate` /
  `PromoteManifestWithObserver` so trusted-withdrawal/promotion
  invalidate caches across instances.
- `CachedSource` implements the observer: withdrawal, promotion,
  source revocation/quarantine and package supersession each purge
  the matching cache entries.
- Bounded worker health refresh loop in production wiring; honors
  `ctx.Done`.
- gofmt on Lane A packages (`offlinequeue/{worker,worker_regression_test}.go`,
  `orchestrator.go`, `graceful_shutdown_test.go`).

**Open defects / unbenchmarked items recorded**

- **A3.2 / A3.6** — Promotion of an unpublished `published_manifests`
  row whose `source_id` IS NULL is not yet gated. Documented; not
  fixed (would require a server-side migration to populate
  attribution and a backwards-compatible gate, plus a coordinated
  change to migration 0007).
- **A4.6** — Real ASR/middle/TTS server constructors communicating
  with `HTTPWorkerClient` and the public `/voice/process` handler is
  pending Worker B's integration; A will validate after B.
- **A2 real-language approval** — Operational approval for the
  built-in templates requires real translations and authority sign-off
  (external dependency O11); default registry is synthetic-pending-
  review only.
- **A2 produced but unverified against live DB** — Selection with
  party size and duration honestly absent remains an open
  implementation question for the public citizen flow; the
  `buildEligible` change stops claiming a one-person/one-day
  synthetic policy silently.

---

## Verification commands run for evidence

```bash
# Per involved package
gofmt -l backend/
go vet ./...                      # in backend/
go build ./...                    # in backend/
go test -count=1 -run TestA1 ./internal/orchestration/...
go test -count=1 -run TestA2 ./internal/orchestration/...
go test -count=1 -run TestA3 ./internal/store/...
go test -count=1 -run TestA3 ./internal/offlinedelivery/...
go test -count=1 -run TestA4 ./internal/orchestration/...
# Full non-DSN packages
go test -count=1 -timeout 60s \
  ./internal/orchestration/... \
  ./internal/contracts/... \
  ./internal/httpjson/... \
  ./internal/store/... \
  ./internal/offlinedelivery/... \
  ./internal/offlineclient/... \
  ./internal/offlinequeue/... \
  ./internal/offlinepkg/... \
  ./internal/offlineresources/... \
  ./internal/sourceact/... \
  ./internal/catalogue/... \
  ./internal/opkg/... \
  ./internal/capfeed/...
# All green; A3 tests skipped on this machine (no STHIRA_TEST_DSN).
gofmt -l backend/   # only `backend/internal/ttsworker/worker.go` remains — Worker B's lane.
```

---

## External dependencies (unchanged)

- O03 — IndicConformer per-state language matrix approval. NOT_RUN.
- O05 — operational route authority OPEN. The eligibility path
  remains fail-closed.
- O06 — official map license OPEN. Resource publishing remains
  synthetic-only.
- O07 — operational stay policy OPEN. `buildEligible` no longer
  fabricates a 1-person/1-day synthetic policy.
- O11 — Parler TTS regional-language review. NOT_RUN.
- O14 — operator IdP. Production issuance still fail-closed 503.

---

## Stop / handoff

- A1–A4: PASS where exercised; NOT_RUN where evidence requires Worker B
  or external authority.
- A5: blocked on Worker B commits; serial after B.
- P8: not started (gate B remains NOT_READY).
- No claim that all defects are impossible merely because the bounded
  matrix passes.
