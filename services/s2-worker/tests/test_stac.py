"""Discovery, run against the two catalogs' own recorded answers.

These are the payloads CDSE and earth-search actually returned for the Kilifi
AOI, so what is under test is the parsing and the ranking, not a sketch of what
a STAC item might look like.
"""

from __future__ import annotations

import datetime as dt
import re

import pytest

from s2_worker import cogs, job, stac

KILIFI = {"type": "Polygon", "coordinates": [[[39.83, -3.65], [39.84, -3.65],
                                              [39.84, -3.63], [39.83, -3.63],
                                              [39.83, -3.65]]]}
START = dt.datetime(2026, 9, 26, tzinfo=dt.timezone.utc)
END = dt.datetime(2026, 11, 10, 23, 59, 59, tzinfo=dt.timezone.utc)


@pytest.fixture
def catalogs(stac_features, monkeypatch):
    """Both sources answering from the recordings, dispatched by their own URL."""

    def fake_search(url, body, timeout):
        for name, source in (("cdse", stac.CDSE), ("element84", stac.ELEMENT84)):
            if url.startswith(source.url):
                return stac_features[name]
        raise AssertionError(f"no recording for {url}")

    monkeypatch.setattr(stac, "_search", fake_search)
    return (stac.CDSE, stac.ELEMENT84)


def test_every_recorded_item_parses(stac_features):
    for name, source in (("cdse", stac.CDSE), ("element84", stac.ELEMENT84)):
        scenes = [source._to_scene(item) for item in stac_features[name]]
        assert len(scenes) == len(stac_features[name])
        assert all(scene.scene_id for scene in scenes)
        assert all(scene.acquired_at.tzinfo is not None for scene in scenes)


def test_element84_items_carry_https_cogs_and_cdse_items_do_not(stac_features):
    """The reason pixels and discovery are separate concerns (spec 3)."""
    earth = [stac.ELEMENT84._to_scene(item) for item in stac_features["element84"]]
    assert all(scene.has_pixels() for scene in earth)
    assert all(href.startswith("https://") for scene in earth for href in scene.bands.values())

    copernicus = [stac.CDSE._to_scene(item) for item in stac_features["cdse"]]
    assert all(not scene.has_pixels() for scene in copernicus), \
        "CDSE assets are not https COGs; adopting them as a pixel source would fail on every read"


def test_band_urls_are_limited_to_the_host_the_worker_is_allowed_to_open(stac_features):
    for item in stac_features["element84"]:
        scene = stac.ELEMENT84._to_scene(item)
        for href in scene.bands.values():
            host = href.split("/", 3)[2]
            assert host in cogs.ALLOWED_HOSTS, f"{scene.scene_id} serves {host}"
            cogs._vsicurl(href)  # the same check the read path applies


def test_cdse_granule_name_yields_orbit_tile_and_baseline(stac_features):
    item = next(item for item in stac_features["cdse"] if "_N" in item["id"])
    scene = stac.CDSE._to_scene(item)
    match = stac.GRANULE.search(item["id"])
    assert match, f"{item['id']} should look like a granule"
    assert (scene.orbit, scene.tile) == (f"R{match['orbit']}", match["tile"])
    assert re.fullmatch(r"0[45]\.[0-9]{2}", scene.baseline), scene.baseline


def test_a_granule_id_without_the_processor_field_is_not_given_a_guess(stac_features):
    """earth-search publishes no N0513, so the baseline comes from the properties."""
    scene = stac.ELEMENT84._to_scene(stac_features["element84"][0])
    assert "_N" not in scene.scene_id
    assert scene.baseline, "no baseline means the class table cannot be trusted"
    assert stac.table_supported(scene)
    assert scene.orbit == "" or scene.orbit.startswith("R")


def test_baseline_parsing_refuses_shapes_it_cannot_read():
    assert stac._baseline("05.13") == "05.13"
    assert stac._baseline("N0509") == "05.09"
    assert stac._baseline("") == ""
    assert stac._baseline("04") == ""
    assert stac._baseline("20261105T073039") == "", \
        "fourteen digits is a timestamp, not a processor version"


def test_the_class_table_gate_refuses_pre_0509_and_silent_items():
    def at(baseline):
        return stac.Scene("x", dt.datetime(2026, 1, 1, tzinfo=dt.timezone.utc), 1.0,
                          "S2A", 32637, baseline=baseline)

    assert stac.table_supported(at("05.09"))
    assert stac.table_supported(at("05.13"))
    assert not stac.table_supported(at("04.09"))
    assert not stac.table_supported(at("")), "an unknown baseline is not a supported one"


def test_the_pool_dedupes_by_scene_id_and_reports_the_refusals(catalogs):
    pool, notes = stac.candidate_pool(catalogs, KILIFI, START, END, require_pixels=False)
    assert len(pool) == len({scene.scene_id for scene in pool})
    assert pool, "the Kilifi window has scenes in it"
    assert any("refused" in note for note in notes), \
        "CDSE alone cannot supply pixels and the note has to say so"


def test_a_pool_of_only_cdse_items_is_empty_not_a_lie(catalogs):
    pool, _ = stac.candidate_pool([stac.CDSE], KILIFI, START, END)
    assert pool == [], "no https COGs, so no scene this worker could ever measure"


