# Backend Authority Closure — Worker 1 Handoff & Acceptance Table

Branch: `codex/backend-authority-closure`
Worktree: `/private/tmp/mz-worktrees/backend-authority-closure`
Baseline Commit: `37ecd2b` (cherry-picked 12 unique commits from `b63e469..codex/p567-integration-repair` on top of CLEAN `ed0b538`)
Current Head Commit: `5713322`
Worker 2 Status: Worktree `/private/tmp/mz-worktrees/inference-runtime-closure` on branch `codex/inference-runtime-closure` (uncommitted work in progress; preserved without cross-worktree interference).

---

## 1. Imported Evidence (C & D) with Honest Attribution
- **Worker D (Auth & Replay Audit, commit 9c372f0)**:
  - Imported: `plan/evidence/auth-replay-review.md`, `plan/evidence/auth-replay-probes/migration0005_populated_probe.sh`.
  - Excluded: `.txt` files (`quarantine-no-authz-cross-jurisdiction.go.txt`, `results.txt`) per strict standing rule: do not modify or newly import `.txt` files.
  - Attribution: Worker D verified grant serialization, migration 0005 legacy revocation, and reproduced confirmed defects D1 (quarantine skips jurisdiction check when no live authorization exists) and D2 (`reservationPayloadHash` omits party size and snapshot version).
- **Worker C (Model Evidence, commit 64b9ea9)**:
  - Imported: `plan/evidence/model-integration-verification.md`, `plan/evidence/model-api-probes/probe_indic_conformer_asr.py`, `probe_indic_parler_tts.py`, `probe_sarvam_chat_template.py`.
  - Attribution: Worker C documented primary model integration requirements for IndicConformer-600M-Multi, Sarvam-30B, and Indic Parler-TTS. Probes are marked as opt-in model evidence, not certified production execution.

---

## 2. Compact Acceptance Table

