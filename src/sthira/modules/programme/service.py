"""
Sthira Programme & Compliance Module (ARC-C01 / C1-01).
Normative Reference: rules.md (RUL-001-007, RUL-054) and trd.md §3.
"""

from typing import Dict, Optional
from pydantic import BaseModel, Field

from sthira.core.contracts import GeographyScope, UserContext, utc_now
from sthira.core.enums import AuthorityState, RoleType
from sthira.core.errors import AuthorityBypassError, UnauthorizedGeographyAccessError
from sthira.core.audit import global_audit_ledger


class ProgrammeRecord(BaseModel):
    programme_id: str
    title: str
    state: str
    district: str
    lead_authority: str
    mandate_legal_basis: str
    active_policy_version: str = "1.0"
    created_at: str = Field(default_factory=lambda: utc_now().isoformat())
    status: AuthorityState = AuthorityState.ANALYTICAL


class ProgrammeService:
    """
    Manages statutory programme context, jurisdiction validation, and compliance tracking.
    """

    def __init__(self):
        self._programmes: Dict[str, ProgrammeRecord] = {}

    def register_programme(self, record: ProgrammeRecord, user: UserContext, reason: str) -> ProgrammeRecord:
        """Register a new permanent relocation programme within authorized jurisdiction."""
        target_geo = GeographyScope(state=record.state, district=record.district)
        if not user.geography_scope.contains(target_geo):
            raise UnauthorizedGeographyAccessError(
                user_scope=f"{user.geography_scope.state}/{user.geography_scope.district}",
                requested_scope=f"{record.state}/{record.district}",
            )

        if not user.has_role(RoleType.GOVERNMENT_APPROVER) and not user.has_role(RoleType.DISASTER_MANAGEMENT_OFFICER):
            raise AuthorityBypassError("Only authorized DDMA/SDMA officers can register relocation programmes.")

        self._programmes[record.programme_id] = record

        global_audit_ledger.log(
            actor_id=user.user_id,
            authority_scope=f"{record.state}/{record.district}",
            action="REGISTER_PROGRAMME",
            entity_type="Programme",
            entity_id=record.programme_id,
            version_id="1.0",
            reason=reason,
            correlation_id=user.correlation_id,
        )
        return record

    def get_programme(self, programme_id: str) -> Optional[ProgrammeRecord]:
        return self._programmes.get(programme_id)

    def verify_authority(self, user: UserContext, required_role: RoleType, target_scope: GeographyScope):
        """Strict authority and jurisdiction verification (RUL-002, RUL-054)."""
        if not user.has_role(required_role):
            raise AuthorityBypassError(f"User '{user.username}' lacks required role '{required_role.value}'.")
        if not user.geography_scope.contains(target_scope):
            raise UnauthorizedGeographyAccessError(
                user_scope=f"{user.geography_scope.state}/{user.geography_scope.district}",
                requested_scope=f"{target_scope.state}/{target_scope.district}",
            )


# Global singleton instance
programme_service = ProgrammeService()