def test_the_recorded_window_offers_a_scene_within_every_ceiling(catalogs):
    pool, _ = stac.candidate_pool(catalogs, KILIFI, START, END)
    assert pool
    assert min(scene.cloud_pct for scene in pool if scene.cloud_pct is not None) <= 40.0


def test_ranking_is_cloud_tier_first_then_recency():
    def at(scene_id, cloud, month, day):
        return stac.Scene(scene_id, dt.datetime(2026, month, day, tzinfo=dt.timezone.utc),
                          cloud, "S2A", 32637)

    ranked = stac.order_candidates([at("hazy-new", 55.0, 11, 10),
                                    at("clean-old", 12.0, 9, 1),
                                    at("unfigure", None, 11, 8)])
    ids = [scene.scene_id for scene in ranked]
    assert ids == ["clean-old", "hazy-new", "unfigure"], (
        "a 12% scene from nine weeks ago beats a 55% scene from today, and a scene "
        "with no cloud figure at all trails both because nothing screened it")


def test_within_a_tier_the_newest_pass_wins():
    def at(day):
        return stac.Scene(f"d{day}", dt.datetime(2026, 11, day, tzinfo=dt.timezone.utc),
                          12.0, "S2A", 32637)

    assert [scene.scene_id for scene in stac.order_candidates([at(3), at(9), at(5)])] == \
        ["d9", "d5", "d3"]


class _OneSceneSource:
    """A catalog that answers with exactly the scenes a test needs."""

    def __init__(self, name: str, scenes):
        self.name, self.url, self.timeout, self.pixels = name, f"https://{name}/v1", 5.0, True
        self._scenes = scenes

    def candidates(self, geometry, start, end, limit=50):
        return list(self._scenes)


def test_the_pool_drops_scenes_above_the_top_ceiling():
    """A 95% granule is not the last resort; it is not a candidate."""
    def at(scene_id, cloud):
        return stac.Scene(scene_id, dt.datetime(2026, 11, 5, tzinfo=dt.timezone.utc), cloud,
                          "S2A", 32637, baseline="05.13",
                          bands={role: f"https://sentinel-cogs.s3.us-west-2.amazonaws.com/x/{r}.tif"
                                 for role, r in (("red", "B04"), ("nir", "B08"), ("scl", "SCL"))})

    pool, notes = stac.candidate_pool([_OneSceneSource("fake", [at("clear", 8.0), at("soot", 95.0)])],
                                      KILIFI, START, END)
    assert [scene.scene_id for scene in pool] == ["clear"]
    assert notes and "above 70%" in notes[0]


def test_the_ladder_widens_without_relabeling_a_hazy_scene_as_clean():
    assert job.ladder(None) == stac.CLOUD_CEILINGS
    assert job.ladder(55.0) == (40.0, 55.0)
    assert job.ladder(5.0) == (5.0,)


def test_a_bare_date_end_means_the_whole_day():
    """Midnight would drop that morning's pass, the freshest one asked for."""
    assert job.parse_end("2026-11-10").hour == 23
    assert job.parse_end("2026-11-10T07:30:00Z").hour == 7
    assert job.parse_end("2026-11-10T09:55:22+02:00").hour == 7, "stored in UTC"
    assert job.parse_end(None).tzinfo is not None
    start, end = job.window_for(job.parse_end("2026-11-10"), 45)
    assert (end - start).days == 45


def test_scene_ref_round_trips_through_the_payload_a_caller_sends_back():
    scene = stac.Scene("S2C_37MER_20261105_1_L2A",
                       dt.datetime(2026, 11, 5, 7, 30, tzinfo=dt.timezone.utc), 29.6,
                       "Sentinel-2C", 32637, orbit="R049", tile="T37MER", baseline="05.13",
                       discovery_source="element84-stac",
                       bands={"red": "https://sentinel-cogs.s3.us-west-2.amazonaws.com/a/B04.tif"},
                       footprint={"type": "Polygon",
                                  "coordinates": [[[0, 0], [1, 0], [1, 1], [0, 0]]]})
    payload = job.scene_ref(scene)
    rebuilt = job.scene_from_ref(payload)
    for field in ("scene_id", "acquired_at", "cloud_pct", "platform", "epsg", "baseline",
                  "tile", "orbit", "discovery_source", "footprint"):
        assert getattr(rebuilt, field) == getattr(scene, field), field
    assert rebuilt.bands == scene.bands


def test_a_scene_without_the_three_roles_is_refused_before_the_first_read():
    scene = stac.Scene("S", dt.datetime(2026, 11, 5, tzinfo=dt.timezone.utc), 1.0, "S2A", 32637,
                       baseline="05.13",
                       bands={"red": "https://x/B04.tif", "nir": "https://x/B08.tif"})
    with pytest.raises(job.Unavailable, match="no COG for scl"):
        job.validate_bands(scene)


def test_a_pre_0509_scene_is_refused_rather_than_masked_with_the_wrong_table():
    scene = stac.Scene("S", dt.datetime(2026, 11, 5, tzinfo=dt.timezone.utc), 1.0, "S2A", 32637,
                       baseline="04.09",
                       bands={"red": "https://x/B04.tif", "nir": "https://x/B08.tif",
                              "scl": "https://x/SCL.tif"})
    with pytest.raises(job.Unavailable, match="class table"):
        job.validate_bands(scene)
