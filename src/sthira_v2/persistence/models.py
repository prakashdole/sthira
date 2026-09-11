"""Typed SQLAlchemy 2 models for versioned official facts and operations."""

from __future__ import annotations

from datetime import datetime
from typing import Any

from geoalchemy2 import Geometry
from sqlalchemy import JSON, BigInteger, Boolean, CheckConstraint, DateTime, ForeignKey, Index, Integer, String, Text, UniqueConstraint, text
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


JSONValue = dict[str, Any]
Geometry4326 = Geometry(geometry_type="GEOMETRY", srid=4326, spatial_index=True).with_variant(JSON(), "sqlite")


class SourceArtifact(Base):
    __tablename__ = "v2_source_artifacts"
    artifact_id: Mapped[str] = mapped_column(String(200), primary_key=True)
    checksum_sha256: Mapped[str] = mapped_column(String(64), nullable=False, unique=True)
    media_type: Mapped[str] = mapped_column(String(200), nullable=False)
    received_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    authority: Mapped[str] = mapped_column(String(300), nullable=False)
    retention_class: Mapped[str] = mapped_column(String(100), nullable=False)
    payload: Mapped[bytes] = mapped_column(nullable=False)
    __table_args__ = (CheckConstraint("length(checksum_sha256) = 64", name="ck_artifact_sha256_length"),)


class SourceState(Base):
    __tablename__ = "v2_source_states"
    source_id: Mapped[str] = mapped_column(String(200), primary_key=True)
    state: Mapped[str] = mapped_column(String(32), nullable=False)
    changed_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    authority: Mapped[str] = mapped_column(String(300), nullable=False)
    evidence: Mapped[JSONValue] = mapped_column(JSON, nullable=False, default=dict)
    __table_args__ = (CheckConstraint("state IN ('DISCOVERED','ACCESS_REQUESTED','SAMPLE_ACQUIRED','VALIDATED','AUTHORIZED','OPERATIONAL','SUSPENDED','RETIRED')", name="ck_source_state"),)


class VersionedFactMixin:
    row_id: Mapped[int] = mapped_column(BigInteger().with_variant(Integer, "sqlite"), primary_key=True, autoincrement=True)
    fact_id: Mapped[str] = mapped_column(String(200), nullable=False)
    version: Mapped[int] = mapped_column(Integer, nullable=False)
    effective_from: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    effective_until: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    system_from: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    system_until: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    source_id: Mapped[str] = mapped_column(ForeignKey("v2_source_states.source_id"), nullable=False)
    artifact_id: Mapped[str] = mapped_column(ForeignKey("v2_source_artifacts.artifact_id"), nullable=False)
    authority: Mapped[str] = mapped_column(String(300), nullable=False)
    validation_state: Mapped[str] = mapped_column(String(32), nullable=False)
    payload: Mapped[JSONValue] = mapped_column(JSON, nullable=False)


_fact_constraints = (
    UniqueConstraint("fact_id", "version", name="uq_fact_version"),
    CheckConstraint("version > 0", name="ck_fact_version_positive"),
    CheckConstraint("effective_until > effective_from", name="ck_fact_effective_window"),
    CheckConstraint("system_until IS NULL OR system_until > system_from", name="ck_fact_system_window"),
    CheckConstraint("validation_state IN ('PENDING','VALID','INVALID','QUARANTINED','CONFLICTING')", name="ck_fact_validation"),
)


class AlertVersion(VersionedFactMixin, Base):
    __tablename__ = "v2_alert_versions"
    lifecycle_state: Mapped[str] = mapped_column(String(32), nullable=False)
    __table_args__ = _fact_constraints + (Index("ix_alert_current", "fact_id", "system_until"),)


class ZoneVersion(VersionedFactMixin, Base):
    __tablename__ = "v2_zone_versions"
    zone_type: Mapped[str] = mapped_column(String(8), nullable=False)
    geometry: Mapped[Any] = mapped_column(Geometry4326, nullable=False)
    __table_args__ = _fact_constraints + (CheckConstraint("zone_type IN ('RED','SAFE')", name="ck_zone_type"),)


