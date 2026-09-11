"""Strict, persistence-independent contracts for the Sthira v2 boundary.

All text in these models is inert data.  Consumers must escape it for their
output context and must never evaluate imported source text.
"""

from __future__ import annotations

from datetime import datetime, timezone
from enum import StrEnum
from typing import Generic, Literal, TypeVar

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator


Identifier = str
Version = int
LanguageTag = str


class ContractModel(BaseModel):
    model_config = ConfigDict(strict=True, extra="forbid", frozen=True, use_enum_values=False)


def _utc(value: datetime) -> datetime:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("timestamp must be timezone-aware")
    if value.utcoffset() != timezone.utc.utcoffset(value):
        raise ValueError("timestamp must use UTC")
    return value


class EvidenceClass(StrEnum):
    SYNTHETIC_DEMO = "SYNTHETIC_DEMO"
    OFFICIAL = "OFFICIAL"


class ValidationState(StrEnum):
    PENDING = "PENDING"
    VALID = "VALID"
    INVALID = "INVALID"
    QUARANTINED = "QUARANTINED"
    CONFLICTING = "CONFLICTING"


class FreshnessState(StrEnum):
    CURRENT = "CURRENT"
    STALE = "STALE"
    EXPIRED = "EXPIRED"
    UNAVAILABLE = "UNAVAILABLE"
    UNKNOWN = "UNKNOWN"


class SourceProvenance(ContractModel):
    source_id: Identifier = Field(min_length=1, max_length=200)
    authority_name: str = Field(min_length=1, max_length=300)
    source_uri: str = Field(min_length=1, max_length=2048)
    artifact_id: Identifier = Field(min_length=1, max_length=200)
    artifact_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    retrieved_at: datetime
    issued_at: datetime
    version: Version = Field(ge=1)
    evidence_class: EvidenceClass

    _timestamps = field_validator("retrieved_at", "issued_at")(_utc)


class FactState(ContractModel):
    validation: ValidationState
    freshness: FreshnessState
    last_refreshed_at: datetime

    _timestamp = field_validator("last_refreshed_at")(_utc)


class LocalizedText(ContractModel):
    language: LanguageTag = Field(pattern=r"^[a-z]{2,3}(?:-[A-Z]{2})?$")
    text: str = Field(min_length=1, max_length=20_000)
    human_reviewed: bool
    reviewer: str | None = Field(default=None, max_length=200)

    @model_validator(mode="after")
    def reviewed_content_has_reviewer(self) -> "LocalizedText":
        if self.human_reviewed and not self.reviewer:
            raise ValueError("human-reviewed text requires a reviewer")
        return self


class Point(ContractModel):
    type: Literal["Point"] = "Point"
    coordinates: tuple[float, float]

    @field_validator("coordinates")
    @classmethod
    def valid_coordinates(cls, value: tuple[float, float]) -> tuple[float, float]:
        longitude, latitude = value
        if not (-180 <= longitude <= 180 and -90 <= latitude <= 90):
            raise ValueError("point is outside longitude/latitude bounds")
        return value


class LineString(ContractModel):
    type: Literal["LineString"] = "LineString"
    coordinates: tuple[tuple[float, float], ...] = Field(min_length=2)

    @field_validator("coordinates")
    @classmethod
    def valid_coordinates(cls, value: tuple[tuple[float, float], ...]):
        for longitude, latitude in value:
            if not (-180 <= longitude <= 180 and -90 <= latitude <= 90):
                raise ValueError("line coordinate is outside longitude/latitude bounds")
        return value


class Polygon(ContractModel):
    type: Literal["Polygon"] = "Polygon"
    coordinates: tuple[tuple[tuple[float, float], ...], ...] = Field(min_length=1)

    @field_validator("coordinates")
    @classmethod
    def valid_coordinates(cls, value: tuple[tuple[tuple[float, float], ...], ...]):
        for ring in value:
            if len(ring) < 4 or ring[0] != ring[-1]:
                raise ValueError("polygon rings must be closed and contain at least four positions")
            for longitude, latitude in ring:
                if not (-180 <= longitude <= 180 and -90 <= latitude <= 90):
                    raise ValueError("polygon coordinate is outside longitude/latitude bounds")
        return value


