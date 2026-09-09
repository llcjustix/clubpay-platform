-- Value is a non-cash gaming credit: seconds * zone hourly price in tiyin.
-- Keep whole units so changing zones never discards fractional seconds.
ALTER TABLE player_club_balances ADD COLUMN IF NOT EXISTS time_value_units BIGINT;
ALTER TABLE player_club_balances ADD COLUMN IF NOT EXISTS reference_price_tiyin BIGINT;
ALTER TABLE player_time_ledger ADD COLUMN IF NOT EXISTS time_value_delta BIGINT;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS time_value_rate BIGINT;

-- Capture zone pricing once, including cash and legacy checkout paths.
CREATE OR REPLACE FUNCTION capture_grant_time_rate() RETURNS trigger AS $$
BEGIN
  IF NEW.time_value_rate IS NULL THEN
    IF NEW.parent_grant_id IS NOT NULL THEN
      SELECT time_value_rate INTO NEW.time_value_rate FROM game_access_grants WHERE id=NEW.parent_grant_id;
    END IF;
    IF NEW.time_value_rate IS NULL THEN
      SELECT z.hourly_price_tiyin INTO NEW.time_value_rate FROM pc_refs p JOIN zones z ON z.id=p.zone_id WHERE p.id=NEW.pc_ref_id;
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS grant_time_rate ON game_access_grants;
CREATE TRIGGER grant_time_rate BEFORE INSERT ON game_access_grants FOR EACH ROW EXECUTE FUNCTION capture_grant_time_rate();

-- Historic seconds have no source-zone provenance. Preserve them in the
-- club's lowest-priced current zone; no balance is silently discarded.
UPDATE player_club_balances b SET reference_price_tiyin=(SELECT MIN(hourly_price_tiyin) FROM zones WHERE club_id=b.club_id AND status<>'deleted') WHERE reference_price_tiyin IS NULL;
UPDATE player_club_balances SET time_value_units=seconds_balance::bigint*reference_price_tiyin WHERE time_value_units IS NULL AND reference_price_tiyin>0;
UPDATE game_access_grants g SET time_value_rate=z.hourly_price_tiyin FROM pc_refs p JOIN zones z ON z.id=p.zone_id WHERE g.pc_ref_id=p.id AND g.time_value_rate IS NULL;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS controller_synced_at TIMESTAMPTZ;
ALTER TABLE player_club_balances DROP CONSTRAINT IF EXISTS player_balance_value_nonnegative;
ALTER TABLE player_club_balances ADD CONSTRAINT player_balance_value_nonnegative CHECK(time_value_units IS NULL OR time_value_units>=0);
ALTER TABLE player_club_balances DROP CONSTRAINT IF EXISTS player_balance_reference_positive;
ALTER TABLE player_club_balances ADD CONSTRAINT player_balance_reference_positive CHECK(reference_price_tiyin IS NULL OR reference_price_tiyin>0);
ALTER TABLE game_access_grants DROP CONSTRAINT IF EXISTS grant_time_rate_positive;
ALTER TABLE game_access_grants ADD CONSTRAINT grant_time_rate_positive CHECK(time_value_rate IS NULL OR time_value_rate>0);
