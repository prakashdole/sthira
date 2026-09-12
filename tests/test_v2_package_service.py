import copy

import pytest

from sthira_v2.fixtures import load_v2_fixture
from sthira_v2.operational_package import package_checksum
from sthira_v2.package_service import OperationalPackageService, PackageAuthorizationError


def package(jurisdiction="Wayanad"):
    value = copy.deepcopy(load_v2_fixture())
    value["provenance"]["jurisdiction"] = jurisdiction
    value["provenance"]["checksum_sha256"] = package_checksum(value)
    return value


def test_package_publish_supersede_and_rollback_are_atomic():
    service = OperationalPackageService()
    first = service.preview(package(), jurisdiction="Wayanad")
    service.publish(first.validation.package_id, operator_jurisdiction="Wayanad", authenticated=True)
    second_value = package()
    second_value["provenance"]["dataset_id"] = "STHIRA-V2-WAYANAD-DEMO-002"
    second_value["provenance"]["version"] = 2
    second_value["provenance"]["checksum_sha256"] = package_checksum(second_value)
    second = service.preview(second_value, jurisdiction="Wayanad")
    service.publish(second.validation.package_id, operator_jurisdiction="Wayanad", authenticated=True)
    assert service.active(jurisdiction="Wayanad").validation.package_id == second.validation.package_id
    restored = service.rollback(jurisdiction="Wayanad", package_id=first.validation.package_id, authenticated=True)
    assert restored.state == "PUBLISHED"
    assert service.active(jurisdiction="Wayanad").validation.package_id == first.validation.package_id


def test_package_publication_requires_authenticated_matching_operator():
    service = OperationalPackageService()
    record = service.preview(package(), jurisdiction="Wayanad")
    with pytest.raises(PackageAuthorizationError):
        service.publish(record.validation.package_id, operator_jurisdiction="Wayanad", authenticated=False)
    with pytest.raises(PackageAuthorizationError):
        service.publish(record.validation.package_id, operator_jurisdiction="Idukki", authenticated=True)
