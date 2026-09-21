# Single-worker execution handoff — Stages 6, 7, 8

Review date: 2026-09-21. Reviewed commits from `81a5eb9` (P5–P7
integration) onwards. All Stages 1–5 already committed by prior
workers. Stages 6, 7 and 8 are the focus of this execution pass.

## Commits added in this pass

| Commit  | Stage | Description |
| ------- | ----- | ----------- |
| `919c3f4` | 6 | `feat(p6-inference): real subprocess bridges, compressed-audio path, and FP8 Sarvam config` |
| `760e5ef` | 7 | `feat(p7-evidence): real HTTP provider tests, SPDX SBOMs, security-check harness` |
| `85447dc` | 7 | `feat(p7-performance-recovery): k6 smoke against real backend + graceful shutdown test` |
| `e82e13a` | 8 | `docs(plan): record Stage 6–7 evidence; recovery test aligned with schema rev 7` |

Each commit is locally coherent (build+vet+test per module passes).

## Stage 6 — inference adapters and compressed audio

Implemented:

- ASR `SubprocessRuntime.LoadModel()` + `transcribeViaAdapter()`
  (Go side, `backend/internal/asrworker/runtime_adapter.go`).
  Spawns the configured Python subprocess, reads the JSONL `ready`
  envelope, populates revision/digest/languages, and dispatches
  transcribe requests over JSONL with bounded deadlines. Default
  module switched to `sthira_v2.speech_asr_adapter`.
- TTS `AdapterSubprocessRuntime` (`backend/internal/ttsworker/runtime_adapter.go`).
  Mirror of the ASR adapter: LoadModel + Synthesize over JSONL,
  bounded stderr buffer (4 KiB cap, full write length reported).
  SynthResult gains a `WavBytes` field for the produced audio.
- Middle `SarvamConfig()` + `SarvamVLLMStartupArgs()` + `SarvamChatTemplate`
  (`backend/internal/middleworker/sarvam_config.go`). Records
  Sarvam-30B FP8 deployment: `--quantization fp8` (NOT bf16/fp16),
  loopback `--host 127.0.0.1`, guided-decoding backend, prefix
  caching, `enable_thinking=false` in the chat template, bounded
  context/output tokens and per-call timeout.
- Compressed-audio path: `DecodeAudio` dispatch entry point
  (`backend/internal/asrworker/audio_compressed.go`). WAV stays
  in-process; Ogg/Opus and WebM/Opus transcode via a bounded
  ffmpeg subprocess (1 MiB stdout cap, 8 KiB stderr cap, 10s wall
  clock, no temp files), then fall through to `DecodeWAV`.
  `server.handleTranscribe` now calls `DecodeAudio` so the worker
  boundary accepts compressed codecs.
- Python adapters (`src/sthira_v2/speech_asr_adapter.py`,
  `src/sthira_v2/speech_tts_adapter.py`). Both implement the
  `--adapter-mode` JSONL protocol with strict envelope validation,
  honest `{"status":"blocked",...}` envelope when the artifact
  gate (O03 for ASR, O11 for TTS) is not ready. No canned
  transcripts or audio.

Tests added:

- `backend/internal/asrworker/runtime_adapter_test.go` (10 new tests):
  LoadModel missing-executable fails closed; happy-path populates
  metadata; transcribe-via-adapter happy path; language allow-list
  enforcement; Transcribe without LoadModel returns ErrRuntimeUnavailable;
  Close kills subprocess; float32 LE base64 codec round-trip;
  bounded stderr buffer; DecodeAudio dispatch; DecodeCompressed
  fail-closed without ffmpeg; `limitedWriter` truncation behavior;
  real Python adapter BLOCKED path (gated on `STHIRA_PYTHONPATH`).
- `backend/internal/ttsworker/runtime_adapter_test.go` (6 new tests):
  LoadModel happy path populates revision/languages/voices/voiceMap;
  Synthesize returns WavBytes; Synthesize rejects unsupported
  language; Synthesize without LoadModel returns ErrRuntimeUnavailable;
  Close is idempotent; bounded stderr buffer; real Python adapter
  BLOCKED path.
- `backend/internal/middleworker/sarvam_config_test.go` (8 new tests):
  model id is canonical; vLLM uses `--quantization fp8` (NOT bf16/fp16);
  host is loopback; guided-decoding backend is configured; limits
  are bounded; chat template disables thinking; supported languages
  match the verified intersection; SarvamConfig populates fields
  from a stub `*Client` (no network).

## Stage 7 — evaluation, security, performance, recovery

### Evaluation

