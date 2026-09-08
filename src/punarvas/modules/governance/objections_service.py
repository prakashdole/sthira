"""
PUNARVAS-AI Objections, Appeals & Grievance Remedy Engine (Phase 9 / ARC-C09 / FEAT-015, FEAT-016).
Normative Reference: plan.md (#9), trd.md (§5.6, FR-047-FR-052), rules.md (RUL-006, RUL-046-RUL-049), DEC-028, AT-09.
"""

from datetime import datetime, timezone, timedelta
from enum import Enum
from typing import Any, Dict, List, Optional, Tuple
from uuid import uuid4
from pydantic import BaseModel, Field

from punarvas.core.audit import global_audit_ledger
from punarvas.core.contracts import utc_now, UserContext
from punarvas.core.enums import DecisionState, RoleType


class ObjectionCategory(str, Enum):
    INCLUSION_ERROR = "INCLUSION_ERROR"
    EXCLUSION_ERROR = "EXCLUSION_ERROR"
    PATHWAY_PREFERENCE = "PATHWAY_PREFERENCE"
    FOREST_RIGHTS_FRA = "FOREST_RIGHTS_FRA"
    SITE_BOUNDARY_AND_SAFETY = "SITE_BOUNDARY_AND_SAFETY"
    WATER_INADEQUACY = "WATER_INADEQUACY"
    TENANCY_AND_OCCUPATION = "TENANCY_AND_OCCUPATION"
    OTHER_GRIEVANCE = "OTHER_GRIEVANCE"


class ObjectionAdmissibility(str, Enum):
    PENDING_REVIEW = "PENDING_REVIEW"
    ADMISSIBLE = "ADMISSIBLE"
    INADMISSIBLE = "INADMISSIBLE"


class ObjectionFilingChannel(str, Enum):
    ASSISTED_SERVICE_DESK = "ASSISTED_SERVICE_DESK"  # RUL-045: assisted service for digital divide
    PHYSICAL_PAPER_PETITION = "PHYSICAL_PAPER_PETITION"
    CITIZEN_PORTAL = "CITIZEN_PORTAL"
    GRAM_SABHA_RECORD = "GRAM_SABHA_RECORD"


class HearingNotice(BaseModel):
    notice_id: str
    objection_id: str
    hearing_date: datetime
    venue: str
    presiding_officer: str
    notified_parties: List[str]
    issued_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    is_dispatched: bool = True


class ObjectionDecisionOrder(BaseModel):
    order_id: str
    objection_id: str
    deciding_authority: str
    statutory_authority_basis: str
    order_date: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    relief_granted: bool
    summary_of_grounds: str
    remedy_notes: str
    appeal_window_days: int = 30
    appeal_deadline: datetime
    audit_event_id: str


class ObjectionCase(BaseModel):
    objection_id: str
    receipt_token: str  # Immutable receipt token provided to citizen (RUL-048)
    household_id: str
    filer_name: str
    is_representative: bool = False
    representative_authorization_doc: Optional[str] = None
    target_entity_type: str  # DRAFT_BENEFICIARY_LIST, ALLOCATION_PROPOSAL, SITE_SELECTION
    target_entity_id: str
    target_version_id: str
    reason_category: ObjectionCategory
    statement: str
    evidence_attachment_hashes: List[str] = Field(default_factory=list)
    filing_channel: ObjectionFilingChannel = ObjectionFilingChannel.ASSISTED_SERVICE_DESK
    filing_timestamp: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    sla_deadline: datetime
    state: DecisionState = DecisionState.OBJECTION_FILED
    admissibility: ObjectionAdmissibility = ObjectionAdmissibility.PENDING_REVIEW
    inadmissibility_reason: Optional[str] = None
    assigned_officer_id: str
    assigned_officer_name: str
    hearings: List[HearingNotice] = Field(default_factory=list)
    decision_order: Optional[ObjectionDecisionOrder] = None
    is_appealed: bool = False
    appeal_case_id: Optional[str] = None
    created_audit_event_id: str


class SLAEscalationRecommendation(BaseModel):
    recommendation_id: str
    objection_id: str
    days_overdue: int
    target_authority: str
    recommended_action: str
    rationale: str
    is_advisory: bool = True  # RUL-006: Recommendation only, never autonomous bypass!
    generated_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))