class CAPStatus(StrEnum):
    ACTUAL = "Actual"
    EXERCISE = "Exercise"
    SYSTEM = "System"
    TEST = "Test"
    DRAFT = "Draft"


class CAPMessageType(StrEnum):
    ALERT = "Alert"
    UPDATE = "Update"
    CANCEL = "Cancel"
    ACK = "Ack"
    ERROR = "Error"


class CAPScope(StrEnum):
    PUBLIC = "Public"
    RESTRICTED = "Restricted"
    PRIVATE = "Private"


class CAPCategory(StrEnum):
    GEO = "Geo"
    MET = "Met"
    SAFETY = "Safety"
    SECURITY = "Security"
    RESCUE = "Rescue"
    FIRE = "Fire"
    HEALTH = "Health"
    ENV = "Env"
    TRANSPORT = "Transport"
    INFRA = "Infra"
    CBRNE = "CBRNE"
    OTHER = "Other"


class CAPUrgency(StrEnum):
    IMMEDIATE = "Immediate"
    EXPECTED = "Expected"
    FUTURE = "Future"
    PAST = "Past"
    UNKNOWN = "Unknown"


class CAPSeverity(StrEnum):
    EXTREME = "Extreme"
    SEVERE = "Severe"
    MODERATE = "Moderate"
    MINOR = "Minor"
    UNKNOWN = "Unknown"


class CAPCertainty(StrEnum):
    OBSERVED = "Observed"
    LIKELY = "Likely"
    POSSIBLE = "Possible"
    UNLIKELY = "Unlikely"
    UNKNOWN = "Unknown"


class AlertState(StrEnum):
    RECEIVED = "RECEIVED"
    VALIDATED = "VALIDATED"
    ACTIVE = "ACTIVE"
    EXPIRED = "EXPIRED"
    CANCELLED = "CANCELLED"
    SUPERSEDED = "SUPERSEDED"


class CAPArea(ContractModel):
    area_description: str = Field(min_length=1, max_length=2000)
    polygons: tuple[Polygon, ...] = ()
    geocodes: dict[str, str] = Field(default_factory=dict)


class CAPResource(ContractModel):
    description: str = Field(min_length=1, max_length=1000)
    mime_type: str | None = Field(default=None, max_length=200)
    uri: str = Field(min_length=1, max_length=2048)
    digest: str | None = Field(default=None, max_length=200)


_ALERT_TRANSITIONS = {
    AlertState.RECEIVED: {AlertState.VALIDATED},
    AlertState.VALIDATED: {AlertState.ACTIVE},
    AlertState.ACTIVE: {AlertState.EXPIRED, AlertState.CANCELLED, AlertState.SUPERSEDED},
}


class OfficialAlert(ContractModel):
    identifier: Identifier = Field(min_length=1, max_length=200)
    sender: str = Field(min_length=1, max_length=300)
    sent: datetime
    issued: datetime
    status: CAPStatus
    message_type: CAPMessageType
    scope: CAPScope
    source: str | None = Field(default=None, max_length=2000)
    restriction: str | None = Field(default=None, max_length=2000)
    addresses: tuple[str, ...] = ()
    codes: dict[str, str] = Field(default_factory=dict)
    note: str | None = Field(default=None, max_length=20_000)
    references: tuple[str, ...] = ()
    incidents: tuple[str, ...] = ()
    language: LanguageTag = Field(pattern=r"^[a-z]{2,3}(?:-[A-Z]{2})?$")
    categories: tuple[CAPCategory, ...] = Field(min_length=1)
    event: str = Field(min_length=1, max_length=1000)
    response_types: tuple[str, ...] = ()
    urgency: CAPUrgency
    severity: CAPSeverity
    certainty: CAPCertainty
    audience: str | None = Field(default=None, max_length=2000)
    event_codes: dict[str, str] = Field(default_factory=dict)
    effective: datetime
    onset: datetime | None = None
    expires: datetime
    sender_name: str = Field(min_length=1, max_length=300)
    headline: str = Field(min_length=1, max_length=2000)
    description: str = Field(min_length=1, max_length=20_000)
    instructions: tuple[LocalizedText, ...] = Field(min_length=1)
    areas: tuple[CAPArea, ...] = Field(min_length=1)
    resources: tuple[CAPResource, ...] = ()
    lifecycle_state: AlertState
    version: Version = Field(ge=1)
    provenance: SourceProvenance
    fact_state: FactState

    _timestamps = field_validator("sent", "issued", "effective", "onset", "expires")(
        lambda value: None if value is None else _utc(value)
    )

    @model_validator(mode="after")
    def valid_window_and_cap_lifecycle(self) -> "OfficialAlert":
        if self.expires <= self.effective:
            raise ValueError("alert expiry must be after its effective time")
        if self.message_type in {CAPMessageType.UPDATE, CAPMessageType.CANCEL} and not self.references:
            raise ValueError("CAP updates and cancellations require references")
        return self

    def transition_to(self, state: AlertState) -> "OfficialAlert":
        validate_state_transition(self.lifecycle_state, state, _ALERT_TRANSITIONS)
        return self.model_copy(update={"lifecycle_state": state})


