"""The tile store: key shape first, bytes second."""

from __future__ import annotations

import io
import pathlib
from contextlib import closing
from unittest.mock import Mock

import boto3
import pytest
from botocore.exceptions import ClientError
from botocore.response import StreamingBody

from s2_worker import config, store


class FakeS3Client:
    """Boto3's tile surface, including bounded reads and paginated deletion."""

    def __init__(self):
        self.objects: dict[str, bytes] = {}
        self.list_calls = 0
        self.get_calls = []
        self.get_error = None
        self.body = None

    def put_object(self, Bucket, Key, Body, ContentType, CacheControl):
        self.objects[Key] = Body

    def get_object(self, Bucket, Key):
        self.get_calls.append((Bucket, Key))
        if self.get_error is not None:
            raise self.get_error
        if Key not in self.objects:
            raise ClientError({"Error": {"Code": "NoSuchKey"}}, "GetObject")
        payload = self.objects[Key]
        self.stream = io.BytesIO(payload)
        self.body = Mock(wraps=StreamingBody(self.stream, len(payload)))
        return {"Body": self.body}

    def list_objects_v2(self, Bucket, Prefix, ContinuationToken=None):
        self.list_calls += 1
        keys = [key for key in self.objects if key.startswith(Prefix)]
        page, token = keys[:1], (keys[1:] or [None])[0]
        return {"Contents": [{"Key": key} for key in page],
                "IsTruncated": token is not None, "NextContinuationToken": token}

    def delete_objects(self, Bucket, Delete):
        for item in Delete["Objects"]:
            self.objects.pop(item["Key"], None)
        return {"Deleted": Delete["Objects"]}

    def head_bucket(self, Bucket):
        return {}


@pytest.fixture(autouse=True)
def fake_s3_client(monkeypatch):
    """No S3 test may contact a bucket or the credential metadata service."""
    client = FakeS3Client()
    monkeypatch.setattr(boto3, "client", lambda *args, **kwargs: client)
    return client


@pytest.fixture
def s3():
    return store.S3Store("tiles", "https://account.r2.cloudflarestorage.com")


@pytest.fixture
def volume(tmp_path):
    return store.VolumeStore(tmp_path / "tiles")


@pytest.mark.parametrize("key", [
    "proj-1/S2C_37MER_20261105_1_L2A/14/10005/8357.png",
    "a/b/8/0/0.png",
    "a/b/16/65535/65535.png",
])
def test_valid_tile_keys_pass(key):
    assert store.check_tile_key(key) == key


@pytest.mark.parametrize("key", [
    "../../etc/passwd.png",
    "proj-1/../../secrets/14/1/1.png",
    "/absolute/14/1/1.png",
    "proj-1/scene/14/1/1.jpg",
    "proj-1/scene/14/1.png",
    "proj-1/scene/17/1/1.png",
    "proj-1//14/1/1.png",
    "proj 1/scene/14/1/1.png",
    "proj-1/scene/14/1/1.png/../../x",
    "proj-1/scene/14/1/1.png\n",
    "proj%2f1/scene/14/1/1.png",
    "proj-1/scene%5cother/14/1/1.png",
    "",
])
def test_any_key_that_could_escape_the_root_is_refused(key):
    with pytest.raises(store.BadKey):
        store.check_tile_key(key)


@pytest.mark.parametrize("prefix", ["proj-1/S2C_37MER_20261105_1_L2A", "a/b"])
def test_valid_prefixes_pass(prefix):
    assert store.check_prefix(prefix) == prefix


@pytest.mark.parametrize("prefix", ["..", "a/../../b", "/a/b", "a/b/c", "a/b/", "a", "a/b\n"])
def test_any_prefix_that_could_widen_the_purge_is_refused(prefix):
    with pytest.raises(store.BadKey):
        store.check_prefix(prefix)


def test_volume_store_writes_under_the_root_and_nowhere_else(volume, tmp_path):
    volume.put("p/s/14/1/1.png", b"png")
    assert (tmp_path / "tiles/p/s/14/1/1.png").read_bytes() == b"png"
    with pytest.raises(store.BadKey):
        volume.put("../escaped.png", b"png")
    assert list(tmp_path.parent.glob("escaped.png")) == []


def test_volume_store_leaves_no_temporary_files(volume, tmp_path):
    volume.put("p/s/14/1/1.png", b"png")
    assert (tmp_path / "tiles/p/s/14/1/1.png").read_bytes() == b"png"
    strays = [path for path in (tmp_path / "tiles").rglob("*")
              if path.is_file() and path.name != "1.png"]
    assert strays == []


