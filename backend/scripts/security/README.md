# P7 preparation — runners, scripts, and the threat model

This directory is the **P7 preparation** layer for the `sthira`
backend. It is *not* a security certificate or a Gate B pass.

## What lives here

```
backend/scripts/security/
  _lib.sh                       # shared shell helpers + snapshot metadata
  versions.env                  # pinned versions + snapshot dates
  run-static-analysis.sh        # go vet + staticcheck + govulncheck + gosec
  run-fuzz.sh                   # native go fuzz bounded time per target
  run-secret-scan.sh            # gitleaks, with redaction policy
  build-sbom.sh                 # Go modules dep graph + syft / trivy (when installed)
  run-zap-staging.sh            # ZAP API scan against an explicit allowlisted target
  run-strix-prep.sh             # Strix CONFIG-ONLY; live run gated by O15
  gitleaks.toml                 # repo allow-list; rationale per rule
  zap_stop_conditions.env       # documented stop rules for ZAP
  reports/                      # run artifacts (snapshot.json, tool-resolved.json, sbom/)

backend/security/
  threat-model.md               # current-state threat model (T1..T10 + trust boundaries)
  pending-p6.md                 # findings whose fix surface is a P6 worker
  findings.md                   # findings register index (manual + scanner-derived)
  redacted-findings.json        # machine-readable, redacted findings register
  ci-delta.md                   # PROPOSED CI step (text-only — coordinator owns CI edits)
```

## Where narrowly-scoped security tests live

The narrow tests are *attached to the package they exercise* and
named `*_security_test.go`:

  - `internal/capfeed/capfeed_security_test.go` —
    loopback / RFC-1918 / link-local reject; redirect-loop
    fallback to preserved cache (F-CAPREDIRECT-01).
  - `internal/httpjson/httpjson_security_test.go` — oversize,
    depth, duplicate keys, unknown fields, trailing data, hostile
    encoding, BOM, lone NUL.
  - `internal/offlinequeue/offlinequeue_security_test.go` —
    `validateEndpoint` reject matrix + strict-redirect dispatcher
    (F-OUTGOINGEGRESS-01).
  - `internal/httpserver/httpserver_security_test.go` —
    `bearerToken` strictness (F-BEARERBP-01) and slow-headers bound.
  - `internal/offlineclient/offlineclient_security_test.go` —
    `absoluteURL` scheme / host inheritance, forged-checksum
    resource reject, signed-manifest gate.
  - `internal/sourceact/sourceact_security_test.go` — Source
    struct has no credential fields; non-Operational source
    cannot drive guidance (F-OPSIGMAUTH-01).
  - `internal/contracts/contracts_security_test.go` —
    pipeline / TTS request shapes: no arbitrary tools / URLs /
    HTML at the envelope; state codes are finite.

## What does NOT live here

  - **Shared CI edits**: `backend/scripts/ci.sh`,
    `.github/workflows/...` are owned by the coordinator. The
    proposed step is documented in `ci-delta.md` only.
  - **Production code**: any patch to existing security surfaces
    (`capfeed`, `httpjson`, `offlineclient`, etc.) belongs to the
    respective worker module.
  - **Real Strix execution**: blocked on O15; the runner at
    `scripts/security/run-strix-prep.sh` records the configuration
    only and refuses to invoke paid inference.
  - **Real ZAP execution**: blocked on the absence of a
    disposable, allowlisted staging target. The runner refuses
    any non-loopback / non-private target and records the
    stop conditions.
  - **Blanket suppression of scanner findings**: forbidden by the
    task brief. Every allowed rule carries a rationale; every
    acceptance row in `findings.md` carries evidence.

## Run order (P7-prep)

```sh
# 1. Static analysis (vet always; staticcheck/govulncheck/gosec when installed).
bash backend/scripts/security/run-static-analysis.sh

# 2. Native fuzz for the parsing / boundary packages.
FUZZ_TIME=30s bash backend/scripts/security/run-fuzz.sh

# 3. SBOM.
bash backend/scripts/security/build-sbom.sh

# 4. Secret scan (when gitleaks is installed).
bash backend/scripts/security/run-secret-scan.sh

# 5. Strix *config-only* record (no live invocation).
bash backend/scripts/security/run-strix-prep.sh

# 6. ZAP staging, ONLY against an explicit allowlisted local target.
ZAP_TARGET_URL=http://127.0.0.1:8080 \
ZAP_API_KEY="$ZAP_LOCAL_KEY" \
    bash backend/scripts/security/run-zap-staging.sh
```

Every step records an artifact under
`backend/scripts/security/reports/` and exits 77 (TAP-style
*skip*) when the tool is absent — never `0 found`.

## Promotion checklist (P7-prep → P7 body)

See `backend/security/ci-delta.md` for the full promotion
checklist. In summary: install the pinned tools, surface a
disposable staging target, resolve O15, and adopt the proposed
CI step without modification.

## Why this directory is small

The brief warned against spending hours running overlapping
tools to generate a cosmetic score. Only the runners with
*evidence* (scripted or static) commit; the rest are
configuration-only.
