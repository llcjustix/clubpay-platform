\set ON_ERROR_STOP on
BEGIN READ ONLY;
SELECT 'payments' AS protected_table,count(*) AS rows,md5(COALESCE(string_agg(md5((to_jsonb(t)-'updated_at')::text),'' ORDER BY md5((to_jsonb(t)-'updated_at')::text)),'')) AS content_fingerprint_excluding_updated_at FROM public.payments t;
SELECT 'payment_orders' AS protected_table,count(*) AS rows,md5(COALESCE(string_agg(md5((to_jsonb(t)-'updated_at')::text),'' ORDER BY md5((to_jsonb(t)-'updated_at')::text)),'')) AS content_fingerprint_excluding_updated_at FROM public.payment_orders t;
SELECT 'cash_payments' AS protected_table,count(*) AS rows,md5(COALESCE(string_agg(md5((to_jsonb(t)-'updated_at')::text),'' ORDER BY md5((to_jsonb(t)-'updated_at')::text)),'')) AS content_fingerprint_excluding_updated_at FROM public.cash_payments t;
SELECT 'game_access_grants' AS protected_table,count(*) AS rows,md5(COALESCE(string_agg(md5((to_jsonb(t)-'updated_at')::text),'' ORDER BY md5((to_jsonb(t)-'updated_at')::text)),'')) AS content_fingerprint_excluding_updated_at FROM public.game_access_grants t;
SELECT 'player_club_balances' AS protected_table,count(*) AS rows,md5(COALESCE(string_agg(md5((to_jsonb(t)-'updated_at')::text),'' ORDER BY md5((to_jsonb(t)-'updated_at')::text)),'')) AS content_fingerprint_excluding_updated_at FROM public.player_club_balances t;
SELECT 'player_time_ledger' AS protected_table,count(*) AS rows,md5(COALESCE(string_agg(md5((to_jsonb(t)-'updated_at')::text),'' ORDER BY md5((to_jsonb(t)-'updated_at')::text)),'')) AS content_fingerprint_excluding_updated_at FROM public.player_time_ledger t;
SELECT 'mobile_reservations' AS protected_table,count(*) AS rows,md5(COALESCE(string_agg(md5((to_jsonb(t)-'updated_at')::text),'' ORDER BY md5((to_jsonb(t)-'updated_at')::text)),'')) AS content_fingerprint_excluding_updated_at FROM public.mobile_reservations t;
SELECT 'vouchers' AS protected_table,count(*) AS rows,md5(COALESCE(string_agg(md5((to_jsonb(t)-'updated_at')::text),'' ORDER BY md5((to_jsonb(t)-'updated_at')::text)),'')) AS content_fingerprint_excluding_updated_at FROM public.vouchers t;
COMMIT;
