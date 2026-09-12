"""Source-stamped emergency cache and explicit dialler handoff helpers."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from threading import Lock


@dataclass(frozen=True)
class CachedGuidance:
    alert_id: str
    source_id: str
    version: str
    expires_at: datetime
    route_steps: tuple[str, ...]
    emergency_number: str

    def state(self, now: datetime) -> str:
        if now.tzinfo is None or now.utcoffset() != timezone.utc.utcoffset(now):
            raise ValueError("now must be timezone-aware UTC")
        return "CURRENT" if now <= self.expires_at else "EXPIRED"


def dialler_uri(number: str, *, explicit_confirmation: bool) -> str | None:
    """Return a device dialler URI only after an explicit citizen action."""
    if not explicit_confirmation:
        return None
    if not number.isdigit() or not 3 <= len(number) <= 15:
        raise ValueError("emergency number must be digits")
    return f"tel:{number}"


class OfflineGuidanceCache:
    """Small in-memory seam mirroring browser-cache expiry and cancellation rules."""

    def __init__(self) -> None:
        self._items: dict[str, CachedGuidance] = {}
        self._lock = Lock()

    def put(self, key: str, guidance: CachedGuidance) -> None:
        if not key.strip():
            raise ValueError("cache key is required")
        with self._lock:
            self._items[key] = guidance

    def get(self, key: str, *, now: datetime) -> CachedGuidance | None:
        with self._lock:
            guidance = self._items.get(key)
            if guidance is None or guidance.state(now) == "EXPIRED":
                return None
            return guidance

    def invalidate(self, key: str) -> bool:
        with self._lock:
            return self._items.pop(key, None) is not None

    def invalidate_alert(self, alert_id: str) -> int:
        with self._lock:
            keys = [key for key, value in self._items.items() if value.alert_id == alert_id]
            for key in keys:
                del self._items[key]
            return len(keys)
