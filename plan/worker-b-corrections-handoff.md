# Worker B corrections handoff — b0b9772 → codex/p567-inference-corrections

This handoff documents the bounded correction work performed by
Worker B on the review at `b0b9772`. Each row records the
reproduced defect, the root-cause fix, the exact test/command
exercising it, the revision it ran on, and any unverified claim.

## Branch / revision

- Branch: `codex/p567-inference-corrections`
- Base SHA: `b0b9772` (review-b0b9772)
- Worktree: `/private/tmp/mz-worktrees/p567-inference-corrections`
- HEAD commit: see `git log codex/p567-inference-corrections -5`

## Owned modules

- `backend/internal/asrworker` and its `go.mod`
- `backend/internal/ttsworker` and its `go.mod`
- `backend/internal/middleworker` and its `go.mod`
- `backend/eval` and its `go.mod`
- `loadmodel` and its `go.mod`
- `src/sthira_v2/speech_asr_adapter.py`, `src/sthira_v2/speech_tts_adapter.py`
- `tests/test_b2_adapters.py`
- `loadmodel/k6/smoke_real.js`, `loadmodel/k6/smoke_business.js`
- `scripts/run_security_checks.sh`, `scripts/run_k6_smoke_real.sh`, `scripts/gen_spdx.py`
- `plan/evidence/sbom-*.spdx.json`, `plan/evidence/security-checks.md`, `plan/evidence/security-checks.json`

Out of lane (do not touch): `backend/cmd/sthira`, `backend/internal/orchestration`,
`backend/internal/store`, `backend/internal/httpserver`, `backend/internal/offlinequeue`,
`backend/internal/contracts`, migrations, shared OpenAPI.

## Summary matrix

| ID  | Description                                              | Status | Evidence |
|-----|----------------------------------------------------------|--------|----------|
| B1  | Real ffmpeg Ogg/Opus/WebM decoding                        | PASS   | `backend/internal/asrworker/b1_compressed_test.go` (8 tests) |
| B2  | Real Python ASR/TTS adapter load/inference               | PASS   | `tests/test_b2_adapters.py` (8 tests) + `src/sthira_v2/{speech_asr_adapter,speech_tts_adapter}.py` |
| B3  | Subprocess correlation, concurrency, cancellation        | PASS   | `backend/internal/asrworker/b3_ipc_test.go` (8 tests) + `backend/internal/ttsworker/b3_ipc_test.go` (7 tests) |
| B4  | TTS /health wire shape + Sarvam chat_template wired      | PASS   | `backend/internal/ttsworker/b4_health_test.go` (2 tests) + `backend/internal/middleworker/b4_sarvam_test.go` (1 test) |
| B5  | Eval provider wire shapes match contracts                 | PASS   | `backend/eval/provider/http_integration_test.go` (5 tests) |
| B6  | k6 business flow + security evidence + SPDX root         | PASS   | `scripts/run_k6_smoke_real.sh`, `loadmodel/k6/smoke_business.js`, `plan/evidence/security-checks.{md,json}`, `scripts/gen_spdx.py` |

## B1 — real compressed audio decoding

### Defect

`backend/internal/asrworker/audio.go:decodeRIFFWAV` rejected
ffmpeg-produced WAV with `decode: wav data chunk truncated`. ffmpeg
writes unknown-length (0xFFFFFFFF) data chunks to nonseekable stdout
because the total byte count cannot be known up-front.

### Fix

`backend/internal/asrworker/audio.go` accepts the 0xFFFFFFFF
streaming sentinel and decodes to EOF; ordinary known-size
truncation validation is preserved (a known-size chunk that runs
past the file boundary is still rejected). The `LIST` chunk
between `fmt` and `data` is now walked correctly (ffmpeg adds one).

### Tests

`backend/internal/asrworker/b1_compressed_test.go`:

- `TestB1_DecodeAudio_AcceptsRealOggOpus` — real ffmpeg-generated
  Ogg/Opus, end-to-end through `DecodeAudio`.
