"""Persistence-independent, tamper-evident audit contracts.

The repository protocol is the production boundary.  The deterministic
implementation in this module is intentionally only suitable for tests and demos.
"""

from __future__ import annotations

import hashlib
import json
from datetime import datetime, timezone
from typing import Mapping, Protocol, Sequence

from pydantic import Field, field_validator

from .contracts import ContractModel, Identifier


GENESIS_HASH = "0" * 64


def _utc(value: datetime) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("timestamp must be timezone-aware")
    if value.utcoffset() != timezone.utc.utcoffset(value):
        raise ValueError("timestamp must use UTC")
    return value


class AuditIntegrityError(RuntimeError):
    """Raised when an audit chain is discontinuous or has been modified."""


class AuditEvent(ContractModel):
    event_id: Identifier = Field(min_length=1, max_length=200)
    occurred_at: datetime
    actor_id: Identifier = Field(min_length=1, max_length=200)
    action: str = Field(min_length=1, max_length=200)
    subject_type: str = Field(min_length=1, max_length=200)
    subject_id: Identifier = Field(min_length=1, max_length=200)
    reason: str = Field(min_length=1, max_length=4000)
    attributes: dict[str, str] = Field(default_factory=dict)
    previous_hash: str = Field(pattern=r"^[0-9a-f]{64}$")
    event_hash: str = Field(pattern=r"^[0-9a-f]{64}$")

    _timestamp = field_validator("occurred_at")(_utc)


def compute_event_hash(
    *,
    event_id: str,
    occurred_at: datetime,
    actor_id: str,
    action: str,
    subject_type: str,
    subject_id: str,
    reason: str,
    attributes: Mapping[str, str],
    previous_hash: str,
) -> str:
    """Hash the canonical event representation, including chain linkage."""
    canonical = json.dumps(
        {
            "action": action,
            "actor_id": actor_id,
            "attributes": dict(attributes),
            "event_id": event_id,
            "occurred_at": occurred_at.isoformat(),
            "previous_hash": previous_hash,
            "reason": reason,
            "subject_id": subject_id,
            "subject_type": subject_type,
        },
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    )
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


class AuditRepository(Protocol):
    """Append-only persistence port; implementations must serialize appends."""

    def head_hash(self) -> str: ...

    def append(self, event: AuditEvent) -> None: ...

    def events(self) -> Sequence[AuditEvent]: ...


def verify_chain(events: Sequence[AuditEvent]) -> bool:
    expected_previous = GENESIS_HASH
    seen: set[str] = set()
    for event in events:
        if event.event_id in seen or event.previous_hash != expected_previous:
            raise AuditIntegrityError(f"invalid audit linkage at {event.event_id}")
        expected = compute_event_hash(
            event_id=event.event_id,
            occurred_at=event.occurred_at,
            actor_id=event.actor_id,
            action=event.action,
            subject_type=event.subject_type,
            subject_id=event.subject_id,
            reason=event.reason,
            attributes=event.attributes,
            previous_hash=event.previous_hash,
        )
        if event.event_hash != expected:
            raise AuditIntegrityError(f"invalid audit hash at {event.event_id}")
        seen.add(event.event_id)
        expected_previous = event.event_hash
    return True


class DeterministicAuditRepository:
    """Deterministic unit repository for tests/demos; not production storage."""

    def __init__(self) -> None:
        self._events: list[AuditEvent] = []

    def head_hash(self) -> str:
        return self._events[-1].event_hash if self._events else GENESIS_HASH

    def append(self, event: AuditEvent) -> None:
        if any(existing.event_id == event.event_id for existing in self._events):
            raise ValueError(f"duplicate audit event: {event.event_id}")
        if event.previous_hash != self.head_hash():
            raise AuditIntegrityError("audit append does not extend current head")
        verify_chain((*self._events, event))
        self._events.append(event)

    def events(self) -> tuple[AuditEvent, ...]:
        return tuple(self._events)


class AuditService:
    def __init__(self, repository: AuditRepository) -> None:
        self._repository = repository

    def record(
        self,
        *,
        event_id: str,
        occurred_at: datetime,
        actor_id: str,
        action: str,
        subject_type: str,
        subject_id: str,
        reason: str,
        attributes: Mapping[str, str] | None = None,
    ) -> AuditEvent:
        values = dict(attributes or {})
        previous_hash = self._repository.head_hash()
        event_hash = compute_event_hash(
            event_id=event_id,
            occurred_at=occurred_at,
            actor_id=actor_id,
            action=action,
            subject_type=subject_type,
            subject_id=subject_id,
            reason=reason,
            attributes=values,
            previous_hash=previous_hash,
        )
        event = AuditEvent(
            event_id=event_id,
            occurred_at=occurred_at,
            actor_id=actor_id,
            action=action,
            subject_type=subject_type,
            subject_id=subject_id,
            reason=reason,
            attributes=values,
            previous_hash=previous_hash,
            event_hash=event_hash,
        )
        self._repository.append(event)
        return event
