#!/usr/bin/env bash
# One-shot GitHub Actions storage hygiene for artifact + cache quotas.
# Emergency full wipe — prefer scripts/gh-actions-cache-expire.sh for caches.
#
# Requires: gh CLI authenticated with repo scope (admin:repo for retention API).
# Usage:
#   GH_REPO=owner/repo ./scripts/gh-actions-quota-cleanup.sh            # delete all + set 3-day default
#   GH_REPO=owner/repo ./scripts/gh-actions-quota-cleanup.sh --dry-run  # print counts only
#   ./scripts/gh-actions-quota-cleanup.sh --artifacts-only
#   ./scripts/gh-actions-quota-cleanup.sh --caches-only
#   ./scripts/gh-actions-quota-cleanup.sh --retention-only
#   ./scripts/gh-actions-quota-cleanup.sh --ghcr-prune-untagged
set -euo pipefail

REPO="${GH_REPO:-}"
if [[ -z "$REPO" ]]; then
  REPO="$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || true)"
fi
if [[ -z "$REPO" ]]; then
  echo "Set GH_REPO=owner/repo or run from a git checkout with a GitHub remote." >&2
  exit 1
fi

ORG="${REPO%%/*}"
RETENTION_DAYS="${RETENTION_DAYS:-3}"
DRY_RUN=0
DO_ARTIFACTS=1
DO_CACHES=1
DO_RETENTION=1
DO_GHCR=0

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    --artifacts-only) DO_CACHES=0; DO_RETENTION=0; DO_GHCR=0 ;;
    --caches-only) DO_ARTIFACTS=0; DO_RETENTION=0; DO_GHCR=0 ;;
    --retention-only) DO_ARTIFACTS=0; DO_CACHES=0; DO_GHCR=0 ;;
    --ghcr-prune-untagged) DO_ARTIFACTS=0; DO_CACHES=0; DO_RETENTION=0; DO_GHCR=1 ;;
    -h|--help)
      sed -n '2,13p' "$0"
      exit 0
      ;;
    *)
      echo "Unknown option: $arg" >&2
      exit 2
      ;;
  esac
done

artifact_count() {
  gh api "repos/${REPO}/actions/artifacts" --jq '.total_count'
}

cache_count() {
  gh cache list -R "$REPO" --json id --jq 'length'
}

delete_artifacts() {
  local ids deleted=0
  mapfile -t ids < <(gh api --paginate "repos/${REPO}/actions/artifacts" --jq '.artifacts[].id')
  if ((${#ids[@]} == 0)); then
    echo "No artifacts to delete."
    return
  fi
  echo "Deleting ${#ids[@]} artifact(s) from ${REPO}..."
  for id in "${ids[@]}"; do
    if [[ "$DRY_RUN" -eq 1 ]]; then
      echo "  would delete artifact $id"
    else
      gh api -X DELETE "repos/${REPO}/actions/artifacts/${id}" >/dev/null
      deleted=$((deleted + 1))
      if ((deleted % 25 == 0)); then
        echo "  deleted ${deleted}/${#ids[@]}..."
      fi
    fi
  done
  if [[ "$DRY_RUN" -eq 0 ]]; then
    echo "Deleted ${deleted} artifact(s)."
  fi
}

delete_caches() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    local n
    n="$(cache_count)"
    echo "Would delete ${n} cache entr(ies) via: gh cache delete --all -R ${REPO}"
    return
  fi
  echo "Deleting all Actions caches for ${REPO}..."
  gh cache delete --all -R "$REPO"
  echo "Caches cleared."
}

set_retention() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "Would set artifact+log retention to ${RETENTION_DAYS} day(s) for ${REPO}."
    return
  fi
  echo "Setting artifact and log retention to ${RETENTION_DAYS} day(s)..."
  gh api -X PUT "repos/${REPO}/actions/permissions/artifact-and-log-retention" \
    -F "days=${RETENTION_DAYS}" >/dev/null
  echo "Retention updated."
}

prune_untagged_ghcr() {
  local packages=(one-api one-worker) pkg id deleted=0
  for pkg in "${packages[@]}"; do
    mapfile -t ids < <(
      gh api --paginate "orgs/${ORG}/packages/container/${pkg}/versions" \
        --jq '.[] | select((.metadata.container.tags // []) | length == 0) | .id'
    )
    if ((${#ids[@]} == 0)); then
      echo "No untagged GHCR versions for ${pkg}."
      continue
    fi
    echo "Pruning ${#ids[@]} untagged GHCR version(s) for ${pkg}..."
    for id in "${ids[@]}"; do
      if [[ "$DRY_RUN" -eq 1 ]]; then
        echo "  would delete ${pkg} version ${id}"
      else
        gh api -X DELETE "orgs/${ORG}/packages/container/${pkg}/versions/${id}" >/dev/null
        deleted=$((deleted + 1))
      fi
    done
  done
  if [[ "$DRY_RUN" -eq 0 ]]; then
    echo "Deleted ${deleted} untagged GHCR version(s)."
  fi
}

echo "Repository: ${REPO}"
echo "Artifacts: $(artifact_count) | Caches (sample page): $(cache_count)"
echo

if [[ "$DO_ARTIFACTS" -eq 1 ]]; then
  delete_artifacts
  echo
fi
if [[ "$DO_CACHES" -eq 1 ]]; then
  delete_caches
  echo
fi
if [[ "$DO_RETENTION" -eq 1 ]]; then
  set_retention
fi
if [[ "$DO_GHCR" -eq 1 ]]; then
  prune_untagged_ghcr
fi
