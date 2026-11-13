ALTER TABLE project_ndvi_summaries ADD COLUMN IF NOT EXISTS tile_min_zoom INTEGER;
ALTER TABLE project_ndvi_summaries ADD COLUMN IF NOT EXISTS tile_max_zoom INTEGER;
