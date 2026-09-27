# Real-model launch checklist — MiniMax J corrected edition

**Source**: `plan/evidence/real-inference-launch-check.md` at `d95df9e`
**Corrected against**: `backend/internal/{asrworker,middleworker,ttsworker}/cmd/*/main.go`,
`src/sthira_v2/speech_{asr,tts}_adapter.py`,
`backend/internal/middleworker/sarvam_config.go`,
`tools/preflight/main.go`,
`backend/cmd/sthira-exercise/main.go`
**Scope**: Real-model paid GPU launch preconditions and smoke sequence

---

## Corrections to `plan/evidence/real-inference-launch-check.md` §7

The following items differ from what the runbook recorded, based on source-code
inspection at `d95df9e`.

### C1. PYTHONPATH is not read by Go workers

The runbook says "export PYTHONPATH in the worker's shell" and §7 host environment
shows `export PYTHONPATH=/opt/pytorch/lib/python3.12/site-packages`. This is
correct for the **operator's shell** (before starting the Go binary), but
`PYTHONPATH` is **not** read by any Go `main.go`. It is inherited by the Python
child subprocess via `cmd.Env = append(os.Environ(), ...)`. The preflight tool
checks `STHIRA_PREFLIGHT_PYTHONPATH` only for its own validation; the Go
workers themselves do not reference this env var.

**Correction**: PYTHONPATH must be set in the shell environment before starting
each Go worker. It is not a Go-read variable.

### C2. TTS has a separate `STHIRA_TTS_PYTHON` env var

The runbook uses a single `$PY` (mapped to `STHIRA_ASR_PYTHON`) for both
workers. The Go source confirms:

| Worker | Env var | Default |
|--------|---------|---------|
| asrworker | `STHIRA_ASR_PYTHON` | `"python3"` |
| ttsworker | `STHIRA_TTS_PYTHON` | `"python3"` |

Both workers' `main.go` read their respective variables independently. If both
Python interpreters are the same binary (as on the GPU host at
`/home/ubuntu/.venvs/voice/bin/python`), the runbook's single `$PY` variable
is valid in practice, but the Go code requires both to be set separately if
they differ. The preflight tool checks only `STHIRA_ASR_PYTHON` as a
convenience; it does not validate `STHIRA_TTS_PYTHON` independently.

### C3. `sthira-exercise` requires six worker URL/token env vars

The runbook's Step 5 smoke sequence references local `sthira-exercise` but does
not show its startup command. The Go source at
`backend/cmd/sthira-exercise/main.go:290–295` reads:

```
STHIRA_ASR_URL, STHIRA_ASR_TOKEN
STHIRA_MIDDLE_URL, STHIRA_MIDDLE_TOKEN
STHIRA_TTS_URL, STHIRA_TTS_TOKEN
```

These are required to wire the HTTP server to the three workers. The runbook
must include this startup command.

### C4. `STHIRA_TTS_VOICES_FILE` is required for TTS

The Python adapter (`speech_tts_adapter.py:117`) reads
`STHIRA_TTS_VOICES_FILE`. If unset, the adapter returns zero advertised voices.
The Go worker (`ttsworker/cmd/ttsworker/main.go`) does not read this variable;
it is purely a Python-adapter concern. The preflight tool checks it conditionally
(if set), but the runbook should note it is required for real TTS synthesis.

### C5. `STHIRA_TTS_DEVICE` defaults to `"cpu"`, not `"cuda"`

The Python adapter (`speech_tts_adapter.py:302`) uses:
```python
model.to(os.environ.get("STHIRA_TTS_DEVICE", "cpu"))
```
The runbook Step 4 command passes `STHIRA_TTS_DEVICE=cuda`, which is correct for
the GPU host, but the default in the absence of this variable is CPU — not
cuda. This is a common misconfiguration risk if the env var is forgotten.

### C6. No ASR language autodetection — language is always UI-supplied

The runbook correctly states this, but it is confirmed against source:
`speech_asr_adapter.py` takes `lang_bcp47` as an explicit parameter; the Go
caller supplies `X-Language` header / pipeline `language`. No inference path
exists without this field.

### C7. vLLM `--port 8000` is private loopback — confirmed

`sarvam_config.go:199` confirms `--host 127.0.0.1 --port 8000`. The runbook
correctly documents this. The check `curl -s 127.0.0.1:8000/v1/models` is the
correct step to verify the served model name.

### C8. Sarvam model ID confirmed: `"sarvamai/sarvam-30b"`

