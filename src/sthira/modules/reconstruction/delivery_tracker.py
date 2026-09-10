"""
Sthira Delivery Execution, Funding Gap Calculator, Defect Clearance & External Handoff Engine (Phase 10 / ARC-C12 / FEAT-023, FEAT-025).
Normative Reference: plan.md (#10), trd.md (§3.11, FR-063-FR-070), rules.md (RUL-067, RUL-068, RUL-069, RUL-072), DEC-026, AT-18, AT-22.
"""

from datetime import datetime, timezone, timedelta
from enum import Enum
from typing import Any, Dict, List, Optional, Tuple
from uuid import uuid4
from pydantic import BaseModel, Field

from sthira.core.audit import global_audit_ledger
from sthira.core.contracts import utc_now, UserContext
from sthira.core.enums import AuthorityState, FundingState, RelocationPathway, RoleType
from sthira.core.errors import (
    DefectsBlockCompletionError,
    UnservicedUnitHandoverError,
)


class DefectSeverity(str, Enum):
    CRITICAL = "CRITICAL"  # Life safety, water contamination, structural instability: strictly blocks handover
    MAJOR = "MAJOR"        # Severe leak, road access blockage, power failure: blocks occupation
    MINOR = "MINOR"        # Cosmetic finishes, paint touch-up: does not block occupation with warranty


class DefectCategory(str, Enum):
    STRUCTURAL = "STRUCTURAL"
    WATER_SUPPLY = "WATER_SUPPLY"
    ELECTRICAL = "ELECTRICAL"
    ACCESS_ROAD = "ACCESS_ROAD"
    DRAINAGE_SANITATION = "DRAINAGE_SANITATION"
    FINISHES = "FINISHES"


class DefectRecord(BaseModel):
    defect_id: str
    case_id: str
    site_id: str
    unit_id: str
    category: DefectCategory
    severity: DefectSeverity
    description: str
    is_resolved: bool = False
    logged_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    resolved_at: Optional[datetime] = None
    resolution_evidence_ref: Optional[str] = None
    logged_by_officer: str
    verified_by_officer: Optional[str] = None


class RelocationNecessityReview(BaseModel):
    """
    FR-063 / RUL-067: Competent assessment of permanent relocation necessity
    versus feasible in-situ risk reduction alternatives.
    """
    review_id: str
    case_id: str
    household_id: str
    in_situ_mitigation_feasible: bool
    in_situ_mitigation_description: Optional[str] = None
    estimated_in_situ_cost_inr: Optional[float] = None
    permanent_relocation_necessary: bool
    competent_reviewer_name: str
    competent_reviewer_credentials: str
    reasons: str
    uncertainty_level: str  # LOW, MEDIUM, HIGH
    settlement_community_effects: str
    authority_state: AuthorityState = AuthorityState.FIELD_VERIFIED
    reviewed_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    audit_event_id: str


class HouseholdSchemeAssessment(BaseModel):
    """
    FR-064, FR-065 / RUL-068, AT-18: Scheme-specific eligibility assessment.
    Preserves relocation need independently from scheme eligibility.
    """
    assessment_id: str
    household_id: str
    tenure_category: str  # OWNER, TENANT, LANDLESS
    relocation_need_verified: bool = True
    relocation_need_preserved: bool = True  # Invariant: tenant failing owner-only rule retains need!
    scheme_name: str
    is_scheme_eligible: bool
    ineligibility_reasons: List[str] = Field(default_factory=list)
    alternative_pathway_task: Optional[str] = None
    cost_heads: Dict[str, float] = Field(default_factory=dict)
    total_eligible_cost_inr: float = 0.0
    evaluated_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))


class FundingRecord(BaseModel):
    """
    FR-066 / RUL-069: Multi-tier funding lifecycle record.
    Announced budgets MUST NOT be treated as received funds.
    """
    funding_id: str
    case_id: str
    source_agency: str  # SDRF, NDRF, CMDRF, LIFE_MISSION, JJM
    cost_head: str      # LAND_ACQUISITION, HOUSING_CONSTRUCTION, INFRASTRUCTURE, RENTAL_ALLOWANCE
    state: FundingState
    amount_inr: float
    sanction_order_ref: Optional[str] = None
    is_announced_budget_only: bool = False
    received_at: Optional[datetime] = None
    spent_at: Optional[datetime] = None
    reconciled_at: Optional[datetime] = None
    audit_event_id: str


