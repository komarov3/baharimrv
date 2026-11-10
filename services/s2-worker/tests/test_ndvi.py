"""The arithmetic an MRV figure is anchored to, and the table that says what enters it."""

from __future__ import annotations

import numpy as np
import pytest

from s2_worker import ndvi


def dn(reflectance: float) -> int:
    """A raw digital number for the reflectance the product would store."""
    return int(round(reflectance / ndvi.SCALE))


def test_index_is_nir_minus_red_over_the_sum():
    red = np.full((1, 1), dn(0.10), dtype=np.uint16)
    nir = np.full((1, 1), dn(0.50), dtype=np.uint16)
    index = ndvi.from_reflectance(red, nir, np.array([[4]], dtype=np.uint8))
    assert index[0, 0] == pytest.approx((0.50 - 0.10) / (0.50 + 0.10))


def test_scale_factor_is_applied_once():
    """If the scale were applied twice the whole product would read near zero."""
    red = np.full((1, 1), 1000, dtype=np.uint16)
    nir = np.full((1, 1), 3000, dtype=np.uint16)
    index = ndvi.from_reflectance(red, nir, np.array([[4]], dtype=np.uint8))
    assert index[0, 0] == pytest.approx(0.5)


@pytest.mark.parametrize("cls", sorted(ndvi.USABLE_CLASSES))
def test_the_two_surface_classes_survive(cls):
    index = ndvi.from_reflectance(np.full((2, 2), 1000, np.uint16),
                                 np.full((2, 2), 5000, np.uint16),
                                 np.full((2, 2), cls, np.uint8))
    assert np.isfinite(index).all()


@pytest.mark.parametrize("cls", [0, 1, 2, 3, 6, 7, 8, 9, 10, 11])
def test_every_class_outside_the_whitelist_is_dropped(cls):
    """Water and cloud are the two that fake a trend if they are averaged in.

    Mangrove plots sit at the tidal edge, so an unmasked water pixel is a very
    negative NDVI inside a plot that is actually canopy.
    """
    index = ndvi.from_reflectance(np.full((2, 2), 1000, np.uint16),
                                 np.full((2, 2), 5000, np.uint16),
                                 np.full((2, 2), cls, np.uint8))
    assert np.isnan(index).all()
    assert ndvi.stats(index, 4) == {"mean_ndvi": None, "median_ndvi": None, "valid_pixels": 0,
                                   "expected_pixels": 4, "coverage_pct": 0.0}


def test_an_unmapped_class_is_excluded_not_included():
    """The whitelist is a whitelist for exactly this case.

    Class 12 and above do not exist in `msk-0509/1`. If Copernicus adds one, the
    pixels vanish from the mean and coverage falls, which is visible. A blacklist
    would have folded the new class into the headline number in silence.
    """
    index = ndvi.from_reflectance(np.full((2, 2), 1000, np.uint16),
                                 np.full((2, 2), 5000, np.uint16),
                                 np.full((2, 2), 12, np.uint8))
    assert np.isnan(index).all()


def test_a_zero_denominator_is_not_a_number():
    zero = np.zeros((2, 2), dtype=np.uint16)
    index = ndvi.from_reflectance(zero, zero, np.full((2, 2), 4, np.uint8))
    assert np.isnan(index).all()


def test_negative_and_saturated_reflectance_stay_in_range():
    dark = ndvi.from_reflectance(np.full((1, 1), dn(0.4), np.uint16),
                                np.full((1, 1), dn(0.001), np.uint16),
                                np.full((1, 1), 5, np.uint8))
    assert dark[0, 0] == pytest.approx(-0.9950, abs=1e-3)
    dense = ndvi.from_reflectance(np.full((1, 1), dn(0.02), np.uint16),
                                 np.full((1, 1), dn(0.5), np.uint16),
                                 np.full((1, 1), 4, np.uint8))
    assert dense[0, 0] == pytest.approx(0.923, abs=1e-3)


def test_mask_argument_narrows_a_window_without_a_class_layer():
    """Plot polygons arrive here when a pass has no usable class layer."""
    index = ndvi.from_reflectance(np.full((2, 2), 1000, np.uint16),
                                 np.full((2, 2), 5000, np.uint16), None,
                                 mask=np.array([[True, False], [True, True]]))
    assert np.isnan(index[0, 1])
    assert ndvi.stats(index, 4)["valid_pixels"] == 3


def test_scl_and_mask_compose():
    index = ndvi.from_reflectance(np.full((2, 2), 1000, np.uint16),
                                 np.full((2, 2), 5000, np.uint16),
                                 np.array([[4, 4], [4, 6]], np.uint8),
                                 mask=np.array([[True, False], [False, True]]))
    assert np.isnan(index).sum() == 3  # one pixel to the class layer, two to the mask
    assert not np.isnan(index[0, 0])


def test_coverage_is_reported_even_when_it_is_low():
    index = np.array([[0.8, np.nan], [0.6, np.nan]], dtype=np.float64)
    figure = ndvi.stats(index, 4)
    assert figure["coverage_pct"] == 50.0
    assert figure["mean_ndvi"] == 0.7
    assert figure["median_ndvi"] == 0.7


def test_mean_and_median_are_rounded_where_they_are_computed():
    index = np.array([1/3, 2/7, np.nan], dtype=np.float64)
    figure = ndvi.stats(index, 3)
    assert figure["mean_ndvi"] == round(float(np.nanmean(index)), 4)
    assert figure["valid_pixels"] == 2


def test_per_cell_slices_windows_and_names_them():
    index = np.arange(16, dtype=np.float64).reshape(4, 4) / 100.0
    index[0, 0] = np.nan
    rows = ndvi.per_cell(index, [(0, 0, 2, 2), (2, 2, 4, 4)], ["plot-a", "plot-b"])
    assert [row["plot_id"] for row in rows] == ["plot-a", "plot-b"]
    assert rows[0]["valid_pixels"] == 3 and rows[0]["expected_pixels"] == 4
    assert rows[1]["valid_pixels"] == 4


def test_the_table_this_code_implements():
    assert ndvi.CLASS_TABLE == "msk-0509/1"
    assert ndvi.USABLE_CLASSES == frozenset({4, 5})
    assert ndvi.SCALE == 0.0001
