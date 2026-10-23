\set ON_ERROR_STOP on
\pset pager off
BEGIN READ ONLY;
SET LOCAL statement_timeout='15s'; SET LOCAL lock_timeout='2s';
SELECT current_database(), current_user, inet_server_addr(), inet_server_port(), current_setting('server_version'), pg_database_size(current_database());
SELECT rolname,rolsuper,rolcreatedb,rolreplication FROM pg_roles WHERE rolname=current_user;
SELECT name,setting FROM pg_settings WHERE name IN ('data_directory','archive_mode','max_wal_size','min_wal_size','wal_level','max_slot_wal_keep_size');
SELECT slot_type,active,count(*),max(pg_wal_lsn_diff(pg_current_wal_lsn(),restart_lsn)) AS max_retained_wal_bytes FROM pg_replication_slots GROUP BY 1,2;
SELECT extname,extversion FROM pg_extension;
SELECT count(*) AS eligible_snapshot_rows FROM core_events WHERE event_type='edge_edge_snapshot' AND status='processed' AND club_id IS NOT NULL AND grant_id IS NULL AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL AND created_at<timestamptz '2026-10-10T13:25:16Z';
SELECT relname,pg_total_relation_size(relid) AS total_bytes FROM pg_stat_user_tables ORDER BY 2 DESC LIMIT 8;
SELECT conname,conrelid::regclass,confrelid::regclass FROM pg_constraint WHERE contype='f' AND confrelid='public.core_events'::regclass;
SELECT tgname,tgenabled,pg_get_triggerdef(oid) FROM pg_trigger WHERE tgrelid='public.core_events'::regclass AND NOT tgisinternal;
SELECT p.command,p.relid::regclass,p.bytes_processed,p.bytes_total,p.tuples_processed,clock_timestamp()-a.query_start AS elapsed FROM pg_stat_progress_copy p JOIN pg_stat_activity a ON a.pid=p.pid WHERE a.application_name='clubpay-precleanup-backup';
COMMIT;
