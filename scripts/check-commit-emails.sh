#!/usr/bin/env bash
# Fail when a commit author, committer, or identity trailer uses an address
# outside the allowlist. Cursor agents use cursoragent@cursor.com. Humans
# use *@users.noreply.github.com. GitHub uses noreply@github.com. Dependabot
# authors use *@users.noreply.github.com and sign off as support@github.com.
#
# Usage:
#   ./scripts/check-commit-emails.sh
#   ./scripts/check-commit-emails.sh --self-test
set -euo pipefail

allow() {
  case "$1" in
    cursoragent@cursor.com|noreply@github.com|support@github.com|*@users.noreply.github.com) return 0 ;;
    *) return 1 ;;
  esac
}

if [ "${1:-}" = "--self-test" ]; then
  allow "cursoragent@cursor.com"
  allow "noreply@github.com"
  allow "maintainer@users.noreply.github.com"
  allow "49699333+dependabot[bot]@users.noreply.github.com"
  allow "support@github.com"
  if allow "person@example.test"; then
    echo "self-test: personal address was allowed" >&2
    exit 1
  fi
  echo "ok"
  exit 0
fi

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

git log --all --format='%H%x1f%ae%x1f%ce%x1f%b%x1e' | python3 -c '
import sys
data = sys.stdin.read().split("\x1e")
for rec in data:
    if not rec.strip():
        continue
    parts = rec.split("\x1f", 3)
    if len(parts) < 3:
        continue
    sha, author, committer = parts[0].strip(), parts[1].strip(), parts[2].strip()
    body = parts[3] if len(parts) > 3 else ""
    sys.stdout.write(f"{sha}\tauthor\t{author}\n")
    sys.stdout.write(f"{sha}\tcommitter\t{committer}\n")
    for line in body.splitlines():
        lowered = line.lower()
        if lowered.startswith(("co-authored-by:", "signed-off-by:", "reviewed-by:")) and "<" in line and line.endswith(">"):
            email = line[line.rfind("<") + 1:-1].strip()
            sys.stdout.write(f"{sha}\ttrailer\t{email}\n")
' > "$tmp"

bad=0
while IFS=$'\t' read -r sha kind email; do
  if [ -z "${email:-}" ]; then
    continue
  fi
  if ! allow "$email"; then
    echo "disallowed ${kind} email on ${sha}: ${email}" >&2
    bad=1
  fi
done < "$tmp"

exit "$bad"
