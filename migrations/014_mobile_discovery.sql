-- A club's public map point is managed by its owner or manager.  Keeping it
-- separate from the postal address lets the mobile catalogue navigate a
-- player to the actual entrance rather than a loosely formatted address.
ALTER TABLE clubs
  ADD COLUMN IF NOT EXISTS latitude double precision,
  ADD COLUMN IF NOT EXISTS longitude double precision;

ALTER TABLE clubs
  DROP CONSTRAINT IF EXISTS clubs_valid_coordinates;
ALTER TABLE clubs
  ADD CONSTRAINT clubs_valid_coordinates CHECK (
    (latitude IS NULL AND longitude IS NULL) OR
    (latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180)
  );

-- Product analytics deliberately stores action names rather than device IDs,
-- location or free-form user input. It is enough to measure funnels without
-- turning ClubPay into a behavioural tracker.
CREATE TABLE IF NOT EXISTS mobile_analytics_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  player_id uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  event_name text NOT NULL CHECK (event_name ~ '^[a-z][a-z0-9_]{1,63}$'),
  screen text NOT NULL DEFAULT '' CHECK (length(screen) <= 64),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS mobile_analytics_events_created_idx
  ON mobile_analytics_events (created_at DESC, event_name);
