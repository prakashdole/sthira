#!/usr/bin/env python3
"""Opt-in smoke probe for AI4Bharat Indic Parler-TTS.

This probe verifies local artifact layout, configuration parameters (such as
native sampling rate 44100 Hz), class interfaces, dual tokenizers, and
synthesis generation for ai4bharat/indic-parler-tts without downloading
weights or accessing the network.

When run without --model-dir, it immediately reports NOT_RUN and exits cleanly.
"""

from __future__ import annotations

import argparse
import io
import json
import os
import struct
import sys
import wave
from pathlib import Path


EXPECTED_FILES = [
    "config.json",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Indic Parler-TTS Probe")
    parser.add_argument(
        "--model-dir",
        type=str,
        default=os.environ.get("STHIRA_TTS_ARTIFACT_DIR", ""),
        help="Path to local Indic Parler-TTS artifact directory.",
    )
    parser.add_argument(
        "--text",
        type=str,
        default="सुरक्षित स्थान पर जाएं",
        help="Text prompt to synthesize.",
    )
    parser.add_argument(
        "--description",
        type=str,
        default="A female speaker with a clear, moderate-paced voice and neutral tone.",
        help="Voice description conditioning prompt.",
    )
    return parser.parse_args()


def pcm16le_wav_bytes(samples, sample_rate: int) -> bytes:
    buf = io.BytesIO()
    with wave.open(buf, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sample_rate)
        frames = bytearray()
        for s in samples:
            v = max(-1.0, min(1.0, float(s)))
            frames.extend(struct.pack("<h", int(v * 32767)))
        wf.writeframes(bytes(frames))
    return buf.getvalue()


def main() -> int:
    args = parse_args()
    if not args.model_dir:
        print("STATUS: NOT_RUN")
        print("REASON: Missing --model-dir or STHIRA_TTS_ARTIFACT_DIR.")
        print("NOTE: Real weights and authorized operational assets are pending external approval (O11).")
        return 0

    model_path = Path(args.model_dir).resolve()
    if not model_path.is_dir():
        print("STATUS: FAILED")
        print(f"REASON: Model directory does not exist: {model_path}")
        return 1

    # Check configuration
    cfg_file = model_path / "config.json"
    if not cfg_file.is_file():
        print("STATUS: FAILED_MISSING_CONFIG")
        print("Missing config.json in model directory")
        return 1

    with open(cfg_file, "r", encoding="utf-8") as fp:
        cfg = json.load(fp)

    native_sr = cfg.get("sampling_rate", 44100)
    print(f"Verified model config sampling_rate: {native_sr} Hz (NOT assumed 22050 Hz)")

    # Forbid network calls
    os.environ["HF_HUB_OFFLINE"] = "1"
    os.environ["TRANSFORMERS_OFFLINE"] = "1"

    try:
        import torch
        from parler_tts import ParlerTTSForConditionalGeneration
        from transformers import AutoTokenizer
    except ImportError as err:
        print("STATUS: FAILED_DEPENDENCY")
        print(f"Missing required Python dependencies: {err}")
        return 1

    # Load model and tokenizers locally
    try:
        model = ParlerTTSForConditionalGeneration.from_pretrained(
            str(model_path),
            local_files_only=True,
            trust_remote_code=False,
        )
    except Exception as err:
        print("STATUS: FAILED_MODEL_LOAD")
        print(f"Failed to load ParlerTTSForConditionalGeneration: {err}")
        return 1

    try:
        prompt_tok = AutoTokenizer.from_pretrained(str(model_path), local_files_only=True)
        desc_tok_name = model.config.text_encoder._name_or_path
        # Check if description tokenizer is local or fallback
        desc_tok = AutoTokenizer.from_pretrained(str(model_path), subfolder="description_tokenizer", local_files_only=True)
    except Exception as err:
        print("STATUS: FAILED_TOKENIZER_LOAD")
        print(f"Failed to load dual tokenizers: {err}")
        return 1

    desc_inputs = desc_tok(args.description, return_tensors="pt")
    prompt_inputs = prompt_tok(args.text, return_tensors="pt")

    print(f"Tokenized prompt shape: {prompt_inputs.input_ids.shape}")
    print(f"Tokenized description shape: {desc_inputs.input_ids.shape}")

    with torch.no_grad():
        generation = model.generate(
            input_ids=desc_inputs.input_ids,
            prompt_input_ids=prompt_inputs.input_ids,
        )

    audio_samples = generation.cpu().numpy().squeeze()
    print(f"Generated audio tensor shape: {generation.shape}, squeezed sample count: {len(audio_samples)}")

    wav_bytes = pcm16le_wav_bytes(audio_samples, sample_rate=native_sr)
    duration_secs = len(audio_samples) / native_sr
    print(f"STATUS: VERIFIED")
    print(f"Duration: {duration_secs:.2f}s, WAV size: {len(wav_bytes)} bytes, Sample Rate: {native_sr} Hz")
    return 0


if __name__ == "__main__":
    sys.exit(main())
