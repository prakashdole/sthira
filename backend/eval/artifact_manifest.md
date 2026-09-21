# P6 evaluation harness — artifact manifest

This document records what `backend/eval/` owns and what it
deliberately does NOT own, for the P6 evaluation task (`p6-evaluation`).
Cross-check with `plan/p6-contract.md` (Worker-8 ownership column).

## Owned

| File / package | Purpose |
|----------------|---------|
| `backend/eval/go.mod` | Private module (`sthira/backend/eval`), stdlib only, Go 1.27 |
| `backend/eval/doc.go` | Package overview and the three load-time invariants |
| `backend/eval/cmd/eval-run/main.go` | Runner binary: loads, dispatches, reconciles, reports |
| `backend/eval/corpus/schema.go` | Case schema with provenance/consent/license and locked Expected outcomes |
| `backend/eval/corpus/load.go` | JSON loaders, audit hooks, repo discovery |
| `backend/eval/corpus/digest.go` | sha256 content digest for the audit trail |
| `backend/eval/corpus/schema_test.go` | Schema invariants (probe rules) |
| `backend/eval/provider/provider.go` | Stage-shape interface and `Result` definition |
| `backend/eval/provider/deterministic.go` | Deterministic rule-based provider (harness validation only) |
| `backend/eval/provider/deterministic_test.go` | Provider rule table coverage |
| `backend/eval/provider/http.go` | HTTP provider over frozen wire shapes (real-worker run only) |
| `backend/eval/provider/b64.go` | base64 shim — stdlib only |
| `backend/eval/runner/runner.go` | Run loop, warm/cold gate, reconcile, context propagation |
| `backend/eval/runner/runner_test.go` | End-to-end tests against the synthetic suite |
| `backend/eval/runner/testdata.go` | `loadBuiltIn` helper for tests |
| `backend/eval/report/report.go` | Aggregations + Markdown writer |
| `backend/eval/report/report_test.go` | Reporter invariants |
| `backend/eval/cases/synthetic/suite.json` | **Representative SYNTHETIC suite — 20 cases** covering every Category in the brief |
| `backend/eval/cases/real/requirements.md` | Consent, recording, reviewer requirements for real-language gates |
| `backend/eval/commands.md` | Launch runbook |

## Delivered counts

- 20 synthetic cases (one per Coverage dimension from the brief,
  including: AMBIGUOUS_LOCALITY, CODE_SWITCH, UNKNOWN_LOCALITY,
  UNKNOWN_LANGUAGE, NOISY_AUDIO, CLIPPED_AUDIO, NEAREST_ROUTE,
  OTHER_ROUTE, CLAIMED_SHORTCUT, ARRIVAL_SELF_REPORT,
  RESERVATION_REQUEST, EMERGENCY_CALL, PROMPT_INJECTION,
  INVENTED_ID, EXPIRED_SNAPSHOT, CANCELLATION, MODEL_OUTAGE,
  CAMERA_MOVE, REPEAT_GUIDANCE, CHANGE_LANGUAGE)
- 16 GO tests passing under `-race`
- 0 third-party deps
- 0 edits outside `backend/eval/`
- 0 edits to `contracts/`, `cmd/`, `migrations/`, `internal/`, or any
  worker module

## Deliberately NOT owned

| Concern | Owner |
|---------|-------|
| Real ASR worker | `codex/p6-asr` (Worker 5) at `0c96410` on `codex/p6-asr` |
| Real TTS worker | `codex/p6-tts` (Worker 7) shipped in this session |
| Real middle-model orchestration | Worker 9 (`p6-orchestration`) |
| Live category / language matrix | O03 (per `plan/open-decisions.md`) |
| Live ISL / qualified reviewer | O11 |
| Live hazard / facility data | O01, O05, O07 (Government sources) |
| Production validators | NOT in this directory |

## Constraints kept

