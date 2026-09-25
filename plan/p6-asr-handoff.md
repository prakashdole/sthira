# Worker 5 — P6 ASR worker and audio boundary

Worker 5 (`p6-asr`) owns `backend/internal/asrworker/`, a private
Go module that implements the typed private protocol from
`plan/p6-contract.md` and consumes only `contracts.{ASRWorkerRequest,
ASRWorkerResponse, WorkerHealth}` shapes via JSON. The Go module is
isolated from `backend/go.mod` so the orchestrator can integrate
without taking on inference-runtime dependencies.

Branch: `codex/p6-asr` from base `2fe0fee` (P6 contract freeze).
Commit on this branch is the W5 deliverable.

## What shipped

- `asrworker/audio.go` — pure-Go PCM 16-bit signed little-endian
  WAV decoder with strict header validation (RIFF / WAVE / fmt /
  data), block-align and channel count checks, mono-only mode
  with explicit multi-channel rejection, and an in-package linear
  resampler to 16 kHz mono. Oversize-byte, oversize-duration,
  truncated-header, truncated-data, and out-of-band sample rates
  are all rejected at the typed-error level.
- `asrworker/runtime.go` — the `Runtime` interface and two
  implementations:
    - `StubRuntime` — deterministic, in-process. Silence produces
      an empty transcript (canned marker ""). Non-silence produces
      a clearly synthetic canned marker ("[stub:non-silence]").
      Confidence is always nil. LatencyMs is configurable so
      deadline tests are deterministic.
    - `SubprocessRuntime` — structural stub. Constructing it does
      NOT spawn anything; the first `Transcribe` call returns
      `ErrRuntimeUnavailable` until a real adapter is wired in.
      This is the explicit "blocked real-inference" signal: the
      orchestrator sees `TranscriptionState = UNAVAILABLE` and
      returns a typed error to the client. The stub never produces
      a canned transcript on real inference paths.
- `asrworker/worker.go` — lifecycle:
    - `LoadAndVerify` requires non-empty PendingCritical == 0 from
      the inventory AND a non-empty revision/digest from the
      runtime AND a non-empty SupportedLanguages list. Fails
      closed on any miss.
    - `Dispatch` enforces per-request deadline, queue saturation,
      language allow-list, and per-call cancellation. A request
      whose context is cancelled never reaches the runtime
      (`TestDispatch_CancellationDoesNotInvokeRuntime`).
    - `Shutdown` drains the worker pool, closes the runtime, and
      is idempotent.
    - Concurrency is bounded: `MaxInFlight` is the worker pool
      size; `QueueDepth` is the channel capacity; both are
      validated at construction. Goroutines are spawned once, not
      per request.
- `asrworker/server.go` — HTTP front door:
    - `GET  /health` returns `WorkerHealth` JSON (ready, warm,
      models, artifacts, supported_languages, queue stats,
      started_at, build_revision, last_inventory_scan,
      pending_critical).
    - `POST /transcribe` decodes `contracts.ASRWorkerRequest`
      JSON, base64-decodes the audio, decodes the WAV, calls
      `Worker.Dispatch`, and writes a typed
      `contracts.ASRWorkerResponse`. Errors map to typed states:
        * `ErrUnsupportedCodec` → `AUDIO_UNAVAILABLE`
        * `DecodeError` (oversize/truncated/malformed) → `AUDIO_UNAVAILABLE`
        * `ErrLanguageUnsupported` → `UNSUPPORTED_LANGUAGE`
        * `ErrQueueSaturated` / `ErrWorkerNotReady` /
          `ErrWorkerShutdown` / `ErrRuntimeUnavailable` → `UNAVAILABLE`
        * `context.DeadlineExceeded` → `TIMEOUT`
        * `context.Canceled` → `CANCELED`
      The handler scrubs the audio buffer before returning.
    - `POST /shutdown` performs a graceful drain bounded by a 5s
      deadline; subsequent `/transcribe` calls return
      `UNAVAILABLE`.
    - Bearer-token authentication is enforced whenever the server
      was constructed with a non-empty token; absent token means
      tests run unauthenticated and production fails closed at
      startup (the orchestrator refuses to dispatch without a
      bearer).
