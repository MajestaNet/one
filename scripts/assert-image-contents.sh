#!/usr/bin/env bash
# Fail if a Majesta One product image contains vendor/agent-plane paths or unexpected
# top-level entries beyond the distroless runtime + /one + /migrations.
#
# Usage:
#   bash ./scripts/assert-image-contents.sh <image-ref>
# Example:
#   bash ./scripts/assert-image-contents.sh one-api:ci
set -euo pipefail

fail() {
  echo "image-contents: ERROR: $*" >&2
  exit 1
}

IMAGE="${1:-}"
[[ -n "$IMAGE" ]] || fail "usage: $0 <image-ref>"

command -v docker >/dev/null 2>&1 || fail "docker is required"

echo "image-contents: auditing ${IMAGE}…"

cid="$(docker create "$IMAGE" 2>/dev/null)" || fail "docker create failed for ${IMAGE}"
cleanup() { docker rm -f "$cid" >/dev/null 2>&1 || true; }
trap cleanup EXIT

# Distroless has no shell; export filesystem listing via tar. Use a read loop
# instead of mapfile so this CI helper also works with macOS' Bash 3.2.
entries=()
while IFS= read -r entry; do
  entries[${#entries[@]}]="$entry"
done < <(docker export "$cid" | tar -t)

[[ ${#entries[@]} -gt 0 ]] || fail "empty filesystem listing for ${IMAGE}"

has_one=0
has_migrations=0
violations=()

for entry in "${entries[@]}"; do
  # Normalize: strip trailing slash and optional ./ prefix from tar listings.
  path="${entry%/}"
  path="${path#./}"
  case "$path" in
    one|/one) has_one=1 ;;
    migrations|/migrations|migrations/*|/migrations/*) has_migrations=1 ;;
  esac

  # Forbidden vendor/agent plane or secrets leaked into the image.
  case "$path" in
    docs|docs/*|/docs|/docs/*|\
    backlog|backlog/*|/backlog|/backlog/*|\
    .cursor|.cursor/*|/.cursor|/.cursor/*|\
    tools|tools/*|/tools|/tools/*|\
    scripts|scripts/*|/scripts|/scripts/*|\
    AGENTS.md|/AGENTS.md|*/AGENTS.md|\
    .env|.env.*|*/.env|*/.env.*)
      violations+=("$path")
      ;;
  esac
  # Any markdown at image root or under product paths is vendor docs leakage.
  if [[ "$path" == *.md ]]; then
    violations+=("$path")
  fi
done

if [[ "$has_one" -ne 1 ]]; then
  fail "missing /one binary in ${IMAGE}"
fi
if [[ "$has_migrations" -ne 1 ]]; then
  fail "missing /migrations kernel SQL in ${IMAGE}"
fi

if [[ ${#violations[@]} -gt 0 ]]; then
  printf '%s\n' "${violations[@]}" >&2
  fail "forbidden paths present in ${IMAGE} (vendor/agent plane or secrets)"
fi

echo "image-contents: OK (${IMAGE}: /one + /migrations present; vendor plane absent)"
