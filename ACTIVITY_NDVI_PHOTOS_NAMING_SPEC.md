# Planting Activity: Naming, Evidence Photos, and Auto NDVI Baseline — Spec

Status: APPROVED, backend and Android implementation complete. API deployed to Fly release v31 on 2026-11-16 from commit 166d2ee. Read-only emulator checks passed; full write-flow acceptance and isolated PostgreSQL migration testing remain pending. Three workstreams, sequenced naming → photos → NDVI.
Scope: backend (Go), Android, s2-worker (read-only reuse). Web out of scope.
Build order approved by user; NDVI search window locked at ±15 days.

This rides on the project-management work shipped in `c9cf41a`
(see `PROJECT_MANAGEMENT_SPEC.md`) and the plot/project boundary + NDVI work
in `ddbc4a0` / `SATELLITE_NDVI_SPEC.md`.

## 1. Problem

Three gaps, one broken behaviour, in the planting-activity flow:

1. **Activities cannot be named.** `ConservationActivity` has only `activityType`
   (`models.go:261`), no user-facing title. Every planting reads "planting".
2. **Photos captured during activity creation are silently dropped.**
   `PlantingActivityScreen.kt:111` collects `pendingPhotos` locally, but
   `PlantingActivityViewModel.create()` (`PlantingActivityViewModel.kt:97`) never
   sends them; the create body (`ActivityInput` `project.go:1486`) has no photo
   field. Evidence upload exists only against a **monitoring record**
   (`POST /monitoring-records/:id/evidence`, `monitoring.go:249`), which does not
   exist yet at activity-creation time. So there is nowhere to attach the photos.
3. **No automated vegetation baseline.** A planting activity should capture the
   plot's NDVI at planting time as a reference point, so later passes report change
   against it and the Report tab can render an auto-populated monitoring trend
   instead of a blank section.

## 2. What already exists (do not rebuild)

- **Per-activity NDVI storage is already modelled.** `SatelliteObservation` is keyed
  by `activityId` (`models.go:321`) and stores `adapter_mode`, `status`, `scene_id`,
  `acquired_at`, canonical `result` (mean_ndvi / baseline / change_vs_baseline) and
  `result_hash`. Read via `satellite.Latest(activityID)` / `Summaries(activityID)`
  (`service.go:96,145`). Today only `AttachSandbox` (`service.go:116`) writes it.
- **Live NDVI computation is wired in Go.** `OpenDataStacProvider.observe()` calls
  the worker `/v1/scene` then `/v1/stats` (`worker.go:80`); `BaselineObservation`
  and `LatestObservation` already take an AOI + window (`worker.go:65,69`). The
  worker ranks scenes by cloud tier then recency, screens real usable coverage over
  the plots, and masks cloud/cirrus/shadow/water (`s2-worker/s2_worker/job.py`,
  `ndvi.py` USABLE_CLASSES={4,5}).
- **Async refresh + quota already exist.** `QueueRefresh(projectID, ...)` runs a job
  that makes at most four worker calls, bounded, with a hard limit of **6 refreshes
  per project per hour** (`refresh.go:31`). Poll cadence 15s (`refresh.go:33`).
- **Evidence storage + hash verify exist** for monitoring records: `evidence.Store.Put`
  writes to disk and rejects bytes that do not match the declared sha256
  (`monitoring.go:267`, `evidence/store.go`). `EvidenceObject` model (`models.go:309`).
- **Report tab + proof** already surface satellite corroboration
  (`PlantingDetailScreen` report tab; `proof/service.go` references the corroboration
  hash). The seam to add an NDVI section is present.
- Android already computes nothing NDVI-related client-side and shows only
  server-returned figures with sandbox/live labels (integrity rule, carried over).

## 3. Locked decisions

1. **Sequencing**: Phase 1 naming, Phase 2 photos, Phase 3 NDVI. Each ships and is
   verified before the next.
2. **NDVI window**: ±15 days centred on the planting date.
3. **Activity name is optional.** `activityType` stays the machine label; `title` is
   a human label, editable after creation.
4. **Activity photos are manager-declared evidence, not attested monitoring
   evidence.** They are hashed on-device for integrity of transport, but they are
   NOT part of a signed monitoring record and must never be presented as verified or
   anchored. They attach to the activity.
