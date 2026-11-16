# Plot Boundary & Derived Area — Spec

Status: DRAFT for approval. Not implemented.
Scope: backend (Go), s2-worker (Python), Android. Web add-plot is unchanged (boundary is optional, so point+area keeps working there).

## 1. Problem

A plot is registered from a single GPS point plus a *stated* hectare size. No real
outline is captured. The satellite layer then invents a boundary: an axis-aligned
square centred on the point whose area equals the stated hectares
(`backend/internal/satellite/aoi.go:38`, mirrored by `services/s2-worker/s2_worker/aoi.py:22`).
That square assumes the centroid lies inside the plot; GPS error beyond half the
side moves the NDVI sample off the trees. The stated area is also never checked
against any geometry, so reported hectares can disagree with the ground.

## 2. What already exists (do not rebuild)

- `plots.boundary` (JSONB / TEXT) and `plots.gps_accuracy_m` columns already exist:
  `migrations/postgres/000009_satellite_ndvi_overlay.sql:7-8`,
  `migrations/sqlite/000009_satellite_ndvi_overlay.sql:6-7`. **No new migration needed.**
- `models.Plot.Boundary` (GeoJSON Polygon, EPSG:4326) and `.GpsAccuracyM` exist but are
  written by nobody and read by nobody: `backend/internal/models/models.go:251-252`.
- Add-plot endpoint exists: `POST /api/v1/projects/:id/plots` → `Service.AddPlot`
  (`router.go:151`, `mrv/project.go:1054`). `CreateProject` also accepts plots
  (`mrv/project.go:949-969`). Both use `PlotInput` (`mrv/project.go:885`), which has
  **no** boundary field today. Area is parsed and stored through `money.ParseQuantity`
  / `money.Quantity` (numeric(20,6)), so a derived area must use that same path.
- Android `AddPlotScreen` Save is a stub that never calls the repository
  (`ui/projects/AddPlotScreen.kt:89`). Android has no `POST .../plots` API method
  (only `PATCH`, `Api.kt:968`); `CreatePlotRequestDto` (`Api.kt:214`) is used only
  inside project creation.

## 3. Design decisions

1. **Capture a real polygon.** Two capture modes in Android, both producing a
   GeoJSON Polygon ring:
   - *Tap vertices* on an osmdroid map (tap adds a corner; undo / clear; live outline).
   - *Walk perimeter* with GPS (a "record vertex" button, plus optional auto-sample
     every N metres); stores `gpsAccuracyM` from the fix.
   Manual point + stated area remains as a fallback, clearly labelled **Approximate**.
2. **Boundary is optional.** Plots without a boundary keep today's behaviour exactly
   (square cell, stated area). Nothing regresses for existing or web-created plots.
3. **Area is derived, not stated, when a boundary exists.** On write, if a valid
   boundary is supplied, `AreaHectares` is recomputed from the polygon and the typed
   value is ignored. With no boundary, the typed value is used as today.
4. **Centroid follows the boundary.** When a boundary exists, `latitude/longitude`
   become the polygon's area-weighted centroid so the pin sits inside the outline.
   With no boundary they stay the recorded point.
5. **AOI prefers the boundary.** `aoiFromPlots` (Go) and the worker cell builder
   (Python) use the stored polygon as the analysis cell when present; otherwise the
   existing square. The MultiPolygon-union + framing-bbox logic is unchanged and
   already works with arbitrary rings.
6. **One shared geometry algorithm** implemented identically in Go, Python and Kotlin
   so the three never disagree (the worker fixture replay depends on this).

## 4. Shared geometry (local equirectangular + shoelace)

Dependency-free, consistent with the existing `metresPerDegreeLat = 111320` convention.
Adequate for plots up to a few km; document the approximation.

