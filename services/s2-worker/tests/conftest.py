"""Loadings of the recordings in `fixtures/`, and the replay that replaces S3.

Nothing in this suite reaches the network. `tests/capture_fixtures.py` did that
once, with a real Kilifi granule, and what it wrote is what the tests run on.
A hand-written fixture only ever proves the code that wrote it, which for a
number that anchors an MRV record is no proof at all.

`Replay` is the other half: it serves those recorded band reads through the same
`cogs` entry points production uses, so `job.measure` and `job.tile_png` run
their real arithmetic, real windows and real mask reprojection over them. Only
the bytes are recorded; the math is live.
"""

from __future__ import annotations

import json
import pathlib

import numpy as np
import pytest
import rasterio
from rasterio.windows import Window

from s2_worker import cogs

HERE = pathlib.Path(__file__).resolve().parent
FIXTURES = HERE / "fixtures"

ROLES = ("red", "nir", "scl")


def recorded(name: str) -> pathlib.Path:
    path = FIXTURES / name
    if not path.exists():
        raise FileNotFoundError(
            f"{path} is not in the repository. Record it with "
            f".venv/bin/python tests/capture_fixtures.py (needs network) and commit it.")
    return path


@pytest.fixture(scope="session")
def stac_features():
    """Each catalog's own answer for the Kilifi AOI, verbatim."""
    return {name: json.loads(recorded(f"stac_{name}.json").read_text())["features"]
            for name in ("cdse", "element84")}


@pytest.fixture(scope="session")
def class_histogram():
    return json.loads(recorded("class_histogram.json").read_text())


@pytest.fixture(scope="session")
def bands():
    """The recorded reads plus the metadata the capture wrote beside them."""
    npz = np.load(recorded("bands.npz"), allow_pickle=True)
    return {"npz": npz, "meta": json.loads(str(npz["meta"][0]))}


def _window_key(window) -> tuple:
    """The four integers a recorded window is stored as; rasterio 1.5 has no totuple()."""
    return (int(window.col_off), int(window.row_off), int(window.width), int(window.height))


def _read(npz, prefix: str) -> cogs.BandRead:
    return cogs.BandRead(values=npz[f"{prefix}"],
                         window=Window(*[int(v) for v in npz[f"{prefix}_window"]]),
                         transform=rasterio.Affine(*[float(v) for v in npz[f"{prefix}_transform"]]),
                         crs=str(npz[f"{prefix}_crs"]))


class Replay:
    """`cogs` reads answered from the recording, with the call log a test needs.

    Keys are the band URL plus the window or tile index the caller asked for, so
    a read the capture never made raises instead of quietly returning someone
    else's pixels. That strictness is the point: a replay that fills in gaps
    would let a windowing bug look like a passing test.
    """

    def __init__(self, npz, meta):
        self.by_geometry: dict[str, tuple[str, cogs.BandRead]] = {}
        self.by_window: dict[tuple, tuple[str, cogs.BandRead]] = {}
        self.by_tile: dict[tuple, tuple[str, cogs.BandRead]] = {}
        self.reads: list[str] = []
        zxy = (meta["tile"]["z"], meta["tile"]["x"], meta["tile"]["y"])
        for role in ROLES:
            url = meta["scene"]["bands"][role]
            cell, tile = _read(npz, f"cell_{role}"), _read(npz, f"tile_{role}")
            self.by_geometry[url] = (role, cell)
            self.by_tile[(url, *zxy)] = (role, tile)
            # read_at is handed the window of the *other* band, which is the same
            # extent, so indexing by the recorded window is indexing by the
            # request.
            self.by_window[(url, _window_key(cell.window))] = (role, cell)
            self.by_window[(url, _window_key(tile.window))] = (f"tile:{role}", tile)

    def _serve(self, entry, budget) -> cogs.BandRead:
        label, read = entry
        self.reads.append(label)
        if budget is not None:
            budget.charge(int(read.values.size))
        return cogs.BandRead(values=read.values.copy(), window=read.window,
                             transform=read.transform, crs=read.crs)

    def install(self, monkeypatch) -> "Replay":
        def read_for_geometry(url, geometry, pad_pixels=2, budget=None):
            return self._serve(self.by_geometry[url], budget)

        def read_at(url, window, out_shape=None, budget=None):
            key = (url, _window_key(window))
            if key not in self.by_window:
                raise AssertionError(f"the capture never read {url} at window {key[1]}")
            return self._serve(self.by_window[key], budget)

        def read_tile(url, z, x, y, tilesize=256, oversample=2, budget=None):
            key = (url, z, x, y)
            if key not in self.by_tile:
                raise AssertionError(f"the capture never read tile {z}/{x}/{y} of {url}")
            return self._serve(self.by_tile[key], budget)

        for name, function in (("read_for_geometry", read_for_geometry),
                               ("read_at", read_at), ("read_tile", read_tile)):
            monkeypatch.setattr(cogs, name, function)
        return self


@pytest.fixture
def replay(monkeypatch, bands) -> Replay:
    return Replay(bands["npz"], bands["meta"]).install(monkeypatch)


@pytest.fixture(scope="session")
def golden_scene(bands):
    """The scene the golden figures came from, rebuilt from its own recording."""
    from s2_worker import job

    return job.scene_from_ref(bands["meta"]["scene"])


@pytest.fixture(scope="session")
def golden_stats(bands):
    return bands["meta"]["measure"]


@pytest.fixture(scope="session")
def golden_cell(bands):
    return bands["meta"]["cell_geometry"]
