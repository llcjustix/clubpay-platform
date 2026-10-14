#!/usr/bin/env bash
# Operator-only maintenance AFTER the snapshot fix is deployed and verified.
# Use an existing libpq connection; credentials must never be passed as args.
# Default is a read-only count. --execute deletes only old processed snapshots
# in bounded transactions. No VACUUM FULL, service stop or backup is automatic.
set -euo pipefail

snapshot_cutoff=""
snapshot_execute=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --before) snapshot_cutoff="${2:-}"; shift 2 ;;
    --execute) snapshot_execute=true; shift ;;
    *) echo 'Usage: purge-edge-snapshots.sh --before UTC_DEPLOY_TIMESTAMP [--execute]' >&2; exit 2 ;;
  esac
done
if [[ ! "$snapshot_cutoff" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$ ]]; then
  echo 'A fixed UTC cutoff is required, for example 2026-10-10T13:00:00Z' >&2
  exit 2
fi
if [ "${PGDATABASE:-}" != clubpay ]; then
  echo 'Set PGDATABASE=clubpay in your existing secure database connection' >&2
  exit 2
fi
export PGAPPNAME=clubpay-snapshot-maintenance
export PGCONNECT_TIMEOUT=5
export PGOPTIONS="${PGOPTIONS:-} -c statement_timeout=15000 -c lock_timeout=1000"
snapshot_psql=(psql -X --no-password -v ON_ERROR_STOP=1 -v "cutoff=$snapshot_cutoff" -At)
snapshot_authorized="$("${snapshot_psql[@]}" <<'SQL'
BEGIN READ ONLY;
SELECT current_database()='clubpay' AND :'cutoff'::timestamptz < now();
COMMIT;
SQL
)"
if ! printf '%s\n' "$snapshot_authorized" | grep -qx t; then
  echo 'Wrong database or future cutoff; no rows were deleted' >&2
  exit 2
fi

# Guard against a snapshot label accidentally being applied to a session row.
# Real events, failed records, grants and all financial tables are untouched.
snapshot_remaining() {
  "${snapshot_psql[@]}" <<'SQL'
BEGIN READ ONLY;
SELECT count(*) FROM public.core_events
WHERE event_type='edge_edge_snapshot' AND status='processed'
  AND club_id IS NOT NULL AND grant_id IS NULL
  AND NULLIF(core_session_id,'') IS NULL
  AND NULLIF(external_pc_id,'') IS NULL
  AND created_at < :'cutoff'::timestamptz;
COMMIT;
SQL
}
if [ "$snapshot_execute" = false ]; then
  snapshot_count="$(snapshot_remaining | sed -n '/^[0-9][0-9]*$/p')"
  echo "Dry run: $snapshot_count eligible technical snapshots before $snapshot_cutoff. Nothing deleted."
  exit 0
fi

snapshot_after=""
snapshot_total=0
while :; do
  snapshot_result="$("${snapshot_psql[@]}" -v "after=$snapshot_after" <<'SQL'
WITH batch AS MATERIALIZED (
  SELECT id FROM public.core_events
  WHERE event_type='edge_edge_snapshot' AND status='processed'
    AND club_id IS NOT NULL AND grant_id IS NULL
    AND NULLIF(core_session_id,'') IS NULL
    AND NULLIF(external_pc_id,'') IS NULL
    AND created_at < :'cutoff'::timestamptz
    AND (NULLIF(:'after','') IS NULL OR id > NULLIF(:'after','')::uuid)
  ORDER BY id LIMIT 1000 FOR UPDATE
), deleted AS (
  DELETE FROM public.core_events e USING batch b WHERE e.id=b.id RETURNING e.id
)
SELECT count(*), COALESCE((SELECT max(id::text) FROM batch),'') FROM deleted;
SQL
)"
  IFS='|' read -r snapshot_deleted snapshot_after <<< "$snapshot_result"
  if [[ ! "$snapshot_deleted" =~ ^[0-9]+$ ]]; then
    echo 'Unexpected maintenance result; stopped without retrying the batch' >&2
    exit 1
  fi
  if [ "$snapshot_deleted" -eq 0 ]; then break; fi
  snapshot_total=$((snapshot_total + snapshot_deleted))
  echo "Deleted technical snapshots: $snapshot_total"
done
snapshot_count="$(snapshot_remaining | sed -n '/^[0-9][0-9]*$/p')"
if [ "$snapshot_count" != 0 ]; then
  echo "Stopped with $snapshot_count eligible rows remaining; inspect concurrent writers before retrying" >&2
  exit 1
fi
echo "Deleted $snapshot_total eligible technical snapshots. Financial and session tables were not modified."
echo 'Physical disk shrink is separate: schedule VACUUM (FULL, ANALYZE) public.core_events; during maintenance.'
