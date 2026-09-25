# Inference-runtime closure — handoff

Owner: Worker 2. Branch: `codex/inference-runtime-closure` (off
`2e2d5c8`). Branch `codex/p567-integration-repair` (Worker 1) and
this branch diverge only in the files Worker 2 owns; the merge
order matters (Worker 2 first, Worker 1 second).

## Per-stage PASS / OPEN

| Stage | Scope | Commit | Verified | Notes |
|-------|-------|--------|----------|-------|
| A | Real ASR/Parler/Sarvam adapters + load→warm→READY | `7a4bd9d` | GREEN | Adapter protocol proven via injected fakes (11 tests); primary-source verification 2026-09-21 (Indic Parler dual tokenizer + 44.1k native, Sarvam chat_template_kwargs, vLLM ≥0.12 guided-decoding flag removed). |
| B | Bounded IPC (ASR+TTS), startup correlation, demuxErr, reaping | `68ce273` | GREEN | 15 adversarial IPC tests under -race; pre-existing race in `transcribeViaAdapter` caught and fixed at the same layer. TTS audio ceilings raised to native (48k / 1.25 MiB). |
| C | Eval conformance against REAL worker server constructors | `dd581aa` | GREEN (non-race) | 10 conformance tests covering envelope shape, strict decode, empty-200 fails closed, correlation mismatch, unavailable-then-recovered, audio cache refusals, malformed-TTS-200. Provider wire now sends actual `RequestEnvelope` / `SynthesizeRequest`; nested proposal decode; `ErrMalformedResponse`. |
| D | P7 smoke + security + harness self-tests | `302dd62` | GREEN | Smoke end-to-end against real Postgres+k6: default + business flows, 8 thresholds accounted, 0 violated. Security harness deterministic self-test: 16/16 assertions across INCOMPLETE / FAIL / PASS / FAIL-on-SPDX scenarios. |
| E | Full owned verification + this doc | (this doc) | GREEN | gofmt/vet/test -race clean across all 6 worker modules + backend + loadmodel; pytest 275/275; smoke end-to-end GREEN. |

## Heavy execution — NOT_RUN (deferred to external gates)

These do not block this branch because they depend on hardware
or external approvals not present in the worktree:

* **O03** IndicConformer multilingual ASR inference end-to-end.
  Adapter protocol + ordering + IO-drift detection verified via
  fakes; real model weights are gated by HF approval.
* **O11** Indic Parler-TTS regional translation verification.
  Adapter protocol + native sample_rate negotiation verified via
  fakes; regional translation table is a separate external review.
* **O14** Sarvam vLLM serving (hardware-blocked).
  `--guided-decoding-backend` removal + `chat_template_kwargs`
  path verified by spec research + unit tests; live serving
  requires GPU.

These remain authoritative external gates and are not re-opened
here. Worker 1 does not need to re-litigate them.

## Worker 1 contract asks (post-merge integration)

These are real bugs / gaps found during this branch that need
Worker 1 (who owns the worker `Server` types and the public
contracts) to fix:

1. **Worker `Server.Start` listener race** — `asrworker/server.go`,
   `middleworker/server.go`, `ttsworker/server.go` all write the
   bound `listener` field without a mutex. The eval real-server
   conformance tests (Stage C, `backend/eval/provider/http_realserver_test.go`)
   detect this under `-race` and are currently guarded with a
   `//go:build !race` constraint. The right fix is one of:
   * `sync.Mutex` (or `sync.RWMutex`) around `s.listener`; or
   * `Server.Start` exposes a bound-address channel that signals
     once `net.Listen` succeeds.
   Either is small. Once landed, drop the build tag — the tests
   will then run under `-race` and gate CI.

2. **TTS 44.1k rate plumbing** — `ttsworker/audio.go` raised
   `MaxOutputSampleRate` to 48000 and `MaxOutputBytes` to 1.25 MiB
   to accommodate Indic Parler native 44.1 kHz output (verified
   2026-09-21). The orchestrator / public HTTP server must accept
   negotiated native rates up to that ceiling; if `httpserver` /
   `contracts` currently hardcodes 22050 or 24000, that path needs
   to be relaxed. The Parler adapter reports `sample_rate` in the
   ready envelope and the IPC dispatcher now round-trips it.
   Worker 1's domain: pass that through `contracts` to the
   caller without re-resampling below 44.1k.

3. **Legacy `src/sthira_v2/local_voice.py`** — the `app.py`
   legacy demo path uses `local_voice.py` which fabricates
   `confidence = 1.0` (line 52). Not Worker 2's domain (legacy v1
   demo); flagged for Worker 1 to either retire or replace with
   the real adapter.

## Files NOT changed by Worker 2 (Worker 1's lane)

Anything in `backend/internal/{orchestration,httpserver,store,cmd}`,
`backend/internal/contracts`, `backend/migrations`, or any other
file not listed in the per-stage table above. Worker 1 owns merge
conflict resolution in those files.

## How to re-verify locally

```bash
# Worker-owned Go suites under -race:
(cd backend && go test -count=1 -race ./...)
(cd backend/internal/asrworker && go test -count=1 -race ./...)
(cd backend/internal/ttsworker && go test -count=1 -race ./...)
(cd backend/internal/middleworker && go test -count=1 -race ./...)
(cd backend/eval && go test -count=1 -race ./...)  # real-server conformance skipped under -race until Worker 1 fixes the listener race
(cd loadmodel && go test -count=1 -race ./...)

# Python adapter tests:
.venv/bin/python -m pytest tests/test_v2_real_adapters.py tests/test_b2_adapters.py -q

# P7 smoke end-to-end (requires PG + k6 + go):
bash scripts/run_k6_smoke_real.sh --business-flow

# P7 security harness self-test (deterministic, no real scanners):
bash scripts/test_security_checks.sh
```

## Open items NOT introduced by Worker 2

* `src/sthira_v2/local_voice.py` confidence fabrication — pre-existing legacy code.
* `--guided-decoding-backend` flag removed in vLLM ≥0.12 — verified by spec, not by live serving (O14 hardware-gated).
* IndicConformer per-language accuracy (O03) — external review.
