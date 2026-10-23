set -euo pipefail
worker=cpb-clubpay-core-compaction-20261024-r1
if docker container inspect "$worker" >/dev/null 2>&1; then echo 'Compaction worker already exists; refusing duplicate'; exit 1; fi
# Cancel only the ordinary automatic cleaner of this exact ClubPay TOAST table.
# Anti-wraparound workers and every other database/table are excluded.
docker exec -i -u postgres postgres psql -X --no-password -v ON_ERROR_STOP=1 -U postgres -d clubpay <<'SQL'
SET statement_timeout='5s';
SELECT pg_cancel_backend(v.pid) AS replaced_ordinary_clubpay_toast_vacuum
FROM pg_stat_progress_vacuum v JOIN pg_stat_activity a ON a.pid=v.pid
WHERE v.datname=current_database()
AND v.relid=(SELECT reltoastrelid FROM pg_class WHERE oid='public.core_events'::regclass)
AND a.backend_type='autovacuum worker' AND a.query NOT LIKE '%to prevent wraparound%';
SQL
docker create --name "$worker" --network "container:${api_containers[0]}" --memory 128m --cpus .25 \
 -e PGHOST -e PGPORT -e PGDATABASE -e PGUSER -e PGPASSWORD -e PGSSLMODE=disable -e PGCONNECT_TIMEOUT=5 \
 --mount type=bind,src=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1,dst=/backup \
 postgres:18 bash /worker.sh >/dev/null
docker cp scripts/diagnostics/compact-worker.sh "$worker":/worker.sh
docker cp scripts/diagnostics/protected-fingerprints.sql "$worker":/protected-fingerprints.sql
docker start -a "$worker"
[ "$(docker inspect "$worker" --format '{{.State.Status}}:{{.State.ExitCode}}')" = 'exited:0' ]
docker run --rm --network none --mount type=bind,src=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1,dst=/backup,readonly postgres:18 bash -c 'for f in COMPACTION_STARTED COMPACTION_COMPLETED compact-before-bytes.txt compact-after-bytes-and-counts.txt compact.stderr; do echo "$f"; cat "/backup/$f"; done; df -Pk /backup'
