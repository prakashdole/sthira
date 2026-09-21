# Worker D — bounded authorization & idempotent-replay audit

Date: 2026-09-21. Inspected code revision: `4095400` (CLEAN tip at capture).
Repair-branch cross-check: `d8573b8` (codex/p567-integration-repair) — all
findings below still present at that SHA (verified by diff and direct
inspection); none superseded. No product code modified by this audit.
Scope: operator/citizen identity, live grants, resource-scoped idempotent
replay, protected stay mutations. This report is evidence, not phase approval;
O14 remains open (no real IdP was or can be exercised through these seams).

## Environment and honesty labels

- Real HTTP handlers (`httptest` over the production mux) against real
  PostgreSQL 18 + PostGIS, disposable uniquely owned DBs
  `authreplay_wd_d1` (migrated 0001→0007) and `authreplay_wd_mig` (populated
  pre-0005 data, then actual migration sequence applied), DSN explicitly set,
  failures treated as failures. Both DBs removed after evidence capture.
- Identities were issued through the test-only `syntheticVerifier` seam
  (`WithOperatorVerifier`, never wired by the production binary). That proves
  the grant/replay/jurisdiction logic behind issuance; it does not certify any
  live IdP/MFA integration.
- Labels: VERIFIED = reproduced behavior with a runnable check; SOURCE =
  source inspection only; FAILING = confirmed defect reproduced.

## Confirmed defects

### D1 (medium) — quarantine skips the jurisdiction check when the source has no live authorization

- File/function: `backend/internal/httpserver/operator_handlers.go`,
  `handleSourceQuarantine` (guard at line 364):
  `if has && (op.Jurisdiction == nil || jur != *op.Jurisdiction)` — when
  `AuthorizationJurisdiction` reports `has=false` (authorization expired, or
  never recorded), the jurisdiction check is skipped and the quarantine
  commits.
- Trigger: operator holding a live grant in jurisdiction X quarantines a
  source in jurisdiction Y whose `source_authorizations` row is expired or
  absent. Sibling `handleSourceTransition` denies the identical condition with
  409 `errNoAuthorization`; quarantine acts.
