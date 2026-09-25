# Task R02 Evidence — Worker listener races, TTS settings propagation, RIFF validation

**Task**: R02 — Worker/contract correctness and plumbing-only verification
**Owner lane**: `backend/internal/{asrworker,ttsworker,contracts,orchestration,httpserver}`, `backend/eval`
**Base commit**: CLEAN (pre-R02 state; R03/R04/B03 committed concurrently by other lanes)
**Host Environment**: macOS (Darwin arm64), Go 1.27.1, PostgreSQL 18.6 (Homebrew localhost:5432, user `apple`). No Docker (`CONTAINER_RUNTIME=NOT_RUN` for R06). No GPU, no model weights, no authorized spend → `REAL_INFERENCE=NOT_RUN`. No live government calls.

---

## 1. Acceptance Checklist & Verification Status

| Requirement | Implementation & Verification | Status |
| --- | --- | --- |
| Listener race: ASR worker | Added `listenerMu sync.Mutex` in `asrworker/server.go`; guard `s.listener` write/read in `Start`/`Addr`. Covered by existing + race tests. | **PASS** |
| Listener race: TTS worker | Added `listenerMu sync.Mutex` in `ttsworker/server.go`; same guard pattern. | **PASS** |
| Listener race: middle worker | Inspected `middleworker/server.go`: listener is bound once in `NewServer` before return, never mutated after construction; subsequent `Start`/`Addr` only read. No post-construction write race; no mutex required. Confirmed under `go test -race`. | **PASS (no mutation → no race)** |
| TTS response includes Settings | `contracts.TTSWorkerResponse` and `ttsworker.SynthesizeResponse` gained `Settings`; worker fills it on cache hit and miss paths (`ttsworker/worker.go`). | **PASS** |
| Orchestrator propagates worker-returned Settings (not request-side) | `stageTTS` in `orchestration/orchestrator.go` prefers `effectiveSettings` from the worker response, falling back to request-side only if absent; writes into `PipelineAudio.Settings`. Test `TestStageTTS_PropagatesReturnedSettingsNotRequestSide`. | **PASS** |
| RIFF/WAV header validation | New `orchestration/audio_meta.go` with `readWAVHeader` (rejects non-PCM, non-16-bit, truncated, zero fields). `stageTTS` parses worker audio, fails closed with reason `tts audio declared settings do not match RIFF header` on mismatch. Tests: `TestStageTTS_RejectsMismatchedSettings`, `TestReadWAVHeader_*` (5 cases). | **PASS** |
| Fake-audio fixtures exercise intended paths | Fixed `voice_integration_test.go` (`generateWAVBytes(500)`), `a1_regression_test.go` (`makeTestWAV`/`sha256Hex`), `orchestrator_external_test.go` (`TestSynthesize_InvalidReturnedAudio` → valid WAV + `"corrupted_checksum"`). | **PASS** |
| eval realserver race tag | Removed stale `//go:build !race` from `eval/provider/http_realserver_test.go`. | **PASS** |
| Import cycle split | External package `orchestration_test` (uses `orchestrationtest`) vs internal `orchestration` (for unexported `readWAVHeader`) — breaks cycle (`orchestrationtest` → `orchestration`). Shared helpers `makeTestWAV`/`sha256Hex` live in `audio_meta_test.go`. | **PASS** |
| Full suite race-clean | `go test -race` green in asrworker, ttsworker, middleworker, eval, main module. Full main-module suite green against disposable DB (store 31s, httpserver 20s). | **PASS** |
| **REAL_INFERENCE** | No GPU, no weights, no authorized spend → cannot run real model inference. Evidence records command only. | **NOT_RUN (`BLOCKED_HARDWARE`)** |

---

## 2. Commands, Exit Codes, Environment

```
# Module-level race tests (all exit 0)
cd backend/internal/asrworker   && go test -race ./...
cd backend/internal/ttsworker   && go test -race ./...
cd backend/internal/middleworker && go test -race ./...
cd backend/eval                 && go test -race ./...

# Main module (orchestration, httpserver, contracts) + full suite against disposable DB
export STHIRA_TEST_ADMIN_DSN='postgres://apple@localhost/postgres?sslmode=disable'
# create disposable DB, apply migrations/[0-9]*.sql, then:
export STHIRA_TEST_DSN='postgres://apple@localhost/<db>?sslmode=disable'
cd backend && go test -race ./internal/... ./cmd/...

# gofmt (R02-owned files: clean; pre-existing scenarioprep/cmd-scenario-prep failures left untouched)
gofmt -l backend/internal/{asrworker,ttsworker,orchestration,contracts,httpserver} backend/eval

# vet
cd backend && go vet ./internal/... ./cmd/...
cd backend/internal/{asrworker,ttsworker,middleworker} && go vet ./...
cd backend/eval && go vet ./...
```

