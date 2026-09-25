# Integration & repair handoff — acceptance table

Branch: `codex/p567-integration-repair` (worktree `/private/tmp/mz-worktrees/p567-integration-repair`).
HEAD after this session: `b3ddc9a`. Prior tip `d8573b8`. Merge-base with
`CLEAN`: `b63e469`. Current `CLEAN` is `4095400`, which is **not** an
ancestor of this branch (CLEAN advanced past the merge-base with review/
prompt doc commits). **CLEAN cannot be fast-forwarded** to this branch; it
would need a rebase of this branch onto `CLEAN` or a merge commit. Neither
was performed (no push, no main move).

> Integrity note. The two lane commits were integrated by the prior agent as
> re-authored commits (A1–A4 / B1–B6), not the literal cherry-picks listed in
> the prompt, and the initial combined tree was **not** verified. This session
> found and fixed a broken test file and a failing Stage 1 real-HTTP gate on
> the committed tree before claiming anything.

## Ground truth re-checked this session

- `go build ./...` + `gofmt -l` + `go vet` clean across all six modules
  (`backend`, `internal/{asr,tts,middle}worker`, `eval`, `loadmodel`).
- `go test ./...` green: backend module, asrworker, ttsworker,
  middleworker(+eval), eval, loadmodel.
- A live PostgreSQL + `psql` are present; `disposableTestDB` creates a
  migrated DB per test, so the real-DB HTTP suites genuinely run (not skip).

## Acceptance table (verified vs open — NOT a completion claim)

| Stage | Item | Status | Evidence / exact remaining defect |
| ----- | ---- | ------ | --------------------------------- |
| 1 | Public `PipelineRequest` strict handling (citizen bytes) vs model-schema strict handling (model bytes), distinct limits | **DONE** | `orchestrator.go` re-decodes `rawBody` as `PipelineRequest`; `http_client.go` validates model bytes with `validateModelResponseRaw` before typed decode. `b3ddc9a`. |
| 1 | Presence-level model fields (explicit zero/empty forbidden, required, enums, tagged variants) | **DONE** | `model_strict.go` + `TestA1_ModelBytes_*` run through the real `HTTPWorkerClient.Propose` over httptest. |
| 1 | Real public HTTP happy path + failure checks w/ ProductionValidator + actual handlers | **DONE** | `TestVoiceProcess_RealHTTP_PersistedScopedContext_Pipeline` (was 400→422→**200+audio**), `..._StaleSnapshotDuringInference` 409. |
| 1 | Malformed WAV `0xFFFFFFFF` fmt panic bounded; unknown-length only for `data` | **DONE** | `decodeRIFFWAV` uint64 overflow bounds; `TestDecodeWAV_Malformed_*` + `FuzzDecodeWAVChunkSizes` clean over ~360k execs. |
| 1 | Ogg/WebM success + ordinary truncated WAV rejection preserved | **DONE** | lane-B `b1_compressed_test.go` still green in this tree. |
| 2 | ScopedContext binds `TemplateKeys` to **persisted** jurisdiction/source approval | **DONE** | migration `0008 approved_translations` + `store/scoped.go readApprovedSpeechKeys`; empty ⇒ fail-closed preserved. |
| 2 | Remove observer wrapper; trusted publish→promote→delivery wired to persisted authority with a **real production caller** | **OPEN** | `Publisher.WithObserver`/in-process fan-out still the only mechanism; prompt rejects in-memory bus as cross-process invalidation. |
| 2 | Legacy unattributed rows cannot promote (fix, not doc) | **OPEN** | `A3.2` is documented-only; promotion path does not gate `source_id IS NULL`. |
| 2 | A3 tests run on a real migrated DB (no skip-as-PASS) | **OPEN** | `store/a3_lifecycle_test.go` `t.Skip`s when `STHIRA_TEST_DSN` unset (160) and `t.Skipf`s on connect errors (148/151); inserts nonexistent `artifacts` (186); reuses `MNF-LEGACY-1` (122). Must fail on DSN error and use existing schema/seed helpers. |
| 2 | Cross-instance invalidation / independent instances under a declared bound | **OPEN** | in-process only; no independent-instance acceptance without manual invalidation. |
| 2 | Lock-order source/package with real backend PID/lock contention evidence | **OPEN** | `A3.1` probe is a no-deadlock assertion, not PID/lock evidence. |
| 2 | Suitability: drop one-day/party-size-zero passes; honest unknown eligibility; no `PermittedRank` on alphabetic IDs | **OPEN** | only the prior lane-A removals present; not re-verified against accepted inputs. |
| 3 | Real ASR/TTS load→warm→READY (not file presence); remove `[unverified:real-inference-stub]` | **OPEN** | not exercised this session; real inference remains O03/O11 external. Python entry-point malformed/import-failure tests not run here. |
| 3 | IPC boundedness across admission AND stdin write; subprocess `-race` suite | **OPEN** | lane-B `b3_ipc_test.go` green under `go test`, but not run under `-race` with `STHIRA_RUN_PROCESS_TESTS=1`/`crashtest` this session. |
| 3 | Sarvam pinned tokenizer template + vLLM settings, no unverified custom template | **OPEN** | lane-B wired a chat_template; outbound-path evidence + exact revision not re-verified. |
| 4 | Endpoint-specific protocols; eval provider run against the **actual** ASR/middle/TTS servers via public handler | **OPEN** | lane-B rewired `eval/provider/http.go` (B5) but the "run real server constructors through HTTPWorkerClient + public handler and point the eval provider at that host" conformance was not demonstrated this session. |
| 5 | Bounded smoke with ownership-tracked PID/exit trap + seeded success case asserting semantic fields | **OPEN** | not run this session. |
| 5 | Security results distinguish required NOT_RUN vs clean audit; pinned tools; failure propagation | **OPEN** | not run this session; scanners absent. |
| 5 | Full `-race` + process-tag suites + real-DB suites + Python + smoke all green | **OPEN** | only default (non-`-race`, non-process) Go suites run here. |

## Gates / stop

- P5/P6/P7 acceptance: **OPEN**. Gate B: **NOT_READY**. P8: not started.
- Stage 1 is closed with public/worker/database boundary evidence.
- Stage 2 is partially closed (persisted template approval); the rest is
  unfinished engineering, not an external blocker.
- Stages 3–5 retain genuine external gates (O03 model-language matrix, O11
  translation/voice review, O14 operator IdP, GPU/artifact hardware) but also
  contain **internal unfinished work** (real-adapter READY semantics, IPC
  `-race` boundedness, actual-handler eval conformance, smoke/security
  harnesses) that must not be reported complete.

## Commands to continue (honest next actions)

```bash
# Stage 2 — make A3 real, not skipped:
#   migrate a3_lifecycle_test.go + a3_cache_invalidation_test.go to the
#   disposableTestDB pattern, fix `artifacts`→source_artifacts, unique
#   manifest IDs, fail on DSN error, add promotion attempts on unchanged state.
STHIRA_TEST_DSN='postgres://…' go test -count=1 -run TestA3 ./internal/store ./internal/offlinedelivery

# Stage 3/5 — process + race gates:
crashtest=1 STHIRA_RUN_PROCESS_TESTS=1 go test -race -count=1 ./internal/asrworker ./internal/ttsworker
# Stage 4 — actual-handler conformance; Stage 5 — run_security_checks.sh + run_k6_smoke_real.sh
```
