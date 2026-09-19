# Phase execution prompts and evidence ledger

2026-09-19 · Go/mobile production-preparation plan. The 2026-09-19 planning task changes documents only; these prompts are instructions for subsequently authorized implementation. No phase is automatically completed by generating this file.

## How to continue

When the user says “move to the next phase,” inspect this ledger and the working tree, verify the last phase's evidence against the current revision, then execute the earliest eligible unfinished phase. Do not replay completed Python phases or simply describe a plan. Preserve the original objective and user clarifications. The frontend decision/implementation starts only after backend gate B.

If only part of a phase can run, finish that independent part and record it, but do not mark the phase DONE. Independent later backend work may use a completed prerequisite contract slice where explicitly listed; a missing user dataset/GPU never grants a false phase pass. Government/live activation remains disabled. No automatic push/deployment/active attack on external systems.

## Master execution rules

1. Read plan/prd.md, rules.md, the phase's listed files and applicable root instructions. Inspect staged/unstaged changes and callers before edits. Proposed backend/contract paths below are new deliverables, not existing files; verify conventions first.
2. Write observable acceptance before implementation. For a defect, reproduce the failure. For a migration, capture required behavior and consciously reject obsolete/unsafe demo semantics. Existing test counts are not evidence for the new revision.
3. Use the simplicity ladder: existing code/pattern, stdlib/native capability, justified dependency, minimal code. No blanket line-for-line translation, speculative services, empty scaffolding or UI redesign before P8.
4. Do not invent government URLs/auth/quotas, route policy, model language support, tool APIs, benchmark numbers, legal approval, public claims or test results. Check installed/pinned docs/types first. Record proposals and missing evidence in open-decisions.md.
5. Retain government authority, explicit confirmation, privacy and signed/fresh-source boundaries. LLM/schema validation is not authorization. Test real DB/device/model boundaries where the phase requires them; mocks prove only mocked seams.
6. Run relevant checks and inspect the final diff. Record commit/revision, exact command/tool/version, fixture/data/model hashes where applicable, environment, result and limitations. Fixes invalidate affected earlier checks.
7. Commit every coherent verified stage by default, stage only your changes and preserve others. If a safe commit cannot be separated, ask about the concrete overlap. Do not amend/reset/push/merge/publish/deploy without authorization. Never change .txt files under these instructions.
8. Update the existing ledger/decisions/changes concisely. Before pausing, record files, decisions, tests, commits, blockers and next step. No transcript-sized notes or new tracking framework.
9. After three unsuccessful attempts on the same issue without new evidence, stop that issue and report what is required; continue independent authorized work. Respect user cost/time caps; do not start paid inference/security jobs without their approved budget/scope.

## Ledger

Status vocabulary: NOT_STARTED, IN_PROGRESS, BLOCKED_EXTERNAL, DONE. Evidence applies only to the revision checked. `BLOCKED_EXTERNAL` here is a document status, not a claim that code is complete. The former Python/hackathon ledger is historical in changes.md.

| Phase | Name | Prerequisites | Status | Evidence / commit |
| --- | --- | --- | --- | --- |
| P0 | Reconcile scope and freeze migration evidence | None | DONE | Baseline frozen at `ce6adca`; see "P0 baseline evidence" below |
| P1 | Go foundation and executable contracts | P0 | DONE | Go 1.27.1 pinned; bounded `/api/v3` slice in `backend/`; see "P1 completion record" below |
| P2 | Government-data contracts and scenario ingestion | P1 | PARTIAL | Infra DONE (capfeed/opkg/sourceact/catalogue/context-resolver); catalogue acceptance BLOCKED on O01 user data; see "P2 completion record" below |
| P3 | Durable storage, authorization and ledger foundation | P1 + P2 contract slice | IN_PROGRESS | Checkpoint A DONE; storage layer + migration written; real-DB verification BLOCKED_EXTERNAL (no PostgreSQL/PostGIS, sandbox denies driver fetch); see "P3 Checkpoint B record" below |
| P4 | Destination choice and immediate/temporary stays | P2 + P3 | NOT_STARTED | None for new implementation |
| P5 | Offline package and map-delivery protocol | P2 + P3 | NOT_STARTED | None for new implementation |
| P6 | Regional ASR, constrained middle model and TTS | P1 + P4 + P5 | NOT_STARTED | None for new implementation |
| P7 | Backend security, performance and handoff gate B | P0–P6 acceptance evidence | NOT_STARTED | None for new implementation |
| P8 | Select and implement Android and iPhone clients | Gate B | NOT_STARTED | None for new implementation |
| P9 | Whole-system readiness, regional drills and release assurance | P8; full P2/P6 data/language acceptance | NOT_STARTED | None for new implementation |
| P10 | Retire obsolete files and verify the final artifact | P9 | NOT_STARTED | None for new implementation |
| P11 | Authorized government integration and shadow exercises | Gate S + O05/O07/O08 evidence | BLOCKED_EXTERNAL | None for new implementation |
| P12 | Controlled launch and operational scale-out | P11 + explicit release authorization | BLOCKED_EXTERNAL | None for new implementation |

Planning preparation: documents updated; not counted as P0 implementation. Replace “None” only with actual evidence. P11/P12 are blocked by authorization/source/field evidence, not by a presumed coding defect.

## P0 baseline evidence (frozen 2026-09-19)

Verified against starting commit `ce6adca093b2c6cd62baa01d0b90716290a82abd`. This is a snapshot of actual behavior, not a quality/security certification. Historical Python results are reference evidence only; they do not satisfy Go/mobile/live gates.

**Scope reconciliation.** Root instructions (CLAUDE.md, GEMINI.md) reflect the 2026-09-19 pivot: citizen emergency guidance over government data, no hazard/safe-land prediction, route authority OPEN (O05). The governing plan fixes Go as the product backend, Android+iPhone at launch, the constrained middle model, immediate + 7–30 day temporary scope, one million total users (incident concurrency to be measured, P7) and 10–15 demo states with regional languages (zones supplied later). At P0 the root instruction *content* was already aligned, but `tasks/todo.md` still presented the Python/hackathon workflow as the active tracker; that reconciliation gap was corrected in P1 by marking `tasks/todo.md` a historical Python tracker and pointing active tracking at this Go/mobile phase ledger. Completed Python items remain as historical evidence in `tasks/todo.md` and `plan/changes.md`; they do not satisfy Go/mobile/live gates.

**Actual API inventory** — entry `src/sthira/api/app.py` (FastAPI title still "permanent relocation … Wayanad") mounts `sthira_v2` at `/api/v2` and adds CORS + `security_middleware`; `/ui` and `/v2` static mounts. 17 v2 routes, all synthetic/in-memory:

- Runtime/status: `GET /status`, `GET /health/readiness`, `GET /readiness` (compact compat shape).
- AI/voice: `GET /ai/provider/status` (config only, no paid call), `GET /voice/status`, `POST /voice/transcriptions`, `POST /voice/speech` (allow-listed text only), `POST /voice/commands` (constrained map contract).
- Alerts: `GET /alerts/active`, `GET /alerts/health`, `GET /alerts/quarantine`, `GET /alerts/{identifier}`.
- Demo/package: `GET /demo/scenario` (reads `frontend/v2/src/scenario.json`, requires `evidence_class==SYNTHETIC_DEMO`), `GET /operational-packages/active`.
- Assignment: `POST /assignments`, `GET /assignments/{id}`, `POST /assignments/{id}/arrival-confirmations` (idempotency-keyed, in-memory).

`alert_service`, `allocation_service`, `package_service` are **process-memory** singletons built at import; SQLAlchemy/PostGIS scaffolding exists but the API does not use durable state. Voice/transcription/synthesis instantiate runtimes per request. These are demo seams to port (P1–P7), not contracts to preserve.

**Dependency traps confirmed.** `tests/conftest.py` imports legacy `live_ops`; `tests/test_v2_*` API tests import the legacy entry `sthira.api.app`, so deleting `src/sthira` breaks v2 tests. Two parallel CAP paths exist: `src/sthira_v2/cap.py` (lifecycle + HTTP adapter) and `src/sthira_v2/xml_parser.py` — consumer/evidence mapping required before dedup (P2). Scenario catalogue lives under frontend source ownership (`frontend/v2/src/scenario.json`) and must migrate out before that path retires (P2/P8).

**Frozen invariants to port** (evidence, not target truth): source provenance + evidence class on every fact; CAP update/cancel/supersession; capacity conservation with explicit party size and idempotent reserve/arrival (arrival converts hold→occupied without a second decrement); duplicate-retry returns stored result / changed payload conflicts; stale/expired cache never authorizes guidance; voice cannot call, confirm arrival, or mutate capacity.

**Toolchain.** `.venv` Python 3.11.16, fastapi 0.141.1, pydantic 2.13.5, SQLAlchemy/geoalchemy2/alembic/psycopg present, pytest 9.1.1; node v26.8.1, npm 11.19.0. **Go toolchain: not installed** — P1 must pin and install a supported Go before any Go work. No GPU/model benchmark run.

**Baseline checks (this revision, no paid calls, no data change).** `compileall src tests` exit 0. `pytest -q`: **258 passed, 1 failed** (259 total). Frontend `npm run build` exit 0 (chunk-size warning only). The one failure is `tests/test_v2_demo_scenario_api.py::test_active_alert_api_exposes_synthetic_provenance_and_raw_artifact`: bundled `fixtures/synthetic_cap_alert.xml` has `<expires>2026-09-13T04:00:00Z</expires>`, now in the past, so `alerts/active` returns empty and the test IndexErrors. Time-dependent fixture expiry, not a code regression; not repaired under P0 — fixture freshness semantics belong to P2. All `.txt` files verified untouched (staged + unstaged diffs contain no `.txt`).

## P1 completion record (2026-09-19)

```text
Phase / status: P1 — Go foundation and executable contracts — DONE
Starting and checked revision: CLEAN branch at merge 2bea1fc (P0 baseline ce6adca reconciled)
Scope completed and changed files:
  - P0 reconciliation gap closed: tasks/todo.md marked a historical Python
    tracker (active tracking points here); the P0 scope-reconciliation
    statement above corrected to describe the verified state.
  - backend/go.mod (module sthira/backend, go 1.27.1)
  - backend/cmd/sthira/main.go (entrypoint, graceful shutdown, optional
    STHIRA_DEMO_CONTEXT for manual smoke only)
  - backend/internal/contracts: envelope.go, errors.go, geometry.go, model.go
    (frozen /api/v3 types, stable error codes, provenance/evidence/freshness,
    GeoJSON lon/lat, constrained middle-model contract + independent validator)
  - backend/internal/httpjson/decode.go (strict JSON: size/depth limits,
    duplicate-key, unknown-field, trailing-data, malformed detection)
  - backend/internal/httpserver: config.go, server.go, handlers.go (bounded
    server, method/content-type enforcement, timeouts, cancellation, graceful
    shutdown, separate liveness/readiness, readiness prober seam)
  - backend/contracts/openapi.yaml (frozen contract for the implemented slice)
  - backend/testdata/json/* and backend/testdata/model/* (golden fixtures)
  - Tests: internal/httpjson/decode_test.go, internal/contracts/model_test.go,
    internal/httpserver/server_test.go
  - Makefile check-go/test-go targets; CI setup-go 1.27.1 (Python/frontend
    checks preserved)
Tests/commands, environment and results (Go 1.27.1 darwin/arm64, GOCACHE=$TMPDIR):
  - gofmt -l . → clean
  - go vet ./... → clean
  - go build ./... → ok
  - go test ./... -count=1 → ok (contracts, httpjson, httpserver); cmd has no tests
  - Real HTTP smoke (go run ./cmd/sthira): /health/live=200; /health/ready=503
    (BLOCKED, no false READY); valid proposal=200 with STHIRA_DEMO_CONTEXT=1 and
    =422 fail-closed without it; duplicate key=400; prohibited action=422;
    wrong method=405; oversized body=413 (unit test)
Source/model/data/build versions used: Go 1.27.1 (Homebrew bottle, arm64);
  standard library only; no external modules; no model/data sources.
Commit(s): 796ff45081c8443b970ca760f7cdaa6fad62a05e (CLEAN branch)
Unresolved internal work: known-IDs context is a fixture seam (per-request
  source-snapshot resolution arrives in P2/P4); readiness prober is a seam with
  no real dependencies yet; v2 compatibility strategy recorded in trd.md.
External dependency, owner and exact evidence needed: none for P1. (Go install
  required running outside the agent sandbox; now pinned.)
Next eligible step: P2 — Government-data contracts and scenario ingestion.
```

## P2 completion record (2026-09-19)

