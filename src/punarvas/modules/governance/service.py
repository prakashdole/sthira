"""
PUNARVAS-AI Governance, Objections & Schemes Module (ARC-C09, ARC-C12 / C1-08).
Normative Reference: rules.md (RUL-006, RUL-046-049, RUL-068, RUL-069, RUL-072).
"""

from typing import Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import utc_now
from punarvas.core.enums import DecisionState, FundingState, RelocationPathway
from punarvas.core.audit import global_audit_ledger


class ObjectionRecord(BaseModel):
    objection_id: str
    household_id: str
    target_decision_id: str
    filing_timestamp: str = Field(default_factory=lambda: utc_now().isoformat())
    reason_category: str  # INCLUSION_ERROR, EXCLUSION_ERROR, PATHWAY_PREFERENCE, FRA_RIGHTS
    statement: str
    assigned_officer_id: str
    hearing_date: Optional[str] = None
    state: DecisionState = DecisionState.OBJECTION_FILED
    remedy_notes: Optional[str] = None
    resolved_at: Optional[str] = None


class SchemeMilestoneTracker(BaseModel):
    """
    Tracks scheme funding and completion states (RUL-068, RUL-069, RUL-072).
    Approval does NOT equal completed relocation (RUL-072).
    """
    case_id: str
    household_id: str
    pathway: RelocationPathway
    scheme_name: str  # e.g. "Kerala VLRS" or "Wayanad Model Township Package"
    sanctioned_amount_inr: float
    funding_state: FundingState = FundingState.IDENTIFIED
    released_amount_inr: float = 0.0
    unit_ready: bool = False
    water_service_functioning: bool = False
    electricity_service_functioning: bool = False
    handover_possession_signed: bool = False
    occupation_verified: bool = False
    is_relocation_complete: bool = False  # Set ONLY when all conditions pass (RUL-072)


class GovernanceService:
    """
    Manages objection remedy lifecycles and scheme delivery completion.
    """

    def __init__(self):
        self._objections: Dict[str, ObjectionRecord] = {}
        self._schemes: Dict[str, SchemeMilestoneTracker] = {}

    def file_objection(
        self,
        household_id: str,
        target_decision_id: str,
        reason_category: str,
        statement: str,
        assigned_officer_id: str,
        actor_id: str,
    ) -> ObjectionRecord:
        """Register citizen objection with immutable receipt (RUL-048)."""
        obj_id = f"OBJ-{int(utc_now().timestamp())}-{household_id}"
        rec = ObjectionRecord(
            objection_id=obj_id,
            household_id=household_id,
            target_decision_id=target_decision_id,
            reason_category=reason_category,
            statement=statement,
            assigned_officer_id=assigned_officer_id,
        )
        self._objections[obj_id] = rec

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/Appeals",
            action="FILE_OBJECTION",
            entity_type="Objection",
            entity_id=obj_id,
            version_id="1.0",
            reason=f"Category: {reason_category}. Statement: {statement}",
        )
        return rec

    def resolve_objection(
        self,
        objection_id: str,
        remedy_granted: bool,
        remedy_notes: str,
        officer_id: str,
    ) -> ObjectionRecord:
        """Resolve objection following hearing step (RUL-047, RUL-048)."""
        obj = self._objections.get(objection_id)
        if not obj:
            raise KeyError(f"Objection '{objection_id}' not found.")

        obj.state = DecisionState.REMEDY_GRANTED if remedy_granted else DecisionState.OBJECTION_DISMISSED
        obj.remedy_notes = remedy_notes
        obj.resolved_at = utc_now().isoformat()

        global_audit_ledger.log(
            actor_id=officer_id,
            authority_scope="Wayanad/Appeals",
            action="RESOLVE_OBJECTION",
            entity_type="Objection",
            entity_id=objection_id,
            version_id="1.0",
            reason=f"Remedy Granted: {remedy_granted}. Notes: {remedy_notes}",
        )
        return obj

    def update_scheme_progress(self, tracker: SchemeMilestoneTracker, actor_id: str) -> SchemeMilestoneTracker:
        """
        Update milestone progress.
        Enforces RUL-072: is_relocation_complete requires unit readiness, water, power,
        handover, and actual occupation.
        """
        all_met = (
            tracker.unit_ready
            and tracker.water_service_functioning
            and tracker.electricity_service_functioning
            and tracker.handover_possession_signed
            and tracker.occupation_verified
        )
        tracker.is_relocation_complete = all_met
        self._schemes[tracker.case_id] = tracker

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/Delivery",
            action="UPDATE_SCHEME_MILESTONES",
            entity_type="SchemeMilestone",
            entity_id=tracker.case_id,
            version_id="1.0",
            reason=f"Relocation complete: {tracker.is_relocation_complete} (RUL-072).",
        )
        return tracker

    def get_objection(self, objection_id: str) -> Optional[ObjectionRecord]:
        return self._objections.get(objection_id)


# Global singleton instance
governance_service = GovernanceService()
