set -euo pipefail
worker=cpb-clubpay-snapshot-purge-20261024-r1
if docker container inspect "$worker" >/dev/null 2>&1; then echo 'Purge worker already exists; refusing duplicate'; exit 1; fi
docker create --name "$worker" --network "container:${api_containers[0]}" --memory 256m --cpus .5 \
 -e PGHOST -e PGPORT -e PGDATABASE -e PGUSER -e PGPASSWORD -e PGSSLMODE=disable -e PGCONNECT_TIMEOUT=5 \
 --mount type=bind,src=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1,dst=/backup \
 postgres:18 bash /worker.sh >/dev/null
docker cp scripts/diagnostics/purge-worker.sh "$worker":/worker.sh
docker cp scripts/diagnostics/protected-fingerprints.sql "$worker":/protected-fingerprints.sql
docker start "$worker" >/dev/null
echo 'Selective old technical snapshot purge launched after verified backup. No service stop or VACUUM FULL.'
