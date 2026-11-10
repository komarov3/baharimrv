"""Buffered square analysis cells derived from plot centroids plus area.

The project-level NDVI figure is computed over the union of these cells rather
than over the bounding box of the centroids: a bbox around scattered plots
averages in farmland, settlement and open water, which drags the project figure
toward the surrounding landscape. See SATELLITE_NDVI_SPEC.md section 5.1.
"""

from __future__ import annotations

import math
from typing import Iterable, Sequence

# A 30 m floor guarantees at least 3x3 of the 10 m bands for sub-0.1 ha records.
MIN_SIDE_METRES = 30.0
METRES_PER_DEGREE_LAT = 111_320.0

Ring = list[list[float]]
Geometry = dict


def cell(lat: float, lon: float, area_hectares: float | None,
         min_side_metres: float = MIN_SIDE_METRES) -> Geometry:
    """Square cell centred on (lat, lon) whose area equals the plot's hectares."""
    side = math.sqrt(max(float(area_hectares or 0.0), 0.0) * 10_000.0)
    side = max(side, min_side_metres)
    dlat = side / 2.0 / METRES_PER_DEGREE_LAT
    cos_lat = math.cos(math.radians(lat))
    dlon = side / 2.0 / (METRES_PER_DEGREE_LAT * (cos_lat if cos_lat else 1.0))
    west, east = lon - dlon, lon + dlon
    south, north = lat - dlat, lat + dlat
    return {
        "type": "Polygon",
        "coordinates": [[[west, south], [east, south], [east, north], [west, north], [west, south]]],
    }


def bbox(geometry: Geometry) -> tuple[float, float, float, float]:
    """(min_lon, min_lat, max_lon, max_lat) over every ring vertex."""
    lons: list[float] = []
    lats: list[float] = []
    for ring in rings(geometry):
        for x, y in ring:
            lons.append(x)
            lats.append(y)
    if not lons:
        raise ValueError("geometry has no vertices")
    return min(lons), min(lats), max(lons), max(lats)


def rings(geometry: Geometry) -> Iterable[Ring]:
    """Every linear ring in a Polygon or MultiPolygon, flattened."""
    kind = geometry.get("type")
    if kind == "Polygon":
        return list(geometry["coordinates"])
    if kind == "MultiPolygon":
        return [ring for polygon in geometry["coordinates"] for ring in polygon]
    raise ValueError(f"unsupported geometry type: {kind}")


def union_bbox(geometries: Sequence[Geometry]) -> tuple[float, float, float, float]:
    """Framing bbox across cells. Not the analysis area; only framing."""
    parts = [bbox(g) for g in geometries]
    return (min(p[0] for p in parts), min(p[1] for p in parts),
            max(p[2] for p in parts), max(p[3] for p in parts))


def search_geometry(geometries: Sequence[Geometry]) -> Geometry:
    """One MultiPolygon for a STAC `intersects` clause.

    A MultiPolygon's coordinates are a list of polygons, and a polygon is a list
    of rings, so a cell's own `coordinates` value is already one polygon. Nesting
    it twice is a valid-looking body both catalogs reject, CDSE with a message
    about Point that points at nothing useful.
    """
    return {"type": "MultiPolygon", "coordinates": [geometry["coordinates"]
                                                    for geometry in geometries]}


def intersects(a: Geometry, box: tuple[float, float, float, float]) -> bool:
    min_lon, min_lat, max_lon, max_lat = box
    for ring in rings(a):
        for x, y in ring:
            if max_lon >= x >= min_lon and max_lat >= y >= min_lat:
                return True
    return False
