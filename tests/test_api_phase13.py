"""
Integration tests for Phase 13 Source Access, Provider Health & Blocker Gating REST API endpoints.
Normative Reference: source-register.md, rules.md (RUL-076-RUL-083), trd.md (FR-076-FR-084).
"""

import hashlib
from datetime import datetime, timedelta, timezone
import pytest
from fastapi.testclient import TestClient

from punarvas.api.app import app


@pytest.fixture
def client():
    return TestClient(app)


def test_api_list_and_get_capabilities(client):
    """
    Test GET /api/v1/sources/capabilities and GET /api/v1/sources/capabilities/{source_id}
    """
    # 1. List all capabilities
    resp = client.get("/api/v1/sources/capabilities")
    assert resp.status_code == 200
    data = resp.json()["data"]
    assert len(data) == 54

    # 2. Filter by priority_class=AGENCY_BLOCKER
    resp_blockers = client.get("/api/v1/sources/capabilities?priority_class=AGENCY_BLOCKER")
    assert resp_blockers.status_code == 200
    blockers = resp_blockers.json()["data"]
    assert len(blockers) == 4
    blocker_ids = {b["source_id"] for b in blockers}
    assert blocker_ids == {"S45", "S46", "S47", "S50"}

    # 3. Get single capability
    resp_s01 = client.get("/api/v1/sources/capabilities/S01")
    assert resp_s01.status_code == 200
    s01 = resp_s01.json()["data"]
    assert s01["name"] == "KSDMA/GSI Landslide Susceptibility"
    assert s01["capability_type"] == "PRODUCT"
    assert len(s01["explicit_non_uses"]) >= 1

    # 4. Non-existent capability
    resp_404 = client.get("/api/v1/sources/capabilities/S99")
    assert resp_404.status_code == 404


def test_api_catalog_search_separation(client):
    """
    Test POST /api/v1/sources/catalog-search
    Affirms CATALOG_VISIBLE != APPROVED_FOR_USE (AT-31).
    """
    payload = {
        "source_id": "S10",
        "query_filter": "bbox=75.8,11.5,76.3,11.9",
        "actor_id": "api-analyst",
    }
    resp = client.post("/api/v1/sources/catalog-search", json=payload)
    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data["source_id"] == "S10"
    assert data["current_state"] == "CATALOG_VISIBLE"
    assert data["is_usable"] is False
    assert data["dependent_workflow_status"] == "HOLD"


def test_api_aoi_sample_validation_pass_and_quarantine(client):
    """
    Test POST /api/v1/sources/aoi-sample/validate
    Validates sample; verifies quarantine on checksum failure (AT-38).
    """
    checksum = hashlib.sha256(b"VALID_RAINFALL_NETCDF_SAMPLE").hexdigest()
    valid_payload = {
        "source_id": "S32",
        "sample_id": "SAMPLE-IMD-GRID-001",
        "license_type": "IMD Open Government Data License",
        "has_redistribution_and_offline_rights": True,
        "min_lat": 11.50,
        "max_lat": 11.85,
        "min_lon": 75.90,
        "max_lon": 76.30,
        "observation_timestamp": (datetime.now(timezone.utc) - timedelta(days=15)).isoformat(),
        "schema_format": "NetCDF",
        "crs": "EPSG:4326",
        "resolution_meters": 27000.0,
        "units": "mm/day",
        "nodata_value": "-999.0",
        "raw_payload_checksum": checksum,
        "claimed_checksum": checksum,
        "reviewer_id": "REV-METEOROLOGIST-01",
        "reproducibility_notes": "Retrieved from IMD Pune gridded archive.",
        "cost_usd": 0.0,
    }
    resp = client.post("/api/v1/sources/aoi-sample/validate", json=valid_payload)
    assert resp.status_code == 200
    res_data = resp.json()["data"]
    assert res_data["passed"] is True
    assert res_data["status"] == "APPROVED_FOR_USE"

    # Corrupt checksum test
    corrupt_payload = dict(valid_payload)
    corrupt_payload["sample_id"] = "SAMPLE-IMD-CORRUPT"
    corrupt_payload["claimed_checksum"] = "bad_checksum_12345"
    resp_corrupt = client.post("/api/v1/sources/aoi-sample/validate", json=corrupt_payload)
    assert resp_corrupt.status_code == 200
    corrupt_data = resp_corrupt.json()["data"]
    assert corrupt_data["passed"] is False
    assert corrupt_data["status"] == "QUARANTINED"
    assert len(corrupt_data["quarantine_reasons"]) >= 1