- `asrworker/manifest.go` — IndicConformer artifact inventory.
    - Lists the four critical components (model_checkpoint,
      tokenizer, runtime, onnx_config).
    - `ScanLocalInventory` performs a bounded file-existence +
      ≤64 KiB SHA-256 probe of the configured LocalPath.
    - All recorded digests are PLACEHOLDERS (`HashZero`) until an
      authorized verifier records them. The orchestrator refuses
      Ready=true while any PendingCritical is non-zero.
    - Languages reported in `WorkerHealth.SupportedLanguages` are
      derived from the runtime, NEVER from the model name. The
      reference runtime restricts configured languages to
      `hi-IN` and `ml-IN` even though the artifact is
      multilingual.

## Inventory of IndicConformer (metadata only — no tensors read)

| Field              | Value                                                        |
| ------------------ | ------------------------------------------------------------ |
| ModelID            | `ai4bharat/indic-conformer-600m-multilingual`                |
| PretrainedArchive  | `ai4bharat/indic-conformer-600m-multilingual`                |
| LocalPath          | `models/indic-conformer-600m-multilingual`                   |
| LicenseRef (claim) | Apache-2.0 (claimed upstream; pending verification)          |
| ModelCheckpoint    | pending — no authorized digest recorded                      |
| Tokenizer          | pending — no authorized digest recorded                      |
| Runtime            | pending — no authorized digest recorded                      |
| ONNXConfig         | pending — no authorized digest recorded                      |
| SupportedLanguages | `hi-IN`, `ml-IN` (per reference runtime, NOT model name)     |
| RemoteCode         | false (the reference runtime imports a local .py file, not remote-code trust) |

The reference Python runtime (`src/sthira_v2/local_voice.py`,
`speech_stt.py`) imports the artifact via `importlib.util` at
`model_onnx.py`. That import is the gateway; if the file is
absent, the worker cannot advertise Ready=true.

The reference runtime returns confidence = 1.0 unconditionally
(see `local_voice.py:52`). Worker 5 reports confidence = nil from
this adapter until a calibrated source exists. The stub also
reports nil. Production must wire a real adapter that exposes
calibrated n-best before confidence becomes a populated value.

## What is NOT shipping yet (explicit)

The subprocess-backed real inference adapter is intentionally
absent. The `SubprocessRuntime` type is a structural stub that:

- never spawns Python;
- returns `ErrRuntimeUnavailable` to every `Transcribe` call;
- exposes empty Revision/Digest/SupportedLanguages;
- causes `LoadAndVerify` to fail with "runtime revision is empty";
- propagates `TranscriptionState = UNAVAILABLE` to the
  orchestrator.

This is the explicit "blocked real-inference" check the worker
prompt asks for. A test in `runtime_test.go` asserts every one
of these properties. We do not silently fall back to a canned
transcript.

Wiring the real adapter is gated on:

- O01/O05/O06/O07 closure (audio/data artifacts authorization).
- A subprocess harness (currently not present in this directory)
  that supports bounded stdin/stdout, a kill-after-deadline timer,
  and a strict success envelope.
- An authorized verifier recording real SHA-256 digests for the
  four critical components.

The handoff to Worker 9 (orchestration) notes this:

- /health will report `Warm=false, Ready=false` until the real
  adapter is wired in. The orchestrator's startup probe should
  treat this as `503 MODEL_UNAVAILABLE` for the ASR stage.
- A request reaching the ASR worker during the blocked window
  surfaces as `TranscriptionState = UNAVAILABLE`. The
  orchestrator's /voice/process and /voice/transcriptions
  endpoints translate that into the documented 503 and never
  into a transcript.

## Tests

58 tests, all passing under `-race -timeout 60s`:

- 15 audio decode tests (mono, tone resample, oversize,
  truncated, multi-channel rejection, out-of-band sample rate,
  unsupported codec, empty bytes, resampler identity / empty /
  upsample 2x, wipe).
- 9 runtime tests (unsupported language, silence→empty,
  non-silence→canned, deadline, Close→ErrRuntimeClosed,
  SubprocessRuntime blocked, empty Revision/Digest, idempotent
  Close).
