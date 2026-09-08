"""
PUNARVAS-AI Official Approvals, Statutory Notifications & Supersession Engine (Phase 9 / ARC-C08, ARC-C09 / FEAT-017, FEAT-024).
Normative Reference: plan.md (#9), trd.md (§3.8, §3.11, FR-047-FR-052, FR-070), rules.md (RUL-003, RUL-004, RUL-005, RUL-040, RUL-049, RUL-054), DEC-025, DEC-028, AT-06, AT-10, AT-20, AT-21.
"""

from datetime import datetime, timezone
from enum import Enum
from typing import Any, Dict, List, Optional
from uuid import uuid4
from pydantic import BaseModel, Field

from punarvas.core.audit import global_audit_ledger
from punarvas.core.contracts import utc_now, UserContext
from punarvas.core.enums import AuthorityState, RoleType
from punarvas.core.errors import (
    ApprovalConditionUnmetError,
    EntityFrozenByObjectionError,
    UnauthorizedActionError,
)
from punarvas.modules.governance.objections_service import (
    objections_service,
)


class ApprovalConditionType(str, Enum):
    WATER_YIELD_VERIFICATION = "WATER_YIELD_VERIFICATION"
    FRA_CONSULTATION_CLEARANCE = "FRA_CONSULTATION_CLEARANCE"
    GEOTECHNICAL_STABILITY_CLEARANCE = "GEOTECHNICAL_STABILITY_CLEARANCE"
    REVENUE_TITLE_MUTATION = "REVENUE_TITLE_MUTATION"
    GRAMA_SABHA_RESOLUTION = "GRAMA_SABHA_RESOLUTION"
    BUDGET_SANCTION_CONFIRMATION = "BUDGET_SANCTION_CONFIRMATION"


class ApprovalCondition(BaseModel):
    condition_id: str
    condition_type: ApprovalConditionType
    description: str
    is_blocking_for_allocation: bool = True  # AT-20: blocking vs non-blocking gate
    is_satisfied: bool = False
    satisfied_at: Optional[datetime] = None
    satisfied_by_officer: Optional[str] = None
    verification_document_hash: Optional[str] = None


class OfficialApprovalRecord(BaseModel):
    """
    Formal administrative approval by competent statutory authority (RUL-003, RUL-004).
    Approval does NOT equal completed relocation (RUL-072).
    Approval is distinct from statutory notification (RUL-003, AT-21).
    """
    approval_id: str
    entity_type: str  # SITE_SELECTION, ALLOCATION_SCENARIO, BENEFICIARY_LIST, RELOCATION_PROGRAMME
    entity_id: str
    entity_version: str
    authority_state: AuthorityState = AuthorityState.OFFICIALLY_APPROVED
    approving_authority_role: RoleType
    approving_officer_name: str
    approving_officer_designation: str
    statutory_authority_basis: str  # e.g. "Disaster Management Act 2005 §30(2)(v)"
    approval_order_number: str
    approval_date: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    step_up_token_verified: bool = True  # RUL-054
    conditions: List[ApprovalCondition] = Field(default_factory=list)
    supersedes_approval_id: Optional[str] = None
    is_superseded: bool = False
    superseded_by_approval_id: Optional[str] = None
    is_withdrawn: bool = False
    withdrawn_reason: Optional[str] = None
    statutory_notification_id: Optional[str] = None
    audit_event_id: str


class StatutoryNotificationRecord(BaseModel):
    """
    Official Gazette or District Extraordinary Order Publication (AT-21, RUL-003).
    Bilingual (English & Malayalam) statutory notification artifact.
    """
    notification_id: str
    approval_id: str
    entity_type: str
    entity_id: str
    gazette_notification_number: str
    gazette_volume_number: str
    gazette_issue_date: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    effective_date: datetime
    notification_title_en: str
    notification_title_ml: str
    notification_text_en: str
    notification_text_ml: str
    issuing_authority: str
    signing_officer_name: str
    digital_signature_hash: str
    superseded_notification_id: Optional[str] = None
    is_active: bool = True
    audit_event_id: str


