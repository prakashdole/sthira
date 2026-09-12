"""Small v2 API boundary mounted beside the legacy v1 application."""

from fastapi import APIRouter, HTTPException, Response, status
from pydantic import BaseModel, Field
import json
import os
from pathlib import Path

from sthira_v2.config import V2Settings, profile_metadata, validate_startup
from sthira_v2.readiness import ReadinessState, build_readiness_report
from sthira_v2.cap import AlertLifecycleService
from sthira_v2.allocation import AssignmentUnavailable, FacilityCapacity, InMemoryAllocationService
from sthira_v2.contracts import ArrivalResponse
from sthira_v2.package_service import OperationalPackageService, PackageAuthorizationError

router = APIRouter(prefix="/api/v2", tags=["v2-runtime"])
_DEMO_SCENARIO = Path(__file__).resolve().parents[2] / "frontend" / "v2" / "src" / "scenario.json"
_DEMO_CAP = Path(__file__).resolve().parents[2] / "fixtures" / "synthetic_cap_alert.xml"

alert_service = AlertLifecycleService(sender_allow_list={"synthetic.ndma.example"})
_demo_cap = alert_service.ingest(_DEMO_CAP.read_bytes(), source_uri="fixture://synthetic-cap")
if _demo_cap is None:
    raise RuntimeError("bundled synthetic CAP fixture failed validation")
allocation_service = InMemoryAllocationService(("SZ-DEMO-01", "SZ-DEMO-02", "SZ-DEMO-03"))
for _facility in (
    FacilityCapacity("SZ-DEMO-01", 1, 120, 120),
    FacilityCapacity("SZ-DEMO-02", 1, 80, 80),
    FacilityCapacity("SZ-DEMO-03", 1, 160, 160),
):
    allocation_service.register_facility(_facility)

package_service = OperationalPackageService()
_demo_package = json.loads((Path(__file__).resolve().parents[2] / "fixtures" / "v2_wayanad_demo.json").read_text(encoding="utf-8"))
_demo_package_record = package_service.preview(_demo_package, jurisdiction="Wayanad")
package_service.publish(_demo_package_record.validation.package_id, operator_jurisdiction="Wayanad", authenticated=True)


class AssignmentRequest(BaseModel):
    assignment_id: str = Field(min_length=1, max_length=200)
    alert_id: str = Field(min_length=1, max_length=200)
    citizen_session_id: str = Field(min_length=16, max_length=200)
    idempotency_key: str = Field(min_length=8, max_length=200)
    party_size: int = Field(ge=1, le=50)


class ArrivalRequest(BaseModel):
    response: ArrivalResponse
    idempotency_key: str = Field(min_length=8, max_length=200)


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
    settings = V2Settings.from_environment()
    return {
        "data": {
            "provider": "AZURE_OPENAI",
            "configured_model_id": "gpt-4.1-mini",
            "configured": bool(os.getenv("AZURE_OPENAI_RESPONSES_URL") and os.getenv("AZURE_OPENAI_API_KEY")),
            "purpose": "VOICE_MAP_INTERPRETATION_ONLY",
            "requires_validated_map_actions": True,
        },
        "source_status": "SYNTHETIC_DEMO",
        "degraded": not settings.nemotron_enabled,
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
    return {"status": "READY" if report.state is not ReadinessState.BLOCKED_EXTERNAL else "BLOCKED_EXTERNAL", "checks": [
        {"name": "database", "ready": report.database != "missing", "detail": report.database},
        {"name": "artifact_storage", "ready": report.artifact_storage != "missing", "detail": report.artifact_storage},
        {"name": "active_source_configuration", "ready": report.source_configuration == "configured", "detail": report.source_configuration},
        {"name": "migrations", "ready": report.migrations != "missing", "detail": report.migrations},
    ]}


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
    record = package_service.active(jurisdiction="Wayanad")
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


@router.get("/alerts/{identifier}", tags=["v2-alerts"])
def get_alert(identifier: str) -> dict[str, object]:
    parsed = alert_service.get(identifier)
    if parsed is None:
        raise HTTPException(status_code=404, detail="alert unavailable")
    return _alert_response(parsed)


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
    return {"data": {"assignment_id": allocation.assignment_id, "alert_id": allocation.alert_id, "citizen_session_id": allocation.citizen_session_id, "safe_zone_id": allocation.facility_id, "party_size": allocation.party_size, "state": allocation.state.value}, "source_status": "SYNTHETIC_DEMO", "degraded": True}


@router.get("/assignments/{assignment_id}", tags=["v2-assignment"])
def get_assignment(assignment_id: str) -> dict[str, object]:
    allocation = allocation_service.get(assignment_id)
    if allocation is None:
        raise HTTPException(status_code=404, detail="assignment unavailable")
    return {"data": {"assignment_id": allocation.assignment_id, "alert_id": allocation.alert_id, "citizen_session_id": allocation.citizen_session_id, "safe_zone_id": allocation.facility_id, "party_size": allocation.party_size, "state": allocation.state.value}, "source_status": "SYNTHETIC_DEMO", "degraded": True}


@router.post("/assignments/{assignment_id}/arrival-confirmations", tags=["v2-assignment"])
def confirm_arrival(assignment_id: str, request: ArrivalRequest) -> dict[str, object]:
    try:
        allocation = allocation_service.confirm_arrival(assignment_id, request.response, idempotency_key=request.idempotency_key)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="assignment unavailable") from exc
    return {"data": {"assignment_id": allocation.assignment_id, "alert_id": allocation.alert_id, "citizen_session_id": allocation.citizen_session_id, "safe_zone_id": allocation.facility_id, "party_size": allocation.party_size, "state": allocation.state.value}, "source_status": "SYNTHETIC_DEMO", "degraded": True}
