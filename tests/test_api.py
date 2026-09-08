"""
Tests for PUNARVAS-AI FastAPI REST Endpoints.
"""

from fastapi.testclient import TestClient
from punarvas.api.app import app

client = TestClient(app)


def test_api_health():
    response = client.get("/health")
    assert response.status_code == 200
    data = response.json()
    assert data["success"] is True
    assert data["data"]["status"] == "HEALTHY"
    assert data["data"]["audit_chain_valid"] is True
    assert data["advisory"]["is_advisory"] is True


def test_api_get_programme():
    response = client.get("/api/v1/programme/PRG-KL-WYD-2024")
    assert response.status_code == 200
    data = response.json()
    assert data["data"]["district"] == "Wayanad"
    assert data["data"]["title"] == "Wayanad Landslide Rehabilitation & Resettlement Programme"


def test_api_catalog_sources():
    response = client.get("/api/v1/catalog/sources")
    assert response.status_code == 200
    sources = response.json()["data"]
    assert len(sources) >= 10
    assert any(s["source_id"] == "S01" for s in sources)


def test_api_evaluate_exposure():
    payload = {
        "parcel_id": "PARCEL-KL-WYD-MEP-001",
        "coordinates": [76.135, 11.542],
        "local_slope_deg": 12.0,
    }
    response = client.post("/api/v1/hazard/evaluate-exposure", json=payload)
    assert response.status_code == 200
    res = response.json()["data"]
    assert res["parcel_id"] == "PARCEL-KL-WYD-MEP-001"
    assert res["requires_relocation_review"] is True


def test_api_evaluate_site_gates():
    payload = {
        "site_id": "SITE-ELSTONE-01",
        "hazard_susceptibility_level": "LOW",
        "in_debris_flow_runout": False,
        "title_clearance_status": "VERIFIED_CLEAR",
        "forest_clearance_required": False,
        "has_dry_season_yield_test": True,
        "lean_season_tested_lpcd": 85.0,
        "road_access_width_m": 12.0,
        "distance_to_hospital_km": 3.2,
        "distance_to_school_km": 1.5,
        "dwelling_capacity": 250,
    }
    response = client.post("/api/v1/site/evaluate-gates", json=payload)
    assert response.status_code == 200
    res = response.json()["data"]
    assert res["overall_gate_pass"] is True
    assert len(res["gate_results"]) == 4


def test_api_detect_discrepancies():
    response = client.post("/api/v1/land/detect-discrepancies/PARCEL-KL-WYD-MEP-003")
    assert response.status_code == 200
    tasks = response.json()["data"]
    assert len(tasks) >= 1
    assert any(t["discrepancy_type"] == "PAPER_VACANT_GROUND_OCCUPIED" for t in tasks)


def test_api_simulate_allocation():
    response = client.post("/api/v1/allocation/simulate-scenario")
    assert response.status_code == 200
    scen = response.json()["data"]
    assert scen["total_households"] == 4
    assert scen["assigned_count"] >= 3
    assert scen["is_simulation_only"] is True


def test_api_bilingual_dossier():
    response = client.get("/api/v1/reporting/dossier/HH-WYD-001")
    assert response.status_code == 200
    res = response.json()
    assert "html" in res
    assert "പുനർവാസ്" in res["html"]
    assert "checksum_sha256" in res
    assert len(res["checksum_sha256"]) == 64


def test_api_public_projection():
    response = client.get("/api/v1/reporting/public-projection")
    assert response.status_code == 200
    res = response.json()["data"]
    assert res["district"] == "Wayanad"
    assert res["data_classification"] == "PUBLIC_AGGREGATE"
    assert res["metrics"]["total_verified_eligible_households"] == 430


def test_api_audit_verify():
    response = client.get("/api/v1/audit/verify")
    assert response.status_code == 200
    res = response.json()["data"]
    assert res["chain_valid"] is True
    assert res["total_records"] > 0
