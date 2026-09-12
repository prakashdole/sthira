import pytest

from sthira_v2.source_health import FixtureSourceAdapter, unavailable_known_sources, unavailable_source


def test_unavailable_source_is_explicitly_degraded():
    health = unavailable_source("imd", "IMD", "district rainfall", "Wayanad")
    assert health.status == "BLOCKED_EXTERNAL"
    assert health.last_valid_artifact_at is None
    assert health.degraded_reason


def test_fixture_adapter_preserves_observation_metadata_and_provenance():
    adapter = FixtureSourceAdapter("fixture:imd-demo")
    artifact = adapter.ingest_fixture({
        "source_id": "fixture:imd-demo",
        "authority": "Synthetic IMD-shaped fixture",
        "product": "rainfall",
        "coverage": "Wayanad",
        "schema_version": "demo-1",
        "observed_at": "2026-09-11T09:00:00Z",
        "issued_at": "2026-09-11T09:05:00Z",
        "valid_until": "2026-09-11T10:00:00Z",
        "units": "mm",
        "payload": {"value": 12.5, "kind": "OBSERVATION"},
        "evidence_class": "SYNTHETIC_DEMO",
    })
    assert artifact.evidence_class == "SYNTHETIC_DEMO"
    assert artifact.units == "mm"
    assert artifact.payload["kind"] == "OBSERVATION"


def test_fixture_adapter_rejects_wrong_source_and_bad_time_or_units_metadata():
    adapter = FixtureSourceAdapter("fixture:imd-demo")
    base = {
        "source_id": "fixture:other", "authority": "Demo", "product": "rainfall", "coverage": "Wayanad",
        "schema_version": "1", "observed_at": "2026-09-11T09:00:00Z", "issued_at": "2026-09-11T09:05:00Z",
        "units": "mm", "payload": {}, "evidence_class": "SYNTHETIC_DEMO",
    }
    with pytest.raises(ValueError, match="source_id"):
        adapter.ingest_fixture(base)
    base["source_id"] = "fixture:imd-demo"
    base["observed_at"] = "not-a-date"
    with pytest.raises(ValueError, match="observed_at"):
        adapter.ingest_fixture(base)


def test_all_known_live_connectors_are_explicitly_blocked():
    health = unavailable_known_sources()
    assert len(health) == 8
    assert {item.status for item in health} == {"BLOCKED_EXTERNAL"}