```text
Phase / status: P2 — Government-data contracts and scenario ingestion —
  INFRASTRUCTURE DONE; catalogue acceptance BLOCKED on O01 (missing user data).
  Per the phase gate, missing user zones/reports prevent a full P2 scenario
  acceptance claim, so P2 is not marked wholly DONE.
Starting and checked revision: CLEAN branch at fceef6d (P1 796ff45 + 0761ae9).
Scope completed and changed files (all new Go, stdlib-only, offline-buildable):
  - backend/internal/capfeed/cap.go — bounded CAP 1.2 parse: 1 MiB limit,
    explicit <!DOCTYPE/<!ENTITY rejection (Go encoding/xml tolerates a bare
    DOCTYPE, so it is rejected by pre-check), malformed/empty/oversized
    rejection, sender allow-list, RFC 3339 timezone-aware timestamps,
    expires>effective, CAP lat,lon → GeoJSON lon,lat polygon conversion with
    closure + CRS bounds, raw-artifact preservation + SHA-256 digest, and
    Actual+Public operational distinction (Exercise/Test/System/Draft and
    non-Public are never operational).
  - backend/internal/capfeed/lifecycle.go — injected-clock lifecycle: dedup
    (idempotent), Update→SUPERSEDED, Cancel→CANCELLED, unknown-reference and
    out-of-order (stale) quarantine with safe reason codes, Expire() excludes
    expired alerts from Active. Concurrent-safe.
  - backend/internal/capfeed/transport.go — conditional retrieval over an
    injectable Fetcher: 200 updates cache, 304 revalidates via stored ETag,
    bounded retries with exponential backoff, per-attempt timeout, stale-cache
    preservation (STALE_CACHE) or UNAVAILABLE when no cache; never fabricates.
  - backend/internal/opkg/package.go — operational-package validation: evidence
    class, version, jurisdiction, effective/expiry window, explicit
    non-negative safe-zone capacity, CRS-bounded zone locations and LineString
    route geometry, red/safe-zone cross-references, authorized route approval,
    and an allocation policy that must explicitly order every safe zone.
    Canonical checksum excludes checksum_sha256 and signature (integrity, not
    authority); signature validity is verified by an injected Verifier and is
    distinct from signer authorization. Self-consistent across marshal round-trip.
  - backend/internal/sourceact/activation.go — source activation lifecycle
    DISCOVERED→…→OPERATIONAL with legal-successor transitions, optimistic
    version increments, per-transition audit; only OPERATIONAL may drive
    guidance; SUSPENDED blocks; RETIRED is terminal. In-memory (preview/tests).
  - backend/internal/catalogue/catalogue.go — canonical scenario catalogue
    validator owned outside frontend source. Historical event evidence is kept
    distinct from synthetic geometry/exercise time; historical scenarios must
    reference a known evidence record, synthetic must not. Structural failures
    are errors; missing user data (states, language evidence) is a blocking
    Gap, never invented. frontend/v2/src/scenario.json remains a consumer until
    deliberately migrated.
  - backend/internal/httpserver/{server,handlers}.go + cmd/sthira/main.go —
    closed the client-echo trust gap in handleVoiceCommands: authoritative data
    version/jurisdiction/permitted IDs are now resolved per request via a
    ContextResolver against server state; client-echoed request_id/data_version
    are correlated, never trusted. No resolver → fail closed 503; stale client
    data_version → 409. Removed dead knownIDs/enabledLanguages server fields;
    demo smoke path uses a static server-side snapshot resolver.
Tests/commands, environment and results (Go 1.27.1 darwin/arm64, GOCACHE=$TMPDIR):
  - gofmt -l . → clean; go vet ./... → clean; go build ./... → ok
  - go test (non-socket pkgs) -count=1 → ok: capfeed (30), opkg (24),
    sourceact (13), catalogue (12), contracts, httpjson
  - Bounded parser fuzz: go test -fuzz FuzzParse -fuzztime 15s → PASS
    (~1.5M execs, no panic/hang; non-CAPError results rejected by invariant)
  - Socket-bound HTTP suite run in user terminal (sandbox blocks bind):
    go test ./internal/httpserver/ -v → 14 PASS incl.
    TestVoiceCommandsFailsClosedWithoutResolver (503) and
    TestVoiceCommandsRejectsStaleDataVersion (409)
Source/model/data/build versions used: Go 1.27.1; standard library only; no
  external modules; no model/data sources; no paid calls; no .txt changes.
Commit(s) (CLEAN branch):
  06537fd CAP ingestion/lifecycle/transport
  9360178 operational-package validation
  c0ec0c6 source activation lifecycle
  b1eb86c scenario catalogue validator
  fceef6d server-side context resolution (trust fix)
Python CAP-expiry resolution (Slice 2): the existing
  tests/test_v2_cap.py::test_lifecycle_deduplicates_updates_cancels_and_expires
  already uses controlled test-time semantics — an injected mutable clock
  (now[0] advanced +3h) drives expiry, with a separate assertion that expired
  alerts are excluded from active(). No fixture date was moved, no expiry check
  disabled, no assertion weakened. It passes.
Python baseline drift (recorded honestly; sandbox/product-separate; NOT repaired
  under P2 Go scope): full suite is now 7 failed / 259 passed, not the 1-failure
  baseline P0 recorded. CORRECTION (P3 Checkpoint A): the seven failures are NOT
  all authentication failures. Verified breakdown after collaborator commit
  d1ce025 ("snapshot polished emergency guidance") rewrote
  src/sthira/api/middleware.py AND deleted routes from src/sthira_v2/app.py:
    - 5x 401 auth-gating (test_v2_assignment_api, test_v2_cap alert API,
      test_v2_demo_scenario_api x2, test_v2_operational_package_api) after
      d1ce025 dropped /api/v2/alerts, /api/v2/demo, /api/v2/operational-packages
      and the assignment POST/GET exemptions from the public set.
    - 1x 404: GET /api/v2/ai/provider/status (test_v2_nemotron) — d1ce025
      deleted the route outright; not an auth failure.
    - 1x missing X-Request-ID header (test_v2_request_observability) — d1ce025
      removed the request-correlation middleware; not an auth failure.
  Resolution is recorded in the P3 Checkpoint A completion record below.
Unresolved internal work: catalogue has no user-supplied states/zones yet, so
  acceptance is blocked by O01; source activation and catalogue are in-memory
  (durable persistence is P3); voice-commands resolver is a static seam until
  P4 binds a real source snapshot; the two parallel Python CAP paths
  (cap.py vs xml_parser.py) still need consumer/evidence mapping before dedup.
External dependency, owner and exact evidence needed: O01 — user/authority must
  supply demo states, zones, routes, approvals and historical-event references
  before catalogue acceptance can be claimed. Python middleware auth policy for
  v2 demo routes needs an owner decision.
Next eligible step: P3 — Durable storage, authorization and ledger foundation
  (P1 + P2 contract slice are satisfied).
```

## P3 Checkpoint A completion record (2026-09-19) — handoff correction

