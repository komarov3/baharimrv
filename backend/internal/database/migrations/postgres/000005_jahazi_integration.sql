-- Jahazi cross-platform integration
-- Adds RestorationRequest table and Jahazi fields to existing tables.

-- Extend conservation_projects with Jahazi integration fields.
ALTER TABLE conservation_projects ADD COLUMN IF NOT EXISTS jahazi_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE conservation_projects ADD COLUMN IF NOT EXISTS jahazi_project_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_conservation_projects_jahazi_project_id ON conservation_projects (jahazi_project_id) WHERE jahazi_project_id IS NOT NULL;

-- Extend conservation_activities with Jahazi matching and MRV link tracking.
ALTER TABLE conservation_activities ADD COLUMN IF NOT EXISTS jahazi_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE conservation_activities ADD COLUMN IF NOT EXISTS jahazi_funded BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE conservation_activities ADD COLUMN IF NOT EXISTS restoration_request_id UUID;
ALTER TABLE conservation_activities ADD COLUMN IF NOT EXISTS mrv_link_generated BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE conservation_activities ADD COLUMN IF NOT EXISTS mrv_link_pushed_at TIMESTAMP;
CREATE INDEX IF NOT EXISTS idx_conservation_activities_restoration_request_id ON conservation_activities (restoration_request_id) WHERE restoration_request_id IS NOT NULL;

-- RestorationRequest: explicit claims from Jahazi for tourist-funded restoration.
CREATE TABLE IF NOT EXISTS restoration_requests (
    id                  UUID PRIMARY KEY,
    project_id          UUID NOT NULL REFERENCES conservation_projects(id) ON DELETE CASCADE,
    jahazi_claim_id     VARCHAR(255) NOT NULL UNIQUE,
    jahazi_project_id   VARCHAR(255) NOT NULL,
    period_start        TIMESTAMP NOT NULL,
    period_end          TIMESTAMP NOT NULL,
    total_cost_usd      NUMERIC(30,2) NOT NULL,
    total_units         INTEGER NOT NULL,
    species             VARCHAR(255) NOT NULL,
    contributor_count   INTEGER NOT NULL,
    contributor_breakdown JSONB NOT NULL,
    nonce               VARCHAR(255) NOT NULL UNIQUE,
    signature           TEXT NOT NULL,
    status              VARCHAR(50) NOT NULL DEFAULT 'open',
    matched_activity_id  UUID,
    fulfilled_units     INTEGER NOT NULL DEFAULT 0,
    expires_at          TIMESTAMP NOT NULL,
    created_at          TIMESTAMP NOT NULL,
    updated_at          TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_restoration_requests_project_id ON restoration_requests (project_id);
CREATE INDEX IF NOT EXISTS idx_restoration_requests_status ON restoration_requests (status);
CREATE INDEX IF NOT EXISTS idx_restoration_requests_expires_at ON restoration_requests (expires_at);
CREATE INDEX IF NOT EXISTS idx_restoration_requests_matched_activity_id ON restoration_requests (matched_activity_id) WHERE matched_activity_id IS NOT NULL;
