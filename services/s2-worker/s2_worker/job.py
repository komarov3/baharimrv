"""The work behind each route, free of HTTP.

The phase 1 spike and the service call these functions, so the figures the spec
quotes are the figures the endpoints return. Two rules from
SATELLITE_NDVI_SPEC.md 8 shape the whole module: the coverage screen decides
which granule a project gets, and one union window decides how few remote opens
that costs.
"""

from __future__ import annotations

import datetime as dt
from dataclasses import dataclass, field
from typing import Sequence

import numpy as np
from rasterio.windows import from_bounds as pixel_window
from rasterio.warp import transform_bounds

from . import aoi, cogs, ndvi, palette, render, stac

SOURCES: tuple[stac.StacSource, ...] = (stac.CDSE, stac.ELEMENT84)

# 5.2: the scene-wide cloud figure says nothing about the plots, so a candidate
# is only adopted once this much of the class layer over the AOI is a surface
# that can be indexed.
MIN_COVERAGE_PCT = 60.0
MAX_SCENE_TRIES = 5

# Rendering walks tiles, and each tile is three remote opens at zoom 14 and
# above. The cap is what stops one refresh from spending the container.
MAX_TILES_PER_JOB = 64

ZOOM_MIN = palette.ZOOM_MIN
ZOOM_MAX = palette.ZOOM_MAX


class Unavailable(RuntimeError):
    """Nothing usable exists for this AOI in this window.

    The API renders this as the existing unavailable state. The worker never
    returns a synthetic pass, and never measures the nearest scene it could
    reach.
    """


@dataclass
class Cell:
    """One plot's buffered analysis cell."""

    plot_id: str
    geometry: dict
    area_hectares: float | None = None


@dataclass
class Screened:
    """One candidate the coverage screen looked at, kept so the walk is auditable."""

    scene_id: str
    usable_pct: float
    cloud_pct: float | None
    acquired_at: str


@dataclass
class Choice:
    scene: stac.Scene
    quality: str
    screened: list[Screened] = field(default_factory=list)
    notes: list[str] = field(default_factory=list)


def ladder(max_cloud_pct: float | None = None) -> tuple[float, ...]:
    """Cloud rungs up to the caller's ceiling.

    The first rung is still a complete pass and anything accepted above it is
    marked cloudy, so raising the ceiling widens the search without quietly
    presenting a hazy scene as a clean one.
    """
    if max_cloud_pct is None:
        return stac.CLOUD_CEILINGS
    rungs = [rung for rung in stac.CLOUD_CEILINGS if rung < max_cloud_pct]
    rungs.append(max_cloud_pct)
    return tuple(rungs)


def parse_end(value: str | None) -> dt.datetime:
    """An upper bound for discovery, in UTC.

    A bare date is the whole day. Midnight would drop that morning's pass, which
    is exactly the freshest scene the operator asked for; the spike lost a
    same-day granule that way before this rule existed.
    """
    if not value:
        stamp = dt.datetime.now(dt.timezone.utc)
        return stamp.replace(hour=23, minute=59, second=59)
    parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    parsed = (parsed.astimezone(dt.timezone.utc) if parsed.tzinfo
              else parsed.replace(tzinfo=dt.timezone.utc))
    if len(value) == 10:
        parsed = parsed.replace(hour=23, minute=59, second=59)
    return parsed


def window_for(end: dt.datetime, days: int) -> tuple[dt.datetime, dt.datetime]:
    return end - dt.timedelta(days=days), end


def usable_pct(scene: stac.Scene, geometry: dict,
               budget: cogs.ReadBudget | None = None) -> float:
    """Percent of the class layer over the AOI that the mask can use.

    One 20 m read, the cheapest honest look at whether a granule is worth its
    two 10 m siblings. Over Kilifi creek the 2026-11-10 pass reported 24.9
    percent cloud and was entirely cloud over the plot, while a pass five days
    older reported 29.6 percent and was entirely vegetation.
    """
    scl = cogs.read_for_geometry(scene.bands["scl"], geometry, budget=budget)
    return round(100.0 * float(np.isin(scl.values, sorted(ndvi.USABLE_CLASSES)).mean()), 2)


