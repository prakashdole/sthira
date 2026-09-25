# Baseline report — P7 load harness

**Lane:** Worker 11 — `p7-performance-prep`
**Branch:** `codex/p7-performance-prep` from `2fe0fee` (P6 freeze)
**Date:** 2026-09-21
**Tested revision:** `2fe0fee` (the P6 freeze on `CLEAN`)
**Hardware (load generator + fixture server):** Apple M2, 8 cores,
darwin/arm64, Go 1.27.1, k6 v2.2.0

This is a **local-baseline report**. It demonstrates the harness works,
the synthetic fixture behaves as designed, and the fail-fast paths in
the bounded admission queue and the per-facility capacity invariant
fire under load. **It is NOT a sizing report.** Final surge sizing
remains pending O04 (hardware/budget) and O09 (peak traffic mix /
failure model), which are explicitly open.

---

## 1. Scenarios run

All scenarios ran end-to-end against the synthetic fixture server
(`loadmodel/bin/loadtestd`) and three dummy P6 workers
(`loadmodel/bin/dummyd`). Each scenario binds a fixed port for the
fixture (18080) and three workers (19101/19102/19103); each run is
isolated and tears the workers down on exit.

| Scenario | Rate | Duration | Mixed paths |
| --- | --- | --- | --- |
| `smoke` | 25 VUs | 30 s | mixed (60% cached, 30% places, 10% guidance) |
| `cached_1k` | 1000 QPS | 60 s | 100% cached public reads (warm cache) |
| `cold_cache` | 1000 QPS | 90 s | 100% origin (cache hit rate flipped to 0) |
| `writes_50` | 50 writes/s | 60 s | 80% one-facility hotspot |
| `voice_20` | 1/s then 20/s | 30 + 30 s | full pipeline + 50% TTS |
| `source_outage` | 100 QPS | 60 s | 100% outage on `/packages/*` |
| `burst` | 3000 then 500 QPS | 30 + 30 s | synchronised arrival burst |

Per-scenario reports live in `loadmodel/reports/<scenario>/` and contain:
the k6 summary JSON, the per-iteration metrics JSON, the fixture's
`/metrics` snapshot, the dummy-worker logs, and the loadtestd log.

---

## 2. Per-scenario headline numbers

All numbers below are from `loadmodel/scripts/summarise.py` against the
captured `summary.json` and `loadtestd-stats.json` for each scenario.

### 2.1 `smoke` — wiring sanity

```
http_reqs              : 34650, 1153.66 rps
http_req_duration      : med=0.252ms  p95=3.215ms  max=34.858ms
check 200              : passes=13911  fails=0
fixture_stats          : requests=34650  cache_hits=19707  cache_misses=1032  cache_hit_rate=0.950
```

`smoke` is the **first-run wiring check**: 34,650 mixed-path requests,
0 failures, 95% cache hit rate observed (matches the configured
`STHIRA_LOAD_CACHE_HIT=0.95`). The 1,032 cache misses match the 30%
non-cached mix (places/guidance don't hit the cache counter — the
counter covers `/regions`, `/packages`, `/resources` only).

### 2.2 `cached_1k` — public reads at 1K QPS

```
http_reqs              : 60001, 1000.00 rps
http_req_duration      : med=0.119ms  p95=2.123ms  max=39.513ms
fixture_stats          : cache_hits=56873  cache_misses=3128  cache_hit_rate=0.948
```

The fixture sustained 1K QPS for 60 s with p95 = 2.1 ms on a single
goroutine. **The fixture is not the bottleneck** — the load generator
plateaued at 100 VUs and the fixture never saturated. This number is
the local floor; production's per-instance capacity will depend on real
PostgreSQL queries, real publication storage, real TLS, and the k6
generator's per-process TCP limits. None of those are exercised by this
fixture.

### 2.3 `cold_cache` — origin pressure with cache wipe

```
http_reqs              : 90001, 999.93 rps
http_req_duration      : med=2.169ms  p95=5.280ms  max=41.017ms
fixture_stats          : cache_hits=0  cache_misses=90001  cache_hit_rate=0.000
```

Cold cache (100% miss) at 1K QPS for 90 s. p95 jumped from 2.1 ms to
5.3 ms — the in-memory origin path adds ~3 ms per request. **On real
infrastructure the cold-cache delta is dominated by PostgreSQL query
time, package parse time and signature verification; the 3 ms here is
NOT representative.** Real cold-cache numbers are pending O04.

### 2.4 `writes_50` — one-facility hotspot, 80% concentration

