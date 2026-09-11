"""Loading and basic integrity checks for bundled v2 demo fixtures."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any


DEFAULT_FIXTURE = Path(__file__).resolve().parents[2] / "fixtures" / "v2_wayanad_demo.json"


class FixtureValidationError(ValueError):
    """Raised when a v2 fixture does not satisfy the demo contract."""


def _require(condition: bool, message: str) -> None:
    if not condition:
        raise FixtureValidationError(message)


def _ids(items: Any, field: str) -> set[str]:
    _require(isinstance(items, list) and items, f"{field} must be a non-empty list")
    values: list[str] = []
    for item in items:
        _require(isinstance(item, dict), f"{field} entries must be objects")
        value = item.get("id")
        _require(isinstance(value, str) and bool(value.strip()), f"{field} entries require id")
        values.append(value)
    _require(len(values) == len(set(values)), f"{field} ids must be unique")
    return set(values)


def validate_v2_fixture(data: Any) -> dict[str, Any]:
    """Validate the small, safety-relevant contract shared by v2 demo fixtures."""

    _require(isinstance(data, dict), "fixture root must be an object")
    required = {"schema_version", "provenance", "alert", "red_zones", "safe_zones", "approved_routes", "instruction_assets"}
    missing = sorted(required - data.keys())
    _require(not missing, f"missing required fields: {', '.join(missing)}")

    provenance = data["provenance"]
    _require(isinstance(provenance, dict), "provenance must be an object")
    _require(provenance.get("evidence_class") == "SYNTHETIC_DEMO", "fixture must declare SYNTHETIC_DEMO provenance")
    _require(bool(provenance.get("disclaimer")), "synthetic fixture requires a disclaimer")

    alert = data["alert"]
    _require(isinstance(alert, dict), "alert must be an object")
    for field in ("identifier", "sender", "sent", "status", "msgType", "scope", "info"):
        _require(field in alert, f"alert requires CAP field {field}")
    _require(alert.get("active") is True, "fixture alert must be active")
    _require(alert.get("status") == "Exercise", "synthetic alert status must be Exercise")
    _require(isinstance(alert["info"], dict), "alert info must be an object")

    red_ids = _ids(data["red_zones"], "red_zones")
    safe_ids = _ids(data["safe_zones"], "safe_zones")
    _require(len(red_ids) == 1, "fixture must contain exactly one red zone")
    _require(len(safe_ids) == 3, "fixture must contain exactly three safe zones")
    for zone in data["safe_zones"]:
        capacity = zone.get("capacity")
        _require(isinstance(capacity, int) and not isinstance(capacity, bool) and capacity > 0, "safe zone capacity must be a positive integer")

    route_ids = _ids(data["approved_routes"], "approved_routes")
    _require(bool(route_ids), "fixture requires approved routes")
    for route in data["approved_routes"]:
        _require(route.get("approval") == "SYNTHETIC_DEMO", "route approval must be SYNTHETIC_DEMO")
        _require(route.get("from_zone_id") in red_ids, "route must start at a known red zone")
        _require(route.get("to_safe_zone_id") in safe_ids, "route must end at a known safe zone")

    _ids(data["instruction_assets"], "instruction_assets")
    languages = {asset.get("language") for asset in data["instruction_assets"]}
    _require({"en-IN", "ml-IN"}.issubset(languages), "English and Malayalam instruction assets are required")
    for asset in data["instruction_assets"]:
        _require(bool(asset.get("text")), "instruction asset text is required")
    return data


def load_v2_fixture(path: str | Path | None = None) -> dict[str, Any]:
    """Load a JSON fixture and reject malformed or non-demo content."""

    fixture_path = Path(path) if path is not None else DEFAULT_FIXTURE
    try:
        with fixture_path.open(encoding="utf-8") as handle:
            data = json.load(handle)
    except (OSError, json.JSONDecodeError) as exc:
        raise FixtureValidationError(f"unable to load fixture {fixture_path}: {exc}") from exc
    return validate_v2_fixture(data)
