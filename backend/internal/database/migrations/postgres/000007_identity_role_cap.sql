-- Add role_cap_object_id to identity_provisions.
-- Stores the on-chain capability object ID issued after provisioning.
-- Empty for DID-only roles (buyer, finance_operator, platform_administrator).
ALTER TABLE identity_provisions
    ADD COLUMN IF NOT EXISTS role_cap_object_id TEXT NOT NULL DEFAULT '';

-- SQLite mirror (kept in sync for local dev)
