# P6 contract freeze

2026-09-21. Frozen by the P6 coordinator on `CLEAN` at base commit `bf17515`
(commit immediately preceding this document). This is the contract that
authorizes independent P6 engineering; P6 acceptance remains gated. P5
corrections (Areas A–G) are integrated at this base and remain in their
state — the P5 acceptance matrix is the source of truth for P5, not this
document.

The P5 worker's tasks (Areas A–G) and the P6 workers' tasks (six parallel
lanes) may both proceed. This document deliberately defines ONLY the
boundaries the six workers need to compile and integrate against. It does
not invent features, claim real-inference evidence, or approve any
authority-owned data.

## Base commit

`bf17515 docs(plan): split P5 corrections and P6-P7 parallel work`

This is the commit every P6 worker creates its `codex/<task-name>` branch
from. The P6 contract types in `backend/internal/contracts/` and the
additive OpenAPI slices in `backend/contracts/openapi.yaml` are committed
ON TOP of this base as the coordinator's contract-freeze commit (reported
in the coordinator's response and recorded in the ledger). Workers do not
amend the freeze; they consume it.

## File ownership for the six P6 workers

| Worker | Task name | Owns (writes) | May NOT write |
| --- | --- | --- | --- |
| W4 | `p6-context` | `backend/internal/contracts/context*.go`, `backend/internal/store/context*.go`, `backend/internal/store/place*.go`, `backend/internal/store/choice*.go` (extension only) | `backend/internal/contracts/model.go`, `backend/contracts/openapi.yaml`, `backend/internal/httpserver/server.go`, `cmd/sthira/main.go`, migrations, `backend/go.mod` |
| W5 | `p6-asr` | `backend/internal/asrworker/` (new directory), `backend/internal/asrworker/go.mod`, ASR private protocol shapes reused from `backend/internal/contracts/transcription.go` and `worker_health.go` (READ-ONLY) | `backend/contracts/openapi.yaml`, `backend/internal/httpserver/*`, shared `backend/go.mod`; may PROPOSE deltas via the coordinator handoff |
| W6 | `p6-middle` | `backend/internal/middleworker/` (new directory), `backend/internal/middleworker/go.mod`, structured-output schema file referenced from `backend/internal/contracts/middle_protocol.go` (READ-ONLY) | `backend/contracts/openapi.yaml`, shared `backend/go.mod`; may PROPOSE Go client deltas via the coordinator handoff |
| W7 | `p6-tts` | `backend/internal/ttsworker/` (new directory), `backend/internal/ttsworker/go.mod`, approved-template registry consumed from `backend/internal/contracts/tts.go` (READ-ONLY) | `backend/contracts/openapi.yaml`, template TEXT inside `backend/internal/contracts/`; template text is the coordinator's contract — workers reference it but never edit it |
| W8 | `p6-evaluation` | `backend/eval/` (new directory), corpus schema/fixtures, runner harness | Any production validator code, any worker implementation |
| W9 | `p6-orchestration` | `backend/internal/orchestration/` (new package), dedicated HTTP handler file under `backend/internal/httpserver/voice_process.go` (new file only); registry of wiring deltas | `backend/internal/httpserver/server.go`, `cmd/sthira/main.go`, `backend/contracts/openapi.yaml`, shared `backend/go.mod` |

Shared files (`backend/go.mod`, `backend/go.sum`, `backend/migrations/*.sql`,
`backend/contracts/openapi.yaml`, `backend/internal/httpserver/server.go`,
`cmd/sthira/main.go`, `plan/prompt.md`) remain under coordinator ownership.
Workers PROPOSE exact edits in their handoff Markdown file. P5 publication
code (`backend/internal/store/publication.go`,
`backend/internal/offlinedelivery/`, `backend/internal/httpserver/publication_adapter.go`)
is owned by the P5 lane and out of scope for P6 — P6 borrows the
publication seam (manifest_version / source_version / template_version)
without re-implementing it.

## Resolved concrete boundaries

### 1. Trust context beyond flat `KnownIDs`

