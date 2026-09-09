"""
PUNARVAS-AI Source Access, Provider Health & Operational Readiness Contracts.
Normative Reference: ARC-C13, FR-076-FR-084, RUL-076-RUL-083, source-register.md.
"""

from datetime import datetime, timezone
from enum import Enum
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


class CapabilityType(str, Enum):
    PRODUCT = "PRODUCT"
    CATALOG = "CATALOG"
    DOWNLOAD = "DOWNLOAD"
    PROCESSING = "PROCESSING"
    DISPLAY = "DISPLAY"
    AGENCY_RECORD = "AGENCY_RECORD"
    FIELD_ACQUISITION = "FIELD_ACQUISITION"


class PriorityClass(str, Enum):
    CORE = "CORE"
    SUPPORT = "SUPPORT"
    AGENCY = "AGENCY"
    CONDITIONAL = "CONDITIONAL"
    CORE_API = "CORE_API"
    ALTERNATIVE_API = "ALTERNATIVE_API"
    OPTIONAL_PROCESSING = "OPTIONAL_PROCESSING"
    SPECIALIST = "SPECIALIST"
    ALTERNATIVE = "ALTERNATIVE"
    OPTIONAL = "OPTIONAL"
    CORE_CONTEXT = "CORE_CONTEXT"
    SUPPORT_API = "SUPPORT_API"
    AGENCY_BLOCKER = "AGENCY_BLOCKER"
    FIELD_BLOCKER = "FIELD_BLOCKER"
    LEGACY_OPTIONAL = "LEGACY_OPTIONAL"
    GEOGRAPHY_CONDITIONAL = "GEOGRAPHY_CONDITIONAL"
    DISPLAY_ONLY = "DISPLAY_ONLY"
    OPTIONAL_PAID = "OPTIONAL_PAID"


class ActivationState(str, Enum):
    REGISTERED = "REGISTERED"
    DOCUMENTED = "DOCUMENTED"
    CATALOG_VISIBLE = "CATALOG_VISIBLE"
    SAMPLE_ACQUIRED = "SAMPLE_ACQUIRED"
    QUARANTINED = "QUARANTINED"
    APPROVED_FOR_USE = "APPROVED_FOR_USE"
    PAUSED = "PAUSED"
    UNSUPPORTED = "UNSUPPORTED"
    DEPRECATED = "DEPRECATED"


class EndpointType(str, Enum):
    DISCOVERY = "DISCOVERY"
    AUTHENTICATION = "AUTHENTICATION"
    ENTITLEMENT = "ENTITLEMENT"
    DOWNLOAD_RECEIPT = "DOWNLOAD_RECEIPT"
    PROCESSING = "PROCESSING"
    DISPLAY = "DISPLAY"


class ReconciliationMethod(str, Enum):
    PRIMARY_AUTHORITATIVE = "PRIMARY_AUTHORITATIVE"
    UNION_DEDUPLICATED = "UNION_DEDUPLICATED"
    LATEST_STABLE_ONLY = "LATEST_STABLE_ONLY"


class BlockerStatus(str, Enum):
    VERIFIED = "VERIFIED"
    MISSING = "MISSING"
    PENDING_REVIEW = "PENDING_REVIEW"
    BLOCKED = "BLOCKED"


class EntitlementState(str, Enum):
    ACTIVE = "ACTIVE"
    PENDING_REVIEW = "PENDING_REVIEW"
    EXPIRED = "EXPIRED"
    UNENTITLED = "UNENTITLED"


class TokenState(str, Enum):
    VALID = "VALID"
    EXPIRING_SOON = "EXPIRING_SOON"
    EXPIRED = "EXPIRED"
    NOT_CONFIGURED = "NOT_CONFIGURED"


class SourceCapabilityRecord(BaseModel):
    """
    S01-S54 operational source record (FR-076, RUL-076).
    """
    source_id: str
    name: str
    capability_type: CapabilityType
    priority_class: PriorityClass
    custodian: str
    owner_role: str
    intended_use: str
    explicit_non_uses: List[str]
    supported_geography: str
    state: ActivationState = ActivationState.REGISTERED
    endpoints: List[EndpointType] = Field(default_factory=lambda: [EndpointType.DISCOVERY])
    mirror_group_id: Optional[str] = None
    is_sandbox_only: bool = False
    is_paused: bool = False
    quarantine_reason: Optional[str] = None
    last_evaluated_at: Optional[datetime] = None
    latest_checksum: Optional[str] = None