```text
Phase / status: P3 Checkpoint A — correct and complete the P2 handoff — DONE
  (Python reference contract restored; Go contract reconciliation done).
Starting and checked revision: CLEAN branch at 24bddf1 (P2 ledger record).
Scope completed and changed files:
  A1. Corrected the P2 evidence record above: the seven Python failures were
      5x 401 + 1x 404 (deleted /ai/provider/status) + 1x missing X-Request-ID,
      not seven authentication failures.
  A2. Restored the Python reference contract broken by collaborator d1ce025:
      - src/sthira/api/middleware.py — restored X-Request-ID correlation
        (validated via sthira_v2.security.validate_public_identifier, safe
        req- fallback on invalid), content-length bounding, and security
        headers (X-Content-Type-Options, Referrer-Policy, X-Frame-Options) on
        EVERY response including 401/413/503 errors. Public-read set covers
        citizen guidance only: /api/v2/alerts/active, /demo/scenario,
        /operational-packages/active, /ai/provider/status, /readiness,
        /status, /health/readiness, /voice/status. Private reservations
        (/api/v2/assignments) and operator source-health (/alerts/health,
        /alerts/quarantine) require a verified session (R22); they are NOT
        broadly exposed just to pass tests.
      - src/sthira_v2/app.py — restored the routes d1ce025 deleted:
        ai/provider/status, /readiness (compact compat), alerts/active,
        alerts/health, alerts/quarantine, alerts/{id}, demo/scenario,
        operational-packages/active, assignments POST/GET/arrival. Kept the
        newer guidance/chat route. Verified the frontend demo client only
        consumes chat/readiness/status/voice (all still public); no other
        consumer breaks.
  A3. Resolved the CAP-expiry API test with controlled test-time at the
      service boundary: app.py now exposes a settable demo_clock and
      reset_demo_alert_service(); tests inject a "now" inside the fixture
      validity window (2026-09-12/13) and separately assert expired alerts are
      excluded once the window passes. The fixture date was NOT moved and
      expiry was NOT disabled. Expiry is terminal in a lifecycle service, so
      the expiry assertion uses an isolated service, not the shared singleton.
  A4. backend/internal/catalogue — separated structural validation from launch
      acceptance. Validate() stays structural + data gaps; new AssessLaunch()
      applies the T13 launch bar (10-15 states, 2-3 sourced scenarios/state).
      Added structural regressions: duplicate scenario IDs across states and
      cross-state historical references (a scenario may not borrow another
      state's event). Missing user data stays an explicit Gap; a draft manifest
      does not require completed P6 language benchmarks.
  A5. backend/internal/opkg — reconciled the package contract with the
      persisted model (trd.md domain boundaries): Route gains mode (required),
      verified_by/verified_at (paired), valid_from/valid_until (paired,
      ordered); Zone gains role; AllocationPolicy gains reservation_expiry,
      temporary_stay min/max days (paired, ordered), allow_walk_ins,
      allow_transfers. Documented that structural validity is distinct from
      current/authenticated/authorized operational status.
Tests/commands, environment and results:
  - Python (.venv Python 3.11.16, pytest 9.1.1): full suite 267 passed
    (was 7 failed / 259 passed at P2 handoff). No paid calls; no .txt changes.
  - Go 1.27.1 (GOCACHE=$TMPDIR): gofmt clean, go vet clean, go build ok,
    go test ./internal/catalogue ./internal/opkg -count=1 ok.
Commit(s) (CLEAN branch):
  324b019 restore Python reference contract (middleware + routes + CAP-expiry)
  c522bd2 catalogue structural-vs-launch separation + regressions
  cc7f4c0 package contract persisted-model reconciliation
Unresolved internal work: none for Checkpoint A. The two parallel Python CAP
  paths (cap.py vs xml_parser.py) still need consumer/evidence mapping before
  dedup (deferred; not P3 scope).
External dependency, owner and exact evidence needed: none for Checkpoint A.
Next eligible step: P3 Checkpoint B — durable storage/authorization/ledger.
  PostgreSQL/PostGIS is NOT installed and the sandbox blocks network egress
  (no Homebrew, no Go module fetch for a Postgres driver), so real-DB
  verification is BLOCKED_EXTERNAL; see the P3 record when written.
```

## P3 Checkpoint B record (2026-09-19) — durable storage foundation

```text
Phase / status: P3 Checkpoint B — durable storage, authorization and ledger
  foundation — IN_PROGRESS. Storage layer and migration written and compile/
  unit-verified offline; real PostgreSQL/PostGIS verification BLOCKED_EXTERNAL.
Starting and checked revision: CLEAN branch at 2814e04 (Checkpoint A record).
Scope completed and changed files:
  - backend/migrations/0001_p3_foundation.sql — additive migration: sources,
    source_authorizations (activation requires recorded evidence, not just an
    enum advance), source_artifacts, packages (with supersession), versioned
    zone/route/facility facts (PostGIS geography SRID 4326, wrong-SRID rejected
    by the type), sessions (CITIZEN / jurisdiction-scoped OPERATOR),
    facility_inventory (reserved<=capacity conservation CHECK), reservations
    (RESERVED/ARRIVED/DEPARTED/CANCELLED/EXPIRED), scoped idempotency_keys
    (payload-hash bound, lost-response replay), append-only audit_events with a
    sha256 hash chain, outbox_events, schema_migrations. DOWN section documented.
  - backend/internal/store/ — database/sql layer over a DBTX seam so SQL and
    transaction scope are concrete without a live DB:
    store.go (InTx atomic change+audit+outbox; execConditional translates zero
    RowsAffected into ErrVersionConflict/ErrNotFound — real optimistic
    concurrency via UPDATE...WHERE version=$expected, never a mutex);
    source.go (transitions gated on HasAuthorization evidence);
    audit.go (ChainAuditor hash chain + VerifyChain for restore/consistency);
    idempotency.go (Begin/Complete/Fail/Sweep, payload-conflict and in-progress
    distinction); reservation.go (atomic inventory conservation, EnsureInventory
    first-insert path); readiness.go (probes DB ping + migration revision,
    distinguishes DB/app readiness from operational source readiness, redacts
    DSN detail). Open() documents the blocked pgx wiring explicitly.
Tests/commands, environment and results:
  - Go 1.27.1 (GOCACHE=$TMPDIR): gofmt clean, go vet clean, go build ./... ok.
  - go test ./internal/store ./internal/sourceact ./internal/opkg
    ./internal/catalogue ./internal/contracts ./internal/capfeed
    ./internal/httpjson — all ok. store has unit tests for the pure hash-chain
    computation only (deterministic, field-sensitive, chain-linked).
  - internal/httpserver tests FAIL in this environment with
    "bind: operation not permitted" — the sandbox blocks socket bind; this is
    pre-existing and unrelated to the store change (httptest cannot open a port).
Commit(s) (CLEAN branch): 53acd93.
Unresolved internal work: none for the offline-deliverable slice.
External dependency, owner and exact evidence needed (BLOCKED_EXTERNAL):
  Real-DB verification cannot run here. Needed: PostgreSQL 15+ with PostGIS 3.x
  and the pgx/v5 driver. The sandbox denies network egress (proxy.golang.org
  Forbidden, Homebrew denied) and socket bind, and no Docker is present. The
  bundled provisioning/verify command is supplied to the user separately; once
  a DSN is available the steps are: apply 0001_p3_foundation.sql to a fresh
  instance, fetch pgx, then run the migration/concurrency/restart/restore/
  authorization tests the mandate lists (fresh-migration apply, PostGIS SRID
  constraint, competing processes for last spaces, stale-version conflict,
  duplicate idempotency keys, crash-after-commit retry, restart persistence,
  cross-session/jurisdiction denial, backup restore, unavailable-DB readiness).
  P3 is NOT DONE until those pass on the real database.
Next eligible step: provision PostgreSQL/PostGIS + pgx (user-run command), then
  execute the real-DB verification suite. P4 stay workflows remain out of scope.
```

## Required completion record

```text
Phase / status:
Starting and checked revision:
Scope completed and changed files:
Tests/commands, environment and results:
Source/model/data/build versions used:
Commit(s):
Unresolved internal work:
External dependency, owner and exact evidence needed:
Next eligible step:
```

Use DONE only when all exit conditions pass. “Software ready for government integration” requires gate S (P0–P10), including both mobile platforms and selected regional languages. “Operational” additionally requires P11/P12. No budget exhaustion, documentation completion or synthetic demo can close those gates.

