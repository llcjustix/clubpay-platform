ALTER TABLE player_auth_challenges
  ADD COLUMN IF NOT EXISTS return_url TEXT;
