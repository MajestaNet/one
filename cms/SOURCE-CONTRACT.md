# Source notify contract (optional, later)

Source product repos **do not implement this in the extraction PR**. When a product wants automatic overlay updates, it adds a tiny workflow (or an org GitHub App) that POSTs to the CMS repo. The CMS agent consumes the payload. No Netlify secrets belong in the source repo.

## Payload (stdout / `client_payload`)

```json
{
  "source": "MajestaNet/ide",
  "kind": "merge",
  "ref": "refs/heads/main",
  "sha": "<commit sha>",
  "base": "<merge-base sha or empty>",
  "merged_pr": 123,
  "skip": false,
  "skip_reason": null,
  "paths": [
    "internal/httpapi/client_extras.go",
    "docs/api-families.md"
  ]
}
```

| Field | Meaning |
|---|---|
| `kind` | `merge` (default branch) or `tag` (`v*`) |
| `skip` | CMS agent should no-op |
| `skip_reason` | `docs-only` \| `no-mapped-paths` \| `duplicate` \| `cms-update-label` |
| `paths` | Changed files in the source repo (forward slashes) |

`skip_reason` values are hints. The CMS repo’s content map decides which public pages those paths belong to.

## Suggested source wiring (do not add to One in this extract)

- `on.push` to default branch and `v*` tags, path-filtered to routes / allowlisted markdown / install packaging.
- `repository_dispatch` (or `workflow_dispatch` on the CMS repo) with the JSON above.
- Permissions on the source workflow: `contents: read`. A GitHub App or `CMS_DISPATCH_TOKEN` with **dispatch + contents: write on the CMS repo only** — never `NETLIFY_*`.

Until that exists, a human pastes a payload (or a compare URL) into the CMS agent prompt.

## What stays in the source repo forever

- Customer-facing markdown operators read on GitHub (`docs/self-host.md`, family APIs, …).
- Contributor / agent playbooks (not published).
- **No** Astro, **no** root `netlify.toml`, **no** `tools/one-docs`, **no** product `make docs` that builds this aggregator.

A path → page table for One lives in [sites/one/README.md](./sites/one/README.md). The source repo may later duplicate a thin `docs-impact` script; it is not required to extract the CMS.
