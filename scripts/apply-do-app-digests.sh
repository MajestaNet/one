#!/usr/bin/env bash
# Apply GHCR digests from a release digests file into an App Spec (in place).
#
# Usage: scripts/apply-do-app-digests.sh image-digests-X.Y.Z.txt [app.yaml]
#
# Rewrites api/worker image digest lines whether they are commented placeholders
# or already-pinned sha256 values. Sets image tag and PRODUCT_VERSION from the
# filename semver (image-digests-X.Y.Z.txt) even when the spec is not 0.1.0.
#
# API_REVISION_CURRENT / API_REVISION_MIN are left unchanged. If a future
# image-digests file grows those keys, extend this script then.
set -euo pipefail

DIGESTS_FILE="${1:?usage: $0 image-digests-X.Y.Z.txt [app.yaml]}"
SPEC="${2:-deploy/digitalocean/app.yaml}"

api_digest="$(awk -F= '/^api_digest=/{print $2; exit}' "$DIGESTS_FILE")"
worker_digest="$(awk -F= '/^worker_digest=/{print $2; exit}' "$DIGESTS_FILE")"
version="$(basename "$DIGESTS_FILE" | sed -n 's/^image-digests-\(.*\)\.txt$/\1/p')"

if [[ -z "$api_digest" || -z "$worker_digest" ]]; then
  echo "missing api_digest or worker_digest in $DIGESTS_FILE" >&2
  exit 1
fi

tmp="$(mktemp)"
# shellcheck disable=SC2016
awk -v api="$api_digest" -v worker="$worker_digest" -v ver="$version" '
  BEGIN { in_api=0; in_worker=0; prev_product=0 }
  /^  - name: api$/ { in_api=1; in_worker=0 }
  /^  - name: worker$/ { in_worker=1; in_api=0 }
  /^[a-z]/ { if ($0 !~ /^  /) { in_api=0; in_worker=0 } }
  {
    if (in_api && $0 ~ /^      tag:/) {
      if (ver != "") { print "      tag: \"" ver "\""; next }
    }
    if (in_worker && $0 ~ /^      tag:/) {
      if (ver != "") { print "      tag: \"" ver "\""; next }
    }
    if (in_api && $0 ~ /^[[:space:]]*(#[[:space:]]*)?digest:[[:space:]]/) {
      print "      digest: " api
      next
    }
    if (in_worker && $0 ~ /^[[:space:]]*(#[[:space:]]*)?digest:[[:space:]]/) {
      print "      digest: " worker
      next
    }
    if (prev_product && $0 ~ /^        value:/) {
      if (ver != "") { print "        value: \"" ver "\""; prev_product=0; next }
    }
    prev_product = ($0 ~ /key: PRODUCT_VERSION/)
    print
  }
' "$SPEC" > "$tmp"
mv "$tmp" "$SPEC"
echo "updated $SPEC with api=$api_digest worker=$worker_digest version=${version:-unchanged}"
