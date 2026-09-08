"""
Tests for Synthetic Wayanad Fixtures & Spikes (C0-01).
"""

from punarvas.spikes import load_wayanad_fixture


def test_synthetic_wayanad_fixture_integrity():
    data = load_wayanad_fixture()
    assert "_metadata" in data
    assert len(data["_metadata"]["checksum_sha256"]) == 64
    assert data["programme"]["district"] == "Wayanad"
    assert len(data["affected_parcels"]) == 4
    assert len(data["candidate_sites"]) == 3
    assert len(data["synthetic_households"]) == 4

    # Verify Elstone Estate candidate site
    elstone = next(s for s in data["candidate_sites"] if s["site_id"] == "SITE-ELSTONE-01")
    assert elstone["dwelling_capacity"] == 250
    assert elstone["unit_plot_cents"] == 7.0
    assert elstone["lean_season_water_state"] == "PASS"
    assert elstone["tested_water_lpcd"] >= 55.0  # Above JJM baseline (RUL-030)
