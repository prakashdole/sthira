"""
Sthira Tamper-Evident Append-Only Audit Logger (ARC-C11).
Normative Reference: rules.md (RUL-056, RUL-057) and architecture.md §7.3.
"""

import hashlib
import json
from datetime import datetime, timezone
from typing import List, Optional
from uuid import uuid4

from sthira.core.contracts import AuditEntry
from sthira.core.errors import AuditIntegrityError

GENESIS_PREV_HASH = "0" * 64


def compute_event_hash(
    prev_hash: str,
    actor_id: str,
    authority_scope: str,
    action: str,
    entity_type: str,
    entity_id: str,
    version_id: str,
    timestamp_iso: str,
    reason: str,
    correlation_id: str,
) -> str:
    """Compute deterministic SHA-256 hash of event fields."""
    canonical_str = json.dumps(
        {
            "prev_hash": prev_hash,
            "actor_id": actor_id,
            "authority_scope": authority_scope,
            "action": action,
            "entity_type": entity_type,
            "entity_id": entity_id,
            "version_id": version_id,
            "timestamp": timestamp_iso,
            "reason": reason,
            "correlation_id": correlation_id,
        },
        sort_keys=True,
    )
    return hashlib.sha256(canonical_str.encode("utf-8")).hexdigest()


class AuditLedger:
    """
    Append-only in-memory/persisted audit ledger with SHA-256 hash chaining.
    """

    def __init__(self):
        self._entries: List[AuditEntry] = []

    @property
    def entries(self) -> List[AuditEntry]:
        return list(self._entries)

    @property
    def head_hash(self) -> str:
        if not self._entries:
            return GENESIS_PREV_HASH
        return self._entries[-1].event_hash

    def log(
        self,
        actor_id: str,
        authority_scope: str,
        action: str,
        entity_type: str,
        entity_id: str,
        version_id: str,
        reason: str,
        correlation_id: Optional[str] = None,
        timestamp: Optional[datetime] = None,
    ) -> AuditEntry:
        """Atomically append a tamper-evident audit record."""
        ts = timestamp or datetime.now(timezone.utc)
        ts_iso = ts.isoformat()
        corr_id = correlation_id or str(uuid4())
        prev_h = self.head_hash

        curr_hash = compute_event_hash(
            prev_hash=prev_h,
            actor_id=actor_id,
            authority_scope=authority_scope,
            action=action,
            entity_type=entity_type,
            entity_id=entity_id,
            version_id=version_id,
            timestamp_iso=ts_iso,
            reason=reason,
            correlation_id=corr_id,
        )

        entry = AuditEntry(
            event_id=str(uuid4()),
            timestamp=ts,
            actor_id=actor_id,
            authority_scope=authority_scope,
            action=action,
            entity_type=entity_type,
            entity_id=entity_id,
            version_id=version_id,
            prev_hash=prev_h,
            event_hash=curr_hash,
            reason=reason,
            correlation_id=corr_id,
        )
        self._entries.append(entry)
        return entry

    def append_event(
        self,
        action: str,
        actor_id: str,
        resource_type: str,
        resource_id: str,
        payload: Optional[dict] = None,
        authority_scope: str = "GLOBAL",
        version_id: str = "1.0",
        reason: Optional[str] = None,
    ) -> AuditEntry:
        """Compatibility helper mapping append_event calls to log."""
        reason_str = reason or (json.dumps(payload, sort_keys=True) if payload else "")
        return self.log(
            actor_id=actor_id,
            authority_scope=authority_scope,
            action=action,
            entity_type=resource_type,
            entity_id=resource_id,
            version_id=version_id,
            reason=reason_str,
        )

    def verify_integrity(self) -> bool:
        """
        Cryptographically verify the entire chain from genesis to head.
        Raises AuditIntegrityError if any modification is detected.
        """
        expected_prev_hash = GENESIS_PREV_HASH

        for idx, entry in enumerate(self._entries):
            if entry.prev_hash != expected_prev_hash:
                raise AuditIntegrityError(
                    event_id=entry.event_id,
                    expected_hash=expected_prev_hash,
                    actual_hash=entry.prev_hash,
                )

            recomputed_hash = compute_event_hash(
                prev_hash=entry.prev_hash,
                actor_id=entry.actor_id,
                authority_scope=entry.authority_scope,
                action=entry.action,
                entity_type=entry.entity_type,
                entity_id=entry.entity_id,
                version_id=entry.version_id,
                timestamp_iso=entry.timestamp.isoformat(),
                reason=entry.reason,
                correlation_id=entry.correlation_id,
            )

            if entry.event_hash != recomputed_hash:
                raise AuditIntegrityError(
                    event_id=entry.event_id,
                    expected_hash=recomputed_hash,
                    actual_hash=entry.event_hash,
                )

            expected_prev_hash = entry.event_hash

        return True


# Global default in-process audit ledger singleton
global_audit_ledger = AuditLedger()
