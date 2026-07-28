-- Every successful session extension is a separate, idempotent grant.
ALTER TABLE game_access_grants
  ADD COLUMN IF NOT EXISTS parent_grant_id UUID REFERENCES game_access_grants(id);

CREATE INDEX IF NOT EXISTS game_access_grants_parent_idx
  ON game_access_grants(parent_grant_id)
  WHERE parent_grant_id IS NOT NULL;
