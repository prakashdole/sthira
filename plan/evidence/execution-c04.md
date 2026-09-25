# Task C04 Evidence — Wire Selected Models into Isolated Demo & Verification

> **2026-09-25 review — LOCAL VERIFICATION COMPLETE / CLOUD ACCEPTANCE DEFERRED.** Local executable composition, strict guided action schemas (`FOCUS_FEATURE`, `HIGHLIGHT_FEATURE`, `SHOW_CHOICES`, `SHOW_ROUTE`, `FIT_FEATURES`, `OPEN_PANEL`, `ZOOM`, `PAN`, `RECENTER`, `SET_LANGUAGE`, `SET_LAYER_VISIBILITY`), independent `FIT_FEATURES` validation, template text propagation, and language-bound digest forwarding are implemented and tested across all Go modules and Python adapters. Real cloud inference (Stage L5) remains strictly deferred with instance `i-01d17e39266c292c2` STOPPED. See [prompt section 0.0](../prompt.md).

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

## 3. Worker Startup Instructions & Verified Entry Points

The real serving executable entry points have been implemented and verified in each worker module under `backend/internal/*/cmd/`:

1. **ASR Worker** (`backend/internal/asrworker`):
   - **Entry point**: `cmd/asrworker/main.go`
   - **Build**: `cd backend/internal/asrworker && go build ./cmd/asrworker`
   - **Run**:
     ```bash
     cd backend/internal/asrworker
     STHIRA_ASR_ADDR="127.0.0.1:8001" \
     STHIRA_ASR_PYTHON="python3" \
     STHIRA_ASR_ADAPTER="sthira_v2.speech_asr_adapter" \
     ./asrworker
     ```
   - **Behavior**: Wraps `sthira_v2.speech_asr_adapter` over stdin/stdout JSONL. Calls `worker.LoadAndVerify(ctx)`. Fails closed if the Python runtime or ONNX model weights are missing or report blocked status.

2. **Middle Worker** (`backend/internal/middleworker`):
   - **Entry point**: `cmd/middleworker/main.go`
   - **Build**: `cd backend/internal/middleworker && go build ./cmd/middleworker`
   - **Run**:
     ```bash
     cd backend/internal/middleworker
     STHIRA_MIDDLE_ADDR="127.0.0.1:8002" \
     STHIRA_VLLM_URL="http://127.0.0.1:8000" \
     ./middleworker
     ```
   - **Behavior**: Wraps upstream private vLLM endpoint (`sarvamai/sarvam-30b` FP8 MoE) with canonical Sarvam system prompt, embedded JSON schema (`model_output.schema.json`), and 512 max output tokens.

3. **TTS Worker** (`backend/internal/ttsworker`):
   - **Entry point**: `cmd/ttsworker/main.go`
   - **Build**: `cd backend/internal/ttsworker && go build ./cmd/ttsworker`
   - **Run**:
     ```bash
     cd backend/internal/ttsworker
     STHIRA_TTS_ADDR="127.0.0.1:8003" \
     STHIRA_TTS_PYTHON="python3" \
     STHIRA_TTS_ADAPTER="sthira_v2.speech_tts_adapter" \
     ./ttsworker
     ```
   - **Behavior**: Wraps `sthira_v2.speech_tts_adapter` over stdin/stdout JSONL with bounded template catalog and verified WAV caching. Calls `rt.LoadModel()`. Fails closed if Parler-TTS weights or voices are missing or report blocked status.

4. **Public Backend / Exercise Wiring**:
   - The public backend receives private worker URLs (`STHIRA_ASR_URL="http://127.0.0.1:8001"`, `STHIRA_MIDDLE_URL="http://127.0.0.1:8002"`, `STHIRA_TTS_URL="http://127.0.0.1:8003"`), not raw Python adapter or vLLM URLs.
   - The production binary (`cmd/sthira`) rejects synthetic templates.
   - For isolated exercise / demonstration with labelled synthetic fixtures:
     ```bash
     cd backend
     STHIRA_ASR_URL="http://127.0.0.1:8001" \
     STHIRA_MIDDLE_URL="http://127.0.0.1:8002" \
     STHIRA_TTS_URL="http://127.0.0.1:8003" \
     go run ./cmd/sthira-exercise
     ```

Real inference and supported-language acceptance remain separate from launch/build checks. No instance access, spending or model download was performed by this review.
