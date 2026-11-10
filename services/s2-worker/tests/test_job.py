"""The job layer: the golden replay, the cell arithmetic, the screening walk.

Two kinds of test live here. The first replays the recorded Kilifi reads through
`job.measure` and demands the exact figures the capture wrote down, so the
numbers the spec quotes are the numbers `/v1/stats` returns. The second uses a
synthetic grid behind a fake dataset, because union-window reads, the fallback
when a window is refused, and the ceiling behaviour need many cells and known
class rows, and no single real scene can supply them on demand.

The fake dataset implements `read()` and nothing else, and the reads still run
through `cogs._read`, so the ceilings, the pixel charging and the window
transforms under test are production code, not a stand-in for it.
"""

from __future__ import annotations

import datetime as dt
import pathlib

import numpy as np
import pytest
import rasterio
from rasterio.transform import from_bounds
from rasterio.windows import transform as window_transform
from rasterio.windows import from_bounds as pixel_window

from s2_worker import aoi, cogs, job, ndvi, palette, stac
from fakes import MemoryStore

HERE = pathlib.Path(__file__).resolve().parent
SIZE = 512
RES = 10.0
CRS = "EPSG:32737"
X0, Y_TOP = 500_000.0, 9_600_000.0
TRANSFORM = from_bounds(X0, Y_TOP - SIZE * RES, X0 + SIZE * RES, Y_TOP, SIZE, SIZE)
GRID_TRANSFORM = rasterio.Affine(RES, 0.0, X0, 0.0, -RES, Y_TOP)


class FakeDataset:
    """The four attributes `cogs._read` and `cogs.geo_window` touch."""

    crs = CRS
    transform = GRID_TRANSFORM
    width = SIZE
    height = SIZE

    def __init__(self, array):
        self.array = array

    def read(self, indexes=1, window=None, out_shape=None):
        array = self.array
        if window is not None:
            row_off, col_off = int(window.row_off), int(window.col_off)
            array = array[row_off:row_off + int(window.height),
                          col_off:col_off + int(window.width)]
        if out_shape is not None:
            rows, cols = out_shape
            ys = np.linspace(0, array.shape[0] - 1, rows).round().astype(int)
            xs = np.linspace(0, array.shape[1] - 1, cols).round().astype(int)
            array = array[np.ix_(ys, xs)]
        return array


def grid_arrays():
    rows, cols = np.mgrid[0:SIZE, 0:SIZE]
    red = (800 + cols).astype(np.uint16)
    nir = (5200 - rows).astype(np.uint16)
    scl = np.full((SIZE, SIZE), 4, np.uint8)
    scl[256:, :] = 6  # the lower half is creek: real pixels, no surface
    return red, nir, scl


RED, NIR, SCL = grid_arrays()
DATASETS = {"B04.tif": FakeDataset(RED), "B08.tif": FakeDataset(NIR), "SCL.tif": FakeDataset(SCL)}


def cell_at(row: int, col: int, side_metres: float = 200.0) -> dict:
    x = GRID_TRANSFORM.c + (col + 0.5) * GRID_TRANSFORM.a
    y = GRID_TRANSFORM.f + (row + 0.5) * GRID_TRANSFORM.e
    lon, lat = rasterio.warp.transform(CRS, "EPSG:4326", [x], [y])
    return aoi.cell(lat[0], lon[0], side_metres ** 2 / 10_000.0)


def install_reads(monkeypatch, max_window_pixels: int = 10 ** 9):
    """Point the read entry points at the grid, counting what they ask for."""
    calls: list[str] = []

    def dataset_for(url: str) -> FakeDataset:
        return DATASETS[url.rsplit("/", 1)[-1]]

    def read_for_geometry(url, geometry, pad_pixels=2, budget=None):
        calls.append(f"geometry:{url.rsplit('/', 1)[-1]}")
        dataset = dataset_for(url)
        return cogs._read(dataset, cogs.geo_window(dataset, geometry, pad_pixels), budget,
                           None, max_window_pixels)

    def read_at(url, window, out_shape=None, budget=None):
        calls.append(f"at:{url.rsplit('/', 1)[-1]}")
        return cogs._read(dataset_for(url), window, budget, out_shape, max_window_pixels)

    def read_tile(url, z, x, y, tilesize=256, oversample=2, budget=None):
        calls.append(f"tile:{url.rsplit('/', 1)[-1]}")
        return None

    monkeypatch.setattr(cogs, "read_for_geometry", read_for_geometry)
    monkeypatch.setattr(cogs, "read_at", read_at)
    monkeypatch.setattr(cogs, "read_tile", read_tile)
    return calls