@pytest.mark.parametrize("payload", [b"\x89PNG\r\n\x1a\n\x00\xff", b"x" * store.MAX_TILE_BYTES])
def test_volume_get_returns_exact_bytes(volume, payload):
    volume.put("p/s/14/1/1.png", payload)
    assert volume.get("p/s/14/1/1.png") == payload


def test_volume_get_missing_is_typed(volume):
    with pytest.raises(store.TileNotFound, match="^tile not found$"):
        volume.get("p/missing/14/1/1.png")


def test_volume_get_rejects_oversized_tiles(volume, monkeypatch):
    volume.put("p/s/14/1/1.png", b"x" * (store.MAX_TILE_BYTES + 2))
    with (volume.root / "p/s/14/1/1.png").open("rb") as tile:
        opened = Mock(wraps=tile)
        monkeypatch.setattr(pathlib.Path, "open", lambda *args, **kwargs: closing(opened))
        with pytest.raises(store.StoreUnavailable, match="size limit"):
            volume.get("p/s/14/1/1.png")
        opened.read.assert_called_once_with(store.MAX_TILE_BYTES + 1)
        opened.close.assert_called_once_with()


@pytest.mark.parametrize("escape", ["file", "directory", "missing"])
def test_volume_get_rejects_symlinks_outside_resolved_root(volume, tmp_path, escape):
    key = "p/s/14/1/1.png"
    volume.put(key, b"inside")
    outside = tmp_path / "tiles-other"
    outside.mkdir()
    (outside / "1.png").write_bytes(b"outside")
    if escape == "directory":
        link = volume.root / "linked"
        link.symlink_to(outside, target_is_directory=True)
        key = "linked/s/14/1/1.png"
    else:
        link = volume.root / key
        link.unlink()
        link.symlink_to(outside / ("1.png" if escape == "file" else "missing.png"))
    with pytest.raises(store.BadKey, match="^tile path escapes store root$"):
        volume.get(key)


def test_volume_get_accepts_a_symlink_inside_a_symlinked_root(volume, tmp_path):
    volume.put("p/s/14/1/1.png", b"inside")
    (volume.root / "p/s/14/1/2.png").symlink_to("1.png")
    root_link = tmp_path / "root-link"
    root_link.symlink_to(volume.root, target_is_directory=True)
    assert store.VolumeStore(root_link).get("p/s/14/1/2.png") == b"inside"


@pytest.mark.parametrize("error_type", [PermissionError, IsADirectoryError, NotADirectoryError, OSError])
def test_volume_get_io_failures_are_unavailable_without_filesystem_paths(volume, monkeypatch, error_type):
    def fail_open(path, *args, **kwargs):
        raise error_type(f"private filesystem path: {path}")

    monkeypatch.setattr(pathlib.Path, "open", fail_open)
    with pytest.raises(store.StoreUnavailable, match="^tile could not be read$"):
        volume.get("p/s/14/1/1.png")


def test_volume_get_symlink_loop_is_unavailable(volume):
    volume.put("p/s/14/1/1.png", b"inside")
    path = volume.root / "p/s/14/1/1.png"
    path.unlink()
    path.symlink_to("1.png")
    with pytest.raises(store.StoreUnavailable, match="^tile could not be read$"):
        volume.get("p/s/14/1/1.png")


@pytest.mark.parametrize("key", ["../secret.png", "p/s/14/1/1.png\n", "p%2fs/s/14/1/1.png"])
def test_get_validates_key_before_io(volume, s3, monkeypatch, key):
    monkeypatch.setattr(pathlib.Path, "resolve", Mock(side_effect=AssertionError("filesystem accessed")))
    for backend in (volume, s3):
        with pytest.raises(store.BadKey):
            backend.get(key)
    assert s3.client.get_calls == []


def test_purge_removes_one_scene_and_counts_it(volume):
    for key in ("p/s1/14/1/1.png", "p/s1/14/1/2.png", "p/s2/14/1/1.png"):
        volume.put(key, b"png")
    assert volume.delete_prefix("p/s1") == 2
    assert not (volume.root / "p/s1").exists()
    assert (volume.root / "p/s2/14/1/1.png").exists()


def test_purge_of_an_absent_prefix_is_a_zero_not_an_error(volume):
    assert volume.delete_prefix("p/never-rendered") == 0


def test_probe_reports_a_store_that_cannot_be_written(tmp_path):
    blocker = tmp_path / "a-file"
    blocker.write_bytes(b"not a directory")
    assert store.VolumeStore(blocker).probe() != ""
    assert store.VolumeStore(tmp_path / "ok").probe() == ""


