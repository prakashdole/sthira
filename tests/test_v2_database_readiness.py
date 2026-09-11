from pathlib import Path

from scripts.check_v2_database import inspect_prerequisites


def test_database_readiness_reports_every_missing_external_prerequisite(tmp_path: Path):
    blockers = inspect_prerequisites(
        environ={},
        root=tmp_path,
        module_available=lambda _name: False,
    )

    assert blockers == [
        "missing Python package: sqlalchemy",
        "missing Python package: alembic",
        "missing Python package: psycopg",
        "DATABASE_URL is not configured",
        "alembic.ini is missing",
        "migrations/env.py is missing",
    ]


def test_database_readiness_rejects_non_postgresql_database(tmp_path: Path):
    (tmp_path / "migrations").mkdir()
    (tmp_path / "alembic.ini").touch()
    (tmp_path / "migrations" / "env.py").touch()

    blockers = inspect_prerequisites(
        environ={"DATABASE_URL": "sqlite:///local.db"},
        root=tmp_path,
        module_available=lambda _name: True,
    )

    assert blockers == ["DATABASE_URL is not a PostgreSQL URL"]
