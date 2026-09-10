"""
Unit tests for Phase 14: Platform Reliability, Multi-Channel Security, Restore Validation & Release Assurance.
Normative Reference: NFR-028 to NFR-035, AT-24 to AT-30, ARC-C01, ARC-C11, DEC-045.
"""

from datetime import datetime, timezone
import pytest

from sthira.modules.resilience.contracts import (
    ChannelType,
    CoordinatedRestorePackage,
    CrossChannelAccessRequest,
    DataClassification,
    OfflineDeviceRecord,
)
from sthira.modules.resilience.service import ResilienceService


@pytest.fixture
def service():
    return ResilienceService()


def test_outbox_relay_success(service):
    """Test successful outbox delivery without transient failures."""
    messages = [
        {"id": "msg-1", "event_type": "PARCEL_VALIDATED", "payload": {"parcel_id": "P-01"}},
        {"id": "msg-2", "event_type": "HAZARD_SCREENED", "payload": {"hazard": "LOW"}},
    ]
    result = service.relay_and_reconcile_outbox(messages)
    assert result.total_messages == 2
    assert result.published_count == 2
    assert result.failed_count == 0
    assert result.dead_letter_count == 0
    assert result.is_resilient is True


def test_outbox_retry_and_dead_letter_quarantine(service):
    """Test retry backoff and dead-letter quarantine exceeding threshold (NFR-028, AT-25)."""
    messages = [
        {"id": "msg-fail-1", "event_type": "PAYMENT_NOTIFY", "retry_count": 0},
    ]
    # First failure -> retry 1, pending
    res1 = service.relay_and_reconcile_outbox(messages, max_retries=3, force_fail_pattern="PAYMENT")
    assert res1.failed_count == 1
    assert res1.dead_letter_count == 0
    assert messages[0]["status"] == "PENDING_RETRY"

    # Second failure -> retry 2, pending
    res2 = service.relay_and_reconcile_outbox(messages, max_retries=3, force_fail_pattern="PAYMENT")
    assert messages[0]["status"] == "PENDING_RETRY"

    # Third failure -> retry 3 >= max_retries -> DEAD_LETTER
    res3 = service.relay_and_reconcile_outbox(messages, max_retries=3, force_fail_pattern="PAYMENT")
    assert res3.dead_letter_count == 1
    assert messages[0]["status"] == "DEAD_LETTER"


def test_outbox_dead_letter_reconciliation(service):
    """Test idempotent dead-letter queue reconciliation."""
    messages = [
        {"id": "msg-fail-recon", "event_type": "ALLOCATION_EVENT", "retry_count": 3, "status": "DEAD_LETTER"}
    ]
    # Reconcile when error condition clears
    result = service.relay_and_reconcile_outbox(messages, reconcile_dead_letter=True)
    assert result.reconciled_count == 1
    assert result.published_count == 1
    assert messages[0]["status"] == "PUBLISHED"
    assert "reconciled_at" in messages[0]


def test_cross_channel_access_official_public(service):
    """Official public assets should be accessible across all channels (AT-24)."""
    for ch in ChannelType:
        req = CrossChannelAccessRequest(
            channel=ch,
            resource_id="PUB-MAP-01",
            resource_classification=DataClassification.OFFICIAL_PUBLIC,
            user_role="CITIZEN",
            user_jurisdiction="Kerala",
        )
        res = service.evaluate_cross_channel_access(req)
        assert res.allowed is True
        assert res.projection == "GENERALIZED_PUBLIC"
        assert len(res.redacted_fields) == 0


def test_cross_channel_access_restricted_privileged_vs_unprivileged(service):
    """Privileged collector gets full access; unprivileged gets generalized on tiles/search or denied on exports."""
    # Privileged officer in Wayanad
    req_priv = CrossChannelAccessRequest(
        channel=ChannelType.EXPORT_REPORT_CACHE,
        resource_id="INT-SITE-99",
        resource_classification=DataClassification.INTERNAL_RESTRICTED,
        user_role="DISTRICT_COLLECTOR",
        user_jurisdiction="Wayanad",
        requested_scope="Wayanad",
    )
    res_priv = service.evaluate_cross_channel_access(req_priv)
    assert res_priv.allowed is True
    assert res_priv.projection == "FULL_RESTRICTED"

    # Unprivileged citizen on vector tile -> generalized view
    req_tile = CrossChannelAccessRequest(
        channel=ChannelType.VECTOR_TILE,
        resource_id="INT-SITE-99",
        resource_classification=DataClassification.INTERNAL_RESTRICTED,
        user_role="CITIZEN",
        user_jurisdiction="Kozhikode",
        requested_scope="Wayanad",
    )
    res_tile = service.evaluate_cross_channel_access(req_tile)
    assert res_tile.allowed is True
    assert res_tile.projection == "GENERALIZED_PUBLIC"
    assert "exact_coordinates" in res_tile.redacted_fields

    # Unprivileged citizen on raw COG range -> denied
    req_cog = CrossChannelAccessRequest(
        channel=ChannelType.COG_OBJECT_RANGE,
        resource_id="INT-SITE-99",
        resource_classification=DataClassification.INTERNAL_RESTRICTED,
        user_role="CITIZEN",
        user_jurisdiction="Kozhikode",
        requested_scope="Wayanad",
    )
    res_cog = service.evaluate_cross_channel_access(req_cog)
    assert res_cog.allowed is False
    assert res_cog.projection == "DENIED"


