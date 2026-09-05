# BP-067: Public docs site (`one.majesta.net`)

- **Severity:** Medium
- **Status:** Open (publisher extracted; **this repo** now has customer-cut `docs/api/*.md` + `docs/objects.md` for the CMS map — production ingest still waits on a `v*` pin in the CMS)
- **Area:** GitHub `docs/` only (no publisher code in this monorepo)
- **Pointer:** [public-docs-site.md](../docs/architecture/public-docs-site.md)
- **Related:** [ADR-005](../docs/adr/005-go-runtime.md) · [ADR-025](../docs/adr/025-api-revision-versioning.md) · [ADR-030](../docs/adr/030-install-agent-runtime.md) · [release-cicd.md](../docs/release-cicd.md) · [BP-026](./BP-026-oss-security-public-backlog.md)

## Problem

Majesta One is API-first with **no embedded UI**. Customer-facing docs live in this monorepo mixed with agent playbooks. Operators need a public host; this **product** repo must not grow Astro, Netlify, or a docs-deploy agent.

## Direction (locked)

A **separate Majesta CMS aggregator** (its own public Git repo) publishes `one.majesta.net` and later other Majesta subdomains. This monorepo stays source markdown on GitHub. **No CI, submodule, secret, or path dependency** on the CMS.

| This repo | CMS repo |
|---|---|
| Operator/contributor markdown under `docs/` | Starlight, overlay, pin, Netlify sites |
| No `tools/one-docs`, no `netlify.toml`, no `docs-impact` workflow | CMS agent + Deploy Previews + production after human merge |
| Product `v*` publishes GHCR, not the docs site | One production pin is `v*` (not `ide` `main`) |

Do not implement or import the CMS from product CI.

## Explicit non-goals (this repo)

- Serving docs from `one-api` or a customer install
- Dumping `backlog/` or live vulnerability detail onto the subdomain ([BP-026](./BP-026-oss-security-public-backlog.md))
- Making Control IDE the docs host
- Folding Astro into product `make ci`
- Granting this repo `NETLIFY_*` secrets
- Auto-publishing production from this repo’s `main`

## This repo remainder (source, not publisher)

Customer markdown the CMS should overlay after a notify / `cms-update` payload (map in [public-docs-site.md](../docs/architecture/public-docs-site.md)):

| Source | Public intent |
|---|---|
| [`docs/api-families.md`](../docs/api-families.md) | Overview only — do not paste onto five family routes |
| [`docs/api/{client,metadata,deploy,ops,auth}.md`](../docs/api/) | Family HTTP (path, method, scope, does / does not) |
| [`docs/modules/*.md`](../docs/modules/README.md) | Already customer voice; `include: true` |
| [`docs/objects.md`](../docs/objects.md) | Mapped `/objects` (or similar). **Not** `GET /describe`. [`data-model.md`](../docs/data-model.md) stays contributor |

Do **not** generate OpenAPI in this repo. Do **not** default production pin to `main`.

## Implementation

Do **not** paste an in-tree publisher prompt. Scaffold and host the aggregator in the CMS repo. Overlay work happens there after this repo’s family pages exist.
