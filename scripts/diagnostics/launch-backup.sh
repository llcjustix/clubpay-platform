set -euo pipefail
backup_root=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1
worker=cpb-clubpay-db-backup-20261024-r1
if docker container inspect "$worker" >/dev/null 2>&1; then echo 'Backup worker already exists; not starting a duplicate.'; exit 1; fi
# Make only the dedicated backup directory on the known DB volume.
docker run --rm --network none --mount type=bind,src=/mnt/HC_Volume_106788447/backups,dst=/backups postgres:18 bash -c 'umask 077; mkdir -p /backups/clubpay/precleanup-20261024-r1; test "$(df -Pk /backups | awk "NR==2 {print \$4}")" -gt 12582912'
docker create --name "$worker" --network "container:${api_containers[0]}" --memory 512m --cpus 1 \
  -e PGHOST -e PGPORT -e PGDATABASE -e PGUSER -e PGPASSWORD -e PGSSLMODE=disable -e PGCONNECT_TIMEOUT=5 \
  --mount "type=bind,src=$backup_root,dst=/backup" postgres:18 bash /worker.sh >/dev/null
docker cp scripts/diagnostics/backup-worker.sh "$worker":/worker.sh
docker start "$worker" >/dev/null
echo 'Fresh full ClubPay backup started; no production rows deleted.'
