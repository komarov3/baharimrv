-- Activity naming (PostgreSQL).
-- A manager's own label for an activity record. It attests nothing and sits outside the
-- verification chain, so it is plain non-null text with an empty default meaning "no
-- name" — the clients then fall back to showing the activity type. No existing row is
-- changed and no anchor is affected.

ALTER TABLE conservation_activities ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
