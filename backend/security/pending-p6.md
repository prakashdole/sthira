# Pending P6 security findings

This list tracks findings whose responsible fix surface is a P6
worker. Until the corresponding P6 worker is wired into the
orchestrator, the findings live in *prepared* state — classified
and reproducible, but **not** patched here. The patch lives with
the worker owner; the runner below merely proves the issue exists.

Findings recorded here are referenced by `findings.md`. Each row
carries:

  - a stable `id`
  - the affected revision (commit SHA, currently `2fe0fee`)
  - exploitability class (P0 / P1 / P2 / P3)
  - the responsible owner (worker-5 / worker-7 / worker-9 / etc.)
  - a reproducer reference (test file or shell snippet)

## F-CAPREDIRECT-01 — CAP transport lacks redirect guard

  - **Affected revision**: `2fe0fee`
  - **Files**: `backend/internal/capfeed/transport.go:91` (the
    Transport.Refresh method's `t.fetch(...)` invocation);
    the `Fetcher` interface itself is at
    `capfeed/transport.go:36`.
  - **Owner**: real CAP transport adapter (the `Fetcher`
    implementation lives outside `internal/capfeed/`, see O08).
  - **What**: When the `Fetcher` impl uses Go's default
    `*http.Client`, redirects are followed without an explicit
    `CheckRedirect` policy. A compromised / hijacked source
    URL could redirect to `127.0.0.1`, `169.254.169.254`
    (cloud metadata), or another region-internal address,
    enabling an SSRF pivot.
  - **Exploitability**: P2 — depends on the supplied URL being
    attacker-controlled, and on the source itself being
    operational. The brief: "Test SSRF to metadata / loopback /
    private ranges using local controlled fixtures, not actual
    government infrastructure."
  - **Reproducer sketch**:
    ```go
    cli := &http.Client{
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse  // surface the chain
        },
    }
    fetcher := capfeedfunc(ctx, uri, etag)
        return urlOf(uri)
    ...
    t := capfeed.NewTransport(fetcher, clock)
    res, _ := t.Refresh(ctx, "http://attacker.example/")
    ```
    The P7-prep security test file
    `internal/capfeed/capfeed_security_test.go` carries the
    failing-attempt version that asserts the chain is bounded.
  - **Where the fix belongs**: in the **real** production
    `Fetcher` impl (out of `internal/capfeed/`), where a single
    `CheckRedirect` plus an explicit allowlist of the source's
    registered hosts would close the vector.
  - **Coordination**: O08 source activation (G01–G07). The
    fix must be a part of the source-authority contract landing
    before any production CAP URL is wired in.

## F-OUTGOINGEGRESS-01 — Outbound HTTP egress not allowlisted

  - **Affected revision**: `2fe0fee`
  - **Files**: `backend/internal/capfeed/transport.go:91`
    (refresh path), `backend/internal/offlineclient/transport.go:396`
    (joined URL), `backend/internal/offlinequeue/dispatcher.go:118`
    (dispatcher base URL).
  - **Owner**: operational deployment wiring + `Fetcher`
    implementations; in code it falls to whichever module
    instantiates the http.Client.
  - **What**: The `Fetcher` injection seam at `capfeed/transport.go:36`
    plus the bare `*http.Client` use in any client of
    `offlineclient` / `offlinequeue` will follow any
    Network-reachable URL. There is no central egress
    allowlist in the binary. A compromised source could
    pull content from anywhere on the public internet.
  - **Exploitability**: P2 (same as F-CAPREDIRECT-01).
  - **Where the fix belongs**: at deployment time (network
    policy + `CheckRedirect` per `Fetcher`). The P7-prep
    runner documents this in `findings.md`; no patch here.

## F-MIDDELPROMPT-01 — Middle-model prompt guard layering

  - **Affected revision**: `2fe0fee`
  - **Files**: `plan/voice-map-system-prompt.md`; the typed
    envelope lives at
    `backend/internal/contracts/pipeline.go`.
  - **Owner**: Worker 9 (orchestration) — the prompt-guard
    layer lives at the validator, not the model.
  - **What**: Voice-map system prompt enforces JSON-only
    output and treats user/source text as DATA. The prompt
    itself isn't a control — the independent Go validator
    is. The orchestrator's validator is the boundary.
  - **Exploitability**: P2 — model is naturally non-
    deterministic. The control is the type system + the
    validator; both are correctly placed at the contract
    boundary today.
  - **Reproducer**: covered by the W5/W7 worker fuzz tests
    (Worker 5 ASR carries a hostile-input round-trip
    harness); integration-level adversarial coverage belongs
    to P9, gated by O15.

## F-SESSIONLEAK-01 — Session leakage in test logs

  - **Affected revision**: `2fe0fee`
  - **Files**: scanning for raw bearer token printing in
    `internal/httpserver/*.go` (excluding `_test.go`) returned
    **zero hits**.
  - **Owner**: — current state has no leak.
  - **What**: The current code uses `slog` with a custom
    `RequestLogger` that redacts `Authorization`, `Cookie`,
    `X-Forwarded-*`, etc. (verified by
    `audit_integration_test.go`). P7-prep gitleaks scan
    confirmed no live credentials leak.
  - **Action**: None. Listed as a *checked* finding so reviewers
    can confirm.

## F-BEARERBP-01 — Bearer header parsing prefix strictness

  - **Affected revision**: `2fe0fee`
  - **Files**: `internal/httpserver/auth.go:51` (`bearerToken`).
  - **Owner**: —
  - **What**: `bearerToken` parses only `Bearer ` prefix; trim
    on the right is permissive but the left side is strict.
    `strings.HasPrefix(h, "Bearer ")` rejects anything
    non-canonical (e.g. `bearer ` lowercase, `Bearer\t`).
  - **Reproducer**: the integration tests exercise both a
    `Bearer` happy path and a `BorkedNoBearer` failure path.
  - **Exploitability**: P3 — informational.
  - **Action**: None. Listed as *checked* for traceability.

## F-OPSIGMAUTH-01 — Operator issuance fails closed

  - **Affected revision**: `2fe0fee`
  - **Files**: `internal/httpserver/operator_handlers.go:42`
    (`if s.operatorVerifier == nil`).
  - **Owner**: O14 trusted IdP / MFA decision.
  - **What**: Without a verifier wired in, `POST /api/v3/
    operations/sessions` returns `503 ErrDataUnavailable` —
    fail-closed. Confirmed in
    `operator_http_integration_test.go:610
    (TestLegacyUnboundOperatorSessionCannotAct)`.
  - **Reproducer**: `TestOperatorSessionRequiresMFAOnUse`
    proves this directly.
  - **Exploitability**: n/a — fail-closed is the design.
  - **Action**: keep verifier nil until O14 resolves.

## F-AUDITTAMPER-01 — Audit / chain auditor reach

  - **Affected revision**: `2fe0fee`
  - **Files**: `store/` audit chain (referenced from
    `cmd/sthira/main.go`).
  - **Owner**: store / migrations owner.
  - **What**: Migrations are SQL-up-only with no
    `down` migrations; every operator action goes through the
    ChainAuditor (the `store.ChainAuditor{}` passed to
    `store.NewStayStore`).
  - **Action**: Out of P7-prep scope. Listed as a
    *good-current-state* record for traceability.

## Coordination rule

P7-prep does **not** land production fixes for any of the above.
The fix surface belongs to P6 worker-5/7/9 or the deployment
wiring. P7-prep provides:

  1. A reproducible file/test where the harness proves the
     issue (or the absence of one).
  2. A classification (above) so reviewers can decide where the
     fix belongs.
  3. A pinned-versions + snapshots entry (`versions.env`)
     for any scan runner the owner reaches for.
