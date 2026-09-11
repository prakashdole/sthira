"""Runtime profile configuration and fail-closed startup checks for v2."""

from __future__ import annotations

import os
from dataclasses import dataclass
from enum import StrEnum


class RuntimeProfile(StrEnum):
    DEMO = "DEMO"
    SHADOW = "SHADOW"
    PILOT = "PILOT"
    PRODUCTION = "PRODUCTION"


@dataclass(frozen=True)
class V2Settings:
    profile: RuntimeProfile
    db_dsn: str | None
    source_authorization: str | None
    auth_issuer: str | None
    operations_owner: str | None

    @classmethod
    def from_environment(cls) -> "V2Settings":
        raw_profile = os.getenv("STHIRA_PROFILE", RuntimeProfile.DEMO.value).strip().upper()
        try:
            profile = RuntimeProfile(raw_profile)
        except ValueError as exc:
            allowed = ", ".join(item.value for item in RuntimeProfile)
            raise RuntimeError(
                f"Invalid STHIRA_PROFILE={raw_profile!r}; expected one of: {allowed}"
            ) from exc
        return cls(
            profile=profile,
            db_dsn=os.getenv("STHIRA_DB_DSN"),
            source_authorization=os.getenv("STHIRA_SOURCE_AUTHORIZATION"),
            auth_issuer=os.getenv("STHIRA_AUTH_ISSUER"),
            operations_owner=os.getenv("STHIRA_OPERATIONS_OWNER"),
        )

    @property
    def is_demo(self) -> bool:
        return self.profile is RuntimeProfile.DEMO

    @property
    def citizen_guidance_enabled(self) -> bool:
        return self.profile in {RuntimeProfile.PILOT, RuntimeProfile.PRODUCTION}

    def missing_production_requirements(self) -> tuple[str, ...]:
        required = {
            "STHIRA_DB_DSN": self.db_dsn,
            "STHIRA_SOURCE_AUTHORIZATION": self.source_authorization,
            "STHIRA_AUTH_ISSUER": self.auth_issuer,
            "STHIRA_OPERATIONS_OWNER": self.operations_owner,
        }
        return tuple(name for name, value in required.items() if not value)


def validate_startup(settings: V2Settings | None = None) -> V2Settings:
    """Fail closed for production-like profiles."""

    resolved = settings or V2Settings.from_environment()
    if resolved.profile in {RuntimeProfile.PILOT, RuntimeProfile.PRODUCTION}:
        missing = resolved.missing_production_requirements()
        if missing:
            raise RuntimeError(
                f"{resolved.profile.value} startup blocked; missing: {', '.join(missing)}"
            )
    return resolved


def profile_metadata(settings: V2Settings | None = None) -> dict[str, object]:
    resolved = settings or V2Settings.from_environment()
    missing = resolved.missing_production_requirements()
    return {
        "product": "Sthira Citizen Emergency Guidance",
        "version": "2.0.0-dev",
        "profile": resolved.profile.value,
        "evidence_class": "SYNTHETIC_DEMO" if resolved.is_demo else "UNVERIFIED_RUNTIME",
        "citizen_guidance_enabled": resolved.citizen_guidance_enabled,
        "startup_ready": not missing if resolved.profile in {RuntimeProfile.PILOT, RuntimeProfile.PRODUCTION} else True,
        "missing_production_requirements": list(missing),
        "legacy_v1_isolation": True,
    }

