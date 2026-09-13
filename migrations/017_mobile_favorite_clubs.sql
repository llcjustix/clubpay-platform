CREATE TABLE IF NOT EXISTS mobile_favorite_clubs (
  player_id uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  club_id uuid NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (player_id, club_id)
);
CREATE INDEX IF NOT EXISTS mobile_favorite_clubs_player_idx
  ON mobile_favorite_clubs(player_id, created_at DESC);