def choose_scene(geometry: dict, start: dt.datetime, end: dt.datetime, *,
                 sources: Sequence[stac.StacSource] = SOURCES,
                 ceilings: Sequence[float] | None = None,
                 min_coverage_pct: float = MIN_COVERAGE_PCT,
                 max_tries: int = MAX_SCENE_TRIES,
                 budget: cogs.ReadBudget | None = None) -> Choice:
    """Walk candidates by rank until one is clear enough over the plots.

    Rank is cloud tier then recency; the screen is what turns a rank into an
    adoption. Nothing meeting the coverage floor is not a failure: the
    best-seen candidate is returned with quality `partial`, because the operator
    needs the number that exists plus the coverage it came with.
    """
    rungs = tuple(ceilings) if ceilings else stac.CLOUD_CEILINGS
    pool, notes = stac.candidate_pool(sources, geometry, start, end, ceilings=rungs)
    ranked = stac.order_candidates(pool, rungs)
    if not ranked:
        raise Unavailable("; ".join(notes) or "no scene in the window serves usable pixels")
    screened: list[Screened] = []
    best: tuple[stac.Scene, float] | None = None
    for scene in ranked[:max_tries]:
        try:
            fraction = usable_pct(scene, geometry, budget)
        except cogs.BudgetExceeded as exc:
            # Stop rather than try the next candidate: the guard is on the job,
            # and a fresh granule would spend the same pixels.
            notes.append(str(exc))
            break
        except cogs.BandUnavailable as exc:
            notes.append(f"{scene.scene_id}: unreadable ({str(exc)[:90]})")
            continue
        screened.append(Screened(scene.scene_id, fraction, scene.cloud_pct,
                                 scene.acquired_at.isoformat()))
        if fraction >= min_coverage_pct:
            quality = "cloudy" if stac.cloud_rung(scene, rungs) else "complete"
            if len(screened) > 1:
                notes.append(f"{len(screened)} candidates screened before {scene.scene_id} "
                             f"cleared {min_coverage_pct}% over the plots")
            return Choice(scene, quality, screened, notes)
        if best is None or fraction > best[1]:
            best = (scene, fraction)
    if best is None:
        raise Unavailable("; ".join(notes) or "no candidate could be read")
    notes.append(f"none of {len(screened)} screened scene(s) reached {min_coverage_pct}% usable; "
                 f"best was {best[0].scene_id} at {best[1]}%")
    return Choice(best[0], "partial", screened, notes)


def index_at(scene: stac.Scene, geometry: dict,
             budget: cogs.ReadBudget | None = None) -> tuple[np.ndarray, cogs.BandRead]:
    """Masked NDVI over one geometry, with the grid the figures are reported on."""
    red = cogs.read_for_geometry(scene.bands["red"], geometry, budget=budget)
    nir = cogs.read_at(scene.bands["nir"], red.window, budget=budget)
    scl = cogs.read_for_geometry(scene.bands["scl"], geometry, budget=budget)
    aligned = cogs.rescale_mask(scl.values, scl.transform, scl.crs,
                                red.values.shape, red.transform, red.crs)
    return ndvi.from_reflectance(red.values, nir.values, aligned), red


def _polygon(box: tuple[float, float, float, float]) -> dict:
    west, south, east, north = box
    return {"type": "Polygon",
            "coordinates": [[[west, south], [east, south], [east, north],
                             [west, north], [west, south]]]}


def _cell_window(box: tuple[float, float, float, float], read: cogs.BandRead) -> tuple[int, int, int, int]:
    """Row/col slice of one cell inside a band read covering a wider area."""
    west, south, east, north = transform_bounds("EPSG:4326", read.crs, *box)
    window = pixel_window(west, south, east, north, transform=read.transform)
    rows, cols = read.values.shape
    col_start = max(int(np.floor(window.col_off)), 0)
    row_start = max(int(np.floor(window.row_off)), 0)
    col_stop = min(int(np.ceil(window.col_off + window.width)), cols)
    row_stop = min(int(np.ceil(window.row_off + window.height)), rows)
    if col_stop <= col_start or row_stop <= row_start:
        raise cogs.NoOverlap("a cell falls outside the window read for this project")
    return row_start, col_start, row_stop, col_stop


