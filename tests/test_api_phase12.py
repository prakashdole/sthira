"""
Integration Tests for Phase 12 Formula, Parameter & Statutory Compliance Control API Endpoints.
Normative Reference: trd.md (§3.12, FR-071–FR-075), rules.md (RUL-061–RUL-066), DEC-043.
"""

import pytest
from fastapi.testclient import TestClient
from tests.authutil import authed_client


@pytest.fixture
def client():
    return authed_client()


def test_api_list_and_get_formulas(client: TestClient):
    """Test GET /api/v1/compliance/formulas and GET by ID."""
    # List all formulas
    res = client.get("/api/v1/compliance/formulas")
    assert res.status_code == 200
    data = res.json()["data"]
    assert len(data) >= 8
    formula_ids = [f["formula_id"] for f in data]
    assert "E01" in formula_ids
    assert "E06" in formula_ids
    assert "E11" in formula_ids
    assert "E60" in formula_ids

    # Filter by classification
    res_core = client.get("/api/v1/compliance/formulas?classification=CORE")
    assert res_core.status_code == 200
    for f in res_core.json()["data"]:
        assert f["classification"] == "CORE"

    # Get single formula
    res_e01 = client.get("/api/v1/compliance/formulas/E01")
    assert res_e01.status_code == 200
    e01 = res_e01.json()["data"]
    assert e01["name"] == "Unit and Time Conversions"
    assert e01["sha256_hash"] != ""

    # Non-existent formula returns 404
    res_404 = client.get("/api/v1/compliance/formulas/E999")
    assert res_404.status_code == 404


def test_api_list_and_get_parameters(client: TestClient):
    """Test GET /api/v1/compliance/parameters and GET by ID."""
    res = client.get("/api/v1/compliance/parameters")
    assert res.status_code == 200
    data = res.json()["data"]
    assert len(data) >= 5
    param_ids = [p["parameter_id"] for p in data]
    assert "PAR-006" in param_ids

    res_p6 = client.get("/api/v1/compliance/parameters/PAR-006")
    assert res_p6.status_code == 200
    p6 = res_p6.json()["data"]
    assert p6["value"] == 55.0
    assert p6["unit"] == "LPCD"


def test_api_policy_activation_and_execution(client: TestClient):
    """Test POST /api/v1/compliance/activations and /execute endpoints."""
    # 1. Create policy activation with E01, E11
    res_act = client.post(
        "/api/v1/compliance/activations",
        json={
            "policy_id": "POL-TEST-API-01",
            "programme_id": "PROG-WYD",
            "activated_formula_ids": ["E01", "E11"],
            "authorized_by": "DDMA_API_TEST",
        },
    )
    assert res_act.status_code == 200
    act_id = res_act.json()["data"]["activation_id"]

    # 2. Execute E01 with activation
    res_exec = client.post(
        "/api/v1/compliance/execute",
        json={
            "formula_id": "E01",
            "inputs": {"mode": "HA_TO_SQM", "val": 2.5},
            "policy_activation_id": act_id,
            "reviewer_id": "SURVEYOR_01",
        },
    )
    assert res_exec.status_code == 200
    exec_data = res_exec.json()["data"]
    assert exec_data["status"] == "SUCCESS"
    assert exec_data["computed_value"] == 25000.0
    exec_id = exec_data["execution_id"]

    # 3. Verify replay endpoint
    res_replay = client.get(f"/api/v1/compliance/replay/{exec_id}")
    assert res_replay.status_code == 200
    replay_data = res_replay.json()["data"]
    assert replay_data["is_bit_for_bit_identical"] is True
    assert replay_data["reproduced_value"] == 25000.0


def test_api_deny_list_and_guards(client: TestClient):
    """Test that executing rejected formulas returns HTTP 400/403."""
    # 1. Executing E60 (Rejected Omega score) -> 400 Bad Request
    res_e60 = client.post(
        "/api/v1/compliance/execute",
        json={
            "formula_id": "E60",
            "inputs": {"H": 0.5, "E": 0.5, "V": 0.5, "C": 0.5},
        },
    )
    assert res_e60.status_code == 400
    assert "RUL-035 / RUL-061 explicitly prohibits" in res_e60.json()["detail"]

    # 2. Executing E37 (Specialist formula) -> 400 Bad Request
    res_e37 = client.post(
        "/api/v1/compliance/execute",
        json={
            "formula_id": "E37",
            "inputs": {"beta": 30.0},
        },
    )
    assert res_e37.status_code == 400
    assert "cannot execute natively" in res_e37.json()["detail"]


def test_api_statutory_compliance_posture(client: TestClient):
    """Test GET /api/v1/compliance/statutes and POST /api/v1/compliance/evaluate-posture."""
    # 1. List statutes
    res_stats = client.get("/api/v1/compliance/statutes")
    assert res_stats.status_code == 200
    stats = res_stats.json()["data"]
    assert len(stats) >= 5
    stat_ids = [s["statute_id"] for s in stats]
    assert "DMA-2005" in stat_ids
    assert "DPDP-2023" in stat_ids

    # 2. Evaluate posture with missing controls
    res_eval = client.post(
        "/api/v1/compliance/evaluate-posture",
        json={
            "programme_id": "PROG-WYD-AUDIT",
            "active_control_ids": [
                "CONTROL_DDMA_APPROVAL_MANDATORY",
                "CONTROL_NTP_CLOCK_SYNC",
            ],
        },
    )
    assert res_eval.status_code == 200
    eval_data = res_eval.json()["data"]
    assert eval_data["is_fully_compliant"] is False
    assert len(eval_data["open_blockers"]) > 0
