#!/usr/bin/env python3
"""Record the fixtures the test suite runs on. Run this with network; CI never does.

    GDAL_HTTP_TIMEOUT=180 .venv/bin/python tests/capture_fixtures.py

The timeout override is for a laptop far from the bucket: one cell window still
fetches whole 1024x1024 blocks, and that transfer has taken 114s from Nairobi
against a few seconds from California. Run without it and the capture spends its
retries on the same killed transfer.

It writes, for one real Kilifi scene:

  stac_cdse.json, stac_element84.json   the two catalogs' own answers, verbatim
  class_histogram.json                  each granule's scene-wide class shares
                                        beside the `s2:*_percentage` fields the
                                        same STAC item publishes, which is the
                                        only evidence that the mask table means
                                        what the code says it means
  bands.npz                             the windowed band reads behind the golden
                                        figures, with the geometry, window,
                                        transform and CRS needed to replay them
  tile_14_10005_8357.png                the golden rendered tile

Everything is a recording of a real response. Nothing here is hand-written,
because a hand-written fixture only ever tests the code that wrote it.

Re-run it when a fixture has to change, then read the diff: the golden mean moves
only if the scene, the table or the math moved.
"""

from __future__ import annotations

import datetime as dt
import json
import pathlib
import sys

import numpy as np
import rasterio

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))

from s2_worker import aoi, cogs, job, ndvi, palette, render, stac  # noqa: E402

HERE = pathlib.Path(__file__).resolve().parent
FIXTURES = HERE / "fixtures"

LAT, LON, HECTARES = -3.6400, 39.8360, 2.5
END = dt.datetime(2026, 11, 10, 23, 59, 59, tzinfo=dt.timezone.utc)
WINDOW_DAYS = 45
ZOOM, TILE_X, TILE_Y = 14, 10005, 8357

# One decimated read across the granule is 262k pixels and answers the class
# question for the whole product rather than for one plot.
HISTOGRAM_GRID = 512


def dump_stac(name: str, source: stac.StacSource, geometry: dict, start, end) -> dict:
    body = {
        "collections": [stac.COLLECTION],
        "intersects": geometry,
        "datetime": f"{start.strftime('%Y-%m-%dT%H:%M:%SZ')}/{end.strftime('%Y-%m-%dT%H:%M:%SZ')}",
        "sort": [{"by": "datetime", "direction": "desc"}],
        "limit": 50,
    }
    raw = stac._search(source.url, body, source.timeout)
    (FIXTURES / f"stac_{name}.json").write_text(json.dumps({"features": raw}, indent=1))
    print(f"stac_{name}.json: {len(raw)} items", file=sys.stderr)
    return {item["id"]: item for item in raw if item.get("id")}


def class_shares(url: str, geometry: dict | None = None) -> dict:
    """Class histogram of one granule, decimated, beside one cell's own counts."""
    dataset = cogs.open_band(url)
    try:
        whole = dataset.read(1, out_shape=(HISTOGRAM_GRID, HISTOGRAM_GRID))
    finally:
        dataset.close()
    values = whole.astype(np.int64).ravel()
    total = int(values.size)
    valid = int((values != 0).sum())
    counts = {int(cls): int((values == cls).sum()) for cls in np.unique(values)}
    entry = {
        "scene_wide": {
            "total_pixels": total,
            "valid_pixels": valid,
            "pct_of_all": {cls: round(100.0 * count / total, 2) for cls, count in counts.items()},
            "pct_of_valid": {cls: round(100.0 * count / valid, 2) for cls, count in counts.items()}
            if valid else {},
        }
    }
    if geometry is not None:
        read = cogs.read_for_geometry(url, geometry)
        cell = read.values.astype(np.int64).ravel()
        cell_counts = {int(cls): int((cell == cls).sum()) for cls in np.unique(cell)}
        entry["over_cell"] = {"total_pixels": int(cell.size),
                             "pct_of_all": {cls: round(100.0 * count / cell.size, 2)
                                            for cls, count in cell_counts.items()}}
    return entry


def window_tuple(window) -> tuple:
    """rasterio 1.5 windows have no totuple(), and the ints are what travel."""
    return (int(window.col_off), int(window.row_off), int(window.width), int(window.height))