def cell_arrays(scene: stac.Scene, cells: Sequence[Cell], budget: cogs.ReadBudget | None,
                notes: list[str]) -> list[np.ndarray]:
    """One masked NDVI array per cell, from as few remote reads as possible.

    The union window is the whole point: the cost of a read is the open against
    the imagery host, not the pixels, so 40 plots spread over one rectangle are
    three opens rather than a hundred and twenty. A union too large for the
    window ceiling falls back to per-cell reads and says so in the notes,
    because the fallback is slower and the operator should see that it happened.

    Each returned array is the cell's own pixels, not the padded window the read
    asked for. The pad exists so the mask reprojection has context at the edges;
    counting it would measure a 200 m square and call it a 158 m plot, which at
    a tidal edge means averaging water into a mangrove figure.
    """
    try:
        index, read = index_at(scene, _polygon(aoi.union_bbox([c.geometry for c in cells])), budget)
        arrays = []
        for cell in cells:
            row_start, col_start, row_stop, col_stop = _cell_window(aoi.bbox(cell.geometry), read)
            arrays.append(index[row_start:row_stop, col_start:col_stop])
        return arrays
    except (cogs.WindowTooLarge, cogs.NoOverlap) as exc:
        if len(cells) == 1:
            raise
        notes.append(f"union window refused ({str(exc)[:90]}); reading {len(cells)} cells separately")
    arrays: list[np.ndarray] = []
    for cell in cells:
        try:
            arrays.append(index_at(scene, cell.geometry, budget)[0])
        except cogs.BudgetExceeded as exc:
            notes.append(f"{str(exc)}; reporting the {len(arrays)} cell(s) already read")
            break
    if not arrays:
        raise cogs.BudgetExceeded("no cell fit the job budget")
    return arrays


def project_mean(rows: Sequence[dict], weight_by_area: bool = True) -> float | None:
    """Area-weighted mean of the cell means. A 0.2 ha plot is not a 5 ha plot."""
    usable = [(row["mean_ndvi"], row["area_hectares"]) for row in rows if row["mean_ndvi"] is not None]
    if not usable:
        return None
    if not weight_by_area:
        return round(sum(value for value, _ in usable) / len(usable), 4)
    total = sum(hectares or 0.0 for _, hectares in usable)
    if total <= 0:
        return round(sum(value for value, _ in usable) / len(usable), 4)
    return round(sum(value * (hectares or 0.0) for value, hectares in usable) / total, 4)


def measure(scene: stac.Scene, cells: Sequence[Cell], *, quality: str = "complete",
            notes: list[str] | None = None, budget: cogs.ReadBudget | None = None,
            weight_by_area: bool = True) -> dict:
    """Everything `/v1/stats` returns for one scene and one project.

    `mean_ndvi` is area-weighted across cells; the median and the coverage are
    pooled over the same pixels, which is the difference between a mean of
    means and a figure a reader can check against the raster.
    """
    notes = notes if notes is not None else []
    try:
        arrays = cell_arrays(scene, cells, budget, notes)
    except cogs.BudgetExceeded as exc:
        notes.append(f"{str(exc)}; no cell could be measured")
        arrays = []

    rows = [{"plot_id": cell.plot_id, "area_hectares": cell.area_hectares,
             **ndvi.stats(array, array.size)}
            for cell, array in zip(cells, arrays)]

    pooled = (np.concatenate([array[np.isfinite(array)] for array in arrays])
              if arrays else np.empty(0))
    expected = sum(row["expected_pixels"] for row in rows)
    figure = ndvi.stats(pooled, expected)
    if not rows:
        quality = "partial"
    return {
        **_envelope(scene, quality, notes),
        "mean_ndvi": project_mean(rows, weight_by_area),
        "median_ndvi": figure["median_ndvi"],
        "valid_pixels": figure["valid_pixels"],
        "expected_pixels": expected,
        "coverage_pct": figure["coverage_pct"],
        "per_cell": rows,
    }


def _envelope(scene: stac.Scene, quality: str, notes: Sequence[str]) -> dict:
    """The provenance every response carries, so a figure cannot be read without it."""
    return {
        "scene_id": scene.scene_id,
        "acquired_at": scene.acquired_at.isoformat(),
        "cloud_pct": scene.cloud_pct,
        "platform": scene.platform,
        "epsg": scene.epsg,
        "relative_orbit": scene.orbit,
        "grid": scene.tile,
        "processor_baseline": scene.baseline,
        "discovery_source": scene.discovery_source,
        "footprint_geojson": scene.footprint,
        "quality": quality,
        "class_table": ndvi.CLASS_TABLE,
        "palette_version": palette.PALETTE_VERSION,
        "notes": list(notes),
    }


def scene_ref(scene: stac.Scene) -> dict:
    """What `/v1/scene` hands back, including the band URLs a later call needs."""
    return {**_envelope(scene, "complete", []), "bands": dict(scene.bands)}