# Implementation prompts

## P0 — Reconcile scope and freeze migration evidence

Prerequisites: None. Status: DONE 2026-09-19 at `ce6adca`; evidence in "P0 baseline evidence" above.

### Read and establish

Read plan/plan.md, plan/prd.md, plan/rules.md, plan/cleanup.md, CLAUDE.md, GEMINI.md, tasks/lessons.md and the current git status/staged diff. Read the actual entry point, middleware, test bootstrap, Makefile, Dockerfile, Compose, pyproject and frontend package manifest. Treat the inventory as a dated snapshot and verify it before deleting or porting anything.

### Execute

1. Confirm this phase is now authorized for implementation work; the 2026-09-19 task itself was planning-only. Preserve unrelated edits and staged moves. Record the starting commit and a compact path/status inventory.
2. Reconcile root agent instructions and tasks/todo.md with this plan: Go product backend, both mobile platforms, constrained middle model, immediate/temporary scope and open route authority. Remove obsolete active hackathon TODOs, retaining concise historical evidence links. Do not modify .txt files.
3. Trace src/sthira/api/app.py → middleware → sthira_v2 router → fixtures/local voice and tests/conftest.py. List actual /api/v2 routes, request/response/error/auth/cache behavior and known reference-only behavior. Do not treat defects or unsafe demo defaults as contracts to preserve.
4. Mark each legacy module PORT/RETIRE/KEEP with caller/build/test evidence; identify CAP/parser duplication and frontend-owned scenario data. Note any actual non-demo stored data and external clients before proposing migration/deletion.
5. Freeze representative synthetic fixtures and expected invariants: provenance, cancel/update, capacity conservation, duplicate retry, stale cache and forbidden voice action. Existing code is evidence, not automatic target truth.
6. Record selected test/tool versions and missing hardware/data/policy decisions. Update only durable evidence in the existing plan/tracker; no new generic project-management framework.

### Verify

Run existing relevant baseline checks once, without making paid model calls or changing data: inspect Makefile targets first; use local pytest/current frontend build where dependencies are available. Record failures and environmental limits honestly. Verify the route/import/dependency inventory, .txt preservation and final diff. A failed old baseline is not silently repaired under cleanup scope.

### Deliver and stop

Deliver a reconciled instruction/tracker baseline, actual API/invariant inventory and refined cleanup manifest. No legacy files deleted and no Go scaffolding required. Baseline limitations have owners and do not become claimed passes.

## P1 — Go foundation and executable contracts

Prerequisites: P0. Initial status: NOT_STARTED.

### Read and establish

Read src/sthira_v2/contracts.py, config.py, app.py, security.py; tests/test_v2_contracts.py, test_v2_openapi.py, test_v2_phase0.py; plan/trd.md and plan/voice-map-system-prompt.md. Inspect existing CI/build conventions. These are references; proposed /api/v3 paths do not exist yet.

### Execute

1. Pin a maintained Go toolchain after checking current support/security. Create backend/go.mod and backend/cmd/sthira/main.go as the proposed isolated location, unless P0 establishes a better existing convention. Create only packages used by this slice; no empty layer scaffolding or generic repository framework.
2. Use net/http, context deadlines, bounded readers and structured safe logs. Define method/path behavior, Content-Type, size/depth limits, cancellation, timeouts and graceful shutdown. Separate liveness from dependency/source readiness; missing infrastructure must not report READY.
3. Freeze /api/v3 schemas in a versioned OpenAPI/JSON contract under backend/contracts/. Include provenance/evidence classes, unknown values, IDs, UTC timestamps, coordinate order, facility/stay/route policy fields, error envelope and session authorization boundary. Record the deliberate v2 compatibility strategy.
4. Implement strict input/output validation and fixture-driven handlers for the minimal health/version/contract slice. Standard JSON decoding alone does not reject duplicate keys; cover the chosen strict-boundary behavior. Do not fabricate database or source success.
5. Define the middle-model schema/tagged actions and golden examples from voice-map-system-prompt.md, independent of a model provider. Validate status/action combinations and known IDs in a test context. No LLM/network is required here.
6. Add the smallest CI/build/check targets for Go alongside existing reference checks. Existing Python/web code remains runnable; do not delete it or port unrelated legacy endpoints.

### Verify

Before relying on new behavior, create failing tests for malformed/oversized/trailing/duplicate-key JSON, unknown enums/fields, nil/unknown versus zero, invalid time/coordinates, method/content-type errors, unsupported model action and false readiness. Run gofmt, go vet ./... and go test ./... from backend; race checks where shared state exists. Exercise the real HTTP server and generated contract, including failure status codes.

### Deliver and stop

Deliver a bootable bounded Go boundary, executable schemas/golden fixtures, exact toolchain/build instructions and CI evidence. No fake source/database readiness, no unrequested domain scaffolding and no frontend implementation. Commit the verified slice.

## P2 — Government-data contracts and scenario ingestion

Prerequisites: P1. Initial status: NOT_STARTED.

### Read and establish

Read src/sthira_v2/cap.py, xml_parser.py, operational_package.py, package_service.py, source_activation.py, source_health.py and fixtures; associated CAP/package/source tests. Read plan/source-register.md and plan/feature.md. No source access or demo geometry may be invented.

### Execute

1. Implement Go parsers/validators and source-state rules against recorded synthetic transport fixtures. Support raw CAP preservation, sender+identifier references, effective/expiry windows, Public/Exercise/Test distinction, update/cancel, out-of-order references, 200/304 and bounded safe retry/backoff.
2. Reject DTD/entities, malformed/oversized data, unsupported coordinate systems, invalid geometry/times and foreign-jurisdiction references. Resolve CAP latitude/longitude versus GeoJSON longitude/latitude deliberately. Quarantine rejected inputs with restricted evidence and safe reason codes.
3. Define incident package, route approval/candidate status, facility/stay policy and version/signature boundaries. A checksum is not publisher authentication. Keep live adapters disabled behind activation evidence; make HTTP transport injectable for fixtures without inventing official endpoints/quotas.
4. Establish one canonical scenario catalogue outside frontend source ownership. Keep historical event evidence separate from synthetic operational geometry and exercise clock. Do not hard-code a new current date to make expired historical fixtures appear live.
5. Prepare the proposed state/language manifest and per-case schema. Ingest user-supplied zones only when received and validated; preserve original provenance and reuse permission. Research/verify actual historical cases against official reports before counting them. Missing zones/reports remain an explicit data dependency.
6. Expose only test/preview validation at this stage; durable publication and source ownership enforcement complete in P3. Do not build optional unrelated hazard connectors.

### Verify

Run CAP lifecycle/HTTP fixture tests (including duplicate-conflicting, update-before-target, cancel and outages), CRS/ring/coordinate/schema tests, wrong signer/jurisdiction tests and cross-reference/expired package rejection. Fuzz parsers with bounded time/resources. Validate scenario manifest counts and labels; do not count placeholders as complete cases.

### Deliver and stop

