"""Small v2 API boundary mounted beside the legacy v1 application."""

import base64
from fastapi import APIRouter, HTTPException, Response, status
from pydantic import BaseModel, Field

from sthira_v2.chat import respond
from sthira_v2.config import profile_metadata
from sthira_v2.contracts import ChatRequest, ChatResponse
from sthira_v2.local_voice import LocalVoiceUnavailable, asr_model_path, asr_runtime
from sthira_v2.readiness import ReadinessState, build_readiness_report
from sthira_v2.speech_stt import INDIC_CONFORMER_MODEL, IndicConformerAdapter, STTArtifactGate, STTState

router = APIRouter(prefix="/api/v2", tags=["v2-runtime"])


class VoiceTranscriptionRequest(BaseModel):
    audio_base64: str = Field(min_length=1, max_length=13_500_000)
    language: str = Field(pattern=r"^(hi|ml)-IN$")
    duration_seconds: float = Field(gt=0, le=30)
    media_type: str = Field(pattern=r"^audio/(wav|ogg|webm)$")


@router.get("/status")
def v2_status() -> dict[str, object]:
    """Return runtime metadata without claiming live data."""

    return {
        "data": profile_metadata(),
        "source_status": "NO_LIVE_GOVERNMENT_SOURCE_CONFIGURED",
    }


@router.get("/health/readiness")
def v2_readiness(response: Response) -> dict[str, object]:
    """Expose operational prerequisites without claiming source activation."""

    report = build_readiness_report()
    if report.state is ReadinessState.BLOCKED_EXTERNAL:
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
    return {"data": report.as_dict()}


@router.get("/voice/status", tags=["v2-voice"])
def voice_status() -> dict[str, object]:
    """Report the local 600M speech artifact without loading model weights."""

    model_path = asr_model_path()
    ready = (model_path / "model_onnx.py").is_file()
    return {
        "data": {
            "provider": "LOCAL_AI4BHARAT",
            "model_id": INDIC_CONFORMER_MODEL,
            "ready": ready,
            "local_only": True,
            "supported_languages": ["hi-IN", "ml-IN"],
        },
        "degraded": not ready,
    }


@router.post("/voice/transcriptions", tags=["v2-voice"])
def transcribe_voice(request: VoiceTranscriptionRequest) -> dict[str, object]:
    """Transcribe bounded audio locally without retaining the recording."""

    try:
        audio = base64.b64decode(request.audio_base64, validate=True)
        adapter = IndicConformerAdapter(
            STTArtifactGate(
                INDIC_CONFORMER_MODEL,
                "e9b71b369c048e2c6b634d4c131061c34e441179",
                None,
                "MIT",
                "TorchScript+ONNX",
                "local",
                STTState.READY,
                ("hi-IN", "ml-IN"),
            ),
            asr_runtime(),
        )
        transcript = adapter.transcribe(
            audio,
            language=request.language,
            duration_seconds=request.duration_seconds,
            media_type=request.media_type,
        )
    except (LocalVoiceUnavailable, ValueError) as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail="local speech recognition unavailable") from exc
    return {
        "data": {
            "request_id": transcript.request_id,
            "language": transcript.language,
            "text": transcript.text,
            "confidence": transcript.confidence,
            "raw_audio_retained": False,
        },
        "source_status": "SYNTHETIC_DEMO",
    }


@router.post("/guidance/chat", response_model=ChatResponse)
def v2_guidance_chat(request: ChatRequest) -> ChatResponse:
    """Answer a general conversation turn and return any safe map action."""

    return respond(request)
