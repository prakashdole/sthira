"""
Tests for Phase 3: Controlled Live Wayanad Deployment (PH-3 / C3-01, C3-02, C3-03).
Normative Reference: phases.md §6 & §12.6, rules.md (RUL-001, RUL-005, RUL-052, RUL-054, RUL-072, RUL-075), trd.md (NFR-006, NFR-013, NFR-032, AT-22, AT-23, AT-27, AT-28).
"""

import time
import pytest
from datetime import datetime, timezone
from fastapi.testclient import TestClient

from punarvas.api.app import app
from punarvas.core.contracts import UserContext
from punarvas.core.enums import AuthorityState, RoleType
from punarvas.modules.live_ops import (
    StepUpAuthManager,
    BreakGlassManager,
    DisasterRecoveryHarness,
    DegradedModeController,
    ManualContinuityReconciler,
    RollbackController,
    step_up_auth_manager,
    break_glass_manager,
    disaster_recovery_harness,
    degraded_mode_controller,
    manual_continuity_reconciler,
    rollback_controller,
)
from punarvas.modules.reconstruction import (
    CompletionMilestoneType,
    DeliveryCompletionTracker,
    DecisionReconstructionEngine,
    DisclosureReviewEngine,
    delivery_completion_tracker,
    decision_reconstruction_engine,
    disclosure_review_engine,
)

client = TestClient(app)


# --- C3-01: Step-Up Auth & MFA Tests (AT-28) ---

def test_step_up_token_issuance_and_verification():
    user = UserContext(
        user_id="officer_wyd_01",
        username="collector_wayanad",
        roles=[RoleType.GOVERNMENT_APPROVER],
    )
    # Privileged action generates token
    tok = step_up_auth_manager.issue_step_up_token(user, "APPROVE_DECISION", valid_seconds=10)
    assert tok.token_id is not None
    assert tok.user_id == "officer_wyd_01"
    assert tok.target_action == "APPROVE_DECISION"

    # Token verifies correctly
    assert step_up_auth_manager.verify_step_up(tok.token_id, "officer_wyd_01", "APPROVE_DECISION") is True

    # Token fails with wrong user or wrong action
    assert step_up_auth_manager.verify_step_up(tok.token_id, "other_user", "APPROVE_DECISION") is False
    assert step_up_auth_manager.verify_step_up(tok.token_id, "officer_wyd_01", "BREAK_GLASS") is False

    # Unregistered action is rejected
    with pytest.raises(ValueError):
        step_up_auth_manager.issue_step_up_token(user, "UNPRIVILEGED_ACTION")


# --- C3-01: Break-Glass Emergency Session Tests (NFR-013) ---

def test_break_glass_lifecycle():
    mgr = BreakGlassManager()

    # Short reason is rejected
    with pytest.raises(ValueError):
        mgr.request_break_glass("admin_01", "short", "COMMUNICATION_OUTAGE", "DDMA Chairman")

    # Valid break-glass session
    session = mgr.request_break_glass(
        user_id="admin_01",
        reason="Severed fiber link during Meppadi landslide emergency; urgent caseworker lookup required",
        justification_category="COMMUNICATION_OUTAGE",
        approving_authority="District Collector, Wayanad",
        duration_minutes=30,
    )
    assert session.session_id.startswith("BG-")
    assert session.active is True
    assert mgr.is_session_valid(session.session_id) is True

    # Manual revocation
    revoked = mgr.revoke_break_glass(session.session_id, "sec_auditor", "Incident resolved, network restored")
    assert revoked.active is False
    assert revoked.revoked is True
    assert mgr.is_session_valid(session.session_id) is False


# --- C3-01: Coordinated Disaster Recovery Harness (NFR-032 / AT-27) ---

