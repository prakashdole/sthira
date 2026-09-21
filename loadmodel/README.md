# P7 load harness

Worker 11 (`p7-performance-prep`) deliverable. Isolated, self-contained
load + benchmark harness for the Sthira `/api/v3` boundary and the frozen
P6 voice pipeline.

This directory **does not depend on the production server**, the project
PostgreSQL, or any external service. It is an in-memory synthetic
fixture plus a Go-only voice-load harness plus k6 scenarios and Go
microbenchmarks.

## What is here

| Path | What |
| --- | --- |
| `WORKLOAD.md` | The mathematical conversion of `plan/parameters.md` rates into per-tier arrival rates. Every input is labelled. |
| `loadfixtures/` | Stdlib-only synthetic fixture server (loadtestd binary) and the P6 dummy worker (dummyd binary). |
| `voice/voiceload/` | Go-only voice-load harness that drives the frozen P6 protocol against the dummy workers. |
| `cmd/loadtestd/`, `cmd/dummyd/`, `cmd/voiceloadd/` | The three binaries. |
| `k6/*.js` | Seven k6 scenarios: smoke, cached_reads, cold_cache, writes_hotspot, voice_process, source_outage, burst. |
| `scripts/run_scenario.sh` | The canonical runner: builds, starts workers, runs k6, captures reports. |
| `scripts/run_all.sh` | Runs every scenario in sequence. |
| `scripts/summarise.py` | Emits a per-scenario one-paragraph summary from the captured JSON. |
| `reports/BASELINE.md` | The truthful local baseline at the tested revision. |
| `reports/INSTRUMENTATION.md` | Production-server instrumentation proposals for the coordinator. |
| `reports/COVERAGE.md` | Mapping of every requirement from `plan/p6-p7-parallel-prompts.md` Worker 11 to the file/test that satisfies it. |
| `reports/<scenario>/` | Per-scenario JSON output: `summary.json`, `metrics.json`, `loadtestd-stats.json`, and worker logs. |
| `Makefile` | `make all`, `make bench`, `make smoke`, `make voice`, … |

## How to run

```bash
# build everything
make all

# run the Go microbenchmarks
make bench

# run a single k6 scenario
bash scripts/run_scenario.sh smoke
bash scripts/run_scenario.sh cached_1k
bash scripts/run_scenario.sh voice_20
bash scripts/run_scenario.sh writes_50
bash scripts/run_scenario.sh cold_cache
bash scripts/run_scenario.sh source_outage
bash scripts/run_scenario.sh burst

# run every scenario
bash scripts/run_all.sh

# emit a summary of the captured JSONs
python3 scripts/summarise.py reports
```

## What this proves

* The harness works end-to-end: synthetic fixture + dummy workers + k6
  produce reproducible, inspectable JSON output.
* The orchestrator's bounded admission queue fail-fasts correctly under
  surge (`voice_20/surge_20`: 89.8% saturation, sub-ms response).
* The per-`(facility_id, service_date)` capacity invariant is preserved
  (`writes_50`: 10 succeed, 2990 conflict; no overbooking).
* The upstream-shield cache pattern returns 503 in p95 < 0.3 ms.
* The cache-hit knob is honoured (cold_cache vs cached_1k).
* Burst traffic (3K QPS, 30 s) is absorbable on this host.

## What this does NOT prove

Real production-server latency, real PostgreSQL query/lock cost, real
per-stage voice compute, replica counts, cost-per-task, GPU outage
behaviour. All of those require the integrated binary under O04/O09.

See `reports/BASELINE.md` for the full truthful baseline and
`reports/INSTRUMENTATION.md` for the production-server wiring proposals
the integration stage should apply.

## Files NOT modified

* `backend/` (production server) — untouched.
* `cmd/sthira/main.go` — untouched.
* `backend/contracts/openapi.yaml` — untouched.
* `backend/go.mod` — untouched.
* `backend/migrations/*.sql` — untouched.
* Any existing `.txt` file — not modified.

The lane owns `loadmodel/` exclusively and a single handoff Markdown
file (`plan/p7-performance-prep-handoff.md`) at the worktree root.
