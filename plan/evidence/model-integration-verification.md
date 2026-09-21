# Model Integration Verification Evidence

**Date:** 2026-09-21  
**Worker:** Parallel Worker C (codex/model-integration-evidence)  
**Target Revisions Inspected:**
- Baseline CLEAN: `4095400`
- Worker B branch `codex/p567-inference-corrections`: `3e6daac`
- Current integration repair branch `codex/p567-integration-repair`: `b3ddc9a`
- Worker A branch `codex/p567-backend-corrections`: `e0babda`

**Governing Rules & Constraints:**
- Sthira v2 rules (`plan/rules.md`, `plan/architecture.md`, `plan/decisions.md`, `plan/open-decisions.md`).
- User-selected models are fixed:
  1. **ASR:** AI4Bharat IndicConformer-600M-Multi (`ai4bharat/indic-conformer-600m-multilingual`)
  2. **Middle:** Sarvam-30B (`sarvamai/sarvam-30b` / `sarvamai/sarvam-30b-fp8`, 2.4B active non-embedding parameters)
  3. **TTS:** AI4Bharat Indic Parler-TTS (`ai4bharat/indic-parler-tts`)
- External authority & gate preservation: O03 (ASR language evaluation matrix) and O11 (TTS verified regional translations) remain open external approval blockers.
- No paid API calls, GPU rentals, weight downloads, or production credentials were used. Probes without local weights report `NOT_RUN`.

---

## 1. Executive Summary & Implementation Evidence Table

