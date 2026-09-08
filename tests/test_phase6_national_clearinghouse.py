"""
Unit and integration tests for Phase 6 National NDMA Clearinghouse,
Inter-State Federation, Trilingual Localization, and State Legal Isolation.
Normative Reference: Disaster Management Act 2005 §3/§6, rules.md (RUL-001, RUL-002, RUL-054, RUL-055), DEC-038.
"""

import pytest
from fastapi.testclient import TestClient

from punarvas.api.app import app
from punarvas.core.contracts import GeographyScope, UserContext
from punarvas.core.enums import RoleType
from punarvas.core.errors import AuthorityBypassError
from punarvas.core.localization import (
    LOCALIZATION_REGISTRY,
    get_supported_languages,
    translate_text,
)
from punarvas.modules.governance.national_clearinghouse import (
    InterStateRelocationRequest,
    NationalRegistryManifest,
    national_clearinghouse_service,
)
from punarvas.modules.programme.state_package import state_package_loader
from punarvas.spikes.fixture_loader import load_uttarakhand_fixture


client = TestClient(app)


def test_trilingual_localization_engine():
    """Verify RUL-055: English, Malayalam, and Hindi status and warning parity."""
    langs = get_supported_languages()
    assert "en" in langs
    assert "ml" in langs
    assert "hi" in langs

    for lang in ["en", "ml", "hi"]:
        # Verify mandatory advisory banner translated
        banner = translate_text("advisory_banner", lang)
        assert len(banner) > 10

        # Verify all hard gate states translated
        assert translate_text("gate_pass", lang) != ""
        assert translate_text("gate_fail", lang) != ""
        assert translate_text("gate_unknown", lang) != ""
        assert translate_text("gate_blocked", lang) != ""


def test_national_clearinghouse_corridors():
    """Verify NDMA cross-border inter-state hazard corridor registry."""
    corridors = national_clearinghouse_service.list_corridors()
    assert len(corridors) >= 2

    c_ids = [c.corridor_id for c in corridors]
    assert "CORR-WG-01" in c_ids  # Western Ghats Nilgiri-Wayanad
    assert "CORR-HIM-02" in c_ids  # Upper Ganga-Alaknanda

    wg = next(c for c in corridors if c.corridor_id == "CORR-WG-01")
    assert "Kerala" in wg.affected_states
    assert "Tamil Nadu" in wg.affected_states
    assert "Karnataka" in wg.affected_states


def test_interstate_relocation_request_submission():
    """Verify inter-state assistance submission and role enforcement (RUL-002)."""
    # 1. Non-approver user must be blocked
    analyst_user = UserContext(
        user_id="analyst_01",
        username="gis_analyst",
        roles=[RoleType.GIS_ANALYST],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    req = InterStateRelocationRequest(
        origin_state="Kerala",
        origin_district="Wayanad",
        destination_state="Karnataka",
        disaster_event="Wayanad Landslides 2024",
        total_affected_households=50,
        requested_assistance_type="INTERSTATE_LAND_EXCHANGE",
    )
    with pytest.raises(AuthorityBypassError):
        national_clearinghouse_service.submit_interstate_request(
            req=req,
            user=analyst_user,
            reason="Unauthorized submission test",
        )

    # 2. Government Approver successfully submits
    approver_user = UserContext(
        user_id="ksdma_secretary",
        username="sec_ksdma",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state="Kerala", district="*"),
    )
    res = national_clearinghouse_service.submit_interstate_request(
        req=req,
        user=approver_user,
        reason="Inter-state plantation worker rehabilitation coordination",
    )
    assert res.request_id.startswith("NDMA-REQ-")
    assert res.status == "SUBMITTED"

    # Verify query
    requests = national_clearinghouse_service.list_requests(state="Kerala")
    assert any(r.request_id == res.request_id for r in requests)


