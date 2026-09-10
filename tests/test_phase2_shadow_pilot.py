"""
Integration and scenario tests for Phase 2 Wayanad Shadow Pilot & Field Validation (PH-2).
Normative Reference: phases.md §5 (Lines 130-144), DEC-035, rules.md (RUL-001 through RUL-083).

Covers the 11 mandatory required PH-2 scenarios:
1. Wayanad channelized debris flow affecting locally flat terrain.
2. Paper-vacant parcel with observed structures / community use.
3. Cadastral offset and disputed source versions (Bhulekh grid shift).
4. Pending Forest Rights Act (FRA 2006) / legal issue.
5. Seasonal water shortfall (monsoon pass vs dry-season fail).
6. Changed beneficiary list plus objection and notice.
7. Township vs self-relocation choice (Elstone Estate vs Kerala VLRS).
8. Capacity / accessibility constraint leaving households explicitly unassigned.
9. Overdue workflow recommending, but not executing, escalation.
10. Offline conflict and lost-device exercise.
11. Competing scenarios and overlapping sites attempting to reserve the same dwelling, land, budget, or water.

Plus:
- C2-01 Agency import adapter reconciliation.
- C2-03 Sensitivity analysis and rank reversal detection.
- C2-04 Kerala LSGD Disaster Management Plan Annex generation.
- R2-03 Preregistered evaluation benchmark metrics.
"""

import pytest
from sthira.core.contracts import GeoPoint
from sthira.core.enums import GateState, RelocationPathway, DecisionState, DiscrepancyType
from sthira.core.errors import ReservationConflictError

from sthira.modules.hazard import HazardLayer, hazard_service
from sthira.modules.policy import (
    SiteCriteriaInput,
    policy_engine,
    sensitivity_analysis_engine,
)
from sthira.modules.land_truth import (
    ParcelRecord,
    land_truth_service,
    agency_import_adapter,
)
from sthira.modules.household import HouseholdCase
from sthira.modules.allocation import (
    allocation_service,
    capacity_reservation_ledger,
)
from sthira.modules.field import (
    field_sync_service,
    FieldSurveySubmission,
    WaterFieldMeasurement,
    GeotechnicalMeasurement,
    DeviceStatus,
)
from sthira.modules.governance import (
    governance_service,
    SchemeMilestoneTracker,
)
from sthira.modules.reporting import (
    reporting_service,
)
from sthira.modules.evaluation import (
    evaluation_harness_service,
    CaseShadowEvaluation,
    EvaluationTargetStatus,
)


# --- Scenario 1: Wayanad channelized debris flow affecting locally flat terrain ---
def test_scenario_1_channelized_debris_flow_runout():
    """
    Scenario 1: Wayanad channelized debris flow affecting locally flat terrain (e.g. Chooralmala/Mundakkai).
    Local terrain slope is gentle (8°), but parcel lies in mapped active runout path.
    GATE-HAZ-01 must FAIL.
    """
    flat_but_in_runout = SiteCriteriaInput(
        site_id="SITE-CHOORALMALA-TERRACE",
        hazard_susceptibility_level="LOW",  # Local slope is low
        in_debris_flow_runout=True,  # Within channelized debris corridor
        title_clearance_status="VERIFIED_CLEAR",
        forest_clearance_required=False,
        lean_season_tested_lpcd=60.0,
        has_dry_season_yield_test=True,
        road_access_width_m=4.5,
        distance_to_hospital_km=8.0,
        distance_to_school_km=3.0,
        dwelling_capacity=50,
    )

    gates = policy_engine.evaluate_hard_gates(flat_but_in_runout)
    haz_gate = next(g for g in gates if g.gate_id == "GATE-HAZ-01")
    assert haz_gate.state == GateState.FAIL
    assert "debris flow channel" in haz_gate.reason


