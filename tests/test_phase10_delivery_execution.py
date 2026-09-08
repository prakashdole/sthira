"""
Tests for Phase 10: Delivery Execution, Funding Gap Calculator, Defect Clearance & External Handoff (ARC-C12 / FEAT-023, FEAT-025).
Normative Reference: plan.md (#10), trd.md (§3.11, FR-063-FR-070), rules.md (RUL-067-072), DEC-026, DEC-040, AT-18, AT-22.
"""

import pytest
from punarvas.core.enums import FundingState, RelocationPathway
from punarvas.core.errors import DefectsBlockCompletionError, UnservicedUnitHandoverError
from punarvas.modules.reconstruction.delivery_tracker import (
    DefectCategory,
    DefectSeverity,
    CaseDeliveryTracker,
)


def test_relocation_necessity_review():
    """Verify FR-063 / RUL-067: Competent necessity review before relocation."""
    tracker = CaseDeliveryTracker()
    case_id = "CASE-NECESSITY-01"

    # Review establishing necessity (in-situ mitigation not feasible due to active debris channel)
    rev = tracker.record_necessity_review(
        case_id=case_id,
        household_id="HH-WYD-001",
        in_situ_mitigation_feasible=False,
        permanent_relocation_necessary=True,
        reviewer_name="Dr. V. K. Sharma",
        reviewer_credentials="Senior Geotechnical Consultant, GSI Panel",
        reasons="Parcel directly intersects the 2024 Mundakkai debris channel; slope retention wall unfeasible.",
        uncertainty_level="LOW",
        settlement_community_effects="Whole-hamlet relocation required to preserve community cohesion.",
    )
    assert rev.permanent_relocation_necessary is True
    assert rev.in_situ_mitigation_feasible is False
    assert tracker.get_necessity_review(case_id) is not None


def test_scheme_assessment_preserves_relocation_need():
    """Verify FR-064 / RUL-068 / AT-18: Scheme ineligibility does not erase relocation need."""
    tracker = CaseDeliveryTracker()

    # Tenant household failing owner-only scheme rule
    assess = tracker.assess_household_scheme(
        household_id="HH-TENANT-001",
        tenure_category="TENANT",
        pathway=RelocationPathway.SELF_RELOCATION_ASSISTANCE,
    )
    assert assess.is_scheme_eligible is False
    # RUL-068 Invariant: relocation need remains verified and preserved!
    assert assess.relocation_need_preserved is True
    assert "TASK-ALT-TENANT-RENTAL" in assess.alternative_pathway_task


def test_funding_gap_calculator_e17():
    """Verify FR-066, FR-067 / RUL-069 / Equation E17: Funding gap calculator."""
    tracker = CaseDeliveryTracker()
    case_id = "CASE-FUNDING-01"
    required_cost = 1500000.0  # ₹15 Lakhs required

    # Announced budget: ₹10 Lakhs announced (MUST NOT reduce gap! RUL-069)
    tracker.record_funding(
        case_id=case_id,
        source_agency="CMDRF",
        cost_head="HOUSING_CONSTRUCTION",
        state=FundingState.SANCTIONED,
        amount_inr=1000000.0,
        actor_id="treasury_officer",
        is_announced_budget_only=True,
    )

    # Gap report: Announced budget does NOT reduce gap
    gap1 = tracker.calculate_funding_gap(case_id, required_cost)
    assert gap1.total_required_cost_inr == 1500000.0
    assert gap1.total_received_funds_inr == 0.0
    assert gap1.funding_gap_inr == 1500000.0
    assert gap1.is_fully_funded is False

    # Receive ₹10 Lakhs
    tracker.record_funding(
        case_id=case_id,
        source_agency="SDRF",
        cost_head="HOUSING_CONSTRUCTION",
        state=FundingState.RECEIVED,
        amount_inr=1000000.0,
        actor_id="treasury_officer",
    )

    gap2 = tracker.calculate_funding_gap(case_id, required_cost)
    assert gap2.total_received_funds_inr == 1000000.0
    assert gap2.funding_gap_inr == 500000.0  # ₹15L - ₹10L = ₹5L gap
    assert gap2.is_fully_funded is False

    # Receive remaining ₹5 Lakhs
    tracker.record_funding(
        case_id=case_id,
        source_agency="CSR_PARTNER",
        cost_head="INFRASTRUCTURE",
        state=FundingState.RECEIVED,
        amount_inr=500000.0,
        actor_id="treasury_officer",
    )

    gap3 = tracker.calculate_funding_gap(case_id, required_cost)
    assert gap3.funding_gap_inr == 0.0
    assert gap3.is_fully_funded is True


