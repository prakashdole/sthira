"""
PUNARVAS-AI Multi-District Scaling & Tenant Isolation Engine (PH-4 / C4-01).
Normative Reference: phases.md §7 & §12.7, rules.md (RUL-002, RUL-054), trd.md (FR-001, FR-002).

Enforces:
1. Configuration-driven district onboarding without hard-coded Wayanad assumptions.
2. Strict row/geography-level tenant isolation (FR-002) - zero leakage between districts.
3. Policy inheritance: state-level defaults with district-specific parameter overrides.
4. Separate administrative oversight (SDMA) vs operational district execution (DDMA).
"""

from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import GeographyScope, UserContext, utc_now
from punarvas.core.enums import AuthorityState, RoleType
from punarvas.core.errors import AuthorityBypassError, UnauthorizedGeographyAccessError
from punarvas.core.audit import global_audit_ledger


class DistrictPolicyOverrides(BaseModel):
    """District-specific parameter overrides against state baseline."""
    max_safe_slope_degrees: Optional[float] = None
    min_road_access_width_m: Optional[float] = None
    min_lean_season_lpcd: Optional[float] = None
    high_hazard_exclusion_levels: List[str] = Field(default_factory=lambda: ["HIGH", "VERY_HIGH"])
    mandatory_runout_buffer_m: Optional[float] = None


class DistrictConfig(BaseModel):
    """Configuration-driven onboarding contract for a district (C4-01)."""
    district_id: str
    state: str = "Kerala"
    name: str
    headquarters: str
    taluks: List[str] = Field(default_factory=list)
    local_self_governments: List[str] = Field(default_factory=list)  # Grama Panchayats / Municipalities
    lead_authority: str = "District Disaster Management Authority (DDMA)"
    competent_signatory_title: str = "District Collector & District Magistrate"
    recognized_hazard_types: List[str] = Field(default_factory=list)
    policy_overrides: DistrictPolicyOverrides = Field(default_factory=DistrictPolicyOverrides)
    active_policy_version: str = "1.0"
    status: AuthorityState = AuthorityState.ANALYTICAL
    created_at: str = Field(default_factory=lambda: utc_now().isoformat())


