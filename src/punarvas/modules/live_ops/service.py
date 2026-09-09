"""
Phase 3 Live Operations and Platform Hardening Service (C3-01, C3-02).
Normative Reference: phases.md §6 & §12.6, architecture.md §9.2 & §12.3, rules.md (RUL-001, RUL-004, RUL-006, RUL-054).
"""

import hashlib
import json
import secrets
import time
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from punarvas.core.audit import global_audit_ledger
from punarvas.core.contracts import UserContext
from punarvas.core.enums import AuthorityState, ClassificationLevel, RoleType
from punarvas.core.errors import DegradedModeError


class StepUpToken(BaseModel):
    token_id: str
    user_id: str
    target_action: str
    expires_at_epoch: float
    nonce: str
    consumed: bool = False
    verified: bool = True
    sha256_hash: str


class BreakGlassSession(BaseModel):
    session_id: str
    user_id: str
    reason: str
    justification_category: str
    approved_by_authority: str
    issued_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    expires_at: datetime
    active: bool = True
    revoked: bool = False
    audit_event_id: str


class RestoreCheckItem(BaseModel):
    component: str
    expected_hash: str
    actual_hash: str
    status: str  # MATCH, MISMATCH, MISSING


class RestoreVerificationResult(BaseModel):
    status: str  # PASSED, FAILED
    can_resume_authoritative_writes: bool
    checks: List[RestoreCheckItem]
    verified_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    failure_reasons: List[str] = Field(default_factory=list)


class ManualDecisionRecord(BaseModel):
    manual_decision_id: str
    case_id: str
    jurisdiction_id: str
    approving_authority: str
    statutory_basis: str
    decision_summary: str
    paper_notice_reference: str
    signed_offline_time: datetime
    reconciled_system_time: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    reconciled_by_user_id: str
    authority_state: AuthorityState = AuthorityState.OFFICIALLY_APPROVED
    audit_event_id: str


class RollbackRecord(BaseModel):
    rollback_id: str
    programme_id: str
    initiated_by_user_id: str
    authority_order_reference: str
    reason: str
    status: str  # SUSPENDED_ROLLBACK
    initiated_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    reverted_to_manual: bool = True
    active_projections_revoked: int = 0
    read_only_retained: bool = True
    audit_event_id: str


class StepUpAuthManager:
    """Enforces MFA step-up verification for privileged and legal actions (RUL-054)."""

    PRIVILEGED_ACTIONS = {
        "APPROVE_DECISION",
        "PUBLISH_PROJECTION",
        "EXPORT_RESTRICTED_DATA",
        "ACTIVATE_POLICY",
        "BREAK_GLASS",
    }

    def __init__(self):
        self._tokens: Dict[str, StepUpToken] = {}

    def issue_step_up_token(self, user: UserContext, action: str, valid_seconds: int = 300) -> StepUpToken:
        if action not in self.PRIVILEGED_ACTIONS:
            raise ValueError(f"Action {action} is not a registered privileged step-up action.")
        token_str = secrets.token_hex(24)
        nonce = secrets.token_hex(16)
        expires_at = time.time() + valid_seconds
        token_hash = hashlib.sha256(f"{user.user_id}:{action}:{nonce}:{token_str}".encode()).hexdigest()

        rec = StepUpToken(
            token_id=token_str,
            user_id=user.user_id,
            target_action=action,
            expires_at_epoch=expires_at,
            nonce=nonce,
            consumed=False,
            verified=True,
            sha256_hash=token_hash,
        )
        self._tokens[token_str] = rec

        global_audit_ledger.append_event(
            action="MFA_STEP_UP_ISSUED",
            actor_id=user.user_id,
            resource_type="STEP_UP_TOKEN",
            resource_id=rec.token_id,
            payload={"action": action, "expires_at": expires_at},
        )
        return rec

    def verify_step_up(self, token_id: str, user_id: str, action: str) -> bool:
        rec = self._tokens.get(token_id)
        if not rec or rec.consumed:
            return False
        if rec.user_id != user_id or rec.target_action != action:
            return False
        if time.time() > rec.expires_at_epoch:
            return False
        rec.consumed = True
        return True


