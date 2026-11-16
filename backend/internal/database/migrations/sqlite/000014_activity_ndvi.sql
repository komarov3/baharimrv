-- Durable planting baselines share the project refresh queue and quota.
CREATE TABLE IF NOT EXISTS activity_ndvi_requests (
  activity_id TEXT PRIMARY KEY NOT NULL REFERENCES conservation_activities(id),
  project_id TEXT NOT NULL REFERENCES conservation_projects(id),
  plot_id TEXT NOT NULL REFERENCES plots(id),
  center_at TIMESTAMP NOT NULL,
  aoi TEXT NOT NULL,
  geometry_hash TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'queued', 'complete')),
  job_id TEXT NOT NULL DEFAULT '',
  observation_id TEXT NOT NULL DEFAULT '',
  requested_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_activity_ndvi_requests_project_id
  ON activity_ndvi_requests (project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_activity_ndvi_requests_job
  ON activity_ndvi_requests (job_id) WHERE job_id <> '';
CREATE INDEX IF NOT EXISTS idx_activity_ndvi_requests_waiting
  ON activity_ndvi_requests (status, created_at, activity_id);

ALTER TABLE satellite_refresh_jobs ADD COLUMN activity_id TEXT DEFAULT '';
ALTER TABLE satellite_refresh_jobs ADD COLUMN observation_id TEXT DEFAULT '';

ALTER TABLE satellite_observations ADD COLUMN purpose TEXT NOT NULL DEFAULT 'corroboration';
ALTER TABLE satellite_observations ADD COLUMN plot_id TEXT DEFAULT '';
ALTER TABLE satellite_observations ADD COLUMN geometry_hash TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_satellite_observations_geometry
  ON satellite_observations (purpose, plot_id, geometry_hash);
