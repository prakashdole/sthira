# Instrumentation proposals

**Lane:** Worker 11 — `p7-performance-prep`
**Branch:** `codex/p7-performance-prep` from `2fe0fee` (P6 freeze)
**Date:** 2026-09-21

This file lists the production-server instrumentation changes the
integration stage should apply. **No shared files are modified by this
lane.** All proposals here are written so the coordinator can apply
them once the integrated binary exists and the O04/O09 gates permit
representative load.

Every proposal says **what to add**, **what problem it solves**, and
**what ownership boundary applies**. The intent is to enable gate-B
evidence without retroactively over-instrumenting the code.

---

## 1. `net/http/pprof` endpoints behind a flag

**Add** to `backend/internal/httpserver/server.go`:

* `/debug/pprof/cmdline`, `/debug/pprof/profile`,
  `/debug/pprof/symbol`, `/debug/pprof/trace`,
  `/debug/pprof/{heap,goroutine,allocs,block,mutex,threadcreate}`.
* All behind `cfg.EnablePprof bool` AND `cfg.PprofToken string`, so
  the default build has no pprof surface.

**Why:** pprof is the **only** way to capture a real on-host
goroutine/heap profile during a 50K-utterance surge. `go tool pprof`
+ `top10 -cum` is the standard triage entry point. The flag-and-token
gate keeps it out of the production surface by default.

**Owner:** coordinator (shared wiring).

**Cost:** ~10 lines + a guarded `mux.HandleFunc` block. No
dependencies.

---

## 2. Per-stage timing histograms in `orchestration.Metrics`

**Already in the orchestrator lane** (per its handoff, the
`orchestration.Metrics` recorder exposes per-stage timing histograms
and counters). **Add** at the integration stage:

* The `NopMetrics` no-op recorder in `loadfixtures` is the harness's
  reference behaviour; production wires
  `orchestration.PrometheusRecorder` (or the equivalent OTel sink).
