ALTER TABLE mobile_reservations
  ADD COLUMN IF NOT EXISTS entry_code char(6);

UPDATE mobile_reservations
SET entry_code = lpad(((random() * 999999)::int)::text, 6, '0')
WHERE entry_code IS NULL;

ALTER TABLE mobile_reservations
  ALTER COLUMN entry_code SET NOT NULL;
