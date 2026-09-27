# MiniMax J — Real-Model Launch Runbook Readiness
**Report**: `plan/worker-reports/round-2/minimax-j/report.md`
**Worker**: MiniMax J
**Timestamp**: 2026-09-27T00:00:00Z
**HEAD**: `d95df9e609454877a91b7c82146b3c6368181b17`
**Assignment**: Round 2, MiniMax J

---

## 1. Scope Inspected

| Area | Source | Key Finding |
|------|--------|-------------|
| ASR worker entrypoint | `backend/internal/asrworker/cmd/asrworker/main.go` | Reads `STHIRA_ASR_*`; Python path via `STHIRA_ASR_PYTHON` |
| Middle worker entrypoint | `backend/internal/middleworker/cmd/middleworker/main.go` | Reads `STHIRA_MIDDLE_*`; calls vLLM at `STHIRA_VLLM_URL` |
| TTS worker entrypoint | `backend/internal/ttsworker/cmd/ttsworker/main.go` | Requires `STHIRA_TTS_SYNTHETIC_EXERCISE=1`; separate `STHIRA_TTS_PYTHON` |
| ASR Python adapter | `src/sthira_v2/speech_asr_adapter.py` | Reads `STHIRA_ASR_ARTIFACT_DIR`, `STHIRA_ASR_APPROVED_LANGUAGES`; `lang_bcp47` is explicit parameter |
| TTS Python adapter | `src/sthira_v2/speech_tts_adapter.py` | Reads `STHIRA_TTS_*`; `STHIRA_TTS_DEVICE` defaults to `"cpu"`; sample rate from `model.config.sampling_rate` |
| vLLM Sarvam config | `backend/internal/middleworker/sarvam_config.go` | `SarvamModelID = "sarvamai/sarvam-30b"`; `MaxOutputTokens: 512`; `--port 8000` |
| Preflight tool | `tools/preflight/main.go` | Checks env vars present in source; checks artifact dirs, ports |
| Exercise server | `backend/cmd/sthira-exercise/main.go:290–295` | Reads 6 worker URL/token env vars |
| Existing runbook | `plan/evidence/real-inference-launch-check.md` | Baseline at `d95df9e`; owned by prior session |

**Dirty files** (unrelated to this work):
- `backend/internal/asrworker/audio_mime_test.go`, `backend/internal/ttsworker/audio_mime_test.go`
- `backend/internal/middleworker/eval/` corpus changes
- `frontend/v2/src/styles.css`

---

## 2. Deliverables

| Artifact | Path |
|----------|------|
| Corrected launch checklist | `plan/worker-reports/round-2/minimax-j/launch-checklist.md` |
| This report | `plan/worker-reports/round-2/minimax-j/report.md` |

---

## 3. Findings

### F1. PYTHONPATH is shell-inherited, not a Go-read variable

**Evidence**: None of the three Go `main.go` files contain `Getenv("PYTHONPATH")`. The child
Python process inherits `os.Environ()` from the Go worker (`cmd.Env = append(os.Environ(), ...)`).

**Impact**: Runbook §7 host environment command is correct (export PYTHONPATH before starting
Go worker), but the interpretation that PYTHONPATH is "read by the Go worker" is wrong.
Preflight checks `STHIRA_PREFLIGHT_PYTHONPATH` only for its own validation; this is not a Go
worker env var.

**Correction**: PYTHONPATH must be exported in the operator's shell before starting each
Go worker binary.

### F2. TTS has separate `STHIRA_TTS_PYTHON` — not the same var as ASR

**Evidence**:
- `asrworker/main.go:26`: `pyCmd := os.Getenv("STHIRA_ASR_PYTHON")`
- `ttsworker/main.go:33`: `pyCmd := os.Getenv("STHIRA_TTS_PYTHON")`
- Both default to `"python3"` independently.

