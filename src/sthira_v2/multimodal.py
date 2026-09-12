"""Fail-closed speech and accessibility seams for source-approved instructions."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum
import hashlib


class TTSState(StrEnum):
    BLOCKED_EXTERNAL = "BLOCKED_EXTERNAL"
    READY = "READY"
    UNSUPPORTED_LANGUAGE = "UNSUPPORTED_LANGUAGE"
    AUDIO_UNAVAILABLE = "AUDIO_UNAVAILABLE"


@dataclass(frozen=True)
class TTSArtifactGate:
    model_id: str | None
    revision: str | None
    license_ref: str | None
    runtime: str | None
    hardware: str | None
    state: TTSState
    supported_languages: tuple[str, ...]


@dataclass(frozen=True)
class SpeechCacheEntry:
    content_hash: str
    language: str
    voice_version: str
    instruction_version: str
    audio: bytes


class TTSAdapter:
    """Adapter has no default model and never synthesizes unapproved content."""

    def __init__(self, gate: TTSArtifactGate) -> None:
        self.gate = gate
        self._cache: dict[tuple[str, str, str, str], SpeechCacheEntry] = {}

    def synthesize(self, text: str, *, language: str, instruction_version: str) -> SpeechCacheEntry:
        if self.gate.state is not TTSState.READY:
            raise RuntimeError("TTS artifact gate is not ready")
        if language not in self.gate.supported_languages:
            raise ValueError("TTS language is not approved")
        if not text.strip() or len(text) > 10000:
            raise ValueError("instruction text is invalid")
        content_hash = hashlib.sha256(text.encode("utf-8")).hexdigest()
        key = (content_hash, language, self.gate.revision or "", instruction_version)
        existing = self._cache.get(key)
        if existing is not None:
            return existing
        raise RuntimeError("TTS synthesis implementation requires an approved runtime artifact")

    def cache_approved_audio(self, text: str, audio: bytes, *, language: str, instruction_version: str) -> SpeechCacheEntry:
        if language not in self.gate.supported_languages:
            raise ValueError("TTS language is not approved")
        if self.gate.state is not TTSState.READY:
            raise RuntimeError("TTS artifact gate is not ready for this language")
        if not audio:
            raise ValueError("audio is empty")
        content_hash = hashlib.sha256(text.encode("utf-8")).hexdigest()
        entry = SpeechCacheEntry(content_hash, language, self.gate.revision or "", instruction_version, bytes(audio))
        self._cache[(content_hash, language, entry.voice_version, instruction_version)] = entry
        return entry

    def purge_instruction(self, instruction_version: str) -> int:
        keys = [key for key in self._cache if key[3] == instruction_version]
        for key in keys:
            del self._cache[key]
        return len(keys)


@dataclass(frozen=True)
class ISLAssetStatus:
    state: str
    asset_id: str | None
    caption: str
    transcript: str
    evidence_class: str
    reason: str


def pending_isl_asset() -> ISLAssetStatus:
    return ISLAssetStatus(
        state="PENDING_APPROVAL",
        asset_id=None,
        caption="ISL media is not available in this synthetic demo.",
        transcript="Text instructions remain available as the accessible fallback.",
        evidence_class="SYNTHETIC_DEMO",
        reason="authority-approved media and Deaf/ISL review are still required",
    )


def validate_localization_catalog(catalog: dict[str, dict[str, str]], required_keys: set[str]) -> None:
    for language, values in catalog.items():
        missing = required_keys - values.keys()
        if missing:
            raise ValueError(f"{language} localization missing: {sorted(missing)}")