class ZoneType(StrEnum):
    RED = "RED"
    SAFE = "SAFE"


class ZoneStatus(StrEnum):
    DRAFT = "DRAFT"
    PUBLISHED = "PUBLISHED"
    ACTIVE = "ACTIVE"
    EXPIRED = "EXPIRED"
    CANCELLED = "CANCELLED"
    SUPERSEDED = "SUPERSEDED"


class VersionedOfficialFact(ContractModel):
    version: Version = Field(ge=1)
    effective_from: datetime
    effective_until: datetime
    crs: Literal["EPSG:4326"]
    provenance: SourceProvenance
    fact_state: FactState

    _timestamps = field_validator("effective_from", "effective_until")(_utc)

    @model_validator(mode="after")
    def valid_effective_window(self) -> "VersionedOfficialFact":
        if self.effective_until <= self.effective_from:
            raise ValueError("effective_until must be after effective_from")
        return self


class OperationalZoneVersion(VersionedOfficialFact):
    zone_id: Identifier = Field(min_length=1, max_length=200)
    zone_type: ZoneType
    geometry: Polygon
    authority: str = Field(min_length=1, max_length=300)
    status: ZoneStatus


class AccessibilityFeature(StrEnum):
    STEP_FREE = "STEP_FREE"
    ACCESSIBLE_TOILET = "ACCESSIBLE_TOILET"
    SIGN_LANGUAGE_SUPPORT = "SIGN_LANGUAGE_SUPPORT"
    ASSISTED_EVACUATION = "ASSISTED_EVACUATION"


class SafeZoneState(StrEnum):
    DRAFT = "DRAFT"
    PUBLISHED = "PUBLISHED"
    OPEN = "OPEN"
    FULL = "FULL"
    CLOSED = "CLOSED"
    REOPENED = "REOPENED"


_SAFE_ZONE_TRANSITIONS = {
    SafeZoneState.DRAFT: {SafeZoneState.PUBLISHED},
    SafeZoneState.PUBLISHED: {SafeZoneState.OPEN},
    SafeZoneState.OPEN: {SafeZoneState.FULL, SafeZoneState.CLOSED},
    SafeZoneState.FULL: {SafeZoneState.CLOSED, SafeZoneState.REOPENED},
    SafeZoneState.CLOSED: {SafeZoneState.REOPENED},
    SafeZoneState.REOPENED: {SafeZoneState.FULL, SafeZoneState.CLOSED},
}


class SafeZoneVersion(VersionedOfficialFact):
    safe_zone_id: Identifier = Field(min_length=1, max_length=200)
    official_facility_id: Identifier = Field(min_length=1, max_length=200)
    name: tuple[LocalizedText, ...] = Field(min_length=1)
    location: Point
    accessibility: tuple[AccessibilityFeature, ...] = ()
    contacts: tuple[str, ...] = ()
    total_capacity: int = Field(ge=0)
    state: SafeZoneState
    authority_version: str = Field(min_length=1, max_length=200)

    def transition_to(self, state: SafeZoneState) -> "SafeZoneVersion":
        validate_state_transition(self.state, state, _SAFE_ZONE_TRANSITIONS)
        return self.model_copy(update={"state": state})


