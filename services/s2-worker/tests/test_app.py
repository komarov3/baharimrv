"""The HTTP contract of SATELLITE_NDVI_SPEC.md 8: six routes, one error shape.

The Go API is the only caller, and it branches on `error` rather than parsing a
message, so what these tests hold the service to is the closed code list and the
order of the checks: a request that is not authenticated must not spend a pixel,
a body that cannot be parsed must not arrive as FastAPI's 422 shape, and a scene
whose band URL names a host nobody vetted must fail at the allow list rather
than at the imagery host.

Two kinds of fixture appear here. The golden replay from `conftest.py` answers
the reads, so `/v1/stats` and `/v1/scene` run the real arithmetic over the
recorded Kilifi bytes and the figures are the ones the spec quotes. Where a
route fails before it reads, no replay is installed and a real outbound request
would be a hang rather than a clean failure.
"""

from __future__ import annotations

import socket
from unittest.mock import Mock

import numpy as np
import pytest
import rasterio
import uvicorn
from fastapi.testclient import TestClient
from rasterio.windows import Window

from s2_worker import aoi, cogs, job, ndvi, palette, stac, store as store_module
from s2_worker.app import MAX_CELLS, create_app
from s2_worker.config import Settings
from fakes import BrokenStore, MemoryStore, StubSource, scene_with_pixels

TOKEN = "worker-test-token"
AUTH = {"Authorization": f"Bearer {TOKEN}"}
KILIFI = (-3.6400, 39.8360)
CELL = scene_with_pixels("S2C_37MER_20261105_1_L2A")


def test_main_refuses_empty_token(monkeypatch, capsys):
    from s2_worker.__main__ import main

    monkeypatch.setenv("S2_WORKER_TOKEN", "")
    monkeypatch.setattr(uvicorn.Server, "run", lambda *args, **kwargs: pytest.fail("server started"))
    assert main() == 1
    assert "S2_WORKER_TOKEN is unset" in capsys.readouterr().err


@pytest.mark.parametrize("host", [None, "0.0.0.0", "::"])
def test_main_listener_accepts_health_and_private_connections(monkeypatch, tmp_path, host):
    from s2_worker.__main__ import main

    if host == "::" and not socket.has_dualstack_ipv6():
        pytest.skip("dual-stack sockets are unavailable")
    monkeypatch.setenv("S2_WORKER_TOKEN", TOKEN)
    monkeypatch.setenv("TILE_STORE_ROOT", str(tmp_path))
    monkeypatch.setenv("PORT", "0")
    if host is None:
        monkeypatch.delenv("HOST", raising=False)
    else:
        monkeypatch.setenv("HOST", host)
    listeners = []

    def run(server, sockets):
        listeners.extend(sockets)
        listener = sockets[0]
        port = listener.getsockname()[1]
        addresses = ["127.0.0.1", "::1"] if host == "::" else ["127.0.0.1"]
        for address in addresses:
            with socket.create_connection((address, port), timeout=1):
                accepted, _ = listener.accept()
                accepted.close()

    monkeypatch.setattr(uvicorn.Server, "run", run)
    assert main() == 0
    assert len(listeners) == 1
    assert listeners[0].fileno() == -1


@pytest.fixture(autouse=True)
def restore_module_state():
    """`Settings.apply()` writes the allow list into `cogs` for the process."""
    saved = cogs.ALLOWED_HOSTS
    yield
    cogs.ALLOWED_HOSTS = saved


def build_client(store=None, sources=(), token=TOKEN, **overrides):
    settings = Settings(token=token, max_concurrency=1, **overrides).apply()
    tile_store = store if store is not None else MemoryStore()
    app = create_app(settings, tile_store, sources)
    return TestClient(app), app.state.worker, tile_store


def stats_body(bands, **overrides):
    body = {"cells": [{"plot_id": "plot-1", "geometry": bands["meta"]["cell_geometry"],
                       "area_hectares": 2.5}],
            "scene": bands["meta"]["scene"], "weight_by_area": True}
    body.update(overrides)
    return body


def render_body(bands, **overrides):
    body = {"bbox": [39.83, -3.65, 39.85, -3.63], "zoom_min": 14, "zoom_max": 14,
            "scene": bands["meta"]["scene"], "palette_version": palette.PALETTE_VERSION,
            "key_prefix": f"proj-1/{CELL.scene_id}"}
    body.update(overrides)
    return body


