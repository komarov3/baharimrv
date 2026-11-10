"""s2-worker: Sentinel-2 NDVI discovery, arithmetic and tile rendering.

The only component in BahariMRV that talks to external imagery hosts. It holds
no project state and performs no authorization; the Go API decides which AOI
belongs to which caller and persists every result.
"""

__all__ = ["aoi", "cogs", "ndvi", "palette", "render", "stac"]
