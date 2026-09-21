-- Mobile requests reach Cloud, while the Agent WebSocket belongs to the
-- selected LAN Controller.  Carry an authenticated session-end command across
-- that existing durable queue instead of trying to command a Cloud-local Agent.
ALTER TABLE edge_pc_commands
  ADD COLUMN IF NOT EXISTS grant_id UUID REFERENCES game_access_grants(id) ON DELETE SET NULL;

ALTER TABLE edge_pc_commands
  ADD COLUMN IF NOT EXISTS core_session_id TEXT;

ALTER TABLE edge_pc_commands
  DROP CONSTRAINT IF EXISTS edge_pc_commands_desired_status_check;

ALTER TABLE edge_pc_commands
  ADD CONSTRAINT edge_pc_commands_desired_status_check
  CHECK (desired_status IN ('available', 'sleeping', 'maintenance', 'blocked', 'end_session'));