def scene_body(**overrides):
    body = {"aoi_geojson": aoi.cell(KILIFI[0], KILIFI[1], 2.5), "window_days": 45}
    body.update(overrides)
    return body


@pytest.fixture
def screen(monkeypatch):
    """The coverage screen answered from a class code per band URL.

    The screen is one 20 m read, so a route test of the candidate walk needs
    known classes rather than a granule per candidate. Tests register what each
    candidate reports; anything unregistered reads as vegetation, because the
    point of a walk test is the candidate it stops on, not the pixels.
    """
    classes: dict[str, int] = {}

    def fake_read(url, geometry, pad_pixels=2, budget=None):
        if budget is not None:
            budget.charge(16)
        return cogs.BandRead(values=np.full((4, 4), classes.get(url, 4), np.uint8),
                             window=Window(0, 0, 4, 4),
                             transform=rasterio.Affine(20.0, 0.0, 0.0, 0.0, -20.0, 0.0),
                             crs="EPSG:32737")

    monkeypatch.setattr(cogs, "read_for_geometry", fake_read)
    return classes


# --- the bearer check --------------------------------------------------------


def test_a_request_with_no_token_is_a_401_and_spends_no_pixel(replay, bands):
    """Auth is a dependency, so it runs before the route body and before any read."""
    client, _, _ = build_client()
    response = client.post("/v1/stats", json=stats_body(bands))
    assert response.status_code == 401
    assert response.json() == {"error": "unauthorized", "detail": "missing or wrong bearer token",
                               "notes": []}
    assert replay.reads == [], "an unauthenticated call must not reach the imagery"


def test_the_wrong_token_is_a_401():
    client, _, _ = build_client()
    response = client.post("/v1/scene", json=scene_body(),
                           headers={"Authorization": "Bearer not-the-token"})
    assert response.status_code == 401


def test_a_deployment_without_the_secret_refuses_every_route_and_says_unconfigured():
    """`fly secrets list` empty is the failure this protects against."""
    client, _, _ = build_client(token="")
    assert client.post("/v1/scene", json=scene_body(), headers=AUTH).status_code == 401
    assert client.get("/v1/health").json()["status"] == "unconfigured"


def test_health_needs_no_token():
    client, _, _ = build_client()
    assert client.get("/v1/health").status_code == 200


# --- /v1/scene ---------------------------------------------------------------


def test_the_scene_route_returns_the_recorded_granule_with_its_screen_evidence(replay, bands):
    source = StubSource("element84-stac", [job.scene_from_ref(bands["meta"]["scene"])])
    client, _, _ = build_client(sources=(source,))
    body = client.post("/v1/scene", json=scene_body(), headers=AUTH).json()
    assert body["scene_id"] == bands["meta"]["scene"]["scene_id"]
    assert body["quality"] == "complete"
    assert body["discovery_source"] == "element84-stac"
    assert body["class_table"] == ndvi.CLASS_TABLE
    assert set(body["bands"]) == set(stac.BAND_ASSETS)
    assert body["screened"] == [{"scene_id": body["scene_id"], "usable_pct": 100.0,
                                 "cloud_pct": 29.627669,
                                 "acquired_at": "2026-11-05T07:50:55.647000+00:00"}]
    assert body["window"]["days"] == 45
    assert body["pixels_read"] > 0


def test_a_newer_pass_with_no_surface_over_the_plots_is_not_adopted_and_says_why(screen):
    """The 5.2 screen, over HTTP: rank is a guess until the class layer says otherwise."""
    newer = scene_with_pixels("S2B_37MER_20261110_0_L2A", acquired="2026-11-10T07:51:04+00:00",
                              cloud_pct=24.9)
    older = scene_with_pixels("S2C_37MER_20261105_1_L2A", cloud_pct=29.6)
    screen[newer.bands["scl"]] = 9  # cloud
    screen[older.bands["scl"]] = 4  # vegetation
    client, _, _ = build_client(sources=(StubSource("element84-stac", [newer, older]),))
    body = client.post("/v1/scene", json=scene_body(), headers=AUTH).json()
    assert body["scene_id"] == older.scene_id, "the cleanest-looking pass was all cloud here"
    assert [row["usable_pct"] for row in body["screened"]] == [0.0, 100.0]
    assert [row["scene_id"] for row in body["screened"]] == [newer.scene_id, older.scene_id]
    assert any("cleared" in note for note in body["notes"]), body["notes"]


