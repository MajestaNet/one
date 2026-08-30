#!/usr/bin/env bash
# Fail if Control IDE build/dist artifacts contain source maps, env files, or
# vendor-plane docs that must not ship in desktop installers.
#
# Usage (from repo root or tools/control-ide):
#   bash ./scripts/assert-ide-artifacts.sh [control-ide-root]
# Default root: tools/control-ide relative to repo root.
set -euo pipefail

fail() {
  echo "ide-artifacts: ERROR: $*" >&2
  exit 1
}

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IDE_ROOT="${1:-$ROOT/tools/control-ide}"
[[ -d "$IDE_ROOT" ]] || fail "Control IDE root not found: $IDE_ROOT"

echo "ide-artifacts: auditing ${IDE_ROOT}…"

# Directories that may hold packaged output.
scan_dirs=()
for d in dist dist-electron release; do
  if [[ -d "$IDE_ROOT/$d" ]]; then
    scan_dirs+=("$IDE_ROOT/$d")
  fi
done

if [[ ${#scan_dirs[@]} -eq 0 ]]; then
  fail "no dist/, dist-electron/, or release/ under $IDE_ROOT — run build/dist first"
fi

violations=()
while IFS= read -r -d '' f; do
  rel="${f#"$IDE_ROOT"/}"
  case "$rel" in
    *.map|.env|.env.*|*/.env|*/.env.*)
      violations+=("$rel")
      ;;
  esac
  base="$(basename "$f")"
  case "$base" in
    AGENTS.md|*.cursor*)
      violations+=("$rel")
      ;;
  esac
done < <(find "${scan_dirs[@]}" -type f -print0 2>/dev/null)

# Also reject vendor-plane directory names accidentally copied into artifacts.
while IFS= read -r -d '' d; do
  rel="${d#"$IDE_ROOT"/}"
  base="$(basename "$d")"
  case "$base" in
    docs|backlog|.cursor|AGENTS.md)
      violations+=("$rel/")
      ;;
  esac
done < <(find "${scan_dirs[@]}" -type d -print0 2>/dev/null)

if [[ ${#violations[@]} -gt 0 ]]; then
  printf '%s\n' "${violations[@]}" >&2
  fail "forbidden files in Control IDE artifacts (sourcemaps, env, or vendor docs)"
fi

echo "ide-artifacts: OK (no .map / .env / vendor-plane paths under packaged dirs)"
