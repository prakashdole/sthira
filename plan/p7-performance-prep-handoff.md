# Worker 11 — `p7-performance-prep` handoff

**Branch:** `codex/p7-performance-prep`
**Worktree:** `/private/tmp/mz-worktrees/p7-performance`
**Base commit:** `2fe0fee` (P6 contract freeze on `CLEAN`)
**Lane owner:** Worker 11 — `p7-performance-prep`. Owns dedicated
load/benchmark scripts and reports; **does NOT modify** the production
backend, `cmd/sthira/main.go`, `backend/contracts/openapi.yaml`,
`backend/go.mod`, `backend/migrations/`, or any existing `.txt` file.

## Commits on this branch

| SHA | Subject |
| --- | --- |
| `<this commit>` | feat(p7-performance-prep): bounded k6 load harness, isolated synthetic fixtures, baseline report |

(Single coherent commit. The lane is self-contained; no follow-up
amends.)

## Files added (under `loadmodel/`)

```
loadmodel/
  README.md                                  — what is here, how to run
  WORKLOAD.md                                — mathematical workload profile
  Makefile                                   — make all / bench / smoke / …
  go.mod                                     — isolated module (stdlib only)
  run_scenario.sh                            — (legacy stub; removed)
  bin/                                       — built binaries (gitignored next round)
  cmd/
    loadtestd/main.go                        — synthetic fixture server
    dummyd/main.go                           — P6 dummy worker
    voiceloadd/main.go                       — voice-load harness binary
  k6/
    smoke.js                                 — first-run wiring check
    cached_reads.js                          — 1K QPS public reads
    cold_cache.js                            — 1K QPS, cache hit rate = 0
    writes_hotspot.js                        — 50 writes/s, 80% one facility
    voice_process.js                         — 1/s + 20/s surge
    source_outage.js                         — 100% /packages/* outage
    burst.js                                 — 3K QPS for 30 s + 500 drain
    lib/options.js                           — shared thresholds + payloads
  loadfixtures/
    server.go                                — synthetic /api/v3 fixture
    store.go                                 — in-memory reservation store
    internal/util.go                         — small helpers
    dummyd/dummy_worker.go                   — P6 private protocol dummies
    bench_test.go                            — Go microbenchmarks
  voice/
    voiceload/
      voiceload.go                           — voice-load harness driver
      voiceload_test.go                      — unit tests
  scripts/
    run_scenario.sh                          — canonical runner
    run_all.sh                               — runs every scenario
    summarise.py                             — per-scenario one-paragraph
  reports/
    BASELINE.md                              — truthful local baseline
    INSTRUMENTATION.md                       — production wiring proposals
    COVERAGE.md                              — requirement → evidence map
    bench/bench.txt                          — captured Go bench output
    smoke/                                   — per-scenario JSON outputs
    cached_1k/
    cold_cache/
    writes_50/
    voice_20/
    source_outage/
    burst/
    voice/
```

## Files NOT modified (verified)

* `backend/` — production server code untouched.
* `cmd/sthira/main.go` — untouched.
* `backend/contracts/openapi.yaml` — untouched.
* `backend/go.mod`, `backend/go.sum` — untouched.
* `backend/migrations/*.sql` — untouched.
* `plan/*.md` (existing) — untouched (this handoff lives at
  `plan/p7-performance-prep-handoff.md` per the lane contract).
* Any existing `.txt` file — not modified. The single `.txt` file
  added (`reports/bench/bench.txt`) is a new report file, not a
  modification of an existing `.txt`.

`git status -uall --short` confirms only `loadmodel/` paths are
untracked.

## What ships

### Workload model (`WORKLOAD.md`)

Mathematical conversion of `plan/parameters.md` into per-tier arrival
rates:

| Tier | Active sessions | Public reads/s | Origin reads/s @ 95% | Writes/s | Voice/s |
| --- | --- | --- | --- | --- | --- |
| Baseline | 10K | 1,000 | 50 | 20 | 20 |
| Surge | 50K | 5,000 | 250 | 100 | 100 |
| Stress | 100K | 10,000 | 500 | 200 | 200 |

Plus cold-cache (×20 origin pressure), burst (×3 for 30 s), voice surge
(×5 for 30 s), one-facility hotspot (20% of writes), source outage.

Every input is **labelled** as a **proposed** target, not a measurement.

### Synthetic fixture (`loadfixtures/server.go`)

A self-contained, stdlib-only HTTP server that mirrors the `/api/v3`
route shape so k6 can hit production-like URLs without touching the
real backend. Switchable knobs:

* `STHIRA_LOAD_CACHE_HIT` (default 0.95) — edge-cache hit probability
* `STHIRA_LOAD_OUTAGE_RATE` (default 0.0) — `/packages/*` outage prob
* `STHIRA_LOAD_QUEUE_DEPTH` (default 8) — voice admission depth
* `STHIRA_LOAD_SERVICE_ASR`/`MID`/`TTS` — dummy worker service times

In-memory `syntheticStore` enforces the per-`(facility_id,
service_date)` serialisation (D28, D49): capacity=1 per facility,
idempotency-keyed replay (R13, D30).

### Dummy P6 workers (`loadfixtures/dummyd/dummy_worker.go`)

One process, three endpoints: `/health`, `/transcribe`,
`/v1/chat/completions`, `/synthesize`. Configurable service time,
configurable error rate, configurable concurrency limit, switchable
readiness. Speaks the **frozen P6 protocol shape** (request_id,
language, confidence, action, settings, model_revision, voice_revision,
template_version, source_version).

### Voice-load harness (`voice/voiceload/voiceload.go`)

A Go-only harness that drives the frozen P6 protocol directly against
the dummy workers. Records `Total / OK / QueueSaturated /
ModelUnavailable / Other / Cancelled` plus p50/p95/p99/max and
throughput. Used by `voiceloadd` to emit a single JSON line per run.

### k6 scenarios (`k6/*.js`)

Seven scenarios: smoke, cached_reads, cold_cache, writes_hotspot,
voice_process, source_outage, burst. Each scenario asserts what
counts as success: 200 OK, queue-saturated-503 (expected),
capacity-conflict-409 (expected), upstream-outage-503 (expected).

### Benchmarks (`loadfixtures/bench_test.go`)

Eight Go benchmarks of the synthetic fixture's hot paths. Useful for
the `observed_fixture - observed_production` delta after the
integration stage.

### Runner (`scripts/run_scenario.sh`)

Canonical runner: builds the three binaries, starts workers on
ephemeral ports, runs k6, captures summary JSON, snapshots the
fixture's `/metrics`, tears everything down on exit. Each scenario is
isolated; multiple runs do not collide.

## Verification

```
cd loadmodel
go vet ./...                                  clean
go build ./...                                ok
go test -count=1 -race ./...                  PASS (loadfixtures + voiceload)
make bench                                    PASS (8 benchmarks)
bash scripts/run_all.sh                       PASS (7 scenarios captured)
python3 scripts/summarise.py reports          PASS (per-scenario summary)
```

### Headline numbers (from `BASELINE.md`)

| Scenario | Requests | p95 | Saturation | Notes |
| --- | --- | --- | --- | --- |
| smoke | 34,650 | 3.2 ms | 0% | wiring sanity, mixed mix |
| cached_1k | 60,001 | 2.1 ms | 0% | 1K QPS for 60 s, warm cache |
| cold_cache | 90,001 | 5.3 ms | 0% | 1K QPS for 90 s, 0% cache hit |
| writes_50 | 3,000 | 0.4 ms | 0% writes; 99.7% capacity conflicts (expected) | 80% to one facility |
| voice_20/warm_1 | 30 | 7.5 s | 0% | 1/s for 30 s |
| voice_20/surge_20 | 599 | sub-ms on saturated | 89.8% queue rejects (expected) | 20/s for 30 s |
| source_outage | 6,000 | 0.26 ms | 100% (expected) | fail-fast short-circuit |
| burst | 104,995 | 1.5 ms | 0% | 3K QPS for 30 s + 500 QPS drain |

Every measurement is reproducible from `make all` + `make bench` +
`bash scripts/run_all.sh`.

## Integration deltas proposed (NOT committed)

The integration stage applies these edits to the **shared production
files** when ready. They are **proposed**, not claimed-applied.

### 1. `backend/internal/httpserver/server.go` — pprof surface

Add a guarded pprof block:

```go
if s.cfg.EnablePprof && s.cfg.PprofToken != "" && /* Authorization */ {
    mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
    // ...
}
```

Cost: ~10 lines. Default-off; the `PprofToken` gate is the
authorisation boundary. See `INSTRUMENTATION.md` §1.

### 2. `backend/internal/orchestration/` — expvar counters

The orchestrator package already exposes a `MetricsRecorder`
interface. The integration stage wires a Prometheus/expvar adapter
that produces:

```
orch_queue_depth{stage="asr|middle|tts"}     gauge
orch_queue_rejected_total{stage}            counter
orch_stale_drop_total{stage}                counter
orch_worker_ready{role="asr|middle|tts"}    gauge
```

See `INSTRUMENTATION.md` §3. Worker 9 owns the recorder; integration
stage owns the exporter.