def main() -> int:
    FIXTURES.mkdir(parents=True, exist_ok=True)
    cell = aoi.cell(LAT, LON, HECTARES)
    search_geometry = aoi.search_geometry([cell])
    start = END - dt.timedelta(days=WINDOW_DAYS)

    dump_stac("cdse", stac.CDSE, search_geometry, start, END)
    element84_items = dump_stac("element84", stac.ELEMENT84, search_geometry, start, END)

    choice = job.choose_scene(search_geometry, start, END, sources=job.SOURCES,
                              budget=cogs.ReadBudget(max_seconds=600))
    scene = choice.scene
    print(f"chosen {scene.scene_id} quality={choice.quality} "
          f"screened={[(row.scene_id, row.usable_pct) for row in choice.screened]}",
          file=sys.stderr)

    # The mask table is the one number in this service that cannot be checked by
    # looking plausible. Record it against the product's own metadata for a clear
    # granule, a cloudy one and the scene the golden figures come from.
    pool, _ = stac.candidate_pool(job.SOURCES, search_geometry, start, END)
    cloudy = sorted((item for item in pool if item.scene_id != scene.scene_id),
                    key=lambda item: -(item.cloud_pct or 0.0))[:1]
    evidence = []
    for record in [scene, *cloudy]:
        props = element84_items.get(record.scene_id, {}).get("properties", {})
        evidence.append({
            "scene_id": record.scene_id,
            "processor_baseline": record.baseline,
            "cloud_pct": record.cloud_pct,
            "item_percentages": {key: value for key, value in props.items()
                                 if key.startswith("s2:") and key.endswith("percentage")},
            "raster": class_shares(record.bands["scl"], cell if record.scene_id == scene.scene_id else None),
        })
        print(f"  histogram {record.scene_id}", file=sys.stderr)
    (FIXTURES / "class_histogram.json").write_text(json.dumps({
        "usable_classes": sorted(ndvi.USABLE_CLASSES),
        "class_table": ndvi.CLASS_TABLE,
        "granules": evidence,
    }, indent=1))

    measure_budget = cogs.ReadBudget(max_seconds=600)
    notes: list[str] = []
    stats = job.measure(scene, [job.Cell("plot-1", cell, HECTARES)],
                        quality=choice.quality, notes=notes, budget=measure_budget)

    # The raw reads behind those figures, so a test can replay measure() without
    # a network and get the same numbers.
    replay = cogs.ReadBudget(max_seconds=600)
    red = cogs.read_for_geometry(scene.bands["red"], cell, budget=replay)
    nir = cogs.read_at(scene.bands["nir"], red.window, budget=replay)
    scl = cogs.read_for_geometry(scene.bands["scl"], cell, budget=replay)

    tile_budget = cogs.ReadBudget(max_seconds=600)
    tile_red = cogs.read_tile(scene.bands["red"], ZOOM, TILE_X, TILE_Y, budget=tile_budget)
    tile_nir = cogs.read_at(scene.bands["nir"], tile_red.window,
                            out_shape=tile_red.values.shape, budget=tile_budget)
    tile_scl = cogs.read_tile(scene.bands["scl"], ZOOM, TILE_X, TILE_Y, budget=tile_budget)

    records = {}
    for kind, reads in (("cell", {"red": red, "nir": nir, "scl": scl}),
                        ("tile", {"red": tile_red, "nir": tile_nir, "scl": tile_scl})):
        for role, read in reads.items():
            prefix = f"{kind}_{role}"
            records[prefix] = read.values
            records[f"{prefix}_window"] = window_tuple(read.window)
            records[f"{prefix}_transform"] = list(read.transform)
            records[f"{prefix}_crs"] = str(read.crs)

    np.savez_compressed(
        FIXTURES / "bands.npz",
        meta=np.array([json.dumps({
            "scene": job.scene_ref(scene),
            "cell_geometry": cell,
            "search_geometry": search_geometry,
            "measure": stats,
            "screened": [{"scene_id": row.scene_id, "usable_pct": row.usable_pct,
                          "cloud_pct": row.cloud_pct, "acquired_at": row.acquired_at}
                         for row in choice.screened],
            "choose_notes": choice.notes,
            "tile": {"z": ZOOM, "x": TILE_X, "y": TILE_Y},
            "rasterio_version": rasterio.__version__,
            "gdal_version": rasterio.gdal_version(),
            "palette_version": palette.PALETTE_VERSION,
            "pixels": {"measure": measure_budget.pixels_read,
                       "replay": replay.pixels_read,
                       "tile": tile_budget.pixels_read},
        })], dtype=object),
        **records,
        allow_pickle=True,
    )

    aligned = cogs.rescale_mask(tile_scl.values, tile_scl.transform, tile_scl.crs,
                                tile_red.values.shape, tile_red.transform, tile_red.crs)
    index = ndvi.from_reflectance(tile_red.values, tile_nir.values, aligned)
    on_grid = render.reproject_index(index, tile_red.transform, tile_red.crs, ZOOM, TILE_X, TILE_Y)
    png = FIXTURES / f"tile_{ZOOM}_{TILE_X}_{TILE_Y}.png"
    png.write_bytes(render.png_bytes(palette.colorize(on_grid)))

    print(json.dumps({"scene": scene.scene_id, "measure": {
        key: stats[key] for key in ("mean_ndvi", "median_ndvi", "coverage_pct",
                                    "valid_pixels", "expected_pixels", "quality")},
        "golden_png_bytes": png.stat().st_size}, indent=1))
    return 0


if __name__ == "__main__":
    sys.exit(main())
