CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS clubs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  slug TEXT,
  legal_name TEXT,
  tin TEXT,
  address TEXT,
  timezone TEXT NOT NULL DEFAULT 'Asia/Tashkent',
  status TEXT NOT NULL DEFAULT 'active',
  click_merchant_id TEXT,
  click_service_id TEXT,
  click_secret_key TEXT,
  payme_merchant_id TEXT,
  payme_secret_key TEXT,
  platform_fee_bps INT NOT NULL DEFAULT 0,
  ofd_mxik TEXT,
  ofd_package_code TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE clubs ADD COLUMN IF NOT EXISTS slug TEXT;
ALTER TABLE clubs DROP COLUMN IF EXISTS rahmat_store_id;
ALTER TABLE clubs DROP COLUMN IF EXISTS rahmat_recipient_uuid;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS click_merchant_id TEXT;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS click_service_id TEXT;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS click_secret_key TEXT;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS payme_merchant_id TEXT;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS payme_secret_key TEXT;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS platform_fee_bps INT NOT NULL DEFAULT 0;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS ofd_mxik TEXT;
ALTER TABLE clubs ADD COLUMN IF NOT EXISTS ofd_package_code TEXT;
UPDATE clubs SET click_merchant_id = NULL WHERE lower(COALESCE(click_merchant_id, '')) IN ('click_test_merchant', 'test_merchant');
UPDATE clubs SET click_service_id = NULL WHERE lower(COALESCE(click_service_id, '')) IN ('click_test_service', 'test_service');
UPDATE clubs SET click_secret_key = NULL WHERE lower(COALESCE(click_secret_key, '')) IN ('click_test_secret', 'test_secret');
UPDATE clubs SET payme_merchant_id = NULL WHERE lower(COALESCE(payme_merchant_id, '')) IN ('payme_test_merchant', 'test_merchant');
UPDATE clubs SET payme_secret_key = NULL WHERE lower(COALESCE(payme_secret_key, '')) IN ('payme_test_secret', 'test_secret');
ALTER TABLE clubs ALTER COLUMN timezone SET DEFAULT 'Asia/Tashkent';
UPDATE clubs SET timezone = 'Asia/Tashkent' WHERE timezone IS NULL OR timezone = '' OR timezone = 'Asia/Samarkand';
UPDATE clubs
SET slug = lower(regexp_replace(regexp_replace(name, '[^a-zA-Z0-9]+', '-', 'g'), '(^-|-$)', '', 'g'))
WHERE slug IS NULL OR slug = '';
CREATE UNIQUE INDEX IF NOT EXISTS clubs_slug_uidx ON clubs(slug) WHERE slug IS NOT NULL;

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID REFERENCES clubs(id),
  name TEXT NOT NULL,
  email TEXT,
  phone TEXT,
  role TEXT NOT NULL CHECK (role IN ('owner', 'manager', 'admin')),
  global_role TEXT NOT NULL DEFAULT '',
  password_hash TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS global_role TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
UPDATE users SET role = 'admin' WHERE role = 'support';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('owner', 'manager', 'admin', 'super_admin'));
CREATE UNIQUE INDEX IF NOT EXISTS users_email_uidx ON users(lower(email)) WHERE email IS NOT NULL AND email <> '';
CREATE UNIQUE INDEX IF NOT EXISTS users_phone_uidx ON users(phone) WHERE phone IS NOT NULL AND phone <> '';

CREATE TABLE IF NOT EXISTS user_club_roles (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('owner', 'manager', 'admin')),
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, club_id)
);
ALTER TABLE user_club_roles DROP CONSTRAINT IF EXISTS user_club_roles_role_check;
UPDATE user_club_roles SET role = 'admin' WHERE role = 'support';
ALTER TABLE user_club_roles ADD CONSTRAINT user_club_roles_role_check CHECK (role IN ('owner', 'manager', 'admin'));

