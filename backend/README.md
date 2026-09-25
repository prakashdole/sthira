# Sthira backend (Go)

The active product backend. Bounded `/api/v3` HTTP boundary in front of
PostgreSQL/PostGIS storage, voice orchestration (private ASR / middle / TTS
worker protocol over HTTP), and offline package / queue delivery.

## Toolchain

Pinned Go version: **1.27.1** (see `go.mod`). Verify:

```bash
go version   # must report go1.27.1 or a compatible supported release
```

## Layout (current, not P1-only)

- `cmd/sthira/main.go` — entrypoint with graceful shutdown, migration-aware
  readiness prober, persisted context resolver, bounded expiry worker, voice
  orchestrator (when private workers are wired).
- `cmd/scenario-prep/` — `sthira-scenario-prep` CLI (init/validate/report/
  bundle) operating on local scenario workspaces.
- `internal/contracts/` — frozen `/api/v3` types: envelope, provenance,
  evidence class, freshness, stable error codes, GeoJSON geometry, and the
  constrained middle-model output contract with an independent validator.
- `internal/httpjson/` — strict JSON boundary: size/depth limits, duplicate
  key and unknown-field rejection, trailing-data and malformed-JSON
  detection.
- `internal/httpserver/` — bounded server: method/content-type enforcement,
  timeouts, cancellation, graceful shutdown, separate liveness and readiness
  prober seam, voice + voice-p6 handlers, stay handlers, operations handlers
  (fail-closed without a wired IdP), public offline delivery handlers.
- `internal/store/` — durable P3 persistence layer: SQL + transactions +
  optimistic-concurrency guards, sources/artifacts, packages, sessions,
  reservations/inventory, idempotency, audit/outbox, expiry worker,
  migration-aware readiness prober. **Required schema revision is 10**
  (constant `SchemaRevision` in `store/store.go`); a DB at revision < 10
  fails readiness.
- `internal/capfeed/` — bounded CAP 1.2 ingestion (1 MiB parse limit,
  DOCTYPE rejection, dedup, supersession/cancel/quarantine lifecycle,
  ETag/conditional retrieval, singleflight upstream shield).
- `internal/opkg/` — operational-package validation (provenance, version,
  effective/expiry, safe-zone capacity, route↔zone binding, allocation
  policy ordering).
- `internal/sourceact/` — source activation lifecycle
  (DISCOVERED→…→OPERATIONAL/SUSPENDED/RETIRED/QUARANTINED).
- `internal/catalogue/` — scenario catalogue validator outside frontend
  ownership.
- `internal/asrworker/`, `ttsworker/`, `middleworker/` — private HTTP worker
  adapters that call into Python/native ML runtimes.
- `internal/orchestration/` — voice pipeline (snapshot capture → ASR →
  middle → validator → render → TTS), TTS cache binding source / template /
  voice / settings, scoped-context production validator, post-synthesis
  revalidation.
- `internal/offlinepkg/`, `offlineclient/`, `offlinedelivery/`,
  `offlinequeue/`, `offlineresources/` — public/private offline protocol,
  publication storage, durable pending-queue, ranged download, version /
  digest / freshness semantics.
- `internal/scenarioprep/` — scenario workspace loader + validator +
  bundler used by `cmd/scenario-prep`.
- `migrations/` — versioned SQL migrations 0001–0010. Apply in order.
- `contracts/openapi.yaml` — frozen contract for the implemented slice
  (health, voice, voice-p6, stay, operations, offline delivery).
- `testdata/` — golden fixtures.
- `scripts/recovery/` — backup/restore, dependency outage, graceful-drain
  recovery harnesses (auto-derive `SchemaRevision` from the constant).
- `eval/` — synthetic evaluation harness for the voice worker protocol.

## Build, check, test

```bash
cd backend
gofmt -l .            # must print nothing for changed files
go vet ./...
go test ./...         # unit + integration where STHIRA_TEST_ADMIN_DSN is set
go build ./cmd/sthira
go build ./cmd/scenario-prep
```

## Run (foundation mode)

```bash
cd backend
STHIRA_ADDR=127.0.0.1:8080 go run ./cmd/sthira
```

- `GET /health/live` — liveness only (always 200 while the process runs).
- `GET /health/ready` — 503 in foundation mode (no DB); 200 only when
  `STHIRA_DATABASE_DSN` is set AND the DB is reachable AND `schema_migrations`
  reached `SchemaRevision` (10).

## Run (durable mode)

```bash
cd backend
STHIRA_DATABASE_DSN='postgres://localhost/<owned_db>?sslmode=disable' \
  STHIRA_ASR_URL=http://127.0.0.1:9001 \
  STHIRA_MIDDLE_URL=http://127.0.0.1:9002 \
  STHIRA_TTS_URL=http://127.0.0.1:9003 \
  STHIRA_ADDR=127.0.0.1:8080 go run ./cmd/sthira
```

Wires the migration-aware readiness prober, the persisted context resolver,
the bounded expiry worker and (when all three worker URLs are set) the voice
orchestrator. Without worker URLs the voice endpoints fail closed 503.

## Python / ML runtime adapters

`src/sthira_v2/` is retained as the reference Python implementation and as
the runtime for the three selected models: IndicConformer-600M-Multi (ASR),
Sarvam-30B FP8 (middle), Indic Parler-TTS. The Go workers in
`backend/internal/asrworker|ttsworker|middleworker/` call into them over the
private HTTP worker protocol defined by `orchestration.WorkerRequest` /
`WorkerResponse` and the per-stage `WorkerHealth` / `ModelInfo` envelopes.
Real inference is BLOCKED_HARDWARE until approved hardware is wired and
reported (see `plan/open-decisions.md` O03/O04/O11).

## Scope

In-scope: durable store, voice orchestration, offline publication, scenario
preparation, recovery harnesses, scenario fixtures, the bounded migration set.
Out of scope until Gate B: P8 native Android/iPhone clients (selection
deferred), P9 regional drills, live government endpoints (O05/O06/O07/O08/O14).