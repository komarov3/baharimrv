"""Scene discovery against two anonymous STAC catalogs.

CDSE first, then earth-search. Both answers are recorded, because the two
catalogs differ in what provenance they expose: a CDSE granule name encodes the
relative orbit and the processing baseline, an earth-search item does not. Only
earth-search serves band COGs over plain HTTPS, so discovery and pixel source
are deliberately separate concerns. See SATELLITE_NDVI_SPEC.md 3 and 5.2.
"""

from __future__ import annotations

import datetime as dt
import re
from dataclasses import dataclass, field
from typing import Sequence

import requests

COLLECTION = "sentinel-2-l2a"
USER_AGENT = "baharimrv-s2-worker/0.1 (research; +https://baharilink.co.ke)"

# A CDSE granule looks like
# S2B_MSIL2A_20261110T073039_N0513_R049_T37MER_20261110T095522
GRANULE = re.compile(r"_N(?P<baseline>[0-9]{4})_R(?P<orbit>[0-9]{3})_(?P<tile>T[0-9]{2}[A-Z]{3})")

BAND_ASSETS = ("red", "nir", "scl")


class DiscoveryError(RuntimeError):
    """A catalog was unreachable or answered in a shape we cannot read."""


@dataclass
class Scene:
    scene_id: str
    acquired_at: dt.datetime
    cloud_pct: float | None
    platform: str
    epsg: int | None
    orbit: str = ""
    tile: str = ""
    baseline: str = ""
    discovery_source: str = ""
    # https COG hrefs keyed by band role, empty when this source has no pixels
    bands: dict[str, str] = field(default_factory=dict)
    footprint: dict | None = None

    def has_pixels(self) -> bool:
        return all(role in self.bands for role in BAND_ASSETS)


def _as_utc(value: str) -> dt.datetime:
    parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    return parsed.astimezone(dt.timezone.utc) if parsed.tzinfo else parsed.replace(tzinfo=dt.timezone.utc)


def _baseline(raw: str) -> str:
    """Processor baseline as 05.13, or an empty string when a catalog is silent.

    This is not decoration: the class layer is numbered differently either side
    of 05.09, so the mask is only correct once the baseline is known.
    """
    digits = re.sub(r"\D", "", raw or "")
    return f"{digits[:2]}.{digits[2:]}" if len(digits) == 4 else ""


def _search(url: str, body: dict, timeout: float) -> list[dict]:
    try:
        response = requests.post(
            f"{url.rstrip('/')}/search",
            json=body,
            headers={"Content-Type": "application/json", "User-Agent": USER_AGENT},
            timeout=timeout,
        )
    except requests.RequestException as exc:  # network, TLS, DNS, timeout
        raise DiscoveryError(f"{url}: {exc}") from exc
    if response.status_code != 200:
        # Both catalogs word their validation errors differently, and CDSE's
        # geometry errors name a geometry type that was never sent, so the body
        # has to travel with the failure for an operator to have anything to read.
        detail = response.text[:300].replace("\n", " ")
        raise DiscoveryError(f"{url}: HTTP {response.status_code} {detail}")
    try:
        return response.json().get("features", [])
    except ValueError as exc:
        raise DiscoveryError(f"{url}: unreadable JSON body") from exc