Environment: Go 1.27.1, PostgreSQL 18.6, macOS Darwin arm64. Docker absent.

---

## 3. Real vs Fake Evidence / Hardware Blockers

- All PASS rows above are deterministic unit/integration tests using fake/synthetic WAV and in-process HTTP fakes (`orchestrationtest`).
- **REAL_INFERENCE=NOT_RUN** — no GPU, no model weights, no authorized spend. R02 must not be marked fully accepted until real inference runs on designated hardware.
- Demo case/language: user will supply later; interim fixture = `fixtures/synthetic_wayanad.json`.
- R06 `CONTAINER_RUNTIME=NOT_RUN` (no Docker) — separate task.

---

## 4. Exact Next Command (Real Inference — when hardware/artifacts available)

Per `backend/eval/commands.md` + `backend/README.md` worker URL wiring:

```bash
# 1. Start the three private loopback workers on approved hardware with weights
#    (worker launch is owned by R02 operational setup; URLs below assume defaults)
#    ASR    → http://localhost:7101
#    Middle → http://localhost:7201
#    TTS    → http://localhost:7301

# 2. Export worker bearer (if required by worker config)
export WORKER_BEARER='<shared-private-token>'

# 3. Run real-inference eval (ml-IN slice of synthetic suite)
cd backend/eval
go run ./cmd/eval-run -real \
    -suite ./cases/synthetic \
    -asr    http://localhost:7101 \
    -middle http://localhost:7201 \
    -tts    http://localhost:7301 \
    -bearer "$WORKER_BEARER" \
    -filter "language=ml-IN"
```

Worker URLs for the API are set via `STHIRA_ASR_URL`, `STHIRA_MIDDLE_URL`, `STHIRA_TTS_URL` (see `backend/README.md`). When unset, voice endpoint fails closed 503.

---

## 5. Changed Files (R02 stage)

- `backend/eval/provider/http_realserver_test.go` — removed `//go:build !race`
- `backend/internal/asrworker/server.go` — `listenerMu` mutex
- `backend/internal/ttsworker/server.go` — `listenerMu` mutex
- `backend/internal/ttsworker/worker.go` — fill `Settings` on cache hit
- `backend/internal/contracts/worker_health.go` — `TTSWorkerResponse.Settings`
- `backend/internal/orchestration/audio_meta.go` — `WAVHeader`, `readWAVHeader` (new)
- `backend/internal/orchestration/audio_meta_test.go` — external tests + `makeTestWAV`/`sha256Hex` (new)
- `backend/internal/orchestration/audio_meta_internal_test.go` — internal `TestReadWAVHeader_*` (new)
- `backend/internal/orchestration/orchestrator.go` — `stageTTS` settings/RIFF validation
- `backend/internal/orchestration/orchestrationtest/fakes.go` — `silenceWAV`, default `audioSettings`
- `backend/internal/orchestration/a1_regression_test.go` — valid WAV fixtures
- `backend/internal/orchestration/orchestrator_external_test.go` — valid WAV + checksum-invalid case
- `backend/internal/httpserver/voice_integration_test.go` — `generateWAVBytes(500)` TTS mock
- `plan/evidence/execution-r02.md` — this file

Not staged (other workers' WIP): `backend/internal/scenarioprep/*`, `backend/cmd/scenario-prep/report.go`, `backend/scripts/{recovery,security}/*`, `loadmodel/*`, `plan/evidence/loadrun/*`, `.gitignore`.

Also present untracked: `plan/evidence/execution-r00.md` (R00 evidence never committed) — include in this commit.

---

## 6. Shared-Contract Changes

- Added `Settings` field to `contracts.TTSWorkerResponse` (additive, backward-compatible).
- No OpenAPI/public API changes. Private worker protocol only.

---

## 7. Blockers / NOT_RUN Summary

| Item | Status |
| --- | --- |
| REAL_INFERENCE | **NOT_RUN** — `BLOCKED_HARDWARE` (no GPU/weights/spend) |
| CONTAINER_RUNTIME (R06) | NOT_RUN — no Docker |
| Demo case/language | User will supply; fixture `synthetic_wayanad.json` |
| R02 full acceptance | **NOT accepted** until real inference runs |

Do not mark R02 or R07 fully accepted on this evidence alone.
