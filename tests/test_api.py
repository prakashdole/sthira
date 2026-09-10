"""
Tests for Sthira FastAPI REST Endpoints.
"""

from tests.authutil import authed_client

client = authed_client()


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


def test_api_agency_import_e_rekha():
    payload = {
        "batch_id": "API-BATCH-01",
        "records": [
            {
                "record_id": "API-REC-01",
                "survey_number": "12/1",
                "village": "Meppadi",
                "lsg_name": "Meppadi Grama Panchayat",
                "area_cents": 15.0,
                "measured_offset_m": 12.0,
                "classification": "REVENUE",
            }
        ],
    }
    response = client.post("/api/v1/agency-import/e-rekha", json=payload)
    assert response.status_code == 200
    results = response.json()["data"]
    assert len(results) == 1
    assert results[0]["survey_number"] == "12/1"


def test_api_capacity_reserve_and_conflict():
    # Configure initial limits
    from sthira.modules.allocation import capacity_reservation_ledger
    capacity_reservation_ledger.configure_capacities(
        site_dwellings={"SITE-API-01": 50},
        site_land_cents={"SITE-API-01": 350.0},
        programme_budget_inr=10000000.0,
        site_water_m3_day={"SITE-API-01": 25.0},
    )

    reserve_payload = {
        "scenario_id": "SCENARIO-API-1",
        "site_id": "SITE-API-01",
        "dwellings": 40,
        "land_cents": 280.0,
        "budget_inr": 8000000.0,
        "water_m3_day": 20.0,
    }
    res = client.post("/api/v1/capacity/reserve", json=reserve_payload)
    assert res.status_code == 200
    assert res.json()["data"]["dwellings_reserved"] == 40

    # Competing reservation exceeding remaining capacity (40 + 20 > 50)
    conflict_payload = {
        "scenario_id": "SCENARIO-API-2",
        "site_id": "SITE-API-01",
        "dwellings": 20,
        "land_cents": 140.0,
        "budget_inr": 4000000.0,
        "water_m3_day": 10.0,
    }
    conflict_res = client.post("/api/v1/capacity/reserve", json=conflict_payload)
    assert conflict_res.status_code == 409
    assert "Capacity reservation conflict" in conflict_res.json()["detail"]


def test_api_policy_sensitivity():
    payload = {
        "sites": [
            {
                "site_id": "SITE-SENS-1",
                "hazard_susceptibility_level": "LOW",
                "in_debris_flow_runout": False,
                "title_clearance_status": "VERIFIED_CLEAR",
                "forest_clearance_required": False,
                "lean_season_tested_lpcd": 75.0,
                "has_dry_season_yield_test": True,
                "road_access_width_m": 5.0,
                "distance_to_hospital_km": 3.0,
                "distance_to_school_km": 1.0,
                "dwelling_capacity": 80,
            },
            {
                "site_id": "SITE-SENS-2",
                "hazard_susceptibility_level": "LOW",
                "in_debris_flow_runout": False,
                "title_clearance_status": "VERIFIED_CLEAR",
                "forest_clearance_required": False,
                "lean_season_tested_lpcd": 70.0,
                "has_dry_season_yield_test": True,
                "road_access_width_m": 4.5,
                "distance_to_hospital_km": 10.0,
                "distance_to_school_km": 5.0,
                "dwelling_capacity": 150,
            },
        ],
        "perturbation_factor": 0.20,
    }
    response = client.post("/api/v1/policy/sensitivity", json=payload)
    assert response.status_code == 200
    res = response.json()["data"]
    assert len(res) > 0


def test_api_lsgd_dm_plan_annex():
    response = client.get("/api/v1/reporting/lsgd-plan-annex?lsg_name=Meppadi Grama Panchayat")
    assert response.status_code == 200
    annex = response.json()["data"]
    assert annex["lsg_name"] == "Meppadi Grama Panchayat"
    assert annex["section_d_statutory_approvals"]["gram_ward_sabha_resolution"] == "PENDING_GRAM_SABHA_APPROVAL"
    assert "ചേർക്കുക" not in annex["statutory_note_ml"]  # Ensures formal text
    assert len(annex["sha256_checksum"]) == 64


def test_api_evaluation_metrics():
    response = client.get("/api/v1/evaluation/metrics")
    assert response.status_code == 200
    metrics = response.json()["data"]
    assert "median_time_reduction_pct" in metrics
    assert "sha256_checksum" in metrics


def test_api_scaling_districts():
    response = client.get("/api/v1/scaling/districts")
    assert response.status_code == 200
    districts = response.json()["data"]
    assert len(districts) >= 3
    district_ids = [d["district_id"] for d in districts]
    assert "KL-WYD" in district_ids
    assert "KL-IDU" in district_ids
    assert "KL-ALP" in district_ids


def test_api_scaling_statewide_dashboard():
    response = client.get("/api/v1/scaling/statewide-dashboard")
    assert response.status_code == 200
    dash = response.json()["data"]
    assert dash["state"] == "Kerala"
    assert dash["total_districts_onboarded"] >= 3
    assert dash["data_classification"] == "STATEWIDE_PUBLIC_AGGREGATE"
    assert dash["total_eligible_households"] > 0


def test_api_scaling_district_fixtures():
    response = client.get("/api/v1/scaling/districts/KL-IDU/fixtures")
    assert response.status_code == 200
    fixtures = response.json()["data"]
    assert fixtures["district_name"] == "Idukki"
    assert fixtures["sample_site"]["village"] == "KDH Village (Munnar)"
    assert fixtures["sample_household"]["head_of_household"] == "Murugan S"


def test_api_adaptation_tenants():
    response = client.get("/api/v1/adaptation/tenants")
    assert response.status_code == 200
    tenants = response.json()["data"]
    state_codes = [t["state_code"] for t in tenants]
    assert "KL" in state_codes
    assert "UK" in state_codes


def test_api_adaptation_dossier_header_leakage_checks():
    # Kerala Header
    res_kl = client.get("/api/v1/adaptation/dossier-header/KL?language=ml")
    assert res_kl.status_code == 200
    hdr_kl = res_kl.json()["data"]
    assert "KSDMA" in hdr_kl["statutory_authority"]
    assert "USDMA" not in hdr_kl["statutory_authority"]
    assert "Devbhoomi" not in hdr_kl["land_tenure_system"]
    assert hdr_kl["crs"] == "EPSG:32643"

    # Uttarakhand Header
    res_uk = client.get("/api/v1/adaptation/dossier-header/UK?language=hi")
    assert res_uk.status_code == 200
    hdr_uk = res_uk.json()["data"]
    assert "USDMA" in hdr_uk["statutory_authority"]
    assert "KSDMA" not in hdr_uk["statutory_authority"]
    assert "e-Rekha" not in hdr_uk["land_tenure_system"]
    assert hdr_uk["crs"] == "EPSG:32644"
    assert "पुनर्वास" in hdr_uk["glossary"]["title"]


def test_api_adaptation_state_fixtures():
    response = client.get("/api/v1/adaptation/tenants/UK/fixtures")
    assert response.status_code == 200
    fixtures = response.json()["data"]
    assert fixtures["state_code"] == "UK"
    assert "subsidence" in fixtures["hazard_context"].lower()
    assert fixtures["sample_site"]["district"] == "Chamoli"
    assert fixtures["sample_household"]["head_of_household"] == "Ram Prasad Semwal"