`sarvam_config.go:27`: `const SarvamModelID = "sarvamai/sarvam-30b"`. The runbook
Step 1 check must verify this exact string is listed, not a variant.

### C9. TTS native sample rate is 44,100 Hz — not hardcoded in adapter

`speech_tts_adapter.py:41–43`: `sample_rate` is read from
`model.config.sampling_rate` at load time. The runbook's "WAV header = declared
rate (44,100 Hz)" pass criterion is correct.

### C10. Middle worker sends `model = "sarvamai/sarvam-30b"` with `max_tokens 512`

`sarvam_config.go:34`: `MaxOutputTokens: 512`. The request-level JSON schema
enforcement is via `response_format json_schema` (vLLM structured outputs).
`sarvam_config.go:131`: `ChatTemplateKwargs = {"enable_thinking": false}`.

---

## Corrected launch checklist

### Host setup (one-time, remote GPU host shell)

```bash
# Record commit under test
cd ~/MonitoringZ && git rev-parse HEAD

# Required environment for all three workers
export PYTHONPATH=/opt/pytorch/lib/python3.12/site-packages

# Shared auth token (never paste into evidence)
export TOK="$(openssl rand -hex 24)"
```

### Preflight (remote host, before any paid inference)

```bash
cd ~/MonitoringZ
(cd tools/preflight && go run . \
  -asr-dir /home/ubuntu/models/indic-conformer-600m-multilingual \
  -tts-dir /home/ubuntu/models/indic-parler-tts \
  -python /home/ubuntu/.venvs/voice/bin/python \
  -pythonpath /opt/pytorch/lib/python3.12/site-packages \
  -tts-text-encoder-dir /home/ubuntu/models/flan-t5-large-tokenizer \
  -sarvam-dir /home/ubuntu/models/sarvam-30b-fp8)
# All artifact/interpreter/PYTHONPATH checks must PASS before proceeding.
```

### Step 0 — Instance + preflight

| Check | Command | Pass criterion |
|-------|---------|----------------|
| Instance started | owner-authorized AWS action | `i-01d17e39266c292c2` → `running` |
| Preflight clean | above preflight command | all host checks PASS; loopback ports free |

### Step 1 — vLLM container

```bash
docker start sthira-sarvam
sleep 5
curl -s 127.0.0.1:8000/v1/models
```

| Check | Pass criterion |
|-------|----------------|
| Served model name | `"sarvamai/sarvam-30b"` in model list |
| vLLM responds | HTTP 200, non-empty JSON |

### Step 2 — ASR worker alone

```bash
STHIRA_ASR_ADDR=127.0.0.1:8001 \
STHIRA_ASR_TOKEN="$TOK" \
STHIRA_ASR_PYTHON=/home/ubuntu/.venvs/voice/bin/python \
STHIRA_ASR_ADAPTER=sthira_v2.speech_asr_adapter \
STHIRA_ASR_ARTIFACT_DIR=/home/ubuntu/models/indic-conformer-600m-multilingual \
STHIRA_ASR_APPROVED_LANGUAGES=hi-IN,ml-IN \
./asrworker &
```

| Check | Pass criterion |
|-------|----------------|
| `/health` | `ready+warm`, `hi-IN` and `ml-IN` listed |
| hi-IN WAV | non-empty transcription text |
| Silence | no command emitted (empty result, not an error) |

### Step 3 — Middle worker alone

```bash
STHIRA_MIDDLE_ADDR=127.0.0.1:8002 \
STHIRA_MIDDLE_TOKEN="$TOK" \
STHIRA_VLLM_URL=http://127.0.0.1:8000 \
./middleworker &
```

| Check | Pass criterion |
|-------|----------------|
| Silent zoom | strict JSON only; no thinking/prose |
| Destination request | `facility_id` (not `safe_zone_id`), in server-permitted order |
| Arrival | `OPEN_PANEL` + `ARRIVAL_CONFIRMATION` only |
| Invalid output | rejected by Go validator, not repaired |
| `data_version` | matches guidance `PKGDEMO-1:1` in response |

### Step 4 — TTS worker alone

