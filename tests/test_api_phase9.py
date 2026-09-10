"""
Phase 9 REST API Integration Tests (ARC-C08, ARC-C09 / FEAT-015, FEAT-016, FEAT-017, FEAT-024).
"""

from datetime import datetime, timezone, timedelta
from tests.authutil import authed_client, issue_step_up
from sthira.core.identity import get_prototype_user

client = authed_client()


def test_api_phase9_approval_and_statutory_notification_flow():
    # 1. Issue official approval with step-up MFA
    app_payload = {
        "entity_type": "SITE_SELECTION",
        "entity_id": "SITE-API-TEST-01",
        "entity_version": "v1.0",
        "approving_officer_name": "District Collector, Wayanad",
        "approving_officer_designation": "DDMA Chairperson",
        "statutory_authority_basis": "Disaster Management Act 2005 §30(2)(v)",
        "approval_order_number": "DDMA/WYD/2024/APP-901",
        "step_up_token": issue_step_up(get_prototype_user("collector.wayanad")),
        "conditions": [
            {
                "condition_id": "COND-WATER-01",
                "condition_type": "WATER_YIELD_VERIFICATION",
                "description": "TWAD / Kerala Water Authority lean season yield test certificate.",
                "is_blocking_for_allocation": True,
                "is_satisfied": False,
            }
        ],
    }
    resp = client.post("/api/v1/governance/approvals", json=app_payload)
    assert resp.status_code == 200, resp.text
    data = resp.json()["data"]
    approval_id = data["approval_id"]
    assert approval_id.startswith("APP-SITE-")
    assert data["authority_state"] == "OFFICIALLY_APPROVED"

    # 2. Check can-allocate is blocked by unmet condition
    resp_alloc = client.get(f"/api/v1/governance/approvals/{approval_id}/can-allocate")
    assert resp_alloc.status_code == 200
    assert resp_alloc.json()["data"]["can_allocate"] is False
    assert len(resp_alloc.json()["data"]["blocking_failures"]) == 1

    # 3. Satisfy the blocking condition
    resp_sat = client.post(
        f"/api/v1/governance/approvals/{approval_id}/conditions/COND-WATER-01/satisfy",
        json={
            "verification_doc_hash": "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef0",
            "officer_name": "Executive Engineer KWA",
        },
    )
    assert resp_sat.status_code == 200
    assert resp_sat.json()["data"]["is_satisfied"] is True

    # Check can-allocate is now True
    resp_alloc2 = client.get(f"/api/v1/governance/approvals/{approval_id}/can-allocate")
    assert resp_alloc2.status_code == 200
    assert resp_alloc2.json()["data"]["can_allocate"] is True

    # 4. Publish statutory notification (bilingual gazette publication)
    notif_payload = {
        "approval_id": approval_id,
        "gazette_notification_number": "EXTRA-GAZ-2024-WYD-441",
        "gazette_volume_number": "Vol. XIII No. 209",
        "effective_date": (datetime.now(timezone.utc) + timedelta(days=1)).isoformat(),
        "notification_title_en": "Statutory Notification of Resettlement Site Acquisition",
        "notification_title_ml": "പുനരധിവാസ ഭൂമി ഏറ്റെടുക്കൽ സംബന്ധിച്ച വിജ്ഞാപനം",
        "notification_text_en": "The District Disaster Management Authority hereby notifies the acquisition...",
        "notification_text_ml": "ദുരന്ത നിവാരണ അതോറിറ്റി ഇതിനാൽ പുനരധിവാസ ഭൂമി വിജ്ഞാപനം ചെയ്യുന്നു...",
        "issuing_authority": "Revenue (Disaster Management) Department, Govt. of Kerala",
        "signing_officer_name": "Dr. D. S. Collector IAS",
        "digital_signature_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    }
    resp_notif = client.post("/api/v1/governance/notifications", json=notif_payload)
    assert resp_notif.status_code == 200, resp_notif.text
    notif_data = resp_notif.json()["data"]
    notif_id = notif_data["notification_id"]
    assert notif_id.startswith("NOTIF-GAZ-")
    assert notif_data["is_active"] is True

    # 5. Fetch notification by ID
    resp_get_notif = client.get(f"/api/v1/governance/notifications/{notif_id}")
    assert resp_get_notif.status_code == 200
    assert resp_get_notif.json()["data"]["notification_title_ml"] == "പുനരധിവാസ ഭൂമി ഏറ്റെടുക്കൽ സംബന്ധിച്ച വിജ്ഞാപനം"


def test_api_phase9_approval_unauthorized_token():
    app_payload = {
        "entity_type": "SITE_SELECTION",
        "entity_id": "SITE-API-FAIL-01",
        "entity_version": "v1.0",
        "approving_officer_name": "Imposter User",
        "approving_officer_designation": "Staff",
        "statutory_authority_basis": "DM Act §30",
        "approval_order_number": "ORD-001",
        "step_up_token": "INVALID-TOKEN",
    }
    resp = client.post("/api/v1/governance/approvals", json=app_payload)
    assert resp.status_code == 403


def test_api_phase9_citizen_objection_freezing_and_remedy_flow():
    target_entity = "SITE-FREEZE-TEST-01"

    # 1. File objection
    file_payload = {
        "household_id": "HH-API-OBJ-01",
        "filer_name": "Damodaran N.",
        "target_entity_type": "SITE_SELECTION",
        "target_entity_id": target_entity,
        "target_version_id": "v1.0",
        "category": "WATER_INADEQUACY",
        "statement": "Proposed site experiences severe water deficit between February and May.",
        "assigned_officer_id": "OFF-REV-001",
        "assigned_officer_name": "Tahsildar Vythiri",
        "filing_channel": "ASSISTED_SERVICE_DESK",
        "sla_days": 21,
    }
    resp = client.post("/api/v1/governance/objections/file", json=file_payload)
    assert resp.status_code == 200, resp.text
    case_data = resp.json()["data"]
    objection_id = case_data["objection_id"]
    assert case_data["receipt_token"].startswith("RCPT-Sthira-")

    # 2. Check target entity frozen status
    resp_frozen = client.get(f"/api/v1/governance/objections/entities/{target_entity}/frozen")
    assert resp_frozen.status_code == 200
    assert resp_frozen.json()["data"]["is_frozen"] is True
    assert objection_id in resp_frozen.json()["data"]["pending_objection_ids"]

    # 3. Attempt to issue approval while frozen should return 409
    resp_app_fail = client.post(
        "/api/v1/governance/approvals",
        json={
            "entity_type": "SITE_SELECTION",
            "entity_id": target_entity,
            "entity_version": "v1.0",
            "approving_officer_name": "Collector",
            "approving_officer_designation": "DDMA",
            "statutory_authority_basis": "DM Act §30",
            "approval_order_number": "ORD-FROZEN",
            "step_up_token": "MFA-STEPUP-SUPER-TOKEN",
        },
    )
    assert resp_app_fail.status_code == 409

    # 4. Admit objection
    resp_admit = client.post(
        f"/api/v1/governance/objections/{objection_id}/admissibility",
        json={"is_admissible": True},
    )
    assert resp_admit.status_code == 200
    assert resp_admit.json()["data"]["admissibility"] == "ADMISSIBLE"

    # 5. Schedule hearing
    hearing_date = (datetime.now(timezone.utc) + timedelta(days=7)).isoformat()
    resp_hear = client.post(
        f"/api/v1/governance/objections/{objection_id}/hearings",
        json={
            "hearing_date": hearing_date,
            "venue": "Taluk Office Vythiri",
            "presiding_officer": "Deputy Collector (LR)",
            "notified_parties": ["Damodaran N.", "Panchayat Secretary"],
        },
    )
    assert resp_hear.status_code == 200
    assert resp_hear.json()["data"]["notice_id"].startswith("NOT-HEAR-")

    # 6. Issue decision order granting remedy
    resp_dec = client.post(
        f"/api/v1/governance/objections/{objection_id}/decision",
        json={
            "relief_granted": True,
            "summary_of_grounds": "Independent borewell yield log confirms 55% deficit in summer.",
            "remedy_notes": "Mandate dedicated pipeline linkage from Chembra perennial stream before layout finalization.",
            "deciding_authority": "District Magistrate / DDMA Chairperson",
        },
    )
    assert resp_dec.status_code == 200
    assert resp_dec.json()["data"]["relief_granted"] is True

    # 7. Entity is now unfrozen
    resp_unfrozen = client.get(f"/api/v1/governance/objections/entities/{target_entity}/frozen")
    assert resp_unfrozen.status_code == 200
    assert resp_unfrozen.json()["data"]["is_frozen"] is False


def test_api_phase9_capacity_reservation_ledger_flow():
    site_id = "SITE-CAP-TEST-01"

    # 1. Configure site capacity
    conf_payload = {
        "site_id": site_id,
        "district": "Wayanad",
        "dwellings_max": 100,
        "land_cents_max": 500.0,
        "water_m3_day_max": 80.0,
    }
    resp_conf = client.post("/api/v1/capacity/sites/configure", json=conf_payload)
    assert resp_conf.status_code == 200

    # 2. Set budget
    resp_bud = client.post("/api/v1/capacity/programme-budget", json={"budget_inr": 50000000.0})
    assert resp_bud.status_code == 200

    # 3. Query remaining capacity
    resp_rem = client.get(f"/api/v1/capacity/sites/{site_id}/remaining")
    assert resp_rem.status_code == 200
    assert resp_rem.json()["data"]["dwellings_remaining"] == 100.0

    # 4. Simulate draft scenario (reserves 0 capacity)
    sim_payload = {
        "scenario_id": "SCEN-DRAFT-01",
        "site_id": site_id,
        "dwellings": 40,
        "land_cents": 200.0,
        "budget_inr": 20000000.0,
        "water_m3_day": 30.0,
    }
    resp_sim = client.post("/api/v1/capacity/simulate", json=sim_payload)
    assert resp_sim.status_code == 200
    assert resp_sim.json()["data"]["dwellings_reserved"] == 0
    assert resp_sim.json()["data"]["status"] == "SIMULATED"

    # Check remaining capacity is still 100
    resp_rem2 = client.get(f"/api/v1/capacity/sites/{site_id}/remaining")
    assert resp_rem2.json()["data"]["dwellings_remaining"] == 100.0

    # 5. Hold capacity reservation
    hold_payload = {
        "scenario_id": "SCEN-PROP-01",
        "site_id": site_id,
        "dwellings": 30,
        "land_cents": 150.0,
        "budget_inr": 15000000.0,
        "water_m3_day": 25.0,
    }
    resp_hold = client.post("/api/v1/capacity/hold", json=hold_payload)
    assert resp_hold.status_code == 200
    res_id = resp_hold.json()["data"]["reservation_id"]
    assert resp_hold.json()["data"]["status"] == "RESERVED"

    # Remaining dwellings should now be 70
    resp_rem3 = client.get(f"/api/v1/capacity/sites/{site_id}/remaining")
    assert resp_rem3.json()["data"]["dwellings_remaining"] == 70.0

    # 6. Release reservation cleanly
    resp_rel = client.post(
        "/api/v1/capacity/release",
        json={"reservation_id": res_id, "reason": "Alternative site selected in planning meeting."},
    )
    assert resp_rel.status_code == 200
    assert resp_rel.json()["data"]["status"] == "RELEASED"

    # Capacity should be returned
    resp_rem4 = client.get(f"/api/v1/capacity/sites/{site_id}/remaining")
    assert resp_rem4.json()["data"]["dwellings_remaining"] == 100.0
