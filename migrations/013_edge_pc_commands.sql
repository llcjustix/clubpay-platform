-- A Manager has a read-only local cache and must not pretend it can control an
-- Agent connected to the primary Controller.  It queues a request here; the
-- primary Controller picks it up during its normal Cloud synchronization.
CREATE TABLE IF NOT EXISTS edge_pc_commands (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  pc_ref_id UUID NOT NULL REFERENCES pc_refs(id) ON DELETE CASCADE,
  external_pc_id TEXT NOT NULL,
  desired_status TEXT NOT NULL CHECK (desired_status IN ('available', 'sleeping', 'maintenance', 'blocked')),
  reason TEXT,
  requested_by_node TEXT,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'succeeded', 'failed')),
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS edge_pc_commands_pending_idx
  ON edge_pc_commands (club_id, created_at)
  WHERE status = 'pending';
