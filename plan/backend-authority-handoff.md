# Backend Authority Closure — Worker 1 Handoff & Acceptance Table

Branch: `codex/backend-authority-closure`
Worktree: `/private/tmp/mz-worktrees/backend-authority-closure`
Baseline Commit: `37ecd2b` (cherry-picked 12 unique commits from `b63e469..codex/p567-integration-repair` on top of CLEAN `ed0b538`)
Worker 2 base SHA: `37ecd2b` (Worker 2 can independently branch from `2e2d5c8` or `37ecd2b`)

## Imported Evidence (C & D) with Honest Attribution
- **Worker D (Auth & Replay Audit, commit 9c372f0)**:
  - Imported: `plan/evidence/auth-replay-review.md`, `plan/evidence/auth-replay-probes/migration0005_populated_probe.sh`.
  - Excluded: `.txt` files (`quarantine-no-authz-cross-jurisdiction.go.txt`, `results.txt`) per strict standing rule: do not modify or newly import `.txt` files.
  - Attribution: Worker D verified grant serialization, migration 0005 legacy revocation, and reproduced confirmed defects D1 (quarantine skips jurisdiction check when no live authorization exists) and D2 (`reservationPayloadHash` omits party size and snapshot version).
- **Worker C (Model Evidence, commit 64b9ea9)**:
  - Imported: `plan/evidence/model-integration-verification.md`, `plan/evidence/model-api-probes/probe_indic_conformer_asr.py`, `probe_indic_parler_tts.py`, `probe_sarvam_chat_template.py`.
  - Attribution: Worker C documented primary model integration requirements for IndicConformer-600M-Multi, Sarvam-30B, and Indic Parler-TTS. Probes are marked as opt-in model evidence, not certified production execution.

## Compact Acceptance Table

| Step | Scope / Item | Status | Verified Evidence / Exact Defect |
| :--- | :--- | :--- | :--- |
| **1** | Establish common integration baseline on `codex/backend-authority-closure` | **DONE** | 12 unique commits cherry-picked cleanly onto CLEAN (`ed0b538`). Base SHA `37ecd2b`. Selective C/D evidence imported without `.txt` files. |
| **2** | Repair D1: `handleSourceQuarantine` jurisdiction check when no live authz | **DONE** | Denies absent/expired authorization (`!has` -> `errNoAuthorization` / 409 Conflict), foreign operator denied (403). Reproduced defect before fix, verified regression `TestOperatorQuarantine_AbsentOrExpiredAuthz_Denied` on disposable DB. D61 recorded. |
| **2** | Repair D2: `reservationPayloadHash` omits `PartySize` & `SnapshotVersion` | **DONE** | Unambiguous deterministic JSON encoding across all 9 discriminating fields. Changed party size (2->5) or snapshot version (snap->snap+7) returns 409 `IDEMPOTENCY_CONFLICT`; identical replay returns 200 OK. Existing hashes preserved & expired per TTL. `TestHTTPReservationPayloadBinding_ChangedPayloadConflicts` verified on disposable DB. |
| **3** | Publication lifecycle: transactionally bind attributed promotion | **OPEN** | Reject legacy unattributed promotion; ensure newer publications do not hide current; restrict unchecked store paths; prove lock order with PID/blocking observation. |
| **3** | Repair A3 tests: schema-correct unique fixtures, fail on DSN errors | **OPEN** | Remove `artifacts` table references (use schema tables), remove reused `MNF-LEGACY-1`, ensure tests fail on connection error rather than skip. |
| **4** | Scoped guidance / translation authority: migration 0009 forward migration | **OPEN** | Exact language, rendered template version/content, source identity binding; forward migration for revision 8 databases; revalidate during slow inference. |
| **4** | Separate browsing from capacity / repair `buildEligible` | **OPEN** | Stop supplying PartySize=0, 1-day dates, ID sort labeled `PermittedRank`; no zero-capacity suggestions; authoritative ordering. |
| **5** | Integrate Worker 2 & full acceptance gates | **OPEN** | Integrate verified Worker 2 commits; verify all 6 Go modules; disposable DB tests; IPC `-race` + process tests; final ledger update. |
