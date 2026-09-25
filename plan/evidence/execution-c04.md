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

## 3. Worker Startup & Launch Procedures

Two distinct procedures are supported:

### Procedure A: Local Plumbing & Exercise Rehearsal (PLUMBING_ONLY)
Use this procedure for local frontend + backend integration, test suites, and browser verification without GPU/cloud inference:

1. **Fresh Task-Owned Disposable Database**:
   ```bash
   # Create a unique disposable database migrated to SchemaRevision 10
   TEST_DB="sthira_task_$(date +%s)"
   createdb "$TEST_DB"
   STHIRA_DB_DSN="postgres://apple@localhost:5432/${TEST_DB}?sslmode=disable"
   for f in $(ls -1 backend/migrations/*.sql | sort); do
     psql "$STHIRA_DB_DSN" -v ON_ERROR_STOP=1 -q -f "$f"
   done
   ```

2. **Mock Protocol Workers**:
   ```bash
   # Run the tested repository protocol mock worker on an owned ephemeral/sentinel port (e.g. 50616)
   cd backend && go run ./cmd/mock-workers -port 50616
   # Serves typed envelopes for IndicConformer, Sarvam-30B, and Indic Parler-TTS
   ```

3. **Exercise Backend**:
   ```bash
   # Start backend on owned port 8080 wiring the mock worker port
   cd backend
   STHIRA_DATABASE_DSN="postgres://apple@localhost:5432/${TEST_DB}?sslmode=disable" \
   STHIRA_ADDR="127.0.0.1:8080" \
   STHIRA_ASR_URL="http://127.0.0.1:50616" \
   STHIRA_MIDDLE_URL="http://127.0.0.1:50616" \
   STHIRA_TTS_URL="http://127.0.0.1:50616" \
   STHIRA_EXERCISE_SEED=1 \
   go run ./cmd/sthira-exercise
   ```
   *Persistence Check*: To verify persistence across server restarts, restart the process **without** `STHIRA_EXERCISE_SEED=1` (or rely on safe idempotent seeding) to verify clean reload of stored stays without destructive package operations.

4. **Vite Frontend Proxying**:
   ```bash
   cd frontend/v2
   VITE_BACKEND_URL="http://127.0.0.1:8080" npm run dev -- --port 5173
   ```
   Open `http://127.0.0.1:5173` to test actions, silent zoom/pan, arrival confirmation panels, and stay reservation flows.

5. **Teardown**:
   Terminate owned processes and clean up the disposable database:
   `dropdb "$TEST_DB"`

---

### Procedure B: Deferred Real Models on AWS GPU (REAL_INFERENCE)
**STRICTLY UNEXECUTED PENDING AWS RESTART AUTHORIZATION.**
Instance `i-01d17e39266c292c2` is STOPPED in region `us-east-2`.
Weights are preserved on root EBS volume `vol-0b4e1d279d21586e2`:
- Sarvam-30B FP8 MoE (~37 GB)
- IndicConformer-600M-Multi ONNX (~2.4 GB)
- Indic Parler-TTS PyTorch (~3.6 GB)

When authorized by the owner:
1. **Discover New Public IP**:
   AWS stop releases the previous public IP; query the new assigned IP upon authorized start:
   ```bash
   aws ec2 describe-instances --instance-ids i-01d17e39266c292c2 --query "Reservations[0].Instances[0].PublicIpAddress" --output text
   ```

2. **Private Upstream vLLM Service (Port 8000)**:
   ```bash
   vllm serve /path/to/sarvam-30b --port 8000 --max-model-len 4096 --dtype float8
   ```
   *Distinction*: Raw vLLM implements OpenAI-compatible `/v1/chat/completions`, NOT the private typed Sthira worker HTTP protocol.

3. **Module-Aware Real Worker Binaries**:
   Run the dedicated Go worker binaries wrapping the model runtimes on private loopback ports:
   - **ASR Worker** (`backend/internal/asrworker/cmd/asrworker`):
     ```bash
     cd backend/internal/asrworker
     STHIRA_ASR_ADDR="127.0.0.1:8001" \
     STHIRA_ASR_PYTHON="python3" \
     STHIRA_ASR_ADAPTER="sthira_v2.speech_asr_adapter" \
     ./asrworker
     ```
     *Note*: IndicConformer CPU ONNX inference is verified; language is client-passed via `speechLanguageTag` (no auto-detection).
   - **Middle Worker** (`backend/internal/middleworker/cmd/middleworker`):
     ```bash
     cd backend/internal/middleworker
     STHIRA_MIDDLE_ADDR="127.0.0.1:8002" \
     STHIRA_VLLM_URL="http://127.0.0.1:8000" \
     ./middleworker
     ```
     Wraps upstream raw vLLM with canonical Sarvam prompt, embedded guided JSON schema, and strict 512 max output tokens.
   - **TTS Worker** (`backend/internal/ttsworker/cmd/ttsworker`):
     ```bash
     cd backend/internal/ttsworker
     STHIRA_TTS_ADDR="127.0.0.1:8003" \
     STHIRA_TTS_PYTHON="python3" \
     STHIRA_TTS_ADAPTER="sthira_v2.speech_tts_adapter" \
     ./ttsworker
     ```
     Wraps Indic Parler-TTS on CUDA (`STHIRA_TTS_DEVICE=cuda`) with native 44,100 Hz WAV caching and SHA-256 integrity verification.

4. **Public Backend / Exercise Wiring**:
   The public backend receives private worker URLs (`STHIRA_ASR_URL="http://127.0.0.1:8001"`, `STHIRA_MIDDLE_URL="http://127.0.0.1:8002"`, `STHIRA_TTS_URL="http://127.0.0.1:8003"`), not raw Python adapter or vLLM URLs.
   For isolated exercise / demonstration with labelled synthetic fixtures:
   ```bash
   cd backend
   STHIRA_ASR_URL="http://127.0.0.1:8001" \
   STHIRA_MIDDLE_URL="http://127.0.0.1:8002" \
   STHIRA_TTS_URL="http://127.0.0.1:8003" \
   go run ./cmd/sthira-exercise
   ```

Real inference and supported-language acceptance remain separate from launch/build checks. No instance access, spending or model download was performed by this review.

