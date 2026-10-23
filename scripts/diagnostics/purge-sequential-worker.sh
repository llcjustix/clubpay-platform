#!/bin/bash
set -euo pipefail
umask 077
cd /backup
[ -f BACKUP_READABLE ] && [ -f PROTECTED_DATA_RESTORE_VERIFIED ]
[ ! -f PURGE_SEQUENTIAL_STARTED ]
sha256sum -c clubpay-full.dump.sha256 > purge-backup-check.txt
export PGAPPNAME=clubpay-authorized-old-snapshot-purge
export PGCONNECT_TIMEOUT=5
export PGOPTIONS='-c statement_timeout=15000 -c lock_timeout=1000'
psql=(psql -X --no-password -v ON_ERROR_STOP=1 -At)
qualified="event_type='edge_edge_snapshot' AND status='processed' AND club_id IS NOT NULL AND grant_id IS NULL AND NULLIF(core_session_id,'') IS NULL AND NULLIF(external_pc_id,'') IS NULL AND created_at<timestamptz '2026-10-10T13:25:16Z'"
check=$("${psql[@]}" -c "SELECT current_database()='clubpay' AND NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.core_events'::regclass AND NOT tgisinternal) AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE contype='f' AND confrelid='public.core_events'::regclass) AND NOT EXISTS(SELECT 1 FROM public.core_events WHERE event_type='edge_edge_snapshot' AND created_at>=timestamptz '2026-10-10T13:25:16Z');")
[ "$check" = t ]
expected=$("${psql[@]}" -c "SELECT count(*) FROM public.core_events WHERE $qualified;")
[ "$expected" -gt 0 ] && [ "$expected" -le 1932639 ]
removed_before=$((1932639-expected))
blocks=$("${psql[@]}" -c "SELECT (pg_relation_size('public.core_events')+current_setting('block_size')::bigint-1)/current_setting('block_size')::bigint;")
"${psql[@]}" -f /protected-fingerprints.sql > protected-fingerprints-sequential-start.txt
cmp -s protected-fingerprints-before.txt protected-fingerprints-sequential-start.txt
date -u +%FT%TZ > PURGE_SEQUENTIAL_STARTED
trap 'rc=$?; if [ "$rc" -ne 0 ]; then date -u +%FT%TZ > PURGE_SEQUENTIAL_FAILED; fi' EXIT
total=0; block=0; started=$SECONDS
while [ "$block" -lt "$blocks" ]; do
 [ "$(df -Pk /backup | awk 'NR==2 {print $4}')" -gt 8388608 ]
 [ "$((SECONDS-started))" -lt 1800 ]
 n=$("${psql[@]}" -v "start_tid=($block,0)" -v "end_tid=($((block+128)),0)" -f /sequential-batch.sql)
 [[ "$n" =~ ^[0-9]+$ ]]
 if [ "$n" -eq 0 ]; then block=$((block+128)); continue; fi
 total=$((total+n)); [ "$total" -le "$expected" ]
 printf '%s|%s|%s|%s\n' "$((removed_before+total))" "$block" "$blocks" "$(date -u +%FT%TZ)" > purge-sequential-progress.tmp
 mv purge-sequential-progress.tmp purge-sequential-progress.txt
 echo "Total deleted $((removed_before+total)) of 1932639; current block $block of $blocks"
 sleep .02
done
[ "$total" -eq "$expected" ]
[ "$("${psql[@]}" -c "SELECT count(*) FROM public.core_events WHERE $qualified;")" -eq 0 ]
"${psql[@]}" -c "SELECT 'protected_events',count(*) FROM public.core_events; SELECT 'payments',count(*) FROM public.payments; SELECT 'payment_orders',count(*) FROM public.payment_orders; SELECT 'grants',count(*) FROM public.game_access_grants; SELECT 'ledger',count(*) FROM public.player_time_ledger;" > purge-after-counts.txt
"${psql[@]}" -f /protected-fingerprints.sql > protected-fingerprints-after.txt
if cmp -s protected-fingerprints-before.txt protected-fingerprints-after.txt; then
 date -u +%FT%TZ > PROTECTED_FINGERPRINTS_MATCH
else
 echo 'Protected fingerprints differ; inspect legitimate concurrent writes before any compaction.' > PROTECTED_FINGERPRINTS_DIFFER
 exit 1
fi
printf '%s rows deleted at %s\n' "$((removed_before+total))" "$(date -u +%FT%TZ)" > PURGE_COMPLETED
