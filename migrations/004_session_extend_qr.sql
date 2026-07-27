-- A dynamic extension QR is valid only for the game session that created it.
ALTER TABLE qr_codes
  ADD COLUMN IF NOT EXISTS session_grant_id UUID REFERENCES game_access_grants(id) ON DELETE CASCADE;

ALTER TABLE qr_codes
  ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS qr_codes_session_grant_idx
  ON qr_codes(session_grant_id)
  WHERE session_grant_id IS NOT NULL;