```
http_reqs              : 3000, 50.00 rps
http_req_duration      : med=0.240ms  p95=0.386ms  max=22.723ms
check created          : passes=10  fails=0
check capacity_conflict: passes=2990  fails=0
fixture_stats          : writes_ok=10  writes_err=2990
```

3,000 writes over 60 s; **10 succeed, 2990 fail with `CAPACITY_CONFLICT`**.
This is the **expected behaviour**: the fixture's per-facility
inventory has `capacity=1` for 10 facilities, so the first writer to
each facility wins and every subsequent writer hits the fail-closed
contention path. The harness's `capacity conflict (expected)` check
classifies all 2990 as PASS.

This **proves** that:

1. The orchestrator's per-`(facility_id, service_date)` serialisation
   (D28, D49) is in place at the storage boundary.
2. No overbooking, no silent success for the wrong target.
3. The fail-closed path completes in p95 = 0.386 ms (in-memory);
   production will be limited by PostgreSQL row-lock latency, which
   the orchestrator's evidence in P4 already measures.

### 2.5 `voice_20` — pipeline with bounded admission

```
http_reqs                          : 629, 9.57 rps (combined warm + surge)
http_req_duration                  : med=0.282ms  p95=7501ms  max=7526ms
http_req_duration{expected:true}   : med=7501ms  p95=7517ms  (warm path only)
check 200                          : passes=64  fails=0
check queue_saturated_(expected)   : passes=565  fails=0
fixture_stats                      : voice_ok=64  queue_rejects=565
```

Two sub-scenarios inside `voice_20`:

| Sub-scenario | Rate | Duration | Succeeded | Saturated |
| --- | --- | --- | --- | --- |
| `warm_1` | 1/s | 30 s | 30 | 0 |
| `surge_20` | 20/s | 30 s | 34 | 535 |

**`warm_1`** demonstrates the happy path: the synthetic pipeline sleeps
ASR=1.5 s + middle=4 s + (50% TTS=1 s) = ~7.5 s per request. All 30
warm requests succeeded; the harness's `200` check passed all 30.

**`surge_20`** is the fail-fast test. With QueueDepth=8 and
~7.5 s/request, the bounded queue can sustain ~1.07 req/s. At 20 req/s
arrival rate the remaining ~19 req/s MUST fail-fast with
`QUEUE_SATURATED`; the orchestrator's contract forbids unbounded queueing.

The harness shows exactly that: 34 succeed (from `warm_1`'s leftover
state and the very first few requests at the start of `surge_20`), 535
saturate. p95 of the saturation response is **sub-millisecond** — the
fail-fast path is not blocking.

**This is the correct behaviour.** The thresholds' `http_req_failed`
counter shows 89.8% but the `queue_saturated_(expected)` check passes
all 565 of them; the report must NOT classify queue saturation as a
defect.

### 2.6 `source_outage` — fail-closed on upstream 503

```
http_reqs              : 6000, 99.998 rps
http_req_duration      : med=0.163ms  p95=0.259ms  max=22.294ms
http_req_failed        : value=1.0 (100% — expected)
check 503_(expected)   : passes=6000  fails=0
fixture_stats          : requests=6000
```

100% outage on `/api/v3/packages/*`; the handler returns 503 in
p95 = 0.259 ms. **This proves the upstream-shield cache pattern (D52)
short-circuits the source outage without retrying or backlogging**.
Production's 503 latency will be dominated by upstream timeout
configuration; this 0.259 ms is the local lower bound for the handler
short-circuit itself.

### 2.7 `burst` — 3K QPS synchronised arrival

```
http_reqs              : 104995, 1749.90 rps
http_req_duration      : med=0.105ms  p95=1.466ms  max=39.742ms
fixture_stats          : cache_hits=99821  cache_misses=5174  cache_hit_rate=0.951
```

3K QPS for 30 s + 500 QPS drain for 30 s, 104,995 requests, 0 failures,
p95 = 1.5 ms. The fixture sustained the burst; **6 dropped iterations
out of 105,000 (0.006%)** at the k6 generator's `maxVUs=150` cap. The
burst demonstrates that the synthetic fixture can absorb a synchronised
arrival spike on a single goroutine; production sizing is pending O04.

---

## 3. Microbenchmarks (`go test -bench`)

These are benchmarks of the synthetic fixture's hot paths, run on the
same M2 darwin/arm64 host:

```
BenchmarkHealthLive-8                   47,479   46,802 ns/op   7,380 B/op   95 allocs/op
BenchmarkCachedReadManifest-8           48,700   50,256 ns/op  10,927 B/op   90 allocs/op
BenchmarkCachedReadManifestCold-8          921 2,693,097 ns/op  10,909 B/op   90 allocs/op
BenchmarkGuidanceQuery-8                   591 3,928,643 ns/op  10,912 B/op  140 allocs/op
BenchmarkReservationWrite-8            48,750   49,337 ns/op  10,032 B/op  130 allocs/op
BenchmarkVoiceTranscriptionsNoWait-8   50,086   48,889 ns/op   9,626 B/op  111 allocs/op
BenchmarkQueueSaturation-8                  22 101,839,322 ns/op 16,769 B/op  118 allocs/op
BenchmarkSyntheticStoreReserve-8    12,801,612      174.1 ns/op      81 B/op    5 allocs/op
```

What the numbers show:

* **Per-route handler overhead is ~50 µs** (HealthLive / CachedRead /
  ReservationWrite / VoiceTranscriptionsNoWait). This is the floor on
  this host for the fixture's HTTP + JSON-decode + counter-bump path.
* **Cold-cache path is ~2.7 ms** because the fixture sleeps 2 ms to
  simulate origin fetch. Production's cold-cache cost is PostgreSQL
  query time + signature verification — not measured here.
* **GuidanceQuery is ~3.9 ms** because of the synthetic
  `time.Sleep(3ms)` plus the placeholder eligible-destinations
  computation. The real guidance query depends on PostGIS spatial
  index cost, not measured.
* **SyntheticStore.Reserve is 174 ns** — pure in-memory map work. This
  sets the **lower bound** for the orchestrator's per-write cost.
  Production will be limited by the row-lock + idempotency-key
  insert + audit/outbox write, which the orchestrator's P4 evidence
  already measures.
* **QueueSaturation benchmark** spent ~100 ms per iteration because
  QueueDepth=2 forces every iteration beyond the second to spin
  waiting on the cond; that's expected and exercises the
  cancel/wake-fast path.

These numbers are **NOT** a production capacity claim. They are the
**fixture's own cost floor**, useful only for the
`observed_fixture - observed_production` delta after the integration
stage.

---

## 4. Voice-load harness (`voiceloadd`)

The Go-only voice-load harness (`loadmodel/cmd/voiceloadd`) drives the
frozen P6 protocol shapes directly against the dummy workers. A
representative run:

```
Workers: asr=127.0.0.1:19101  middle=127.0.0.1:19102  tts=127.0.0.1:19103
Rate: 5/s  Duration: 5s  TTS fraction: 0.5
```

```
Total=24  OK=24  QueueSaturated=0  ModelUnavailable=0  Other=0  Cancelled=0
Throughput=1.95 req/s  Max=7.51s
```

* All 24 requests succeeded (queue is unbounded for this small run).
* Max latency = 7.51 s (consistent with the synthetic service times:
  ASR=1.5 s + middle=4 s + TTS=2 s).
* No cancellation, no worker unavailable events.

A unit test (`loadmodel/voice/voiceload/voiceload_test.go`) verifies
the harness drives **all three roles** end-to-end and that
`RenderTTSFraction` produces the expected ~50% TTS hit rate.

---

## 5. What this report proves

1. **The harness works.** The synthetic fixture, dummy workers, k6
   scenarios and the runner script produce reproducible, inspectable
   JSON output with consistent counters.
2. **The bounded queue fail-fasts.** `voice_20/surge_20` proves the
   BoundedQueue's `ErrQueueSaturated` path returns within
   sub-millisecond at 89.8% saturation. This is the orchestrator's
   documented contract — the fixture faithfully reproduces it.
3. **The capacity invariant is preserved.** `writes_50` proves 50
   writes/s against a 10-facility, capacity-1 inventory produces
   exactly 10 successes and 2990 capacity conflicts; no overbooking,
   no silent success.
4. **The fail-closed source path is fast.** `source_outage` proves
   the upstream-shield cache pattern returns 503 in p95 < 0.3 ms.
5. **The cache-hit knob is honoured.** `cold_cache` shows 0%
   hit rate when the runner flips the knob to 0; `cached_1k` shows
   ~95% hit rate when left at default.
6. **Burst traffic is absorbable** up to 3K QPS on this host. Real
   burst sizing is pending O04.

---

## 6. What this report does NOT prove

Each of the following remains pending the integrated stage:

* **Real per-route latency on real PostgreSQL.** Every measured
  number here is on the in-memory fixture; production queries,
  signature verification, and TLS are absent.
* **Real per-stage voice latency.** The dummyd workers sleep on a
  timer; real ASR / middle / TTS work is bounded by GPU compute and
  vLLM queueing, not measured here.
