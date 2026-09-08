"""
Land truth and discrepancy package.
"""
from punarvas.modules.land_truth.service import (
    ParcelRecord,
    DiscrepancyTask,
    LandTruthService,
    land_truth_service,
)

__all__ = [
    "ParcelRecord",
    "DiscrepancyTask",
    "LandTruthService",
    "land_truth_service",
]
