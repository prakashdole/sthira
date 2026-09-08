"""
PUNARVAS-AI Household & Participation Module (ARC-C06 / C1-05).
Normative Reference: rules.md (RUL-019, RUL-038-045, RUL-044 consent separation).
"""

from typing import Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.enums import ConsentPurpose, RelocationPathway
from punarvas.core.contracts import utc_now
from punarvas.core.audit import global_audit_ledger


class HouseholdCase(BaseModel):
    household_id: str
    head_of_household: str
    member_count: int
    elderly_count: int = 0
    disabled_count: int = 0
    children_count: int = 0
    requires_ground_floor: bool = False
    source_parcel_id: str
    verified_eligibility: bool = False
    consents: Dict[ConsentPurpose, bool] = Field(default_factory=dict)
    chosen_pathway: RelocationPathway = RelocationPathway.TOWNSHIP
    preferred_site_ids: List[str] = Field(default_factory=list)
    has_pending_objection: bool = False
    created_at: str = Field(default_factory=lambda: utc_now().isoformat())


class HouseholdService:
    """
    Manages verified household cases, vulnerability accommodations, and consent purposes.
    """

    def __init__(self):
        self._cases: Dict[str, HouseholdCase] = {}

    def register_case(self, case: HouseholdCase, actor_id: str, reason: str) -> HouseholdCase:
        self._cases[case.household_id] = case

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/SocialCasework",
            action="REGISTER_HOUSEHOLD",
            entity_type="Household",
            entity_id=case.household_id,
            version_id="1.0",
            reason=reason,
        )
        return case

    def record_consent(
        self,
        household_id: str,
        purpose: ConsentPurpose,
        consented: bool,
        actor_id: str,
        reason: str,
    ) -> HouseholdCase:
        """
        Record purpose-specific consent (RUL-044).
        Silence or missing contact is never treated as consent.
        """
        case = self._cases.get(household_id)
        if not case:
            raise KeyError(f"Household '{household_id}' not found.")

        case.consents[purpose] = consented

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/SocialCasework",
            action="RECORD_CONSENT",
            entity_type="Household",
            entity_id=household_id,
            version_id="1.0",
            reason=f"Purpose: {purpose.value}, Consented: {consented}. Rationale: {reason}",
        )
        return case

    def get_case(self, household_id: str) -> Optional[HouseholdCase]:
        return self._cases.get(household_id)

    def list_eligible_cases(self) -> List[HouseholdCase]:
        """Return cases that have verified eligibility and programme consent."""
        return [
            c for c in self._cases.values()
            if c.verified_eligibility and c.consents.get(ConsentPurpose.PROGRAMME_PARTICIPATION, False)
        ]


# Global singleton instance
household_service = HouseholdService()
