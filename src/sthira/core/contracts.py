"""
Sthira Shared Domain Contracts & Envelopes (PKG-0C).
Normative Reference: trd.md §3, architecture.md §7, rules.md (RUL-001, RUL-005, RUL-008, RUL-056).
"""

from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Union
from uuid import uuid4
from pydantic import BaseModel, Field, ConfigDict, field_validator

from sthira.core.enums import (
    AuthorityState,
    ClassificationLevel,
    GateState,
    RoleType,
)


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


class BitemporalRecord(BaseModel):
    """
    Bitemporal fact envelope (RUL-005, DEC-004, architecture.md §7.2).
    - valid_time: when the fact applies in the real world
    - system_time: when Sthira recorded or superseded it
    """
    model_config = ConfigDict(frozen=True)

    version_id: str = Field(default_factory=lambda: str(uuid4()))
    valid_time_start: datetime = Field(default_factory=utc_now)
    valid_time_end: Optional[datetime] = None
    system_time: datetime = Field(default_factory=utc_now)
    superseded_by: Optional[str] = None
    is_current: bool = True


class GeographyScope(BaseModel):
    """
    Jurisdictional authorization boundary (RUL-054, ARC-C01).
    """
    model_config = ConfigDict(frozen=True)

    state: str = "Kerala"
    district: Optional[str] = "Wayanad"
    taluk: Optional[str] = None
    lsg_name: Optional[str] = None  # Local Self Government / Grama Panchayat / Municipality
    village: Optional[str] = None

    def contains(self, other: "GeographyScope") -> bool:
        """Verify if current scope encompasses target geography."""
        if self.state != other.state:
            return False
        if self.district and self.district != other.district:
            return False
        if self.taluk and self.taluk != other.taluk:
            return False
        if self.lsg_name and self.lsg_name != other.lsg_name:
            return False
        if self.village and self.village != other.village:
            return False
        return True


class UserContext(BaseModel):
    """
    Authenticated user context and authorization boundary (RUL-054).
    """
    model_config = ConfigDict(frozen=True)

    user_id: str
    username: str
    roles: List[RoleType] = Field(default_factory=lambda: [RoleType.PUBLIC_VIEWER])
    geography_scope: GeographyScope = Field(default_factory=GeographyScope)
    classification_level: ClassificationLevel = ClassificationLevel.INTERNAL
    correlation_id: str = Field(default_factory=lambda: str(uuid4()))

    def has_role(self, role: RoleType) -> bool:
        return role in self.roles


class GeoPoint(BaseModel):
    model_config = ConfigDict(frozen=True)
    type: str = "Point"
    coordinates: List[float]  # [longitude, latitude]

    @field_validator("coordinates")
    @classmethod
    def validate_coords(cls, v: List[float]) -> List[float]:
        if len(v) != 2:
            raise ValueError("Point coordinates must be [longitude, latitude]")
        lon, lat = v
        if not (-180.0 <= lon <= 180.0 and -90.0 <= lat <= 90.0):
            raise ValueError(f"Invalid coordinate bounds: lon={lon}, lat={lat}")
        return v


class GeoPolygon(BaseModel):
    model_config = ConfigDict(frozen=True)
    type: str = "Polygon"
    coordinates: List[List[List[float]]]  # exterior ring + optional interior holes


class GeoMultiPolygon(BaseModel):
    model_config = ConfigDict(frozen=True)
    type: str = "MultiPolygon"
    coordinates: List[List[List[List[float]]]]


class SourceMetadata(BaseModel):
    """
    Strict provenance and evidence metadata required for every decision input (RUL-008).
    """
    model_config = ConfigDict(frozen=True)

    source_id: str  # S01..S54 or custom
    publisher: str
    license_basis: str
    checksum_sha256: str
    acquisition_time: datetime = Field(default_factory=utc_now)
    observation_time: datetime = Field(default_factory=utc_now)
    geographic_scope: str = "Wayanad, Kerala"
    crs: str = "EPSG:4326"
    resolution_m: Optional[float] = None
    lineage_notes: str = ""
    reviewer_id: str
    version: str = "1.0"


class HardGateResult(BaseModel):
    """
    Evaluation of a mandatory hard gate (RUL-029).
    State MUST be one of PASS, FAIL, UNKNOWN, or BLOCKED.
    """
    model_config = ConfigDict(frozen=True)

    gate_id: str
    gate_name: str
    gate_group: str  # Hazard safety, Land/legal, Water/environment, etc.
    state: GateState
    reason: str
    evidence_sources: List[str] = Field(default_factory=list)
    reviewer_id: Optional[str] = None
    policy_version: str = "1.0"
    evaluated_at: datetime = Field(default_factory=utc_now)


class DimensionScore(BaseModel):
    """
    One dimension of a candidate site or household priority (RUL-028, RUL-036).
    Kept separate from hard gates and never combined into an opaque Omega score (RUL-035).
    """
    model_config = ConfigDict(frozen=True)

    dimension_id: str
    name: str
    raw_value: float
    unit: str
    normalized_score: float  # [0.0, 1.0]
    weight: float
    confidence: float  # [0.0, 1.0]
    source_id: str
    contribution: float


class AdvisoryEnvelope(BaseModel):
    """
    Mandatory envelope ensuring every algorithmic output is explicitly advisory (RUL-001).
    """
    is_advisory: bool = True
    statutory_authority: str = "District Disaster Management Authority (DDMA), Wayanad"
    advisory_notice: str = (
        "This algorithmic analysis is advisory decision-support only. "
        "It does not constitute statutory clearance, title verification, or gazetted notification."
    )
    authority_state: AuthorityState = AuthorityState.ANALYTICAL
    policy_version: str = "1.0"
    generated_at: datetime = Field(default_factory=utc_now)
    approver_id: Optional[str] = None
    approval_timestamp: Optional[datetime] = None


class AuditEntry(BaseModel):
    """
    Append-only, tamper-evident audit record (RUL-056, architecture.md §7.3).
    """
    model_config = ConfigDict(frozen=True)

    event_id: str = Field(default_factory=lambda: str(uuid4()))
    timestamp: datetime = Field(default_factory=utc_now)
    actor_id: str
    authority_scope: str
    action: str
    entity_type: str
    entity_id: str
    version_id: str
    prev_hash: str  # SHA-256 of preceding event
    event_hash: str  # SHA-256 of this canonical event payload
    reason: str
    correlation_id: str


class APIResponseEnvelope(BaseModel):
    """
    Uniform API response structure.
    """
    success: bool = True
    data: Optional[Any] = None
    advisory: AdvisoryEnvelope = Field(default_factory=AdvisoryEnvelope)
    meta: Dict[str, Any] = Field(default_factory=dict)
    errors: List[str] = Field(default_factory=list)
