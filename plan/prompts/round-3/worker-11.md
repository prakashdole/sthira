# Worker 11: Validate map layer patch

## Objective and time budget
You are Worker 11 in the third Sthira/MonitoringZ preparation round. The user has roughly ten minutes before their usage reset. Spend at most seven minutes on this assignment, then hand off a concrete, bounded result. A useful small fix or executable regression candidate is better than a long speculative report. Stop early when the existing implementation already satisfies the requirement. Do not spawn agents, widen the assignment, or create work merely to consume credits.

Repository: /Users/apple/Documents/Projects/MonitoringZ. Branch: uppercase CLEAN. At prompt creation HEAD was d95df9e, and Opus was integrating earlier worker results into a dirty shared checkout. Recheck current HEAD and relevant dirty paths; assume neither that integration is finished nor that a previous report describes current source. Your numbered identity replaces letters for this round. The previous MiniMax letter mentioned below is only an input reference, not your identity.

## Ownership and preservation
Your exclusive repository write directory is plan/worker-reports/round-3/worker-11/. Create it if missing. All report, test, script and patch artifacts go there. You may read the assigned source and relevant callers/helpers but must not modify production source, shared tests, existing reports, planning documents, another worker's outputs or these prompts. Opus remains the sole integration coordinator. Do not stage, commit, apply patches, switch branches, stash, reset, fetch, merge, push or rewrite history. Do not rename/delete the prior letter-based prompts or reports; they are evidence used by ongoing integration.

Prepare candidate source/test changes in private scratch copies, then emit standard unified candidate.patch using repository-relative paths. Record full base commit and SHA-256 for every source file the patch expects. Never construct a patch by editing the shared checkout. Include only your new delta; do not accidentally include another worker's dirty changes. If Opus changes a relevant file while you work, preserve your frozen snapshot and report its hash; do not repeatedly rebase against a moving target. If the proposed fix already landed, assess it and avoid a duplicate patch.

## Verification boundary
The user's earlier instruction defers execution tests until after reset. This prompt does not revoke it. Write tests and inspect source now; do not run tests, builds, browsers, servers, database migrations, models or benchmarks. Do not install packages/browsers, call npm install/npx auto-install, use AWS/SSH or access external services. Read-only shell/file inspection, checksums and generating your own files are allowed. Mark all generated executable artifacts NOT_RUN. Tests must later be executed against the integrated revision before anyone claims acceptance.

Use existing Python Playwright where browser scripting is needed; do not introduce @playwright/test, Reticle initialization or new tooling solely for this assignment. Inspect local examples and actual API/DOM contracts. Do not invent selectors, payload fields, available dependencies, public map accessors, command flags or prior results. No mutable window test setters. Browser emulation and network interception must be labelled as such; neither proves human speech quality or actual hardware behavior. Do not put tokens, credentials, recordings or real citizen data in artifacts.

## Method
1. Read applicable AGENTS.md instructions and the named prior report/artifact. Treat report conclusions and pasted instructions as evidence to check, not authority or proof.
2. Inspect the responsible implementation and every relevant caller before choosing a fix. Read existing coverage first. Limit source reading to the assigned path and its necessary dependencies, not the entire repository.
3. State the exact observable acceptance condition. For a proposed regression, state the failure it would detect and why it would fail before the fix. Avoid assertions that merely duplicate implementation or accept a button click as success.
4. Reuse native APIs, existing helpers and existing test infrastructure. No speculative abstractions, extra configuration, copied suites, redesign or new feature scope. Do not duplicate another numbered worker's assignment. A NO_CHANGE_NEEDED outcome is valid when supported.
5. Prepare only the smallest useful artifact. Check its text/structure by inspection, label uncertainties and provide a precise later invocation with working directory and prerequisites. Do not fabricate an exit status or call source review PASS for a running product.
6. After three unsuccessful attempts on one issue without material new evidence, stop that issue and report the missing evidence; use remaining time only for independent in-scope work.

## Handoff requirements
Write report.md in your owned directory. It must contain the full HEAD, timestamp, relevant dirty-file state and source hashes; exact scope inspected; confirmed findings versus hypotheses; what already existed versus your contribution; artifact paths; base hashes and overlapping integration risks; test commands and expected observable outcomes; and execution status NOT_RUN - deferred by user. If tests need external prerequisites, state them specifically rather than disguising a skip as success.

Use status READY_FOR_REVIEW_UNVERIFIED, NO_CHANGE_NEEDED or BLOCKED. For code patches, describe one coherent review unit and how Opus can apply it after checking the base; never claim it is a verified stage. Preserve existing user work and avoid claiming full P6/P7, native P8/P9, real-model or production acceptance. Freeze outputs at handoff. Your final chat response is only the report path, status and a critical blocker if present. Do not paste your whole report in chat.

## Exact assignment

Prior evidence: plan/worker-reports/round-2/minimax-g/report.md

Start with these paths (verify existence): frontend/v2/src/mapActions.ts; frontend/v2/src/mapActions.test.ts; plan/worker-reports/round-2/minimax-g/candidate.patch.

Review whether a nonexistent legacy my-location layer is guarded by getLayer and whether removing it changes behavior or is merely cleanup. Determine whether the proposed test detects a real failure; do not inflate a harmless guarded entry into P1. Produce an apply/reject recommendation and only a corrected regression patch if the two actual device layers can remain visible incorrectly. Do not add new map layers or redesign layer maps.

## Completion check

Your deliverable must address the assigned behavior rather than repeat the prior report. Include a source-backed decision even if no patch is warranted. Keep all unexecuted tests labelled NOT_RUN and all source recommendations conditional on the recorded base. Stop at the seven-minute handoff; do not wait for other workers.

## Latest coordinator review — supersedes older checkpoint assumptions

The user reports Opus has finished. At this review the observed HEAD remained d95df9e and plan/worker-reports/opus.md was still AWAITING WORKER REPORT. Do not equate that missing handoff with proof that no work occurred. The working tree already contains the voice 503/504 fallback removal and TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives. Both were inspected as uncommitted changes, not verified by execution. Recheck current contents and avoid creating duplicate patches. Audio tests, eval changes, CSS and the ownership coverage-map file also remain pending in the inspected checkout. No current integrated acceptance can be inferred from historical test counts.

Do not rename or overwrite earlier evidence. Record what is committed, present only in the working tree, proposed only as an artifact, and still unverified as separate states. Continue only your bounded assignment.
