# Parallel worker D — bounded authorization and replay audit

Execute this entire prompt in a second new chat while the integration worker repairs code. Find concrete trust-boundary failures in existing P3/P4 paths, not another broad static-analysis report. Do not modify product code.

## Shared boundaries

Repository: /Users/apple/Documents/Projects/MonitoringZ. Integration target: uppercase CLEAN, never main. An integration worker is executing plan/next-action-integration-repair.md; do not duplicate its implementation or modify its checkout. This task produces actionable evidence, not product changes or phase approval.

Inspect status, branch tips and worktrees before starting. Read applicable instructions, GEMINI.md, relevant plan/rules.md, architecture.md, decisions.md, open-decisions.md, and plan/reviews/review-e0babda-3e6daac.md. Preserve unrelated changes and all .txt files. Do not read giant raw metrics, model weights or unrelated private files. Do not push, reset, rewrite history, merge branches or change CLEAN. No paid API, GPU rental, weight downloads, production access or messages to outside parties.

Create your own sibling worktree and codex/ branch from current CLEAN. Record the exact inspected code revision for every finding. You may inspect the repair branch read-only, but do not treat changing code as a stable baseline; capture its SHA first. Temporary experiments belong in your own disposable directory/worktree. Commit only your assigned Markdown evidence and small non-sensitive reproducer artifacts, never modified product files or generated bulk data. Use separate uniquely owned DBs/ports and remove only resources you created.

Communicate useful findings to the integration worker early using available task messaging; if unavailable, put the exact finding and file path in your handoff for the user. Do not wait until all research is done to flag a confirmed blocker. Coordinate observations without assigning yourself the other worker's code. Passing mocks do not prove real boundaries; distinguish source inspection, reproduced failures, verified behavior and NOT_RUN. After three unsuccessful attempts without new evidence, stop that issue and record what is needed.

## Ownership and objective

Use branch codex/auth-replay-evidence. Own only plan/evidence/auth-replay-review.md and small source-only reproduction files under plan/evidence/auth-replay-probes/ if useful. Do not edit server/store code, migrations, shared tests, security scripts or shared ledgers. The integration worker owns fixes and acceptance.

Scope: operator/citizen identity, live grants, resource-scoped idempotent replay and protected stay mutations. Exclude inference, publication/cache, audio decoding, model schemas and load/scanner scripting; those are already assigned. Avoid re-auditing unrelated Python legacy routes.

## Read and define the invariant matrix

Trace backend/internal/httpserver/operator.go, operator_handlers.go, stay_handlers.go; backend/internal/store/session.go, operatorgrant.go, idempotency.go, stay.go, audit.go; their current migrations and focused integration tests. Read only callers relevant to these boundaries.

For each protected operation record caller identity, jurisdiction/resource scope, current-grant check, transaction/lock location, idempotency lookup key, replay response disclosure and mutation/audit behavior. Separate operator authorization revalidation from the deliberate citizen committed-reservation replay semantics: an already committed citizen reservation may return its stored result after source withdrawal. Do not flag intentional accepted behavior as a bug.

## Targeted checks

1. Operator session issuance remains fail-closed without trusted verifier; citizen credentials, body MFA flags and requested jurisdiction cannot create operator authority. Verify persisted identity/grant binding rather than trusting role strings.

2. Revoke/change an operator grant after session issuance, then attempt an operation and a replay. Verify current grant/jurisdiction/privilege is checked before protected response disclosure or mutation. Exercise two connections and deterministic synchronization for a grant-revocation race where transaction semantics matter.

3. Reuse idempotency keys across different actors, jurisdictions, target resources and payloads. Demonstrate correct replay for the original committed request and denial/conflict for incompatible requests, without exposing another actor's response or changing capacity. Check whether operation names actually bind target IDs for every protected sibling path, not just the original fixed handler.

4. Verify legacy self-attested operator sessions remain revoked after migrations while citizen sessions are preserved. Use populated disposable data and the actual migration sequence; never edit migration history to make a test pass.

5. For the scoped mutation attempts above, assert no partial reservation/stay/capacity/audit changes on denial. Check successful retries yield one mutation/audit event and correct actor attribution. Do not run an unrelated full capacity/load campaign.

## Execution discipline

Start with existing tests and the smallest missing behavior probe. Use actual public handlers and real migrated PostgreSQL/PostGIS where persistence/locking is the boundary. Fake verifier fixtures are permitted solely to issue controlled identities through the trusted test seam; they do not certify live IdP integration. Test connection failures with an explicitly configured DSN are failures, not skips.

Use a uniquely owned disposable DB and distinct fixture identities. Never read application secrets or probe production/external endpoints. Do not rely on sleeps for race ordering; use existing lock-observation/barrier helpers. Bound each test/process. Keep temporary test additions only in your worktree and remove them after extracting a small portable reproducer; do not commit edits inside backend/. Capture exact SHA, invocation, result and relevant compact output.

If a likely defect is fixed on the integration branch since your captured revision, inspect that specific diff and report it as superseded/needs-retest rather than filing a duplicate. Do not cherry-pick moving implementation into your audit worktree without recording a new stable baseline. No findings is an acceptable result; do not invent work to consume credits.

## Deliverable and stop

Report each matrix row as verified, failed or NOT_RUN. For confirmed defects include severity, concrete trigger, affected file/function, observable impact, minimal reproducer and narrow suggested fix; send them promptly to the integration worker. Label unproven suspicions separately. Report successful checks and limits without claiming complete security or closing O14.

Commit the bounded report/reproducers, provide SHA and path, and stop. The integration worker decides and implements repairs. No full-repository security sweep, feature work, phase advancement or production attack testing.
