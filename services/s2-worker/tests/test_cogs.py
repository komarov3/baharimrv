"""The read path against a COG served from localhost over plain HTTPS-free HTTP.

`cogs` is the module where a wrong number becomes a wrong MRV figure, and the
part that matters is how it talks to an object store: windowed range requests,
decimated reads through the overviews, a mask resampled nearest-only, and a
retry that survives a truncated response. None of that can be checked with
recorded arrays, because the arrays are the thing being read.

So this file builds small synthetic granules, serves them from a handler that
knows Range requests and counts the bytes it sends, and reads them through the
production code. The handler can also shorten a response on purpose, which is
how the retry got its test: the failure mode is real and it is not reproducible
on demand against the actual bucket.
"""

from __future__ import annotations

import http.server
import os
import socketserver
import threading
import time
import urllib.parse

import numpy as np
import pytest
import rasterio
from rasterio.transform import from_bounds
from rasterio.warp import transform as warp_points

from s2_worker import aoi, cogs, ndvi

# 2048 px a side, so the granule is 20 km and one band is a couple of megabytes.
# This is not decoration: GDAL fetches an object smaller than its read chunk
# whole and caches it by URL, so a kilobyte-scale fixture cannot demonstrate
# that a windowed read is a range request. It can only demonstrate a download.
SIZE = 2048
ORIGIN_X, ORIGIN_Y = 500_000.0, 9_600_000.0  # UTM 37S, where the spike's scene lives
RES = 10.0
CRS = "EPSG:32737"


def _arrays():
    """Four class bands a quarter of the granule each, and two ramps that do not
    compress into nothing. The dither is deterministic, so a test can recompute
    any pixel; without it DEFLATE shrinks a ramp to a few hundred bytes and the
    range-request claims become unfalsifiable."""
    rows, cols = np.mgrid[0:SIZE, 0:SIZE]
    dither = (((rows * 2654435761 + cols * 40503) % 129) - 64).astype(np.int32)
    red = (1000 + cols + dither).astype(np.uint16)
    nir = (5000 + (SIZE - rows) - dither).astype(np.uint16)
    quarter = SIZE // 4
    scl = np.full((SIZE, SIZE), 4, np.uint8)
    scl[quarter:2 * quarter, :] = 5
    scl[2 * quarter:3 * quarter, :] = 6  # water, and the reason it is masked
    scl[3 * quarter:, :] = 9  # high probability cloud
    scl[0:8, 0:8] = 0  # no data at the granule margin
    return red, nir, scl


def write_cog(path, array):
    # driver="COG" is what the imagery host serves, and it lays out the tiles
    # and overviews itself; the block-size creation options are not its options.
    profile = {
        "driver": "COG", "dtype": str(array.dtype), "count": 1, "width": array.shape[1],
        "height": array.shape[0], "crs": CRS,
        "transform": from_bounds(ORIGIN_X, ORIGIN_Y - SIZE * RES,
                                 ORIGIN_X + SIZE * RES, ORIGIN_Y, array.shape[1], array.shape[0]),
        "nodata": 0 if array.dtype == np.uint8 else None,
        "compress": "DEFLATE",
    }
    with rasterio.open(path, "w", **profile) as dataset:
        dataset.write(array, 1)
    return profile["transform"]


