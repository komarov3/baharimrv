"""Where rendered tiles live: a mounted volume or an S3-compatible bucket.

Keys arrive from the API, which composes them from a project id and a scene id.
Both end up in a filesystem path or an object key, so every key is validated
against one shape here rather than trusted upstream. A tile key that could carry
`..` would let a render call write anywhere the container can reach.
"""

from __future__ import annotations

import os
import pathlib
import re
import tempfile
from contextlib import closing

# {project_id}/{scene_id}/{z}/{x}/{y}.png  -- see SATELLITE_NDVI_SPEC.md 8 rule 7.
# The zoom group is the spec's 8-16 clamp written into the key shape, so a key
# cannot smuggle past the route a zoom the renderer would refuse to produce.
TILE_KEY = re.compile(
    r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}"
    r"/[A-Za-z0-9][A-Za-z0-9._:-]{0,127}"
    r"/(?:[89]|1[0-6])/\d{1,9}/\d{1,9}\.png$")

# A prefix is a key with the tile coordinates removed.
KEY_PREFIX = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}"
                        r"/[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$")

CONTENT_TYPE = "image/png"
CACHE_CONTROL = "public, max-age=31536000, immutable"
MAX_TILE_BYTES = 1 << 20


class BadKey(ValueError):
    """The caller asked for a path this store will not access."""


class TileNotFound(Exception):
    """The requested tile does not exist."""


class StoreUnavailable(RuntimeError):
    """The tile store could not be reached, read or written."""


def check_tile_key(key: str) -> str:
    if not TILE_KEY.fullmatch(key):
        raise BadKey(f"{key!r} is not a {{project}}/{{scene}}/{{z}}/{{x}}/{{y}}.png key")
    return key


def check_prefix(prefix: str) -> str:
    if not KEY_PREFIX.fullmatch(prefix):
        raise BadKey(f"{prefix!r} is not a {{project}}/{{scene}} prefix")
    return prefix


class VolumeStore:
    """A directory on a mounted Fly volume.

    Simple, and coupled to one machine: if the worker ever runs on more than one
    instance the tiles have to move to the object-store backend.
    """

    name = "volume"

    def __init__(self, root: str | pathlib.Path):
        self.root = pathlib.Path(root)

    def put(self, key: str, payload: bytes) -> None:
        check_tile_key(key)
        path = self.root / key
        try:
            path.parent.mkdir(parents=True, exist_ok=True)
            # Write-then-rename, so a reader that arrives mid-render gets the
            # previous complete file rather than half a PNG. The temporary name
            # is unique because two jobs can be asked for the same tile at once.
            handle, temp_name = tempfile.mkstemp(dir=path.parent, suffix=".png.tmp")
            try:
                with os.fdopen(handle, "wb") as temp:
                    temp.write(payload)
                os.replace(temp_name, path)
            finally:
                if os.path.exists(temp_name):
                    os.unlink(temp_name)
        except OSError as exc:
            raise StoreUnavailable(f"{path}: {exc}") from exc

    def get(self, key: str) -> bytes:
        check_tile_key(key)
        try:
            root = self.root.resolve()
            path = (root / key).resolve()
            if not path.is_relative_to(root):
                raise BadKey("tile path escapes store root")
            with path.open("rb") as tile:
                payload = tile.read(MAX_TILE_BYTES + 1)
        except FileNotFoundError as exc:
            raise TileNotFound("tile not found") from exc
        except (OSError, RuntimeError) as exc:
            # resolve() can raise RuntimeError for a symlink loop on Python 3.12.
            raise StoreUnavailable("tile could not be read") from exc
        if len(payload) > MAX_TILE_BYTES:
            raise StoreUnavailable("tile exceeds size limit")
        return payload

    def delete_prefix(self, prefix: str) -> int:
        check_prefix(prefix)
        directory = self.root / prefix
        if not directory.is_dir():
            return 0
        purged = 0
        for path in sorted(directory.rglob("*.png")):
            path.unlink(missing_ok=True)
            purged += 1
        # Deepest first, the prefix directory itself last, so a purged scene
        # leaves nothing behind. An empty directory is the one thing a later
        # "has this scene been rendered?" check would misread as yes.
        emptied = [directory, *(p for p in directory.rglob("*") if p.is_dir())]
        for path in sorted(emptied, reverse=True):
            try:
                path.rmdir()
            except OSError:
                pass  # a job refilled it mid-purge; its tiles carry the old scene id anyway
        return purged

    def probe(self) -> str:
        """Write one object and remove it. Returns "" when the store is usable."""
        try:
            self.root.mkdir(parents=True, exist_ok=True)
            marker = self.root / ".health"
            marker.write_bytes(b"ok")
            marker.unlink()
        except OSError as exc:
            return str(exc)
        return ""


