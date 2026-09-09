"""
Integration tests for Phase 14 Platform Reliability, Security & Release Assurance REST API endpoints.
Normative Reference: NFR-028 to NFR-035, AT-24 to AT-30, ARC-C01, ARC-C11, DEC-045.
"""

from datetime import datetime, timezone
import pytest
from fastapi.testclient import TestClient

from punarvas.api.app import app


@pytest.fixture
def client():
    return TestClient(app)


def test_api_outbox_relay_and_reconcile(client):
    """
    Test POST /api/v1/resilience/outbox/relay-reconcile
    """
    # 1. Successful relay
    payload = {
        "messages": [
            {"id": "msg-api-1", "event_type": "BENEFICIARY_ADDED", "payload": {"h_id": "H-1"}},
            {"id": "msg-api-2", "event_type": "PARCEL_SURVEYED", "payload": {"p_id": "P-1"}},
        ],
        "max_retries": 3,
    }
    resp = client.post("/api/v1/resilience/outbox/relay-reconcile", json=payload)
    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data["total_messages"] == 2
    assert data["published_count"] == 2
    assert data["failed_count"] == 0
    assert data["dead_letter_count"] == 0

    # 2. Dead-letter quarantine with forced failure
    fail_payload = {
        "messages": [
            {"id": "msg-fail", "event_type": "PAYMENT_NOTIFY", "retry_count": 2},
        ],
        "max_retries": 3,
        "force_fail_pattern": "PAYMENT",
    }
    resp_fail = client.post("/api/v1/resilience/outbox/relay-reconcile", json=fail_payload)
    assert resp_fail.status_code == 200
    data_fail = resp_fail.json()["data"]
    assert data_fail["failed_count"] == 1
    assert data_fail["dead_letter_count"] == 1

    # 3. Idempotent dead-letter reconciliation
    reconcile_payload = {
        "messages": [
            {"id": "msg-fail", "event_type": "PAYMENT_NOTIFY", "status": "DEAD_LETTER"},
        ],
        "reconcile_dead_letter": True,
    }
    resp_rec = client.post("/api/v1/resilience/outbox/relay-reconcile", json=reconcile_payload)
    assert resp_rec.status_code == 200
    data_rec = resp_rec.json()["data"]
    assert data_rec["reconciled_count"] == 1


def test_api_cross_channel_access_evaluation(client):
    """
    Test POST /api/v1/resilience/access/check-cross-channel
    """
    # 1. Official public asset
    req_pub = {
        "channel": "VECTOR_TILE",
        "resource_id": "PUB-LAYER-01",
        "resource_classification": "OFFICIAL_PUBLIC",
        "user_role": "CITIZEN",
        "user_jurisdiction": "Kerala",
    }
    resp_pub = client.post("/api/v1/resilience/access/check-cross-channel", json=req_pub)
    assert resp_pub.status_code == 200
    data_pub = resp_pub.json()["data"]
    assert data_pub["allowed"] is True
    assert data_pub["projection"] == "GENERALIZED_PUBLIC"

    # 2. Confidential beneficiary data on vector tile -> denied & shielded
    req_leak = {
        "channel": "VECTOR_TILE",
        "resource_id": "BEN-CARD-99",
        "resource_classification": "CONFIDENTIAL_BENEFICIARY",
        "user_role": "PUBLIC_VIEWER",
        "user_jurisdiction": "Wayanad",
    }
    resp_leak = client.post("/api/v1/resilience/access/check-cross-channel", json=req_leak)
    assert resp_leak.status_code == 200
    data_leak = resp_leak.json()["data"]
    assert data_leak["allowed"] is False
    assert data_leak["projection"] == "DENIED"
    assert "aadhaar_token" in data_leak["redacted_fields"]

    # 3. Explicit RLS bypass attempt -> unconditionally rejected
    req_bypass = {
        "channel": "REST_API",
        "resource_id": "BEN-CARD-99",
        "resource_classification": "CONFIDENTIAL_BENEFICIARY",
        "user_role": "DISTRICT_COLLECTOR",
        "user_jurisdiction": "Wayanad",
        "bypass_rls_flag": True,
    }
    resp_bypass = client.post("/api/v1/resilience/access/check-cross-channel", json=req_bypass)
    assert resp_bypass.status_code == 200
    data_bypass = resp_bypass.json()["data"]
    assert data_bypass["allowed"] is False
    assert data_bypass["bypass_rls_rejected"] is True


