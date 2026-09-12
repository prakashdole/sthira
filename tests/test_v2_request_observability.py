from fastapi.testclient import TestClient

from sthira.api.app import app


def test_public_v2_response_has_safe_correlation_id_and_preserves_valid_input():
    client = TestClient(app)
    generated = client.get("/api/v2/status")
    assert generated.headers["x-request-id"].startswith("req-")
    supplied = client.get("/api/v2/status", headers={"X-Request-ID": "demo-request-001"})
    assert supplied.headers["x-request-id"] == "demo-request-001"
    rejected = client.get("/api/v2/status", headers={"X-Request-ID": "../../secret"})
    assert rejected.headers["x-request-id"].startswith("req-")