def test_dr_restore_consistency_checks():
    harness = DisasterRecoveryHarness()

    expected_db_hash = "a" * 64
    obj_inv = {
        "s3://punarvas-vault/gsi_lsm_2022.tif": "b" * 64,
        "s3://punarvas-vault/signed_order_001.pdf": "c" * 64,
    }

    # 1. Matching hashes and complete inventory -> PASS
    res = harness.verify_restore_consistency(
        db_snapshot_hash=expected_db_hash,
        actual_db_hash=expected_db_hash,
        object_inventory=obj_inv,
        actual_objects=dict(obj_inv),
        audit_checkpoint_valid=True,
    )
    assert res.status == "PASSED"
    assert res.can_resume_authoritative_writes is True
    assert len(res.failure_reasons) == 0

    # 2. Missing object store file -> FAIL (authoritative restart blocked per AT-27)
    corrupted_objects = {"s3://punarvas-vault/gsi_lsm_2022.tif": "b" * 64}  # pdf missing
    res_fail = harness.verify_restore_consistency(
        db_snapshot_hash=expected_db_hash,
        actual_db_hash=expected_db_hash,
        object_inventory=obj_inv,
        actual_objects=corrupted_objects,
        audit_checkpoint_valid=True,
    )
    assert res_fail.status == "FAILED"
    assert res_fail.can_resume_authoritative_writes is False
    assert any("missing" in f.lower() for f in res_fail.failure_reasons)

    # 3. Database snapshot hash mismatch -> FAIL
    res_mismatch = harness.verify_restore_consistency(
        db_snapshot_hash=expected_db_hash,
        actual_db_hash="z" * 64,
        object_inventory=obj_inv,
        actual_objects=dict(obj_inv),
        audit_checkpoint_valid=True,
    )
    assert res_mismatch.status == "FAILED"
    assert res_mismatch.can_resume_authoritative_writes is False


# --- C3-01: Read-Only Degraded Mode (NFR-006) ---

def test_degraded_mode_circuit_breaker():
    ctrl = DegradedModeController()
    assert ctrl.is_degraded is False
    ctrl.assert_writes_allowed()  # Does not raise

    # Engage degraded mode during broker failure
    ctrl.engage_degraded_mode("Outbox message broker unreachable", "system_health_probe")
    assert ctrl.is_degraded is True
    with pytest.raises(RuntimeError) as exc:
        ctrl.assert_writes_allowed()
    assert "DEGRADED_MODE" in str(exc.value)

    # Disengage
    ctrl.disengage_degraded_mode("system_health_probe")
    assert ctrl.is_degraded is False
    ctrl.assert_writes_allowed()


# --- C3-01: Manual Continuity Reconciler & Rollback Controller ---

def test_manual_continuity_reconciliation():
    reconciler = ManualContinuityReconciler()
    offline_dt = datetime(2024, 8, 1, 10, 0, 0, tzinfo=timezone.utc)
    rec = reconciler.reconcile_manual_decision(
        manual_decision_id="MAN-2024-WYD-001",
        case_id="CASE-WYD-001",
        jurisdiction_id="Wayanad/Meppadi",
        approving_authority="District Collector, Wayanad",
        statutory_basis="Disaster Management Act 2005 §30(2)",
        decision_summary="Manual offline sanction of interim shelter assistance",
        paper_notice_reference="NOT-WYD-2024-884-OFFLINE",
        signed_offline_time=offline_dt,
        actor_id="relief_camp_officer_01",
    )
    assert rec.manual_decision_id == "MAN-2024-WYD-001"
    assert rec.authority_state == AuthorityState.OFFICIALLY_APPROVED
    assert reconciler.get_manual_record("MAN-2024-WYD-001") is not None


def test_controlled_live_rollback():
    rb = RollbackController()
    res = rb.initiate_rollback(
        programme_id="PRG-KL-WYD-2024",
        authority_order_ref="G.O.(Ms) No. 99/2024/DMD",
        reason="Judicial interim injunction halting township construction pending survey boundary verification",
        actor_id="state_programme_admin",
        active_projections_count=2,
    )
    assert res.status == "SUSPENDED_ROLLBACK"
    assert res.reverted_to_manual is True
    assert res.read_only_retained is True
    assert res.active_projections_revoked == 2


# --- C3-02 / C3-03: Post-Approval Delivery Milestones Tracker (RUL-072 / AT-22) ---