class AOISampleGateInput(BaseModel):
    """
    Candidate AOI sample evaluation input (FR-078, RUL-077).
    """
    source_id: str
    sample_id: str
    license_type: str
    has_redistribution_and_offline_rights: bool
    min_lat: float
    max_lat: float
    min_lon: float
    max_lon: float
    observation_timestamp: datetime
    schema_format: str
    crs: str
    vertical_datum: Optional[str] = None
    resolution_meters: float
    units: str
    nodata_value: Optional[str] = None
    raw_payload_checksum: str
    claimed_checksum: str
    reviewer_id: str
    reproducibility_notes: Optional[str] = None
    cost_usd: float = 0.0


class AOISampleGateResult(BaseModel):
    """
    Outcome of Section 6 AOI acquisition gate (FR-078, AT-38).
    """
    sample_id: str
    source_id: str
    status: str  # "APPROVED_FOR_USE" or "QUARANTINED"
    passed: bool
    checks: Dict[str, bool]
    quarantine_reasons: List[str]
    evaluated_at: datetime = Field(default_factory=utc_now)


class MirrorGroupRecord(BaseModel):
    """
    Shared lineage / mirror group specification (FR-079, RUL-078).
    """
    group_id: str
    observation_description: str
    member_source_ids: List[str]
    primary_source_id: str
    reconciliation_strategy: ReconciliationMethod
    deduplication_rule: str


class ObservationReconciliationRequest(BaseModel):
    group_id: str
    observations: List[Dict[str, Any]]


class ObservationReconciliationResult(BaseModel):
    group_id: str
    reconciliation_method: ReconciliationMethod
    total_input_count: int
    reconciled_count: int
    duplicate_count: int
    is_independent_corroboration_rejected: bool = True
    explanation: str


class ProviderHealthStatus(BaseModel):
    """
    Adapter health and telemetry with strict secret redaction (FR-081, RUL-081).
    """
    source_id: str
    provider_name: str
    entitlement_state: EntitlementState
    token_state: TokenState
    quota_used: int
    quota_limit: int
    rate_limit_rpm: int
    estimated_cost_usd: float
    latency_ms: float
    error_rate_pct: float
    source_observation_age_hours: float
    governed_file_fallback_available: bool
    secrets_redacted: bool = True
    redacted_token_preview: str = "***REDACTED***"


class BlockerItem(BaseModel):
    blocker_id: str  # S45..S50
    title: str
    status: BlockerStatus
    evidence_summary: str
    missing_requirements: List[str]


class DependencyBlockerEvaluationRequest(BaseModel):
    site_id: str
    allocation_action: str  # e.g., "LIVE_SITE_APPROVAL" or "BENEFICIARY_ALLOCATION"
    evidence_records: Dict[str, Dict[str, Any]] = Field(default_factory=dict)


class DependencyBlockerReport(BaseModel):
    """
    S45-S50 Blocker Gate report (FR-083, RUL-079, AT-35).
    """
    site_id: str
    action: str
    can_proceed: bool
    overall_status: str  # "PASS", "BLOCKED", "HOLD"
    blockers: Dict[str, BlockerItem]
    blocking_reasons: List[str]
    audit_hash: str
    evaluated_at: datetime = Field(default_factory=utc_now)


class BasemapConfigRecord(BaseModel):
    """
    Basemap display infrastructure and lineage decoupling (FR-084, RUL-082, AT-37).
    """
    provider_id: str
    style_version: str
    attribution: str
    permitted_offline_use: bool
    prohibit_osm_tile_bulk_download: bool = True
    pixels_decoupled_from_analytical_lineage: bool = True
    fallback_mode: str = "NON_MAP_TABULAR_VECTOR"
    is_healthy: bool = True
