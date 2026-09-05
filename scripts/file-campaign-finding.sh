#!/usr/bin/env bash
# File a confirmed customer-rollout finding as a GitHub issue (not a backlog BP).
# Then paste the printed URL into docs/customer-rollout-gap-log.md Issue registry.
#
#   scripts/file-campaign-finding.sh G-MIGRATE-RACE "short title" ./body.md [labels]
#
# Body should follow .github/ISSUE_TEMPLATE/campaign-finding.md
# Fourth arg is comma-separated labels (default: bug).
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "usage: $0 G-ID title body.md [labels]" >&2
  exit 2
fi

gid=$1
title=$2
body=$3

command -v gh >/dev/null 2>&1 || { echo "gh CLI required" >&2; exit 1; }
[[ -f "$body" ]] || { echo "missing body file: $body" >&2; exit 1; }

# Extra labels: comma-separated 4th arg (default bug). Adds campaign when that label exists.
labels=${4:-bug}
if gh label list --json name -q '.[]|.name' 2>/dev/null | grep -qx campaign; then
  labels="campaign,${labels}"
fi

url="$(gh issue create \
  --title "[campaign ${gid}] ${title}" \
  --label "$labels" \
  --body-file "$body")"

echo "filed ${url}"
echo "Next: add ${url} to docs/customer-rollout-gap-log.md Issue registry (beat ${gid})."
echo "Fix PR: cite Fixes #<n> and do not open a new backlog/BP-*.md item."
