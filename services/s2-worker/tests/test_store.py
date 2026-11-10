"""The tile store: key shape first, bytes second."""

from __future__ import annotations

import pathlib

import pytest

from s2_worker import config, store


class FakeS3Client:
    """Enough of boto3's S3 surface to exercise the paging and delete logic."""

    def __init__(self):
        self.objects: dict[str, bytes] = {}
        self.list_calls = 0

    def put_object(self, Bucket, Key, Body, ContentType, CacheControl):
        self.objects[Key] = Body

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
    "",
])
def test_any_key_that_could_escape_the_root_is_refused(key):
    with pytest.raises(store.BadKey):
        store.check_tile_key(key)


@pytest.mark.parametrize("prefix", ["proj-1/S2C_37MER_20261105_1_L2A", "a/b"])
def test_valid_prefixes_pass(prefix):
    assert store.check_prefix(prefix) == prefix


@pytest.mark.parametrize("prefix", ["..", "a/../../b", "/a/b", "a/b/c", "a/b/", "a"])
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
