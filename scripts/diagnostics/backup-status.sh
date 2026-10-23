set -euo pipefail
docker inspect cpb-clubpay-db-backup-20261024-r1 --format 'backup_state={{.State.Status}} exit={{.State.ExitCode}} oom={{.State.OOMKilled}} started={{.State.StartedAt}} finished={{.State.FinishedAt}}'
docker run --rm --network none --memory 128m --cpus .25 --mount type=bind,src=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1,dst=/backup,readonly postgres:18 bash -c 'ls -ln /backup; df -Pk /backup; for f in started-at.txt completed-at.txt FAILED backup.stderr archive-readcheck.stderr clubpay-full.dump.sha256; do if [ -f /backup/$f ]; then echo "$f"; tail -c 2000 "/backup/$f"; fi; done'
docker exec "${api_containers[0]}" wget -q -O /dev/null http://127.0.0.1:8080/api/health && echo 'API_HEALTH_OK'

echo 'BACKUP_RESOURCE_USAGE'
docker stats --no-stream --format '{{.Name}} cpu={{.CPUPerc}} memory={{.MemUsage}} block={{.BlockIO}}' cpb-clubpay-db-backup-20261024-r1 postgres "${api_containers[0]}"

if docker container inspect cpb-clubpay-snapshot-purge-20261024-r1 >/dev/null 2>&1; then
 docker inspect cpb-clubpay-snapshot-purge-20261024-r1 --format 'purge_state={{.State.Status}} exit={{.State.ExitCode}} oom={{.State.OOMKilled}} started={{.State.StartedAt}} finished={{.State.FinishedAt}}'
 docker logs --tail 4 cpb-clubpay-snapshot-purge-20261024-r1
fi
docker run --rm --network none --memory 128m --cpus .25 --mount type=bind,src=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1,dst=/backup,readonly postgres:18 bash -c 'for f in PROTECTED_DATA_RESTORE_VERIFIED PURGE_STARTED purge-progress.txt PURGE_FAILED PURGE_COMPLETED purge-before-counts.txt purge-after-counts.txt protected-fingerprints-before.txt protected-fingerprints-after.txt; do if [ -f /backup/$f ]; then echo "$f"; cat "/backup/$f"; fi; done'
