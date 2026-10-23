set -euo pipefail
source scripts/diagnostics/final-check.sh
for worker in cpb-clubpay-db-backup-20261024-r1 cpb-clubpay-snapshot-purge-20261024-r1 cpb-clubpay-snapshot-purge-sequential-20261024-r2 cpb-clubpay-core-compaction-20261024-r1; do
 if docker container inspect "$worker" >/dev/null 2>&1; then
  [ "$(docker inspect "$worker" --format '{{.State.Status}}')" = exited ]
  docker logs "$worker" 2>&1 | docker run --rm -i --network none --mount "type=bind,src=$backup_root,dst=/backup" postgres:18 bash -c 'umask 077; cat > "/backup/$1.log"' _ "$worker"
  docker rm "$worker" >/dev/null
  echo "Removed stopped maintenance container $worker; backup and log retained."
 fi
done
echo MAINTENANCE_WORKERS_CLOSED
