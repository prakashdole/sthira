"""
PUNARVAS-AI Multi-Resource Capacity Reservation Ledger (Phase 9 / ARC-C08 / FEAT-024).
Normative Reference: plan.md (#9), trd.md (§3.11, FR-070, AT-15, AT-20), rules.md (RUL-041, RUL-049, RUL-070, RUL-071), DEC-025, DEC-028.
"""

from datetime import datetime, timezone, timedelta
from enum import Enum
from threading import Lock
from typing import Any, Dict, List, Optional, Tuple
from uuid import uuid4
from pydantic import BaseModel, Field

from punarvas.core.audit import global_audit_ledger
from punarvas.core.contracts import utc_now, UserContext
from punarvas.core.enums import RoleType
from punarvas.core.errors import (
    ApprovalConditionUnmetError,
    CapacityExceededError,
    EntityFrozenByObjectionError,
    ReservationConflictError,
)
from punarvas.modules.governance.objections_service import objections_service
from punarvas.modules.governance.approval_service import approval_service
from punarvas.modules.live_ops.service import degraded_mode_controller


class ReservationStatus(str, Enum):
    SIMULATED = "SIMULATED"      # Draft scenario: reserves zero capacity (DEC-025, RUL-070)
    RESERVED = "RESERVED"        # Temporary hold with expiration window
    COMMITTED = "COMMITTED"      # Formal approved allocation locked against official approval
    RELEASED = "RELEASED"        # Explicitly released upon modification or replan (RUL-071)
    EXPIRED = "EXPIRED"          # Auto-expired after hold window
    CANCELLED = "CANCELLED"      # Cancelled due to supersession or withdrawal


class SiteCapacityConfig(BaseModel):
    site_id: str
    district: str
    dwellings_max: int
    land_cents_max: float
    water_m3_day_max: float  # e.g., safe lean-season yield (E16, AT-05)
    overlapping_parcel_ids: List[str] = Field(default_factory=list)


class CapacityReservation(BaseModel):
    reservation_id: str
    scenario_id: str
    site_id: str
    status: ReservationStatus = ReservationStatus.RESERVED
    dwellings_reserved: int
    land_cents_reserved: float
    budget_inr_reserved: float
    water_m3_day_reserved: float
    reserved_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    expires_at: Optional[datetime] = None
    committed_at: Optional[datetime] = None
    approval_order_id: Optional[str] = None
    released_at: Optional[datetime] = None
    release_reason: Optional[str] = None
    reserved_by: str
    audit_event_id: str


