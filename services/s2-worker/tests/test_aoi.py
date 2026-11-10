"""Buffered-cell geometry. The analysis area, not the plot boundary."""

from __future__ import annotations

import math

import pytest

from s2_worker import aoi


def side_metres(geometry: dict) -> float:
    ring = geometry["coordinates"][0]
    west, south = ring[0][0], ring[0][1]
    east, north = ring[2][0], ring[2][1]
    return (east - west) * aoi.METRES_PER_DEGREE_LAT * math.cos(math.radians(south))


def test_cell_side_comes_from_area_not_from_a_default():
    # 0.5 ha is 5000 m2, so 70.7 m a side. The spec's verification list names
    # this figure because a 0.5 ha plot at 30 m would be measured twice over.
    assert aoi.cell(-3.6, 39.8, 0.5)["type"] == "Polygon"
    assert side_metres(aoi.cell(-3.6, 39.8, 0.5)) == pytest.approx(70.7, abs=0.5)
    assert side_metres(aoi.cell(-3.6, 39.8, 2.5)) == pytest.approx(158.1, abs=0.5)


def test_tiny_plots_still_reach_the_30_metre_floor():
    assert side_metres(aoi.cell(-3.6, 39.8, 0.01)) == pytest.approx(30.0, abs=0.5)
    assert side_metres(aoi.cell(-3.6, 39.8, None)) == pytest.approx(30.0, abs=0.5)


def test_longitude_scales_with_latitude_so_the_cell_stays_square():
    tropic = aoi.cell(-3.6, 39.8, 1.0)
    high = aoi.cell(-55.0, 39.8, 1.0)
    assert side_metres(tropic) == pytest.approx(side_metres(high), rel=1e-3)
    degrees_tropic = tropic["coordinates"][0][2][0] - tropic["coordinates"][0][0][0]
    degrees_high = high["coordinates"][0][2][0] - high["coordinates"][0][0][0]
    assert degrees_tropic < degrees_high  # a degree of longitude shrinks toward the poles


def test_missing_area_floors_instead_of_collapsing():
    assert side_metres(aoi.cell(-3.6, 39.8, 0.0)) == pytest.approx(30.0, abs=0.5)


def test_ring_closes():
    ring = aoi.cell(-3.6, 39.8, 1.0)["coordinates"][0]
    assert ring[0] == ring[-1]
    assert len(ring) == 5


def test_search_geometry_nests_one_level_per_polygon_not_two():
    """A cell's `coordinates` is already a polygon, so wrapping it once is correct.

    Wrapping it twice is valid-looking JSON that both catalogs reject, and CDSE
    answers with a message about a Point geometry that points at nothing.
    """
    cells = [aoi.cell(-3.6, 39.8, 1.0), aoi.cell(-3.7, 39.9, 2.0)]
    geometry = aoi.search_geometry(cells)
    assert geometry["type"] == "MultiPolygon"
    assert len(geometry["coordinates"]) == 2
    # Depth is the whole bug: coordinates[i] must be a polygon (a list of rings),
    # so each entry has to be exactly the cell's own coordinates value.
    assert [polygon for polygon in geometry["coordinates"]] == [c["coordinates"] for c in cells]
    assert geometry["coordinates"][0][0] == cells[0]["coordinates"][0]


def test_union_bbox_frames_cells_without_becoming_the_analysis_area():
    cells = [aoi.cell(-3.6, 39.8, 1.0), aoi.cell(-3.7, 39.9, 1.0)]
    box = aoi.union_bbox(cells)
    assert box[0] < box[2] and box[1] < box[3]
    assert aoi.intersects(cells[0], box) and aoi.intersects(cells[1], box)
    far = aoi.cell(-4.5, 39.2, 1.0)
    assert not aoi.intersects(far, box)


def test_bbox_handles_multipolygons():
    geometry = aoi.search_geometry([aoi.cell(-3.6, 39.8, 1.0), aoi.cell(-3.7, 39.9, 1.0)])
    min_lon, min_lat, max_lon, max_lat = aoi.bbox(geometry)
    assert min_lon < max_lon and min_lat < max_lat


def test_rejects_geometry_types_the_read_path_cannot_window():
    with pytest.raises(ValueError):
        aoi.bbox({"type": "Point", "coordinates": [39.8, -3.6]})
    with pytest.raises(ValueError):
        aoi.rings({"type": "GeometryCollection", "geometries": []})