| Step | Scope / Item | Status | Verified Evidence / Exact Defect |
| :--- | :--- | :--- | :--- |
| **1** | Establish common integration baseline on `codex/backend-authority-closure` | **DONE** | 12 unique commits cherry-picked cleanly onto CLEAN (`ed0b538`). Base SHA `37ecd2b`. Selective C/D evidence imported without `.txt` files (commit `dfc8420`). |
| **2** | Repair D1: `handleSourceQuarantine` jurisdiction check when no live authz | **DONE** | Denies absent/expired authorization (`!has` -> `errNoAuthorization` / 409 Conflict), foreign operator denied (403). Reproduced defect before fix, verified regression `TestOperatorQuarantine_AbsentOrExpiredAuthz_Denied` on disposable DB. D61 recorded in `plan/decisions.md` (commit `3aaa807`). |
| **2** | Repair D2: `reservationPayloadHash` omits `PartySize` & `SnapshotVersion` | **DONE** | Unambiguous deterministic JSON encoding across all 9 discriminating fields. Changed party size (2->5) or snapshot version (snap->snap+7) returns 409 `IDEMPOTENCY_CONFLICT`; identical replay returns 200 OK. Existing hashes preserved & expired per TTL. `TestHTTPReservationPayloadBinding_ChangedPayloadConflicts` verified on disposable DB (commit `3aaa807`). |
| **3** | Publication lifecycle: transactionally bind attributed promotion | **DONE** | Strict lock order (`sources` -> `packages` -> `published_*`) in `publisher.go` and `Store.Promote*`. Contention serialization verified via PID observation `TestA3_LockOrder_PIDObservation` and concurrent probe `TestA3_LockOrderProbe`. Attributed promotion rejects unattributed rows (`source_id` NULL/empty) with `ErrPublicationAuthority` and zero writes (`TestA3_LegacyUnattributedManifestNotPromoted`, `TestA3_LegacyUnattributedRowCannotBePromoted`) (commit `a60c6eb`). |
| **3** | Offline delivery staging isolation & lifecycle invalidation | **DONE** | `GetPublishedManifest` excludes `STAGED` and prioritizes `CURRENT` so staged revisions do not hide valid CURRENT manifest. `publication_adapter` restricts citizen delivery to `SourceStatus == "CURRENT"`. Verified cross-instance invalidation on withdrawal (`TestA3_LifecycleInvalidatesAcrossInstances`) without manual test invalidation (commit `a60c6eb`). |
| **3** | Repair A3 tests: schema-correct unique fixtures, fail on DSN errors | **DONE** | Replaced non-existent `artifacts` with `source_artifacts` schema; replaced hardcoded `MNF-LEGACY-1` with unique IDs; connection/ping errors in `openTestStoreAt` fail (`t.Fatalf`) on supplied DSN rather than skip (commit `a60c6eb`). |
| **4** | Scoped guidance / translation authority: migration 0009 forward migration | **DONE** | Forward migration `0009_p6_template_binding.sql` adds `source_id` & `template_sha256` index. `readApprovedSpeechKeys` binds exact `source_version`, `template_version`, `source_id`, returning exact `ApprovedSpeechKeys` language mapping. `EnforceScopedContext` rejects language leakage (`TestEnforce_LanguageSpecificSpeechKeyApproval`, `TestScopedGuidance_ExactLanguageBinding`). `SnapshotRevalidate` checks approval revocation during slow inference (`TestScopedGuidance_RevocationDuringInference`). Mismatched template versions and invented IDs in `speech_args` rejected (`TestSynthesize_InventedArgIDRejected`, `TestSynthesize_MismatchedTemplateVersionRejected`). Silent actions pass without speech approval (`TestEnforce_SilentActionDoesNotRequireSpeechApproval`) (commit `1afbada`). |
| **4** | Separate browsing from capacity / repair `buildEligible` | **DONE** | `buildEligible` removes fake PartySize=0 / 1-day queries and alphabetical sorting. Strictly respects package `allocation_policy.order`, sets `CapacityKnown=false, Free=0` for browsing suitability without false capacity promises (`TestScopedGuidance_BuildEligible_PolicyOrderingAndZeroCapacity`). Empty policy leaves `EligibleDestinations` empty (`TestScopedGuidance_BuildEligible_EmptyPolicyYieldsNoEligibleChoices`). Zero-capacity safe zones and zero-capacity inventory excluded from eligibility (`ChoiceQuerier` and `buildEligible`) (commit `1afbada`). |
| **5** | Verification gates: all 6 Go modules, DB regressions, race tests | **DONE** | All 6 modules compile, pass `go vet`, and pass unit/integration test suites. `-race` clean on all worker/concurrency packages. Process crash tests pass under `-tags crashtest`. k6 smoke pass (commit `5713322` + current). |

---

## 3. Full Verification Results

### A. All 6 Go Modules Verified
| Module Path | `gofmt` | `go vet` | `go build` | `go test` | Duration |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `backend` | Clean | Clean | Clean | PASS (real DB migrations 0001-0009) | ~15s |
| `backend/internal/asrworker` | Clean | Clean | Clean | PASS (33.95s) | 33.95s |
| `backend/internal/ttsworker` | Clean | Clean | Clean | PASS (3.08s + 0.37s) | 3.45s |
| `backend/internal/middleworker` | Clean | Clean | Clean | PASS (3.48s + 0.38s) | 3.86s |
| `backend/eval` | Clean | Clean | Clean | PASS (corpus, provider, report, runner) | 1.10s |
| `loadmodel` | Clean | Clean | Clean | PASS (fixtures, voiceload) | 1.80s |

### B. Race Detector Checks (`go test -race -count=1`)
| Package / Module | Result | Notes |
| :--- | :--- | :--- |
| `backend/internal/asrworker` | PASS (35.52s) | 0 race conditions |
| `backend/internal/ttsworker` | PASS (2.73s + 1.37s) | 0 race conditions |
| `backend/internal/middleworker` | PASS (6.53s + 1.40s) | 0 race conditions |
| `backend/eval` (all subpackages) | PASS (7.17s) | 0 race conditions |
| `loadmodel` (all subpackages) | PASS (4.36s) | 0 race conditions |
| `backend` (`contracts`, `orchestration`, `offlinedelivery`) | PASS (4.63s) | 0 race conditions |