The current `ValidateModelOutput` accepts a flat `map[string]bool` of known
IDs. That is insufficient for P6 semantics: an ID alone cannot say whether
it is a route, a facility, a safe zone or a red zone; whether a route is
verified, valid, unclosed and bound to a specific safe zone; or whether a
facility has a permitted order. The coordinator defines a richer
`contracts.ScopedContext` (see `backend/internal/contracts/context.go`):

- Typed `PlaceCandidate`, `RouteRef`, `FacilityRef`, `ZoneRef` (each with
  the minimum fields P6 needs: ID, jurisdiction, version, status/freshness,
  binding relationships).
- Typed `EligibleChoice` (facility + safe zone + verified-route references
  + capacity-known + permitted-order). The `EligibleDestinations` slice is
  the single source of permitted order for `SHOW_CHOICES`.
- `AllowedLanguages []string` and `TemplateKeys []string` are explicit
  slices (not maps); order is not semantic.
- `SourceVersion int` and `TemplateVersion int` are explicit so workers can
  revalidate without re-parsing the package body.
- `SnapshotRevalidate(ctx) error` is the seam Worker 4 implements so
  Worker 9 can revalidate after slow inference.

Worker 4 owns the file; Worker 9 calls it; the validator in
`contracts/model.go` (existing) stays as-is for raw output shape; the new
semantic validator lives in `contracts/scoped.go` (Worker 4). The validator
must additionally enforce:

- A `RouteRef` referenced by `SHOW_ROUTE` must exist in `VerifiedRoutes`
  and be `Verified && !Closed && ValidNow`.
- A `FacilityRef` cannot appear in `SHOW_CHOICES` (only valid for
  `OPEN_PANEL DESTINATION_PREVIEW`).
- A `RouteRef` cannot appear in `SHOW_CHOICES`.
- The `PermittedOrder` of `EligibleDestinations` must match the slice the
  model returns (same IDs, same order); reorder → rejection.

Worker 4 MAY extend `ResolveContext`/`ResolveAnyOperationalContext`
(read-extensions only); Worker 4 MAY NOT redefine authoritative snapshot
selection logic (D36 already owns that boundary).

### 2. `/api/v3/voice/commands` compatibility

The existing `/api/v3/voice/commands` endpoint contract is preserved
bit-for-bit: same request body shape (`request_id`, `data_version`,
`jurisdiction`, `proposal`), same response envelope, same status codes.
P6 does not introduce a v2 or a new endpoint there. P6 adds three new
endpoints (see below) and one new internal pipeline endpoint used only by
the orchestration handler in process. The middleware model output schema
remains `3.0`; the `ModelOutput` type does not change; `Validation_*` and
status codes do not change.

### 3. Transcript / audio entry points, response envelopes, cancellation

Three new public endpoints (additive only). The endpoint paths live under
`/api/v3/voice/...` to match the existing convention. All three are
strictly bounded:

#### `POST /api/v3/voice/transcriptions`

Accept an audio upload and return a transcript. This is the ASR entry
point. The current OpenAPI names `/voice/transcriptions` only in the
historical Python stub; the Go implementation is fresh.

Request:

- Headers: `Content-Type: audio/wav | audio/webm | audio/ogg` (negotiated
  compressed mono). `X-Request-ID` (optional; if absent the server
  assigns one). `X-Language` (required): an enabled context language.
- Body: bounded compressed audio bytes. Limits:
  - Compressed bytes: ≤ 512 KiB.
  - Decoded mono duration: ≤ 20 seconds.
  - Sample rate: 8 kHz – 48 kHz (resampled server-side or rejected).
  - Channels: 1 (mono only; multi-channel rejected).
  - PCM WAV: 16-bit signed little-endian.
- The MIME type and the `X-Request-ID` header are the ONLY client-supplied
  identifiers. The body is NOT trusted as JSON; it is decoded and
  transcoded under the request context.

Response (typed envelope `contracts.TranscriptionResponse`):

