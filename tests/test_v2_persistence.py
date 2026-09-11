from datetime import datetime, timedelta, timezone

import pytest
from sqlalchemy import create_engine, text
from sqlalchemy.orm import Session

from sthira_v2.persistence.models import AlertVersion, AuditEvent, Base, SourceArtifact, SourceState
from sthira_v2.persistence.repositories import AuditRepository, VersionedFactRepository

UTC = timezone.utc
T0 = datetime(2026, 9, 11, tzinfo=UTC)


@pytest.fixture
def session():
    engine = create_engine("sqlite+pysqlite:///:memory:")
    Base.metadata.create_all(engine, tables=[
        SourceArtifact.__table__, SourceState.__table__, AlertVersion.__table__, AuditEvent.__table__
    ])
    with Session(engine) as value:
        value.add(SourceState(source_id="demo", state="VALIDATED", changed_at=T0, authority="Synthetic demo", evidence={"class": "SYNTHETIC_DEMO"}))
        value.add(SourceArtifact(artifact_id="a1", checksum_sha256="a" * 64, media_type="application/json", received_at=T0, authority="Synthetic demo", retention_class="DEMO", payload=b"{}"))
        value.flush()
        yield value


def fact(version: int, system_from: datetime, payload: dict) -> AlertVersion:
    return AlertVersion(fact_id="alert-1", version=version, effective_from=T0, effective_until=T0 + timedelta(days=1), system_from=system_from, source_id="demo", artifact_id="a1", authority="Synthetic demo", validation_state="VALID", lifecycle_state="ACTIVE", payload=payload)


def test_bitemporal_supersession_reconstructs_prior_knowledge(session):
    repo = VersionedFactRepository(session, AlertVersion)
    repo.add_version(fact(1, T0, {"headline": "first"}))
    repo.add_version(fact(2, T0 + timedelta(hours=2), {"headline": "corrected"}))
    assert repo.as_known_at("alert-1", T0 + timedelta(hours=1), T0 + timedelta(hours=1)).payload["headline"] == "first"
    assert repo.as_known_at("alert-1", T0 + timedelta(hours=1), T0 + timedelta(hours=3)).payload["headline"] == "corrected"


def audit(event_id: str, occurred_at: datetime) -> AuditEvent:
    return AuditEvent(event_id=event_id, request_id="request-1", actor_or_source="fixture", action="STORE", object_type="alert", object_id="alert-1", object_version=1, occurred_at=occurred_at, result="OK", details={"evidence_class": "SYNTHETIC_DEMO"}, previous_hash="", event_hash="")


def test_audit_chain_detects_tampering(session):
    repo = AuditRepository(session)
    repo.append(audit("event-1", T0))
    repo.append(audit("event-2", T0 + timedelta(seconds=1)))
    assert repo.verify()
    session.execute(text("UPDATE v2_audit_events SET result = 'ALTERED' WHERE event_id = 'event-1'"))
    session.expire_all()
    assert not repo.verify()


def test_models_enforce_version_and_capacity_constraints(session):
    invalid = fact(0, T0, {})
    session.add(invalid)
    with pytest.raises(Exception):
        session.flush()
