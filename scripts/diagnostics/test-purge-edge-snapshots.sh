#!/usr/bin/env bash
# CI only: refuses any database with an existing public.core_events table.
set -euo pipefail
if [ "${SNAPSHOT_PURGE_DISPOSABLE_TEST:-}" != 1 ] || [ "${PGHOST:-}" != 127.0.0.1 ] || [ "${PGDATABASE:-}" != clubpay ]; then
  echo 'Requires explicitly disposable localhost PostgreSQL' >&2
  exit 2
fi
snapshot_test_dir="$(cd "$(dirname "$0")" && pwd)"
snapshot_test_psql=(psql -X --no-password -v ON_ERROR_STOP=1 -At)
snapshot_empty="$("${snapshot_test_psql[@]}" <<'SQL'
SELECT to_regclass('public.core_events') IS NULL
   AND to_regclass('public.snapshot_cleanup_financial_fixture') IS NULL;
SQL
)"
if [ "$snapshot_empty" != t ]; then
  echo 'Refusing to touch a database containing existing tables' >&2
  exit 2
fi
"${snapshot_test_psql[@]}" <<'SQL'
BEGIN;
CREATE TABLE public.core_events(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),event_type text,status text,club_id uuid,grant_id uuid,core_session_id text,external_pc_id text,created_at timestamptz,payload jsonb DEFAULT '{}'::jsonb);
CREATE TABLE public.snapshot_cleanup_financial_fixture(id int,amount bigint);
INSERT INTO public.snapshot_cleanup_financial_fixture VALUES(1,500),(2,1500);
INSERT INTO public.core_events(event_type,status,club_id,created_at)
SELECT 'edge_edge_snapshot','processed',gen_random_uuid(),'2019-01-01Z' FROM generate_series(1,2005);
INSERT INTO public.core_events(event_type,status,club_id,created_at)
SELECT 'session_started','processed',gen_random_uuid(),'2019-01-01Z' FROM generate_series(1,7);
INSERT INTO public.core_events(event_type,status,club_id,created_at) VALUES
('edge_edge_snapshot','processed',gen_random_uuid(),'2021-01-01Z'),
('edge_edge_snapshot','failed',gen_random_uuid(),'2019-01-01Z'),
('edge_edge_snapshot','received',gen_random_uuid(),'2019-01-01Z');
INSERT INTO public.core_events(event_type,status,club_id,created_at,grant_id,core_session_id,external_pc_id) VALUES
('edge_edge_snapshot','processed',gen_random_uuid(),'2019-01-01Z',gen_random_uuid(),NULL,NULL),
('edge_edge_snapshot','processed',gen_random_uuid(),'2019-01-01Z',NULL,'real-session',NULL),
('edge_edge_snapshot','processed',gen_random_uuid(),'2019-01-01Z',NULL,NULL,'pc-01');
COMMIT;
SQL
snapshot_fixture_cleanup() {
  "${snapshot_test_psql[@]}" <<'SQL'
DROP TABLE public.core_events;
DROP TABLE public.snapshot_cleanup_financial_fixture;
SQL
}
trap snapshot_fixture_cleanup EXIT
snapshot_fixture_state() {
  "${snapshot_test_psql[@]}" <<'SQL'
SELECT (SELECT count(*) FROM public.core_events),
       (SELECT sum(amount) FROM public.snapshot_cleanup_financial_fixture);
SQL
}
snapshot_before="$(snapshot_fixture_state)"
bash "$snapshot_test_dir/purge-edge-snapshots.sh" --before 2020-01-01T00:00:00Z
[ "$(snapshot_fixture_state)" = "$snapshot_before" ]
if bash "$snapshot_test_dir/purge-edge-snapshots.sh" --before 2099-01-01T00:00:00Z --execute; then
  echo 'Future cutoff was accepted' >&2; exit 1
fi
if PGDATABASE=postgres bash "$snapshot_test_dir/purge-edge-snapshots.sh" --before 2020-01-01T00:00:00Z --execute; then
  echo 'Wrong database was accepted' >&2; exit 1
fi
bash "$snapshot_test_dir/purge-edge-snapshots.sh" --before 2020-01-01T00:00:00Z --execute
[ "$(snapshot_fixture_state)" = '13|2000' ]
snapshot_sessions="$("${snapshot_test_psql[@]}" <<'SQL'
SELECT count(*) FROM public.core_events WHERE event_type='session_started';
SQL
)"
[ "$snapshot_sessions" = 7 ]
echo 'PASS: three cleanup batches, dry-run, future/wrong DB guards, all protected rows and financial sentinel.'
