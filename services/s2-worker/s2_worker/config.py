"""Environment-driven settings. One place, so `fly secrets list` is the whole config story."""

from __future__ import annotations

import os
from dataclasses import dataclass

from . import cogs, job, stac


def _float(name: str, default: float) -> float:
    raw = os.environ.get(name, "")
    try:
        return float(raw) if raw else default
    except ValueError:
        return default


def _int(name: str, default: int) -> int:
    raw = os.environ.get(name, "")
    try:
        return int(raw) if raw else default
    except ValueError:
        return default


def _hosts(raw: str) -> tuple[str, ...]:
    return tuple(host.strip() for host in raw.split(",") if host.strip())


@dataclass
class Settings:
    """Everything the worker needs to answer a request.

    There is no default token on purpose: a deployment that forgets the secret
    gets a service that refuses every call rather than one that anyone can use.
    """

    token: str = ""
    primary_url: str = stac.CDSE.url
    fallback_url: str = stac.ELEMENT84.url
    allowed_hosts: tuple[str, ...] = cogs.ALLOWED_HOSTS
    max_pixels_per_job: int = cogs.MAX_PIXELS_PER_JOB
    job_timeout_s: float = cogs.JOB_TIMEOUT_S
    max_concurrency: int = 2
    min_coverage_pct: float = job.MIN_COVERAGE_PCT
    max_scene_tries: int = job.MAX_SCENE_TRIES
    max_tiles_per_job: int = job.MAX_TILES_PER_JOB
    tile_store_backend: str = "volume"
    tile_store_bucket: str = ""
    tile_store_endpoint: str = ""
    tile_store_access_key_id: str = ""
    tile_store_secret_access_key: str = ""
    tile_store_root: str = "/data/tiles"

    @classmethod
    def from_env(cls) -> "Settings":
        return cls(
            token=os.environ.get("S2_WORKER_TOKEN", ""),
            primary_url=os.environ.get("STAC_PRIMARY_URL", stac.CDSE.url),
            fallback_url=os.environ.get("STAC_FALLBACK_URL", stac.ELEMENT84.url),
            allowed_hosts=_hosts(os.environ.get("COG_ALLOWED_HOSTS", "")) or cogs.ALLOWED_HOSTS,
            max_pixels_per_job=_int("MAX_PIXELS_PER_JOB", cogs.MAX_PIXELS_PER_JOB),
            job_timeout_s=_float("JOB_TIMEOUT_S", cogs.JOB_TIMEOUT_S),
            max_concurrency=max(1, _int("MAX_CONCURRENCY", 2)),
            min_coverage_pct=_float("MIN_COVERAGE_PCT", job.MIN_COVERAGE_PCT),
            max_scene_tries=max(1, _int("MAX_SCENE_TRIES", job.MAX_SCENE_TRIES)),
            max_tiles_per_job=max(1, _int("MAX_TILES_PER_JOB", job.MAX_TILES_PER_JOB)),
            tile_store_backend=os.environ.get("TILE_STORE_BACKEND", "volume"),
            tile_store_bucket=os.environ.get("TILE_STORE_BUCKET", ""),
            tile_store_endpoint=os.environ.get("TILE_STORE_ENDPOINT", ""),
            tile_store_access_key_id=os.environ.get("TILE_STORE_ACCESS_KEY_ID", ""),
            tile_store_secret_access_key=os.environ.get("TILE_STORE_SECRET_ACCESS_KEY", ""),
            tile_store_root=os.environ.get("TILE_STORE_ROOT", "/data/tiles"),
        )

    def apply(self) -> "Settings":
        """Push the band allow list into the read path.

        The list is the one thing standing between a caller-supplied scene
        payload and the worker fetching an arbitrary URL, so it has to be the
        deployment's value and not the module's convenience default.
        """
        cogs.ALLOWED_HOSTS = self.allowed_hosts
        return self

    def sources(self) -> tuple[stac.StacSource, ...]:
        """CDSE first, then earth-search, with the URLs this deployment was given.

        `pixels` travels with the source rather than the URL: only a catalog that
        publishes https band assets can hand the worker something to read, and a
        re-pointed primary URL must not silently inherit that assumption.
        """
        return (
            stac.StacSource(stac.CDSE.name, self.primary_url, pixels=False),
            stac.StacSource(stac.ELEMENT84.name, self.fallback_url, pixels=True),
        )

    def new_budget(self) -> cogs.ReadBudget:
        return cogs.ReadBudget(max_pixels=self.max_pixels_per_job,
                               max_seconds=self.job_timeout_s)