Given a closed ring of `[lon, lat]` (degrees), let `lat0`, `lon0` = ring vertex mean:
- `x_i = (lon_i - lon0) * cos(lat0) * 111320`
- `y_i = (lat_i - lat0) * 111320`
- `A_m2 = 0.5 * |Σ (x_i * y_{i+1} - x_{i+1} * y_i)|`
- `hectares = A_m2 / 10000`
- Centroid (area-weighted, standard shoelace centroid):
  `Cx = Σ (x_i + x_{i+1}) * cross_i / (6 * A_signed)`, same for `Cy`, where
  `cross_i = x_i * y_{i+1} - x_{i+1} * y_i`; convert back:
  `lon_c = lon0 + Cx / (cos(lat0) * 111320)`, `lat_c = lat0 + Cy / 111320`.

## 5. Validation (backend, on write)

Reject with `domain.Invalid` unless: GeoJSON `type == "Polygon"`; exactly one outer
ring (holes rejected for now); ring closed (first == last); >= 4 points; all
lon/lat in range; derived area > 0 and finite. Self-intersection is not strictly
checked in v1 (note as a follow-up); the capture UI prevents obvious cases.
On success store `Boundary`, `GpsAccuracyM`, derived `AreaHectares`, centroid.

## 6. Per-layer changes

### Backend (Go)
- `PlotInput`: add `Boundary json.RawMessage` (optional) + `GpsAccuracyM *float64`.
- `AddPlot` + `CreateProject` plot loop: validate boundary, derive area + centroid,
  persist. Return `boundary` / `gpsAccuracyM` in plot payloads (`api/handlers.go:52,866`).
- `satellite/aoi.go`: `aoiFromPlots` uses `plot.Boundary` ring when present (parse to
  ring, reuse for cell + MultiPolygon + bbox); else `cellRing`. Add shared
  `geometry.go` with the section-4 area/centroid helpers.
- Tests: boundary preferred over square; fallback unchanged (existing fixture replay
  must still pass); derived-area + centroid math; validation rejects bad rings.

### s2-worker (Python)
- The live worker does not fetch plots: the Go API builds the cells (now boundary-
  preferring) and posts them to `/stats`, so boundary selection already flows from Go.
- `aoi.py`: add boundary-preferred `plot_cell` plus shared `area_hectares` / `centroid`
  / `boundary_ring` mirroring Go, for the standalone/spike path and to pin the two
  implementations together. The square `cell()` fallback is byte-identical to today so
  the recorded Kilifi fixture still replays.
- Tests: mirror Go cases (boundary preferred, fallback, area/centroid parity).

### Android
- `Api.kt`: add `@POST api/v1/projects/{id}/plots` returning `PlotResponse`; extend
  `CreatePlotRequestDto` with `boundary` (GeoJSON) + `gpsAccuracyM`; extend `PlotDto`
  with `boundary` for map outline rendering.
- `ProjectRepository`: add `createPlot(projectId, request)`.
- `AddPlotScreen`: replace stub with a real form + osmdroid boundary capture (tap /
  walk modes), live derived-area readout (section-4 math in Kotlin), undo/clear,
  approximate-vs-surveyed label, and a wired Save that POSTs. Keep manual point+area
  fallback.
- Plot maps (dashboard / satellite): draw the stored outline when present, else pin.
- Tests: Kotlin area/centroid parity with Go/Python; DTO round-trip.

## 7. Integrity rules

- Never present a derived/surveyed figure as verified unless the server returned it.
- Label boundary-less plots **Approximate (point + stated area)**; boundary plots
  **Surveyed outline**. Do not silently upgrade one to the other.
- Keep `gpsAccuracyM` beside the centroid; surface it in the plot detail.
- No fabricated geometry: an invalid or absent boundary falls back, never invents.

## 8. Phasing

- A: backend accept/validate/derive/persist + return boundary. (Go tests)
- B: Go AOI + Python worker prefer boundary; parity + fixture tests.
- C: Android capture UI + wiring + outline rendering. (Kotlin tests)
- D: build all layers, run suites, emulator visual pass of capture + outline.

## 9. Open defaults (redirect if you disagree)

- Both capture modes (tap + walk) in v1; holes and self-intersection checks deferred.
- Stated area is overridden by derived area whenever a valid boundary exists.
- Web add-plot unchanged in v1 (boundary optional).