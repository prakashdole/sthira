"""Smoke coverage for the public v2 status contract."""

from sthira.api.app import app


def test_v2_status_is_exposed_in_openapi():
    schema = app.openapi()
    operation = schema["paths"]["/api/v2/status"]["get"]

    assert "v2-runtime" in operation["tags"]
    assert operation["responses"]["200"]["content"]["application/json"]["schema"]


def test_openapi_document_is_a_valid_v3_boundary():
    schema = app.openapi()

    assert schema["openapi"].startswith("3.")
    assert schema["info"]["title"]
    assert schema["info"]["version"]
