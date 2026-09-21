#!/usr/bin/env python3
"""Opt-in smoke probe for AI4Bharat IndicConformer-600M-Multi ASR.

This probe verifies local artifact layout, class interfaces, and execution
mechanics for ai4bharat/indic-conformer-600m-multilingual without downloading
weights or accessing the network.

When run without --model-dir, it immediately reports NOT_RUN and exits cleanly.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import sys
from pathlib import Path


EXPECTED_FILES = [
    "model_onnx.py",
    "config.json",
    "assets/preprocessor.ts",
    "assets/encoder.onnx",
    "assets/ctc_decoder.onnx",
    "assets/vocab.json",
    "assets/language_masks.json",
]

SUPPORTED_LANGUAGES = ["hi", "ml"]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="IndicConformer ASR Probe")
    parser.add_argument(
        "--model-dir",
        type=str,
        default=os.environ.get("STHIRA_ASR_ARTIFACT_DIR", ""),
        help="Path to local IndicConformer artifact directory.",
    )
    parser.add_argument(
        "--lang",
        type=str,
        default="hi",
        choices=["hi", "ml"],
        help="Language code to test (hi or ml).",
    )
    parser.add_argument(
        "--strategy",
        type=str,
        default="ctc",
        choices=["ctc", "rnnt"],
        help="Decoding strategy (ctc or rnnt).",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not args.model_dir:
        print("STATUS: NOT_RUN")
        print("REASON: Missing --model-dir or STHIRA_ASR_ARTIFACT_DIR.")
        print("NOTE: Real weights and authorized operational assets are pending external approval (O03).")
        return 0

    model_path = Path(args.model_dir).resolve()
    if not model_path.is_dir():
        print(f"STATUS: FAILED")
        print(f"REASON: Model directory does not exist: {model_path}")
        return 1

    # Check required artifact components
    missing = []
    for rel_path in EXPECTED_FILES:
        full_path = model_path / rel_path
        if not full_path.is_file():
            missing.append(rel_path)

    if missing:
        print(f"STATUS: FAILED_INCOMPLETE_ARTIFACT")
        print(f"Missing required artifact files: {missing}")
        return 1

    print(f"Artifact directory verified at: {model_path}")
    print(f"All required components present: {EXPECTED_FILES}")

    # Forbid network calls
    os.environ["HF_HUB_OFFLINE"] = "1"
    os.environ["TRANSFORMERS_OFFLINE"] = "1"

    try:
        import numpy as np
        import onnxruntime as ort
        import torch
    except ImportError as err:
        print(f"STATUS: FAILED_DEPENDENCY")
        print(f"Missing required Python dependencies: {err}")
        return 1

    # Load model_onnx.py dynamically
    module_path = model_path / "model_onnx.py"
    spec = importlib.util.spec_from_file_location("indic_asr_onnx", module_path)
    if spec is None or spec.loader is None:
        print("STATUS: FAILED_IMPORT")
        print(f"Could not load spec for {module_path}")
        return 1

    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)

    if not hasattr(mod, "IndicASRConfig") or not hasattr(mod, "IndicASRModel"):
        print("STATUS: FAILED_SCHEMA")
        print("model_onnx.py does not define IndicASRConfig and IndicASRModel")
        return 1

    print("Successfully imported IndicASRConfig and IndicASRModel from local artifact.")

    # Initialize model
    config = mod.IndicASRConfig(ts_folder=str(model_path))
    model = mod.IndicASRModel(config)

    # Test with 1 second of 16kHz synthetic mono silence/sine
    dummy_waveform = torch.zeros((1, 16000), dtype=torch.float32)
    print(f"Running inference with dummy waveform shape: {dummy_waveform.shape} in {args.lang} ({args.strategy})")

    with torch.no_grad():
        output = model(dummy_waveform, lang=args.lang, decoding=args.strategy)

    print(f"STATUS: VERIFIED")
    print(f"Return type: {type(output).__name__}")
    print(f"Transcript output: {output!r}")
    print("NOTE: Confidence is not returned by the model API (it returns str only).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