class FacilityVersion(VersionedFactMixin, Base):
    __tablename__ = "v2_facility_versions"
    location: Mapped[Any] = mapped_column(Geometry(geometry_type="POINT", srid=4326).with_variant(JSON(), "sqlite"), nullable=False)
    total_capacity: Mapped[int] = mapped_column(Integer, nullable=False)
    operational_state: Mapped[str] = mapped_column(String(32), nullable=False)
    __table_args__ = _fact_constraints + (CheckConstraint("total_capacity >= 0", name="ck_facility_capacity_nonnegative"),)


class RouteVersion(VersionedFactMixin, Base):
    __tablename__ = "v2_route_versions"
    destination_facility_id: Mapped[str] = mapped_column(String(200), nullable=False)
    geometry: Mapped[Any] = mapped_column(Geometry(geometry_type="LINESTRING", srid=4326).with_variant(JSON(), "sqlite"), nullable=False)
    active: Mapped[bool] = mapped_column(Boolean, nullable=False)
    __table_args__ = _fact_constraints


class InstructionVersion(VersionedFactMixin, Base):
    __tablename__ = "v2_instruction_versions"
    alert_id: Mapped[str] = mapped_column(String(200), nullable=False)
    __table_args__ = _fact_constraints


class AssignmentRecord(Base):
    __tablename__ = "v2_assignments"
    assignment_id: Mapped[str] = mapped_column(String(200), primary_key=True)
    alert_id: Mapped[str] = mapped_column(String(200), nullable=False)
    citizen_session_id: Mapped[str] = mapped_column(String(200), nullable=False)
    safe_zone_id: Mapped[str] = mapped_column(String(200), nullable=False)
    safe_zone_version: Mapped[int] = mapped_column(Integer, nullable=False)
    route_id: Mapped[str] = mapped_column(String(200), nullable=False)
    route_version: Mapped[int] = mapped_column(Integer, nullable=False)
    party_size: Mapped[int] = mapped_column(Integer, nullable=False)
    state: Mapped[str] = mapped_column(String(32), nullable=False)
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    allocation_policy_version: Mapped[str] = mapped_column(String(200), nullable=False)
    capacity_reserved: Mapped[bool] = mapped_column(Boolean, nullable=False)
    __table_args__ = (CheckConstraint("party_size BETWEEN 1 AND 50", name="ck_assignment_party_size"),)


class CapacityEventRecord(Base):
    __tablename__ = "v2_capacity_events"
    event_id: Mapped[str] = mapped_column(String(200), primary_key=True)
    idempotency_key: Mapped[str] = mapped_column(String(200), nullable=False, unique=True)
    safe_zone_id: Mapped[str] = mapped_column(String(200), nullable=False)
    safe_zone_version: Mapped[int] = mapped_column(Integer, nullable=False)
    assignment_id: Mapped[str | None] = mapped_column(ForeignKey("v2_assignments.assignment_id"))
    delta: Mapped[int] = mapped_column(Integer, nullable=False)
    event_type: Mapped[str] = mapped_column(String(32), nullable=False)
    occurred_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    actor_or_source: Mapped[str] = mapped_column(String(300), nullable=False)
    artifact_id: Mapped[str] = mapped_column(ForeignKey("v2_source_artifacts.artifact_id"), nullable=False)
    __table_args__ = (CheckConstraint("delta <> 0", name="ck_capacity_delta_nonzero"),)


class AuditEvent(Base):
    __tablename__ = "v2_audit_events"
    sequence: Mapped[int] = mapped_column(BigInteger().with_variant(Integer, "sqlite"), primary_key=True, autoincrement=True)
    event_id: Mapped[str] = mapped_column(String(200), nullable=False, unique=True)
    request_id: Mapped[str] = mapped_column(String(200), nullable=False)
    actor_or_source: Mapped[str] = mapped_column(String(300), nullable=False)
    action: Mapped[str] = mapped_column(String(200), nullable=False)
    object_type: Mapped[str] = mapped_column(String(100), nullable=False)
    object_id: Mapped[str] = mapped_column(String(200), nullable=False)
    object_version: Mapped[int | None] = mapped_column(Integer)
    occurred_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    result: Mapped[str] = mapped_column(String(100), nullable=False)
    details: Mapped[JSONValue] = mapped_column(JSON, nullable=False, default=dict)
    previous_hash: Mapped[str] = mapped_column(String(64), nullable=False)
    event_hash: Mapped[str] = mapped_column(String(64), nullable=False, unique=True)