- `TestB1_DecodeAudio_AcceptsRealWebmOpus` — same for WebM/Opus.
- `TestB1_DecodeWAV_AcceptsUnknownLengthDataChunk` — minimal WAV
  with 0xFFFFFFFF data chunk decodes to EOF.
- `TestB1_DecodeCompressed_HandlesRealFFmpegOutput` — ffmpeg
  output (with LIST chunk) decodes.
- `TestB1_DecodeWAV_StillRejectsTruncatedKnownSize` —
  ordinary truncation validation preserved.
- `TestB1_DecodeWAV_RejectsOversizeDuration` — duration cap
  fires post-resample.
- `TestB1_DecodeWAV_AcceptsOrdinaryValidWAV` — sanity check.
- `TestB1_DecodeAudio_RejectsMalformedCompressed` — ffmpeg
  rejects bad input.
- `TestB1_DecodeAudio_CancelKillsSubprocess` — cancellation
  path fires (skipped when ffmpeg unavailable).
- `TestB1_DecodeCompressed_DeadlineKillsProcess` — wall-clock
  deadline fires.
- `TestB1_DecodeAudio_NoLeakTempFiles` — DecodeAudio writes
  no files.

### Result

All 11 tests pass under `-race`. The previously-passing
`TestServer_Transcribe_RejectsUnsupportedCodec` was updated to
use `audio/aac` (now genuinely unsupported) instead of `audio/webm`
(which is now a supported compressed codec).

## B2 — real Python adapter load/inference

### Defect

`src/sthira_v2/speech_asr_adapter.py` and
`src/sthira_v2/speech_tts_adapter.py` were hardcoded to BLOCKED
state. They had no real load attempt; the Go side could not
exercise the actual ONNX / transformers loading path.

### Fix

- ASR adapter probes `STHIRA_ASR_ARTIFACT_DIR` for the pinned
  AI4Bharat IndicConformer-600M-Multi artifact directory and
  loads the encoder/joint ONNX sessions via `onnxruntime` when
  the artifact is present. The model is loaded lazily on the
  first `transcribe` op so the `ready` envelope responds
  immediately.
- TTS adapter probes `STHIRA_TTS_ARTIFACT_DIR` and loads the
  Parler-TTS model via `transformers.AutoModel.from_pretrained`
  (with `trust_remote_code=false` so no remote code is fetched).
- When the artifact directory is missing or incomplete, both
  adapters emit an honest `{"status":"blocked","reason":"..."}`
  envelope with the specific missing-file list. No auto-download.
- The Python TTS adapter's module name in the Go default config
  was the wrong `sthira_v2.speech_tts`; corrected to
  `sthira_v2.speech_tts_adapter` in
  `backend/internal/ttsworker/runtime_adapter.go`.

### Tests

`tests/test_b2_adapters.py` (8 tests):

- `test_b2_asr_artifact_gate_blocks_without_artifacts`
- `test_b2_asr_transcribe_returns_error_without_artifacts`
- `test_b2_asr_envelope_shape_when_artifact_present`
- `test_b2_asr_real_artifact_opt_in_NOT_RUN`
- `test_b2_tts_artifact_gate_blocks_without_artifacts`
- `test_b2_tts_transcribe_returns_error_without_artifacts`
- `test_b2_tts_envelope_shape_when_artifact_present`
- `test_b2_tts_real_artifact_opt_in_NOT_RUN`

The tests run the Python adapters as subprocesses with
`PYTHONPATH=src` so the in-development code is exercised.

### Unverified claim

Real inference (weights loaded, audio transcribed or text
synthesized) is NOT_RUN until `STHIRA_ASR_ARTIFACT_DIR` and
`STHIRA_TTS_ARTIFACT_DIR` point at the pinned, O03/O11-approved
artifact directories. The tests prove the load attempt fires and
the wire envelope is correct; they do not assert model quality.