| Model | Current Code Assumption (at 3e6daac / b3ddc9a) | Verified API / Config | Pinned Primary Source | Required Correction | Verification Still Needed |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **ASR** (IndicConformer-600M-Multi) | Guesses ONNX layout (`preprocessor.ts` at root, `assets/joint_post_net_hi.onnx` executed alone). Fabricated confidence=1.0 in `local_voice.py`; stub transcript `[unverified:real-inference-stub]` in `3e6daac`. Manifest checks for `.safetensors`/`config.yaml`. | Custom PyTorch/ONNX hybrid via `model_onnx.py` (`IndicASRModel`). Preprocessor is TorchScript in `assets/preprocessor.ts`. Acoustic encoder is `assets/encoder.onnx`. CTC decoder is `assets/ctc_decoder.onnx` with `assets/language_masks.json` and `assets/vocab.json`. Forward returns `str` only (NO confidence score). | [AI4Bharat IndicConformer ONNX](https://huggingface.co/ai4bharat/bhili-asr-conformer-600m-onnx/raw/main/model_onnx.py), commit `main`, MIT License, retrieved 2026-09-21. | Check full ONNX asset inventory (`assets/preprocessor.ts`, `assets/encoder.onnx`, `assets/ctc_decoder.onnx`, `vocab.json`, `language_masks.json`). Audio must be 16 kHz 1D/2D float32 tensor. Return transcript string; set `confidence: None` (unknown). Remove fake 1.0. | Real local weights benchmark & WER measurement on Indian languages (blocked by O03). |
| **TTS** (Indic Parler-TTS) | Calls `AutoModel.from_pretrained(..., trust_remote_code=False)`, then `model.generate(text=..., language=..., voice=...)` expecting `.wav`. Hardcoded sample rate 22,050 Hz. | Dedicated class `ParlerTTSForConditionalGeneration` from `parler_tts`. Dual tokenizers: prompt tokenizer (model repo) + description tokenizer (`model.config.text_encoder._name_or_path`). Generates audio tensor `(1, N)`. Native sampling rate is **44,100 Hz** from `model.config.sampling_rate`. | [Parler-TTS Official Repo](https://github.com/huggingface/parler-tts), PyPI `parler-tts==0.2.3`, Apache-2.0 License; [ai4bharat/indic-parler-tts](https://huggingface.co/ai4bharat/indic-parler-tts), retrieved 2026-09-21. | Import `ParlerTTSForConditionalGeneration` from `parler_tts`. Tokenize text and description separately. Pass `model.generate(input_ids=desc_ids, prompt_input_ids=prompt_ids)`. Extract squeezed numpy array and encode PCM16LE WAV at `model.config.sampling_rate` (44.1 kHz). Hardcoding 22,050 Hz halves playback speed and doubles duration! | Real local weights audio fidelity & RTF measurement (blocked by O11). |
| **Middle** (Sarvam-30B) | Invented ChatML template (`<|im_start|>...<|im_end|>`) with dead Jinja variable `{%- set enable_thinking = false -%}` never used in template. Believed client can pass per-request chat template to vLLM. Speculative vLLM flags. | Official Gemma-style template (`[@BOS@]`, `<|start_of_turn|><|system|>...<|end_of_turn|>`). Thinking suppression token is `<|nothink|>` appended to user message. vLLM applies server-level template only; per-request template overrides in `/v1/chat/completions` are unsupported. Structured output enforced via vLLM `response_format` (outlines/xgrammar). MoE architecture: 19 layers, 128 experts, top-6 routed, 2.4B active params, ~30 GB resident in FP8 (`sarvamai/sarvam-30b-fp8`). | [Sarvam-30B Model Card & chat_template.jinja](https://huggingface.co/sarvamai/sarvam-30b/raw/main/chat_template.jinja), [config.json](https://huggingface.co/sarvamai/sarvam-30b/raw/main/config.json), [hotpatch_vllm.py](https://huggingface.co/sarvamai/sarvam-30b/raw/main/hotpatch_vllm.py), Apache-2.0 License, retrieved 2026-09-21. | Remove ChatML template from client request path. Launch vLLM with official template. Append `<|nothink|>` to user prompt content or pass `chat_template_kwargs={"enable_thinking": False}` if supported. Pin vLLM 0.15.0 with `hotpatch_vllm.py` or vLLM PR #33942. Target Ada/Hopper GPU for native FP8. Distinguish 30B resident weights from 2.4B active compute. | GPU hardware allocation & token-latency benchmarking on target server (requires GPU access). |

---

## 2. ASR: AI4Bharat IndicConformer-600M-Multi Evidence

### 2.1 Pinned Primary Sources
- **Hugging Face Model Hub:** `https://huggingface.co/ai4bharat/indic-conformer-600m-multilingual` (Gated model; requires HF authentication).
- **Inference Reference Implementation:** `https://huggingface.co/ai4bharat/bhili-asr-conformer-600m-onnx/raw/main/model_onnx.py` (MIT License, inspected commit `main` 2026-09-21).
- **Upstream Repository:** `https://github.com/AI4Bharat/IndicConformerASR` and NeMo ASR integration.
- **License:** MIT License.

### 2.2 Exact Artifact Format & Directory Structure
The deployable ONNX artifact for IndicConformer does **not** use raw `.safetensors` or `.bin` checkpoints for inference. Instead, it is an ONNX/TorchScript composite:

```
<model_dir>/
├── config.json                 # Model hyper-parameters (BLANK_ID=256, SOS=5632, etc.)
├── model_onnx.py               # Hugging Face PreTrainedModel wrapper class IndicASRModel
└── assets/
    ├── preprocessor.ts         # TorchScript JIT module for filterbank feature extraction
    ├── encoder.onnx            # Conformer acoustic encoder (audio_signal, length) -> (outputs, encoded_lengths)
    ├── ctc_decoder.onnx        # CTC decoder projection: encoder_output -> logprobs
    ├── rnnt_decoder.onnx       # RNN-T decoder/prediction net
    ├── joint_enc.onnx          # Joint encoder projection
    ├── joint_pred.onnx         # Joint prediction net projection
    ├── joint_pre_net.onnx      # Joint pre-net
    ├── joint_post_net_<lang>.onnx # Language-specific RNN-T heads (22 languages)
    ├── language_masks.json     # Vocabulary masks per language code for CTC decoding
    └── vocab.json              # Token-to-character vocabulary dictionaries
```

### 2.3 Concrete Failures in Current Code
1. **Guessed Asset Paths in `3e6daac`:**
   In `src/sthira_v2/speech_asr_adapter.py` (`3e6daac`), `_artifact_files_present` checked:
   `["config.json", "preprocessor.ts", "model_onnx.py", "assets/encoder.onnx"]`
   `preprocessor.ts` was looked for at the root instead of under `assets/`. `vocab.json` and `language_masks.json` were omitted completely.
2. **Guessed Manifest in `manifest.go`:**
   In `backend/internal/asrworker/manifest.go`, `ScanLocalInventory` looked for `pytorch_model.bin`, `model.safetensors`, `consolidated.safetensors`, `config.yaml`. None of these exist in the ONNX deployment artifact.
3. **Inference Never Executed:**
   In `3e6daac`, `speech_asr_adapter.py` instantiated sessions but in `_run()` only evaluated audio energy and returned `[unverified:real-inference-stub]`.
4. **Fabricated Confidence in `local_voice.py`:**
   `local_voice.py` line 52: `return self.model(wav, model_language, "ctc"), 1.0`. The `1.0` was invented because the underlying model does not output a confidence score.

### 2.4 Verified Execution Mechanics
- **Audio Inputs:** Single-channel mono PCM audio, 16,000 Hz sample rate. Must be decoded to little-endian float32 in range `[-1.0, 1.0]`. Input tensor: `torch.Tensor` of shape `[1, num_samples]` or `[num_samples]`.
- **Preprocessing:** Audio tensor is processed by `torch.jit.load(f"{ts_folder}/assets/preprocessor.ts")` which outputs `audio_signal` (spectrogram/filterbank) and `length`.
- **Encoder Execution:**
  ```python
  outputs, encoded_lengths = encoder_session.run(
      ['outputs', 'encoded_lengths'],
      {'audio_signal': audio_signal.cpu().numpy(), 'length': length.cpu().numpy()}
  )
  ```
- **CTC Decoding (Recommended for Fast Deterministic Inference):**
  ```python
  logprobs = ctc_session.run(['logprobs'], {'encoder_output': outputs})[0]
  # Slice by language mask
  logprobs = torch.from_numpy(logprobs[:, :, language_masks[lang]]).log_softmax(dim=-1)
  indices = torch.argmax(logprobs[0], dim=-1)
  collapsed = torch.unique_consecutive(indices, dim=-1)
  transcript = ''.join([vocab[lang][x] for x in collapsed if x != config.BLANK_ID]).replace('▁', ' ').strip()
  ```
- **Confidence Semantics:** Upstream `IndicASRModel` returns **only string text**. No calibrated word-level or sentence-level posterior is emitted. The adapter **must report confidence as null (`None`)**, signalling to the Go worker that confidence is uncalibrated/unknown.
- **Language Codes:** 2-letter ISO codes (`hi`, `ml`). Sthira's BCP-47 (`hi-IN`, `ml-IN`) maps to `hi`, `ml`.

### 2.5 Minimal Implementation Snippet for Integration Worker
```python
import importlib.util
from pathlib import Path
import numpy as np
import torch

def load_indic_conformer_model(artifact_dir: str):
    p = Path(artifact_dir)
    # 1. Structural verification
    required = [
        p / "model_onnx.py",
        p / "config.json",
        p / "assets" / "preprocessor.ts",
        p / "assets" / "encoder.onnx",
        p / "assets" / "ctc_decoder.onnx",
        p / "assets" / "vocab.json",
        p / "assets" / "language_masks.json",
    ]
    for f in required:
        if not f.is_file():
            raise FileNotFoundError(f"Missing required ASR artifact: {f}")

    # 2. Dynamic import of model_onnx.py
    spec = importlib.util.spec_from_file_location("sthira_indic_asr", p / "model_onnx.py")
    if spec is None or spec.loader is None:
        raise ImportError(f"Cannot load module from {p / 'model_onnx.py'}")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)

    config = mod.IndicASRConfig(ts_folder=str(p))
    model = mod.IndicASRModel(config)
    
    # 3. Warm-up pass to verify readiness
    dummy = torch.zeros((1, 16000), dtype=torch.float32)
    with torch.no_grad():
        _ = model(dummy, lang="hi", decoding="ctc")

    def transcribe(samples_float32: list[float], sample_rate: int, lang_code: str) -> dict:
        if sample_rate != 16000:
            raise ValueError(f"IndicConformer requires 16000 Hz, got {sample_rate}")
        lang_key = "hi" if "hi" in lang_code else ("ml" if "ml" in lang_code else None)
        if not lang_key:
            raise ValueError(f"Unsupported language {lang_code}")
        
        wav = torch.tensor([samples_float32], dtype=torch.float32)
        with torch.no_grad():
            text = model(wav, lang=lang_key, decoding="ctc")
        
        # Confidence is not calibrated by upstream; return None
        return {
            "text": text,
            "confidence": None,
        }

    return transcribe
```

---

## 3. TTS: AI4Bharat Indic Parler-TTS Evidence

### 3.1 Pinned Primary Sources
- **Hugging Face Model Hub:** `https://huggingface.co/ai4bharat/indic-parler-tts` (Gated model; requires HF authentication).
- **Core Architecture Framework:** `https://github.com/huggingface/parler-tts` (Inspected package `parler_tts==0.2.3`, Apache-2.0 License, 2026-09-21).
- **License:** Apache-2.0 License.

### 3.2 Concrete Failures in Current Code
1. **Generic `AutoModel` Fallacy:**
   `speech_tts_adapter.py` (`3e6daac`) called:
   `AutoModel.from_pretrained(art_dir, trust_remote_code=False)`
   `out = model.generate(text=text, language=language, voice=voice or "default")`
   `wav = getattr(out, "wav", None)`
   `AutoModel` in `transformers` does not implement Parler-TTS. It has no `.generate(text=..., language=..., voice=...)` method and produces no object with a `.wav` attribute.
2. **Catastrophic Sampling Rate Bug (2x Audio Distortion):**
   `speech_tts_adapter.py` assumed `TARGET_SAMPLE_RATE = 22050` Hz and wrote audio frames into a 22,050 Hz WAV header.
   **Fact:** `ai4bharat/indic-parler-tts` natively outputs audio at **44,100 Hz** (`model.config.sampling_rate == 44100`).
   Writing 44.1 kHz audio into a 22.05 kHz WAV file without resampling causes the audio to play at **half speed and double duration**, severely degrading intelligibility.
3. **Dual Tokenizer Ignored:**
   Parler-TTS requires two tokenizers:
   - A prompt tokenizer for the text to be spoken (`AutoTokenizer.from_pretrained(model_dir)`).
   - A description tokenizer for the speaker voice prompt (`AutoTokenizer.from_pretrained(model.config.text_encoder._name_or_path)`, e.g., Flan-T5 tokenizer).
4. **Invalid Dependencies in `requirements-voice.txt`:**
   `requirements-voice.txt` listed `torch==2.14.0`, `torchaudio==2.11.0`, and `safetensors==0.8.0`, which are fabricated version numbers that do not exist.

### 3.3 Verified Class, Signatures & Dependencies
- **Class:** `from parler_tts import ParlerTTSForConditionalGeneration`.
- **Generation Signature:**
  ```python
  generation = model.generate(
      input_ids=description_input_ids,   # Description prompt tokens (from text_encoder tokenizer)
      prompt_input_ids=prompt_input_ids  # Spoken text tokens (from prompt tokenizer)
  )
  ```
- **Output:** PyTorch tensor of shape `(batch_size, num_samples)` with float values in `[-1.0, 1.0]`.
  Squeezed to 1D numpy array: `audio_samples = generation.cpu().numpy().squeeze()`.
- **Native Sampling Rate:** `sample_rate = model.config.sampling_rate` (44,100 Hz).
- **Remote Code & Least Privilege:**
  When `parler-tts` is installed (`pip install parler-tts==0.2.3`), `ParlerTTSForConditionalGeneration.from_pretrained` loads natively with `trust_remote_code=False` and `local_files_only=True`. There is no need to execute untrusted remote code.

### 3.4 Minimal Implementation Snippet for Integration Worker
```python
import io
import struct
import wave
from pathlib import Path
import numpy as np
import torch
from parler_tts import ParlerTTSForConditionalGeneration
from transformers import AutoTokenizer

def load_indic_parler_tts(artifact_dir: str):
    p = Path(artifact_dir)
    if not (p / "config.json").is_file():
        raise FileNotFoundError(f"Missing config.json in {artifact_dir}")

    # Load model and tokenizers strictly locally
    model = ParlerTTSForConditionalGeneration.from_pretrained(
        str(p),
        local_files_only=True,
        trust_remote_code=False,
    )
    model.eval()

    prompt_tokenizer = AutoTokenizer.from_pretrained(str(p), local_files_only=True)
    # Description tokenizer points to text_encoder path (e.g. google/flan-t5-large or local copy)
    desc_tokenizer = AutoTokenizer.from_pretrained(
        str(p / "description_tokenizer") if (p / "description_tokenizer").is_dir() else model.config.text_encoder._name_or_path,
        local_files_only=True if (p / "description_tokenizer").is_dir() else False,
    )

    sampling_rate = model.config.sampling_rate  # 44100 Hz

    # Standard authoritative voice description
    default_desc = "A clear, moderate-paced voice with neutral tone and high recording quality."

    # Warm-up pass
    w_desc = desc_tokenizer(default_desc, return_tensors="pt").input_ids
    w_prompt = prompt_tokenizer("परीक्षण", return_tensors="pt").input_ids
    with torch.no_grad():
        _ = model.generate(input_ids=w_desc, prompt_input_ids=w_prompt)

    def synthesize(text: str, description: str = "") -> tuple[bytes, float, int]:
        desc_text = description if description.strip() else default_desc
        desc_ids = desc_tokenizer(desc_text, return_tensors="pt").input_ids
        prompt_ids = prompt_tokenizer(text, return_tensors="pt").input_ids

        with torch.no_grad():
            output_tensor = model.generate(input_ids=desc_ids, prompt_input_ids=prompt_ids)

        samples = output_tensor.cpu().numpy().squeeze()
        duration_secs = len(samples) / float(sampling_rate)

        # Convert to PCM16LE WAV at EXACT sampling_rate (44100 Hz)
        buf = io.BytesIO()
        with wave.open(buf, "wb") as wf:
            wf.setnchannels(1)
            wf.setsampwidth(2)
            wf.setframerate(sampling_rate)
            frames = bytearray()
            for s in samples:
                v = max(-1.0, min(1.0, float(s)))
                frames.extend(struct.pack("<h", int(v * 32767)))
            wf.writeframes(bytes(frames))

        return buf.getvalue(), duration_secs, sampling_rate

    return synthesize
```

---

## 4. Middle Model: Sarvam-30B Evidence

### 4.1 Pinned Primary Sources
- **Hugging Face Model Hub:** `https://huggingface.co/sarvamai/sarvam-30b` (Apache-2.0 License).
- **FP8 Quantized Repository:** `https://huggingface.co/sarvamai/sarvam-30b-fp8` (Apache-2.0 License).
- **Official Jinja Template:** `https://huggingface.co/sarvamai/sarvam-30b/raw/main/chat_template.jinja`.
- **Model Config:** `https://huggingface.co/sarvamai/sarvam-30b/raw/main/config.json`.
- **Serving Script:** `https://huggingface.co/sarvamai/sarvam-30b/raw/main/hotpatch_vllm.py` targeting `vllm==0.15.0`.
- **Upstream vLLM PR:** `https://github.com/vllm-project/vllm/pull/33942`.
- **Exact Retrieval Date:** 2026-09-21.

### 4.2 Architectural Parameters (from `config.json`)
- `model_type`: `"sarvam_moe"`
- `architectures`: `["SarvamMoEForCausalLM"]`
- `num_hidden_layers`: 19
- `hidden_size`: 4096
- `num_attention_heads`: 64
- `num_key_value_heads`: 4 (GQA 16:1 ratio)
- `intermediate_size`: 8192 (dense layers)
- `num_experts`: 128
- `num_experts_per_tok`: 6 (top-6 routed)
- `num_shared_experts`: 1
- `moe_intermediate_size`: 1024
- `vocab_size`: 262,144
- `max_position_embeddings`: 131,072
- **Parameters:**
  - Total resident parameter weights: **~30 Billion**
  - Active non-embedding compute parameters per token: **2.4 Billion**

### 4.3 Chat Template & Reasoning Controls
#### Concrete Failures in `sarvam_config.go`
In `backend/internal/middleworker/sarvam_config.go`, Worker B wrote:
```jinja2
{%- set enable_thinking = false -%}
{%- for message in messages -%}
{%- if message.role == 'system' -%}
<|im_start|>system
{{ message.content }}<|im_end|>
...
```
This failed because:
1. It used ChatML syntax (`<|im_start|>`, `<|im_end|>`), whereas Sarvam-30B uses Gemma-style `<|start_of_turn|>` tags.
2. `{%- set enable_thinking = false -%}` was never consulted in the template body!
3. vLLM's `/v1/chat/completions` API **does not support per-request chat template overrides**. The template is configured at the server level.

#### The Verified Official Jinja Template (`chat_template.jinja`)
Key tokens and formatting from the official repository:
- **BOS:** `[@BOS@]\n`
- **System Turn:** `<|start_of_turn|><|system|>\n{{ content }}<|end_of_turn|>\n`
- **User Turn:** `<|start_of_turn|><|user|>\n{{ content }}`
- **Thinking Suppression (Line 44):**
  ```jinja2
  {{- '<|nothink|>' if (enable_thinking is defined and not enable_thinking and not visible_text(m.content).endswith("<|nothink|>")) else '' -}}
  ```
  When `enable_thinking` is False, the token `<|nothink|>` is appended to the user prompt immediately before `<|end_of_turn|>\n`!
- **Assistant Turn:** `<|start_of_turn|><|assistant|>\n` followed by `<think>...</think>` (or `<think></think>` if empty) and response text, ending with `<|end_of_turn|>\n`.
- **Generation Prompt:** `<|start_of_turn|><|assistant|>\n`.

#### How to Enforce Reasoning Suppression & Structured JSON
1. **Server Configuration:** Use the model's official `chat_template.jinja` at vLLM startup (or set `enable_thinking = false` inside the server's default template).
2. **Client Action in Go Middleware (`middleworker`):**
   In the user message payload sent to `/v1/chat/completions`, ensure the message text ends with `<|nothink|>` (or pass `"chat_template_kwargs": {"enable_thinking": false}`).
3. **Structured Output (Guided Decoding):**
   vLLM provides guided generation using `--guided-decoding-backend outlines` (or `xgrammar`).
   In the `/v1/chat/completions` request body, specify:
   ```json
   {
     "model": "sarvamai/sarvam-30b-fp8",
     "messages": [...],
     "temperature": 0.0,
     "max_tokens": 256,
     "response_format": {
       "type": "json_schema",
       "json_schema": {
         "name": "model_output",
         "strict": true,
         "schema": { ... }
       }
     }
   }
   ```
   Guided decoding guarantees that token sampling is restricted strictly to characters forming valid JSON matching the schema, mathematically preventing `<think>` tokens from leaking into the output.

### 4.4 Serving Stack, FP8 Quantization & Hardware Constraints
1. **vLLM Support:**
   - Native PR: `vllm-project/vllm/pull/33942`.
   - Sarvam official hotpatch: `hotpatch_vllm.py` applied against `vllm==0.15.0`.
2. **Pre-Quantized Repository:**
   - Use `sarvamai/sarvam-30b-fp8` on Hugging Face rather than on-the-fly FP8 quantization.
3. **Hardware Prerequisites:**
   - Resident Memory: ~30 GB VRAM in FP8 (plus KV cache allocation; requires a 48 GB or 80 GB GPU like A100-80GB, H100, L40S, or dual L4).
   - FP8 Compute: Requires NVIDIA Ada Lovelace (Compute Capability 8.9) or Hopper (Compute Capability 9.0) for native FP8 Tensor Cores. On Ampere (A100, CC 8.0), FP8 weights require software dequantization.
   - Recommended Environment Variable: `VLLM_USE_FLASHINFER_MOE_FP8=0` to bypass compilation bugs with FlashInfer FP8 MoE kernels.

### 4.5 Recommended Server Launch Command
```bash
python3 -m vllm.entrypoints.openai.api_server \
  --model sarvamai/sarvam-30b-fp8 \
  --trust-remote-code \
  --dtype auto \
  --max-model-len 4096 \
  --gpu-memory-utilization 0.90 \
  --guided-decoding-backend outlines \
  --host 127.0.0.1 \
  --port 8000
```

---

## 5. Readiness Definition & Mock vs Real Boundary

### 5.1 Honest Readiness State Machine
A model worker must never report `READY=true` based on file existence or dummy `{}` files. Readiness must be earned via a verified 5-step sequence:

```
[INIT / BLOCKED]
      │
      ▼
1. Validate filesystem artifacts (SHA-256 matches catalog; all files present)
      │ (fail -> BLOCKED)
      ▼
2. Import dependencies (torch, onnxruntime, transformers, parler_tts)
      │ (fail -> BLOCKED_DEPENDENCY)
      ▼
3. Construct model instances (InferenceSession, from_pretrained)
      │ (fail -> BLOCKED_LOAD)
      ▼
4. Warm-up dry run (execute 1 inference pass with synthetic zero input)
      │ (fail -> BLOCKED_WARMUP)
      ▼
[READY = true]
```

### 5.2 Fake Dependencies vs Real Local Artifacts
- **Fake Dependencies (Component & IPC Tests):**
  - Used in CI and developer environments where weights and GPUs are absent.
  - Return deterministic dummy responses conforming to wire schema (`request_id`, `text`, `confidence: None`, `audio_b64`).
  - Must never pretend to be real weights or claim external approval.
- **Real Local Artifacts (Integration & Production):**
  - Require explicit environment variables: `STHIRA_ASR_ARTIFACT_DIR`, `STHIRA_TTS_ARTIFACT_DIR`.
  - When absent, the adapter responds with:
    ```json
    {"status": "blocked", "model_id": "...", "reason": "artifact gate not ready (O03/O11)"}
    ```
  - Probes exit with `STATUS: NOT_RUN`.

---

## 6. Opt-In Diagnostic Probes

Three source-only opt-in probes have been placed in `plan/evidence/model-api-probes/`:
1. `plan/evidence/model-api-probes/probe_indic_conformer_asr.py`:
   - Validates presence of `assets/preprocessor.ts`, `assets/encoder.onnx`, `assets/ctc_decoder.onnx`, `vocab.json`, `language_masks.json`.
   - Runs synthetic 16 kHz audio through TorchScript preprocessor and ONNX CTC decoder.
   - When run without `--model-dir`, exits with `STATUS: NOT_RUN` (verified).
2. `plan/evidence/model-api-probes/probe_indic_parler_tts.py`:
   - Validates `config.json` native sampling rate (44,100 Hz).
   - Loads `ParlerTTSForConditionalGeneration` with dual tokenizers and verifies WAV encoding.
   - When run without `--model-dir`, exits with `STATUS: NOT_RUN` (verified).
3. `plan/evidence/model-api-probes/probe_sarvam_chat_template.py`:
   - Runs offline structural test on official Sarvam Jinja template.
   - Verifies injection of `<|nothink|>` when `enable_thinking=False` and presence of `<|start_of_turn|>` tokens (verified: PASS).

---

## 7. Unresolved Questions & External Gates

| Category | Item | Status | Action Required |
| :--- | :--- | :--- | :--- |
| **Authority** | O03: ASR Language Evaluation Matrix | OPEN | Requires state disaster management authority approval for Hindi and Malayalam dialect thresholds before production deployment. |
| **Authority** | O11: Approved Audio Translations | OPEN | Requires certified human translations and sign-off for emergency instructions before enabling automated TTS delivery. |
| **Licensing / Access** | Hugging Face Model Gating | OPEN | `ai4bharat/indic-conformer-600m-multilingual` and `ai4bharat/indic-parler-tts` require authorized Hugging Face organization access tokens to mirror into sovereign infrastructure. |
| **Hardware** | Sarvam-30B FP8 Inference Server | NOT_RUN | Requires dedicated Ada Lovelace or Hopper GPU instance (minimum 48 GB VRAM) to verify TTFT, TPOT, and memory utilization under load. |
