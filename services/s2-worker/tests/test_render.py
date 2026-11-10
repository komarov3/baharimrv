"""Tile geometry, the palette, and the PNG the phone draws."""

from __future__ import annotations

import io

import numpy as np
import pytest
from PIL import Image

from s2_worker import palette, render


def test_zoom_zero_is_the_whole_world():
    west, south, east, north = render.tile_bounds(0, 0, 0)
    assert (west, south, east, north) == pytest.approx(
        (-20037508.34, -20037508.34, 20037508.34, 20037508.34), abs=1.0)


def test_tile_y_increases_southwards_like_the_xyz_convention():
    _, south_of_first, _, north_of_first = render.tile_bounds(8, 0, 0)
    assert north_of_first == pytest.approx(20037508.34, abs=1.0)
    _, top_south, _, top_north = render.tile_bounds(14, 10006, 8356)
    _, below_south, _, below_north = render.tile_bounds(14, 10006, 8357)
    assert below_north < top_north and below_south < top_south  # y+1 lies south
    assert below_north == pytest.approx(top_south)  # and abuts it: no gap, no overlap


def test_tile_transform_maps_the_first_pixel_to_the_north_west_corner():
    transform = render.tile_transform(14, 10006, 8356)
    west, south, east, north = render.tile_bounds(14, 10006, 8356)
    assert (transform.c, transform.f) == pytest.approx((west, north))
    assert transform.a == pytest.approx((east - west) / render.TILE_SIZE)
    assert transform.e == pytest.approx(-(north - south) / render.TILE_SIZE)


def test_a_point_lands_in_exactly_one_tile_per_zoom():
    """The inverse of `tiles_covering`: find the tile, then be inside it."""
    from rasterio.warp import transform as warp

    lon, lat = 39.8360, -3.6400  # the Kilifi spike cell
    x, y = warp("EPSG:4326", render.WEB_MERCATOR, [lon], [lat])
    span = render.EARTH_CIRCUMFERENCE_METRES / 2
    for z in range(palette.ZOOM_MIN, palette.ZOOM_MAX + 1):
        size = render.EARTH_CIRCUMFERENCE_METRES / (2 ** z)
        tx = int(np.floor((x[0] + span) / size))
        ty = int(np.floor((span - y[0]) / size))
        west, south, east, north = render.tile_bounds(z, tx, ty)
        assert west <= x[0] < east and south <= y[0] < north
        assert (tx, ty) in list(render.tiles_covering((lon, lat, lon, lat), z))


def test_tiles_covering_returns_the_straddling_pair():
    """A box sitting on a tile edge must return both neighbours.

    One tile either side of an edge is a hole through the middle of the project,
    which on a map reads as missing imagery rather than as a geometry bug.
    """
    from rasterio.warp import transform as warp

    _, south, east, north = render.tile_bounds(14, 10006, 8356)
    edge_lon, mid_lat = warp(render.WEB_MERCATOR, "EPSG:4326",
                             [east], [(south + north) / 2.0])
    half_width_degrees = 0.005  # about 550 m either side, and a z14 tile is 2443 m
    tiles = list(render.tiles_covering((edge_lon[0] - half_width_degrees, mid_lat[0],
                                       edge_lon[0] + half_width_degrees, mid_lat[0] + 0.001), 14))
    assert {x for x, _ in tiles} == {10006, 10007}
    assert {y for _, y in tiles} == {8356}  # a 111 m tall box stays in one row


def test_tiles_covering_clamps_to_the_world():
    tiles = list(render.tiles_covering((-200.0, -89.0, 200.0, 89.0), 3))
    assert tiles
    assert all(0 <= x < 2 ** 3 and 0 <= y < 2 ** 3 for x, y in tiles)


def test_reproject_index_keeps_the_value_and_the_nan():
    """A band grid over the tile's own metres lands on it, NaN and all."""
    from rasterio.transform import from_bounds

    index = np.array([[0.8, np.nan], [0.2, 0.6]], dtype=np.float32)
    west, south, east, north = render.tile_bounds(14, 10006, 8356)
    transform = from_bounds(west, south, east, north, 2, 2)
    tile = render.reproject_index(index, transform, render.WEB_MERCATOR, 14, 10006, 8356)
    assert tile.shape == (render.TILE_SIZE, render.TILE_SIZE)
    assert np.isnan(tile).any() and np.isfinite(tile).any()
    assert float(np.nanmax(tile)) == pytest.approx(0.8, abs=0.02)
    assert np.nanmin(tile) >= -1.0 and np.nanmax(tile) <= 1.0  # nothing interpolated invented


def test_a_source_outside_the_tile_renders_no_pixels_not_stretched_ones():
    """Off-tile input is NoData across the whole grid, never an edge value."""
    from rasterio.transform import from_bounds

    index = np.array([[0.8, 0.7], [0.2, 0.6]], dtype=np.float32)
    elsewhere = from_bounds(0.0, 0.0, 1000.0, 1000.0, 2, 2)  # Gulf of Guinea
    tile = render.reproject_index(index, elsewhere, render.WEB_MERCATOR, 14, 10006, 8356)
    assert np.isnan(tile).all()


def test_nan_pixels_are_transparent_not_painted():
    """A masked pixel must show the basemap through it, never a colour."""
    rgba = palette.colorize(np.array([[np.nan, 0.8]]))
    assert tuple(rgba[0, 0]) == (0, 0, 0, 0)
    assert tuple(rgba[0, 1]) == (0x2A, 0x7D, 0x1E, 255)


def test_palette_stops_are_half_open_and_contiguous():
    assert palette.band_of(-1.0) == 0
    assert palette.band_of(0.0999) == 0
    assert palette.band_of(0.10) == 1      # lower bound inclusive
    assert palette.band_of(0.2999) == 1
    assert palette.band_of(0.30) == 2
    assert palette.band_of(1.0) == len(palette.STOPS) - 1
    assert palette.band_of(np.nan) == -1


def test_legend_covers_the_whole_range_without_gaps():
    stops = palette.legend()
    assert stops[0]["min"] == -1.0 and stops[-1]["max"] == 1.0
    for previous, following in zip(stops, stops[1:]):
        assert previous["max"] == following["min"]
    assert all(len(stop["color"]) == 7 and stop["color"].startswith("#") for stop in stops)
    assert all(stop["labelKey"].startswith("satellite.legend.") for stop in stops)


def test_png_round_trips_through_the_encoder():
    rgba = palette.colorize(np.linspace(-1, 1, 64, dtype=np.float32).reshape(8, 8))
    image = Image.open(io.BytesIO(render.png_bytes(rgba)))
    assert image.mode == "RGBA"
    assert np.array_equal(np.asarray(image), rgba)


def test_version_strings_are_pinned_because_provenance_depends_on_them():
    assert palette.PALETTE_VERSION == "ndvi-5band/1"
    assert palette.ZOOM_MIN == 8 and palette.ZOOM_MAX == 16