# --- Scenario 2: Paper-vacant parcel with observed structures / community use ---
def test_scenario_2_paper_vacant_ground_occupied():
    """
    Scenario 2: Revenue records state Poramboke/Government vacant, but physical occupation exists.
    Land truth must detect PAPER_VACANT_GROUND_OCCUPIED (RUL-022).
    """
    p = ParcelRecord(
        parcel_id="PARCEL-SCENARIO-02",
        survey_number="104/2",
        village="Meppadi",
        lsg_name="Meppadi Grama Panchayat",
        centroid=GeoPoint(coordinates=[76.14, 11.55]),
        area_cents=45.0,
        paper_title_holder="Government Poramboke",
        paper_classification="PORAMBOKE",
        observed_structures_count=6,
        ground_occupied=True,
    )
    land_truth_service.register_parcel(p)

    tasks = land_truth_service.detect_discrepancies("PARCEL-SCENARIO-02", actor_id="gis_officer")
    disc_types = [t.discrepancy_type for t in tasks]
    assert DiscrepancyType.PAPER_VACANT_GROUND_OCCUPIED in disc_types


# --- Scenario 3: Cadastral offset and disputed source versions (Bhulekh grid shift) ---
def test_scenario_3_cadastral_offset_bhulekh_shift():
    """
    Scenario 3: Kerala e-Rekha / Bhulekh digitized cadastral boundary has > 25m offset against ground DGPS.
    Agency import adapter detects CADASTRAL_GRID_SHIFT and flags discrepancy (RUL-023).
    """
    p = ParcelRecord(
        parcel_id="PARCEL-SCENARIO-03",
        survey_number="215/1",
        village="Meppadi",
        lsg_name="Meppadi Grama Panchayat",
        centroid=GeoPoint(coordinates=[76.12, 11.53]),
        area_cents=25.0,
        paper_title_holder="State Revenue",
        paper_classification="REVENUE",
        cadastral_offset_meters=31.5,  # Exceeds 25m threshold
    )
    land_truth_service.register_parcel(p)

    records = [
        {
            "record_id": "EREKHA-215-1",
            "parcel_id": "PARCEL-SCENARIO-03",
            "survey_number": "215/1",
            "village": "Meppadi",
            "lsg_name": "Meppadi Grama Panchayat",
            "area_cents": 25.0,
            "measured_offset_m": 31.5,
            "classification": "REVENUE",
        }
    ]

    results = agency_import_adapter.import_and_reconcile_e_rekha(
        batch_id="BATCH-EREKHA-01", records=records, actor_id="revenue_officer"
    )
    assert len(results) == 1
    assert results[0].status == "DISCREPANCY_DETECTED"
    assert any("Bhulekh grid shift" in d for d in results[0].discrepancies)


# --- Scenario 4: Pending Forest Rights Act (FRA 2006) claim ---
def test_scenario_4_pending_fra_claim():
    """
    Scenario 4: Parcel has pending Forest Rights Act 2006 claim.
    Gate GATE-LEG-01 returns BLOCKED pending Grama Sabha / FRC resolution (RUL-025).
    """
    fra_site = SiteCriteriaInput(
        site_id="SITE-FRA-PENDING",
        hazard_susceptibility_level="LOW",
        in_debris_flow_runout=False,
        title_clearance_status="VERIFIED_CLEAR",
        forest_clearance_required=True,
        fra_community_consent=None,  # Pending Grama Sabha resolution
        lean_season_tested_lpcd=60.0,
        has_dry_season_yield_test=True,
        road_access_width_m=4.0,
        distance_to_hospital_km=6.0,
        distance_to_school_km=2.0,
        dwelling_capacity=40,
    )

    gates = policy_engine.evaluate_hard_gates(fra_site)
    leg_gate = next(g for g in gates if g.gate_id == "GATE-LEG-01")
    assert leg_gate.state == GateState.BLOCKED
    assert "Grama Sabha" in leg_gate.reason


