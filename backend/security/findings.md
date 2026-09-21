# Findings register — P7 preparation

Findings collected from the manual surface review at
`security/threat-model.md`. They map to the rows in
`security/pending-p6.md` and the narrowly-scoped security test
files added in this directory.

Schema: every entry carries `id`, `revision`, `reproducer`,
`exploitability`, `owner` (worker / phase), `state`
(`open | ready-for-fix | accepted | closing`), and `redacted` =
`true` so the registry never carries a cleartext secret / raw
fuzz-payload / API key.

Versions and snapshot dates live in
`backend/scripts/security/versions.env`. The reports tree at
`backend/scripts/security/reports/` carries the run artifacts.

## Index (manual surface scan + observed scanner state)

The scanner columns summarize what is known AFTER running the
available tooling at P7-prep. Tools missing on the host (see
`reports/<tool>-resolved.json`):

  - staticcheck — `absent` (pin `dominikh/go-staticcheck v0.7.0`)
  - govulncheck — `absent` (pin `golang/vuln v1.1.4`)
  - gosec — `absent` (pin `securego/gosec v2.22.10`)
  - gitleaks — `absent` (pin `gitleaks/gitleaks v8.27.0`)
  - trivy — `absent` (pin `aquasecurity/trivy v0.67.0`)
  - syft — `absent` (pin `anchore/syft v1.31.0`)
  - ZAP — `absent` (pin `ghcr.io/zaproxy/zaproxy:2.16.0`)
  - Strix — `absent` (O15 open; configuration-only)

Until the team installs the pinned versions, the scanner-derived
columns read **NOT_EVALUATED** instead of "0 found". An absent
tool is never reported as "0 vulnerabilities observed" — that
is exactly the fabrication the brief forbids.

| ID | Class | Owner | Exploitability | State | Notes |
|----|-------|-------|----------------|-------|-------|
| F-CAPREDIRECT-01 | SSRF / redirect | O08 source activation | P2 | ready-for-fix | reproducer in `internal/capfeed/capfeed_security_test.go` |
| F-OUTGOINGEGRESS-01 | Egress allowlist | O08 / O14 deployment wiring | P2 | ready-for-fix | noted in `threat-model.md` T3 / T5 |
| F-MIDDELPROMPT-01 | Prompt injection | Worker 9 (W9) | P2 | accepted | control lives at the validator, not the model |
| F-SESSIONLEAK-01 | Log scrubbing | — | n/a | accepted | current `RequestLogger` redacts Authorization / Cookie / X-Forwarded-* |
| F-BEARERBP-01 | Header parsing strictness | — | P3 (informational) | accepted | `Bearer ` prefix only; tested in `operator_http_integration_test.go` |
| F-OPSIGMAUTH-01 | Operator issuance fail-closed | O14 IdP/MFA | n/a | accepted | fail-closed is the design; O14 unblocks go-live |
| F-AUDITTAMPER-01 | Audit chain reach | store / migrations | n/a | accepted | SQL-up-only migrations; ChainAuditor wired |

## Scanner-derived columns (data: 2026-09-21)

| Tool | Status | Result | Notes |
|------|--------|--------|-------|
| `go vet`           | ran | clean | no findings |
| native fuzz (`go test -fuzz`) | ran | clean (10 s per target; capfeed 215k inputs, httpjson/offlinepkg trivial) | runner at `scripts/security/run-fuzz.sh` |
| `staticcheck`      | NOT_EVALUATED | tool absent | install at run-window |
| `govulncheck`      | NOT_EVALUATED | tool absent | install at run-window |
| `gosec`            | NOT_EVALUATED | tool absent | install at run-window; targeted scope pre-recorded |
| `gitleaks`         | NOT_EVALUATED | tool absent | redacted output path configured |
| `trivy` (SBOM/fs)  | NOT_EVALUATED | tool absent | `syft` is the alternative |
| `syft` (SBOM)      | NOT_EVALUATED | tool absent | `go-mod-graph` was used as a stand-in |
| `go mod graph`     | ran | n/a — record only | `reports/sbom/go-mod-graph.txt.{sha256}` |
| `ZAP`              | NOT_EVALUATED | tool absent + no disposable staging | `scripts/security/run-zap-staging.sh` enforces loopback / RFC-1918 target check |
| `Strix`            | NOT_EVALUATED | O15 open | `scripts/security/run-strix-prep.sh` records the config block only |

## Findings classification (per brief)

  - **Reproduction**: every entry in the manual register has a
    reproducer (a Go test file path or a curl/CLI snippet).
    Scanner entries above carry the raw reproducer inside
    `reports/<tool>.{json,txt}` once the runner fires.
  - **Affected revision**: every entry carries the SHA / tree
    path. The current snapshot is `2fe0fee` (P6 contract
    freeze).
  - **Exploitability**: P0–P3 plus `accepted | n/a`. The
    entries marked `accepted` are NOT open vulnerabilities —
    they are explicit descriptions of *good* current state so
    a reviewer can confirm.
  - **Ownership**: each entry names the worker / phase / O
    decision that owns the eventual fix. P7-prep does not
    patch any of these.

## Redaction discipline

  - No finding carries a raw secret, credential, or live API
    key. Reproducers that need a token use the placeholder
    `REPLACE_WITH_TOKEN` and fail loudly if not replaced at
    run-time.
  - Fuzz crash-corpus artifacts are not committed; they're
    written to `reports/<tool>.txt` only on demand.
  - Suppressions, where present, are tied to a rule-id and a
    signed rationale (see `gitleaks.toml`); blanket
    suppressions are not allowed.

## What this register is *not*

  - It is not a security certificate.
  - It is not a Gate B pass.
  - It is not a Strix run.
  - It is not an attestation about production safety.

It is a snapshot of the current P7-prep surface and the
findings-classification habits the team will need at P7 body.

## Redacted JSON

A machine-readable, redacted copy is at `redacted-findings.json`.
It carries the same classification plus SHA-256s of the report
artifacts that would otherwise need to be diffed by hand.
