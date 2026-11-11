# Satellite NDVI overlay and project-level change

**Status:** Draft for review, not approved for implementation
**Date:** 2026-11-10
**Owner:** BahariMRV engineering
**Scope:** Sentinel-2 NDVI visual overlay on the Android satellite screen, plus an honest project-level NDVI change figure. Research phase, non-commercial.
**Related:** [SPEC.md](SPEC.md) section 2.2 and section 3 principle 9

---

## 1. Scope

### 1.1 In scope

1. A real Sentinel-2 derived NDVI raster overlay drawn on a map in `SatelliteMonitoringScreen`.
2. Project-level mean NDVI and change versus a seasonal baseline, computed server-side from the project's own plots.
3. Real "last pass" data: scene identifier, acquisition time, and cloud cover from the catalog item that was actually read, replacing the synthetic guess.
4. Authenticated tile delivery so no upstream host, credential, or storage key is exposed to the device or to other tenants.
5. Anchoring of the computed NDVI summary hash through the existing IOTA path.

### 1.2 Out of scope

1. Per-plot NDVI accuracy and per-plot boundary capture. Plots are centroids today, so per-plot numbers stay indicative. Phase 6 covers boundary capture.
2. Sentinel-1 radar, biomass, or carbon estimation.
3. Provider accounts, keys, and paid imagery plans. Section 3 uses anonymous public endpoints and open Copernicus data, so nothing here waits on an application or a licence. The triggers that would change that are listed in section 12.
4. Automated re-anchoring history and correction records beyond the one superseding-hash rule in section 9.
5. Web dashboard changes beyond reading the new endpoints. The web satellite page keeps its current layout.

### 1.3 SPEC.md constraint

[SPEC.md](SPEC.md) section 2.2 item 6 excludes live satellite integration "without provider credentials and an approved test environment" and requires explicit unavailable or sandbox states. This spec satisfies that condition by keeping the sandbox adapter intact, adding a separate live adapter that fails closed, and gating the whole feature behind `SATELLITE_PROVIDER=opendata-s2`. Merging this spec is an amendment to that exclusion and needs your approval.

---

## 2. Verified current state

These are the facts the design has to work around.