def test_delivery_completion_milestones_and_defect_blocking():
    tracker = DeliveryCompletionTracker()
    case_id = "CASE-DELIVERY-TEST-001"

    # Step 1: Funding Sanctioned
    tracker.record_milestone(
        case_id=case_id,
        milestone_type=CompletionMilestoneType.FUNDING_SANCTIONED,
        verified_by="Treasury Officer",
        evidence_doc_ref="SDRF-SANCTION-2024-01",
    )
    summary = tracker.get_completion_summary(case_id)
    assert summary.is_fully_completed is False
    assert any("Pending milestone" in b for b in summary.blocking_reasons)

    # Step 2: Unit Constructed
    tracker.record_milestone(
        case_id=case_id,
        milestone_type=CompletionMilestoneType.UNIT_CONSTRUCTED,
        verified_by="PWD Assistant Executive Engineer",
        evidence_doc_ref="PWD-COMPLETION-CERT-01",
    )

    # Step 3: Services Functional
    tracker.record_milestone(
        case_id=case_id,
        milestone_type=CompletionMilestoneType.SERVICES_FUNCTIONAL,
        verified_by="KWA / KSEB Engineers",
        evidence_doc_ref="KWA-WATER-CONN-01",
    )

    # Step 4: Cannot record POSSESSION_HANDED_OVER if unresolved defects remain (AT-22)
    with pytest.raises(ValueError) as exc:
        tracker.record_milestone(
            case_id=case_id,
            milestone_type=CompletionMilestoneType.POSSESSION_HANDED_OVER,
            verified_by="Site Officer",
            evidence_doc_ref="HANDOVER-DRAFT-01",
            unresolved_defects=3,  # 3 defects reported (e.g. water leakage)
        )
    assert "unresolved defects" in str(exc.value)

    # Step 5: Clear defects
    tracker.record_milestone(
        case_id=case_id,
        milestone_type=CompletionMilestoneType.DEFECTS_CLEARED,
        verified_by="Quality Inspector",
        evidence_doc_ref="QC-CLEARANCE-01",
        unresolved_defects=0,
    )
    tracker.record_milestone(
        case_id=case_id,
        milestone_type=CompletionMilestoneType.BENEFICIARY_ACCEPTED,
        verified_by="Social Welfare Officer",
        evidence_doc_ref="BENEFICIARY-ACCEPTANCE-01",
    )
    tracker.record_milestone(
        case_id=case_id,
        milestone_type=CompletionMilestoneType.POSSESSION_HANDED_OVER,
        verified_by="Tahsildar",
        evidence_doc_ref="KEYS-HANDOVER-01",
    )
    tracker.record_milestone(
        case_id=case_id,
        milestone_type=CompletionMilestoneType.OCCUPIED,
        verified_by="Village Officer",
        evidence_doc_ref="OCCUPATION-INSPECTION-01",
    )
    tracker.record_milestone(
        case_id=case_id,
        milestone_type=CompletionMilestoneType.FOLLOW_UP_COMPLETED,
        verified_by="KSDMA Resettlement Officer",
        evidence_doc_ref="SIX-MONTH-MONITORING-01",
    )

    # Now fully completed
    full_summary = tracker.get_completion_summary(case_id)
    assert full_summary.is_fully_completed is True
    assert len(full_summary.blocking_reasons) == 0


# --- C3-02: Decision Provenance DAG Reconstruction (R3-01 / FEAT-020) ---

def test_decision_reconstruction_dag():
    engine = DecisionReconstructionEngine()
    report = engine.reconstruct_decision(
        decision_id="DEC-2024-WYD-0042",
        case_id="CASE-WYD-042",
        authority_state="OFFICIALLY_APPROVED",
        approving_authority="District Collector, Wayanad",
        statutory_basis="Disaster Management Act 2005 §31",
        effective_valid_time="2024-09-01T00:00:00Z",
        inputs=[
            {"type": "SOURCE_DATASET", "version": "S01-2022.1", "content": "GSI Landslide Susceptibility"},
            {"type": "POLICY_VERSION", "version": "POL-WYD-2024.1", "content": "Wayanad Township Policy"},
            {"type": "GATE_EVALUATION", "version": "1.0", "content": "PASS_DEBRIS_FLOW_RUNOUT"},
            {"type": "HOUSEHOLD_PREFERENCE", "version": "1.0", "content": "TOWNSHIP_FIRST_CHOICE"},
        ],
    )
    assert report.decision_id == "DEC-2024-WYD-0042"
    assert len(report.provenance_dag) == 4
    assert report.audit_chain_valid is True
    assert len(report.reconstruction_checksum) == 64


# --- C3-03: Disclosure Review & k-Anonymity (FEAT-019 / RUL-075 / AT-23) ---

