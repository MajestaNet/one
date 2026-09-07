#!/usr/bin/env bash
# Idempotent Cloud Agent install for Majesta One.
# Prepares Postgres 16, Go module cache, a local .env, and applies kernel
# migrations. Safe to re-run: every step is guarded.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_DIR"

export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"

echo "==> Ensuring Postgres 16 is installed"
if ! command -v pg_ctlcluster >/dev/null 2>&1; then
  sudo apt-get update -qq
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq postgresql postgresql-contrib
fi

echo "==> Starting Postgres cluster"
sudo pg_ctlcluster 16 main start 2>/dev/null || true
for _ in $(seq 1 30); do
  if sudo -u postgres pg_isready -q; then break; fi
  sleep 1
done

echo "==> Provisioning role and database (one/one/one)"
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='one'" | grep -q 1; then
  sudo -u postgres psql -c "CREATE ROLE one LOGIN PASSWORD 'one' SUPERUSER"
fi
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='one'" | grep -q 1; then
  sudo -u postgres createdb -O one one
fi

echo "==> Ensuring local .env"
[ -f .env ] || cp .env.example .env

echo "==> Downloading Go modules"
go mod download

echo "==> Applying kernel migrations"
set -a; . ./.env; set +a
go run ./cmd/migrate

echo "==> Install complete"
