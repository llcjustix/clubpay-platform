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

-- 30 minutes before arrival through the requested game time plus the 15 minute
-- check-in allowance is unavailable to other reservations on the same PC.
ALTER TABLE mobile_reservations DROP CONSTRAINT IF EXISTS mobile_reservations_pc_window_excl;
ALTER TABLE mobile_reservations ADD CONSTRAINT mobile_reservations_pc_window_excl
  EXCLUDE USING gist (
    pc_ref_id WITH =,
    tstzrange(starts_at - interval '30 minutes',
      starts_at + make_interval(mins => duration_minutes + 15), '[)') WITH &&
  ) WHERE (status IN ('confirmed','checked_in'));

CREATE INDEX IF NOT EXISTS mobile_reservations_player_idx
  ON mobile_reservations(player_id, starts_at DESC);
CREATE INDEX IF NOT EXISTS mobile_reservations_pc_idx
  ON mobile_reservations(pc_ref_id, starts_at);
