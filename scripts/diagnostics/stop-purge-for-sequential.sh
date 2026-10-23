set -euo pipefail
worker=cpb-clubpay-snapshot-purge-20261024-r1
[ "$(docker inspect "$worker" --format '{{.State.Status}}')" = running ]
docker stop -t 25 "$worker" >/dev/null
echo 'Only the named maintenance worker stopped. Each completed SQL batch was atomic; remaining rows will be recounted.'
