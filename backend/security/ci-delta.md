# P7 preparation — CI delta

The P7-prep window identifies security-tooling steps that should
run on every commit and on every dependency surface change. The
proposed deltas below are **NOT** applied to shared CI by this
directory; they live here so a reviewer can lift them into the
shared owner-controlled CI config (`.github/workflows/`,
`backend/scripts/ci.sh`, or equivalent).

## Why this is a proposal, not a change

`backend/scripts/` already contains owner-controlled CI scripts
(`p4_*.sh`); the coordinator owns the CI layer. P7-prep ships
the P7 runners and a *proposed* CI step the coordinator can adopt
when ready. Adding the step unchanged anywhere outside this
directory would be a shared CI edit, which is out of scope here.

## Proposed step (text-only, not applied)

```yaml
# Proposal — for the shared CI workflow, owned by Coordinator.
# Apply verbatim once the team accepts the P7-prep thresholds.

security-prep:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0   # full history; gitleaks needs it

    - uses: actions/setup-go@v5
      with:
        go-version-file: backend/go.mod

    # 1. Stdlib static analysis (always-on).
    - name: go vet
      run: go -C backend vet ./...

    # 2. Pinned staticcheck. Tooling install is in shared CI today;
    #    P7-prep keeps the pin in
    #    `backend/scripts/security/versions.env`.
    - name: staticcheck
      run: bash backend/scripts/security/run-static-analysis.sh

    # 3. Native fuzz (bounded time; the corpus is on-disk).
    - name: fuzz
      run: FUZZ_TIME=30s bash backend/scripts/security/run-fuzz.sh

    # 4. Govulncheck is part of run-static-analysis.sh; kept in
    #    one step to bound runtime.

    # 5. Secret scan. Suppressions live in
    #    `backend/scripts/security/gitleaks.toml`. The runner
    #    refuses to print secrets in raw form.
    - name: secrets
      run: bash backend/scripts/security/run-secret-scan.sh

    # 6. SBOM. Uploaded as an artifact for Gate B evidence; not
    #    gated on the script exit, only on the hash.
    - name: sbom
      run: bash backend/scripts/security/build-sbom.sh

    # 7. Findings register is consulted (the runner itself does
    #    not depend on it).
    - name: register current
      run: test -f backend/security/findings.md
```

## Thresholds (proposed)

| Tool | Pass criterion | Failure criterion |
|------|----------------|-------------------|
| `go vet`               | clean | any issue |
| `staticcheck`          | clean | any HIGH (style fixes are advisory only) |
| `govulncheck`          | no Symbol-level findings | any confirmed CVE |
| `gosec` (when present) | scoped list `internal/...` `cmd/...` | any HIGH or CRITICAL |
| `gitleaks`             | clean | any non-allowlisted match; secrets are **never** printed by the runner — only the rule-id and the file path |
| fuzz (`go test -fuzz`) | clean for `FUZZ_TIME` per target | any crash |

## What this does NOT do (per the task brief)

  - It does **not** auto-blanket-suppress any scanner finding.
  - It does **not** invoke Strix or ZAP against any real
    target. Both runners are configuration-only at P7-prep;
    the coordinator promotes them into a staging job once a
    disposable staging host exists (not yet at P7-prep; O14
    and O15 must resolve first).
  - It does **not** edit the shared CI files in
    `.github/workflows/`. That belongs to the coordinator.

## Reporting

Run artifacts land in `backend/scripts/security/reports/`. The
proposed CI upload step:

```yaml
    - uses: actions/upload-artifact@v4
      if: always()
      with:
        name: p7-prep-reports
        path: backend/scripts/security/reports/
```

Reviewers diff the SHA-256s across runs to detect drift; the
SBOM sha256 is the dependency-state evidence that lands in
Gate B.

## Promotion checklist (from P7-prep to P7 body)

  1. Pinned tools installed at the documented versions and
     SHA-pinned.
  2. The proposed CI step above is adopted by the coordinator
     without modification (or with explicit reviewer notes on
     what changed).
  3. A disposable staging target exists with synthetic
     accounts; ZAP staging step promoted from proposal to
     implementation.
  4. O15 is resolved (Strix model/spend/scope).
  5. The findings register's open entries are closed or
     have a signed owner + bounded disposition; nothing is
     blanket-suppressed.
