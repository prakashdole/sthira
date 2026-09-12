from concurrent.futures import ThreadPoolExecutor

import pytest

from sthira_v2.allocation import AssignmentUnavailable, FacilityCapacity, InMemoryAllocationService
from sthira_v2.contracts import ArrivalResponse, AssignmentState


def service() -> InMemoryAllocationService:
    value = InMemoryAllocationService(("first", "second"))
    value.register_facility(FacilityCapacity("first", 1, 2, 2))
    value.register_facility(FacilityCapacity("second", 1, 3, 3))
    return value


def test_assignment_uses_only_policy_order_and_never_overbooks():
    value = service()

    assert value.assign("a1", 2).facility_id == "first"
    assert value.assign("a2", 2).facility_id == "second"
    with pytest.raises(AssignmentUnavailable):
        value.assign("a3", 2)
    assert value.remaining("first") == 0


def test_arrival_is_idempotent_and_no_does_not_change_occupancy():
    value = service()
    value.assign("a1", 1)

    assert value.confirm_arrival("a1", ArrivalResponse.NO).state is AssignmentState.RESERVED
    first = value.confirm_arrival("a1", ArrivalResponse.YES)
    second = value.confirm_arrival("a1", ArrivalResponse.YES)
    assert first == second
    assert value.remaining("first") == 1


def test_concurrent_last_capacity_assignments_have_one_winner():
    value = InMemoryAllocationService(("first",))
    value.register_facility(FacilityCapacity("first", 1, 1, 1))

    with ThreadPoolExecutor(max_workers=8) as pool:
        results = list(pool.map(lambda index: _try_assign(value, f"a{index}"), range(8)))

    assert sum(result is not None for result in results) == 1
    assert value.remaining("first") == 0


def _try_assign(service: InMemoryAllocationService, assignment_id: str):
    try:
        return service.assign(assignment_id, 1)
    except AssignmentUnavailable:
        return None
