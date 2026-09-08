"""
PUNARVAS-AI Hazard, Exposure & Geospatial Module (ARC-C03 / C1-03).
Normative Reference: rules.md (RUL-015-020, RUL-017 C-FLOOD exclusion, RUL-018 slope runout).
"""

from typing import Dict, List, Optional
from pydantic import BaseModel, Field
from shapely.geometry import Point, Polygon

from punarvas.core.contracts import GeoPoint, GeoPolygon
from punarvas.core.errors import OutOfCoverageError


class HazardLayer(BaseModel):
    layer_id: str
    name: str
    source_id: str  # S01, S02, etc.
    hazard_type: str  # LANDSLIDE_SUSCEPTIBILITY, FLOOD_PROBABILITY
    hazard_level: str  # VERY_HIGH, HIGH, MODERATE, LOW
    is_debris_flow_channel: bool = False
    polygon_coords: List[List[float]]  # list of [lon, lat]
    documented_coverage: str = "Wayanad"


class ExposureEstimate(BaseModel):
    parcel_id: str
    exposed_hazard_layers: List[str]
    max_hazard_level: str
    in_debris_flow_runout: bool
    requires_relocation_review: bool
    uncertainty_notes: str


class HazardService:
    """
    Evaluates spatial hazard overlays and exposure without training custom models (RUL-015).
    Enforces RUL-017 (C-FLOOD exclusion) and RUL-018 (debris flow channel/runout).
    """

    def __init__(self):
        self._layers: Dict[str, HazardLayer] = {}
        self._shapely_polygons: Dict[str, Polygon] = {}

    def register_hazard_layer(self, layer: HazardLayer):
        """Ingest approved official hazard product."""
        # Enforce RUL-017: C-FLOOD cannot be registered for Wayanad operations
        if layer.source_id == "S07" or "C-FLOOD" in layer.name.upper():
            if "WAYANAD" in layer.documented_coverage.upper() or "KERALA" in layer.documented_coverage.upper():
                raise OutOfCoverageError(
                    source_name="C-FLOOD",
                    requested_aoi="Wayanad",
                    valid_coverage="Mahanadi, Godavari, Tapi river basins only",
                )

        self._layers[layer.layer_id] = layer
        poly = Polygon(layer.polygon_coords)
        self._shapely_polygons[layer.layer_id] = poly

    def evaluate_point_exposure(self, parcel_id: str, point: GeoPoint, local_slope_deg: float) -> ExposureEstimate:
        """
        Evaluate parcel exposure against registered hazard zones.
        Enforces RUL-018: local slope alone does NOT establish safety if in debris flow channel.
        """
        pt = Point(point.coordinates[0], point.coordinates[1])
        matched_layers: List[str] = []
        highest_hazard = "NONE"
        in_runout = False

        for layer_id, poly in self._shapely_polygons.items():
            if poly.contains(pt) or poly.touches(pt):
                matched_layers.append(layer_id)
                layer = self._layers[layer_id]
                if layer.hazard_level == "VERY_HIGH":
                    highest_hazard = "VERY_HIGH"
                elif layer.hazard_level == "HIGH" and highest_hazard != "VERY_HIGH":
                    highest_hazard = "HIGH"

                if layer.is_debris_flow_channel:
                    in_runout = True

        # RUL-018 Check: Even if slope < 15 degrees, if in_runout is True, hazard remains critical
        requires_relocation = highest_hazard in ("VERY_HIGH", "HIGH") or in_runout
        uncertainty = "Based on GSI 1:50,000 scale screening. Meso-scale field review required."

        return ExposureEstimate(
            parcel_id=parcel_id,
            exposed_hazard_layers=matched_layers,
            max_hazard_level=highest_hazard,
            in_debris_flow_runout=in_runout,
            requires_relocation_review=requires_relocation,
            uncertainty_notes=uncertainty,
        )


# Global singleton instance
hazard_service = HazardService()
