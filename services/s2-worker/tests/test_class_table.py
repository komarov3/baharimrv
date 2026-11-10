"""The class table, checked against the granule's own published percentages.

`ndvi.USABLE_CLASSES` says classes 4 and 5 are vegetation and bare ground. If
that numbering were wrong nothing downstream would look wrong: the mean would
come back plausible and it would be an average of cloud. So this file states the
table a second time, independently of ndvi.py, and requires the recorded raster
histogram and the STAC item's own `s2:*_percentage` fields to agree with it. A
silent renumbering upstream then fails a build instead of poisoning a figure an
MRV record is anchored to.

The tolerance is 3 percentage points because the two figures are not the same
sample: the histogram is one decimated 512x512 read of the whole granule, the
percentages are computed over every pixel. The recorded agreement is 0.0 to 2.8
points per class, and 0.06 to 0.38 points for the vegetation-plus-bare-ground
pair the whitelist is made of.
"""

from __future__ import annotations

import pytest

from s2_worker import ndvi, stac

# Class number -> the property the product publishes for it. Spelled out here on
# purpose: this mapping is what the raster has to corroborate, and a test that
# read the table from the module under test would corroborate nothing.
PERCENTAGE_TO_CLASS = {
    "s2:saturated_defective_pixel_percentage": 1,
    "s2:cloud_shadow_percentage": 3,
    "s2:vegetation_percentage": 4,
    "s2:not_vegetated_percentage": 5,
    "s2:water_percentage": 6,
    "s2:unclassified_percentage": 7,
    "s2:medium_proba_clouds_percentage": 8,
    "s2:high_proba_clouds_percentage": 9,
    "s2:thin_cirrus_percentage": 10,
    "s2:snow_ice_percentage": 11,
}
NO_DATA_CLASS = 0
UNPUBLISHED_CLASSES = {2}  # dark area pixels: a class, never a percentage
TOLERANCE_POINTS = 3.0


@pytest.fixture(params=("clear", "cloudy"))
def granule(request, class_histogram):
    """One recorded granule and its percentages, run through every test here."""
    want_cloudy = request.param == "cloudy"
    rows = [row for row in class_histogram["granules"] if (row["cloud_pct"] > 40.0) == want_cloudy]
    assert len(rows) == 1, (
        f"the recording needs exactly one {request.param} granule; "
        f"found {len(rows)} - re-run tests/capture_fixtures.py")
    return rows[0]


def _share(shares: dict, cls: int) -> float:
    return float(shares.get(str(cls), shares.get(cls, 0.0)))


def test_the_recording_is_of_a_granule_this_worker_would_accept(granule):
    assert granule["processor_baseline"] >= stac.MIN_CLASS_TABLE_BASELINE
    assert set(granule["item_percentages"]) >= set(PERCENTAGE_TO_CLASS) | {"s2:nodata_pixel_percentage"}


def test_the_usable_whitelist_is_vegetation_and_bare_ground():
    assert set(ndvi.USABLE_CLASSES) == {PERCENTAGE_TO_CLASS["s2:vegetation_percentage"],
                                        PERCENTAGE_TO_CLASS["s2:not_vegetated_percentage"]}
    assert ndvi.CLASS_TABLE == "msk-0509/1"


def test_every_published_percentage_matches_the_raster_histogram(granule):
    shares = granule["raster"]["scene_wide"]["pct_of_valid"]
    for name, cls in PERCENTAGE_TO_CLASS.items():
        published = granule["item_percentages"][name]
        measured = _share(shares, cls)
        assert abs(measured - published) <= TOLERANCE_POINTS, (
            f"{granule['scene_id']} class {cls}: the raster says {measured}%, the item "
            f"says {published}% for {name}")


def test_the_no_data_share_matches_too(granule):
    """Scene-wide and over all pixels: no-data is what the granule margins are."""
    published = granule["item_percentages"]["s2:nodata_pixel_percentage"]
    measured = _share(granule["raster"]["scene_wide"]["pct_of_all"], NO_DATA_CLASS)
    assert abs(measured - published) <= TOLERANCE_POINTS


def test_no_class_outside_the_known_range_appears_in_the_raster(granule):
    known = set(PERCENTAGE_TO_CLASS.values()) | {NO_DATA_CLASS} | UNPUBLISHED_CLASSES
    present = {int(cls) for cls, share in granule["raster"]["scene_wide"]["pct_of_all"].items()
               if float(share) > 0.0}
    assert present <= known, f"{granule['scene_id']} holds {sorted(present - known)}"


def test_the_cloud_tiers_are_not_where_the_usable_classes_are(granule):
    """The assertion that would catch the pre-05.09 numbering.

    Read with the older table, classes 3, 4 and 5 are cloud shadow, medium and
    high probability cloud. This granule's classes 4 and 5 would then have to
    carry the two published cloud percentages between them. They carry the
    vegetation and bare-ground ones instead, and miss the cloud pair by 5 to 24
    points, which is only true of the numbering this worker whitelists.
    """
    shares = granule["raster"]["scene_wide"]["pct_of_valid"]
    published = granule["item_percentages"]
    surface = _share(shares, 4) + _share(shares, 5)
    as_surface = published["s2:vegetation_percentage"] + published["s2:not_vegetated_percentage"]
    as_cloud = (published["s2:medium_proba_clouds_percentage"]
                + published["s2:high_proba_clouds_percentage"])

    assert abs(surface - as_surface) <= TOLERANCE_POINTS
    assert abs(surface - as_cloud) > TOLERANCE_POINTS, (
        f"{granule['scene_id']}: classes 4 and 5 match the cloud totals, so the numbering "
        f"has changed and {ndvi.CLASS_TABLE} no longer means what ndvi.py says it means")


def test_the_water_class_is_excluded_because_a_creek_edge_is_not_a_surface(granule):
    """6 is water, and water is the number that would fake a mangrove trend."""
    published = granule["item_percentages"]["s2:water_percentage"]
    measured = _share(granule["raster"]["scene_wide"]["pct_of_valid"],
                      PERCENTAGE_TO_CLASS["s2:water_percentage"])
    assert abs(measured - published) <= TOLERANCE_POINTS
    assert PERCENTAGE_TO_CLASS["s2:water_percentage"] not in ndvi.USABLE_CLASSES


def test_the_cell_the_golden_figures_come_from_was_all_vegetation(class_histogram):
    clear = next(row for row in class_histogram["granules"] if row["cloud_pct"] <= 40.0)
    over_cell = clear["raster"].get("over_cell")
    assert over_cell, "the chosen scene is the one whose own cell was recorded"
    assert list(over_cell["pct_of_all"]) == ["4"], over_cell["pct_of_all"]
    assert over_cell["pct_of_all"]["4"] == 100.0
