"""
Phase 3 Decision Reconstruction, Disclosure Review, and Post-Approval Completion Service (C3-02, C3-03).
Normative Reference: phases.md §6 & §12.6, rules.md (RUL-005, RUL-052, RUL-072, RUL-075), trd.md (AT-22, AT-23).
"""

import hashlib
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

from sthira.core.audit import global_audit_ledger
from sthira.core.enums import AuthorityState


class CompletionMilestoneType(str, Enum):
    FUNDING_SANCTIONED = "FUNDING_SANCTIONED"
    UNIT_CONSTRUCTED = "UNIT_CONSTRUCTED"
    SERVICES_FUNCTIONAL = "SERVICES_FUNCTIONAL"  # Water, electricity, sanitation, road access
    DEFECTS_CLEARED = "DEFECTS_CLEARED"
    BENEFICIARY_ACCEPTED = "BENEFICIARY_ACCEPTED"
    POSSESSION_HANDED_OVER = "POSSESSION_HANDED_OVER"
    OCCUPIED = "OCCUPIED"
    FOLLOW_UP_COMPLETED = "FOLLOW_UP_COMPLETED"
    EXTERNAL_SYSTEM_HANDOFF = "EXTERNAL_SYSTEM_HANDOFF"


class MilestoneRecord(BaseModel):
    milestone_id: str
    case_id: str
    milestone_type: CompletionMilestoneType
    achieved_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    verified_by_officer: str
    evidence_document_ref: str
    notes: Optional[str] = None
    unresolved_defects_count: int = 0
    external_system_id: Optional[str] = None
    audit_event_id: str


class CaseCompletionSummary(BaseModel):
    case_id: str
    is_fully_completed: bool
    current_highest_milestone: Optional[CompletionMilestoneType]
    milestones: List[MilestoneRecord]
    blocking_reasons: List[str] = Field(default_factory=list)


class ProvenanceDAGNode(BaseModel):
    node_id: str
    node_type: str  # SOURCE_DATASET, POLICY_VERSION, GATE_EVALUATION, HOUSEHOLD_PREFERENCE, APPROVAL_ORDER
    version: str
    sha256_checksum: str
    recorded_at: str


class DecisionReconstructionReport(BaseModel):
    decision_id: str
    case_id: str
    authority_state: str
    approving_authority: str
    statutory_basis: str
    system_decision_time: str
    effective_valid_time: str
    provenance_dag: List[ProvenanceDAGNode]
    audit_chain_valid: bool
    audit_chain_length: int
    reconstruction_checksum: str


class DisclosureReviewResult(BaseModel):
    approved_for_public_release: bool
    k_anonymity_satisfied: bool
    suppressed_cell_count: int
    differencing_risk_detected: bool
    spatial_coordinates_generalized: bool
    generalized_records: List[Dict[str, Any]]
    threat_model_notes: List[str] = Field(default_factory=list)


