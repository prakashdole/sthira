# Coverage matrix — P7 load harness requirements

**Lane:** Worker 11 — `p7-performance-prep`
**Branch:** `codex/p7-performance-prep` from `2fe0fee` (P6 freeze)
**Date:** 2026-09-21

This matrix maps each requirement from the lane prompt
(`plan/p6-p7-parallel-prompts.md` Worker 11) to the file/test that
satisfies it. The intent is to make coverage auditable; each row says
**what is proved at the tested revision**, **what remains pending**, and
the owner.

---

## 1. Workload model

| Requirement | Path under test | Evidence |
| --- | --- | --- |
| 1M means registered/total users, not concurrent | `WORKLOAD.md` §1 | `loadmodel/WORKLOAD.md` (the inputs table) |
| Explicit workload profiles: 10K / 50K / 100K active sessions | `WORKLOAD.md` §2 | Per-tier arrival rates; converted mathematically |
| Read/cache/write/voice rates | `WORKLOAD.md` §2 | Per-tier table; `r_reads=0.1`, `r_writes=0.002`, `r_voice=0.002` |
| Cold cache | `loadmodel/k6/cold_cache.js` | `loadmodel/reports/cold_cache/` |
| One-facility hotspot | `loadmodel/k6/writes_hotspot.js` | `loadmodel/reports/writes_50/` |
| Source outage | `loadmodel/k6/source_outage.js` | `loadmodel/reports/source_outage/` |
| Convert rates mathematically and label assumptions | `WORKLOAD.md` §1, §2 | Inputs labelled as **proposed**, not measured |
| Do not try 100K local VUs blindly | `WORKLOAD.md` §6 | Bounded VUs per scenario; explicit stop thresholds |

## 2. k6 scenarios + synthetic fixtures

| Requirement | Path under test | Evidence |
| --- | --- | --- |
| k6 scenarios against isolated synthetic HTTP fixtures | `loadmodel/k6/*.js` | 7 scenarios; loadtestd binary; in-memory store |
| DB fixtures (synthetic) | `loadmodel/loadfixtures/store.go` | `syntheticStore`; capacity=1 per facility |
| Actual success/failure/capacity assertions | `loadmodel/scripts/run_scenario.sh` + k6 `checks` map | `check 200`, `check queue saturated (expected)`, `check capacity conflict (expected)`, `check 503 (expected)` |
| Stable idempotency rules | `syntheticStore.Reserve(idemKey)` | `loadmodel/loadfixtures/store.go:Reserve` |
| Representative geography skew | `WORKLOAD.md` §1 | `Jurisdiction="KL"` (Kerala Wayanad-style), 10 facilities, 3 route kinds |
| Do not silently benchmark 401/503 as success | k6 checks for HTTP code mismatch | `check 503 (expected)` only fires on `status===503`; otherwise `http_req_failed` increments |
| Separate cached public delivery, authenticated writes, inference | 7 scenarios | `cached_1k` / `cold_cache` / `writes_50` / `voice_20` are four separate runs |

## 3. Metrics collection

| Requirement | Path under test | Evidence |
| --- | --- | --- |
| p50/p95/p99 latency | k6 `summary.json` | `metrics.http_req_duration.{p(50),p(95),p(99)}` per scenario |
| Throughput | k6 `summary.json` | `metrics.http_reqs.rate` per scenario |
| Error categories | k6 `checks` + fixture `/metrics` | `check 200`, `check queue saturated (expected)`, `check capacity conflict (expected)`, `check 503 (expected)`, fixture `voice_ok`, `voice_err`, `writes_ok`, `writes_err`, `queue_rejects` |
| CPU/memory | `INSTRUMENTATION.md` §1, §6 | Proposed; not wired on the harness (the harness targets the fixture, not real CPU/memory on a server host) |
| DB query/lock pressure | `INSTRUMENTATION.md` §4 | Proposed; the harness is in-memory |
| Queue depths | fixture `/metrics` returns `queue_rejects` and tracks `vqNow` | `loadmodel/loadfixtures/server.go:admitVoice` |
| `pprof`/`benchstat` only on controlled private targets | `loadmodel/loadfixtures/bench_test.go` | 8 benchmarks; all in-process; no shared target |
| Run duration / warmup / stop thresholds | `WORKLOAD.md` §6 | Per-scenario duration; warmup; `local-smoke` first |
| Bounded local smoke first | `scripts/run_scenario.sh smoke` | Smoke runs before any other scenario |

