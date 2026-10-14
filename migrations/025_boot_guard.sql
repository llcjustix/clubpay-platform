-- CPB is opt-in per station. Expiry removes mutation authority, never the
-- quarantine: a process crash cannot silently make a disk switch safe to play.
CREATE TABLE IF NOT EXISTS boot_guard_stations (
 pc_ref_id UUID PRIMARY KEY REFERENCES pc_refs(id),
 cold_provision BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS boot_guard_leases (
 pc_ref_id UUID PRIMARY KEY REFERENCES boot_guard_stations(pc_ref_id),
 command_id TEXT NOT NULL UNIQUE,
 lease_id TEXT NOT NULL UNIQUE,
 fence BIGINT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 held BOOLEAN NOT NULL DEFAULT true,
 released_at TIMESTAMPTZ
);
CREATE SEQUENCE IF NOT EXISTS boot_guard_fence;
CREATE OR REPLACE FUNCTION cpb_block_scheduling() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM boot_guard_stations WHERE pc_ref_id=NEW.pc_ref_id) THEN RETURN NEW; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('cpb:' || NEW.pc_ref_id::text,0));
 IF EXISTS(SELECT 1 FROM boot_guard_leases WHERE pc_ref_id=NEW.pc_ref_id AND held) THEN
  RAISE EXCEPTION 'cpb_station_fenced' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
-- Cover all cash, mobile, payment and background grant writers, including
-- another Controller connected to this same database.
CREATE OR REPLACE FUNCTION cpb_grant_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' AND OLD.pc_ref_id=NEW.pc_ref_id AND OLD.status IN ('pending','accepted') THEN RETURN NEW; END IF;
 IF NEW.status IN ('pending','accepted') THEN
  IF EXISTS(SELECT 1 FROM boot_guard_stations WHERE pc_ref_id=NEW.pc_ref_id) THEN
   PERFORM pg_advisory_xact_lock(hashtextextended('cpb:' || NEW.pc_ref_id::text,0));
   IF EXISTS(SELECT 1 FROM boot_guard_leases WHERE pc_ref_id=NEW.pc_ref_id AND held) THEN
    RAISE EXCEPTION 'cpb_station_fenced' USING ERRCODE='55000';
   END IF;
  END IF;
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS cpb_grant_guard ON game_access_grants;
CREATE TRIGGER cpb_grant_guard BEFORE INSERT OR UPDATE OF status,pc_ref_id ON game_access_grants FOR EACH ROW EXECUTE FUNCTION cpb_grant_guard();
DROP TRIGGER IF EXISTS cpb_booking_guard ON mobile_reservations;
CREATE TRIGGER cpb_booking_guard BEFORE INSERT ON mobile_reservations FOR EACH ROW EXECUTE FUNCTION cpb_block_scheduling();
