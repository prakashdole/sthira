from datetime import datetime, timedelta, timezone

import pytest

from sthira_v2.audit import (
    AuditIntegrityError,
    AuditService,
    DeterministicAuditRepository,
    verify_chain,
)
from sthira_v2.source_activation import (
    DeterministicSourceRepository,
    SourceActivationService,
    SourceActivationState,
    SourceRecord,
)


NOW = datetime(2026, 9, 11, 6, tzinfo=timezone.utc)


def services():
    audit_repository = DeterministicAuditRepository()
    sources = DeterministicSourceRepository()
    return SourceActivationService(sources, AuditService(audit_repository)), audit_repository


def source():
    return SourceRecord(
        source_id="GOV-001",
        government_owner="Synthetic DDMA",
        official_domain="example.invalid",
        updated_at=NOW,
    )


def test_audit_is_deterministic_append_only_and_tamper_evident():
    repository = DeterministicAuditRepository()
    audit = AuditService(repository)
    first = audit.record(
        event_id="event-1", occurred_at=NOW, actor_id="operator-1",
        action="DISCOVER", subject_type="SourceRecord", subject_id="GOV-001",
        reason="register source",
    )
    second = audit.record(
        event_id="event-2", occurred_at=NOW + timedelta(seconds=1), actor_id="operator-1",
        action="REQUEST_ACCESS", subject_type="SourceRecord", subject_id="GOV-001",
        reason="begin authorization",
    )
    assert second.previous_hash == first.event_hash
    assert verify_chain(repository.events()) is True
    tampered = first.model_copy(update={"reason": "rewritten"})
    with pytest.raises(AuditIntegrityError):
        verify_chain((tampered, second))
    with pytest.raises(ValueError, match="duplicate"):
        repository.append(second)


def test_source_must_follow_activation_gate_and_only_operational_is_usable():
    service, audit_repository = services()
    current = service.discover(source())
    with pytest.raises(PermissionError):
        service.require_operational(current.source_id)
    path = [
        SourceActivationState.ACCESS_REQUESTED,
        SourceActivationState.SAMPLE_ACQUIRED,
        SourceActivationState.VALIDATED,
        SourceActivationState.AUTHORIZED,
        SourceActivationState.OPERATIONAL,
    ]
    for index, target in enumerate(path, start=1):
        current = service.transition(
            current.source_id, target, actor_id="owner-1", reason=f"gate {index} passed",
            occurred_at=NOW + timedelta(minutes=index), event_id=f"transition-{index}",
        )
    assert service.require_operational(current.source_id) == current
    assert current.version == 6
    assert len(audit_repository.events()) == 5


def test_skipping_gate_and_leaving_retired_are_rejected():
    service, _ = services()
    service.discover(source())
    with pytest.raises(ValueError, match="illegal source transition"):
        service.transition(
            "GOV-001", SourceActivationState.AUTHORIZED, actor_id="owner-1",
            reason="skip", occurred_at=NOW, event_id="bad-1",
        )


def test_operational_source_can_suspend_and_resume():
    service, _ = services()
    current = service.discover(source())
    path = list(SourceActivationState)[1:6]
    for index, target in enumerate(path, start=1):
        current = service.transition(
            current.source_id, target, actor_id="owner-1", reason="approved",
            occurred_at=NOW + timedelta(minutes=index), event_id=f"event-{index}",
        )
    suspended = service.transition(
        current.source_id, SourceActivationState.SUSPENDED, actor_id="owner-1",
        reason="outage", occurred_at=NOW + timedelta(hours=1), event_id="suspend",
    )
    with pytest.raises(PermissionError):
        service.require_operational(suspended.source_id)
    resumed = service.transition(
        suspended.source_id, SourceActivationState.OPERATIONAL, actor_id="owner-1",
        reason="revalidated", occurred_at=NOW + timedelta(hours=2), event_id="resume",
    )
    assert resumed.may_drive_citizen_guidance