def test_cross_channel_beneficiary_leakage_prevention(service):
    """Confidential beneficiary data strictly shielded across all public channels (NFR-029, NFR-030)."""
    leakage_channels = [
        ChannelType.VECTOR_TILE,
        ChannelType.SEARCH_INDEX,
        ChannelType.OGC_STAC_METADATA,
        ChannelType.COG_OBJECT_RANGE,
        ChannelType.EXPORT_REPORT_CACHE,
    ]
    for ch in leakage_channels:
        req = CrossChannelAccessRequest(
            channel=ch,
            resource_id="BEN-HH-4401",
            resource_classification=DataClassification.CONFIDENTIAL_BENEFICIARY,
            user_role="VIEWER",
            user_jurisdiction="Wayanad",
        )
        res = service.evaluate_cross_channel_access(req)
        assert res.allowed is False
        assert res.projection == "DENIED"
        assert "aadhaar_token" in res.redacted_fields
        assert "vulnerability_score" in res.redacted_fields
        assert res.rls_context_reset_enforced is True


def test_cross_channel_bypass_rls_flag_rejection(service):
    """Explicit bypass RLS flag must be rejected unconditionally."""
    req = CrossChannelAccessRequest(
        channel=ChannelType.REST_API,
        resource_id="RES-01",
        resource_classification=DataClassification.INTERNAL_RESTRICTED,
        user_role="DISTRICT_COLLECTOR",
        user_jurisdiction="Wayanad",
        bypass_rls_flag=True,
    )
    res = service.evaluate_cross_channel_access(req)
    assert res.allowed is False
    assert res.projection == "DENIED"
    assert res.bypass_rls_rejected is True


def test_offline_storage_eviction_and_token_revocation(service):
    """IndexedDB eviction exports signed payload, revokes future sync, disclaims remote wipe (NFR-031, AT-26)."""
    dev = OfflineDeviceRecord(
        device_id="DEV-TEST-01",
        officer_name="P. Vijayan",
        browser_fingerprint="fp-12345",
        binding_token="tok-live-valid",
        key_custody_status="LOCKED",
    )
    service.register_device(dev)

    unsynced = [
        {"parcel_id": "P-W-101", "observed_slope": 12.4},
        {"parcel_id": "P-W-102", "observed_slope": 14.1},
    ]

    recovery = service.handle_storage_eviction("DEV-TEST-01", unsynced)
    assert recovery.recovery_package_exported is True
    assert recovery.unsynced_items_recovered == 2
    assert recovery.future_sync_revoked is True
    assert recovery.remote_wipe_guarantee_disclaimed is True

    stored = service.get_device("DEV-TEST-01")
    assert stored.storage_evicted is True
    assert stored.key_custody_status == "REVOKED"


def test_offline_lost_stolen_revocation(service):
    """Lost/stolen device notification revokes binding token immediately."""
    dev = OfflineDeviceRecord(
        device_id="DEV-LOST-99",
        officer_name="A. Kumar",
        browser_fingerprint="fp-8888",
        binding_token="tok-active-99",
        key_custody_status="LOCKED",
    )
    service.register_device(dev)

    res = service.revoke_lost_device("DEV-LOST-99")
    assert res.future_sync_revoked is True
    stored = service.get_device("DEV-LOST-99")
    assert stored.is_lost_or_stolen is True
    assert "stolen-revoked" in stored.binding_token