class TransportMode(StrEnum):
    WALK = "WALK"
    ROAD = "ROAD"
    MIXED = "MIXED"


class ApprovedRouteVersion(VersionedOfficialFact):
    route_id: Identifier = Field(min_length=1, max_length=200)
    alert_id: Identifier = Field(min_length=1, max_length=200)
    origin_zone_id: Identifier = Field(min_length=1, max_length=200)
    destination_safe_zone_id: Identifier = Field(min_length=1, max_length=200)
    geometry: LineString
    ordered_instructions: tuple[LocalizedText, ...] = Field(min_length=1)
    transport_mode: TransportMode
    closures_and_constraints: tuple[LocalizedText, ...] = ()
    authority_version: str = Field(min_length=1, max_length=200)
    active: bool


class OfficialInstructionSet(VersionedOfficialFact):
    instruction_set_id: Identifier = Field(min_length=1, max_length=200)
    alert_id: Identifier = Field(min_length=1, max_length=200)
    instructions: tuple[LocalizedText, ...] = Field(min_length=1)
    authority_version: str = Field(min_length=1, max_length=200)

    @model_validator(mode="after")
    def unique_languages(self) -> "OfficialInstructionSet":
        languages = [item.language for item in self.instructions]
        if len(languages) != len(set(languages)):
            raise ValueError("instruction languages must be unique")
        return self


class ConsentState(StrEnum):
    NOT_ASKED = "NOT_ASKED"
    GRANTED = "GRANTED"
    DENIED = "DENIED"
    WITHDRAWN = "WITHDRAWN"


class CitizenSession(ContractModel):
    session_id: Identifier = Field(min_length=16, max_length=200)
    language: LanguageTag = Field(pattern=r"^[a-z]{2,3}(?:-[A-Z]{2})?$")
    accessibility_preferences: tuple[str, ...] = ()
    location_consent: ConsentState = ConsentState.NOT_ASKED
    voice_consent: ConsentState = ConsentState.NOT_ASKED
    created_at: datetime
    expires_at: datetime

    _timestamps = field_validator("created_at", "expires_at")(_utc)

    @model_validator(mode="after")
    def valid_window(self) -> "CitizenSession":
        if self.expires_at <= self.created_at:
            raise ValueError("session expiry must be after creation")
        return self


class AssignmentState(StrEnum):
    CREATED = "CREATED"
    RESERVED = "RESERVED"
    ARRIVED = "ARRIVED"
    DECLINED = "DECLINED"
    EXPIRED = "EXPIRED"
    CANCELLED = "CANCELLED"


_ASSIGNMENT_TRANSITIONS = {
    AssignmentState.CREATED: {AssignmentState.RESERVED, AssignmentState.CANCELLED},
    AssignmentState.RESERVED: {
        AssignmentState.ARRIVED,
        AssignmentState.DECLINED,
        AssignmentState.EXPIRED,
        AssignmentState.CANCELLED,
    },
}


class Assignment(ContractModel):
    assignment_id: Identifier = Field(min_length=1, max_length=200)
    alert_id: Identifier = Field(min_length=1, max_length=200)
    citizen_session_id: Identifier = Field(min_length=16, max_length=200)
    safe_zone_id: Identifier = Field(min_length=1, max_length=200)
    safe_zone_version: Version = Field(ge=1)
    route_id: Identifier = Field(min_length=1, max_length=200)
    route_version: Version = Field(ge=1)
    party_size: int = Field(ge=1, le=50)
    state: AssignmentState
    expires_at: datetime
    allocation_policy_version: str = Field(min_length=1, max_length=200)
    capacity_reserved: bool
    created_at: datetime

    _timestamps = field_validator("created_at", "expires_at")(_utc)

    @model_validator(mode="after")
    def valid_window_and_reservation(self) -> "Assignment":
        if self.expires_at <= self.created_at:
            raise ValueError("assignment expiry must be after creation")
        if self.state is AssignmentState.RESERVED and not self.capacity_reserved:
            raise ValueError("a RESERVED assignment must reserve capacity")
        return self

    def transition_to(self, state: AssignmentState) -> "Assignment":
        validate_state_transition(self.state, state, _ASSIGNMENT_TRANSITIONS)
        return self.model_copy(update={"state": state})