- Observable impact (reproduced): HTTP 200, source state becomes QUARANTINED
  — a **terminal** state (`ErrQuarantineTerminal` blocks any later transition,
  including re-OPERATIONAL after the Y authorization is renewed, so the
  foreign action permanently disables the source's evidence path) — and
  `Transition` additionally flags all of the source's published manifests and
  cards quarantined. Cross-jurisdiction mutation + evidence withdrawal by an
  operator with zero authority over the target; no audit reason gate, no
  confirmation from the owning jurisdiction.
- Repro: `auth-replay-probes/quarantine-no-authz-cross-jurisdiction.go.txt`
  probes A/A2; output in `auth-replay-probes/results.txt`.
- Narrow suggested fix: mirror the transitions handler — deny when `!has`
  (return `errNoAuthorization` before disclosure or mutation). If the intent
  was "quarantine does not require a live authorization," it must still bind
  to the source's last-recorded authorization jurisdiction, which is a policy
  decision for the integration worker, not a silent fail-open.

### D2 (low–medium, violates R13) — reservation idempotency payload hash omits party_size and snapshot_version

- File/function: `backend/internal/httpserver/stay_handlers.go`,
  `reservationPayloadHash` (lines 224–227): hashes session/facility/package/
  route/dates/key but **not** `PartySize` or `SnapshotVersion`.
- Trigger: same session, same `idempotency_key`, changed `party_size` (2→5) or
  changed `snapshot_version`. R13: "changed payloads conflict."
- Observable impact (reproduced): the changed-payload retry returns 200 with
  the original stay's stored result instead of 409
  `IDEMPOTENCY_CONFLICT`. Capacity is NOT mutated (held stayed 2, single stay),
  so this is not an overbooking bug; it is a silent wrong-acknowledgment: a
  client that retries a corrected party size is told its 5-person party is
  booked while the server holds 2 — a false "reserved" claim of exactly the
  kind R13/R24 forbid. Stay-event and operator paths hash their full
  discriminating payloads; reservation.create is the only field-subset hash.
  (Also: the `|`-joined hash is ambiguity-prone, but every field is
  server-visible and the scope is the actor's own session, so it is only
  reachable by the actor against themselves; noted, not filed.)
- Repro: probe B in `auth-replay-probes/quarantine-no-authz-cross-jurisdiction.go.txt`; output in `results.txt`.
- Narrow suggested fix: include `PartySize` and `SnapshotVersion` in the hash,
  or hash the raw body like the operator handlers do. Add a regression: same
  key + changed party size must be 409 with unchanged capacity.

## Invariant matrix (per protected operation)

| # | Row (identity / grant / replay / mutation boundary) | Result | Evidence |
|---|---|---|---|
| 1 | Issuance fails closed with no verifier (production default) | VERIFIED | `TestOperatorIssuanceFailsClosedNoVerifier` (green on rev-7 DB); grep: `WithOperatorVerifier` has no non-test caller |
| 2 | Body `mfa_verified` flag / requested jurisdiction cannot create operator authority | VERIFIED | `TestOperatorIssuanceRejectsBodyMFA`; handler never reads a body; jurisdiction derived only from `operator_grants` (`TestOperatorIssuanceDerivesJurisdiction`) |
| 3 | Persisted identity/grant binding (subject+grant stored, re-checked, not role string) | VERIFIED | `sessions.operator_subject/operator_grant_id`; `TestOperatorAuditResolvesToVerifiedIdentity`; probe C read the session row via `store.Authenticate` |
| 4 | Citizen token on operator route denied; non-MFA / unbound operator denied | VERIFIED | `TestOperatorSessionRequiresMFAOnUse`, `TestCitizenSessionUnaffectedByOperatorGrant`, `TestLegacyUnboundOperatorSessionCannotAct` |
| 5 | Grant revoked after issuance → fresh op AND replay of completed key denied, no disclosure, no mutation | VERIFIED | `TestOperatorGrantRevokedDeniesOperationAndReplay` (403 both; state unchanged) |
| 6 | Grant expiry honored mid-session | VERIFIED | `TestOperatorGrantExpiredDeniesSession` (existing test uses a 3s sleep; acceptable, existing) |
| 7 | Deterministic two-connection grant-revocation race: FOR UPDATE on the grant row serializes revocation after an in-flight op; post-commit revoke then denies | VERIFIED | probe C: `pg_blocking_pids` observation, production `revalidateOperatorGrant`/`LiveGrantForUpdate` path, READ COMMITTED (`InTx` uses default isolation) |
| 8 | Current grant + target jurisdiction checked before replay disclosure — transitions & stay corrections | VERIFIED | source order `Begin → revalidate → jur → disclosure`; `TestOperatorCorrectionCrossJurisdictionDenied` asserts held unchanged on 403 |
| 9 | Same, quarantine when source has NO live authorization | FAILING | D1 (repro A/A2: 200 + terminal QUARANTINED by foreign operator) |
| 10 | Idempotency key bound to target resource on every operator path (not just the original handler) | VERIFIED (source) | all three opNames embed the path ID: `source.transition:{id}`, `source.quarantine:{id}`, `stay.correction:{id}`; `TestOperatorIdempotencyTargetBinding` |
| 11 | Same key across different actors → independent scopes, no cross-actor response disclosure | VERIFIED | probe E2 (two operators, same key: each got its own result, 2 audit events, own attribution; no stored-result leak); `TestHTTPCrossSessionDenial` |
| 12 | Same key + changed payload → 409 (operator paths) | VERIFIED | full-body hash; `TestOperatorIdempotencyPayloadConflict` |
| 13 | Same key + changed payload → 409 (citizen reservation.create) | FAILING | D2 (probe B: 200 replay on party_size 2→5 and snapshot change; capacity intact) |
| 14 | Same key + changed payload (citizen stay events) | VERIFIED (source) | event hash includes stayID, type, new end/facility/route, snapshot, key |
| 15 | Citizen committed-reservation replay after source withdrawal is intentional behavior (do not fix) | VERIFIED (deliberate) | replay short-circuits before `RevalidateReservationContext`; a *new* (different-key) request is still denied (`TestHTTPQuarantineDeniesNewReservation`, `TestHTTPRevokedAuthorizationDeniesReservation`) |
| 16 | Legacy self-attested operator sessions revoked by the actual migration sequence while citizens are preserved; revocation not resurrected by 0006/0007; revoked legacy token gets 401 over real HTTP | VERIFIED | probe D (populated `authreplay_wd_mig` DB, migrations 0001→0007 in order) + probe E1 |
| 17 | Denied scoped mutations leave no partial reservation/stay/capacity/audit change | VERIFIED | `TestOperatorCrossJurisdictionDenied`, `TestOperatorCorrectionCrossJurisdictionDenied`, `TestHTTPCapacityConflictNoSubstitution`, `TestHTTPFailedTransferPreservesOriginal` (green on rev-7 DB) |
| 18 | Successful retry yields exactly one mutation + one audit event with correct actor attribution | VERIFIED | `TestHTTPDuplicateConfirmationReplay` (held=1, same stay), `TestOperatorStayCorrectionAudit` (actor=operator session), `TestOperatorQuarantine` (replay adds no transition) |
| 19 | Audit chain append serialization under concurrency (advisory lock) | NOT_RUN | source inspection only (`pg_advisory_xact_lock(7301)`); concurrent append stress is outside this lane's bound; other lanes exercised audit integration |
| 20 | Live IdP/MFA verifier integration | NOT_RUN (BLOCKED_EXTERNAL) | O14; production binary wires no verifier — that fail-closed state is itself row 1 |

## Observations (not filed as defects)

- `withSession` (citizen routes) accepts a live OPERATOR session: an operator
  token can create/read/modify reservations under its own session owner checks.
  No privilege escalation is demonstrated (owner checks are per-session), but
  whether operators may consume citizen capacity at all is a product/policy
  question for the ledger, not a code fix here.
- `store.RevokeSession` has no production caller: session-level emergency
  logout is only achieved by migration SQL or grant revocation. Grant
  revocation covers authority on every protected op, so the gap is
  defense-in-depth only.
- Idempotency scope is per-session, so the same verified subject operating two
  concurrent sessions can double-apply an operation with the same key (probe
  E2). This matches the documented (scope, operation, key) design; flag only if
  per-subject dedupe is ever required.
- Begin-order note: an idempotency conflict (409) can be surfaced by a
  revoked-grant operator on their own previously-committed key because
  `idem.Begin` runs before grant revalidation. It discloses only the caller's
  own key state; harmless, listed for completeness.

## Checks run (compact)

- Baseline existing suites on migrated rev-7 disposable DB:
  `go test ./internal/httpserver -run 'TestOperator|TestLegacy|TestCitizenSessionUnaffected'` → ok (all pass).
  Replay/denial subset of stay tests → all pass. See `results.txt`.
- Probes A/A2/B → FAIL against secure expectations, i.e. defects D1/D2
  reproduced deterministically; probe C/E1/E2 and populated-migration probe D
  → secure behavior observed.
- Temporary probe file removed from `backend/`; disposable DBs dropped; no
  product file modified; nothing pushed. Repro code preserved read-only under
  `plan/evidence/auth-replay-probes/` (`.go.txt` so it never compiles into the tree).

Stop condition met: assigned scope exercised; two concrete defects delivered
to the integration worker (see handoff), matrix rows closed or labeled
NOT_RUN; no phase claims, no gate closures.
