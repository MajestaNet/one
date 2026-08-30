#!/usr/bin/env python3
"""Expire GitHub Actions caches last accessed more than MAX_AGE_DAYS ago (default 14).

Keeps recently used keys (including the go.sum-keyed module cache restored on PRs).

Usage:
  GH_REPO=owner/repo ./scripts/gh-actions-cache-expire.py
  DRY_RUN=1 ./scripts/gh-actions-cache-expire.py
  MAX_AGE_DAYS=7 ./scripts/gh-actions-cache-expire.py
  ./scripts/gh-actions-cache-expire.py --self-test
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
from datetime import datetime, timedelta, timezone


def parse_cache_pages(raw: str) -> list[dict]:
    """Flatten `gh api --paginate` JSON pages into cache objects."""
    caches: list[dict] = []
    decoder = json.JSONDecoder()
    idx = 0
    s = raw.strip()
    while idx < len(s):
        while idx < len(s) and s[idx].isspace():
            idx += 1
        if idx >= len(s):
            break
        obj, end = decoder.raw_decode(s, idx)
        if isinstance(obj, dict) and "actions_caches" in obj:
            caches.extend(obj.get("actions_caches") or [])
        elif isinstance(obj, list):
            caches.extend(obj)
        else:
            caches.append(obj)
        idx = end
    return caches


def accessed_at(cache: dict) -> datetime | None:
    raw = cache.get("last_accessed_at") or cache.get("created_at") or ""
    if not raw:
        return None
    return datetime.fromisoformat(str(raw).replace("Z", "+00:00"))


def stale_caches(caches: list[dict], cutoff: datetime) -> list[dict]:
    stale = []
    for cache in caches:
        when = accessed_at(cache)
        if when is not None and when < cutoff:
            stale.append(cache)
    return stale


def self_test() -> None:
    page1 = {
        "total_count": 3,
        "actions_caches": [
            {
                "id": 1,
                "key": "go-mod-Linux-abc",
                "last_accessed_at": "2026-08-18T00:00:00Z",
                "size_in_bytes": 100,
            },
            {
                "id": 2,
                "key": "old",
                "last_accessed_at": "2026-01-01T00:00:00Z",
                "size_in_bytes": 200,
            },
        ],
    }
    page2 = {
        "total_count": 3,
        "actions_caches": [
            {
                "id": 3,
                "key": "older",
                "created_at": "2025-12-01T00:00:00Z",
                "size_in_bytes": 50,
            }
        ],
    }
    raw = json.dumps(page1) + "\n" + json.dumps(page2)
    caches = parse_cache_pages(raw)
    assert [c["id"] for c in caches] == [1, 2, 3], caches
    cutoff = datetime(2026, 7, 1, tzinfo=timezone.utc)
    stale = stale_caches(caches, cutoff)
    assert [c["id"] for c in stale] == [2, 3], stale
    print("self-test ok")


def main() -> int:
    if "--self-test" in sys.argv:
        self_test()
        return 0

    repo = os.environ.get("GH_REPO") or os.environ.get("GITHUB_REPOSITORY") or ""
    if not repo:
        try:
            repo = subprocess.check_output(
                ["gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"],
                text=True,
            ).strip()
        except (subprocess.CalledProcessError, FileNotFoundError):
            repo = ""
    if not repo:
        print("Set GH_REPO=owner/repo or GITHUB_REPOSITORY.", file=sys.stderr)
        return 1

    days = int(os.environ.get("MAX_AGE_DAYS", "14"))
    dry_run = os.environ.get("DRY_RUN", "0") == "1"
    cutoff = datetime.now(timezone.utc) - timedelta(days=days)

    raw = subprocess.check_output(
        ["gh", "api", "--paginate", f"repos/{repo}/actions/caches?per_page=100"],
        text=True,
    )
    caches = parse_cache_pages(raw)
    stale = stale_caches(caches, cutoff)

    print(
        f"Repository: {repo}\n"
        f"Caches: {len(caches)} | stale (>{days}d): {len(stale)} | cutoff: {cutoff.isoformat()}"
    )
    for cache in stale:
        cid = cache["id"]
        key = cache.get("key", "")
        accessed = cache.get("last_accessed_at") or cache.get("created_at")
        size = int(cache.get("size_in_bytes") or 0)
        print(f"  {cid}  {size:9d}B  {accessed}  {key}")
        if dry_run:
            continue
        subprocess.check_call(["gh", "cache", "delete", str(cid), "-R", repo])

    if dry_run:
        print("Dry run; nothing deleted.")
    else:
        print(f"Deleted {len(stale)} stale cache(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
