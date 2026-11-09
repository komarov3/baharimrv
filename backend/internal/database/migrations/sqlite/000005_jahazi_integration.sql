-- Jahazi cross-platform integration (SQLite variant)
-- Adds RestorationRequest table and Jahazi fields to existing tables.

-- Extend conservation_projects with Jahazi integration fields.
ALTER TABLE conservation_projects ADD COLUMN jahazi_enabled BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE conservation_projects ADD COLUMN jahazi_project_id TEXT;
CREATE INDEX IF NOT EXISTS idx_conservation_projects_jahazi_project_id ON conservation_projects (jahazi_project_id);

-- Extend conservation_activities with Jahazi matching and MRV link tracking.
ALTER TABLE conservation_activities ADD COLUMN jahazi_funded BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE conservation_activities ADD COLUMN restoration_request_id TEXT;
ALTER TABLE conservation_activities ADD COLUMN mrv_link_generated BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE conservation_activities ADD COLUMN mrv_link_pushed_at DATETIME;
CREATE INDEX IF NOT EXISTS idx_conservation_activities_restoration_request_id ON conservation_activities (restoration_request_id);

-- RestorationRequest: explicit claims from Jahazi for tourist-funded restoration.
CREATE TABLE IF NOT EXISTS restoration_requests (
    id                  TEXT PRIMARY KEY,
    project_id          TEXT NOT NULL REFERENCES conservation_projects(id) ON DELETE CASCADE,
    jahazi_claim_id     TEXT NOT NULL UNIQUE,
    jahazi_project_id   TEXT NOT NULL,
    period_start        DATETIME NOT NULL,
    period_end          DATETIME NOT NULL,
    total_cost_usd      TEXT NOT NULL,
    total_units         INTEGER NOT NULL,
    species             TEXT NOT NULL,
    contributor_count   INTEGER NOT NULL,
    contributor_breakdown TEXT NOT NULL,
    nonce               TEXT NOT NULL UNIQUE,
    signature           TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'open',
    matched_activity_id  TEXT,
    fulfilled_units     INTEGER NOT NULL DEFAULT 0,
    expires_at          DATETIME NOT NULL,
    created_at          DATETIME NOT NULL,
    updated_at          DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_restoration_requests_project_id ON restoration_requests (project_id);
CREATE INDEX IF NOT EXISTS idx_restoration_requests_status ON restoration_requests (status);
CREATE INDEX IF NOT EXISTS idx_restoration_requests_expires_at ON restoration_requests (expires_at);
