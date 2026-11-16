-- Project boundary (PostgreSQL).
-- The project outline that gates where its plots may sit, mirroring plots.boundary:
-- GeoJSON in jsonb, no PostGIS, so both drivers and the GORM models keep working.
-- Nullable by design. A project with no boundary enforces no containment, so existing
-- and web-created projects are untouched.

ALTER TABLE conservation_projects ADD COLUMN IF NOT EXISTS boundary JSONB;
