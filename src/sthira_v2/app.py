"""Small v2 API boundary mounted beside the legacy v1 application."""

import base64
import json
import os
from datetime import datetime, timezone
from pathlib import Path

from fastapi import APIRouter, HTTPException, Response, status
from pydantic import BaseModel, Field

from sthira_v2.allocation import AssignmentUnavailable, FacilityCapacity, InMemoryAllocationService
from sthira_v2.azure_openai import AzureOpenAIResponses
from sthira_v2.cap import AlertLifecycleService
from sthira_v2.chat import respond
from sthira_v2.config import V2Settings, profile_metadata
from sthira_v2.contracts import ArrivalResponse, ChatRequest, ChatResponse
from sthira_v2.local_voice import LocalVoiceUnavailable, asr_model_path, asr_runtime
from sthira_v2.package_service import OperationalPackageService
from sthira_v2.readiness import ReadinessState, build_readiness_report
from sthira_v2.speech_stt import INDIC_CONFORMER_MODEL, IndicConformerAdapter, STTArtifactGate, STTState

router = APIRouter(prefix="/api/v2", tags=["v2-runtime"])
_DEMO_SCENARIO = Path(__file__).resolve().parents[2] / "frontend" / "v2" / "src" / "scenario.json"
_DEMO_CAP = Path(__file__).resolve().parents[2] / "fixtures" / "synthetic_cap_alert.xml"

# The demo alert service reads time through a settable holder so tests inject a
# controlled "now" instead of relying on the wall clock. Production wiring uses
# the real clock; tests set an instant inside the fixture's validity window and
# separately assert expiry excludes the alert once the window passes. The
# fixture date is never moved and expiry is never disabled.
class _DemoClock:
    def __init__(self) -> None:
        self._override: datetime | None = None

    def set(self, instant: datetime | None) -> None:
        self._override = instant

    def __call__(self) -> datetime:
        return self._override or datetime.now(timezone.utc)


demo_clock = _DemoClock()


def _build_alert_service(clock) -> AlertLifecycleService:
    service = AlertLifecycleService(sender_allow_list={"synthetic.ndma.example"}, clock=clock)
    if service.ingest(_DEMO_CAP.read_bytes(), source_uri="fixture://synthetic-cap") is None:
        raise RuntimeError("bundled synthetic CAP fixture failed validation")
    return service


alert_service = _build_alert_service(demo_clock)


def reset_demo_alert_service() -> None:
    """Rebuild the demo alert service against the current demo clock.

    Tests use this to inject a controlled "now" before the fixture is ingested,
    so expiry is exercised deterministically. Expiry is terminal in a lifecycle
    service, so a service that has already expired the fixture cannot be
    rewound; it must be rebuilt. Production wiring never calls this.
    """

    global alert_service
    alert_service = _build_alert_service(demo_clock)

allocation_service = InMemoryAllocationService(("SZ-DEMO-01", "SZ-DEMO-02", "SZ-DEMO-03"))
for _facility in (
    FacilityCapacity("SZ-DEMO-01", 1, 120, 120),
    FacilityCapacity("SZ-DEMO-02", 1, 80, 80),
    FacilityCapacity("SZ-DEMO-03", 1, 160, 160),
):
    allocation_service.register_facility(_facility)

package_service = OperationalPackageService()
_demo_package = json.loads((Path(__file__).resolve().parents[2] / "fixtures" / "v2_wayanad_demo.json").read_text(encoding="utf-8"))
_demo_package_record = package_service.preview(_demo_package, jurisdiction="SYNTHETIC_DEMO")
package_service.publish(_demo_package_record.validation.package_id, operator_jurisdiction="SYNTHETIC_DEMO", authenticated=True)


class AssignmentRequest(BaseModel):
    assignment_id: str = Field(min_length=1, max_length=200)
    alert_id: str = Field(min_length=1, max_length=200)
    citizen_session_id: str = Field(min_length=16, max_length=200)
    idempotency_key: str = Field(min_length=8, max_length=200)
    party_size: int = Field(ge=1, le=50)


class ArrivalRequest(BaseModel):
    response: ArrivalResponse
    idempotency_key: str = Field(min_length=8, max_length=200)


class VoiceTranscriptionRequest(BaseModel):
    audio_base64: str = Field(min_length=1, max_length=13_500_000)
    language: str = Field(pattern=r"^(hi|ml)-IN$")
    duration_seconds: float = Field(gt=0, le=30)
    media_type: str = Field(pattern=r"^audio/(wav|ogg|webm)$")


def _alert_response(parsed) -> dict[str, object]:
    alert = parsed.alert
    return {
        "data": alert.model_dump(mode="json"),
        "raw_xml": parsed.raw_xml.decode("utf-8", errors="replace"),
        "source": {
            "id": alert.provenance.source_id,
            "authority": alert.provenance.authority_name,
            "uri": alert.provenance.source_uri,
            "evidence_class": alert.provenance.evidence_class.value,
        },
        "freshness": alert.fact_state.freshness.value,
        "degraded": True,
    }


def _assignment_response(allocation) -> dict[str, object]:
    return {
        "data": {
            "assignment_id": allocation.assignment_id,
            "alert_id": allocation.alert_id,
            "citizen_session_id": allocation.citizen_session_id,
            "safe_zone_id": allocation.facility_id,
            "party_size": allocation.party_size,
            "state": allocation.state.value,
        },
        "source_status": "SYNTHETIC_DEMO",
        "degraded": True,
    }


@router.get("/status")
def v2_status() -> dict[str, object]:
    """Return runtime metadata without claiming live data."""

    return {
        "data": profile_metadata(),
        "source_status": "NO_LIVE_GOVERNMENT_SOURCE_CONFIGURED",
    }