**Impact**: If the two Python interpreters differ, using `$PY` (pointing to ASR's Python)
for both workers would silently use the wrong interpreter for TTS. On the GPU host they
are the same binary, so the runbook's single `$PY` works in practice, but the Go code
requires separate variables.

**Correction**: Use `STHIRA_TTS_PYTHON` explicitly for the TTS worker, or ensure both env
vars point to the same binary.

### F3. `sthira-exercise` startup requires six worker URL/token env vars

**Evidence**: `sthira-exercise/main.go:290–295` reads:
`STHIRA_ASR_URL`, `STHIRA_ASR_TOKEN`, `STHIRA_MIDDLE_URL`, `STHIRA_MIDDLE_TOKEN`,
`STHIRA_TTS_URL`, `STHIRA_TTS_TOKEN`.

**Impact**: The runbook Step 5 references `sthira-exercise` without showing these required
variables. Without them, the exercise server cannot route requests to any worker.

**Correction**: Add the full `sthira-exercise` startup command with all six env vars to the
checklist.

### F4. `STHIRA_TTS_VOICES_FILE` is required for real TTS; absent = zero voices

**Evidence**: `speech_tts_adapter.py:117` — if unset, `voices = {}`. The Go worker does not
check this variable; the Python adapter silently advertises zero voices.

**Impact**: TTS worker starts and stays up, but no TTS synthesis is possible. This is a
silent failure mode.

**Correction**: Note in checklist that `STHIRA_TTS_VOICES_FILE` must be set to an approved
voices JSON file before starting the TTS worker.

### F5. `STHIRA_TTS_DEVICE` defaults to `"cpu"`, not `"cuda"`

**Evidence**: `speech_tts_adapter.py:302`: `model.to(os.environ.get("STHIRA_TTS_DEVICE", "cpu"))`.

**Impact**: If `STHIRA_TTS_DEVICE=cuda` is accidentally omitted, TTS inference runs on CPU
with severe performance degradation. The runbook Step 4 command correctly passes it, but
the default is CPU.

**Correction**: Checklist pass criterion should verify TTS is using CUDA by checking the
worker logs or a subsequent preflight health indicator.

### F6. ASR language is always explicit; no autodetection

**Evidence**: `speech_asr_adapter.py` takes `lang_bcp47` as an explicit protocol parameter.
The Go caller (`asrworker/runtime_adapter.go`) supplies the UI-selected `language` field.
No autodetection code path exists.

**Impact**: None — this is already correctly stated in the runbook. Confirmed as correct.

### F7. vLLM port 8000 and SarvamModelID confirmed correct

**Evidence**:
- `sarvam_config.go:199`: `--host 127.0.0.1 --port 8000`
- `sarvam_config.go:27`: `SarvamModelID = "sarvamai/sarvam-30b"`

**Impact**: None — runbook is correct. Confirmed.

### F8. TTS native sample rate is 44,100 Hz read from model config

**Evidence**: `speech_tts_adapter.py:41–43` reads `model.config.sampling_rate`. Not hardcoded.

**Impact**: None — runbook pass criterion ("WAV header = declared rate, 44,100 Hz") is
correct. Confirmed.

### F9. Middle worker max_tokens is 512, not 256

**Evidence**: `sarvam_config.go:34`: `MaxOutputTokens: 512`.

**Impact**: The runbook Step 3 criterion should say `max_tokens 512` in outbound request
shape check. The system prompt is already written for the 512-token contract.

### F10. Full pipeline requires SSH tunnels; runbook Step 5 omits them

**Evidence**: Step 5 says "SSH `-L 8001/8002/8003`" in the pass criterion description but
does not show the actual SSH command. The `sthira-exercise` server binds `127.0.0.1` on
the GPU host; SSH tunnels are required to reach it from a laptop browser.

**Impact**: The full pipeline step cannot be run from the laptop without explicit SSH
tunnel commands.

**Correction**: Add explicit SSH tunnel command to checklist Step 5.

---

## 4. Proposed Patch

No source patch. All corrections are documented in the launch checklist. The checklist
is the integration artifact.

**Base hash for reference**:
- `plan/evidence/real-inference-launch-check.md`: `d95df9e` (unchanged — this worker produced a new candidate, not an edit)

If Opus needs a diff against the shared runbook, a `candidate.patch` can be generated
from `diff -u plan/evidence/real-inference-launch-check.md plan/worker-reports/round-2/minimax-j/launch-checklist.md`.

---

## 5. Verification

**Execution tests NOT_RUN — deferred by user (paid GPU session not authorized).**

### Later Commands (on the authorized GPU host, `i-01d17e39266c292c2`)

```bash
# 1. Preflight — must pass all artifact/interpreter checks before any paid work
cd ~/MonitoringZ
(cd tools/preflight && go run . \
  -asr-dir /home/ubuntu/models/indic-conformer-600m-multilingual \
  -tts-dir /home/ubuntu/models/indic-parler-tts \
  -python /home/ubuntu/.venvs/voice/bin/python \
  -pythonpath /opt/pytorch/lib/python3.12/site-packages \
  -tts-text-encoder-dir /home/ubuntu/models/flan-t5-large-tokenizer \
  -sarvam-dir /home/ubuntu/models/sarvam-30b-fp8)

# 2. vLLM model name verification
curl -s 127.0.0.1:8000/v1/models | python3 -c \
  "import sys,json; m=json.load(sys.stdin)['data'][0]['id']; print('MODEL:', m); assert m=='sarvamai/sarvam-30b', f'Got {m}'"

# 3. Full pipeline test from laptop
ssh -L 8001:127.0.0.1:8001 -L 8002:127.0.0.1:8002 -L 8003:127.0.0.1:8003 ubuntu@<host>
# Then POST to http://127.0.0.1:8002/api/v3/voice/process with hi-IN language
```

### Expected Outcomes (at authorized session)

| Step | Expected |
|------|----------|
| Preflight | All PASS (artifacts, interpreter, ports free) |
| vLLM model check | `"sarvamai/sarvam-30b"` served |
| ASR alone | ready+warm; hi-IN WAV → non-empty text; silence → empty |
| Middle alone | strict JSON; `facility_id` not `safe_zone_id`; `data_version` correct |
| TTS alone | pre-generation completes; WAV header rate = 44,100 Hz |
| Full pipeline | `data_version` = `PKGDEMO-1:1`; hi-IN audio present |
| Browser+mic | hi-IN spoken → chips + Hindi audio; replay refused after lang switch |

### Negative Controls

| Scenario | Expected |
|----------|----------|
| vLLM returns wrong model name | hard stop — do not proceed |
| TTS starts without `STHIRA_TTS_VOICES_FILE` | zero voices; no synthesis possible |
| Middle receives malformed JSON | Go validator rejects; no prose emitted |
| Arrival without reservation | `OPEN_PANEL ARRIVAL_CONFIRMATION` only; no state mutation |
| ASR unknown language | `blocked` on `/health` |

---

## 6. Status

**READINESS: READY_FOR_REVIEW_UNVERIFIED**

Ten corrections identified and documented in the launch checklist. The most consequential:
- Missing `sthira-exercise` startup env vars (F3) — without these, the exercise server cannot route to any worker
- `STHIRA_TTS_VOICES_FILE` absence causes silent zero-voice behavior (F4)
- `STHIRA_TTS_DEVICE` defaults to CPU (F5) — easily forgotten

**Next action for Opus**: Merge `launch-checklist.md` into `plan/evidence/real-inference-launch-check.md` §7, or replace §7 with a reference to the new checklist. No source changes required.
