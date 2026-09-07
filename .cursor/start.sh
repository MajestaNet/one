#!/usr/bin/env bash
# Per-boot reconciliation for Majesta One: bring up the Postgres cluster and
# wait until it accepts connections. Idempotent and safe to re-run.
set -euo pipefail

sudo pg_ctlcluster 16 main start 2>/dev/null || true

for _ in $(seq 1 30); do
  if sudo -u postgres pg_isready -q; then
    echo "postgres ready"
    exit 0
  fi
  sleep 1
done

echo "postgres did not become ready" >&2
exit 1