class ArrivalResponse(StrEnum):
    YES = "YES"
    NO = "NO"


class ArrivalConfirmation(ContractModel):
    confirmation_id: Identifier = Field(min_length=1, max_length=200)
    idempotency_key: Identifier = Field(min_length=8, max_length=200)
    assignment_id: Identifier = Field(min_length=1, max_length=200)
    response: ArrivalResponse
    confirmed_party_size: int = Field(ge=1, le=50)
    confirmed_at: datetime
    input_method: Literal["TOUCH", "KEYBOARD"]

    _timestamp = field_validator("confirmed_at")(_utc)


class CapacityEventType(StrEnum):
    RESERVATION = "RESERVATION"
    ARRIVAL = "ARRIVAL"
    RELEASE = "RELEASE"
    AUTHORIZED_ADJUSTMENT = "AUTHORIZED_ADJUSTMENT"


class CapacityEvent(ContractModel):
    event_id: Identifier = Field(min_length=1, max_length=200)
    idempotency_key: Identifier = Field(min_length=8, max_length=200)
    safe_zone_id: Identifier = Field(min_length=1, max_length=200)
    safe_zone_version: Version = Field(ge=1)
    assignment_id: Identifier | None = Field(default=None, max_length=200)
    delta: int
    event_type: CapacityEventType
    occurred_at: datetime
    actor_or_source: str = Field(min_length=1, max_length=300)
    provenance: SourceProvenance

    _timestamp = field_validator("occurred_at")(_utc)

    @model_validator(mode="after")
    def valid_delta(self) -> "CapacityEvent":
        if self.delta == 0:
            raise ValueError("capacity delta cannot be zero")
        if self.event_type in {CapacityEventType.RESERVATION, CapacityEventType.ARRIVAL} and self.delta >= 0:
            raise ValueError("reservation and arrival deltas must be negative")
        if self.event_type is CapacityEventType.RELEASE and self.delta <= 0:
            raise ValueError("release deltas must be positive")
        if self.event_type is not CapacityEventType.AUTHORIZED_ADJUSTMENT and not self.assignment_id:
            raise ValueError("assignment-linked capacity events require assignment_id")
        return self


class APIError(ContractModel):
    code: str = Field(pattern=r"^[A-Z][A-Z0-9_]*$", max_length=100)
    message: LocalizedText
    field: str | None = Field(default=None, max_length=300)
    correlation_id: Identifier = Field(min_length=1, max_length=200)
    retryable: bool = False


DataT = TypeVar("DataT")


class APIEnvelope(ContractModel, Generic[DataT]):
    request_id: Identifier = Field(min_length=1, max_length=200)
    generated_at: datetime
    data_version: str = Field(min_length=1, max_length=200)
    source_status: FreshnessState
    data: DataT | None = None
    errors: tuple[APIError, ...] = ()

    _timestamp = field_validator("generated_at")(_utc)

    @model_validator(mode="after")
    def data_or_errors(self) -> "APIEnvelope[DataT]":
        if (self.data is None) == (not self.errors):
            raise ValueError("envelope must contain exactly one of data or errors")
        return self


# Readable compatibility names for callers that prefer the longer contract terms.
AlertLifecycleState = AlertState
OperationalZoneType = ZoneType
APIResponseEnvelope = APIEnvelope


def validate_state_transition(current: StrEnum, target: StrEnum, transitions: dict) -> None:
    """Raise a stable conflict-style error for an illegal domain transition."""
    if target not in transitions.get(current, set()):
        raise ValueError(f"illegal state transition: {current.value} -> {target.value}")
