#!/bin/bash
set -euo pipefail
umask 077
cd /backup
[ -f BACKUP_READABLE ] && [ -f PROTECTED_DATA_RESTORE_VERIFIED ]
[ ! -f PURGE_STARTED ]
sha256sum -c clubpay-full.dump.sha256 > purge-backup-check.txt
export PGAPPNAME=clubpay-authorized-old-snapshot-purge
export PGCONNECT_TIMEOUT=5
export PGOPTIONS='-c statement_timeout=15000 -c lock_timeout=1000'
psql=(psql -X --no-password -v ON_ERROR_STOP=1 -At)
qualified="event_type='edge_edge_snapshot' AND status='processed' AND club_id IS NOT NULL AND grant_id IS NULL AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL AND created_at<timestamptz '2026-10-10T13:25:16Z'"
check=$("${psql[@]}" -c "SELECT current_database()='clubpay' AND NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.core_events'::regclass AND NOT tgisinternal) AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE contype='f' AND confrelid='public.core_events'::regclass) AND NOT EXISTS(SELECT 1 FROM public.core_events WHERE event_type='edge_edge_snapshot' AND created_at>=timestamptz '2026-10-10T13:25:16Z');")
[ "$check" = t ]
expected=$("${psql[@]}" -c "SELECT count(*) FROM public.core_events WHERE $qualified;")
[ "$expected" -eq 1932639 ]
"${psql[@]}" -c "SELECT 'protected_events',count(*) FROM public.core_events WHERE NOT ($qualified) OR ($qualified) IS NULL; SELECT 'payments',count(*) FROM public.payments; SELECT 'payment_orders',count(*) FROM public.payment_orders; SELECT 'grants',count(*) FROM public.game_access_grants; SELECT 'ledger',count(*) FROM public.player_time_ledger;" > purge-before-counts.txt
date -u +%FT%TZ > PURGE_STARTED
trap 'rc=$?; if [ "$rc" -ne 0 ]; then date -u +%FT%TZ > PURGE_FAILED; fi' EXIT
cursor=''; total=0; started=$SECONDS
while :; do
 [ "$(df -Pk /backup | awk 'NR==2 {print $4}')" -gt 8388608 ]
 [ "$((SECONDS-started))" -lt 1800 ]
 result=$("${psql[@]}" -v "after=$cursor" <<'SQL'
WITH batch AS MATERIALIZED (
 SELECT id FROM public.core_events
 WHERE event_type='edge_edge_snapshot' AND status='processed'
 AND club_id IS NOT NULL AND grant_id IS NULL
 AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL
 AND created_at<timestamptz '2026-10-10T13:25:16Z'
 AND (NULLIF(:'after','') IS NULL OR id>NULLIF(:'after','')::uuid)
 ORDER BY id LIMIT 1000 FOR UPDATE
), deleted AS (
 DELETE FROM public.core_events e USING batch b WHERE e.id=b.id RETURNING e.id
)
SELECT count(*),COALESCE((SELECT max(id::text) FROM batch),'') FROM deleted;
SQL
 )
 IFS='|' read -r n cursor <<< "$result"
 [[ "$n" =~ ^[0-9]+$ ]]
 [ "$n" -ne 0 ] || break
 total=$((total+n)); [ "$total" -le "$expected" ]
 printf '%s|%s|%s\n' "$total" "$cursor" "$(date -u +%FT%TZ)" > purge-progress.tmp
 mv purge-progress.tmp purge-progress.txt
 if [ "$((total%10000))" -eq 0 ]; then echo "Deleted $total of $expected old technical snapshots"; fi
 sleep .1
done
[ "$total" -eq "$expected" ]
[ "$("${psql[@]}" -c "SELECT count(*) FROM public.core_events WHERE $qualified;")" -eq 0 ]
"${psql[@]}" -c "SELECT 'protected_events',count(*) FROM public.core_events; SELECT 'payments',count(*) FROM public.payments; SELECT 'payment_orders',count(*) FROM public.payment_orders; SELECT 'grants',count(*) FROM public.game_access_grants; SELECT 'ledger',count(*) FROM public.player_time_ledger;" > purge-after-counts.txt
printf '%s rows deleted at %s\n' "$total" "$(date -u +%FT%TZ)" > PURGE_COMPLETED
