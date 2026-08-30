# Majesta CMS (aggregator)

Seed for a **separate** public-docs aggregator: one Git repo, one CMS agent, many Netlify sites (one custom subdomain each). It is **not** a Contentful-style editor, not Path A, and not part of any product image.

**Extract this entire `cms/` directory into a new public GitHub repository** (suggested name: `MajestaNet/cms`). After that copy, delete `cms/` from the Majesta One monorepo. Source product repos must not import, submodule, or CI-depend on this tree.

| Doc | Role |
|---|---|
| [DESIGN.md](./DESIGN.md) | Thesis, roles, pin + overlay, non-goals |
| [BUILD-PLAN.md](./BUILD-PLAN.md) | Phases to scaffold the aggregator |
| [AGENT.md](./AGENT.md) | CMS agent playbook + paste prompts |
| [NETLIFY.md](./NETLIFY.md) | One repo → many sites → many subdomains |
| [SOURCE-CONTRACT.md](./SOURCE-CONTRACT.md) | Optional notify payload (source repos implement later) |
| [sites/README.md](./sites/README.md) | Per-product site layout |
| [sites/one/README.md](./sites/one/README.md) | First site: `one.majesta.net` |

All Majesta source repos are assumed **public and Apache-2.0**. The aggregator reads them over GitHub; it does not need install credentials.

## After you copy this folder

1. Create the empty CMS repo; move these files to its root (drop the extra `cms/` prefix if you want `DESIGN.md` at repo root).
2. Scaffold Astro Starlight **there**, not in product repos.
3. Connect **one Netlify site per product** to that CMS repo ([NETLIFY.md](./NETLIFY.md)).
4. Do **not** add `netlify.toml`, Astro, or a docs-impact workflow to Majesta One (`MajestaNet/ide`) as part of this extraction.

## What this is not

- A runtime service, database, or login portal
- A second source of truth that replaces product `docs/` in each source repo
- Automatic production publish of Majesta One docs from `ide` `main` (One pins `v*`; see [sites/one/README.md](./sites/one/README.md))
- Something product `make ci` or GHCR images build