class _Handler(http.server.BaseHTTPRequestHandler):
    """Range requests, byte accounting, and an injectable short body.

    HTTP/1.1 with keep-alive is not a nicety: GDAL reads a COG as dozens of range
    requests against one object, and on HTTP/1.0 each one opens a new connection.
    With the accept queue full those connects are refused outright, so a read that
    should take a second spends its retry budget on a socket that never opened.
    """

    protocol_version = "HTTP/1.1"
    root = "."
    served: dict[str, int] = {}
    requests: dict[str, int] = {}
    truncate: dict[str, int] = {}  # path substring -> how many GETs to shorten
    truncate_forever: set[str] = set()  # path substrings that never recover
    lock = threading.Lock()

    def log_message(self, *args):
        pass

    def _body(self, payload: bytes) -> None:
        # A client that gives up mid-transfer is normal here; GDAL abandons
        # connections when a read fails, and the traceback is noise.
        try:
            self.wfile.write(payload)
            self.wfile.flush()
        except (BrokenPipeError, ConnectionResetError):
            self.close_connection = True

    def _target(self) -> str:
        return urllib.parse.unquote(self.path.lstrip("/").split("?")[0])

    def _count(self, key: str, amount: int) -> None:
        with self.lock:
            self.served[key] = self.served.get(key, 0) + amount
            self.requests[key] = self.requests.get(key, 0) + 1

    def _not_found(self):
        """A 404 that keeps the connection open, the way S3 answers one.

        `send_error` closes the socket, and GDAL probes sidecars that do not
        exist (`.aux.xml`, `.msk`) on the same pooled handle it just used for
        the band. Tearing that connection down makes the next request wait for
        a new one; measured here, one open went from milliseconds to 10s per
        probe. On the real host the probe comes back 404 on a live connection
        and the open is instant, so this is the behaviour worth copying.
        """
        self.send_response(404)
        self.send_header("Content-Length", "0")
        self.send_header("Accept-Ranges", "bytes")
        self.end_headers()

    def do_HEAD(self):
        path = os.path.join(self.root, self._target())
        if not os.path.isfile(path):
            self._not_found()
            return
        self.send_response(200)
        self.send_header("Content-Length", str(os.path.getsize(path)))
        self.send_header("Accept-Ranges", "bytes")
        self.end_headers()

    def do_GET(self):
        target = self._target()
        path = os.path.join(self.root, target)
        if not os.path.isfile(path):
            self._not_found()
            self._count(target, 0)
            return
        size = os.path.getsize(path)
        start, end = 0, size - 1
        header = self.headers.get("Range")
        partial = False
        if header and header.startswith("bytes="):
            first = header[len("bytes="):].split(",")[0].strip()
            left, _, right = first.partition("-")
            partial = True
            if left == "":
                start = max(size - int(right), 0)
            else:
                start = int(left)
                end = min(int(right), size - 1) if right else size - 1
        length = end - start + 1
        with open(path, "rb") as handle:
            handle.seek(start)
            body = handle.read(length)
        self.send_response(206 if partial else 200)
        self.send_header("Content-Length", str(length))
        if partial:
            self.send_header("Content-Range", f"bytes {start}-{end}/{size}")
        self.send_header("Accept-Ranges", "bytes")
        self.end_headers()
        # A short body with a full Content-Length, on a connection that then
        # goes quiet, is exactly what the imagery host does when a transfer
        # dies. The close matters: leaving it open would make the next request
        # on it read the missing bytes as a header, which is a different bug.
        cut = 0
        with self.lock:
            if any(fragment in target for fragment in self.truncate_forever):
                cut = max(len(body) // 2, 1)
            else:
                for fragment, remaining in list(self.truncate.items()):
                    if remaining > 0 and fragment in target:
                        self.truncate[fragment] = remaining - 1
                        cut = max(len(body) // 2, 1)
                        break
        if cut:
            self.close_connection = True
        self._body(body[:len(body) - cut] if cut else body)
        self._count(target, len(body) - cut)


class _Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True
    # GDAL keeps several range reads in flight against one object; the default
    # backlog of 5 is what turns a fast local server into refused connections.
    request_queue_size = 128

    def handle_error(self, request, client_address):
        # The handler already tolerates a client that hangs up mid-transfer, and
        # a traceback per abandoned socket drowns the test output.
        pass


@pytest.fixture(scope="module")
def granule(tmp_path_factory):
    """Three COGs on disk, plus the geometry that sits inside them."""
    root = tmp_path_factory.mktemp("cogs")
    red, nir, scl = _arrays()
    transforms = {}
    for name, array in (("red.tif", red), ("nir.tif", nir), ("scl.tif", scl)):
        transforms[name] = write_cog(str(root / name), array)
    return {"root": root, "arrays": {"red": red, "nir": nir, "scl": scl},
            "transform": transforms["red.tif"], "files": sorted(os.listdir(root))}


@pytest.fixture
def served(granule, monkeypatch):
    """The granule over HTTP, with the byte counters reset for one test."""
    _Handler.root = str(granule["root"])
    _Handler.served, _Handler.requests, _Handler.truncate = {}, {}, {}
    _Handler.truncate_forever = set()
    server = _Server(("127.0.0.1", 0), _Handler)
    port = server.server_address[1]
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    host = f"127.0.0.1:{port}"
    monkeypatch.setattr(cogs, "ALLOWED_HOSTS", (host,))
    urls = {role: f"http://{host}/{role}.tif" for role in ("red", "nir", "scl")}
    try:
        yield {"urls": urls, "served": _Handler.served, "requests": _Handler.requests,
               "truncate": _Handler.truncate, "truncate_forever": _Handler.truncate_forever,
               "granule": granule, "port": port}
    finally:
        server.shutdown()
        server.server_close()


def cell_at(dataset, row: int, col: int, side_metres: float = 100.0) -> dict:
    """A GeoJSON cell centred on one pixel of the synthetic granule."""
    x = dataset.transform.c + (col + 0.5) * dataset.transform.a
    y = dataset.transform.f + (row + 0.5) * dataset.transform.e
    lon, lat = warp_points(CRS, "EPSG:4326", [x], [y])
    return aoi.cell(lat[0], lon[0], side_metres ** 2 / 10_000.0)


@pytest.fixture(scope="module")
def geometry(granule):
    with rasterio.open(str(granule["root"] / "red.tif")) as dataset:
        return cell_at(dataset, 200, 200)  # inside the class-4 quarter, clear of the margin


def test_a_windowed_read_fetches_part_of_the_file_not_all_of_it(served, geometry):
    """The claim in the module docstring, measured: range requests only."""
    read = cogs.read_for_geometry(served["urls"]["red"], geometry)
    on_disk = os.path.getsize(os.path.join(served["granule"]["root"], "red.tif"))
    fetched = served["served"]["red.tif"]
    assert read.values.shape == (14, 14)  # a 100 m cell is 10 px, plus the 2 px pad each side
    assert fetched < on_disk, f"{fetched} of {on_disk} bytes means the whole object came across"


def test_the_read_lands_on_the_pixels_the_window_claims(served, geometry):
    """Values, not just shapes: the window transform has to travel with them."""
    read = cogs.read_for_geometry(served["urls"]["red"], geometry)
    window = read.window
    block = served["granule"]["arrays"]["red"][int(window.row_off):int(window.row_off + window.height),
                                               int(window.col_off):int(window.col_off + window.width)]
    assert np.array_equal(read.values, block)
    assert read.crs is not None


def test_a_geometry_off_the_granule_is_refused_not_clamped(served):
    """Clamping would measure the edge strip and call it the plot."""
    off_north = {"type": "Polygon", "coordinates": [[[39.0, 5.0], [39.1, 5.0],
                                                     [39.1, 5.1], [39.0, 5.1], [39.0, 5.0]]]}
    with pytest.raises(cogs.NoOverlap):
        cogs.read_for_geometry(served["urls"]["red"], off_north)


def test_a_window_over_the_ceiling_is_refused_before_the_bytes_move(served, geometry, monkeypatch):
    """The whole granule is 262k pixels; a 100 pixel ceiling is the test's stand-in."""
    monkeypatch.setattr(cogs, "MAX_WINDOW_PIXELS", 100)
    with pytest.raises(cogs.WindowTooLarge, match="ceiling"):
        cogs.read_for_geometry(served["urls"]["red"], geometry)
    assert served["served"]["red.tif"] < os.path.getsize(
        os.path.join(served["granule"]["root"], "red.tif"))


def test_the_pixel_ceiling_is_charged_before_the_bytes_move(served, geometry):
    budget = cogs.ReadBudget(max_pixels=100)
    with pytest.raises(cogs.BudgetExceeded):
        cogs.read_for_geometry(served["urls"]["red"], geometry, budget=budget)
    assert budget.pixels_read == 0, "a ceiling that is charged after the read is not a ceiling"


def test_the_wall_clock_ceiling_stops_a_later_read_in_the_same_job(served, geometry):
    budget = cogs.ReadBudget(max_pixels=1_000_000, max_seconds=5.0)
    cogs.read_for_geometry(served["urls"]["red"], geometry, budget=budget)
    assert budget.pixels_read > 0
    budget.started_at = time.monotonic() - 6.0
    with pytest.raises(cogs.BudgetExceeded, match="ceiling"):
        cogs.read_at(served["urls"]["nir"], cogs.Window(0, 0, 20, 20), budget=budget)


def test_a_truncated_response_is_retried_and_the_read_still_happens(served, geometry):
    """The failure the spike actually hit, injected, and the recovery."""
    served["truncate"]["red.tif"] = 2  # two short bodies, then honest ones
    read = cogs.read_for_geometry(served["urls"]["red"], geometry)
    assert read.values.shape == (14, 14)
    assert served["requests"]["red.tif"] > 2, "the retries re-requested the blocks"


def test_a_persistent_failure_becomes_band_unavailable(served, geometry):
    served["truncate_forever"].add("red.tif")
    with pytest.raises(cogs.BandUnavailable, match=f"{cogs.READ_ATTEMPTS} attempts"):
        cogs.read_for_geometry(served["urls"]["red"], geometry)
    assert served["requests"]["red.tif"] > cogs.READ_ATTEMPTS, \
        "a logical read is several range requests, and every attempt made them"


def test_read_at_reuses_another_band_s_window(served, geometry):
    """The point of read_at: a second band lands on the first one's exact grid."""
    red = cogs.read_for_geometry(served["urls"]["red"], geometry)
    nir = cogs.read_at(served["urls"]["nir"], red.window)
    window = red.window
    rows, cols = (int(window.row_off), int(window.col_off)), (
        int(window.row_off + window.height), int(window.col_off + window.width))
    assert nir.window == red.window
    assert np.array_equal(nir.values, served["granule"]["arrays"]["nir"][rows[0]:cols[0], rows[1]:cols[1]])


def test_out_shape_buys_the_overview_not_the_full_grid(served, geometry):
    red = cogs.read_for_geometry(served["urls"]["red"], geometry)
    decimated = cogs.read_at(served["urls"]["nir"], red.window, out_shape=(5, 5))
    assert decimated.values.shape == (5, 5)
    assert decimated.window == red.window, "the window is the extent, the shape is the sample"


def test_a_tile_fully_outside_the_granule_reads_as_none(served):
    assert cogs.read_tile(served["urls"]["red"], 14, 1, 1) is None, \
        "a tile over the mid-Pacific is not an error, it is nothing to draw"


def test_a_tile_inside_the_granule_comes_back_at_tile_resolution(served):
    """256 px of map, fetched at 2x so the overlay lands clean, never at 10 m."""
    from s2_worker import render

    with rasterio.open(str(served["granule"]["root"] / "red.tif")) as dataset:
        west, north = dataset.transform.c, dataset.transform.f
    lon, lat = warp_points(CRS, "EPSG:4326", [west + 400.0], [north - 400.0])
    z = 14
    tile_x, tile_y = _tile_of(lon[0], lat[0], z)
    read = cogs.read_tile(served["urls"]["red"], z, tile_x, tile_y)
    assert read is not None
    assert read.values.shape == (render.TILE_SIZE * 2, render.TILE_SIZE * 2)
    assert read.window.width < SIZE and read.window.height < SIZE, \
        "a z14 tile is 2.4 km and this granule is 5.1 km, so it must fit inside"


def _tile_of(lon: float, lat: float, z: int) -> tuple[int, int]:
    from rasterio.warp import transform as warp

    from s2_worker import render

    x, y = warp("EPSG:4326", render.WEB_MERCATOR, [lon], [lat])
    span = render.EARTH_CIRCUMFERENCE_METRES / (2 ** z)
    half = render.EARTH_CIRCUMFERENCE_METRES / 2
    return int(np.floor((x[0] + half) / span)), int(np.floor((half - y[0]) / span))


def test_rescale_mask_nearest_neighbour_never_invents_a_class():
    """Bilinear on a class raster would manufacture cloud-free pixels.

    Each 20 m pixel becomes a solid 2x2 of itself, and no output value is a
    number the input did not already contain.
    """
    classes = np.array([[4, 6], [9, 5]], dtype=np.uint8)
    src = from_bounds(0.0, -40.0, 40.0, 0.0, 2, 2)
    dst = from_bounds(0.0, -40.0, 40.0, 0.0, 4, 4)
    aligned = cogs.rescale_mask(classes, src, "EPSG:3857", (4, 4), dst, "EPSG:3857")

    assert aligned.shape == (4, 4)
    assert set(int(value) for value in np.unique(aligned)) <= {4, 5, 6, 9}
    assert np.array_equal(aligned[0:2, 0:2], np.full((2, 2), 4))
    assert np.array_equal(aligned[2:4, 2:4], np.full((2, 2), 5))
    assert np.array_equal(aligned[0:2, 2:4], np.full((2, 2), 6))
    assert np.array_equal(aligned[2:4, 0:2], np.full((2, 2), 9))


def test_the_class_whitelist_keeps_the_surface_and_drops_creek_and_cloud(served):
    """The four rows the synthetic granule was built out of, indexed."""
    with rasterio.open(str(served["granule"]["root"] / "scl.tif")) as dataset:
        mask = dataset.read(1)
    share = {int(cls): float((mask == cls).mean()) for cls in np.unique(mask)}
    assert share[6] == pytest.approx(0.25) and share[9] == pytest.approx(0.25)

    quarter = SIZE // 4
    usable = np.isin(mask, sorted(ndvi.USABLE_CLASSES))
    assert float(usable.mean()) == pytest.approx(share[4] + share[5])
    assert not usable[2 * quarter + 10, 10], "the third quarter is the creek"
    assert not usable[3 * quarter + 10, 10], "the last quarter is the cloud"

    red, nir, _ = _arrays()
    index = ndvi.from_reflectance(red, nir, mask)
    assert np.isnan(index[2 * quarter + 10, 10]) and np.isnan(index[3 * quarter + 10, 10])
    assert np.isfinite(index[10, 100]), "the vegetation quarter is a surface"
    # A band, not a constant: the fixture's own dither is what keeps the file
    # from compressing into nothing, and it moves each pixel a few tens of DN.
    assert 0.4 < float(np.nanmean(index[0:quarter, 100])) < 0.8


def test_the_window_pad_is_read_but_the_geometry_itself_is_what_counts(served, geometry):
    """The 2 px pad is context for the mask reprojection, not pixels to average."""
    read = cogs.read_for_geometry(served["urls"]["red"], geometry)
    with rasterio.open(str(served["granule"]["root"] / "red.tif")) as dataset:
        exact = cogs.geo_window(dataset, geometry, pad_pixels=0)
    assert read.window.width == exact.width + 4 and read.window.height == exact.height + 4