Parser/activation/preview infrastructure passes deterministic tests. Catalogue completion is separately recorded against O01; missing user data does not prevent independent schema work but prevents a full P2 scenario acceptance claim. No live connector is enabled.

## P3 — Durable storage, authorization and ledger foundation

Prerequisites: P1 + P2 contract slice. Initial status: NOT_STARTED.

### Read and establish

Read src/sthira_v2/persistence/, migrations/, audit.py, allocation.py, security.py, source_activation.py and relevant persistence/allocation/security tests. Inspect whether any current database contains real data. Read plan/equations.md and the authorization fields in trd.md.

### Execute

1. Use PostgreSQL/PostGIS with a pinned compatible version and SQL migrations. Prefer explicit SQL/pgx and concrete transaction functions. Do not equate SQLite tests with PostGIS/locking proof.
2. Implement sources/artifacts, versioned facts/packages, sessions, reservations/stays, facility/date inventory, durable idempotency and audit/outbox storage. Enforce uniqueness, foreign keys, valid ranges, scope and nonnegative balances in the responsible layer.
3. Define transaction boundaries for publish/version supersession and capacity reserve/arrival/release. Persist result, event and outbox atomically. Handle no-row/first-insert contention, not only locking an already existing row.
4. Bind private reads/writes to the authenticated session or jurisdiction-scoped operator. Establish operator authentication integration boundary, least-privileged DB roles and redacted error/log behavior. Demo IDs/headers are not production authentication.
5. Move applicable Go endpoints onto real repositories. No process-memory authoritative ledger and no fallback to empty successful storage after database failure. Readiness actively probes migrations/DB/storage dependencies without leaking secrets.
6. Preserve any existing real data with a documented tested export/migrate/rollback approach; if only synthetic data exists, record evidence and provision isolated test databases. Do not run destructive reset against unknown data.

### Verify

Apply migrations to a fresh real PostgreSQL/PostGIS instance and test upgrade/rollback recovery strategy. Race multiple server processes for the last spaces, initial audit/source rows and duplicate idempotency keys. Kill a request after commit/before response, restart and retry; verify no lost/double events. Test cross-session/jurisdiction access, wrong SRID, backup restore and unavailable-DB readiness. Run Go race detector in addition to DB tests.

### Deliver and stop

Go API uses durable storage and verified authorization; conservation, restart/retry and version invariants pass on real DB. Missing DB/hardware yields BLOCKED_EXTERNAL with exact evidence needed, never a substitute green in-memory result.

## P4 — Destination choice and immediate/temporary stays

Prerequisites: P2 + P3. Initial status: NOT_STARTED.

### Read and establish

Read prd.md, architecture.md route section, equations.md and O05/O07. Trace reference allocation/contracts/package code and tests. Route authority remains open; synthetic policy fixtures are not operational policy.

### Execute

1. Implement jurisdiction-aware place resolution from known aliases/admin IDs with ambiguity results. No LLM-generated geocodes or mandatory multi-field location wizard.
2. Return only eligible destinations under supplied facility, party, stay, capacity and route policy. Support emergency assembly versus overnight and temporary accommodation. Display unknown attributes honestly. Rank by supplied route length only if permitted; do not rank by guessed safety.
3. Separate read-only choice/preview from explicit reservation. Bind chosen facility/route/source version and party size to the confirmation; revalidate at commit. Concurrent capacity changes return a recoverable conflict instead of silently picking a different facility.
4. Implement reserve, explicit arrival, no-show expiry, cancellation, departure, authorized correction, temporary-stay extension and transfer. Maintain date-range capacity where needed. Arrival converts a hold to occupancy; transfer preserves old stay until the new transition is safely confirmed. Handle expiry/arrival races and invalid transitions.
5. Add route import/validation/display contracts for geometry, origin/destination, mode, verification/closures and landmarks. Operational routing remains disabled until O05 closes. No straight-line navigation, auto-approved local shortcut or terrain-derived safety.
6. Provide minimal scoped operator APIs for publish/revoke/correction/quarantine and auditable assistance. Voice can request previews/confirmation screens only. Do not add permanent relocation, scheme eligibility or background geofencing.

### Verify

Exercise API/domain cases for choice, full/closed/unknown capacity, missing policy/route, same-name place, accessibility/transport mismatch, stale selection, duplicate confirmation, multi-day overlap, extension beyond scope, transfer failure and operator scope denial. Assert capacity conservation after every event and restart. Verify red-zone exits are not governed by naive polygon intersection alone.

### Deliver and stop

End-to-end synthetic choice→reservation→arrival→temporary stay→departure/transfer works through real Go+DB boundaries, with no unauthorized live routing. Every consequential action requires explicit confirmation/authorization and durable idempotency.

## P5 — Offline package and map-delivery protocol

Prerequisites: P2 + P3. Initial status: NOT_STARTED.

### Read and establish

Read src/sthira_v2/offline.py, frontend/v2/public/sw.js, current scenario-cache behavior and offline/package tests. Read architecture.md, parameters.md and O06. No native frontend implementation yet.

### Execute

1. Publish minimal public incident cards and versioned manifests separate from private session data. Include provenance, expiry, revision, language resources, cancellations and signed digests. Define key trust/rotation/revocation and monotonic-version handling.
2. Implement conditional requests, immutable object/cache policy, byte/duration limits, atomic verified download replacement and resumable transfer. A failed refresh cannot destroy valid local evidence; an old manifest cannot resurrect a revoked route.
3. Specify downloadable regional vector maps, style/glyph/sprite completeness, gazetteer and approved audio. Use legal sources with explicit offline/redistribution rights; do not bulk-download public OSM tiles or assume Google caching rights.
4. Build a protocol test client/fixture harness, not the deferred production UI. Verify package budgets and untrusted-clock behavior, independent map/data expiries and cold/empty storage states.
5. Define pending offline writes: durable local request reference, unchanged idempotency key, renewed server validation and explicit handling when selection/hold expires. Pending does not mean reserved.
6. Add manifest-based update and revocation propagation with bounded polling/jitter. Choose notification transport later; current push delivery cannot be guaranteed. Avoid custom delta/pack protocols if HTTP/standard signed manifests are adequate.

### Verify

Simulate poor network, truncation/resume, corrupt signature/digest, wrong key, revoked/expired/older package, clock rollback, cache eviction and unavailable map/audio. Measure compressed critical-card bytes and transfer budget. Prove public caching excludes private assignment data and government requests do not scale with users.

### Deliver and stop

Documented tested client protocol and package delivery meet declared byte/security/freshness contracts. Map data licensing/pack-format choices stay blocked where unverified; no claim that a native offline client exists yet.

## P6 — Regional ASR, constrained middle model and TTS

Prerequisites: P1 + P4 + P5. Initial status: NOT_STARTED.

### Read and establish

Read local_voice.py, speech_stt.py, multimodal.py, voice_commands.py, voice_map.py, azure_openai.py and their tests; preserve useful artifact evidence. Read tech-stack.md, voice-map-system-prompt.md, parameters.md and the state-language matrix. Do not infer coverage from model name.

### Execute

