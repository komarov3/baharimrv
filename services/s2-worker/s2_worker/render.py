"""Tile geometry and PNG encoding.

Tiles are addressed by scene, never by wall clock: the object key carries the
scene identifier so a stale cache entry cannot serve last month's pixels under
this month's label. See SATELLITE_NDVI_SPEC.md 8 rule 7.
"""

from __future__ import annotations

import io

import numpy as np
import rasterio
from rasterio.enums import Resampling
from rasterio.transform import from_bounds
from rasterio.warp import reproject

TILE_SIZE = 256
WEB_MERCATOR = "EPSG:3857"
EARTH_CIRCUMFERENCE_METRES = 40_075_016.686


def tile_bounds(z: int, x: int, y: int) -> tuple[float, float, float, float]:
    """(west, south, east, north) of a tile in Web Mercator metres."""
    span = EARTH_CIRCUMFERENCE_METRES / (2 ** z)
    half = EARTH_CIRCUMFERENCE_METRES / 2
    west = -half + x * span
    north = half - y * span
    return west, north - span, west + span, north


def tile_transform(z: int, x: int, y: int, tilesize: int = TILE_SIZE):
    west, south, east, north = tile_bounds(z, x, y)
    return from_bounds(west, south, east, north, tilesize, tilesize)


def reproject_index(index: np.ndarray, src_transform, src_crs, z: int, x: int, y: int,
                    tilesize: int = TILE_SIZE) -> np.ndarray:
    """Project a native-resolution index onto one tile's 256 px grid.

    Bilinear is allowed here and nowhere else: NDVI is a continuous value, so
    interpolating it is ordinary resampling. The same choice on a class raster
    would manufacture cloud-free pixels, which is why the mask path in cogs.py
    is nearest-neighbour only.
    """
    destination = np.full((tilesize, tilesize), np.nan, dtype=np.float32)
    source = np.where(np.isfinite(index), index, np.nan).astype(np.float32)
    reproject(
        source=source,
        destination=destination,
        src_transform=src_transform,
        src_crs=src_crs,
        dst_transform=tile_transform(z, x, y, tilesize),
        dst_crs=WEB_MERCATOR,
        resampling=Resampling.bilinear,
        src_nodata=np.nan,
        dst_nodata=np.nan,
    )
    return destination


def png_bytes(rgba: np.ndarray) -> bytes:
    from PIL import Image

    buffer = io.BytesIO()
    Image.fromarray(rgba, mode="RGBA").save(buffer, format="PNG", optimize=True)
    return buffer.getvalue()


def tiles_covering(bounds_wgs84: tuple[float, float, float, float], z: int):
    """Tile indices overlapping a (min_lon, min_lat, max_lon, max_lat) box."""
    from rasterio.warp import transform_bounds

    min_x, min_y, max_x, max_y = transform_bounds("EPSG:4326", WEB_MERCATOR, *bounds_wgs84)
    span = EARTH_CIRCUMFERENCE_METRES / (2 ** z)
    half = EARTH_CIRCUMFERENCE_METRES / 2
    n = 2 ** z
    x_min = int(np.floor((min_x + half) / span))
    x_max = int(np.floor((max_x + half) / span))
    y_min = int(np.floor((half - max_y) / span))
    y_max = int(np.floor((half - min_y) / span))
    for x in range(max(x_min, 0), min(x_max, n - 1) + 1):
        for y in range(max(y_min, 0), min(y_max, n - 1) + 1):
            yield x, y