class ObjectionsAppealsService:
    """
    Phase 9 Objections, Appeals & Citizen Remedy Service.
    Enforces FR-047 through FR-051 and RUL-046 through RUL-049.
    """

    DEFAULT_SLA_DAYS = 21  # Configurable by district order

    def __init__(self):
        self._cases: Dict[str, ObjectionCase] = {}
        # Index of frozen dependent decisions: target_entity_id -> list of pending objection_ids
        self._frozen_entities: Dict[str, List[str]] = {}

    def file_objection(
        self,
        household_id: str,
        filer_name: str,
        target_entity_type: str,
        target_entity_id: str,
        target_version_id: str,
        category: ObjectionCategory,
        statement: str,
        assigned_officer_id: str,
        assigned_officer_name: str,
        actor: UserContext,
        evidence_hashes: Optional[List[str]] = None,
        is_representative: bool = False,
        representative_doc: Optional[str] = None,
        filing_channel: ObjectionFilingChannel = ObjectionFilingChannel.ASSISTED_SERVICE_DESK,
        sla_days: int = DEFAULT_SLA_DAYS,
    ) -> ObjectionCase:
        now = datetime.now(timezone.utc)
        deadline = now + timedelta(days=sla_days)
        obj_id = f'OBJ-{now.strftime("%Y%m%d")}-{uuid4().hex[:6].upper()}'
        receipt = f'RCPT-PUNARVAS-{obj_id}'

        # Audit filing with immutable timestamp
        audit_entry = global_audit_ledger.append_event(
            action='OBJECTION_FILED',
            actor_id=actor.user_id,
            authority_scope=f'{actor.geography_scope.state}/{actor.geography_scope.district or "Wayanad"}',
            resource_type='OBJECTION_CASE',
            resource_id=obj_id,
            payload={
                'household_id': household_id,
                'category': category.value,
                'target_entity': f'{target_entity_type}:{target_entity_id}@{target_version_id}',
                'receipt': receipt,
                'sla_days': sla_days,
            },
            reason=statement[:100],
        )

        case = ObjectionCase(
            objection_id=obj_id,
            receipt_token=receipt,
            household_id=household_id,
            filer_name=filer_name,
            is_representative=is_representative,
            representative_authorization_doc=representative_doc,
            target_entity_type=target_entity_type,
            target_entity_id=target_entity_id,
            target_version_id=target_version_id,
            reason_category=category,
            statement=statement,
            evidence_attachment_hashes=evidence_hashes or [],
            filing_channel=filing_channel,
            filing_timestamp=now,
            sla_deadline=deadline,
            state=DecisionState.OBJECTION_FILED,
            admissibility=ObjectionAdmissibility.PENDING_REVIEW,
            assigned_officer_id=assigned_officer_id,
            assigned_officer_name=assigned_officer_name,
            created_audit_event_id=audit_entry.event_id,
        )
        self._cases[obj_id] = case

        # RUL-049: Freeze dependent decision or list while objection is active
        if target_entity_id not in self._frozen_entities:
            self._frozen_entities[target_entity_id] = []
        self._frozen_entities[target_entity_id].append(obj_id)

        return case

    def review_admissibility(
        self,
        objection_id: str,
        is_admissible: bool,
        officer: UserContext,
        rejection_reason: Optional[str] = None,
    ) -> ObjectionCase:
        case = self._cases.get(objection_id)
        if not case:
            raise KeyError(f'Objection {objection_id} not found.')

        if is_admissible:
            case.admissibility = ObjectionAdmissibility.ADMISSIBLE
            case.state = DecisionState.UNDER_REVIEW
        else:
            case.admissibility = ObjectionAdmissibility.INADMISSIBLE
            case.inadmissibility_reason = rejection_reason or 'Does not satisfy statutory criteria for relocation objection.'
            case.state = DecisionState.OBJECTION_DISMISSED
            self._unfreeze_if_clean(case.target_entity_id, objection_id)

        global_audit_ledger.append_event(
            action='OBJECTION_ADMISSIBILITY_DETERMINED',
            actor_id=officer.user_id,
            authority_scope=f'{officer.geography_scope.state}/{officer.geography_scope.district or "Wayanad"}',
            resource_type='OBJECTION_CASE',
            resource_id=objection_id,
            payload={'admissible': is_admissible, 'reason': rejection_reason},
        )
        return case

    def schedule_hearing(
        self,
        objection_id: str,
        hearing_date: datetime,
        venue: str,
        presiding_officer: str,
        notified_parties: List[str],
        actor: Optional[UserContext] = None,
        officer: Optional[UserContext] = None,
    ) -> HearingNotice:
        case = self._cases.get(objection_id)
        if not case:
            raise KeyError(f'Objection {objection_id} not found.')

        ctx = actor or officer
        actor_id = ctx.user_id if ctx else "officer_hearing"
        actor_scope = f'{ctx.geography_scope.state}/{ctx.geography_scope.district or "Wayanad"}' if ctx else "Kerala/Wayanad"

        notice_id = f'NOT-HEAR-{objection_id}-{len(case.hearings) + 1}'
        notice = HearingNotice(
            notice_id=notice_id,
            objection_id=objection_id,
            hearing_date=hearing_date,
            venue=venue,
            presiding_officer=presiding_officer,
            notified_parties=notified_parties,
        )
        case.hearings.append(notice)
        case.state = DecisionState.UNDER_REVIEW

        global_audit_ledger.append_event(
            action='HEARING_SCHEDULED',
            actor_id=actor_id,
            authority_scope=actor_scope,
            resource_type='HEARING_NOTICE',
            resource_id=notice_id,
            payload={
                'objection_id': objection_id,
                'hearing_date': hearing_date.isoformat(),
                'venue': venue,
            },
        )
        return notice

    def issue_decision_order(
        self,
        objection_id: str,
        relief_granted: bool,
        summary_of_grounds: str,
        remedy_notes: str,
        deciding_authority: str,
        statutory_basis: Optional[str] = None,
        statutory_authority_basis: Optional[str] = None,
        officer: Optional[UserContext] = None,
        appeal_window_days: int = 30,
    ) -> ObjectionDecisionOrder:
        case = self._cases.get(objection_id)
        if not case:
            raise KeyError(f'Objection {objection_id} not found.')

        basis = statutory_basis or statutory_authority_basis or "Disaster Management Act 2005 §30"
        actor_id = officer.user_id if officer else deciding_authority
        actor_scope = f'{officer.geography_scope.state}/{officer.geography_scope.district or "Wayanad"}' if officer else "Kerala/Wayanad"

        now = datetime.now(timezone.utc)
        appeal_deadline = now + timedelta(days=appeal_window_days)
        order_id = f'ORD-OBJ-{objection_id}'

        audit_entry = global_audit_ledger.append_event(
            action='OBJECTION_DECISION_ORDER_ISSUED',
            actor_id=actor_id,
            authority_scope=actor_scope,
            resource_type='DECISION_ORDER',
            resource_id=order_id,
            payload={
                'relief_granted': relief_granted,
                'authority': deciding_authority,
                'statutory_basis': basis,
                'appeal_deadline': appeal_deadline.isoformat(),
            },
            reason=remedy_notes[:100],
        )
        order = ObjectionDecisionOrder(
            order_id=order_id,
            objection_id=objection_id,
            deciding_authority=deciding_authority,
            statutory_authority_basis=basis,
            order_date=now,
            relief_granted=relief_granted,
            summary_of_grounds=summary_of_grounds,
            remedy_notes=remedy_notes,
            appeal_window_days=appeal_window_days,
            appeal_deadline=appeal_deadline,
            audit_event_id=audit_entry.event_id,
        )
        case.decision_order = order
        case.state = DecisionState.REMEDY_GRANTED if relief_granted else DecisionState.OBJECTION_DISMISSED

        # Remove from active frozen entities if resolved
        self._unfreeze_if_clean(case.target_entity_id, objection_id)
        return order

    def file_appeal(
        self,
        objection_id: str,
        appellate_statement: str,
        officer: UserContext,
    ) -> ObjectionCase:
        """File statutory appeal against an adverse objection decision order (RUL-047)."""
        case = self._cases.get(objection_id)
        if not case:
            raise KeyError(f'Objection {objection_id} not found.')
        if not case.decision_order:
            raise ValueError('Cannot appeal an objection before a decision order is issued.')
        if datetime.now(timezone.utc) > case.decision_order.appeal_deadline:
            raise ValueError('Appeal period has expired under statutory timeline.')

        case.is_appealed = True
        case.appeal_case_id = f'APL-{objection_id}'
        case.state = DecisionState.APPEALED

        # Re-freeze the entity during appellate review
        if case.target_entity_id not in self._frozen_entities:
            self._frozen_entities[case.target_entity_id] = []
        if objection_id not in self._frozen_entities[case.target_entity_id]:
            self._frozen_entities[case.target_entity_id].append(objection_id)

        global_audit_ledger.append_event(
            action='OBJECTION_APPEALED',
            actor_id=officer.user_id,
            authority_scope=f'{officer.geography_scope.state}/{officer.geography_scope.district or "Wayanad"}',
            resource_type='OBJECTION_CASE',
            resource_id=objection_id,
            payload={'appeal_id': case.appeal_case_id, 'grounds': appellate_statement[:100]},
        )
        return case

    def check_is_entity_frozen(self, entity_id: str) -> Tuple[bool, Optional[str]]:
        """RUL-049: Check if target decision/entity is blocked by pending objections or appeals."""
        pending = self._frozen_entities.get(entity_id, [])
        if pending:
            return True, f"Entity '{entity_id}' is frozen pending resolution of objections: {', '.join(pending)} (RUL-049)"
        return False, None

    def get_pending_objections_for_entity(self, entity_id: str) -> List[str]:
        return list(self._frozen_entities.get(entity_id, []))

    def evaluate_sla_and_recommend_escalation(
        self,
        objection_id: str,
        supervisory_authority: str = 'District Collector & DDMA Chairperson, Wayanad',
    ) -> Optional[SLAEscalationRecommendation]:
        """
        FR-051 / RUL-006 / AT-09:
        Overdue objection workflow recommends escalation to authorized role.
        Never executes automated authority bypass!
        """
        case = self._cases.get(objection_id)
        if not case:
            return None

        if case.state in (DecisionState.REMEDY_GRANTED, DecisionState.OBJECTION_DISMISSED):
            return None

        now = datetime.now(timezone.utc)
        if now > case.sla_deadline:
            overdue_days = (now - case.sla_deadline).days + 1
            rec = SLAEscalationRecommendation(
                recommendation_id=f'ESC-{objection_id}',
                objection_id=objection_id,
                days_overdue=overdue_days,
                target_authority=supervisory_authority,
                recommended_action=f'Urgent hearing review recommended for {case.objection_id} ({overdue_days} days overdue)',
                rationale=f'Citizen grievance filed on {case.filing_timestamp.date()} exceeds statutory SLA of {case.sla_deadline.date()}.',
                is_advisory=True,
            )
            global_audit_ledger.append_event(
                action='SLA_ESCALATION_RECOMMENDED',
                actor_id='system_sla_monitor',
                authority_scope='Wayanad/DDMA',
                resource_type='SLA_RECOMMENDATION',
                resource_id=rec.recommendation_id,
                payload={
                    'objection_id': objection_id,
                    'overdue_days': overdue_days,
                    'target_authority': supervisory_authority,
                    'is_advisory': True,
                },
            )
            return rec
        return None

    def check_sla_escalations(self, supervisory_authority: str = 'District Collector & DDMA Chairperson, Wayanad') -> List[SLAEscalationRecommendation]:
        """Scan all registered cases and return advisory escalation recommendations for any overdue ones."""
        recs = []
        for obj_id in self._cases:
            rec = self.evaluate_sla_and_recommend_escalation(obj_id, supervisory_authority=supervisory_authority)
            if rec:
                recs.append(rec)
        return recs

    def _unfreeze_if_clean(self, entity_id: str, objection_id: str):
        if entity_id in self._frozen_entities:
            if objection_id in self._frozen_entities[entity_id]:
                self._frozen_entities[entity_id].remove(objection_id)
            if not self._frozen_entities[entity_id]:
                del self._frozen_entities[entity_id]

    def get_case(self, objection_id: str) -> Optional[ObjectionCase]:
        return self._cases.get(objection_id)

    def list_cases(self, household_id: Optional[str] = None) -> List[ObjectionCase]:
        if household_id:
            return [c for c in self._cases.values() if c.household_id == household_id]
        return list(self._cases.values())


# Global singleton instance
objections_appeals_service = ObjectionsAppealsService()

# Alias for convenience
objections_service = objections_appeals_service
