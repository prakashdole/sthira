from datetime import datetime, timedelta, timezone

import pytest

from sthira_v2.security import PrivacyRecord, PrivacyStore, safe_imported_text, validate_public_identifier


def test_imported_text_is_escaped_and_identifiers_are_bounded():
    assert safe_imported_text('<script>alert(1)</script>') == '&lt;script&gt;alert(1)&lt;/script&gt;'
    with pytest.raises(ValueError):
        validate_public_identifier("../secret")


def test_session_expiry_and_raw_voice_deletion_are_enforceable():
    now = datetime(2026, 9, 12, tzinfo=timezone.utc)
    store = PrivacyStore()
    store.put(PrivacyRecord("session-demo-001", now + timedelta(minutes=5), (76.1, 11.5), b"raw"))
    assert store.delete_raw_voice("session-demo-001")
    assert store.get("session-demo-001").raw_voice is None
    store.put(PrivacyRecord("session-expired-001", now - timedelta(seconds=1), None, None))
    assert store.purge_expired(now=now) == 1
