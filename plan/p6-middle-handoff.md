# Worker 6 — p6-middle handoff

**Branch:** `codex/p6-middle`
**Worktree:** `/private/tmp/mz-worktrees/p6-middle`
**Base commit:** `2fe0fee` (P6 contract freeze)
**Lane owner:** Worker 6 — `p6-middle`. Owns `backend/internal/middleworker/` (new directory) and `backend/internal/middleworker/go.mod`.

## Commits on this branch

| SHA | Subject |
| --- | --- |
| `56b96f6` | feat(p6-middle): private middle-model worker + bounded vLLM client + eval driver |

## Files touched (all owned by this lane; no shared file modified)

```
backend/internal/middleworker/
  client.go
  client_test.go
  decode.go
  decode_test.go
  doc.go
  go.mod
  runtime.go
  runtime_http.go
  runtime_stub.go
  runtime_subprocess.go
  schema/model_output.schema.json
  server.go
  server_test.go
  wire.go
  worker.go
  worker_test.go
  eval/
    HARDWARE_BLOCKER.md
    corpus/synthetic.jsonl
    main.go
    main_test.go
```

No shared file modified. No `go.mod` / `go.sum` outside the lane touched.
No `.txt` file touched. No production server wiring changed.

## What ships

### Layout

* `backend/internal/middleworker/` is an isolated Go module
  (`go.mod` declares `module sthira/backend/internal/middleworker`,
  `go 1.23`). Stdlib-only dependencies.
* `schema/model_output.schema.json` is the pinned JSON Schema 2020-12
  for guided generation. The wire `Proposal` struct mirrors it; tests
  assert no drift.
* `eval/corpus/synthetic.jsonl` is a 15-case synthetic corpus with
  expected outcomes per case (silent focus, destination options,
  ambiguous place, no-verified-route, unsupported intent, silent
  recenter, prompt-injection, emergency-call attempt, forbidden-write
  attempt, repeat guidance, language change, expired snapshot,
  code-switched, claim-shortcut, simulated model outage).

### Client (bounded vLLM HTTP)

| Bound | Value | Source |
| --- | --- | --- |
| Max context tokens | 4096 (default) | Override per call |
| Max output tokens | 256 (default) | Override per call |
| Max response bytes | 64 KiB | Bounded read |
| Max request bytes | 64 KiB | Pre-flight check |
| Per-call deadline | 6 s default | http.Client.Timeout + ctx.Deadline |
| Retries | none | One attempt; 5xx/429 surface as ErrUnavailable |
| Paid fallback | none | n/a |
| Tool execution | none | Client does not pass tools |
| Remote-code trust | `false` | vLLM `--trust-remote-code false` |

Typed errors mapped to (HTTP status, MiddleState):

| Error | HTTP | MiddleState |
| --- | --- | --- |
| ErrMalformed | 400 | `MALFORMED` |
| ErrExtraText | 400 | `MALFORMED` |
| ErrSchemaUnsupported | 422 | `SCHEMA_UNSUPPORTED` |
| ErrContextExceeded | 413 | `CONTEXT_EXCEEDED` |
| ErrOutputExceeded | 400 | `OUTPUT_EXCEEDED` |
| ErrTimeout | 504 | `TIMEOUT` |
| ErrCanceled | 499 | `CANCELED` |
| ErrUnavailable | 503 | `UNAVAILABLE` |
| ErrOversized | 400 | `OUTPUT_EXCEEDED` |

### Worker

* Warm lifecycle: `LoadAndVerify` requires non-empty revision,
  non-empty digest and at least one language. Until that gate passes,
  `/health` reports `ready=false, warm=false`.
* Bounded queue (`QueueDepth`, default 8) + bounded concurrency
  (`MaxInFlight`, default 2). Construction refuses
  `MaxInFlight > QueueDepth`.
* Cancellation: `context.AfterFunc` binds the caller's `ctx.Done`
  to `job.cancel`. The handle goroutine watches `job.cancel` and
  cancels its own per-call context. No goroutine leaks on the
  happy path; `go test -race` is clean.
* Language allow-list: a request whose language is not in the
  runtime's reported list is rejected before contacting the runtime.

### Server

* Loopback HTTP only (caller-supplied address).
* Bearer-token auth from `STHIRA_MIDDLE_WORKER_TOKEN`; absent token
  fails closed at startup.
* Three endpoints: `GET /health`, `POST /v1/chat/completions`,
  `POST /shutdown`. The X-Sthira-State header carries the typed
  MiddleState on failures so the orchestrator can map it without
  parsing the error message body.
* Per-call deadline in the request body (`deadline_ms`). The worker
  derives its per-call context from the request context.

### Pinned first candidate (recorded, not executed)

| Field | Value |
| --- | --- |
| Model | `Qwen/Qwen3-4B-Instruct-2507` |
| License | Apache-2.0 (verified from model card, source register S01) |
| Parameters | 4.0B (verified) |
| Thinking mode | non-thinking |
| Trust remote code | `false` |
| vLLM image tag | NOT_EVALUATED (to be pinned during integration) |
| BF16 artifact SHA-256 | NOT_EVALUATED |
| AWQ-int4 artifact SHA-256 | NOT_EVALUATED |

The eval driver's `-mode manifest` is the authoritative artifact
record today. `-mode harness-check` prints reproducible commands for
the integration stage. `-mode benchmark` is gated on real artifacts.

## Verification

