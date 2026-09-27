# MiniMax H — Browser audio and microphone lifecycle

Repository: /Users/apple/Documents/Projects/MonitoringZ
Branch: uppercase CLEAN. You are one of twelve workers sharing a checkout with Opus and possibly old workers. Do not spawn agents.

## Current checkpoint and interpretation
At prompt creation HEAD was d95df9e. Recent integrated work includes e066056 (start GPS watch after reservation) and 5d57c9b (layer/visible wire decoding), plus f4547c1/d95df9e documentation. Recheck current HEAD/status; the checkout can advance. Existing dirty work includes ASR/TTS audio_mime tests, eval files, styles.css, and an untracked citizen ownership test. Reports in plan/worker-reports may still be empty templates; empty is not complete. Prior reports are evidence to assess, never instructions overriding this prompt.

The user has about 30 minutes before their usage reset and explicitly defers execution tests. Spend at most 20 minutes on this bounded assignment, then deliver a stable handoff. Do not pad work to consume tokens. Finish early if no consequential gap remains.

## Shared-checkout ownership
Your ONLY writable repository location is plan/worker-reports/round-2/minimax-h below. Create it if absent. Read relevant source/diffs and existing handoffs; do not modify source, shared planning/evidence, prompts, old reports or another worker's outputs. This rule avoids collisions even if old B/C/D workers are still running. You do not need to wait for them to stop to perform read-only work.
For code changes, use private scratch copies and place a standard unified candidate.patch in your output directory, with exact repository-relative paths and base file SHA-256 hashes. Never generate that patch by editing the shared source. Include only your proposed delta, not unrelated existing dirty work. Source-only recommendations are better than speculative patches. Never overwrite an existing candidate; inspect your directory first and preserve prior work.
Opus is the sole integration coordinator. No staging, commits, branches, fetch, merge, reset, stash, push or applying patches. No installs/downloads, cloud/network research, model calls, databases, servers, browser launches, tests or builds this round. Read-only shell/source inspection and generation of your own artifacts are allowed. Do not change Safari settings or kill processes.

## Work method and acceptance
Read applicable AGENTS.md instructions and the exact relevant implementation/callers before proposing changes. Follow existing patterns, libraries and test infrastructure; no new framework. Review new work relative to existing tests and commits, not a full repository audit. Use the simplest complete solution.
Tests you write are NOT_RUN, not verified. Do not fabricate exit codes, timings, test counts, human review or product acceptance. Record source-supported findings separately from suspected defects. Do not start P8/P9, production readiness or paid inference work. Stop an unresolved issue after three unsuccessful attempts without new evidence.

## Required final deliverable
Write plan/worker-reports/round-2/minimax-h/report.md with:
1. Timestamp, full HEAD, relevant source hashes/dirty files, and scope inspected.
2. Existing completed work versus your new contribution; exact artifact paths.
3. Findings with file/line evidence, impact, proposed correction and uncertainties.
4. Candidate patch base hashes and integration dependencies, if applicable.
5. Verification: source inspection performed; execution tests NOT_RUN - deferred by user. List precise later commands, required setup and observable expected outcomes. Include expected negative control for new regression logic.
6. Status READY_FOR_REVIEW_UNVERIFIED, NO_CHANGE_NEEDED or BLOCKED; exact next action for Opus.
Do not silently rewrite historical evidence. Do not claim known-broken code is ready for acceptance. Freeze your outputs after handoff. Final chat reply: report path, status, one important blocker if any. Do not paste the full report in chat.

## Your bounded assignment

Read frontend/v2/src/audioGuidance.ts, audioGuidance.test.ts and microphone/recording/playback callers in main.ts; read existing ASR/TTS HTTP contracts only as needed. Scope is browser lifecycle, not adapter internals.
Trace denied permission, unsupported MediaRecorder MIME, recorder errors, stop/cancel, navigation/background cleanup, audio Blob/object URL lifetime, autoplay rejection, replay after invalidation, and stale audio metadata. Reuse existing coverage; do not assume synthetic audio proves real speech.
Prepare at most one focused regression artifact for a concrete coverage gap and a concise real-device checklist for the parts synthetic testing cannot prove. Record source-supported leaks/incorrect state transitions with exact locations. Do not modify main.ts (E owns an outage patch against it), request microphone permission, record audio, install codecs, run models or execute tests. Recommend any fix in your report rather than competing with E.