## B3 — subprocess correlation, concurrency, cancellation

### Defect

Both ASR and TTS adapters spawned a `Scanner` reader goroutine
then released the exchange mutex before reading the response.
Concurrent pool calls could read another request's stdout. TTS
lacked request ID correlation entirely. The 256 KiB scanner cap
rejected valid audio beyond roughly 4.45 seconds while 12
seconds is allowed.

### Fix

`backend/internal/asrworker/runtime_ipc.go` and
`backend/internal/ttsworker/runtime_ipc.go` introduce a shared
`ipcDispatcher` type:

- One goroutine owns the subprocess stdout scanner and routes
  each parsed response to the matching pending request via
  request_id.
- The mutex serializes the complete exchange (write + register
  + map lookup), not just the write. Concurrent requests can no
  longer cross-consume responses.
- Cancellation deregisters the request; the late response is
  dropped on the floor.
- Scanner buffer cap = 1 MiB (12 s @ 22 050 Hz mono PCM16 + 33 %
  base64 + JSON overhead) — accepts valid 12 s audio.
- Subprocess exit / malformed response / wrong ID all surface as
  `ErrRuntimeUnavailable` and reset the dispatcher cleanly.

### Tests

`backend/internal/asrworker/b3_ipc_test.go` (8 tests) and
`backend/internal/ttsworker/b3_ipc_test.go` (7 tests):

- Concurrent distinct requests get their own results under
  `-race`.
- Cancel then immediate retry cannot consume the first
  response.
- Wrong / missing request ID is rejected.
- Subprocess exit releases all pending requests.
- Malformed response resets cleanly.
- Shutdown leaves no live demultiplexer goroutine.
- Near-limit audio succeeds; over-limit is rejected.
- 50-concurrent run with `-race` exercises the demultiplexer
  under heavy contention.

## B4 — TTS /health wire shape + Sarvam chat_template

### Defect

`backend/internal/ttsworker/worker.go`'s `Health()` returned an
internal shape (`runtime_languages`, `inventory`,
`current_source_version`, `metrics`) that did not match
`contracts.WorkerHealth`. The orchestrator had to special-case
TTS even though ASR and middle already conformed.

Separately, the Sarvam-30B chat template was a constant with no
wire path: `SarvamConfig` populated fields but the chat_template
never reached the vLLM request body. Reasoning controls could
not be exercised end-to-end.

### Fix

TTS `WorkerHealth` now uses the same field names as
`contracts.WorkerHealth` (`ready`, `warm`, `models`,
`artifacts`, `supported_languages`, `queue`, `started_at`,
`build_revision`). Internal diagnostics move under `omitempty`
so they appear only when populated.

The Sarvam chat template is wired into the request path:
`HTTPClientRuntimeConfig.ChatTemplate` →
`ProposeInput.ChatTemplate` →
`chatCompletionRequest.ChatTemplate` (omitempty). The template
now uses Gemma-style turn tokens
(`<|start_of_turn|>...<|end_of_turn|>`) per the documented
Sarvam-30B model card (the previous `<|im_start|>` shape was
honest Qwen-style and not derived from the pinned artifact).
Deployment-time verification of the exact tokens is documented
inline; if vLLM rejects them, the worker surfaces 400 MALFORMED
or 422 SCHEMA_UNSUPPORTED.

### Tests

`backend/internal/ttsworker/b4_health_test.go` (2 tests):

- `TestB4_TTS_HealthWireShapeMatchesContractsWorkerHealth`
  asserts the required fields (`ready`, `warm`, `models`,
  `artifacts`, `supported_languages`, `queue` with `depth /
  max_depth / max_concurrency`) are present and the legacy
  `runtime_languages` is absent.
- `TestB4_TTS_HealthEnvelopeIsJSONObject` sanity check.

`backend/internal/middleworker/b4_sarvam_test.go` (1 test):

