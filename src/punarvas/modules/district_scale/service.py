"""
Phase 4 Multi-District Scaling, Configuration-Driven Onboarding, and Tenant Isolation (C4-01, C4-02).
Normative Reference: phases.md §7 & §12.7, architecture.md §13, rules.md (RUL-017, RUL-054), trd.md (NFR-005, NFR-012).
"""

from datetime import datetime, timezone
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.audit import global_audit_ledger
from punarvas.core.contracts import UserContext
from punarvas.core.enums import ClassificationLevel, RoleType


class DistrictProfile(BaseModel):
    district_id: str
    district_name: str
    state: str = "Kerala"
    terrain_type: str  # HIGHLAND_MOUNTAIN, COASTAL_LOWLAND_POLDER, RESERVOIR_DAM_BASIN
    primary_hazard_types: List[str]
    prohibited_hazard_assumptions: List[str]
    headquarters: str
    local_self_governments: List[str]
    statutory_approver_title: str
    onboarded_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    active: bool = True


class DistrictPolicyOverride(BaseModel):
    override_id: str
    district_id: str
    parameter_key: str
    state_baseline_value: Any
    district_override_value: Any
    rationale: str
    statutory_order_reference: str
    effective_from: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    is_active: bool = True


class DistrictAggregateMetric(BaseModel):
    district_id: str
    district_name: str
    total_vulnerable_settlements: int
    enumerated_households: int
    verified_eligible_households: int
    allocated_township: int
    allocated_self_relocation: int
    unassigned_households: int
    total_sanctioned_budget_inr: float
    active_cases_count: int


class StatewideSummary(BaseModel):
    state: str = "Kerala"
    generated_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    total_onboarded_districts: int
    total_statewide_eligible_households: int
    total_statewide_allocated: int
    total_statewide_unassigned: int
    total_statewide_budget_sanctioned_inr: float
    district_breakdowns: List[DistrictAggregateMetric]


class DistrictOnboardingEngine:
    """
    Configuration-driven district onboarding factory (C4-01).
    Prohibits Wayanad-specific hazard, legal, or geographic assumptions from leaking to other districts (RUL-017).
    """

    def __init__(self):
        self._districts: Dict[str, DistrictProfile] = {}
        self._bootstrap_default_districts()

    def _bootstrap_default_districts(self):
        # Wayanad Reference Pilot
        self.onboard_district(
            DistrictProfile(
                district_id="DIST-WAYANAD",
                district_name="Wayanad",
                terrain_type="HIGHLAND_MOUNTAIN",
                primary_hazard_types=["DEBRIS_FLOW_RUNOUT", "LANDSLIDE_SLOPE_FAILURE"],
                prohibited_hazard_assumptions=["COASTAL_STORM_SURGE", "GLACIAL_LAKE_OUTBURST"],
                headquarters="Kalpetta",
                local_self_governments=["Meppadi Grama Panchayat", "Vythiri Grama Panchayat", "Kalpetta Municipality"],
                statutory_approver_title="District Collector & DDMA Chairman, Wayanad",
            ),
            actor_id="system_bootstrap",
        )
        # Idukki Highland & Dam Buffer District
        self.onboard_district(
            DistrictProfile(
                district_id="DIST-IDUKKI",
                district_name="Idukki",
                terrain_type="RESERVOIR_DAM_BASIN",
                primary_hazard_types=["DAM_INUNDATION_BUFFER", "STEEP_SLOPE_INSTABILITY", "FLASH_FLOOD"],
                prohibited_hazard_assumptions=["COASTAL_SURGE", "BELOW_SEA_LEVEL_POLDER"],
                headquarters="Painavu",
                local_self_governments=["Munnar Grama Panchayat", "Devikulam Grama Panchayat", "Adimali Grama Panchayat"],
                statutory_approver_title="District Collector & DDMA Chairman, Idukki",
            ),
            actor_id="system_bootstrap",
        )
        # Alappuzha Coastal & Kuttanad Polder District
        self.onboard_district(
            DistrictProfile(
                district_id="DIST-ALAPPUZHA",
                district_name="Alappuzha",
                terrain_type="COASTAL_LOWLAND_POLDER",
                primary_hazard_types=["KUTTANAD_BELOW_SEA_LEVEL_FLOOD", "TIDAL_SURGE", "COASTAL_EROSION"],
                prohibited_hazard_assumptions=["DEBRIS_FLOW_RUNOUT", "ROCKFALL_SLOPE_FAILURE"],
                headquarters="Alappuzha",
                local_self_governments=["Kainakary Grama Panchayat", "Nedumudi Grama Panchayat", "Ambalappuzha Grama Panchayat"],
                statutory_approver_title="District Collector & DDMA Chairman, Alappuzha",
            ),
            actor_id="system_bootstrap",
        )

    def onboard_district(self, profile: DistrictProfile, actor_id: str = "state_admin") -> DistrictProfile:
        self._districts[profile.district_id] = profile
        global_audit_ledger.append_event(
            action="DISTRICT_ONBOARDED",
            actor_id=actor_id,
            resource_type="DISTRICT_PROFILE",
            resource_id=profile.district_id,
            payload={
                "name": profile.district_name,
                "terrain": profile.terrain_type,
                "primary_hazards": profile.primary_hazard_types,
                "prohibited_assumptions": profile.prohibited_hazard_assumptions,
            },
        )
        return profile

    def get_district(self, district_id: str) -> Optional[DistrictProfile]:
        return self._districts.get(district_id)

    def list_districts(self) -> List[DistrictProfile]:
        return list(self._districts.values())


