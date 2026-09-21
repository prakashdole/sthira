# Running the P6 evaluation harness

This document is the launch runbook for the P6 voice-pipeline
evaluation. It describes the exact commands the team runs in
development, in CI, and in a real-inference sweep.

## Layout

```
backend/eval/
  cmd/eval-run          # the runner binary
  corpus/                # case schema + JSON loaders
  provider/              # Deterministic + HTTP provider
  runner/                # the harness driver
  report/                # per-language / per-cohort / per-category report
  cases/synthetic/       # representative SYNTHETIC suite (Git)
  cases/real/            # requirements doc for real-language runs
```

The runner is a single binary, `eval-run`, that loads a suite,
dispatches it through a Provider, and writes a Markdown + JSON
report. No production validator or worker adapter lives here.

## Build

```sh
cd backend/eval
go build ./...
go test ./...
```

All tests are stdlib-only; no third-party dependencies are pulled in.

## Run against the synthetic suite (default / harness validation)

```sh
cd backend/eval
go run ./cmd/eval-run -suite ./cases/synthetic
```

This is what CI runs. It uses the **Deterministic** provider, which
is rule-based and explicit about every Category in the suite. The
expected outcome is `20 pass / 0 fail`. The report covers:

  - per-language: pass / fail / not_evaluated
  - per-cohort: ADULT_SYNTHETIC only (synthetic)
  - per-category: one row per Case.Category
  - aggregate: useful-action / clarify / rejection / FA rates with
    explicit `n`. With 20 samples, most rates are reported as
    `NOT_EVALUATED (n < 5)` — and that is correct. The synthetic
    suite is small on purpose.

```sh
cd backend/eval
go run ./cmd/eval-run -suite ./cases/synthetic -json-out report.json -md-out report.md
```

writes the same report in JSON + Markdown to those file paths.

## Run against a single category / language

The runner accepts comma-list key=value filters:

```sh
cd backend/eval
# only ml-IN, ADULT_SYNTHETIC, CAMERA_MOVE cases
go run ./cmd/eval-run -suite ./cases/synthetic \
    -filter "language=ml-IN,cohort=ADULT_SYNTHETIC,category=CAMERA_MOVE"
```

Supported filter keys: `language`, `dialect`, `region`, `split`,
`category`, `cohort`. Filters AND-combine; absence of a key means
"no filter".

## Dev / Train / Eval splits

The runner ships with `SplitEval` as the default. To exercise the
non-measure surfaces:

```sh
go run ./cmd/eval-run -suite ./cases/synthetic -split DEV
go run ./cmd/eval-run -suite ./cases/synthetic -split TRAIN
```

DEV/TRAIN cases never contribute to a measured rate; the reporter
records their numbers but flags them as `not_evaluated` in
`summary.total`. The corpus schema enforces `Split ∈ {TRAIN, DEV,
EVAL}` at parse time.

## Real-inference run

A real-inference run drives the HTTP provider against the live
worker processes. The runner does not bundle worker adapters — the
orchestrator's `/api/v3` envelope is the contract and the worker
owns adapter logic.

```sh
cd backend/eval
go run ./cmd/eval-run -real \
    -suite ./cases/synthetic \
    -asr    http://localhost:7101 \
    -middle http://localhost:7201 \
    -tts    http://localhost:7301 \
    -bearer "$WORKER_BEARER" \
    -filter "language=ml-IN"
```

Flags:

  - `-real`                opt in to a real run; SYNTHETIC fixtures
                           are filtered out
  - `-asr` `-middle` `-tts`  worker URLs (orchestrator endpoint
                           surface); required
  - `-bearer`              shared bearer token (optional)
  - `-budget-ms`           default per-case latency budget ms
                           (overridden by case-level `latency_budget_ms`)
  - `-warm`                warm-up dispatches per (language, category)
                           (default 1)

When `-real` is set:

  - only `CONSENTED` provenance cases are loaded; synthetic fixtures
    are filtered. Combine with `-real-manifest` (see below) for
    CONSENTED cases.
  - any error class from a stage (5xx, decode failure, timeout) is
    mapped to `NOT_EVALUATED` and reported separately from a
    reconciliation failure.

## Real language manifest (CONSENTED cases)

Real recorded cases live outside the repo. A manifest pairs the case
ID with the on-disk audio path + sha256. Until a manifest exists, the
real-inference run is a no-op (zero measured cases → the reporter
prints `NOT_EVALUATED (n = 0)`).

```sh
go run ./cmd/eval-run -real \
    -suite ./cases/synthetic \
    -real-manifest ./outside-repo/manifest.json \
    -asr    http://localhost:7101 \
    -middle http://localhost:7201 \
    -tts    http://localhost:7301 \
    -bearer "$WORKER_BEARER"
```

The manifest schema lives next to this doc — see
`cases/real/requirements.md`. Until O03 / O11 resolve, manifests are
not expected to exist; the runner prints the empty run honestly
rather than papering over it.

## CI

```sh
cd backend/eval
go vet ./...
gofmt -l .
go test -race -count=1 ./...
go run ./cmd/eval-run -suite ./cases/synthetic
```

CI enforces:

  1. `go vet` clean
  2. `gofmt -l .` empty
  3. `go test -race` passes
  4. the synthetic suite run passes (20 / 0)

Anything beyond the synthetic run (real ASR / middle / TTS workers,
real-language manifests) is **gated** by O03 / O11. CI does not
attempt a real run; that is a release-time check owned by W9 / W11.

## Common pitfalls

  - `load suite: id "...": non-OK outcome ...`: the case's Expected
    carries speech/intents that don't match its outcome. Check the
    schema's outcome-vs-field-compatibility rule in
    `corpus/schema.go`.
  - `runner: provider nil`: the binary was invoked without a working
    provider. For real runs, all three of `-asr -middle -tts` must be
    set; for synthetic runs the default Deterministic provider is
    already wired.
  - `NOT_EVALUATED (n < 5)` in the report: this is correct. The
    reporter refuses to project a fraction from fewer than 5
    samples. Add more data before quoting rates.
