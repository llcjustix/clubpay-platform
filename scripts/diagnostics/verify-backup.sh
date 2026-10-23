set -euo pipefail
backup_root=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1
restore=cpb-clubpay-backup-restore-20261024-r1
[ "$(docker inspect cpb-clubpay-db-backup-20261024-r1 --format '{{.State.Status}}:{{.State.ExitCode}}')" = 'exited:0' ]
docker run --rm --network none --mount "type=bind,src=$backup_root,dst=/backup,readonly" postgres:18 bash -c 'cd /backup; test -f BACKUP_READABLE; sha256sum -c clubpay-full.dump.sha256'
if docker container inspect "$restore" >/dev/null 2>&1; then echo 'Restore verifier already exists; refusing duplicate'; exit 1; fi
docker run -d --name "$restore" --network none --memory 512m --cpus .5 \
  --tmpfs /var/lib/postgresql:rw,size=268435456 \
  --mount "type=bind,src=$backup_root,dst=/backup,readonly" \
  -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=clubpay_restore_check postgres:18 >/dev/null
trap 'docker rm -f "$restore" >/dev/null' EXIT
for attempt in $(seq 1 60); do
 if docker exec -u postgres "$restore" pg_isready -d clubpay_restore_check >/dev/null 2>&1; then break; fi
 sleep 1
done
docker exec -u postgres "$restore" pg_isready -d clubpay_restore_check >/dev/null
# Root can read the 0600 archive; -U postgres uses the isolated local socket only.
docker exec "$restore" bash -c "grep -v ' TABLE DATA public core_events ' /backup/archive.list > /tmp/restore.list; pg_restore --exit-on-error --no-owner --no-privileges -U postgres -d clubpay_restore_check --use-list=/tmp/restore.list /backup/clubpay-full.dump" > "$RUNNER_TEMP/restore-check.txt" 2>&1
docker exec -i -u postgres "$restore" psql -X -v ON_ERROR_STOP=1 -d clubpay_restore_check >> "$RUNNER_TEMP/restore-check.txt" <<'SQL'
BEGIN READ ONLY;
SELECT current_database(), pg_database_size(current_database());
SELECT format('SELECT %L AS table_name, count(*) AS rows FROM %I.%I;', tablename,schemaname,tablename)
FROM pg_tables WHERE schemaname='public' AND tablename<>'core_events' ORDER BY tablename \gexec
COMMIT;
SQL
cat "$RUNNER_TEMP/restore-check.txt"
# A persisted receipt is created only after successful archive verification and isolated restore.
docker run --rm --network none --mount "type=bind,src=$backup_root,dst=/backup" postgres:18 bash -c 'umask 077; date -u +%FT%TZ > PROTECTED_DATA_RESTORE_VERIFIED'
echo 'BACKUP_VERIFIED: full archive readable; all non-core_events data restored with schema/constraints in isolated temporary PostgreSQL.'
