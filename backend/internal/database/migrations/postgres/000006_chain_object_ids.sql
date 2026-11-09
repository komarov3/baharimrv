-- Add on-chain object ID fields to support conservation_chain integration
ALTER TABLE conservation_projects ADD COLUMN IF NOT EXISTS ledger_object_id VARCHAR(255);
ALTER TABLE restoration_requests ADD COLUMN IF NOT EXISTS chain_anchor_object_id VARCHAR(255);