CREATE TABLE IF NOT EXISTS auth_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS auth_sessions_user_idx ON auth_sessions(user_id);
CREATE INDEX IF NOT EXISTS user_club_roles_club_idx ON user_club_roles(club_id, role);

CREATE TABLE IF NOT EXISTS zones (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  hourly_price_tiyin BIGINT NOT NULL DEFAULT 1500000 CHECK (hourly_price_tiyin > 0),
  sort_order INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (club_id, name)
);
ALTER TABLE zones ADD COLUMN IF NOT EXISTS hourly_price_tiyin BIGINT NOT NULL DEFAULT 1500000;

CREATE TABLE IF NOT EXISTS pc_refs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  zone_id UUID NOT NULL REFERENCES zones(id),
  external_pc_id TEXT NOT NULL,
  number INT NOT NULL,
  label TEXT NOT NULL,
  status_cache TEXT NOT NULL DEFAULT 'available',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (club_id, external_pc_id),
  UNIQUE (club_id, number)
);

CREATE TABLE IF NOT EXISTS tariff_blocks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  zone_id UUID NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  duration_minutes INT NOT NULL CHECK (duration_minutes > 0),
  price_tiyin BIGINT NOT NULL CHECK (price_tiyin > 0),
  status TEXT NOT NULL DEFAULT 'active',
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (zone_id, duration_minutes)
);
UPDATE zones z
SET hourly_price_tiyin = t.price_tiyin
FROM tariff_blocks t
WHERE t.zone_id = z.id
  AND t.duration_minutes = 60
  AND t.status <> 'deleted';

CREATE TABLE IF NOT EXISTS qr_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  pc_ref_id UUID NOT NULL REFERENCES pc_refs(id) ON DELETE CASCADE,
  public_token TEXT NOT NULL UNIQUE,
  type TEXT NOT NULL CHECK (type IN ('static_pc', 'session_extend')),
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payment_orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id TEXT NOT NULL UNIQUE,
  provider TEXT NOT NULL DEFAULT 'mock',
  provider_payment_id TEXT,
  provider_prepare_id TEXT,
  provider_time_ms BIGINT,
  provider_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  club_id UUID NOT NULL REFERENCES clubs(id),
  pc_ref_id UUID NOT NULL REFERENCES pc_refs(id),
  tariff_block_id UUID REFERENCES tariff_blocks(id),
  amount_tiyin BIGINT NOT NULL,
  duration_minutes INT NOT NULL,
  duration_seconds INT NOT NULL DEFAULT 0,
  voucher_id UUID,
  status TEXT NOT NULL DEFAULT 'created',
  provider_status TEXT,
  checkout_url TEXT,
  receipt_url TEXT,
  receipt_kind TEXT NOT NULL DEFAULT 'provider_receipt',
  fiscal_status TEXT NOT NULL DEFAULT 'not_requested',
  split_platform_amount_tiyin BIGINT NOT NULL DEFAULT 0,
  split_club_amount_tiyin BIGINT NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ,
  paid_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE payment_orders DROP COLUMN IF EXISTS provider_uuid;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'mock';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider_payment_id TEXT;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider_prepare_id TEXT;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider_time_ms BIGINT;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider_payload JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS provider_status TEXT;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS receipt_kind TEXT NOT NULL DEFAULT 'provider_receipt';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS fiscal_status TEXT NOT NULL DEFAULT 'not_requested';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS split_platform_amount_tiyin BIGINT NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS split_club_amount_tiyin BIGINT NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS extension_grant_id UUID;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS duration_seconds INT NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS voucher_id UUID;
ALTER TABLE payment_orders ALTER COLUMN tariff_block_id DROP NOT NULL;
ALTER TABLE payment_orders ALTER COLUMN receipt_kind SET DEFAULT 'provider_receipt';
UPDATE payment_orders SET receipt_kind = 'provider_receipt' WHERE receipt_kind = 'multicard_receipt';
UPDATE payment_orders SET duration_seconds = duration_minutes * 60 WHERE duration_seconds <= 0;