def test_coordinated_restore_pass(service):
    """Coordinated restore passes when DB blob references, checkpoints, and keys exist (NFR-032, AT-27)."""
    pkg = CoordinatedRestorePackage(
        backup_id="BK-2026-09-09-FULL",
        snapshot_timestamp=datetime(2026, 9, 9, 0, 0, tzinfo=timezone.utc),
        database_records=[
            {"id": "doc-1", "scan_blob_uri": "s3://sthira-data/scans/p1.pdf"},
            {"id": "doc-2", "photo_blob_uri": "blob://sthira-photos/site1.jpg"},
        ],
        object_blobs={
            "s3://sthira-data/scans/p1.pdf": "sha256-abc111",
            "blob://sthira-photos/site1.jpg": "sha256-def222",
        },
        audit_checkpoints=[
            {"checkpoint_id": "CP-ROOT-01", "checkpoint_hash": "sha256-chain-genesis"}
        ],
        active_signing_keys=["ED25519-KEY-AUDIT-2026-PRIMARY"],
        export_manifests=["manifest-01"],
    )

    eval_result = service.validate_coordinated_restore(pkg)
    assert eval_result.restore_permitted is True
    assert eval_result.status == "RESTORE_PASSED_CONSISTENCY_SET"
    assert eval_result.authoritative_writes_enabled is True
    assert len(eval_result.missing_objects) == 0
    assert len(eval_result.audit_hash) == 64


def test_coordinated_restore_fail_missing_blob(service):
    """Restore fails and blocks authoritative writes if referenced blob is missing (AT-27)."""
    pkg = CoordinatedRestorePackage(
        backup_id="BK-CORRUPT-BLOB",
        snapshot_timestamp=datetime(2026, 9, 9, 0, 0, tzinfo=timezone.utc),
        database_records=[
            {"id": "doc-1", "scan_blob_uri": "s3://sthira-data/scans/missing.pdf"},
        ],
        object_blobs={},  # Missing!
        audit_checkpoints=[{"checkpoint_id": "CP-1", "checkpoint_hash": "hash-1"}],
        active_signing_keys=["KEY-1"],
        export_manifests=[],
    )

    eval_result = service.validate_coordinated_restore(pkg)
    assert eval_result.restore_permitted is False
    assert eval_result.status == "RESTORE_FAILED_INCONSISTENCY_SET"
    assert eval_result.authoritative_writes_enabled is False
    assert "s3://sthira-data/scans/missing.pdf" in eval_result.missing_objects


def test_coordinated_restore_fail_missing_keys_or_checkpoints(service):
    """Restore fails if signing keys or audit checkpoints are omitted."""
    pkg = CoordinatedRestorePackage(
        backup_id="BK-NO-KEYS",
        snapshot_timestamp=datetime(2026, 9, 9, 0, 0, tzinfo=timezone.utc),
        database_records=[],
        object_blobs={},
        audit_checkpoints=[],  # Missing!
        active_signing_keys=[],  # Missing!
        export_manifests=[],
    )

    eval_result = service.validate_coordinated_restore(pkg)
    assert eval_result.restore_permitted is False
    assert len(eval_result.missing_checkpoints) > 0
    assert len(eval_result.missing_signing_keys) > 0


def test_cert_in_clock_sync_and_6hour_incident(service):
    """Verify NTP drift compliance (<1000ms) and statutory 6-hour CERT-In incident deadline (NFR-033, RUL-020)."""
    # 1. Clock drift check
    clk_ok = service.verify_clock_synchronization(current_drift_ms=12.5)
    assert clk_ok["is_synchronized"] is True
    assert clk_ok["status"] == "COMPLIANT"

    clk_bad = service.verify_clock_synchronization(current_drift_ms=1450.0)
    assert clk_bad["is_synchronized"] is False
    assert clk_bad["status"] == "NON_COMPLIANT_EXCESSIVE_DRIFT"

    # 2. Statutory Incident Notification
    det_time = datetime(2026, 9, 9, 10, 0, 0, tzinfo=timezone.utc)
    incident = service.generate_cert_in_incident(
        incident_category="UNAUTHORIZED_ACCESS_ATTEMPT",
        severity="HIGH",
        impacted_assets=["API Gateway", "Outbox Relay"],
        remedial_measures=["Revoked sync tokens", "Reset connection pools"],
        detection_time=det_time,
    )
    assert incident.audit_retention_days == 180
    assert incident.jurisdiction == "India (Resident)"
    # Deadline must be exactly 6 hours later: 16:00:00
    diff_seconds = (incident.statutory_deadline_timestamp - incident.detection_timestamp).total_seconds()
    assert diff_seconds == 6 * 3600


def test_release_assurance_report(service):
    """Release assurance report seals 119 requirements and 100% normative coverage (NFR-035, trd.md §9)."""
    report = service.generate_release_assurance_report(automated_tests_passed=166)
    assert report.total_requirements_count == 119
    assert report.verified_requirements_count == 119
    assert report.normative_rules_coverage_pct == 100.0
    assert report.s01_s54_sources_count == 54
    assert report.release_status == "APPROVED_FOR_RELEASE"
    assert len(report.release_digest_sha256) == 64