def test_defect_severity_gating_and_service_readiness():
    """Verify FR-070 / RUL-072 / AT-22: Unresolved defects and unserviced units block handover."""
    tracker = CaseDeliveryTracker()
    case_id = "CASE-DEFECT-01"

    # Step 1: Mark unit constructed
    tracker.mark_unit_constructed(case_id, "PWD Engineer")

    # Step 2: Attempting handover without verified services raises UnservicedUnitHandoverError
    with pytest.raises(UnservicedUnitHandoverError) as exc_serv:
        tracker.record_possession_handover(case_id, "Tahsildar")
    assert "Services readiness not verified" in str(exc_serv.value)

    # Step 3: Verify basic services (water 65 LPCD, electricity, road, sanitation)
    tracker.verify_services_readiness(
        case_id=case_id,
        water_supply_lpcd=65.0,
        electricity_energised=True,
        all_weather_road_functional=True,
        sanitation_drainage_functional=True,
        officer_name="KWA / PWD / KSEB joint inspection",
    )

    # Step 4: Log a CRITICAL structural defect
    d = tracker.log_defect(
        case_id=case_id,
        site_id="SITE-ELSTONE-01",
        unit_id="UNIT-A12",
        category=DefectCategory.STRUCTURAL,
        severity=DefectSeverity.CRITICAL,
        description="Foundation settlement crack in rear retaining wall",
        officer_name="Structural Engineer",
    )

    # Step 5: Handover strictly blocked by critical defect (AT-22)
    with pytest.raises(DefectsBlockCompletionError) as exc_def:
        tracker.record_possession_handover(case_id, "Tahsildar")
    assert "critical/major defects unresolved" in str(exc_def.value)

    # Step 6: Resolve the defect with evidence
    tracker.resolve_defect(
        case_id=case_id,
        defect_id=d.defect_id,
        evidence_ref="PWD-RET-WALL-RETROFIT-CERT-01",
        officer_name="Executive Engineer PWD",
    )

    # Step 7: Handover now succeeds!
    tracker.record_possession_handover(case_id, "Tahsildar")
    assert tracker._possession_handed_over[case_id] is True

    # Step 8: Beneficiary accepts offer and physically moves in
    tracker.record_offer_acceptance(case_id, "Welfare Officer")
    tracker.record_physical_occupation(case_id, "Village Officer")

    # Full completion verified
    is_complete, blockers = tracker.evaluate_relocation_completion(case_id)
    assert is_complete is True
    assert len(blockers) == 0


def test_external_system_handoff_and_followup():
    """Verify FR-068, FR-069: External handoff does not falsely report completed relocation."""
    tracker = CaseDeliveryTracker()
    case_id = "CASE-HANDOFF-01"

    # Delegate to LIFE Mission
    handoff = tracker.register_external_handoff(
        case_id=case_id,
        external_system_name="LIFE_MISSION",
        external_reference_id="LIFE-KL-2024-8842",
        accountable_agency="Local Self Government Department (LSGD)",
        accountable_officer="Joint Director, LIFE Mission",
        delegated_scope="Beneficiary-led individual house construction subsidy",
    )
    # Invariant: External handoff is NEVER reported as completed relocation (FR-069 / RUL-072)
    assert handoff.is_physically_completed is False
    assert handoff.external_system_name == "LIFE_MISSION"

    # Record 6-month longitudinal follow-up (FR-068)
    followup = tracker.record_livelihood_followup(
        case_id=case_id,
        milestone_stage="6_MONTH",
        livelihood_restored=True,
        income_restoration_pct=92.0,
        schooling_continuity=True,
        healthcare_accessible=True,
        infrastructure_rating="SATISFACTORY",
        community_satisfaction=0.9,
        officer_name="KSDMA Social Audit Team",
    )
    assert followup.milestone_stage == "6_MONTH"
    assert followup.livelihood_restored is True
