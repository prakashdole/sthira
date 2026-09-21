# scenario-prep — offline data preparation CLI

`scenario-prep` is a Go command-line tool that turns a user-supplied
catalogue + per-scenario operational-package inputs into a consistent,
reviewable, deterministic exercise handoff bundle. It is the data
preparation stage for Sthira scenario work; it is **not** a P5 offline
package, **not** a production importer, **not** a signing authority, and
**not** an independent schema implementation. It runs entirely offline:
no database, model, network, or government API is touched.

## Install / build

```sh
cd backend
go build -o scenario-prep ./cmd/scenario-prep
```

The binary uses only the existing backend Go module and standard
library. No new dependencies are added; the existing catalogue and opkg
validators are reused.

## Subcommands

| Subcommand | Writes files? | Description |
| --- | --- | --- |
| `init`     | yes (DRAFT templates) | produce a minimal draft workspace for the user to fill in |
| `validate` | no  | structural and cross-reference check; reports findings to stdout |
| `report`   | yes (JSON + Markdown) | machine- and human-readable review reports |
| `bundle`   | yes (handoff dir) | deterministic copy of validated inputs + manifest |

Each subcommand accepts `--help` for full flag documentation.

## Quick start

```sh
scenario-prep init --workspace ./ws
scenario-prep validate --workspace ./ws --index examples/index.json
scenario-prep report   --workspace ./ws --index examples/index.json
scenario-prep bundle   --workspace ./ws --index examples/index.json --output ./ws-out --allow-draft
```

## Exit codes (documented for automation)

| Code | Meaning |
| --- | --- |
| 0 | SUCCESS — `PREPARATION_READY` bundle produced (or `validate` returns 0 when status is `READY`) |
| 2 | INVALID — structural validation failure |
| 3 | INCOMPLETE — structurally valid but missing data |
| 4 | IO_ERROR — file system, permission, or unsafe layout |
| 1 | OTHER — usage or internal error |

## What the tool does

- Strict JSON decoding: bounded reads, rejects duplicate keys, unknown
  fields, trailing data, depth abuse, and oversize bodies via
  `internal/httpjson`.
- Reuses `catalogue.Validate` (structural) and `catalogue.Validation.AssessLaunch`
  (10–15 states / 2–3 scenarios-per-state threshold).
- Reuses `opkg.Validate` under an explicitly declared
  exercise/preparation context; `requireSignature=false`, signature
  status reported as `UNVERIFIED` because the tool has no trusted
  verifier. There is no dummy verifier returning true.
- Cross-checks the scenario/package mappings: unknown scenario IDs,
  duplicates, missing files, expected jurisdiction mismatch, conflicting
  references, missing zone/route/policy data, missing historical
  evidence, and synthetic-with-event-ref misuses.
- Rejects workspace escape (`..`), absolute paths, `~`-expansion, unsafe
  symlinks, output-inside-input recursion, and unbounded attachments.
- Copies validated inputs into a deterministic directory with sorted
  index, raw SHA256 (separate from the opkg canonical checksum), and
  original provenance label. Atomic publication, refuses overwrite,
  never writes to the database, never generates signing key material,
  never includes private keys.

## What the tool does NOT do

- Sign packages or claim P5 offline-client compatibility.
- Publish to citizens, activate sources, or generate signing keys.
- Contact any government system, model, or database.
- Infer routes, zones, languages, or scenarios the user did not supply.
- Call signature verification trusted (it reports `UNVERIFIED`).

## Trust boundaries

The tool sits between the user/curator and the P5 offline-client
package builder. It produces a reviewable handoff bundle; it does **not**
introduce a competing manifest format that claims P5 compatibility. Worker
1 owns the P5 offline-client package format. This tool is a
data-preparation step, not a publication step.

## Example

See `example/` for a small clearly synthetic workspace demonstrating
the full workflow (init → validate → report → bundle).

The example catalogue is below the 10-state launch minimum; running
`bundle` against it without `--allow-draft` produces a rejection summary
because `INCOMPLETE` inputs cannot yield a `PREPARATION_READY` bundle.
Use `--allow-draft` to publish a `PREPARATION_DRAFT` bundle for review.

## History

See `HANDOFF.md` for worker instructions, integration notes, and the
ordered commits this lane produces for final integration.