1. **No real PII in source.** All committed cases carry
   `Provenance.Kind: SYNTHETIC`. CONSENTED cases load via an
   outside-repo manifest, never via source.
2. **No coordinator validators / worker adapters in this directory.**
   The runner uses a generic HTTP client + JSON envelopes that mirror
   the orchestrator's typed contract. Real adapter logic lives in the
   workers.
3. **Expected outcomes locked before execution.** Cases carry a
   `Probe()` invariant that fails them at load if any reconciliation
   field collides with the outcome class. The runner refrains from
   mutating Expected after dispatch.
4. **Real/synthetic separation.** `ModeSynthetic` filters
   CONSENTED, `ModeReal` filters SYNTHETIC. A single run never mixes
   the two populations.
5. **NOT_EVALUATED is honest.** With `n < MinSampleForClaim` the
   reporter refuses to project a fraction. A real-language run with
   zero reviewers prints `NOT_EVALUATED (n = 0)`, never
   `100% / 0%`.
6. **Stage timings separately recorded.** ASR / Middle / TTS each
   carry their own latency row, plus end-to-end. Aggregate p50/p95
   covers warm-up-filtered, non-ERROR, cold-aware timings.
7. **False-acceptance is a separate metric.** A case expected to
   refuse that produces an OK answer increments the FA rate. A
   reconcile-failure on the same expected outcome does NOT count
   (the harness caught it; the danger is in the cases it didn't).
8. **No invented translations.** Every translation used in a
   synthetic case is the literal text of the deterministic rule's
   `SpeechKey`. No auto-translated emergency wording ships.
9. **No duplicated adapters.** The HTTP provider only marshals the
   orchestrator's typed envelope; the worker process owns all
   adapter logic.

## Open dependencies

The runner is fully wired to operate against the ASR / TTS worker
HTTP envelopes. To run a real-inference sweep requires:

  1. W9 orchestration handler (`codex/p9-orchestration`) exposing
     `/api/v3/pipeline` typed envelope on a known port;
  2. Workers 5 (`p6-asr`) and 7 (`p6-tts`) binaries running on the
     configured URLs, each with its bearer token;
  3. O03 decision (regional language matrix) and O11 decision
     (qualified reviewers) — currently `OPEN` per
     `plan/open-decisions.md`. Until resolved, the real-run path
     reports `NOT_EVALUATED` with the empty-population honesty rule
     documented above.

## Where the synthetic suite's coverage map lands

| Brief category | Synthetic case |
|----------------|----------------|
| Ambiguous village names | `syn.ambig.place.village-name-collision.001` |
| Code-switching | `syn.codeswitch.hi-en.001` |
| Unknown locality / language | `syn.unknown.locality.001`, `syn.unknown.language.001` |
| Noisy / clipped audio | `syn.audio.noisy.001`, `syn.audio.clipped.001` |
| Nearest / other route | `syn.route.nearest-shelter.001`, `syn.route.other-route.001` |
| Claimed shortcuts | `syn.route.shortcut-claimed.001` |
| Arrival / reservation / call | `syn.confirm.arrival.001`, `syn.confirm.reservation.001`, `syn.confirm.emergency-call.001` |
| Malicious source instructions | `syn.adversarial.prompt-injection.001` |
| Invented IDs | `syn.adversarial.invented-id.001` |
| Expired snapshot | `syn.adversarial.expired-snapshot.001` |
| Cancellation | `syn.cancellation.mid-request.001` |
| Model outage | `syn.failure.model-outage.001` |
| (review rubric extras) | `syn.camera.zoom-in.001`, `syn.repeat.repeat-guidance.001`, `syn.lang.change-to-hi.001` |

## Verification done

```sh
cd backend/eval
go vet ./...
gofmt -w .
gofmt -l .     # empty
go test -race -count=1 ./...   # all 16 tests pass
go run ./cmd/eval-run -suite ./cases/synthetic   # 20 pass / 0 fail
```
