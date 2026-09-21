# Worker A — backend authority, orchestration and final integration

Send this entire file to one new chat. Execute the work; do not merely return a plan.

## Common instructions — read before editing

Repository: `/Users/apple/Documents/Projects/MonitoringZ`. Integration branch is uppercase `CLEAN`, never main. Reviewed implementation base is `b0b9772`. This is bounded correction work on P5/P6/P7, not a new phase or rewrite. Another worker runs concurrently; obey ownership below.

First inspect status, branches, worktrees and HEAD. Read GEMINI.md and applicable instructions, then plan/prd.md, rules.md, trd.md, architecture.md, source-register.md, decisions.md, open-decisions.md, phases.md and plan.md in that order. Read relevant sections selectively, not giant outputs. Then read plan/reviews/review-b0b9772.md, plan/next-action-single-worker.md and its handoff for historical requirements; this prompt governs the current assignment. Trace callers and the actual contract before changing code. Do not read weights or raw metrics.

Use an isolated sibling worktree and a `codex/` branch from the same captured CLEAN revision containing these prompts. Record base SHA; verify b0b9772 is its ancestor. Do not work concurrently in the primary checkout. Preserve unrelated files/worktrees/staged work. Do not edit any .txt file, push, reset, amend, rewrite history or touch main. Commit coherent verified stages with only your own files. Do not globally update shared ledgers while the other lane runs. Put your evidence in your assigned Markdown handoff.

User-selected models: IndicConformer-600M-Multi ASR, Sarvam-30B (2.4B active non-embedding) middle, Indic Parler-TTS. Do not revert to Qwen or infer that active parameters determine weight memory. Model outputs propose constrained display actions; they cannot authorize routes, capacity, arrivals or emergency calls. Preserve synthetic/operational isolation and fail-closed behavior. No paid calls, rented GPU or model-weight downloads without existing explicit authorization. Missing runtime assets are NOT_RUN evidence, not successful inference and not an excuse for unimplemented integration code.

Reuse existing helpers/contracts. Fix root causes, avoid parallel schemas/frameworks. For each reproduced defect, add a regression exercising the real affected boundary, demonstrate the intended failure before the fix when practical, then verify the fix. Fake inference is allowed to test transport, clearly labelled; fake replacement HTTP handlers cannot prove actual worker/public HTTP conformance. Do not invent authoritative data, policy or translation approval to make tests green.

Use unique owned disposable test databases and ports. Apply the repository migration runner through the current revision; configure STHIRA_TEST_DSN and STHIRA_TEST_ADMIN_DSN where required, and STHIRA_PYTHONPATH for process tests. Never truncate/drop shared databases. Cleanup only resources you created. Retain bounded JSON/Markdown summaries and failures, not millions of metric lines. Preserve command exit codes (no pipelines masking test failure). Record exact revision, environment, executed/failed/skipped/uncompiled tests and unavailable checks. After three unsuccessful attempts on the same issue without new evidence, report it and continue independent work.

## Worktree and ownership

Create/reuse only your own `codex/p567-backend-corrections` branch in a sibling worktree. Own backend/cmd/sthira, backend/internal/orchestration, store, httpserver, offlinedelivery, offlinequeue, shared contracts/OpenAPI and required migrations. Own final integration and final plan reconciliation. Do not edit the separate asrworker/middleworker/ttsworker/eval modules, speech adapters, loadmodel or security/load scripts during parallel work: Worker B owns those.

Your handoff: plan/worker-a-corrections-handoff.md. B's handoff: plan/worker-b-corrections-handoff.md. Shared public contracts stay frozen unless a demonstrated compatibility correction requires changing them; you own any such edit and must communicate the exact diff to B. Existing contracts, not B's test stubs, are the starting truth. Communicate via available agent messaging or concise handoff through the user; do not assume chats share memory.

## A1 — complete strict model boundary and safe final response

Read orchestration/orchestrator.go, http_client.go, registry.go, synthesize.go, the actual proposal JSON schema and contracts, and ProductionValidator callers.

Reproduce: Process currently accepts an empty returned request ID and RECENTER carrying TargetID; wrong schema and invalid zoom were already repaired, preserve them. Enforce exact correlation, mandatory fields, schema/data versions, supported status enums and per-action tagged shapes at raw decode and typed validation boundaries as appropriate. Do not broaden the schema to accept NEED_CLARIFICATION merely because implementation currently allows it. Reject missing versus zero values where contract distinguishes them, forbidden known fields, unknown/duplicate fields, trailing content and oversized responses. Detect max+1 overflow rather than decoding a potentially truncated io.LimitReader result. Ensure local/fake clients cannot bypass the mandatory semantic validator.

Regressions: missing/mismatched ID, missing required field, extra action-specific field, invalid status, overlimit/trailing JSON; preserve valid actions. Exercise Process with the production validator and HTTP decoding separately.

Reproduce: TTS hook withdraws source and fails after middle validation; Process currently returns OK with stale actions. Revalidate final scoped context on both successful and failed TTS paths before releasing actions, text or audio. If TTS alone fails and authoritative context is still valid, preserve usable validated text/actions with honest audio status. If source/version changes, remove stale actionable output and return the established fail-closed state. Cover withdrawal and version change during synthesis, failed synthesis with unchanged context, and cancellation. Apply the same lifecycle rules to direct synthesis.

## A2 — authoritative templates and destination facts

Built-in registry translations currently have SyntheticOnly=false and invented approval/version metadata; empty context TemplateKeys falls back to global registry. Remove implicit operational approval. Bind approved templates to jurisdiction, source/version, approval lifecycle and requested language using existing authority mechanisms. If evidence is absent, fail closed or use explicitly isolated synthetic fixtures; do not manufacture approval. Keep non-map/text alternatives within the same authority boundary.

