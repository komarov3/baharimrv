"""Record the route bodies the Go provider seam is tested against.

    python tests/record_route_fixtures.py

`backend/internal/satellite` decodes these responses, and a Go test written against a
hand-typed guess of the JSON proves only that the guess matched. These files come out
of the same `job.py` functions the routes call, applied to the recording
`tests/capture_fixtures.py` made of the Kilifi granule, so the contract is checked
against the service rather than against a description of it.

The three bodies are the success shapes of `POST /v1/scene`, `POST /v1/stats` and
`POST /v1/tiles/render`. Errors are not recorded here: the Go side matches on the
`error` code alone, and `tests/test_app.py` already pins that envelope.
"""

from __future__ import annotations

import datetime as dt
import json
import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))

import numpy as np  # noqa: E402

from s2_worker import job, stac  # noqa: E402

REPO = pathlib.Path(__file__).resolve().parents[3]
FIXTURES = REPO / "services" / "s2-worker" / "tests" / "fixtures"
OUT = REPO / "backend" / "internal" / "satellite" / "testdata"

# The spike asked for a 45 day window ending after the 2026-11-05 sensing time.
WINDOW_END = dt.datetime(2026, 11, 5, 12, 0, tzinfo=dt.timezone.utc)
WINDOW_DAYS = 45

# A project id that is not real, because these fixtures are read by a Go test that
# only compares shapes. The scene id and the prefix rule are the real parts.
PROJECT_ID = "5f0c8fa4-6f3c-4f2c-9b41-7d8e0c1a2b3c"


def recorded() -> dict:
    meta = json.loads(str(np.load(FIXTURES / "bands.npz", allow_pickle=True)["meta"][0]))
    meta["_tile_png_bytes"] = (FIXTURES / "tile_14_10005_8357.png").stat().st_size
    return meta


def scene_body(meta: dict) -> dict:
    start, end = job.window_for(WINDOW_END, WINDOW_DAYS)
    return {
        **meta["scene"],
        "notes": meta["choose_notes"],
        "window": {"start": start.isoformat(), "end": end.isoformat(),
                   "days": WINDOW_DAYS, "cloud_ceilings": list(stac.CLOUD_CEILINGS)},
        "screened": meta["screened"],
        "pixels_read": meta["pixels"]["replay"],
    }


def stats_body(meta: dict) -> dict:
    return {**meta["measure"], "pixels_read": meta["pixels"]["measure"], "weight_by_area": True}


def render_body(meta: dict) -> dict:
    """The one z14 tile the spike rendered, in the shape `/v1/tiles/render` answers.

    `tiles_rendered` and the byte count come from the recording; the plan it was the
    only member of is the tile position `meta["tile"]` stores.
    """
    tile = meta["tile"]
    return {
        "scene_id": meta["scene"]["scene_id"],
        "key_prefix": f"{PROJECT_ID}/{meta['scene']['scene_id']}",
        "palette_version": meta["scene"]["palette_version"],
        "class_table": meta["scene"]["class_table"],
        "quality": "complete",
        "tiles_rendered": 1,
        "tiles_empty": 0,
        "tiles_omitted": 0,
        "bytes_written": meta["_tile_png_bytes"],
        "pixels_read": meta["pixels"]["tile"],
        "zoom_min": tile["z"],
        "zoom_max": tile["z"],
        "notes": [],
    }


def main() -> int:
    meta = recorded()
    OUT.mkdir(parents=True, exist_ok=True)
    for name, body in (("scene.json", scene_body(meta)), ("stats.json", stats_body(meta)),
                       ("render.json", render_body(meta))):
        (OUT / name).write_text(json.dumps(body, indent=2, default=str) + "\n")
        print(f"wrote {OUT / name}")
    print(f"scene {meta['scene']['scene_id']}, mean {meta['measure']['mean_ndvi']}, "
          f"coverage {meta['measure']['coverage_pct']}%")
    return 0


if __name__ == "__main__":
    sys.exit(main())
