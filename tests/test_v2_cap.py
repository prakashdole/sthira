from datetime import datetime, timezone, timedelta
from types import SimpleNamespace

from sthira_v2.cap import AlertLifecycleService, CAPError, CAPHTTPAdapter, parse_cap
from fastapi.testclient import TestClient
from sthira.api.app import app


NOW = datetime(2026, 9, 11, 12, tzinfo=timezone.utc)


def xml(identifier="a-1", msg="Alert", refs="", sender="synthetic.ndma.example", expires="2026-09-11T14:00:00Z"):
    return f"""<alert xmlns="urn:oasis:names:tc:emergency:cap:1.2">
      <identifier>{identifier}</identifier><sender>{sender}</sender><sent>2026-09-11T12:00:00Z</sent>
      <status>Actual</status><msgType>{msg}</msgType><scope>Public</scope><references>{refs}</references>
      <info><language>en-US</language><category>Geo</category><event>Flood</event>
      <urgency>Immediate</urgency><severity>Severe</severity><certainty>Observed</certainty>
      <effective>2026-09-11T12:00:00Z</effective><expires>{expires}</expires>
      <headline>Flood warning</headline><description>Move to the approved safe zone.</description>
      <instruction>Follow official instructions.</instruction>
      <area><areaDesc>Wayanad</areaDesc><polygon>11,75 11,75.1 11.1,75.1 11,75</polygon></area>
      </info></alert>""".encode()


def test_valid_alert_preserves_raw_and_fields():
    parsed = parse_cap(xml(), sender_allow_list={"synthetic.ndma.example"}, retrieved_at=NOW)
    assert parsed.raw_xml.startswith(b"<alert")
    assert parsed.alert.identifier == "a-1"
    assert parsed.alert.areas[0].polygons[0].coordinates[0][0] == (75.0, 11.0)


def test_malformed_and_xxe_are_rejected():
    try:
        parse_cap(b"<alert>")
        assert False
    except CAPError:
        pass
    xxe = b'<!DOCTYPE alert [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><alert/>'
    try:
        parse_cap(xxe)
        assert False
    except CAPError:
        pass


def test_lifecycle_deduplicates_updates_cancels_and_expires():
    now = [NOW]
    service = AlertLifecycleService(sender_allow_list={"synthetic.ndma.example"}, clock=lambda: now[0])
    service.ingest(xml(), source_uri="fixture://cap")
    service.ingest(xml(), source_uri="fixture://cap")
    assert len(service.alerts) == 1
    service.ingest(xml("a-2", "Update", "a-1"), source_uri="fixture://cap")
    assert service.get("a-1").alert.lifecycle_state.value == "SUPERSEDED"
    service.ingest(xml("a-3", "Cancel", "a-2"), source_uri="fixture://cap")
    assert service.get("a-2").alert.lifecycle_state.value == "CANCELLED"
    now[0] = NOW + timedelta(hours=3)
    assert service.active() == []


def test_wrong_sender_is_quarantined():
    service = AlertLifecycleService(sender_allow_list={"synthetic.ndma.example"}, clock=lambda: NOW)
    assert service.ingest(xml(sender="untrusted.example")) is None
    assert service.quarantine and "allow-listed" in service.quarantine[0]["reason"]


def test_http_adapter_uses_etag_and_preserves_cache_on_refresh_failure():
    calls = []
    responses = [
        SimpleNamespace(status_code=200, headers={"ETag": '"v1"'}, content=xml()),
        SimpleNamespace(status_code=304, headers={}, content=b""),
        TimeoutError("source timeout"),
    ]

    def fetch(headers, timeout):
        calls.append((headers, timeout))
        response = responses.pop(0)
        if isinstance(response, Exception):
            raise response
        return response

    adapter = CAPHTTPAdapter(fetch, source_uri="fixture://cap", sender_allow_list={"synthetic.ndma.example"}, max_attempts=1)
    first = adapter.refresh(retrieved_at=NOW)
    assert first.state == "UPDATED" and first.parsed is not None
    second = adapter.refresh(retrieved_at=NOW)
    assert second.state == "CACHED_NOT_MODIFIED" and second.parsed is first.parsed
    third = adapter.refresh(retrieved_at=NOW)
    assert third.state == "STALE_CACHE" and third.parsed is first.parsed
    assert calls[1][0]["If-None-Match"] == '"v1"'


def test_http_adapter_never_treats_invalid_first_refresh_as_success():
    def fetch(_headers, _timeout):
        return SimpleNamespace(status_code=200, headers={}, content=b"<not-cap/>")

    result = CAPHTTPAdapter(fetch, source_uri="fixture://cap", sender_allow_list=set(), max_attempts=1).refresh(retrieved_at=NOW)
    assert result.state == "UNAVAILABLE"
    assert result.parsed is None


def test_alert_api_exposes_synthetic_feed_health_and_provenance():
    client = TestClient(app)
    active = client.get("/api/v2/alerts/active")
    assert active.status_code == 200
    assert active.json()["source"] == "SYNTHETIC_DEMO"
    assert client.get("/api/v2/alerts/health").json()["data"]["status"] == "SYNTHETIC_DEMO"
    assert client.get("/api/v2/alerts/quarantine").json()["source_status"] == "SYNTHETIC_DEMO"
