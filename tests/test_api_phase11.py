"""
Integration tests for Phase 11 REST APIs (Government Dossiers, Manifests, Spatial Exports & Transparency).
Normative Reference: plan.md (#11), trd.md (§3.9, FR-053-FR-058, NFR-019-NFR-022, AT-12, AT-13, AT-14, AT-21, AT-23),
rules.md (RUL-052-RUL-060, RUL-075), DEC-012, DEC-013, DEC-042.
"""

import json
import pytest
from tests.authutil import authed_client


@pytest.fixture
def client():
    return authed_client()


def test_api_site_dossier_generation(client):
    """Test POST /api/v1/reporting/dossiers/site and GET /dossiers/site/{site_id}."""
    payload = {
        "site_id": "SITE-TEST-API-01",
        "site_name": "API Test Resettlement Site",
        "district": "Wayanad",
        "taluk": "Vythiri",
        "village": "Meppadi",
        "gross_area_cents": 500.0,
        "usable_area_cents": 420.0,
        "dwelling_capacity": 75,
        "water_source_description": "Perennial gravity spring connection",
        "lean_season_yield_lpcd": 82.0,
        "hazard_buffer_distance_m": 400.0,
        "slope_mean_deg": 12.0,
        "road_access_width_m": 5.0,
        "evidence_links": [
            {
                "field_name": "hazard_buffer_distance_m",
                "statement": "Located 400m outside active debris flow perimeter",
                "source_id": "S06_KSDMA_RUNOUT",
                "evidence_hash": "hash_val_400m",
                "is_verified": True,
            }
        ],
        "unresolved_conditions": [],
        "generating_user_id": "town_planner_api",
        "approval_ref": "ORD-DDMA-2025-09",
    }

    # 1. POST creation
    res = client.post("/api/v1/reporting/dossiers/site", json=payload)
    assert res.status_code == 200
    json_data = res.json()["data"]
    assert json_data["site_id"] == "SITE-TEST-API-01"
    assert json_data["dwelling_capacity"] == 75
    assert len(json_data["sha256_checksum"]) == 64

    # 2. GET retrieval
    res_get = client.get("/api/v1/reporting/dossiers/site/SITE-TEST-API-01")
    assert res_get.status_code == 200
    assert res_get.json()["data"]["site_id"] == "SITE-TEST-API-01"


def test_api_beneficiary_pack_generation(client):
    """Test POST /api/v1/reporting/dossiers/beneficiary and GET /dossiers/beneficiary/{household_id}."""
    payload = {
        "household_id": "HH-TEST-API-99",
        "head_of_household": "Devaki Amma",
        "member_count": 3,
        "vulnerability_score": 92.0,
        "disability_or_special_needs": True,
        "tenure_category": "OWNER",
        "relocation_necessity_review_id": "REV-NEC-99",
        "preferred_pathway": "TOWNSHIP",
        "assigned_site_id": "SITE-TEST-API-01",
        "eligible_schemes": ["PUNARJANI_LAND_GRANT", "LIFE_MISSION_HOUSING"],
        "evidence_links": [
            {
                "field_name": "vulnerability_score",
                "statement": "Single elderly woman with special medical accessibility needs",
                "source_id": "S49_FIELD_SURVEY",
                "evidence_hash": "hash_survey_hh_99",
                "is_verified": True,
            }
        ],
        "consent_token_ref": "CONSENT-TKN-99",
        "unresolved_conditions": [],
        "generating_user_id": "welfare_officer",
    }

    res = client.post("/api/v1/reporting/dossiers/beneficiary", json=payload)
    assert res.status_code == 200
    data = res.json()["data"]
    assert data["household_id"] == "HH-TEST-API-99"
    assert data["vulnerability_score"] == 92.0

    res_get = client.get("/api/v1/reporting/dossiers/beneficiary/HH-TEST-API-99")
    assert res_get.status_code == 200
    assert res_get.json()["data"]["household_id"] == "HH-TEST-API-99"


def test_api_field_checklist_and_decision_summary(client):
    """Test checklist and decision summary endpoints."""
    # 1. Checklist
    chk_payload = {
        "target_type": "SITE",
        "target_id": "SITE-ELSTONE-01",
        "items": [
            {
                "item_id": "ITEM-ROAD-01",
                "description": "Confirm access road minimum width >= 3.66m",
                "mandatory": True,
                "verification_method": "DGPS_WHEEL",
                "status": "PASS",
                "officer_notes": "Measured 4.1m functional width",
            }
        ],
        "required_equipment": ["Trimble DGPS", "Laser Measure"],
        "safety_precautions": ["High-visibility vests required"],
        "generating_user_id": "inspector_01",
    }
    res_chk = client.post("/api/v1/reporting/dossiers/checklist", json=chk_payload)
    assert res_chk.status_code == 200
    assert res_chk.json()["data"]["target_id"] == "SITE-ELSTONE-01"

    # 2. Decision Summary Dossier
    dec_payload = {
        "decision_id": "DEC-API-SUMMARY-01",
        "entity_type": "ALLOCATION_SCENARIO",
        "entity_id": "SCEN-API-01",
        "policy_version": "POL-WYD-2024.1",
        "source_checksums": {"S01": "hash_s01", "S06": "hash_s06"},
        "evidence_chain_hash": "chain_hash_12345",
        "generating_user_id": "appellate_clerk",
        "solver_seed": 42,
        "solver_tolerances": {"mip_gap": 0.01, "time_limit_sec": 60.0},
        "approval_order_id": "ORD-GOV-2025-001",
        "statutory_gazette_id": "GAZ-KL-2025-01",
        "objection_token_refs": ["RCPT-Sthira-OBJ-010"],
    }
    res_dec = client.post("/api/v1/reporting/dossiers/decision-summary", json=dec_payload)
    assert res_dec.status_code == 200
    assert res_dec.json()["data"]["decision_id"] == "DEC-API-SUMMARY-01"


