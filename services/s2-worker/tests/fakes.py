"""The stand-ins the route tests need: a tile store in memory, a catalog that
answers from a list, and a store that fails on purpose.

These live here rather than in one test file because both `test_job.py` and
`test_app.py` need them, and a fake that exists twice drifts into two shapes
that each prove a different thing.
"""

from __future__ import annotations

import datetime as dt
from typing import Sequence

from s2_worker import stac, store as store_module


class MemoryStore:
    """The tile store contract with no disk and no bucket."""

    name = "memory"

    def __init__(self):
        self.objects: dict[str, bytes] = {}

    def put(self, key, payload):
        store_module.check_tile_key(key)
        self.objects[key] = payload

    def delete_prefix(self, prefix):
        purged = sum(1 for key in self.objects if key.startswith(f"{prefix}/"))
        self.objects = {k: v for k, v in self.objects.items() if not k.startswith(f"{prefix}/")}
        return purged

    def probe(self):
        return ""


class BrokenStore:
    """A volume that is read-only or a bucket that went quiet.

    `probe` returns text on purpose: `/v1/health` reports the store as unwritable
    from that string, so a test that only exercised `put` would miss half of it.
    """

    name = "memory-broken"

    def __init__(self, message="no space left on device"):
        self.message = message

    def put(self, key, payload):
        raise store_module.StoreUnavailable(self.message)

    def delete_prefix(self, prefix):
        raise store_module.StoreUnavailable(self.message)

    def probe(self):
        return self.message


class StubSource:
    """One catalog whose answer is a list, or an outage.

    `error` makes it raise exactly what the real `StacSource` raises when the
    host is unreachable, so the notes the route records are production notes.
    """

    timeout = 5.0

    def __init__(self, name: str, scenes: Sequence[stac.Scene] = (), error: str = "",
                 pixels: bool = True, url: str = "https://catalog.invalid/v1"):
        self.name = name
        self.pixels = pixels
        self.url = url
        self.scenes = list(scenes)
        self.error = error
        self.calls = 0

    def candidates(self, geometry, start, end, limit=50):
        self.calls += 1
        if self.error:
            raise stac.DiscoveryError(f"{self.url}: {self.error}")
        return list(self.scenes)


def scene_with_pixels(scene_id: str, acquired: str = "2026-11-05T07:50:55+00:00",
                      cloud_pct: float = 29.6, **overrides) -> stac.Scene:
    """A candidate the pool cannot refuse: three https roles, a known baseline.

    The scene id is in the band paths because a test that screens two
    candidates has to be able to hand each one different pixels.
    """
    fields = dict(
        scene_id=scene_id,
        acquired_at=dt.datetime.fromisoformat(acquired).replace(tzinfo=dt.timezone.utc),
        cloud_pct=cloud_pct,
        platform="sentinel-2c",
        epsg=32737,
        orbit="R049",
        tile="37MER",
        baseline="05.13",
        discovery_source="element84-stac",
        bands={role: f"https://sentinel-cogs.s3.us-west-2.amazonaws.com/{scene_id}/{role}.tif"
               for role in stac.BAND_ASSETS},
        footprint=None,
    )
    fields.update(overrides)
    return stac.Scene(**fields)
