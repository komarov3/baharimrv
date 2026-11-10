"""Windowed reads from Cloud-Optimized Geotiffs over plain HTTPS.

Every read here is a range request against a public object. Nothing is
downloaded in full: a Sentinel-2 band tile is 109 km of pixels and a project
needs a block a few hundred metres wide. GDAL handles the COG overviews, which
is precisely why the worker is Python and not Node.
"""

from __future__ import annotations

import os
import time
from dataclasses import dataclass, field

# GDAL reads its HTTP defaults once, at library start, so these have to be in
# the environment before rasterio is imported. Without them a stalled socket to
# the imagery host holds a worker for the platform's own TCP timeout, which on
# the measured link can exceed the whole job ceiling.
#
# 60s is a measured number, not a guess. One windowed read of a 20x20 pixel cell
# still fetches whole 1024x1024 JPEG-compressed blocks, and a block of B04.tif
# for this granule arrived in 114s from a client far from us-west-2 and in 10s
# from a warm one. At 40s that first transfer is not slow, it is killed, and the
# job loses a scene that exists. Deployed next to the bucket the same read is
# seconds; the ceiling is there to bound a dead socket, not to police a working
# one, so it stays well below JOB_TIMEOUT_S and above the slow tail.
os.environ.setdefault("GDAL_HTTP_CONNECTTIMEOUT", "5")
os.environ.setdefault("GDAL_HTTP_TIMEOUT", "60")

# GDAL scans the "directory" of an object it is about to open, which over
# /vsicurl/ is a `GET /` against the bucket root. This worker reads named COGs
# and never lists anything, so the scan is a round trip to an anonymous S3 root
# on every single open. Measured against a local server with the scan enabled,
# one open took 16.2s of pure probe waits; with it off, 0.06s. The same option
# also drops the PAM (.aux.xml) sidecar hunt, worth another 4s of them.
os.environ.setdefault("GDAL_DISABLE_READDIR_ON_OPEN", "EMPTY_DIR")
os.environ.setdefault("GDAL_PAM_ENABLED", "NO")

import numpy as np
import rasterio
from rasterio.enums import Resampling
from rasterio.warp import reproject, transform_bounds
from rasterio.windows import Window, from_bounds
from rasterio.transform import from_bounds as grid_transform

ALLOWED_HOSTS: tuple[str, ...] = ("sentinel-cogs.s3.us-west-2.amazonaws.com",)

# A full Sentinel-2 tile is 10980x10980 and is never read whole. The ceiling
# that matters is on the window, not the dataset.
MAX_WINDOW_PIXELS = 4_000_000

# The per-job ceilings from SATELLITE_NDVI_SPEC.md 8 rule 3. 8M pixels keeps a
# job inside a 512 MB container through the float64 math in ndvi.py; the whole
# two-scene-plus-tiles spike measured 1.57M of them.
MAX_PIXELS_PER_JOB = 8_000_000
JOB_TIMEOUT_S = 120.0

# Attempts per logical read, and the pause between them. See _retry.
READ_ATTEMPTS = 3
READ_RETRY_BACKOFF_S = 0.5


class BandUnavailable(RuntimeError):
    """The COG could not be read. The caller fails over instead of guessing."""


class NoOverlap(BandUnavailable):
    """The AOI is not inside this granule.

    Clamping instead of refusing would measure a strip at the tile edge and
    report it as the plot, which is the worst kind of wrong for an MRV number.
    """


class WindowTooLarge(RuntimeError):
    """The requested window would not fit the job's memory ceiling."""


class BudgetExceeded(RuntimeError):
    """The job hit its pixel or wall-clock ceiling.

    A caller that already has pixels reports them as `partial` with the coverage
    actually achieved. It does not stretch the window and call the result
    complete. See SATELLITE_NDVI_SPEC.md 8 rule 3.
    """


@dataclass
class ReadBudget:
    """Guard rails for one job. Exceeding either ceiling is a partial result.

    Pixels and wall clock are the two the worker can count itself. GDAL does
    not return a range-request total for a `/vsicurl/` read, so the spec treats
    that figure as an operator metric instead of enforcing a number this process
    cannot observe.

    The clock is checked at each read boundary, so a job can overshoot it by the
    read already in flight. That overshoot is what GDAL_HTTP_TIMEOUT above
    bounds: the worst cell read measured from a client far from `us-west-2` was
    114s for one block, which is a link property, not a code path. Deployed
    beside the bucket these reads are seconds.
    """

    max_pixels: int = MAX_PIXELS_PER_JOB
    max_seconds: float = JOB_TIMEOUT_S
    pixels_read: int = 0
    started_at: float = field(default_factory=time.monotonic)

    def charge(self, pixels: int) -> None:
        elapsed = time.monotonic() - self.started_at
        if elapsed > self.max_seconds:
            raise BudgetExceeded(
                f"job reached {elapsed:.1f}s against a {self.max_seconds}s ceiling "
                f"after {self.pixels_read} pixels")
        if self.pixels_read + pixels > self.max_pixels:
            raise BudgetExceeded(
                f"job would read {self.pixels_read + pixels} pixels, ceiling {self.max_pixels}")
        self.pixels_read += pixels