CREATE INDEX IF NOT EXISTS payment_orders_provider_payment_idx ON payment_orders(provider, provider_payment_id);
CREATE INDEX IF NOT EXISTS payment_orders_status_idx ON payment_orders(status);

CREATE TABLE IF NOT EXISTS payments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  payment_order_id UUID NOT NULL REFERENCES payment_orders(id) ON DELETE CASCADE,
  provider TEXT NOT NULL DEFAULT 'mock',
  provider_payment_id TEXT NOT NULL,
  amount_tiyin BIGINT NOT NULL,
  status TEXT NOT NULL,
  ps TEXT,
  card_pan TEXT,
  receipt_url TEXT,
  paid_at TIMESTAMPTZ,
  raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_payment_id)
);

ALTER TABLE payments ADD COLUMN IF NOT EXISTS provider_payment_id TEXT;
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payments' AND column_name = 'provider_payment_uuid'
  ) THEN
    EXECUTE 'UPDATE payments SET provider_payment_id = provider_payment_uuid WHERE provider_payment_id IS NULL';
  END IF;
END $$;
ALTER TABLE payments DROP COLUMN IF EXISTS provider_payment_uuid;
ALTER TABLE payments ALTER COLUMN provider SET DEFAULT 'mock';
CREATE UNIQUE INDEX IF NOT EXISTS payments_provider_payment_uidx ON payments(provider, provider_payment_id);

CREATE TABLE IF NOT EXISTS provider_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider TEXT NOT NULL DEFAULT 'mock',
  event_type TEXT NOT NULL,
  external_id TEXT,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'received',
  processed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS game_access_grants (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id),
  pc_ref_id UUID NOT NULL REFERENCES pc_refs(id),
  payment_order_id UUID REFERENCES payment_orders(id),
  cash_payment_id UUID,
  duration_minutes INT NOT NULL,
  duration_seconds INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  core_session_id TEXT,
  voucher_id UUID,
  returned_voucher_id UUID,
  source TEXT NOT NULL DEFAULT 'online_payment',
  accepted_at TIMESTAMPTZ,
  planned_ends_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  end_reason TEXT,
  remaining_minutes INT NOT NULL DEFAULT 0,
  remaining_seconds INT NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS voucher_id UUID;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS returned_voucher_id UUID;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS planned_ends_at TIMESTAMPTZ;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS ended_at TIMESTAMPTZ;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS end_reason TEXT;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS remaining_minutes INT NOT NULL DEFAULT 0;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS duration_seconds INT NOT NULL DEFAULT 0;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS remaining_seconds INT NOT NULL DEFAULT 0;
ALTER TABLE game_access_grants ADD COLUMN IF NOT EXISTS last_error TEXT;
UPDATE game_access_grants SET duration_seconds = duration_minutes * 60 WHERE duration_seconds <= 0;
UPDATE game_access_grants SET remaining_seconds = remaining_minutes * 60 WHERE remaining_seconds <= 0 AND remaining_minutes > 0;

CREATE INDEX IF NOT EXISTS game_access_grants_pc_idx ON game_access_grants(pc_ref_id, created_at DESC);

CREATE TABLE IF NOT EXISTS vouchers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id),
  original_payment_order_id UUID REFERENCES payment_orders(id),
  minutes_left INT NOT NULL CHECK (minutes_left > 0),
  seconds_left INT NOT NULL DEFAULT 0,
  code_hash TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'active',
  expires_at TIMESTAMPTZ NOT NULL,
  redeemed_grant_id UUID,
  redeemed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE vouchers ADD COLUMN IF NOT EXISTS redeemed_grant_id UUID;