## 4. Voice-load harness

| Requirement | Path under test | Evidence |
| --- | --- | --- |
| Target the frozen protocol | `loadmodel/voice/voiceload/voiceload.go` | Speaks `/transcribe`, `/v1/chat/completions`, `/synthesize` against the dummyd workers |
| Dummy-model throughput is not GPU throughput | `INSTRUMENTATION.md` header + report `§4` | Labeled explicitly in WORKLOAD.md §4, BASELINE.md §6 |
| Prepare for later real-worker measurements | `loadfixtures/dummyd` interface + voice harness driver | Both are swappable: replace `StartDummyWorker` with the real ASR/middle/TTS HTTP clients |
| Replica/failure-reserve/cost calculations | **NOT** in this lane | Pending O04 (hardware/budget); the report explicitly does NOT recommend replica counts |
| Missing O04/O09 hardware/budget/traffic approval stays explicit | `BASELINE.md` §6 | Listed in the "what this report does NOT prove" section |

## 5. Anti-speculative guidance

| Requirement | Path under test | Evidence |
| --- | --- | --- |
| Do not optimise based on static guesses | `INSTRUMENTATION.md` §9 | "What the lane does NOT recommend" section explicitly lists non-additions |
| Do not add Redis, brokers, indexes speculatively | `INSTRUMENTATION.md` §9 | Same section; rationale |
| Report measured bottlenecks | `BASELINE.md` §7 | Per-route percentiles; explicit "no bottlenecks on the local test host" claim |
| Specific owner patches | `INSTRUMENTATION.md` §1-§8 | Each proposal has a "Why" and an "Owner" |
| No shared-host heavy run while another agent measures inference | `loadmodel/scripts/run_scenario.sh` | Each scenario binds its own ports; processes are torn down on exit |

## 6. Acceptance matrix (gate-B)

| Acceptance row | Path | Status |
| --- | --- | --- |
| Runnable workload definitions | `loadmodel/k6/*.js`, `loadmodel/scripts/run_scenario.sh` | ✅ shipped |
| Truthful baseline at the tested revision | `loadmodel/reports/BASELINE.md` | ✅ shipped |
| Final surge sizing | n/a | ⏳ pending O04/O09 |
| Gate B | n/a | ⏳ pending integrated P6 + representative hardware |
| Instrumentation proposals to coordinator | `loadmodel/reports/INSTRUMENTATION.md` | ✅ shipped |

---

## 7. Honest non-coverage

These items are NOT proved by this lane and are explicitly NOT claimed:

| Item | Why not |
| --- | --- |
| Real PostgreSQL query latency | The fixture is in-memory; no DB connection |
| Real per-stage voice latency | dummyd workers sleep on a timer, not GPU |
| Replica count | O04 not approved; no `measured_sustainable_rate_per_replica` |
| `cost_per_successful_voice_task` | O04 / O09 not approved; no provider prices or hardware benchmarks |
| Disaster-recovery behaviour | Worker 12 owns this lane; not duplicated here |
| Live security testing | Worker 10 owns this lane |
| 100K VU burst | `WORKLOAD.md` §6 explicitly caps the load generator's per-host capacity |
| Real DB query/lock telemetry | The integration stage wires `pgx.Tracer` (INSTRUMENTATION.md §4); not exercised on the fixture |

The lane finishes the part it can finish without O04/O09 and
explicitly defers the rest to the integration stage.