1. Inventory exact local ASR/TTS model/tokenizer artifacts, licenses, language IDs, remote-code requirements and runnable formats without loading huge tensors unnecessarily. Add an isolated pinned inference worker; no app-side model or in-process Go embedding of Python framework code.
2. Create a consented/approved multilingual corpus with expected place IDs, intents, ambiguity/failure outcomes and critical spoken templates. Cover every selected state's service languages and dialect/code-switching needs. ASR with no calibrated confidence returns unknown, never a fabricated 1.0.
3. Benchmark Qwen3-4B-Instruct-2507 as the first middle candidate on private vLLM with short context and structured output. Compare a smaller candidate only if useful; no model above the requested approximate 5–6B ceiling without a revised decision. Pin image/weights/tokenizer/quantization and test BF16 against supported quantized deployment.
4. Go retrieves scoped candidate IDs and validates every result against request/version/scope/policy. Reject invalid JSON/IDs/actions, stale responses, unauthorized tools or arbitrary speech. Use reviewed template keys for screen/TTS output; no filler acknowledgments. Simple local camera actions need no model call.
5. Add bounded separate ASR/middle/TTS queues, timeouts, cancellation, warm workers, metrics and load shedding. Binary compressed audio requires actual codec validation/resampling tests and decoded limits. No automatic external paid-model fallback.
6. Cache approved generated audio by language/text/source/model/voice version; invalidate withdrawn content. Test actual speech intelligibility and latency, not just a file header. Keep touch/chat behavior available when any model fails.

### Verify

Run real microphone/recording corpus through ASR→Go→vLLM→validated actions and approved TTS on declared hardware, plus adversarial validator tests and all outage/cancel states. Report per-language critical entity/intent success, false acceptance, ambiguity, speech review, p95 latency, memory and sustainable concurrency. Record model-serving costs; do not execute paid provider calls without an approved spend/scope.

### Deliver and stop

Selected languages and artifacts have evidence; model output cannot authorize safety/capacity/calls; measured serving plan fits the proposed workload or names the gap. Missing GPU/language reviewers means relevant acceptance remains blocked, even when adapter tests pass.

## P7 — Backend security, performance and handoff gate B

Prerequisites: P0–P6 acceptance evidence. Initial status: NOT_STARTED.

### Read and establish

Read assurance.md, parameters.md, equations.md and open-decisions.md. Inspect actual Go deployment/dataflow, dependency versions, authorization, ingress/egress, migrations and model-serving configuration. No native UI work yet.

### Execute

1. Finish a threat model covering public API, government ingestion, operator publish/correction, session ownership, signing keys, audio/LLM and private data. Test every role/scope boundary and ensure only ingestion holds government credentials.
2. Add/run Go vet/race/fuzz, Staticcheck, govulncheck, targeted gosec, Trivy and Gitleaks using pinned tools. Generate an SBOM and review model/code supply-chain assumptions. Triage findings; no blanket ignore lists to turn the build green.
3. Run authenticated API checks and ZAP against isolated owned staging. Prepare a concrete Strix run scope, model, spend/runtime cap, synthetic credentials, outbound restrictions and rollback snapshot. Execute only within authorized scope; never target real government systems. Reproduce/fix findings with deterministic regressions.
4. Use k6 plus pprof/DB query/lock metrics for normal/surge/stress tiers, cold caches, one-shelter hotspots, voice bursts and source outage. Compare equal workloads before optimizing. Fix the measured bottleneck without introducing speculative caches/services.
5. Measure actual per-worker voice capacity and calculate replicas, failure reserve, cost and deployment needs. Record hardware/pricing assumptions and unresolved budget; no fixed server count from total-user population alone.
6. Prove dependency readiness, cancellation/backpressure, graceful restart, backup restore, DB failover and recovery of committed operations. Redact telemetry. Produce backend handoff evidence covering contracts, data ingestion, choice/stay, offline protocol, AI, security and performance.

### Verify

Pass critical invariants, no unremediated critical/high exploitable findings, representative sustained/burst load within approved budgets, recovery tests and measured language/worker gates. Compare final revision against all fixes; rerun affected tests. Government-only external evidence may remain separate, but missing internal DB/GPU/security work cannot be marked done.

### Deliver and stop

Gate B passes only when P0–P7 backend requirements are verified, or explicitly remains NOT_READY with exact internal blockers. This is the user's 80–90% handoff point defined by capabilities, not LOC. P8 production frontend begins only after B passes; planning can continue independently.

## P8 — Select and implement Android and iPhone clients

Prerequisites: Gate B. Initial status: NOT_STARTED.

### Read and establish

Read prd.md, architecture.md offline/voice flow, tech-stack.md candidates, trd.md frozen APIs and device/language budgets. Inspect the old web client only for reusable experience/fixture evidence; do not port its layout blindly.

### Execute

1. Resolve minimum OS/test devices, supported state/language matrix and distribution constraints. Prototype the narrow map download/offline boot/audio/screen-reader slice on Android and iPhone before choosing native Kotlin+Swift, Kotlin Multiplatform/platform UI or a justified alternative.
2. Record measured download size, memory, cold startup, battery/thermal behavior, map offline compatibility, maintenance cost and accessibility. Kotlin by itself does not satisfy iPhone delivery; no silent platform omission.
3. Build the local UI/assets/storage using the selected solution. A large speak control, labelled accessible corner chat button, clear map/card and one next action are the baseline; no heavy animations, mandatory registration or form-heavy location wizard.
4. Connect voice/chat to the same validated API. Implement ambiguity choices, eligible destination preview/selection, explicit reservation/arrival/stay events, route/non-map parity and OS dialler confirmation. Distinguish candidate/unknown/expired/informational states visually and accessibly.
5. Implement verified downloadable maps/gazetteer/audio/packages, safe refresh, revocation, cache limits, permission denial and queued retry semantics. Process location locally when practical; never include government keys or model weights in the app.
6. Add the necessary scoped operator interface/workflow for publish/revoke/source health and corrections, reusing an appropriate small client surface; no unrelated administration suite. Handle mobile audio interruptions, notification permission and background limits, clear privacy disclosure and signed app updates.

### Verify

Exercise actual Android and iPhone builds on physical target devices: voice and chat, disabled microphone/location, screen reader/large text, cold offline boot, expired/empty pack, poor network, low storage/memory, source/model/map outage, audio interruption and confirmed capacity/call paths. Verify model actions are revalidated against the client snapshot. Native UI requires more than a browser screenshot.

### Deliver and stop

Both supported platforms and operator workflows meet behavior/accessibility/offline budgets with real evidence. If one platform or state language fails, launch scope is not complete. No government operational activation is implied.

## P9 — Whole-system readiness, regional drills and release assurance

Prerequisites: P8; full P2/P6 data/language acceptance. Initial status: NOT_STARTED.

### Read and establish

Read all unresolved O-items and previous evidence. Use real selected Android/iPhone devices, actual Go/database/model deployment, approved scenario catalogue and the assurance matrix; no mock replacing the boundary under test.

### Execute