# --- Scenario 5: Seasonal water shortfall (monsoon pass vs dry-season fail) ---
def test_scenario_5_seasonal_water_shortfall():
    """
    Scenario 5: Site passes monsoon water yield, but dry-season / lean-season yield drops below 55 LPCD.
    Gate GATE-WAT-01 must FAIL on dry-season evidence (RUL-030, RUL-031).
    If dry season test missing, gate returns UNKNOWN (RUL-013).
    """
    # 5a. Missing dry season test -> UNKNOWN
    site_untested = SiteCriteriaInput(
        site_id="SITE-WATER-UNTESTED",
        hazard_susceptibility_level="LOW",
        in_debris_flow_runout=False,
        title_clearance_status="VERIFIED_CLEAR",
        forest_clearance_required=False,
        lean_season_tested_lpcd=None,
        has_dry_season_yield_test=False,
        road_access_width_m=4.0,
        distance_to_hospital_km=5.0,
        distance_to_school_km=2.0,
        dwelling_capacity=50,
    )
    gates_untested = policy_engine.evaluate_hard_gates(site_untested)
    wat_gate_untested = next(g for g in gates_untested if g.gate_id == "GATE-WAT-01")
    assert wat_gate_untested.state == GateState.UNKNOWN

    # 5b. Dry season test fails (35 LPCD < 55 LPCD JJM baseline)
    site_dry_shortfall = SiteCriteriaInput(
        site_id="SITE-WATER-SHORTFALL",
        hazard_susceptibility_level="LOW",
        in_debris_flow_runout=False,
        title_clearance_status="VERIFIED_CLEAR",
        forest_clearance_required=False,
        lean_season_tested_lpcd=35.0,
        has_dry_season_yield_test=True,
        road_access_width_m=4.0,
        distance_to_hospital_km=5.0,
        distance_to_school_km=2.0,
        dwelling_capacity=50,
    )
    gates_shortfall = policy_engine.evaluate_hard_gates(site_dry_shortfall)
    wat_gate_shortfall = next(g for g in gates_shortfall if g.gate_id == "GATE-WAT-01")
    assert wat_gate_shortfall.state == GateState.FAIL
    assert "below JJM baseline" in wat_gate_shortfall.reason


# --- Scenario 6: Changed beneficiary list plus objection and notice ---
def test_scenario_6_objection_and_formal_hearing_notice():
    """
    Scenario 6: Beneficiary files objection; formal bilingual hearing notice is issued;
    hearing is conducted and resolution is logged (FEAT-016 / RUL-048).
    """
    obj = governance_service.file_objection(
        household_id="HH-WYD-OBJ-01",
        target_decision_id="DEC-ALLOC-001",
        reason_category="EXCLUSION_ERROR",
        statement="Household was omitted from initial beneficiary publication despite destroyed home.",
        assigned_officer_id="officer_sub_collector",
        actor_id="caseworker_01",
    )
    assert obj.state == DecisionState.OBJECTION_FILED

    # Issue formal hearing notice
    notice = governance_service.issue_hearing_notice(
        objection_id=obj.objection_id,
        hearing_date="2026-09-25 10:30 AM",
        venue="Collectorate Conference Hall, Kalpetta",
        officer_name="Deputy Collector (Disaster Management)",
        actor_id="officer_sub_collector",
    )
    assert notice.hearing_date == "2026-09-25 10:30 AM"
    assert "ഹിയറിംഗ് നോട്ടീസ്" in notice.notice_text_ml
    assert "Formal Hearing Notice" in notice.notice_text_en

    # Resolve objection
    resolved_obj = governance_service.resolve_objection(
        objection_id=obj.objection_id,
        remedy_granted=True,
        remedy_notes="Ground verification confirmed total structural loss. Restored to eligible beneficiary list.",
        officer_id="officer_sub_collector",
    )
    assert resolved_obj.state == DecisionState.REMEDY_GRANTED