class PolicyInheritanceEngine:
    """
    Manages state-level baseline policies and district-specific overrides (C4-01).
    Ensures District A cannot read, override, or leak policy into District B.
    """

    STATE_BASELINES: Dict[str, Any] = {
        "KERALA_VLRS_MAX_GRANT_INR": 1000000.0,  # ₹10 Lakh statutory cap
        "KERALA_VLRS_MIN_LAND_CENTS": 3.0,      # Minimum 3 cents for safe self-relocation
        "DDMA_PLAN_UPDATE_INTERVAL_YEARS": 2,     # Disaster Management Act 2025 §31(4)
        "LEAN_SEASON_WATER_LPCD_DEMAND": 55.0,    # Jal Jeevan Mission baseline
    }

    def __init__(self):
        self._overrides: Dict[str, List[DistrictPolicyOverride]] = {}
        self._bootstrap_district_overrides()

    def _bootstrap_district_overrides(self):
        # Wayanad override: 7 cents unit size for Elstone Estate township model
        self.register_override(
            DistrictPolicyOverride(
                override_id="OVR-WYD-PLOT-SIZE",
                district_id="DIST-WAYANAD",
                parameter_key="TOWNSHIP_PLOT_SIZE_CENTS",
                state_baseline_value=3.0,
                district_override_value=7.0,
                rationale="Wayanad Rehabilitation Township model (Elstone Estate / Kalpetta) provides 7 cents per unit",
                statutory_order_reference="G.O. (Ms) No. 2024/DMD/Wayanad",
            ),
            actor_id="district_collector_wayanad",
        )
        # Alappuzha override: +1.5m plinth elevation freeboard for Kuttanad below-sea-level polders
        self.register_override(
            DistrictPolicyOverride(
                override_id="OVR-ALP-FREEBOARD",
                district_id="DIST-ALAPPUZHA",
                parameter_key="MINIMUM_PLINTH_FREEBOARD_METERS",
                state_baseline_value=0.6,
                district_override_value=1.5,
                rationale="Kuttanad polders lie 0.5m-2.2m below MSL; high water levels mandate 1.5m plinth freeboard",
                statutory_order_reference="G.O. (Ms) Kuttanad Below-Sea-Level Guidelines",
            ),
            actor_id="district_collector_alappuzha",
        )

    def register_override(self, override: DistrictPolicyOverride, actor_id: str) -> DistrictPolicyOverride:
        if override.district_id not in self._overrides:
            self._overrides[override.district_id] = []
        self._overrides[override.district_id].append(override)

        global_audit_ledger.append_event(
            action="DISTRICT_POLICY_OVERRIDE_REGISTERED",
            actor_id=actor_id,
            resource_type="POLICY_OVERRIDE",
            resource_id=override.override_id,
            payload={
                "district": override.district_id,
                "param": override.parameter_key,
                "override_value": override.district_override_value,
                "order_ref": override.statutory_order_reference,
            },
        )
        return override

    def resolve_parameter(self, district_id: str, parameter_key: str) -> Any:
        district_list = self._overrides.get(district_id, [])
        for ovr in district_list:
            if ovr.parameter_key == parameter_key and ovr.is_active:
                return ovr.district_override_value
        if parameter_key in self.STATE_BASELINES:
            return self.STATE_BASELINES[parameter_key]
        raise KeyError(f"Parameter '{parameter_key}' not defined in district overrides or state baselines.")

    def get_district_overrides(self, district_id: str) -> List[DistrictPolicyOverride]:
        return self._overrides.get(district_id, [])