class DeliveryCompletionTracker:
    """
    Tracks post-approval physical delivery, unit/service readiness, defect clearance,
    handover, and actual occupation (FEAT-025 / ARC-C12 / RUL-072).
    Approval or ceremonies NEVER count as completed relocation!
    """

    # Mandatory milestone progression required for full completion
    REQUIRED_MILESTONES_ORDER = [
        CompletionMilestoneType.FUNDING_SANCTIONED,
        CompletionMilestoneType.UNIT_CONSTRUCTED,
        CompletionMilestoneType.SERVICES_FUNCTIONAL,
        CompletionMilestoneType.DEFECTS_CLEARED,
        CompletionMilestoneType.BENEFICIARY_ACCEPTED,
        CompletionMilestoneType.POSSESSION_HANDED_OVER,
        CompletionMilestoneType.OCCUPIED,
        CompletionMilestoneType.FOLLOW_UP_COMPLETED,
    ]

    def __init__(self):
        self._case_milestones: Dict[str, List[MilestoneRecord]] = {}

    def record_milestone(
        self,
        case_id: str,
        milestone_type: CompletionMilestoneType,
        verified_by: str,
        evidence_doc_ref: str,
        notes: Optional[str] = None,
        unresolved_defects: int = 0,
        external_system_id: Optional[str] = None,
        actor_id: str = "delivery_officer",
    ) -> MilestoneRecord:
        # AT-22: Approved household offered unfinished or unserviced unit cannot be handed over
        if milestone_type in (CompletionMilestoneType.POSSESSION_HANDED_OVER, CompletionMilestoneType.OCCUPIED):
            if unresolved_defects > 0:
                raise ValueError(
                    f"Cannot record {milestone_type.value}: {unresolved_defects} unresolved defects remain on site/unit (RUL-072)."
                )

        audit_entry = global_audit_ledger.append_event(
            action="DELIVERY_MILESTONE_RECORDED",
            actor_id=actor_id,
            resource_type="DELIVERY_CASE",
            resource_id=case_id,
            payload={
                "milestone": milestone_type.value,
                "verified_by": verified_by,
                "evidence_ref": evidence_doc_ref,
                "defects": unresolved_defects,
                "external_system": external_system_id,
            },
        )

        record = MilestoneRecord(
            milestone_id=f"MLS-{case_id}-{milestone_type.value}",
            case_id=case_id,
            milestone_type=milestone_type,
            verified_by_officer=verified_by,
            evidence_document_ref=evidence_doc_ref,
            notes=notes,
            unresolved_defects_count=unresolved_defects,
            external_system_id=external_system_id,
            audit_event_id=audit_entry.event_id,
        )

        if case_id not in self._case_milestones:
            self._case_milestones[case_id] = []
        self._case_milestones[case_id].append(record)
        return record

    def get_completion_summary(self, case_id: str) -> CaseCompletionSummary:
        milestones = self._case_milestones.get(case_id, [])
        types_present = {m.milestone_type for m in milestones}

        blocking_reasons: List[str] = []

        # Check if external handoff applies
        if CompletionMilestoneType.EXTERNAL_SYSTEM_HANDOFF in types_present:
            handoff_m = next(m for m in milestones if m.milestone_type == CompletionMilestoneType.EXTERNAL_SYSTEM_HANDOFF)
            return CaseCompletionSummary(
                case_id=case_id,
                is_fully_completed=True,
                current_highest_milestone=CompletionMilestoneType.EXTERNAL_SYSTEM_HANDOFF,
                milestones=milestones,
                blocking_reasons=[],
            )

        # Check in-situ / township completion progression
        for req in self.REQUIRED_MILESTONES_ORDER:
            if req not in types_present:
                blocking_reasons.append(f"Pending milestone: {req.value}")

        # Check defects on last recorded milestones
        for m in milestones:
            if m.unresolved_defects_count > 0:
                blocking_reasons.append(f"Unresolved defects ({m.unresolved_defects_count}) flagged on milestone {m.milestone_type.value}")

        is_completed = len(blocking_reasons) == 0
        highest = milestones[-1].milestone_type if milestones else None

        return CaseCompletionSummary(
            case_id=case_id,
            is_fully_completed=is_completed,
            current_highest_milestone=highest,
            milestones=milestones,
            blocking_reasons=blocking_reasons,
        )


class DecisionReconstructionEngine:
    """
    Reconstructs complete bitemporal decision provenance DAGs for independent audit (R3-01 / FEAT-020).
    """

    def reconstruct_decision(
        self,
        decision_id: str,
        case_id: str,
        authority_state: str,
        approving_authority: str,
        statutory_basis: str,
        effective_valid_time: str,
        inputs: List[Dict[str, Any]],
    ) -> DecisionReconstructionReport:
        dag_nodes: List[ProvenanceDAGNode] = []

        for inp in inputs:
            raw_str = f"{inp.get('type')}:{inp.get('version')}:{inp.get('content')}"
            sha = hashlib.sha256(raw_str.encode()).hexdigest()
            dag_nodes.append(
                ProvenanceDAGNode(
                    node_id=inp.get("id", f"NODE-{len(dag_nodes)+1}"),
                    node_type=inp.get("type", "UNKNOWN"),
                    version=inp.get("version", "1.0.0"),
                    sha256_checksum=sha,
                    recorded_at=inp.get("recorded_at", datetime.now(timezone.utc).isoformat()),
                )
            )

        # Calculate overall reconstruction hash
        composite_content = f"{decision_id}:{case_id}:{authority_state}:{approving_authority}:{[n.sha256_checksum for n in dag_nodes]}"
        recon_hash = hashlib.sha256(composite_content.encode()).hexdigest()

        chain_valid = global_audit_ledger.verify_integrity()

        report = DecisionReconstructionReport(
            decision_id=decision_id,
            case_id=case_id,
            authority_state=authority_state,
            approving_authority=approving_authority,
            statutory_basis=statutory_basis,
            system_decision_time=datetime.now(timezone.utc).isoformat(),
            effective_valid_time=effective_valid_time,
            provenance_dag=dag_nodes,
            audit_chain_valid=chain_valid,
            audit_chain_length=len(global_audit_ledger.entries),
            reconstruction_checksum=recon_hash,
        )

        global_audit_ledger.append_event(
            action="DECISION_RECONSTRUCTION_AUDITED",
            actor_id="auditor",
            resource_type="DECISION_AUDIT",
            resource_id=decision_id,
            payload={"reconstruction_hash": recon_hash, "chain_valid": chain_valid},
        )
        return report