ALTER TABLE vouchers ADD COLUMN IF NOT EXISTS recipient_phone TEXT;
ALTER TABLE vouchers ADD COLUMN IF NOT EXISTS delivery_channel TEXT;
ALTER TABLE vouchers ADD COLUMN IF NOT EXISTS delivery_status TEXT NOT NULL DEFAULT 'not_requested';
ALTER TABLE vouchers ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;
ALTER TABLE vouchers ADD COLUMN IF NOT EXISTS public_code TEXT;
ALTER TABLE vouchers ADD COLUMN IF NOT EXISTS seconds_left INT NOT NULL DEFAULT 0;
UPDATE vouchers SET seconds_left = minutes_left * 60 WHERE seconds_left <= 0;
UPDATE game_access_grants g
SET returned_voucher_id = g.voucher_id
FROM vouchers v
WHERE g.returned_voucher_id IS NULL
  AND g.voucher_id = v.id
  AND NOT EXISTS (
    SELECT 1
    FROM payment_orders po
    WHERE po.id = g.payment_order_id AND po.voucher_id = g.voucher_id
  )
  AND (v.redeemed_grant_id IS NULL OR v.redeemed_grant_id <> g.id);

CREATE TABLE IF NOT EXISTS telegram_users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  phone TEXT NOT NULL UNIQUE,
  chat_id TEXT NOT NULL UNIQUE,
  username TEXT,
  first_name TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS telegram_link_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  phone TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'active',
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  chat_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS telegram_link_tokens_phone_idx ON telegram_link_tokens(phone, status, expires_at);

CREATE TABLE IF NOT EXISTS core_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id TEXT NOT NULL UNIQUE,
  event_type TEXT NOT NULL,
  club_id UUID REFERENCES clubs(id),
  external_pc_id TEXT,
  core_session_id TEXT,
  grant_id UUID,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL DEFAULT 'received',
  occurred_at TIMESTAMPTZ,
  processed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS edge_sync_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  node_id TEXT NOT NULL,
  club_id UUID,
  direction TEXT NOT NULL CHECK (direction IN ('push', 'pull')),
  status TEXT NOT NULL DEFAULT 'started',
  event_id TEXT,
  error TEXT,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS edge_sync_runs_node_idx ON edge_sync_runs(node_id, started_at DESC);

CREATE TABLE IF NOT EXISTS cash_payments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID NOT NULL REFERENCES clubs(id),
  admin_user_id UUID REFERENCES users(id),
  pc_ref_id UUID NOT NULL REFERENCES pc_refs(id),
  tariff_block_id UUID REFERENCES tariff_blocks(id),
  amount_tiyin BIGINT NOT NULL,
  duration_minutes INT NOT NULL,
  duration_seconds INT NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT 'cash',
  fiscal_reference TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE cash_payments ALTER COLUMN tariff_block_id DROP NOT NULL;
ALTER TABLE cash_payments ADD COLUMN IF NOT EXISTS duration_seconds INT NOT NULL DEFAULT 0;
UPDATE cash_payments SET duration_seconds = duration_minutes * 60 WHERE duration_seconds <= 0;

CREATE TABLE IF NOT EXISTS audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id UUID REFERENCES clubs(id),
  actor_user_id UUID REFERENCES users(id),
  action TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

UPDATE users
SET
  name = 'Clubpay Super Admin',
  email = 'superadmin@clubpay.local',
  role = 'super_admin',
  global_role = 'super_admin',
  password_hash = '14fc0a272b7f9316762fee996f8a553f76acdb155113f7e22020ba25c87e687c',
  status = 'active',
  updated_at = now()
WHERE phone = '+998900000000'
   OR email = 'superadmin@clubpay.local'
   OR (global_role = 'super_admin' AND email = 'admin@clubpay.local');

UPDATE users
SET
  email = 'owner@clubpay.local',
  role = 'owner',
  password_hash = 'f1aacffd5fc4cd7e018dd516d1e4b2b29e8618292024e3d3995456b588f769c8',
  status = 'active',
  updated_at = now()
WHERE phone = '+998000000000'
  AND NOT EXISTS (
    SELECT 1 FROM users u
    WHERE lower(COALESCE(u.email, '')) = 'owner@clubpay.local'
      AND u.phone <> '+998000000000'
  );