class BreakGlassManager:
    """Manages audited, time-bound emergency break-glass sessions (NFR-013)."""

    def __init__(self):
        self._sessions: Dict[str, BreakGlassSession] = {}

    def request_break_glass(
        self,
        user_id: str,
        reason: str,
        justification_category: str,
        approving_authority: str,
        duration_minutes: int = 60,
    ) -> BreakGlassSession:
        if not reason or len(reason.strip()) < 10:
            raise ValueError("Break-glass access requires a substantive justification (minimum 10 characters).")

        now = datetime.now(timezone.utc)
        expires = datetime.fromtimestamp(now.timestamp() + (duration_minutes * 60), tz=timezone.utc)
        session_id = f"BG-{secrets.token_hex(8).upper()}"

        audit_entry = global_audit_ledger.append_event(
            action="BREAK_GLASS_ACTIVATED",
            actor_id=user_id,
            resource_type="BREAK_GLASS_SESSION",
            resource_id=session_id,
            payload={
                "reason": reason,
                "category": justification_category,
                "authority": approving_authority,
                "duration_minutes": duration_minutes,
                "alert": "SECURITY_ALERT_BREAK_GLASS_ENGAGED",
            },
        )

        session = BreakGlassSession(
            session_id=session_id,
            user_id=user_id,
            reason=reason,
            justification_category=justification_category,
            approved_by_authority=approving_authority,
            issued_at=now,
            expires_at=expires,
            active=True,
            revoked=False,
            audit_event_id=audit_entry.event_id,
        )
        self._sessions[session_id] = session
        return session

    def revoke_break_glass(self, session_id: str, actor_id: str, revocation_reason: str) -> BreakGlassSession:
        session = self._sessions.get(session_id)
        if not session:
            raise KeyError(f"Break-glass session {session_id} not found.")

        session.active = False
        session.revoked = True

        global_audit_ledger.append_event(
            action="BREAK_GLASS_REVOKED",
            actor_id=actor_id,
            resource_type="BREAK_GLASS_SESSION",
            resource_id=session_id,
            payload={"reason": revocation_reason},
        )
        return session

    def is_session_valid(self, session_id: str) -> bool:
        session = self._sessions.get(session_id)
        if not session or not session.active or session.revoked:
            return False
        if datetime.now(timezone.utc) > session.expires_at:
            session.active = False
            return False
        return True


class DisasterRecoveryHarness:
    """
    Coordinated restore verifier across database state, object store hashes, audit checkpoints,
    and manifest inventories (NFR-032, AT-27). Rejects restart if any item is missing or corrupt.
    """

    def verify_restore_consistency(
        self,
        db_snapshot_hash: str,
        actual_db_hash: str,
        object_inventory: Dict[str, str],  # object_uri -> expected_sha256
        actual_objects: Dict[str, str],    # object_uri -> actual_sha256
        audit_checkpoint_valid: bool,
    ) -> RestoreVerificationResult:
        checks: List[RestoreCheckItem] = []
        failures: List[str] = []

        # DB snapshot check
        if db_snapshot_hash == actual_db_hash:
            checks.append(RestoreCheckItem(
                component="POSTGRES_RELATIONAL_STATE",
                expected_hash=db_snapshot_hash,
                actual_hash=actual_db_hash,
                status="MATCH",
            ))
        else:
            checks.append(RestoreCheckItem(
                component="POSTGRES_RELATIONAL_STATE",
                expected_hash=db_snapshot_hash,
                actual_hash=actual_db_hash,
                status="MISMATCH",
            ))
            failures.append("Relational database snapshot hash mismatch")

        # Audit ledger checkpoint check
        if audit_checkpoint_valid:
            checks.append(RestoreCheckItem(
                component="AUDIT_LEDGER_CHECKPOINT",
                expected_hash="VALID_CHAIN",
                actual_hash="VALID_CHAIN",
                status="MATCH",
            ))
        else:
            checks.append(RestoreCheckItem(
                component="AUDIT_LEDGER_CHECKPOINT",
                expected_hash="VALID_CHAIN",
                actual_hash="CORRUPTED",
                status="MISMATCH",
            ))
            failures.append("Audit checkpoint integrity failure")

        # Object store verification (NFR-032 / AT-27: missing referenced evidence fails restart)
        for uri, exp_hash in object_inventory.items():
            act_hash = actual_objects.get(uri)
            if act_hash is None:
                checks.append(RestoreCheckItem(
                    component=f"OBJECT_STORE_{uri}",
                    expected_hash=exp_hash,
                    actual_hash="MISSING",
                    status="MISSING",
                ))
                failures.append(f"Referenced evidence object {uri} missing from restored store")
            elif act_hash != exp_hash:
                checks.append(RestoreCheckItem(
                    component=f"OBJECT_STORE_{uri}",
                    expected_hash=exp_hash,
                    actual_hash=act_hash,
                    status="MISMATCH",
                ))
                failures.append(f"Checksum mismatch for object {uri}")
            else:
                checks.append(RestoreCheckItem(
                    component=f"OBJECT_STORE_{uri}",
                    expected_hash=exp_hash,
                    actual_hash=act_hash,
                    status="MATCH",
                ))

        passed = len(failures) == 0
        result = RestoreVerificationResult(
            status="PASSED" if passed else "FAILED",
            can_resume_authoritative_writes=passed,
            checks=checks,
            failure_reasons=failures,
        )

        global_audit_ledger.append_event(
            action="DISASTER_RECOVERY_RESTORE_VERIFIED",
            actor_id="dr_operator",
            resource_type="RESTORATION_SET",
            resource_id=db_snapshot_hash[:16],
            payload={"status": result.status, "can_resume": passed, "failures_count": len(failures)},
        )
        return result