def synthetic_scene(**overrides) -> stac.Scene:
    bands = {role: f"https://sentinel-cogs.s3.us-west-2.amazonaws.com/x/{name}"
             for role, name in (("red", "B04.tif"), ("nir", "B08.tif"), ("scl", "SCL.tif"))}
    fields = dict(scene_id="S2A_FAKE_37MER", acquired_at=dt.datetime(2026, 11, 5,
                                                                     tzinfo=dt.timezone.utc),
                  cloud_pct=5.0, platform="Sentinel-2A", epsg=32737, orbit="R049",
                  tile="T37MER", baseline="05.13", discovery_source="element84-stac",
                  bands=bands, footprint=None)
    fields.update(overrides)
    return stac.Scene(**fields)


# --- the golden replay -------------------------------------------------------


def test_the_replay_reproduces_the_recorded_figures_exactly(replay, golden_scene, golden_cell,
                                                            golden_stats):
    """What `/v1/stats` returns for the scene the spec quotes, pixel for pixel."""
    result = job.measure(golden_scene, [job.Cell("plot-1", golden_cell, 2.5)],
                         budget=cogs.ReadBudget(max_pixels=10_000_000, max_seconds=600))
    for key in ("mean_ndvi", "median_ndvi", "valid_pixels", "expected_pixels",
                "coverage_pct", "quality", "scene_id", "class_table", "palette_version"):
        assert result[key] == golden_stats[key], key
    assert result["mean_ndvi"] is not None


def test_the_golden_cell_is_measured_over_its_own_area_not_the_read_pad(replay, golden_scene,
                                                                        golden_cell, golden_stats):
    """A 158 m square is about 16x16 of the 10 m grid: 256 px, never the 400 read."""
    result = job.measure(golden_scene, [job.Cell("plot-1", golden_cell, 2.5)],
                         budget=cogs.ReadBudget())
    assert result["expected_pixels"] == golden_stats["expected_pixels"]
    assert result["expected_pixels"] < 400
    assert 200 <= result["expected_pixels"] <= 289


def test_the_golden_kilifi_plot_is_dense_vegetation(replay, golden_scene, golden_cell):
    result = job.measure(golden_scene, [job.Cell("plot-1", golden_cell, 2.5)])
    assert 0.5 < result["mean_ndvi"] <= 0.95, "a mangrove stand, not farmland and not a fluke"
    assert result["coverage_pct"] > 60.0


def test_the_replay_records_the_reads_the_service_actually_makes(replay, golden_scene, golden_cell):
    job.measure(golden_scene, [job.Cell("plot-1", golden_cell, 2.5)])
    assert replay.reads == ["red", "nir", "scl"], \
        "three opens for one project: red, nir at red's window, then the class layer"


def test_the_rendered_golden_tile_is_reproducible_from_the_recording(replay, golden_scene, bands):
    """Byte-identical output is what makes a stored tile auditable later."""
    tile = bands["meta"]["tile"]
    payload = job.tile_png(golden_scene, tile["z"], tile["x"], tile["y"])
    assert isinstance(payload, bytes) and payload[:8] == b"\x89PNG\r\n\x1a\n"
    again = job.tile_png(golden_scene, tile["z"], tile["x"], tile["y"])
    assert again == payload, "the same scene and tile must not paint two different images"


