"""Government source activation lifecycle and persistence ports."""

from __future__ import annotations

from datetime import datetime, timezone
from enum import StrEnum
from typing import Protocol, Sequence

from pydantic import Field, field_validator

from .audit import AuditService
from .contracts import ContractModel, Identifier


def _utc(value: datetime) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("timestamp must be timezone-aware")
    if value.utcoffset() != timezone.utc.utcoffset(value):
        raise ValueError("timestamp must use UTC")
    return value


class SourceActivationState(StrEnum):
    DISCOVERED = "DISCOVERED"
    ACCESS_REQUESTED = "ACCESS_REQUESTED"
    SAMPLE_ACQUIRED = "SAMPLE_ACQUIRED"
    VALIDATED = "VALIDATED"
    AUTHORIZED = "AUTHORIZED"
    OPERATIONAL = "OPERATIONAL"
    SUSPENDED = "SUSPENDED"
    RETIRED = "RETIRED"


_TRANSITIONS = {
    SourceActivationState.DISCOVERED: {SourceActivationState.ACCESS_REQUESTED},
    SourceActivationState.ACCESS_REQUESTED: {SourceActivationState.SAMPLE_ACQUIRED},
    SourceActivationState.SAMPLE_ACQUIRED: {SourceActivationState.VALIDATED},
    SourceActivationState.VALIDATED: {SourceActivationState.AUTHORIZED},
    SourceActivationState.AUTHORIZED: {SourceActivationState.OPERATIONAL},
    SourceActivationState.OPERATIONAL: {
        SourceActivationState.SUSPENDED,
        SourceActivationState.RETIRED,
    },
    SourceActivationState.SUSPENDED: {
        SourceActivationState.OPERATIONAL,
        SourceActivationState.RETIRED,
    },
}


class SourceRecord(ContractModel):
    source_id: Identifier = Field(min_length=1, max_length=200)
    government_owner: str = Field(min_length=1, max_length=300)
    official_domain: str = Field(min_length=1, max_length=300)
    state: SourceActivationState = SourceActivationState.DISCOVERED
    version: int = Field(default=1, ge=1)
    updated_at: datetime

    _timestamp = field_validator("updated_at")(_utc)

    @property
    def may_drive_citizen_guidance(self) -> bool:
        return self.state is SourceActivationState.OPERATIONAL


class SourceRepository(Protocol):
    """Persistence port for source activation state."""

    def get(self, source_id: str) -> SourceRecord | None: ...

    def add(self, source: SourceRecord) -> None: ...

    def replace(self, source: SourceRecord, *, expected_version: int) -> None: ...

    def list(self) -> Sequence[SourceRecord]: ...


class SourceNotFoundError(LookupError):
    pass


class SourceVersionConflictError(RuntimeError):
    pass


class DeterministicSourceRepository:
    """Deterministic unit repository for tests/demos; not production storage."""

    def __init__(self) -> None:
        self._sources: dict[str, SourceRecord] = {}

    def get(self, source_id: str) -> SourceRecord | None:
        return self._sources.get(source_id)

    def add(self, source: SourceRecord) -> None:
        if source.source_id in self._sources:
            raise ValueError(f"duplicate source: {source.source_id}")
        self._sources[source.source_id] = source

    def replace(self, source: SourceRecord, *, expected_version: int) -> None:
        current = self._sources.get(source.source_id)
        if current is None:
            raise SourceNotFoundError(source.source_id)
        if current.version != expected_version:
            raise SourceVersionConflictError(source.source_id)
        self._sources[source.source_id] = source

    def list(self) -> tuple[SourceRecord, ...]:
        return tuple(self._sources[key] for key in sorted(self._sources))


class SourceActivationService:
    def __init__(self, repository: SourceRepository, audit: AuditService) -> None:
        self._repository = repository
        self._audit = audit

    def discover(self, source: SourceRecord) -> SourceRecord:
        if source.state is not SourceActivationState.DISCOVERED:
            raise ValueError("new sources must begin in DISCOVERED")
        self._repository.add(source)
        return source

    def transition(
        self,
        source_id: str,
        target: SourceActivationState,
        *,
        actor_id: str,
        reason: str,
        occurred_at: datetime,
        event_id: str,
    ) -> SourceRecord:
        current = self._repository.get(source_id)
        if current is None:
            raise SourceNotFoundError(source_id)
        if target not in _TRANSITIONS.get(current.state, set()):
            raise ValueError(f"illegal source transition: {current.state.value} -> {target.value}")
        updated = current.model_copy(
            update={"state": target, "version": current.version + 1, "updated_at": occurred_at}
        )
        self._repository.replace(updated, expected_version=current.version)
        self._audit.record(
            event_id=event_id,
            occurred_at=occurred_at,
            actor_id=actor_id,
            action="SOURCE_STATE_TRANSITION",
            subject_type="SourceRecord",
            subject_id=source_id,
            reason=reason,
            attributes={"from_state": current.state.value, "to_state": target.value},
        )
        return updated

    def require_operational(self, source_id: str) -> SourceRecord:
        source = self._repository.get(source_id)
        if source is None:
            raise SourceNotFoundError(source_id)
        if not source.may_drive_citizen_guidance:
            raise PermissionError(f"source {source_id} is not OPERATIONAL")
        return source
