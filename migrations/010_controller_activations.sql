-- A Controller Node is enrolled through a short-lived, one-time code created
-- by a club owner.  The code itself is never stored, only its SHA-256 hash.
CREATE TABLE IF NOT EXISTS controller_activation_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  code_hash TEXT NOT NULL UNIQUE,
  node_mode TEXT NOT NULL CHECK (node_mode IN ('edge', 'manager')),
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  consumed_node_id TEXT,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS controller_activation_codes_active_idx
  ON controller_activation_codes (club_id, expires_at)
  WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS controller_nodes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  node_id TEXT NOT NULL UNIQUE,
  node_name TEXT NOT NULL DEFAULT '',
  node_mode TEXT NOT NULL CHECK (node_mode IN ('edge', 'manager')),
  sync_token_hash TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
  activated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_activated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS controller_nodes_club_idx ON controller_nodes (club_id, status);
