"""The s2-worker HTTP surface: six routes, no project state.

Everything here is a thin envelope around `job.py`, plus the two things that only
belong at the edge: the bearer check and the concurrency cap. The worker is the
only component that talks to the imagery hosts, so it must never become an open
proxy for a caller it has not authenticated, and a burst of refresh taps from one
pilot must not saturate the container the others are queued behind.

Error shape is the contract the Go API reads. Every failure is
`{"error": <code>, "detail": <text>, "notes": [...]}` with `error` from a closed
list, because the API has to tell "no scene exists" from "the catalog was down"
and render them differently. It never has to parse a message.
"""

from __future__ import annotations

import datetime as dt
import hmac
import threading
from contextlib import contextmanager
from typing import Any, Iterator

from fastapi import Depends, FastAPI, Header, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse, Response
from pydantic import BaseModel, Field

from . import cogs, job, ndvi, palette, stac, store as store_module
from .config import Settings

VERSION = "0.1.0"

MAX_CELLS = 500


class WorkerError(Exception):
    def __init__(self, status: int, code: str, detail: str, notes: list[str] | None = None):
        super().__init__(detail)
        self.status = status
        self.code = code
        self.detail = detail
        self.notes = notes or []


def _unauthorized() -> WorkerError:
    return WorkerError(401, "unauthorized", "missing or wrong bearer token")


class SceneRequest(BaseModel):
    aoi_geojson: dict = Field(description="GeoJSON Polygon or MultiPolygon over the project cells")
    window_days: int = 45
    max_cloud_pct: float | None = None
    end_time: str | None = Field(default=None, description="ISO timestamp or date; a bare date means the whole day")
    min_coverage_pct: float | None = None
    max_scene_tries: int | None = None


class CellModel(BaseModel):
    plot_id: str
    geometry: dict
    area_hectares: float | None = None


class StatsRequest(BaseModel):
    cells: list[CellModel]
    scene: dict
    weight_by_area: bool = True


class RenderRequest(BaseModel):
    bbox: list[float] = Field(description="min_lon, min_lat, max_lon, max_lat in WGS 84")
    zoom_min: int
    zoom_max: int
    scene: dict
    palette_version: str
    key_prefix: str


class PurgeRequest(BaseModel):
    key_prefix: str


class Worker:
    """The per-application state: settings, the tile store, the slot, the log."""

    def __init__(self, settings: Settings, tile_store: Any, sources: tuple[stac.StacSource, ...] | None = None):
        self.settings = settings
        self.store = tile_store
        self.sources = sources if sources is not None else settings.sources()
        self.slot = threading.BoundedSemaphore(settings.max_concurrency)
        self.queue_wait_s = 30.0
        self.sources_seen: dict[str, dict] = {}
        self.last_error = ""

    @contextmanager
    def job_slot(self) -> Iterator[None]:
        if not self.slot.acquire(timeout=self.queue_wait_s):
            raise WorkerError(503, "busy",
                              f"{self.settings.max_concurrency} job(s) already running "
                              f"and none freed in {self.queue_wait_s:.0f}s")
        try:
            yield
        finally:
            self.slot.release()

    def record_discovery(self, notes: list[str]) -> None:
        """Remember which catalogs answered, for `/v1/health`, from the last real job.

        A failure note is always prefixed with the source name, so health can
        report reachability without making its own calls to two public services
        every time the platform probes this container.
        """
        stamp = dt.datetime.now(dt.timezone.utc).isoformat()
        for source in self.sources:
            failed = [note for note in notes if note.startswith(f"{source.name}:")]
            self.sources_seen[source.name] = {
                "reachable": not failed, "checked_at": stamp,
                "error": (failed[0][:200] if failed else "")}

    def fail(self, exc: Exception) -> WorkerError:
        """Turn anything a job raised into the typed error the API expects."""
        if isinstance(exc, cogs.BandUnavailable):
            error = WorkerError(502, "band_unavailable", str(exc)[:300])
        elif isinstance(exc, store_module.StoreUnavailable):
            error = WorkerError(503, "store_unavailable", str(exc)[:300])
        elif isinstance(exc, store_module.BadKey):
            error = WorkerError(400, "bad_request", str(exc))
        elif isinstance(exc, ValueError):
            error = WorkerError(400, "bad_request", str(exc))
        else:
            # A BudgetExceeded lands here. It should not: measure() and
            # render_tiles() both absorb it into a partial result, so seeing one
            # means a call site forgot to.
            error = WorkerError(500, "internal", f"{type(exc).__name__}: {exc}"[:300])
        self.last_error = f"{dt.datetime.now(dt.timezone.utc).isoformat()} {error.code}: {error.detail}"
        return error