```bash
STHIRA_TTS_ADDR=127.0.0.1:8003 \
STHIRA_TTS_TOKEN="$TOK" \
STHIRA_TTS_PYTHON=/home/ubuntu/.venvs/voice/bin/python \
STHIRA_TTS_ADAPTER=sthira_v2.speech_tts_adapter \
STHIRA_TTS_ARTIFACT_DIR=/home/ubuntu/models/indic-parler-tts \
STHIRA_TTS_TEXT_ENCODER_DIR=/home/ubuntu/models/flan-t5-large-tokenizer \
STHIRA_TTS_VOICES_FILE=/home/ubuntu/models/approved_voices.json \
STHIRA_TTS_DEVICE=cuda \
STHIRA_TTS_SYNTHETIC_EXERCISE=1 \
./ttsworker &
```

| Check | Pass criterion |
|-------|----------------|
| Startup pre-generation | completes for all 4 template keys × 3 languages |
| en-IN / hi-IN cache hit | WAV returned, no model call needed |
| WAV sample rate | matches `model.config.sampling_rate` (44,100 Hz native) |
| WAV checksum | matches recorded digest for template+language |
| Unknown template | rejected, not fabricated |
| `STHIRA_TTS_VOICES_FILE` absent | zero advertised voices (fails closed) |

### Step 5 — Full pipeline via sthira-exercise

```bash
# In one shell on GPU host (sthira-exercise wires HTTP server to all three workers):
STHIRA_EXERCISE_SEED=1 \
STHIRA_ASR_URL=http://127.0.0.1:8001 \
STHIRA_ASR_TOKEN="$TOK" \
STHIRA_MIDDLE_URL=http://127.0.0.1:8002 \
STHIRA_MIDDLE_TOKEN="$TOK" \
STHIRA_TTS_URL=http://127.0.0.1:8003 \
STHIRA_TTS_TOKEN="$TOK" \
STHIRA_DATABASE_URL="postgres://..." \
./sthira-exercise &
```

```bash
# From laptop (SSH tunnels established):
# ssh -L 8001:127.0.0.1:8001 -L 8002:127.0.0.1:8002 -L 8003:127.0.0.1:8003 ubuntu@<host>
curl -s -X POST http://127.0.0.1:8001/health   # ASR alive
curl -s -X POST http://127.0.0.1:8002/health   # middle alive
curl -s -X POST http://127.0.0.1:8003/health   # TTS alive
```

| Check | Pass criterion |
|-------|----------------|
| en-IN destination | `data_version` = `PKGDEMO-1:1`; audio with `language=hi-IN` in response |
| hi-IN destination | `language=hi-IN` in audio response |
| Silent zoom | no TTS in response |
| `stage_failures` | absent on success response |

### Step 6 — Browser + microphone (separate authorized session)

Requires Vite dev server and real browser on laptop/phone with microphone.

| Check | Pass criterion |
|-------|----------------|
| Mic permission granted | transcript appears |
| Mic permission denied | graceful denial message |
| Spoken hi-IN destination | chips displayed + audible Hindi audio |
| Replay after language switch | refused (no approved audio) |
| Arrival only after server ack | `Recorded at` timestamp shown |

---

## Unresolved facts (checks for authorized session)

These require the paid GPU session; they cannot be verified locally.

1. **ASR ONNX inference latency** on CPU (no GPU ASR demonstrated yet).
2. **vLLM Sarvam-30B FP8 cold-start time** — how long from `docker start` to first response.
3. **Actual TTS GPU memory** consumption with `STHIRA_TTS_DEVICE=cuda`.
4. **End-to-end latency** from spoken hi-IN audio → TTS audio playback on device.
5. **Human judgment** on Hindi/Malayalam speech quality and destination ID accuracy.

---

## Negative controls for regression logic

| Scenario | Expected |
|----------|----------|
| vLLM serves different model name | stop immediately, do not edit code on host |
| TTS worker starts without `STHIRA_TTS_VOICES_FILE` | zero voices advertised; worker stays up (log warning, not fatal) |
| ASR receives unknown language code | `blocked` status on `/health`; requests return 400 |
| Middle worker receives non-JSON to vLLM | Go validator rejects before sending; no prose leak |
| Arrival confirmed without prior reservation | `ARRIVAL_CONFIRMATION` only; no state mutation |
| ASR `confidence: null` is returned | correct — adapter never fabricates a score |

---

## Items owned by other workers (not in this checklist)

| Excluded | Owner |
|----------|-------|
| Layout / overflow at multiple viewports | G |
| Audio replay after language switch | H |
| Camera / MapLibre layer controls | L |
| Browser duplicate-start protection | MiniMax B |
| Real browser outage/recovery | E/F |
| Delayed-language race (switch while request pending) | C |