@pytest.mark.parametrize("payload", [b"\x89PNG\r\n\x1a\n\x00\xff", b"x" * store.MAX_TILE_BYTES])
def test_s3_get_reads_exact_bytes_with_a_limit_and_closes_body(s3, payload):
    key = "p/s/14/1/1.png"
    s3.put(key, payload)
    assert s3.get(key) == payload
    assert s3.client.get_calls == [("tiles", key)]
    s3.client.body.read.assert_called_once_with(store.MAX_TILE_BYTES + 1)
    s3.client.body.close.assert_called_once_with()
    assert s3.client.stream.closed


def test_s3_get_missing_is_typed(s3):
    with pytest.raises(store.TileNotFound, match="^tile not found$"):
        s3.get("p/missing/14/1/1.png")
    assert s3.client.body is None


@pytest.mark.parametrize("code", ["NoSuchKey", "NotFound", "404"])
def test_s3_get_only_recognized_missing_codes_are_not_found(s3, code):
    s3.client.get_error = ClientError({"Error": {"Code": code}}, "GetObject")
    with pytest.raises(store.TileNotFound, match="^tile not found$"):
        s3.get("p/s/14/1/1.png")


@pytest.mark.parametrize("error", [
    ClientError({"Error": {"Code": "AccessDenied"}}, "GetObject"),
    ClientError({"Error": {"Code": "NoSuchBucket"}, "ResponseMetadata": {"HTTPStatusCode": 404}},
                "GetObject"),
    ClientError({"Error": {"Code": "InternalError"}}, "GetObject"),
    ClientError({"ResponseMetadata": {"HTTPStatusCode": 404}}, "GetObject"),
    RuntimeError("NoSuchKey is just text, not a missing response"),
])
def test_s3_get_other_errors_are_unavailable(s3, error):
    s3.client.get_error = error
    with pytest.raises(store.StoreUnavailable, match="^tile could not be read$"):
        s3.get("p/s/14/1/1.png")


def test_s3_get_rejects_oversized_tiles_and_closes_body(s3):
    s3.put("p/s/14/1/1.png", b"x" * (store.MAX_TILE_BYTES + 2))
    with pytest.raises(store.StoreUnavailable, match="size limit"):
        s3.get("p/s/14/1/1.png")
    s3.client.body.read.assert_called_once_with(store.MAX_TILE_BYTES + 1)
    s3.client.body.close.assert_called_once_with()
    assert s3.client.stream.closed


@pytest.mark.parametrize("error", [OSError("read failed"),
                                   ClientError({"Error": {"Code": "NoSuchKey"}}, "GetObject")])
def test_s3_get_read_failures_close_body_and_are_not_missing(s3, monkeypatch, error):
    s3.put("p/s/14/1/1.png", b"png")
    monkeypatch.setattr(StreamingBody, "read", Mock(side_effect=error))
    with pytest.raises(store.StoreUnavailable, match="^tile could not be read$"):
        s3.get("p/s/14/1/1.png")
    s3.client.body.close.assert_called_once_with()
    assert s3.client.stream.closed


def test_s3_get_close_failure_is_unavailable(s3, monkeypatch):
    s3.put("p/s/14/1/1.png", b"png")
    close = StreamingBody.close

    def fail_close(body):
        close(body)
        raise OSError("close failed")

    monkeypatch.setattr(StreamingBody, "close", fail_close)
    with pytest.raises(store.StoreUnavailable, match="^tile could not be read$"):
        s3.get("p/s/14/1/1.png")
    s3.client.body.close.assert_called_once_with()
    assert s3.client.stream.closed


def test_s3_store_pages_through_a_prefix_and_deletes_in_batches():
    client = FakeS3Client()
    s3 = store.S3Store("tiles", "https://account.r2.cloudflarestorage.com")
    s3.client = client
    for index in range(3):
        s3.put(f"p/s/14/{index}/1.png", b"png")
    s3.put("p/other/14/1/1.png", b"png")
    assert s3.delete_prefix("p/s") == 3
    assert list(s3.client.objects) == ["p/other/14/1/1.png"]
    assert s3.probe() == ""


def test_s3_store_wraps_a_client_failure(tmp_path):
    class Broken:
        def put_object(self, **kwargs):
            raise RuntimeError("nope")

    s3 = store.S3Store("tiles", "https://account.r2.cloudflarestorage.com")
    s3.client = Broken()
    with pytest.raises(store.StoreUnavailable):
        s3.put("p/s/14/1/1.png", b"png")


def test_s3_backend_is_chosen_by_name():
    settings = config.Settings(tile_store_backend="volume", tile_store_root="/tmp/tiles")
    assert isinstance(store.from_settings(settings), store.VolumeStore)
    with pytest.raises(store.StoreUnavailable):
        store.from_settings(config.Settings(tile_store_backend="postgres"))


def test_r2_needs_a_bucket():
    settings = config.Settings(tile_store_backend="r2", tile_store_bucket="")
    with pytest.raises(store.StoreUnavailable):
        store.from_settings(settings)
