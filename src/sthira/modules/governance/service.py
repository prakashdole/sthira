"""
Sthira Governance, Objections & Schemes Module (ARC-C09, ARC-C12 / C1-08).
Normative Reference: rules.md (RUL-006, RUL-046-049, RUL-068, RUL-069, RUL-072).
"""

from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from sthira.core.contracts import utc_now
from sthira.core.enums import DecisionState, FundingState, RelocationPathway
from sthira.core.audit import global_audit_ledger


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

    def issue_hearing_notice(
        self,
        objection_id: str,
        hearing_date: str,
        venue: str,
        officer_name: str,
        actor_id: str,
    ) -> "HearingNoticeRecord":
        """
        Issue formal bilingual hearing notice for an active objection (FEAT-016 / RUL-048).
        """
        obj = self._objections.get(objection_id)
        if not obj:
            raise KeyError(f"Objection '{objection_id}' not found.")

        obj.hearing_date = hearing_date
        obj.state = DecisionState.HEARING_SCHEDULED

        notice_id = f"NOT-{objection_id}-{int(utc_now().timestamp())}"
        notice = HearingNoticeRecord(
            notice_id=notice_id,
            objection_id=objection_id,
            household_id=obj.household_id,
            hearing_date=hearing_date,
            venue=venue,
            hearing_officer=officer_name,
            notice_text_en=(
                f"Formal Hearing Notice: Your objection ({objection_id}) regarding relocation casework "
                f"is scheduled for personal hearing on {hearing_date} at {venue} before {officer_name}."
            ),
            notice_text_ml=(
                f"ഔദ്യോഗിക ഹിയറിംഗ് നോട്ടീസ്: പുനരധിവാസ കേസുമായി ബന്ധപ്പെട്ട താങ്കളുടെ പരാതി ({objection_id}) "
                f"ന്മേലുള്ള വ്യക്തിഗത ഹിയറിംഗ് {hearing_date}-ന് {venue}-ൽ {officer_name} മുൻപാകെ നടക്കുന്നതാണ്."
            ),
        )

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/Appeals",
            action="ISSUE_HEARING_NOTICE",
            entity_type="HearingNotice",
            entity_id=notice_id,
            version_id="1.0",
            reason=f"Notice issued for hearing on {hearing_date} at {venue}.",
        )
        return notice

    def assess_scheme_entitlements(
        self,
        household_id: str,
        tenure_category: str,  # "OWNER", "TENANT", "LANDLESS"
        chosen_pathway: RelocationPathway,
        actor_id: str,
    ) -> "SchemeAssessmentResult":
        """
        FEAT-023: Scheme, entitlement, and funding assessment (RUL-068, RUL-069, DEC-018).
        Preserves relocation need even when ineligible for owner schemes.
        """
        assessment_id = f"SCH-ASSESS-{household_id}-{int(utc_now().timestamp())}"

        if tenure_category == "OWNER":
            if chosen_pathway == RelocationPathway.SELF_RELOCATION_ASSISTANCE:
                # Kerala VLRS: 6 Lakh land + 4 Lakh house
                scheme_name = "Kerala Vulnerability Linked Relocation Scheme (VLRS G.O. Ms 6/2018/DMD)"
                sanctioned_amt = 1000000.0
                tranches = [
                    {"tranche": 1, "description": "Safe land acquisition (>= 3 cents)", "amount_inr": 600000.0},
                    {"tranche": 2, "description": "Foundation & Plinth level construction", "amount_inr": 150000.0},
                    {"tranche": 3, "description": "Roof level completion", "amount_inr": 150000.0},
                    {"tranche": 4, "description": "Completion, electricity & water connection", "amount_inr": 100000.0},
                ]
                notes_en = "Eligible for Kerala VLRS ₹10 Lakh assistance. Agricultural ownership preserved."
                notes_ml = "കേരള VLRS ₹10 ലക്ഷം ധനസഹായത്തിന് അർഹതയുണ്ട്. കൃഷിഭൂമി ഉടമസ്ഥാവകാശം നിലനിർത്താം."
            else:
                # Township Package
                scheme_name = "Wayanad Model Township Rehabilitation Package (DM Act §65 Acquisition)"
                sanctioned_amt = 1500000.0  # Estimated per-unit state construction cost
                tranches = [
                    {"tranche": 1, "description": "Township Plot & Structural Unit", "amount_inr": 1200000.0},
                    {"tranche": 2, "description": "Common Community Infrastructure & JJM Water", "amount_inr": 300000.0},
                ]
                notes_en = "Eligible for state-constructed model township dwelling on ~7 cents plot."
                notes_ml = "ഏകദേശം 7 സെന്റ് സ്ഥലത്ത് സർക്കാർ നിർമ്മിക്കുന്ന മാതൃകാ ടൗൺഷിപ്പ് ഭവനത്തിന് അർഹതയുണ്ട്."
        elif tenure_category == "TENANT":
            # Tenant cannot get owner land grant, but RUL-068: retains relocation need!
            scheme_name = "State Disaster Tenant Rental & Transit Housing Assistance"
            sanctioned_amt = 300000.0
            tranches = [
                {"tranche": 1, "description": "Immediate rental allowance (12 months)", "amount_inr": 120000.0},
                {"tranche": 2, "description": "Livelihood transition & deposit assistance", "amount_inr": 180000.0},
            ]
            notes_en = (
                "Tenant household ineligible for landowner acquisition grant, but retains full relocation need (RUL-068). "
                "Enrolled in 12-month rental allowance and Priority Affordable Rental Housing."
            )
            notes_ml = (
                "വാടകക്കാരനായതിനാൽ ഭൂവുടമ നഷ്ടപരിഹാരത്തിന് അർഹതയില്ലെങ്കിലും പുനരധിവാസ ആവശ്യം നിലനിൽക്കുന്നു (RUL-068). "
                "12 മാസത്തെ വാടക അലവൻസിനും മുൻഗണനാ ഭവന പദ്ധതിക്കും ശുപാർശ ചെയ്യുന്നു."
            )
        else:  # LANDLESS
            scheme_name = "Kerala Punarjani Landless Housing Scheme"
            sanctioned_amt = 1000000.0
            tranches = [
                {"tranche": 1, "description": "State poramboke land patta allotment (3 cents)", "amount_inr": 400000.0},
                {"tranche": 2, "description": "Life Mission house construction grant", "amount_inr": 600000.0},
            ]
            notes_en = "Landless disaster-affected family allotted safe 3-cent parcel plus Life Mission dwelling."
            notes_ml = "ഭൂമിയില്ലാത്ത ദുരന്തബാധിത കുടുംബത്തിന് 3 സെന്റ് സുരക്ഷിത ഭൂമിയും ലൈഫ് മിഷൻ വീടും അനുവദിക്കുന്നു."

        res = SchemeAssessmentResult(
            assessment_id=assessment_id,
            household_id=household_id,
            tenure_category=tenure_category,
            retains_relocation_need=True,
            recommended_pathway=chosen_pathway,
            scheme_name=scheme_name,
            sanctioned_amount_inr=sanctioned_amt,
            milestone_tranches=tranches,
            advisory_notes_en=notes_en,
            advisory_notes_ml=notes_ml,
        )

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/Finance",
            action="ASSESS_SCHEME_ENTITLEMENT",
            entity_type="SchemeAssessment",
            entity_id=assessment_id,
            version_id="1.0",
            reason=f"Assessed {tenure_category} for {scheme_name} (INR {sanctioned_amt:,.0f}).",
        )
        return res


class HearingNoticeRecord(BaseModel):
    notice_id: str
    objection_id: str
    household_id: str
    hearing_date: str
    venue: str
    hearing_officer: str
    notice_text_en: str
    notice_text_ml: str
    delivery_channel: str = "REGISTERED_POST_AND_SMS"
    issued_at: str = Field(default_factory=lambda: utc_now().isoformat())


class SchemeAssessmentResult(BaseModel):
    assessment_id: str
    household_id: str
    tenure_category: str  # OWNER, TENANT, LANDLESS
    retains_relocation_need: bool = True  # RUL-068
    recommended_pathway: RelocationPathway
    scheme_name: str
    sanctioned_amount_inr: float
    milestone_tranches: List[Dict[str, Any]]
    advisory_notes_en: str
    advisory_notes_ml: str
    assessed_at: str = Field(default_factory=lambda: utc_now().isoformat())


# Global singleton instance
governance_service = GovernanceService()