def test_disclosure_review_k_anonymity_and_differencing():
    engine = DisclosureReviewEngine()

    sample_records = [
        {"category": "General Township", "count": 140, "coordinates": [76.12345, 11.56789], "head_name": "John Doe"},
        {"category": "Special Vulnerability", "count": 3, "coordinates": [76.12891, 11.56123]},  # small cell < 5
    ]

    # Review suppresses count < 5, generalises coordinates, and strips PII
    res = engine.review_and_generalize_public_projection(sample_records, min_k_threshold=5)
    assert res.approved_for_public_release is True
    assert res.suppressed_cell_count == 1
    assert res.generalized_records[1]["count"] == "<5"
    assert "head_name" not in res.generalized_records[0]
    assert res.generalized_records[0]["coordinates"] == [76.12, 11.57]

    # Differencing attack: delta of 1 between versions flags risk
    prior_records = [{"category": "General Township", "count": 139}]
    new_records = [{"category": "General Township", "count": 140}]
    diff_res = engine.review_and_generalize_public_projection(new_records, prior_published_records=prior_records)
    assert diff_res.differencing_risk_detected is True
    assert diff_res.approved_for_public_release is False


# --- FastAPI Phase 3 Endpoints Integration Tests ---

def test_api_phase3_endpoints():
    # 1. Step-up auth endpoint
    resp = client.post("/api/v1/auth/step-up", json={"user_id": "collector_01", "action": "APPROVE_DECISION"})
    assert resp.status_code == 200
    tok = resp.json()["data"]["token_id"]
    assert tok is not None

    # 2. Break-glass endpoint
    bg_resp = client.post(
        "/api/v1/auth/break-glass",
        json={
            "user_id": "officer_emergency",
            "reason": "Bridge collapse in Mundakkai; communications disconnected; manual rescue triage",
            "justification_category": "COMMUNICATION_OUTAGE",
            "approving_authority": "DDMA Wayanad",
            "duration_minutes": 60,
        },
    )
    assert bg_resp.status_code == 200
    bg_session = bg_resp.json()["data"]
    assert bg_session["session_id"].startswith("BG-")

    # Revoke break-glass
    rev_resp = client.post(
        f"/api/v1/auth/break-glass/{bg_session['session_id']}/revoke",
        json={"actor_id": "sec_officer", "reason": "Communication restored"},
    )
    assert rev_resp.status_code == 200
    assert rev_resp.json()["data"]["active"] is False

    # 3. Degraded mode endpoints
    deg_resp = client.get("/api/v1/recovery/degraded-mode")
    assert deg_resp.status_code == 200
    assert "is_degraded" in deg_resp.json()["data"]

    deg_toggle = client.post(
        "/api/v1/recovery/degraded-mode",
        json={"engage": True, "reason": "Scheduled database failover exercise", "actor_id": "devops_lead"},
    )
    assert deg_toggle.status_code == 200
    assert deg_toggle.json()["data"]["is_degraded"] is True

    # Disengage
    deg_toggle2 = client.post(
        "/api/v1/recovery/degraded-mode",
        json={"engage": False, "actor_id": "devops_lead"},
    )
    assert deg_toggle2.status_code == 200
    assert deg_toggle2.json()["data"]["is_degraded"] is False

    # 4. DR Restore verification endpoint
    dr_resp = client.post(
        "/api/v1/recovery/verify-restore",
        json={
            "db_snapshot_hash": "db_hash_valid",
            "actual_db_hash": "db_hash_valid",
            "object_inventory": {"s3://bucket/test.tif": "hash123"},
            "actual_objects": {"s3://bucket/test.tif": "hash123"},
            "audit_checkpoint_valid": True,
        },
    )
    assert dr_resp.status_code == 200
    assert dr_resp.json()["data"]["status"] == "PASSED"

    # 5. Milestone tracking endpoints
    m_resp = client.post(
        "/api/v1/live/completion-milestones/CASE-API-001",
        json={
            "milestone_type": "FUNDING_SANCTIONED",
            "verified_by": "Finance Dept",
            "evidence_doc_ref": "FIN-2024-001",
            "notes": "First tranche disbursed",
            "unresolved_defects": 0,
        },
    )
    assert m_resp.status_code == 200

    sum_resp = client.get("/api/v1/live/completion-milestones/CASE-API-001")
    assert sum_resp.status_code == 200
    assert sum_resp.json()["data"]["case_id"] == "CASE-API-001"
