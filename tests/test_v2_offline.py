from datetime import datetime, timedelta, timezone

import pytest

from sthira_v2.offline import CachedGuidance, OfflineGuidanceCache, dialler_uri

UTC = timezone.utc


def test_cached_guidance_reports_expiry_without_claiming_currentness():
    cached = CachedGuidance("alert-1", "demo", "v1", datetime(2026, 9, 11, 12, tzinfo=UTC), ("step",), "112")
    assert cached.state(datetime(2026, 9, 11, 11, tzinfo=UTC)) == "CURRENT"
    assert cached.state(datetime(2026, 9, 11, 13, tzinfo=UTC)) == "EXPIRED"


def test_dialler_requires_explicit_action_and_does_not_claim_connection():
    assert dialler_uri("112", explicit_confirmation=False) is None
    assert dialler_uri("112", explicit_confirmation=True) == "tel:112"
    with pytest.raises(ValueError):
        dialler_uri("not-a-number", explicit_confirmation=True)


def test_offline_cache_hides_expired_or_cancelled_guidance():
    cache = OfflineGuidanceCache()
    guidance = CachedGuidance("alert-1", "demo", "v1", datetime(2026, 9, 11, 12, tzinfo=UTC), ("step",), "112")
    cache.put("route", guidance)
    assert cache.get("route", now=datetime(2026, 9, 11, 11, tzinfo=UTC)) == guidance
    assert cache.get("route", now=datetime(2026, 9, 11, 13, tzinfo=UTC)) is None
    cache.put("route", guidance)
    assert cache.invalidate_alert("alert-1") == 1
    assert cache.get("route", now=datetime(2026, 9, 11, 11, tzinfo=UTC)) is None
