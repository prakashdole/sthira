# Worker 4 handoff — scenario-preparation-cli

Base SHA: `53302b26f886f8cf3d4de326fb1d43423da0c54e` (CLEAN).

## Deliverable

A new offline CLI `scenario-prep` under `backend/cmd/scenario-prep/`
plus docs and a small clearly synthetic example under
`tools/scenario-prep/`. The tool reuses the existing
`internal/catalogue` and `internal/opkg` validators and the existing
`internal/httpjson` strict JSON decoder.

| Path | Status |
| --- | --- |
| `backend/cmd/scenario-prep/` | new (main, init, validate, report, bundle, prepare helpers) |
| `backend/internal/scenarioprep/` | new (Prepare, Bundle, helpers) |
| `tools/scenario-prep/` | new (README, HANDOFF, example) |

No root dependency edits. No concurrent edits to catalogue/opkg/shared
contracts. No worker-1 or worker-2 files touched. All `.txt` files
preserved.

## Verified commands and results

| Command | Result | Exit |
| --- | --- | --- |
| `go build ./...` (backend) | success | 0 |
| `go vet ./...` (backend)   | success | 0 |
| `go test ./...` (backend)  | all packages PASS, including existing catalogue/opkg tests | 0 |
| `go test ./internal/scenarioprep/...` | 5/5 PASS (unknown scenario, path escape, dup keys, refuse overwrite, deterministic repeat) | 0 |
| `scenario-prep init --workspace <tmp>` | creates 7 files (templates + 4 example files) | 0 |
| `scenario-prep validate --workspace <tmp> --index examples/index.json` | status=INCOMPLETE (1 state vs 10 min), exits 3 | 3 |
| `scenario-prep report ... --out-dir <dir>` | writes report.json + report.md | 3 |
| `scenario-prep bundle ... --output <dir>` | writes bundle-rejection.json (INCOMPLETE) | 3 |
| `scenario-prep bundle ... --allow-draft` | writes PREPARATION_DRAFT bundle with manifest + copied inputs | 3 |
| Deterministic repeat of `bundle` | byte-identical manifest across two runs | n/a |
| Malformed catalogue (unknown field) | strict decode rejected, exit 2 | 2 |
| Duplicate scenario key in index | strict decode rejected, exit 2 | 2 |
| Absolute path escape | rejected as UNSAFE_REFERENCE_PATH | 2 |
| `..` path escape | rejected as UNSAFE_REFERENCE_PATH | 2 |
| `~` expansion | rejected as UNSAFE_REFERENCE_PATH | 2 |
| Output-inside-input | rejected as ErrUnsafeLayout, exit 4 | 4 |
| Overwrite existing output dir | refused, exit 4 | 4 |

The verify suite is bounded: it exercises one valid example and one
invalid input. It does not run a broader audit, load campaign, or
repeated full test suite, per the lane instructions.

## New commits (local, not pushed)

| Stage | SHA | Description |
| --- | --- | --- |
| 1 | `cf5947fbb2b091f14efd250b9d7a85563e5d12fc` | initial CLI + internal/scenarioprep + example workspace + docs |

Worker 1 should cherry-pick these onto its
`codex/backend-authority-closure` branch. No shared contract changes
are required.

## API gap report

None of the catalogue or opkg APIs needed modification. The tool reuses:

- `catalog.Manifest`, `catalogue.Validate`, `catalogue.Validation`,
  `catalogue.Validation.AssessLaunch`, `catalogue.Acceptable`
- `opkg.Package`, `opkg.Provenance`, `opkg.Validate`, `opkg.Checksum`,
  `opkg.AllocationPolicy`, `opkg.Zone`, `opkg.Route`, etc.
- `httpjson.DecodeStrict` for the catalogue, the index, and every
  referenced package

No changes were needed to any shared contract, store, httpserver,
orchestration, or migrations. Worker 1's owned files are not edited
concurrently.

