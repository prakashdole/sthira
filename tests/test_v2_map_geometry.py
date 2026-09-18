import json
from pathlib import Path

from shapely.geometry import LineString, Point, Polygon


MAP_DATA = Path(__file__).parents[1] / "frontend" / "v2" / "src" / "mapData.json"


def test_relocation_zones_and_shelter_are_outside_red_zone():
    data = json.loads(MAP_DATA.read_text(encoding="utf-8"))
    hazard = Polygon(data["hazard"][0])

    assert not hazard.contains(Point(data["shelter"]))
    for coordinates in data["relocationZones"]:
        assert not hazard.intersects(Polygon(coordinates[0]))


def test_route_is_multisegment_and_connects_user_to_shelter():
    data = json.loads(MAP_DATA.read_text(encoding="utf-8"))
    route = LineString(data["route"])

    assert len(data["route"]) >= 6
    assert tuple(route.coords[0]) == tuple(data["user"])
    assert tuple(route.coords[-1]) == tuple(data["shelter"])
