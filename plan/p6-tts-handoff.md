# Worker 7 — P6 approved templates, TTS and audio invalidation

Worker 7 (`p6-tts`) owns `backend/internal/ttsworker/`, a private
Go module that implements the frozen private TTS protocol from
`plan/p6-contract.md` and consumes the typed envelopes from
`contracts.TTSWorkerRequest`, `contracts.TTSWorkerResponse` and
`contracts.WorkerHealth`. The Go module is isolated from
`backend/go.mod` so the orchestrator integrates without taking on
synthesis-runtime dependencies.

Branch: `codex/p6-tts` from base `2fe0fee` (P6 contract freeze).

## What shipped

**Templates (`backend/internal/ttsworker/templates/`):**
- Frozen template catalog with per-template `Status`
  (`APPROVED | PENDING_REVIEW | SYNTHETIC_ONLY | WITHDRAWN`).
- Each template declares:
  - exact `Languages []string` allow-list,
  - `ArgSchema map[string]ArgType` (string/int only), and
  - `Translations map[string]string` per language.
- Renderer takes a typed `[]Argument` (validated)
  — never an arbitrary user map. Substitution is bounded:
  `{name}` placeholders only, no recursion, no conditionals,
  no string-key parsing at render time.
- Argument validation rejects empty strings, non-integer floats,
  unknown arg names, special characters that could smuggle SSTI
  payloads (`<`, `>`, `'`, `"`, `\\`, `;`, etc.).
- Test/demo templates marked `SYNTHETIC_ONLY` are rejected by
  default; an explicit `AllowSynthetic(true)` toggle is required
  to render them. Synthetic content never reaches a production
  worker.
- No filler acknowledgments for map movement. The catalog only
  carries reviewed, structured templates.
- `Withdraw(k, version)` flips a template to `WITHDRAWN`; cached
  audio for that key is no longer served.
- `Snapshot()` deep-copies the catalog; the test
  `TestSnapshotIsolatesAfterMutation` proves concurrent mutation
  cannot leak into a snapshot.

**Worker (`backend/internal/ttsworker/`):**
- `ParlerTTS` artifact inventory with metadata-only fields:
  `ModelID`, `Revision`, `License`, `Runtime`, `Hardware`,
  `RemoteCode` (must be `false`), `Voices`, `SupportedLanguages`.
- `DefaultParlerTTS()` advertises `LicensePending`, `Runtime
  LicensePending`, `Hardware LicensePending` — refuses `Ready=true`
  until authorized artifacts land.
- `ScanLocalInventory(root, parlertts)` probes the local cache
  without reading `.safetensors` or `.bin` payloads. Bounded
  ≤ 64 KiB SHA-256 probe per small file; missing files surface
  as `MISSING_ON_DISK`.
- `Inventory.Ready()` gates on `PendingCritical == 0`,
  `License != LicensePending`, `Runtime`/`Hardware` recorded,
  `Revision` recorded, `SupportedLanguages` non-empty,
  `RemoteCodeTrust == false`. Anything less → no `Ready=true`.
- `SourceVersionBroadcaster` (`StandaloneSourceVersionClock`)
  surface. The cache subscribes to `OnAdvance(ctx, fn)`; on every
  monotonic `Advance(to)` the clock fires the registered listener
  OUTSIDE the clock mutex so the cache's `handleSourceAdvance`
  can acquire its own mutex without deadlocking against a
  concurrent `Get`.

**Runtime (`backend/internal/ttsworker/runtime.go`):**
- `Runtime` interface: `Synthesize(ctx, text, lang, voice)`,
  `Close`, `Languages()`, `Revision()`, `Voice(lang)`, `Voices()`.
- `StubRuntime` (deterministic, in-process). Silence → empty
  text; non-silence → canned marker; confidence always `nil`.
  Used by tests; never produces bytes.
- `SubprocessRuntime` (structural stub). Construction does not
  spawn anything; every `Synthesize` call returns
  `ErrRuntimeUnavailable`. Wiring the real adapter is gated on
  the authorized-artifact acceptance criteria.
- Typed errors the runtime returns: `ErrRuntimeUnavailable`,
  `ErrLanguageUnsupported`, `ErrVoiceUnsupported`,
  `ErrRuntimeClosed`, `ErrDeadlineExceeded`. The worker maps
  each to a typed TTSState.

**Audio (`backend/internal/ttsworker/audio.go`):**
- Pure-Go PCM 16-bit signed little-endian WAV encoder.
- Constants: `DefaultOutputSampleRate = 22050`,
  `MaxOutputSampleRate = 24000`,
  `MaxOutputDurationSeconds = 12.0`,
  `MaxOutputBytes = 256 KiB`,
  `MaxSynthesizedTextBytes = 1024`.
- `EncodeSilenceWav(rate, duration)` is a worker primitive,
  NOT a synthesizer. Used to build canonical test fixtures
  with deterministic content-addressed checksums.
