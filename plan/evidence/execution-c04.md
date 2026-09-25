# Task C04 Evidence — Wire Selected Models into Isolated Demo & Verification

> **2026-09-25 review — PARTIAL / REOPENED.** Shared API-to-fake-worker plumbing and isolated templates are implemented; real worker executable composition and browser codec/map integration remain engineering work. Actual models are NOT_VERIFIED; user provisioning is in progress. See [review](../reviews/review-recovery-2026-09-25.md) and [prompt section 0.0](../prompt.md). Tests returning model-name strings do not prove the selected models ran.

**Task**: C04 — Wire the selected models into the isolated demo (inference; C03 integrated)
**Lane**: Backend (`cmd/sthira`, `cmd/sthira-exercise`, `internal/httpserver`, `internal/orchestration`)
**Host Environment**: macOS (Darwin arm64), Go 1.27.1, PostgreSQL 18.6 (`STHIRA_TEST_ADMIN_DSN="postgres://apple@localhost:5432/postgres?sslmode=disable"`)
**Status**: 
- Row (a) **PLUMBING_ONLY**: **PASS**
- Row (b) **REAL_INFERENCE**: **NOT_RUN (`BLOCKED_HARDWARE`)**

---

## 1. Acceptance Checklist & Verification Status

| Requirement | Implementation & Verification | Status |
| --- | --- | --- |
| **Shared Pipeline Wiring** | Refactored `cmd/sthira/main.go` and `cmd/sthira-exercise/main.go` to share `httpserver.WireVoicePipeline`, eliminating duplicate setup code. | **PASS** |
| **Exercise Voice Wiring** | `cmd/sthira-exercise/main.go` constructs worker clients and wires `httpserver.WithVoiceProcess`, accepting `POST /api/v3/voice/process` with persisted context. | **PASS** |
| **Exercise Fixture Seeding** | `seedExercise` seeds `place_aliases` (including ambiguous "meppadi" aliases for candidate chips) and `approved_translations` for `DEMO-EXERCISE` (`PKGDEMO-1:1`) matching `ExerciseTemplateRegistry(1, 1)`. | **PASS** |
| **Synthetic Template Isolation** | Added `AllowSyntheticTemplates` to `orchestration.PipelineConfig`. Process-isolated to `cmd/sthira-exercise` (`true`); `cmd/sthira` production binary defaults to `false` (fail-closed with 422). Request headers/params cannot bypass this. | **PASS** |
| **PLUMBING_ONLY End-to-End** | Verified via `TestC04_PlumbingOnly_FullVoicePipeline`: public API `/api/v3/voice/process` + real PostgreSQL DB + private HTTP workers runs audio (ASR → Middle → Template → TTS) and transcript inputs, returns valid proposal and verified WAV audio with zero consequential DB writes. | **PASS** |
| **Disambiguation Candidate Chips** | Verified via `TestC04_AmbiguousLocationDisambiguation_CandidateChips`: `POST /api/v3/places/resolve` with `query="meppadi"` returns HTTP 409 Conflict with multiple candidates (`SZDEMO-1` and `FACDEMO-1`) in `details.candidates`. | **PASS** |
| **Selected Models Specification** | Verified via `TestC04_SelectedModelsSpecification_ContractIntegrity`: IndicConformer-600M-Multi (ASR), Sarvam-30B MoE 2.4B active params (Middle, `enable_thinking=false`), Indic Parler-TTS (TTS). | **PASS** |
| **REAL_INFERENCE** | Actual weights executing on GPU hardware: **NOT_RUN (`BLOCKED_HARDWARE`)**. | **NOT_RUN (`BLOCKED_HARDWARE`)** |

---

## 2. Missing Resources for REAL_INFERENCE

Real weight inference on this host is blocked by external hardware and resource prerequisites:
1. **ASR Model**: `ai4bharat/indic-conformer-600m-multilingual` ONNX model weights and runtime assets (~1.2 GB).
2. **Intent/Middle Model**: `sarvamai/sarvam-30b` FP8 MoE weights (~30 GB) requiring a dedicated GPU cluster (e.g. NVIDIA A100/H100 80GB VRAM) running vLLM.
3. **TTS Model**: `ai4bharat/indic-parler-tts` PyTorch safetensors checkpoint and vocoder assets.
4. **Authority & Spend**: No budget, cloud compute rental, or model weight downloads are authorized on this machine.

---

## 3. Worker startup instructions — withdrawn pending correction

The former commands were not runnable evidence and must not be used for deployment:

- `speech_asr_adapter.py` and `speech_tts_adapter.py` speak JSON lines over stdin/stdout; they do not expose the claimed `--port` HTTP server interface.
- Raw vLLM exposes an OpenAI-compatible API, not Sthira's typed middle-worker request/response/health contract. `STHIRA_MIDDLE_URL` must point to the private Sthira wrapper, whose upstream is vLLM.
- Repository Go ASR/TTS/middle worker packages exist, but no real serving executable entry points were found in this review. `cmd/mock-workers` and dummy/eval drivers cannot substitute for those services.
- `go run ./loadmodel/...` and `go run ./cmd/sthira-exercise` refer to different module working directories. Each replacement command must specify its actual module path and validated flags.
- The production binary rejects synthetic templates. Use the explicitly isolated exercise binary/database for a labelled synthetic-case demonstration; do not disable production checks.

C04 must reuse the existing runtime and HTTP server implementations to supply minimal launch entry points with private binding, auth, required artifact configuration, load/warm health, bounded lifecycle and clean shutdown. Test build/startup failure without weights; then provide actual authorized instance commands with pinned runtime/artifact details. No speculative A100/H100/MLX compatibility claim substitutes for that evidence. The public backend receives private worker URLs, not Python adapter or raw vLLM URLs. Keep secrets out of documentation and output.

Real inference and supported-language acceptance remain separate from launch/build checks. No instance access, spending or model download was performed by this review.