Validate rendered argument facts, not just characters: facility/route/zone IDs must have the right entity type and membership; names, counts and other asserted values must come from the current scoped snapshot. argsForTemplate must not call every target a facility. Test wrong entity, cross-jurisdiction, version mismatch, withdrawn/unapproved template and invented count. Keep production blocked where language approval remains external.

Read store/scoped.go and its consumers. Remove unsupported alphabetical-as-nearest behavior and assumed party-size=1/one-day duration where they affect eligibility or claims. Use explicit known inputs and approved policy; request clarification or omit unsupported eligibility/ranking when inputs are absent. Do not invent routing/distance authority. Test that missing party size/duration cannot produce a falsely authoritative suitability claim. Preserve actual selection options rather than claiming a preferred destination without evidence.

## A3 — complete production publication lifecycle

Trace store/publisher.go, publication.go, source transitions, publication_adapter.go, offlinedelivery/cache.go and all Publish/Promote/Invalidate callers. Preserve the new single-transaction validation/write fix.

Wire the validating publisher, promotion and withdrawal into the actual trusted application lifecycle. Reuse operator/session/grant checks; do not expose a citizen publication route or add a permissive verifier. Remove/restrict unchecked Store bypasses from application paths. Check source/package/jurisdiction/signing authority/manifest-card/version relationships consistently. Revalidate current authorization, effective/expiry, supersession and attribution at promotion in the same transaction as state changes. Legacy rows lacking proven attribution must not silently become current.

Define one coherent current publication selection: staging a newer revision must not hide the current one; an old/superseded revision cannot be promoted; conflicting concurrent promotion cannot create contradictory current states. Preserve immutable bytes and identical retry semantics. Check manifest source→package versus card package→source lock order using controlled transactions; align shared lock order if necessary, without claiming an unobserved deadlock as reproduced.

Wire actual lifecycle invalidation to cached delivery across instances under an explicit consistency bound. Do not rely only on process-local manual Invalidate calls. Cover source withdrawal/quarantine/expiry/supersession and cached/in-flight fetches. Preserve signed revocation delivery and honest immutable-card/CDN validity semantics; never promise global instantaneous removal.

Acceptance: actual trusted lifecycle operation → persisted signed publication → real cached HTTP fetch → authority withdrawal → subsequent denial/revocation, without tests manually invalidating caches. Include current+new-staged selection, stale/unauthorized promotion, cross-jurisdiction/card-version mismatch, legacy unattributed rows, concurrent publish/promote conflict and idempotent same-content retry. Use real DB and signature checks; fixtures may be explicitly synthetic under isolated test configuration.

## A4 — health recovery and cross-module integration

Initial health probing was added. Add bounded refresh/recovery and shutdown behavior so workers unavailable at startup can later become ready, and later failures clear readiness. Reuse existing health machinery; avoid another independent monitor. Health language support must reflect actual loaded runtimes. Coordinate B's actual TTS/common worker protocol repair.

After B is integrated, prove actual ASR/middle/TTS server constructors communicate with the real HTTPWorkerClient and public pipeline handler. Fake only inference runtimes. Test healthy→unhealthy→recovered, required language unsupported, cancellation, bounded responses and valid returned audio. Use real contract types; no replacement httptest handler reproducing invented fields.

## A5 — final integration (serial, after B finishes)

Do your lane's tests and commit first. If B is still working, finish your handoff with exact pending integration steps; do not claim overall completion. Once B provides its ordered commits, inspect them and integrate only those commits into your isolated integration worktree (cherry-pick, no history rewrite). If edits overlap, resolve by contract/behavior and rerun affected checks. Do not merge into main or overwrite independent CLEAN changes.

Run formatting/vet/build and tests for backend plus all five separate Go modules (ASR, middle, TTS, eval, loadmodel). Use disposable DB for real backend flows. Explicitly run both TestCrossProcessLastSpace and TestCrashAfterCommitBeforeResponse with `-tags crashtest` and STHIRA_RUN_PROCESS_TESTS=1; no tests-to-run or SKIP is not a pass. Run affected Python adapter tests and B's bounded real-server evaluation/load checks. Verify worker IPC race tests, publication lifecycle and pipeline regressions, not just aggregate counts. Do not repeat expensive green suites without new changes.

Correct the three reviewed formatting failures in your lane; B owns the TTS one. Reconcile plan/prompt.md, decisions.md, open-decisions.md and active phase status with actual evidence. Preserve history; identify superseded claims. P5 cannot stay DONE while required publication engineering or external acceptance gates remain open. Keep P6/P7 IN_PROGRESS and gate B NOT_READY until their full evidence exists; do not close external decisions without named evidence. New scanner reports go to Markdown/JSON, never edit existing .txt evidence.

Report integrated commit IDs and leave the combined result committed on your integration branch. To place it on CLEAN, use only a fast-forward if CLEAN still matches the captured base and its checkout has no unrelated changes; otherwise leave the integrated branch intact and report divergence. This authorization is limited to integrating these two lanes, never main or a push.

## Handoff / stop

For A1–A4 and B's rows provide PASS/FAIL/BLOCKED/NOT_RUN, exact boundary tested, revision and retained evidence. Separate implemented-but-unbenchmarked inference from real model execution. Report unresolved defects and external dependencies individually. Stop before P8 and do not declare production readiness. No claim that all defects are impossible merely because the bounded matrix passes.
