# Task C04 Evidence — Wire Selected Models into Isolated Demo & Verification

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

## 3. Runnable Worker Startup Instructions

### Mode A: Development / Protocol Testing (Loopback HTTP Workers)

For local development and automated plumbing verification, run the private protocol workers:

```bash
# 1. Start private loopback workers on separate ports
# ASR worker (port 7101)
go run ./loadmodel/cmd/dummyd --kind asr --addr 127.0.0.1:7101 &
PID_ASR=$!

# Middle worker (port 7201)
go run ./loadmodel/cmd/dummyd --kind middle --addr 127.0.0.1:7201 &
PID_MID=$!

# TTS worker (port 7301)
go run ./loadmodel/cmd/dummyd --kind tts --addr 127.0.0.1:7301 &
PID_TTS=$!

# 2. Export worker endpoints
export STHIRA_ASR_URL="http://127.0.0.1:7101"
export STHIRA_MIDDLE_URL="http://127.0.0.1:7201"
export STHIRA_TTS_URL="http://127.0.0.1:7301"

# 3. Start exercise server
export STHIRA_DATABASE_DSN="postgres://apple@localhost:5432/sthira_dev?sslmode=disable"
export STHIRA_EXERCISE_SEED=1
go run ./cmd/sthira-exercise
```

### Mode B: GPU Deployment with Real Model Weights (Authorized Hardware)

When authorized GPU hardware and weights are provisioned:

```bash
# 1. Start ASR Worker (IndicConformer-600M-Multi ONNX)
python3 -m sthira_v2.speech_asr_adapter \
    --model-dir /opt/models/indic-conformer-600m-multilingual \
    --port 7101 \
    --token "$STHIRA_ASR_TOKEN" &

# 2. Start Middle Worker (Sarvam-30B MoE FP8 via vLLM)
vllm serve sarvamai/sarvam-30b \
    --host 127.0.0.1 \
    --port 7201 \
    --tensor-parallel-size 1 \
    --kv-cache-dtype fp8 \
    --max-model-len 4096 \
    --chat-template-kwargs '{"enable_thinking":false}' \
    --api-key "$STHIRA_MIDDLE_TOKEN" &

# 3. Start TTS Worker (Indic Parler-TTS)
python3 -m sthira_v2.speech_tts_adapter \
    --model-dir /opt/models/indic-parler-tts \
    --port 7301 \
    --token "$STHIRA_TTS_TOKEN" &

# 4. Start Production Backend
export STHIRA_ASR_URL="http://127.0.0.1:7101"
export STHIRA_ASR_TOKEN="$STHIRA_ASR_TOKEN"
export STHIRA_MIDDLE_URL="http://127.0.0.1:7201"
export STHIRA_MIDDLE_TOKEN="$STHIRA_MIDDLE_TOKEN"
export STHIRA_TTS_URL="http://127.0.0.1:7301"
export STHIRA_TTS_TOKEN="$STHIRA_TTS_TOKEN"
export STHIRA_DATABASE_DSN="postgres://user:pass@db:5432/sthira_prod?sslmode=verify-full"
./bin/sthira
```
