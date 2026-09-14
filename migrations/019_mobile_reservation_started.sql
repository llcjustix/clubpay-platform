-- A reservation moves to started as soon as its owner opens the normal mobile
-- payment/session flow. This removes the stale reservation card and releases
-- the kiosk from its reservation-only screen while still protecting the PC.
ALTER TABLE mobile_reservations
  DROP CONSTRAINT IF EXISTS mobile_reservations_status_check;
ALTER TABLE mobile_reservations
  ADD CONSTRAINT mobile_reservations_status_check
  CHECK (status IN ('confirmed','checked_in','started','cancelled','expired','completed'));