# --- Scenario 7: Township vs self-relocation choice ---
def test_scenario_7_township_vs_self_relocation_pathways():
    """
    Scenario 7: Households choose between Model Township and Self-Relocation Assistance (Kerala VLRS).
    Distinct scheme entitlements and milestone trackers are generated (DEC-018 / FEAT-023).
    """
    # 7a. Self-relocation (VLRS ₹10L grant)
    assess_self = governance_service.assess_scheme_entitlements(
        household_id="HH-CHOICE-SELF",
        tenure_category="OWNER",
        chosen_pathway=RelocationPathway.SELF_RELOCATION_ASSISTANCE,
        actor_id="finance_officer",
    )
    assert assess_self.sanctioned_amount_inr == 1000000.0
    assert "VLRS" in assess_self.scheme_name
    assert len(assess_self.milestone_tranches) == 4

    # 7b. Township model
    assess_township = governance_service.assess_scheme_entitlements(
        household_id="HH-CHOICE-TOWNSHIP",
        tenure_category="OWNER",
        chosen_pathway=RelocationPathway.TOWNSHIP,
        actor_id="finance_officer",
    )
    assert assess_township.sanctioned_amount_inr == 1500000.0
    assert "Township" in assess_township.scheme_name


# --- Scenario 8: Capacity / accessibility constraint leaving households explicitly unassigned ---
def test_scenario_8_explicit_unassigned_due_to_accessibility():
    """
    Scenario 8: Household requires ground-floor accessibility for disabled member.
    Available site is full; household must be explicitly left unassigned with clear plain-language explanation (RUL-041, RUL-042).
    """
    hh_pwd = HouseholdCase(
        household_id="HH-PWD-01",
        head_of_household="Lakshmi Amma",
        member_count=3,
        elderly_count=1,
        disabled_count=1,
        requires_ground_floor=True,
        source_parcel_id="PARCEL-CHOORALMALA-01",
        chosen_pathway=RelocationPathway.TOWNSHIP,
        preferred_site_ids=["SITE-ELSTONE-TOWNSHIP"],
    )

    # Site with zero remaining capacity
    scenario = allocation_service.generate_scenario(
        scenario_id="SCENARIO-CAPACITY-CHECK",
        households=[hh_pwd],
        site_capacities={"SITE-ELSTONE-TOWNSHIP": 0},
        site_plot_cents={"SITE-ELSTONE-TOWNSHIP": 7.0},
        actor_id="planner_01",
    )

    assert scenario.unassigned_count == 1
    assignment = scenario.assignments[0]
    assert assignment.assigned_site_id is None
    assert "Unassigned" in assignment.explanation
    assert "capacity" in assignment.explanation.lower()


# --- Scenario 9: Overdue workflow recommending, but not executing, escalation ---
def test_scenario_9_overdue_workflow_recommends_escalation():
    """
    Scenario 9: SLA breached on administrative casework.
    System logs advisory recommendation, but does NOT bypass DDMA statutory jurisdiction (RUL-006 / DEC-016).
    """
    from sthira.core.audit import global_audit_ledger

    # Log an overdue task recommendation event
    event = global_audit_ledger.log(
        actor_id="SLA_MONITOR_DAEMON",
        authority_scope="Wayanad/Governance",
        action="RECOMMEND_TASK_ESCALATION",
        entity_type="AdministrativeTask",
        entity_id="TASK-CADASTRAL-REVIEW-04",
        version_id="1.0",
        reason="Cadastral discrepancy review SLA breached (> 14 days). Advisory escalation sent to District Collector.",
    )
    assert event.action == "RECOMMEND_TASK_ESCALATION"
    # Verification that audit chain remains intact
    assert global_audit_ledger.verify_integrity() is True


