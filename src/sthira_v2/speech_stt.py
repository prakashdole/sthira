"""Bounded IndicConformer seam; no model artifact is bundled or executed."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum
from typing import Protocol
from uuid import uuid4


INDIC_CONFORMER_MODEL = "ai4bharat/indic-conformer-600m-multilingual"
MAX_AUDIO_BYTES = 10_000_000
MAX_AUDIO_SECONDS = 30


class STTState(StrEnum):
    BLOCKED_EXTERNAL = "BLOCKED_EXTERNAL"
    READY = "READY"


@dataclass(frozen=True)
class STTArtifactGate:
    model_id: str
    revision: str | None
    checksum_sha256: str | None
    license_ref: str | None
    runtime: str | None
    hardware: str | None
    state: STTState
    languages: tuple[str, ...]


@dataclass(frozen=True)
class Transcript:
    request_id: str
    language: str
    text: str
    confidence: float
    raw_audio_retained: bool


class InferenceRuntime(Protocol):
    def transcribe(self, audio: bytes, *, language: str) -> tuple[str, float]: ...


class IndicConformerAdapter:
    def __init__(self, gate: STTArtifactGate, runtime: InferenceRuntime | None = None) -> None:
        if gate.model_id != INDIC_CONFORMER_MODEL:
            raise ValueError("unexpected IndicConformer model id")
        self.gate = gate
        self.runtime = runtime

    def transcribe(self, audio: bytes, *, language: str, duration_seconds: float, media_type: str) -> Transcript:
        if self.gate.state is not STTState.READY or self.runtime is None:
            raise RuntimeError("IndicConformer artifact gate is not ready")
        if language not in self.gate.languages:
            raise ValueError("language is not configured for speech")
        if media_type not in {"audio/wav", "audio/webm", "audio/ogg"}:
            raise ValueError("audio media type is unsupported")
        if not audio or len(audio) > MAX_AUDIO_BYTES or duration_seconds <= 0 or duration_seconds > MAX_AUDIO_SECONDS:
            raise ValueError("audio bounds are invalid")
        request_id = f"stt-{uuid4().hex}"
        try:
            text, confidence = self.runtime.transcribe(bytes(audio), language=language)
        finally:
            # The local bytes object is not returned, cached, or logged.
            audio = b""
        if not isinstance(text, str) or not 0 <= confidence <= 1:
            raise RuntimeError("speech runtime returned invalid output")
        return Transcript(request_id, language, text, confidence, raw_audio_retained=False)