def test_the_rendered_golden_tile_matches_the_capture_pixel_for_pixel(replay, golden_scene, bands):
    """The capture wrote this PNG from the same recording, so the worker's own
    tile path must land on the same bytes, not merely on stable ones."""
    tile = bands["meta"]["tile"]
    golden = (HERE / "fixtures" / f"tile_{tile['z']}_{tile['x']}_{tile['y']}.png").read_bytes()
    assert job.tile_png(golden_scene, tile["z"], tile["x"], tile["y"]) == golden


# --- cell arithmetic on the synthetic grid -----------------------------------


def test_one_union_window_serves_every_cell_in_the_project(monkeypatch):
    calls = install_reads(monkeypatch)
    cells = [job.Cell("a", cell_at(100, 100)), job.Cell("b", cell_at(120, 100)),
             job.Cell("c", cell_at(140, 100))]
    notes: list[str] = []
    result = job.measure(synthetic_scene(), cells, notes=notes)
    assert len(result["per_cell"]) == 3
    assert len(calls) == 3, f"three cells should not cost {len(calls)} reads"
    assert notes == [], "nothing about this project should have forced a fallback"


def test_cells_over_the_creek_report_zero_coverage_instead_of_a_number(monkeypatch):
    install_reads(monkeypatch)
    water = job.Cell("creek", cell_at(400, 100))  # class 6, real pixels, no surface
    result = job.measure(synthetic_scene(), [water])
    assert result["per_cell"][0]["mean_ndvi"] is None
    assert result["per_cell"][0]["coverage_pct"] == 0.0
    assert result["mean_ndvi"] is None
    assert result["quality"] == "complete", "a genuinely cloud-free pass over water is still clean"


def test_a_one_cell_project_over_water_and_one_over_vegetation_weighs_the_area(monkeypatch):
    install_reads(monkeypatch)
    cells = [job.Cell("stand", cell_at(100, 100), area_hectares=9.0),
             job.Cell("clearing", cell_at(200, 100), area_hectares=1.0)]
    weighted = job.measure(synthetic_scene(), cells)
    unweighted = job.measure(synthetic_scene(), cells, weight_by_area=False)
    rows = {row["plot_id"]: row["mean_ndvi"] for row in weighted["per_cell"]}
    assert rows["stand"] != rows["clearing"]
    assert weighted["mean_ndvi"] == pytest.approx(
        (rows["stand"] * 9.0 + rows["clearing"] * 1.0) / 10.0, abs=1e-4)
    assert unweighted["mean_ndvi"] == pytest.approx(
        (rows["stand"] + rows["clearing"]) / 2.0, abs=1e-4)


def test_a_refused_union_window_falls_back_and_says_so(monkeypatch):
    calls = install_reads(monkeypatch, max_window_pixels=900)
    cells = [job.Cell("west", cell_at(100, 40)), job.Cell("east", cell_at(100, 400))]
    notes: list[str] = []
    result = job.measure(synthetic_scene(), cells, notes=notes)
    assert any("union window refused" in note for note in notes), notes
    assert any("separately" in note for note in notes), notes
    assert len(result["per_cell"]) == 2
    assert len(calls) == 7, "the refused union read itself, then three per cell"
    assert result["quality"] == "complete"


def test_a_single_cell_over_the_ceiling_is_not_a_fallback(monkeypatch):
    install_reads(monkeypatch, max_window_pixels=10)
    with pytest.raises(cogs.WindowTooLarge):
        job.measure(synthetic_scene(), [job.Cell("only", cell_at(100, 100))])


def test_an_exhausted_budget_yields_partial_and_the_pixels_it_reached(monkeypatch):
    install_reads(monkeypatch)
    budget = cogs.ReadBudget(max_pixels=100)
    notes: list[str] = []
    result = job.measure(synthetic_scene(), [job.Cell("a", cell_at(100, 100))],
                         notes=notes, budget=budget)
    assert result["quality"] == "partial"
    assert result["mean_ndvi"] is None
    assert any("no cell could be measured" in note for note in notes), notes