def test_max_scene_tries_from_the_request_limits_the_walk(screen):
    """One try means the best candidate so far is adopted as partial, not skipped."""
    newer = scene_with_pixels("S2B_newer", acquired="2026-11-10T07:51:04+00:00", cloud_pct=10.0)
    older = scene_with_pixels("S2C_older", acquired="2026-11-05T07:51:04+00:00", cloud_pct=12.0)
    screen[newer.bands["scl"]] = 9
    screen[older.bands["scl"]] = 4
    client, _, _ = build_client(sources=(StubSource("element84-stac", [newer, older]),))
    body = client.post("/v1/scene", json=scene_body(max_scene_tries=1), headers=AUTH).json()
    assert body["scene_id"] == newer.scene_id
    assert body["quality"] == "partial"
    assert len(body["screened"]) == 1
    assert any(f"{job.MIN_COVERAGE_PCT}% usable" in note for note in body["notes"]), body["notes"]


@pytest.mark.parametrize("days", [0, 366, -1])
def test_a_window_outside_the_supported_range_is_refused_before_the_catalog_is_asked(days):
    source = StubSource("element84-stac", [CELL])
    client, _, _ = build_client(sources=(source,))
    response = client.post("/v1/scene", json=scene_body(window_days=days), headers=AUTH)
    assert response.status_code == 400
    assert response.json()["error"] == "bad_request"
    assert source.calls == 0


def test_an_aoi_that_is_not_a_polygon_is_a_bad_request():
    client, _, _ = build_client()
    point = {"type": "Point", "coordinates": [39.836, -3.64]}
    response = client.post("/v1/scene", json=scene_body(aoi_geojson=point), headers=AUTH)
    assert response.status_code == 400
    assert "Polygon" in response.json()["detail"]


def test_a_bare_date_end_time_is_the_whole_day(screen):
    """Midnight would drop that morning's pass, which is the freshest answer."""
    client, _, _ = build_client(sources=(StubSource("element84-stac", [CELL]),))
    body = client.post("/v1/scene", json=scene_body(end_time="2026-11-06", window_days=10),
                       headers=AUTH).json()
    assert body["window"]["end"] == "2026-11-06T23:59:59+00:00"
    assert body["window"]["start"] == "2026-10-27T23:59:59+00:00"


def test_the_cloud_ceiling_a_caller_asks_for_widens_the_ladder_and_is_echoed(screen):
    client, _, _ = build_client(sources=(StubSource("element84-stac", [CELL]),))
    body = client.post("/v1/scene", json=scene_body(max_cloud_pct=55), headers=AUTH).json()
    assert body["window"]["cloud_ceilings"] == [40.0, 55.0]
    assert body["quality"] == "complete", "29.6% is still inside the first rung"


def test_a_candidate_above_the_callers_ceiling_is_refused_not_adopted(screen):
    client, _, _ = build_client(sources=(StubSource("element84-stac", [CELL]),))
    response = client.post("/v1/scene", json=scene_body(max_cloud_pct=20), headers=AUTH)
    assert response.status_code == 404
    assert response.json()["error"] == "unavailable"
    assert any("refused" in note for note in response.json()["notes"]), response.json()["notes"]


def test_a_silent_catalog_is_an_unavailable_with_its_name_in_the_notes_and_in_health():
    silent = (StubSource("copernicus-stac", error="connection refused"),
              StubSource("element84-stac", error="timed out"))
    client, _, _ = build_client(sources=silent)
    response = client.post("/v1/scene", json=scene_body(), headers=AUTH)
    assert response.status_code == 404
    body = response.json()
    assert body["error"] == "unavailable"
    assert any(note.startswith("copernicus-stac:") for note in body["notes"]), body["notes"]
    assert any(note.startswith("element84-stac:") for note in body["notes"]), body["notes"]
    health = client.get("/v1/health").json()
    assert health["discovery_sources_reachable"] == {"copernicus-stac": False,
                                                     "element84-stac": False}
    assert health["status"] == "degraded"


def test_a_catalog_that_answered_is_the_one_health_reports(screen):
    """Failover is the rule, so one quiet catalog is degraded and one loud one is not."""
    sources = (StubSource("copernicus-stac", error="502 from the edge"),
               StubSource("element84-stac", [CELL]))
    client, _, _ = build_client(sources=sources)
    assert client.post("/v1/scene", json=scene_body(), headers=AUTH).status_code == 200
    health = client.get("/v1/health").json()
    assert health["discovery_sources_reachable"] == {"copernicus-stac": False,
                                                     "element84-stac": True}
    assert health["status"] == "ok"