UPDATE users
SET role = 'owner',
    password_hash = 'f1aacffd5fc4cd7e018dd516d1e4b2b29e8618292024e3d3995456b588f769c8',
    status = 'active',
    updated_at = now()
WHERE phone = '+998000000000';

UPDATE users
SET role = 'owner',
    password_hash = 'f1aacffd5fc4cd7e018dd516d1e4b2b29e8618292024e3d3995456b588f769c8',
    status = 'active',
    updated_at = now()
WHERE email IN ('owner@clubpay.local', 'owner@test.local');

UPDATE users
SET
  email = 'admin@clubpay.local',
  role = 'admin',
  password_hash = 'a3c75316794fb37d045f6b84db41904b645ce82593b439ace3501f86f755e6ec',
  status = 'active',
  updated_at = now()
WHERE phone = '+998000000001'
  AND NOT EXISTS (
    SELECT 1 FROM users u
    WHERE lower(COALESCE(u.email, '')) = 'admin@clubpay.local'
      AND u.phone <> '+998000000001'
  );

UPDATE users
SET role = 'admin',
    password_hash = 'a3c75316794fb37d045f6b84db41904b645ce82593b439ace3501f86f755e6ec',
    status = 'active',
    updated_at = now()
WHERE phone = '+998000000001';

UPDATE users
SET role = 'admin',
    password_hash = 'a3c75316794fb37d045f6b84db41904b645ce82593b439ace3501f86f755e6ec',
    status = 'active',
    updated_at = now()
WHERE email = 'admin@clubpay.local';

