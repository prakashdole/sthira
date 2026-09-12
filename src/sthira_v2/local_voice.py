"""Local-only AI4Bharat speech runtimes; model weights never leave this Mac."""
from __future__ import annotations

import importlib.util
import io
import os
import hashlib
from pathlib import Path
from functools import lru_cache

from dotenv import load_dotenv

load_dotenv()


class LocalVoiceUnavailable(RuntimeError):
    pass


_ASR_LANGUAGE_KEYS = {
    "hi-IN": "hi",
    "ml-IN": "ml",
}


class IndicConformerLocalRuntime:
    def __init__(self, model_dir: str | None = None) -> None:
        self.path = Path(model_dir or os.getenv("STHIRA_ASR_MODEL_DIR", ""))
        if not (self.path / "model_onnx.py").is_file():
            raise LocalVoiceUnavailable("IndicConformer model directory is unavailable")
        spec = importlib.util.spec_from_file_location("sthira_indic_asr", self.path / "model_onnx.py")
        module = importlib.util.module_from_spec(spec)
        assert spec and spec.loader
        spec.loader.exec_module(module)
        self.model = module.IndicASRModel(module.IndicASRConfig(ts_folder=str(self.path)))

    def transcribe(self, audio: bytes, *, language: str) -> tuple[str, float]:
        import soundfile as sf
        import torch
        import torch.nn.functional as functional
        samples, sample_rate = sf.read(io.BytesIO(audio), dtype="float32", always_2d=True)
        wav = torch.from_numpy(samples).mean(dim=1).unsqueeze(0)
        if sample_rate != 16000:
            wav = functional.interpolate(wav.unsqueeze(0), size=round(wav.shape[-1] * 16000 / sample_rate), mode="linear", align_corners=False).squeeze(0)
        model_language = _ASR_LANGUAGE_KEYS.get(language)
        if model_language is None:
            raise ValueError("local IndicConformer is configured only for Hindi and Malayalam speech")
        with torch.inference_mode():
            return self.model(wav, model_language, "ctc"), 1.0


class IndicParlerLocalRuntime:
    def __init__(self, model_dir: str | None = None) -> None:
        self.path = Path(model_dir or os.getenv("STHIRA_TTS_MODEL_DIR", ""))
        if not (self.path / "model.safetensors").is_file():
            raise LocalVoiceUnavailable("Indic Parler-TTS model directory is unavailable")
        self._runtime: tuple[object, object, object, str] | None = None
        self._audio_cache: dict[str, bytes] = {}

    def _load(self) -> tuple[object, object, object, str]:
        if self._runtime is not None:
            return self._runtime
        import torch
        from parler_tts import ParlerTTSForConditionalGeneration
        from transformers import AutoTokenizer
        device = "mps" if torch.backends.mps.is_available() else "cpu"
        model = ParlerTTSForConditionalGeneration.from_pretrained(self.path, local_files_only=True).to(device)
        prompt_tokenizer = AutoTokenizer.from_pretrained(self.path, local_files_only=True)
        description_tokenizer = AutoTokenizer.from_pretrained(model.config.text_encoder._name_or_path, local_files_only=True)
        self._runtime = (model, prompt_tokenizer, description_tokenizer, device)
        return self._runtime

    def synthesize_wav(self, text: str) -> bytes:
        cache_key = hashlib.sha256(text.encode("utf-8")).hexdigest()
        existing = self._audio_cache.get(cache_key)
        if existing is not None:
            return existing
        import soundfile as sf
        import torch
        model, prompt_tokenizer, description_tokenizer, device = self._load()
        prompt = prompt_tokenizer(text, return_tensors="pt").to(device)
        description = description_tokenizer("A clear, calm Indian speaker with a close high quality recording.", return_tensors="pt").to(device)
        with torch.inference_mode():
            audio = model.generate(input_ids=description.input_ids, attention_mask=description.attention_mask, prompt_input_ids=prompt.input_ids, prompt_attention_mask=prompt.attention_mask, do_sample=False, max_new_tokens=120)
        output = io.BytesIO(); sf.write(output, audio.cpu().numpy().squeeze(), model.config.sampling_rate, format="WAV")
        wav = output.getvalue()
        # The citizen endpoint permits only the current approved instruction, so
        # a small process-local cache avoids repeat synthesis without retaining audio input.
        if len(self._audio_cache) >= 4:
            self._audio_cache.clear()
        self._audio_cache[cache_key] = wav
        return wav


@lru_cache(maxsize=1)
def asr_runtime() -> IndicConformerLocalRuntime:
    return IndicConformerLocalRuntime()


@lru_cache(maxsize=1)
def tts_runtime() -> IndicParlerLocalRuntime:
    return IndicParlerLocalRuntime()