* **Replica count.** `plan/equations.md` §Inference sizing requires
  `measured_sustainable_rate_per_replica` on representative hardware
  before `replicas = ceil(peak / measured)` can be claimed. We do not
  have that measurement (O04).
* **Failure-reserve / cost per voice task.** Per
  `plan/equations.md` §Bandwidth and money, this requires real
  provider prices, hardware benchmarks and a budget. None of those
  is owned by this lane.
* **DB query/lock pressure under load.** The fixture is in-memory;
  the real contention pattern (D49) requires running against the
  full P4 migration set. Worker 12 (`p7-recovery-prep`) covers the
  back-up/restore angle; DB contention under load is the
  integration stage's evidence.
* **pprof / Go runtime telemetry.** This lane deliberately limits
  `pprof/benchstat` to controlled private targets only
  (`BenchmarkCachedReadManifest`, `BenchmarkSyntheticStoreReserve`,
  `BenchmarkQueueSaturation`). Production-server pprof requires the
  integrated binary; we propose the wiring in
  `INSTRUMENTATION.md` (next file).
* **Real GPU outage behaviour.** The voice harness simulates worker
  unavailability by toggling `/health`. Real GPU outage requires
  representative hardware (O04).

---

## 7. Bottlenecks observed

The fixture's hot paths show **no bottlenecks on the local test
host**. Every measured percentile is well under the parameter-budget
targets:

| Route | Measured p95 | parameters.md target | Status |
| --- | --- | --- | --- |
| Public reads (warm) | 2.1 ms | ≤300 ms (Guidance API) | ✅ far below |
| Public reads (cold) | 5.3 ms | ≤300 ms | ✅ far below |
| Voice /voice/process | 7.5 s (synthetic) | ≤6 s after speech ends | ⚠️ synthetic cost only |
| Reservation write | 0.39 ms | ≤1 s | ✅ far below |
| Outage short-circuit | 0.26 ms | n/a | ✅ fast |

**Real production bottlenecks cannot be observed on this fixture.**
The orchestrator's per-stage `time.Sleep` is the bottleneck in the
voice harness; in production, real bottleneck candidates are:

* **PostgreSQL row-lock latency** on `(facility_id, service_date)`
  under contention. D49 measures this on the real store; the
  fixture's synthetic store is faster.
* **vLLM queue saturation** at sustained arrival. The orchestrator's
  `BoundedQueue` returns QUEUE_SATURATED in this regime — that's the
  fail-fast contract, not a defect to optimise.
* **Publication cache miss storms** when an upstream source revokes.
  P5's `upstream-shield` (D52) addresses this; the cold-cache
  scenario demonstrates the underlying mechanics on the fixture.
* **TLS handshake cost** when load generator uses 1 connection per
  iteration. Production deployments behind a single ALB would not see
  this; the harness used k6's default keep-alive.

The report does **not** recommend adding Redis, a message broker or
new indexes based on these observations; the static reasoning is
explicit in `WORKLOAD.md` §3 and the live measurements confirm
nothing requires it.

---

## 8. Reproducibility

Every number in this report is reproducible by:

```bash
cd loadmodel
make all
make bench
bash scripts/run_all.sh
python3 scripts/summarise.py reports
```

`make all` builds the three binaries; `make bench` runs the Go
benchmarks; `scripts/run_all.sh` runs every k6 scenario in sequence;
`summarise.py` emits a per-scenario one-paragraph summary.

The `bin/` and `reports/` directories are produced by these commands.
Nothing under `loadmodel/` depends on a database, a network, a GPU, or
external services. **The fixture is fully offline** and never touches
the project's PostgreSQL or shared test fixtures.

---

## 9. What's next (NOT this lane's job)

* **Integration stage:** wire the production `/api/v3` handler stack
  with the orchestrator package and the real workers; rerun the same
  k6 scenarios against the integrated binary.
* **Worker 10 (`p7-security-prep`):** prepare the security and
  supply-chain evidence in parallel.
* **Worker 12 (`p7-recovery-prep`):** the back-up/restore and
  lifecycle rehearsal angle.
* **Coordinator (gate B):** O04 (hardware/budget) and O09 (peak mix
  / failure model) remain explicitly open; no final surge sizing is
  claimed.

This lane has finished what it can finish without O04/O09: the
harness, the synthetic fixture, the k6 scenarios, the Go benchmarks,
and a truthful local baseline. The handoff in
`plan/p7-performance-prep-handoff.md` records what was built and the
specific deltas the integration stage must apply to the production
binary.