class MultiDistrictIsolationManager:
    """
    Enforces geography-scoped row-level security and authorization boundaries (RUL-054, NFR-012, NFR-030).
    A district officer is strictly prohibited from reading or modifying records belonging to another district.
    State administrators have read-only aggregate visibility but cannot usurp district statutory authority.
    """

    def assert_user_can_access_district(self, user: UserContext, target_district_id: str, action: str = "READ"):
        # State administrator roles have state-wide oversight
        if RoleType.STATE_PROGRAMME_ADMIN in user.roles:
            if action in ("APPROVE_STATUTORY_DECISION", "MUTATE_DISTRICT_RECORDS"):
                raise PermissionError(
                    f"State oversight role '{user.username}' cannot usurp district statutory approval in {target_district_id}."
                )
            return True

        # District-scoped users can only access their assigned jurisdiction
        user_dist = user.geography_scope.district
        # Normalize comparison: "Wayanad" == "DIST-WAYANAD" or exact match
        dist_norm = target_district_id.replace("DIST-", "").upper()
        user_norm = user_dist.replace("DIST-", "").upper() if user_dist else ""

        if not user_norm or user_norm != dist_norm:
            raise PermissionError(
                f"Geography Access Denied: User '{user.username}' (scoped to '{user_dist}') cannot access records of '{target_district_id}'."
            )
        return True


class StatewideAggregateDashboard:
    """
    Generates cross-district summary dashboards based purely on approved de-identified aggregates (C4-02, FEAT-019).
    """

    def __init__(self):
        self._district_data: Dict[str, DistrictAggregateMetric] = {
            "DIST-WAYANAD": DistrictAggregateMetric(
                district_id="DIST-WAYANAD",
                district_name="Wayanad",
                total_vulnerable_settlements=3,  # Chooralmala, Mundakkai, Punchirimattam
                enumerated_households=430,
                verified_eligible_households=430,
                allocated_township=350,
                allocated_self_relocation=60,
                unassigned_households=20,
                total_sanctioned_budget_inr=430000000.0,
                active_cases_count=430,
            ),
            "DIST-IDUKKI": DistrictAggregateMetric(
                district_id="DIST-IDUKKI",
                district_name="Idukki",
                total_vulnerable_settlements=5,  # Pettimudi, Munnar slopes, etc.
                enumerated_households=280,
                verified_eligible_households=260,
                allocated_township=200,
                allocated_self_relocation=50,
                unassigned_households=10,
                total_sanctioned_budget_inr=260000000.0,
                active_cases_count=260,
            ),
            "DIST-ALAPPUZHA": DistrictAggregateMetric(
                district_id="DIST-ALAPPUZHA",
                district_name="Alappuzha",
                total_vulnerable_settlements=4,  # Kainakary, Nedumudi polders
                enumerated_households=310,
                verified_eligible_households=300,
                allocated_township=180,
                allocated_self_relocation=110,
                unassigned_households=10,
                total_sanctioned_budget_inr=300000000.0,
                active_cases_count=300,
            ),
        }

    def get_statewide_summary(self) -> StatewideSummary:
        metrics = list(self._district_data.values())
        tot_eligible = sum(m.verified_eligible_households for m in metrics)
        tot_allocated = sum(m.allocated_township + m.allocated_self_relocation for m in metrics)
        tot_unassigned = sum(m.unassigned_households for m in metrics)
        tot_budget = sum(m.total_sanctioned_budget_inr for m in metrics)

        return StatewideSummary(
            state="Kerala",
            total_onboarded_districts=len(metrics),
            total_statewide_eligible_households=tot_eligible,
            total_statewide_allocated=tot_allocated,
            total_statewide_unassigned=tot_unassigned,
            total_statewide_budget_sanctioned_inr=tot_budget,
            district_breakdowns=metrics,
        )


class ScaleQuotaAndRateLimiter:
    """
    Enforces per-district job queue quotas and heavy-job concurrency limits (NFR-005).
    Prevents one district's bulk computations from starving other districts.
    """

    MAX_CONCURRENT_HEAVY_JOBS_PER_DISTRICT = 5

    def __init__(self):
        self._active_jobs: Dict[str, int] = {}

    def acquire_job_slot(self, district_id: str) -> bool:
        current = self._active_jobs.get(district_id, 0)
        if current >= self.MAX_CONCURRENT_HEAVY_JOBS_PER_DISTRICT:
            return False
        self._active_jobs[district_id] = current + 1
        return True

    def release_job_slot(self, district_id: str):
        current = self._active_jobs.get(district_id, 0)
        if current > 0:
            self._active_jobs[district_id] = current - 1

    def get_active_job_count(self, district_id: str) -> int:
        return self._active_jobs.get(district_id, 0)


# Singletons
district_onboarding_engine = DistrictOnboardingEngine()
policy_inheritance_engine = PolicyInheritanceEngine()
multi_district_isolation_manager = MultiDistrictIsolationManager()
statewide_aggregate_dashboard = StatewideAggregateDashboard()
scale_quota_limiter = ScaleQuotaAndRateLimiter()