- `request_id`, `data_version` (= current source snapshot), `language`,
  `text` (transcript), `confidence` (nullable, unknown semantics:
  `null` ⇒ "not calibrated"; absence from the response body is the
  SAME as explicit `null`; an explicit value of `0` would be a bug —
  confidence is unknown or in `[0,1]`),
  `alternatives` (bounded optional list, max 3, only populated when the
  ASR artifact supports n-best), `state` (`OK | UNSUPPORTED_LANGUAGE |
  AUDIO_UNAVAILABLE | TIMEOUT | UNAVAILABLE`).
- Errors (typed):
  - 400 `INVALID_VALUE` for unsupported MIME / multi-channel /
    oversize / zero-length.
  - 415 `UNSUPPORTED_MEDIA_TYPE` for content-type not in the allow list.
  - 422 `LANGUAGE_UNSUPPORTED` for languages outside the enabled set.
  - 503 `MODEL_UNAVAILABLE` if the ASR worker is not ready.
  - 503 `AUDIO_UNAVAILABLE` for worker-side decode failures.
  - 504 for an explicit `TIMEOUT` from the worker.
- Cancellation: the request context propagates to the ASR worker. A
  client disconnect cancels the worker call.

The handler does NOT store the transcript server-side; persistence is the
caller's job (orchestration owns it). It does NOT log audio bytes; it does
log `request_id`, language, state, and bounded timing.

#### `POST /api/v3/voice/process`

The full voice pipeline. Orchestration handler. Used by the frontend
client to drive: audio → ASR → scoped context → middle model →
independent validator → approved template → optional TTS. Worker 9 owns
the handler. Typed request/response is frozen in
`backend/internal/contracts/pipeline.go`:

Request:

- `request_id` (string, server-assigned if absent).
- `jurisdiction` (string, required).
- `language` (string, required, must be in `ScopedContext.AllowedLanguages`).
- `input` is ONE of:
  - `{"kind":"audio", "content_type":"audio/wav", "body_b64": "..."}` —
    base64-encoded bounded audio (same codec/duration limits as the
    transcription endpoint).
  - `{"kind":"transcript", "text":"...", "confidence": 0.83}` — direct
    text input with optional confidence (nullable / unknown OK).
- `render` is ONE of:
  - `{"kind":"none"}` (return only the validated proposal — no audio).
  - `{"kind":"tts"}` (also synthesize the approved-template text).
- `idempotency_key` (string, optional but recommended).

Response:

- `request_id`, `data_version`, `validated_proposal`
  (`contracts.ModelOutput`), `template` (`{speech_key, args}`),
  `audio` (when `render=tts`: `{audio_id, content_type, byte_size,
  checksum_sha256, cache_hit, language, voice_version,
  template_version, source_version}`), `state` (`OK | CLARIFY |
  UNSUPPORTED | DATA_UNAVAILABLE | MODEL_UNAVAILABLE | CANCELED`).

Errors:

- 400 `INVALID_VALUE` for missing jurisdiction / language / input kind.
- 409 `STALE_VERSION` if the orchestrator revalidates and the source
  snapshot changed since the proposal was issued.
- 503 `MODEL_UNAVAILABLE` if any required worker (ASR/middle/TTS) is
  not ready; the response indicates which stage failed.
- 504 `TIMEOUT` for explicit per-stage or total-deadline exhaustion.
- The endpoint performs NO write, call, capacity mutation or outbound
  network other than to the private workers.

The orchestrator assigns correlation IDs, owns the per-stage deadlines,
and discards stale results (a model response that arrives after a
withdrawal during inference is dropped, not delivered).

#### `POST /api/v3/voice/speech`

Synthesize an approved template to audio. TTS entry point.

Request:

- `request_id` (string).
- `speech_key` (string, must be in `ScopedContext.TemplateKeys`).
- `language` (string, must be in `ScopedContext.AllowedLanguages`).
- `args` (object: only validated template-arg IDs/values; arbitrary text
  is rejected).
- `source_version` (int, must equal current source version).

Response:

- `audio_id` (content-addressed), `content_type` (`audio/wav`),
  `byte_size`, `checksum_sha256`, `cache_hit` (bool),
  `language`, `voice_version`, `template_version`,
  `source_version`, `model_revision`, `synthesis_settings`
  (deterministic snapshot of synthesis parameters).

