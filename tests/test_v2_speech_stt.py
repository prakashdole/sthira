import base64

import pytest
from fastapi.testclient import TestClient

from sthira.api.app import app
from sthira_v2.speech_stt import INDIC_CONFORMER_MODEL, IndicConformerAdapter, STTArtifactGate, STTState


def test_indicconformer_requires_external_artifact_gate():
    adapter = IndicConformerAdapter(
        STTArtifactGate(INDIC_CONFORMER_MODEL, None, None, None, None, None, STTState.BLOCKED_EXTERNAL, ())
    )
    with pytest.raises(RuntimeError, match="artifact gate"):
        adapter.transcribe(b"wav", language="ml-IN", duration_seconds=1, media_type="audio/wav")


def test_ready_runtime_is_bounded_and_does_not_retain_audio():
    class Runtime:
        def transcribe(self, audio: bytes, *, language: str):
            assert audio == b"wav"
            assert language == "ml-IN"
            return "സുരക്ഷിത വഴി കാണിക്കുക", 0.96

    adapter = IndicConformerAdapter(
        STTArtifactGate(INDIC_CONFORMER_MODEL, "revision", None, "MIT", "ONNX", "local", STTState.READY, ("ml-IN",)),
        Runtime(),
    )
    result = adapter.transcribe(b"wav", language="ml-IN", duration_seconds=1, media_type="audio/wav")

    assert result.text == "സുരക്ഷിത വഴി കാണിക്കുക"
    assert result.raw_audio_retained is False


def test_voice_transcription_endpoint_is_public_and_reports_missing_model(monkeypatch):
    monkeypatch.delenv("STHIRA_ASR_MODEL_DIR", raising=False)
    response = TestClient(app).post(
        "/api/v2/voice/transcriptions",
        json={
            "audio_base64": base64.b64encode(b"wav").decode(),
            "language": "ml-IN",
            "duration_seconds": 1,
            "media_type": "audio/wav",
        },
    )

    assert response.status_code == 422
    assert "model directory" in response.json()["detail"]