def create_app(settings: Settings | None = None, tile_store: Any = None,
               sources: tuple[stac.StacSource, ...] | None = None) -> FastAPI:
    """Build the service. Every argument is injectable so tests need no network,
    no credentials and no mounted volume."""
    settings = (settings or Settings.from_env()).apply()
    tile_store = tile_store if tile_store is not None else store_module.from_settings(settings)
    worker = Worker(settings, tile_store, sources)

    app = FastAPI(title="baharimrv s2-worker", version=VERSION, docs_url=None, redoc_url=None)
    app.state.worker = worker

    @app.exception_handler(WorkerError)
    async def on_worker_error(_: Request, exc: WorkerError) -> JSONResponse:
        return JSONResponse(status_code=exc.status,
                            content={"error": exc.code, "detail": exc.detail, "notes": exc.notes})

    @app.exception_handler(RequestValidationError)
    async def on_validation_error(_: Request, exc: RequestValidationError) -> JSONResponse:
        """FastAPI's own 422 shape is a list of objects; the API parses one field.

        A body pydantic cannot read is still a bad request, so it gets the same
        envelope as the checks below rather than a second error format.
        """
        fields = "; ".join(f"{'.'.join(str(part) for part in err['loc'][1:]) or 'body'}: {err['msg']}"
                           for err in exc.errors()[:5])
        return JSONResponse(status_code=400,
                            content={"error": "bad_request", "detail": fields[:300], "notes": []})

    def authorize(authorization: str = Header(default="")) -> None:
        expected = f"Bearer {settings.token}"
        if not settings.token or not hmac.compare_digest(authorization, expected):
            raise _unauthorized()

    def geometry_of(payload: dict) -> dict:
        if payload.get("type") not in ("Polygon", "MultiPolygon"):
            raise WorkerError(400, "bad_request",
                              f"aoi_geojson must be a Polygon or MultiPolygon, not {payload.get('type')!r}")
        return payload

    def prefix_of(value: str) -> str:
        """`store.check_prefix` with the failure typed, so both routes answer 400.

        The store raises `BadKey` for a prefix that could carry `..`, which
        would otherwise reach the caller as an internal error: a validation
        failure wearing the wrong label.
        """
        try:
            return store_module.check_prefix(value)
        except store_module.BadKey as exc:
            raise WorkerError(400, "bad_request", str(exc)) from exc

    @app.post("/v1/scene", dependencies=[Depends(authorize)])
    def choose_scene(request: SceneRequest) -> dict:
        """The granule this project should be measured on, screen included.

        The screen is a pixel read, so it lives here and not in the Go API: the
        caller asks once and gets the scene that passed, plus every candidate it
        was preferred over.
        """
        geometry = geometry_of(request.aoi_geojson)
        if request.window_days < 1 or request.window_days > 365:
            raise WorkerError(400, "bad_request", "window_days must be between 1 and 365")
        end = job.parse_end(request.end_time)
        start, end = job.window_for(end, request.window_days)
        ceilings = job.ladder(request.max_cloud_pct)
        with worker.job_slot():
            budget = settings.new_budget()
            try:
                choice = job.choose_scene(
                    geometry, start, end, sources=worker.sources, ceilings=ceilings,
                    min_coverage_pct=(request.min_coverage_pct if request.min_coverage_pct is not None
                                      else settings.min_coverage_pct),
                    max_tries=(request.max_scene_tries if request.max_scene_tries is not None
                               else settings.max_scene_tries),
                    budget=budget)
            except job.Unavailable as exc:
                notes = str(exc).split("; ")
                worker.record_discovery(notes)
                raise WorkerError(404, "unavailable", str(exc)[:300], notes) from exc
            except cogs.BandUnavailable as exc:
                raise worker.fail(exc) from exc
        worker.record_discovery(choice.notes)
        return {
            **job.scene_ref(choice.scene),
            "quality": choice.quality,
            # A scene on its own has no history, which is why `scene_ref` leaves
            # `notes` empty. The walk that reached this granule does, and the
            # caller has to be able to say why an older pass beat a newer one.
            "notes": choice.notes,
            "window": {"start": start.isoformat(), "end": end.isoformat(),
                       "days": request.window_days, "cloud_ceilings": list(ceilings)},
            "screened": [{"scene_id": row.scene_id, "usable_pct": row.usable_pct,
                          "cloud_pct": row.cloud_pct, "acquired_at": row.acquired_at}
                         for row in choice.screened],
            "pixels_read": budget.pixels_read,
        }

    @app.post("/v1/stats", dependencies=[Depends(authorize)])
    def stats(request: StatsRequest) -> dict:
        """Per-cell and project figures for one scene.

        The scene arrives from the caller, which is why the band host allow list
        is enforced on every read: this route must not fetch a URL nobody vetted.
        """
        if not request.cells:
            raise WorkerError(400, "bad_request", "cells is empty")
        if len(request.cells) > MAX_CELLS:
            raise WorkerError(400, "bad_request", f"{len(request.cells)} cells over the {MAX_CELLS} ceiling")
        cells = [job.Cell(plot_id=cell.plot_id, geometry=cell.geometry,
                          area_hectares=cell.area_hectares) for cell in request.cells]
        for cell in cells:
            if cell.geometry.get("type") not in ("Polygon", "MultiPolygon"):
                raise WorkerError(400, "bad_request", f"{cell.plot_id}: geometry must be a Polygon")
        scene = job.scene_from_ref(request.scene)
        try:
            job.validate_bands(scene)
        except job.Unavailable as exc:
            raise WorkerError(404, "unavailable", str(exc)) from exc
        with worker.job_slot():
            budget = settings.new_budget()
            notes: list[str] = []
            try:
                result = job.measure(scene, cells, quality="complete", notes=notes,
                                     budget=budget, weight_by_area=request.weight_by_area)
            except cogs.BandUnavailable as exc:  # NoOverlap is one of these
                raise worker.fail(exc) from exc
        result["pixels_read"] = budget.pixels_read
        result["weight_by_area"] = request.weight_by_area
        return result

    @app.post("/v1/tiles/render", dependencies=[Depends(authorize)])
    def render(request: RenderRequest) -> dict:
        """Render a scene into stored tiles under a caller-chosen prefix."""
        if len(request.bbox) != 4:
            raise WorkerError(400, "bad_request", "bbox needs min_lon, min_lat, max_lon, max_lat")
        min_lon, min_lat, max_lon, max_lat = request.bbox
        if not (min_lon < max_lon and min_lat < max_lat and -180 <= min_lon and max_lon <= 180):
            raise WorkerError(400, "bad_request", f"bbox {request.bbox} is not an ordered WGS 84 box")
        if request.palette_version != palette.PALETTE_VERSION:
            # Rule 7 exists so the map and the legend cannot disagree. A client
            # naming another palette is a client that must be redeployed, not one
            # whose tiles should be quietly painted with today's colours.
            raise WorkerError(409, "palette_mismatch",
                              f"worker paints {palette.PALETTE_VERSION}, request asked for "
                              f"{request.palette_version}")
        if not (job.ZOOM_MIN <= request.zoom_min <= request.zoom_max <= job.ZOOM_MAX):
            raise WorkerError(400, "bad_request",
                              f"zoom must stay inside {job.ZOOM_MIN} to {job.ZOOM_MAX}")
        prefix = prefix_of(request.key_prefix)
        scene = job.scene_from_ref(request.scene)
        try:
            job.validate_bands(scene)
        except job.Unavailable as exc:
            raise WorkerError(404, "unavailable", str(exc)) from exc
        with worker.job_slot():
            budget = settings.new_budget()
            try:
                result = job.render_tiles(scene, (min_lon, min_lat, max_lon, max_lat),
                                          request.zoom_min, request.zoom_max, worker.store,
                                          prefix, budget=budget,
                                          max_tiles=settings.max_tiles_per_job)
            except cogs.BandUnavailable as exc:
                raise worker.fail(exc) from exc
            except store_module.StoreUnavailable as exc:
                raise worker.fail(exc) from exc
        return result

    @app.get("/v1/tiles/{project_id}/{scene_id}/{z}/{x}/{y}.png",
             dependencies=[Depends(authorize)])
    def read_tile(request: Request, project_id: str, scene_id: str, z: int, x: int, y: int) -> Response:
        # The router's '$' regex also matches before a trailing newline.
        if not request.scope["path"].endswith(".png"):
            raise WorkerError(400, "bad_request", "tile path must end with .png")
        if not (job.ZOOM_MIN <= z <= job.ZOOM_MAX):
            raise WorkerError(400, "bad_request",
                              f"zoom must stay inside {job.ZOOM_MIN} to {job.ZOOM_MAX}")
        if not (0 <= x < 2**z and 0 <= y < 2**z):
            raise WorkerError(400, "bad_request", "tile coordinates are outside the zoom bounds")
        try:
            key = store_module.check_tile_key(f"{project_id}/{scene_id}/{z}/{x}/{y}.png")
            payload = worker.store.get(key)
        except store_module.TileNotFound as exc:
            raise WorkerError(404, "unavailable", "tile not found") from exc
        except (store_module.BadKey, store_module.StoreUnavailable) as exc:
            raise worker.fail(exc) from exc
        return Response(content=payload, media_type=store_module.CONTENT_TYPE,
                        headers={"Cache-Control": "no-store"})

    @app.delete("/v1/tiles", dependencies=[Depends(authorize)])
    def purge(request: PurgeRequest) -> dict:
        """Drop every tile of a scene a project has stopped using."""
        prefix = prefix_of(request.key_prefix)
        try:
            purged = worker.store.delete_prefix(prefix)
        except store_module.StoreUnavailable as exc:
            raise worker.fail(exc) from exc
        return {"key_prefix": prefix, "purged": purged}

    @app.get("/v1/health")
    def health() -> dict:
        """Liveness plus the two facts an operator actually needs.

        No token: Fly probes this, and the payload holds no project data and
        performs no imagery fetches.
        """
        store_error = worker.store.probe()
        reachable = {name: row["reachable"] for name, row in worker.sources_seen.items()}
        if not settings.token:
            status = "unconfigured"
        elif store_error or (reachable and not any(reachable.values())):
            status = "degraded"
        else:
            status = "ok"
        return {
            "status": status,
            "version": VERSION,
            "discovery_sources_reachable": reachable or None,
            "discovery_detail": worker.sources_seen or None,
            "tile_store_writable": not store_error,
            "tile_store_backend": getattr(worker.store, "name", "unknown"),
            "last_error": worker.last_error or (f"tile store: {store_error}" if store_error else ""),
            "palette_version": palette.PALETTE_VERSION,
            "class_table": ndvi.CLASS_TABLE,
            "limits": {"max_concurrency": settings.max_concurrency,
                       "max_pixels_per_job": settings.max_pixels_per_job,
                       "job_timeout_s": settings.job_timeout_s,
                       "min_coverage_pct": settings.min_coverage_pct,
                       "max_scene_tries": settings.max_scene_tries,
                       "max_tiles_per_job": settings.max_tiles_per_job},
            "band_hosts": list(settings.allowed_hosts),
        }

    return app
