# Real-instance inference check — 2026-09-25

Baseline `a225560` on `CLEAN`; ASR correction committed as `924177d`.
This was bounded real inference testing, not browser or production acceptance.

## Environment verified

Authorized instance `i-01d17e39266c292c2`, us-east-2, g7e.2xlarge.
SSH worked; GPU reported NVIDIA RTX PRO 6000 Blackwell Server Edition, 72,215 MiB used / 25,038 MiB free before probes. Existing `sthira-sarvam` container remained running throughout inference. Its loopback API advertised `sarvamai/sarvam-30b`, context length 51,200. This does not change the application's 4k cap.

The scheduled poweroff file recorded 2026-09-25 14:19:11.430060 UTC, consistent with the supplied guard. Tests used existing artifacts and libraries, offline model loading, no downloads or new instance. No security group, vLLM launch configuration or credentials were changed. No artifact-supplied Python model file was executed by these probes; ASR used the repository adapter's TorchScript/ONNX path, TTS used installed Parler/Transformers classes and local assets.

## Results

| Check | Observed result | Verdict |
| --- | --- | --- |
| Sarvam tiny strict schema | `{"ok":true}`, 6 completion tokens, 0.77 s, `finish_reason=stop`, no thinking text | PASS for this minimal request only |
| Sarvam actual action schema, original system text | Hindi zoom, injected invented route, and Hindi arrival each exhausted 256 tokens; incomplete JSON/whitespace. Times 2.43 / 1.11 / 1.11 s. | FAIL; no usable action output |
| Sarvam expanded prompt | Added existing top-level contract/action definitions and a compact-JSON instruction. Zoom returned complete JSON in 0.66 s / 143 completion tokens, but invented `speech_key=ZOOM_IN_INSTRUCTION` when no template keys were approved. | FAIL semantic acceptance |
| Independent Go validator | Captured expanded-prompt output passed structural validation; `EnforceScopedContext` rejected exactly `speech_key "ZOOM_IN_INSTRUCTION" not approved for language "hi-IN"`. Temporary probe first needed its missing jurisdiction corrected; the final assertion required the speech-key reason, not just any error. | PASS rejection boundary |
| ASR list vocabulary regression | Existing test expanded to mapping/list layouts. List case failed before fix; 21 adapter tests passed after fix. | PASS narrow correction |
| ASR actual artifact | Corrected adapter loaded/warmed in 3.45 s; CPU ONNX execution. 0.16 s silence → empty transcript; 1 s silence → `ह`. No vocabulary exception; confidence remained null. | Decode fix PASS; silence behavior FAIL |
| Parler real generation | CUDA generation while vLLM remained loaded: 96,768 finite samples, 44,100 Hz, 2.194 s audio, peak 0.5252. 21.25 s includes load/tokenizers/generation, not warm latency. | Waveform generation PASS; quality NOT_REVIEWED |
| Synthetic TTS→ASR sanity | Input `मुझे सुरक्षित स्थान दिखाइए।`; ASR returned `मुझे सुरक्ष स्थान दिखाइए` in 0.17 s. | Pipeline mechanics observed; word fidelity mismatch; not human-speech accuracy evidence |
| Browser full voice flow | No running local frontend/backend listeners; no supplied hosted demo URL. Local map validator still requires `1.0`, while Go emits `3.0`; real worker executable composition remains open. | BLOCKED, not tested |

The corrected adapter was copied only to `/home/ubuntu/sthira-live-review-20260925/speech_asr_adapter.py` for the probe. The original `/home/ubuntu/speech_asr_adapter.py` was preserved. This is not deployment of a running ASR service. Neither ASR GPU execution nor an approved TTS voice was demonstrated.

Generated synthetic audio was copied locally to `/tmp/sthira-live-tts-hindi.wav`, verified SHA-256 `6e4277892eb4cc065fdc160772f84770807736e3d87bdc5a4f261368cdc7cd24`. It is not committed; listen before making quality claims. Local temporary Sarvam response files contain only synthetic probes. Temporary Go validator probe was removed; no raw logs or weights added to Git.

## Next bounded work before another paid session

1. Preserve `924177d`; cover real silent input without bypassing ASR warm-up. The existing Go `SilenceDetector` is documented as stub-only: real-path silence handling is still needed. Exact-zero suppression alone would not prove noise/VAD robustness.
2. Assemble Sarvam's prompt with both system instructions and exact contract/action definitions. The short system block alone does not describe the full contract. Prompt-only expansion still hallucinated a speech key: verify context-constrained output and strict action variants, finish-reason handling and independent validation. Do not strip arbitrary output or accept unknown speech keys. Retain the 256-token failure cases as regressions; do not claim thinking suppression works universally from a single successful response.
3. Finish C04 real private worker entry points and C02 browser schema/MIME/session/freshness/audio fixes from `prompt.md` section 0.0. Reuse adapters; do not build an alternate demo bypass around validation. This work does not require leaving the GPU running.
4. Next session: restart the existing instance, rediscover its public IP, confirm worker health after restart, run one complete valid typed action through Go, then the real microphone→map→approved-caption/audio browser flow. Obtain a human recording and review in the demo language. Continue the 18-case checklist only once that flow passes.

No real route, source, policy, identity, language approval or production readiness gate was closed. Instance stop was requested after preserving local outputs, following the owner's cost instruction; confirm final EC2 state in the handoff. Stopping preserves the EBS volume, which continues incurring storage charges; it is not termination.
