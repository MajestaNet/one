# Design — Majesta CMS aggregator

Public customer docs for Majesta products live on product subdomains (`one.majesta.net`, later others). This repo is the **publisher and overlay**. Each product Git repo remains the **source** of contract markdown (install, API, CLI). GitHub stays the contributor / coding-agent host.

## Thesis

> One public aggregator repo. A CMS agent in *this* repo is notified when a source repo changes. It reviews the public diff and updates that product’s overlay (and, for versioned products, the source pin). A human merges the PR. Netlify auto-deploys **that product’s site**. Agents never hold `NETLIFY_*`. Product images never contain this tree.

```text
source repo (public OSS)
  merge to main and/or v* tag
       │  notify (dispatch: repo, sha, paths)
       ▼
CMS repo
  cms agent → draft PR (overlay ± pin)
       │  human merge
       ▼
Netlify site for that package → product subdomain
```

## Why an aggregator (not in-product publisher)

Putting Starlight, `netlify.toml`, and a docs agent inside an installable product repo mixes vendor hosting with Go CI, image-boundary checks, and agent fences. A company-wide **CMS-as-source-of-truth** would duplicate prose and drift from releases. An aggregator keeps:

- **Write / validate / publish** unmixed
- **Pin + overlay** so public pages are not a second wiki
- **One Netlify vendor** and one agent playbook for every Majesta public host

## Roles (never mixed)

| Role | Owner | Must not |
|---|---|---|
| **Source** | Each product repo’s allowlisted markdown | Host Netlify; run Astro |
| **Write** | CMS agent (this repo) | Merge its PR; `netlify deploy --prod`; edit product Go |
| **Validate** | CMS CI (`docs-check` / Starlight build) | Generate markdown or OpenAPI |
| **Publish** | Netlify GitHub app on **this** repo’s `main` (per site) | Run on a source-repo push; live in product images |
| **Human** | Merge the CMS PR; first GA; DNS; site create | Be an in-browser CMS |

CI never writes markdown. The agent never deploys production.

## Pin + overlay (avoid a second wiki)

If the agent copies full markdown into this repo, two corpora drift. Prefer:

| Layer | Lives | Who changes it |
|---|---|---|
| **Pin** | `sites/<product>/pin` (git SHA or `vX.Y.Z` of the source repo) | Agent on a **release** notify; human on first cut |
| **Source markdown** | The product repo at that pin (fetched at **build time**) | Product contributors / feature agents |
| **Overlay** | `sites/<product>/src` — sidebar, customer-tone wrappers, IA | CMS agent |

The Starlight build **includes** allowlisted files from the pinned checkout. It does not commit those files as the source of truth.

**Per-product version rule** (do not flatten):

| Product | Production pin | Notify on `main` |
|---|---|---|
| Majesta One | Last `v*` tag (same string as GHCR) | Draft overlay PR only; do **not** move `one.majesta.net` |
| Marketing / company sites that already publish from `main` | Source `main` (or aggregator merge) | May auto-publish that site after the CMS PR merges |

`/v/X.Y/` snapshots for One stay **in that site’s build output** (last N=2 or 3 minors), not as Netlify deploy permalinks. Older than N stays on GitHub at the product tag.

## Two corpora (still)

| Corpus | Audience | Where | On the subdomain? |
|---|---|---|---|
| Public product docs | Operators, builders, ISVs | Allowlisted paths in the **source** repo + overlay here | **Yes** |
| Vendor / agent plane | Coding agents, Majesta engineers | Source `docs/architecture/*-playbook*`, `*-build-plan.md`, `backlog/`, `.cursor/` | **No** |

Do not glob a whole `docs/` tree. Use a per-site content map ([sites/one/content-map.yaml](./sites/one/content-map.yaml)).

## API docs

**Now:** curated markdown per family (path, method, scope, what it does / does not). Overlay may customer-cut source pages that still speak in playbook voice.

**Later:** OpenAPI generated **from the product’s Go** (or YAML diffed against that product’s mux), rendered with Scalar **inside** that product’s Starlight site. Never from `GET /describe` (install-local, includes customer objects). Never restore deleted Node OpenAPI stubs.

## Secrets

| Secret | Where | Used for |
|---|---|---|
| GitHub token / App | CMS repo (PR write on **this** repo only) | Agent opens draft PRs |
| `NETLIFY_AUTH_TOKEN` / `NETLIFY_SITE_ID` | Optional; only if a site cannot use the GitHub app | Human or release job — **never** the agent |
| Netlify GitHub App | Each Netlify site | Deploy Preview + production on CMS `main` |

Source repos stay readable without tokens (public). Do not put Netlify secrets in product repos.

## Non-goals

- Serving docs from a product API or a customer install
- Control IDE as the docs host
- A CMS database, Identity, Forms, or Netlify Functions (static HTML only)
- Auto-merging the agent PR
- Auto-publishing One production from `ide` `main` or from the agent
- A second static vendor (Vercel, Cloudflare Pages)
- Per-customer doc tenants or a login portal
- Publishing `backlog/` or live vulnerability detail
- Folding this build into any product `make ci`
- Domain aliases that serve the **same** HTML on every Majesta hostname
