"""Focused Phase 1 contract tests."""

from datetime import datetime, timedelta, timezone

import pytest
from pydantic import ValidationError

from sthira_v2.contracts import (
    APIEnvelope,
    AlertState,
    Assignment,
    AssignmentState,
    CAPArea,
    CAPCategory,
    CAPCertainty,
    CAPMessageType,
    CAPScope,
    CAPSeverity,
    CAPStatus,
    CAPUrgency,
    CitizenSession,
    ConsentState,
    EvidenceClass,
    FactState,
    FreshnessState,
    LineString,
    LocalizedText,
    OfficialAlert,
    Point,
    Polygon,
    SourceProvenance,
    ValidationState,
)


NOW = datetime(2026, 9, 11, 6, tzinfo=timezone.utc)


def provenance() -> SourceProvenance:
    return SourceProvenance(
        source_id="GOV-DEMO-001",
        authority_name="Synthetic DDMA Wayanad",
        source_uri="https://example.invalid/synthetic/alert.xml",
        artifact_id="artifact-001",
        artifact_sha256="a" * 64,
        retrieved_at=NOW,
        issued_at=NOW,
        version=1,
        evidence_class=EvidenceClass.SYNTHETIC_DEMO,
    )


def fact_state() -> FactState:
    return FactState(
        validation=ValidationState.VALID,
        freshness=FreshnessState.CURRENT,
        last_refreshed_at=NOW,
    )


def localized(text: str = "Move to the published safe zone.") -> LocalizedText:
    return LocalizedText(language="en", text=text, human_reviewed=True, reviewer="demo-reviewer")


def polygon() -> Polygon:
    return Polygon(
        coordinates=(((76.0, 11.5), (76.1, 11.5), (76.1, 11.6), (76.0, 11.5)),)
    )


def alert(**updates) -> OfficialAlert:
    values = dict(
        identifier="SYNTHETIC-WAYANAD-001",
        sender="demo@example.invalid",
        sent=NOW,
        issued=NOW,
        status=CAPStatus.TEST,
        message_type=CAPMessageType.ALERT,
        scope=CAPScope.PUBLIC,
        language="en",
        categories=(CAPCategory.GEO,),
        event="Synthetic landslide evacuation exercise",
        urgency=CAPUrgency.IMMEDIATE,
        severity=CAPSeverity.SEVERE,
        certainty=CAPCertainty.LIKELY,
        effective=NOW,
        expires=NOW + timedelta(hours=6),
        sender_name="Synthetic DDMA Wayanad",
        headline="Synthetic demo alert",
        description="Synthetic data; not a real emergency.",
        instructions=(localized(),),
        areas=(CAPArea(area_description="Synthetic Meppadi area", polygons=(polygon(),)),),
        lifecycle_state=AlertState.RECEIVED,
        version=1,
        provenance=provenance(),
        fact_state=fact_state(),
    )
    values.update(updates)
    return OfficialAlert(**values)


def test_valid_alert_round_trips_and_schema_exposes_provenance():
    item = alert()
    assert OfficialAlert.model_validate_json(item.model_dump_json()) == item
    schema = OfficialAlert.model_json_schema()
    assert "provenance" in schema["required"]
    assert schema["properties"]["identifier"]["minLength"] == 1


def test_contracts_are_strict_and_forbid_unknown_fields():
    with pytest.raises(ValidationError):
        Point(type="Point", coordinates=[76.1, 11.6])  # strict tuple contract
    with pytest.raises(ValidationError):
        Point(type="Point", coordinates=(76.1, 11.6), invented=True)


def test_all_timestamps_are_timezone_aware_utc():
    with pytest.raises(ValidationError, match="timezone-aware"):
        alert(sent=NOW.replace(tzinfo=None))
    with pytest.raises(ValidationError, match="must use UTC"):
        alert(sent=datetime(2026, 9, 11, 11, 30, tzinfo=timezone(timedelta(hours=5, minutes=30))))


def test_alert_expiry_and_cap_update_reference_are_enforced():
    with pytest.raises(ValidationError, match="expiry"):
        alert(expires=NOW)
    with pytest.raises(ValidationError, match="require references"):
        alert(message_type=CAPMessageType.UPDATE)


def test_geojson_crs_and_geometry_boundaries_are_enforced():
    assert Point(coordinates=(76.1, 11.6)).type == "Point"
    assert LineString(coordinates=((76.0, 11.5), (76.1, 11.6))).type == "LineString"
    with pytest.raises(ValidationError, match="closed"):
        Polygon(coordinates=(((76.0, 11.5), (76.1, 11.5), (76.1, 11.6), (76.0, 11.6)),))
    with pytest.raises(ValidationError, match="bounds"):
        Point(coordinates=(181.0, 11.6))


def test_party_size_and_session_identifier_boundaries():
    base = dict(
        assignment_id="assignment-1",
        alert_id="alert-1",
        citizen_session_id="0123456789abcdef",
        safe_zone_id="safe-1",
        safe_zone_version=1,
        route_id="route-1",
        route_version=1,
        state=AssignmentState.CREATED,
        expires_at=NOW + timedelta(hours=1),
        allocation_policy_version="demo-v1",
        capacity_reserved=False,
        created_at=NOW,
    )
    assert Assignment(party_size=1, **base).party_size == 1
    assert Assignment(party_size=50, **base).party_size == 50
    for size in (0, 51):
        with pytest.raises(ValidationError):
            Assignment(party_size=size, **base)
    with pytest.raises(ValidationError):
        CitizenSession(
            session_id="short",
            language="en",
            location_consent=ConsentState.DENIED,
            voice_consent=ConsentState.DENIED,
            created_at=NOW,
            expires_at=NOW + timedelta(hours=1),
        )


def test_legal_and_illegal_state_transitions():
    assert alert().transition_to(AlertState.VALIDATED).lifecycle_state is AlertState.VALIDATED
    with pytest.raises(ValueError, match="illegal state transition"):
        alert().transition_to(AlertState.ACTIVE)


def test_api_envelope_requires_exactly_data_or_errors():
    envelope = APIEnvelope[OfficialAlert](
        request_id="request-1",
        generated_at=NOW,
        data_version="1",
        source_status=FreshnessState.CURRENT,
        data=alert(),
    )
    assert envelope.data is not None
    with pytest.raises(ValidationError, match="exactly one"):
        APIEnvelope[str](
            request_id="request-2",
            generated_at=NOW,
            data_version="1",
            source_status=FreshnessState.UNKNOWN,
        )
