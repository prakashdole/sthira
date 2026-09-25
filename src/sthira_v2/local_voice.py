"""Lazy local runtime for AI4Bharat IndicConformer-600M."""

from __future__ import annotations

import importlib.util
import io
import os
from functools import lru_cache
from pathlib import Path


class LocalVoiceUnavailable(RuntimeError):
    pass


_ASR_LANGUAGE_KEYS = {"hi-IN": "hi", "ml-IN": "ml"}
_DEFAULT_ASR_MODEL_DIR = Path(__file__).resolve().parents[2] / "models" / "indic-conformer-600m-multilingual"


def asr_model_path() -> Path:
    configured = os.getenv("STHIRA_ASR_MODEL_DIR")
    return Path(configured) if configured else _DEFAULT_ASR_MODEL_DIR


class IndicConformerLocalRuntime:
    def __init__(self, model_dir: str | None = None) -> None:
        self.path = Path(model_dir) if model_dir else asr_model_path()
        model_module = self.path / "model_onnx.py"
        if not model_module.is_file():
            raise LocalVoiceUnavailable("IndicConformer model directory is unavailable")
        spec = importlib.util.spec_from_file_location("sthira_indic_asr", model_module)
        if spec is None or spec.loader is None:
            raise LocalVoiceUnavailable("IndicConformer runtime could not be loaded")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        self.model = module.IndicASRModel(module.IndicASRConfig(ts_folder=str(self.path)))

    def transcribe(self, audio: bytes, *, language: str) -> tuple[str, float]:
        import soundfile as sf
        import torch
        import torch.nn.functional as functional

        model_language = _ASR_LANGUAGE_KEYS.get(language)
        if model_language is None:
            raise ValueError("IndicConformer is configured for Hindi and Malayalam speech")
        samples, sample_rate = sf.read(io.BytesIO(audio), dtype="float32", always_2d=True)
        wav = torch.from_numpy(samples).mean(dim=1).unsqueeze(0)
        if sample_rate != 16000:
            output_size = round(wav.shape[-1] * 16000 / sample_rate)
            wav = functional.interpolate(wav.unsqueeze(0), size=output_size, mode="linear", align_corners=False).squeeze(0)
        with torch.inference_mode():
            return self.model(wav, model_language, "ctc"), 1.0


@lru_cache(maxsize=1)
def asr_runtime() -> IndicConformerLocalRuntime:
    return IndicConformerLocalRuntime()