- `Float32ToWav([]float32, rate)` rejects out-of-range samples
  with a typed error rather than silently clipping.

**Cache (`backend/internal/ttsworker/cache.go`):**
- Cache identity: `(text, template_key, template_version,
  source_version, language, model_revision, voice_revision,
  synthesis_settings)`. JSON-encoded canonical.
  Every field change → distinct identity string. The cache
  identity encodes the typed `contracts.TTSCacheKey` shape so
  a parameter change deterministically invalidates the cache.
- LRU + max bytes (default 16 MiB). TTL (default 24 h).
- `Get` honors two eviction triggers atomically under the codec
  mutex:
  1. TTL expiry (`now - created_at > ttl`).
  2. **Source-version advance**: if `current > entry.source_version_at_entry`,
     the entry is dropped and `Get` returns `false`.
- The wired `OnAdvance` listener fires on every source-version
  bump and walks the cache dropping entries whose
  `SourceVersionAtEntry` precedes the new version. This is the
  REAL invalidation path the prompt requires; `InvalidateBySourceVersion`
  is the helper retained for diagnostics (and labeled as such
  in the test).
- `Identity.Validate()` rejects partial identities so a missing
  field cannot silently map to an existing entry from a
  different request.

**Worker lifecycle (`backend/internal/ttsworker/worker.go`):**
- `New(cfg)` validates queue/concurrency bounds and inventory
  readiness, then spawns the worker pool at construction
  (`MaxInFlight` goroutines, no per-request spawning).
- `Synthesize(req)` returns immediately on
  `ErrWorkerNotReady`/`ErrWorkerShutdown`/`ErrQueueSaturated`.
  Honors per-call deadline via `makeContext(deadlineMillis)`.
- Hot path is cache-only. Missing cache entry → `AUDIO_UNAVAILABLE`,
  NOT a synthesized waveform. Production pre-generates approved
  common speech via the offline workflow which calls
  `runtime.Synthesize` and `codec.Put` directly (then scrubs
  sourceVersionAtEntry to the current clock).
- Source-version check happens AT REQUEST RECEIPT so a
  withdrawal that fires while a request is in flight cannot
  produce a successful synthesis.
- `handleJob` re-checks template status (`Withdrawn`) against
  the catalog at the moment of the job — an audit-stage
  withdrawal between orchestrator render and worker pickup is
  honored.
- `Shutdown()` is idempotent.

**HTTP server (`backend/internal/ttsworker/server.go`):**
- `GET  /health` → typed `WorkerHealth` snapshot.
- `POST /synthesize` → typed request/response.
- `POST /shutdown` → graceful drain; idempotent.
- Bearer-token authentication enforced whenever a token is
  configured; absent token means tests run unauthenticated and
  production fails closed at startup (orchestrator refuses
  to dispatch without a bearer).
- 1 MiB request body cap (configurable). `http.MaxBytesReader`
  translates `*http.MaxBytesError` to 413.

## Inventory of Parler-TTS (metadata only — no tensors read)

| Field             | Value                                          |
| ----------------- | ---------------------------------------------- |
| ModelID           | `ai4bharat/indic-parler-tts`                   |
| PretrainedFile    | `model.safetensors` (or `pytorch_model.bin`)   |
| LocalPath         | `models/indic-parler-tts` (configured)         |
| License (claim)   | Apache-2.0 (claimed upstream; pending verifier)|
| Revision          | pending — no authorized digest recorded        |
| Runtime           | pending                                        |
| Hardware          | pending                                        |
| RemoteCode        | false (production invariant)                   |
| Voices            | `ml-IN-female-1` (and others; listed by runtime)|
| SupportedLanguages| derived from runtime adapter (NOT model name) |

Per-language intelligibility is NOT claimed. The runtime
adapter — once authorized — exposes the language allow-list.
Production deploys without an authorized runtime reject
`Ready=true`. Worked example:

```
DefaultParlerTTS()'s Inventory is NOT Ready by definition:
  - License == "LicensePending"
  - Runtime  == "LicensePending"
  - Hardware == "LicensePending"
  - Revision == ""
  - SupportedLanguages == nil
```

The Parler-TTS multi-language claim from the model name is
intentionally NOT taken at face value. The runtime's exposed
language list is the only source of truth.

## Wiring deltas proposed for the integration coordinator

Worker 9's integration stage will need to:

1. Pull-in the worker's directory: the private `go.mod` is
   self-contained, so this is a directory pull-in, NOT a
   `go.mod` edit on `backend/go.mod`.
2. On startup, Worker 9 reads `STHIRA_TTS_WORKER_URL` and
   `STHIRA_TTS_WORKER_TOKEN` from env, builds the HTTP client,
   and refuses to start when either is missing.
3. `/voice/speech` handler maps the public
   `contracts.TTSRequest` to the worker's wire envelope
   (mirrored shape), forwards `X-Sthira-Request-ID`, treats
   `STALE_VERSION` and `AUDIO_UNAVAILABLE` as documented. Maps
   typed worker states to `contracts.TTSState` for the public
   response.
