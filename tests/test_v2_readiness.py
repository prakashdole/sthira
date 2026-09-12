from sthira_v2.config import RuntimeProfile, V2Settings
from sthira_v2.readiness import ReadinessState, build_readiness_report


def settings(profile: RuntimeProfile) -> V2Settings:
    return V2Settings(
        profile=profile,
        db_dsn=None,
        source_authorization=None,
        auth_issuer=None,
        operations_owner=None,
    )


def test_demo_readiness_is_explicitly_synthetic_and_does_not_require_external_services():
    report = build_readiness_report(settings(RuntimeProfile.DEMO), environ={})

    assert report.state is ReadinessState.DEMO_READY
    assert report.source_configuration == "SYNTHETIC_DEMO"
    assert report.blockers == ()


def test_production_readiness_fails_closed_for_missing_operational_configuration():
    report = build_readiness_report(settings(RuntimeProfile.PRODUCTION), environ={})

    assert report.state is ReadinessState.BLOCKED_EXTERNAL
    assert report.database == "missing"
    assert report.artifact_storage == "missing"
    assert report.source_configuration == "missing"
    assert report.migrations == "missing"
    assert len(report.blockers) == 4


def test_pilot_readiness_requires_all_declared_runtime_prerequisites():
    configured = {
        "STHIRA_ARTIFACT_STORE": "s3://approved-artifacts",
        "STHIRA_MIGRATIONS_HEAD": "20260911_01",
    }
    runtime = settings(RuntimeProfile.PILOT)
    runtime = V2Settings(
        profile=runtime.profile,
        db_dsn="postgresql+psycopg://configured",
        source_authorization="approved-source-record",
        auth_issuer=runtime.auth_issuer,
        operations_owner=runtime.operations_owner,
    )

    report = build_readiness_report(runtime, environ=configured)

    assert report.state is ReadinessState.READY
    assert report.blockers == ()