- `TestB4_SarvamChatTemplateWiredIntoRequestPath` records the
  outbound request body via a fake vLLM and asserts the
  `chat_template` field is present and contains
  `enable_thinking = false` plus the Gemma-style turn tokens.
  Also asserts the field is omitted when the runtime config has
  no override.

## B5 — eval provider wire shapes match contracts

### Defect

`backend/eval/provider/http.go` used invented wire shapes:

- `PipelineEnvelopeBridge` sent `{request_id, language, input,
  render, context, case_id}` — the actual
  `contracts.PipelineRequest` requires `{request_id,
  jurisdiction, language, input, render, idempotency_key}`; the
  loose `context` map and `case_id` are not in the contract.
- `PipelineResponseEnvelope` decoded flat
  `{status, intent, actions, speech_key, ...}` — the actual
  `contracts.PipelineResponse` returns `{state,
  validated_proposal: ModelOutput, template, audio}`. Status /
  intent / actions live inside `validated_proposal`.
- `TTSEnvelopeBridge` sent `{args, settings}` with free-text
  args — the actual `contracts.TTSWorkerRequest` takes
  server-rendered text only; client-supplied free text is a
  security violation (R17).
- ASR provider sent raw audio bytes in the body with metadata
  in headers — the actual `contracts.ASRWorkerRequest` is a
  JSON envelope with `audio_b64`.

### Fix

`backend/eval/provider/http.go` rewires each provider against
the contract types. Because the eval module is stdlib-only and
cannot import `backend/internal/contracts`, the wire types are
hand-mirrored (`ASRWorkerResponseWire`, `PipelineResponseWire`,
`ModelOutputWire`, `ActionWire`, `PipelineTemplateWire`,
`PipelineAudioWire`, `TTSWorkerResponseWire`).

`backend/eval/corpus/schema.go` gains two new fields:
`Jurisdiction` and `TemplateVersion` (both `omitempty`). The
existing tests do not need to migrate because the new fields are
optional.

### Tests

`backend/eval/provider/http_integration_test.go` (5 tests):

- `TestB5_HTTPProvider_ASR_SendsRealEnvelope` — asserts the
  ASR provider sends the typed ASRWorkerRequest JSON and
  decodes the typed ASRWorkerResponse (with `model_revision`
  populated). Verifies `request_id`, `language`,
  `content_type`, `audio_b64` are forwarded.
- `TestB5_HTTPProvider_Middle_SendsPipelineRequest` — asserts
  the Middle provider sends the typed PipelineRequest envelope
  (with `jurisdiction`, `source_version`, no `case_id`) and
  decodes the typed PipelineResponse. The runner's reconcile
  fields come from `validated_proposal` and `template`.
- `TestB5_HTTPProvider_Middle_RejectsMissingJurisdiction` —
  fail-closed: missing jurisdiction is rejected before any
  HTTP call (D36 boundary).
- `TestB5_HTTPProvider_TTS_SendsTTSWorkerRequest` — asserts
  the TTS provider sends the typed TTSWorkerRequest envelope
  with `speech_key`, `jurisdiction`, `source_version`,
  `template_version`. Asserts no `args` field is sent (free
  text is forbidden).
- `TestB5_HTTPProvider_TTS_RejectsMissingJurisdiction` —
  same fail-closed boundary for TTS.

The existing 503 / empty-audio / bad-base64 tests are preserved
with the new wire shape.

## B6 — smoke harness, security/SBOM tooling

### k6 smoke harness

`loadmodel/k6/smoke_real.js`:

- Adds explicit `connect_failures` (rate), `http_5xx_failures`
  (count), `http_4xx_expected` (count) metrics. The
  `http_req_duration p(95)<1000` threshold is preserved.
- Thresholds: `connect_failures: rate==0`,
  `http_5xx_failures: count==0`. A startup failure (binary
  down, port collision) is reported as FAIL via the rate
  threshold rather than silently absorbed by the `status<500`
  check that also accepts status 0.

`loadmodel/k6/smoke_business.js` (new, opt-in):