* Recorder writes MUST go through a low-cardinality enum (the
  orchestrator's `MetricsRecorder` interface enforces this); the
  integration stage adds the OTel adapter.

**Why:** Without per-stage timing, voice p95 = 7.5 s is a single
number with no diagnosis. With the histogram, the report can say
"middle p95 = 4.2 s, ASR p95 = 1.6 s, TTS p95 = 2.1 s" and the
replica-sizing decision becomes actionable.

**Owner:** integration stage; recorder package goes under
`backend/internal/telemetry/recorder/` (new).

---

## 3. `expvar` counters for the orchestrator's queue & drop stats

**Add** to the orchestrator (Worker 9 owns the orchestrator's
internals):

* `orch_queue_depth{stage=asr|middle|tts}` gauge
* `orch_queue_rejected_total{stage}` counter
* `orch_stale_drop_total{stage}` counter
* `orch_worker_ready{role}` gauge (0/1)

**Why:** The harness measured `queue_rejects=565` and `voice_ok=64`
on the fixture; production needs the same counters on the real
binary. Exposing them via `expvar` keeps the existing
`/metrics`-style surface intact and lets the integration stage feed
them to Prometheus without a new exporter.

**Owner:** Worker 9 (orchestrator package) — already exposes the
`MetricsRecorder` interface; integration stage implements the
expvar/OTel adapter.

---

## 4. PostgreSQL query/lock telemetry

**Add** to `backend/internal/store/` via the integration stage:

* Wire `pgx` `Tracer` to log slow queries (>100 ms) and lock-wait
  events.
* Add a `pg_stat_activity` sampler that runs every 5 s and exposes:
  `db_active_conns`, `db_idle_conns`, `db_waiting_lock`,
  `db_longest_query_ms`.

**Why:** The fixture is in-memory; the real contention story lives in
PostgreSQL. D49 already measures targeted lock contention on the
P4 dataset; the integration stage must wire the runtime telemetry so
the load harness's `db_lock_pressure` metric has a producer.

**Owner:** coordinator + P4 lane. P4's evidence already covers the
schema; P7 needs the runtime exporter.

**Cost:** pgx ships a `pgx.Tracer` interface; the integration stage
adds ~50 lines.

---

## 5. HTTP server access log with bounded fields

**Already** wired in `backend/internal/httpserver/server.go` via the
`writeData`/`writeError` envelopes. **Augment** the integration stage:

* Add a request-completion log line with:
  `request_id`, `route`, `method`, `status`, `duration_ms`,
  `error_code`, `data_version`, `freshness_class`.
* **Never** include: audio bytes, transcripts, locations, bearer
  tokens, capacity values, idempotency keys, or session IDs.

**Why:** The harness's k6 `metrics.json` already records per-iteration
status/duration; production needs the same shape on the server side
to correlate with database telemetry. The strict allowlist keeps
R05 / R19 / R26 / R32 happy.

**Owner:** integration stage; the per-route handler wrappers
(`withRequestID`, `withSession`, `withOperator`) already centralise
the boundary.

---

## 6. cgroup memory/CPU telemetry on Linux

**Add** to the runtime once deployment moves off darwin/arm64:

* On Linux, read `cgroup v2` `memory.current`,
  `memory.high`, `cpu.stat` from inside the process every 10 s.
* Expose via the same `Metrics` recorder.

**Why:** Local darwin doesn't have cgroups; the load harness runs on
darwin/arm64, so this can't be tested here. Once the integrated
binary deploys to a Linux host, the harness's CPU/memory sections of
the report need a producer. This is a small file under
`backend/internal/telemetry/host/` and is gated on
`runtime.GOOS == "linux"`.

**Owner:** integration stage; gated on actual deployment.

---

## 7. Voice-stage dead-letter sampling

**Already** in the orchestrator's `PipelineError` envelope; **add** at
the integration stage:

* On any voice failure (`State != OK`), capture the typed
  `PipelineError.Failures` (slice of `{stage, code, reason, retryable}`)
  and increment a `voice_failures_total{stage,code}` counter.
* **Never** capture the request's transcript, audio bytes, location,
  or session ID.

**Why:** This is the only way to answer "what fraction of voice
failures are at the ASR stage vs the middle stage vs the TTS stage?"
without hand-picking from logs. The harness's `voice_err` counter is
the local floor; production needs the same shape with the real
breakdown.

**Owner:** integration stage; the orchestrator already exposes
`MetricsRecorder.ObserveStageFailure(stage, code)`.

---

## 8. Bench dataset lifecycle

**Add** a `bench/` subcommand to `cmd/sthira/`:

* `go run ./cmd/sthira bench -duration 30s -cpus 4`
* Runs the existing `Benchmark*` functions in
  `backend/internal/...` against an in-process server, capturing
  benchstat-compatible output to a CSV.
* **Gated** behind `cfg.EnableBench` and never enabled by default.

**Why:** `benchstat` needs multiple runs to be meaningful; the
harness's `loadmodel/loadfixtures/bench_test.go` is the local floor.
Production-server benchstat is a separate concern (different binary,
different routes) and lives behind the integration stage.

**Owner:** integration stage.

---

## 9. What the lane does NOT recommend

These are explicit non-additions, recorded so the integration stage
does not relitigate them:

* **No Redis.** No measured cross-instance shared state.
* **No Kafka / NATS / RabbitMQ.** No measured queue requirement
  beyond PostgreSQL outbox + bounded admission queues.
* **No new PostgreSQL indexes** without `EXPLAIN ANALYZE` evidence.
  P4's existing indexes are sufficient for the measured routes; the
  load harness doesn't reveal a missing-index bottleneck because it
  doesn't run real queries.
* **No Kubernetes / Helm / Argo / Istio.** D12 retains the
  smallest operational architecture; expand only on evidence.
* **No app-level response caching.** The harness already models
  edge/CDN caching at the synthetic layer; adding in-process caching
  duplicates state without a measured need.

---

## 10. Acceptance for gate-B instrumentation

The instrumentation set above is **necessary but not sufficient** for
the integrated binary's load evidence. Specifically, gate-B requires:

1. pprof capture under 1K-utterance load.
2. `orch_*` counters feeding a Prometheus sink during a 30-minute
   voice surge.
3. `pg_stat_activity` lock-wait telemetry during a 50-write/s
   hotspot run.
4. HTTP access-log volume compatible with the documented retention
   policy (R32: bounded fields, no PII).

These are **integration-stage work**, owned by the coordinator.
This lane provides the harness, the synthetic fixture, and the
baseline; the coordinator decides when and how to apply these
proposals against the integrated binary under O04/O09's evidence.