def test_api_spatial_and_tabular_exports(client):
    """Test GeoJSON and CSV machine-readable export endpoints."""
    # 1. GeoJSON
    geo_payload = {
        "export_id": "API-GEO-01",
        "features": [
            {
                "type": "Feature",
                "geometry": {"type": "Point", "coordinates": [76.12, 11.52]},
                "properties": {"site_name": "Elstone Estate", "dwellings": 120},
            }
        ],
        "generating_user_id": "gis_officer",
        "classification": "RESTRICTED_OFFICIAL",
    }
    res_geo = client.post("/api/v1/reporting/export/geojson", json=geo_payload)
    assert res_geo.status_code == 200
    geo_data = res_geo.json()["data"]
    assert geo_data["geojson"]["type"] == "FeatureCollection"
    assert geo_data["manifest"]["export_type"] == "SPATIAL_GEOJSON"

    # 2. CSV
    csv_payload = {
        "export_id": "API-CSV-01",
        "headers": ["Case_ID", "Household_ID", "Status"],
        "rows": [["CASE-01", "HH-01", "APPROVED"], ["CASE-02", "HH-02", "IN_PROGRESS"]],
        "generating_user_id": "clerk_01",
        "classification": "RESTRICTED_OFFICIAL",
    }
    res_csv = client.post("/api/v1/reporting/export/csv", json=csv_payload)
    assert res_csv.status_code == 200
    csv_data = res_csv.json()["data"]
    assert "Case_ID,Household_ID,Status" in csv_data["csv_content"]
    assert csv_data["manifest"]["export_type"] == "TABULAR_CSV"


def test_api_manifest_verification_and_transparency_projection(client):
    """Test export manifest verification and k-anonymity public projection."""
    # 1. Public Projection with suppression (< 5) and differencing check
    proj_payload_r1 = {
        "projection_id": "PROJ-API-01",
        "district": "Wayanad",
        "round_number": 1,
        "subregion_counts": {
            "Meppadi_Ward_A": 30,
            "Meppadi_Ward_B": 4,  # < 5: Must be suppressed!
            "Vellarimala_Ward_C": 15,
        },
        "k_threshold": 5,
    }
    res_p1 = client.post("/api/v1/reporting/public-transparency-projection", json=proj_payload_r1)
    assert res_p1.status_code == 200
    data_p1 = res_p1.json()["data"]
    assert data_p1["cell_suppression_applied"] is True
    assert data_p1["suppressed_cell_count"] == 1
    assert data_p1["differencing_risk_detected"] is False

    # Round 2: delta of 1 triggers differencing warning
    proj_payload_r2 = {
        "projection_id": "PROJ-API-02",
        "district": "Wayanad",
        "round_number": 2,
        "subregion_counts": {
            "Meppadi_Ward_A": 31,  # delta of 1
            "Meppadi_Ward_B": 4,
            "Vellarimala_Ward_C": 15,
        },
        "k_threshold": 5,
    }
    res_p2 = client.post("/api/v1/reporting/public-transparency-projection", json=proj_payload_r2)
    assert res_p2.status_code == 200
    data_p2 = res_p2.json()["data"]
    assert data_p2["differencing_risk_detected"] is True

    # 2. Manifest List and Retrieval
    res_list = client.get("/api/v1/reporting/manifests")
    assert res_list.status_code == 200
    assert len(res_list.json()["data"]) > 0

    manifest_id = res_list.json()["data"][0]["manifest_id"]
    res_man = client.get(f"/api/v1/reporting/manifests/{manifest_id}")
    assert res_man.status_code == 200
    assert res_man.json()["data"]["manifest_id"] == manifest_id

    # 3. Manifest Verification
    # Testing tamper verification API
    verify_payload = {
        "manifest_id": manifest_id,
        "payload_content": "some_random_payload_that_will_fail_checksum",
    }
    res_verify = client.post("/api/v1/reporting/manifest/verify", json=verify_payload)
    assert res_verify.status_code == 200
    assert res_verify.json()["data"]["verified"] is False
