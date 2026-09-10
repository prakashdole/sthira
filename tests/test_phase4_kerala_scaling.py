"""
Tests for Phase 4: Kerala Multi-District Scaling, Policy Inheritance, and Row-Level Isolation (PH-4 / C4-01, C4-02, C4-03).
Normative Reference: phases.md §7 & §12.7, rules.md (RUL-017, RUL-018, RUL-054), trd.md (NFR-005, NFR-012, NFR-030), DEC-036.
"""

import pytest
from tests.authutil import authed_client
from sthira.core.contracts import GeographyScope, UserContext
from sthira.core.enums import RoleType
from sthira.modules.district_scale import (
    DistrictProfile,
    DistrictPolicyOverride,
    district_onboarding_engine,
    policy_inheritance_engine,
    multi_district_isolation_manager,
    statewide_aggregate_dashboard,
    scale_quota_limiter,
)
from sthira.spikes import load_district_fixture, load_idukki_fixture, load_alappuzha_fixture

client = authed_client()


# --- C4-01: Configuration-Driven District Onboarding & Profiling (DEC-036) ---

def test_district_onboarding_factory_profiles():
    districts = district_onboarding_engine.list_districts()
    d_ids = [d.district_id for d in districts]
    assert "DIST-WAYANAD" in d_ids
    assert "DIST-IDUKKI" in d_ids
    assert "DIST-ALAPPUZHA" in d_ids

    # Wayanad profile
    wyd = district_onboarding_engine.get_district("DIST-WAYANAD")
    assert wyd.terrain_type == "HIGHLAND_MOUNTAIN"
    assert "DEBRIS_FLOW_RUNOUT" in wyd.primary_hazard_types
    assert "COASTAL_STORM_SURGE" in wyd.prohibited_hazard_assumptions

    # Alappuzha profile
    alp = district_onboarding_engine.get_district("DIST-ALAPPUZHA")
    assert alp.terrain_type == "COASTAL_LOWLAND_POLDER"
    assert "KUTTANAD_BELOW_SEA_LEVEL_FLOOD" in alp.primary_hazard_types
    assert "DEBRIS_FLOW_RUNOUT" in alp.prohibited_hazard_assumptions


# --- C4-01: Policy Inheritance and District Override Isolation (RUL-017) ---

def test_policy_inheritance_and_isolated_overrides():
    # 1. State baseline parameter resolution
    baseline_grant = policy_inheritance_engine.resolve_parameter("DIST-WAYANAD", "KERALA_VLRS_MAX_GRANT_INR")
    assert baseline_grant == 1000000.0  # ₹10 Lakh statutory cap

    # 2. Wayanad local override: 7 cents plot size
    wyd_plot = policy_inheritance_engine.resolve_parameter("DIST-WAYANAD", "TOWNSHIP_PLOT_SIZE_CENTS")
    assert wyd_plot == 7.0

    # 3. Alappuzha local override: 1.5m plinth elevation freeboard for below-sea-level polders
    alp_freeboard = policy_inheritance_engine.resolve_parameter("DIST-ALAPPUZHA", "MINIMUM_PLINTH_FREEBOARD_METERS")
    assert alp_freeboard == 1.5

    # 4. Anti-Leakage (RUL-017): Alappuzha does NOT inherit Wayanad's 7 cents override
    # Alappuzha resolves to state baseline or error if not defined
    with pytest.raises(KeyError):
        policy_inheritance_engine.resolve_parameter("DIST-ALAPPUZHA", "TOWNSHIP_PLOT_SIZE_CENTS")

    with pytest.raises(KeyError):
        policy_inheritance_engine.resolve_parameter("DIST-WAYANAD", "MINIMUM_PLINTH_FREEBOARD_METERS")


# --- C4-01: Row-Level and Geography-Scoped Isolation Enforcement (RUL-054 / NFR-012) ---

