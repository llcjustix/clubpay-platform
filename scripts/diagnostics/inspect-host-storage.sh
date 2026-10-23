set -euo pipefail
echo 'DATABASE_VOLUME_CAPACITY'
docker exec postgres df -h /var/lib/postgresql
echo 'DATABASE_VOLUME_USAGE'
docker exec postgres sh -c 'du -sh /var/lib/postgresql/18/docker/pg_wal 2>/dev/null || true'
echo 'EXISTING_BACKUP_CANDIDATES'
docker run --rm --network none --memory 128m --cpus .25 --mount type=bind,src=/mnt/HC_Volume_106788447,dst=/storage,readonly postgres:18 sh -c 'df -Pk /storage; ls -la /storage; find /storage -maxdepth 3 -type f \( -iname "*clubpay*.dump*" -o -iname "*clubpay*.sql*" -o -iname "*clubpay*.backup*" -o -iname "*clubpay*.tar*" \) -printf "%p %s %TY-%Tm-%TdT%TH:%TM:%TS\n"'

echo 'BACKUP_DIRECTORY_METADATA_ONLY'
docker run --rm --network none --memory 128m --cpus .25 --mount type=bind,src=/mnt/HC_Volume_106788447/backups,dst=/backups,readonly postgres:18 sh -c 'find /backups -maxdepth 5 -printf "%y %p %s %TY-%Tm-%TdT%TH:%TM:%TS\n" | head -150; du -sh /backups'