class DegradedModeController:
    """
    Controls read-only degraded mode during broker/worker outages (NFR-006, architecture §12.3).
    Serves approved historical artifacts while blocking state modifications.
    """

    def __init__(self):
        self._degraded_active: bool = False
        self._degraded_reason: Optional[str] = None

    @property
    def is_degraded(self) -> bool:
        return self._degraded_active

    def engage_degraded_mode(self, reason: str, actor_id: str):
        self._degraded_active = True
        self._degraded_reason = reason
        global_audit_ledger.append_event(
            action="DEGRADED_MODE_ENGAGED",
            actor_id=actor_id,
            resource_type="SYSTEM_LIFECYCLE",
            resource_id="DEGRADED_MODE",
            payload={"reason": reason, "writes_blocked": True, "read_only_allowed": True},
        )

    def disengage_degraded_mode(self, actor_id: str):
        self._degraded_active = False
        self._degraded_reason = None
        global_audit_ledger.append_event(
            action="DEGRADED_MODE_DISENGAGED",
            actor_id=actor_id,
            resource_type="SYSTEM_LIFECYCLE",
            resource_id="DEGRADED_MODE",
            payload={"writes_restored": True},
        )

    def assert_writes_allowed(self):
        if self._degraded_active:
            raise DegradedModeError(
                f"System is in DEGRADED_MODE ({self._degraded_reason}). Authoritative writes and new decisions are suspended."
            )


class ManualContinuityReconciler:
    """
    Reconciles paper-based/offline administrative decisions made during communication outages
    back into the system with evidence, dates, and audit trail (phases.md §6).
    """

    def __init__(self):
        self._manual_records: Dict[str, ManualDecisionRecord] = {}

    def reconcile_manual_decision(
        self,
        manual_decision_id: str,
        case_id: str,
        jurisdiction_id: str,
        approving_authority: str,
        statutory_basis: str,
        decision_summary: str,
        paper_notice_reference: str,
        signed_offline_time: datetime,
        actor_id: str,
    ) -> ManualDecisionRecord:
        audit_entry = global_audit_ledger.append_event(
            action="MANUAL_CONTINUITY_DECISION_RECONCILED",
            actor_id=actor_id,
            resource_type="MANUAL_DECISION",
            resource_id=manual_decision_id,
            payload={
                "case_id": case_id,
                "approving_authority": approving_authority,
                "statutory_basis": statutory_basis,
                "paper_notice_ref": paper_notice_reference,
                "valid_time": signed_offline_time.isoformat(),
            },
        )

        rec = ManualDecisionRecord(
            manual_decision_id=manual_decision_id,
            case_id=case_id,
            jurisdiction_id=jurisdiction_id,
            approving_authority=approving_authority,
            statutory_basis=statutory_basis,
            decision_summary=decision_summary,
            paper_notice_reference=paper_notice_reference,
            signed_offline_time=signed_offline_time,
            reconciled_by_user_id=actor_id,
            audit_event_id=audit_entry.event_id,
        )
        self._manual_records[manual_decision_id] = rec
        return rec

    def get_manual_record(self, manual_decision_id: str) -> Optional[ManualDecisionRecord]:
        return self._manual_records.get(manual_decision_id)


class RollbackController:
    """
    Controls bounded rollback of live controlled deployment without deleting evidence
    or invalidating lawful administrative decisions (phases.md §6).
    """

    def __init__(self):
        self._rollbacks: Dict[str, RollbackRecord] = {}

    def initiate_rollback(
        self,
        programme_id: str,
        authority_order_ref: str,
        reason: str,
        actor_id: str,
        active_projections_count: int = 0,
    ) -> RollbackRecord:
        rollback_id = f"RB-{programme_id}-{int(time.time())}"
        audit_entry = global_audit_ledger.append_event(
            action="PROGRAMME_CONTROLLED_ROLLBACK",
            actor_id=actor_id,
            resource_type="PROGRAMME_ROLLBACK",
            resource_id=rollback_id,
            payload={
                "programme_id": programme_id,
                "authority_order": authority_order_ref,
                "reason": reason,
                "active_projections_revoked": active_projections_count,
                "status": "SUSPENDED_ROLLBACK",
            },
        )

        rec = RollbackRecord(
            rollback_id=rollback_id,
            programme_id=programme_id,
            initiated_by_user_id=actor_id,
            authority_order_reference=authority_order_ref,
            reason=reason,
            status="SUSPENDED_ROLLBACK",
            reverted_to_manual=True,
            active_projections_revoked=active_projections_count,
            read_only_retained=True,
            audit_event_id=audit_entry.event_id,
        )
        self._rollbacks[rollback_id] = rec
        return rec

    def get_rollback(self, rollback_id: str) -> Optional[RollbackRecord]:
        return self._rollbacks.get(rollback_id)


# Singletons
step_up_auth_manager = StepUpAuthManager()
break_glass_manager = BreakGlassManager()
disaster_recovery_harness = DisasterRecoveryHarness()
degraded_mode_controller = DegradedModeController()
manual_continuity_reconciler = ManualContinuityReconciler()
rollback_controller = RollbackController()