def test_geography_scoped_row_level_security():
    wayanad_officer = UserContext(
        user_id="usr_collector_wyd",
        username="collector_wayanad",
        roles=[RoleType.DISASTER_MANAGEMENT_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    idukki_officer = UserContext(
        user_id="usr_collector_idu",
        username="collector_idukki",
        roles=[RoleType.DISASTER_MANAGEMENT_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Idukki"),
    )
    state_admin = UserContext(
        user_id="usr_ksdma_dir",
        username="director_ksdma",
        roles=[RoleType.STATE_PROGRAMME_ADMIN],
        geography_scope=GeographyScope(state="Kerala", district=None),
    )

    # Wayanad officer can access Wayanad
    assert multi_district_isolation_manager.assert_user_can_access_district(wayanad_officer, "DIST-WAYANAD") is True

    # Wayanad officer CANNOT access Idukki (RUL-054)
    with pytest.raises(PermissionError) as exc:
        multi_district_isolation_manager.assert_user_can_access_district(wayanad_officer, "DIST-IDUKKI")
    assert "Geography Access Denied" in str(exc.value)

    # Idukki officer CANNOT access Wayanad
    with pytest.raises(PermissionError):
        multi_district_isolation_manager.assert_user_can_access_district(idukki_officer, "DIST-WAYANAD")

    # State administrator has read-only oversight across all districts
    assert multi_district_isolation_manager.assert_user_can_access_district(state_admin, "DIST-WAYANAD", action="READ") is True
    assert multi_district_isolation_manager.assert_user_can_access_district(state_admin, "DIST-IDUKKI", action="READ") is True

    # State administrator CANNOT usurp local district statutory approval
    with pytest.raises(PermissionError) as exc:
        multi_district_isolation_manager.assert_user_can_access_district(
            state_admin, "DIST-WAYANAD", action="APPROVE_STATUTORY_DECISION"
        )
    assert "cannot usurp district statutory approval" in str(exc.value)


# --- C4-02: Statewide Macro Aggregates & Scale Limiter (NFR-005) ---

def test_statewide_dashboard_and_rate_limiting():
    # Statewide summary
    summary = statewide_aggregate_dashboard.get_statewide_summary()
    assert summary.total_onboarded_districts >= 3
    assert summary.total_statewide_eligible_households > 0
    assert summary.total_statewide_allocated > 0
    assert summary.total_statewide_budget_sanctioned_inr > 0
    assert len(summary.district_breakdowns) >= 3

    # Concurrency and queue quotas per district (NFR-005)
    dist_id = "DIST-IDUKKI"
    # Acquire up to 5 slots
    acquired = [scale_quota_limiter.acquire_job_slot(dist_id) for _ in range(5)]
    assert all(acquired) is True
    assert scale_quota_limiter.get_active_job_count(dist_id) == 5

    # 6th concurrent job exceeds quota and is rejected
    assert scale_quota_limiter.acquire_job_slot(dist_id) is False

    # Release slot restores capacity
    scale_quota_limiter.release_job_slot(dist_id)
    assert scale_quota_limiter.get_active_job_count(dist_id) == 4
    assert scale_quota_limiter.acquire_job_slot(dist_id) is True

    # Clean up
    for _ in range(5):
        scale_quota_limiter.release_job_slot(dist_id)


# --- C4-03: Independent Non-Wayanad Reference Fixtures ---

def test_non_wayanad_reference_fixtures_independence():
    idu_data = load_idukki_fixture()
    assert idu_data["programme"]["district"] == "Idukki"
    assert any("SLOPE" in hz["layer_id"] for hz in idu_data["hazard_layers"])

    alp_data = load_alappuzha_fixture()
    assert alp_data["programme"]["district"] == "Alappuzha"
    assert any("POLDER" in hz["layer_id"] for hz in alp_data["hazard_layers"])
    # Zero debris flow runout channels in Alappuzha delta
    for hz in alp_data["hazard_layers"]:
        assert hz.get("debris_flow_channel", False) is False


# --- FastAPI Phase 4 Endpoints Integration Tests ---

def test_api_phase4_endpoints():
    # 1. List districts
    res = client.get("/api/v1/scaling/districts")
    assert res.status_code == 200
    districts = res.json()["data"]
    assert len(districts) >= 3

    # 2. Get policy for district
    pol_res = client.get("/api/v1/districts/DIST-WAYANAD/policy")
    assert pol_res.status_code == 200
    pol_data = pol_res.json()["data"]
    assert "state_baselines" in pol_data
    assert len(pol_data["overrides"]) > 0

    # 3. Register custom policy override
    ovr_res = client.post(
        "/api/v1/districts/DIST-IDUKKI/policy/override",
        json={
            "override_id": "OVR-IDU-TEA-ROAD",
            "district_id": "DIST-IDUKKI",
            "parameter_key": "TEA_ESTATE_ACCESS_ROAD_MIN_WIDTH_M",
            "state_baseline_value": 3.66,
            "district_override_value": 4.5,
            "rationale": "Tea estate heavy emergency vehicle access clearance",
            "statutory_order_reference": "G.O. (Ms) Idukki Mountain Roads",
            "is_active": True,
        },
    )
    assert ovr_res.status_code == 200
    assert ovr_res.json()["data"]["parameter_key"] == "TEA_ESTATE_ACCESS_ROAD_MIN_WIDTH_M"

    # 4. District isolation check endpoint
    iso_allow = client.post(
        "/api/v1/scaling/district-isolation-check",
        json={
            "user_id": "usr_wyd",
            "username": "officer_wayanad",
            "roles": ["DISASTER_MANAGEMENT_OFFICER"],
            "district_scope": "Wayanad",
            "target_district_id": "DIST-WAYANAD",
            "action": "READ",
        },
    )
    assert iso_allow.status_code == 200
    assert iso_allow.json()["data"]["allowed"] is True

    # Blocked cross-district access returns 403
    iso_deny = client.post(
        "/api/v1/scaling/district-isolation-check",
        json={
            "user_id": "usr_wyd",
            "username": "officer_wayanad",
            "roles": ["DISASTER_MANAGEMENT_OFFICER"],
            "district_scope": "Wayanad",
            "target_district_id": "DIST-IDUKKI",
            "action": "READ",
        },
    )
    assert iso_deny.status_code == 403

    # 5. Rate limiter endpoints
    slot_res = client.post("/api/v1/districts/DIST-WAYANAD/acquire-slot")
    assert slot_res.status_code == 200
    assert slot_res.json()["data"]["acquired"] is True

    rel_res = client.post("/api/v1/districts/DIST-WAYANAD/release-slot")
    assert rel_res.status_code == 200
    assert rel_res.json()["data"]["released"] is True