class ApprovalService:
    """
    Service managing formal approvals, statutory notifications, conditional clearance,
    and supersession workflows (ARC-C08, ARC-C09 / FEAT-017, FEAT-024).
    """

    def __init__(self):
        self._approvals: Dict[str, OfficialApprovalRecord] = {}
        self._notifications: Dict[str, StatutoryNotificationRecord] = {}

    def issue_official_approval(
        self,
        entity_type: str,
        entity_id: str,
        entity_version: str,
        approving_officer_name: str,
        approving_officer_designation: str,
        statutory_authority_basis: str,
        approval_order_number: str,
        conditions: Optional[List[ApprovalCondition]] = None,
        supersedes_approval_id: Optional[str] = None,
        step_up_token: Optional[str] = None,
        context: Optional[UserContext] = None,
    ) -> OfficialApprovalRecord:
        """
        Record official approval by DDMA / State Authority (RUL-003, RUL-004).
        Requires step-up MFA token verification (RUL-054).
        Fails if entity is frozen by pending objections (RUL-049, FR-050).
        """
        # 1. Check pending objections freeze (RUL-049 / FR-050)
        is_frozen, frozen_reason = objections_service.check_is_entity_frozen(entity_id)
        if is_frozen:
            raise EntityFrozenByObjectionError(
                entity_id=entity_id,
                reason=frozen_reason or "Entity is frozen pending objection resolution (RUL-049)."
            )

        # 2. Verify Step-Up MFA Token (RUL-054)
        if not step_up_token or not step_up_token.startswith("MFA-STEPUP-"):
            raise UnauthorizedActionError(
                action="OFFICIAL_APPROVAL",
                reason="Official approval requires valid step-up multi-factor authentication token (RUL-054)."
            )

        # 3. Role authorization check (RUL-002, RUL-054)
        actor_role = context.roles[0] if (context and context.roles) else RoleType.GOVERNMENT_APPROVER
        actor_id = context.user_id if context else approving_officer_name
        actor_scope = f"{context.geography_scope.state}/{context.geography_scope.district or 'Wayanad'}" if context else "Kerala/Wayanad"

        if actor_role not in (RoleType.GOVERNMENT_APPROVER, RoleType.STATE_PROGRAMME_ADMIN):
            raise UnauthorizedActionError(
                action="OFFICIAL_APPROVAL",
                reason=f"Role '{actor_role}' lacks statutory authority to issue official approval (RUL-002)."
            )

        # 4. If superseding, verify and update previous approval (RUL-005, AT-06)
        if supersedes_approval_id:
            prev_approval = self._approvals.get(supersedes_approval_id)
            if not prev_approval:
                raise KeyError(f"Previous approval '{supersedes_approval_id}' to supersede not found.")
            prev_approval.is_superseded = True

        approval_id = f"APP-{entity_type[:4]}-{int(utc_now().timestamp())}-{uuid4().hex[:6]}"
        conditions_list = conditions or []

        audit_entry = global_audit_ledger.append_event(
            action="ISSUE_OFFICIAL_APPROVAL",
            actor_id=actor_id,
            resource_type=entity_type,
            resource_id=entity_id,
            payload={
                "approval_id": approval_id,
                "order_number": approval_order_number,
                "statutory_basis": statutory_authority_basis,
                "conditions_count": len(conditions_list),
                "supersedes_approval_id": supersedes_approval_id,
            },
        )

        record = OfficialApprovalRecord(
            approval_id=approval_id,
            entity_type=entity_type,
            entity_id=entity_id,
            entity_version=entity_version,
            authority_state=AuthorityState.OFFICIALLY_APPROVED,
            approving_authority_role=actor_role,
            approving_officer_name=approving_officer_name,
            approving_officer_designation=approving_officer_designation,
            statutory_authority_basis=statutory_authority_basis,
            approval_order_number=approval_order_number,
            step_up_token_verified=True,
            conditions=conditions_list,
            supersedes_approval_id=supersedes_approval_id,
            audit_event_id=audit_entry.event_id,
        )

        if supersedes_approval_id:
            self._approvals[supersedes_approval_id].superseded_by_approval_id = approval_id

        self._approvals[approval_id] = record
        return record

    def check_can_allocate(self, approval_id: str) -> tuple[bool, List[str]]:
        """
        AT-20: Evaluates whether an approved entity satisfies all blocking conditions
        before capacity reservation or household allocation can occur.
        """
        approval = self._approvals.get(approval_id)
        if not approval:
            raise KeyError(f"Approval '{approval_id}' not found.")

        if approval.is_superseded:
            return False, [f"Approval '{approval_id}' is superseded by '{approval.superseded_by_approval_id}'."]
        if approval.is_withdrawn:
            return False, [f"Approval '{approval_id}' is withdrawn: {approval.withdrawn_reason}."]

        blocking_failures = []
        for cond in approval.conditions:
            if cond.is_blocking_for_allocation and not cond.is_satisfied:
                blocking_failures.append(
                    f"Unmet blocking condition: {cond.condition_type.value} - {cond.description}"
                )

        # Check pending objections as well (RUL-049)
        is_frozen, frozen_reason = objections_service.check_is_entity_frozen(approval.entity_id)
        if is_frozen:
            blocking_failures.append(f"Entity is frozen by active objection: {frozen_reason}")

        can_proceed = len(blocking_failures) == 0
        return can_proceed, blocking_failures

    def satisfy_condition(
        self,
        approval_id: str,
        condition_id: str,
        verification_doc_hash: str,
        officer_name: str,
        context: Optional[UserContext] = None,
    ) -> ApprovalCondition:
        """
        Verify and clear a conditional approval gate (AT-20).
        """
        approval = self._approvals.get(approval_id)
        if not approval:
            raise KeyError(f"Approval '{approval_id}' not found.")

        target_cond: Optional[ApprovalCondition] = None
        for cond in approval.conditions:
            if cond.condition_id == condition_id:
                target_cond = cond
                break

        if not target_cond:
            raise KeyError(f"Condition '{condition_id}' not found on approval '{approval_id}'.")

        target_cond.is_satisfied = True
        target_cond.satisfied_at = datetime.now(timezone.utc)
        target_cond.satisfied_by_officer = officer_name
        target_cond.verification_document_hash = verification_doc_hash

        actor_id = context.user_id if context else officer_name
        global_audit_ledger.append_event(
            action="APPROVAL_CONDITION_SATISFIED",
            actor_id=actor_id,
            resource_type="APPROVAL_CONDITION",
            resource_id=condition_id,
            payload={
                "approval_id": approval_id,
                "condition_type": target_cond.condition_type.value,
                "verification_hash": verification_doc_hash,
            },
        )
        return target_cond

    def publish_statutory_notification(
        self,
        approval_id: str,
        gazette_notification_number: str,
        gazette_volume_number: str,
        effective_date: datetime,
        notification_title_en: str,
        notification_title_ml: str,
        notification_text_en: str,
        notification_text_ml: str,
        issuing_authority: str,
        signing_officer_name: str,
        digital_signature_hash: str,
        context: Optional[UserContext] = None,
    ) -> StatutoryNotificationRecord:
        """
        Separate official statutory notification from approval (RUL-003, AT-21).
        Publishes bilingual gazette / district order artifact.
        Fails if entity is frozen by objections (RUL-049, FR-050).
        """
        approval = self._approvals.get(approval_id)
        if not approval:
            raise KeyError(f"Approval '{approval_id}' not found.")

        # Check pending objections freeze (RUL-049 / FR-050)
        is_frozen, frozen_reason = objections_service.check_is_entity_frozen(approval.entity_id)
        if is_frozen:
            raise EntityFrozenByObjectionError(
                entity_id=approval.entity_id,
                reason=frozen_reason or "Statutory notification blocked: entity has pending objections."
            )

        actor_id = context.user_id if context else signing_officer_name

        notification_id = f"NOTIF-GAZ-{int(utc_now().timestamp())}-{uuid4().hex[:6]}"

        audit_entry = global_audit_ledger.append_event(
            action="PUBLISH_STATUTORY_NOTIFICATION",
            actor_id=actor_id,
            resource_type="STATUTORY_NOTIFICATION",
            resource_id=notification_id,
            payload={
                "approval_id": approval_id,
                "gazette_number": gazette_notification_number,
                "volume": gazette_volume_number,
                "effective_date": effective_date.isoformat(),
                "signature_hash": digital_signature_hash,
            },
        )

        record = StatutoryNotificationRecord(
            notification_id=notification_id,
            approval_id=approval_id,
            entity_type=approval.entity_type,
            entity_id=approval.entity_id,
            gazette_notification_number=gazette_notification_number,
            gazette_volume_number=gazette_volume_number,
            effective_date=effective_date,
            notification_title_en=notification_title_en,
            notification_title_ml=notification_title_ml,
            notification_text_en=notification_text_en,
            notification_text_ml=notification_text_ml,
            issuing_authority=issuing_authority,
            signing_officer_name=signing_officer_name,
            digital_signature_hash=digital_signature_hash,
            audit_event_id=audit_entry.event_id,
        )

        approval.authority_state = AuthorityState.OFFICIALLY_NOTIFIED
        approval.statutory_notification_id = notification_id

        self._notifications[notification_id] = record
        return record

    def withdraw_approval(
        self,
        approval_id: str,
        reason: str,
        revocation_order_ref: str,
        context: Optional[UserContext] = None,
    ) -> OfficialApprovalRecord:
        """
        Formally withdraw an approval or statutory notification (RUL-005).
        Preserves original record and links withdrawal event.
        """
        approval = self._approvals.get(approval_id)
        if not approval:
            raise KeyError(f"Approval '{approval_id}' not found.")

        approval.is_withdrawn = True
        approval.withdrawn_reason = reason
        approval.authority_state = AuthorityState.WITHDRAWN

        if approval.statutory_notification_id:
            notif = self._notifications.get(approval.statutory_notification_id)
            if notif:
                notif.is_active = False

        actor_id = context.user_id if context else "admin_authority"
        global_audit_ledger.append_event(
            action="WITHDRAW_APPROVAL",
            actor_id=actor_id,
            resource_type="OFFICIAL_APPROVAL",
            resource_id=approval_id,
            payload={
                "reason": reason,
                "revocation_order": revocation_order_ref,
            },
        )
        return approval

    def get_approval(self, approval_id: str) -> Optional[OfficialApprovalRecord]:
        return self._approvals.get(approval_id)

    def get_notification(self, notification_id: str) -> Optional[StatutoryNotificationRecord]:
        return self._notifications.get(notification_id)

    def list_approvals(self, entity_id: Optional[str] = None) -> List[OfficialApprovalRecord]:
        if entity_id:
            return [a for a in self._approvals.values() if a.entity_id == entity_id]
        return list(self._approvals.values())


# Global singleton instance
approval_service = ApprovalService()