class S3Store:
    """Cloudflare R2, or any S3-compatible endpoint, through boto3.

    R2 is the intended backend: no egress charge, and tiles are the one artifact
    the Android client pulls over and over.
    """

    name = "s3"

    def __init__(self, bucket: str, endpoint: str = "", access_key_id: str = "",
                 secret_access_key: str = "", region: str = "auto"):
        if not bucket:
            raise StoreUnavailable("TILE_STORE_BUCKET is empty")
        import boto3  # imported here so the volume path needs no AWS SDK installed

        self.bucket = bucket
        self.client = boto3.client(
            "s3",
            endpoint_url=endpoint or None,
            aws_access_key_id=access_key_id or None,
            aws_secret_access_key=secret_access_key or None,
            region_name=region or None,
        )

    def put(self, key: str, payload: bytes) -> None:
        check_tile_key(key)
        try:
            self.client.put_object(Bucket=self.bucket, Key=key, Body=payload,
                                   ContentType=CONTENT_TYPE, CacheControl=CACHE_CONTROL)
        except Exception as exc:  # boto3 raises a dozen unrelated exception types
            raise StoreUnavailable(f"{self.bucket}/{key}: {exc}") from exc

    def get(self, key: str) -> bytes:
        check_tile_key(key)
        from botocore.exceptions import ClientError

        try:
            response = self.client.get_object(Bucket=self.bucket, Key=key)
        except ClientError as exc:
            if exc.response.get("Error", {}).get("Code") in {"NoSuchKey", "NotFound", "404"}:
                raise TileNotFound("tile not found") from exc
            raise StoreUnavailable("tile could not be read") from exc
        except Exception as exc:
            raise StoreUnavailable("tile could not be read") from exc
        try:
            with closing(response["Body"]) as body:
                payload = body.read(MAX_TILE_BYTES + 1)
        except Exception as exc:
            raise StoreUnavailable("tile could not be read") from exc
        if len(payload) > MAX_TILE_BYTES:
            raise StoreUnavailable("tile exceeds size limit")
        return payload

    def delete_prefix(self, prefix: str) -> int:
        check_prefix(prefix)
        purged = 0
        token = None
        try:
            while True:
                response = self.client.list_objects_v2(
                    Bucket=self.bucket, Prefix=f"{prefix}/",
                    **({"ContinuationToken": token} if token else {}))
                keys = [{"Key": item["Key"]} for item in response.get("Contents", [])]
                if keys:
                    self.client.delete_objects(Bucket=self.bucket,
                                               Delete={"Objects": keys, "Quiet": True})
                    purged += len(keys)
                if not response.get("IsTruncated"):
                    return purged
                token = response.get("NextContinuationToken")
        except Exception as exc:
            raise StoreUnavailable(f"{self.bucket}/{prefix}: {exc}") from exc

    def probe(self) -> str:
        try:
            self.client.head_bucket(Bucket=self.bucket)
        except Exception as exc:
            return str(exc)
        return ""


def from_settings(settings) -> object:
    """Build the store this deployment asked for."""
    backend = settings.tile_store_backend
    if backend in ("r2", "s3"):
        return S3Store(settings.tile_store_bucket, settings.tile_store_endpoint,
                       settings.tile_store_access_key_id,
                       settings.tile_store_secret_access_key)
    if backend == "volume":
        return VolumeStore(settings.tile_store_root)
    raise StoreUnavailable(f"unknown TILE_STORE_BACKEND {backend!r}")
