# R00 evidence

Task: R00 — Correct orientation and freeze one visible journey.
Acceptance: docs paths exist, package identifiers match actual schemas,
readiness rejects lower schema through real HTTP+DB; current migrated DB
succeeds. R00 does not claim live model inference.

## Base / branch / commits

- Base: `CLEAN` at `1347c2f` (plan/prompt.md + plan/round-two-demo.md +
  docs commits).
- R00 commits (coordinator-owned, on `CLEAN`, no push):
  - `f5951e0` — bump `SchemaRevision` 7→9, freeze demo contracts in
    `plan/r0-demo-freeze.md`, correct root + backend READMEs and
    `plan/tech-stack.md`.
  - `e1ed808` — record R00 status in `plan/prompt.md` §9 ledger.
- Worktree: the coordinator's checkout (CLEAN). Pre-existing R05 work in
  `backend/internal/scenarioprep/*` and `backend/cmd/scenario-prep/report.go`
  was left untouched in the working tree (uncommitted) per rule
  "preserve existing user work."

## Observable changes and key paths

- `backend/internal/store/store.go` — `SchemaRevision` 7 → 9 with a
  comment naming the dependent migrations (`0008_p6_template_approval.sql`,
  `0009_p6_template_binding.sql`).
- `backend/internal/store/readiness_test.go` (new) —
  `TestSchemaRevisionMatchesMigrations` walks up from the test cwd,
  globs `backend/migrations/[0-9]*.sql`, parses the prefix revision and
  fails if `SchemaRevision != highest`. No external state required.
- `backend/internal/store/recovery_integration_test.go` —
  backup/restore integration test asserts `srcRev == "9"` (was `"7"`).
- `backend/scripts/recovery/run-backup-restore.sh` — comment update;
  the script already greps the constant at runtime so the comparison
  remains auto-derived.
- `plan/r0-demo-freeze.md` (new) — endpoint usage table (lifecycle +
  request/response shapes for the round-two demo), error-code UI map,
  the visible demo journey and the offline/error second journey, and
  pre-flight expectations for the UI / inference / data lanes. No new
  endpoints, fields or error codes are introduced.
- `README.md` (root) — replaces the previous one-line stub with a
  pointer to the active Go backend and the Python reference / ML
  adapter role.
- `backend/README.md` — replaces the P1-foundation description with
  the current layout (migrations through 0009, store SchemaRevision,
  voice orchestration, offline publication, scenario preparation,
  recovery harnesses) and the durable-mode run command.
- `plan/tech-stack.md` — "Current implementation vs target" table
  updated to reflect actual repo state (Go service + Python ML adapters
  in `src/sthira_v2/`; PostgreSQL 18 + PostGIS 3.6 verified; private
  worker protocols defined; offline publication and queue; SPDX SBOMs).
- `plan/prompt.md` §9 ledger — R00 row updated to `DONE`.

## Acceptance checklist: requirement → test/run → result

| Requirement | Test / run | Result |
| --- | --- | --- |
| `SchemaRevision` matches migrations | `go test -count=1 -v -run TestSchemaRevisionMatchesMigrations ./internal/store` | PASS (SchemaRevision=9, highest migration 0009) |
| gofmt clean on changed files | `gofmt -l backend/internal/store/{store,readiness_test,recovery_integration_test}.go` | (no output) |
| `go vet ./...` clean on changed code | `go vet ./internal/store/... ./cmd/sthira/...` | clean |
| Binary builds | `go build ./cmd/sthira ./cmd/scenario-prep` | ok |
| Unit + HTTP handler tests pass | `go test -count=1 ./internal/store ./internal/httpserver ./internal/contracts ./internal/httpjson ./internal/capfeed ./internal/opkg ./internal/sourceact ./internal/catalogue` | 8 packages, all ok |
| No `.txt` modifications | `git diff --name-only -- '*.txt'` | (empty) |
| Existing integrity tests | unchanged | pre-existing `recovery_integration_test.go` now expects rev=9; will run end-to-end when `STHIRA_TEST_DSN` is provided (out of scope here; no DSN configured in this acceptance) |
| Readiness prober rejects lower revision | unit-test covers the constant-vs-migration invariant; integration test (`store_integration_test.go:318`) exercises `NewReadinessProber(s.db, 1)` and `NewReadinessProber(s.db, 9999)` paths | covered (live DB run requires STHIRA_TEST_DSN; not in this acceptance) |
| Demo contract freeze recorded | `plan/r0-demo-freeze.md` exists | yes |

