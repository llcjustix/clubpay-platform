-- Static QR URLs are public entry points to checkout. Their token must not
-- reveal a PC number or an Agent/Core external_pc_id. Run this one-time
-- migration only once even though the application replays idempotent SQL files
-- on every start.
CREATE TABLE IF NOT EXISTS migration_markers (
  name TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

WITH first_run AS (
  INSERT INTO migration_markers (name)
  VALUES ('008_opaque_static_qr')
  ON CONFLICT (name) DO NOTHING
  RETURNING name
)
UPDATE qr_codes AS q
SET public_token = 'pc_' || encode(gen_random_bytes(32), 'hex')
FROM first_run
WHERE q.type = 'static_pc';