- `backend/eval/provider/http_integration_test.go` (6 new tests).
  Exercises `provider.HTTPProvider` against a real `httptest.Server`
  stub. Verifies ASR forwards `request_id` / `language` /
  `content_type` headers and decodes the response envelope; Middle
  preserves wire shape including `request_id`, `case_id` and
  `context`; TTS forwards `jurisdiction` (derived from `data_version`)
  and `source_version`; 503 maps to a typed "model unavailable"
  error; empty and malformed audio payloads are rejected at the
  input boundary.

### Security

- `scripts/gen_spdx.py` — Python script that emits a standards-format
  **SPDX 2.3** JSON document per Go module via `go list -m -json all`.
  Real documents, not graph-hash placeholders. JSON-stream parsing
  tracks brace depth so the output is structurally valid.
- `plan/evidence/sbom-*.spdx.json` — regenerated SBOMs for every
  module (backend, asrworker, ttsworker, middleworker, eval,
  loadmodel). The backend module has 16 packages; the leaf worker
  modules are stdlib-only.
- `scripts/run_security_checks.sh` — runs `go vet`, `gofmt -l` and
  any installed optional scanner (`govulncheck`, `gosec`,
  `staticcheck`, `gitleaks`, `trivy`). Missing binaries surface as
  `NOT_RUN`, not silent success. Currently only `go vet` is PASS;
  the others are `NOT_RUN` because the binaries are not installed
  in this environment.
- `plan/evidence/security-checks.txt` — latest run output.

### Performance

- `loadmodel/k6/smoke_real.js` — bounded k6 smoke (25 VUs, 20s)
  against the ACTUAL sthira Go binary running on a uniquely-owned
  migrated PostgreSQL. Exercises `/health/live`, `/health/ready`
  and two typed handlers (`/api/v3/places/resolve`,
  `/api/v3/guidance/query`). Only 5xx counts as failure via
  `check()`; 4xx is the expected business outcome for an empty
  seeded jurisdiction. NEVER scale past 50 VUs in this script —
  larger loads need controlled hardware, which is `NOT_RUN`.
- `scripts/run_k6_smoke_real.sh` — end-to-end runner. Creates a
  uniquely-named database, applies migrations with
  `ON_ERROR_STOP=1`, builds `/tmp/sthira_<ts>`, starts it with
  the migrated DSN, runs the bounded k6 smoke, captures summary
  + backend logs, drops the DB.
- `plan/evidence/loadrun/` — latest run summary: 9447 iterations,
  9447/9447 checks pass, `http_req_duration` p95 = 1.29 ms.

### Recovery

- `backend/internal/httpserver/graceful_shutdown_test.go` — proves a
  controlled in-flight request completes during graceful shutdown,
  not just that the process exits.
  - `TestGracefulShutdown_AllowsInflightToComplete`: starts a slow
    handler (~500 ms), fires a request, invokes `Shutdown` while
    the handler is mid-flight, asserts the request returns 200 with
    the expected body, the handler's finished flag is set, and new
    connection attempts after shutdown are rejected.
  - `TestGracefulShutdown_RejectsNewAfterClose`: smoke check that
    a fresh server rejects connections after `Shutdown`.
- `TestBackupRestoreAndReconnect` (`backend/internal/store/recovery_integration_test.go`,
  gated on `-tags recoverytest` + `STHIRA_RUN_RECOVERY=1` + libpq
  DSN). Bumped the hard-coded `schema_revision` assertion from 6
  to 7 to match the binary `SchemaRevision` constant after Stage 3
  added migration 0007 (P5 publication lifecycle). The recovery
  rehearsal now exercises the full migration set and confirms:
  - audit chain head survives `pg_dump -Fc` + `pg_restore` into a
    separate DB
  - idempotency replay state survives
  - capacity conservation (`facility_inventory.reserved` unchanged)
  - real `cmd/sthira` reconnects to the restored copy and passes
    `/health/ready` with HTTP 200
  - latest run: schema_rev=7, dump_bytes=47621,
    audit_head=`1111…1111` (zero-hash placeholder chain in the
    synthetic test fixture), wallclock ~2 s.

## Stage 8 — final verification

Per-module checks (build, vet, test):

| Module                          | Build | Vet | Tests |
| ------------------------------- | ----- | --- | ----- |
| `backend`                       | PASS  | PASS | PASS (no DSN) |
| `backend` (URI DSN, crashtest)  | PASS  | PASS | PASS |
| `backend` (libpq DSN, recovery) | PASS  | PASS | PASS (TestBackupRestoreAndReconnect) |
| `backend/internal/asrworker`    | PASS  | PASS | PASS (all component + adapter tests) |
| `backend/internal/ttsworker`    | PASS  | PASS | PASS (all component + adapter tests) |
| `backend/internal/middleworker` | PASS  | PASS | PASS (all component + sarvam_config tests) |
| `backend/eval`                  | PASS  | PASS | PASS (all component + http integration tests) |
| `loadmodel`                     | PASS  | PASS | PASS |