- Exercises a successful `POST /api/v3/places/resolve` round
  trip and asserts a non-empty JSON body. The previous smoke
  only probed health + the expected 4xx outcome of empty
  fixtures.

`scripts/run_k6_smoke_real.sh`:

- Allocates a free loopback port via `python3 socket binding`
  instead of hard-coding `127.0.0.1:18443`, so concurrent
  runs do not collide and team services are not pre-empted.
- Refuses to run if PostgreSQL is unreachable on `localhost`.
- Migration failures now exit 3 (was previously masked by the
  overall pipefail).
- `ON_ERROR_STOP=1` is preserved.
- Optional `--business-flow` flag runs the new
  `smoke_business.js` after `smoke_real.js`.

### Security checks script

`scripts/run_security_checks.sh`:

- Per-module Markdown summary at
  `plan/evidence/security-checks.md`. The legacy
  `plan/evidence/security-checks.txt` is preserved unchanged
  (FAIL record kept).
- Structured JSON ledger at
  `plan/evidence/security-checks.json` (single valid JSON
  array, top-level `{schema, timestamp, overall, records}`).
- Tool versions are resolved before invocation so each record
  carries the actual binary version that ran.
- Missing optional scanners (govulncheck / gosec /
  staticcheck / gitleaks / trivy) are reported `NOT_RUN`,
  not failed. The script honors scanner exit codes: a
  scanner that exits non-zero is FAIL, an exit-0 with no
  findings is PASS, an exit-1 with no output is FAIL (crash).
- Fixed the `gofmt -l` check: with `pipefail` enabled, the
  upstream gofmt's SIGPIPE masked the actual grep match. The
  check now captures the output directly and inspects its
  length.

### SBOM root metadata

`scripts/gen_spdx.py`:

- The root module's `downloadLocation` is no longer the
  fictitious `https://proxy.golang.org/sthira/backend/@v/.zip`
  URL; it is `NOASSERTION` because the build does not publish
  a tarball. An honest SPDX value is required.
- For stdlib-only modules with no `sthira/...` root entry, a
  minimal `SPDXRef-Root` placeholder is emitted so the
  document remains structurally valid.

## Per-module test results

```
=== asrworker ===
ok  	sthira/backend/internal/asrworker	36.052s
=== ttsworker ===
ok  	sthira/backend/internal/ttsworker	4.439s
ok  	sthira/backend/internal/ttsworker/templates	1.502s
=== middleworker ===
ok  	sthira/backend/internal/middleworker	4.604s
ok  	sthira/backend/internal/middleworker/eval	1.460s
=== eval ===
ok  	sthira/backend/eval/corpus	1.422s
ok  	sthira/backend/eval/provider	1.465s
ok  	sthira/backend/eval/report	1.430s
ok  	sthira/backend/eval/runner	1.448s
=== loadmodel ===
ok  	sthira/backend/loadmodel/loadfixtures	1.456s
ok  	sthira/backend/loadmodel/voice/voiceload	2.057s
=== python (PYTHONPATH=src pytest tests/test_b2_adapters.py) ===
8 passed in 0.21s
```

All `gofmt -l .` checks pass for the owned modules:
`backend/internal/asrworker`, `backend/internal/ttsworker`,
`backend/internal/middleworker`, `backend/eval`, `loadmodel`.

The remaining `FAIL` from `scripts/run_security_checks.sh` is
in `backend/internal/offlinequeue/worker.go`,
`backend/internal/offlinequeue/worker_regression_test.go`,
and `backend/internal/httpserver/graceful_shutdown_test.go` —
all files in Worker A's lane, not modified here.

## Cross-boundary contract notes for Worker A

- **TTS /health**: now emits the contract fields
  (`models`, `artifacts`, `supported_languages`, `queue`,
  `started_at`, `build_revision`). The orchestrator's
  `HealthEnvelope` decoder should expect these names; if it
  previously special-cased TTS internals, that special-case
  can be removed.