class FundingGapReport(BaseModel):
    """
    FR-067 / Equation E17: Funding Gap Calculator.
    G = max(0, sum(C_i) - sum(F_j))
    where C_i are eligible required cost heads and F_j are confirmed received non-duplicate funds.
    """
    case_id: str
    total_required_cost_inr: float
    total_sanctioned_funds_inr: float
    total_received_funds_inr: float  # Only RECEIVED funds deduct from gap!
    funding_gap_inr: float
    is_fully_funded: bool
    formula_citation: str = "E17: G = max(0, sum(C_i) - sum(F_j))"
    calculated_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))


class BasicServicesReadiness(BaseModel):
    water_supply_lpcd: float  # Must be >= 55.0 LPCD potable standard (AT-05)
    electricity_energised: bool
    all_weather_road_functional: bool
    sanitation_drainage_functional: bool
    verified_at: Optional[datetime] = None
    verified_by_officer: Optional[str] = None


class ExternalHandoffRecord(BaseModel):
    """
    FR-069: Accountable External System Handoff.
    Handoff to external systems (LIFE Mission, PFMS) is NEVER reported as completed relocation (RUL-072).
    """
    handoff_id: str
    case_id: str
    external_system_name: str  # LIFE_MISSION, PFMS, PWD_KERALA
    external_reference_id: str
    accountable_agency: str
    accountable_officer: str
    delegated_scope: str
    handoff_date: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    last_verified_state: str
    last_reconciliation_date: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    reconciliation_method: str
    unresolved_discrepancies: List[str] = Field(default_factory=list)
    is_physically_completed: bool = False  # FR-069 invariant
    audit_event_id: str


class PostRelocationFollowUp(BaseModel):
    """
    FR-068 / DEC-026: Post-relocation 6-month and 12-month Social & Livelihood Audit Follow-up.
    """
    followup_id: str
    case_id: str
    milestone_stage: str  # "6_MONTH" or "12_MONTH"
    assessment_date: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    livelihood_restored: bool
    income_restoration_pct: float  # e.g., 85.0% of pre-disaster income
    schooling_continuity_verified: bool
    healthcare_accessible: bool
    infrastructure_integrity_rating: str  # SATISFACTORY, DEFECTS_NOTED, UNACCEPTABLE
    community_satisfaction_score: float  # 0.0 to 1.0
    conducted_by_officer: str
    grievance_notes: Optional[str] = None
    audit_event_id: str


