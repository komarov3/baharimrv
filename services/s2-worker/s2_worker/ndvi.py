"""NDVI arithmetic on Sentinel-2 L2A surface reflectance. See 5.3.

Kept deliberately small: the numbers an MRV record is anchored to have to be
reproducible from this file plus the scene identifier, with no hidden
parameters.
"""

from __future__ import annotations

import numpy as np

# L2A reflectance bands are stored as unsigned integers at a fixed scale.
SCALE = 0.0001
OFFSET = 0

# Class layer of a granule processed at baseline 05.09 or later:
# 0 no data, 1 saturated or defective, 2 dark area, 3 cloud shadow,
# 4 vegetation, 5 not vegetated, 6 water, 7 unclassified,
# 8 cloud medium probability, 9 cloud high probability, 10 thin cirrus, 11 snow.
# Read off three real granules by matching this raster's class histogram against
# the s2:*_percentage fields the product publishes, not from a memory of the doc:
# the pre-05.09 numbering put clouds at 3, 4 and 5, which is exactly where the
# vegetation and bare ground live here, so an inherited table silently masks the
# plot and averages the cloud instead. stac.table_supported keeps the two apart.
#
# Only vegetation and not-vegetated are a surface that can be indexed. Water is
# the one that matters most: mangrove plots sit at the tidal edge, and unmasked
# water pulls a mean down hard enough to fake a trend.
USABLE_CLASSES = frozenset({4, 5})
CLASS_TABLE = "msk-0509/1"

MASKED = np.nan


def from_reflectance(red: np.ndarray, nir: np.ndarray,
                     scl: np.ndarray | None = None,
                     mask: np.ndarray | None = None) -> np.ndarray:
    """Unitless NDVI, with NaN wherever a pixel must not be averaged."""
    red_values = red.astype(np.float64) * SCALE + OFFSET
    nir_values = nir.astype(np.float64) * SCALE + OFFSET
    denominator = nir_values + red_values
    with np.errstate(divide="ignore", invalid="ignore"):
        index = (nir_values - red_values) / denominator
    index[~np.isfinite(index)] = MASKED
    unusable = np.zeros(index.shape, dtype=bool)
    if scl is not None:
        unusable |= ~np.isin(scl, sorted(USABLE_CLASSES))
    if mask is not None:
        unusable |= ~mask
    index[unusable] = MASKED
    return index


def stats(index: np.ndarray, expected_pixels: int) -> dict:
    """Mean, median and coverage over the pixels that survived masking.

    coverage_pct is valid pixels divided by expected pixels, and it is reported
    even when low: a figure computed on 4 percent of the area must not look like
    a figure computed on all of it.
    """
    valid = index[np.isfinite(index)]
    count = int(valid.size)
    if count == 0:
        return {"mean_ndvi": None, "median_ndvi": None, "valid_pixels": 0,
                "expected_pixels": int(expected_pixels), "coverage_pct": 0.0}
    return {
        "mean_ndvi": round(float(valid.mean()), 4),
        "median_ndvi": round(float(np.median(valid)), 4),
        "valid_pixels": count,
        "expected_pixels": int(expected_pixels),
        "coverage_pct": round(100.0 * count / float(expected_pixels), 2) if expected_pixels else 0.0,
    }


def per_cell(index: np.ndarray, cells: list[tuple[int, int, int, int]],
             names: list[str]) -> list[dict]:
    """Zonal stats over row/col windows, weighted later by true area in the API."""
    rows = []
    for name, (row_start, col_start, row_stop, col_stop) in zip(names, cells):
        window = index[row_start:row_stop, col_start:col_stop]
        rows.append({"plot_id": name, **stats(window, window.size)})
    return rows
