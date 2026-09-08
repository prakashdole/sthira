"""
Unit tests for Phase 1 Wave A Modular Services.
Covers:
- C1-01 (Programme & Compliance)
- C1-02 (Catalog & AOI Gate)
- C1-03 (Hazard & Exposure)
- C1-05 (Household Casework & Consent)
- C1-06 (Policy Engine & Gate Evaluation)
- C1-11 (Security & Privacy)
"""

import pytest
from punarvas.core.contracts import GeographyScope, UserContext, SourceMetadata, GeoPoint
from punarvas.core.enums import RoleType, GateState, ConsentPurpose, RelocationPathway, SourceActivationState
from punarvas.core.errors import (
    AuthorityBypassError,
    UnauthorizedGeographyAccessError,
    OutOfCoverageError,
    MasterScoreProhibitedError,
)

from punarvas.modules.programme import ProgrammeRecord, programme_service
from punarvas.modules.catalog import catalog_service
from punarvas.modules.hazard import HazardLayer, hazard_service
from punarvas.modules.household import HouseholdCase, household_service
from punarvas.modules.policy import SiteCriteriaInput, policy_engine
from punarvas.modules.security import DataMinimizationValidator, security_service


def test_programme_registration_and_jurisdiction():
    """Verify C1-01 / RUL-002, RUL-054: Authorization and geography boundaries."""
    wayanad_user = UserContext(
        user_id="dmo-01",
        username="wayanad_dmo",
        roles=[RoleType.DISASTER_MANAGEMENT_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    prog = ProgrammeRecord(
        programme_id="PRG-WYD-TEST",
        title="Wayanad Test Programme",
        state="Kerala",
        district="Wayanad",
        lead_authority="DDMA Wayanad",
        mandate_legal_basis="Disaster Management Act 2005",
    )
    saved = programme_service.register_programme(prog, wayanad_user, reason="Initial setup")
    assert saved.programme_id == "PRG-WYD-TEST"

    # User from Idukki cannot register a Wayanad programme
    idukki_user = UserContext(
        user_id="dmo-idukki",
        username="idukki_dmo",
        roles=[RoleType.DISASTER_MANAGEMENT_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Idukki"),
    )
    with pytest.raises(UnauthorizedGeographyAccessError):
        programme_service.register_programme(prog, idukki_user, reason="Unauthorized setup")


def test_catalog_aoi_gate_and_quarantine():
    """Verify C1-02 / RUL-008, RUL-077: Checksum verification and quarantine on failure."""
    meta = SourceMetadata(
        source_id="S01",
        publisher="GSI",
        license_basis="Government Open Data",
        checksum_sha256="abc123expectedchecksum",
        reviewer_id="gis_officer_1",
    )

    # Corrupt/mismatched checksum triggers QUARANTINED state
    res = catalog_service.register_source_version(
        metadata=meta,
        raw_payload_checksum="mismatched_checksum_456",
        actor_id="gis_officer_1",
        reason="Faulty fixture import",
    )
    assert res.state == SourceActivationState.QUARANTINED
    assert "Checksum mismatch" in res.quarantine_reason

    # Matching checksum validates source
    res_valid = catalog_service.register_source_version(
        metadata=meta,
        raw_payload_checksum="abc123expectedchecksum",
        actor_id="gis_officer_1",
        reason="Valid fixture import",
    )
    assert res_valid.state == SourceActivationState.USABLE


def test_hazard_cflood_lockout_and_slope_runout():
    """Verify C1-03 / RUL-017 (C-FLOOD lockout) and RUL-018 (debris flow channel hazard)."""
    # RUL-017: Registering C-FLOOD for Wayanad must fail closed
    cflood_layer = HazardLayer(
        layer_id="HAZ-CFLOOD-01",
        name="C-FLOOD Inundation Forecast",
        source_id="S07",
        hazard_type="FLOOD_INUNDATION",
        hazard_level="HIGH",
        polygon_coords=[[76.1, 11.5], [76.2, 11.5], [76.2, 11.6], [76.1, 11.6], [76.1, 11.5]],
        documented_coverage="Wayanad",
    )
    with pytest.raises(OutOfCoverageError):
        hazard_service.register_hazard_layer(cflood_layer)

    # Register valid KSDMA/GSI debris flow runout layer
    gsi_layer = HazardLayer(
        layer_id="HAZ-GSI-TEST-RUNOUT",
        name="Chooralmala Debris Flow Runout Channel",
        source_id="S01",
        hazard_type="LANDSLIDE_SUSCEPTIBILITY",
        hazard_level="VERY_HIGH",
        is_debris_flow_channel=True,
        polygon_coords=[[76.13, 11.54], [76.15, 11.54], [76.15, 11.56], [76.13, 11.56], [76.13, 11.54]],
        documented_coverage="Wayanad",
    )
    hazard_service.register_hazard_layer(gsi_layer)

    # RUL-018: Flat slope (slope = 5 degrees) inside debris flow channel MUST STILL evaluate as requiring relocation
    point_inside_channel = GeoPoint(coordinates=[76.14, 11.55])
    exposure = hazard_service.evaluate_point_exposure(
        parcel_id="PARCEL-CHOORALMALA-01",
        point=point_inside_channel,
        local_slope_deg=5.0,  # low local slope
    )
    assert exposure.in_debris_flow_runout is True
    assert exposure.max_hazard_level == "VERY_HIGH"
    assert exposure.requires_relocation_review is True


def test_household_casework_and_consent_purposes():
    """Verify C1-05 / RUL-044: Purpose-specific consent recording."""
    case = HouseholdCase(
        household_id="HH-TEST-001",
        head_of_household="Anil Kumar",
        member_count=4,
        source_parcel_id="PARCEL-CHOORALMALA-01",
        verified_eligibility=True,
        chosen_pathway=RelocationPathway.TOWNSHIP,
    )
    household_service.register_case(case, actor_id="social_worker_1", reason="Field survey")

    # Record participation consent
    household_service.record_consent(
        household_id="HH-TEST-001",
        purpose=ConsentPurpose.PROGRAMME_PARTICIPATION,
        consented=True,
        actor_id="social_worker_1",
        reason="Signed voluntary participation form",
    )

    eligible_cases = household_service.list_eligible_cases()
    assert any(c.household_id == "HH-TEST-001" for c in eligible_cases)


def test_policy_engine_gates_and_omega_prohibition():
    """Verify C1-06 / RUL-013, RUL-029, RUL-030, RUL-035."""
    # RUL-035: Calling omega score must raise error
    with pytest.raises(MasterScoreProhibitedError):
        policy_engine.evaluate_omega_score()

    # Test Site failing water gate (lean-season untested -> UNKNOWN) (RUL-013, RUL-031)
    untested_water_site = SiteCriteriaInput(
        site_id="SITE-UNTESTED-WAT",
        hazard_susceptibility_level="LOW",
        in_debris_flow_runout=False,
        title_clearance_status="VERIFIED_CLEAR",
        forest_clearance_required=False,
        has_dry_season_yield_test=False,  # No lean-season yield test
        lean_season_tested_lpcd=None,
        road_access_width_m=6.0,
        distance_to_hospital_km=4.0,
        distance_to_school_km=2.0,
        dwelling_capacity=100,
    )
    report = policy_engine.evaluate_site(untested_water_site)
    assert report.overall_gate_pass is False
    wat_gate = next(g for g in report.gate_results if g.gate_id == "GATE-WAT-01")
    assert wat_gate.state == GateState.UNKNOWN

    # Test Site passing all gates (e.g. Elstone Estate model)
    valid_elstone_site = SiteCriteriaInput(
        site_id="SITE-ELSTONE-MODEL",
        hazard_susceptibility_level="LOW",
        in_debris_flow_runout=False,
        title_clearance_status="VERIFIED_CLEAR",
        forest_clearance_required=False,
        has_dry_season_yield_test=True,
        lean_season_tested_lpcd=80.0,  # > 55 LPCD (RUL-030)
        road_access_width_m=10.0,
        distance_to_hospital_km=3.0,
        distance_to_school_km=1.5,
        dwelling_capacity=200,
    )
    valid_report = policy_engine.evaluate_site(valid_elstone_site)
    assert valid_report.overall_gate_pass is True
    assert all(g.state == GateState.PASS for g in valid_report.gate_results)
    assert len(valid_report.dimension_scores) == 3


def test_security_data_minimization_and_cert_in():
    """Verify C1-11 / RUL-051 (data minimization) and CERT-In 6h incident rule."""
    payload_with_aadhaar = {
        "household_id": "HH-01",
        "aadhaar_number": "1234-5678-9012",  # prohibited by default (RUL-051)
        "member_count": 3,
    }
    violations = DataMinimizationValidator.validate_payload(payload_with_aadhaar)
    assert len(violations) == 1
    assert "Prohibited default collection" in violations[0]

    incident = security_service.create_incident_report(
        incident_type="UNAUTHORIZED_ACCESS_ATTEMPT",
        details="Access outside district boundary",
        severity="HIGH",
    )
    # Check 6-hour CERT-In reporting window
    diff_hours = (incident.cert_in_deadline - incident.detected_at).total_seconds() / 3600.0
    assert abs(diff_hours - 6.0) < 0.1