class CaseDeliveryTracker:
    """
    Tracks complete post-approval delivery progression:
    Necessity review -> Scheme assessment -> Funding gap E17 -> Service readiness ->
    Defect clearance -> Possession handover -> Verified occupation -> Completion certificate (RUL-072).
    """

    def __init__(self):
        self._necessity_reviews: Dict[str, RelocationNecessityReview] = {}
        self._scheme_assessments: Dict[str, HouseholdSchemeAssessment] = {}
        self._funding_records: Dict[str, List[FundingRecord]] = {}
        self._required_costs: Dict[str, float] = {}
        self._defects: Dict[str, List[DefectRecord]] = {}
        self._services: Dict[str, BasicServicesReadiness] = {}
        self._external_handoffs: Dict[str, ExternalHandoffRecord] = {}
        self._followups: Dict[str, List[PostRelocationFollowUp]] = {}

        # Tracking state flags
        self._unit_constructed: Dict[str, bool] = {}
        self._offer_accepted: Dict[str, bool] = {}
        self._possession_handed_over: Dict[str, bool] = {}
        self._physically_occupied: Dict[str, bool] = {}

    def set_required_cost(self, case_id: str, cost_inr: float):
        """Set baseline required cost for case funding gap calculation."""
        self._required_costs[case_id] = cost_inr

    # 1. Necessity Assessment (FR-063 / RUL-067)
    def record_necessity_review(
        self,
        case_id: str,
        household_id: str,
        in_situ_mitigation_feasible: bool,
        permanent_relocation_necessary: bool,
        reviewer_name: str,
        reviewer_credentials: str,
        reasons: str,
        uncertainty_level: str = "LOW",
        settlement_community_effects: str = "Minimal fragmentation",
        in_situ_description: Optional[str] = None,
        estimated_in_situ_cost_inr: Optional[float] = None,
        actor_id: Optional[str] = None,
        **kwargs: Any,
    ) -> RelocationNecessityReview:
        rev_id = f"NEC-{case_id}-{uuid4().hex[:6]}"
        effective_actor = actor_id or reviewer_name
        audit_entry = global_audit_ledger.append_event(
            action="RECORD_RELOCATION_NECESSITY",
            actor_id=effective_actor,
            resource_type="RELOCATION_NECESSITY",
            resource_id=case_id,
            payload={
                "household_id": household_id,
                "in_situ_feasible": in_situ_mitigation_feasible,
                "relocation_necessary": permanent_relocation_necessary,
                "reasons": reasons,
            },
        )
        record = RelocationNecessityReview(
            review_id=rev_id,
            case_id=case_id,
            household_id=household_id,
            in_situ_mitigation_feasible=in_situ_mitigation_feasible,
            in_situ_mitigation_description=in_situ_description,
            estimated_in_situ_cost_inr=estimated_in_situ_cost_inr,
            permanent_relocation_necessary=permanent_relocation_necessary,
            competent_reviewer_name=reviewer_name,
            competent_reviewer_credentials=reviewer_credentials,
            reasons=reasons,
            uncertainty_level=uncertainty_level,
            settlement_community_effects=settlement_community_effects,
            audit_event_id=audit_entry.event_id,
        )
        self._necessity_reviews[case_id] = record
        return record

    def record_scheme_assessment(
        self,
        case_id: str,
        household_id: str,
        tenure_category: str,
        scheme_name: str,
        is_scheme_eligible: bool,
        ineligibility_reasons: Optional[List[str]] = None,
        alternative_pathway_task: Optional[str] = None,
        actor_id: str = "welfare_officer",
        **kwargs: Any,
    ) -> HouseholdSchemeAssessment:
        assess_id = f"SCH-ASSESS-{case_id}-{uuid4().hex[:6]}"
        assessment = HouseholdSchemeAssessment(
            assessment_id=assess_id,
            household_id=household_id,
            tenure_category=tenure_category,
            relocation_need_verified=True,
            relocation_need_preserved=True,  # Invariant: RUL-068 / AT-18
            scheme_name=scheme_name,
            is_scheme_eligible=is_scheme_eligible,
            ineligibility_reasons=ineligibility_reasons or [],
            alternative_pathway_task=alternative_pathway_task,
        )
        self._scheme_assessments[household_id] = assessment
        global_audit_ledger.append_event(
            action="RECORD_SCHEME_ASSESSMENT",
            actor_id=actor_id,
            resource_type="SCHEME_ASSESSMENT",
            resource_id=assess_id,
            payload={
                "case_id": case_id,
                "household_id": household_id,
                "eligible": is_scheme_eligible,
                "preserved": True,
            },
        )
        return assessment

    # 2. Scheme Assessment (FR-064, FR-065 / RUL-068, AT-18)
    def assess_household_scheme(
        self,
        household_id: str,
        tenure_category: str,
        pathway: RelocationPathway,
    ) -> HouseholdSchemeAssessment:
        """
        Evaluates eligibility under state schemes while strictly preserving relocation need
        if tenant/informal occupant fails landowner scheme rules (RUL-068, AT-18).
        """
        assess_id = f"SCH-ASSESS-{household_id}-{uuid4().hex[:6]}"
        if tenure_category == "OWNER":
            if pathway == RelocationPathway.SELF_RELOCATION_ASSISTANCE:
                cost_heads = {"land_purchase": 600000.0, "housing_construction": 400000.0}
                scheme_name = "Kerala VLRS (G.O. Ms 6/2018/DMD)"
                is_eligible = True
                inelig_reasons = []
                alt_task = None
            else:
                cost_heads = {"township_plot": 1200000.0, "township_infrastructure": 300000.0}
                scheme_name = "Wayanad Model Township Package"
                is_eligible = True
                inelig_reasons = []
                alt_task = None
        elif tenure_category == "TENANT":
            # AT-18: Tenant fails owner-only scheme rule, but relocation need remains verified!
            cost_heads = {"rental_allowance_12m": 120000.0, "livelihood_transition": 180000.0}
            scheme_name = "Disaster Tenant Rental Assistance & Priority Housing"
            is_eligible = False  # Ineligible for landowner capital grant
            inelig_reasons = ["Tenant household does not hold land title required for VLRS owner land grant."]
            alt_task = "TASK-ALT-TENANT-RENTAL: Enroll household in 12-month rental allowance and affordable rental housing pool (RUL-068)."
        else:  # LANDLESS
            cost_heads = {"land_patta_allotment": 400000.0, "life_mission_house": 600000.0}
            scheme_name = "Kerala Punarjani Landless Resettlement Scheme"
            is_eligible = True
            inelig_reasons = []
            alt_task = None

        total_cost = sum(cost_heads.values())
        assessment = HouseholdSchemeAssessment(
            assessment_id=assess_id,
            household_id=household_id,
            tenure_category=tenure_category,
            relocation_need_verified=True,
            relocation_need_preserved=True,  # Invariant: RUL-068 & AT-18
            scheme_name=scheme_name,
            is_scheme_eligible=is_eligible,
            ineligibility_reasons=inelig_reasons,
            alternative_pathway_task=alt_task,
            cost_heads=cost_heads,
            total_eligible_cost_inr=total_cost,
        )
        self._scheme_assessments[household_id] = assessment
        return assessment

    # 3. Funding Records & Gap Calculator E17 (FR-066, FR-067 / RUL-069)
    def record_funding(
        self,
        case_id: str,
        source_agency: str,
        cost_head: str,
        state: FundingState,
        amount_inr: float,
        actor_id: str,
        sanction_order_ref: Optional[str] = None,
        is_announced_budget_only: bool = False,
    ) -> FundingRecord:
        """
        Record funding state transition.
        Announced budgets are flagged and NOT counted as received funds (RUL-069).
        """
        f_id = f"FND-{case_id}-{uuid4().hex[:6]}"
        now = datetime.now(timezone.utc)
        audit_entry = global_audit_ledger.append_event(
            action="RECORD_FUNDING_STATE",
            actor_id=actor_id,
            resource_type="FUNDING_RECORD",
            resource_id=f_id,
            payload={
                "case_id": case_id,
                "source": source_agency,
                "state": state.value,
                "amount": amount_inr,
                "is_announced_only": is_announced_budget_only,
            },
        )
        record = FundingRecord(
            funding_id=f_id,
            case_id=case_id,
            source_agency=source_agency,
            cost_head=cost_head,
            state=state,
            amount_inr=amount_inr,
            sanction_order_ref=sanction_order_ref,
            is_announced_budget_only=is_announced_budget_only,
            received_at=now if state in (FundingState.RECEIVED, FundingState.SPENT, FundingState.RECONCILED) else None,
            audit_event_id=audit_entry.event_id,
        )
        if case_id not in self._funding_records:
            self._funding_records[case_id] = []
        self._funding_records[case_id].append(record)
        return record

    def calculate_funding_gap(
        self,
        case_id: str,
        total_required_cost_inr: Optional[float] = None,
    ) -> FundingGapReport:
        """
        Equation E17: G = max(0, sum(C_i) - sum(F_j))
        Only confirmed RECEIVED funds deduct from gap.
        Announced budgets or unreceived sanctions do not deduct from required costs.
        """
        if total_required_cost_inr is None:
            total_required_cost_inr = self._required_costs.get(case_id, 0.0)

        records = self._funding_records.get(case_id, [])

        sanctioned_total = sum(
            r.amount_inr for r in records
            if r.state in (FundingState.SANCTIONED, FundingState.COMMITTED, FundingState.RELEASED, FundingState.RECEIVED, FundingState.SPENT, FundingState.RECONCILED)
            and not r.is_announced_budget_only
        )

        received_total = sum(
            r.amount_inr for r in records
            if r.state in (FundingState.RECEIVED, FundingState.SPENT, FundingState.RECONCILED)
            and not r.is_announced_budget_only
        )

        gap = max(0.0, total_required_cost_inr - received_total)

        return FundingGapReport(
            case_id=case_id,
            total_required_cost_inr=total_required_cost_inr,
            total_sanctioned_funds_inr=sanctioned_total,
            total_received_funds_inr=received_total,
            funding_gap_inr=gap,
            is_fully_funded=(gap == 0.0),
        )

    # 4. Basic Services Verification (FR-068 / AT-05, AT-22)
    def verify_services_readiness(
        self,
        case_id: str,
        water_supply_lpcd: float,
        electricity_energised: bool,
        all_weather_road_functional: bool,
        sanitation_drainage_functional: bool,
        officer_name: str,
    ) -> BasicServicesReadiness:
        record = BasicServicesReadiness(
            water_supply_lpcd=water_supply_lpcd,
            electricity_energised=electricity_energised,
            all_weather_road_functional=all_weather_road_functional,
            sanitation_drainage_functional=sanitation_drainage_functional,
            verified_at=datetime.now(timezone.utc),
            verified_by_officer=officer_name,
        )
        self._services[case_id] = record

        global_audit_ledger.append_event(
            action="VERIFY_BASIC_SERVICES",
            actor_id=officer_name,
            resource_type="BASIC_SERVICES",
            resource_id=case_id,
            payload={
                "water_lpcd": water_supply_lpcd,
                "electricity": electricity_energised,
                "road": all_weather_road_functional,
                "sanitation": sanitation_drainage_functional,
            },
        )
        return record

    def record_basic_services(
        self,
        case_id: str,
        water_lpcd: float,
        electricity: bool,
        road: bool,
        sanitation: bool,
        verified_by: str,
        **kwargs: Any,
    ) -> BasicServicesReadiness:
        return self.verify_services_readiness(
            case_id=case_id,
            water_supply_lpcd=water_lpcd,
            electricity_energised=electricity,
            all_weather_road_functional=road,
            sanitation_drainage_functional=sanitation,
            officer_name=verified_by,
        )

    # 5. Defect Management (AT-22 / RUL-072)
    def log_defect(
        self,
        case_id: str,
        site_id: str,
        unit_id: str,
        category: DefectCategory,
        severity: DefectSeverity,
        description: str,
        officer_name: Optional[str] = None,
        logged_by: Optional[str] = None,
        actor_id: Optional[str] = None,
        **kwargs: Any,
    ) -> DefectRecord:
        d_id = f"DEF-{case_id}-{uuid4().hex[:6]}"
        effective_officer = logged_by or officer_name or actor_id or "inspector"
        record = DefectRecord(
            defect_id=d_id,
            case_id=case_id,
            site_id=site_id,
            unit_id=unit_id,
            category=category,
            severity=severity,
            description=description,
            logged_by_officer=effective_officer,
        )
        if case_id not in self._defects:
            self._defects[case_id] = []
        self._defects[case_id].append(record)

        global_audit_ledger.append_event(
            action="LOG_UNIT_DEFECT",
            actor_id=actor_id or effective_officer,
            resource_type="DEFECT_RECORD",
            resource_id=d_id,
            payload={
                "case_id": case_id,
                "severity": severity.value,
                "category": category.value,
                "description": description,
            },
        )
        return record

    def resolve_defect(
        self,
        defect_id: str,
        evidence_ref: str,
        verified_by: Optional[str] = None,
        officer_name: Optional[str] = None,
        actor_id: Optional[str] = None,
        case_id: Optional[str] = None,
        **kwargs: Any,
    ) -> DefectRecord:
        target = None
        target_case_id = case_id
        effective_verifier = verified_by or officer_name or actor_id or "verifier"

        if target_case_id:
            defects = self._defects.get(target_case_id, [])
            target = next((d for d in defects if d.defect_id == defect_id), None)
        else:
            for cid, d_list in self._defects.items():
                for d in d_list:
                    if d.defect_id == defect_id:
                        target = d
                        target_case_id = cid
                        break
                if target:
                    break

        if not target:
            raise KeyError(f"Defect '{defect_id}' not found.")

        target.is_resolved = True
        target.resolved_at = datetime.now(timezone.utc)
        target.resolution_evidence_ref = evidence_ref
        target.verified_by_officer = effective_verifier

        global_audit_ledger.append_event(
            action="RESOLVE_UNIT_DEFECT",
            actor_id=actor_id or effective_verifier,
            resource_type="DEFECT_RECORD",
            resource_id=defect_id,
            payload={"case_id": target_case_id, "evidence_ref": evidence_ref},
        )
        return target

    def verify_and_handover_possession(
        self,
        case_id: str,
        officer_name: str,
        actor_id: Optional[str] = None,
        **kwargs: Any,
    ) -> Dict[str, Any]:
        """
        Verify conditions and handover possession (RUL-072, AT-22).
        """
        services = self._services.get(case_id)
        if not services:
            raise UnservicedUnitHandoverError(case_id, "Missing functioning services: Services readiness not verified.")

        missing_services = []
        if services.water_supply_lpcd < 55.0:
            missing_services.append(f"Water yield {services.water_supply_lpcd:.1f} LPCD below 55 LPCD standard")
        if not services.electricity_energised:
            missing_services.append("Electricity grid not energised")
        if not services.all_weather_road_functional:
            missing_services.append("All-weather access road not functional")

        if missing_services:
            raise UnservicedUnitHandoverError(case_id, "; ".join(missing_services))

        # Check unresolved defects
        defects = self._defects.get(case_id, [])
        unresolved_serious = [
            d for d in defects
            if not d.is_resolved and d.severity in (DefectSeverity.CRITICAL, DefectSeverity.MAJOR)
        ]
        if unresolved_serious:
            raise DefectsBlockCompletionError(
                case_id=case_id,
                defect_count=len(unresolved_serious),
                reason=f"{len(unresolved_serious)} critical/major defects unresolved: {[d.description for d in unresolved_serious]}",
            )

        self._unit_constructed[case_id] = True
        self._possession_handed_over[case_id] = True
        effective_officer = officer_name or actor_id or "handover_officer"
        global_audit_ledger.append_event(
            action="POSSESSION_HANDED_OVER",
            actor_id=effective_officer,
            resource_type="POSSESSION_HANDOVER",
            resource_id=case_id,
            payload={"status": "POSSESSION_HANDED_OVER"},
        )
        return {
            "status": "POSSESSION_HANDED_OVER",
            "case_id": case_id,
            "officer_name": effective_officer,
            "handed_over_at": datetime.now(timezone.utc).isoformat(),
        }

    # 6. Progression Gates: Unit Ready -> Offer Accepted -> Handover -> Occupation (RUL-072, AT-22)
    def mark_unit_constructed(self, case_id: str, officer_name: str):
        self._unit_constructed[case_id] = True
        global_audit_ledger.append_event(
            action="UNIT_CONSTRUCTION_CERTIFIED",
            actor_id=officer_name,
            resource_type="UNIT_CONSTRUCTION",
            resource_id=case_id,
            payload={"status": "CONSTRUCTED"},
        )

    def record_offer_acceptance(self, case_id: str, officer_name: str):
        self._offer_accepted[case_id] = True
        global_audit_ledger.append_event(
            action="OFFER_ACCEPTED_BY_BENEFICIARY",
            actor_id=officer_name,
            resource_type="BENEFICIARY_OFFER",
            resource_id=case_id,
            payload={"status": "ACCEPTED"},
        )

    def record_possession_handover(self, case_id: str, officer_name: str):
        """
        AT-22: Handover blocked if:
        - Unit not constructed
        - Required basic services missing or water < 55 LPCD
        - Any unresolved CRITICAL or MAJOR defects remain
        """
        if not self._unit_constructed.get(case_id, False):
            raise ValueError(f"Cannot handover possession for case '{case_id}': Physical unit is not constructed.")

        services = self._services.get(case_id)
        if not services:
            raise UnservicedUnitHandoverError(case_id, "Services readiness not verified.")

        missing_services = []
        if services.water_supply_lpcd < 55.0:
            missing_services.append(f"Water yield {services.water_supply_lpcd:.1f} LPCD below 55 LPCD standard")
        if not services.electricity_energised:
            missing_services.append("Electricity grid not energised")
        if not services.all_weather_road_functional:
            missing_services.append("All-weather access road not functional")

        if missing_services:
            raise UnservicedUnitHandoverError(case_id, "; ".join(missing_services))

        # Check unresolved defects
        defects = self._defects.get(case_id, [])
        unresolved_serious = [
            d for d in defects
            if not d.is_resolved and d.severity in (DefectSeverity.CRITICAL, DefectSeverity.MAJOR)
        ]
        if unresolved_serious:
            raise DefectsBlockCompletionError(
                case_id=case_id,
                defect_count=len(unresolved_serious),
                reason=f"{len(unresolved_serious)} critical/major defects unresolved: {[d.description for d in unresolved_serious]}",
            )

        self._possession_handed_over[case_id] = True
        global_audit_ledger.append_event(
            action="POSSESSION_HANDED_OVER",
            actor_id=officer_name,
            resource_type="POSSESSION_HANDOVER",
            resource_id=case_id,
            payload={"status": "HANDED_OVER"},
        )

    def record_physical_occupation(self, case_id: str, field_officer_name: str):
        """
        Verify that family has physically moved in.
        Handover must be complete first.
        """
        if not self._possession_handed_over.get(case_id, False):
            raise ValueError(f"Cannot verify occupation for case '{case_id}': Handover not completed.")

        self._physically_occupied[case_id] = True
        global_audit_ledger.append_event(
            action="PHYSICAL_OCCUPATION_VERIFIED",
            actor_id=field_officer_name,
            resource_type="PHYSICAL_OCCUPATION",
            resource_id=case_id,
            payload={"status": "OCCUPIED"},
        )

    def evaluate_relocation_completion(self, case_id: str) -> Tuple[bool, List[str]]:
        """
        RUL-072: Allocation, sanction, or ceremonies NEVER equal completed relocation.
        Full completion requires:
        1. Unit constructed
        2. Basic services verified (water >= 55 LPCD, power, road)
        3. 0 unresolved critical or major defects
        4. Beneficiary offer accepted
        5. Possession handed over
        6. Actual physical occupation verified
        """
        blocking_reasons = []

        if not self._unit_constructed.get(case_id, False):
            blocking_reasons.append("Physical unit not constructed")

        services = self._services.get(case_id)
        if not services:
            blocking_reasons.append("Basic services readiness not verified")
        else:
            if services.water_supply_lpcd < 55.0:
                blocking_reasons.append(f"Potable water supply ({services.water_supply_lpcd} LPCD) below 55 LPCD standard")
            if not services.electricity_energised:
                blocking_reasons.append("Electricity grid not energised")
            if not services.all_weather_road_functional:
                blocking_reasons.append("All-weather access road not functional")

        defects = self._defects.get(case_id, [])
        serious_defects = [d for d in defects if not d.is_resolved and d.severity in (DefectSeverity.CRITICAL, DefectSeverity.MAJOR)]
        if serious_defects:
            blocking_reasons.append(f"{len(serious_defects)} unresolved critical/major defects remain")

        if not self._offer_accepted.get(case_id, False):
            blocking_reasons.append("Beneficiary offer acceptance not recorded")

        if not self._possession_handed_over.get(case_id, False):
            blocking_reasons.append("Possession handover not completed")

        if not self._physically_occupied.get(case_id, False):
            blocking_reasons.append("Physical on-ground occupation not verified")

        is_complete = len(blocking_reasons) == 0
        return is_complete, blocking_reasons

    # 7. Accountable External Handoff Protocol (FR-069)
    def register_external_handoff(
        self,
        case_id: str,
        external_system_name: str,
        external_reference_id: str,
        accountable_agency: str,
        accountable_officer: str,
        delegated_scope: str,
        reconciliation_method: str = "PERIODIC_API_SYNC_AND_SITE_AUDIT",
    ) -> ExternalHandoffRecord:
        """
        FR-069: If another system owns delivery or payment, record accountable owner,
        external reference, reconciliation method, and unresolved discrepancies.
        Handoff is NEVER reported as completed relocation (RUL-072).
        """
        h_id = f"EXT-{case_id}-{uuid4().hex[:6]}"
        audit_entry = global_audit_ledger.append_event(
            action="EXTERNAL_SYSTEM_HANDOFF_REGISTERED",
            actor_id=accountable_officer,
            resource_type="EXTERNAL_HANDOFF",
            resource_id=h_id,
            payload={
                "case_id": case_id,
                "external_system": external_system_name,
                "external_ref": external_reference_id,
                "accountable_agency": accountable_agency,
                "is_relocation_complete": False,  # Invariant: Never completed!
            },
        )
        record = ExternalHandoffRecord(
            handoff_id=h_id,
            case_id=case_id,
            external_system_name=external_system_name,
            external_reference_id=external_reference_id,
            accountable_agency=accountable_agency,
            accountable_officer=accountable_officer,
            delegated_scope=delegated_scope,
            last_verified_state="HANDOFF_ACTIVE_NOT_COMPLETED",
            reconciliation_method=reconciliation_method,
            is_physically_completed=False,  # FR-069 invariant
            audit_event_id=audit_entry.event_id,
        )
        self._external_handoffs[case_id] = record
        return record

    def record_external_handoff(
        self,
        case_id: str,
        external_system_name: str,
        external_reference_id: str,
        accountable_agency: str,
        accountable_officer: str,
        delegated_scope: str,
        reconciliation_method: str = "PERIODIC_API_SYNC_AND_SITE_AUDIT",
        actor_id: Optional[str] = None,
        **kwargs: Any,
    ) -> ExternalHandoffRecord:
        return self.register_external_handoff(
            case_id=case_id,
            external_system_name=external_system_name,
            external_reference_id=external_reference_id,
            accountable_agency=accountable_agency,
            accountable_officer=accountable_officer,
            delegated_scope=delegated_scope,
            reconciliation_method=reconciliation_method,
        )

    # 8. Post-Relocation Follow-Up (FR-068 / DEC-026)
    def record_livelihood_followup(
        self,
        case_id: str,
        milestone_stage: str,  # "6_MONTH" or "12_MONTH"
        livelihood_restored: bool,
        income_restoration_pct: float,
        schooling_continuity: bool,
        healthcare_accessible: bool,
        infrastructure_rating: str,
        community_satisfaction: float,
        officer_name: str,
        grievance_notes: Optional[str] = None,
    ) -> PostRelocationFollowUp:
        fol_id = f"FOL-{case_id}-{milestone_stage}-{uuid4().hex[:6]}"
        audit_entry = global_audit_ledger.append_event(
            action="POST_RELOCATION_FOLLOWUP_RECORDED",
            actor_id=officer_name,
            resource_type="LIVELIHOOD_FOLLOWUP",
            resource_id=fol_id,
            payload={
                "case_id": case_id,
                "milestone": milestone_stage,
                "livelihood_restored": livelihood_restored,
                "income_pct": income_restoration_pct,
                "community_satisfaction": community_satisfaction,
            },
        )
        record = PostRelocationFollowUp(
            followup_id=fol_id,
            case_id=case_id,
            milestone_stage=milestone_stage,
            livelihood_restored=livelihood_restored,
            income_restoration_pct=income_restoration_pct,
            schooling_continuity_verified=schooling_continuity,
            healthcare_accessible=healthcare_accessible,
            infrastructure_integrity_rating=infrastructure_rating,
            community_satisfaction_score=community_satisfaction,
            conducted_by_officer=officer_name,
            grievance_notes=grievance_notes,
            audit_event_id=audit_entry.event_id,
        )
        if case_id not in self._followups:
            self._followups[case_id] = []
        self._followups[case_id].append(record)
        return record

    def record_post_relocation_followup(
        self,
        case_id: str,
        milestone_stage: str,
        livelihood_restored: bool,
        income_restoration_pct: float,
        schooling_continuity: bool,
        healthcare_accessible: bool,
        infrastructure_rating: str,
        satisfaction_score: float,
        conducted_by: str,
        actor_id: Optional[str] = None,
        grievance_notes: Optional[str] = None,
        **kwargs: Any,
    ) -> PostRelocationFollowUp:
        return self.record_livelihood_followup(
            case_id=case_id,
            milestone_stage=milestone_stage,
            livelihood_restored=livelihood_restored,
            income_restoration_pct=income_restoration_pct,
            schooling_continuity=schooling_continuity,
            healthcare_accessible=healthcare_accessible,
            infrastructure_rating=infrastructure_rating,
            community_satisfaction=satisfaction_score,
            officer_name=conducted_by,
            grievance_notes=grievance_notes,
        )

    def get_necessity_review(self, case_id: str) -> Optional[RelocationNecessityReview]:
        return self._necessity_reviews.get(case_id)

    def get_scheme_assessment(self, household_id: str) -> Optional[HouseholdSchemeAssessment]:
        return self._scheme_assessments.get(household_id)

    def get_external_handoff(self, case_id: str) -> Optional[ExternalHandoffRecord]:
        return self._external_handoffs.get(case_id)

    def list_defects(self, case_id: str) -> List[DefectRecord]:
        return self._defects.get(case_id, [])

    def list_funding(self, case_id: str) -> List[FundingRecord]:
        return self._funding_records.get(case_id, [])

    def list_followups(self, case_id: str) -> List[PostRelocationFollowUp]:
        return self._followups.get(case_id, [])


# Global singleton instance
delivery_tracker = CaseDeliveryTracker()
