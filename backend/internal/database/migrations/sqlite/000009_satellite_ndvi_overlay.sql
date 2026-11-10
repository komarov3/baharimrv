-- Satellite NDVI overlay (SQLite variant).
-- Same tables and columns as the PostgreSQL migration. jsonb becomes TEXT and
-- timestamptz becomes TIMESTAMP, as in sqlite/000001_init.sql. No comment below this
-- one may contain a semicolon, because the runner splits this file on every one.

ALTER TABLE plots ADD COLUMN boundary TEXT;
ALTER TABLE plots ADD COLUMN gps_accuracy_m REAL;

ALTER TABLE satellite_passes ADD COLUMN provider TEXT NOT NULL DEFAULT '';
ALTER TABLE satellite_passes ADD COLUMN ndvi_mean REAL;
ALTER TABLE satellite_passes ADD COLUMN footprint TEXT;
ALTER TABLE satellite_passes ADD COLUMN processing_version TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_satellite_passes_scene
  ON satellite_passes (project_id, mission, scene_id)
  WHERE scene_id IS NOT NULL AND scene_id <> '';

CREATE TABLE IF NOT EXISTS project_ndvi_summaries (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES conservation_projects(id) ON DELETE CASCADE,
  period_start TIMESTAMP NOT NULL,
  period_end TIMESTAMP NOT NULL,
  current_scene_id TEXT NOT NULL,
  -- Nullable, not zero: an unavailable computation has no granule and no figure, and
  -- NDVI 0.0 is a real reading for bare substrate, so it cannot stand in for nothing
  -- being measured. current_scene_id is the empty string in that case.
  current_acquired_at TIMESTAMP,
  baseline_scene_id TEXT NOT NULL DEFAULT '',
  baseline_acquired_at TIMESTAMP,
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
  result TEXT NOT NULL DEFAULT '{}',
  result_hash TEXT NOT NULL,
  processing_version TEXT NOT NULL,
  computed_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_project_ndvi_summaries_project
  ON project_ndvi_summaries (project_id, period_end DESC);

CREATE TABLE IF NOT EXISTS satellite_refresh_jobs (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES conservation_projects(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  requested_by TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  summary_id TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_satellite_refresh_jobs_open
  ON satellite_refresh_jobs (project_id)
  WHERE status IN ('queued', 'running');

CREATE INDEX IF NOT EXISTS idx_satellite_refresh_jobs_status
  ON satellite_refresh_jobs (status);
