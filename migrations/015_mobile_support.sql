CREATE TABLE IF NOT EXISTS mobile_support_tickets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  player_id uuid NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS mobile_support_open_ticket_idx
  ON mobile_support_tickets(player_id) WHERE status = 'open';

CREATE TABLE IF NOT EXISTS mobile_support_messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id uuid NOT NULL REFERENCES mobile_support_tickets(id) ON DELETE CASCADE,
  sender text NOT NULL CHECK (sender IN ('player','support')),
  body text NOT NULL CHECK (length(body) BETWEEN 1 AND 2000),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS mobile_support_messages_ticket_idx
  ON mobile_support_messages(ticket_id, created_at);
