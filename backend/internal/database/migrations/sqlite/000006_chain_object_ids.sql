-- Add on-chain object ID fields to support conservation_chain integration
ALTER TABLE conservation_projects ADD COLUMN ledger_object_id TEXT;
ALTER TABLE restoration_requests ADD COLUMN chain_anchor_object_id TEXT;
