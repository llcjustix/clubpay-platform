set -euo pipefail
worker=cpb-clubpay-online-vacuum-20261024-r1
if docker container inspect "$worker" >/dev/null 2>&1; then echo 'Online vacuum worker already exists; refusing duplicate'; exit 1; fi
docker create --name "$worker" --network "container:${api_containers[0]}" --memory 128m --cpus .25 \
 -e PGHOST -e PGPORT -e PGDATABASE -e PGUSER -e PGPASSWORD -e PGSSLMODE=disable -e PGCONNECT_TIMEOUT=5 \
 --mount type=bind,src=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1,dst=/backup \
 postgres:18 bash /worker.sh >/dev/null
docker cp scripts/diagnostics/vacuum-worker.sh "$worker":/worker.sh
docker start "$worker" >/dev/null
echo 'Standard online VACUUM launched for core_events only. No service stop and no VACUUM FULL.'
