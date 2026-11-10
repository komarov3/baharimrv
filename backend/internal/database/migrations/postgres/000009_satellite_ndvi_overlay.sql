-- Satellite NDVI overlay (PostgreSQL).
-- Plot boundaries for the analysis cells, the granule provenance a pass has to carry,
-- the project-level change figure, and the single-flight lock that makes refresh
-- retryable. Geometry stays GeoJSON in jsonb: no PostGIS dependency, so both drivers
-- and the existing GORM models keep working unchanged.

ALTER TABLE plots ADD COLUMN IF NOT EXISTS boundary JSONB;
ALTER TABLE plots ADD COLUMN IF NOT EXISTS gps_accuracy_m NUMERIC(8,2);

ALTER TABLE satellite_passes ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT '';
ALTER TABLE satellite_passes ADD COLUMN IF NOT EXISTS ndvi_mean NUMERIC(6,4);
ALTER TABLE satellite_passes ADD COLUMN IF NOT EXISTS footprint JSONB;
ALTER TABLE satellite_passes ADD COLUMN IF NOT EXISTS processing_version TEXT NOT NULL DEFAULT '';

-- Refresh is retryable, so the same granule can be reported twice. Estimated revisits
-- have no scene id, which is why the predicate skips them instead of the column.
CREATE UNIQUE INDEX IF NOT EXISTS idx_satellite_passes_scene
  ON satellite_passes (project_id, mission, scene_id)
  WHERE scene_id IS NOT NULL AND scene_id <> '';

-- One row per computation of a project figure, never updated in place: a manager who
-- anchored a figure in March must still be able to read what March said after a
-- re-run in June changes the number.
CREATE TABLE IF NOT EXISTS project_ndvi_summaries (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES conservation_projects(id) ON DELETE CASCADE,
  period_start TIMESTAMPTZ NOT NULL,
  period_end TIMESTAMPTZ NOT NULL,
  current_scene_id TEXT NOT NULL,
  -- Nullable, not zero: an unavailable computation has no granule and no figure, and
  -- NDVI 0.0 is a real reading for bare substrate, so it cannot stand in for "nothing
  -- was measured". current_scene_id is '' in that case, the schema's string convention.
  current_acquired_at TIMESTAMPTZ,
  baseline_scene_id TEXT NOT NULL DEFAULT '',
  baseline_acquired_at TIMESTAMPTZ,
  mean_ndvi NUMERIC(6,4),
  baseline_mean_ndvi NUMERIC(6,4),
  change_abs NUMERIC(6,4),
  change_pct NUMERIC(8,2),
  coverage_pct NUMERIC(6,2) NOT NULL,
  current_cloud_pct NUMERIC(6,2),
  plot_count INTEGER NOT NULL,
  quality TEXT NOT NULL,
  adapter_mode TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT '',
  methodology_version TEXT NOT NULL DEFAULT '',
  result JSONB NOT NULL DEFAULT '{}',
  result_hash TEXT NOT NULL,
  processing_version TEXT NOT NULL,
  computed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_project_ndvi_summaries_project
  ON project_ndvi_summaries (project_id, period_end DESC);

CREATE TABLE IF NOT EXISTS satellite_refresh_jobs (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES conservation_projects(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  requested_by TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  -- Text, not uuid: the Go model holds this as a string and a queued job has no summary
  -- yet, so it follows the '' convention 000001 uses for an absent reference instead of
  -- asking the driver to cast an empty string to a uuid.
  summary_id TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

-- The lock lives here rather than in memory: two refresh taps must not stack jobs on
-- a worker sized for two concurrent runs, and a restart must not forget the queue.
CREATE UNIQUE INDEX IF NOT EXISTS idx_satellite_refresh_jobs_open
  ON satellite_refresh_jobs (project_id)
  WHERE status IN ('queued', 'running');

CREATE INDEX IF NOT EXISTS idx_satellite_refresh_jobs_status
  ON satellite_refresh_jobs (status);