- **Sarvam chat_template**: the `chatCompletionRequest`
  JSON now includes a top-level `chat_template` field when
  the runtime config provides one. The orchestrator's
  middle proxy (if it adds one) should not strip this field.

- **TTS /synthesize request body**: the eval provider now
  sends the typed `TTSWorkerRequest` JSON (with
  `speech_key`, `jurisdiction`, `source_version`,
  `template_version`, `deadline_ms`, `idempotency_key`).
  The real worker-side `ttsworker/server.go::handleSynthesize`
  already decodes this same shape; verify it tolerates the
  `jurisdiction` and `idempotency_key` fields (currently
  ignored — they are advisory for the worker, authoritative
  for the orchestrator).

- **ASR /transcribe request body**: now a JSON envelope with
  `audio_b64`. The worker-side `handleTranscribe` already
  accepts this shape.

- **Python adapters**: the module path in the Go default
  config is `sthira_v2.speech_asr_adapter` and
  `sthira_v2.speech_tts_adapter`. The TTS module was the
  wrong `sthira_v2.speech_tts` (B3 fix). Production wiring
  should match.

- **Real subprocess IPC**: the new `ipcDispatcher` is in
  `runtime_ipc.go` per package. The dispatcher mutex
  serializes the complete exchange; no caller can race on
  a partial state. Worker A's orchestrator must not assume
  ASR/TTS adapters can serve concurrent calls from a single
  runtime (they can, but the bounded queue at the worker
  level still applies).

## Tests Worker A must run after integration

Per the prompt's "Tests it must run after integration":

1. `go test -race -count=1 ./...` for `backend` (orchestration,
   store, httpserver, offlinequeue, contracts) — verify the
   typed handler integration with the worker HTTP boundary.
2. `go test -race -count=1 ./...` for each of
   `backend/internal/asrworker`,
   `backend/internal/middleworker`,
   `backend/internal/ttsworker`, `backend/eval`,
   `loadmodel` — these all pass on `codex/p567-inference-corrections`
   at HEAD.
3. `pytest tests/test_b2_adapters.py` with
   `PYTHONPATH=src` — 8 tests pass.
4. `scripts/run_k6_smoke_real.sh` — exits 0 with PASS evidence
   in `plan/evidence/loadrun/smoke_real_summary.json`.
5. `scripts/run_k6_smoke_real.sh --business-flow` — additionally
   exercises `smoke_business.js`.
6. `scripts/run_security_checks.sh` — exits 0 if every
   owned module is gofmt-clean and every installed scanner
   passes. The pre-existing gofmt drift in
   `backend/internal/offlinequeue/...` and
   `backend/internal/httpserver/...` is Worker A's lane and
   will continue to FAIL until fixed.

## Unverified claims (NOT_RUN)

- **Real ASR inference** — blocked on O03 (AI4Bharat
  IndicConformer-600M-Multi language matrix approval and
  licensed artifact). The load attempt fires; no real
  audio is transcribed.
- **Real TTS inference** — blocked on O11 (Indic Parler-TTS
  regional-language voice review). The load attempt fires;
  no real audio is synthesized.
- **Real GPU / latency / throughput** — blocked on
  hardware. The Sarvam-30B FP8 deployment is configured but
  not exercised end-to-end.
- **Language accuracy** — blocked on the same hardware /
  artifact gates. Synthetic 20/20 evaluation harness results
  are not real language accuracy.
- **Security scanners (govulncheck, gosec, staticcheck,
  gitleaks, trivy)** — NOT_RUN because the binaries are
  not installed in this environment.
- **Plan/architecture compliance with the orchestrator's
  full handler** — Worker A owns the orchestrator-side
  integration tests; my tests exercise the typed envelopes
  against httptest stubs, not against the real handler.

## Stop

No work proceeds to P8. Gate B remains NOT_READY; the
corrections in this handoff are bounded defect fixes, not
phase completion.
