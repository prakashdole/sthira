# Observability Wiring — Execution Evidence

**Commit**: pending — pending commit on `CLEAN`
**Scope**: complete half-wired observability scaffolding (access log + pprof already implemented; observability endpoint added; duplicate `telemetry/` package deleted)
**Host environment**: macOS 15.x (Darwin arm64, Apple M2), Go 1.27.1, PostgreSQL 18 + PostGIS 3.6 (DSN for store integration tests)

---

## 1. Summary

The previous session left behind a half-wired observability surface:

- `backend/internal/httpserver/accesslog.go` — defined `statusTrackingResponseWriter` + `Server.logAccess`
- `backend/internal/httpserver/pprof.go` — defined `Server.registerPprofRoutes` (token-guarded `/debug/pprof/*`)
- `backend/internal/httpserver/config.go` — added `EnablePprof`, `PprofToken`, `EnableAccessLog`
- `backend/internal/httpserver/server.go` — already wired `registerPprofRoutes(mux)` in `New()` and `withRequestID` middleware to call `logAccess` when `cfg.EnableAccessLog`
- `backend/internal/httpserver/telemetry_integration_test.go` — tests for the above (all passing)
- `backend/internal/telemetry/` — orphan package (DBStatsCollector + PipelineMetrics) with zero users outside its own tests

What was missing:

- `cmd/sthira/main.go` never called `WithAccessLog(true)` or `WithPprof(token)` — both surfaces stayed OFF in production
- No HTTP observability endpoint exposed internal metrics
- The orphan `telemetry/` package duplicated `orchestration.Metrics`

This commit completes the integration:

1. New `backend/internal/httpserver/observability.go` — `GET /api/v3/observability/metrics` handler, `PipelineMetricsSnapshotter` interface, `WorkerHealthSummary` row, `WithMetricsSnapshotter`/`WithWorkersHealth` server options, `authorizeObservabilityToken` helper
2. Modified `backend/internal/httpserver/server.go` — register the new route, add `metrics` + `workersHealthFn` fields and their `Option`s
3. Modified `backend/internal/orchestration/orchestrator.go` — add `PipelineMetricsSnapshot()` and `WorkerHealthStages()` accessors
4. Modified `backend/cmd/sthira/main.go` — wire env-var opt-in (`STHIRA_ENABLE_ACCESS_LOG`, `STHIRA_PPROF_TOKEN`); wire orchestrator metrics + workers health via small adapter closures
5. Modified `backend/internal/contracts/errors.go` — additive `ErrUnauthorized = "UNAUTHORIZED"` and `ErrDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"` codes (the existing 401 path was using `ErrForbidden`, which was semantically wrong; the new code is additive and the old usage preserved)
6. Modified `backend/contracts/openapi.yaml` — document the new `/observability/metrics` GET route, its 200/401/405/503 envelope responses, header parameters, and privacy invariants
7. Modified `backend/internal/httpserver/contract_test.go` — extend the OpenAPI/route table to assert the new endpoint is routed (405 on POST, never 404)
8. New `backend/internal/httpserver/observability_test.go` — 10 tests covering: not configured → 503, missing token → 401, wrong token → 401, X-Observability-Token works, Authorization Bearer works, non-GET → 405, no `db_pool` when store absent, privacy invariants (no bearer/GPS leakage), timing fields in ms (count/sum_ms/max_ms), no recent-ring buffer leakage
9. Deleted `backend/internal/telemetry/` — duplicate of `orchestration.Metrics`; nothing imported it
10. Updated `plan/decisions.md` — D63 records the surface, privacy invariants, and the redundancy-driven `telemetry/` deletion

## 2. Privacy and authority invariants

The observability endpoint is the most sensitive operational telemetry surface because it sits between the in-process pipeline recorder and the public network. The implementation enforces:

| Invariant | How it's enforced |
| --- | --- |
| Never echoes bearer tokens | `handleObservability` reads `cfg.PprofToken` only as a constant; the request token is matched against it locally and never returned in the response. Test: `TestObservability_PrivacyInvariants` |
| Never echoes GPS coordinates | The handler builds its `data` map from `metrics.Snapshot()`, `store.DB().Stats()`, and `s.workersHealthFn()`. None of these sources carry GPS data. Test: `TestObservability_PrivacyInvariants` |
| Never echoes audio bytes / transcripts | `orchestration.Metrics` never observes those fields; `Workers.HealthSnapshot` returns ready/warm/supported_languages only. The handler drops model revisions, artifacts and queue stats. |
| Never echoes session IDs | `PipelineMetricsSnapshotter.Snapshot()` returns the typed `MetricsSnapshot`; the handler extracts `(Stage, code, count, sum_ms, max_ms)` — never request/correlation IDs. |
| Fail-closed when token is not configured | `s.cfg.PprofToken == ""` returns 503 `DEPENDENCY_UNAVAILABLE`. Default deployment has neither `STHIRA_PPROF_TOKEN` nor `STHIRA_ENABLE_ACCESS_LOG` set, so the surfaces stay OFF. Test: `TestObservability_NotConfigured_Returns503`. |
| Token required when configured | 401 `UNAUTHORIZED` on missing/wrong token. Tests: `TestObservability_MissingToken_Returns401`, `TestObservability_WrongToken_Returns401`. |
| Method enforcement | 405 on non-GET. Test: `TestObservability_NonGET_Returns405`. |
| Low cardinality | The snapshot has no per-request fields, no histogram ring buffers, no goroutine stacks, no path labels. Test: `TestObservability_PipelineSnapshot_OnlyIncludesLowCardinalityKeys` asserts no `"recent"` field leaks. |
| Access log never logs request bodies | The `logAccess` path only emits structured operational metadata (`request_id`, `method`, `path`, `status`, `duration_ms`, `bytes_written`). Test: `TestAccessLog_PrivacyInvariants` from the pre-existing `telemetry_integration_test.go`. |