@dataclass
class BandRead:
    values: np.ndarray
    window: Window
    transform: object
    crs: object


def _vsicurl(url: str) -> str:
    host = url.split("/", 3)[2]
    if host not in ALLOWED_HOSTS:
        raise BandUnavailable(f"band host {host} is not in the allow list")
    return "/vsicurl/" + url


RETRY_PARAM = "s2w-retry"


def _retry_url(url: str, attempt: int) -> str:
    """The same object under a URL GDAL has not already cached.

    `/vsicurl/` keeps the ranges it has fetched for the life of the process, and
    a truncated block is cached with the good ones. Measured against a local
    server: three attempts at one band URL produced two GETs and the tile failed
    every time, so the retry below would re-ask for bytes it never re-requested.
    One inert query parameter per attempt makes the second and third attempts a
    different cache key and a real fetch, at the cost of a longer URL.

    This works because these are anonymous public objects and nothing about the
    response depends on the query. A signed URL would break here, so if a source
    ever needs one this becomes a cache flush instead.
    """
    if attempt == 0:
        return url
    return f"{url}{'&' if '?' in url else '?'}{RETRY_PARAM}={attempt + 1}"


def open_band(url: str, attempt: int = 0):
    try:
        dataset = rasterio.open(_vsicurl(_retry_url(url, attempt)))
    except rasterio.errors.RasterioIOError as exc:
        raise BandUnavailable(f"{url}: {exc}") from exc
    # A Sentinel-2 granule is about 120 megapixels per band. Anything far beyond
    # that is not the product this worker was written for.
    if dataset.width * dataset.height > 200_000_000:
        dataset.close()
        raise BandUnavailable(f"{url}: {dataset.width}x{dataset.height} is not a Sentinel-2 granule")
    return dataset


def geo_window(dataset, geometry: dict, pad_pixels: int = 2) -> Window:
    """Window covering a GeoJSON geometry, clamped to the dataset extent."""
    vertices = [point for ring in (geometry["coordinates"] if geometry["type"] == "Polygon"
                                   else geometry["coordinates"][0]) for point in ring]
    lons = [point[0] for point in vertices]
    lats = [point[1] for point in vertices]
    west, south, east, north = transform_bounds(
        "EPSG:4326", dataset.crs, min(lons), min(lats), max(lons), max(lats)
    )
    window = from_bounds(west, south, east, north, transform=dataset.transform)
    if window.col_off < 0 or window.row_off < 0:
        raise NoOverlap("the AOI runs off the west or north edge of the granule")
    if window.col_off + window.width > dataset.width or window.row_off + window.height > dataset.height:
        raise NoOverlap("the AOI runs off the east or south edge of the granule")
    col_off = max(int(np.floor(window.col_off)) - pad_pixels, 0)
    row_off = max(int(np.floor(window.row_off)) - pad_pixels, 0)
    width = min(int(np.ceil(window.width)) + 2 * pad_pixels, dataset.width - col_off)
    height = min(int(np.ceil(window.height)) + 2 * pad_pixels, dataset.height - row_off)
    return Window(col_off, row_off, max(width, 1), max(height, 1))


def _retry(operation, url: str):
    """Run one read, re-opening the dataset on a fresh URL between attempts.

    A truncated tile is the failure the spike hit against a granule that had
    read cleanly a minute earlier: `TIFFReadEncodedTile() failed, got 1152230
    bytes, expected 1563178`. The whole dataset is re-opened rather than the
    decode retried in place, because the thing that went bad is the cached curl
    session, not the raster, and because GDAL would otherwise hand the retry the
    same bad bytes from cache. See `_retry_url`.

    The pause separates a reset connection from a slow one, and the second case
    is the one that actually bit: with GDAL_HTTP_TIMEOUT below the time a block
    needs, every attempt failed identically and backoff changed nothing. That is
    a configuration error, so it is fixed in the constants above rather than
    hidden here. Three attempts, 0.5s then 1.0s, so a real hiccup costs about a
    second and a half of a job that has minutes to give; anything still failing
    after that is an outage the caller reports as `unavailable`.
    """
    last: Exception | None = None
    for attempt in range(READ_ATTEMPTS):
        try:
            return operation(attempt)
        except rasterio.errors.RasterioIOError as exc:
            last = exc
            if attempt + 1 < READ_ATTEMPTS:
                time.sleep(READ_RETRY_BACKOFF_S * (attempt + 1))
    raise BandUnavailable(f"{url}: {last} (after {READ_ATTEMPTS} attempts)") from last


def read_for_geometry(url: str, geometry: dict, pad_pixels: int = 2,
                      budget: ReadBudget | None = None) -> BandRead:
    """Read one band over the window that covers a geometry.

    The window transform travels with the values because that is what lets a
    20 m class layer be aligned onto this 10 m grid without guessing.
    """

    def once(attempt: int) -> BandRead:
        dataset = open_band(url, attempt)
        try:
            try:
                return _read(dataset, geo_window(dataset, geometry, pad_pixels), budget)
            except NoOverlap as exc:
                raise NoOverlap(f"{url}: {exc}") from exc
        finally:
            dataset.close()

    return _retry(once, url)