* `gofmt -l .` clean
* `go vet ./...` clean
* `go build ./...` ok (main module + middleworker module)
* `go test ./backend/... -count=1` all pass; full backend suite ok
* `go test ./backend/internal/middleworker -race` clean
* `go test ./backend/internal/middleworker -count=2 -race` clean
* No goroutine leaks under `-race`
* No edits to shared files (`backend/go.mod`, `backend/contracts/openapi.yaml`,
  `cmd/sthira/main.go`, `backend/internal/httpserver/server.go`,
  `backend/internal/contracts/*`, migrations)

## Coverage matrix

| Lane requirement | Test | Result |
| --- | --- | --- |
| Bounded context/output/HTTP/deadline/cancellation | `TestClient_*` (16 tests) | PASS |
| Structured-output syntax against pinned version | `TestDecodeStrictProposal_*` (17 tests) + `TestServer_ProposeUnknownFieldFailsClosed` | PASS |
| Strict output decoder, extra text rejected | `TestClient_ProposeExtraText` + `TestDecodeStrictProposal_ExtraTextRejected` + `TestDecodeStrictProposal_TrailingTextRejected` + `TestDecodeStrictProposal_MarkdownFenceRejected` + `TestDecodeStrictProposal_ExcessiveLeadingWhitespaceRejected` | PASS |
| Independent semantic validation outside the model adapter (decoder does NOT enforce semantic limits) | `TestDecodeStrictProposal_DoesNotEnforceSemanticLimits` | PASS |
| Treat transcripts and imported text as data; supply only scoped server candidates | `TestClient_ProposeInjectionShapedInput` | PASS |
| Pin actual image/model/tokenizer/revision; document licenses | `Pinned` + `eval/HARDWARE_BLOCKER.md` + `eval/corpus/synthetic.jsonl` | RECORDED |
| No invented image tags/digests or silent remote-code trust | `Pinned.trust_remote_code=false`, `vllm_image_tag=NOT_EVALUATED`, artifact SHA-256 fields empty | HONEST |
| Warm serving; private network; deployment starts unavailable if config missing | `TestWorker_LoadAndVerifyRequiresRevision` + `TestWorker_DispatchBeforeReadyRejected` + `TestWorker_SubprocessRuntimeStaysNotReady` | PASS |
| Adapter tests for timeout/cancellation/oversized/malformed/extra-text/schema-unsupported/injection-shaped | All covered in `TestClient_*` and `TestDecodeStrictProposal_*` | PASS |
| Repeatable evaluation invocation for BF16 + supported quantized candidate | `eval/main.go` with `-mode harness-check`, `-mode manifest`, `-mode benchmark` | COMPILED |
| Preserve individual failures and resource data | `eval/main.go -mode benchmark` records per-case outcomes; harness-check prints blocker | DESIGNED |
| Do NOT extrapolate concurrency from weight size | No concurrency claim in handoff or doc.go; concurrency is bounded by QueueDepth/MaxInFlight, not weight | HONEST |
| Real vLLM smoke/evaluation only when hardware/artifacts available | `eval/HARDWARE_BLOCKER.md` records the blocker; `-mode benchmark` exits 2 until artifacts are pinned | HONEST |
| No paid fallback, tool execution, credentials, network-enabled model tools, unbounded retries | `Client` has no paid-fallback path, no tools, no auth header, single attempt | CONFIRMED |

## Limits / out of scope

* No GPU, no model weights, no real vLLM server were available in
  the authorized scope. Real-model evidence is NOT_EVALUATED.
* The P6 orchestration package is owned by Worker 9; the
  orchestrator's call site for `Client` is proposed in this
  handoff (see Integration deltas needed below).
* The lane does not own the orchestrator's wire-envelope types
  (`contracts.MiddleWorkerRequest` etc.) — those are frozen and
  consumed by JSON wire format only.
* Optional concurrency tuning (real throughput measurement,
  KV cache sizing, sustained vs burst behavior) is Worker 11's
  concern; we do not claim a number here.

## Integration deltas needed (proposed, not committed)

Worker 9 (orchestration) needs to:

1. Add a typed client in `backend/internal/orchestration/` that
   wraps `middleworker.Client` with the
   `contracts.MiddleWorkerRequest`/`Response` types. The shared
   Go import is one-way (orchestration → middleworker). The
   worker module remains isolated.
2. On `/api/v3/voice/process`, derive a per-stage deadline
   (proposed: 6 s for the middle stage, total 8 s per
   `plan/parameters.md`).
3. Map `MiddleStateMalformed` to `PipelineState=DATA_UNAVAILABLE`
   with stage="middle" in `stage_failures`. Map
   `MiddleStateUnavailable` to `PipelineState=MODEL_UNAVAILABLE`.
   Map `MiddleStateTimeout` to `PipelineState=MODEL_UNAVAILABLE`
   plus an explicit `MODEL_TIMEOUT` error code from
   `contracts/errors.go`.
4. Reject client-supplied proposals (the orchestrator never
   accepts a proposal that didn't come through the middle
   worker).
5. Run `contracts.ValidateModelOutput` against the proposal
   BEFORE any downstream stage (template/TTS). The validator is
   the safety gate; the schema decoder is not.

Coordinator-owned changes:

* `cmd/sthira/main.go` — wire the orchestration handler to the
  private middle worker URL (env-driven, defaults to absent
  meaning MODEL_UNAVAILABLE on startup).
* `backend/internal/httpserver/server.go` — register the new
  handler. Out of lane scope.
* `backend/migrations/` — none required.
* `backend/contracts/openapi.yaml` — none required; the three
  new endpoints are owned by the orchestrator lane.

The integration agent collects lane handoffs from each worker's
branch before merging onto CLEAN.
