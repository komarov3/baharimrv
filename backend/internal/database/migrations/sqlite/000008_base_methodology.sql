-- Migration 000008: seed the base mangrove restoration methodology
-- This is reference data required for project creation on all environments.
-- The seed package creates this same record when SEED_DEMO_DATA=true;
-- this migration ensures it exists on production and fresh installs too.
-- Uses INSERT OR IGNORE so re-running is safe.

INSERT OR IGNORE INTO methodologies (
    id,
    code,
    name,
    version,
    required_evidence,
    estimation,
    created_at,
    updated_at
) VALUES (
    'c6454811-b062-43b6-bbb7-5c380b8e4ca5',
    'MED-MANGROVE-DEMO',
    'Community mangrove restoration (demo methodology)',
    '1.0',
    'Geo-tagged survival count, average height, species note, at least one field photo or voice log.',
    '{"period_years":1,"source":"Demo parameter set. Not a verified allometric equation.","survival_threshold_pct":70,"tonnes_co2_per_tree":"0.006250"}',
    '2026-11-01T13:32:04.709358+00:00',
    '2026-11-01T13:32:04.709358+00:00'
);
