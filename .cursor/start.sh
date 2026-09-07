#!/usr/bin/env bash
# Per-boot reconciliation for Majesta One: bring up the Postgres cluster and
# wait until it accepts connections. Idempotent and safe to re-run.
#
# The environment build snapshots the VM with Postgres running (install applies
# migrations), so a fresh pod inherits a stale postmaster.pid with no live
# postmaster. Clear that stale pid before starting so boot is deterministic.
set -euo pipefail

PGDATA="/var/lib/postgresql/16/main"
PIDFILE="$PGDATA/postmaster.pid"

# Already accepting connections? Nothing to do.
if sudo -u postgres pg_isready -q 2>/dev/null; then
  echo "postgres already ready"
  exit 0
fi

# Not ready but a pid file exists => it is stale (no live listener). Remove it
# so pg_ctlcluster starts cleanly instead of assuming the cluster is running.
if [ -f "$PIDFILE" ]; then
  sudo rm -f "$PIDFILE" || true
fi

sudo pg_ctlcluster 16 main start 2>/dev/null \
  || sudo pg_ctlcluster 16 main restart 2>/dev/null \
  || true

for _ in $(seq 1 30); do
  if sudo -u postgres pg_isready -q; then
    echo "postgres ready"
    exit 0
  fi
  sleep 1
done

echo "postgres did not become ready" >&2
exit 1
