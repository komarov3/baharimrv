"""The single NDVI palette. Server-rendered tiles and the Android legend both
read these stops, so they cannot drift apart. See SATELLITE_NDVI_SPEC.md 5.4.

A band matches when lower <= ndvi < upper, and the first and last bands are
open at their outer edge. Pixels that are masked or no-data are transparent so
the basemap shows through rather than being painted as a value.
"""

from __future__ import annotations

import numpy as np

PALETTE_VERSION = "ndvi-5band/1"

# (lower inclusive, upper exclusive, hex colour, label key). None is the open end.
STOPS: list[tuple[float | None, float | None, str, str]] = [
    (None, 0.10, "#B7A57A", "satellite.legend.bare"),
    (0.10, 0.30, "#E8E28A", "satellite.legend.verylow"),
    (0.30, 0.45, "#B8DB7A", "satellite.legend.low"),
    (0.45, 0.60, "#6FBE44", "satellite.legend.moderate"),
    (0.60, None, "#2A7D1E", "satellite.legend.dense"),
]

ZOOM_MIN = 8
ZOOM_MAX = 16


def _rgb(hex_colour: str) -> tuple[int, int, int]:
    value = hex_colour.lstrip("#")
    return int(value[0:2], 16), int(value[2:4], 16), int(value[4:6], 16)


def legend() -> list[dict]:
    """Legend stops for the overlay envelope, in the order they are drawn."""
    return [
        {
            "min": lower if lower is not None else -1.0,
            "max": upper if upper is not None else 1.0,
            "color": colour,
            "labelKey": key,
        }
        for lower, upper, colour, key in STOPS
    ]


def band_of(ndvi: float) -> int:
    """Index of the stop a single value falls in, or -1 when it has no data."""
    if not np.isfinite(ndvi):
        return -1
    for index, (lower, upper, _, _) in enumerate(STOPS):
        if lower is not None and ndvi < lower:
            continue
        if upper is not None and ndvi >= upper:
            continue
        return index
    return len(STOPS) - 1


def colorize(ndvi: np.ndarray) -> np.ndarray:
    height, width = ndvi.shape
    rgba = np.zeros((height, width, 4), dtype=np.uint8)
    valid = np.isfinite(ndvi)
    for index, (lower, upper, colour, _) in enumerate(STOPS):
        band = valid.copy()
        if lower is not None:
            band &= ndvi >= lower
        if upper is not None:
            band &= ndvi < upper
        red, green, blue = _rgb(colour)
        rgba[band] = (red, green, blue, 255)
    return rgba
