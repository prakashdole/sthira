"""Small transactional repositories; callers own commit/rollback boundaries."""

from __future__ import annotations

import hashlib
import json
from datetime import datetime, timezone
from typing import Generic, TypeVar

from sqlalchemy import Select, select
from sqlalchemy.orm import Session

from .models import AuditEvent, Base, VersionedFactMixin


FactT = TypeVar("FactT", bound=VersionedFactMixin)


class VersionedFactRepository(Generic[FactT]):
    def __init__(self, session: Session, model: type[FactT]) -> None:
        self.session = session
        self.model = model

    def add_version(self, fact: FactT) -> FactT:
        current = self.session.scalar(
            select(self.model).where(self.model.fact_id == fact.fact_id, self.model.system_until.is_(None)).with_for_update()
        )
        if current is not None:
            current_system_from = current.system_from
            if current_system_from.tzinfo is None:
                current_system_from = current_system_from.replace(tzinfo=timezone.utc)
            if fact.version <= current.version or fact.system_from <= current_system_from:
                raise ValueError("new fact version and system time must increase")
            current.system_until = fact.system_from
        self.session.add(fact)
        self.session.flush()
        return fact

    def as_known_at(self, fact_id: str, effective_at: datetime, system_at: datetime) -> FactT | None:
        statement: Select[tuple[FactT]] = select(self.model).where(
            self.model.fact_id == fact_id,
            self.model.effective_from <= effective_at,
            self.model.effective_until > effective_at,
            self.model.system_from <= system_at,
            (self.model.system_until.is_(None) | (self.model.system_until > system_at)),
        ).order_by(self.model.version.desc())
        return self.session.scalar(statement)


class AuditRepository:
    GENESIS_HASH = "0" * 64

    def __init__(self, session: Session) -> None:
        self.session = session

    @staticmethod
    def _digest(event: AuditEvent) -> str:
        occurred_at = event.occurred_at
        if occurred_at.tzinfo is None:
            occurred_at = occurred_at.replace(tzinfo=timezone.utc)
        data = {
            "event_id": event.event_id, "request_id": event.request_id,
            "actor_or_source": event.actor_or_source, "action": event.action,
            "object_type": event.object_type, "object_id": event.object_id,
            "object_version": event.object_version, "occurred_at": occurred_at.isoformat(),
            "result": event.result, "details": event.details, "previous_hash": event.previous_hash,
        }
        return hashlib.sha256(json.dumps(data, sort_keys=True, separators=(",", ":")).encode()).hexdigest()

    def append(self, event: AuditEvent) -> AuditEvent:
        previous = self.session.scalar(select(AuditEvent).order_by(AuditEvent.sequence.desc()).limit(1).with_for_update())
        event.previous_hash = previous.event_hash if previous else self.GENESIS_HASH
        event.event_hash = self._digest(event)
        self.session.add(event)
        self.session.flush()
        return event

    def verify(self) -> bool:
        previous_hash = self.GENESIS_HASH
        for event in self.session.scalars(select(AuditEvent).order_by(AuditEvent.sequence)):
            if event.previous_hash != previous_hash or event.event_hash != self._digest(event):
                return False
            previous_hash = event.event_hash
        return True
