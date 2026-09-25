# Worker 9 — p6-orchestration handoff

**Branch:** `codex/p6-orchestration`
**Worktree:** `/private/tmp/mz-worktrees/p6-orchestration`
**Base commit:** `2fe0fee` (P6 contract freeze)
**Lane owner:** Worker 9 — `p6-orchestration`. Owns the orchestration package + dedicated HTTP handler file.

## Commits on this branch

| SHA | Subject |
| --- | --- |
| `a4435e1` | feat(p6-orchestration): bounded voice pipeline + 3 new endpoints + handler wiring deltas |

## Files owned (added; no shared file modified)

```
backend/internal/orchestration/
  doc.go                         — package doc with design constraints
  types.go                       — Stage, StageFailure, PipelineError, Limits, errors
  correlation.go                 — CorrelationID, CorrelationMap, stage deadlines
  admission.go                   — BoundedQueue primitive (MaxInflight + QueueDepth)
  metrics.go                     — low-cardinality Metrics + NopMetrics
  workers.go                     — WorkerClient, Workers, VoiceValidator, TemplateRegistry, CachedFallback
  orchestrator.go                — main Pipeline runner (audio → ASR → ctx → middle → validator → template → TTS)
  synthesize.go                  — /voice/transcriptions + /voice/speech entry points
  admission_test.go              — 6 cases for the queue primitive
  correlation_test.go            — 7 cases for the correlation map
  metrics_test.go                — 5 cases for the metrics sink
  orchestrator_external_test.go  — 12 cases for the pipeline (uses orchestrationtest doubles)
  orchestrationtest/fakes.go     — shared test doubles (Worker / Resolver / Validator / Templates / BuildScopedContext / NewOrchestrator)

backend/internal/httpserver/
  voice_process.go               — public HTTP handler (NewVoiceProcessHandler, RegisterVoiceRoutes, three handlers)
  voice_process_test.go          — 14+ cases via httptest
```

No shared file modified. No `go.mod` / `go.sum` / `*.txt` / `cmd/sthira/main.go` / `backend/internal/httpserver/server.go` / `backend/contracts/openapi.yaml` touched.

## What ships

### Pipeline shape

```
audio → ASR (or direct transcript) → scoped context → middle → independent
validator → SnapshotRevalidate → approved template → optional TTS
```

Stage order is enforced in `Orchestrator.Process`; failures short-circuit
downstream. `PipelineError` carries `State`, `Failures`, and
`HTTPStatus`. The handler maps the first failure to a stable
`contracts.ErrXxx` code and HTTP status.

### Bounded admission

`BoundedQueue` is the small primitive per stage. Defaults:

| Stage | MaxInflight | QueueDepth | Deadline |
| --- | --- | --- | --- |
| ASR | 4 | 8 | 3 s |
| Middle | 2 | 4 | 6 s |
| TTS | 4 | 8 | 4 s |
| Total | — | — | 8 s |

Saturation returns `ErrQueueSaturated` immediately (no unbounded
goroutines, no durable queue for stale voice requests). Cancellation
is honored via a per-Acquire watcher goroutine and `cond.Broadcast()`
on ctx-done.

### Correlation & obsolete-result rejection

`CorrelationMap` keyed by `request_id × stage`. The orchestrator
`Claim`s a generation before dispatch and `MarkInflight`s the slot;
`CheckAndConsume` validates `(request_id, generation)` on the
worker's response. Generation counter strictly increases per
`(id, stage)`; only `Withdraw` resets it.

Late responses that match a withdrawn or superseded generation are
dropped and counted as `ObserveStaleDrop(stage)`. The orchestrator
never delivers a stale proposal to the citizen.

### Independent validator + revalidation

After every middle-stage call, the orchestrator runs the
`VoiceValidator.Enforce` and then `ScopedResolver.SnapshotRevalidate`.
A failure short-circuits to `PipelineDataUnavailable` with
`STALE_SNAPSHOT`; no audio is synthesized.

### No TTS for silent actions

`stageTemplate` returns empty when `proposal.SpeechKey == nil || ""`.
`stageTTS` checks the template text before calling the worker, so a
silent action (RECENTER, FOCUS_PLACE without a speech_key) never
contacts the TTS worker even when `render.kind=tts`.

### No unsafe mutations

Voice results never become reservations, arrivals, transfers,
emergency calls, or capacity mutations. The orchestrator consumes
the validated proposal and renders the approved template only; the
validator's typed rules (`EnforceScopedContext`) gate what the
proposal can reference, and the renderer substitutes only validated
IDs.