# --- /v1/stats ---------------------------------------------------------------


def test_the_stats_route_returns_the_recorded_figures_for_the_recorded_cell(replay, bands):
    client, _, _ = build_client()
    response = client.post("/v1/stats", json=stats_body(bands), headers=AUTH)
    assert response.status_code == 200
    body, golden = response.json(), bands["meta"]["measure"]
    for key in ("mean_ndvi", "median_ndvi", "valid_pixels", "expected_pixels", "coverage_pct",
                "quality", "scene_id", "class_table", "palette_version", "epsg"):
        assert body[key] == golden[key], key
    assert body["per_cell"][0]["plot_id"] == "plot-1"
    assert body["per_cell"][0]["mean_ndvi"] == golden["mean_ndvi"]
    assert body["weight_by_area"] is True
    assert body["pixels_read"] > 0


def test_no_cells_is_a_400_and_a_project_over_the_ceiling_is_a_400(bands):
    client, _, _ = build_client()
    assert client.post("/v1/stats", json=stats_body(bands, cells=[]),
                       headers=AUTH).status_code == 400
    geometry = bands["meta"]["cell_geometry"]
    crowd = [{"plot_id": f"plot-{n}", "geometry": geometry} for n in range(MAX_CELLS + 1)]
    response = client.post("/v1/stats", json=stats_body(bands, cells=crowd), headers=AUTH)
    assert response.status_code == 400
    assert str(MAX_CELLS) in response.json()["detail"]


def test_a_cell_geometry_that_is_not_a_polygon_is_a_400_naming_the_plot(bands):
    client, _, _ = build_client()
    cells = [{"plot_id": "plot-9", "geometry": {"type": "LineString", "coordinates": [[0, 0]]}}]
    response = client.post("/v1/stats", json=stats_body(bands, cells=cells), headers=AUTH)
    assert response.status_code == 400
    assert "plot-9" in response.json()["detail"]


def test_a_scene_with_no_scl_asset_is_refused_before_the_first_open(bands):
    """CDSE publishes no https COGs, so its granules are discoverable, not readable."""
    scene = dict(bands["meta"]["scene"])
    scene["bands"] = {role: href for role, href in scene["bands"].items() if role != "scl"}
    client, _, _ = build_client()
    response = client.post("/v1/stats", json=stats_body(bands, scene=scene), headers=AUTH)
    assert response.status_code == 404
    body = response.json()
    assert body["error"] == "unavailable"
    assert "scl" in body["detail"]


def test_a_processor_below_the_known_class_table_is_refused(bands):
    """Pre-05.09 granules number the classes differently, so masking one would drop
    the vegetation and keep the cloud."""
    scene = dict(bands["meta"]["scene"], processor_baseline="05.04")
    client, _, _ = build_client()
    response = client.post("/v1/stats", json=stats_body(bands, scene=scene), headers=AUTH)
    assert response.status_code == 404
    assert stac.MIN_CLASS_TABLE_BASELINE in response.json()["detail"]


def test_a_band_url_from_an_unlisted_host_is_a_502_and_never_a_fetch(bands):
    """The caller supplies the scene, so the allow list is the security boundary.

    No replay is installed here on purpose: if this request did reach the read
    path the test would hang on a real host instead of failing cleanly.
    """
    scene = dict(bands["meta"]["scene"])
    scene["bands"] = dict(scene["bands"], red="https://attacker.example.com/B04.tif")
    client, _, _ = build_client()
    response = client.post("/v1/stats", json=stats_body(bands, scene=scene), headers=AUTH)
    assert response.status_code == 502
    body = response.json()
    assert body["error"] == "band_unavailable"
    assert "attacker.example.com" in body["detail"]
    assert "band_unavailable" in client.get("/v1/health").json()["last_error"]


def test_a_body_pydantic_cannot_read_is_a_bad_request_not_fastapis_422_shape(bands):
    client, _, _ = build_client()
    response = client.post("/v1/stats", json={"scene": bands["meta"]["scene"]}, headers=AUTH)
    assert response.status_code == 400
    body = response.json()
    assert body["error"] == "bad_request"
    assert "cells" in body["detail"]
    assert set(body) == {"error", "detail", "notes"}


