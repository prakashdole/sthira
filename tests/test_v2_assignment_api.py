from fastapi.testclient import TestClient

from sthira.api.app import app


def test_assignment_and_arrival_api_is_idempotent_and_demo_labeled():
    client = TestClient(app)
    request = {"assignment_id": "api-a1", "alert_id": "DEMO-WYD-LANDSLIDE-001", "citizen_session_id": "synthetic-session-001", "idempotency_key": "assignment-key-1", "party_size": 2}
    first = client.post("/api/v2/assignments", json=request)
    repeated = client.post("/api/v2/assignments", json=request)

    assert first.status_code == 200
    assert repeated.json() == first.json()
    assert first.json()["source_status"] == "SYNTHETIC_DEMO"
    conflict = dict(request, party_size=3)
    assert client.post("/api/v2/assignments", json=conflict).status_code == 409

    no = client.post("/api/v2/assignments/api-a1/arrival-confirmations", json={"response": "NO", "idempotency_key": "arrival-key-1"})
    yes = client.post("/api/v2/assignments/api-a1/arrival-confirmations", json={"response": "YES", "idempotency_key": "arrival-key-2"})
    retry = client.post("/api/v2/assignments/api-a1/arrival-confirmations", json={"response": "YES", "idempotency_key": "arrival-key-2"})
    assert no.json()["data"]["state"] == "RESERVED"
    assert yes.json()["data"]["state"] == "ARRIVED"
    assert retry.json() == yes.json()
    fetched = client.get("/api/v2/assignments/api-a1")
    assert fetched.status_code == 200
    assert fetched.json()["data"]["assignment_id"] == "api-a1"
