#!/usr/bin/env python3
"""Report whether the real v2 PostgreSQL/PostGIS migration path is runnable."""

from __future__ import annotations

import importlib.util
import json
import os
from pathlib import Path
from typing import Callable, Mapping


REQUIRED_MODULES = ("sqlalchemy", "alembic", "psycopg")


def inspect_prerequisites(
    *,
    environ: Mapping[str, str] | None = None,
    root: Path | None = None,
    module_available: Callable[[str], bool] | None = None,
) -> list[str]:
    """Return concrete reasons the database verification cannot safely run."""
    environment = os.environ if environ is None else environ
    project_root = Path(__file__).resolve().parents[1] if root is None else root
    available = module_available or (lambda name: importlib.util.find_spec(name) is not None)
    blockers = [
        f"missing Python package: {name}"
        for name in REQUIRED_MODULES
        if not available(name)
    ]

    database_url = environment.get("DATABASE_URL", "")
    if not database_url:
        blockers.append("DATABASE_URL is not configured")
    elif not database_url.startswith(("postgresql://", "postgresql+psycopg://")):
        blockers.append("DATABASE_URL is not a PostgreSQL URL")

    if not (project_root / "alembic.ini").is_file():
        blockers.append("alembic.ini is missing")
    if not (project_root / "migrations" / "env.py").is_file():
        blockers.append("migrations/env.py is missing")
    return blockers


def inspect_database(database_url: str, root: Path) -> list[str]:
    """Check connectivity, PostGIS, and that the database is at Alembic head."""
    from alembic.config import Config
    from alembic.runtime.migration import MigrationContext
    from alembic.script import ScriptDirectory
    from sqlalchemy import create_engine, text

    blockers: list[str] = []
    try:
        engine = create_engine(database_url, pool_pre_ping=True)
        with engine.connect() as connection:
            postgis_version = connection.execute(
                text("SELECT extversion FROM pg_extension WHERE extname = 'postgis'")
            ).scalar_one_or_none()
            if postgis_version is None:
                blockers.append("PostGIS extension is not installed in the target database")

            alembic_config = Config(str(root / "alembic.ini"))
            alembic_config.set_main_option("script_location", str(root / "migrations"))
            expected_heads = set(ScriptDirectory.from_config(alembic_config).get_heads())
            current_heads = set(MigrationContext.configure(connection).get_current_heads())
            if current_heads != expected_heads:
                blockers.append(
                    "database migration heads do not match: "
                    f"current={sorted(current_heads)}, expected={sorted(expected_heads)}"
                )
    except Exception as exc:  # The command must report unavailable infrastructure honestly.
        blockers.append(f"database verification failed: {type(exc).__name__}: {exc}")
    finally:
        if "engine" in locals():
            engine.dispose()
    return blockers


def readiness_report() -> dict[str, object]:
    root = Path(__file__).resolve().parents[1]
    blockers = inspect_prerequisites(root=root)
    if not blockers:
        blockers.extend(inspect_database(os.environ["DATABASE_URL"], root))
    return {
        "status": "READY" if not blockers else "BLOCKED_EXTERNAL",
        "checks": ["SQLAlchemy", "Alembic", "psycopg", "PostgreSQL/PostGIS", "migration heads"],
        "blockers": blockers,
    }


def main() -> int:
    report = readiness_report()
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0 if report["status"] == "READY" else 2


if __name__ == "__main__":
    raise SystemExit(main())