5. **NDVI capture is asynchronous and never blocks activity creation.** The create
   call returns immediately; the baseline is filled in by a background job and the
   Report tab shows whatever the server has recorded (including "no clear scene").

## 4. Design

### 4.1 Phase 1 — Activity naming
- New non-null text column `conservation_activities.title`, default empty string. Migration: postgres **000013_activity_title.sql**, sqlite **000012_activity_title.sql**
  (the two trees stay offset by one; confirm current heads before writing).
- `ActivityInput` gains `Title string`; `CreateActivity` trims, caps length
  (≤120 Unicode code points), stores empty as an empty string. New `PATCH /activities/:id` (manager-or-admin,
  reuses the existing activity-access gate) accepts `title` only.
- Android: a "Name (optional)" field on `PlantingActivityScreen`; title shown in the
  planting list, detail header, and report; editable via a rename action.
- No methodology/measurements change. Rename uses a column-only update, preserving
  UpdatedAt because signed MRV links use it as the capture timestamp. Title does not
  affect anchoring.

### 4.2 Phase 2 — Evidence photos on activities
- Reuse the evidence store but key objects to the **activity**, not a monitoring
  record. Two schema options (default A):
  - **A (recommended): a new `activity_evidence` table** (id, activity_id FK, kind,
    storage_uri, sha256, media_type, size_bytes, captured_at, created_at). Keeps the
    attested `evidence_objects` path untouched so monitoring-record integrity is not
    diluted. Photos live under an activity-scoped prefix in the store.
  - B: add a nullable `activity_id` to `evidence_objects` and relax the
    monitoring_record_id NOT NULL. Rejected as default because it weakens the
    manifest-verification invariant on a table the anchor path trusts.
- New endpoint `POST /api/v1/activities/:id/evidence` (multipart: file, kind,
  sha256, mediaType). Server independently checks bytes, media type and SHA-256 using
  a bounded in-memory read (maximum 5 MiB). Manager-or-admin gate.
- Limits (open default §9): max 10 photos per activity, ≤5 MB each, image media
  types only.
- Android: after `createActivity` returns the id, upload each pending photo with
  device-computed SHA-256 and a bounded 5 MiB read. Show per-photo upload state and
  retry failures without recreating the activity. Selections survive configuration
  changes in the ViewModel, not process death or offline restarts; warn before leaving
  with pending uploads.
- `GET /activities/:id/detail` returns a separate `activityEvidence` list with id,
  kind, mediaType, SHA-256, sizeBytes, uploaded-at timestamp and nullable capturedAt.
  `GET /activities/:id/evidence/:evidenceId` proxies authenticated bytes; storage
  locations are never returned to clients.

### 4.3 Phase 3 — Auto NDVI baseline at planting
- On a planting activity create (and on a later explicit "recapture" action),
  **enqueue** a baseline capture for the activity's plot:
  - Build a single-cell AOI from the plot (surveyed boundary if present, else the
    point buffer the existing AOI code already uses).
  - Center the ±15-day window on the planting date (open default §9: use
    `reportingPeriodStart` when set, else `createdAt`).
  - Add `ObservationWindow(ctx, aoi, start, end)`. The existing
    `BaselineObservation` always ends one year before now and must retain that
    project-summary meaning. Planting uses an explicit 30-day search ending 15 days
    after the planting date, without inventing future acquisitions.
  - Persist an `activity_ndvi_requests` row with the activity in the same database
    transaction. It snapshots the single-plot geometry and date; no imagery calls
    happen during creation. Waiting requests survive restart and quota exhaustion.
  - Admit one request as one ordinary refresh job under the shared project lock
    and six/hour quota. An unrelated open project job cannot consume the request.
    Provider faults fail the job, not the already-created activity.
- Persist the outcome as a `SatelliteObservation` on the **activity**, with purpose
  `planting_baseline`. Existing corroboration/proof reads exclude this purpose, so
  automatic capture does not silently change an attested or anchored record:
  - Clear scene found → status `complete`/`partial`/`cloudy` exactly as the worker
    reports, with scene_id, acquired_at, cloud_pct, coverage_pct, mean_ndvi,
    result_hash.
  - No usable scene in the window → status `no_observation`, mean left null. Never
    invent a number.
- Run it through the existing refresh job machinery so it inherits the 6/hour
  project quota and the poll model; do not open a second, unbounded pipeline.
  Dedupe: skip if a `complete`/`partial` baseline already exists for the same plot +
  overlapping window.
