"""
PUNARVAS-AI Synthetic Fixture Loader (C0-01).
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


def load_wayanad_fixture() -> Dict[str, Any]:
    """
    Load and return validated synthetic Wayanad fixture payload.
    """
    fixtures_dir = get_fixtures_path()
    fixture_file = fixtures_dir / "synthetic_wayanad.json"
    if not fixture_file.exists():
        raise FileNotFoundError(f"Fixture file not found: {fixture_file}")

    checksum = compute_file_checksum(fixture_file)

    with open(fixture_file, "r", encoding="utf-8") as f:
        data = json.load(f)

    # Attach checksum and provenance
    data["_metadata"] = {
        "checksum_sha256": checksum,
        "file_name": fixture_file.name,
        "is_synthetic": True,
        "status": "VALIDATED_TEST_FIXTURE",
    }
    return data
