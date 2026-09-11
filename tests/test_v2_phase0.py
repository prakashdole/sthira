"""Phase 0 tests for the isolated v2 runtime boundary."""

from fastapi.testclient import TestClient

from sthira.api.app import app
from sthira_v2.config import RuntimeProfile, V2Settings, validate_startup


def test_v2_status_is_demo_and_does_not_claim_live_data(monkeypatch):
    monkeypatch.delenv("STHIRA_PROFILE", raising=False)
    response = TestClient(app).get("/api/v2/status")
    assert response.status_code == 200
    payload = response.json()["data"]
    assert payload["product"] == "Sthira Citizen Emergency Guidance"
    assert payload["profile"] == "DEMO"
    assert payload["evidence_class"] == "SYNTHETIC_DEMO"
    assert payload["citizen_guidance_enabled"] is False
    assert response.json()["source_status"] == "NO_LIVE_GOVERNMENT_SOURCE_CONFIGURED"


def test_pilot_and_production_startup_fail_closed_without_requirements():
    for profile in (RuntimeProfile.PILOT, RuntimeProfile.PRODUCTION):
        settings = V2Settings(
            profile=profile,
            db_dsn=None,
            source_authorization=None,
            auth_issuer=None,
            operations_owner=None,
        )
        try:
            validate_startup(settings)
        except RuntimeError as exc:
            assert "startup blocked" in str(exc)
            assert "STHIRA_DB_DSN" in str(exc)
        else:
            raise AssertionError(f"{profile.value} unexpectedly passed startup validation")


def test_shadow_is_not_citizen_operational():
    settings = V2Settings(
        profile=RuntimeProfile.SHADOW,
        db_dsn=None,
        source_authorization=None,
        auth_issuer=None,
        operations_owner=None,
    )
    assert validate_startup(settings) == settings
    assert settings.citizen_guidance_enabled is False
