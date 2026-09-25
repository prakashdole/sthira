# Three-worker review — 2026-09-21

## Revisions and verdict

CLEAN is still 4095400, clean; none of the three workers is integrated there.
Repair: codex/p567-integration-repair 2e2d5c8. Its shared ancestor with CLEAN is b63e469, so it is not a fast-forward target. Inspected unique commit log, cumulative changed-file inventory, Stage 1 implementation diff, relevant current callers and handoff. The prior A/B changes were already reviewed in review-e0babda-3e6daac.md; avoid repeating unchanged failures as fresh discoveries.
Model evidence: 64b9ea9. Auth audit: 9c372f0. All three worktrees were clean when checked.

Verdict: do not advance to P8. Stage 1 progressed; Stages 2–5 explicitly remain open. Two execution lanes should finish these internal gaps, not launch a new phase. No product changes made during this review.

## Verified progress and limitations

- Stage 1 public PipelineRequest/model response separation, raw model validation and WAV overflow handling are implemented.
- Independently reran at repair tip: `go test -count=1 ./internal/orchestration ./internal/httpserver -run 'TestA1_ModelBytes|TestVoiceProcess_HandleProcessHappyPath|TestVoiceProcess_RealHTTP'`. Both packages passed, including persisted public pipeline tests.
- In nested ASR module, `go test -count=1 -run 'TestDecodeWAV_Malformed|TestB1' ./...` passed, preserving compressed audio success.
- No new full integrated DB/race/scanner/GPU/model quality run claimed. The existing audit's real-DB reproductions were read, and the defective paths confirmed unchanged in repair source; they were not independently rerun this turn.
- C is evidence-only; D is audit-only. Their completed assignments do not imply repaired application behavior.

## Remaining findings

1. D1 confirmed by source and D's real-HTTP/DB evidence: quarantine skips target jurisdiction enforcement when no live source authorization exists. Foreign operator can terminally quarantine source and publications. Fix missing-authorization denial at the shared trusted boundary; preserve grant and replay checks.
2. D2 confirmed by source and audit evidence: reservationPayloadHash omits party_size and snapshot_version. Changed request silently receives stored original result. No extra capacity allocation was demonstrated; this is false acknowledgment/idempotency violation. Include all discriminating fields with unambiguous encoding and address stored-hash compatibility.
3. New translation approval is partial, not complete. readApprovedSpeechKeys collapses allowed-language approvals to bare keys, ignores template_version and allows source_version <= current. ProductionValidator checks speech key and language independently. This representation cannot enforce approval of the exact rendered language/template/source. No end-to-end exploit probe run here; this is a source-confirmed loss of binding. Test language leakage, stale template/source, approval revocation during inference, and exact-binding success.
4. Publication production caller, attributed promotion, cross-instance validity and schema-correct A3 tests remain open per source and repair table. No acceptance from in-memory observer wrappers or skip-as-pass tests.
5. buildEligible still uses zero party, one-day dates, ID sort and PermittedRank. Comments asserting policy order do not establish it. Separate unknown browsing suitability from authorized capacity promises.
6. Real ASR adapter still contains [unverified:real-inference-stub]; TTS still guesses generic AutoModel/generate/.wav; readiness is still file-presence based. These are internal code gaps, not only external GPU blockers.
7. IPC blocked-write cancellation, actual-handler eval conformance, safe smoke cleanup and scanner incomplete-coverage semantics remain unclosed. Security script still ends overall PASS with absent scanners and smoke trap still references a PID assigned later.
8. Model evidence is not fully pinned: `main` is mutable, and ASR reference points to a different Bhili repository. Its snippets and Sarvam/vLLM claims need exact selected-model/version validation. No independent web verification was performed for this review; do not promote its assertions to facts.
9. C/D commits are evidence only. D adds .txt probe/result files despite the task's preservation boundary; do not import that commit wholesale or delete its existing files. Reuse its evidence without modifying .txt files.
10. Cumulative diff --check reports whitespace in imported security-checks.md; minor evidence hygiene, not the gate blocker.

## Backend progress assessment

There is no weighted estimate that supports a defensible percentage. Foundations/data ingestion/storage/stays/offline protocol are substantial existing implementation, but authorization/publication acceptance still has defects. P6 model adapters are not yet real functioning inference, and P7 assurance is incomplete. The project is in late backend implementation/correction, before backend acceptance Gate B. It is not 80–90% production-ready merely because phase numbers reached P7. Full backend acceptance needs authority/publication closure, real verified inference and language evidence, then security/performance/recovery acceptance. External government/IdP/license/translation decisions remain separate.

## Next execution

Exactly two self-contained prompts: plan/next-two-worker-1.md (authority/publication/scoped guidance, final coordinator) and plan/next-two-worker-2.md (inference/IPC/eval/ops). Independent worktrees; Worker 1 alone integrates and edits final ledgers. Keep CLEAN unchanged until reviewed integration. No P8 or live activation authorized.