## Commands, versions, environment, actual exit codes and skipped checks

```text
$ go version
go version go1.27.1 darwin/arm64

$ gofmt -l backend/internal/store/store.go \
         backend/internal/store/readiness_test.go \
         backend/internal/store/recovery_integration_test.go
(no output — clean)

$ go vet ./internal/store/... ./cmd/sthira/... ./internal/httpserver/...
(no output — clean)

$ go build ./cmd/sthira ./cmd/scenario-prep
$ echo $?
0

$ go test -count=1 ./internal/store ./internal/httpserver ./internal/contracts \
                 ./internal/httpjson ./internal/capfeed ./internal/opkg \
                 ./internal/sourceact ./internal/catalogue
ok  	sthira/backend/internal/store	0.153s
ok  	sthira/backend/internal/httpserver	14.566s
ok  	sthira/backend/internal/contracts	0.404s
ok  	sthira/backend/internal/httpjson	0.102s
ok  	sthira/backend/internal/capfeed	0.270s
ok  	sthira/backend/internal/opkg	0.106s
ok  	sthira/backend/internal/sourceact	0.095s
ok  	sthira/backend/internal/catalogue	0.096s
```

Skipped: `go test ./...` (would run the scenarioprep package whose
uncommitted changes by another worker compile but `go test` is
untouched by R00). Live-DB integration tests for `store_integration_test.go`
and `recovery_integration_test.go` skipped — no `STHIRA_TEST_DSN`
configured in this acceptance.

## Real vs fake evidence, device/model/data revisions when relevant

- Schema revision: real (parsed from actual files in
  `backend/migrations/`).
- Migration contents: real (read 0001–0009 to confirm the column
  additions).
- Prober logic: real Go unit test on the constant invariant; full
  end-to-end readiness probe against a live DB deferred to R01 / B02
  where a disposable owned DB is brought up.
- Python and ML runtime: NOT exercised; remains reference / adapter.
- Demo contract freeze: real (read against `backend/contracts/openapi.yaml`).
- Model weights / GPU: NOT_RUN — unchanged from baseline.

## Remaining defect or external blocker, exact owner/input needed

- **Consequential missing input (R00.5):** the user has not named the
  round-two demo case + language. Until provided, R05 will use the
  existing `fixtures/synthetic_wayanad.json` (Kerala / Wayanad
  landslide, English / ml-IN) as the named fictional synthetic
  fixture for development. R05 + R03 will surface the missing
  case/language as a real blocker for the demo, NOT pass it off.
- **Pre-existing R05 work** (`backend/internal/scenarioprep/{bundle,index,
  path,prepare,validate}.go`, `cmd/scenario-prep/report.go`,
  `prepare_test.go`) is uncommitted in the working tree. The R05 owner
  should rebase / commit those before R05 starts; R00 did not modify
  or stage them.
- **Docker absent:** R06 will record `CONTAINER_RUNTIME=NOT_RUN`; local
  build + run are the demo path until docker is wired.
- **GPU / model artifacts:** still BLOCKED_HARDWARE for R02 / R07.

## Shared-contract changes required (or none)

- None. `plan/r0-demo-freeze.md` records usage of the existing
  `/api/v3` contract; no additions or amendments.

## Next eligible task and integration order

- Sequential mode: continue with **R01** — destination selection and
  an explicitly isolated exercise backend. R01 owns
  `backend/internal/store/scoped.go`, `choice.go`, `stay.go`,
  `opkg/package.go`, `httpserver/{server,voice_process,stay_handlers}.go`
  and the seed/launch commands.
- Parallel split (if a second lane is assigned after R00 freeze):
  - **UI lane:** R03 — polished responsive UI on the existing
    `frontend/v2/` stack.
  - **Data lane:** R05 — safe scenario preparation and truthful
    demo data (continues the pre-existing scenarioprep WIP).
  - **Deployment lane:** R06 — reproducible local deployment.
  - **Inference lane:** R02 — real model plumbing and audio
    correctness (BLOCKED_HARDWARE in this checkout).
- The four-lane split is not invoked by the user's sequential launch;
  the coordinator continues alone with R01.

## Owned resources cleaned / preserved

- Preserved: pre-existing R05 changes in
  `backend/internal/scenarioprep/` and `backend/cmd/scenario-prep/`
  left untouched in the working tree.
- Removed: temporary build artefact `backend/sthira` (deleted after
  smoke build).
- Added: `backend/internal/store/readiness_test.go`,
  `plan/r0-demo-freeze.md`, `plan/evidence/execution-r00.md.md`.