def test_a_cell_outside_the_granule_is_refused_not_measured_as_a_strip(monkeypatch):
    install_reads(monkeypatch)
    far = aoi.cell(4.0, 30.0, 2.5)  # nowhere near this tile
    with pytest.raises(cogs.NoOverlap):
        job.measure(synthetic_scene(), [job.Cell("ghost", far)])


# --- provenance ------------------------------------------------------------


def test_every_response_carries_the_provenance_a_reader_needs(replay, golden_scene, golden_cell):
    result = job.measure(golden_scene, [job.Cell("plot-1", golden_cell, 2.5)])
    for key in ("scene_id", "acquired_at", "cloud_pct", "platform", "epsg", "relative_orbit",
                "grid", "processor_baseline", "discovery_source", "footprint_geojson",
                "quality", "class_table", "palette_version", "notes"):
        assert key in result, key
    assert result["class_table"] == ndvi.CLASS_TABLE
    assert result["palette_version"] == palette.PALETTE_VERSION


def test_partial_is_a_state_the_worker_can_honestly_report(monkeypatch):
    """Coverage below the floor is reported with its coverage, not hidden."""
    install_reads(monkeypatch)

    def source(scenes):
        class _S:
            name, timeout = "fake", 5.0
            pixels = True

            def __init__(self, scenes):
                self._scenes = scenes

            def candidates(self, geometry, start, end, limit=50):
                return list(self._scenes)

        return _S(scenes)

    # A cell over the creek half of the grid: real pixels, no surface, so the
    # screen reads them as 0% usable and neither granule clears the floor.
    geometry = cell_at(300, 100)
    scenes = [synthetic_scene(scene_id="wet", cloud_pct=10.0),
              synthetic_scene(scene_id="dry", cloud_pct=30.0)]
    choice = job.choose_scene(geometry, dt.datetime(2026, 1, 1, tzinfo=dt.timezone.utc),
                              dt.datetime(2026, 12, 1, tzinfo=dt.timezone.utc),
                              sources=[source(scenes)], min_coverage_pct=100.0,
                              ceilings=(40.0, 70.0))
    assert choice.quality == "partial"
    assert len(choice.screened) == 2
    assert any("none of 2 screened" in note for note in choice.notes), choice.notes


def test_the_screen_walk_stops_at_the_first_candidate_that_clears(monkeypatch):
    install_reads(monkeypatch)

    class _S:
        name, timeout, pixels = "fake", 5.0, True

        def __init__(self, scenes):
            self._scenes = scenes

        def candidates(self, geometry, start, end, limit=50):
            return list(self._scenes)

    cells_inside = aoi.search_geometry([cell_at(100, 100, 400.0)])
    scenes = [synthetic_scene(scene_id="hazy", cloud_pct=55.0),
              synthetic_scene(scene_id="clear", cloud_pct=5.0),
              synthetic_scene(scene_id="cleanest", cloud_pct=1.0)]
    choice = job.choose_scene(cells_inside, dt.datetime(2026, 1, 1, tzinfo=dt.timezone.utc),
                              dt.datetime(2026, 12, 1, tzinfo=dt.timezone.utc),
                              sources=[_S(scenes)], min_coverage_pct=60.0)
    assert choice.scene.scene_id == "clear", "rank order is cloud tier first"
    assert [row.scene_id for row in choice.screened] == ["clear"], \
        "the walk stops the moment a granule clears the floor"
    assert choice.quality == "complete"


def test_no_candidates_is_unavailable_rather_than_a_nearest_guess(monkeypatch):
    install_reads(monkeypatch)

    class _Empty:
        name, timeout, pixels = "fake", 5.0, True

        def candidates(self, geometry, start, end, limit=50):
            return []

    with pytest.raises(job.Unavailable):
        job.choose_scene(aoi.cell(-3.64, 39.84, 2.5),
                         dt.datetime(2026, 1, 1, tzinfo=dt.timezone.utc),
                         dt.datetime(2026, 12, 1, tzinfo=dt.timezone.utc), sources=[_Empty()])


