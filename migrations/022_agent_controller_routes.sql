-- Persist the LAN Controller pair for each Agent. The Agent is allowed to read
-- only its own pair through the authenticated bootstrap endpoint.
ALTER TABLE pc_refs ADD COLUMN IF NOT EXISTS agent_primary_controller_url TEXT;
ALTER TABLE pc_refs ADD COLUMN IF NOT EXISTS agent_fallback_controller_url TEXT;