- Planting activity: a dedicated **NDVI section on the activity itself** (create +
  detail), showing capture state (queued / capturing / done / no clear scene / error)
  and the server-returned baseline once present.
- Report tab: render the activity's baseline observation (mean, scene date, cloud %,
  coverage, quality, sandbox/live label) and, when a later pass exists, the change
  against the earliest usable baseline for that plot with matching geometry,
  provider and processing version. Project refresh summaries retain their per-plot
  geometry and statistics for later comparisons without additional imagery calls.
  Only strictly later acquisitions are compared; absent or incompatible passes leave
  change null. Partial coverage remains labelled. All figures are server-returned.
- **Dependency to flag**: the deployed API currently runs `satellite: sandbox`
  (health). The pipeline is correct in both modes, but real NDVI numbers require
  `SATELLITE_MODE=live` plus worker URL/token configured in the Fly environment.
  Until then every baseline is labelled sandbox, exactly like the dashboard today.

## 5. Integrity rules (carried over, non-negotiable)

- Render only server-returned NDVI states; label sandbox vs live; never derive a
  verified outcome client-side.
- A cloudy/partial scene is never presented as a clean baseline; carry quality,
  cloud %, and coverage with the figure.
- "No clear scene in window" is a real, shown result — not a fabricated number.
- Activity photos are un-attested manager evidence; they are never shown as anchored
  or verified, and they do not enter the monitoring-record manifest.
- Server recomputes the sha256 of uploaded bytes; the client hash is a claim, not
  truth.
- NDVI capture never blocks or fails activity creation; it is best-effort and async.

## 6. Per-layer changes

### Backend (Go)
- `models.go`: `ConservationActivity.Title`; new `ActivityEvidence` model (option A).
- `migrations/postgres/000013_activity_title.sql`, `.../000014_activity_evidence.sql`;
  sqlite `000012_activity_title.sql`, `000013_activity_evidence.sql` (confirm heads).
- `mrv/project.go`: title validation and atomic activity/request creation;
  `mrv/activity.go`: manager/admin rename without changing capture timestamps;
  `mrv/activity_evidence.go`: `UploadActivityEvidence` and activity evidence listing.
- NDVI migrations: postgres `000015_activity_ndvi.sql`, sqlite `000014_activity_ndvi.sql`.
- `satellite/activity.go`: `NewActivityNdviRequest`, `ComputeActivityBaseline`,
  explicit recapture, compatible reuse and server-side report comparisons.
- `satellite/activity_queue.go` and `refresh.go`: durable waiting-request admission,
  one activity per job, shared quota, ordinary retry/reaper lifecycle.
- `satellite/service.go`: planting baselines excluded from corroboration reads;
  `summary.go`: per-plot geometry/statistics included in new summary commitments.
- `api/router.go`: title PATCH, activity evidence upload/list/byte proxy,
  `GET /activities/:id/ndvi`, `POST /activities/:id/ndvi/recapture`.
- `api/handlers.go`: detail payload returns title, separate activityEvidence and NDVI.
- Tests: title validation + PATCH; evidence hash mismatch rejected + kind/size/count
  limits; baseline capture stores complete/partial/no_observation; dedupe; quota
  respected; report renders server states only.

### s2-worker
- No code change expected: `/v1/scene` + `/v1/stats` already answer a single-cell
  baseline request with provenance. Verify a one-cell request within ±15 days
  returns the expected envelope; add a test only if a gap appears.

### Android
- `PlantingActivityScreen` + ViewModel: optional name field; upload `pendingPhotos`
  after create (device sha256 + multipart), per-photo state, inline errors.
- `Api.kt` + `ProjectRepository`: `updateActivityTitle`, `uploadActivityEvidence`,
  `activityEvidence`, `recaptureNdvi`; evidence + title DTOs.
- Planting list + `PlantingDetailScreen`: show title; evidence thumbnails; NDVI
  baseline section in the Report tab (server states, sandbox/live label, quality).
- Tests: DTO round-trips; upload flow error surfacing; report shows only server
  values (no client derivation).

## 7. Checklist (implementation tracker)

Legend: [x] implemented/tested as stated · [ ] remaining. Checks do not imply deployment or device acceptance.

