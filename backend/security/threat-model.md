# Threat model — current state, P7 preparation window

This document maps the **current** security surface of the
`/api/v3` backend as it exists at the P6 contract freeze
(`2fe0fee`). It is a *preparation* threat model, not a deployed-
system threat model. Anything described as "not yet wired" is
intentional and is owned by a follow-up phase.

## Scope

**In scope**

  - `backend/internal/httpserver/` — citizen + operator routes
  - `backend/internal/capfeed/` — government CAP fetch + parse
  - `backend/internal/sourceact/` — source authority gate
  - `backend/internal/offlineclient/` — signed offline-package
    transport (`TrustStore`, manifest signature)
  - `backend/internal/offlinequeue/` — retry-safe queued writes
  - `backend/internal/offlinepkg/` — package signing primitives
  - `backend/internal/offlineresources/` — signed distribution
  - `backend/internal/offlinedelivery/` — package delivery routes
  - `backend/internal/httpjson/` — strict JSON decoder (depth +
    duplicate-key + unknown-field)
  - `backend/internal/contracts/` — frozen wire envelopes
    (P6 typed boundary — `contracts/transcription.go`,
    `contracts/tts.go`, `contracts/worker_health.go`,
    `contracts/pipeline.go`, `contracts/context.go`,
    `contracts/envelope.go`, `contracts/errors.go`)

**Out of scope**

  - mobile client + offline pack runtime (P8, P9)
  - operational source activation (G01–G08) — O08 unresolved
  - live operator identity provider — O14 unresolved; issuance
    fails closed (503) without a wired `OperatorVerifier`
  - real Strix adversarial exploration — O15 unresolved

## Threats and current controls

The list mirrors the assurance.md threat list but is annotated with
the *current* code path.

### T1 — Malicious citizen / client

  - Surface: `/api/v3/...` citizen routes
    (`/sessions`, `/places/resolve`, `/guidance/query`,
    `/reservations`, `/reservations/{id}/events`).
  - Current controls:
    - `Bearer`-token-only auth at the route middleware
      (`internal/httpserver/auth.go:24`), `bearerToken` parses
      only the canonical `Bearer ` prefix, token bytes are NOT
      logged.
    - Session resolution at `store.Authenticate` with bounded
      lookup; expired/revoked sessions reject at the route.
    - `http.MaxBytesReader` on every body at `server.go:398`
      with `MaxBodyBytes: 1 << 20` = 1 MiB.
    - Slowloris-bound `ReadHeaderTimeout: 5s`, `ReadTimeout: 10s`
      (`internal/httpserver/config.go:34`).
    - Strict-JSON decode (`internal/httpjson/decode.go`) with
      `MaxDepth: 32`, duplicate-key detection, and
      `DisallowUnknownFields()`.
    - All reservation/stay handlers roll the change through
      `store.NewStayStore(...)` inside a DB transaction
      (`internal/httpserver/stay_handlers.go:108`, etc.)
  - Notes / follow-up:
    - The route map documents no stateless `/api/v3/voice/...`
      rate limiting yet — voice handlers will inherit the
      shared `MaxBodyBytes`; an explicit per-route token
      bucket is a P9 follow-up.
    - Idle-session GC: not exposed at P7; rely on `store`
      expiry tables (already invented).

### T2 — Stolen / replayed / cross-district session

  - Surface: same as T1.
  - Current controls:
    - Tokens are stored as `sha256(token)`-derived rows in the
      session table (`internal/store/session.go:14`), so a DB
      leak yields only digests, not tokens.
    - The reservation read path uses session-derived IDs
      (`store.Session`) so an attacker holding one session
      cannot mutate another district's reservations — the
      handler checks `(reservation.created_by_session_id ==
      session.id)` in `internal/httpserver/`.
    - `IdempotencyKey` is required on reservation create;
      duplicate keys do not double-book.
  - Notes / follow-up:
    - JWT-style renegotiation does not exist (no JWT lib in
      `go.mod`); tokens are opaque DB-issued. Good.
    - Replay window = "until the session expires". A
      device-side clock-skew test belongs to P8 mobile
      runtime; not P7-prep.

### T3 — Compromised source account (CAP / GeoJSON / XML)

  - Surface: `internal/capfeed/` refresh path; CAP parse at
    `cap.go`; transport at `transport.go`.
  - Current controls:
    - `Fetcher` is an injected interface — production wiring
      happens at `cmd/sthira/main.go` time. The actual adapter
      lives outside this directory.
    - `Transport.Refresh` (`transport.go:91`) bounds attempts
      (`MaxAttempts: 3`), retries with exponential backoff,
      and prefers the **preserved cache** over falling back to
      a forged live response.
    - CAP parse is strict (already fuzz-tested in
      `capfeed/fuzz_test.go`).
  - Open follow-up:
    - `Transport.Refresh` uses the *Go default HTTP client*
      through the injected `Fetcher` interface — when the
      real adapter lands (P8/P11 source activation per O08),
      it MUST carry its own `CheckRedirect` and DNS-rebinding
      guards. See `findings.md` finding `F-CAPREDIRECT-01`.

