"""
Unit tests for PKG-0C Shared Domain Contracts, Enums, Audit, and Outbox.
"""

import pytest
from datetime import datetime, timezone
from punarvas.core import (
    GateState,
    AuthorityState,
    RelocationPathway,
    FundingState,
    UserContext,
    GeographyScope,
    RoleType,
    ClassificationLevel,
    GeoPoint,
    AuditLedger,
    AuditIntegrityError,
    TransactionalOutbox,
    OutboxStatus,
    MasterScoreProhibitedError,
    HardGateBlockedError,
    MissingEvidenceError,
    OutOfCoverageError,
    AdvisoryEnvelope,
)


def test_gate_state_taxonomy():
    """Verify RUL-029: Gates must only return PASS, FAIL, UNKNOWN, BLOCKED."""
    valid_states = {s.value for s in GateState}
    assert valid_states == {"PASS", "FAIL", "UNKNOWN", "BLOCKED"}


def test_user_context_geography_scope():
    """Verify RUL-054: Geography-based access boundaries."""
    collector_scope = GeographyScope(state="Kerala", district="Wayanad")
    village_scope = GeographyScope(state="Kerala", district="Wayanad", village="Chooralmala")
    diff_district = GeographyScope(state="Kerala", district="Idukki")

    assert collector_scope.contains(village_scope) is True
    assert collector_scope.contains(diff_district) is False

    user = UserContext(
        user_id="officer-01",
        username="wayanad_collector",
        roles=[RoleType.GOVERNMENT_APPROVER],
        geography_scope=collector_scope,
        classification_level=ClassificationLevel.RESTRICTED,
    )
    assert user.has_role(RoleType.GOVERNMENT_APPROVER) is True
    assert user.has_role(RoleType.GIS_ANALYST) is False


def test_geo_point_bounds_validation():
    """Validate latitude and longitude bounds."""
    pt = GeoPoint(coordinates=[76.13, 11.55])  # Wayanad coordinates
    assert pt.coordinates == [76.13, 11.55]

    with pytest.raises(ValueError):
        GeoPoint(coordinates=[200.0, 11.55])  # Invalid longitude


def test_advisory_envelope_defaults():
    """Verify RUL-001: All outputs must be advisory until officially approved."""
    env = AdvisoryEnvelope()
    assert env.is_advisory is True
    assert "advisory decision-support only" in env.advisory_notice
    assert env.authority_state == AuthorityState.ANALYTICAL


def test_audit_ledger_hash_chaining():
    """Verify RUL-056: Append-only tamper-evident hash chaining."""
    ledger = AuditLedger()
    e1 = ledger.log(
        actor_id="usr-1",
        authority_scope="Wayanad",
        action="INGEST_HAZARD",
        entity_type="HazardZone",
        entity_id="hz-001",
        version_id="v1",
        reason="KSDMA 2022 GSI update",
    )
    e2 = ledger.log(
        actor_id="usr-2",
        authority_scope="Wayanad",
        action="EVALUATE_SITE",
        entity_type="CandidateSite",
        entity_id="site-elstone",
        version_id="v1",
        reason="Township screening",
    )

    assert e1.prev_hash == "0" * 64
    assert e2.prev_hash == e1.event_hash
    assert ledger.verify_integrity() is True


def test_audit_ledger_detects_tampering():
    """Verify that tampering with any audit record breaks cryptographic verification."""
    ledger = AuditLedger()
    ledger.log("usr-1", "Wayanad", "ACTION_1", "Site", "s1", "v1", "Initial")
    ledger.log("usr-2", "Wayanad", "ACTION_2", "Site", "s1", "v2", "Update")

    # Manually tamper with an entry
    tampered_entry = ledger._entries[0].model_copy(update={"action": "TAMPERED_ACTION"})
    ledger._entries[0] = tampered_entry

    with pytest.raises(AuditIntegrityError):
        ledger.verify_integrity()


def test_transactional_outbox_idempotency_and_relay():
    """Verify outbox deduplication and reliable delivery (architecture.md §6)."""
    outbox = TransactionalOutbox()
    received_payloads = []

    def dummy_handler(msg):
        received_payloads.append(msg.payload)

    outbox.register_handler("site.evaluated", dummy_handler)

    m1 = outbox.enqueue("site.evaluated", {"site_id": "elstone-01"}, idempotency_key="key-123")
    m2 = outbox.enqueue("site.evaluated", {"site_id": "elstone-01"}, idempotency_key="key-123")

    # Idempotent deduplication: m1 and m2 are identical
    assert m1.message_id == m2.message_id
    assert len(outbox.get_pending()) == 1

    # Relay
    relayed = outbox.relay_pending()
    assert relayed == 1
    assert len(received_payloads) == 1
    assert m1.status == OutboxStatus.PUBLISHED


def test_rule_prohibitions():
    """Test prohibition exceptions (RUL-013, RUL-017, RUL-029, RUL-035)."""
    with pytest.raises(MasterScoreProhibitedError):
        raise MasterScoreProhibitedError()

    with pytest.raises(OutOfCoverageError):
        raise OutOfCoverageError("C-FLOOD", "Wayanad", "Godavari, Tapi, Mahanadi")

    with pytest.raises(HardGateBlockedError):
        raise HardGateBlockedError("WaterFeasibility", "FAIL", "Yield under 55 LPCD")

    with pytest.raises(MissingEvidenceError):
        raise MissingEvidenceError("CadastralSurvey")
