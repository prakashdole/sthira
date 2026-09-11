import copy

import pytest

from sthira_v2.fixtures import FixtureValidationError, load_v2_fixture, validate_v2_fixture


def test_wayanad_demo_fixture_has_complete_synthetic_slice():
    fixture = load_v2_fixture()

    assert fixture["provenance"]["evidence_class"] == "SYNTHETIC_DEMO"
    assert fixture["alert"]["active"] is True
    assert fixture["alert"]["status"] == "Exercise"
    assert len(fixture["red_zones"]) == 1
    assert len(fixture["safe_zones"]) == 3
    assert all(zone["capacity"] > 0 for zone in fixture["safe_zones"])
    assert {asset["language"] for asset in fixture["instruction_assets"]} >= {"en-IN", "ml-IN"}


def test_loader_rejects_non_synthetic_provenance():
    fixture = copy.deepcopy(load_v2_fixture())
    fixture["provenance"]["evidence_class"] = "OFFICIAL"

    with pytest.raises(FixtureValidationError, match="SYNTHETIC_DEMO"):
        validate_v2_fixture(fixture)


def test_loader_rejects_route_to_unknown_safe_zone():
    fixture = copy.deepcopy(load_v2_fixture())
    fixture["approved_routes"][0]["to_safe_zone_id"] = "SZ-MISSING"

    with pytest.raises(FixtureValidationError, match="known safe zone"):
        validate_v2_fixture(fixture)


@pytest.mark.parametrize("capacity", [0, -1, 1.5, True])
def test_loader_rejects_invalid_capacity(capacity):
    fixture = copy.deepcopy(load_v2_fixture())
    fixture["safe_zones"][0]["capacity"] = capacity

    with pytest.raises(FixtureValidationError, match="positive integer"):
        validate_v2_fixture(fixture)