def test_api_offline_storage_eviction_and_lost(client):
    """
    Test POST /api/v1/resilience/offline/device-eviction and /revoke-lost
    """
    # 1. Storage eviction recovery
    evict_payload = {
        "device_id": "DEV-WAYANAD-TAB-01",
        "unsynced_records": [
            {"survey_id": "SURV-001", "slope_deg": 18.2},
            {"survey_id": "SURV-002", "slope_deg": 12.0},
        ],
    }
    resp_evict = client.post("/api/v1/resilience/offline/device-eviction", json=evict_payload)
    assert resp_evict.status_code == 200
    data_evict = resp_evict.json()["data"]
    assert data_evict["recovery_package_exported"] is True
    assert data_evict["unsynced_items_recovered"] == 2
    assert data_evict["future_sync_revoked"] is True
    assert data_evict["remote_wipe_guarantee_disclaimed"] is True

    # 2. Revoke lost/stolen device
    resp_lost = client.post("/api/v1/resilience/offline/revoke-lost", json={"device_id": "DEV-WAYANAD-TAB-01"})
    assert resp_lost.status_code == 200
    data_lost = resp_lost.json()["data"]
    assert data_lost["future_sync_revoked"] is True


def test_api_coordinated_restore_validation(client):
    """
    Test POST /api/v1/resilience/restore/validate-consistency (AT-27)
    """
    # 1. Complete consistent set -> permitted
    pass_payload = {
        "backup_id": "BK-20260909-CLEAN",
        "snapshot_timestamp": "2026-09-09T00:00:00Z",
        "database_records": [
            {"id": "doc-1", "doc_blob_uri": "s3://punarvas-bucket/file1.pdf"}
        ],
        "object_blobs": {
            "s3://punarvas-bucket/file1.pdf": "sha256-hash-file1"
        },
        "audit_checkpoints": [
            {"checkpoint_id": "CP-1", "checkpoint_hash": "hash-root"}
        ],
        "active_signing_keys": ["KEY-ED25519-2026"],
        "export_manifests": ["manifest-01"],
    }
    resp_pass = client.post("/api/v1/resilience/restore/validate-consistency", json=pass_payload)
    assert resp_pass.status_code == 200
    data_pass = resp_pass.json()["data"]
    assert data_pass["restore_permitted"] is True
    assert data_pass["status"] == "RESTORE_PASSED_CONSISTENCY_SET"
    assert data_pass["authoritative_writes_enabled"] is True

    # 2. Inconsistent set (missing blob reference) -> failed, writes locked
    fail_payload = dict(pass_payload)
    fail_payload["backup_id"] = "BK-INCOMPLETE"
    fail_payload["object_blobs"] = {}  # missing referenced blob!
    resp_fail = client.post("/api/v1/resilience/restore/validate-consistency", json=fail_payload)
    assert resp_fail.status_code == 200
    data_fail = resp_fail.json()["data"]
    assert data_fail["restore_permitted"] is False
    assert data_fail["status"] == "RESTORE_FAILED_INCONSISTENCY_SET"
    assert data_fail["authoritative_writes_enabled"] is False
    assert len(data_fail["missing_objects"]) == 1


def test_api_ntp_and_cert_in_incident(client):
    """
    Test GET /api/v1/resilience/ntp/verify-clock and POST /api/v1/resilience/cert-in/incident
    """
    # 1. NTP clock check
    resp_clk = client.get("/api/v1/resilience/ntp/verify-clock?drift_ms=18.5")
    assert resp_clk.status_code == 200
    data_clk = resp_clk.json()["data"]
    assert data_clk["is_synchronized"] is True
    assert data_clk["status"] == "COMPLIANT"

    # 2. CERT-In Incident Notification
    incident_payload = {
        "incident_category": "RANSOMWARE_OR_MALICIOUS_CODE",
        "severity": "CRITICAL",
        "impacted_assets": ["DB Shard 01", "Vector Tile Cache"],
        "remedial_measures": ["Isolated cluster", "Switched to read-only replica"],
        "reporting_poc": "ciso@punarvas.kerala.gov.in",
    }
    resp_inc = client.post("/api/v1/resilience/cert-in/incident", json=incident_payload)
    assert resp_inc.status_code == 200
    data_inc = resp_inc.json()["data"]
    assert data_inc["audit_retention_days"] == 180
    assert data_inc["jurisdiction"] == "India (Resident)"
    assert "CERT-IN-" in data_inc["incident_id"]


def test_api_release_assurance_report(client):
    """
    Test GET /api/v1/resilience/release-assurance
    """
    resp = client.get("/api/v1/resilience/release-assurance?version=v1.0.0&tests_passed=172")
    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data["release_version"] == "v1.0.0"
    assert data["total_requirements_count"] == 119
    assert data["verified_requirements_count"] == 119
    assert data["normative_rules_coverage_pct"] == 100.0
    assert data["s01_s54_sources_count"] == 54
    assert data["release_status"] == "APPROVED_FOR_RELEASE"
    assert len(data["release_digest_sha256"]) == 64
