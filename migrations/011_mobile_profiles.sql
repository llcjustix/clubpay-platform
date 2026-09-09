-- Mobile profile credentials are independent of QR-scoped player_auth_challenges.
CREATE TABLE IF NOT EXISTS mobile_auth_challenges (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 token_hash TEXT NOT NULL UNIQUE,
 phone TEXT NOT NULL,
 device_hash TEXT NOT NULL,
 chat_id TEXT,
 player_id UUID REFERENCES players(id),
 otp_hash TEXT,
 attempts INT NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','contact','otp','used','expired')),
 expires_at TIMESTAMPTZ NOT NULL,
 otp_expires_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS mobile_challenge_chat ON mobile_auth_challenges(chat_id, created_at DESC);
CREATE TABLE IF NOT EXISTS mobile_sessions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 player_id UUID NOT NULL REFERENCES players(id),
 device_hash TEXT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS mobile_tokens (
 token_hash TEXT PRIMARY KEY,
 session_id UUID NOT NULL REFERENCES mobile_sessions(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK (kind IN ('access','refresh')),
 expires_at TIMESTAMPTZ NOT NULL,
 used_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS mobile_tokens_session ON mobile_tokens(session_id);
CREATE TABLE IF NOT EXISTS mobile_rate_limits (
 bucket TEXT PRIMARY KEY,
 hits INT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL
);
-- A durable reservation prevents duplicate operations even if a process dies.
CREATE TABLE IF NOT EXISTS mobile_operations (
 player_id UUID NOT NULL REFERENCES players(id),
 request_key TEXT NOT NULL,
 request_hash TEXT NOT NULL,
 http_status INT,
 response JSONB,
 invoice_id TEXT,
 grant_id UUID,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(player_id, request_key)
);
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS client_return_url TEXT;
