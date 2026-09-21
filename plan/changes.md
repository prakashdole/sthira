# Change and evidence record

## 2026-09-21 — integration repair: Stage 1 closed on `codex/p567-integration-repair`

Continued the A/B integration on the existing integration worktree. Re-checked
ground truth first: the committed tree built but **did not** pass its own Stage 1
real-HTTP gate (`TestVoiceProcess_RealHTTP_...` returned 400 `UNKNOWN_FIELD
jurisdiction`, then 422 `speech_key not in approved set`), and the in-flight
`a1_regression_test.go` was left syntactically broken. Fixed the breakage and
closed Stage 1 with real boundary evidence (commit `b3ddc9a`):

- Public citizen bytes are strictly decoded as `PipelineRequest`; model bytes are
  strictly validated (presence/enums/tagged variants, explicit-empty forbidden
  fields) via `model_strict.go` at `HTTPWorkerClient.Propose`, on distinct limits.
- `decodeRIFFWAV` rejects an unknown-length `fmt` chunk and bounds every chunk
  advance with `uint64` checks; added no-panic regressions and a bounded
  `FuzzDecodeWAVChunkSizes` (~360k execs, clean).
- Added persisted, jurisdiction/source-bound template approval
  (`approved_translations`, migration `0008`) that populates `ScopedContext.TemplateKeys`;
  empty approval still fails closed. The real-HTTP happy path now returns 200 with
  audio through `ProductionValidator` and the real handlers.
- All six Go modules build, vet, gofmt-clean; backend + asrworker + ttsworker +
  middleworker + eval + loadmodel test suites are green (default, non-`-race`).

Stages 2–5 remain **open**. Verified status is recorded in
[integration-repair-handoff.md](integration-repair-handoff.md). Notably A3 DB
tests still `t.Skip`/`t.Skipf` (a pass bypass) and reference a nonexistent
`artifacts` table; the observer/in-memory-bus publication path, real-adapter
READY semantics, IPC `-race` boundedness, actual-handler eval conformance and the
P7 smoke/security harnesses are unfinished. Gate B stays **NOT_READY**; P8 not
started. `CLEAN` (`4095400`) cannot fast-forward to this branch (diverged at
`b63e469`).

## 2026-09-19 — production-preparation planning pivot

All sixteen existing Markdown documents under `plan/` are rewritten for immediate evacuation and 7–30 day temporary relocation. Added dedicated stack, cleanup and assurance documents. Added the Go migration sequence, both-platform mobile gate, one-million-total-user workload model, regional demo/language matrix, map/offline architecture, bounded middle-model contract and phase prompts.

User answers incorporated: Android and iPhone at launch; one million total users; regional languages across 10–15 demo states; route authority remains open. Demo zones will be supplied later. No Go code, runtime configuration, datasets or legacy files are changed/deleted by this planning pass.

Removed obsolete hackathon execution instructions and completed Python tasks from the active backlog. Preserved their useful evidence below and in Git. New Go phase statuses are independent and begin NOT_STARTED. The plan does not certify current code quality, security, language support or production readiness.

## Observed repository baseline on this date

- 206 tracked paths before this edit, including 89 Python source files (19,219 lines) and 50 Python test files (7,284 lines). Line totals are inventory measurements, not a scope estimate or quality metric. Frontend/dependencies/data account for more of the user's approximate 40K lines.
- Current entry point is `sthira.api.app:app`; it loads legacy modules and mounts the `sthira_v2` router under `/api/v2`.
- `src/sthira_v2/app.py` creates in-memory alert, package and allocation services. SQLAlchemy/PostGIS models/repositories/migrations exist, but their existence is not proof the API uses durable state.
- Current web client is Vite/TypeScript/MapLibre; remote Esri imagery remains in the demo path. Older frontend files remain connected to legacy serving/checks.
- Current voice path restricts local ASR to Hindi/Malayalam; optional Azure explanation does not supply the proposed general constrained middle-model service. TTS accepts a small approved synthetic-text set.
- Test bootstrap and several v2 API tests import the legacy entry point. Deleting `src/sthira` now would break those boundaries; [cleanup.md](cleanup.md) records the migration gates.
- The user had already staged sixteen document moves into `plan/` and had unstaged edits in root instructions, packaging, runbook, task tracker and the moved prompt. This task preserves unrelated work and the staged baseline.

## Historical evidence retained, not re-certified

The previous prompt/task/acceptance files record completed local Python work for strict contracts, synthetic fixtures, CAP parsing/update/cancel/quarantine, operational-package validation, local assignment concurrency, constrained map actions, offline expiry and privacy/health seams. They also record unresolved real PostgreSQL/PostGIS verification, government authorization, regional language/ISL review, hardware/load/security/restore and operational approvals.

Earlier local model synthesis and demo test counts are historical results for earlier revisions and environments. They are not rerun by this documentation-only task and cannot satisfy Go, mobile or live-operation gates. Use corresponding source/tests as input to P0/P1/P3/P6, then record new checks against the revision actually tested.

## Retired requirements

Permanent relocation, parcel/site selection, programme funding, construction/handover, statutory objections/beneficiary schemes, hazard master scores, Wayanad-only launch assumptions, satellite-style hackathon requirements and third-party large-model demo adapters are not requirements to port. Their source is preserved until dependency/evidence retirement is verified.