Errors:

- 400 `INVALID_VALUE` for unknown template args / wrong arg types.
- 409 `STALE_VERSION` for a withdrawn source_version.
- 422 `LANGUAGE_UNSUPPORTED` / `VALIDATION_FAILED` for languages outside
  the approved set or unknown speech_key.
- 503 `MODEL_UNAVAILABLE` if TTS is not ready.
- 503 `AUDIO_UNAVAILABLE` for synthesis failure (without retry).

TTS receives only Go-approved text (the result of rendering the
template with the validated args) and the template/source versions;
it never receives arbitrary public input.

### 4. Context revalidation, stale/out-of-order results, withdrawn audio

- Worker 4 implements `ScopedContext.SnapshotRevalidate(ctx)` which
  re-reads the persisted snapshot for the same `(jurisdiction, source_id)`
  and returns `ErrStaleSnapshot` if `SourceVersion` or `TemplateVersion`
  changed. Worker 9 calls it after every stage that returns a result.
- Worker 9 owns a correlation map keyed by `(request_id, stage)`:
  results are accepted only if their `request_id` matches a still-pending
  request, and the request has not been canceled; otherwise the result is
  discarded and a metric is incremented. Out-of-order results are dropped.
- Withdrawn audio: TTS audio carries `source_version` and
  `template_version` in the cache key (Worker 7 owns the cache).
  Withdrawn guidance is invalidated by the P5 publication channel;
  Worker 7 wires the cache invalidation to the publication event source
  (via `PublicationSource` interface). Worker 9 records the
  `source_version` in the response so clients can detect a stale audio
  even without a cache hit.

### 5. Worker directory boundaries

P6 workers occupy separate directories and own their private dependencies:

```
backend/
  internal/
    asrworker/         W5  (private go.mod, isolated from main module)
    middleworker/      W6  (private go.mod, isolated)
    ttsworker/         W7  (private go.mod, isolated)
    orchestration/     W9  (part of main module, no private deps)
    contracts/
      context.go       COORD (this freeze)
      scoped.go        W4 (validator built on contracts.ScopedContext)
      transcription.go COORD (typed ASR envelope)
      pipeline.go      COORD (typed pipeline envelope)
      tts.go           COORD (typed TTS envelope)
      middle_protocol.go COORD (typed middle worker protocol)
      worker_health.go COORD (typed worker health/queue shape)
  eval/                W8  (no production code; fixtures only)
```

Shared `backend/go.mod` adds ONE new direct dependency per worker only via
the integration stage; the workers' private `go.mod` files are isolated and
the coordinator integrates the wiring after acceptance.

`backend/migrations/` is coordinator-owned. Any new migration that P6 needs
is proposed by the relevant worker in its handoff and committed by the
coordinator during integration. The current `SchemaRevision` is 6.

### 6. P5 seams without P5 wire-format changes

P5 wire formats (offline delivery manifest/card/resource, publication
signing) are unchanged. P6 borrows the `SourceVersion` from the package
metadata and the `TemplateVersion` is a new P6 concept (a template
revision tied to the source version). Worker 7 derives its cache key from
`(source_version, template_version, speech_key, language, voice_version,
synthesis_settings)` so P5 withdrawals invalidate the audio cache through
the P5 publication event source without P5 knowing about P6 internals.

## New error codes

Added to `backend/internal/contracts/errors.go` (additive only, do not
redefine or reorder existing codes):

- `TRANSCRIPT_UNAVAILABLE` (503)
- `AUDIO_UNAVAILABLE` (503)
- `MODEL_TIMEOUT` (504)
- `QUEUE_SATURATED` (503, retryable)
- `INFERENCE_CANCELLED` (499, client closed)
- `TEMPLATE_UNKNOWN` (422)
- `STALE_SNAPSHOT` (409)

