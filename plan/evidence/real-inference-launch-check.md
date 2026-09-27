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
   (44,100 Hz native, never relabeled), digest, and voice-description
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
