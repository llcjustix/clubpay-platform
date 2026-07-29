-- A session remains extendable while the Agent keeps the PC frozen in grace.
ALTER TABLE game_access_grants
  ADD COLUMN IF NOT EXISTS grace_ends_at TIMESTAMPTZ;

-- Preserve sessions that were already active when the migration was deployed.
UPDATE game_access_grants
SET grace_ends_at = planned_ends_at + interval '10 minutes'
WHERE status = 'accepted'
  AND grace_ends_at IS NULL
  AND planned_ends_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS game_access_grants_grace_ends_idx
  ON game_access_grants(grace_ends_at)
  WHERE status = 'accepted';
