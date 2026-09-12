"""Atomic, idempotent assignment and arrival logic for the demo boundary."""

from __future__ import annotations

from dataclasses import dataclass
from threading import Lock

from .contracts import ArrivalResponse, AssignmentState


class AssignmentUnavailable(RuntimeError):
    """No published facility can satisfy the authority policy and party size."""


@dataclass
class FacilityCapacity:
    facility_id: str
    version: int
    total_capacity: int
    remaining: int
    open: bool = True


@dataclass(frozen=True)
class Allocation:
    assignment_id: str
    facility_id: str
    party_size: int
    state: AssignmentState
    alert_id: str = "ALERT-DEMO-2026-09-12"
    citizen_session_id: str = "synthetic-demo-session"
    idempotency_key: str = "synthetic-demo-assignment"


class InMemoryAllocationService:
    """Thread-safe local seam mirroring the production transaction invariant."""

    def __init__(self, policy_order: tuple[str, ...]) -> None:
        self._policy_order = policy_order
        self._facilities: dict[str, FacilityCapacity] = {}
        self._assignments: dict[str, Allocation] = {}
        self._arrival_results: dict[str, Allocation] = {}
        self._assignment_fingerprints: dict[str, tuple[str, str, int, str]] = {}
        self._lock = Lock()

    def register_facility(self, facility: FacilityCapacity) -> None:
        if facility.remaining < 0 or facility.remaining > facility.total_capacity:
            raise ValueError("facility remaining capacity is outside its bounds")
        with self._lock:
            self._facilities[facility.facility_id] = facility

    def assign(
        self,
        assignment_id: str,
        party_size: int,
        *,
        alert_id: str = "ALERT-DEMO-2026-09-12",
        citizen_session_id: str = "synthetic-demo-session",
        idempotency_key: str | None = None,
    ) -> Allocation:
        if party_size < 1 or party_size > 50:
            raise ValueError("party size must be between 1 and 50")
        with self._lock:
            existing = self._assignments.get(assignment_id)
            if existing is not None:
                fingerprint = (alert_id, citizen_session_id, party_size, idempotency_key or assignment_id)
                if self._assignment_fingerprints[assignment_id] != fingerprint:
                    raise ValueError("assignment idempotency payload conflict")
                return existing
            for facility_id in self._policy_order:
                facility = self._facilities.get(facility_id)
                if facility is None or not facility.open or facility.remaining < party_size:
                    continue
                facility.remaining -= party_size
                allocation = Allocation(
                    assignment_id, facility_id, party_size, AssignmentState.RESERVED,
                    alert_id, citizen_session_id, idempotency_key or assignment_id,
                )
                self._assignments[assignment_id] = allocation
                self._assignment_fingerprints[assignment_id] = (alert_id, citizen_session_id, party_size, idempotency_key or assignment_id)
                return allocation
        raise AssignmentUnavailable("ASSIGNMENT_UNAVAILABLE")

    def confirm_arrival(self, assignment_id: str, response: ArrivalResponse, *, idempotency_key: str | None = None) -> Allocation:
        with self._lock:
            assignment = self._assignments.get(assignment_id)
            if assignment is None:
                raise KeyError(assignment_id)
            if response is ArrivalResponse.NO:
                return assignment
            previous = self._arrival_results.get(assignment_id)
            if previous is not None:
                return previous
            arrived = Allocation(
                assignment.assignment_id,
                assignment.facility_id,
                assignment.party_size,
                AssignmentState.ARRIVED,
                assignment.alert_id,
                assignment.citizen_session_id,
                idempotency_key or assignment.assignment_id,
            )
            self._arrival_results[assignment_id] = arrived
            return arrived

    def remaining(self, facility_id: str) -> int:
        with self._lock:
            return self._facilities[facility_id].remaining

    def get(self, assignment_id: str) -> Allocation | None:
        with self._lock:
            return self._assignments.get(assignment_id)
