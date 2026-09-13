-- A reservation protects a PC shortly before its start, while preserving the
-- full paid play duration if the player checks in during the 15 minute window.
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS mobile_reservations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  player_id uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  club_id uuid NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  pc_ref_id uuid NOT NULL REFERENCES pc_refs(id) ON DELETE CASCADE,
  starts_at timestamptz NOT NULL,
  duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 60 AND 1440),
  status text NOT NULL DEFAULT 'confirmed'
    CHECK (status IN ('confirmed','checked_in','cancelled','expired','completed')),
  cancelled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Overlapping reservations are checked inside the creation transaction while
-- holding an advisory lock for the selected PC. An exclusion index cannot use
-- these timestamptz expressions: PostgreSQL treats interval arithmetic around
-- daylight-saving boundaries as non-immutable and refuses to boot the API.
-- The lookup index below keeps that transactional check fast.

CREATE INDEX IF NOT EXISTS mobile_reservations_player_idx
  ON mobile_reservations(player_id, starts_at DESC);
CREATE INDEX IF NOT EXISTS mobile_reservations_pc_idx
  ON mobile_reservations(pc_ref_id, starts_at);