1. Complete 2–3 sourced historical exercises for each selected state using user-supplied validated demo zones and clearly synthetic operational data. Exercise immediate and 7–30 day temporary stays, accessible alternatives and closure/transfer cases.
2. Run observed task tests with low-literacy, older-adult and disability cohorts and appropriately supervised child interactions. Validate regional instructions/ASR/TTS with qualified speakers; document failures rather than averaging them away.
3. Run constrained-network, offline expiry/revocation, model/map/database/source outage, same-name place, closure and capacity drills end to end on both devices. Test safe fallback and pending-request reconciliation after lost responses.
4. Repeat relevant security testing after frontend/API integration, including mobile storage, deep links, session token leakage, TLS, app packages and update signing. Reproduce Strix findings in deterministic tests; remove debug/operator leakage.
5. Run representative long-lived and burst load with real inference and public delivery. Test node failure, backup/PITR restore, rolling release/rollback and audit reconstruction. Record cost and operations ownership/on-call escalation.
6. Prepare reviewed release packages, signed distribution/update paths, privacy/retention processes, runbooks, support and rollback. List government-only integration/operational approvals separately from every unfinished internal task.

### Verify

All functional, capacity, per-language, accessibility, device, security, load, DR and distribution acceptance records cite the tested release revision. Zero unsupported critical guidance/mutations in the adversarial suite. No unresolved critical/high exploitable findings; all remaining risks have an accountable disposition. Missing cases/devices/reviewers mean incomplete readiness.

### Deliver and stop

A complete software-readiness evidence bundle exists; gate S remains provisional until P10 retirement/clean-build proof. The software may be ready to integrate approved feeds, but remains non-operational until P11/P12.

## P10 — Retire obsolete files and verify the final artifact

Prerequisites: P9. Initial status: NOT_STARTED.

### Read and establish

Read cleanup.md, current Git status/import graph/route inventory, Go/mobile manifests, tests and every deployment reference. Recheck for user edits or new consumers since P0. Do not apply the dated deletion table blindly.

### Execute

1. Confirm replacement tests and real clients no longer need legacy endpoints, frontend-owned fixtures or Python application state. Preserve needed source/contract fixtures and model inference runtimes.
2. Delete superseded permanent-relocation modules, retired web clients/tests and unused Azure/Bedrock adapters only after their final consumer is replaced. Remove matching obsolete build/CI/deploy references in the same coherent stage. Do not create a duplicate legacy backup directory; use Git history.
3. Retain protected .txt files and original data/model/user assets. Do not use broad recursive deletion over untracked/ignored paths. Required inference Python is not obsolete merely because the product backend is Go.
4. Ensure release images/archives contain only required application/runtime assets; no demo keys, synthetic live defaults, unused old endpoints or provider secrets. Demo packages belong in an explicitly separate demo distribution/profile.
5. Refresh current setup/runbook/instructions and active trackers. Old completed tasks stay historical rather than active requirements. Stage only this work, preserve unrelated staged changes and commit verified retirement units.
6. Record final API/schema, dependency/SBOM, release/image/app hashes and clean-install steps; link surviving evidence and decisions.

### Verify

Build from a clean checkout and declared dependencies, apply migrations, run Go/DB/model/client integration and representative mobile flows. Search all retained files for deleted imports/routes/paths, inspect package contents and scan artifacts/secrets. Compare requirements/invariants to P9; .txt and unrelated user work must be preserved.

### Deliver and stop

Gate S passes only after P0–P10 software evidence is complete and the final artifact reproduces it. The only remaining release work is explicitly government-dependent integration/validation and operational authorization; otherwise report NOT_READY with exact internal gaps.

## P11 — Authorized government integration and shadow exercises

Prerequisites: Gate S + O05/O07/O08 evidence. Initial status: BLOCKED_EXTERNAL.

### Read and establish

Read source-register.md and signed source/route/capacity agreements, permitted sample schemas, quotas, credentials policy, operational owner and escalation. Current public portals and synthetic fixtures are insufficient. Do not paste secrets into prompts or commits.

### Execute

1. Verify authorization and documented endpoint/auth/coverage/retention contracts. Use scoped ingestion credentials, source allow-lists and upstream quota. Keep clients/model workers unable to access those credentials or arbitrary upstream URLs.
2. Add/adapt only verified source connectors behind the existing canonical contract. If real schemas require a change, version it and rerun affected contract/client/security checks; planning cannot guarantee unknown APIs need zero adaptation.
3. In shadow mode ingest live permitted data for authorized evaluators only. Reconcile source IDs/times/CRS/updates/cancels, closed/full facilities, route ownership/passability/closure freshness and capacity-policy behavior with the responsible authority.
4. Validate publish/sign/revoke and source conflict handling end to end; rehearse source outage, key rotation, upstream quota exhaustion and rollback without delivering unapproved citizen guidance.
5. Have local operators verify actual routes, landmarks, access modes, shelter suitability and temporary-stay processes. Historical demo success cannot substitute for this field evidence.
6. Record named government, operations, privacy, security and language/accessibility sign-off for a controlled pilot. Any failed source stays suspended; do not downgrade validation to enable it.

### Verify

Pass permitted live contract/replay tests, shadow reconciliation, route/closure/capacity field drills, upstream protection checks and signed acceptance of operational policy. No public routing based solely on OSM/model candidate data.

### Deliver and stop

Authorized sources become OPERATIONAL only with recorded evidence; permission to run a controlled citizen pilot is explicit. Otherwise retain BLOCKED_EXTERNAL with the missing item and continue only independent safe work.

## P12 — Controlled launch and operational scale-out

Prerequisites: P11 + explicit release authorization. Initial status: BLOCKED_EXTERNAL.

### Read and establish

Read controlled-pilot approval, geographic/language boundaries, support/on-call roster, release/rollback plan, privacy process, alert/route/capacity SLAs and latest signed build evidence. Production is not authorized by this planning document.

### Execute

1. Start the approved bounded citizen pilot with named local operators and human fallback. Show source/expiry/limitations clearly; monitor task failures, closure latency, capacity conflicts, model fallbacks and source freshness.
2. Use incident-response and rollback procedures when guidance/data or infrastructure fails. Do not promise notification delivery or responder dispatch without actual confirmation.
3. Compare real load, language/UX success, battery/network behavior and inference cost with assumptions. Update capacity plans from measured peaks before opening more jurisdictions.
4. Review drill/incident findings with authorized owners, patch and repeat affected safety/security/device checks on the new revision. Maintain source/route/capacity approval and model/data version traceability.
5. Expand only through jurisdiction-specific authorization, staffing, validated language/route/facility data and infrastructure headroom. One million total users is a service goal, not evidence of tested simultaneous capacity.

### Verify

Pilot metrics, field/operator feedback, approved source freshness, valid routes, capacity reconciliation, security/privacy monitoring, support coverage and rollback exercise meet the signed operating agreement. Release/expansion actions require their explicit authorization.

### Deliver and stop

Operational release is a documented government/product/operations decision with ongoing monitoring and incident ownership. Never equate a passed demo, scanner report or API connection with flawless real-world evacuation.
