"""
Unit and integration tests for Section 7 (PH-4: Kerala Scaling) and Section 8 (PH-5: Multi-State Adaptation).
Normative Reference: phases.md §7 & §8, DEC-036, DEC-037, rules.md (RUL-017, RUL-018, RUL-054).
"""

import pytest
from sthira.core.contracts import GeographyScope, UserContext
from sthira.core.enums import RoleType
from sthira.core.errors import UnauthorizedGeographyAccessError

from sthira.modules.scaling import (
    DistrictProfile,
    district_onboarding_service,
    statewide_oversight_service,
)
from sthira.modules.adaptation import (
    StateTenantPackage,
    multi_state_adapter_service,
)


# --- Section 7: PH-4 Kerala Scaling Tests ---

def test_district_onboarding_and_profiles():
    """Verify C4-01 / DEC-036: Multi-district onboarding in Kerala."""
    districts = district_onboarding_service.list_districts()
    d_names = [d.district_name for d in districts]

    assert "Wayanad" in d_names
    assert "Idukki" in d_names
    assert "Alappuzha" in d_names

    idu = district_onboarding_service.get_district("KL-IDU")
    assert idu is not None
    assert idu.primary_hazard_profile == "STEEP_SLOPE_LANDSLIDE"
    assert idu.min_road_width_m == 4.0  # Mountain tea road clearance

    alp = district_onboarding_service.get_district("KL-ALP")
    assert alp is not None
    assert alp.primary_hazard_profile == "LOWLAND_COASTAL_FLOOD"


def test_cross_district_isolation_enforcement():
    """Verify C4-01 / RUL-054: Strict row-level and district isolation."""
    wayanad_officer = UserContext(
        user_id="usr_wyd_01",
        username="officer_wayanad",
        roles=[RoleType.DISASTER_MANAGEMENT_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    # Authorized access to Wayanad succeeds
    district_onboarding_service.verify_district_isolation(wayanad_officer, "Wayanad")

    # Access to Idukki must be blocked with UnauthorizedGeographyAccessError (RUL-054)
    with pytest.raises(UnauthorizedGeographyAccessError) as exc_info:
        district_onboarding_service.verify_district_isolation(wayanad_officer, "Idukki")
    assert "Kerala/Wayanad" in str(exc_info.value)
    assert "Kerala/Idukki" in str(exc_info.value)


def test_statewide_oversight_role():
    """Verify C4-02: State-level KSDMA dashboard aggregates across districts without PII."""
    state_officer = UserContext(
        user_id="usr_ksdma_state",
        username="director_ksdma",
        roles=[RoleType.DISASTER_MANAGEMENT_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="*"),  # Statewide
    )

    # State officer can access any district oversight
    district_onboarding_service.verify_district_isolation(state_officer, "Idukki")
    district_onboarding_service.verify_district_isolation(state_officer, "Alappuzha")

    # Statewide dashboard provides privacy-safe macro aggregates
    dash = statewide_oversight_service.get_statewide_dashboard()
    assert dash.total_districts_onboarded >= 3
    assert dash.total_eligible_households > 0
    assert dash.data_classification == "STATEWIDE_PUBLIC_AGGREGATE"


def test_district_fixture_packs():
    """Verify C4-03: Distinct district fixture packs for Idukki and Alappuzha."""
    idu_pack = statewide_oversight_service.get_district_fixture_pack("Idukki")
    assert idu_pack["district_name"] == "Idukki"
    assert "Munnar" in idu_pack["sample_site"]["village"]

    alp_pack = statewide_oversight_service.get_district_fixture_pack("Alappuzha")
    assert alp_pack["district_name"] == "Alappuzha"
    assert "Champakulam" in alp_pack["sample_site"]["village"]


# --- Section 8: PH-5 Multi-State Adaptation Tests ---

def test_multi_state_tenant_onboarding():
    """Verify C5-01 / DEC-037: Multi-state tenant registration and CRS separation."""
    tenants = multi_state_adapter_service.list_tenants()
    codes = [t.state_code for t in tenants]

    assert "KL" in codes
    assert "UK" in codes

    uk = multi_state_adapter_service.get_tenant("UK")
    assert uk is not None
    assert uk.state_name == "Uttarakhand"
    assert uk.default_crs == "EPSG:32644"  # UTM Zone 44N
    assert "hi" in uk.official_languages
    assert "Devbhoomi" in uk.land_record_system

    kl = multi_state_adapter_service.get_tenant("KL")
    assert kl is not None
    assert kl.default_crs == "EPSG:32643"  # UTM Zone 43N
    assert "ml" in kl.official_languages


def test_cross_state_tenant_isolation():
    """Verify C5-01 / RUL-054: State tenant boundary isolation."""
    kerala_officer = UserContext(
        user_id="usr_kerala_dmo",
        username="kerala_officer",
        roles=[RoleType.DISASTER_MANAGEMENT_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    # Kerala officer can access Kerala tenant
    multi_state_adapter_service.verify_tenant_isolation(kerala_officer, "Kerala")

    # Kerala officer attempting to access Uttarakhand raises UnauthorizedGeographyAccessError
    with pytest.raises(UnauthorizedGeographyAccessError) as exc_info:
        multi_state_adapter_service.verify_tenant_isolation(kerala_officer, "Uttarakhand")
    assert "Kerala" in str(exc_info.value)
    assert "Uttarakhand" in str(exc_info.value)


def test_zero_cross_state_leakage():
    """
    Verify DEC-037: Zero cross-state leakage.
    Uttarakhand dossiers must have USDMA, Hindi glossary, Khasra/Khatauni, and NO Kerala G.O.s or Malayalam.
    Kerala dossiers must have KSDMA, Malayalam glossary, Thandaper, and NO Uttarakhand acts or Hindi.
    """
    uk_header = multi_state_adapter_service.generate_state_dossier_header("UK", "CASE-UK-001")
    assert uk_header["state_name"] == "Uttarakhand"
    assert "USDMA" in uk_header["statutory_authority"]
    assert "पुनर्वास" in uk_header["title_local"]
    assert "खसरा" in uk_header["land_tenure_label_local"]
    # Verify NO Kerala G.O.s or Malayalam leaked into Uttarakhand
    assert "VLRS" not in str(uk_header)
    assert "Meppadi" not in str(uk_header)
    assert "തണ്ടപ്പേര്" not in str(uk_header)

    kl_header = multi_state_adapter_service.generate_state_dossier_header("KL", "CASE-KL-001")
    assert kl_header["state_name"] == "Kerala"
    assert "KSDMA" in kl_header["statutory_authority"]
    assert "പുനർവാസ്" in kl_header["title_local"]
    assert "തണ്ടപ്പേര്" in kl_header["land_tenure_label_local"]
    # Verify NO Uttarakhand acts or Hindi leaked into Kerala
    assert "Devbhoomi" not in str(kl_header)
    assert "खसरा" not in str(kl_header)


def test_state_fixture_pack_uttarakhand():
    """Verify C5-02: Uttarakhand reference fixtures (Chamoli / Joshimath)."""
    uk_pack = multi_state_adapter_service.get_state_fixture_pack("UK")
    assert uk_pack["district"] == "Chamoli"
    assert "Dharkot" in uk_pack["sample_site"]["village"]
    assert uk_pack["sample_site"]["projected_crs"] == "EPSG:32644"
    assert uk_pack["sample_household"]["head_of_household"] == "Ram Prasad Semwal"