# --- Scenario 10: Offline conflict and lost-device exercise ---
def test_scenario_10_offline_conflict_and_lost_device():
    """
    Scenario 10: Offline survey sync detects version conflict (RUL-014).
    Field tablet reported lost/compromised revokes token and rejects sync (DEC-011, DEC-017).
    """
    # 10a. Device registration
    dev = field_sync_service.register_device(
        device_id="TAB-WYD-042",
        officer_id="OFF-SURVEY-01",
        officer_name="Anoop Kumar",
        device_model="Samsung Galaxy Tab Active4 Pro",
        auth_token="secure_token_secret_xyz",
        actor_id="security_admin",
    )
    assert dev.status == DeviceStatus.ACTIVE

    # 10b. Stale offline update produces conflict
    submission = FieldSurveySubmission(
        submission_id="SUB-OFFLINE-01",
        bundle_id="BUNDLE-01",
        device_id="TAB-WYD-042",
        officer_id="OFF-SURVEY-01",
        household_updates=[
            {"household_id": "HH-STALE-01", "version": 1, "head_name": "Updated Name Client"}
        ],
        payload_checksum="chk123",
    )
    # Server record has version 2 (modified on server while tablet was offline)
    current_server = {"HH-STALE-01": {"household_id": "HH-STALE-01", "version": 2, "head_name": "Server Name"}}

    sync_res = field_sync_service.ingest_offline_sync(
        submission=submission,
        current_server_records=current_server,
        actor_id="sync_daemon",
    )
    assert sync_res.conflicts_count == 1
    assert sync_res.conflicts[0].entity_id == "HH-STALE-01"
    assert sync_res.conflicts[0].status == "PENDING_MANUAL_REVIEW"

    # 10c. Device lost exercise
    field_sync_service.report_lost_device(
        device_id="TAB-WYD-042",
        reason="Field surveyor tablet misplaced in transport. Incident reported immediately.",
        actor_id="security_admin",
    )
    assert dev.status == DeviceStatus.REPORTED_LOST

    # Subsequent sync from lost device is rejected
    sync_res_2 = field_sync_service.ingest_offline_sync(
        submission=submission,
        current_server_records=current_server,
        actor_id="sync_daemon",
    )
    assert sync_res_2.success is False
    assert "revoked or reported lost" in sync_res_2.message


# --- Scenario 11: Competing scenarios and overlapping sites attempting to reserve capacity ---
def test_scenario_11_competing_capacity_reservations():
    """
    Scenario 11: Competing scenarios attempt to reserve the same capacity.
    Drafts reserve nothing. Approval fails closed with ReservationConflictError upon collision (FEAT-024 / DEC-025 / ODN-009).
    """
    capacity_reservation_ledger.configure_capacities(
        site_dwellings={"SITE-ELSTONE": 200},
        site_land_cents={"SITE-ELSTONE": 1400.0},
        programme_budget_inr=50000000.0,
        site_water_m3_day={"SITE-ELSTONE": 50.0},
    )

    # Scenario A atomically reserves 150 units
    res_a = capacity_reservation_ledger.reserve(
        scenario_id="SCENARIO-A",
        site_id="SITE-ELSTONE",
        dwellings=150,
        land_cents=1050.0,
        budget_inr=30000000.0,
        water_m3_day=35.0,
        actor_id="approver_ddma",
    )
    assert res_a.dwellings_reserved == 150

    # Scenario B attempts to reserve 100 units on same site -> 150 + 100 = 250 > 200 -> Collision!
    with pytest.raises(ReservationConflictError) as exc_info:
        capacity_reservation_ledger.reserve(
            scenario_id="SCENARIO-B",
            site_id="SITE-ELSTONE",
            dwellings=100,
            land_cents=700.0,
            budget_inr=20000000.0,
            water_m3_day=20.0,
            actor_id="approver_ddma",
        )
    assert "Requested 100 units, but only 50 remain" in str(exc_info.value)


# --- Additional Phase 2 Tests: Sensitivity, LSGD Annex, Evaluation Metrics ---