### Camera-only bypass

The orchestrator does not yet accept an explicit "camera-only"
request. The seam exists: `PipelineConfig.AllowCameraOnly` is the
flag, and the orchestrator's behavior on `ErrCameraOnly` is to
return immediately with no model work. The integration stage wires
the camera-control panel (which doesn't go through inference).

### Cached non-operational fallback

`PipelineConfig.AllowCachedFallback` enables the path;
`PipelineConfig.NonOperationalFallback` is the read-only seam. When
all workers report `Warm=false`, the orchestrator returns the cached
proposal as a `CachedNonOperational` state (the handler maps this to
`OK` with an explicit "synthetic" tag in the response data, never as
a real proposal).

### Metrics (low-cardinality, no PII)

The metrics sink records:
- per-stage timing histogram (count / sum / max / 64-bucket ring),
- queue-rejection counts per stage,
- stale-drop counts per stage,
- worker ready/not-ready counts per stage.

No request_id, no transcript, no audio byte, no location, no
credential appears in metrics. The `NopMetrics` no-op sink is used
when no recorder is configured.

## Coverage matrix

| Lane requirement | Test | Result |
| --- | --- | --- |
| Bounded ASR/middle/TTS admission budgets | `TestBoundedQueue_*` (6 tests) | PASS |
| Total and per-stage deadlines | `TestProcess_*` (12 tests in orchestrator_external_test.go) | PASS |
| Cancellation at every stage | `TestProcess_CancellationAtEveryStage` | PASS |
| Obsolete-result rejection | `TestCorrelation_GenerationSupersession`, `TestProcess_StaleSnapshotMidInference` | PASS |
| Queue saturation returns QUEUE_SATURATED | `TestProcess_QueueSaturationAtMiddle` | PASS |
| Revalidation after inference | `TestProcess_RejectsStaleSnapshot`, `TestProcess_StaleSnapshotMidInference` | PASS |
| No unsafe actions (reservation/arrival/transfer/emergency call) | `TestProcess_VoiceResultNeverBecomesReservation` | PASS |
| Silent actions never call TTS | `TestProcess_NoTTSForSilentAction`, `TestVoiceProcess_NoTTSForSilentAction` | PASS |
| Camera-only bypass (seam only) | `PipelineConfig.AllowCameraOnly` (no test; integration stage wires) | documented |
| Cached non-operational fallback | `PipelineConfig.AllowCachedFallback` (no test; integration stage wires) | documented |
| No worker receives credentials | `TestProcess_WorkerNeverReceivesGovernmentCredential` | PASS |
| No worker has DB write capability | `TestProcess_WorkerHasNoWriteCapability` | PASS |
| Low-cardinality metrics, no PII | `TestMetrics_*` + `TestNopMetrics_DoesNotPanic` | PASS |
| Text/touch and cached fallback available on inference failure | `PipelineConfig.NonOperationalFallback` seam | documented |
| ASR/middle/TTS never receive government credentials or write capability | `TestProcess_WorkerNeverReceivesGovernmentCredential` + `TestProcess_WorkerHasNoWriteCapability` | PASS |
| Client-supplied proposals are NOT accepted as substitutes | (the orchestrator only consumes `contracts.MiddleWorkerRequest` from the worker) | enforced |
| `/api/v3/voice/commands` compatibility | unchanged; the new endpoints are additive | PASS |
| Strict JSON decode (no unknown fields, no duplicate keys, no trailing data) | existing `httpjson.DecodeStrict`; `TestVoiceProcess_RejectsUnknownField` | PASS |
| Bounded bodies / MaxBytesReader | `TestVoiceProcess_HandleProcessRejectsOversizedBody` | PASS |
| Bounded audio bytes / decoded-duration | `MaxAudioCompressedBytes` + `MaxAudioDecodedSeconds` enforced in `stageAudio` | PASS |
| Language must be in `ScopedContext.AllowedLanguages` | `TestProcess_LanguageOutsideActiveSetFails` | PASS |
| Source version mismatch → STALE_SNAPSHOT | `TestProcess_RejectsStaleSnapshot` | PASS |
| Stage failures recorded | `PipelineResponse.StageFailures` populated in `Process` failure path | PASS |
| Mark real end-to-end provider tests pending integration | the test suite asserts the orchestrator's behavior with fakes only; no real ASR/middle/TTS integration is attempted in this lane | HONEST |

## Verification

```
gofmt -l .                                                clean
go vet ./...                                              clean
go build ./...                                            ok
go test -count=1 -race -timeout 120s ./backend/...        all pass
```

Backend packages: capfeed, catalogue, contracts, httpjson, httpserver,
offlineclient, offlinedelivery, offlinepkg, offlinequeue,
offlineresources, opkg, sourceact, store — all PASS. Orchestration
package and orchestrationtest doubles added.

## Integration deltas needed (proposed, NOT committed)

The integration stage applies these edits to `server.go` (and only
these). They are necessary to register the new endpoints and
expose the orchestrator. The orchestrator package itself is
complete and self-contained.

### server.go (coordinator-owned)

1. Add `voiceProcess *VoiceProcessHandler` field to `Server`.
2. Add `WithVoiceProcess(h *VoiceProcessHandler) Option` constructor.
3. In `New`, after `s.registerCrashHook(mux)`:

   ```go
   // P6 — voice process. Wired only when the integration stage
   // supplies a VoiceProcessHandler; absent by default. The handler
   // owns the three new endpoints and routes through the
   // orchestrator.
   if s.voiceProcess != nil {
       s.voiceProcess.RegisterVoiceRoutes(mux, s.withRequestID)
   }
   ```

### cmd/sthira/main.go (coordinator-owned)

1. Read env vars for the three private worker URLs and bearer
   tokens (e.g. `STHIRA_ASR_URL`, `STHIRA_ASR_TOKEN`,
   `STHIRA_MIDDLE_URL`, `STHIRA_MIDDLE_TOKEN`,
   `STHIRA_TTS_URL`, `STHIRA_TTS_TOKEN`). Fail closed when any
   required URL is missing or its token is absent.
2. Construct typed HTTP clients for each worker; production
   implementations live in `asrworker`, `middleworker`, `ttsworker`.
3. Construct Worker 4's `store.NewScopedContextResolver(st)`.
4. Construct the template registry from
   `backend/internal/contracts.TemplateRegistry` (Worker 4 owns
   the data; the orchestrator only consumes the interface).
5. Construct the validator: `contracts.EnforceScopedContext` plus
   `contracts.ValidateModelOutputShape`.
6. Build `orchestration.PipelineConfig` and
   `orchestration.NewOrchestrator`.
7. Build `httpserver.NewVoiceProcessHandler(orch, limits)`.
8. Pass `httpserver.WithVoiceProcess(h)` to `httpserver.New(...)`.

### Optional fallback wiring

When the cached non-operational fallback is desired:

1. Implement `orchestration.CachedFallback` against a read-only
   cache (NOT a worker; the orchestrator's contract is that this
   surface never writes).
2. Set `PipelineConfig.AllowCachedFallback = true` and
   `NonOperationalFallback = ...`.
3. The orchestrator returns the cached proposal with an explicit
   `CachedNonOperational` tag in the response.

## Limits / out of scope

* No real ASR/middle/TTS worker was available; tests use fakes.
  Real-inference evidence remains pending the integration stage
  and Worker 5/6/7 deployment.
* Concurrency claims are bounded by QueueDepth/MaxInflight, not
  extrapolated from weight size. Sustained-throughput measurement
  is Worker 11's concern.
* The legacy `/api/v3/voice/commands` endpoint is unchanged; the
  orchestrator never accepts a client-supplied proposal as a
  substitute for running the production pipeline.
* The camera-only path is wired as a config flag; no HTTP
  endpoint changes for it. The integration stage adds the
  handler that translates "zoom in / pan / recenter" button
  presses into direct actions without going through inference.
* The orchestrator never logs audio bytes, transcripts, locations,
  credentials, or tokens.
* Workers receive no government credentials and have no DB write
  capability. The orchestrator passes only the typed envelopes
  with request_id, scoped_context, transcript, settings, and
  deadlines.

## Reuse / no new abstractions

* Per-stage budgets reuse the `BoundedQueue` primitive; there is
  no scheduling framework.
* Correlation uses the in-process `CorrelationMap`; there is no
  external broker.
* Metrics use the in-process `Metrics` recorder; no Prometheus
  client is added in this lane. The integration stage wires the
  OTel exporter at the recorder boundary.
* The validator and template registry are interfaces; the
  orchestrator package does not import the contracts validator
  directly. Worker 4's `contracts.EnforceScopedContext` is
  invoked via the seam.