| Finding | Evidence |
|---|---|
| The two project satellite handlers exist but are mounted on no route, so Android's calls return 404 | `projectSatellitePasses` at [backend/internal/api/handlers.go:603](backend/internal/api/handlers.go#L603) and `projectSatelliteLatest` at [handlers.go:625](backend/internal/api/handlers.go#L625), no reference in [backend/internal/api/router.go](backend/internal/api/router.go); only `/admin/satellite/passes` is registered at [router.go:216](backend/internal/api/router.go#L216) |
| Android calls those unmounted routes | `api/v1/projects/{id}/satellite/passes` at [Api.kt:816](android/app/src/main/java/ke/co/baharilink/baharimrv/data/remote/Api.kt#L816) and `.../satellite/latest` at [Api.kt:824](android/app/src/main/java/ke/co/baharilink/baharimrv/data/remote/Api.kt#L824) |
| Pass data is synthetic in sandbox and reports unavailable in live | `PredictPasses` at [service.go:202](backend/internal/satellite/service.go#L202), `LatestPass` at [service.go:243](backend/internal/satellite/service.go#L243), `ProcessingVersion = "sandbox-ndvi/1"` at [service.go:21](backend/internal/satellite/service.go#L21) |
| Plots are single centroids plus a decimal area string. No polygon, boundary, GeoJSON, geometry, or PostGIS anywhere in the repo | `Plot` at [models.go:227](backend/internal/models/models.go#L227), `plots` table at [000001_init.sql:172](backend/internal/database/migrations/postgres/000001_init.sql#L172), `AddPlotScreen` reads a cached last-known fix from the GPS or network provider at `android/app/src/main/java/ke/co/baharilink/baharimrv/ui/projects/AddPlotScreen.kt:94` and stores no accuracy figure |
| The satellite screen has no map and does not compile | [SatelliteMonitoringScreen.kt](android/app/src/main/java/ke/co/baharilink/baharimrv/ui/satellite/SatelliteMonitoringScreen.kt) uses `Row` (lines 94, 142, 188), `Spacer` and `Modifier.height` (117, 170), `PaddingValues` (180) with no matching imports, and redeclares `private enum class ChipTone` at line 235 while importing `ui.components.ChipTone` at line 35, which then fails the `tone:` parameter typed in [Common.kt:62](android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Common.kt#L62) |
| Maps libraries are on the classpath and unused, and the API key resolves to an empty string | `maps-compose:4.4.1` and `play-services-maps:18.2.0` at [android/app/build.gradle.kts:120](android/app/build.gradle.kts#L120); `manifestPlaceholders["googleMapsApiKey"] = config("GOOGLE_MAPS_API_KEY")` at [build.gradle.kts:53](android/app/build.gradle.kts#L53); `GOOGLE_MAPS_API_KEY` is absent from `android/local.properties` and `config()` returns `""` |
| The satellite route carries no project argument | `const val SATELLITE = "satellite"` at [AppRoot.kt:80](android/app/src/main/java/ke/co/baharilink/baharimrv/ui/AppRoot.kt#L80), registered at [AppRoot.kt:212](android/app/src/main/java/ke/co/baharilink/baharimrv/ui/AppRoot.kt#L212), fed from `shellViewModel.selectedProjectId` |
| NDVI is anchored nowhere. Only a free-form string rides on planting records | `satellite_corrob_hash` at [executor/src/index.ts:432](executor/src/index.ts#L432), populated from Jahazi measurements at [backend/internal/jahazi/chain_ops.go:236](backend/internal/jahazi/chain_ops.go#L236); no satellite case in the executor operation switch |
| Provider mode is exposed publicly | `"satellite": r.config.SatelliteMode` in `health` at [router.go:245](backend/internal/api/router.go#L245), sourced from `SATELLITE_MODE` at [config.go:127](backend/internal/config/config.go#L127) |
| Migrations are hand-written, embedded per driver, and drift-checked by test | [database.go:17](backend/internal/database/database.go#L17) embed block, `TestMigrationMatchesModelColumns` at [schema_test.go:16](backend/internal/database/schema_test.go#L16), model list in [registry.go:3](backend/internal/models/registry.go#L3), latest migration `000008_base_methodology.sql` |
| Executor `case 'anchor'` falls through into `create_ledger` | [executor/src/index.ts:113](executor/src/index.ts#L113) has no `break` |

**Phase 0 of this spec is those four defects:** the two unmounted routes, the non-compiling screen, the missing Maps key handling, and the executor fall-through. Everything else assumes they are fixed.

---

## 3. Data sources

**Chosen: no provider account at all. Discovery from anonymous STAC, pixels from the public Sentinel-2 L2A Cloud-Optimized Geotiffs on AWS Open Data, NDVI computed by a worker we run.**

Everything below was confirmed with plain HTTPS requests on 2026-11-10. No credentials, no tokens, and no signup were used in any of these exchanges.

| Probe | Result |
|---|---|
| `POST https://earth-search.aws.element84.com/v1/search`, collection `sentinel-2-l2a`, point `39.28, -3.63`, window 20 Oct to 10 Nov 2026 | 200, 26 matches, newest `S2A_37MER_20261107_0_L2A` |
| Item properties from that catalog | `datetime`, `eo:cloud_cover` 61.59, `platform`, `proj:epsg` 32737 all present. `s2:relativeOrbitNumber`, `s2:mac_cloud`, and `s2:sensingStarTime` returned null, so this source cannot fill the `orbit` column |
| Byte-range read of `B04.tif` from `sentinel-cogs.s3.us-west-2.amazonaws.com/sentinel-s2-l2a-cogs/37/M/ER/2026/11/S2A_37MER_20261107_0_L2A/` | 206, `Accept-Ranges: bytes`, `Access-Control-Allow-Origin: *`, `Cache-Control: public, max-age=31536000, immutable`. `nir`, `red`, and `SCL.tif` are siblings in the same directory, so the mask band is reachable without an account |
| `POST https://stac.dataspace.copernicus.eu/v1/search`, collection `sentinel-2-l2a`, same point, window 25 Oct to 10 Nov | 200 anonymous, granule `S2B_MSIL2A_20261110T073039_N0513_R049_T37MES_20261110T095522`, `eo:cloud_cover` 58.8, sensed the same day the request ran |
| CDSE item assets and OData endpoint | Assets are `s3://eodata/...` only, and `zipper.dataspace.copernicus.eu` answered 401 without credentials. CDSE is therefore a discovery source here, not a pixel source |

Why this route:

1. Nothing to apply for. There is no review queue, no card, no project number, and no service account between merging this and seeing imagery on a map.
2. The underlying data is Copernicus, free, full, and open including for commercial users, so the licensing cliff that Earth Engine Community and Sentinel Hub free both carry simply does not exist here. Section 12 states the triggers that would still make a paid provider worth revisiting.
3. Two independent anonymous catalogs. CDSE carries richer identifiers, its granule name encodes relative orbit `R049` and grid `37MES`, and its index reached same-day sensing in the probe. earth-search serves band COGs without an account. Using the first for provenance and the second for pixels gets both properties and free bytes.
4. The numbers are ours. We control masking, weighting, and the palette, which means the payload we hash for IOTA is reproducible from documented arithmetic rather than dependent on a third party's default parameters.

Costs accepted:

1. We own the raster math, the tile rendering, and the cache. That is the price of the previous four points and it moves the flakiest component from a vendor API to our own worker, see risk 2 in section 14.
2. The worker is Python. rasterio and rio-tiler are the dependable route. I checked the JavaScript options and they are not ready for this: `georaster` 1.6.0 and `geotiff` 3.0.5 exist on npm, `cogeotiff` is not published, and none of them handle Cloud-Optimized overview selection well enough to trust for evidence work. Do not plan on a Node-only worker.
3. Anonymous public endpoints carry no SLA and no quota promises. Both catalogs could start requiring keys. Mitigated in section 8 by keeping discovery behind an interface with two implementations and an honest `unavailable` state.
4. We must be careful about data transfer costs. See risk 3 in section 14 for the one figure this table does not settle.

Options considered and set aside:

| Option | Why not |
|---|---|
| Google Earth Engine, Community tier | Legitimate for research and the strongest per-AOI statistics tool of the set, but its license is non-commercial only, signup sits on the critical path, and it hands us temporary map-ID handles we would then have to treat as radioactive. Keep as the upgrade path if seasonal composites over thousands of plots become necessary |
| Sentinel Hub OGC and Process API | An anonymous WMS call returns `Invalid instance id`, so every request needs your own configuration and an OAuth token. Their FAQ limits trial accounts to 30,000 requests and 300 processing units per minute, and their terms forbid caching or storing imagery for commercial applications, which is directly opposed to a product that archives evidence. The evalscript and statistics API are the nicest developer experience in this comparison |
| NASA GIBS MODIS 8-day NDVI | Fully anonymous with `Access-Control-Allow-Origin: *`, and the working template is `https://gibs.earthdata.nasa.gov/wmts/epsg3857/best/MODIS_Terra_NDVI_8Day/default/{YYYY-MM-DD}/GoogleMapsCompatible_Level9/{z}/{y}/{x}.png`. The `layer-time-actual` response header is a tidy last-pass signal. Against that, `GoogleMapsCompatible_Level9` means roughly 300 m pixels at the deepest zoom, responses are marked `no-store`, and a color-mapped PNG cannot yield a number. Usable only as an optional regional context layer under `coarse_index_context`, never written into `satellite_passes` and never captioned as corroboration |
| Copernicus Data Space processing and storage | Genuinely generous, their published free quota is 12 TB transfer per month, 2000 S3 or OData requests per minute, and 50k processed requests per month with self-serve registration. It still needs an account and keys, so it is the first fallback rather than the primary |

---

## 4. Architecture

```
Android (GoogleMap + TileOverlay + lists)
      |  authenticated Retrofit/OkHttp
      v
baharimrv-api (Go, Fly.io)  ---- Neon Postgres (plots, passes, summaries, jobs)
      |  internal HTTP, shared bearer token, single-flight
      v
s2-worker (Python, Fly.io)  ---- CDSE STAC, then earth-search (both anonymous)
      |                              and Sentinel-2 L2A COGs on AWS Open Data
      v
object store (R2 or Fly volume): {project_id}/{scene_id}/{z}/{x}/{y}.png
```

Responsibilities:

1. **Go API** owns authorization, the AOI, persistence, caching policy, pass and summary records, and the anchor. It never reads a band file or queries a STAC catalog itself.
2. **s2-worker** owns every outbound imagery request. It is stateless apart from the tiles it writes, is not publicly reachable, and returns scene metadata, statistics, and rendered tiles.
3. **Android** never contacts a public data host. It fetches tiles from the Go API with the same authenticated client it uses for every call.

Two reasons tiles are proxied rather than handed to the device. First, the device cannot do the arithmetic. A raw band COG is 65 million pixels of reflectance values, not a picture, so rendering has to happen server-side regardless of where the bytes are served from. Second, SPEC.md principle 6 is privacy by default, and pointing a fleet of field phones at public data hosts would publish each project's coordinates, repeatedly and in detail, into third-party access logs. Proxying confines that traffic to one authenticated service we control, with tenant isolation enforced once, in section 9.

---

## 5. AOI and NDVI method

### 5.1 AOI from centroid plus area

Do not use the bounding box of all centroids as the analysis area. [ProjectAOI at service.go:175](backend/internal/satellite/service.go#L175) builds exactly that, and a project with scattered plots averages in farmland, settlement, and open water, which drags the change figure toward the surrounding landscape rather than the restoration. Keep the bbox, but only for tile framing and camera fitting.

Per plot, build a square cell whose area equals `area_hectares`:

```
side_m      = sqrt(area_ha * 10_000)
side_m      = max(side_m, 30)          // guarantee at least 3x3 of the 10 m bands
dlat        = side_m / 2 / 111_320
dlon        = side_m / 2 / (111_320 * cos(lat * pi / 180))
cell        = Polygon(lat ± dlat, lon ± dlon)
```

The analysis geometry covers the union of those cells, and is carried as a MultiPolygon whose members are the cells, which is the shape `s2_worker/aoi.py`'s `search_geometry` already builds and the shape `/v1/scene` accepts. A pixel mask counts a shared pixel once, so overlapping cells cover exactly their union without either side needing a boolean geometry operation. Weight each cell by its true area in hectares when computing the project mean. At 10 m resolution a 0.5 ha plot is roughly a 7 by 7 pixel block, so the 30 m floor only matters for sub-0.1 ha records.

Two honesty rules follow from this:

1. A buffered cell assumes the centroid lies inside the plot. Centroid error larger than half the side moves the sample off the trees, so record GPS accuracy and flag plots above 25 m in phase 6.
2. Report coverage, being valid unmasked pixels over expected pixels, on every summary. A summary with coverage below 60 percent is `partial` and the Android UI must say so.

### 5.2 Scene selection

For the current observation, as an anonymous STAC search against CDSE first and earth-search second, both probed in section 3:

```
POST /v1/search
  collections: ["sentinel-2-l2a"]
  intersects:  <AOI geometry>
  datetime:    <now - 45d> / <now>
  sort:        datetime descending
  filter:      eo:cloud_cover <= 40; if nothing passes, retry at <= 70 and mark quality "cloudy"
```

For the baseline, the same calendar window one year earlier, `<now - 410d> / <now - 365d>`, same rules, and store both scene identifiers on the summary. Read the granule identifier, the sensing time, and the cloud percentage from the item rather than from anything rendered, because a rendered tile is derived data. Methodology periods in `MethodologyPeriod` should drive the window once methodologies carry seasonality; until then, 45 days is the documented convention.

Two measured facts from the Kilifi spike change how the window and the screen are used.

A date-only upper bound is midnight UTC, which silently drops that morning's pass. The 2026-11-10 acquisition over Tile 37MER senses at 07:51Z; a window ending `2026-11-10` cannot see it. Send full timestamps, and treat a bare date as end of day.

The scene-wide cloud figure does not describe the plots, so it cannot be the last screen. Rank candidates by cloud tier then recency, then read only the 20 m class layer over the AOI for each of the first five and take the first whose usable fraction reaches the 60 percent floor. On 2026-11-10 over Kilifi creek the freshest granule reported 24.9 percent cloud and was 100 percent cloud class over the plot, while `S2C_37MER_20261105_1_L2A` reported 29.6 percent and was 100 percent vegetation. Recency alone would have published an empty figure, and the ladder alone would have called it clean. If nothing in five tries reaches the floor, measure the best-seen candidate and report it as `partial` with its coverage; an operator needs the number that exists plus the coverage it came with.

### 5.3 Pixel math

`NDVI = (B08 - B04) / (B08 + B04)` on Surface Reflectance bands, scale factor 0.0001.

The class layer is `SCL.tif`, and its numbering depends on the processor. Every granule the catalogs serve today is baseline 05.09 or later, where the classes are 0 no data, 1 saturated or defective, 2 dark area, 3 cloud shadow, 4 vegetation, 5 not vegetated, 6 water, 7 unclassified, 8 cloud medium probability, 9 cloud high probability, 10 thin cirrus, 11 snow. Only 4 and 5 are a surface that can be indexed, so the mask is a whitelist of those two rather than a blacklist, which keeps any class we have not mapped out of the mean instead of into it. Water exclusion matters most here: mangrove plots sit at the tidal edge and unmasked water pulls means down hard enough to fake a trend.

This table was read off the rasters, by matching a decimated class histogram against the `s2:*_percentage` fields the same item publishes. Two granules are recorded as fixtures and the test runs on both: `S2C_37MER_20261105_1_L2A` puts 17.2 percent of its valid pixels at class 9 against `high_proba_clouds` 17.52, and 12.46 at class 8 against `medium_proba_clouds` 12.11; `S2A_37MER_20261018_0_L2A` puts 30.08 at class 9 against 30.04, and 12.72 at class 8 against 15.54. A third, `S2C_37MER_20261026_0_L2A`, was matched the same way during the spike (59.4 percent at class 9 against the 59.33 its item publishes, 40.2 against 40.14); that item is in the recorded catalog, its histogram is not, so what CI checks is the two granules above. The pre-05.09 numbering put clouds at 3, 4 and 5, which is exactly where vegetation and bare ground sit now, so a granule masked with the wrong table averages the cloud and throws away the mangroves. The worker refuses any granule whose baseline is below 05.09 rather than guessing at its table, and records `msk-0509/1` in the summary so a verifier knows which table produced the number.

Measured on the 2.5 ha cell at Kilifi creek, 158 m per side, through the service's own read path and over the cell's 289 pixels of the 10 m grid and nothing else: mean 0.837, median 0.8476, coverage 100 percent on `S2C_37MER_20261105_1_L2A`, against mean 0.7979 and median 0.8107 on the year-earlier `S2C_37MER_20251110_0_L2A`, which is +0.0391 absolute and +4.90 percent. The first spike run of the same pair reported 0.8309 and 0.7933 over 400 pixels, because it averaged the read window including its 2 pixel context pad; the service slices the cell out of that window before the statistics, so the pad serves the mask reprojection and never enters a mean. The 289-pixel figures, the class table and one rendered tile are the golden fixtures in section 13, and `spike/kilifi_scene.py` now computes them through `job.py`, so a re-run re-measures the quoted numbers instead of drifting from them.

### 5.4 Rendered overlay

NDVI palette, applied to the current scene masked exactly as above:

| NDVI | Color | Meaning |
|---|---|---|
| no data or masked | transparent | shows basemap |
| < 0.10 | `#B7A57A` | bare, substrate, or shadow |
| 0.10 to 0.30 | `#E8E28A` | very low |
| 0.30 to 0.45 | `#B8DB7A` | low |
| 0.45 to 0.60 | `#6FBE44` | moderate |
| > 0.60 | `#2A7D1E` | dense canopy |

Reuse the Android marker ramp in `ndviColor` at [SatelliteMonitoringViewModel.kt:77](android/app/src/main/java/ke/co/baharilink/baharimrv/ui/satellite/SatelliteMonitoringViewModel.kt#L77) only for markers, and align its stops to the table above in phase 3 so legend and markers cannot disagree. That function currently has an unreachable branch, since 0.2 to 0.4 is listed after 0.4 to 0.6, and its low band literal `0xFF9E9E36` is olive rather than grey.

### 5.5 Revisit rows

STAC search gives acquisitions, not future passes. Replace the synthetic predicted rows rather than mixing them with real ones:

1. `pass_type = "processed"` only for a catalog-confirmed scene, with real `scene_id`, `acquired_at`, and `cloud_cover_pct`. Populate `orbit` only from a CDSE-sourced item, whose granule name encodes it, for example `..._N0513_R049_T37MES_...`. The earth-search probe in section 3 returned null for `s2:relativeOrbitNumber`, so leave the column empty rather than guessing at it.
2. `pass_type = "estimated_revisit"` with `status_detail` naming the 5 day Sentinel-2 cycle, `adapter_mode` from the provider, and no acquisition fields.
3. Never emit a row whose identifiers were derived from `shortHash(projectID)`.

---

## 6. Schema changes

One migration, `000009_satellite_ndvi_overlay.sql`, written for both `postgres` and `sqlite` directories because both are embedded at [database.go:17](backend/internal/database/database.go#L17) and the drift test only exercises SQLite. Postgres uses `jsonb` and `timestamptz`, SQLite uses `TEXT` and `TIMESTAMP`, matching the style already in each driver's `000001_init.sql`.

The runner cuts each file into statements on semicolons, so two rules follow. A comment must not contain one, or the text after it becomes a statement of its own. And a `$tag$ ... $tag$` body has to stay whole: `000008` writes its conditional ALTER as a `DO $$` block, which the old splitter cut into `DO $$ BEGIN ...`, `END IF` and `END $$`, so a fresh Postgres database failed there and never reached `000009`. `splitStatements` now reads dollar-quoted bodies as one unit, with a regression test over the real file.

### 6.1 Alter `plots`

```sql
ALTER TABLE plots ADD COLUMN boundary jsonb;              -- GeoJSON Polygon, EPSG:4326, nullable
ALTER TABLE plots ADD COLUMN gps_accuracy_m numeric(8,2); -- nullable until field capture adds it
```

No PostGIS dependency. Geometry stays GeoJSON in `jsonb` so the existing GORM models, both drivers, and the drift test keep working unchanged. Revisit PostGIS when per-plot precision becomes a requirement.

### 6.2 Alter `satellite_passes`

```sql
ALTER TABLE satellite_passes ADD COLUMN provider text NOT NULL DEFAULT '';
ALTER TABLE satellite_passes ADD COLUMN ndvi_mean numeric(6,4);
ALTER TABLE satellite_passes ADD COLUMN footprint jsonb;
ALTER TABLE satellite_passes ADD COLUMN processing_version text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_satellite_passes_scene ON satellite_passes(project_id, mission, scene_id)
  WHERE scene_id IS NOT NULL AND scene_id <> '';
```

The unique index makes a scene idempotent, which matters because refresh will be retryable. `provider` records which catalog supplied the item, `copernicus-stac` or `element84-stac`, because the two differ in how much provenance they expose: a CDSE granule name encodes orbit and processing baseline, an earth-search item does not.

### 6.3 New `project_ndvi_summaries`

The project-level change figure, one row per computation.

```sql
CREATE TABLE project_ndvi_summaries (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  period_start timestamptz NOT NULL,
  period_end timestamptz NOT NULL,
  current_scene_id text NOT NULL,
  current_acquired_at timestamptz,
  baseline_scene_id text NOT NULL DEFAULT '',
  baseline_acquired_at timestamptz,
  mean_ndvi numeric(6,4),
  baseline_mean_ndvi numeric(6,4),
  change_abs numeric(6,4),
  change_pct numeric(8,2),
  coverage_pct numeric(6,2) NOT NULL,
  current_cloud_pct numeric(6,2),
  plot_count integer NOT NULL,
  quality text NOT NULL,             -- complete | partial | cloudy | unavailable
  adapter_mode text NOT NULL,        -- live | sandbox
  provider text NOT NULL DEFAULT '',
  methodology_version text NOT NULL DEFAULT '',
  result jsonb NOT NULL DEFAULT '{}',
  result_hash text NOT NULL,
  processing_version text NOT NULL,
  computed_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL
);
CREATE INDEX idx_project_ndvi_summaries_project ON project_ndvi_summaries(project_id, period_end desc);
```

`mean_ndvi` and `current_acquired_at` are nullable, which is a deliberate change from the first draft of this table. A computation that found no usable granule still gets a row, because a manager needs to see that the system looked and what it concluded, but it has no figure and no sensing time to record. Zero cannot stand in for either: NDVI 0.0 is a real reading for bare substrate, and a zero timestamp is a date nobody measured. `current_scene_id` stays `NOT NULL` and is the empty string in that case, following the convention the rest of the schema uses for absent strings. `coverage_pct` stays `NOT NULL` at 0, since no usable pixel is what 0 percent coverage means.

### 6.4 New `satellite_refresh_jobs`

```sql
CREATE TABLE satellite_refresh_jobs (
  id uuid PRIMARY KEY,
  project_id uuid NOT NULL REFERENCES conservation_projects(id),
  status text NOT NULL,              -- queued | running | complete | failed
  requested_by text NOT NULL DEFAULT '',
  reason text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  summary_id text NOT NULL DEFAULT '',
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX idx_satellite_refresh_jobs_open ON satellite_refresh_jobs(project_id)
  WHERE status IN ('queued','running');
```

The partial unique index is the single-flight lock: one open job per project, enforced by the database rather than by memory, so concurrent Android refresh taps cannot stack up jobs on a worker sized for two at a time.

`summary_id` is text defaulting to the empty string rather than a nullable uuid, which is a change from the first draft. The Go model holds it as a plain `string`, and a queued job has no summary yet, so GORM would insert `''`; PostgreSQL refuses to cast that to a uuid. This follows the convention `000001_init.sql` already uses for an absent reference such as `current_event_id` and `superseded_by_id`, and it behaves identically on both drivers.

### 6.5 Go models

1. Add `Boundary datatypes.JSON` and `GpsAccuracyM *float64` to `Plot` at [models.go:227](backend/internal/models/models.go#L227).
2. Extend `SatellitePass` at [models.go:536](backend/internal/models/models.go#L536) with `Provider`, `NdviMean *float64`, `Footprint datatypes.JSON`, `ProcessingVersion`.
3. Add `ProjectNdviSummary` and `SatelliteRefreshJob`, and register both in `models.All()` at [registry.go:3](backend/internal/models/registry.go#L3). The drift test fails otherwise.
4. Run `go test ./internal/database/` after the migration and models land.

---

## 7. API endpoints

All public routes mount on the authenticated `session` group in [router.go](backend/internal/api/router.go), each calling `r.projects.AssertProjectAccess(c.GetString("user_id"), c.Param("id"))` before touching satellite data, exactly as the existing handlers do at [handlers.go:613](backend/internal/api/handlers.go#L613).

### 7.1 Phase 0, mount what already exists

| Method and path | Handler | Notes |
|---|---|---|
| `GET /api/v1/projects/:id/satellite/passes` | `r.projectSatellitePasses` | already written, currently unreachable |
| `GET /api/v1/projects/:id/satellite/latest` | `r.projectSatelliteLatest` | already written, currently unreachable |

Add a regression test asserting both paths appear in the route list, since nothing catches this class of defect today.

### 7.2 New endpoints

| Method and path | Purpose | Access |
|---|---|---|
| `GET /api/v1/projects/:id/satellite/overlay` | Tile base URL, provenance, legend stops, AOI bbox | project member |
| `GET /api/v1/projects/:id/satellite/tiles/{z}/{x}/{y}.png` | Authenticated tile proxy | project member |
| `GET /api/v1/projects/:id/satellite/change` | Project-level mean and change | project member |
| `POST /api/v1/projects/:id/satellite/refresh` | Queue a compute job, returns 202 | manager or admin |
| `GET /api/v1/projects/:id/satellite/refreshes/:jobId` | Job status for the UI spinner | project member |
| `GET /api/v1/admin/satellite/health` | Worker reachability, which discovery sources answered, tile store writability, failed and stuck jobs, cache hit rate | admin, mirrors [router.go:216](backend/internal/api/router.go#L216) |
| `PATCH /api/v1/projects/:id/plots/:plotId` | Accepts optional `boundary` GeoJSON | manager (existing route, extended body) |

### 7.3 Shapes

`GET .../satellite/overlay` returns the addressing the map needs plus the provenance the label needs:

```json
{
  "tileBaseUrl": "/api/v1/projects/8c2.../satellite/tiles",
  "layer": "ndvi",
  "sceneId": "S2A_MSIL2A_20261029T080351_N0510_R121_T37MBT_20261029T113403",
  "mission": "S2",
  "acquiredAt": "2026-10-29T08:03:51Z",
  "cloudCoverPct": 12.4,
  "quality": "complete",
  "adapterMode": "live",
  "provider": "opendata-s2",
  "discoverySource": "copernicus-stac",
  "processingVersion": "ndvi/s2-l2a-opendata/1",
  "bbox": { "minLat": -3.71, "minLon": 39.18, "maxLat": -3.55, "maxLon": 39.36 },
  "zoomRange": { "min": 10, "max": 16 },
  "legend": [
    { "min": 0.10, "max": 0.30, "color": "#E8E28A", "labelKey": "satellite.legend.verylow" }
  ],
  "cacheExpiresAt": "2026-11-11T06:00:00Z",
  "summaryId": "4f9..."
}
```

Sandbox or unconfigured responses keep the same envelope so the client never has to branch on structure. `adapterMode` is `"sandbox"`, `sceneId` is empty, `quality` is `"synthetic"`, and `tileBaseUrl` is absent. The client then draws the no-imagery state rather than a fake layer.

`cacheExpiresAt` is our own re-check deadline, not a credential or a vendor handle. Before it, a cached overlay is still correct; after it, the client may ask again to see whether a newer scene has been ingested. Nothing on this envelope can expire in a way that invalidates pixels already drawn, because the tiles are addressed by scene identifier.

`GET .../satellite/change`:

```json
{
  "summaryId": "4f9...",
  "meanNdvi": 0.4231,
  "baselineMeanNdvi": 0.3918,
  "changeAbs": 0.0313,
  "changePct": 8.0,
  "periodStart": "2026-10-15T00:00:00Z",
  "periodEnd": "2026-11-10T00:00:00Z",
  "currentSceneId": "S2A_MSIL2A_20261029T080351...",
  "baselineSceneId": "S2B_MSIL2A_20251101T081222...",
  "currentAcquiredAt": "2026-10-29T08:03:51Z",
  "coveragePct": 91.2,
  "plotCount": 14,
  "quality": "complete",
  "adapterMode": "live",
  "provider": "opendata-s2",
  "processingVersion": "ndvi/s2-l2a-opendata/1",
  "resultHash": "sha256:...",
  "anchor": { "state": "anchored", "transactionReference": "...", "chainMode": "testnet" },
  "computedAt": "2026-11-10T05:12:44Z"
}
```

`currentAcquiredAt`, `provider` and `processingVersion` are present so the client can label the scene and the arithmetic that produced the number, as section 11 requires. Absent figures stay `null` rather than becoming zero, because NDVI 0.0 is a real reading for bare substrate. `anchor` is not returned until phase 5 writes one, because the client must not be told a commitment was anchored when nothing was submitted.
```

`POST .../satellite/refresh` returns `202` with the job envelope on success, `409` with the open job's envelope when one is already running, and `429 satellite_refresh_rate_limited` when the project has already queued six refreshes in the last hour. The lock is the partial unique index, so two devices tapping refresh cannot stack jobs on a worker sized for two. The rate limit counts rows inserted, not taps: a tap answered by a 409 never reaches the worker and so is not what the limit is protecting.

`GET .../refreshes/:jobId` returns the same envelope and never the worker's or a catalog's raw error text.

`GET .../tiles/{z}/{x}/{y}.png` requires the `layer` and `scene` query parameters and matches `scene` against the project's active summary. It returns `404` for coordinates outside the AOI bbox, `410` when the requested scene has been superseded by a newer computation or its tiles were evicted from the store, so the client re-requests `overlay` instead of painting a gap, and `200` with PNG bytes otherwise, plus `Cache-Control: private, max-age=3600` and an `ETag`. Reject `z` above 16 or below 8 to stop a single project's tiles being walked as a global dump. Because the address carries the scene identifier, a client can never be served pixels from a scene it was not told about, and a stale cache entry is impossible by construction rather than by TTL discipline.

### 7.4 Error codes

| Situation | HTTP | code |
|---|---|---|
| No plots on the project, so no AOI | 200 with `quality: "unavailable"` | n/a |
| No summary computed yet, so the client should offer a refresh | 404 | `satellite_summary_missing` |
| Provider not configured, or catalogs answered with nothing usable | 200 with `quality: "unavailable"` | n/a |
| Worker call failed or timed out | 502 | `satellite_provider_failed`, retried by the job, never written as a pass |
| Tile requested for a superseded or evicted scene | 410 | `satellite_scene_superseded` |
| Another project's tile requested | 403 | `forbidden` |
| All scenes in the window over cloud threshold | 200 with `quality: "cloudy"` | n/a |
| More than six refreshes queued for one project in the last hour | 429 | `satellite_refresh_rate_limited` |

---

## 8. s2-worker service

New directory `services/s2-worker/`, Python 3.12, `rasterio`, `rio-tiler`, `numexpr`, `fastapi` and `uvicorn`, deployed as a second Fly.io app with internal-only networking and no public port, mirroring the deployment shape of `executor/`. It is the only component in the system that talks to external imagery hosts.

Internal API, authenticated with `Authorization: Bearer ${S2_WORKER_TOKEN}`:

| Route | Input | Output |
|---|---|---|
| `POST /v1/scene` | `{ aoi_geojson, window_days, max_cloud_pct, end_time }` | `{ scene_id, acquired_at, cloud_pct, mission, grid, relative_orbit, processor_baseline, footprint_geojson, discovery_source, bands: { red, nir, scl } }` where the band values are COG URLs. This route runs the coverage screen in 5.2 itself, because the screen is a pixel read, and returns the granule that passed rather than the newest one that looked clean. `screened` lists each candidate tried with its usable fraction, so the walk is auditable. |
| `POST /v1/stats` | `{ cells: [{ plot_id, geometry }], scene: { bands, ... }, weight_by_area }` | `{ scene_id, acquired_at, cloud_pct, mean_ndvi, median_ndvi, coverage_pct, quality, class_table, palette_version, per_cell: [{ plot_id, mean_ndvi, valid_pixels, total_pixels }] }` |
| `POST /v1/tiles/render` | `{ bbox, zoom_min, zoom_max, scene, palette_version, key_prefix }` | `{ scene_id, key_prefix, palette_version, class_table, quality, tiles_rendered, tiles_empty, tiles_omitted, bytes_written, pixels_read, zoom_min, zoom_max, notes }` |
| `DELETE /v1/tiles` | `{ key_prefix }` | `{ key_prefix, purged }` |
| `GET /v1/health` | | `{ status, version, discovery_sources_reachable, tile_store_writable, tile_store_backend, last_error, palette_version, class_table, limits, band_hosts }` |

Failures are typed so the API never has to parse a message: every non-2xx body is `{ "error": <code>, "detail": <text>, "notes": [...] }` with `error` one of `unauthorized`, `bad_request`, `unavailable` (HTTP 404, the honest empty), `band_unavailable` (502), `store_unavailable` (503), `busy` (503, the concurrency cap was still full after a 30 s queue wait), `palette_mismatch` (409) or `internal`. `GET /v1/health` needs no token, because the platform probes it and it holds no project data and starts no imagery fetch. It reports discovery reachability from the last real job rather than pinging two public catalogs on every probe.

The worker holds no project state and performs no authorization. The Go API decides which AOI belongs to which caller, chooses the object-store key prefix, and persists every result. That split keeps this service replaceable.

Rules:

1. **Discovery failover, always recorded.** Query CDSE STAC first, then earth-search. Both are anonymous. Write whichever supplied the scene into `discovery_source`, which becomes `provider` on the pass row, so a provenance reader can trace the choice. If a source returns an item whose COG URL 404s, fall through to the other source rather than failing the job.
2. **Never invent a scene.** No item in the window means a typed `unavailable` response, which the API renders as the existing unavailable state. No partial guessing.
3. **Bounded fetches, and bound the thing we can measure.** Windowed reads only, never a whole band. The spike measured what a read actually costs: opening one remote COG and pulling a 20 x 20 pixel window is about six HTTP exchanges and 9 to 11 seconds from a client far from `us-west-2`, while the second read of the same file inside the same process is 0.01 seconds. Whole spike, two scenes measured, two candidates screened, two z14 tiles: 91 seconds and 1,574,752 pixels. So the ceilings are pixels returned per job and wall clock, both of which the worker can count itself, plus a maximum AOI extent per zoom level. Range requests are GDAL's, not ours; report them from its debug stream as an operator metric instead of pretending to enforce them. Exceed any guard and return `partial` with the coverage figure you actually achieved rather than a full-coverage claim.
4. **One window per project, not one per plot.** Because the cost is per remote open, reading each plot cell separately is three opens times the plot count: 40 plots would be 120 opens, roughly twenty minutes on the measured link, for pixels that lie inside one rectangle. Read the union bounding box of all plot cells once, then take zonal stats per cell out of that array. The ceiling in rule 3 caps the union extent, which is what keeps this honest at 400 plots.
5. **Concurrency cap 2** in the container, one job per worker process, so a burst of refresh taps from the pilot cannot saturate its own CPU or hammer two public services.
6. **Masking is nearest, not bilinear.** SCL is a 20 m class raster; reproject with nearest neighbour onto the 10 m grid. Interpolating a class raster manufactures cloud-free pixels.
7. **Palette lives in one module** and is exported through `/v1/health` plus the render payload, so server-rendered tiles and the Android legend cannot drift apart.
8. **Cache keys carry the scene.** Object keys are `{project_id}/{scene_id}/{z}/{x}/{y}.png`. A new scene never overwrites an old tile, and purging is by prefix when a project adopts a different scene.
9. **CI is fixture-driven.** Record STAC responses and band windows in `services/s2-worker/tests/fixtures/`, and run all tests without network. A golden-image test compares rendered tiles against committed PNGs for a fixed Kilifi scene.

Environment: `S2_WORKER_TOKEN`, `STAC_PRIMARY_URL` (`https://stac.dataspace.copernicus.eu/v1`), `STAC_FALLBACK_URL` (`https://earth-search.aws.element84.com/v1`), `COG_ALLOWED_HOSTS` (host allow list for band reads, default `sentinel-cogs.s3.us-west-2.amazonaws.com`), `TILE_STORE_BACKEND` (`r2` or `volume`), `TILE_STORE_BUCKET`, `TILE_STORE_ENDPOINT`, `TILE_STORE_ACCESS_KEY_ID`, `TILE_STORE_SECRET_ACCESS_KEY`, `MAX_PIXELS_PER_JOB` (8000000), `JOB_TIMEOUT_S` (120), `MAX_CONCURRENCY` (2), `MIN_COVERAGE_PCT` (60), `MAX_SCENE_TRIES` (5). Each route call gets its own budget, so a ceiling spent on `/v1/scene` cannot truncate a later `/v1/stats` for the same project.

Sizing: one shared-cpu-1x with 512 MB is enough for the pilot. The image is roughly 250 MB because of the GDAL and rasterio wheels, so build it with a layer-cached Dockerfile rather than expecting fast rebuilds.

---

## 9. Go backend changes

1. Introduce a provider seam in `internal/satellite`:

```go
type Provider interface {
    Name() string
    LatestObservation(ctx context.Context, aoi AOI, window time.Duration) (SceneStats, error)
    BaselineObservation(ctx context.Context, aoi AOI, window time.Duration) (SceneStats, error)
    TileSet(ctx context.Context, aoi AOI, scene SceneStats) (TileSet, error)
}
```

   A `TileSet` is `{KeyPrefix, PaletteVersion, ZoomMin, ZoomMax, SceneId}`, which is an address within our own store rather than a credential, so it has no lifetime and cannot expire mid-session.

   `SandboxProvider` returns today's synthetic values with `AdapterMode: "sandbox"` and no tile set. `OpenDataStacProvider` calls s2-worker over internal HTTP; its `Name()` is `opendata-s2`. `NewService` picks by `SATELLITE_PROVIDER`, defaulting to sandbox, and a live deployment whose worker is not configured returns the existing `unavailable` result instead of a success. That preserves principle 9 in SPEC.md, honest degraded states.

   The label comes from the provider that produced the figure, not from `SATELLITE_MODE`. A sandbox provider can only ever answer `adapterMode: "sandbox"` with `quality: "synthetic"`, so no combination of environment variables can present a modelled number as imagery, and `SATELLITE_MODE=live` with the provider left unset still yields a sandbox label rather than a live one. Only the worker's typed `unavailable` reply, which means the catalogs answered and nothing usable was in the window, degrades an observation to `unavailable` without an error; a refused call, a bad token, a full queue or a dead host is returned as an error so the refresh job records the fault instead of writing a period that never existed. Tiles are the same rule: a render that comes back `partial` is a failure, because a layer with holes in it reads on a map as bare substrate.

2. Bump `ProcessingVersion` from `"sandbox-ndvi/1"` at [service.go:21](backend/internal/satellite/service.go#L21) to `"ndvi/s2-l2a-opendata/1"` for live rows, keeping the sandbox constant untouched so old rows stay interpretable.

3. Replace the centroid-sum AOI in `ProjectAOI` with the buffered-cell union from section 5.1, and return both `AnalysisGeometry` (cell union, GeoJSON) and `FramingBBox`. Persist both on the summary so the method is auditable.

4. On job completion, write `project_ndvi_summaries` with `canon.Hash(result)` and the canonical `result` payload, reusing the pattern in `AttachSandbox` at [service.go:70](backend/internal/satellite/service.go#L70). The hashed payload must contain scene IDs, acquisition times, means, coverage, plot count, processing version, class table version, palette version, and the processor baseline of each scene, so a verifier can re-derive the number and know which mask produced it.

5. Anchor it through the existing generic path. Call the executor `anchor` operation with `kind = "satellite_ndvi"`, `commitment = result_hash`, `idempotency_key = summary id`, and record the transaction in `provenance_events` plus `chain_object_ids`. Two notes: fix the missing `break` after `case 'anchor'` at [executor/src/index.ts:113](executor/src/index.ts#L113) first, since today an anchor also runs `create_ledger`; and follow the existing hash-then-encode convention in `backend/internal/chain/iota.go` rather than hashing in the worker.

6. Add `SATELLITE_PROVIDER`, `S2_WORKER_URL`, and `S2_WORKER_TOKEN` to `config.Config` next to `SatelliteMode` at [config.go:127](backend/internal/config/config.go#L127), and expose `provider` rather than only `SATELLITE_MODE` in `health` at [router.go:245](backend/internal/api/router.go#L245). Keep `/health` unauthenticated but never include tokens or worker internals there; discovery source reachability and job backlog live behind the admin route.

7. Tile proxy handler in `internal/api`: authorize, load the project's active summary, compare the `scene` query parameter against the scene recorded on that summary, validate `z` against the summary's zoom range, then read the object-store key `{project_id}/{scene_id}/{z}/{x}/{y}.png` the worker wrote. Keep a bounded in-memory LRU keyed by project plus scene plus tile so a pan across a small AOI does not hit the store per tile. On a scene mismatch or a missing key, serve `410` and queue a re-render rather than erroring at the user, and never fall back to rendering another scene's pixels.

8. Dashboard: extend `NDVIState` at [mrv/project.go:75](backend/internal/mrv/project.go#L75) with `MeanNDVI`, `ChangePct`, `Quality`, `CoveragePct`, `AdapterMode`, `SceneId` and `Provider`. Drive the headline from `ProjectChange` rather than the per-activity cherry-pick at [project.go:409](backend/internal/mrv/project.go#L409): the old loop picked the maximum NDVI across activities, which could be honest about each plot and wrong about the project. Per-plot `view.NDVI` stays derived from `satellite.Latest(activity.ID)`, because a manager looking at one marker still needs to see what that marker read.

---

## 10. Android

### 10.1 Build and configuration

1. Add `GOOGLE_MAPS_API_KEY` to `android/local.properties` and to the deployment secret store, and document it next to the existing keys. Without it `config()` returns an empty string and every `GoogleMap` fails authentication.
2. Add `buildConfigField("boolean", "MAPS_CONFIGURED", ...)` beside the `authConfigured` block at [build.gradle.kts:37](android/app/build.gradle.kts#L37) and gate the map composable on it, so a checkout without a key shows the list UI instead of a broken map.
3. Restrict the key to the Android application ID and the Maps SDK for Android. Maps SDK usage is billed by Google, so treat it as a real dependency even in research.

### 10.2 Data layer, `data/remote/Api.kt`

```kotlin
@GET("api/v1/projects/{id}/satellite/overlay")
suspend fun satelliteOverlay(@Path("id") projectId: String): SatelliteOverlayDto

@GET("api/v1/projects/{id}/satellite/change")
suspend fun satelliteChange(@Path("id") projectId: String): NdviChangeDto

@POST("api/v1/projects/{id}/satellite/refresh")
suspend fun satelliteRefresh(@Path("id") projectId: String): SatelliteRefreshDto

@GET("api/v1/projects/{id}/satellite/refreshes/{jobId}")
suspend fun satelliteRefreshStatus(
    @Path("id") projectId: String,
    @Path("jobId") jobId: String,
): SatelliteRefreshDto

@GET("api/v1/projects/{id}/satellite/tiles/{z}/{x}/{y}")
suspend fun satelliteTile(
    @Path("id") projectId: String,
    @Path("z") z: Int,
    @Path("x") x: Int,
    @Path("y") y: Int,
    @Query("layer") layer: String,
    @Query("scene") scene: String,
): okhttp3.ResponseBody
```

DTOs mirror section 7.3 with defaults on every field so a sandbox payload deserializes. `SatelliteOverlayDto` carries `tileBaseUrl`, `bbox`, `zoomRange`, `legend`, `sceneId`, `acquiredAt`, `cloudCoverPct`, `quality`, `adapterMode`, `cacheExpiresAt`. Add the same to `SatellitePassDto` at [Api.kt:658](android/app/src/main/java/ke/co/baharilink/baharimrv/data/remote/Api.kt#L658) plus `processingVersion` and `quality`, since it currently has no tile or geometry information at all.

Extend `ProjectRepository` next to the existing `satellitePasses` and `satelliteLatest` wrappers around [ProjectRepository.kt:37](android/app/src/main/java/ke/co/baharilink/baharimrv/data/repository/ProjectRepository.kt#L37).

For tiles, expose a dedicated `@Named("tiles") OkHttpClient` in `di/AppModule.kt` built from the same auth interceptor, with a disk `Cache` of 20 MB in `context.cacheDir/satellite-tiles`. Cache responses by `Cache-Control` so offline reads of the last viewed zoom band work, and mark the UI as cached imagery when they are.

### 10.3 Tile provider

`ui/satellite/NdviTileProvider.kt`, implementing `com.google.android.gms.maps.model.TileProvider`, which must be thread-safe:

```kotlin
class NdviTileProvider(
    private val projectId: String,
    private val source: SatelliteOverlayDto,
    @Named("tiles") private val client: OkHttpClient,
) : TileProvider {
    override fun getTile(x: Int, y: Int, zoom: Int): Tile {
        if (zoom !in source.zoomRange.min..source.zoomRange.max) return NO_TILE
        if (!source.bbox.containsTile(x, y, zoom)) return NO_TILE
        return runCatching {
            val body = fetch(x, y, zoom)
            Tile(body.bytes())
        }.getOrElse { UNKNOWN_TILE }
    }
}
```

`Tile(byte[])` accepts PNG bytes directly, so no bitmap decoding. Return `NO_TILE` for area outside the project, which lets the basemap show through, and `UNKNOWN_TILE` for network failure so the SDK retries. On HTTP 410, which here means the scene this provider was built for has been superseded by a newer computation or its tiles were evicted from the store, notify the ViewModel to refetch `overlay` once, then rebuild the provider with the new scene. Never cache `UNKNOWN_TILE`.

### 10.4 Screen

Restructure `ui/satellite/SatelliteMonitoringScreen.kt`, starting by making it compile: delete the local `private enum class ChipTone` at line 235 and use the imported `ui.components.ChipTone`, whose `stateTone(status)` helper at [Common.kt:89](android/app/src/main/java/ke/co/baharilink/baharimrv/ui/components/Common.kt#L89) already maps sandbox and pending states, and add the missing `Row`, `Spacer`, `height`, and `PaddingValues` imports from `androidx.compose.foundation.layout`.

Layout, top to bottom in a scrollable `Column` rather than the current nested `LazyColumn`, which cannot host a map of fixed height:

1. **Imagery card.** `GoogleMap` at 280 dp inside a `BahariCard`.
   - `MapProperties(mapType = MapType.HYBRID, isMyLocationEnabled = false, isTrafficEnabled = false)`
   - `MapUiSettings(zoomControlsEnabled = false, scrollGesturesEnabled = true, zoomGesturesEnabled = true, rotationGesturesEnabled = false, myLocationButtonEnabled = false)`
   - `TileOverlay(state = rememberTileOverlayState(), tileProvider = ndviTileProvider, alpha = 0.65f, zIndex = 2f)`
   - one `Marker` per plot, tinted by `ndviColor`, with `contentDescription` naming plot code and NDVI band, not the raw number
   - camera animated to the bbox on first composition via `CameraPositionState.animate(CameraUpdateFactory.newLatLngBounds(..., 48.dp))`, remembering the camera across configuration changes
   - overlay chrome: legend strip bottom-left, `StatusChip` top-right reading `adapterMode` through `stateTone`
2. **Last pass card.** Scene identifier, `acquiredAt` as relative age plus absolute timestamp, cloud percentage, quality chip. When `adapterMode == "sandbox"`, render the existing sandbox wording and no imagery card at all.
3. **Change card.** Mean NDVI, baseline mean, change with an explicit `+/-`, coverage percentage, plot count, and an anchored chip sourced from `anchor.state` and `chainMode`. Below 60 percent coverage, add a `InlineNotice`-style warning that the figure is partial.
4. **Passes list.** Keep the list, relabel it to acquisitions, and drop the empty-state copy that promises "provider-neutral tile match" once real scenes populate it.
5. **Refresh action.** Replaces `viewModel::retry` at the current bottom button. Disabled and labelled "Checking for new imagery" while a job is open, and hidden for the field monitor role.

### 10.5 ViewModel

Add to `SatelliteMonitoringUiState` at [SatelliteMonitoringViewModel.kt:18](android/app/src/main/java/ke/co/baharilink/baharimrv/ui/satellite/SatelliteMonitoringViewModel.kt#L18): `overlay`, `change`, `refreshJob`, and `activeScene`. `load()` fetches overlay, change, latest, and passes in one coroutine with `coroutineScope { ... }`, failing the screen only if overlay fails. Add `refresh()` that posts then polls `refreshes/{jobId}` every 3 s to a 90 s deadline, and a `sceneSuperseded()` signal fed from the tile provider. `tileProvider` is rebuilt only when `activeScene` changes, so camera moves do not churn it.

Note the existing `plotMarkers()` at line 50 and line 93 is invoked twice per composition and reads `_state` directly; move it to a derived flow while touching the file.

### 10.6 Navigation

Change `Routes.SATELLITE` at [AppRoot.kt:80](android/app/src/main/java/ke/co/baharilink/baharimrv/ui/AppRoot.kt#L80) to `"satellite/{projectId}"` with a `navArgument`, mirroring `ADD_PLOT` at line 86, and update the caller at line 175 to pass `selection`. The screen already takes `projectId` as a parameter, so this is a plumbing fix that also makes the screen deep-linkable from project detail.

### 10.7 Strings and accessibility

All new labels go through `stringResource` with English and Swahili entries, including the six legend band names and the four quality states. Legend text stays at least 12 sp with 4.5:1 contrast against its scrim. The overlay must not carry meaning by color alone, so every color stop carries a word, and the sandbox or partial state is always stated in text.

---

## 11. Provenance and labeling rules

Non-negotiable, per SPEC.md principles 4 and 9 and the standing rule against fabricated integrity states:

1. `Live: true` may only come from a real scene identifier and acquisition time returned by a STAC catalog search. No client-side derivation.
2. Sandbox and live rows never mix in one response, and the sandbox label uses the existing wording pattern.
3. The device never holds a credential of any kind, an upstream STAC or COG URL, or an object-store key. It holds only our own proxy path and the scene identifier to put in the query string.
4. Anchor the hash of the canonical result payload, never a URL or an object key. Tiles are re-rendered under a new prefix whenever a project adopts a scene; the arithmetic does not move.
5. Corrections are additive. A recomputed summary is a new row; the old hash and its anchor stay queryable.
6. A tile that fails to load is a visible gap, never a stale scene presented as current. Show the acquisition date over the map.
7. Attribution names Copernicus and the European Commission as the data source, plus the catalog that served the item, and appears in the app's about or data-sources screen. Nothing on this route bars caching our own derived tiles, which is why the object store and the Android disk cache are both allowed.

---

## 12. Delivery phases

No phase below waits on an external account, an application, or a vendor review. That was the point of the route change, so treat any new "we need approval first" finding as a design defect worth stopping on.

| Phase | Deliverable | Effort | Depends on |
|---|---|---|---|
| 0 | Mount the two satellite routes with a regression test, fix the satellite screen compile errors, wire the Maps key with a `MAPS_CONFIGURED` gate, fix the executor `anchor` fall-through | 0.5 to 1 day | none |
| 1 | Spike, then skeleton. Half a day proving one real Kilifi scene end to end from a CLI, scene to pixels to a PNG on disk, before any service code. Then `services/s2-worker/` with discovery failover and fixtures, the `Provider` seam in Go, and real scenes filling `satellite_passes` | 2 to 3 days | 0 |
| 2 | AOI buffering, windowed band reads, zonal stats with area weighting, `project_ndvi_summaries`, `/satellite/change`, dashboard headline switch | 2 to 3 days | 1 |
| 3 | Tile pyramid rendering to the object store, authenticated Go proxy with LRU and scene-scoped keys, `/satellite/overlay`, legend contract | 2 to 3 days | 2 |
| 4 | Android map, tile provider, restructured screen, ViewModel polling, EN and SW strings | 2 to 3 days | 3 |
| 5 | Anchor the summary hash, expose anchor state on change and proof payloads | 1 day | 2 |
| 6 | Boundary capture in Add Plot, GPS accuracy storage, per-plot precision upgrade | later, separate spec | 2 |

The research pilot needs 0 through 4. Phase 5 makes the figure verifiable, which matters before any verifier relies on it. Phases 1 to 3 are one engineer's week and a half, and because nothing is gated on a vendor they can run back to back, which is the main schedule gain over the Earth Engine plan.

**What changes if BahariMRV becomes commercial.** Nothing about the imagery. Copernicus Sentinel-2 data is free, full, and open for commercial users, so the licensing cliff that would have arrived with Earth Engine Community or Sentinel Hub free does not exist on this route. The two dependencies that remain are Google Maps SDK billing for the basemap, which is ours regardless of imagery choice, and the anonymous public endpoints covered by risk 1 in section 14.

**Triggers that would make a paid provider worth reopening:**

1. Need for sub-10 m resolution, which means PlanetScope or SkySat rather than Sentinel-2.
2. Persistent cloud during the long rains making optical revisit useless, which means Sentinel-1 radar and an all-weather index.
3. Thousands of plots with daily refresh, where the worker's compute time stops being trivial and a hosted statistics API wins on cost.
4. Either STAC catalog starting to demand keys or rate-limit anonymous search, which turns CDSE with a free self-serve account into the primary source. That switch is a config change plus one adapter, because rule 1 in section 8 keeps discovery behind an interface.

---

## 13. Verification

Backend:

1. `go test ./internal/...` plus `go test ./internal/database/`: the drift check compares the SQLite migrations against every registered model, and a splitter regression test reads the real `postgres/000008` file to prove a `DO $$` block stays one statement. No PostgreSQL server runs in CI, so the Postgres DDL is reviewed against the constructs already in the committed migrations rather than executed.
2. AOI unit tests: a 0.5 ha plot at latitude -3.6 produces a side near 71 m, cells overlap into one geometry when two plots sit 40 m apart, and a project with no plots yields `quality: "unavailable"` with no worker call.
3. Provider contract tests against recorded s2-worker fixtures: `backend/internal/satellite/provider_test.go` runs against `testdata/`, which `services/s2-worker/tests/record_route_fixtures.py` writes out of the same `job.py` functions the routes call, so the seam is checked against the service rather than against a description of it. The golden figures arrive intact, the granule search sends a full-timestamp upper bound and the baseline asks for the same window one year earlier, the cells and the scene reference come back to `/v1/stats` unchanged, the quality of a figure is the worse of the two answers, a typed `unavailable` becomes the empty state with no measurement attempted, and every other failure, a busy queue or a wrong token included, is an error rather than a period with no imagery. The catalog-level cases, STAC timeout, both catalogs empty, an item whose COG URL 404s so the job fails over to the other source, belong to the worker's route tests, because that is where the catalogs are read.
4. Authorization tests: tenant A cannot fetch tenant B's tiles, an unauthenticated tile request is rejected, zoom outside 8 to 16 is rejected, and a tile request naming a scene that is not the project's active one returns `410`.
5. Summary computation tests: a sandbox figure is labelled synthetic with no acquired time, no scene id and no invented granule; a re-run an hour later with the same measurement writes a new row with the same hash so the old commitment survives; a current scene measured at 41 percent coverage produces a `partial` row rather than `complete`; a missing baseline does not hide the measured current mean; a zero baseline publishes no percentage; a provider fault writes no row and answers the stable `ErrProviderFailed`; a project with no plots answers a stored `unavailable` row with `plot_count = 0` and no worker call; an unknown project is refused rather than orphaned; the latest computation is read by period end then computed_at then id.
6. Refresh queue tests: one job is the single-flight lock, a second tap answers with the open job's view, the worker runs the computation and records the summary id, a provider fault retries `jobAttempts` times then writes `failed` with the stable sentence rather than the worker's own text, an orphan job (project gone) fails with the stable refusal, a dead process's running job is reaped at `stuckJobAfter` and the lock is freed, a job inside its own timeout is not reaped, and a cancelled context stops the loop without leaving a half-finished pass.
7. Handler integration tests against the assembled graph: `GET /projects/:id/satellite/change` answers `404 satellite_summary_missing` before any refresh, `POST /projects/:id/satellite/refresh` is manager-only, the 202 body carries the queued job id and a 409 answers with the same id when a job is already open, `GET /projects/:id/satellite/refreshes/:jobId` is scoped to its project, and after a refresh the change response carries the project-level envelope.
5. Handler tests asserting `adapter_mode` is `sandbox` whenever `SATELLITE_PROVIDER` is unset.

Worker: 189 fixture-driven tests with no network in CI. The arithmetic and job layers cover the AOI cells, band reads and retries, NDVI and the class mask, the two-tier cloud ladder, the coverage screen, the palette, tile rendering, and the STAC sources; the route layer (`tests/test_app.py`, 44 tests) covers the bearer check, every typed error code in section 8, the concurrency cap, the palette refusal, a key prefix that is not one project and one scene, a band URL on an unlisted host, and the render-to-purge round trip. Two golden fixtures pin the output: one stats response (289 pixels, mean 0.837 on `S2C_37MER_20261105_1_L2A`) and one PNG per palette version, compared byte for byte rather than by loose pixel similarity, since a palette change is a version bump and not a rounding difference. The ceiling tests assert `partial` rather than a full-coverage result. The class table gets its own test: a recorded STAC item and its raster histogram must agree with the `s2:*_percentage` metadata for two granules, one under the cloud ceiling and one over it, so a silent renumbering upstream fails a build instead of poisoning an anchored figure.

Android: JVM tests for the ViewModel on sandbox, live, partial-coverage, scene-superseded, and offline-cached states; a pure-function test for tile bbox containment and zoom clamping; device QA on a Kenya project covering pan and zoom, legend readability, sandbox labelling, and airplane-mode behavior. The map itself needs a device or emulator, since Compose unit tests cannot verify tile rendering.

Definition of done for the pilot: a manager opens the satellite screen on a real project, sees Sentinel-2 NDVI over their own plots on the map, sees the acquisition date and cloud cover of the scene they are looking at, sees one project-level change number with its coverage and quality state, sees the sandbox label whenever live imagery is absent, and a hash of that change figure is retrievable on IOTA testnet.

---

## 14. Risks

1. **Anonymous endpoints carry no promises.** Both STAC catalogs are unsponsored public services, and either could start requiring keys, rate-limit anonymous search, or move hosts. I already saw one host in their documentation family fail to resolve from this network, which is exactly the kind of churn to expect. Mitigated by two independent sources behind the interface in section 8, a host allow list rather than a hardcoded base URL, the free self-serve CDSE account as the first escalation, and an honest `unavailable` state instead of a synthetic pass when both fail.
2. **The worker is now the flakiest component.** Vendor risk became our risk. GDAL and rasterio wheel friction, memory blowups on oversized windows, JP2 versus COG quirks, partial reads, and retries all land on us. Mitigated by the phase 1 spike before service code, bounded fetches, golden-image fixtures, `partial` as a first-class quality value, and jobs that are retryable because the scene index is unique.
3. **Egress costs are unproven, not free by proof.** The `sentinel-cogs` bucket is distributed through AWS Open Data and public-access requests to sponsored datasets normally carry no data-transfer charge, but I did not verify the sponsorship terms for this bucket. Confirm before launch, because the assumption is load-bearing at scale. If it does not hold, the fixes are running the worker in `us-west-2` beside the data, mirroring tiles in our own store, or switching band reads to CDSE with an account. The per-job volume the worker asks for is bounded by `MAX_PIXELS_PER_JOB`, and the 2026-11-10 spike run on a client far from `us-west-2` gives some idea of what that costs in time rather than money: a scene screen of 16.8 s, a project-level measure job of 106 s for 1,232 pixels read, and a two-tile z14 render job of 167 s for 786,432 pixels, which blew the 120 s wall clock and came back `partial` with zero tiles stored. One machine on one link is not a pricing figure, but it does say the region choice is not cosmetic, and that a render job must be split across requests rather than squeezed by raising the ceiling.
4. **"Last pass" can mislead.** Surface reflectance products reach these catalogs hours to days after sensing, and the two indexes differ, the CDSE probe found a same-day granule while the earth-search index led with a scene three days older. The UI must show the acquisition date over the map and the ingest lag wherever a refresh is offered, never imply imagery from today.
5. **Cloud and tidal water near the Kenyan coast.** The 45 day window, the cloud ladder, and SCL class masking address most of it. The residual case is a genuinely cloudy quarter, which must read as `cloudy` rather than as no change. Consider the cirrus band as a secondary check for thin cloud over mangrove canopy.
6. **Centroids are not boundaries.** Project-level means over buffered cells are defensible. Per-plot claims are not, which is why phase 6 exists and why nothing in this spec claims otherwise.
7. **Maps SDK key and billing.** `GOOGLE_MAPS_API_KEY` is currently absent from `android/local.properties`, so the key resolves to an empty string and no map renders at all. Beyond the key, Google's own usage cap applies to the basemap, which is a second billing dependency unrelated to imagery and worth stating plainly.
8. **Stale tiles are our bug class now.** There is no expiring vendor handle to blame, so a cache key that omits the scene identifier will quietly serve last month's pixels under this month's label. Scene-scoped keys in section 8, a `scene` query parameter on every tile request in section 7, and the Android client rebuilding its provider only when the active scene changes in section 10.5 are one mechanism doing one job. Treat any report of mismatched imagery as a cache-key defect first.
9. **NDVI on mangrove canopy is a weak proxy for carbon.** NDVI saturates in dense canopy, so a healthy stand may show little change while biomass grows. If the research narrative leans on this figure, evaluate EVI, which uses the blue band that is also sitting in the same COG directory, or a canopy-height proxy before making NDVI a headline metric.
10. **The class table is a correctness risk, not a preference.** Sentinel-2 L2A renumbered its per-pixel class layer at processing baseline 05.09, when SCL became MSK. Under the old numbering clouds sit at 3, 4 and 5, which is exactly where vegetation and bare ground sit under the new one, so an inherited table does not merely lose accuracy: it decides which pixels count as a surface. The two mistakes are inverses of each other. A whitelist copied from a pre-2024 recipe ({8, 9}) applied to a current granule selects cloud medium and cloud high, so a mostly clouded scene returns full coverage and a plausible-looking mean of cloud. The current whitelist applied to a pre-05.09 granule drops its vegetation and reports a clear scene as unusable. Mitigated three ways: the whitelist is {4, 5} so an unmapped class is dropped from the mean rather than folded into it, `stac.table_supported()` refuses granules below baseline 05.09 instead of guessing, and section 13 derives the table from a recorded histogram against the same item's `s2:*_percentage` fields, which fails CI if Copernicus renumbers again. The granule that reported 0 percent usable over Kilifi creek while its item published 24.9 percent cloud is not an instance of this risk: it is a current-baseline pass with real cloud sitting over the plots and nowhere else, and it was caught by the coverage screen in 5.2. Keeping those two failures apart matters, because one is fixed by refusing a granule and the other by measuring before adopting.

---

## 15. Open questions

1. Who owns the Fly.io organization the backend and s2-worker deploy into, and who would own the tile bucket? The choice is between Cloudflare R2, which has no egress charge and needs a second account, and a Fly volume attached to the worker, which needs no new account but couples tile storage to one machine.
2. Should refresh be manager-initiated only, or should a daily scheduled job refresh every active project? A scheduler multiplies worker minutes and outbound requests to two public services, which is now our cost and our rate-limit exposure rather than a vendor quota.
3. Do you want the overlay masked to plot cells only, or shown across the full tile extent so users can compare their plots against the surrounding land? Masked reads as more precise but hides context.
4. Should the public proof page eventually show the change figure, or stay field-evidence only? This decides whether phase 5 needs a public read path.