### C. Explicit Process Crash & Concurrency Tests
Tested against uniquely owned, migrated PostgreSQL database (`sthira_proc_test_w1`) with `-tags crashtest` and `STHIRA_RUN_PROCESS_TESTS=1`:
- `TestCrossProcessLastSpace`: **PASS** (1.22s). 6 child `sthira` server processes raced on a single capacity unit; exactly 1 won, conservation held, atomic commit verified.
- `TestCrashAfterCommitBeforeResponse`: **PASS** (0.81s). Child process SIGKILLed after DB commit and before HTTP response; restart and replay of same idempotency key returned stored result without duplicate reservation, capacity, or audit mutation.
- Database dropped immediately upon test completion.

### D. Affected Python Tests
Executed via repository virtual environment (`.venv/bin/pytest`):
- `tests/test_b2_adapters.py`: **8 passed** in 0.30s.
- `tests/test_v2_speech_stt.py` + `tests/test_v2_voice_commands.py`: **5 passed** in 0.61s.

### E. Bounded k6 Smoke Run
Executed `./scripts/run_k6_smoke_real.sh` against live compiled `sthira` Go binary on freshly migrated PostgreSQL:
- Total checks: 45,065 passed, 0 failed (100% success).
- HTTP 5xx errors: 0.
- Connection failures: 0.
- Duration: 25 VUs for 20s.
- HTTP Request Duration: median 305µs, p90 2.61ms, p95 5.35ms.
- Results recorded in `plan/evidence/loadrun/smoke_real_summary.json` and `plan/evidence/loadrun/sthira.log`.

---

## 4. Integration & Worker Status

### Worker 1 (Self)
- Owns backend authority, shared contracts, publication lifecycle, orchestration, migrations, and final integration.
- All Steps 1 through 5 complete and verified.
- Tip commit on `codex/backend-authority-closure`: `5713322` (+ documentation update).

### Worker 2 (Inference Runtime)
- Operating in `/private/tmp/mz-worktrees/inference-runtime-closure` on branch `codex/inference-runtime-closure`.
- Changes are currently uncommitted in Worker 2's worktree (`speech_asr_adapter.py`, `speech_tts_adapter.py`, `manifest.go`, `runtime_ipc.go`, `client.go`, `sarvam_config.go`, `audio.go`, `runtime_adapter.go`, `ipc_bounded_test.go`).
- Per standing rules, Worker 1 does not touch, edit, or commit Worker 2's files. When Worker 2 commits its verified stage, its commits can be cherry-picked onto this integration branch.

---

## 5. Gate B Status & Remaining Blockers

### Gate B Verdict: `NOT_READY`

Phase 7 Gate B remains **`NOT_READY`** per standing instructions. While internal backend authority, publication, scoped guidance, and orchestration are fully closed and verified, the following internal and external criteria remain:

### Internal Tasks Awaiting Completion:
1. **Worker 2 Inference Integration**: Cherry-pick and verify Worker 2's commits once committed (subcommand streaming, bounded stdin writes, subprocess lifecycle).
2. **Scanner & Security Tooling**: Run Trivy/Gosec scanning when installed and available.

### External Prerequisites Remaining (Documented in `plan/open-decisions.md`):
1. **O01 (Government Operational Data)**: Official CAP disaster feeds and authorized zones.
2. **O03 / O11 (Speech & Translation Approvals)**: Certified human audio recordings and government-approved regional translations.
3. **O05 (Route Authority)**: Official government-approved evacuation corridors.
4. **O06 (Map Cartography & Tiles)**: Authorized government basemap tile service.
5. **O07 (Operator IdP Federation)**: Production integration with state/national disaster management single-sign-on.
6. **Hardware Infrastructure**: GPU hardware for self-hosted IndicConformer, Sarvam-30B, and Indic Parler-TTS production inference.

### Prohibited Scope:
- **Phase P8 (Mobile UI)**: NOT STARTED. Under standing instructions, P8 implementation must not begin before Gate B is officially passed.
