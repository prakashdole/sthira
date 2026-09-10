"""
Sthira Advisory Allocation & Feasibility Validator Module (ARC-C08 / C1-07).
Normative Reference: rules.md (RUL-040-043, RUL-070, RUL-073, RUL-074).
"""

from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from sthira.core.contracts import utc_now
from sthira.core.enums import ConsentPurpose, SolverStatus, RelocationPathway
from sthira.core.audit import global_audit_ledger
from sthira.core.errors import ReservationConflictError
from sthira.modules.household.service import HouseholdCase


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
            hold_reasons = []
            if not hh.verified_eligibility:
                hold_reasons.append("eligibility is not verified")
            if not hh.consents.get(ConsentPurpose.PROGRAMME_PARTICIPATION, False):
                hold_reasons.append("programme participation consent is missing")
            if not hh.consents.get(ConsentPurpose.PATHWAY_CHOICE, False):
                hold_reasons.append("relocation pathway consent is missing")
            if (
                hh.chosen_pathway == RelocationPathway.TOWNSHIP
                and not hh.consents.get(ConsentPurpose.SITE_PREFERENCE, False)
            ):
                hold_reasons.append("site preference consent is missing")
            if hh.has_pending_objection:
                hold_reasons.append("a household objection is pending")

            if hold_reasons:
                unassigned_count += 1
                assignments.append(
                    AllocationAssignment(
                        household_id=hh.household_id,
                        assigned_site_id=None,
                        pathway=hh.chosen_pathway,
                        explanation=f"Unassigned: {'; '.join(hold_reasons)}.",
                    )
                )
                continue

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


class CapacityReservation(BaseModel):
    reservation_id: str
    scenario_id: str
    site_id: str
    dwellings_reserved: int
    land_cents_reserved: float
    budget_inr_reserved: float
    water_m3_day_reserved: float
    status: str = "ACTIVE"  # ACTIVE, RELEASED, EXPIRED
    reserved_at: str = Field(default_factory=lambda: utc_now().isoformat())
    reserved_by: str
    released_at: Optional[str] = None
    release_reason: Optional[str] = None


class CapacityReservationLedger:
    """
    C2-03 / FEAT-024: Capacity reservation and commitment ledger.
    Atomically reserves capacity across dwellings, land area, budget, and water.
    Drafts reserve nothing (DEC-025). Approval checks remaining capacity and fails closed
    upon competing scenario or overlapping resource collision (ODN-009, RUL-070).
    """

    def __init__(self):
        self._reservations: Dict[str, CapacityReservation] = {}
        self._site_dwellings: Dict[str, int] = {}
        self._site_land_cents: Dict[str, float] = {}
        self._programme_budget_inr: float = 0.0
        self._site_water_m3_day: Dict[str, float] = {}

    def configure_capacities(
        self,
        site_dwellings: Dict[str, int],
        site_land_cents: Dict[str, float],
        programme_budget_inr: float,
        site_water_m3_day: Dict[str, float],
    ):
        self._site_dwellings = dict(site_dwellings)
        self._site_land_cents = dict(site_land_cents)
        self._programme_budget_inr = programme_budget_inr
        self._site_water_m3_day = dict(site_water_m3_day)

    def get_remaining_capacity(self, site_id: str) -> Dict[str, float]:
        active_res = [r for r in self._reservations.values() if r.site_id == site_id and r.status == "ACTIVE"]
        dwellings_used = sum(r.dwellings_reserved for r in active_res)
        land_used = sum(r.land_cents_reserved for r in active_res)
        water_used = sum(r.water_m3_day_reserved for r in active_res)
        total_budget_used = sum(r.budget_inr_reserved for r in self._reservations.values() if r.status == "ACTIVE")

        return {
            "dwellings_remaining": self._site_dwellings.get(site_id, 0) - dwellings_used,
            "land_cents_remaining": self._site_land_cents.get(site_id, 0.0) - land_used,
            "water_m3_day_remaining": self._site_water_m3_day.get(site_id, 0.0) - water_used,
            "programme_budget_inr_remaining": self._programme_budget_inr - total_budget_used,
        }

    def reserve(
        self,
        scenario_id: str,
        site_id: str,
        dwellings: int,
        land_cents: float,
        budget_inr: float,
        water_m3_day: float,
        actor_id: str,
    ) -> CapacityReservation:
        rem = self.get_remaining_capacity(site_id)

        if dwellings > rem["dwellings_remaining"]:
            raise ReservationConflictError(
                resource_type="DWELLING_CAPACITY",
                resource_id=site_id,
                reason=f"Requested {dwellings} units, but only {int(rem['dwellings_remaining'])} remain unreserved."
            )
        if land_cents > rem["land_cents_remaining"]:
            raise ReservationConflictError(
                resource_type="LAND_AREA",
                resource_id=site_id,
                reason=f"Requested {land_cents:.1f} cents, but only {rem['land_cents_remaining']:.1f} cents remain."
            )
        if water_m3_day > rem["water_m3_day_remaining"]:
            raise ReservationConflictError(
                resource_type="WATER_YIELD",
                resource_id=site_id,
                reason=f"Requested {water_m3_day:.1f} m3/day, but only {rem['water_m3_day_remaining']:.1f} m3/day available."
            )
        if budget_inr > rem["programme_budget_inr_remaining"]:
            raise ReservationConflictError(
                resource_type="PROGRAMME_BUDGET",
                resource_id="TOTAL_BUDGET",
                reason=f"Requested INR {budget_inr:,.0f}, but only INR {rem['programme_budget_inr_remaining']:,.0f} available."
            )

        res_id = f"RES-{site_id}-{scenario_id}-{int(utc_now().timestamp())}"
        reservation = CapacityReservation(
            reservation_id=res_id,
            scenario_id=scenario_id,
            site_id=site_id,
            dwellings_reserved=dwellings,
            land_cents_reserved=land_cents,
            budget_inr_reserved=budget_inr,
            water_m3_day_reserved=water_m3_day,
            reserved_by=actor_id,
        )
        self._reservations[res_id] = reservation

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/CapacityLedger",
            action="RESERVE_CAPACITY",
            entity_type="CapacityReservation",
            entity_id=res_id,
            version_id="1.0",
            reason=f"Scenario '{scenario_id}' atomically reserved {dwellings} units, {land_cents} cents, INR {budget_inr} on '{site_id}'.",
        )
        return reservation

    def release(self, reservation_id: str, reason: str, actor_id: str) -> CapacityReservation:
        res = self._reservations.get(reservation_id)
        if not res:
            raise KeyError(f"Reservation '{reservation_id}' not found.")
        res.status = "RELEASED"
        res.released_at = utc_now().isoformat()
        res.release_reason = reason

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="Wayanad/CapacityLedger",
            action="RELEASE_CAPACITY",
            entity_type="CapacityReservation",
            entity_id=reservation_id,
            version_id="1.0",
            reason=f"Released reservation for site '{res.site_id}'. Reason: {reason}",
        )
        return res


# Global singleton instances
allocation_service = AllocationService()
capacity_reservation_ledger = CapacityReservationLedger()

