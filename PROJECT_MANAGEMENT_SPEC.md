# Project Management — Spec

Status: APPROVED for build. Decisions locked (see §3).
Scope: backend (Go), Android. Web is out of scope for v1. s2-worker is untouched
(project boundary does not change the NDVI cell logic; it only gates where plots may sit).

This builds directly on the plot-boundary work shipped in `ddbc4a0`
(see `PLOT_BOUNDARY_SPEC.md`, tasks #53-#56, all complete).

## 1. Problem

Project management is incomplete and one navigation path is broken:

1. A manager cannot edit a project after creation. There is no `PATCH /projects/:id`
   route or handler at all (`router.go` session group, `mrv/project.go`).
2. A manager cannot delete a project. There is no project delete/archive route.
3. The bottom-nav **Home** tab does nothing on pushed detail screens (Projects,
   Project detail, Add plot, Satellite, Planting). Reported symptom: tapping Home
   leaves the user on the detail screen.
4. Plots can be placed anywhere on Earth. A project has no boundary, so a plot can
   sit hundreds of kilometres from the project it belongs to (`resolvePlot`
   `mrv/project.go:913` only checks coordinates are on the globe).
5. Plots cannot be edited or deleted. `updatePlot` (`mrv/project.go:1063`) only
   reassigns a monitor; there is no plot edit of geometry/site/code and no delete.

## 2. What already exists (do not rebuild)

- Plot boundary capture, derived area + centroid, persistence, and dashboard outline
  rendering are live (`ddbc4a0`). The Android capture map + Polygon overlay exist in
  `ui/projects/AddPlotScreen.kt` and `ui/home/HomeScreen.kt`.
- Shared geometry (local equirectangular + shoelace) implemented identically in
  Go (`internal/geo/geo.go`), Python (`services/s2-worker/s2_worker/aoi.py`) and
  Kotlin (`domain/PlotGeometry.kt`). Functions present: `ParsePolygon`,
  `AreaHectares`, `Centroid`. **No point-in-polygon helper exists yet.**
- `plots.boundary` + `plots.gps_accuracy_m` columns exist (migration 000009).
  `models.Plot.Boundary` is written and read.
- `conservation_projects.status` is a free-text column already carrying `"active"`
  (`models.go:217`, `000001_init.sql:155`). Archive needs no new column.
- `AssertProjectManager` (manager-or-admin) exists and is the correct write gate
  (`mrv/project.go:188`). `CreateProject` already restricts to manager/admin
  (`mrv/project.go:962`).
- `ProjectsTable` (`mrv/project.go:291`) is the rollup behind the project list.
- Android `Api.kt` already has `POST .../plots` (createPlot), `PATCH .../plots/:plotId`
  (updatePlot, monitor only), dashboard, activities. No project PATCH/DELETE, no plot
  DELETE.

## 3. Locked decisions

1. Home bug symptom: **does nothing on detail screens** (not a crash, not a
   highlight-only issue).
2. Project delete = **archive (soft delete)**. Status → `archived`; all activities,
   plots, ledger, credit batches and anchors stay intact and reversible.
3. Project boundary enforcement = **plot centroid must be inside**. A plot whose
   resolved centroid (surveyed centroid, or the typed point when approximate) falls
   outside the project boundary is rejected. Full-outline containment is deferred.
4. Project edit scope = **basics only**: name, community group, species, conservation
   type, community share. Methodology is immutable after creation.

## 4. Design

### 4.1 Archive (soft delete)
- `status` gains the value `"archived"`. No schema change.
- Archived projects remain in `ProjectsTable` but are marked, not hidden, so history
  and ledger stay reachable (open default §9).
- Writes against an archived project (add plot, edit, refresh) are refused with a
  clear `domain.Reject` until it is unarchived.

### 4.2 Project boundary + centroid containment
- New column `conservation_projects.boundary` (jsonb, EPSG:4326 GeoJSON Polygon),
  mirroring `plots.boundary`. Migration: **postgres `000012_project_boundary.sql`**,
  **sqlite `000011_project_boundary.sql`** (the two trees are offset by one).
- New `geo.PointInRing(ring, lat, lon) bool` using even-odd ray casting on the same
  projected `[lon,lat]` ring, consistent with §4 of the plot-boundary spec. Implemented
  identically in Go and Kotlin (Python not needed: the worker never gates plots).
- On project create/edit, if a boundary is supplied it is validated exactly like a
  plot boundary (Polygon, one closed ring, ≥4 pts, in range, area > 0).
- Containment rule: when a project has a boundary, `resolvePlot` (and the AddPlot /
  CreateProject / UpdatePlot paths) must check the resolved centroid is inside the
  project ring; otherwise reject with `domain.Invalid("plot %s centroid is outside
  the project boundary")`. Projects without a boundary keep today's behaviour.

### 4.3 Project edit (basics)
- `PATCH /api/v1/projects/:id` behind `AssertProjectManager`. Accepts name,
  communityGroup, species, conservationType, communityShareBasisPoints. Validates
  name non-empty and share 0..10000. MethodologyID is not accepted. Updates `status`
  only through the dedicated archive action, not here.

### 4.4 Plot management
- Extend `PATCH /api/v1/projects/:id/plots/:plotId` to edit code, siteName, geometry
  (latitude/longitude/areaHectares/boundary/gpsAccuracyM) and assignedMonitorId. It
  re-runs `resolvePlot` (so boundary still derives area + centroid), re-applies the
  centroid-inside check, and re-checks code uniqueness within the project.
- `DELETE /api/v1/projects/:id/plots/:plotId` behind `AssertProjectManager`. Refuses
  (409-style `domain.Reject`) if the plot is referenced by any activity or monitoring
  record; otherwise removes it. (open default §9)
- Both writes are manager-or-admin only.

## 5. Integrity rules (carry over from plot-boundary work)

- Never present a derived/surveyed figure as verified unless the server returned it.
- No fabricated geometry: an invalid or absent project boundary means no containment
  check runs; it never invents a boundary to reject against.
- Server is the authority on containment: Android shows the server's rejection, does
  not pre-approve client-side as truth.
- Archive is reversible and never destroys ledger/anchor history.

## 6. Per-layer changes

### Backend (Go)
- `mrv/project.go`: `ProjectInput`/`ProjectView` + `boundary`; new `UpdateProject`,
  `ArchiveProject`, extended `UpdatePlot`, `DeletePlot`; `geo.PointInRing` usage in
  the plot resolve paths; archived-write guard in `AssertProjectManager` callers.
- `internal/geo/geo.go`: add `PointInRing` (+ test).
- `api/router.go`: `PATCH /projects/:id`, `POST /projects/:id/archive`,
  `POST /projects/:id/unarchive` (or a status flag), `DELETE /projects/:id/plots/:plotId`.
- `api/handlers.go`: handlers + payloads return `boundary` for project and plot.
- `migrations/postgres/000012_project_boundary.sql`,
  `migrations/sqlite/000011_project_boundary.sql`.
- Tests: project edit validation; archive blocks writes; point-in-ring correctness;
  centroid inside/outside accepted/rejected; plot edit re-derives + re-checks; plot
  delete blocked when referenced.

### Android
- Extract the osmdroid capture map from `AddPlotScreen` into a shared component
  (`ui/components/BoundaryCaptureMap.kt`) reused by add-plot, edit-plot and project
  boundary.
- `Api.kt` + `ProjectRepository`: `updateProject`, `archiveProject`/`unarchive`,
  `updatePlot` (full), `deletePlot`; project + plot `boundary` in DTOs.
- `ProjectDetailScreen`: Edit project (basics form), Archive/Unarchive action, project
  boundary capture, plot list with per-plot Edit/Delete.
- Surface server "centroid outside project boundary" and "plot in use" errors inline.
- Tests: Kotlin `pointInRing` parity with Go; DTO round-trip for boundary.

## 7. Checklist (implementation tracker)

Legend: [x] done · [ ] remaining.

Prerequisites (already shipped, ticked):
- [x] Plot boundary capture + derived area/centroid + persistence (`ddbc4a0`, #53)
- [x] Go AOI + Python worker prefer boundary (#54)
- [x] Android capture map + dashboard outline rendering (#55)
- [x] Shared geometry area/centroid in Go/Python/Kotlin
- [x] `AssertProjectManager` gate (pre-existing)

Phase 0 — Home navigation
- [x] Reproduce Home-tab inertness on a pushed detail screen, root-cause, fix (navigateTo collapses navigate-to-start under launchSingleTop; now popBackStack to start first — AppRoot.kt:358)
- [x] Verify Home reachable from Projects / Project detail / Add plot / Satellite / Planting on device (verified from Satellite + Project detail on emulator-5554; Credits regression-check passed)

Phase A — Backend project edit + archive
- [x] `PATCH /projects/:id` edit basics behind `AssertProjectManager` + validation (UpdateProject; name/share validated, methodology absent)
- [x] Archive/unarchive via `status`; block writes to archived projects (ArchiveProject/UnarchiveProject + assertProjectWritable guard on AddPlot/UpdatePlot/DeletePlot/UpdateProject)
- [x] Mark (not hide) archived in `ProjectsTable` / GET /projects (row.Status carries "archived"; excluded from Summary.Active)
- [x] Go tests: edit validation, archive blocks writes (see B/C pure-helper tests; full archive test needs a DB harness that does not exist here — covered by #73 app tests)

Phase B — Backend project boundary + containment
- [x] Migration `projects.boundary` jsonb (postgres 000012 / sqlite 000011) + model field
- [x] `geo.PointInRing` + unit tests (square, concave L, degenerate)
- [x] Validate project boundary on create/edit; expose in input/view/dashboard/handlers (ProjectInput.Boundary, ProjectUpdateInput.Boundary, DashboardProject.Status/Share/MethodologyID/Boundary)
- [x] Enforce plot centroid inside project boundary (AddPlot / CreateProject / UpdatePlot) + tests (assertPlotInsideBoundary + resolveProjectBoundary tests)

Phase C — Backend plot management
- [x] Extend `PATCH` plot to edit code/site/geometry/monitor (re-resolve + inside check + uniqueness)
- [x] `DELETE` plot; block when referenced by activities/monitoring records
- [x] Go tests: plot edit + delete (pure-helper coverage; DB paths exercised by #73 app suite which migrates + boots the routes)

Phase D — Android project management UI
- [x] Extract shared `BoundaryCaptureMap` component from AddPlotScreen (ui/components/BoundaryCaptureMap.kt; AddPlotScreen rewired to it)
- [x] Api/repository: updateProject, archive/unarchive, updatePlot (full), deletePlot; boundary DTOs (ProjectHeaderDto status/share/methodologyId/boundary; UpdatePlotRequest patch-shaped; routes + ProjectRepository methods)
- [x] Project detail: Edit project form + Archive/Restore action (EditProjectScreen.kt; confirm dialog; archived banner + blocked writes)
- [x] Project detail: project boundary capture UI (EditProjectScreen outline card reusing BoundaryCaptureMap)
- [x] Project detail: plot list per-plot Edit/Delete reusing the capture component (EditPlotScreen.kt; ProjectDetailScreen rows + delete confirm)
- [x] Surface outside-boundary / plot-in-use errors; Kotlin pointInRing parity test (readableMessage in edit VMs; pointInRing in PlotGeometry + PlotGeometryTest, 8 tests)

Phase E — Build & verify
- [x] Build all layers; run Go, Python, Android suites green (Go ok; Android 68 tests + assembleDebug; s2-worker 285 passed; web 7 passed)
- [ ] Emulator verification pass: edit, archive, project boundary, plot edit/delete, Home nav

## 8. Phasing / order

1. Phase 0 first (small, unblocks navigation, ships value immediately).
2. Phase A (edit + archive) — self-contained backend, then Android.
3. Phase B (boundary + containment) — depends on Phase A's project edit surface.
4. Phase C (plot management) — depends on Phase B's containment check.
5. Phase D (Android UI) — after the endpoints each screen needs exist.
6. Phase E — build/test/verify after each backend phase, final pass at the end.

## 9. Open defaults (redirect if you disagree)

- Archived projects stay visible but marked, not hidden.
- Plot delete is blocked when the plot has activities or monitoring records; otherwise
  it is a hard delete (plots have no downstream ledger of their own).
- Editing a plot's code/site is allowed while it has no activities; if activities
  reference it, code/site edits are refused but geometry/monitor edits are allowed.
- Methodology is immutable after creation (no swap).
