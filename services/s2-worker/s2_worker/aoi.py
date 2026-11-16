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


# Shared plot geometry, mirrored line for line in backend/internal/geo/geo.go and the
# Android capture screen so a boundary recorded on a phone yields the same hectares and
# centroid on every layer. The live worker receives cells already built by the Go API;
# these serve the standalone/spike path and keep the two implementations pinned.

def _mean(ring: Ring) -> tuple[float, float]:
    n = len(ring) - 1 if len(ring) > 1 else len(ring)
    lon = sum(p[0] for p in ring[:n]) / n
    lat = sum(p[1] for p in ring[:n]) / n
    return lat, lon


def _project(ring: Ring, lat0: float, lon0: float) -> tuple[list[float], list[float]]:
    cos_lat = math.cos(math.radians(lat0)) or 1.0
    xs = [(p[0] - lon0) * cos_lat * METRES_PER_DEGREE_LAT for p in ring]
    ys = [(p[1] - lat0) * METRES_PER_DEGREE_LAT for p in ring]
    return xs, ys


def area_hectares(ring: Ring) -> float:
    """Polygon area in hectares via local equirectangular projection + shoelace."""
    lat0, lon0 = _mean(ring)
    xs, ys = _project(ring, lat0, lon0)
    total = 0.0
    for i in range(len(xs) - 1):
        total += xs[i] * ys[i + 1] - xs[i + 1] * ys[i]
    return abs(total) / 2 / 10_000.0


def centroid(ring: Ring) -> tuple[float, float]:
    """Area-weighted centroid as (lat, lon); vertex mean for a degenerate ring."""
    lat0, lon0 = _mean(ring)
    xs, ys = _project(ring, lat0, lon0)
    a = cx = cy = 0.0
    for i in range(len(xs) - 1):
        cross = xs[i] * ys[i + 1] - xs[i + 1] * ys[i]
        a += cross
        cx += (xs[i] + xs[i + 1]) * cross
        cy += (ys[i] + ys[i + 1]) * cross
    if a == 0:
        return lat0, lon0
    a /= 2
    cx /= 6 * a
    cy /= 6 * a
    cos_lat = math.cos(math.radians(lat0)) or 1.0
    return lat0 + cy / METRES_PER_DEGREE_LAT, lon0 + cx / (METRES_PER_DEGREE_LAT * cos_lat)


def boundary_ring(geometry: Geometry | None) -> Ring | None:
    """Validate a GeoJSON Polygon and return its outer ring, or None if unusable.

    Mirrors geo.ParsePolygon: one ring, closed, at least three vertices plus a closing
    point, finite coordinates inside lon/lat ranges. Holes are rejected for now.
    """
    if not isinstance(geometry, dict) or geometry.get("type") != "Polygon":
        return None
    coords = geometry.get("coordinates")
    if not isinstance(coords, list) or len(coords) != 1:
        return None
    ring = coords[0]
    if not isinstance(ring, list) or len(ring) < 4 or ring[0] != ring[-1]:
        return None
    for point in ring:
        if not isinstance(point, (list, tuple)) or len(point) < 2:
            return None
        x, y = point[0], point[1]
        if not isinstance(x, (int, float)) or not isinstance(y, (int, float)):
            return None
        if math.isnan(x) or math.isnan(y) or math.isinf(x) or math.isinf(y):
            return None
        if x < -180 or x > 180 or y < -90 or y > 90:
            return None
    return ring


def _plot_area(value: float | None) -> float:
    try:
        area = float(value)
    except (TypeError, ValueError):
        return 0.0
    if math.isnan(area) or math.isinf(area) or area < 0:
        return 0.0
    return area


def plot_cell(lat: float, lon: float, hectares: float | None,
              boundary: Geometry | None = None) -> tuple[Geometry, float]:
    """Analysis cell preferring a surveyed boundary over the buffered square.

    Mirrors satellite/aoi.go plotCell: a valid boundary ring is the cell verbatim and
    its derived area replaces the stated hectares; otherwise the square fallback stands.
    """
    ring = boundary_ring(boundary)
    if ring is not None:
        return {"type": "Polygon", "coordinates": [ring]}, area_hectares(ring)
    return cell(lat, lon, hectares), _plot_area(hectares)
