"""Source adapter metadata and health reporting without live-network calls."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any, Mapping, Protocol


@dataclass(frozen=True)
class SourceHealth:
    source_id: str
    authority: str
    product: str
    coverage: str
    schema_version: str
    status: str
    last_success_at: datetime | None
    last_valid_artifact_at: datetime | None
    latency_ms: int | None
    age_seconds: int | None
    quarantine_count: int
    operational_owner: str | None
    degraded_reason: str | None = None


@dataclass(frozen=True)
class SourceArtifact:
    """Validated source envelope; the payload remains source-owned data."""

    source_id: str
    authority: str
    product: str
    coverage: str
    schema_version: str
    observed_at: datetime
    issued_at: datetime
    valid_until: datetime | None
    units: str
    payload: Mapping[str, Any]
    evidence_class: str


class SourceAdapter(Protocol):
    source_id: str

    def ingest_fixture(self, artifact: Mapping[str, Any]) -> SourceArtifact:
        """Validate an already acquired artifact without making a network call."""


def validate_fixture_artifact(artifact: Mapping[str, Any], *, expected_source_id: str) -> SourceArtifact:
    if not isinstance(artifact, Mapping):
        raise ValueError("source artifact must be an object")
    required = ("source_id", "authority", "product", "coverage", "schema_version", "observed_at", "issued_at", "units", "payload", "evidence_class")
    if any(not isinstance(artifact.get(key), str) or not artifact[key].strip() for key in required if key != "payload"):
        raise ValueError("source artifact metadata is incomplete")
    if artifact["source_id"] != expected_source_id:
        raise ValueError("source artifact source_id does not match adapter")
    if artifact["evidence_class"] not in {"SYNTHETIC_DEMO", "CAPTURED_OFFICIAL_SAMPLE", "AUTHORIZED_SHADOW", "AUTHORIZED_OPERATIONAL"}:
        raise ValueError("invalid source artifact evidence class")
    if not isinstance(artifact["payload"], Mapping):
        raise ValueError("source artifact payload must be an object")
    parsed_observed = _parse_datetime(artifact["observed_at"], "observed_at")
    parsed_issued = _parse_datetime(artifact["issued_at"], "issued_at")
    valid_until = artifact.get("valid_until")
    parsed_valid_until = _parse_datetime(valid_until, "valid_until") if valid_until is not None else None
    if parsed_valid_until is not None and parsed_valid_until <= parsed_issued:
        raise ValueError("valid_until must be after issued_at")
    return SourceArtifact(
        source_id=artifact["source_id"], authority=artifact["authority"], product=artifact["product"],
        coverage=artifact["coverage"], schema_version=artifact["schema_version"], observed_at=parsed_observed,
        issued_at=parsed_issued, valid_until=parsed_valid_until, units=artifact["units"],
        payload=artifact["payload"], evidence_class=artifact["evidence_class"],
    )


def _parse_datetime(value: str, label: str) -> datetime:
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ValueError(f"{label} must be ISO-8601") from exc
    if parsed.tzinfo is None:
        raise ValueError(f"{label} must include timezone")
    return parsed


class FixtureSourceAdapter:
    """Explicit demo adapter; it cannot be configured as a live connector."""

    def __init__(self, source_id: str) -> None:
        if not source_id.startswith("fixture:"):
            raise ValueError("fixture adapter source ids must start with fixture:")
        self.source_id = source_id

    def ingest_fixture(self, artifact: Mapping[str, Any]) -> SourceArtifact:
        return validate_fixture_artifact(artifact, expected_source_id=self.source_id)


KNOWN_EXTERNAL_SOURCES = (
    ("imd", "IMD", "district warning/nowcast/rainfall", "India"),
    ("cwc", "CWC/NWIC", "water-level/flood context", "India"),
    ("gsi", "GSI Bhusanket", "landslide context", "India"),
    ("incois", "INCOIS", "coastal/ocean context", "India"),
    ("fsi", "FSI", "forest-fire context", "India"),
    ("ncs", "NCS", "earthquake context", "India"),
    ("ksdma", "KSDMA", "state operational context", "Kerala"),
    ("ndem", "NDEM/Bhuvan", "geospatial context", "India"),
)


def unavailable_known_sources() -> tuple[SourceHealth, ...]:
    return tuple(unavailable_source(*item) for item in KNOWN_EXTERNAL_SOURCES)


def unavailable_source(source_id: str, authority: str, product: str, coverage: str) -> SourceHealth:
    return SourceHealth(
        source_id=source_id,
        authority=authority,
        product=product,
        coverage=coverage,
        schema_version="UNKNOWN",
        status="BLOCKED_EXTERNAL",
        last_success_at=None,
        last_valid_artifact_at=None,
        latency_ms=None,
        age_seconds=None,
        quarantine_count=0,
        operational_owner=None,
        degraded_reason="authorized endpoint and sample are unavailable",
    )
