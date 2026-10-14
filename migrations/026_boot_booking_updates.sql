-- Rescheduling or reactivating a booking must use the same station fence as
-- initial booking creation. Cancellation/completion may still drain old state.
CREATE OR REPLACE FUNCTION cpb_booking_admission_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status NOT IN ('confirmed','checked_in','started') THEN RETURN NEW; END IF;
 IF NOT EXISTS(SELECT 1 FROM boot_guard_stations WHERE pc_ref_id=NEW.pc_ref_id) THEN RETURN NEW; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('cpb:' || NEW.pc_ref_id::text,0));
 IF EXISTS(SELECT 1 FROM boot_guard_leases WHERE pc_ref_id=NEW.pc_ref_id AND held) THEN
  RAISE EXCEPTION 'cpb_station_fenced' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS cpb_booking_guard ON mobile_reservations;
CREATE TRIGGER cpb_booking_guard BEFORE INSERT OR UPDATE OF pc_ref_id,starts_at,duration_minutes,status ON mobile_reservations FOR EACH ROW EXECUTE FUNCTION cpb_booking_admission_guard();
