"""
Unit tests for Phase 1 Wave B Modular Services.
Covers:
- C1-04 (Land Truth & Discrepancies)
- C1-07 (Allocation & Validator)
- C1-08 (Governance, Objections & Schemes)
- C1-09 (Reporting, Manifests & Bilingual Parity)
"""

import pytest
from punarvas.core.contracts import GeoPoint
from punarvas.core.enums import DiscrepancyType, RelocationPathway, DecisionState, FundingState

from punarvas.modules.land_truth import ParcelRecord, land_truth_service
from punarvas.modules.household import HouseholdCase
from punarvas.modules.allocation import AllocationService, AllocationValidator, allocation_service
from punarvas.modules.governance import SchemeMilestoneTracker, governance_service
from punarvas.modules.reporting import reporting_service


def test_land_truth_discrepancy_detection():
    """Verify C1-04 / RUL-021-024: Detects paper vs ground discrepancies."""
    # Parcel with paper poramboke but 4 structures observed on ground
    p1 = ParcelRecord(
        parcel_id="PARCEL-DISC-01",
        survey_number="12/3",
        village="Meppadi",
        lsg_name="Meppadi",
        centroid=GeoPoint(coordinates=[76.13, 11.54]),
        area_cents=10.0,
        paper_title_holder="Government Poramboke",
        paper_classification="PORAMBOKE",
        observed_structures_count=4,
        ground_occupied=True,
        cadastral_offset_meters=35.0,  # > 25m offset (Bhulekh shift)
        has_fra_claim=True,
    )
    land_truth_service.register_parcel(p1)

    tasks = land_truth_service.detect_discrepancies("PARCEL-DISC-01", actor_id="gis_rev_01")
    assert len(tasks) == 3
    types = {t.discrepancy_type for t in tasks}
    assert DiscrepancyType.PAPER_VACANT_GROUND_OCCUPIED in types
    assert DiscrepancyType.CADASTRAL_GRID_SHIFT in types
    assert DiscrepancyType.UNRESOLVED_FRA_CLAIM in types


def test_allocation_scenario_and_validator():
    """Verify C1-07 / RUL-040-043, RUL-074: Capacity limits, indivisibility, and plain-language explanations."""
    households = [
        HouseholdCase(
            household_id="HH-01",
            head_of_household="Person A",
            member_count=4,
            elderly_count=1,
            requires_ground_floor=True,
            source_parcel_id="PARCEL-1",
            chosen_pathway=RelocationPathway.TOWNSHIP,
            preferred_site_ids=["SITE-ELSTONE"],
        ),
        HouseholdCase(
            household_id="HH-02",
            head_of_household="Person B",
            member_count=3,
            source_parcel_id="PARCEL-2",
            chosen_pathway=RelocationPathway.SELF_RELOCATION_ASSISTANCE,  # VLRS ₹10L
        ),
        HouseholdCase(
            household_id="HH-03",
            head_of_household="Person C",
            member_count=2,
            source_parcel_id="PARCEL-3",
            chosen_pathway=RelocationPathway.TOWNSHIP,
            preferred_site_ids=["SITE-ELSTONE"],
        ),
    ]

    # Site has capacity for ONLY 1 dwelling
    site_capacities = {"SITE-ELSTONE": 1}
    site_plot_cents = {"SITE-ELSTONE": 7.0}

    scenario = allocation_service.generate_scenario(
        scenario_id="SCEN-TEST-01",
        households=households,
        site_capacities=site_capacities,
        site_plot_cents=site_plot_cents,
        actor_id="alloc_officer_1",
    )

    assert scenario.assigned_count == 2  # HH-01 (township) + HH-02 (self-relocation assistance)
    assert scenario.unassigned_count == 1  # HH-03 unassigned due to site capacity

    # Check that HH-01 got ground floor accommodation
    a1 = next(a for a in scenario.assignments if a.household_id == "HH-01")
    assert a1.assigned_site_id == "SITE-ELSTONE"
    assert a1.is_ground_floor is True
    assert "ground-floor accessibility" in a1.explanation

    # Check unassigned explanation for HH-03 (RUL-042)
    a3 = next(a for a in scenario.assignments if a.household_id == "HH-03")
    assert a3.assigned_site_id is None
    assert "capacity" in a3.explanation

    # Independent Validator (RUL-074)
    hh_map = {h.household_id: h for h in households}
    violations = AllocationValidator.validate(scenario, site_capacities, hh_map)
    assert len(violations) == 0


