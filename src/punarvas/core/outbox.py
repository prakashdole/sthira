"""
PUNARVAS-AI Transactional Outbox & Event Dispatcher (ARC-C11 / DEC-027).
Normative Reference: architecture.md §6 and trd.md §3.
"""

from datetime import datetime, timezone
from enum import Enum
from typing import Any, Callable, Dict, List, Optional
from uuid import uuid4
from pydantic import BaseModel, Field


class OutboxStatus(str, Enum):
    PENDING = "PENDING"
    PUBLISHED = "PUBLISHED"
    FAILED = "FAILED"
    DEAD_LETTER = "DEAD_LETTER"


class OutboxMessage(BaseModel):
    message_id: str = Field(default_factory=lambda: str(uuid4()))
    topic: str
    payload: Dict[str, Any]
    idempotency_key: str
    status: OutboxStatus = OutboxStatus.PENDING
    retry_count: int = 0
    max_retries: int = 3
    created_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    published_at: Optional[datetime] = None
    error_message: Optional[str] = None


class TransactionalOutbox:
    """
    In-memory / PostGIS transactional outbox for reliable asynchronous messaging.
    """

    def __init__(self):
        self._messages: List[OutboxMessage] = []
        self._idempotency_seen: set = set()
        self._handlers: Dict[str, List[Callable[[OutboxMessage], None]]] = {}

    def enqueue(self, topic: str, payload: Dict[str, Any], idempotency_key: str) -> OutboxMessage:
        """Enqueue message atomically with deduplication protection."""
        if idempotency_key in self._idempotency_seen:
            # Idempotent match: return existing message if found
            for msg in self._messages:
                if msg.idempotency_key == idempotency_key:
                    return msg

        msg = OutboxMessage(
            topic=topic,
            payload=payload,
            idempotency_key=idempotency_key,
        )
        self._messages.append(msg)
        self._idempotency_seen.add(idempotency_key)
        return msg

    def register_handler(self, topic: str, handler: Callable[[OutboxMessage], None]):
        """Register subscriber for given topic."""
        if topic not in self._handlers:
            self._handlers[topic] = []
        self._handlers[topic].append(handler)

    def relay_pending(self) -> int:
        """
        Relay all PENDING messages to registered topic handlers.
        Returns count of successfully published messages.
        """
        published_count = 0

        for msg in self._messages:
            if msg.status == OutboxStatus.PENDING:
                handlers = self._handlers.get(msg.topic, [])
                try:
                    for h in handlers:
                        h(msg)
                    msg.status = OutboxStatus.PUBLISHED
                    msg.published_at = datetime.now(timezone.utc)
                    published_count += 1
                except Exception as ex:
                    msg.retry_count += 1
                    msg.error_message = str(ex)
                    if msg.retry_count >= msg.max_retries:
                        msg.status = OutboxStatus.DEAD_LETTER
                    else:
                        msg.status = OutboxStatus.FAILED

        return published_count

    def get_pending(self) -> List[OutboxMessage]:
        return [m for m in self._messages if m.status == OutboxStatus.PENDING]

    def get_dead_letters(self) -> List[OutboxMessage]:
        return [m for m in self._messages if m.status == OutboxStatus.DEAD_LETTER]


# Global outbox singleton
global_outbox = TransactionalOutbox()
