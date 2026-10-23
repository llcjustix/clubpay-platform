set -euo pipefail
worker=cpb-clubpay-snapshot-purge-20261024-r1
[ "$(docker inspect "$worker" --format '{{.State.Status}}:{{.State.ExitCode}}')" = 'exited:1' ]
docker run --rm --network none --mount type=bind,src=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1,dst=/backup,readonly postgres:18 bash -c 'test -f /backup/PROTECTED_DATA_RESTORE_VERIFIED; test ! -e /backup/PURGE_STARTED'
docker start "$worker" >/dev/null
echo 'Existing worker restarted only after verification receipt, with no previous purge started.'