def test_a_scene_discovered_then_measured_is_the_same_payload_round_trip(replay, bands):
    """The worker keeps no state, so what `/v1/scene` returns must be sendable back
    to `/v1/stats` unchanged. A field the ref could not rebuild would break the
    refresh job rather than this test."""
    source = StubSource("element84-stac", [job.scene_from_ref(bands["meta"]["scene"])])
    client, _, _ = build_client(sources=(source,))
    scene = client.post("/v1/scene", json=scene_body(), headers=AUTH).json()
    body = client.post("/v1/stats", json=stats_body(bands, scene=scene), headers=AUTH).json()
    assert body["scene_id"] == scene["scene_id"]
    assert body["mean_ndvi"] == bands["meta"]["measure"]["mean_ndvi"]
    assert body["epsg"] == 32737


# --- /v1/tiles/render and DELETE /v1/tiles ----------------------------------


@pytest.fixture
def painted(monkeypatch):
    """A tile payload the route can store without reading a granule.

    The pixel-for-pixel render is proven in `test_job.py` against the committed
    PNG; what is under test here is the key shape and the store round trip.
    """
    monkeypatch.setattr(job, "tile_png", lambda scene, z, x, y, budget=None: b"PNGPAYLOAD")


def test_render_writes_scene_scoped_keys_and_purge_removes_them(painted, bands):
    client, _, store = build_client()
    response = client.post("/v1/tiles/render", json=render_body(bands), headers=AUTH)
    assert response.status_code == 200
    body = response.json()
    assert body["quality"] == "complete"
    assert body["tiles_rendered"] == len(store.objects) > 0
    assert body["bytes_written"] == body["tiles_rendered"] * len("PNGPAYLOAD")
    assert body["palette_version"] == palette.PALETTE_VERSION
    for key in store.objects:
        assert key.startswith(f"proj-1/{CELL.scene_id}/14/")
        assert key.endswith(".png")
    purged = client.request("DELETE", "/v1/tiles", json={"key_prefix": body["key_prefix"]},
                            headers=AUTH).json()
    assert purged == {"key_prefix": body["key_prefix"], "purged": body["tiles_rendered"]}
    assert store.objects == {}


def test_a_palette_the_worker_does_not_paint_is_a_409(painted, bands):
    """Rule 7: the legend and the tiles are one contract, not two deployments."""
    client, _, store = build_client()
    response = client.post("/v1/tiles/render",
                           json=render_body(bands, palette_version="ndvi-3band/9"), headers=AUTH)
    assert response.status_code == 409
    assert response.json()["error"] == "palette_mismatch"
    assert store.objects == {}, "a refused palette must not write tiles"


@pytest.mark.parametrize("zooms", [(17, 18), (7, 12), (12, 9)])
def test_a_zoom_the_renderer_will_not_produce_is_a_400(painted, bands, zooms):
    client, _, _ = build_client()
    response = client.post("/v1/tiles/render",
                           json=render_body(bands, zoom_min=zooms[0], zoom_max=zooms[1]),
                           headers=AUTH)
    assert response.status_code == 400
    assert "zoom" in response.json()["detail"]


@pytest.mark.parametrize("bbox", [[39.83, -3.65, 39.85], [39.85, -3.63, 39.83, -3.65],
                                  [170.0, -3.65, 200.0, -3.63]])
def test_a_bbox_that_is_not_an_ordered_wgs84_box_is_a_400(painted, bands, bbox):
    client, _, _ = build_client()
    response = client.post("/v1/tiles/render", json=render_body(bands, bbox=bbox), headers=AUTH)
    assert response.status_code == 400
    assert response.json()["error"] == "bad_request"


@pytest.mark.parametrize("prefix", ["../../etc", "proj-1", "proj-1/scene-1/14",
                                    "proj 1/scene-1", "proj-1/scene-1/../.."])
def test_a_key_prefix_that_is_not_one_project_and_one_scene_is_a_400(painted, bands, prefix):
    """The prefix becomes a path, so its shape is the whole defence.

    This was a 500 `internal`: `store.check_prefix` raises `BadKey` and the
    render route never caught it, which is a validation failure wearing the
    wrong label for the only caller that has to branch on it.
    """
    client, _, store = build_client()
    response = client.post("/v1/tiles/render", json=render_body(bands, key_prefix=prefix),
                           headers=AUTH)
    assert response.status_code == 400
    assert response.json()["error"] == "bad_request"
    assert store.objects == {}