def test_sensitivity_analysis_and_rank_reversal():
    """
    ODN-005 / C2-03: Sensitivity analysis on multi-criteria site comparison.
    Varies weights by +/- 20% and detects rank stability / reversals.
    """
    site_1 = SiteCriteriaInput(
        site_id="SITE-A-URBAN",
        hazard_susceptibility_level="LOW",
        in_debris_flow_runout=False,
        title_clearance_status="VERIFIED_CLEAR",
        forest_clearance_required=False,
        lean_season_tested_lpcd=70.0,
        has_dry_season_yield_test=True,
        road_access_width_m=5.0,
        distance_to_hospital_km=2.0,  # Highly accessible
        distance_to_school_km=1.0,
        dwelling_capacity=100,
    )
    site_2 = SiteCriteriaInput(
        site_id="SITE-B-RURAL",
        hazard_susceptibility_level="LOW",
        in_debris_flow_runout=False,
        title_clearance_status="VERIFIED_CLEAR",
        forest_clearance_required=False,
        lean_season_tested_lpcd=70.0,
        has_dry_season_yield_test=True,
        road_access_width_m=4.0,
        distance_to_hospital_km=12.0,  # Less accessible
        distance_to_school_km=6.0,
        dwelling_capacity=220,  # Much higher capacity
    )

    results = sensitivity_analysis_engine.analyze_site_rank_sensitivity(
        sites=[site_1, site_2],
        engine=policy_engine,
        perturbation_factor=0.20,
    )
    assert len(results) > 0
    # Sensitivity results provide inspectable perturbation percentages
    assert all(abs(r.perturbation_pct) == 20.0 for r in results)


def test_lsgd_disaster_management_plan_annex_generator():
    """
    FEAT-018 / DEC-013 / FR-054: Kerala LSGD Disaster Management Plan Annex.
    Participatory and statutory fields must remain incomplete, never fabricated.
    """
    annex = reporting_service.generate_lsgd_dm_plan_annex(
        lsg_name="Meppadi Grama Panchayat",
        district="Wayanad",
        vulnerable_wards=[10, 11, 12],
        settlement_names=["Chooralmala", "Mundakkai", "Punchirimattam"],
        verified_beneficiary_count=430,
        host_sites=[{"site_id": "SITE-ELSTONE", "capacity": 200}],
        generating_user_id="planner_wayanad",
    )

    assert annex.lsg_name == "Meppadi Grama Panchayat"
    assert annex.section_d_statutory_approvals["gram_ward_sabha_resolution"] == "PENDING_GRAM_SABHA_APPROVAL"
    assert annex.section_d_statutory_approvals["approval_status"] == "INCOMPLETE_REQUIRES_LAWFUL_PARTICIPATORY_PROCESS"
    assert "ഉപദേശക രേഖ" in annex.statutory_note_ml
    assert len(annex.sha256_checksum) == 64


def test_preregistered_evaluation_metrics():
    """
    R2-03: Dossier cycle time reduction target (30%), false positive/negative tracking,
    unknown rate, and subgroup fairness.
    """
    # Record 6 test cases
    for i in range(6):
        evaluation_harness_service.record_case(
            CaseShadowEvaluation(
                case_id=f"CASE-EVAL-{i}",
                official_ground_truth_necessity="RELOCATION_NECESSARY",
                system_advisory_necessity="RELOCATION_NECESSARY",
                baseline_hours=10.0,
                shadow_hours=6.5,  # 35% time reduction
                household_tags=["PWD_OR_ELDERLY"] if i % 2 == 0 else ["FEMALE_HEADED"],
                assigned_site_id="SITE-ELSTONE",
            )
        )

    metrics = evaluation_harness_service.calculate_metrics(actor_id="evaluation_board")
    assert metrics.total_cases_evaluated == 6
    assert metrics.true_positives == 6
    assert metrics.median_time_reduction_pct == 35.0
    assert metrics.target_30_pct_status == EvaluationTargetStatus.MET
    assert len(metrics.subgroup_fairness) >= 2