class DistrictScalingService:
    """
    Manages district onboarding, multi-district tenant isolation,
    and policy inheritance across the state.
    """

    def __init__(self):
        self._districts: Dict[str, DistrictConfig] = {}
        # Pre-seed Wayanad reference configuration
        self._districts["Wayanad"] = DistrictConfig(
            district_id="Wayanad",
            state="Kerala",
            name="Wayanad",
            headquarters="Kalpetta",
            taluks=["Vythiri", "Mananthavady", "Sulthan Bathery"],
            local_self_governments=["Meppadi", "Kalpetta", "Muppainad", "Pozhuthana", "Vythiri"],
            lead_authority="District Disaster Management Authority (DDMA), Wayanad",
            competent_signatory_title="District Collector, Wayanad",
            recognized_hazard_types=["LANDSLIDE_SUSCEPTIBILITY", "DEBRIS_FLOW_RUNOUT", "FLASH_FLOOD"],
            policy_overrides=DistrictPolicyOverrides(
                max_safe_slope_degrees=22.0,
                min_road_access_width_m=3.66,
                min_lean_season_lpcd=55.0,
                mandatory_runout_buffer_m=100.0,
            ),
            status=AuthorityState.ANALYTICAL,
        )

    def onboard_district(self, config: DistrictConfig, user: UserContext, reason: str) -> DistrictConfig:
        """
        Onboard a new district under state programme authority (C4-01).
        Requires state-level authorization.
        """
        # State-level oversight check
        if user.geography_scope.state != config.state:
            raise UnauthorizedGeographyAccessError(
                user_scope=f"{user.geography_scope.state}/{user.geography_scope.district}",
                requested_scope=f"{config.state}/{config.district_id}",
            )

        # Only State Programme Admin / SDMA can onboard a new district
        if not user.has_role(RoleType.GOVERNMENT_APPROVER) and not user.has_role(RoleType.DISASTER_MANAGEMENT_OFFICER):
            raise AuthorityBypassError("Only authorized SDMA / State approvers can onboard districts.")

        self._districts[config.district_id] = config

        global_audit_ledger.log(
            actor_id=user.user_id,
            authority_scope=f"{config.state}/{config.district_id}",
            action="ONBOARD_DISTRICT",
            entity_type="DistrictConfig",
            entity_id=config.district_id,
            version_id=config.active_policy_version,
            reason=reason,
            correlation_id=user.correlation_id,
        )
        return config

    def get_district(self, district_id: str) -> Optional[DistrictConfig]:
        return self._districts.get(district_id)

    def list_districts(self, state: Optional[str] = None) -> List[DistrictConfig]:
        if state:
            return [d for d in self._districts.values() if d.state == state]
        return list(self._districts.values())

    def verify_district_tenant_access(self, user: UserContext, target_district: str) -> bool:
        """
        Enforce strict row-level isolation between districts (FR-002, RUL-054).
        A district user (e.g. Wayanad) CANNOT query another district's records (e.g. Idukki).
        A state-level oversight officer (scope.district is None or '*') CAN query any district in their state.
        """
        # State match is mandatory
        district_obj = self.get_district(target_district)
        target_state = district_obj.state if district_obj else "Kerala"

        if user.geography_scope.state != target_state:
            raise UnauthorizedGeographyAccessError(
                user_scope=f"{user.geography_scope.state}/{user.geography_scope.district}",
                requested_scope=f"{target_state}/{target_district}",
            )

        # District isolation check
        user_district = user.geography_scope.district
        # State-level oversight: user has state-level role with no district or all-district wildcard
        is_state_oversight = (user_district is None or user_district == "*" or user_district == "") and (
            user.has_role(RoleType.GOVERNMENT_APPROVER) or user.has_role(RoleType.DISASTER_MANAGEMENT_OFFICER) or user.has_role(RoleType.AUDITOR)
        )

        if not is_state_oversight and user_district != target_district:
            raise UnauthorizedGeographyAccessError(
                user_scope=f"{user.geography_scope.state}/{user_district}",
                requested_scope=f"{target_state}/{target_district}",
            )
        return True

    def get_effective_district_policy(self, district_id: str) -> Dict[str, Any]:
        """
        Compute effective policy parameters incorporating district-level overrides (C4-01).
        """
        district = self._districts.get(district_id)
        if not district:
            raise KeyError(f"District '{district_id}' is not onboarded.")

        # Baseline state defaults
        policy = {
            "state": district.state,
            "district_id": district.district_id,
            "policy_version": district.active_policy_version,
            "max_safe_slope_degrees": 25.0,  # Standard baseline
            "min_road_access_width_m": 3.66,
            "min_lean_season_lpcd": 55.0,
            "high_hazard_exclusion_levels": ["HIGH", "VERY_HIGH"],
            "mandatory_runout_buffer_m": 50.0,
        }

        # Apply district overrides if defined
        overrides = district.policy_overrides
        if overrides.max_safe_slope_degrees is not None:
            policy["max_safe_slope_degrees"] = overrides.max_safe_slope_degrees
        if overrides.min_road_access_width_m is not None:
            policy["min_road_access_width_m"] = overrides.min_road_access_width_m
        if overrides.min_lean_season_lpcd is not None:
            policy["min_lean_season_lpcd"] = overrides.min_lean_season_lpcd
        if overrides.high_hazard_exclusion_levels:
            policy["high_hazard_exclusion_levels"] = overrides.high_hazard_exclusion_levels
        if overrides.mandatory_runout_buffer_m is not None:
            policy["mandatory_runout_buffer_m"] = overrides.mandatory_runout_buffer_m

        return policy


# Global singleton service
district_scaling_service = DistrictScalingService()