@router.get("/ai/provider/status", tags=["v2-ai"])
def ai_provider_status() -> dict[str, object]:
    """Expose configuration only; this endpoint never makes a paid model call."""

    provider = AzureOpenAIResponses()
    return {
        "data": {
            "provider": "AZURE_OPENAI",
            "configured_model_id": "gpt-4.1-mini",
            "configured": provider.configured,
            "purpose": "VOICE_MAP_INTERPRETATION_ONLY",
            "requires_validated_map_actions": True,
        },
        "source_status": "SYNTHETIC_DEMO",
        "degraded": not provider.configured,
    }


@router.get("/health/readiness")
def v2_readiness(response: Response) -> dict[str, object]:
    """Expose operational prerequisites without claiming source activation."""

    report = build_readiness_report()
    if report.state is ReadinessState.BLOCKED_EXTERNAL:
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
    return {"data": report.as_dict()}


@router.get("/readiness", tags=["v2-runtime"])
def v2_readiness_compat() -> dict[str, object]:
    """Expose the compact readiness shape used by the demo vertical slices."""

    report = build_readiness_report(V2Settings.from_environment())
    return {
        "status": "READY" if report.state is not ReadinessState.BLOCKED_EXTERNAL else "BLOCKED_EXTERNAL",
        "checks": [
            {"name": "database", "ready": report.database != "missing", "detail": report.database},
            {"name": "artifact_storage", "ready": report.artifact_storage != "missing", "detail": report.artifact_storage},
            {"name": "active_source_configuration", "ready": report.source_configuration == "configured", "detail": report.source_configuration},
            {"name": "migrations", "ready": report.migrations != "missing", "detail": report.migrations},
        ],
    }


@router.get("/alerts/active", tags=["v2-alerts"])
def active_alerts() -> dict[str, object]:
    items = [_alert_response(item) for item in alert_service.active()]
    return {"data": items, "source": "SYNTHETIC_DEMO", "freshness": "CURRENT" if items else "UNAVAILABLE", "degraded": True}


@router.get("/alerts/health", tags=["v2-alerts"])
def alert_feed_health() -> dict[str, object]:
    return {
        "data": {
            "source_id": "fixture:synthetic-cap",
            "status": "SYNTHETIC_DEMO",
            "active_count": len(alert_service.active()),
            "quarantine_count": len(alert_service.quarantine),
            "last_valid_artifact": "fixture://synthetic-cap",
        },
        "degraded": True,
    }


@router.get("/alerts/quarantine", tags=["v2-alerts"])
def alert_quarantine() -> dict[str, object]:
    return {"data": list(alert_service.quarantine), "source_status": "SYNTHETIC_DEMO", "degraded": True}


@router.get("/alerts/{identifier}", tags=["v2-alerts"])
def get_alert(identifier: str) -> dict[str, object]:
    parsed = alert_service.get(identifier)
    if parsed is None:
        raise HTTPException(status_code=404, detail="alert unavailable")
    return _alert_response(parsed)


@router.get("/demo/scenario", tags=["v2-demo"])
def demo_scenario() -> dict[str, object]:
    """Serve the versioned synthetic map package without implying official data."""

    try:
        scenario = json.loads(_DEMO_SCENARIO.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise HTTPException(status_code=503, detail="synthetic scenario unavailable") from exc
    if scenario.get("evidence_class") != "SYNTHETIC_DEMO" or not scenario.get("disclaimer"):
        raise HTTPException(status_code=503, detail="synthetic scenario failed provenance validation")
    return {"data": scenario, "source_status": "SYNTHETIC_DEMO", "freshness": "CURRENT", "degraded": True}


@router.get("/operational-packages/active", tags=["v2-operational-package"])
def active_operational_package() -> dict[str, object]:
    """Return the locally published synthetic package, never an external authority."""

    record = package_service.active(jurisdiction="SYNTHETIC_DEMO")
    if record is None or record.state != "PUBLISHED":
        raise HTTPException(status_code=503, detail="operational package unavailable")
    return {
        "data": record.package,
        "package_state": record.state,
        "package_version": record.validation.version,
        "checksum_sha256": record.checksum,
        "source_status": "SYNTHETIC_DEMO",
        "degraded": True,
    }


@router.post("/assignments", tags=["v2-assignment"])
def create_assignment(request: AssignmentRequest) -> dict[str, object]:
    try:
        allocation = allocation_service.assign(
            request.assignment_id,
            request.party_size,
            alert_id=request.alert_id,
            citizen_session_id=request.citizen_session_id,
            idempotency_key=request.idempotency_key,
        )
    except AssignmentUnavailable as exc:
        raise HTTPException(status_code=409, detail=str(exc)) from exc
    except ValueError as exc:
        raise HTTPException(status_code=409, detail="assignment idempotency conflict") from exc
    return _assignment_response(allocation)


@router.get("/assignments/{assignment_id}", tags=["v2-assignment"])
def get_assignment(assignment_id: str) -> dict[str, object]:
    allocation = allocation_service.get(assignment_id)
    if allocation is None:
        raise HTTPException(status_code=404, detail="assignment unavailable")
    return _assignment_response(allocation)


@router.post("/assignments/{assignment_id}/arrival-confirmations", tags=["v2-assignment"])
def confirm_arrival(assignment_id: str, request: ArrivalRequest) -> dict[str, object]:
    try:
        allocation = allocation_service.confirm_arrival(assignment_id, request.response, idempotency_key=request.idempotency_key)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="assignment unavailable") from exc
    return _assignment_response(allocation)


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