def test_a_candidate_that_cannot_be_read_is_skipped_with_a_note(monkeypatch):
    install_reads(monkeypatch)
    original = cogs.read_for_geometry

    def flaky(url, geometry, pad_pixels=2, budget=None):
        if "SCL" in url:
            raise cogs.BandUnavailable("simulated outage")
        return original(url, geometry, pad_pixels, budget)

    monkeypatch.setattr(cogs, "read_for_geometry", flaky)

    class _S:
        name, timeout, pixels = "fake", 5.0, True

        def candidates(self, geometry, start, end, limit=50):
            return [synthetic_scene(scene_id="only")]

    with pytest.raises(job.Unavailable, match="unreadable"):
        job.choose_scene(aoi.cell(-3.64, 39.84, 2.5),
                         dt.datetime(2026, 1, 1, tzinfo=dt.timezone.utc),
                         dt.datetime(2026, 12, 1, tzinfo=dt.timezone.utc), sources=[_S()])


# --- rendering --------------------------------------------------------------


def test_render_tiles_writes_scene_scoped_keys_and_reports_the_counts(monkeypatch):
    install_reads(monkeypatch)
    monkeypatch.setattr(job, "tile_png", lambda scene, z, x, y, budget=None: b"PNGPAYLOAD")
    store = MemoryStore()
    result = job.render_tiles(synthetic_scene(), (39.83, -3.65, 39.85, -3.63), 13, 13, store,
                              "proj-1/scene-1")
    assert result["tiles_rendered"] == len(store.objects)
    assert result["tiles_empty"] == 0 and result["tiles_omitted"] == 0
    assert result["quality"] == "complete"
    assert result["palette_version"] == palette.PALETTE_VERSION
    assert result["class_table"] == ndvi.CLASS_TABLE
    for key in store.objects:
        assert key.startswith("proj-1/scene-1/13/")
        assert key.endswith(".png")
    assert result["bytes_written"] == len(store.objects) * len("PNGPAYLOAD")


def test_a_tile_the_granule_does_not_cover_is_counted_not_invented(monkeypatch):
    install_reads(monkeypatch)  # the fake read_tile always returns None
    store = MemoryStore()
    result = job.render_tiles(synthetic_scene(), (39.83, -3.65, 39.85, -3.63), 14, 14, store,
                              "proj-1/scene-1")
    assert result["tiles_rendered"] == 0
    assert result["tiles_empty"] > 0
    assert store.objects == {}
    assert result["quality"] == "complete", "an empty tile is not a degraded pass"


def test_the_per_job_tile_cap_leaves_a_partial_and_names_what_is_missing(monkeypatch):
    install_reads(monkeypatch)
    monkeypatch.setattr(job, "tile_png", lambda scene, z, x, y, budget=None: b"x")
    result = job.render_tiles(synthetic_scene(), (39.7, -3.8, 40.0, -3.5), 12, 13, MemoryStore(),
                              "proj-1/scene-1", max_tiles=2)
    assert result["quality"] == "partial"
    assert result["tiles_omitted"] > 0
    assert any("per-job cap" in note for note in result["notes"]), result["notes"]


def test_a_zoom_outside_the_palettes_range_is_refused(monkeypatch):
    install_reads(monkeypatch)
    with pytest.raises(ValueError, match="zoom"):
        job.render_tiles(synthetic_scene(), (39.83, -3.65, 39.85, -3.63), 17, 18, MemoryStore(),
                         "proj-1/scene-1")


def test_an_unreadable_tile_is_a_note_and_a_count_not_a_whole_job_failure(monkeypatch):
    install_reads(monkeypatch)

    def boom(scene, z, x, y, budget=None):
        raise cogs.BandUnavailable("simulated outage")

    monkeypatch.setattr(job, "tile_png", boom)
    result = job.render_tiles(synthetic_scene(), (39.83, -3.65, 39.85, -3.63), 14, 14,
                              MemoryStore(), "proj-1/scene-1")
    assert result["tiles_rendered"] == 0
    assert result["tiles_empty"] > 0
    assert any("unreadable" in note for note in result["notes"])