def read_at(url: str, window: Window, out_shape: tuple[int, int] | None = None,
            budget: ReadBudget | None = None) -> BandRead:
    """Read an explicit window, used when two bands must share one exact grid.

    out_shape is how a decimated band and a full resolution band are made to
    cover the same ground: the caller names the grid once and every band lands
    on it, so no arithmetic ever mixes two shapes.
    """

    def once(attempt: int) -> BandRead:
        dataset = open_band(url, attempt)
        try:
            return _read(dataset, window, budget, out_shape)
        finally:
            dataset.close()

    return _retry(once, url)


def _read(dataset, window: Window, budget: ReadBudget | None,
          out_shape: tuple[int, int] | None = None,
          max_window_pixels: int | None = None) -> BandRead:
    # Resolved at call time, not as a default argument: a default binds when the
    # module is imported, which would make the ceiling impossible to set from the
    # environment or to squeeze in a test.
    if max_window_pixels is None:
        max_window_pixels = MAX_WINDOW_PIXELS
    rows, cols = out_shape if out_shape else (int(window.height), int(window.width))
    if rows * cols > max_window_pixels:
        raise WindowTooLarge(
            f"read of {rows}x{cols} exceeds the {max_window_pixels} pixel ceiling")
    # Charged before the read: the ceiling exists to keep a job inside the
    # container, and a byte you have already fetched is a byte you have already
    # allocated.
    if budget is not None:
        budget.charge(rows * cols)
    if out_shape:
        data = dataset.read(1, window=window, out_shape=(rows, cols))
        left, bottom, right, top = rasterio.windows.bounds(window, dataset.transform)
        transform = grid_transform(left, bottom, right, top, cols, rows)
    else:
        data = dataset.read(1, window=window)
        transform = rasterio.windows.transform(window, dataset.transform)
    return BandRead(values=data, window=window, transform=transform, crs=dataset.crs)


def read_tile(url: str, z: int, x: int, y: int, tilesize: int = 256, oversample: int = 2,
              budget: ReadBudget | None = None) -> BandRead | None:
    """Read one Web Mercator tile's worth of a band at tile resolution.

    The window is clamped to the dataset and read with an out_shape, which lets
    GDAL serve it from a COG overview instead of the 10 m full resolution grid.
    Reading a z14 tile natively is about 2400x2400 pixels per band and buys
    nothing an overlay needs, so this is the only tile path the worker has.
    """

    def once(attempt: int) -> BandRead | None:
        dataset = open_band(url, attempt)
        try:
            target = tile_extent(dataset, z, x, y)
            if target is None:
                return None
            col_off = max(int(np.floor(target.col_off)), 0)
            row_off = max(int(np.floor(target.row_off)), 0)
            col_stop = min(int(np.ceil(target.col_off + target.width)), dataset.width)
            row_stop = min(int(np.ceil(target.row_off + target.height)), dataset.height)
            if col_stop <= col_off or row_stop <= row_off:
                return None
            clipped = Window(col_off, row_off, col_stop - col_off, row_stop - row_off)
            return _read(dataset, clipped, budget, out_shape=(tilesize * oversample,) * 2)
        finally:
            dataset.close()

    return _retry(once, url)


def rescale_mask(scl: np.ndarray, src_transform, src_crs,
                 target_shape: tuple[int, int], dst_transform, dst_crs) -> np.ndarray:
    """Nearest-neighbour a 20 m class layer onto the 10 m grid.

    Interpolating a class raster manufactures cloud-free pixels, so this is the
    only resampling the mask is allowed.
    """
    destination = np.zeros(target_shape, dtype=np.uint8)
    reproject(
        source=scl,
        destination=destination,
        src_transform=src_transform,
        src_crs=src_crs,
        dst_transform=dst_transform,
        dst_crs=dst_crs,
        resampling=Resampling.nearest,
        src_nodata=0,
        dst_nodata=0,
    )
    return destination


def tile_extent(dataset, z: int, x: int, y: int) -> Window | None:
    """Window of a Web Mercator tile inside a UTM gridded dataset, if it overlaps."""
    n = 2 ** z
    lon_w = x / n * 360.0 - 180.0
    lon_e = (x + 1) / n * 360.0 - 180.0
    lat_n = np.degrees(np.arctan(np.sinh(np.pi * (1 - 2 * y / n))))
    lat_s = np.degrees(np.arctan(np.sinh(np.pi * (1 - 2 * (y + 1) / n))))
    west, south, east, north = transform_bounds("EPSG:4326", dataset.crs, lon_w, lat_s, lon_e, lat_n)
    window = from_bounds(west, south, east, north, transform=dataset.transform)
    if window.col_off + window.width < 0 or window.row_off + window.height < 0:
        return None
    if window.col_off > dataset.width or window.row_off > dataset.height:
        return None
    return window