def test_purging_a_prefix_that_is_not_a_project_and_a_scene_is_a_400():
    client, _, _ = build_client()
    response = client.request("DELETE", "/v1/tiles", json={"key_prefix": "nope"}, headers=AUTH)
    assert response.status_code == 400
    assert response.json()["error"] == "bad_request"


def test_a_store_that_cannot_be_written_is_a_503_and_health_says_degraded(painted, bands):
    """Half a render is worse than a failed one, and the operator sees why."""
    client, _, _ = build_client(store=BrokenStore())
    response = client.post("/v1/tiles/render", json=render_body(bands), headers=AUTH)
    assert response.status_code == 503
    assert response.json()["error"] == "store_unavailable"
    health = client.get("/v1/health").json()
    assert health["tile_store_writable"] is False
    assert health["status"] == "degraded"
    purge = client.request("DELETE", "/v1/tiles", json={"key_prefix": "proj-1/scene-1"},
                           headers=AUTH)
    assert purge.status_code == 503
    assert purge.json()["error"] == "store_unavailable"


# --- GET /v1/tiles/{project}/{scene}/{z}/{x}/{y}.png ---------------------------


@pytest.mark.parametrize("key", [
    "p/s/8/0/0.png",
    "p/s/8/255/255.png",
    "proj-1/S2C_37MER_20261105_1_L2A/14/10005/8357.png",
    "project_1.v2/scene:1/16/65535/65535.png",
])
def test_tile_read_returns_exact_png_bytes_and_no_store_headers(key):
    tile_store = Mock(wraps=MemoryStore())
    payload = b"\x89PNG\r\n\x1a\n" + bytes(range(256))
    tile_store.put(key, payload)
    client, _, _ = build_client(store=tile_store)
    response = client.get(f"/v1/tiles/{key}", headers=AUTH, follow_redirects=False)
    assert response.status_code == 200
    assert response.content == payload
    assert response.headers["content-type"] == "image/png"
    assert response.headers["cache-control"] == "no-store"
    assert "location" not in response.headers
    tile_store.get.assert_called_once_with(key)


@pytest.mark.parametrize("token,headers", [
    (TOKEN, {}),
    (TOKEN, {"Authorization": "Bearer wrong-token"}),
    ("", AUTH),
    ("", {"Authorization": "Bearer "}),
])
@pytest.mark.parametrize("key", ["p/s/14/1/1.png", "p/s/17/1/1.png", "p%20bad/s/14/1/1.png"])
def test_tile_read_authentication_precedes_validation_and_all_store_access(token, headers, key):
    tile_store = Mock(wraps=MemoryStore())
    client, _, _ = build_client(store=tile_store, token=token)
    response = client.get(f"/v1/tiles/{key}", headers=headers, follow_redirects=False)
    assert response.status_code == 401
    assert response.json() == {"error": "unauthorized", "detail": "missing or wrong bearer token",
                               "notes": []}
    assert "location" not in response.headers
    assert tile_store.mock_calls == []


def test_missing_tile_returns_typed_404():
    client, _, _ = build_client()
    response = client.get("/v1/tiles/p/missing/14/1/1.png", headers=AUTH)
    assert response.status_code == 404
    assert response.json() == {"error": "unavailable", "detail": "tile not found", "notes": []}


def test_tile_store_failure_returns_503_and_is_recorded_by_worker():
    client, worker, _ = build_client(store=BrokenStore("tile could not be read"))
    response = client.get("/v1/tiles/p/s/14/1/1.png", headers=AUTH)
    assert response.status_code == 503
    assert response.json() == {"error": "store_unavailable", "detail": "tile could not be read",
                               "notes": []}
    assert "store_unavailable: tile could not be read" in worker.last_error


def test_tile_read_volume_failure_does_not_expose_filesystem_path(tmp_path):
    root = tmp_path / "private-root"
    root.write_bytes(b"not a directory")
    client, worker, _ = build_client(store=store_module.VolumeStore(root))
    response = client.get("/v1/tiles/p/s/14/1/1.png", headers=AUTH)
    assert response.status_code == 503
    assert response.json()["error"] == "store_unavailable"
    assert str(root) not in response.text
    assert str(root) not in worker.last_error


