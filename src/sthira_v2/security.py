"""Small enforceable privacy and observability primitives for the v2 boundary."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from html import escape
import re
from uuid import uuid4


MAX_REQUEST_BYTES = 1_000_000
MAX_AUDIO_BYTES = 10_000_000
_SAFE_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$")


def new_request_id() -> str:
    return f"req-{uuid4().hex}"


def validate_public_identifier(value: str) -> str:
    if not isinstance(value, str) or not _SAFE_ID.fullmatch(value):
        raise ValueError("invalid public identifier")
    return value


def safe_imported_text(value: str, *, max_length: int = 20_000) -> str:
    if not isinstance(value, str) or len(value) > max_length:
        raise ValueError("imported text exceeds limit")
    return escape(value, quote=True)


@dataclass(frozen=True)
class PrivacyRecord:
    session_id: str
    expires_at: datetime
    location: tuple[float, float] | None = None
    raw_voice: bytes | None = None


class PrivacyStore:
    """Retains only bounded session data and deletes raw voice on request/expiry."""

    def __init__(self) -> None:
        self._sessions: dict[str, PrivacyRecord] = {}

    def put(self, record: PrivacyRecord) -> None:
        if record.expires_at.tzinfo is None:
            raise ValueError("expiry must be timezone-aware")
        validate_public_identifier(record.session_id)
        self._sessions[record.session_id] = record

    def purge_expired(self, *, now: datetime) -> int:
        if now.tzinfo is None:
            raise ValueError("now must be timezone-aware")
        expired = [key for key, value in self._sessions.items() if value.expires_at <= now]
        for key in expired:
            del self._sessions[key]
        return len(expired)

    def delete_raw_voice(self, session_id: str) -> bool:
        record = self._sessions.get(session_id)
        if record is None or record.raw_voice is None:
            return False
        self._sessions[session_id] = PrivacyRecord(record.session_id, record.expires_at, record.location, None)
        return True

    def get(self, session_id: str) -> PrivacyRecord | None:
        return self._sessions.get(session_id)


def is_utc(value: datetime) -> bool:
    return value.tzinfo is not None and value.utcoffset() == timezone.utc.utcoffset(value)
