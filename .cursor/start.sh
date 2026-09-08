#!/usr/bin/env bash
# Bring up the Postgres 16 cluster and wait until it accepts connections.
# Idempotent and safe to run concurrently (e.g. from both the `start` phase and
# a terminal): it only removes a postmaster.pid whose process is actually dead,
# so it never disturbs a postmaster that is still starting up.
#
# The environment build snapshots the VM with Postgres running (install applies
# migrations), so a fresh pod inherits a stale postmaster.pid with no live
# postmaster. Clearing that stale pid makes boot deterministic.
set -euo pipefail

PGDATA="/var/lib/postgresql/16/main"
PIDFILE="$PGDATA/postmaster.pid"

# Already accepting connections? Nothing to do.
if sudo -u postgres pg_isready -q 2>/dev/null; then
  echo "postgres already ready"
  exit 0
fi

# Remove the pid file only if its recorded process is not alive (truly stale).
# A postmaster that is still starting up has a live pid and is left untouched.
if [ -f "$PIDFILE" ]; then
  pid="$(sudo head -n 1 "$PIDFILE" 2>/dev/null || true)"
  if [ -z "$pid" ] || ! sudo kill -0 "$pid" 2>/dev/null; then
    sudo rm -f "$PIDFILE" || true
  fi
fi

sudo pg_ctlcluster 16 main start 2>/dev/null \
  || sudo pg_ctlcluster 16 main restart 2>/dev/null \
  || true

for _ in $(seq 1 60); do
  if sudo -u postgres pg_isready -q; then
    echo "postgres ready"
    exit 0
  fi
  sleep 1
done

echo "postgres did not become ready" >&2
exit 1
