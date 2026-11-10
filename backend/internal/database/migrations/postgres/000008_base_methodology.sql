-- Migration 000008: seed the base mangrove restoration methodology
-- This is reference data required for project creation on all environments.
-- Uses INSERT ... ON CONFLICT DO NOTHING so re-running is safe.

INSERT INTO methodologies (
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
    '2026-11-01 13:32:04.709358+00',
    '2026-11-01 13:32:04.709358+00'
) ON CONFLICT (id) DO NOTHING;