def test_governance_objections_and_milestone_completion():
    """Verify C1-08 / RUL-048, RUL-072: Citizen objections and strict completion invariants."""
    # File objection
    obj = governance_service.file_objection(
        household_id="HH-04",
        target_decision_id="DEC-SITE-ELSTONE",
        reason_category="PATHWAY_PREFERENCE",
        statement="Household requests VLRS self-relocation assistance instead of township.",
        assigned_officer_id="officer_appeals_1",
        actor_id="citizen_rep_1",
    )
    assert obj.state == DecisionState.OBJECTION_FILED

    # Resolve objection
    resolved = governance_service.resolve_objection(
        objection_id=obj.objection_id,
        remedy_granted=True,
        remedy_notes="Changed pathway to VLRS assistance after applicant request.",
        officer_id="officer_appeals_1",
    )
    assert resolved.state == DecisionState.REMEDY_GRANTED

    # Test Milestone Completion Invariant (RUL-072)
    tracker = SchemeMilestoneTracker(
        case_id="CASE-WYD-01",
        household_id="HH-01",
        pathway=RelocationPathway.TOWNSHIP,
        scheme_name="Wayanad Model Township",
        sanctioned_amount_inr=1500000.0,
        funding_state=FundingState.RELEASED,
        unit_ready=True,
        water_service_functioning=True,
        electricity_service_functioning=False,  # Power not yet functioning!
        handover_possession_signed=True,
        occupation_verified=True,
    )
    governance_service.update_scheme_progress(tracker, actor_id="inspector_1")
    # Incomplete because electricity is missing (RUL-072)
    assert tracker.is_relocation_complete is False

    # Now fulfill all conditions
    tracker.electricity_service_functioning = True
    governance_service.update_scheme_progress(tracker, actor_id="inspector_1")
    assert tracker.is_relocation_complete is True


def test_reporting_bilingual_dossier_and_public_projection():
    """Verify C1-09 / RUL-052, RUL-055, RUL-058: Accessible bilingual dossier and de-identified summary."""
    report = reporting_service.generate_bilingual_dossier_html(
        programme_title="Wayanad Resettlement Programme",
        household_id="HH-WYD-001",
        head_name="Ramesh K",
        pathway="TOWNSHIP",
        assigned_site="Elstone Estate Model Township",
        gate_status="PASS",
        generating_user_id="officer_dossier_1",
    )
    html = report["html"]
    # Check English and Malayalam parallel content (RUL-055)
    assert "PUNARVAS-AI Permanent Relocation Advisory Dossier" in html
    assert "പുനർവാസ്-എഐ ശാശ്വത പുനരധിവാസ ഉപദേശക രേഖ" in html
    assert "ശ്രദ്ധിക്കുക" in html
    assert len(report["checksum"]) == 64
    assert report["manifest"].sha256_checksum == report["checksum"]

    # Check De-identified Public Projection (RUL-052)
    public_sum = reporting_service.generate_public_deidentified_summary(
        district="Wayanad",
        total_eligible=430,
        township_count=350,
        self_relocation_count=60,
        unassigned_count=20,
    )
    assert public_sum["data_classification"] == "PUBLIC_AGGREGATE"
    assert "HH-WYD" not in str(public_sum)  # No household ID in public aggregate
    assert public_sum["metrics"]["total_verified_eligible_households"] == 430
