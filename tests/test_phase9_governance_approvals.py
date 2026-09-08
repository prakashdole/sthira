"""
Tests for Phase 9: Governance Approvals, Statutory Notifications, Citizen Remedies, and Multi-Resource Capacity Reservations.
Normative Reference: plan.md (#9), trd.md (§3.8, §3.11, §5.6, FR-047-FR-052, FR-070), rules.md (RUL-003-RUL-006, RUL-040, RUL-046-RUL-049, RUL-070, RUL-071), DEC-025, DEC-028, DEC-041, AT-06, AT-09, AT-10, AT-15, AT-20, AT-21.
"""

from datetime import datetime, timezone, timedelta
import pytest

from punarvas.core.contracts import GeographyScope, UserContext
from punarvas.core.enums import AuthorityState, DecisionState, RoleType
from punarvas.core.errors import (
    ApprovalConditionUnmetError,
    EntityFrozenByObjectionError,
    ReservationConflictError,
    UnauthorizedActionError,
)
from punarvas.modules.governance.objections_service import (
    ObjectionCategory,
    ObjectionFilingChannel,
    ObjectionAdmissibility,
    objections_service,
)
from punarvas.modules.governance.approval_service import (
    ApprovalCondition,
    ApprovalConditionType,
    approval_service,
)
from punarvas.modules.allocation.reservation_ledger import (
    ReservationStatus,
    capacity_ledger,
)


@pytest.fixture(autouse=True)
def reset_phase9_singletons():
    """Reset singletons before each test to maintain state isolation."""
    approval_service._approvals.clear()
    approval_service._notifications.clear()
    objections_service._cases.clear()
    objections_service._frozen_entities.clear()
    capacity_ledger._reservations.clear()
    capacity_ledger._sites.clear()
    capacity_ledger._programme_budget_inr = 50000000.0  # ₹5 Crores default