def scene_from_ref(payload: dict) -> stac.Scene:
    """Rebuild a scene from a caller-supplied payload.

    The worker keeps no state between calls, so the Go API sends the band URLs
    back on `/v1/stats` and `/v1/tiles/render`. That makes the host allow list in
    `cogs._vsicurl` the security boundary for this input: a payload naming any
    other host fails at the first read rather than turning the worker into a
    proxy.
    """
    bands = {role: str(href) for role, href in (payload.get("bands") or {}).items()}
    acquired = payload.get("acquired_at")
    return stac.Scene(
        scene_id=str(payload["scene_id"]),
        acquired_at=parse_end(str(acquired)) if acquired else dt.datetime.fromtimestamp(0, dt.timezone.utc),
        cloud_pct=float(payload["cloud_pct"]) if payload.get("cloud_pct") is not None else None,
        platform=str(payload.get("platform") or ""),
        epsg=payload.get("epsg"),
        orbit=str(payload.get("relative_orbit") or ""),
        tile=str(payload.get("grid") or ""),
        baseline=str(payload.get("processor_baseline") or ""),
        discovery_source=str(payload.get("discovery_source") or ""),
        bands=bands,
        footprint=payload.get("footprint_geojson"),
    )


def validate_bands(scene: stac.Scene) -> None:
    """Fail before the first read if this scene cannot serve the three roles."""
    missing = [role for role in stac.BAND_ASSETS if role not in scene.bands]
    if missing:
        raise Unavailable(f"{scene.scene_id} has no COG for {', '.join(missing)}")
    if not stac.table_supported(scene):
        raise Unavailable(f"{scene.scene_id} is processor {scene.baseline or 'unknown'}, below the "
                          f"class table this worker knows ({stac.MIN_CLASS_TABLE_BASELINE}+)")


def tile_png(scene: stac.Scene, z: int, x: int, y: int,
             budget: cogs.ReadBudget | None = None) -> bytes | None:
    """One rendered RGBA tile, or None where the granule does not reach."""
    red = cogs.read_tile(scene.bands["red"], z, x, y, budget=budget)
    if red is None:
        return None
    nir = cogs.read_at(scene.bands["nir"], red.window, out_shape=red.values.shape, budget=budget)
    scl = cogs.read_tile(scene.bands["scl"], z, x, y, budget=budget)
    if scl is None:
        return None
    aligned = cogs.rescale_mask(scl.values, scl.transform, scl.crs,
                                red.values.shape, red.transform, red.crs)
    index = ndvi.from_reflectance(red.values, nir.values, aligned)
    on_grid = render.reproject_index(index, red.transform, red.crs, z, x, y)
    return render.png_bytes(palette.colorize(on_grid))


def render_tiles(scene: stac.Scene, bounds: tuple[float, float, float, float],
                 zoom_min: int, zoom_max: int, store, key_prefix: str, *,
                 budget: cogs.ReadBudget | None = None,
                 max_tiles: int = MAX_TILES_PER_JOB,
                 notes: list[str] | None = None) -> dict:
    """Render and store every tile of every zoom level touching `bounds`.

    Keys are scene-scoped, so a new granule never overwrites an old tile and a
    stale cache entry cannot serve last month's pixels under this month's label.
    """
    if not ZOOM_MIN <= zoom_min <= zoom_max <= ZOOM_MAX:
        raise ValueError(f"zoom must stay inside {ZOOM_MIN} to {ZOOM_MAX}")
    notes = notes if notes is not None else []
    plan = [(z, x, y) for z in range(zoom_min, zoom_max + 1)
            for x, y in render.tiles_covering(bounds, z)]
    rendered = empty = omitted = 0
    written_bytes = 0
    quality = "complete"
    for position, (z, x, y) in enumerate(plan):
        if position >= max_tiles:
            quality, omitted = "partial", len(plan) - max_tiles
            notes.append(f"{omitted} tile(s) over the {max_tiles} per-job cap were not rendered")
            break
        try:
            payload = tile_png(scene, z, x, y, budget)
        except cogs.BudgetExceeded as exc:
            quality, omitted = "partial", len(plan) - position
            notes.append(f"{str(exc)}; {omitted} tile(s) left unrendered")
            break
        except cogs.BandUnavailable as exc:
            empty += 1
            notes.append(f"{z}/{x}/{y} unreadable ({str(exc)[:90]})")
            continue
        if payload is None:
            empty += 1
            continue
        store.put(f"{key_prefix}/{z}/{x}/{y}.png", payload)
        rendered += 1
        written_bytes += len(payload)
    return {
        "scene_id": scene.scene_id,
        "key_prefix": key_prefix,
        "palette_version": palette.PALETTE_VERSION,
        "class_table": ndvi.CLASS_TABLE,
        "quality": quality,
        "tiles_rendered": rendered,
        "tiles_empty": empty,
        "tiles_omitted": omitted,
        "bytes_written": written_bytes,
        "pixels_read": budget.pixels_read if budget else 0,
        "zoom_min": zoom_min,
        "zoom_max": zoom_max,
        "notes": notes,
    }
