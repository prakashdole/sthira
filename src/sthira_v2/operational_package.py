"""Validation boundary for authority-supplied operational packages.

The validator checks relationships and safety-critical completeness only. It
does not infer missing routes, capacity, status, or allocation policy.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
import hashlib
import json
from typing import Any, Callable, Mapping


class OperationalPackageError(ValueError):
    """Raised when an operational package must be quarantined."""


@dataclass(frozen=True)
class PackageValidation:
    package_id: str
    evidence_class: str
    alert_id: str
    red_zone_ids: tuple[str, ...]
    safe_zone_ids: tuple[str, ...]
    route_ids: tuple[str, ...]
    instruction_languages: tuple[str, ...]
    version: int
    jurisdiction: str
    effective_at: datetime
    expires_at: datetime
    checksum_sha256: str


def _require(condition: bool, message: str) -> None:
    if not condition:
        raise OperationalPackageError(message)


def canonical_package_bytes(package: Mapping[str, Any]) -> bytes:
    """Return the signed/checksummed representation without mutable metadata."""
    value = json.loads(json.dumps(package))
    provenance = value.get("provenance", {})
    if isinstance(provenance, dict):
        provenance.pop("checksum_sha256", None)
        provenance.pop("signature", None)
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


def package_checksum(package: Mapping[str, Any]) -> str:
    return hashlib.sha256(canonical_package_bytes(package)).hexdigest()


def validate_operational_package(
    package: Mapping[str, Any],
    *,
    expected_jurisdiction: str | None = None,
    require_signature: bool = False,
    signature_verifier: Callable[[Mapping[str, Any], Mapping[str, Any]], bool] | None = None,
) -> PackageValidation:
    """Validate a complete package without supplying policy defaults."""
    _require(isinstance(package, Mapping), "package must be an object")
    provenance = package.get("provenance")
    _require(isinstance(provenance, Mapping), "package provenance is required")
    evidence_class = provenance.get("evidence_class")
    _require(evidence_class in {"SYNTHETIC_DEMO", "CAPTURED_OFFICIAL_SAMPLE", "AUTHORIZED_SHADOW", "AUTHORIZED_OPERATIONAL"}, "invalid evidence class")
    package_id = provenance.get("dataset_id")
    _require(isinstance(package_id, str) and package_id.strip(), "package dataset_id is required")
    version = provenance.get("version")
    _require(isinstance(version, int) and not isinstance(version, bool) and version >= 1, "package version must be a positive integer")
    jurisdiction = provenance.get("jurisdiction")
    _require(isinstance(jurisdiction, str) and jurisdiction.strip(), "package jurisdiction is required")
    authority = provenance.get("authority")
    _require(isinstance(authority, str) and authority.strip(), "package authority is required")
    effective_at = _parse_time(provenance.get("effective_at"), "package effective_at")
    expires_at = _parse_time(provenance.get("expires_at"), "package expires_at")
    _require(expires_at > effective_at, "package expiry must be after effective time")
    declared_checksum = provenance.get("checksum_sha256")
    _require(isinstance(declared_checksum, str) and len(declared_checksum) == 64, "package checksum is required")
    _require(declared_checksum == package_checksum(package), "package checksum verification failed")
    if expected_jurisdiction is not None:
        _require(jurisdiction == expected_jurisdiction, "package jurisdiction does not match")
    if require_signature:
        signature = provenance.get("signature")
        _require(isinstance(signature, Mapping) and bool(signature.get("value")), "package signature is required")
        _require(bool(signature.get("algorithm")) and bool(signature.get("key_id")), "package signature metadata is required")
        _require(signature_verifier is not None, "package signature verifier is not configured")
        _require(signature_verifier(package, signature), "package signature verification failed")

    alert = package.get("alert")
    _require(isinstance(alert, Mapping) and isinstance(alert.get("identifier"), str), "package alert identifier is required")
    red_zones = package.get("red_zones")
    safe_zones = package.get("safe_zones")
    routes = package.get("approved_routes")
    instructions = package.get("instruction_assets")
    facilities = package.get("facilities")
    policy = package.get("allocation_policy")
    contacts = package.get("emergency_contacts")
    _require(isinstance(red_zones, list) and red_zones, "red_zones are required")
    _require(isinstance(safe_zones, list) and safe_zones, "safe_zones are required")
    _require(isinstance(routes, list) and routes, "approved_routes are required")
    _require(isinstance(instructions, list) and instructions, "instruction_assets are required")
    _require(isinstance(facilities, list) and facilities, "facilities are required")
    _require(isinstance(policy, Mapping) and isinstance(policy.get("order"), list) and policy["order"], "allocation policy is required")
    _require(isinstance(contacts, list) and contacts, "emergency contacts are required")

    def ids(items: list[Mapping[str, Any]], label: str) -> tuple[str, ...]:
        values = tuple(item.get("id") for item in items if isinstance(item, Mapping))
        _require(all(isinstance(value, str) and value.strip() for value in values), f"{label} ids are required")
        _require(len(values) == len(set(values)), f"{label} ids must be unique")
        return values

    red_ids = ids(red_zones, "red_zones")
    safe_ids = ids(safe_zones, "safe_zones")
    route_ids = ids(routes, "approved_routes")
    instruction_ids = ids(instructions, "instruction_assets")
    facility_ids = ids(facilities, "facilities")
    _require(len(instruction_ids) == len(instructions), "instruction_assets entries must be objects")
    _require(tuple(policy["order"]) == tuple(safe_ids), "allocation policy must explicitly order every safe zone")
    _require(all(isinstance(contact, Mapping) and contact.get("number") for contact in contacts), "emergency contact numbers are required")
    _require(all(facility.get("id") in safe_ids for facility in facilities), "facility must reference a safe zone")

    for zone in safe_zones:
        _require(isinstance(zone.get("capacity"), int) and zone["capacity"] >= 0, "safe-zone capacity must be explicit and non-negative")
        location = zone.get("location")
        _require(isinstance(location, list) and len(location) == 2, "safe-zone location is required")
        _require(all(isinstance(value, (int, float)) and not isinstance(value, bool) for value in location), "safe-zone location must be numeric")
        _require(-180 <= location[0] <= 180 and -90 <= location[1] <= 90, "safe-zone location is outside CRS bounds")
        _require(zone.get("status", "OPEN") in {"OPEN", "CLOSED", "FULL", "PUBLISHED"}, "invalid safe-zone status")
    for route in routes:
        _require(route.get("from_zone_id") in red_ids, "route origin must reference a red zone")
        _require(route.get("to_safe_zone_id") in safe_ids, "route destination must reference a safe zone")
        _require(route.get("approval") in {"SYNTHETIC_DEMO", "AUTHORIZED_OPERATIONAL"}, "route approval is not authorized")
        geometry = route.get("geometry")
        _require(isinstance(geometry, Mapping) and geometry.get("type") == "LineString", "route geometry must be a LineString")
        coordinates = geometry.get("coordinates") if isinstance(geometry, Mapping) else None
        _require(isinstance(coordinates, list) and len(coordinates) >= 2, "route geometry requires at least two positions")
        _require(all(isinstance(point, list) and len(point) == 2 for point in coordinates), "route geometry positions are invalid")

    languages = tuple(asset.get("language") for asset in instructions)
    _require(all(isinstance(language, str) and language for language in languages), "instruction language is required")
    return PackageValidation(
        package_id=package_id,
        evidence_class=evidence_class,
        alert_id=alert["identifier"],
        red_zone_ids=red_ids,
        safe_zone_ids=safe_ids,
        route_ids=route_ids,
        instruction_languages=languages,
        version=version,
        jurisdiction=jurisdiction,
        effective_at=effective_at,
        expires_at=expires_at,
        checksum_sha256=declared_checksum,
    )


def _parse_time(value: Any, label: str) -> datetime:
    _require(isinstance(value, str), f"{label} is required")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise OperationalPackageError(f"{label} must be ISO-8601") from exc
    _require(parsed.tzinfo is not None, f"{label} must include timezone")
    return parsed
