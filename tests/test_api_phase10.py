"""
Phase 10 REST API Integration Tests (ARC-C12 / FEAT-023, FEAT-025).
"""

from tests.authutil import authed_client

client = authed_client()


def test_api_necessity_review():
    payload = {
        "case_id": "CASE-API-NEC-01",
        "household_id": "HH-API-001",
        "in_situ_mitigation_feasible": False,
        "permanent_relocation_necessary": True,
        "reviewer_name": "Dr. P. R. Menon",
        "reviewer_credentials": "Chief Geotechnical Engineer, Kerala PWD",
        "reasons": "Bedrock shear failure zone with recurrent slope instability under monsoon precipitation.",
        "uncertainty_level": "LOW",
        "settlement_community_effects": "Cluster relocation to Elstone Estate recommended.",
    }
    resp = client.post("/api/v1/delivery/necessity-review", json=payload)
    assert resp.status_code == 200, resp.text
    data = resp.json()["data"]
    assert data["case_id"] == "CASE-API-NEC-01"
    assert data["permanent_relocation_necessary"] is True

    # Get by case ID
    get_resp = client.get("/api/v1/delivery/necessity-review/CASE-API-NEC-01")
    assert get_resp.status_code == 200
    assert get_resp.json()["data"]["competent_reviewer_name"] == "Dr. P. R. Menon"


def test_api_scheme_assessment():
    # 1. Owner assessment
    owner_payload = {
        "household_id": "HH-API-OWNER-01",
        "tenure_category": "OWNER",
        "pathway": "SELF_RELOCATION_ASSISTANCE",
    }
    resp_owner = client.post("/api/v1/delivery/scheme-assessment", json=owner_payload)
    assert resp_owner.status_code == 200
    data_owner = resp_owner.json()["data"]
    assert data_owner["is_scheme_eligible"] is True
    assert data_owner["total_eligible_cost_inr"] == 1000000.0

    # 2. Tenant assessment (ineligible for owner grant, but relocation need preserved!)
    tenant_payload = {
        "household_id": "HH-API-TENANT-01",
        "tenure_category": "TENANT",
        "pathway": "SELF_RELOCATION_ASSISTANCE",
    }
    resp_tenant = client.post("/api/v1/delivery/scheme-assessment", json=tenant_payload)
    assert resp_tenant.status_code == 200
    data_tenant = resp_tenant.json()["data"]
    assert data_tenant["is_scheme_eligible"] is False
    assert data_tenant["relocation_need_preserved"] is True
    assert "TASK-ALT-TENANT-RENTAL" in data_tenant["alternative_pathway_task"]


def test_api_funding_gap_calculator():
    case_id = "CASE-API-FND-01"

    # Set required cost to ₹12 Lakhs
    resp_req = client.post(
        "/api/v1/delivery/funding/required-cost",
        json={"case_id": case_id, "required_cost_inr": 1200000.0},
    )
    assert resp_req.status_code == 200

    # Record announced budget of ₹10 Lakhs (MUST NOT reduce gap!)
    resp_ann = client.post(
        "/api/v1/delivery/funding",
        json={
            "case_id": case_id,
            "source_agency": "CMDRF",
            "cost_head": "HOUSING_CONSTRUCTION",
            "state": "SANCTIONED",
            "amount_inr": 1000000.0,
            "is_announced_budget_only": True,
        },
    )
    assert resp_ann.status_code == 200

    # Gap must still be ₹12 Lakhs
    gap1 = client.get(f"/api/v1/delivery/funding-gap/{case_id}").json()["data"]
    assert gap1["total_required_cost_inr"] == 1200000.0
    assert gap1["total_received_funds_inr"] == 0.0
    assert gap1["funding_gap_inr"] == 1200000.0
    assert gap1["is_fully_funded"] is False

    # Receive ₹8 Lakhs from SDRF
    client.post(
        "/api/v1/delivery/funding",
        json={
            "case_id": case_id,
            "source_agency": "SDRF",
            "cost_head": "HOUSING_CONSTRUCTION",
            "state": "RECEIVED",
            "amount_inr": 800000.0,
        },
    )

    # Gap is now ₹4 Lakhs
    gap2 = client.get(f"/api/v1/delivery/funding-gap/{case_id}").json()["data"]
    assert gap2["total_received_funds_inr"] == 800000.0
    assert gap2["funding_gap_inr"] == 400000.0
    assert gap2["is_fully_funded"] is False

    # Receive remaining ₹4 Lakhs from CSR partner
    client.post(
        "/api/v1/delivery/funding",
        json={
            "case_id": case_id,
            "source_agency": "CSR_PARTNER",
            "cost_head": "INFRASTRUCTURE",
            "state": "RECEIVED",
            "amount_inr": 400000.0,
        },
    )

    # Gap is now 0 (fully funded)
    gap3 = client.get(f"/api/v1/delivery/funding-gap/{case_id}").json()["data"]
    assert gap3["funding_gap_inr"] == 0.0
    assert gap3["is_fully_funded"] is True


