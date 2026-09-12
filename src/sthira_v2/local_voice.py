"""Local-only AI4Bharat speech runtimes; model weights never leave this Mac."""
from __future__ import annotations

import importlib.util
import io
import os
from pathlib import Path

from dotenv import load_dotenv

load_dotenv()


class LocalVoiceUnavailable(RuntimeError):
    pass


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
        with torch.inference_mode():
            return self.model(wav, language.split("-")[0], "ctc"), 1.0


class IndicParlerLocalRuntime:
    def __init__(self, model_dir: str | None = None) -> None:
        self.path = Path(model_dir or os.getenv("STHIRA_TTS_MODEL_DIR", ""))
        if not (self.path / "model.safetensors").is_file():
            raise LocalVoiceUnavailable("Indic Parler-TTS model directory is unavailable")

    def synthesize_wav(self, text: str) -> bytes:
        import soundfile as sf
        import torch
        from parler_tts import ParlerTTSForConditionalGeneration
        from transformers import AutoTokenizer
        device = "mps" if torch.backends.mps.is_available() else "cpu"
        model = ParlerTTSForConditionalGeneration.from_pretrained(self.path, local_files_only=True).to(device)
        prompt = AutoTokenizer.from_pretrained(self.path, local_files_only=True)(text, return_tensors="pt").to(device)
        description = AutoTokenizer.from_pretrained(model.config.text_encoder._name_or_path)("A clear, calm Indian speaker with a close high quality recording.", return_tensors="pt").to(device)
        with torch.inference_mode():
            audio = model.generate(input_ids=description.input_ids, attention_mask=description.attention_mask, prompt_input_ids=prompt.input_ids, prompt_attention_mask=prompt.attention_mask, do_sample=False, max_new_tokens=120)
        output = io.BytesIO(); sf.write(output, audio.cpu().numpy().squeeze(), model.config.sampling_rate, format="WAV")
        return output.getvalue()
