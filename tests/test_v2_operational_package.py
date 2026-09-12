import copy

import pytest

from sthira_v2.fixtures import load_v2_fixture
from sthira_v2.operational_package import OperationalPackageError, package_checksum, validate_operational_package


def test_synthetic_operational_package_validates_without_inference():
    result = validate_operational_package(load_v2_fixture())

    assert result.evidence_class == "SYNTHETIC_DEMO"
    assert result.alert_id == "DEMO-WYD-LANDSLIDE-001"
    assert len(result.safe_zone_ids) == 3
    assert set(result.instruction_languages) == {"en-IN", "ml-IN"}


@pytest.mark.parametrize(
    ("field", "value"),
    [
        ("approved_routes", []),
        ("safe_zones", []),
        ("instruction_assets", []),
    ],
)
def test_missing_operational_facts_are_rejected(field, value):
    package = load_v2_fixture()
    package[field] = value

    with pytest.raises(OperationalPackageError):
        validate_operational_package(package)


def test_cross_reference_and_signature_fail_closed():
    package = copy.deepcopy(load_v2_fixture())
    package["approved_routes"][0]["to_safe_zone_id"] = "unknown"
    with pytest.raises(OperationalPackageError):
        validate_operational_package(package)

    signed_package = copy.deepcopy(load_v2_fixture())
    with pytest.raises(OperationalPackageError):
        validate_operational_package(signed_package, require_signature=True)


def test_signature_metadata_and_verifier_are_required_when_enabled():
    signed_package = copy.deepcopy(load_v2_fixture())
    signed_package["provenance"]["signature"] = {"algorithm": "demo", "key_id": "key-1", "value": "sig"}
    with pytest.raises(OperationalPackageError, match="verifier"):
        validate_operational_package(signed_package, require_signature=True)
    assert validate_operational_package(
        signed_package,
        require_signature=True,
        signature_verifier=lambda _package, signature: signature["value"] == "sig",
    ).package_id == "STHIRA-V2-WAYANAD-DEMO-001"


def test_manifest_checksum_and_required_operational_facts_fail_closed():
    package = copy.deepcopy(load_v2_fixture())
    package["safe_zones"][0]["capacity"] = 121
    with pytest.raises(OperationalPackageError, match="checksum"):
        validate_operational_package(package)

    package = copy.deepcopy(load_v2_fixture())
    package["allocation_policy"]["order"] = ["SZ-DEMO-01"]
    package["provenance"]["checksum_sha256"] = package_checksum(package)
    with pytest.raises(OperationalPackageError, match="allocation policy"):
        validate_operational_package(package)