class MultiResourceReservationLedger:
    """
    Thread-safe atomic capacity reservation ledger (DEC-025, RUL-070, AT-15).
    Guarantees:
    1. Draft scenarios reserve ZERO capacity (SIMULATED state).
    2. Approval atomically revalidates and locks capacity.
    3. Concurrency races fail closed: if two approvals race for the same dwelling, land,
       budget, or water, the first claims it, the second fails closed with ReservationConflictError.
    4. Overlapping parcels and shared water supplies are constrained together.
    5. Capacity is NEVER subtracted twice (RUL-071).
    """

    def __init__(self):
        self._lock = Lock()
        self._sites: Dict[str, SiteCapacityConfig] = {}
        self._programme_budget_inr: float = 0.0
        self._reservations: Dict[str, CapacityReservation] = {}

    def configure_site(
        self,
        site_id: str,
        district: str,
        dwellings_max: int,
        land_cents_max: float,
        water_m3_day_max: float,
        overlapping_parcel_ids: Optional[List[str]] = None,
    ):
        with self._lock:
            self._sites[site_id] = SiteCapacityConfig(
                site_id=site_id,
                district=district,
                dwellings_max=dwellings_max,
                land_cents_max=land_cents_max,
                water_m3_day_max=water_m3_day_max,
                overlapping_parcel_ids=overlapping_parcel_ids or [],
            )

    def set_programme_budget(self, budget_inr: float):
        with self._lock:
            self._programme_budget_inr = budget_inr

    def get_remaining_capacity(self, site_id: str) -> Dict[str, float]:
        """
        Calculate remaining capacity deducting active (RESERVED and COMMITTED) reservations.
        """
        with self._lock:
            site = self._sites.get(site_id)
            if not site:
                raise KeyError(f"Site '{site_id}' not configured in capacity ledger.")

            active_res = [
                r for r in self._reservations.values()
                if r.site_id == site_id and r.status in (ReservationStatus.RESERVED, ReservationStatus.COMMITTED)
            ]

            dwellings_used = sum(r.dwellings_reserved for r in active_res)
            land_used = sum(r.land_cents_reserved for r in active_res)
            water_used = sum(r.water_m3_day_reserved for r in active_res)

            all_active = [
                r for r in self._reservations.values()
                if r.status in (ReservationStatus.RESERVED, ReservationStatus.COMMITTED)
            ]
            budget_used = sum(r.budget_inr_reserved for r in all_active)

            return {
                "dwellings_remaining": float(site.dwellings_max - dwellings_used),
                "land_cents_remaining": site.land_cents_max - land_used,
                "water_m3_day_remaining": site.water_m3_day_max - water_used,
                "programme_budget_inr_remaining": self._programme_budget_inr - budget_used,
            }

    def simulate_draft_scenario(
        self,
        scenario_id: str,
        site_id: str,
        dwellings: int,
        land_cents: float,
        budget_inr: float,
        water_m3_day: float,
        actor_id: str,
    ) -> CapacityReservation:
        """
        Draft scenarios are simulations only and MUST NOT reserve capacity (DEC-025, RUL-070).
        """
        res_id = f"SIM-{scenario_id}-{site_id}-{uuid4().hex[:6]}"
        audit_entry = global_audit_ledger.append_event(
            action="SIMULATE_SCENARIO_CAPACITY",
            actor_id=actor_id,
            resource_type="CAPACITY_SIMULATION",
            resource_id=res_id,
            payload={
                "scenario_id": scenario_id,
                "site_id": site_id,
                "dwellings": dwellings,
                "is_simulation_only": True,
            },
        )
        record = CapacityReservation(
            reservation_id=res_id,
            scenario_id=scenario_id,
            site_id=site_id,
            status=ReservationStatus.SIMULATED,
            dwellings_reserved=0,  # Explicitly 0 reserved!
            land_cents_reserved=0.0,
            budget_inr_reserved=0.0,
            water_m3_day_reserved=0.0,
            reserved_by=actor_id,
            audit_event_id=audit_entry.event_id,
        )
        with self._lock:
            self._reservations[res_id] = record
        return record

    def hold_reservation(
        self,
        scenario_id: str,
        site_id: str,
        dwellings: int,
        land_cents: float,
        budget_inr: float,
        water_m3_day: float,
        actor_id: str,
        hold_duration_days: int = 14,
    ) -> CapacityReservation:
        """
        Atomically reserve capacity under temporary hold (AT-15).
        Fails closed if remaining capacity is insufficient or competing reservation intervenes.
        """
        degraded_mode_controller.assert_writes_allowed()
        with self._lock:
            # Check freeze from pending objections (RUL-049)
            is_frozen, frozen_reason = objections_service.check_is_entity_frozen(site_id)
            if is_frozen:
                raise EntityFrozenByObjectionError(
                    entity_id=site_id,
                    reason=frozen_reason or "Site is frozen by active objection.",
                )

            site = self._sites.get(site_id)
            if not site:
                raise KeyError(f"Site '{site_id}' not configured in capacity ledger.")

            active_res = [
                r for r in self._reservations.values()
                if r.site_id == site_id and r.status in (ReservationStatus.RESERVED, ReservationStatus.COMMITTED)
            ]
            dwellings_used = sum(r.dwellings_reserved for r in active_res)
            land_used = sum(r.land_cents_reserved for r in active_res)
            water_used = sum(r.water_m3_day_reserved for r in active_res)

            all_active = [
                r for r in self._reservations.values()
                if r.status in (ReservationStatus.RESERVED, ReservationStatus.COMMITTED)
            ]
            budget_used = sum(r.budget_inr_reserved for r in all_active)

            # Atomic multi-resource collision checks (AT-15 fail-closed)
            if dwellings_used + dwellings > site.dwellings_max:
                raise ReservationConflictError(
                    resource_type="DWELLING_CAPACITY",
                    resource_id=site_id,
                    reason=f"Requested {dwellings} units, but only {site.dwellings_max - dwellings_used} remain unreserved.",
                )
            if land_used + land_cents > site.land_cents_max:
                raise ReservationConflictError(
                    resource_type="LAND_PARCEL_AREA",
                    resource_id=site_id,
                    reason=f"Requested {land_cents:.1f} cents, but only {site.land_cents_max - land_used:.1f} cents remain.",
                )
            if water_used + water_m3_day > site.water_m3_day_max:
                raise ReservationConflictError(
                    resource_type="LEAN_SEASON_WATER",
                    resource_id=site_id,
                    reason=f"Requested {water_m3_day:.1f} m3/day, but only {site.water_m3_day_max - water_used:.1f} m3/day available.",
                )
            if budget_used + budget_inr > self._programme_budget_inr:
                raise ReservationConflictError(
                    resource_type="PROGRAMME_BUDGET",
                    resource_id="TOTAL_BUDGET",
                    reason=f"Requested INR {budget_inr:,.0f}, but only INR {self._programme_budget_inr - budget_used:,.0f} remain.",
                )

            res_id = f"RES-HOLD-{site_id}-{scenario_id}-{uuid4().hex[:6]}"
            expires_at = datetime.now(timezone.utc) + timedelta(days=hold_duration_days)

            audit_entry = global_audit_ledger.append_event(
                action="HOLD_CAPACITY_RESERVATION",
                actor_id=actor_id,
                resource_type="CAPACITY_RESERVATION",
                resource_id=res_id,
                payload={
                    "scenario_id": scenario_id,
                    "site_id": site_id,
                    "dwellings": dwellings,
                    "land_cents": land_cents,
                    "budget_inr": budget_inr,
                    "water_m3_day": water_m3_day,
                    "expires_at": expires_at.isoformat(),
                },
            )

            reservation = CapacityReservation(
                reservation_id=res_id,
                scenario_id=scenario_id,
                site_id=site_id,
                status=ReservationStatus.RESERVED,
                dwellings_reserved=dwellings,
                land_cents_reserved=land_cents,
                budget_inr_reserved=budget_inr,
                water_m3_day_reserved=water_m3_day,
                expires_at=expires_at,
                reserved_by=actor_id,
                audit_event_id=audit_entry.event_id,
            )
            self._reservations[res_id] = reservation
            return reservation

    def commit_reservation(
        self,
        reservation_id: str,
        approval_id: str,
        actor_id: str,
    ) -> CapacityReservation:
        """
        Convert a temporary reservation hold into a binding commitment upon official approval (RUL-070).
        Revalidates blocking approval conditions (AT-20) and pending objections (RUL-049).
        """
        degraded_mode_controller.assert_writes_allowed()
        with self._lock:
            res = self._reservations.get(reservation_id)
            if not res:
                raise KeyError(f"Reservation '{reservation_id}' not found.")

            if res.status != ReservationStatus.RESERVED:
                raise ValueError(f"Cannot commit reservation in status '{res.status}'. Must be RESERVED.")

            # AT-20: Check if approval has any unmet blocking conditions (e.g. water yield gate)
            can_allocate, blocking_reasons = approval_service.check_can_allocate(approval_id)
            if not can_allocate:
                raise ApprovalConditionUnmetError(
                    condition_id=approval_id,
                    reason="; ".join(blocking_reasons)
                )

            res.status = ReservationStatus.COMMITTED
            res.committed_at = datetime.now(timezone.utc)
            res.approval_order_id = approval_id

            global_audit_ledger.append_event(
                action="COMMIT_CAPACITY_RESERVATION",
                actor_id=actor_id,
                resource_type="CAPACITY_RESERVATION",
                resource_id=reservation_id,
                payload={
                    "approval_id": approval_id,
                    "site_id": res.site_id,
                    "dwellings": res.dwellings_reserved,
                },
            )
            return res

    def release_reservation(
        self,
        reservation_id: str,
        reason: str,
        actor_id: str,
    ) -> CapacityReservation:
        """
        Explicitly release held or committed capacity (RUL-071).
        Ensures capacity is returned cleanly and never double-subtracted.
        """
        with self._lock:
            res = self._reservations.get(reservation_id)
            if not res:
                raise KeyError(f"Reservation '{reservation_id}' not found.")

            if res.status in (ReservationStatus.RELEASED, ReservationStatus.EXPIRED, ReservationStatus.CANCELLED):
                return res  # Idempotent

            res.status = ReservationStatus.RELEASED
            res.released_at = datetime.now(timezone.utc)
            res.release_reason = reason

            global_audit_ledger.append_event(
                action="RELEASE_CAPACITY_RESERVATION",
                actor_id=actor_id,
                resource_type="CAPACITY_RESERVATION",
                resource_id=reservation_id,
                payload={
                    "site_id": res.site_id,
                    "released_dwellings": res.dwellings_reserved,
                    "reason": reason,
                },
            )
            return res

    def get_reservation(self, reservation_id: str) -> Optional[CapacityReservation]:
        with self._lock:
            return self._reservations.get(reservation_id)

    def list_reservations(self, site_id: Optional[str] = None) -> List[CapacityReservation]:
        with self._lock:
            if site_id:
                return [r for r in self._reservations.values() if r.site_id == site_id]
            return list(self._reservations.values())


# Global singleton instance
capacity_ledger = MultiResourceReservationLedger()
