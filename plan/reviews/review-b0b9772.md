# Completion review at b0b9772

Reviewed 2026-09-21 on uppercase CLEAN. Verdict: corrections required; do not advance to P8 or declare gate B ready. This is a review, not product implementation. The two accompanying worker prompts supersede the outstanding execution instructions in next-action-single-worker.md; historical evidence remains valid for its tested revision.

## Revision and evidence

Inspected the ten commits after c0a312d: 9751d7a, 95e33a1, 6c103a1, 8543a0a, a5b3d87, 919c3f4, 760e5ef, 85447dc, e82e13a, b0b9772, tracing changed paths and callers. No raw metric dumps or model weights were read.

Existing suites passed on this revision: backend 721 test/subtest pass events; separate ASR 77, middle 61, TTS 67, evaluation 42, loadmodel 5. These are pass events, not distinct acceptance requirements. Backend used a uniquely owned PostgreSQL/PostGIS database migrated through revision 7 with STHIRA_TEST_DSN and STHIRA_TEST_ADMIN_DSN. The two explicitly enabled crashtest process tests also passed. Owned database and temporary probes were removed. Real model/GPU accuracy and throughput were not measured.

Confirmed progress: raw metrics and compiled load binaries removed from tracked files; publication validation and writes share transactions; initial worker probes wired; scoped speech requests and returned audio added; proposal validation and strict JSON handling improved; subprocess transport exists. Preserve these changes.

## Remaining findings

| ID | Evidence and consequence | Owner |
|---|---|---|
| A1 | A probe through Orchestrator.Process and ProductionValidator accepted an empty proposal request ID and RECENTER with an illegal TargetID: state OK, no error. validateProposal only compares nonempty IDs and permits an extra status. Strict per-action required/forbidden fields remain unenforced. | A |
| A2 | A probe withdrew context during failing TTS. Process returned OK with old actions because final revalidation only follows successful TTS. | A |
| A3 | registry.go still marks built-in translations operational without approval evidence; empty template keys fall back to global registry. Arguments are syntactically checked but not all are bound to authoritative facts. scoped.go still sorts facilities alphabetically and assumes party size 1/one-day stay. | A |
| A4 | Startup probes do not establish health recovery after startup failure. Actual worker-to-backend conformance is not established by replacement HTTP handlers. | A+B |
| A5 | NewPublisher/promote/invalidate have no complete trusted production lifecycle caller. Raw publication persistence bypasses checks. Promotion lacks full current-authority/expiry/supersession validation. Highest staged manifest may shadow current; lifecycle database updates do not establish cache invalidation. Nullable legacy attribution needs fail-closed handling. | A |
| B1 | A real ffmpeg-generated one-second Ogg/Opus recording fails DecodeAudio with `decode: wav data chunk truncated`: ffmpeg pipe WAV uses unknown chunk length. | B |
| B2 | Python ASR/TTS adapters have no implemented load/inference branch and always refuse. Default TTS module name is wrong. This is unfinished engineering, separate from unavailable hardware. | B |
| B3 | Subprocess adapters release the exchange lock while goroutines scan shared stdout; cancellation leaves readers alive. Concurrent calls can cross-consume responses. TTS lacks correlation; its 256 KiB line cap is below allowed 12-second audio payloads. | B |
| B4 | Evaluation HTTP provider uses flat/wrong shapes and omits required jurisdiction; tests mimic the invented contract. Actual public handlers use data envelopes and PipelineRequest/PipelineResponse. | B |
| B5 | Sarvam thinking-template configuration is referenced by tests but not the serving/request path. TTS health emits internal fields instead of the shared worker health contract. | B |
| B6 | k6 smoke lacks a successful business flow; checks have no failure threshold and status<500 accepts status 0. Script resource ownership/startup handling needs repair. Security evidence reports FAIL, scanners NOT_RUN; three files remain unformatted. SPDX root metadata has a fictitious proxy URL. | B |

A5 also needs a focused lock-order check: manifest publication acquires source then package while card publication does the reverse. This is a deadlock risk from inspection, not a reproduced deadlock. Test it before claiming a failure or repair.

Formatting failures: backend/internal/offlinequeue/worker.go, offlinequeue/worker_regression_test.go, ttsworker/worker.go. Existing plan/evidence/security-checks.txt reports failure; preserve it under the no-.txt-edit rule and put new evidence in Markdown or JSON.

## Acceptance boundary and launch

Send all of plan/next-action-worker-a.md to the first new chat and all of plan/next-action-worker-b.md to the second. Both include common instructions. Do not send a separate coordinator prompt. They work concurrently in separate worktrees; A performs final integration only after B supplies verified commits. No third worker is needed.

P5 publication engineering remains open. P6/P7 remain IN_PROGRESS and gate B NOT_READY. O01/O03/O04/O05/O06/O07/O11/O14 and other recorded external gates must retain their evidence-based status; do not hide internal defects behind them. Passing these corrections will not itself establish real model quality, government authorization or production readiness.

The huge raw metrics are already removed from tracked files. Do not delete more evidence or rewrite Git history. Earlier commits may still contain the old blobs; removing history is a separate authorized task.
