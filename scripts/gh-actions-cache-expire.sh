#!/usr/bin/env bash
# Delete GitHub Actions caches last accessed more than MAX_AGE_DAYS ago (default 14).
# Keeps the current go.sum-keyed module cache and other recently used entries.
#
# Usage:
#   GH_REPO=owner/repo ./scripts/gh-actions-cache-expire.sh
#   GH_REPO=owner/repo DRY_RUN=1 ./scripts/gh-actions-cache-expire.sh
#   MAX_AGE_DAYS=7 ./scripts/gh-actions-cache-expire.sh
#   ./scripts/gh-actions-cache-expire.sh --self-test
set -euo pipefail
exec python3 "$(dirname "$0")/gh-actions-cache-expire.py" "$@"
