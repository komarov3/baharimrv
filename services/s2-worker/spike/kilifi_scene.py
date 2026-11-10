#!/usr/bin/env python3
"""Phase 1 spike: one real Kilifi scene, from catalog search to PNG on disk.

Run this before believing any of the service design, and again when a figure in
SATELLITE_NDVI_SPEC.md needs re-measuring. It answers the three questions the
spec could not answer on paper: how many pixels a buffered cell really pulls,
how long the arithmetic takes on a laptop, and whether the masked NDVI of a
known mangrove stand lands in a plausible band.

It calls the service's own `job.py` rather than a copy of its arithmetic, which
is the point of this second version: a spike that windows the cell its own way
is a spike whose numbers drift away from the ones the routes return. What it
adds on top of the library is timing and a JSON summary.

Range requests are GDAL's own, not ours, so they are counted from its debug
stream rather than from this script:

    CPL_DEBUG=ON .venv/bin/python spike/kilifi_scene.py --lat -3.64 --lon 39.836 2>gdal.log
    grep -c "Range bytes request" gdal.log
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import pathlib
import sys
import time

import rasterio

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))

from s2_worker import aoi, cogs, job, ndvi, palette, stac, store  # noqa: E402

SOURCES = (stac.CDSE, stac.ELEMENT84)


def observe(label: str, geometry: dict, start: dt.datetime, end: dt.datetime,
            cells: list[job.Cell], budget: cogs.ReadBudget) -> tuple[stac.Scene, dict]:
    """The 5.2 candidate walk, then the measurement, with the walk printed."""
    started = time.perf_counter()
    choice = job.choose_scene(geometry, start, end, sources=SOURCES, budget=budget)
    for row in choice.screened:
        print(f"  {label} screen: {row.scene_id} at {row.usable_pct}% usable "
              f"(scene cloud {row.cloud_pct}%)", file=sys.stderr)
    print(f"  {label}: {choice.scene.scene_id} adopted as {choice.quality} "
          f"in {round(time.perf_counter() - started, 2)}s", file=sys.stderr)
    result = job.measure(choice.scene, cells, quality=choice.quality, notes=choice.notes,
                         budget=budget)
    for row in result["per_cell"]:
        print(f"  {label} {row['plot_id']}: mean={row['mean_ndvi']} "
              f"coverage={row['coverage_pct']}% {row['valid_pixels']}/{row['expected_pixels']} px",
              file=sys.stderr)
    return choice.scene, result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--lat", type=float, required=True)
    parser.add_argument("--lon", type=float, required=True)
    parser.add_argument("--hectares", type=float, default=2.5)
    parser.add_argument("--end", type=str,
                        default=dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%d"))
    parser.add_argument("--window-days", type=int, default=45)
    parser.add_argument("--baseline-window-days", type=int, default=365)
    parser.add_argument("--zoom", type=int, default=14)
    parser.add_argument("--out-dir", type=pathlib.Path, default=pathlib.Path("/tmp/s2-spike"))
    parser.add_argument("--skip-baseline", action="store_true")
    parser.add_argument("--skip-tiles", action="store_true")
    args = parser.parse_args()

    end = job.parse_end(args.end)
    cell = aoi.cell(args.lat, args.lon, args.hectares)
    cells = [job.Cell("spike-plot-1", cell, args.hectares)]
    search_geometry = aoi.search_geometry([c.geometry for c in cells])
    bounds = aoi.bbox(cell)
    args.out_dir.mkdir(parents=True, exist_ok=True)

    # One budget per service call, not one per CLI run: the worker sees /v1/scene
    # plus /v1/stats as one job and /v1/tiles/render as another, so a ceiling
    # spent measuring the current scene must not truncate the baseline.
    jobs: dict[str, dict] = {}

    def budget_for(name: str) -> cogs.ReadBudget:
        budget = cogs.ReadBudget()
        jobs[name] = {"budget": budget, "seconds": None}
        return budget

    def close_out(name: str) -> None:
        """Time a job when it finishes. Measured at the summary instead, every
        job's seconds would read as the wall clock since the process started."""
        jobs[name]["seconds"] = round(time.monotonic() - jobs[name]["budget"].started_at, 2)

    started = time.perf_counter()
    print(f"discovery: {stac.CDSE.name} then {stac.ELEMENT84.name}, "
          f"window {end - dt.timedelta(days=args.window_days):%Y-%m-%d} to {end:%Y-%m-%d}",
          file=sys.stderr)
    current, result = observe("current", search_geometry,
                              end - dt.timedelta(days=args.window_days), end, cells,
                              budget_for("current"))
    close_out("current")
    summary = {"current": result}

    if not args.skip_baseline:
        baseline_end = end - dt.timedelta(days=args.baseline_window_days)
        try:
            _scene, baseline = observe("baseline", search_geometry,
                                       baseline_end - dt.timedelta(days=args.window_days),
                                       baseline_end, cells, budget_for("baseline"))
            close_out("baseline")
            summary["baseline"] = baseline
            if baseline["mean_ndvi"] and result["mean_ndvi"] is not None:
                change = round(result["mean_ndvi"] - baseline["mean_ndvi"], 4)
                summary["change"] = {"change_abs": change,
                                     "change_pct": round(100.0 * change / baseline["mean_ndvi"], 2)}
        except job.Unavailable as exc:
            close_out("baseline")
            summary["baseline"] = {"error": str(exc)}
            print(f"  baseline: no scene ({exc})", file=sys.stderr)

    if not args.skip_tiles:
        prefix = f"spike/{current.scene_id}"
        tiles = job.render_tiles(current, bounds, args.zoom, args.zoom,
                                 store.VolumeStore(args.out_dir), prefix,
                                 budget=budget_for("tiles"))
        close_out("tiles")
        print(f"  tiles: {tiles['tiles_rendered']} written, {tiles['tiles_empty']} empty, "
              f"{tiles['tiles_omitted']} omitted, under {args.out_dir}/{prefix}", file=sys.stderr)
        summary["tiles"] = {**tiles, "out_dir": str(args.out_dir / prefix)}

    summary["spend"] = {
        "wall_clock_s": round(time.perf_counter() - started, 2),
        "pixel_ceiling": cogs.MAX_PIXELS_PER_JOB,
        "job_timeout_ceiling_s": cogs.JOB_TIMEOUT_S,
        "per_job": {name: {"pixels_read": row["budget"].pixels_read, "seconds": row["seconds"]}
                    for name, row in jobs.items()},
        "coverage_floor_pct": job.MIN_COVERAGE_PCT,
        "scene_tries": job.MAX_SCENE_TRIES,
        "palette_version": palette.PALETTE_VERSION,
        "class_table": ndvi.CLASS_TABLE,
        "rasterio_version": rasterio.__version__,
        "gdal_version": rasterio.gdal_version(),
    }
    print(json.dumps(summary, indent=2, default=str))
    return 0


if __name__ == "__main__":
    sys.exit(main())