class StacSource:
    """One catalog. `pixels` says whether its items can also serve band bytes."""

    def __init__(self, name: str, url: str, timeout: float = 20.0, pixels: bool = False):
        self.name = name
        self.url = url
        self.timeout = timeout
        self.pixels = pixels

    def candidates(self, geometry: dict, start: dt.datetime, end: dt.datetime,
                   limit: int = 50) -> list[Scene]:
        body = {
            "collections": [COLLECTION],
            "intersects": geometry,
            "datetime": f"{start.strftime('%Y-%m-%dT%H:%M:%SZ')}/{end.strftime('%Y-%m-%dT%H:%M:%SZ')}",
            "sort": [{"by": "datetime", "direction": "desc"}],
            "limit": limit,
        }
        return [self._to_scene(item) for item in _search(self.url, body, self.timeout)
                if item.get("properties") and item.get("id")]

    def _to_scene(self, item: dict) -> Scene:
        props = item["properties"]
        cloud = props.get("eo:cloud_cover")
        orbit = tile = ""
        # The granule identifier is the only place CDSE publishes orbit and
        # processing baseline, and only for its own naming scheme.
        match = GRANULE.search(item.get("id", ""))
        baseline = _baseline(match["baseline"]) if match else ""
        if match:
            orbit = f"R{match['orbit']}"
            tile = match["tile"]
        if not baseline:
            baseline = _baseline(props.get("s2:processing_baseline") or "")
        if not tile:
            code = props.get("grid:code") or ""
            tile = (props.get("s2:tileid") or props.get("mgrs:tileid")
                    or (code.split("-", 1)[1] if "-" in code else ""))
        if not orbit:
            relative = props.get("s2:relativeOrbitNumber")
            orbit = f"R{int(relative):03d}" if relative is not None else ""
        bands = {}
        if self.pixels:
            assets = item.get("assets", {})
            for role, key in (("red", "red"), ("nir", "nir"), ("scl", "scl")):
                href = (assets.get(key) or {}).get("href", "")
                if href.startswith("http"):
                    bands[role] = href
        return Scene(
            scene_id=item["id"],
            acquired_at=_as_utc(props["datetime"]),
            cloud_pct=float(cloud) if cloud is not None else None,
            platform=props.get("platform", ""),
            epsg=props.get("proj:epsg"),
            orbit=orbit,
            tile=tile,
            baseline=baseline,
            discovery_source=self.name,
            bands=bands,
            footprint=item.get("geometry"),
        )


CDSE = StacSource("copernicus-stac", "https://stac.dataspace.copernicus.eu/v1", pixels=False)
ELEMENT84 = StacSource("element84-stac", "https://earth-search.aws.element84.com/v1", pixels=True)

# The ladder from 5.2: prefer a clean scene, accept a hazier one and mark the
# result cloudy rather than silently switching to a worse product. Above the
# top rung nothing is used.
CLOUD_CEILINGS = (40.0, 70.0)

# ndvi.USABLE_CLASSES describes the classifier introduced with this baseline.
# Granules below it are refused rather than masked with a table that would drop
# the vegetation and keep the cloud.
MIN_CLASS_TABLE_BASELINE = "05.09"


def table_supported(scene: Scene) -> bool:
    return bool(scene.baseline) and scene.baseline >= MIN_CLASS_TABLE_BASELINE


def cloud_rung(scene: Scene, ceilings: Sequence[float] = CLOUD_CEILINGS) -> int:
    """Which ceiling the scene satisfies, or len(ceilings) when none of them do.

    A scene with no published cloud figure has not been screened at all, so it
    ranks below every screened candidate instead of being skipped: it may still
    be the only pass over the plot, and the note says so.
    """
    if scene.cloud_pct is None:
        return len(ceilings)
    for index, ceiling in enumerate(ceilings):
        if scene.cloud_pct <= ceiling:
            return index
    return len(ceilings)


def order_candidates(scenes: Sequence[Scene],
                    ceilings: Sequence[float] = CLOUD_CEILINGS) -> list[Scene]:
    """Cleanest cloud tier first, then recency."""
    return sorted(scenes, key=lambda s: (cloud_rung(s, ceilings), -s.acquired_at.timestamp()))


def candidate_pool(sources: Sequence[StacSource], geometry: dict, start: dt.datetime,
                   end: dt.datetime, require_pixels: bool = True,
                   ceilings: Sequence[float] = CLOUD_CEILINGS) -> tuple[list[Scene], list[str]]:
    """Every scene the catalogs will hand us for this AOI and window, deduped.

    The notes carry each catalog's refusals, so an operator sees which source
    went quiet instead of reading one swallowed exception.
    """
    notes: list[str] = []
    pool: dict[str, Scene] = {}
    top = max(ceilings)
    for source in sources:
        try:
            scenes = source.candidates(geometry, start, end)
        except DiscoveryError as exc:
            notes.append(f"{source.name}: {exc}")
            continue
        kept = [scene for scene in scenes
                if (not require_pixels or scene.has_pixels())
                and table_supported(scene)
                and (scene.cloud_pct is None or scene.cloud_pct <= top)]
        if scenes and len(kept) != len(scenes):
            notes.append(f"{source.name}: {len(scenes)} items, {len(scenes) - len(kept)} refused "
                         f"for no https COGs, cloud above {top:.0f}%, or a processor below "
                         f"{MIN_CLASS_TABLE_BASELINE}")
        for scene in kept:
            pool.setdefault(scene.scene_id, scene)
    return list(pool.values()), notes
