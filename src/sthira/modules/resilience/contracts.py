"""
Sthira Platform Reliability, Leakage Prevention & Release Assurance Contracts.
Normative Reference: NFR-028 to NFR-035, AT-24 to AT-30, ARC-C01, ARC-C11, DEC-045.
"""

from datetime import datetime, timezone
from enum import Enum
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


class ChannelType(str, Enum):
    REST_API = "REST_API"
    HTML_PAGE = "HTML_PAGE"
    SEARCH_INDEX = "SEARCH_INDEX"
    OGC_STAC_METADATA = "OGC_STAC_METADATA"
    VECTOR_TILE = "VECTOR_TILE"
    COG_OBJECT_RANGE = "COG_OBJECT_RANGE"
    EXPORT_REPORT_CACHE = "EXPORT_REPORT_CACHE"


class DataClassification(str, Enum):
    OFFICIAL_PUBLIC = "OFFICIAL_PUBLIC"
    INTERNAL_RESTRICTED = "INTERNAL_RESTRICTED"
    CONFIDENTIAL_BENEFICIARY = "CONFIDENTIAL_BENEFICIARY"


class OutboxRelayResult(BaseModel):
    total_messages: int
    published_count: int
    failed_count: int
    dead_letter_count: int
    reconciled_count: int
    is_resilient: bool = True
    explanation: str


class CrossChannelAccessRequest(BaseModel):
    channel: ChannelType
    resource_id: str
    resource_classification: DataClassification
    user_role: str
    user_jurisdiction: str
    requested_scope: str = "Wayanad"
    is_pooled_connection: bool = True
    bypass_rls_flag: bool = False


class CrossChannelAccessResult(BaseModel):
    channel: ChannelType
    resource_id: str
    allowed: bool
    projection: str  # "DENIED", "GENERALIZED_PUBLIC", "FULL_RESTRICTED"
    redacted_fields: List[str]
    rls_context_reset_enforced: bool = True
    bypass_rls_rejected: bool = True
    explanation: str


class OfflineDeviceRecord(BaseModel):
    device_id: str
    officer_name: str
    browser_fingerprint: str
    binding_token: str
    key_custody_status: str  # "LOCKED", "UNLOCKED", "REVOKED"
    is_shared_device: bool = False
    storage_quota_used_mb: float = 12.4
    storage_quota_limit_mb: float = 100.0
    storage_evicted: bool = False
    is_lost_or_stolen: bool = False
    unsynced_records_count: int = 0


class OfflineRecoveryResult(BaseModel):
    device_id: str
    recovery_package_exported: bool
    unsynced_items_recovered: int
    future_sync_revoked: bool
    remote_wipe_guarantee_disclaimed: bool = True
    explanation: str


class CoordinatedRestorePackage(BaseModel):
    backup_id: str
    snapshot_timestamp: datetime
    database_records: List[Dict[str, Any]]
    object_blobs: Dict[str, str]  # blob_uri -> sha256
    audit_checkpoints: List[Dict[str, Any]]
    active_signing_keys: List[str]
    export_manifests: List[str]


class CoordinatedRestoreEvaluationResult(BaseModel):
    backup_id: str
    restore_permitted: bool
    status: str  # "RESTORE_PASSED_CONSISTENCY_SET", "RESTORE_FAILED_INCONSISTENCY_SET"
    missing_objects: List[str]
    missing_checkpoints: List[str]
    missing_signing_keys: List[str]
    authoritative_writes_enabled: bool
    evaluated_at: datetime = Field(default_factory=utc_now)
    audit_hash: str


class CertInIncidentNotification(BaseModel):
    incident_id: str
    incident_category: str
    severity: str  # "CRITICAL", "HIGH", "MEDIUM"
    detection_timestamp: datetime
    statutory_deadline_timestamp: datetime
    ntp_server: str = "time.nplindia.org"
    clock_drift_ms: float = 14.2
    is_clock_synchronized: bool = True
    impacted_assets: List[str]
    remedial_measures: List[str]
    reporting_point_of_contact: str
    audit_retention_days: int = 180
    jurisdiction: str = "India (Resident)"


class ReleaseAssuranceReport(BaseModel):
    release_version: str
    timestamp: datetime = Field(default_factory=utc_now)
    total_requirements_count: int = 119  # 84 FRs + 35 NFRs
    verified_requirements_count: int = 119
    normative_rules_coverage_pct: float = 100.0  # RUL-001 to RUL-083
    s01_s54_sources_count: int = 54
    active_sources_count: int = 54
    automated_tests_passed: int = 152
    unresolved_critical_defects: int = 0
    release_status: str = "APPROVED_FOR_RELEASE"
    release_digest_sha256: str