def test_api_mirror_groups_and_reconciliation(client):
    """
    Test GET /api/v1/sources/mirror-groups and POST /api/v1/sources/mirror-groups/reconcile (AT-32)
    """
    resp_groups = client.get("/api/v1/sources/mirror-groups")
    assert resp_groups.status_code == 200
    groups = resp_groups.json()["data"]
    assert len(groups) >= 5

    # Reconcile building footprints
    obs_id = "FOOTPRINT-WAYANAD-CHOORALMALA-001"
    reconcile_payload = {
        "group_id": "MIRROR_BUILDING_FOOTPRINTS",
        "observations": [
            {"source_id": "S22", "footprint_id": obs_id, "provider": "GoogleOpenBuildings", "area_sqm": 84.5},
            {"source_id": "S23", "footprint_id": obs_id, "provider": "MicrosoftML", "area_sqm": 82.1},
            {"source_id": "S41", "footprint_id": obs_id, "provider": "OpenStreetMap", "area_sqm": 85.0},
        ],
    }
    resp_rec = client.post("/api/v1/sources/mirror-groups/reconcile", json=reconcile_payload)
    assert resp_rec.status_code == 200
    rec_data = resp_rec.json()["data"]
    assert rec_data["total_input_count"] == 3
    assert rec_data["reconciled_count"] == 1
    assert rec_data["duplicate_count"] == 2
    assert rec_data["is_independent_corroboration_rejected"] is True


def test_api_geography_and_health(client):
    """
    Test GET /api/v1/sources/check-governance and GET /api/v1/sources/health (AT-33, AT-34)
    """
    # 1. C-FLOOD on Wayanad -> Unsupported
    resp_geo = client.get("/api/v1/sources/check-governance?source_id=S07&target_geography=Wayanad")
    assert resp_geo.status_code == 200
    geo_data = resp_geo.json()["data"]
    assert geo_data["can_link"] is False
    assert geo_data["status"] == "UNSUPPORTED_GEOGRAPHY"

    # 2. Provider Health with secret redaction
    resp_health = client.get("/api/v1/sources/health")
    assert resp_health.status_code == 200
    health_list = resp_health.json()["data"]
    assert len(health_list) >= 6
    for h in health_list:
        assert h["secrets_redacted"] is True
        assert h["redacted_token_preview"] == "***REDACTED***"


def test_api_dependency_blockers_and_basemap(client):
    """
    Test POST /api/v1/sources/blockers/evaluate and basemap endpoints (AT-35, AT-37)
    """
    # 1. Blocker evaluation with missing data
    incomplete_eval = {
        "site_id": "SITE-ELSTONE-01",
        "allocation_action": "LIVE_SITE_APPROVAL",
        "evidence_records": {
            "S45": {"status": "VERIFIED", "summary": "Tahsildar clear title verified"},
        },
    }
    resp_block = client.post("/api/v1/sources/blockers/evaluate", json=incomplete_eval)
    assert resp_block.status_code == 200
    block_data = resp_block.json()["data"]
    assert block_data["can_proceed"] is False
    assert block_data["overall_status"] == "BLOCKED"
    assert len(block_data["blocking_reasons"]) >= 4

    # 2. Complete S45-S50 evidence
    complete_eval = {
        "site_id": "SITE-ELSTONE-01",
        "allocation_action": "LIVE_SITE_APPROVAL",
        "evidence_records": {
            "S45": {"status": "VERIFIED", "summary": "Resurvey verified"},
            "S46": {"status": "VERIFIED", "sustainable_yield_lpcd": 70, "potability_certified": True},
            "S47": {"status": "VERIFIED", "summary": "FRA Gram Sabha resolution #12/2024"},
            "S48": {"status": "VERIFIED", "consent_percentage": 100},
            "S49": {"status": "VERIFIED", "factor_of_safety": 1.42},
            "S50": {"status": "VERIFIED", "administrative_sanction_number": "GO-452-2024-DMD"},
        },
    }
    resp_pass = client.post("/api/v1/sources/blockers/evaluate", json=complete_eval)
    assert resp_pass.status_code == 200
    pass_data = resp_pass.json()["data"]
    assert pass_data["can_proceed"] is True
    assert pass_data["overall_status"] == "PASS"

    # 3. Basemap config and simulation
    resp_bm = client.get("/api/v1/sources/basemaps")
    assert resp_bm.status_code == 200
    bm_data = resp_bm.json()["data"]
    assert bm_data["prohibit_osm_tile_bulk_download"] is True

    resp_fail = client.post("/api/v1/sources/basemaps/simulate-failure", json={"provider_id": "S53"})
    assert resp_fail.status_code == 200
    fail_data = resp_fail.json()["data"]
    assert fail_data["basemap_healthy"] is False
    assert fail_data["fallback_mode"] == "NON_MAP_TABULAR_VECTOR"
