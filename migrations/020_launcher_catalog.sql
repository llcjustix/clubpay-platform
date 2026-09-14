-- Applications are discovered by each Agent but categories are managed centrally.
CREATE TABLE IF NOT EXISTS launcher_apps (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  pc_ref_id UUID NOT NULL REFERENCES pc_refs(id) ON DELETE CASCADE,
  app_key TEXT NOT NULL,
  name TEXT NOT NULL,
  exe_path TEXT NOT NULL DEFAULT '',
  args TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT 'other' CHECK (category IN ('shooter', 'strategy', 'other')),
  source TEXT NOT NULL DEFAULT 'agent',
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (pc_ref_id, app_key)
);
CREATE INDEX IF NOT EXISTS launcher_apps_club_last_seen_idx ON launcher_apps(club_id, last_seen_at DESC);
