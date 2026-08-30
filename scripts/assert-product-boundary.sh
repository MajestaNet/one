#!/usr/bin/env bash
# Fail if customer customization scratch or non-product paths threaten a release.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() {
  echo "product-boundary: ERROR: $*" >&2
  exit 1
}

echo "product-boundary: checking git hygiene…"

if git ls-files --error-unmatch '.customer-sandbox' >/dev/null 2>&1; then
  fail ".customer-sandbox is tracked; customer scratch must remain gitignored"
fi

tracked_sandbox="$(git ls-files '.customer-sandbox/**' 'customer-sandbox/**' 2>/dev/null || true)"
if [[ -n "${tracked_sandbox}" ]]; then
  echo "${tracked_sandbox}" >&2
  fail "tracked files under customer sandbox paths"
fi

tracked_exports="$(git ls-files | grep -E '(^|/)customer-exports(/|$)|\.customer-bundle\.json$' || true)"
if [[ -n "${tracked_exports}" ]]; then
  echo "${tracked_exports}" >&2
  fail "tracked customer export / bundle files"
fi

echo "product-boundary: checking deploy/Dockerfile COPY allowlist…"

DOCKERFILE="deploy/Dockerfile"
[[ -f "$DOCKERFILE" ]] || fail "missing $DOCKERFILE"

# Collect COPY sources from the build stage (ignore distroless stage COPY --from=).
# Use a read loop instead of mapfile so `make boundary` also works with the
# Bash 3.2 shipped on supported macOS developer machines.
copy_lines=()
while IFS= read -r line; do
  copy_lines[${#copy_lines[@]}]="$line"
done < <(grep -E '^COPY ' "$DOCKERFILE" | grep -v 'COPY --from=' || true)

for line in "${copy_lines[@]}"; do
  # parse: COPY src [src...] dest
  # shellcheck disable=SC2206
  parts=( $line )
  if [[ ${#parts[@]} -lt 3 ]]; then
    fail "unparseable COPY line: $line"
  fi
  for ((i=1; i<${#parts[@]}-1; i++)); do
    src="${parts[$i]}"
    case "$src" in
      go.mod|go.sum|cmd|./cmd|internal|./internal|migrations|./migrations)
        ;;
      *)
        fail "Dockerfile COPY source not in product allowlist: $src (line: $line)"
        ;;
    esac
  done
done

[[ ${#copy_lines[@]} -ge 1 ]] || fail "no COPY instructions found in $DOCKERFILE"

echo "product-boundary: checking .dockerignore excludes customer scratch…"
grep -qE '^\.customer-sandbox$' .dockerignore || fail ".dockerignore must exclude .customer-sandbox"
grep -qE '^customer-sandbox$' .dockerignore || fail ".dockerignore must exclude customer-sandbox"

echo "product-boundary: checking .dockerignore excludes vendor/agent plane…"
grep -qE '^docs$' .dockerignore || fail ".dockerignore must exclude docs"
grep -qE '^backlog$' .dockerignore || fail ".dockerignore must exclude backlog"
grep -qE '^\.cursor$' .dockerignore || fail ".dockerignore must exclude .cursor"
grep -qE '^\*\.md$' .dockerignore || fail ".dockerignore must exclude *.md (AGENTS.md and docs)"
grep -qE '^tools$' .dockerignore || fail ".dockerignore must exclude tools (Control IDE / vendor helpers)"
grep -qE '^scripts$' .dockerignore || fail ".dockerignore must exclude scripts (vendor automation)"
grep -qE '^sdk$' .dockerignore || fail ".dockerignore must exclude sdk (community cloud SDKs)"

echo "product-boundary: OK (product paths only; customer scratch untracked; vendor/agent plane excluded)"