def test_tile_read_handles_bad_key_from_store():
    tile_store = Mock(wraps=MemoryStore())
    tile_store.get.side_effect = store_module.BadKey("tile path escapes store root")
    client, _, _ = build_client(store=tile_store)
    response = client.get("/v1/tiles/p/s/14/1/1.png", headers=AUTH)
    assert response.status_code == 400
    assert response.json()["error"] == "bad_request"


@pytest.mark.parametrize("key", [
    "p/s/7/0/0.png", "p/s/17/0/0.png", "p/s/-1/0/0.png", "p/s/999999/0/0.png",
    "p/s/8/-1/0.png", "p/s/8/0/-1.png", "p/s/8/256/0.png", "p/s/8/0/256.png",
    "p/s/16/65536/0.png", "p/s/16/0/65536.png",
    "p/s/zoom/0/0.png", "p/s/14/x/0.png", "p/s/14/0/y.png", "p/s/14/1.5/0.png",
    "p%20bad/s/14/0/0.png", "p/s%20bad/14/0/0.png",
    "p%0a/s/14/0/0.png", "p/s%0a/14/0/0.png",
    "p" * 65 + "/s/14/0/0.png", "p/" + "s" * 129 + "/14/0/0.png",
    "%2e%2e/s/14/0/0.png", "p/%2e%2e/14/0/0.png",
    "p%5cother/s/14/0/0.png", "p/s%5cother/14/0/0.png",
    "p/s/14/0/0.png%0a",
])
def test_invalid_tile_keys_and_coordinates_are_400_before_store_access(key):
    tile_store = Mock(wraps=MemoryStore())
    client, _, _ = build_client(store=tile_store)
    response = client.get(f"/v1/tiles/{key}", headers=AUTH, follow_redirects=False)
    assert response.status_code == 400
    assert response.json()["error"] == "bad_request"
    assert tile_store.mock_calls == []


@pytest.mark.parametrize("key", [
    "p%2fother/s/14/0/0.png", "p/s%2fother/14/0/0.png",
    "p/%2e%2e%2fsecret/14/0/0.png", "%2fetc/passwd",
    "p/s/14/0/0.jpg", "p/s/14/0/0.png/extra", "p/%252f/14/0/0.png",
])
def test_paths_outside_tile_route_never_access_store_or_redirect(key):
    # Decoded slashes create extra segments, so these do not match a tile route.
    tile_store = Mock(wraps=MemoryStore())
    client, _, _ = build_client(store=tile_store)
    response = client.get(f"/v1/tiles/{key}", headers=AUTH, follow_redirects=False)
    assert response.status_code == 404
    assert "location" not in response.headers
    assert tile_store.mock_calls == []


# --- the cap, and the shape of every failure ----------------------------------


def test_two_jobs_at_the_cap_and_a_third_is_told_busy(bands):
    client, worker, _ = build_client()
    worker.queue_wait_s = 0.05
    assert worker.slot.acquire()
    try:
        response = client.post("/v1/stats", json=stats_body(bands), headers=AUTH)
    finally:
        worker.slot.release()
    assert response.status_code == 503
    body = response.json()
    assert body["error"] == "busy"
    assert "1 job(s) already running" in body["detail"]


@pytest.mark.parametrize("path", ["/v1/scene", "/v1/stats", "/v1/tiles/render"])
def test_every_failure_carries_the_same_three_fields(path):
    client, _, _ = build_client()
    response = client.post(path, json={}, headers=AUTH)
    body = response.json()
    assert set(body) == {"error", "detail", "notes"}
    assert body["error"] in {"unauthorized", "bad_request", "unavailable"}, body
    assert isinstance(body["notes"], list)


def test_health_reports_the_versions_limits_and_hosts_the_api_checks():
    client, _, _ = build_client()
    body = client.get("/v1/health").json()
    assert body["status"] == "ok"
    assert body["tile_store_backend"] == "memory"
    assert body["discovery_sources_reachable"] is None, "no job has run yet"
    assert body["class_table"] == ndvi.CLASS_TABLE
    assert body["palette_version"] == palette.PALETTE_VERSION
    assert body["band_hosts"] == list(cogs.ALLOWED_HOSTS)
    assert body["limits"]["max_concurrency"] == 1
    assert set(body["limits"]) == {"max_concurrency", "max_pixels_per_job", "job_timeout_s",
                                   "min_coverage_pct", "max_scene_tries", "max_tiles_per_job"}
