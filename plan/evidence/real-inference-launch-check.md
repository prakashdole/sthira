# Real-model launch readiness — corrected host paths & preflight checklist

Author: launch/preflight tooling owner (this session). Baseline: `CLEAN` at
`93fc752`. This supersedes path values recorded in `plan/prompt.md` and
`plan/evidence/execution-c04.md` for future launches; it does not erase or
invalidate the prior evidence, which remains historically accurate for the
session it documents.

## 1. Corrected host paths vs. repo-recorded paths

The owner supplied the following as **previously observed** (not freshly
re-verified) host paths for the AWS GPU instance `i-01d17e39266c292c2`
(region `us-east-2`, currently `stopped`, confirmed via a read-only
`aws ec2 describe-instances` call in this session — no start/stop performed).
These differ from the paths recorded in `plan/prompt.md` and
`plan/evidence/execution-c04.md`, which were written from an earlier
session's memory and are now understood to be inaccurate for at least the
model/interpreter directories:

| Purpose | Owner-corrected path (this session) | Path previously recorded in `plan/prompt.md` / `execution-c04.md` |
| --- | --- | --- |
| Sarvam-30B FP8 weights | `/home/ubuntu/models/sarvam-30b-fp8` | not previously given a host path (only "inside existing `sthira-sarvam` Docker container") |
| ASR artifact (`STHIRA_ASR_ARTIFACT_DIR`) | `/home/ubuntu/models/indic-conformer-600m-multilingual` | `/models/indic-conformer-600m-multilingual` |
| TTS artifact (`STHIRA_TTS_ARTIFACT_DIR`) | `/home/ubuntu/models/indic-parler-tts` | `/models/indic-parler-tts` |
| Python interpreter (`STHIRA_ASR_PYTHON` / `STHIRA_TTS_PYTHON`) | `/home/ubuntu/.venvs/voice/bin/python` | `/home/ubuntu/.venv/bin/python3` |
| `PYTHONPATH` (torch/transformers/parler_tts/onnxruntime) | `/opt/pytorch/lib/python3.12/site-packages` | not previously recorded as a separate `PYTHONPATH` entry |
| TTS description tokenizer (`STHIRA_TTS_TEXT_ENCODER_DIR`) | `/home/ubuntu/models/flan-t5-large-tokenizer` | not previously recorded (the code's `_resolve_description_tokenizer_path` fallback was undocumented at the path level) |

**Distinction: host paths vs. container mounts.** `deploy/docker-compose.demo.yml`
and `deploy/.env.example` mount `STHIRA_ASR_MODEL_HOST_DIR` /
`STHIRA_TTS_MODEL_HOST_DIR` (host) to fixed in-container paths
`/models/indic-conformer-600m-multilingual` and `/models/indic-parler-tts`
(container). The `/models/...` paths recorded in the older evidence describe
the **container-internal** mount target for the local Docker demo profile,
which is a distinct deployment path from the bare-metal EC2 GPU launch
procedure (Procedure B) that runs the Go worker binaries directly on the
host without a container for ASR/TTS. Do not conflate the two: a host path
correction does not change the container-mount contract, and vice versa.

None of the corrected paths above were freshly verified as existing on the
remote host in this session (the instance is stopped; this tool only ran
locally against this Mac, where none of these directories exist — see §3).
They are recorded as the owner's stated corrected values, to be used verbatim
at the next authorized launch and re-verified with `tools/preflight` running
**on the remote host** before any real inference is attempted.

## 2. Preserved existing infrastructure (unchanged by this session)

- `sthira-sarvam` vLLM Docker container: preserved as-is, private loopback
  port **8000**, serving `model_id = sarvamai/sarvam-30b`. Confirmed against
  `backend/internal/middleworker/sarvam_config.go`: `SarvamModelID =
  "sarvamai/sarvam-30b"` and the documented `SarvamVLLMStartupArgs()` pin
  `--host 127.0.0.1 --port 8000`. This session made no changes to that
  configuration.
- The middle worker (`middleworker`) is a **separate private Go service**
  (default `STHIRA_MIDDLE_ADDR=127.0.0.1:8002`) that calls the vLLM HTTP API
  at `STHIRA_VLLM_URL` (default `http://127.0.0.1:8000`); it does not load
  weights itself. Do not confuse the vLLM container's port 8000 with any
  other service that happens to reuse port 8000 (e.g. the legacy Python demo
  API in `deploy/RUNBOOK.md` also defaults to port 8000 — that is the
  retained-reference Python product, unrelated to the vLLM deployment, and
  the two must never be run against the same port simultaneously).
- Instance `i-01d17e39266c292c2`: confirmed `stopped`, no public IP assigned
  (both expected for a stopped instance). No start/stop/SSH/security-group
  change was performed. EBS volume `vol-0b4e1d279d21586e2` was not touched.

## 3. Local preflight tool

`tools/preflight/` (standalone Go module, standard library only) runs static,
offline checks:

- Module-aware `go build` for `asrworker`, `middleworker`, `ttsworker` with
  `GOPROXY=off` (no network module resolution).
- Greps each worker's `main.go` / Python adapter for every environment
  variable this document and `plan/prompt.md` claim it reads, failing
  explicitly if a documented flag/env var is not actually present in source.
- Confirms `SarvamModelID` and the documented vLLM port match the specified
  deployment.
- Checks existence (not content/weights) of the ASR/TTS artifact
  directories, the TTS description-tokenizer directory, the Sarvam weights
  directory, the Python interpreter (exists + executable bit, never
  invoked), and the `PYTHONPATH` site-packages directory.
- Checks the three private worker ports are loopback-only and currently
  free.
- Explicitly reports `NOT_RUN` (never a fabricated `PASS`) for vLLM
  reachability, AWS instance state/SSH, and any weight loading — all
  correctly out of scope for a local, offline, no-spend check.

Run: `cd tools/preflight && go run . [-asr-dir ... -tts-dir ... -python ... -pythonpath ... -tts-text-encoder-dir ...]`
(flags default to the corrected paths in §1; override with `-flag value` or
matching `STHIRA_*` env vars when running on the actual GPU host).

**Result on this local Mac (expected FAILs — this is not the GPU host):**
all Go builds, env-var-in-source checks, and model-ID/port checks **PASS**;
all host artifact/interpreter/PYTHONPATH directory checks **FAIL** because
none of those directories exist on this machine; loopback ports **PASS**
(free); remote/paid checks report **NOT_RUN**. Exit code 1 (as designed,
since failing artifact checks must not be silently downgraded to a warning).

## 4. Required real-inference checklist (future authorized run only)

None of the following is `PASS` today. Each requires the bounded, explicitly
authorized paid GPU session the owner must approve separately; this session
performed no paid inference.

1. **NOT_RUN** — Human Hindi microphone transcription, including silence.
2. **NOT_RUN** — Sarvam silent zoom and destination requests with correct
   contextual identifiers (facility IDs for destinations, not safe-zone IDs).
3. **NOT_RUN** — Strict action JSON: reject thinking/prose leakage,
   fabricated entities, and invalid actions (prior probe on 2026-09-25 showed
   both failure modes — 256-token exhaustion with incomplete JSON, and a
   hallucinated `speech_key` correctly rejected by the independent Go
   validator; see `plan/evidence/live-inference-2026-09-25.md`).
4. **NOT_RUN** — Approved Indic Parler-TTS output with truthful rate
   (the model's `config.sampling_rate`, recorded as observed and never relabeled), digest, and voice-description
   provenance from `STHIRA_TTS_VOICES_FILE`.
5. **NOT_RUN** — Arrival confirmation only (`OPEN_PANEL` /
   `ARRIVAL_CONFIRMATION`), followed by an explicit user-confirmed server
   operation; no model action may itself mutate reservation state.
6. **NOT_RUN** — Worker outage, language switch, and stale-response/replay
   invalidation.
7. **NOT_RUN** — Laptop and phone-browser microphone permission, denied
   permission, and playback-gesture behavior.

Latency and speech-quality review will be recorded here after the
authorized run, not estimated in advance.

## 5. Explicit non-claims (do not restate these as fixed facts elsewhere)

- **No automatic ASR language detection is claimed or implemented.** The
  adapter's `decode()` takes `lang_bcp47` as an explicit parameter; language
  is UI-selected and supplied by the caller, never inferred from audio.
- **No GPU ASR is claimed.** The only verified ASR execution
  (`plan/evidence/live-inference-2026-09-25.md`) ran ONNX inference on
  **CPU**. GPU ASR execution has not been demonstrated.
- **No claim of a current/latest end-to-end real-inference run.** The most
  recent real-model evidence is the 2026-09-25 session, which was bounded,
  partially failing (destination IDs, arrival wording, full-pipeline TTS
  failures per `plan/prompt.md`'s "Failures before the last unverified
  drafts" section), and is **not** superseded by a newer real-model run in
  this session. This session performed zero paid inference; all items in
  §4 remain `NOT_RUN`.

## 6. What this session verified locally (evidence, not inference)

- `tools/preflight` builds, vets, and runs successfully on this Mac (see §3).
- `go build ./...` and `go vet ./...` pass with zero errors for the backend
  root module and all three nested worker modules (`asrworker`,
  `middleworker`, `ttsworker`) at the pinned baseline commit `93fc752`,
  verified via a clean `git archive` export to avoid disturbing Agent 1/2's
  concurrent unstaged work in the shared checkout.
- `aws ec2 describe-instances --instance-ids i-01d17e39266c292c2` (read-only)
  returned state `stopped`, `PublicIpAddress: null` — consistent with prior
  handoff notes. No mutating AWS call was made.

## 7. Reconciled launch commands and ordered paid smoke sequence (2026-09-27, local reconciliation only)

Reconciled against source, not memory: `backend/internal/{asrworker,middleworker,ttsworker}/cmd/*/main.go`,
`src/sthira_v2/speech_{asr,tts}_adapter.py`, `middleworker/sarvam_config.go`, `tools/preflight/main.go`.
Nothing here was executed against the GPU host.

**Corrections to `plan/prompt.md` Procedure B (which is now superseded by this section):**

- Paths: use §1 corrected host paths (`/home/ubuntu/models/...`, `/home/ubuntu/.venvs/voice/bin/python`),
  not `/models/...` (container mount) or `/home/ubuntu/.venv/bin/python3`.
- `PYTHONPATH` is **not** read by any Go worker; the child adapter inherits the worker's environment
  (`cmd.Env = append(os.Environ(), ...)`), so export `PYTHONPATH` in the worker's shell.
  `STHIRA_PYTHONPATH` is only read by opt-in adapter tests.
- `STHIRA_{ASR,TTS}_ARTIFACT_DIR`, `STHIRA_TTS_DEVICE`, `STHIRA_TTS_TEXT_ENCODER_DIR`,
  `STHIRA_TTS_VOICES_FILE`, `STHIRA_*_APPROVED_LANGUAGES` are read by the Python adapters (inherited env),
  not by Go `main.go`. Procedure B omitted `STHIRA_TTS_TEXT_ENCODER_DIR` and `PYTHONPATH`.
- The middle worker sends `model = "sarvamai/sarvam-30b"` (`SarvamModelID`) with `max_tokens 512` and a
  request-level JSON schema. The `sthira-sarvam` container must serve that exact name: check
  `curl -s 127.0.0.1:8000/v1/models`. If the served name differs, stop — do not edit code on the host.
  The reported revision label defaults to `sarvam-30b-fp8-v1` (`STHIRA_MIDDLE_REVISION`); it is a label,
  not a weight digest.
- Ports (loopback only): vLLM 8000 (container), ASR 8001, middle 8002, TTS 8003. Do not reuse 8000.
- TTS `ttsworker` refuses to start without `STHIRA_TTS_SYNTHETIC_EXERCISE=1` (demo catalog only) and
  pre-generates the approved template catalog at startup; hot path is cache-only.
- ASR language is always the UI-selected `language` field (`X-Language` / pipeline `language`).
  No autodetection exists.

**Host environment (remote shell, once):**

```bash
export PYTHONPATH=/opt/pytorch/lib/python3.12/site-packages
export PY=/home/ubuntu/.venvs/voice/bin/python
export TOK="$(openssl rand -hex 24)"   # shared worker token; never commit or paste into evidence
cd ~/MonitoringZ && git rev-parse HEAD  # record; must equal the reviewed local commit
(cd tools/preflight && go run .)        # must show artifact/interpreter/PYTHONPATH PASS on the host
```

**Ordered smoke sequence — stop at the first FAIL; each step has a budget.**

| # | Step | Command sketch | Pass criterion | Budget |
|---|---|---|---|---|
| 0 | Instance + preflight | owner-authorized start; preflight above | all host checks PASS | 10 min |
| 1 | vLLM up | `docker start sthira-sarvam`; `curl 127.0.0.1:8000/v1/models` | lists `sarvamai/sarvam-30b` | 10 min |
| 2 | ASR worker alone | `STHIRA_ASR_ADDR=127.0.0.1:8001 STHIRA_ASR_TOKEN=$TOK STHIRA_ASR_PYTHON=$PY STHIRA_ASR_ADAPTER=sthira_v2.speech_asr_adapter STHIRA_ASR_ARTIFACT_DIR=/home/ubuntu/models/indic-conformer-600m-multilingual ./asrworker`; `GET /health` | ready+warm, languages listed; one hi-IN WAV → non-empty text; silence → no command | 10 min |
| 3 | Middle worker alone | `STHIRA_MIDDLE_ADDR=127.0.0.1:8002 STHIRA_MIDDLE_TOKEN=$TOK STHIRA_VLLM_URL=http://127.0.0.1:8000 ./middleworker`; three typed requests (silent zoom, destination, arrival) | strict JSON only; destination uses facility IDs; arrival = `OPEN_PANEL ARRIVAL_CONFIRMATION`; no prose/thinking; invalid output rejected, not repaired | 15 min |
| 4 | TTS worker alone | `STHIRA_TTS_ADDR=127.0.0.1:8003 STHIRA_TTS_TOKEN=$TOK STHIRA_TTS_PYTHON=$PY STHIRA_TTS_ADAPTER=sthira_v2.speech_tts_adapter STHIRA_TTS_ARTIFACT_DIR=/home/ubuntu/models/indic-parler-tts STHIRA_TTS_TEXT_ENCODER_DIR=/home/ubuntu/models/flan-t5-large-tokenizer STHIRA_TTS_DEVICE=cuda STHIRA_TTS_SYNTHETIC_EXERCISE=1 STHIRA_TTS_VOICES_FILE=<reviewed voices JSON; operator-supplied, not in the repo> ./ttsworker` | startup pre-generation completes; en-IN and hi-IN cache hits; WAV header rate equals the declared `model.config.sampling_rate` (record the value), checksum matches; unknown template/digest rejected | 20 min |
| 4b | TTS voices file | Before step 4: start once with `STHIRA_TTS_VOICES_FILE` unset | adapter probe is BLOCKED ("no approved voice descriptions"); never a fabricated default voice. With the reviewed file (`{"<bcp47>": {"<name>": "<description>"}}`) only listed names ∩ approved languages are advertised | 2 min |
| 5 | Full pipeline | SSH `-L 8001/8002/8003`; exercise server: `STHIRA_EXERCISE_SEED=1 STHIRA_DATABASE_DSN=<disposable DB with migrations 0001–0010 applied; placeholder> STHIRA_ASR_URL=http://127.0.0.1:8001 STHIRA_ASR_TOKEN=$TOK STHIRA_MIDDLE_URL=http://127.0.0.1:8002 STHIRA_MIDDLE_TOKEN=$TOK STHIRA_TTS_URL=http://127.0.0.1:8003 STHIRA_TTS_TOKEN=$TOK ./sthira-exercise`; `POST /api/v3/voice/process` (en-IN destination, hi-IN destination, silent zoom) | `data_version` equals guidance `PKGDEMO-1:1`; hi-IN audio `language=hi-IN`; no `stage_failures` on success; zoom has no TTS | 15 min |
| 6 | Real browser + microphone | Vite → local backend; laptop Safari/phone browser | mic permission grant/deny; spoken hi-IN destination → chips + audible approved Hindi; replay refused after language switch; arrival only after server ack | 20 min |

Hard stop: 100 minutes from instance start, or any FAIL in steps 1–4 that is not a documented
configuration typo. Stop the instance (not terminate) at the end; confirm `stopped` read-only.

**Preserve as evidence (compact Markdown in this file, no raw logs/recordings/weights/tokens):**
commit SHA; `/v1/models` served name; each step's PASS/FAIL with HTTP status and 1–2 line excerpts;
TTS WAV header fields + checksums (not audio bytes); per-step latency; the exact failing request/response
shape for any FAIL. Keep evidence classes separate: adapter smoke, full pipeline, browser, microphone,
human listening quality. Human speech quality is recorded only as a reviewer's observation, never inferred.