### T4 — Replayed / expired / cancelled CAP package

  - Surface: same as T3.
  - Current controls: parse + lifecycle already enforce signed
    `expires` and superseded cancellation (see
    `capfeed/lifecycle_test.go:217`). Cancel/correction update
    before original is a P5 deliverable, already covered there.

### T5 — Exposed signing keys / signing bypass

  - Surface: `internal/offlinepkg/` signing primitives; the
    TrustStore in `internal/offlineclient/transport.go:412+`.
  - Current controls:
    - `verifyManifest` checks **both** checksum and signature
      (`transport.go:421`); a missing signature is a hard fail.
    - `verifySignature` is called with `jurisdiction` derived
      from the manifest — a same-digest cross-jurisdiction
      replay is rejected.
    - `offlinepkg.ValidateManifestStructure` validates shape
      before signature verification.
  - Open follow-up:
    - Key rotation: revocation tombstones are persisted but
      the TrustStore's freshness window is set per-build.
      Rotation cadence belongs to O08, not P7.

### T6 — Prompt injection / source-text abuse

  - Surface: `internal/httpserver/handlers.go` voice-commands
    pipeline + `internal/contracts/` typed middleware envelope.
  - Current controls:
    - Typed envelope at `contracts/pipeline.go` + voice-map
      system prompt enforces a JSON-only response; the
      orchestrator's validator is independent of the LLM
      parse (`voice-map-system-prompt.md` line 12-50).
    - User / source text is explicitly treated as DATA, not
      instructions, both at the prompt level and the validation
      layer.

### T7 — Audio decoding abuse

  - Surface: Worker 5 ASR; out of httpserver's reach.
  - Current controls: bounded audio (512 KiB compressed, 20 s
    decoded, 8–48 kHz sample rate) per
    `contracts/transcription.go:90`.

### T8 — GPU exhaustion / capacity race / queue flood

  - Surface: `internal/offlinequeue/` dispatcher.
  - Current controls:
    - `validateEndpoint` (`offlinequeue/dispatcher.go:144`)
      rejects non-`/api/` paths, scheme injection, and `..`.
    - `dispatcher` honours `BaseURL` only — explicit
      allowlist is the next open item.

### T9 — Malicious dependency / model artifact

  - Surface: `go.mod`, future model artifact downloads.
  - Current controls:
    - `go.sum` is committed (see `go.sum`); reproducible
      builds will lean on this.
    - `provenance` for model artifacts: future; not at P7-prep.
  - Open follow-up: govulncheck on every dependency and an SBOM
    gate at release. Both runners ship at P7-prep; install of
    the tooling is the P7 body.

### T10 — Privileged audit tampering

  - Surface: store + migrations.
  - Current controls: Migrations are SQL-up-only at
    `backend/migrations/`; the ChainAuditor (mentioned in
    `cmd/sthira/main.go` via `store.NewExpiryWorker`) records
    every operator action.

## Trust boundaries

| Edge | Incoming | Outgoing | Notes |
|------|----------|----------|-------|
| `citizen → /api/v3` | Bearer token | JSON response | `MaxBodyBytes` 1 MiB, depth 32, no duplicate keys, no unknown fields |
| `operator → /api/v3/operations/...` | Bearer issued by trusted IdP | JSON response | **`operatorVerifier == nil` → issuance 503**. No verifier is wired in current binary |
| `source → capfeed` | pulled HTTP | stored package | `Fetcher` injected, Attempts ≤ 3, backoff, prefer cache |
| `opkg → repo` | signed manifest | verify before publish | `verifyChecksum` + `verifySignature` |
| `offlinequeue → /api/v3` | `BaseURL` validated | `validateEndpoint` requires `/api/` prefix |

## Open / pending

  - O03 regional language matrix (linguistic / dialect review).
  - O08 source activation (NDMA/SDMA/DDMA, G02–G07 are
    candidate only at planning level).
  - O11 ISL + qualified reviewer.
  - O14 trusted operator IdP (issuance will still fail closed
    without it — verified in
    `operator_http_integration_test.go`).
  - O15 Strix (model/spend/scope). The P7-prep runner
    records the configuration without invoking paid inference.
  - P9 deferred: image SBOM per Trivy, full live gitleaks
    sweep, gosec full-tree (not just `internal/...`),
    full-module govulncheck.

## What's NOT in this document

This is **not** a description of a deployed architecture. There is
no production deployment at P7-prep. Anything beginning "in
production" is a missed edit; please raise it.

## Sources read for this model

  - `plan/assurance.md` (the canonical assurance map)
  - `plan/voice-map-system-prompt.md` (typed LLM contract)
  - `internal/httpserver/server.go`,
    `internal/httpserver/auth.go`,
    `internal/httpserver/operator_handlers.go`,
    `internal/httpserver/handlers.go`,
    `internal/httpserver/stay_handlers.go`
  - `internal/capfeed/cap.go`, `lifecycle.go`, `transport.go`
  - `internal/sourceact/activation.go`
  - `internal/offlineclient/transport.go`,
    `internal/offlinepkg/`, `internal/offlinequeue/dispatcher.go`
  - `internal/contracts/{transcription.go, tts.go,
     context.go, pipeline.go, errors.go, envelope.go,
     worker_health.go}`
  - `cmd/sthira/main.go`