These cover the P6 worker error taxonomy and the orchestrator's
cancellation/error paths. Existing codes (`MODEL_UNAVAILABLE`,
`LANGUAGE_UNSUPPORTED`, `STALE_VERSION`, `VALIDATION_FAILED`) are reused
where they already fit.

## Private worker protocol

The Go orchestrator speaks to each private worker over a typed HTTP
protocol. Worker code lives in private `go.mod`s but uses the same
contract shapes. Each worker exposes:

- `GET /health` → `WorkerHealth` (see `contracts/worker_health.go`):
  `ready bool`, `models []ModelInfo`, `artifacts []ArtifactDigest`,
  `supported_languages []string`, `queue_depth int`,
  `max_concurrency int`, `warm bool`.
- `POST /transcribe` (ASR) / `POST /v1/chat/completions` (middle) /
  `POST /synthesize` (TTS) — bounded request/response with the same
  `request_id` the public handler saw.
- `POST /shutdown` (graceful drain) — returns 200 only after pending
  requests complete or hit the deadline.

Worker → Go events:

- Worker 5/6/7 may emit `ArtifactLoaded` / `ArtifactRevoked` events via
  the publication source seam (D52, D55).

The Go orchestrator treats worker URLs as untrusted by default; it
authenticates with a per-worker bearer token from environment
configuration (Worker 5/6/7 keys — not present by default; absent token
fails closed at startup). Endpoints bind to `127.0.0.1` in foundation
mode; external network exposure is NOT enabled.

## Acceptance matrix

The contract is accepted when every row passes against the freeze commit:

| # | Requirement | Path under test | Acceptance signal |
| --- | --- | --- | --- |
| A1 | The new typed contracts compile against the existing `/api/v3/voice/commands` and stay-internal callers. | `go build ./...` after the freeze commit | exit 0 |
| A2 | Existing voice-command validation tests still pass. | `go test ./internal/contracts ./internal/httpserver -count=1` | exit 0 |
| A3 | Existing OpenAPI route test still passes. | `go test ./internal/httpserver -run TestServedRoutesMatchOpenAPI -count=1` | exit 0 |
| A4 | The new typed envelopes marshal/unmarshal losslessly to the documented JSON. | `contracts/model_test.go` plus a new `contracts/pipeline_test.go` and `contracts/tts_test.go` golden table | exit 0 |
| A5 | `ScopedContext` is JSON-roundtrip stable (a fixture load produces the same struct). | `contracts/context_test.go` | exit 0 |
| A6 | New error codes are present and documented. | `contracts/errors.go` symbol list | grep matches |
| A7 | The OpenAPI additive slices for the three new endpoints lint clean (no removal of existing paths). | `backend/contracts/openapi.yaml` and `TestServedRoutesMatchOpenAPI` | exit 0 |
| A8 | The freeze document records base commit, ownership and the seven resolved boundaries. | this file | text review |
| A9 | No shared file outside the contract ownership list is modified by this commit. | `git show --stat <freeze>` | review |
| A10 | No `.txt` file is modified by this commit. | `git show --stat <freeze>` | review |

Workers MUST compile and test against this freeze. They MUST NOT modify
it; they request amendments via the coordinator handoff.

## Worker launch instructions

When the freeze commit is reported, six P6 workers are released in parallel
as separate worktrees (one per `codex/<task-name>` branch from the freeze
commit). Each worker receives:

1. The Common instructions from `plan/p6-p7-parallel-prompts.md`.
2. The frozen base commit (the freeze commit, not `bf17515`).
3. Its own worker prompt (Worker 4 / 5 / 6 / 7 / 8 / 9) verbatim.
4. A pointer to `plan/p6-contract.md` (this file).
5. A pointer to the typed contracts in `backend/internal/contracts/` —
   workers do not edit those files; they consume the public types.

Workers implement, run their build/vet/tests, and return a handoff in
the lane-specific Markdown file the coordinator assigns (one per lane;
no global tracker; no cross-lane edits).

The coordinator stops here. No worker results are invented. P5 corrections
remain IN_PROGRESS and are owned by their lanes; P6 acceptance is gated
until the integrated stage reports real per-worker evidence.
