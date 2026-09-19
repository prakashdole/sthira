# Sthira backend (Go)

P1 foundation slice: the bounded `/api/v3` boundary. Standard library only; no
external modules, so it builds offline.

## Toolchain

Pinned Go version: **1.27.1** (see `go.mod`). Install a supported Go and verify:

```bash
go version   # must report go1.27.1 or a compatible supported release
```

## Layout

- `cmd/sthira/main.go` — entrypoint with graceful shutdown (SIGINT/SIGTERM).
- `internal/contracts` — frozen `/api/v3` types: envelope, provenance, evidence
  class, freshness, stable error codes, GeoJSON geometry, and the constrained
  middle-model output contract with an independent validator.
- `internal/httpjson` — strict JSON boundary: size/depth limits, duplicate-key
  and unknown-field rejection, trailing-data and malformed-JSON detection.
- `internal/httpserver` — bounded server: method/content-type enforcement,
  timeouts, cancellation, graceful shutdown, separate liveness and readiness.
- `contracts/openapi.yaml` — the frozen contract for the implemented slice.
- `testdata/` — golden fixtures (strict-JSON and middle-model examples).

## Build, check, test

```bash
cd backend
gofmt -l .            # must print nothing
go vet ./...
go test ./...
go build ./cmd/sthira
```

## Run

```bash
cd backend
STHIRA_ADDR=127.0.0.1:8080 go run ./cmd/sthira
```

- `GET /health/live` — liveness only (always 200 while the process runs).
- `GET /health/ready` — readiness; **503 BLOCKED** in P1 because no real
  dependency prober is wired. It never reports a false READY.
- `POST /api/v3/voice/commands` — validates a middle-model proposal against the
  current snapshot and active-context IDs. No write, call or capacity mutation.

## Scope

No frontend, database workflows, live government integrations or model serving
yet. The Python reference application under `src/` is preserved and unchanged.
