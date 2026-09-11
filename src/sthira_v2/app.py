"""Small v2 API boundary mounted beside the legacy v1 application."""

from fastapi import APIRouter, Response, status

from sthira_v2.config import profile_metadata
from sthira_v2.readiness import ReadinessState, build_readiness_report

router = APIRouter(prefix="/api/v2", tags=["v2-runtime"])


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