def test_objection_filing_and_receipt_token():
    """Verify FR-047 / RUL-048 / AT-09: Objection filing generates immutable receipt token with SLA."""
    actor = UserContext(
        user_id="usr_helpdesk_01",
        username="clerk_meppadi",
        roles=[RoleType.COMMUNITY_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    case = objections_service.file_objection(
        household_id="HH-WYD-OBJ-01",
        filer_name="Kunjiraman Nair",
        target_entity_type="DRAFT_BENEFICIARY_LIST",
        target_entity_id="LIST-WYD-2024-DRAFT-01",
        target_version_id="1.0",
        category=ObjectionCategory.EXCLUSION_ERROR,
        statement="Family home at Mundakkai destroyed in debris flow; omitted from draft beneficiary list.",
        assigned_officer_id="officer_revenue_01",
        assigned_officer_name="K. Ramanathan (Tahsildar)",
        actor=actor,
        filing_channel=ObjectionFilingChannel.ASSISTED_SERVICE_DESK,
        sla_days=21,
    )

    assert case.objection_id.startswith("OBJ-")
    assert case.receipt_token.startswith("RCPT-PUNARVAS-OBJ-")
    assert case.state == DecisionState.OBJECTION_FILED
    assert case.admissibility == ObjectionAdmissibility.PENDING_REVIEW
    # Verify entity is recorded as frozen
    is_frozen, reason = objections_service.check_is_entity_frozen("LIST-WYD-2024-DRAFT-01")
    assert is_frozen is True
    assert case.objection_id in reason


def test_objection_freezes_dependent_approval_and_reservation():
    """Verify FR-051 / RUL-047 / RUL-049: Active objection freezes dependent actions."""
    actor = UserContext(
        user_id="usr_helpdesk_01",
        username="clerk_meppadi",
        roles=[RoleType.COMMUNITY_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    site_id = "SITE-DISPUTED-01"
    capacity_ledger.configure_site(
        site_id=site_id,
        district="Wayanad",
        dwellings_max=50,
        land_cents_max=350.0,
        water_m3_day_max=20.0,
    )

    # Citizen files objection against the candidate site
    objections_service.file_objection(
        household_id="HH-WYD-002",
        filer_name="Amina Beevi",
        target_entity_type="CANDIDATE_SITE",
        target_entity_id=site_id,
        target_version_id="1.0",
        category=ObjectionCategory.SITE_BOUNDARY_AND_SAFETY,
        statement="Site boundary encroaches upon community burial grounds and steep stream buffer.",
        assigned_officer_id="officer_rev_02",
        assigned_officer_name="S. Mathew",
        actor=actor,
    )

    # 1. Attempt to issue official approval on frozen site must raise EntityFrozenByObjectionError
    approver = UserContext(
        user_id="usr_collector_01",
        username="collector_wayanad",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    with pytest.raises(EntityFrozenByObjectionError) as exc_info:
        approval_service.issue_official_approval(
            entity_type="SITE_SELECTION",
            entity_id=site_id,
            entity_version="1.0",
            approving_officer_name="District Collector",
            approving_officer_designation="DDMA Chairperson",
            statutory_authority_basis="DM Act 2005 §30(2)(v)",
            approval_order_number="ORDER-DDMA-2024-001",
            step_up_token="MFA-STEPUP-VALID-TOKEN",
            context=approver,
        )
    assert site_id in str(exc_info.value)

    # 2. Attempt to hold capacity reservation on frozen site must raise EntityFrozenByObjectionError
    with pytest.raises(EntityFrozenByObjectionError):
        capacity_ledger.hold_reservation(
            scenario_id="SCENARIO-A",
            site_id=site_id,
            dwellings=20,
            land_cents=140.0,
            budget_inr=10000000.0,
            water_m3_day=8.0,
            actor_id="planner_01",
        )


def test_objection_dismissal_unfreezes_entity():
    """Verify RUL-047 / FR-048: Determining objection inadmissible unfreezes entity."""
    officer = UserContext(
        user_id="usr_revenue_officer",
        username="tahsildar_meppadi",
        roles=[RoleType.REVENUE_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    entity_id = "SITE-CLEARED-01"

    case = objections_service.file_objection(
        household_id="HH-SPURIOUS-01",
        filer_name="Frivolous Objector",
        target_entity_type="SITE_SELECTION",
        target_entity_id=entity_id,
        target_version_id="1.0",
        category=ObjectionCategory.OTHER_GRIEVANCE,
        statement="Objecting without legal standing or evidence.",
        assigned_officer_id="usr_revenue_officer",
        assigned_officer_name="Tahsildar",
        actor=officer,
    )
    assert objections_service.check_is_entity_frozen(entity_id)[0] is True

    # Reject as inadmissible
    objections_service.review_admissibility(
        objection_id=case.objection_id,
        is_admissible=False,
        officer=officer,
        rejection_reason="Filer lacks locus standi; property is located in another revenue taluk.",
    )

    # Verify entity is un乘zen
    is_frozen, _ = objections_service.check_is_entity_frozen(entity_id)
    assert is_frozen is False


def test_hearing_notice_and_decision_order_remedy():
    """Verify FR-048, FR-049 / RUL-049: Formal hearing and relief granting with appeal window."""
    officer = UserContext(
        user_id="usr_revenue_officer",
        username="tahsildar_meppadi",
        roles=[RoleType.REVENUE_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    case = objections_service.file_objection(
        household_id="HH-EXCLUDE-02",
        filer_name="Balan K.",
        target_entity_type="DRAFT_BENEFICIARY_LIST",
        target_entity_id="LIST-DRAFT-02",
        target_version_id="1.0",
        category=ObjectionCategory.EXCLUSION_ERROR,
        statement="Ration card address verifies Chooralmala residence, omitted in preliminary survey.",
        assigned_officer_id=officer.user_id,
        assigned_officer_name="Tahsildar",
        actor=officer,
    )

    # 1. Admit objection
    objections_service.review_admissibility(case.objection_id, is_admissible=True, officer=officer)

    # 2. Schedule hearing
    hearing = objections_service.schedule_hearing(
        objection_id=case.objection_id,
        hearing_date=datetime.now(timezone.utc) + timedelta(days=5),
        venue="Collectorate Mini Conference Hall, Kalpetta",
        presiding_officer="Deputy Collector (Disaster Management)",
        notified_parties=["Balan K.", "Village Officer Meppadi"],
        officer=officer,
    )
    assert hearing.notice_id.startswith("NOT-HEAR-")

    # 3. Issue decision order granting relief
    order = objections_service.issue_decision_order(
        objection_id=case.objection_id,
        deciding_authority="District Magistrate / DDMA Chairperson",
        statutory_authority_basis="Disaster Management Act 2005 §30",
        relief_granted=True,
        summary_of_grounds="Physical ration card and local electoral roll confirm continuous residence prior to disaster.",
        remedy_notes="Direct inclusion of Balan K. into Verified Beneficiary Roster under Category A.",
        appeal_window_days=30,
        officer=officer,
    )
    assert order.relief_granted is True
    assert order.appeal_window_days == 30
    assert case.state == DecisionState.REMEDY_GRANTED
    # Entity should unfreeze upon resolution
    assert objections_service.check_is_entity_frozen("LIST-DRAFT-02")[0] is False


def test_overdue_objection_sla_escalation():
    """Verify FR-050 / RUL-006: Advisory escalation for overdue objections without autonomous bypass."""
    officer = UserContext(
        user_id="usr_officer_01",
        username="officer_01",
        roles=[RoleType.REVENUE_OFFICER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    case = objections_service.file_objection(
        household_id="HH-OVERDUE-01",
        filer_name="Raji Thomas",
        target_entity_type="SITE_SELECTION",
        target_entity_id="SITE-NEDUMBALA-02",
        target_version_id="1.0",
        category=ObjectionCategory.WATER_INADEQUACY,
        statement="Borewell test records indicate summer drying; site cannot support 100 families.",
        assigned_officer_id="usr_officer_01",
        assigned_officer_name="Officer",
        actor=officer,
        sla_days=10,
    )

    # Simulate passage of time past SLA deadline
    case.sla_deadline = datetime.now(timezone.utc) - timedelta(days=5)

    recs = objections_service.check_sla_escalations()
    assert len(recs) >= 1
    target_rec = next((r for r in recs if r.objection_id == case.objection_id), None)
    assert target_rec is not None
    assert target_rec.days_overdue >= 5
    assert target_rec.is_advisory is True  # RUL-006: recommendation only!


def test_official_approval_stepup_mfa_and_role_authorization():
    """Verify RUL-002, RUL-054: Approval requires Step-Up MFA and competent statutory role."""
    collector = UserContext(
        user_id="usr_collector_01",
        username="collector_wayanad",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    analyst = UserContext(
        user_id="usr_analyst_01",
        username="analyst_01",
        roles=[RoleType.GIS_ANALYST],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    # 1. Missing Step-Up Token raises UnauthorizedActionError
    with pytest.raises(UnauthorizedActionError) as exc1:
        approval_service.issue_official_approval(
            entity_type="SITE_SELECTION",
            entity_id="SITE-ELSTONE-SAFE",
            entity_version="1.0",
            approving_officer_name="Collector",
            approving_officer_designation="Chairperson",
            statutory_authority_basis="DM Act §30",
            approval_order_number="ORD-001",
            step_up_token=None,  # Missing!
            context=collector,
        )
    assert "step-up" in str(exc1.value)

    # 2. Non-approver role raises UnauthorizedActionError
    with pytest.raises(UnauthorizedActionError) as exc2:
        approval_service.issue_official_approval(
            entity_type="SITE_SELECTION",
            entity_id="SITE-ELSTONE-SAFE",
            entity_version="1.0",
            approving_officer_name="GIS Analyst",
            approving_officer_designation="Analyst",
            statutory_authority_basis="DM Act §30",
            approval_order_number="ORD-002",
            step_up_token="MFA-STEPUP-TOKEN-12345",
            context=analyst,  # Lacks authority!
        )
    assert "statutory authority" in str(exc2.value)

    # 3. Valid Step-Up Token and Approver succeeds
    approval = approval_service.issue_official_approval(
        entity_type="SITE_SELECTION",
        entity_id="SITE-ELSTONE-SAFE",
        entity_version="1.0",
        approving_officer_name="Dr. Meghashree IAS",
        approving_officer_designation="District Collector & DDMA Chairperson",
        statutory_authority_basis="Disaster Management Act 2005 §30(2)(v)",
        approval_order_number="G.O.(Ms) No. 44/2024/DMD",
        step_up_token="MFA-STEPUP-VERIFIED-TOKEN-999",
        context=collector,
    )
    assert approval.approval_id.startswith("APP-SITE-")
    assert approval.authority_state == AuthorityState.OFFICIALLY_APPROVED


def test_conditional_approval_blocking_gates():
    """Verify RUL-004 / AT-06 / AT-20: Blocking conditions gate capacity commitment."""
    collector = UserContext(
        user_id="usr_collector_01",
        username="collector_wayanad",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )
    site_id = "SITE-CONDITIONAL-01"
    capacity_ledger.configure_site(
        site_id=site_id,
        district="Wayanad",
        dwellings_max=100,
        land_cents_max=700.0,
        water_m3_day_max=50.0,
    )

    cond_water = ApprovalCondition(
        condition_id="COND-WATER-01",
        condition_type=ApprovalConditionType.WATER_YIELD_VERIFICATION,
        description="Formal dry-season pump test confirming 55 LPCD yield across 100 units.",
        is_blocking_for_allocation=True,  # Blocking!
    )
    cond_cosmetic = ApprovalCondition(
        condition_id="COND-LANDSCAPE-02",
        condition_type=ApprovalConditionType.BUDGET_SANCTION_CONFIRMATION,
        description="Landscape plantation plan submission.",
        is_blocking_for_allocation=False,  # Non-blocking
    )

    approval = approval_service.issue_official_approval(
        entity_type="SITE_SELECTION",
        entity_id=site_id,
        entity_version="1.0",
        approving_officer_name="Collector",
        approving_officer_designation="DDMA Chairperson",
        statutory_authority_basis="DM Act §30",
        approval_order_number="ORD-COND-01",
        conditions=[cond_water, cond_cosmetic],
        step_up_token="MFA-STEPUP-TOKEN-12345",
        context=collector,
    )

    # 1. Check can allocate fails due to unmet blocking condition
    can_alloc, reasons = approval_service.check_can_allocate(approval.approval_id)
    assert can_alloc is False
    assert any("WATER_YIELD_VERIFICATION" in r for r in reasons)

    # 2. Capacity ledger commit must fail with ApprovalConditionUnmetError
    hold = capacity_ledger.hold_reservation(
        scenario_id="SCEN-01",
        site_id=site_id,
        dwellings=40,
        land_cents=280.0,
        budget_inr=20000000.0,
        water_m3_day=15.0,
        actor_id="planner_01",
    )
    with pytest.raises(ApprovalConditionUnmetError):
        capacity_ledger.commit_reservation(
            reservation_id=hold.reservation_id,
            approval_id=approval.approval_id,
            actor_id="collector_wayanad",
        )

    # 3. Satisfy the blocking condition
    approval_service.satisfy_condition(
        approval_id=approval.approval_id,
        condition_id="COND-WATER-01",
        verification_doc_hash="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        officer_name="Executive Engineer, KWA",
        context=collector,
    )

    can_alloc_after, _ = approval_service.check_can_allocate(approval.approval_id)
    assert can_alloc_after is True

    # 4. Capacity ledger commit now succeeds
    committed = capacity_ledger.commit_reservation(
        reservation_id=hold.reservation_id,
        approval_id=approval.approval_id,
        actor_id="collector_wayanad",
    )
    assert committed.status == ReservationStatus.COMMITTED
    assert committed.approval_order_id == approval.approval_id


def test_statutory_notification_distinct_from_approval():
    """Verify RUL-003 / AT-21: Official notification is a separate legal act from approval."""
    collector = UserContext(
        user_id="usr_collector_01",
        username="collector_wayanad",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    approval = approval_service.issue_official_approval(
        entity_type="RELOCATION_PROGRAMME",
        entity_id="PRG-KL-WYD-2024",
        entity_version="1.0",
        approving_officer_name="Chief Secretary, Govt of Kerala",
        approving_officer_designation="State Executive Committee Chairperson",
        statutory_authority_basis="Disaster Management Act 2005 §22",
        approval_order_number="G.O.(P) No. 12/2024/DMD",
        step_up_token="MFA-STEPUP-TOKEN-12345",
        context=collector,
    )
    assert approval.authority_state == AuthorityState.OFFICIALLY_APPROVED
    assert approval.statutory_notification_id is None

    # Separate statutory publication act
    effective_dt = datetime.now(timezone.utc) + timedelta(days=1)
    notif = approval_service.publish_statutory_notification(
        approval_id=approval.approval_id,
        gazette_notification_number="KL-WYD-EXT-2024-8891",
        gazette_volume_number="Vol. XIII, No. 2451",
        effective_date=effective_dt,
        notification_title_en="Statutory Notification of Wayanad Landslide Relocation Zone",
        notification_title_ml="വയനാട് ഉരുൾപൊട്ടൽ ശാശ്വത പുനരധിവാസ വിജ്ഞാപനം",
        notification_text_en="In exercise of powers conferred under Section 30 of the DM Act...",
        notification_text_ml="ദുരന്ത നിവാരണ നിയമം 30-ാം വകുപ്പ് പ്രകാരം പ്രഖ്യാപിക്കുന്നു...",
        issuing_authority="Government of Kerala (Disaster Management Department)",
        signing_officer_name="Additional Chief Secretary",
        digital_signature_hash="sha256-sig-abcdef0123456789",
        context=collector,
    )

    assert notif.notification_id.startswith("NOTIF-GAZ-")
    assert approval.authority_state == AuthorityState.OFFICIALLY_NOTIFIED
    assert approval.statutory_notification_id == notif.notification_id


def test_supersession_workflow_preserving_history():
    """Verify RUL-005 / AT-10: Superseding an approval creates new version preserving prior audit trail."""
    collector = UserContext(
        user_id="usr_collector_01",
        username="collector_wayanad",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=GeographyScope(state="Kerala", district="Wayanad"),
    )

    # Approval Version 1
    app_v1 = approval_service.issue_official_approval(
        entity_type="ALLOCATION_SCENARIO",
        entity_id="SCENARIO-WYD-ELSTONE",
        entity_version="1.0",
        approving_officer_name="Collector",
        approving_officer_designation="DDMA Chairperson",
        statutory_authority_basis="DM Act §30",
        approval_order_number="ORDER-V1",
        step_up_token="MFA-STEPUP-TOKEN-12345",
        context=collector,
    )
    assert app_v1.is_superseded is False

    # Approval Version 2 superseding Version 1
    app_v2 = approval_service.issue_official_approval(
        entity_type="ALLOCATION_SCENARIO",
        entity_id="SCENARIO-WYD-ELSTONE",
        entity_version="2.0",
        approving_officer_name="Collector",
        approving_officer_designation="DDMA Chairperson",
        statutory_authority_basis="DM Act §30",
        approval_order_number="ORDER-V2-REVISED",
        supersedes_approval_id=app_v1.approval_id,
        step_up_token="MFA-STEPUP-TOKEN-12345",
        context=collector,
    )

    assert app_v1.is_superseded is True
    assert app_v1.superseded_by_approval_id == app_v2.approval_id
    assert app_v2.supersedes_approval_id == app_v1.approval_id

    # v1 can no longer allocate
    can_alloc, reasons = approval_service.check_can_allocate(app_v1.approval_id)
    assert can_alloc is False
    assert "superseded" in reasons[0]


def test_multi_resource_capacity_reservation_and_race_fail_closed():
    """Verify RUL-070, RUL-071 / FR-070 / AT-15: Multi-resource atomic reservation and race collision handling."""
    site_id = "SITE-ELSTONE-TEST"
    capacity_ledger.configure_site(
        site_id=site_id,
        district="Wayanad",
        dwellings_max=100,
        land_cents_max=700.0,
        water_m3_day_max=55.0,
    )

    # 1. Draft simulation reserves ZERO capacity (RUL-070)
    sim = capacity_ledger.simulate_draft_scenario(
        scenario_id="SCEN-DRAFT-01",
        site_id=site_id,
        dwellings=80,
        land_cents=560.0,
        budget_inr=40000000.0,
        water_m3_day=44.0,
        actor_id="planner_sim",
    )
    assert sim.dwellings_reserved == 0
    rem = capacity_ledger.get_remaining_capacity(site_id)
    assert rem["dwellings_remaining"] == 100.0

    # 2. Hold Reservation A for 70 units
    hold_a = capacity_ledger.hold_reservation(
        scenario_id="SCEN-A",
        site_id=site_id,
        dwellings=70,
        land_cents=490.0,
        budget_inr=35000000.0,
        water_m3_day=38.5,
        actor_id="planner_a",
    )
    rem_after_a = capacity_ledger.get_remaining_capacity(site_id)
    assert rem_after_a["dwellings_remaining"] == 30.0

    # 3. Competing Reservation B requests 40 units -> fails closed with ReservationConflictError
    with pytest.raises(ReservationConflictError) as exc_info:
        capacity_ledger.hold_reservation(
            scenario_id="SCEN-B",
            site_id=site_id,
            dwellings=40,  # 70 + 40 = 110 > 100!
            land_cents=280.0,
            budget_inr=20000000.0,
            water_m3_day=22.0,
            actor_id="planner_b",
        )
    assert "DWELLING_CAPACITY" in str(exc_info.value)

    # 4. Explicitly release Reservation A -> returns capacity without double-counting (RUL-071)
    capacity_ledger.release_reservation(
        reservation_id=hold_a.reservation_id,
        reason="Scenario A rejected by DDMA in favour of Scenario B.",
        actor_id="collector_wayanad",
    )
    rem_after_rel = capacity_ledger.get_remaining_capacity(site_id)
    assert rem_after_rel["dwellings_remaining"] == 100.0

    # 5. Reservation B can now succeed
    hold_b = capacity_ledger.hold_reservation(
        scenario_id="SCEN-B",
        site_id=site_id,
        dwellings=40,
        land_cents=280.0,
        budget_inr=20000000.0,
        water_m3_day=22.0,
        actor_id="planner_b",
    )
    assert hold_b.status == ReservationStatus.RESERVED
    assert capacity_ledger.get_remaining_capacity(site_id)["dwellings_remaining"] == 60.0