def test_api_services_defects_and_completion_gates():
    case_id = "CASE-API-GATES-01"

    # Step 1: Mark unit constructed
    client.post("/api/v1/delivery/unit-constructed", json={"case_id": case_id, "officer_name": "PWD Assistant Engineer"})

    # Step 2: Handover attempt before services verified fails with 412
    resp_h1 = client.post("/api/v1/delivery/handover", json={"case_id": case_id, "officer_name": "Tahsildar"})
    assert resp_h1.status_code == 412

    # Step 3: Verify basic services with deficient water (< 55 LPCD)
    client.post(
        "/api/v1/delivery/services-readiness",
        json={
            "case_id": case_id,
            "water_supply_lpcd": 40.0,
            "electricity_energised": True,
            "all_weather_road_functional": True,
            "sanitation_drainage_functional": True,
            "officer_name": "Joint Inspection Team",
        },
    )
    # Handover fails because water < 55 LPCD
    resp_h2 = client.post("/api/v1/delivery/handover", json={"case_id": case_id, "officer_name": "Tahsildar"})
    assert resp_h2.status_code == 412
    assert "below 55 LPCD" in resp_h2.json()["detail"]

    # Step 4: Verify services with compliant water (75 LPCD)
    client.post(
        "/api/v1/delivery/services-readiness",
        json={
            "case_id": case_id,
            "water_supply_lpcd": 75.0,
            "electricity_energised": True,
            "all_weather_road_functional": True,
            "sanitation_drainage_functional": True,
            "officer_name": "Joint Inspection Team",
        },
    )

    # Step 5: Log a CRITICAL structural defect
    d_resp = client.post(
        "/api/v1/delivery/defects",
        json={
            "case_id": case_id,
            "site_id": "SITE-ELSTONE-01",
            "unit_id": "UNIT-B04",
            "category": "STRUCTURAL",
            "severity": "CRITICAL",
            "description": "Roof truss deflection exceeds safety limits",
            "officer_name": "Quality Control Inspector",
        },
    )
    assert d_resp.status_code == 200
    defect_id = d_resp.json()["data"]["defect_id"]

    # Step 6: Handover blocked by unresolved critical defect (409 Conflict)
    resp_h3 = client.post("/api/v1/delivery/handover", json={"case_id": case_id, "officer_name": "Tahsildar"})
    assert resp_h3.status_code == 409
    assert "critical/major defects unresolved" in resp_h3.json()["detail"]

    # Step 7: Resolve the defect with engineering certificate
    res_def = client.post(
        "/api/v1/delivery/defects/resolve",
        json={
            "defect_id": defect_id,
            "evidence_ref": "PWD-TRUSS-RETROFIT-CERT-2024",
            "officer_name": "Executive Engineer PWD Buildings",
            "case_id": case_id,
        },
    )
    assert res_def.status_code == 200
    assert res_def.json()["data"]["is_resolved"] is True

    # Step 8: Handover now succeeds!
    resp_h4 = client.post("/api/v1/delivery/handover", json={"case_id": case_id, "officer_name": "Tahsildar"})
    assert resp_h4.status_code == 200
    assert resp_h4.json()["data"]["status"] == "HANDED_OVER"

    # Step 9: Offer accepted & physical occupation verified
    client.post("/api/v1/delivery/offer-acceptance", json={"case_id": case_id, "officer_name": "Community Officer"})
    client.post("/api/v1/delivery/occupation", json={"case_id": case_id, "field_officer_name": "Village Officer Meppadi"})

    # Step 10: Full relocation completion status check (all 6 gates satisfied)
    status_resp = client.get(f"/api/v1/delivery/completion-status/{case_id}")
    assert status_resp.status_code == 200
    assert status_resp.json()["data"]["is_relocation_complete"] is True
    assert len(status_resp.json()["data"]["blocking_reasons"]) == 0


def test_api_external_handoff_and_followup():
    case_id = "CASE-API-EXT-01"

    # Register external handoff to LIFE Mission
    resp_handoff = client.post(
        "/api/v1/delivery/external-handoff",
        json={
            "case_id": case_id,
            "external_system_name": "LIFE_MISSION",
            "external_reference_id": "LIFE-KL-WYD-2024-9912",
            "accountable_agency": "Local Self Government Department",
            "accountable_officer": "District Mission Coordinator, Wayanad",
            "delegated_scope": "Individual subsidy milestone monitoring",
        },
    )
    assert resp_handoff.status_code == 200
    assert resp_handoff.json()["data"]["is_physically_completed"] is False

    # Record 6-month livelihood follow-up
    resp_fol = client.post(
        "/api/v1/delivery/followup",
        json={
            "case_id": case_id,
            "milestone_stage": "6_MONTH",
            "livelihood_restored": True,
            "income_restoration_pct": 88.5,
            "schooling_continuity": True,
            "healthcare_accessible": True,
            "infrastructure_rating": "SATISFACTORY",
            "community_satisfaction": 0.85,
            "officer_name": "KSDMA Social Audit Wing",
        },
    )
    assert resp_fol.status_code == 200
    assert resp_fol.json()["data"]["livelihood_restored"] is True

    # Query follow-ups
    get_fol = client.get(f"/api/v1/delivery/followup/{case_id}")
    assert get_fol.status_code == 200
    assert len(get_fol.json()["data"]) == 1