class DisclosureReviewEngine:
    """
    Evaluates privacy threat models before public disclosure (FEAT-019 / RUL-052 / RUL-075 / AT-23):
    - k-anonymity (suppression of small cells < 5).
    - Differencing attack prevention across publication versions.
    - Coordinate generalization (jittering / bounding precision).
    """

    def review_and_generalize_public_projection(
        self,
        records: List[Dict[str, Any]],
        prior_published_records: Optional[List[Dict[str, Any]]] = None,
        min_k_threshold: int = 5,
    ) -> DisclosureReviewResult:
        suppressed_count = 0
        generalized_records: List[Dict[str, Any]] = []
        threat_notes: List[str] = []

        # 1. k-anonymity check on counts / small demographic cells (AT-23)
        for r in records:
            rec = dict(r)
            count_val = rec.get("count", 10)
            if isinstance(count_val, (int, float)) and 0 < count_val < min_k_threshold:
                # Suppress small cell to prevent household re-identification
                rec["count"] = f"<{min_k_threshold}"
                rec["suppressed"] = True
                suppressed_count += 1
                threat_notes.append(f"Cell '{rec.get('category', 'unknown')}' suppressed (count < {min_k_threshold})")

            # 2. Coordinate generalization (RUL-007, RUL-053: fine preliminary site coordinates generalized)
            if "coordinates" in rec and isinstance(rec["coordinates"], list):
                coords = rec["coordinates"]
                if len(coords) == 2:
                    # Generalize precision to 2 decimal places (~1.1 km bounding area)
                    rec["coordinates"] = [round(coords[0], 2), round(coords[1], 2)]
                    rec["coordinates_precision"] = "GENERALIZED_APPROX_1KM"

            # Strip any individual identity names/Aadhaar/disability fields
            for sensitive_key in ["head_name", "aadhaar_ref", "disability_details", "phone"]:
                if sensitive_key in rec:
                    del rec[sensitive_key]

            generalized_records.append(rec)

        # 3. Differencing attack detection against prior versions
        differencing_risk = False
        if prior_published_records:
            # Check if difference between versions results in a residual of 1 (singling out a household)
            for old, new in zip(prior_published_records, records):
                old_c = old.get("count")
                new_c = new.get("count")
                if isinstance(old_c, int) and isinstance(new_c, int):
                    diff = abs(new_c - old_c)
                    if diff == 1:
                        differencing_risk = True
                        threat_notes.append("Potential differencing leakage detected: single-unit delta between publication rounds.")
                        break

        approved = not differencing_risk

        global_audit_ledger.append_event(
            action="DISCLOSURE_REVIEW_COMPLETED",
            actor_id="privacy_officer",
            resource_type="PUBLIC_PROJECTION_REVIEW",
            resource_id=f"DISC-{int(datetime.now().timestamp())}",
            payload={
                "approved": approved,
                "suppressed_count": suppressed_count,
                "differencing_risk": differencing_risk,
            },
        )

        return DisclosureReviewResult(
            approved_for_public_release=approved,
            k_anonymity_satisfied=True,
            suppressed_cell_count=suppressed_count,
            differencing_risk_detected=differencing_risk,
            spatial_coordinates_generalized=True,
            generalized_records=generalized_records,
            threat_model_notes=threat_notes,
        )


# Singletons
delivery_completion_tracker = DeliveryCompletionTracker()
decision_reconstruction_engine = DecisionReconstructionEngine()
disclosure_review_engine = DisclosureReviewEngine()