def test_state_manifest_federation_to_ndma():
    """Verify state relocation registry manifest federation with cryptographic hash."""
    user = UserContext(
        user_id="state_daemon",
        username="sdma_registry_daemon",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    manifest = NationalRegistryManifest(
        state="Kerala",
        district="Wayanad",
        programme_id="PRG-WYD-2024",
        verified_eligible_count=380,
        allocated_count=250,
        state_audit_head_hash="a1b2c3d4e5f67890abcdef1234567890abcdef1234567890abcdef1234567890",
    )
    registered = national_clearinghouse_service.federate_state_manifest(
        manifest=manifest,
        user=user,
        reason="Periodic NDMA national registry federation",
    )
    assert registered.manifest_id.startswith("NDMA-REG-")

    manifests = national_clearinghouse_service.list_manifests(state="Kerala")
    assert any(m.manifest_id == registered.manifest_id for m in manifests)


def test_state_package_loader_and_legal_isolation():
    """Verify C5-01 / DEC-037: Strict legal and scheme isolation between Kerala and Uttarakhand."""
    pkgs = state_package_loader.list_state_packages()
    state_ids = [p.state_id for p in pkgs]
    assert "Kerala" in state_ids
    assert "Uttarakhand" in state_ids

    # Kerala schemes must contain VLRS and Wayanad Township, but NO Uttarakhand schemes
    kl_schemes = state_package_loader.get_state_schemes("Kerala")
    kl_names = [s.name for s in kl_schemes]
    assert any("Vulnerability Linked Relocation Scheme" in n for n in kl_names)
    assert not any("Mukhya Mantri Punarvas" in n for n in kl_names)
    assert not any("Joshimath" in n for n in kl_names)

    # Uttarakhand schemes must contain Mukhya Mantri Punarvas and Joshimath, but NO Kerala schemes
    uk_schemes = state_package_loader.get_state_schemes("Uttarakhand")
    uk_names = [s.name for s in uk_schemes]
    assert any("Mukhya Mantri Punarvas" in n for n in uk_names)
    assert any("Joshimath" in n for n in uk_names)
    assert not any("VLRS" in n for n in uk_names)

    # Workflow map isolation
    kl_wf = state_package_loader.get_state_workflow_map("Kerala")
    assert "Grama Sabha" in kl_wf.forest_rights_consultation_body
    assert "LSGD" in kl_wf.local_disaster_plan_format

    uk_wf = state_package_loader.get_state_workflow_map("Uttarakhand")
    assert "Van Panchayat" in uk_wf.forest_rights_consultation_body
    assert "DDMA" in uk_wf.local_disaster_plan_format


def test_load_uttarakhand_fixture():
    """Verify C5-02: Second state reference pilot fixture loads correctly."""
    data = load_uttarakhand_fixture()
    assert data["state_package"]["state_name"] == "Uttarakhand"
    assert data["programme"]["district"] == "Chamoli"
    assert len(data["hazard_layers"]) >= 2
    assert len(data["affected_parcels"]) >= 2
    assert len(data["candidate_sites"]) >= 2
    assert len(data["synthetic_households"]) >= 2


def test_phase6_api_endpoints():
    """Verify FastAPI integration for Phase 6 clearinghouse and localization."""
    # 1. Supported languages
    resp = client.get("/api/v1/localization/languages")
    assert resp.status_code == 200
    assert "hi" in resp.json()["data"]["supported_languages"]

    # 2. Localized dictionary
    resp = client.get("/api/v1/localization/hi")
    assert resp.status_code == 200
    assert "पुनर्वास" in resp.json()["data"]["brand_title"]

    # 3. Inter-state hazard corridors
    resp = client.get("/api/v1/national/clearinghouse/corridors")
    assert resp.status_code == 200
    assert len(resp.json()["data"]) >= 2

    # 4. Submit inter-state request
    req_payload = {
        "origin_state": "Uttarakhand",
        "origin_district": "Chamoli",
        "destination_state": "Himachal Pradesh",
        "disaster_event": "Joshimath Land Subsidence",
        "total_affected_households": 120,
        "requested_assistance_type": "NDRF_SPECIAL_PACKAGE",
    }
    resp = client.post("/api/v1/national/clearinghouse/requests", json=req_payload)
    assert resp.status_code == 200
    assert resp.json()["data"]["origin_state"] == "Uttarakhand"

    # 5. List requests
    resp = client.get("/api/v1/national/clearinghouse/requests?state=Uttarakhand")
    assert resp.status_code == 200
    assert len(resp.json()["data"]) >= 1
