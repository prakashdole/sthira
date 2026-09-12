from fastapi.testclient import TestClient

from sthira.api.app import app


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
    response = TestClient(app).get("/api/v2/alerts/active")
    assert response.status_code == 200
    body = response.json()
    assert body["source"] == "SYNTHETIC_DEMO"
    assert body["degraded"] is True
    assert body["data"][0]["source"]["evidence_class"] == "SYNTHETIC_DEMO"
    assert body["data"][0]["raw_xml"].startswith("<?xml")
