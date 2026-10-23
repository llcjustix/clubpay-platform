#!/bin/bash
set -euo pipefail
umask 077
cd /backup
[ ! -e clubpay-full.dump ] && [ ! -e clubpay-full.dump.partial ]
date -u +%FT%TZ > started-at.txt
# Bound this archive to 8 GiB; retain a substantial free-space reserve.
ulimit -f 8388608
export PGAPPNAME=clubpay-precleanup-backup
export PGOPTIONS='-c default_transaction_read_only=on -c statement_timeout=0 -c lock_timeout=2000'
pg_dump --format=custom --compress=zstd:3 --no-owner --no-privileges --lock-wait-timeout=2s --file=clubpay-full.dump.partial > backup.stdout 2> backup.stderr &
dump_pid=$!
while kill -0 "$dump_pid" 2>/dev/null; do
  free_kib=$(df -Pk /backup | awk 'NR==2 {print $4}')
  if [ "$free_kib" -lt 8388608 ]; then
    kill -TERM "$dump_pid" || true
    wait "$dump_pid" || true
    echo 'Stopped: free-space reserve reached; partial backup is not valid.' > FAILED
    exit 1
  fi
  sleep 5
done
if ! wait "$dump_pid"; then echo 'pg_dump failed; no deletion authorized.' > FAILED; exit 1; fi
mv clubpay-full.dump.partial clubpay-full.dump
pg_restore --list clubpay-full.dump > archive.list
# Read every archive data block and validate decompression, without outputting rows.
pg_restore --file=/dev/null clubpay-full.dump 2> archive-readcheck.stderr
sha256sum clubpay-full.dump > clubpay-full.dump.sha256
date -u +%FT%TZ > completed-at.txt
touch BACKUP_READABLE
