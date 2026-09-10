"""
Sthira Synthetic Fixture Loader (C0-01).
Loads and validates the Wayanad reference test dataset (RUL-008, RUL-079).
"""

import hashlib
import json
from pathlib import Path
from typing import Any, Dict


def get_fixtures_path() -> Path:
    # Look for fixtures directory relative to repo root
    current = Path(__file__).resolve().parent
    while current != current.parent:
        candidate = current / "fixtures"
        if candidate.is_dir():
            return candidate
        current = current.parent
    raise FileNotFoundError("Could not find fixtures directory in repository.")


def compute_file_checksum(file_path: Path) -> str:
    """Compute SHA-256 checksum of an evidence or fixture file."""
    sha256 = hashlib.sha256()
    with open(file_path, "rb") as f:
        for block in iter(lambda: f.read(65536), b""):
            sha256.update(block)
    return sha256.hexdigest()


def load_district_fixture(district_name: str) -> Dict[str, Any]:
    """
    Load and return validated synthetic fixture payload for a specific Kerala district.
    Supports 'wayanad', 'idukki', 'alappuzha' (C4-03).
    """
    fixtures_dir = get_fixtures_path()
    slug = district_name.lower().strip()
    fixture_file = fixtures_dir / f"synthetic_{slug}.json"
    if not fixture_file.exists():
        raise FileNotFoundError(f"District fixture file not found: {fixture_file}")

    checksum = compute_file_checksum(fixture_file)

    with open(fixture_file, "r", encoding="utf-8") as f:
        data = json.load(f)

    data["_metadata"] = {
        "checksum_sha256": checksum,
        "file_name": fixture_file.name,
        "district": district_name,
        "is_synthetic": True,
        "status": "VALIDATED_TEST_FIXTURE",
    }
    return data


def load_wayanad_fixture() -> Dict[str, Any]:
    """Load and return validated synthetic Wayanad fixture payload."""
    return load_district_fixture("wayanad")


def load_idukki_fixture() -> Dict[str, Any]:
    """Load and return validated synthetic Idukki fixture payload (C4-03)."""
    return load_district_fixture("idukki")


def load_alappuzha_fixture() -> Dict[str, Any]:
    """Load and return validated synthetic Alappuzha fixture payload (C4-03)."""
    return load_district_fixture("alappuzha")


def load_uttarakhand_fixture() -> Dict[str, Any]:
    """Load and return validated synthetic Uttarakhand reference pilot payload (C5-02)."""
    fixtures_dir = get_fixtures_path()
    fixture_file = fixtures_dir / "uttarakhand_fixture.json"
    if not fixture_file.exists():
        raise FileNotFoundError(f"Uttarakhand fixture file not found: {fixture_file}")

    checksum = compute_file_checksum(fixture_file)
    with open(fixture_file, "r", encoding="utf-8") as f:
        data = json.load(f)

    data["_metadata"] = {
        "checksum_sha256": checksum,
        "file_name": fixture_file.name,
        "state": "Uttarakhand",
        "district": "Chamoli",
        "is_synthetic": True,
        "status": "VALIDATED_TEST_FIXTURE",
    }
    return data


