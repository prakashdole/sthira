# Four-worker integration and round-two checkpoint

Review date: 2026-09-21. Integrated product revision: `cdd11e2`. Review is focused on assigned changes and their callers, not an exhaustive certification. No product corrections were made by the reviewer during this integration.

## Integrated work

| Worker | Source tip | Integration |
| --- | --- | --- |
| Backend authority | daa7325 | Merged, retaining worker history and current planning files |
| Inference/runtime | 57ffa4d | Five new commits cherry-picked as 767ec31, 781dde5, fe0931d, be83461, c0f6fed; earlier repair work was already imported by worker 1 |
| Go deployment | e49b40d | Merged as 5ca95f2; experimental, not runtime-certified |
| Scenario preparation | 34a492c | Merged as cdd11e2 |

Worker 1 delivered quarantine authorization and reservation-payload corrections, publication lifecycle work, and scoped guidance changes. Worker 2 delivered model adapters, bounded IPC, real-server protocol evaluation and smoke/security harness corrections. Worker 3 delivered deployment/migration/backup scripts. Worker 4 delivered scenario init/validate/report/bundle tooling. None establishes production readiness by itself.

All four workers' committed deliverables are integrated. Uncommitted worker scratch logs and generated smoke summaries remain in their worktree, untouched. The intentional requirements-voice.txt adapter dependency change was imported with worker 2; no blanket .txt cleanup was performed.

## Independently executed checks

- Build, vet and regular tests PASS in seven modules: backend, ASR worker, TTS worker, middle worker, eval, loadmodel, deployment migration utility.
- Full backend suite PASS on a newly created PostgreSQL 18.6 database with migrations 0001–0009 and a separate explicit admin DSN. Database removed after the run.
- Crash/retry and cross-process last-space tests PASS with both required process-test gates enabled on the earlier disposable database.
- Python adapter tests: 20 PASS. These use substitute runtime objects, not real model weights.
- Security harness self-check: all 16 assertions PASS. This checks result aggregation, not the application against real security scanners.
- Initial full DB invocation failed because disposableTestDB replaces literal /postgres in its admin URL and fell back to the test database URL. Explicit STHIRA_TEST_ADMIN_DSN corrected invocation; this is a brittle test helper, not evidence of 12 independent product failures.
- Imported evidence Markdown has trailing whitespace. Not silently presented as a clean diff-check result.
- No new race-detector, actual-model, GPU, Docker, representative load or full security-scanner acceptance claimed in this review. Docker is unavailable on this host.

## Known gaps; do not repeat the handoffs' full-closure claims

### Fix on the selected voice/demo path

1. **Policy identifiers disagree.** `opkg.AllocationPolicy.Order` orders safe-zone IDs; `store.buildEligible` treats each entry as a facility ID. Valid package order can therefore yield no destinations. Map policy zone order to member facilities; keep browsing distinct from party/date-specific allocation. Its inventory queries also currently ignore database errors.
2. **TTS metadata is not negotiated through the pipeline.** `orchestration.stageTTS` hardcodes 16000 Hz and returns those request settings, while the adapter now supports Indic Parler's native output. Verify actual returned WAV metadata and propagate consistent settings; do not claim audio works from adapter unit tests alone.
3. **Worker startup race remains acknowledged.** Real-server eval tests carry a !race build constraint. ASR/TTS listener access needs synchronization and the conformance tests must then participate in race runs. The handoff's claim about every worker should be checked per server, not copied blindly.
4. **Real model execution is NOT_RUN.** Verify exact model artifacts, dependency compatibility, load/warmup, actual audio/text/output and selected-language quality. Do not use live IdP decision O14 as shorthand for a GPU blocker: O14 is operator identity, O07 is stay policy.
5. **Demo setup is not supplied by production authorization.** Build an explicit isolated exercise flow and fixture/setup command without weakening production authority checks. The ordinary pipeline rejects synthetic-only speech templates; do not relabel a synthetic template as approved operational content to make the demo work.

### Retain as production blockers or optional-tool corrections

6. **Translation binding remains incomplete.** `readApprovedSpeechKeys` still accepts NULL source IDs, and a NULL language authorizes every allowed language. Migration 0009 adds template_sha256 but runtime resolution does not enforce it. Zero template/source versions bypass the new registry comparisons. This is not strict source/template/language binding. Keep live guidance disabled until repaired and verified.
7. **Readiness minimum is stale.** `store.SchemaRevision` remains 7 although scoped queries require migrations 8/9. The current prober accepts revision >=7, so an incomplete database can report ready. A revision-9 test database does not prove revision-7 rejection.
8. **Deployment is not runnable as documented.** Dockerfile builds the nested migration module from the parent module; Compose publishes an IP-and-port string in the numeric published field and binds the API to container loopback. Migration entrypoint needs psql and migration files, neither supplied by the distroless image. Fixed container/volume names also defeat project isolation. Keep this package experimental; avoid making it the round-two critical path before real image/Compose checks.
9. **Scenario CLI containment is incomplete.** `safeResolve` checks lexical paths, not resolved parent symlinks. Leaf Lstat in package loading does not protect symlinked parents; catalogue reading lacks that leaf check. Reads use os.ReadFile before bounding bytes. Use only trusted local inputs pending containment and size-bound corrections; do not expose this CLI as an upload service.
10. **Reservation old-hash compatibility is limited.** New hashing includes missing discriminating fields, but old persisted hashes cannot replay identically under the new algorithm. The handoff relies on expiration; do not deploy across live reservations without an explicit compatibility/reconciliation policy.

These findings are primarily source-derived; the passing suites do not exercise all of them. Do not start another blanket production closure campaign before round two. Prioritize the actual demonstrated journey and keep the remainder visible.

## GitHub publication blocker

Remote CLEAN at review: `2bea1fc`. Fetch completed; main untouched. Current tree contains no file larger than 245 KB, but unpushed history includes deleted metrics blobs of approximately 395 MB, 341 MB, 229 MB, 129 MB and other generated artifacts. Four exceed GitHub's 100 MiB per-file limit. Deleting files again at HEAD cannot remove historical blobs from a push.

An incoming-object scan found no matches for high-confidence private-key, AWS access-key or GitHub-token patterns. This limited check is not a complete secret audit.

Proposed publication: preserve the full local integrated history under a backup branch; publish the exact reviewed final tree as a consolidated commit descended from existing origin/CLEAN, excluding obsolete historical generated blobs. This requires explicit approval under the user's no-squash/no-history-rewrite rule. Do not force-push main or rewrite existing remote commits. No push has been performed at this checkpoint.

The current user priority is recorded in [round-two-demo.md](../round-two-demo.md). It supersedes the old frontend-waits-for-Gate-B sequencing only for the prototype.
