from fastapi.testclient import TestClient

from sthira.api.app import app


def test_active_operational_package_is_complete_and_demo_labeled():
    response = TestClient(app).get("/api/v2/operational-packages/active")
    assert response.status_code == 200
    body = response.json()
    assert body["source_status"] == "SYNTHETIC_DEMO"
    assert body["package_state"] == "PUBLISHED"
    assert body["data"]["provenance"]["checksum_sha256"] == body["checksum_sha256"]
    assert body["data"]["allocation_policy"]["order"] == ["SZ-DEMO-01", "SZ-DEMO-02", "SZ-DEMO-03"]
