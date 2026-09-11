"""Small v2 API boundary mounted beside the legacy v1 application."""

from fastapi import APIRouter

from sthira_v2.config import profile_metadata

router = APIRouter(prefix="/api/v2", tags=["v2-runtime"])


@router.get("/status")
def v2_status() -> dict[str, object]:
    """Return runtime metadata without claiming live data."""

    return {
        "data": profile_metadata(),
        "source_status": "NO_LIVE_GOVERNMENT_SOURCE_CONFIGURED",
    }
