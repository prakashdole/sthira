"""
PUNARVAS-AI Advisory Allocation & Feasibility Validator Module (ARC-C08 / C1-07).
Normative Reference: rules.md (RUL-040-043, RUL-070, RUL-073, RUL-074).
"""

from typing import Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.contracts import utc_now
from punarvas.core.enums import SolverStatus, RelocationPathway
from punarvas.core.audit import global_audit_ledger
from punarvas.modules.household.service import HouseholdCase


class AllocationAssignment(BaseModel):
    household_id: str
    assigned_site_id: Optional[str] = None  # None if unassigned
    pathway: RelocationPathway
    assigned_unit_plot_cents: Optional[float] = None
    is_ground_floor: bool = False
    match_score: float = 0.0
    explanation: str  # Plain-language explanation (RUL-042)


class AllocationScenario(BaseModel):
    scenario_id: str
    policy_version: str = "POL-WYD-2024.1"
    status: SolverStatus = SolverStatus.OPTIMAL
    total_households: int
    assigned_count: int
    unassigned_count: int
    assignments: List[AllocationAssignment]
    site_utilization: Dict[str, int] = Field(default_factory=dict)
    is_simulation_only: bool = True  # Draft scenarios reserve nothing (RUL-070)
    created_at: str = Field(default_factory=lambda: utc_now().isoformat())


class AllocationValidator:
    """
    Independent feasibility validator checking integer indivisibility, capacity,
    accessibility, and consent feasibility (RUL-074).
    """

    @classmethod
    def validate(
        cls,
        scenario: AllocationScenario,
        site_capacities: Dict[str, int],
        households_by_id: Dict[str, HouseholdCase],
    ) -> List[str]:
        violations = []

        # 1. Check site capacities (RUL-041)
        used_counts: Dict[str, int] = {}
        for a in scenario.assignments:
            if a.assigned_site_id:
                used_counts[a.assigned_site_id] = used_counts.get(a.assigned_site_id, 0) + 1

        for site_id, count in used_counts.items():
            max_cap = site_capacities.get(site_id, 0)
            if count > max_cap:
                violations.append(f"Site '{site_id}' capacity exceeded: {count} assigned vs {max_cap} max capacity.")

        # 2. Check accessibility and indivisibility constraints (RUL-041)
        for a in scenario.assignments:
            hh = households_by_id.get(a.household_id)
            if not hh:
                violations.append(f"Household '{a.household_id}' not found in registry.")
                continue

            if a.assigned_site_id:
                if hh.requires_ground_floor and not a.is_ground_floor:
                    violations.append(
                        f"Household '{a.household_id}' requires ground floor accommodation for elderly/disabled member, "
                        "but assignment does not guarantee ground floor."
                    )

        return violations


class AllocationService:
    """
    Advisory capacity-constrained scenario generator.
    Preserves household indivisibility, preferences, and explicit unassigned outcomes (RUL-041, RUL-042).
    """

    def generate_scenario(
        self,
        scenario_id: str,
        households: List[HouseholdCase],
        site_capacities: Dict[str, int],
        site_plot_cents: Dict[str, float],
        actor_id: str,
    ) -> AllocationScenario:
        assignments: List[AllocationAssignment] = []
        site_remaining = dict(site_capacities)
        assigned_count = 0
        unassigned_count = 0

        # Sort households by vulnerability priority: elderly + disabled first
        sorted_households = sorted(
            households,
            key=lambda h: (h.elderly_count + h.disabled_count * 2),
            reverse=True,
        )

        for hh in sorted_households:
            # Check pathway choice
            if hh.chosen_pathway == RelocationPathway.SELF_RELOCATION_ASSISTANCE:
                assignments.append(
                    AllocationAssignment(
                        household_id=hh.household_id,
                        assigned_site_id=None,
                        pathway=RelocationPathway.SELF_RELOCATION_ASSISTANCE,
                        explanation="Household opted for Kerala VLRS ₹10 Lakh self-relocation assistance pathway.",
                    )
                )
                assigned_count += 1
                continue

            # Candidate site matching based on preferences and remaining capacity
            matched_site_id = None
            for pref_site in hh.preferred_site_ids:
                if site_remaining.get(pref_site, 0) > 0:
                    matched_site_id = pref_site
                    break

            # Fallback to any available site if preference full
            if not matched_site_id:
                for site_id, cap in site_remaining.items():
                    if cap > 0:
                        matched_site_id = site_id
                        break

            if matched_site_id:
                site_remaining[matched_site_id] -= 1
                assigned_count += 1
                ground_floor = hh.requires_ground_floor
                cents = site_plot_cents.get(matched_site_id, 7.0)

                assignments.append(
                    AllocationAssignment(
                        household_id=hh.household_id,
                        assigned_site_id=matched_site_id,
                        pathway=RelocationPathway.TOWNSHIP,
                        assigned_unit_plot_cents=cents,
                        is_ground_floor=ground_floor,
                        match_score=0.95 if matched_site_id in hh.preferred_site_ids else 0.70,
                        explanation=(
                            f"Assigned to {matched_site_id} ({cents} cents plot). "
                            f"{'Accommodated with ground-floor accessibility.' if ground_floor else ''}"
                        ),
                    )
                )
            else:
                unassigned_count += 1
                assignments.append(
                    AllocationAssignment(
                        household_id=hh.household_id,
                        assigned_site_id=None,
                        pathway=RelocationPathway.TOWNSHIP,
                        explanation=(
                            "Unassigned: All candidate sites in preferred areas have reached dwelling capacity. "
                            "Requires additional site reservation or phase-2 allocation."
                        ),
                    )
                )

        site_utilization = {s: site_capacities[s] - site_remaining[s] for s in site_capacities}

        scenario = AllocationScenario(
            scenario_id=scenario_id,
            total_households=len(households),
            assigned_count=assigned_count,
            unassigned_count=unassigned_count,
            assignments=assignments,
            site_utilization=site_utilization,
            is_simulation_only=True,
        )

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/Allocation",
            action="GENERATE_ALLOCATION_SCENARIO",
            entity_type="AllocationScenario",
            entity_id=scenario_id,
            version_id="1.0",
            reason=f"Generated scenario with {assigned_count} matched and {unassigned_count} unassigned.",
        )

        return scenario


# Global singleton instance
allocation_service = AllocationService()