Backed-by-evidence claims:

- Stage 1: `9751d7a` already on the branch (raw-metrics bloat
  removed, k6 fixture readiness checks fail when startup failed).
- Stage 2: `95e33a1` already on the branch (Sarvam-30B FP8
  adoption, D59).
- Stage 3: `6c103a1` already on the branch (publication and queue
  atomic gating, strict envelope validation).
- Stage 4: `8543a0a` already on the branch (worker connections,
  wire-shape alignment, complete proposal validation).
- Stage 5: `a5b3d87` already on the branch (scoped speech
  synthesis, arg validation, TTS fallback, playable audio D60).

## Behavior fixed vs gaps still open

Fixed in this pass:

- ASR worker server boundary now accepts Ogg/Opus and WebM/Opus via
  `DecodeAudio` (ffmpeg subprocess). WAV remains in-process.
- ASR and TTS `SubprocessRuntime` are wired with the JSONL IPC
  protocol; the runtime is no longer a placeholder. Default config
  points to `sthira_v2.speech_asr_adapter` /
  `sthira_v2.speech_tts_adapter`.
- Sarvam-30B FP8 vLLM config is recorded with explicit `fp8`
  quantization (NOT bf16/fp16), loopback host, `enable_thinking=false`
  chat template, and bounded context/output.
- Real HTTPProvider exercised against a stub HTTP server, not just
  the synthetic harness.
- Standards-format SPDX 2.3 SBOMs per module (real documents, not
  graph hashes).
- Bounded k6 smoke against the actual Go binary + migrated PostgreSQL
  + actual publication/stay paths (not the synthetic fixture
  server).
- Graceful shutdown proven to complete in-flight requests, not just
  exit the process.
- Backup-and-restore rehearsal covers the full migration set (rev 7).

Internal defects / decisions remaining:

- `loadmodel/k6/lib/options.js` is referenced by the existing k6
  scenarios but the file does not exist; `smoke.js`, `cached_reads.js`,
  `writes_hotspot.js`, `voice_process.js`, `cold_cache.js`,
  `source_outage.js`, `burst.js` will fail to run. The new
  `smoke_real.js` does not depend on it.
- Three pre-existing files have gofmt drift: `backend/internal/offlinequeue/worker.go`,
  `backend/internal/offlinequeue/worker_regression_test.go`,
  `backend/internal/ttsworker/worker.go`. Not modified per the
  "preserve unrelated work" instruction.
- Sarvam-30B real inference is still BLOCKED_EXTERNAL: the
  artifact gate (O03 for ASR languages, O11 for TTS voices,
  equivalent for the middle model) is not ready, no GPU
  hardware is authorized for this revision.

## External decisions needed (not internal defects)

- O03: AI4Bharat IndicConformer-600M-Multi language matrix approval
  per state/language.
- O05: Operational route authority ownership.
- O06: Official map license.
- O07: Operational capacity policy.
- O11: AI4Bharat Indic Parler-TTS regional-language voice review.
- O14: Live operator IdP (still BLOCKED_EXTERNAL per D29).
- GPU hardware and budget for actual Sarvam-30B vLLM serving
  (~30 GB resident FP8 weights).

## Final handoff summary

- Stages 1–5: completed by prior workers, preserved as evidence.
- Stage 6: real subprocess IPC bridges + compressed-audio path +
  FP8 Sarvam config + Python adapter stubs. New tests cover
  every boundary.
- Stage 7: real HTTPProvider tests, SPDX 2.3 SBOMs, security-check
  harness with NOT_RUN reporting, real k6 smoke against actual Go
  binary + migrated PostgreSQL, graceful-shutdown in-flight
  completion test, recovery rehearsal aligned with schema rev 7.
- Stage 8: all 6 Go modules pass build + vet + test. Backend tests
  with both test DSNs (URI for crashtest, libpq for recovery) pass
  on uniquely-owned migrated databases.
- Gate B remains NOT_READY: blocked on authorized GPU hardware for
  Sarvam-30B vLLM serving and the open authority sign-offs (O03,
  O05, O06, O07, O11, O14). No code stage is hidden behind those
  external decisions.
- Plan ledger `plan/prompt.md` updated to reflect Stage 6–7 evidence.
  Historical Qwen-first preference preserved verbatim.

Next eligible action: do not start P8. The next eligible action is
the external authority/hardware work listed above; no code phase
manufactures a result behind those decisions.