## Limitations and explicit NOT_RUN items

- **No real trusted verifier.** Signature status is reported UNVERIFIED
  in every run. The tool never wires a verifier returning true; it
  surfaces `CodeSignatureUnverified` as an `INFO` finding and labels
  the report's `signature_status` field accordingly.
- **No live external access.** The tool never follows URLs in input
  fields, never calls any government endpoint, and never crawls private
  files. It only reads files inside the user-supplied workspace.
- **No P5 compatibility claim.** The bundle manifest carries raw file
  hashes (separate from opkg checksums), an honest preparation status
  (`PREPARATION_READY`/`PREPARATION_DRAFT`/`PREPARATION_REJECTED`), and
  the original provenance label. It does not claim P5 offline-client
  compatibility and never generates signing keys or signature
  placeholders.
- **No closure of external gates.** O01, O05, O07, O08, O11, and O14
  remain open. The tool lists them as `UnresolvedGates` in every
  report; it does not claim the example satisfies any of them.
- **Catalogue-level structural thresholds are preserved.** The tool
  reuses `catalogue.MinLaunchStates = 10`, `MaxLaunchStates = 15`,
  `MinScenariosPerState = 2`, `MaxScenariosPerState = 3`. It does not
  invent a different acceptance standard.

## Exit code semantics

| Code | Meaning | Trigger |
| --- | --- | --- |
| 0 | SUCCESS | `bundle` produced `PREPARATION_READY`, or `validate`/`report` status READY |
| 2 | INVALID | structural validation failure (strict-decode errors, opkg errors, cross-check errors, jurisdiction mismatch, unsafe paths, missing files) |
| 3 | INCOMPLETE | structurally valid but data is missing; `bundle` refused unless `--allow-draft` is set |
| 4 | IO_ERROR | file not found, permission, layout (output-inside-input, overwrite) |
| 1 | OTHER | usage or unexpected internal error |

## Example output (truncated)

```
$ scenario-prep validate --workspace ./ws --index examples/index.json
scenario-prep validate: status=INCOMPLETE states=1 scenarios=2 historical=1 findings=2
warnings:
  [LAUNCH_GAP] catalogue: INSUFFICIENT_STATES: 1 selected states is below the launch minimum of 10
info:
  [SIGNATURE_UNVERIFIED] scenario-prep has no trusted verifier configured; signature status is UNVERIFIED
$ echo $?
3
```

## Coordination with Worker 1

Worker 1 owns final integration onto `codex/backend-authority-closure`.
Worker 1 should:

1. Pull this branch (`codex/scenario-preparation-cli`).
2. Cherry-pick or merge the ordered commits produced here (commit list
   is appended at the end of this file as commits are made).
3. No shared contract changes are required: this lane does not edit
   `catalogue`, `opkg`, `httpjson`, store, httpserver, or orchestration.
4. Run `go build ./...` and `go test ./...` after integration to
   confirm no regressions.

## Coordination with Worker 2

Worker 2 owns inference modules, eval, loadmodel and operational
scripts. None of those files are touched by this lane; no shared types
are edited. No coordination request is needed.

## Self-check on the strict rules

- "no new dependencies": confirmed — `go.mod` is unchanged.
- "no main / push / reset / amend / rebase / history rewrite":
  confirmed — all commits are local on a fresh worktree.
- "no .txt file edits": confirmed — only new files created.
- "no live government / model / network": confirmed — tool is offline
  only and never imports network primitives.
- "no broad audit / load campaign / repeated full suites": confirmed —
  only one example workspace + one invalid input + 5 regression tests
  in `internal/scenarioprep`.
- "no weakening of safety to demonstrate success": confirmed — exit
  code 3 for INCOMPLETE is correct per the documented contract.
- "do not close O01 / language approval / route/stay policy / any
  phase gate": confirmed — gates listed as `UnresolvedGates` and never
  claimed closed.