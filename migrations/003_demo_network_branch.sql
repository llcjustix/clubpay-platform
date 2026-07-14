WITH demo_network AS (
  SELECT id FROM club_networks WHERE slug = 'clubpay-demo-network'
), branch_club AS (
  INSERT INTO clubs (
    network_id, name, slug, legal_name, timezone, status,
    ofd_mxik, ofd_package_code, ofd_service_name
  )
  SELECT
    id,
    'ClubPay Demo Branch',
    'clubpay-demo-branch',
    'ClubPay Demo Branch LLC',
    'Asia/Tashkent',
    'active',
    '06401004002000000',
    '1506113',
    'Компьютерное время'
  FROM demo_network
  ON CONFLICT (name) DO UPDATE
  SET network_id = EXCLUDED.network_id,
      slug = EXCLUDED.slug,
      legal_name = EXCLUDED.legal_name,
      timezone = EXCLUDED.timezone,
      status = 'active',
      ofd_mxik = EXCLUDED.ofd_mxik,
      ofd_package_code = EXCLUDED.ofd_package_code,
      ofd_service_name = EXCLUDED.ofd_service_name
  RETURNING id
), selected_branch_club AS (
  SELECT id FROM branch_club
  UNION ALL
  SELECT id FROM clubs WHERE slug = 'clubpay-demo-branch'
  LIMIT 1
), standard_zone AS (
  INSERT INTO zones (club_id, name, hourly_price_tiyin, sort_order, status)
  SELECT id, 'Standard', 1500000, 1, 'active' FROM selected_branch_club
  ON CONFLICT (club_id, name) DO UPDATE
  SET hourly_price_tiyin = EXCLUDED.hourly_price_tiyin,
      sort_order = EXCLUDED.sort_order,
      status = 'active'
  RETURNING id, club_id
), vip_zone AS (
  INSERT INTO zones (club_id, name, hourly_price_tiyin, sort_order, status)
  SELECT id, 'VIP', 2500000, 2, 'active' FROM selected_branch_club
  ON CONFLICT (club_id, name) DO UPDATE
  SET hourly_price_tiyin = EXCLUDED.hourly_price_tiyin,
      sort_order = EXCLUDED.sort_order,
      status = 'active'
  RETURNING id, club_id
), standard_tariffs AS (
  INSERT INTO tariff_blocks (club_id, zone_id, name, duration_minutes, price_tiyin, sort_order, status)
  SELECT club_id, id, '1 минута', 1, 100000, 0, 'active' FROM standard_zone
  UNION ALL SELECT club_id, id, '30 минут', 30, 800000, 1, 'active' FROM standard_zone
  UNION ALL SELECT club_id, id, '1 час', 60, 1500000, 2, 'active' FROM standard_zone
  UNION ALL SELECT club_id, id, '2 часа', 120, 2800000, 3, 'active' FROM standard_zone
  ON CONFLICT (zone_id, duration_minutes) DO UPDATE
  SET price_tiyin = EXCLUDED.price_tiyin,
      name = EXCLUDED.name,
      sort_order = EXCLUDED.sort_order,
      status = 'active'
), vip_tariffs AS (
  INSERT INTO tariff_blocks (club_id, zone_id, name, duration_minutes, price_tiyin, sort_order, status)
  SELECT club_id, id, '1 час', 60, 2500000, 1, 'active' FROM vip_zone
  UNION ALL SELECT club_id, id, '2 часа', 120, 4500000, 2, 'active' FROM vip_zone
  ON CONFLICT (zone_id, duration_minutes) DO UPDATE
  SET price_tiyin = EXCLUDED.price_tiyin,
      name = EXCLUDED.name,
      sort_order = EXCLUDED.sort_order,
      status = 'active'
), seeded_pcs AS (
  INSERT INTO pc_refs (club_id, zone_id, external_pc_id, number, label, status_cache)
  SELECT sz.club_id, sz.id, 'demo-branch-pc-' || lpad(gs::text, 3, '0'), gs, 'Branch PC #' || lpad(gs::text, 2, '0'), 'available'
  FROM standard_zone sz, generate_series(1, 8) gs
  UNION ALL
  SELECT vz.club_id, vz.id, 'demo-branch-pc-' || lpad(gs::text, 3, '0'), gs, 'Branch PC #' || lpad(gs::text, 2, '0'), 'available'
  FROM vip_zone vz, generate_series(9, 10) gs
  ON CONFLICT (club_id, number) DO UPDATE
  SET zone_id = EXCLUDED.zone_id,
      external_pc_id = EXCLUDED.external_pc_id,
      label = EXCLUDED.label,
      status_cache = 'available'
  RETURNING id, club_id, external_pc_id
)
INSERT INTO qr_codes (club_id, pc_ref_id, public_token, type, status)
SELECT club_id, id, external_pc_id, 'static_pc', 'active'
FROM seeded_pcs
ON CONFLICT (public_token) DO UPDATE
SET club_id = EXCLUDED.club_id,
    pc_ref_id = EXCLUDED.pc_ref_id,
    status = 'active';
