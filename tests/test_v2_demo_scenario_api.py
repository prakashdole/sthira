from datetime import datetime, timezone
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from sthira.api.app import app
from sthira_v2.app import demo_clock, reset_demo_alert_service
from sthira_v2.cap import AlertLifecycleService

# The bundled synthetic CAP fixture is valid 2026-09-12T04:00:00Z to
# 2026-09-13T04:00:00Z. Tests inject a controlled "now" inside that window
# rather than moving the fixture date or disabling expiry.
WITHIN_FIXTURE = datetime(2026, 9, 12, 12, tzinfo=timezone.utc)
AFTER_FIXTURE_EXPIRY = datetime(2026, 9, 13, 5, tzinfo=timezone.utc)
_DEMO_CAP = Path(__file__).resolve().parents[1] / "fixtures" / "synthetic_cap_alert.xml"


@pytest.fixture(autouse=True)
def _reset_demo_clock():
    demo_clock.set(None)
    yield
    demo_clock.set(None)
    reset_demo_alert_service()


def test_demo_scenario_api_is_explicitly_synthetic_and_versioned():
    response = TestClient(app).get("/api/v2/demo/scenario")
    assert response.status_code == 200
    body = response.json()
    assert body["source_status"] == "SYNTHETIC_DEMO"
    assert body["data"]["evidence_class"] == "SYNTHETIC_DEMO"
    assert body["data"]["scenario_date"] == "2026-09-12"
    assert body["data"]["expires_at"] == "2026-09-12T18:00:00+05:30"
    assert {zone["id"] for zone in body["data"]["safe_zones"]} == {"SZ-DEMO-01", "SZ-DEMO-02", "SZ-DEMO-03"}


def test_active_alert_api_exposes_synthetic_provenance_and_raw_artifact():
    # Rebuild the demo service against the controlled clock so the fixture is
    # ingested inside its validity window regardless of test ordering.
    demo_clock.set(WITHIN_FIXTURE)
    reset_demo_alert_service()
    response = TestClient(app).get("/api/v2/alerts/active")
    assert response.status_code == 200
    body = response.json()
    assert body["source"] == "SYNTHETIC_DEMO"
    assert body["degraded"] is True
    assert body["data"][0]["source"]["evidence_class"] == "SYNTHETIC_DEMO"
    assert body["data"][0]["raw_xml"].startswith("<?xml")


def test_active_alert_api_excludes_expired_alert():
    # Separate assertion: once the controlled clock passes the fixture expiry,
    # the alert is no longer active. Expiry is terminal in the shared service,
    # so this uses an isolated service rather than mutating the app singleton.
    # Expiry is exercised, never disabled.
    now = [WITHIN_FIXTURE]
    service = AlertLifecycleService(sender_allow_list={"synthetic.ndma.example"}, clock=lambda: now[0])
    assert service.ingest(_DEMO_CAP.read_bytes(), source_uri="fixture://synthetic-cap") is not None
    assert len(service.active()) == 1
    now[0] = AFTER_FIXTURE_EXPIRY
    assert service.active() == []
