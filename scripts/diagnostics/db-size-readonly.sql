\set ON_ERROR_STOP on
\pset pager off
\timing on
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '15s';
SET LOCAL lock_timeout = '2s';
SELECT current_database() = 'clubpay' AS expected_database \gset
\if :expected_database
\else
  \echo 'Refusing to inspect a database other than clubpay'
  -- psql 16 ignores numeric arguments to \quit. ON_ERROR_STOP makes this
  -- deliberate read-only error terminate with a nonzero status instead.
  SELECT 1 / 0 AS wrong_database_refused;
\endif

-- No row contents, user identities, credentials or payment payloads are output.
SELECT current_database() AS database_name, clock_timestamp() AS observed_at_utc,
       current_setting('server_version') AS postgres_version,
       current_setting('transaction_read_only') AS transaction_read_only,
       pg_database_size(current_database()) AS database_bytes,
       pg_size_pretty(pg_database_size(current_database())) AS database_size;

SELECT schemaname, relname,
       pg_total_relation_size(relid) AS total_bytes,
       pg_size_pretty(pg_total_relation_size(relid)) AS total,
       pg_size_pretty(pg_table_size(relid)) AS data_and_toast,
       pg_size_pretty(pg_indexes_size(relid)) AS indexes,
       n_live_tup AS estimated_live_rows, n_dead_tup AS estimated_dead_rows,
       last_autovacuum, last_autoanalyze
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(relid) DESC
LIMIT 20;

-- Catalog statistics are estimates; they do not prove how much disk is bloat.
SELECT schemaname, relname, n_tup_ins, n_tup_upd, n_tup_del, n_tup_hot_upd,
       vacuum_count, autovacuum_count, analyze_count, autoanalyze_count
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(relid) DESC
LIMIT 10;

SELECT name, setting, unit
FROM pg_settings
WHERE name IN ('autovacuum', 'autovacuum_vacuum_scale_factor',
               'autovacuum_vacuum_threshold', 'autovacuum_naptime')
ORDER BY name;

-- A small block sample avoids a full COUNT/GROUP BY over a potentially 37 GB
-- table. Counts below refer only to sampled rows, never exact event totals.
SELECT to_regclass('public.core_events') IS NOT NULL AS has_core_events \gset
\if :has_core_events
SELECT pg_size_pretty(pg_relation_size(c.oid)) AS main_heap,
       pg_size_pretty(pg_total_relation_size(c.reltoastrelid)) AS toast_with_indexes
FROM pg_class c WHERE c.oid='public.core_events'::regclass;
SELECT count(*) FILTER (WHERE event_type='edge_edge_snapshot') AS snapshots_last_two_minutes,
       count(*) FILTER (WHERE event_type<>'edge_edge_snapshot') AS other_events_last_two_minutes,
       max(created_at) FILTER (WHERE event_type='edge_edge_snapshot') AS latest_recent_snapshot
FROM public.core_events
WHERE created_at >= now() - interval '2 minutes';
SELECT event_type, count(*) AS sampled_rows,
       round(avg(pg_column_size(payload))) AS average_stored_payload_bytes,
       min(created_at) AS first_sample_timestamp,
       max(created_at) AS last_sample_timestamp
FROM public.core_events TABLESAMPLE SYSTEM (0.01) REPEATABLE (20261010)
GROUP BY event_type
ORDER BY sampled_rows DESC
LIMIT 20;
\endif
SELECT to_regclass('public.clubs') IS NOT NULL AS has_clubs \gset
\if :has_clubs
SELECT max(controller_synced_at) AS latest_controller_sync,
       count(*) FILTER (WHERE controller_synced_at >= now() - interval '2 minutes') AS recently_synchronized_clubs
FROM public.clubs;
\endif

-- Exact aggregate over event headers, no payload values or personal data.
-- Covers the whole retained history, rather than a sample or two-minute window.
SELECT CASE WHEN created_at < timestamptz '2026-10-10T13:25:16Z'
            THEN 'before_snapshot_fix' ELSE 'after_snapshot_fix' END AS period,
       CASE WHEN club_id=uuid '21bbd882-bf06-4f8d-b2f0-04cf8399c65a'
            THEN 'cpb_physical_pilot' ELSE 'other_or_unassigned' END AS source_group,
       event_type, count(*) AS exact_retained_rows,
       min(created_at) AS first_created_at, max(created_at) AS last_created_at
FROM public.core_events
GROUP BY 1,2,3 ORDER BY 1,2,4 DESC;
SELECT count(*) AS eligible_old_processed_snapshots
FROM public.core_events
WHERE event_type='edge_edge_snapshot' AND status='processed'
  AND club_id IS NOT NULL AND grant_id IS NULL
  AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL
  AND created_at < timestamptz '2026-10-10T13:25:16Z';
SELECT stats_reset FROM pg_stat_database WHERE datname=current_database();

COMMIT;
