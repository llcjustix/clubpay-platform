CREATE TABLE IF NOT EXISTS club_networks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  slug TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE clubs ADD COLUMN IF NOT EXISTS network_id UUID REFERENCES club_networks(id);
CREATE INDEX IF NOT EXISTS clubs_network_idx ON clubs(network_id);

CREATE TABLE IF NOT EXISTS user_network_roles (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  network_id UUID NOT NULL REFERENCES club_networks(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('owner')),
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, network_id)
);
CREATE INDEX IF NOT EXISTS user_network_roles_network_idx ON user_network_roles(network_id, role);

INSERT INTO club_networks (name, slug, status)
VALUES
  ('ClubPay Demo Network', 'clubpay-demo-network', 'active'),
  ('Next Level Network', 'next-level-network', 'active')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, status = EXCLUDED.status, updated_at = now();

UPDATE clubs
SET network_id = (SELECT id FROM club_networks WHERE slug = 'clubpay-demo-network')
WHERE network_id IS NULL;

INSERT INTO user_network_roles (user_id, network_id, role, status)
SELECT DISTINCT ucr.user_id, c.network_id, 'owner', 'active'
FROM user_club_roles ucr
JOIN clubs c ON c.id = ucr.club_id
WHERE ucr.role = 'owner'
  AND ucr.status = 'active'
  AND c.network_id IS NOT NULL
ON CONFLICT (user_id, network_id) DO UPDATE SET role = EXCLUDED.role, status = 'active', updated_at = now();

UPDATE user_club_roles ucr
SET status = 'deleted', updated_at = now()
FROM clubs c
WHERE c.id = ucr.club_id
  AND c.network_id IS NOT NULL
  AND ucr.role = 'owner'
  AND ucr.status = 'active';

WITH next_network AS (
  SELECT id FROM club_networks WHERE slug = 'next-level-network'
), next_club AS (
  INSERT INTO clubs (network_id, name, slug, legal_name, timezone, status, ofd_service_name)
  SELECT id, 'Next Level Cyber Club', 'next-level-cyber-club', 'Next Level Cyber Club LLC', 'Asia/Tashkent', 'active', 'Компьютерное время'
  FROM next_network
  ON CONFLICT (name) DO UPDATE SET network_id = EXCLUDED.network_id, status = 'active'
  RETURNING id
), selected_next_club AS (
  SELECT id FROM next_club
  UNION ALL
  SELECT id FROM clubs WHERE slug = 'next-level-cyber-club'
  LIMIT 1
), standard_zone AS (
  INSERT INTO zones (club_id, name, hourly_price_tiyin, sort_order, status)
  SELECT id, 'Standard', 1800000, 1, 'active' FROM selected_next_club
  ON CONFLICT (club_id, name) DO UPDATE SET hourly_price_tiyin = EXCLUDED.hourly_price_tiyin, sort_order = EXCLUDED.sort_order, status = 'active'
  RETURNING id, club_id
), vip_zone AS (
  INSERT INTO zones (club_id, name, hourly_price_tiyin, sort_order, status)
  SELECT id, 'VIP', 3000000, 2, 'active' FROM selected_next_club
  ON CONFLICT (club_id, name) DO UPDATE SET hourly_price_tiyin = EXCLUDED.hourly_price_tiyin, sort_order = EXCLUDED.sort_order, status = 'active'
  RETURNING id, club_id
), standard_tariffs AS (
  INSERT INTO tariff_blocks (club_id, zone_id, name, duration_minutes, price_tiyin, sort_order, status)
  SELECT club_id, id, '1 час', 60, 1800000, 1, 'active' FROM standard_zone
  UNION ALL SELECT club_id, id, '2 часа', 120, 3400000, 2, 'active' FROM standard_zone
  ON CONFLICT (zone_id, duration_minutes) DO UPDATE SET price_tiyin = EXCLUDED.price_tiyin, name = EXCLUDED.name, sort_order = EXCLUDED.sort_order, status = 'active'
), vip_tariffs AS (
  INSERT INTO tariff_blocks (club_id, zone_id, name, duration_minutes, price_tiyin, sort_order, status)
  SELECT club_id, id, '1 час', 60, 3000000, 1, 'active' FROM vip_zone
  ON CONFLICT (zone_id, duration_minutes) DO UPDATE SET price_tiyin = EXCLUDED.price_tiyin, name = EXCLUDED.name, sort_order = EXCLUDED.sort_order, status = 'active'
), seeded_pcs AS (
  INSERT INTO pc_refs (club_id, zone_id, external_pc_id, number, label, status_cache)
  SELECT sz.club_id, sz.id, 'next-pc-' || lpad(gs::text, 3, '0'), gs, 'Next PC #' || lpad(gs::text, 2, '0'), 'available'
  FROM standard_zone sz, generate_series(1, 3) gs
  UNION ALL
  SELECT vz.club_id, vz.id, 'next-pc-004', 4, 'Next PC #04', 'available'
  FROM vip_zone vz
  ON CONFLICT (club_id, number) DO UPDATE SET zone_id = EXCLUDED.zone_id, label = EXCLUDED.label, status_cache = 'available'
  RETURNING id, club_id, external_pc_id
)
INSERT INTO qr_codes (club_id, pc_ref_id, public_token, type, status)
SELECT club_id, id, external_pc_id, 'static_pc', 'active'
FROM seeded_pcs
ON CONFLICT (public_token) DO UPDATE SET club_id = EXCLUDED.club_id, pc_ref_id = EXCLUDED.pc_ref_id, status = 'active';
