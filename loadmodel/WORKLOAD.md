# Workload profile derivation

**Lane:** Worker 11 — `p7-performance-prep`
**Branch:** `codex/p7-performance-prep` from `2fe0fee` (P6 freeze)
**Date:** 2026-09-21
**Status:** Proposed engineering targets. Numbers below are derived from
`plan/parameters.md` and `plan/equations.md`, **not measured**, and **not a
hardware/budget approval** (O04 / O09 remain open).

---

## 1. Inputs and assumptions

Every input below is **labelled** so the load harness does not silently mix
measured numbers with proposed targets.

| Symbol | Value | Source | Notes |
| --- | --- | --- | --- |
| `N_total` | 1,000,000 total users | `parameters.md` §Accepted | "Registered/total users", not concurrent requests |
| `N_active` | 10,000 / 50,000 / 100,000 simultaneously active sessions | `parameters.md` §Accepted | Treated as engineered session populations, three test tiers |
| `r_reads` | 0.1 reads/s per active user, before cache | `parameters.md` §Accepted | Synthetic, conservative; revisit after P5/P6 measurement |
| `r_writes` | 0.002 writes/s per active user | `parameters.md` §Accepted | Reservation/event/extension/transfer/queue-flush |
| `r_voice` | 0.002 utterances/s per active user | `parameters.md` §Accepted | Voice traffic, with proposed 5× surge tested |
| `H_pub` | 95% public read cache hit (CDN/edge) | `parameters.md` §Accepted | "Public reads only" — does not reduce private writes |
| `H_voice` | 0% (voice is per-call, not cached at origin) | derived | Conservative |
| `T_total` | 8 s total per voice request | `parameters.md` §Voice | Engineering target, not measured |
| `T_asr` | 3 s | `parameters.md` §Voice | ASR service time, target |
| `T_mid` | 6 s | `parameters.md` §Voice | Middle model service time, target |
| `T_tts` | 4 s | `parameters.md` §Voice | TTS service time, target |
| `cold_factor` | 20× baseline for first 5 minutes of cold cache | derived | Models "edge cache wipe" cold-start |
| `burst_window` | 30 s | derived | Synchronised arrival burst (alert push, evacuation broadcast) |
| `burst_factor` | 3× baseline rate during burst | derived | Surge multiple within burst window |
| `voice_surge` | 5× baseline voice rate, max 30 s | `parameters.md` §Voice | Test voice burst |

These values are **proposed**, not measured. The harness converts each
rate into an arrival process; nothing here is silent.

---

## 2. Per-tier arrival rates (mathematical conversion)

Equation references come from `plan/equations.md` §Load example and §Inference
sizing.

### Baseline tier (`N_active = 10,000`)

```text
public_request_rate    = 10,000  × 0.1    = 1,000 req/s before cache
origin_public_reads    = 1,000   × (1-0.95)=    50 req/s  (CDN origin pressure)
write_rate             = 10,000  × 0.002  =    20 writes/s
voice_arrival_rate     = 10,000  × 0.002  =    20 utterances/s
```

### Surge tier (`N_active = 50,000`)

```text
public_request_rate    = 50,000  × 0.1    = 5,000 req/s before cache
origin_public_reads    = 5,000   × 0.05   =   250 req/s
write_rate             = 50,000  × 0.002  =   100 writes/s
voice_arrival_rate     = 50,000  × 0.002  =   100 utterances/s
```

### Stress tier (`N_active = 100,000`)

```text
public_request_rate    = 100,000 × 0.1    = 10,000 req/s before cache
origin_public_reads    = 10,000  × 0.05   =   500 req/s
write_rate             = 100,000 × 0.002  =   200 writes/s
voice_arrival_rate     = 100,000 × 0.002  =   200 utterances/s
```

### Cold-cache failure (no edge cache for 5 minutes)

The harness models cold cache by sending **100%** of reads to origin for a
short window instead of the 95% edge hit:

```text
cold_origin_reads      = public_request_rate × 1.0  (no cache hit)
```

For the **10K tier**, that is `1,000 req/s` to origin for the duration
of the cold window — a 20× jump from the warm 50 req/s.

### Burst window (synchronised arrival)

During a 30 s burst the harness drives `burst_factor × baseline` arrivals:

```text
burst_origin_reads_10k = 1,000 req/s × 3  = 3,000 req/s for 30 s
burst_writes_10k        =    20 × 3       =    60 writes/s for 30 s
burst_voice_10k         =    20 × 5       =   100 utterances/s for 30 s
```

### One-facility hotspot

20% of writes target one of N facilities (10 facilities default →
20% × writes ⇒ ≥2 writes/s at the hotspot at the 10K tier; ≥20 writes/s at
50K, ≥40 writes/s at 100K). This is the contention scenario D28/D49
already worry about.

### Source outage

A source outage flips the package resolver to a stale state and
exercises the failed-revalidation path (D34, D48). The harness toggles
the source mock's response code from 200 to 503 for the duration, and
measures the reservation/lookup failure rate and the latency of the
fail-closed path.

---

## 3. Average inflight and replica sizing (math only)

From `plan/equations.md` §Inference sizing:

```text
average_inflight ≈ arrival_rate × mean_service_seconds
replicas        ≥ ceil(peak_arrival_rate / measured_sustainable_rate_per_replica)
```