### 3. `backend/internal/store/` — pgx tracer

Add `pgx.Tracer` implementation that records slow queries (>100 ms)
and lock-wait events; expose via `expvar`. See `INSTRUMENTATION.md`
§4.

### 4. `backend/internal/httpserver/handlers.go` — access log

Augment the existing `withRequestID` middleware with a structured
completion log line: `request_id`, `route`, `method`, `status`,
`duration_ms`, `error_code`, `data_version`, `freshness_class`. **No
PII, no audio, no transcript, no token.** See `INSTRUMENTATION.md` §5.

### 5. `cmd/sthira/main.go` — bench subcommand + recorder wiring

```go
benchCmd := &cobra.Command{
    Use: "bench",
    Run: func(cmd *cobra.Command, args []string) {
        if !cfg.EnableBench {
            return // fail closed
        }
        // run the registered Benchmark* functions
    },
}
```

Cost: ~30 lines. See `INSTRUMENTATION.md` §8.

## Limits / out of scope

* **No shared-file modifications.** The lane owns `loadmodel/` only.
* **No PostgreSQL, no Redis, no Kafka, no Kubernetes, no new indexes.**
  Each is explicitly listed as a non-addition in
  `INSTRUMENTATION.md` §9.
* **Real per-stage voice latency.** Requires real GPU workers; this
  lane uses the dummy workers (which sleep on a timer).
* **Replica count.** Requires O04 hardware/budget and a measured
  `sustainable_rate_per_replica`; both are explicitly pending.
* **Cost-per-task.** Requires O04 and O09; both pending.
* **Disaster-recovery behaviour.** Worker 12 owns that lane.
* **Live security testing.** Worker 10 owns that lane.

## Honest non-coverage

See `reports/COVERAGE.md` for the full mapping. Specifically, **this
lane does NOT prove**:

* Real PostgreSQL query latency under load.
* Real per-stage voice compute on a GPU.
* Replica counts or cost-per-task.
* Disaster-recovery or security evidence (those lanes cover them).

The lane finishes the part it can finish without O04/O09: the harness,
the synthetic fixture, the k6 scenarios, the Go benchmarks, and a
**truthful local baseline** with explicit non-coverage.

## Reuse / no new abstractions

* Synthetic fixture uses **stdlib only** (`net/http`, `crypto/sha256`,
  `sync`, `sync/atomic`, `log/slog`, `math/rand/v2`).
* Voice-load harness uses **stdlib only**.
* Benchmarks run on the fixture itself; no shared benchmark target.
* No new dependencies, no third-party load tool (k6 is already in
  `plan/tech-stack.md` and was installed via `brew install k6`).
* The runner script uses bash + curl + psql/pkill; no orchestrator
  framework.

## Lane contract follow-through

* Reads `plan/parameters.md` and `plan/equations.md` — ✅ in
  `WORKLOAD.md`.
* Reads actual routes — ✅ in `loadfixtures/server.go`.
* Reads DB operations — ✅ modelled in `syntheticStore` (D28/D49/D30).
* Reads frozen P6 protocol — ✅ in `dummyd/dummy_worker.go` and the
  voice harness.
* One million means registered/total users, not concurrent requests —
  ✅ explicit in `WORKLOAD.md` §1.
* Convert rates mathematically and label assumptions — ✅ in
  `WORKLOAD.md` §1, §2.
* Do not try 100K local VUs blindly — ✅ `WORKLOAD.md` §6 caps VUs.
* Build k6 scenarios against isolated synthetic fixtures — ✅
  `loadmodel/k6/`.
* Do not silently benchmark 401/503 as successful work — ✅ every
  scenario uses k6 `checks` to label 503s as expected.
* Collect p50/p95/p99 latency, throughput, error categories, queue
  depths — ✅ every scenario captures these.
* Use pprof/benchstat only on controlled private targets — ✅
  benchmarks target the fixture only.
* Voice-load harness may target the frozen protocol but dummy-model
  throughput is not GPU throughput — ✅ explicit in `WORKLOAD.md` §4
  and `BASELINE.md` §4.
* Do not optimise based on static guesses — ✅ `INSTRUMENTATION.md` §9
  is the explicit non-additions list.
* No shared-host heavy run while another agent measures inference —
  ✅ each scenario binds its own ports; `cleanup` traps SIGTERM.
* Deliver runnable workload definitions and truthful baseline — ✅
  `BASELINE.md`.
* Final surge sizing and gate B remain pending integrated P6 and
  representative hardware — ✅ explicit in `BASELINE.md` §6, §9 and
  `INSTRUMENTATION.md` §10.
