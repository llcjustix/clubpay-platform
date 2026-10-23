set -euo pipefail
# Local socket, read-only metadata for the authorized clubpay database only.
docker exec -i -u postgres -e 'PGOPTIONS=-c default_transaction_read_only=on -c statement_timeout=15000 -c lock_timeout=1000' postgres psql -X --no-password -v ON_ERROR_STOP=1 -U postgres -d clubpay <<'SQL'
BEGIN READ ONLY;
SELECT current_database(),current_setting('transaction_read_only');
SELECT n.nspname,c.relname,v.phase,v.heap_blks_total,v.heap_blks_scanned,v.heap_blks_vacuumed,clock_timestamp()-a.query_start AS elapsed,a.wait_event_type,a.wait_event
FROM pg_stat_progress_vacuum v JOIN pg_class c ON c.oid=v.relid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_stat_activity a ON a.pid=v.pid WHERE v.datname=current_database();
SELECT c.relname,pg_relation_size(c.oid) AS heap_bytes,pg_indexes_size(c.oid) AS index_bytes,s.n_live_tup,s.n_dead_tup,s.last_autovacuum FROM pg_class c LEFT JOIN pg_stat_all_tables s ON s.relid=c.oid WHERE c.oid=(SELECT reltoastrelid FROM pg_class WHERE oid='public.core_events'::regclass);
COMMIT;
SQL
