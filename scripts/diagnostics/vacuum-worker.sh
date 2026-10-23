#!/bin/bash
set -euo pipefail
umask 077
cd /backup
[ -f PURGE_COMPLETED ] && [ -f PROTECTED_FINGERPRINTS_MATCH ]
[ ! -f ONLINE_VACUUM_STARTED ]
export PGAPPNAME=clubpay-postpurge-online-vacuum
export PGOPTIONS='-c statement_timeout=600000 -c lock_timeout=1000 -c vacuum_cost_delay=2ms -c vacuum_cost_limit=1000'
date -u +%FT%TZ > ONLINE_VACUUM_STARTED
trap 'rc=$?; if [ "$rc" -ne 0 ]; then date -u +%FT%TZ > ONLINE_VACUUM_FAILED; fi' EXIT
psql -X --no-password -v ON_ERROR_STOP=1 -c 'VACUUM (VERBOSE, ANALYZE) public.core_events;' > online-vacuum.stdout 2> online-vacuum.stderr
date -u +%FT%TZ > ONLINE_VACUUM_COMPLETED