| Stage | mean service | arrival (10K burst) | avg inflight | arrival (100K) | avg inflight |
| --- | --- | --- | --- | --- | --- |
| ASR | 1.5 s | 100/s | 150 | 200/s | 300 |
| Middle | 4 s | 100/s | 400 | 200/s | 800 |
| TTS | 2 s | 50/s (TTS skipped when `render=none` or silent action) | 100 | 100/s | 200 |

These are **proposed engineering targets**, not measured. With
`T_asr=3s`, ASR peak inflight at 200/s is 600 — well above the bounded
admission queue (4 inflight / 8 deep from the P6 orchestrator lane).
That **proves that the integrated orchestrator's QUEUE_SATURATED
back-pressure is the correct behaviour at peak load**; saturating the
queue is what D26/D34 expect. The harness reports the saturation rate
and the fail-fast latency.

**Replica counts** are intentionally **not** proposed here. That
requires:

1. A measured `measured_sustainable_rate_per_replica` on representative
   hardware (O04).
2. A measured `failure_reserve` and `headroom` policy (O09).
3. Cost attribution per `plan/equations.md` §Bandwidth and money.

The lane explicitly does **not** recommend adding a Redis, broker or
new index based on these numbers.

---

## 4. Voice harness boundary

Voice-load testing uses the **frozen P6 protocol** (worker `/health`,
`/transcribe`, `/v1/chat/completions`, `/synthesize`) with **dummy**
workers that:

* parse the request and reply in the typed envelope,
* hold the connection for a configurable service time (`T_asr`, `T_mid`,
  `T_tts`),
* optionally return a `MODEL_UNAVAILABLE` or `MODEL_TIMEOUT` envelope.

**Dummy-model throughput is not GPU throughput.** The report labels
every voice result with the worker backend. Production voice sizing
remains pending real workers on real hardware; the harness is for
**end-to-end orchestration behaviour** only (correlation, revalidation,
cancellation, queue saturation, TTS-not-called-for-silent-action).

---

## 5. Stop thresholds and run-time guardrails

The harness **fails the run** when any of the following is observed
during measurement:

* Any 5xx response in the **success path** for routes other than the
  pre-registered outage scenarios.
* Cache miss rate > expected cache miss rate + 5%.
* p95 latency > 2× the per-route target on the relevant tier.
* p99 latency > 5× the per-route target.
* CPU saturation > 90% for >30 s.
* Goroutine count rising monotonically without bound.
* DB connection pool exhaustion for >5 s.
* Stale or unordered voice responses reaching the handler (count > 0).

These thresholds are intentionally **strict**. The harness is a
**floor** for the integrated P6/P7 system; not a ceiling.

---

## 6. Run-time budget

| Tier | VUs (local) | Duration | Warmup | Notes |
| --- | --- | --- | --- | --- |
| smoke (`local-smoke`) | 25 | 30 s | 10 s | First run; catches wiring bugs |
| cached-1k | 50 | 60 s | 10 s | Public reads, 1K QPS target |
| cached-2k | 100 | 60 s | 10 s | Public reads, 2K QPS target |
| write-50 | 50 | 60 s | 10 s | 50 writes/s, one facility |
| voice-20 | 25 | 60 s | 10 s | 20 voice/s (ASR+middle+TTS) |
| cold-cache-1k | 50 | 30 s cold + 60 s warm | 5 s | 1K QPS to origin for 30 s |
| burst-3k | 100 | 30 s burst + 30 s drain | 5 s | 3K QPS, 30 s window |
| hotspot-50 | 50 | 60 s | 10 s | 50 writes/s, 80% to facility `F1` |
| source-outage-30 | 30 | 60 s | 10 s | One source returns 503 |

`local-smoke` must pass before any other scenario. `voice-20` requires
the dummy voice workers to be running.

**Do NOT** run `cached-1k`, `cached-2k`, `burst-3k` at 100K VUs on a
single laptop; the load generator itself becomes the bottleneck.
These are scaled-down *proxies*; real surge sizing is a separate
hardware decision (O04).

---

## 7. Honest baseline claims

What the harness can prove **at the tested revision**:

1. The orchestrator fail-fast path works under load (queue saturation
   returns `QUEUE_SATURATED` within the latency target; no goroutine
   leak).
2. Public read path serves cached responses with bounded latency on the
   synthetic fixture.
3. The write path is serialised at the per-`(facility_id, service_date)`
   level (D28/D49), so no overbooking, even under contention.
4. Voice correlation drops stale or unordered responses (`STALE_SNAPSHOT`,
   drop count > 0 ⇒ success for the in-flight slot only).
5. No 401/503 is silently benchmarked as success.
6. The fixture isolation seam works (no shared DB, no shared fixtures,
   no shared host contention).

What the harness **cannot** prove until integrated P6 + representative
hardware:

* Real `T_asr` / `T_mid` / `T_tts` on a real GPU.
* Real cross-tier replica behaviour.
* Real bandwidth and cost (`cost_per_successful_voice_task`).
* Real disaster-recovery behaviour.

The baseline report at the end of this lane is **explicit** about which
of those are demonstrated and which remain pending.

---

## 8. References

* `plan/parameters.md` §Accepted scope and proposed resource budgets
* `plan/parameters.md` §Voice/resource limits
* `plan/equations.md` §Load example
* `plan/equations.md` §Inference sizing
* `plan/equations.md` §Bandwidth and money
* `plan/architecture.md` §Scale and deployment
* `plan/open-decisions.md` O04, O09 (hardware/budget)
* `plan/decisions.md` D26 (capacity conservation), D28 (lock order),
  D34 (revalidation at commit), D49 (targeted lock observation)