- 11 worker tests (runtime required, inflight>queue rejection,
  empty-inventory-not-ready, subprocess-stub-not-ready,
  pre-ready, unsupported language, queue saturation, cancellation
  does not invoke runtime, deadline, shutdown, idempotent
  shutdown, snapshot, max in-flight observation at the runtime).
- 13 server tests (health 200, GET-only, bearer required, bad
  JSON, empty audio, unsupported codec, unsupported language,
  ok-on-valid-WAV, oversize body → 413, oversize audio at
  decode, no-leak, queue saturation → UNAVAILABLE, shutdown
  drains).
- 8 inventory / digest tests (empty root, missing root, partial
  presence, hash probe, default catalog, HasRecordedDigest,
  Ready requires languages, ComponentCritical table).

`go vet ./...` and `gofmt -l .` both clean.

## Verification

```
cd backend/internal/asrworker
go test ./... -count=1             # 58 tests pass
go test ./... -count=1 -race       # 58 tests pass, no races
gofmt -l .                          # clean
go vet ./...                        # clean
go build ./...                      # clean
```

The module has zero third-party dependencies; stdlib only.

## Wiring deltas proposed for the integration coordinator

Worker 9's integration stage will need to:

1. Add the worker's directory to the integration manifest: the
   private `go.mod` is self-contained, so this is a directory
   pull-in, NOT a `go.mod` edit on `backend/go.mod`.
2. On startup, Worker 9 reads `STHIRA_ASR_WORKER_URL` and
   `STHIRA_ASR_WORKER_TOKEN` from env, builds the HTTP client,
   and refuses to start when either is missing.
3. The /voice/transcriptions handler maps the worker's response
   into the public envelope from `contracts.TranscriptionResponse`.
   The mapping is one-to-one because the worker's wire shape
   mirrors it (this is why the worker keeps the JSON shapes
   locally rather than sharing a Go import).
4. The /voice/process handler treats `UNAVAILABLE` from ASR as
   `503 MODEL_UNAVAILABLE`. `UNSUPPORTED_LANGUAGE` maps to
   `422 LANGUAGE_UNSUPPORTED`. `TIMEOUT` maps to
   `504 MODEL_TIMEOUT`. `CANCELED` maps to `499
   INFERENCE_CANCELLED`. `AUDIO_UNAVAILABLE` maps to
   `503 AUDIO_UNAVAILABLE`.

## Honest limits (acknowledged)

- The codec allow-list at the worker is narrower than the public
  handler's. Until a real WEBM/OGG transcoder is available
  upstream of the worker, only `audio/wav` (PCM 16-bit LE mono)
  is accepted. The worker handler rejects the other two MIME
  types with a typed `AUDIO_UNAVAILABLE`. Production deployment
  must add a transcoding pre-step in the orchestrator if WEBM/OGG
  is required.
- The artifact digests are placeholders. Worker 5 honors the
  freeze's "do not import huge tensors just to inventory them"
  rule and records PLACEHOLDER digests until an authorized
  verifier records real ones.
- Confidence is always `nil` from this revision. This is the
  honest answer to the worker prompt's "do not copy the
  reference runtime's fabricated confidence=1.0" rule.
- Real inference evidence is BLOCKED, not fake-PASSed. The
  `SubprocessRuntime` tests assert this property explicitly.

## Files added

```
backend/internal/asrworker/doc.go
backend/internal/asrworker/go.mod
backend/internal/asrworker/manifest.go
backend/internal/asrworker/audio.go
backend/internal/asrworker/runtime.go
backend/internal/asrworker/worker.go
backend/internal/asrworker/server.go
backend/internal/asrworker/audio_test.go
backend/internal/asrworker/runtime_test.go
backend/internal/asrworker/worker_test.go
backend/internal/asrworker/server_test.go
backend/internal/asrworker/manifest_test.go
```

No edits to existing files outside `backend/internal/asrworker/`.
No shared `backend/go.mod` edits. No shared `backend/migrations`
edits. No `.txt` edits. No edits to contracts, server, or
orchestration code.