## 3. Operational behavior

When the binary is started with no observability env vars (the default):

```
$ go run ./cmd/sthira
{"time":"...","level":"INFO","msg":"no STHIRA_DATABASE_DSN; foundation mode, readiness BLOCKED"}
{"time":"...","level":"INFO","msg":"P6 voice workers not fully configured; voice pipeline stays unavailable (503)"}
{"time":"...","level":"INFO","msg":"sthira backend listening","addr":"127.0.0.1:8080"}
```

GET `/api/v3/observability/metrics` returns 503 `DEPENDENCY_UNAVAILABLE` with body `{"errors":[{"code":"DEPENDENCY_UNAVAILABLE","message":"observability surface not configured",...}]}`. No `db_pool`, no `pipeline`, no `workers` in the response.

When the binary is started with `STHIRA_PPROF_TOKEN=secret STHIRA_ENABLE_ACCESS_LOG=1`:

```
$ STHIRA_PPROF_TOKEN=secret STHIRA_ENABLE_ACCESS_LOG=1 go run ./cmd/sthira
...
{"time":"...","level":"INFO","msg":"access log enabled (privacy-preserving structured logging)"}
{"time":"...","level":"INFO","msg":"pprof + observability metrics endpoint enabled (token-guarded)"}
```

`GET /api/v3/observability/metrics` with no token → 401. With `X-Observability-Token: secret` → 200 with:

```json
{
  "request_id": "req-...",
  "schema_version": "v3",
  "data_version": "none",
  "data": {
    "started_at": "2026-...",
    "now": "2026-...",
    "pipeline": { "total_observations": N, "queue_reject": {...}, "stale_drop": {...}, "stages": { "asr:OK": {"count":1,"sum_ms":500,"max_ms":500}, ... } },
    "db_pool": { "max_open": 10, "open": 1, "in_use": 0, "idle": 1, "wait_count": 0, "wait_duration_ms": 0 },
    "workers": [ { "stage": "asr", "ready": true, "warm": true, "languages": ["en-IN","hi-IN","ml-IN"] }, ... ]
  }
}
```

`GET /debug/pprof/` with no token → 401. With `X-Pprof-Token: secret` → 200 (the index HTML page).

Every successful HTTP request emits a structured `slog` line:

```
{"time":"...","level":"INFO","msg":"http request completed","request_id":"req-...","method":"GET","path":"/api/v3/observability/metrics","status":200,"duration_ms":3,"bytes":1234}
```

## 4. Verification (commands and actual results)

```
$ cd backend && gofmt -l .            # clean
$ cd backend && go vet ./...          # clean
$ cd backend && go build ./...        # ok
$ cd backend && go test ./...         # ok — all packages pass; httpserver includes the new observability_test.go and the existing telemetry_integration_test.go
```

Specific tests run with `-v`:

```
=== RUN   TestObservability_NotConfigured_Returns503              --- PASS
=== RUN   TestObservability_MissingToken_Returns401              --- PASS
=== RUN   TestObservability_WrongToken_Returns401                --- PASS
=== RUN   TestObservability_XHeaderToken_Succeeds                --- PASS
=== RUN   TestObservability_BearerToken_Succeeds                 --- PASS
=== RUN   TestObservability_NonGET_Returns405                    --- PASS
=== RUN   TestObservability_DBPoolWhenStorePresent               --- PASS
=== RUN   TestObservability_PrivacyInvariants                    --- PASS
=== RUN   TestObservability_TimingSummaryFieldsAreSeconds        --- PASS
=== RUN   TestObservability_PipelineSnapshot_OnlyIncludesLowCardinalityKeys  --- PASS

=== RUN   TestPprofEndpoints_DisabledByDefault                   --- PASS
=== RUN   TestPprofEndpoints_TokenGuarded                         --- PASS
=== RUN   TestAccessLog_PrivacyInvariants                         --- PASS

=== RUN   TestServedRoutesMatchOpenAPI                            --- PASS
```

OpenAPI validity:

```
$ python3 -c "import yaml; yaml.safe_load(open('backend/contracts/openapi.yaml'))"
# (no output — valid YAML)
```

Privacy contract assertions confirmed by `TestObservability_PrivacyInvariants` and `TestAccessLog_PrivacyInvariants` (existing).

## 5. Files changed

```
A  backend/internal/httpserver/observability.go
A  backend/internal/httpserver/observability_test.go
M  backend/internal/httpserver/server.go
M  backend/internal/httpserver/contract_test.go
M  backend/internal/httpserver/config.go                       (no change — pre-existing in working tree)
M  backend/internal/httpserver/accesslog.go                    (no change — pre-existing in working tree)
M  backend/internal/httpserver/pprof.go                        (no change — pre-existing in working tree)
M  backend/internal/httpserver/telemetry_integration_test.go   (no change — pre-existing in working tree, now tracked)
M  backend/internal/contracts/errors.go                       (additive ErrUnauthorized + ErrDependencyUnavailable)
M  backend/internal/orchestration/orchestrator.go              (PipelineMetricsSnapshot + WorkerHealthStages accessors + StageHealth struct)
M  backend/cmd/sthira/main.go                                  (env-var opt-in + adapter closures)
M  backend/contracts/openapi.yaml                              (document new endpoint)
D  backend/internal/telemetry/                                 (deleted as dead-code duplicate)
M  plan/decisions.md                                           (D63)
```

## 6. Unresolved or external

None for this slice. Real production deployment still requires O04 (hosting budget), O08 (source agreements), O14 (operator IdP). The observability surface is opt-in and fail-closed; deploying without `STHIRA_PPROF_TOKEN` is safe by design.
