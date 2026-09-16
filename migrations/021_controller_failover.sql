-- Cloud keeps a short lease for the exact Agent WebSocket connection that is
-- currently authoritative.  A new Agent connection fences the former
-- Controller immediately; the old Controller may still be alive, but it can
-- no longer receive Cloud commands or overwrite live status.
CREATE TABLE IF NOT EXISTS controller_agent_leases (
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  external_pc_id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  agent_connection_id TEXT NOT NULL,
  acquired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  renewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  lease_expires_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (club_id, external_pc_id)
);

CREATE INDEX IF NOT EXISTS controller_agent_leases_active_idx
  ON controller_agent_leases (club_id, lease_expires_at DESC);

-- A Cloud command is delivered to one Controller only.  This prevents a
-- returning primary and an already-active Manager from executing the same
-- lock/sleep/session action concurrently.
ALTER TABLE edge_pc_commands
  ADD COLUMN IF NOT EXISTS target_node_id TEXT;

CREATE INDEX IF NOT EXISTS edge_pc_commands_target_pending_idx
  ON edge_pc_commands (club_id, target_node_id, created_at)
  WHERE status = 'pending';

-- Node liveness is a separate signal from the Agent lease: it lets Cloud pick
-- a currently synchronizing Controller for wake commands while every Agent is
-- asleep and therefore has no WebSocket lease.
ALTER TABLE controller_nodes
  ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS controller_nodes_seen_idx
  ON controller_nodes (club_id, status, last_seen_at DESC);
