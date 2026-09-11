"""Fail-closed operational readiness checks for the v2 API boundary."""

from __future__ import annotations

import os
from dataclasses import asdict, dataclass
from enum import StrEnum

from .config import RuntimeProfile, V2Settings


class ReadinessState(StrEnum):
    DEMO_READY = "DEMO_READY"
    READY = "READY"
    BLOCKED_EXTERNAL = "BLOCKED_EXTERNAL"


@dataclass(frozen=True)
class ReadinessReport:
    state: ReadinessState
    profile: str
    database: str
    artifact_storage: str
    source_configuration: str
    migrations: str
    blockers: tuple[str, ...]

    def as_dict(self) -> dict[str, object]:
        report = asdict(self)
        report["state"] = self.state.value
        report["blockers"] = list(self.blockers)
        return report


def build_readiness_report(
    settings: V2Settings | None = None,
    *,
    environ: dict[str, str] | None = None,
) -> ReadinessReport:
    resolved = settings or V2Settings.from_environment()
    environment = os.environ if environ is None else environ
    blockers: list[str] = []

    database_configured = bool(resolved.db_dsn or environment.get("STHIRA_DATABASE_URL"))
    artifact_configured = bool(environment.get("STHIRA_ARTIFACT_STORE"))
    source_configured = bool(resolved.source_authorization)
    migrations_configured = bool(environment.get("STHIRA_MIGRATIONS_HEAD"))

    if resolved.is_demo:
        return ReadinessReport(
            state=ReadinessState.DEMO_READY,
            profile=resolved.profile.value,
            database="not_required_in_demo",
            artifact_storage="synthetic_local_only",
            source_configuration="SYNTHETIC_DEMO",
            migrations="not_required_in_demo",
            blockers=(),
        )

    if not database_configured:
        blockers.append("database configuration is missing")
    if not artifact_configured:
        blockers.append("artifact storage configuration is missing")
    if not source_configured:
        blockers.append("authorized source configuration is missing")
    if not migrations_configured:
        blockers.append("migration head is not declared")

    state = ReadinessState.READY if not blockers else ReadinessState.BLOCKED_EXTERNAL
    return ReadinessReport(
        state=state,
        profile=resolved.profile.value,
        database="configured" if database_configured else "missing",
        artifact_storage="configured" if artifact_configured else "missing",
        source_configuration="configured" if source_configured else "missing",
        migrations="declared" if migrations_configured else "missing",
        blockers=tuple(blockers),
    )
