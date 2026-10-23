#!/bin/bash
set -euo pipefail
umask 077
cd /backup
[ -f PURGE_COMPLETED ] && [ -f PROTECTED_FINGERPRINTS_MATCH ] && [ -f BACKUP_READABLE ]
[ ! -f COMPACTION_STARTED ]
[ "$(df -Pk /backup | awk 'NR==2 {print $4}')" -gt 8388608 ]
sha256sum -c clubpay-full.dump.sha256 > compact-backup-check.txt
export PGAPPNAME=clubpay-final-core-events-compaction
export PGOPTIONS='-c statement_timeout=90000 -c lock_timeout=2000'
psql=(psql -X --no-password -v ON_ERROR_STOP=1 -At)
check=$("${psql[@]}" -c "SELECT current_database()='clubpay' AND NOT EXISTS(SELECT 1 FROM core_events WHERE event_type='edge_edge_snapshot') AND (SELECT count(*) FROM core_events)>=430000 AND NOT EXISTS(SELECT 1 FROM game_access_grants WHERE status='active' AND ended_at IS NULL AND (planned_ends_at IS NULL OR planned_ends_at>now()));")
[ "$check" = t ]
"${psql[@]}" -f /protected-fingerprints.sql > protected-fingerprints-before-compaction.txt
cmp -s protected-fingerprints-before.txt protected-fingerprints-before-compaction.txt
"${psql[@]}" -c 'SELECT pg_database_size(current_database()),pg_total_relation_size('\''public.core_events'\'');' > compact-before-bytes.txt
date -u +%FT%TZ > COMPACTION_STARTED
trap 'rc=$?; if [ "$rc" -ne 0 ]; then date -u +%FT%TZ > COMPACTION_FAILED; fi' EXIT
"${psql[@]}" -c 'VACUUM (FULL, VERBOSE, ANALYZE) public.core_events;' > compact.stdout 2> compact.stderr
"${psql[@]}" -f /protected-fingerprints.sql > protected-fingerprints-after-compaction.txt
cmp -s protected-fingerprints-before.txt protected-fingerprints-after-compaction.txt
"${psql[@]}" -c 'SELECT pg_database_size(current_database()),pg_total_relation_size('\''public.core_events'\''); SELECT count(*) FROM core_events; SELECT count(*) FROM core_events WHERE event_type='\''edge_edge_snapshot'\'';' > compact-after-bytes-and-counts.txt
date -u +%FT%TZ > COMPACTION_COMPLETED