Prerequisites (already shipped):
- [x] `SatelliteObservation` keyed by activityId
- [x] Live Go provider and worker scene/statistics endpoints
- [x] Worker cloud ceilings, class-layer mask and coverage screen
- [x] Async refresh job and 6/hour/project quota
- [x] Evidence store with SHA-256 verification
- [x] Activity report and proof satellite-corroboration seam

Phase 1: Naming
- [x] Title migrations (postgres 000013 / sqlite 000012) and model field
- [x] Create validation and manager/admin title PATCH
- [x] Android name field, list/detail/report display and rename
- [x] Integration/unit tests, timestamp preservation and CSV formula escaping

Phase 2: Evidence photos
- [x] Separate activity_evidence table and registered model
- [x] Upload/list/authenticated byte proxy, independent hash/type checks and limits
- [x] Android upload after create, per-photo state and retry without recreating activity
- [x] Detail/report gallery, monitoring-evidence separation and pending-leave warning
- [x] Go validation/access/dedupe tests and Android multipart/hash/proxy tests

Phase 3: Auto NDVI baseline
- [x] Explicit provider date window and planting-purpose observations in live/sandbox modes
- [x] Durable date/geometry snapshot; one activity per shared-quota refresh job
- [x] Transactional planting create and manager/admin recapture endpoint
- [x] Compatible baseline reuse by plot geometry, window, provider and processing
- [x] NDVI section in create/report; server-side comparisons with later compatible passes
- [x] Focused tests: quality/null states, windows/geometry, reuse, quotas, provenance separation,
  future acquisitions, percentage precision, write rollback, API access and server-only Android values
- [x] Production API deployment: Fly release v31, commit 166d2ee, health and authenticated
  activity detail/NDVI reads checked on 2026-11-16
- [ ] Enable live imagery, separate approval required; deployed mode remains sandbox

Phase V: Build and verify
- [x] Android assembleDebug and all 78 unit tests, BUILD SUCCESSFUL
- [x] Existing worker suite: 285 tests passed, no worker code changes
- [x] Focused Go satellite/activity API/photo/title tests passed
- [x] Complete Go suite passed with a longer timeout after the initial host-pressure timeout
- [x] Final complete Go suite and build, satellite percentage precision regression,
  and planting NDVI/queue race checks repeated three times
- [ ] PostgreSQL migration execution on an isolated test database
- [x] Read-only BlueStacks acceptance after deployment: list/detail reload, Rename dialog
  and cancel, NDVI refresh and server-returned no-baseline state for an old activity
- [ ] Full write-flow acceptance: name/save, photo upload/retry/reopen and automatic
  NDVI capture for a new planting activity; no production test accounts, test data
  or automated login were used.

Automatic reports read subsequent project imagery refreshes; no periodic monitoring
scheduler or NDVI-derived survival/carbon verification was introduced.

## 8. Phasing / order

1. Phase 1 (naming) — smallest, unblocks clearer activity identity.
2. Phase 2 (photos) — fixes the reported data-loss bug.
3. Phase 3 (NDVI) — largest, depends on a named/photographed activity existing so the
   report has a coherent subject.
4. Phase V — build/test/verify after each phase; final device pass at the end.

## 9. Open defaults (redirect if you disagree)

- **Photos**: lightweight manager-declared evidence in a separate `activity_evidence`
  table (option A), not folded into the attested `evidence_objects`. Limits: 10
  photos, 5 MB each, images only. Photos are uploaded right after create; a failed
  upload is shown and retryable, but does not roll back the activity.
- **Name**: optional, ≤120 chars, editable; never required; does not affect anchoring.
- **NDVI window center**: `reportingPeriodStart` if set, else activity `createdAt`.
- **Baseline per plot**: the first `complete`/`partial` observation for a plot is the
  reference; the report shows change of later passes against it. Re-capture is manual.
- **Live imagery**: shipped sandbox-correct; enabling real numbers in Fly
  (`SATELLITE_MODE=live` + worker URL/token) is a separate, explicitly-approved step.
- **Storage target**: activity evidence on the same filesystem store as monitoring
  evidence (`evidence/store.go`), activity-scoped key prefix. The Fly app has no
  persistent volume; adding one was explicitly declined on 2026-11-16. Metadata
  persists in PostgreSQL, but photo files are machine-local and can disappear on
  replacement or redeployment. Shared durable object storage was not added.