WITH club AS (
  INSERT INTO clubs (name, slug, legal_name, timezone, status, ofd_mxik, ofd_package_code)
  VALUES ('Test Cyber Club', 'test-cyber-club', 'Test Cyber Club LLC', 'Asia/Tashkent', 'active', '06401004002000000', '1506113')
  ON CONFLICT DO NOTHING
  RETURNING id
), selected_club AS (
  SELECT id FROM club
  UNION
  SELECT id FROM clubs WHERE name = 'Test Cyber Club'
  LIMIT 1
), super_user AS (
  INSERT INTO users (name, email, phone, role, global_role, password_hash)
  VALUES ('Clubpay Super Admin', 'superadmin@clubpay.local', '+998900000000', 'super_admin', 'super_admin', '14fc0a272b7f9316762fee996f8a553f76acdb155113f7e22020ba25c87e687c')
  ON CONFLICT DO NOTHING
  RETURNING id
), owner_user AS (
  INSERT INTO users (club_id, name, email, phone, role, password_hash)
  SELECT id, 'Owner Demo', 'owner@clubpay.local', '+998000000000', 'owner', 'f1aacffd5fc4cd7e018dd516d1e4b2b29e8618292024e3d3995456b588f769c8'
  FROM selected_club
  WHERE NOT EXISTS (
    SELECT 1 FROM users u WHERE u.phone = '+998000000000' OR lower(COALESCE(u.email, '')) = 'owner@clubpay.local'
  )
  RETURNING id
), selected_owner AS (
  SELECT id FROM owner_user
  UNION
  SELECT id FROM users WHERE phone = '+998000000000' OR lower(COALESCE(email, '')) = 'owner@clubpay.local'
  LIMIT 1
), owner_membership AS (
  INSERT INTO user_club_roles (user_id, club_id, role)
  SELECT selected_owner.id, selected_club.id, 'owner'
  FROM selected_owner, selected_club
  ON CONFLICT (user_id, club_id) DO UPDATE SET role = EXCLUDED.role, status = 'active'
  RETURNING user_id
), admin_user AS (
  INSERT INTO users (club_id, name, email, phone, role, password_hash)
  SELECT id, 'Admin Demo', 'admin@clubpay.local', '+998000000001', 'admin', 'a3c75316794fb37d045f6b84db41904b645ce82593b439ace3501f86f755e6ec'
  FROM selected_club
  WHERE NOT EXISTS (
    SELECT 1 FROM users u WHERE u.phone = '+998000000001' OR lower(COALESCE(u.email, '')) = 'admin@clubpay.local'
  )
  RETURNING id
), selected_admin AS (
  SELECT id FROM admin_user
  UNION
  SELECT id FROM users WHERE phone = '+998000000001' OR lower(COALESCE(email, '')) = 'admin@clubpay.local'
  LIMIT 1
), admin_membership AS (
  INSERT INTO user_club_roles (user_id, club_id, role)
  SELECT selected_admin.id, selected_club.id, 'admin'
  FROM selected_admin, selected_club
  ON CONFLICT (user_id, club_id) DO UPDATE SET role = EXCLUDED.role, status = 'active'
  RETURNING user_id
), standard_zone AS (
  INSERT INTO zones (club_id, name, hourly_price_tiyin, sort_order)
  SELECT id, 'Standard', 1500000, 1 FROM selected_club
  ON CONFLICT (club_id, name) DO UPDATE SET hourly_price_tiyin = EXCLUDED.hourly_price_tiyin, sort_order = EXCLUDED.sort_order
  RETURNING id, club_id
), vip_zone AS (
  INSERT INTO zones (club_id, name, hourly_price_tiyin, sort_order)
  SELECT id, 'VIP', 2500000, 2 FROM selected_club
  ON CONFLICT (club_id, name) DO UPDATE SET hourly_price_tiyin = EXCLUDED.hourly_price_tiyin, sort_order = EXCLUDED.sort_order
  RETURNING id, club_id
), standard_tariffs AS (
  INSERT INTO tariff_blocks (club_id, zone_id, name, duration_minutes, price_tiyin, sort_order)
  SELECT club_id, id, '1 минута', 1, 100000, 0 FROM standard_zone
  UNION ALL SELECT club_id, id, '30 минут', 30, 800000, 1 FROM standard_zone
  UNION ALL SELECT club_id, id, '1 час', 60, 1500000, 2 FROM standard_zone
  UNION ALL SELECT club_id, id, '2 часа', 120, 2800000, 3 FROM standard_zone
  ON CONFLICT (zone_id, duration_minutes) DO UPDATE SET price_tiyin = EXCLUDED.price_tiyin, name = EXCLUDED.name, sort_order = EXCLUDED.sort_order
), vip_tariffs AS (
  INSERT INTO tariff_blocks (club_id, zone_id, name, duration_minutes, price_tiyin, sort_order)
  SELECT club_id, id, '1 час', 60, 2500000, 1 FROM vip_zone
  UNION ALL SELECT club_id, id, '2 часа', 120, 4500000, 2 FROM vip_zone
  ON CONFLICT (zone_id, duration_minutes) DO UPDATE SET price_tiyin = EXCLUDED.price_tiyin, name = EXCLUDED.name, sort_order = EXCLUDED.sort_order
), seeded_pcs AS (
  INSERT INTO pc_refs (club_id, zone_id, external_pc_id, number, label, status_cache)
  SELECT sz.club_id, sz.id, 'pc-' || lpad(gs::text, 3, '0'), gs, 'PC #' || lpad(gs::text, 2, '0'), 'available'
  FROM standard_zone sz, generate_series(1, 8) gs
  UNION ALL
  SELECT vz.club_id, vz.id, 'pc-' || lpad(gs::text, 3, '0'), gs, 'PC #' || lpad(gs::text, 2, '0'), 'available'
  FROM vip_zone vz, generate_series(9, 10) gs
  ON CONFLICT (club_id, number) DO UPDATE SET zone_id = EXCLUDED.zone_id, label = EXCLUDED.label
  RETURNING id, club_id, number
)
INSERT INTO qr_codes (club_id, pc_ref_id, public_token, type)
SELECT club_id, id, 'pc-' || lpad(number::text, 3, '0'), 'static_pc'
FROM seeded_pcs
ON CONFLICT (public_token) DO NOTHING;
