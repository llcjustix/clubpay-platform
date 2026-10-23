set -euo pipefail
backup_root=/mnt/HC_Volume_106788447/backups/clubpay/precleanup-20261024-r1
docker run --rm --network none --memory 128m --cpus .25 --mount "type=bind,src=$backup_root,dst=/backup,readonly" postgres:18 bash -c 'cd /backup; test -f COMPACTION_COMPLETED; cmp protected-fingerprints-before.txt protected-fingerprints-after.txt; cmp protected-fingerprints-before.txt protected-fingerprints-after-compaction.txt; sha256sum -c clubpay-full.dump.sha256; cat COMPACTION_STARTED COMPACTION_COMPLETED compact-before-bytes.txt compact-after-bytes-and-counts.txt; df -Pk /backup; echo FINAL_PROTECTED_DATA_MATCH'
docker exec "${api_containers[0]}" wget -q -O /dev/null http://127.0.0.1:8080/api/health && echo API_HEALTH_OK
