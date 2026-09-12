import pytest

from sthira_v2.speech_stt import INDIC_CONFORMER_MODEL, IndicConformerAdapter, STTArtifactGate, STTState


def test_indicconformer_requires_exact_external_artifact_gate():
    adapter = IndicConformerAdapter(STTArtifactGate(INDIC_CONFORMER_MODEL, None, None, None, None, None, STTState.BLOCKED_EXTERNAL, ()))
    with pytest.raises(RuntimeError, match="artifact gate"):
        adapter.transcribe(b"wav", language="ml-IN", duration_seconds=1, media_type="audio/wav")


def test_ready_fixture_runtime_is_bounded_and_does_not_retain_audio():
    class Runtime:
        def transcribe(self, audio: bytes, *, language: str):
            assert audio == b"wav"
            return "show my location", 0.96

    adapter = IndicConformerAdapter(STTArtifactGate(INDIC_CONFORMER_MODEL, "demo-rev", "a" * 64, "demo-license", "demo-runtime", "demo-hardware", STTState.READY, ("en-IN", "ml-IN")), Runtime())
    result = adapter.transcribe(b"wav", language="en-IN", duration_seconds=1, media_type="audio/wav")
    assert result.text == "show my location"
    assert result.raw_audio_retained is False
    with pytest.raises(ValueError, match="media type"):
        adapter.transcribe(b"wav", language="en-IN", duration_seconds=1, media_type="text/plain")
    with pytest.raises(ValueError, match="bounds"):
        adapter.transcribe(b"wav", language="en-IN", duration_seconds=31, media_type="audio/wav")