4. The `/voice/speech` handler validates `args` against the
   template's `ArgSchema` before forwarding; the worker
   re-validates and the worker's `Renderer` is the last word on
   what reaches synthesis. The orchestrator passes
   `args` as the structured `map[string]any`; the worker
   renders the `text` field and may produce a different value
   than the orchestrator sent (e.g. a withdrawal means the
   worker returns `STALE_VERSION` regardless of what the
   orchestrator sent).
5. The wired `SourceVersionBroadcaster` MUST be backed by the
   P5 publication channel. Until that wiring lands, the worker
   uses `StandaloneSourceVersionClock` (a single-process mock)
   and tests exercise it directly. Wiring it to P5 publication
   semantics is part of the coordinator's integration step;
   the worker's seam (`SourceVersionBroadcaster`) does not
   need to change.

## What does NOT ship yet (explicit)

- **`runtime.Synthesize` is offline-only.** Hot path serves
  approved cache entries only. Synthesizing on demand would
  contradict the "do not fake audio as successful speech"
  rule. The bridge from offline workflow to cache is by
  definition a separate code path that production deploys
  against the authorized runtime.
- **Real inference is BLOCKED.** The `SubprocessRuntime`
  returns `ErrRuntimeUnavailable` on every call. Wired
  synthesis adapters (Python or native) are gated on:
  - Authorized artifact + license verification recorded in
    `Inventory.HashMatches` (no `LicensePending` left).
  - Verified language intelligibility on the candidate list
    (`ml-IN`, `hi-IN`, etc.) is a Worker-8 evidence task;
    W7 does not claim this until W8 reports it.
  - A bounded subprocess harness with stdin/stdout framing,
    per-request timeout, and strict success envelope.
- **Confidence field is always nil.** The reference Python
  TTS adapter does not exist; W7 has no calibration source.
- **Cache storage on disk** is currently in-memory only. The
  cache may be persisted by the integration stage but must
  honor the same source-version invalidation pattern.

## Tests

60 tests, all passing under `-race -timeout 60s`:

- 14 templates: register, withdraw, render, validation, snapshot,
  unknown key, wrong language, pending review, synthetic only,
  undeclared translation, injection attempts, schema mismatch,
  Args.
- 7 audio: silence WAV header, oversize duration / sample rate,
  out-of-range samples, byte-budget checks, content addressing.
- 7 cache: identity string canonical, identity changes per
  field, store and serve, LRU eviction under budget, **wired
  source-version advance drops entries**, withdrawal race
  concurrent Get+Advance, helper-level purge, TTL expiry.
- 6 inventory: empty root, partial presence, default
  inventory not Ready, license pending, RemoteCode must be
  false, component table.
- 3 runtime: stub refuses synthesize, subprocess stub refuses
  synthesize, subprocess has no languages.
- 11 worker: hot path serves cache, hot path miss is
  AUDIO_UNAVAILABLE, stale source version is refused,
  unknown template, unsupported language, withdrawn template,
  queue saturation, idempotent shutdown, post-shutdown
  rejection, **withdrawal-races-serve concurrent
  deterministic**, voice not supported, health snapshot.
- 12 server: health 200, reject bad method, bearer required,
  bad JSON, empty language, oversize body → 413, invalid JSON,
  shutdown drains existing, shutdown bad method,
  synthesize OK on cache hit (full end-to-end round trip).

`gofmt -l .`, `go vet ./...`, and `go build ./...` all clean.

## Verification

```
cd backend/internal/ttsworker
go test ./...                          # 60 tests pass
go test ./... -count=1 -race           # 60 tests pass, no races
gofmt -l .                             # clean
go vet ./...                           # clean
go build ./...                         # clean
```

The module has zero third-party dependencies; stdlib only.

## Files added

```
backend/internal/ttsworker/doc.go
backend/internal/ttsworker/go.mod
backend/internal/ttsworker/inventory.go
backend/internal/ttsworker/inventory_test.go
backend/internal/ttsworker/audio.go
backend/internal/ttsworker/audio_test.go
backend/internal/ttsworker/cache.go
backend/internal/ttsworker/cache_test.go
backend/internal/ttsworker/runtime.go
backend/internal/ttsworker/runtime_test.go
backend/internal/ttsworker/worker.go
backend/internal/ttsworker/worker_test.go
backend/internal/ttsworker/server.go
backend/internal/ttsworker/server_test.go
backend/internal/ttsworker/testruntime_test.go
backend/internal/ttsworker/testbytes_test.go
backend/internal/ttsworker/templates/templates.go
backend/internal/ttsworker/templates/templates_test.go
backend/internal/ttsworker/templates/errors.go
```

No edits to existing files outside `backend/internal/ttsworker/`.
No shared `backend/go.mod` edits. No shared `backend/migrations`
edits. No `.txt` edits. No edits to contracts, server, or
orchestration code.
