# Public docs site (`one.majesta.net`)

Recommendation for publishing **customer-facing** Majesta One docs (overview, install, API, MCP/CLI) on `one.majesta.net`, in lockstep with product releases.

**Status:** design locked — executable plan [public-docs-site-build-plan.md](./public-docs-site-build-plan.md); no site scaffold yet. **Host:** Netlify (vendor static; not Path A).  
**Plane:** vendor (`tools/` + selected `docs/`). Never product image.  
**Backlog:** [BP-067](../../backlog/BP-067-public-docs-site.md) · **Playbook:** [agent-public-docs.md](./agent-public-docs.md)  
**Related:** [monorepo.md](../monorepo.md) · [release-cicd.md](../release-cicd.md) · [api-families.md](../api-families.md) · [ADR-005](../adr/005-go-runtime.md) · [ADR-025](../adr/025-api-revision-versioning.md) · [ADR-030](../adr/030-install-agent-runtime.md)

---

## Thesis

> Keep one markdown source of truth in this monorepo. A small static site under `tools/` is a **publisher**, not a second wiki. Netlify serves it (Deploy Previews on PRs; production on `v*` tags, same as GHCR). A merge-event docs agent updates public pages in a **follow-up draft PR** (same-PR edits are optional). Agents do not own the live subdomain. CI never writes markdown.

That matches how the product already ships: trunk + semver tags; release publishes artifacts; operators pin versions; product images never contain `docs/` or `tools/`.

---

## What exists today (gap)

| Fact | Implication for a public site |
|---|---|
| ~155 markdown files under `docs/` plus `backlog/` and agent playbooks | Dumping the tree onto `one.majesta.net` would mix customer docs with coding-agent operating manuals |
| OpenAPI was **purged** (ADR-005); `/openapi.json` is asserted gone | Public API reference must be curated markdown first; generated OpenAPI is a later artifact, not a revival of Node stubs |
| `GET /client/v1/describe` is **install-local** (managed + custom objects) | Do not scrape a seed install’s describe as the public contract |
| Three version axes (ADR-025): product semver, family major (`/client/v1`), API revision `N` | The site must version **product tags**; API revision is a changelog / pin guide, not a parallel site tree |
| `release.yml` on `v*` publishes images + binaries only | Docs are not a release artifact today — that is the missing step |
| Product runtime has **no embedded UI** (ADR-030) | Do not serve marketing/docs from `one-api` or from customer installs |
| TypeScript is allowed under `tools/` (Control IDE, `one-mcp`) | Astro/Starlight in `tools/one-docs` is vendor plane, same fence as the IDE |
| Path A is DigitalOcean App Platform | That is the **customer install** path. The docs site is vendor marketing — host it on **Netlify** (already used for other Majesta static sites), not in the product image |

GitHub already *is* the contributor/agent docs host. `one.majesta.net` should be the **product** host.

---

## Decision

### 1. Two corpora, one repo

| Corpus | Audience | Lives | Published to `one.majesta.net`? |
|---|---|---|---|
| **Public product docs** | Operators, builders, ISVs | Selected files under `docs/` (allowlist) | **Yes** |
| **Vendor/agent plane** | Coding agents, Majesta engineers | `docs/architecture/*-playbook*`, `docs/architecture/*-build-plan.md`, `backlog/`, `.cursor/`, `AGENTS.md` | **No** (GitHub only) |
| **Optional curated ADRs** | Curious operators | `docs/adr/` subset (001, 004, 005, 006, 010, 012, 014, 015, 019, 020, 025, 029, 030) | Later, as “Architecture notes” — not the homepage |

Do **not** copy pages into a second tree. The site **includes** existing markdown via an allowlist (`tools/one-docs/content-map.yaml` or Starlight `sidebar` + `src/content` collections that glob listed paths). Duplication is how docs drift from releases.

### 2. Site = Astro Starlight under `tools/one-docs`

A small static frontend is the right shape:

- Markdown-first (this repo already is).
- Zero runtime on the subdomain (static HTML/CSS/JS).
- TypeScript under `tools/` is already the vendor convention ([tech-stack.md](../tech-stack.md)).
- Starlight gives search, versioned docs, and API-doc layouts without inventing a design system.
- `.dockerignore` already excludes `tools/` — no image-boundary change.

**Not** Docusaurus (heavier React app), **not** Next.js, **not** Control IDE chrome, **not** GitBook/Notion as source of truth.

When scaffolding, fill in Astro/Starlight versions on the **Vendor plane — public docs site** row in [tech-stack.md](../tech-stack.md) (host is already **Netlify**).

### 3. Agents write source; CI publishes; humans own GA

Three roles, never mixed. Detail: [public-docs-site-build-plan.md](./public-docs-site-build-plan.md).

| Role | Does | Does not |
|---|---|---|
| Docs-update agent ([agent-public-docs.md](./agent-public-docs.md)) | On merge (impact report), patch allowlisted pages and open a **draft** `docs-update` PR | Merge that PR; `netlify deploy --prod`; touch Go product |
| Domain agent (existing playbooks) | Land the feature PR. **May** update the mapped public page in the same PR | Be required to write docs; auto-push to the subdomain |
| CI (`ci.yml`) | Build Starlight on PRs that touch `tools/one-docs` or allowlisted docs; fail if the content map points at a missing file | Generate markdown or OpenAPI; `netlify deploy --prod` |
| Netlify (GitHub app) | Deploy Preview on docs-touching PRs | Publish `one.majesta.net` |
| `release.yml` on `v*` | Build site with `PRODUCT_VERSION=X.Y.Z`; `netlify deploy --prod` to `one.majesta.net` **and** archive `/v/X.Y/` in that build | Roll customer installs; auto-publish every `main` push |
| Human | Approve first GA cut; approve raising `API_REVISION_MIN` copy; configure Cursor Automation / Netlify site | Be the CMS |

This is the same split as product release: PRs validate; tags publish. Writer path of record is the merge-event agent, not same-PR.

---

## Public IA (what `one.majesta.net` should contain)

Start thin. Every page must already exist (or be a short wrapper) in `docs/`.

```text
one.majesta.net
├── /                      Overview (short rewrite of README + glossary nouns)
├── /install               Path A App Platform + Path B Compose/Helm   ← self-host.md
├── /connect               MCP + JWT + CLI                             ← builder-connect.md, customer-connect.md
├── /cli                   one project / org validate|deploy           ← customer-repo.md, customer-developer-workflow.md
├── /api
│   ├── /families          Client / Metadata / Deploy / Ops / Auth     ← api-families.md (customer cut)
│   ├── /revision          Pin One-API-Revision; min/current           ← ADR-025 summary
│   ├── /client            Curated endpoint list (not describe)
│   ├── /metadata
│   ├── /deploy
│   ├── /ops
│   └── /auth
├── /modules               Managed packages                            ← docs/modules/*
├── /customization         custom vs managed; never fork product       ← customer-customizations.md
├── /upgrades              Image roll vs Deploy promote                ← product-upgrades.md, ops.md (customer cut)
├── /security              AuthZ posture, SECURITY.md pointer
└── /releases              GitHub Release + digest pin                 ← release-cicd.md (customer cut)
```

**Keep on GitHub only** (link from a footer “Source & contributing”, do not nav):

- Agent playbooks (`agent-*.md`, `agent-routing.md`, `module-map.md`)
- Build plans (`*-build-plan.md`)
- `backlog/BP-*`
- Control IDE internals (`control-ide-*.md`, Operate/Run graph plans)
- Community `sdk/aws/docs/*`
- `local-development-mac.md` (contributor DX)

Control IDE is optional/frozen ([ADR-030](../adr/030-install-agent-runtime.md)). Public docs should lead with **MCP + `one` + family HTTP**, and mention Control IDE as an optional JWT client — not as the product shell.

### API reference (now vs later)

**Now (GA of the site):** curated markdown per family — path, method, scope, what it does, what it does *not* do. Source: `docs/api-families.md` plus a new thin `docs/api/` folder of endpoint pages **only when** a family page gets too long. The merge-event docs agent is the path of record when routes change ([agent-public-docs.md](./agent-public-docs.md)); feature agents may still edit the page in the same PR.

**Later (separate plan, after the site ships):** generate OpenAPI **from Go** (or a checked-in YAML that CI diffs against the mux). Render with Scalar/Stoplight **inside** Starlight. Split specs by family (`client.yaml`, `metadata.yaml`, …). Spec `info.version` = API revision integer; `x-product-version` = tag. This is **not** restoring the deleted Node `openapi/` stubs (ADR-005 non-goal).

Never generate the public catalog from `GET /describe` — that payload includes customer objects.

---

## Versioning on the subdomain

Mirror ADR-025. Do not invent a fourth axis.

| URL | Meaning |
|---|---|
| `https://one.majesta.net/` | Docs for the **latest `v*` product tag** (same bits as GHCR `:X.Y.Z`) |
| `https://one.majesta.net/v/0.14/` | Frozen snapshot for operators still on `PRODUCT_VERSION=0.14.x` |
| `/api/revision` on each snapshot | `apiRevision.min` / `current` for **that** image; how to send `One-API-Revision` |

Rules:

1. A `vX.Y.Z` tag builds the site with that version string in the header (same `VERSION` as `release.yml`).
2. Keep **last N minors** on the subdomain (start with N=2 or 3). Older snapshots stay on GitHub at the tag.
3. Do **not** version the site as `/client/v1` vs `/client/v2` — family majors are path prefixes inside the API, not site versions.
4. Bumping `API_REVISION_CURRENT` or raising `min` **requires** a public revision changelog paragraph in that same release (already required in [release-cicd.md](../release-cicd.md)).
5. Netlify **Deploy Previews** cover PRs. A `main` branch deploy may exist as a trunk URL. Production `one.majesta.net` moves only on `v*` (`netlify deploy --prod`), same as GHCR.

This is the same “tag publishes; `:latest` is forbidden” rule as images.

---

## Hosting (`one.majesta.net`) — Netlify

**Choice:** Netlify static site, same pattern as the other Majesta static-site repo. This is **vendor-plane hosting**. It is not Path A. Customer installs stay on DigitalOcean App Platform / Helm; docs never ride `one-api`.

| Option | Fit | Notes |
|---|---|---|
| **Netlify** (chosen) | High | Git-connected static site, Deploy Previews, custom domain. Matches existing Majesta static-site ops |
| DigitalOcean App Platform static / Spaces | Fallback | Same cloud as Path A; use only if Netlify is unavailable |
| GitHub Pages | Fallback | Weaker preview UX than Netlify |
| Vercel / Cloudflare Pages | Avoid | Do not add a second static vendor |
| Inside `one-api` or a customer install | **Forbidden** | Violates no-embedded-UI, mixes vendor content with install plane, bloats images |

### Site settings (Netlify UI + `netlify.toml`)

Connect **this** GitHub repo (`MajestaNet/ide`) as a second Netlify site (do not reuse the other repo’s site). Root `netlify.toml` when `tools/one-docs` exists:

```toml
# Vendor-plane docs publisher. Not copied into product images.
[build]
  base = "tools/one-docs"
  command = "npm ci && npm run build"
  publish = "dist"
  # Skip Netlify builds when the PR did not touch public docs.
  ignore = "git diff --quiet $CACHED_COMMIT_REF $COMMIT_REF -- tools/one-docs docs README.md netlify.toml"

[build.environment]
  NODE_VERSION = "22"

[context.deploy-preview]
  command = "npm ci && npm run build"

[context.branch-deploy]
  command = "npm ci && npm run build"
```

| Netlify knob | Value |
|---|---|
| Base directory | `tools/one-docs` |
| Build command | `npm ci && npm run build` |
| Publish directory | `dist` (relative to base) |
| Node | 22 (same floor as Control IDE) |
| Functions / Identity / Forms | **Off** — static HTML only |
| Production auto-publish from `main` | **Off** (stop auto publishing) |
| Deploy Previews | **On** (PRs) — same as the other site |
| Branch deploys | Optional `main` → `main--<site>.netlify.app` (trunk, not GA) |

### DNS

CNAME `one.majesta.net` → the Netlify site hostname (`<site>.netlify.app`, or Netlify DNS). Apex `majesta.net` stays the company site and links here.

### Credentials

| Secret | Where | Used for |
|---|---|---|
| `NETLIFY_AUTH_TOKEN` | GitHub Environment `docs-production` | `release.yml` `netlify deploy --prod` on `v*` |
| `NETLIFY_SITE_ID` | same | Target site |
| Netlify GitHub App | Netlify site settings | Deploy Previews only |

These are **not** product-image or install secrets. A tag must not mutate customer installs ([release-cicd.md](../release-cicd.md)).

Do **not** grant coding agents the Netlify token.

### Production vs preview (the one difference from a typical marketing site)

The other Majesta static site likely publishes **production on `main`**. This repo’s installable product publishes on **`v*` tags**. Docs that describe a released API should follow the tag:

```text
PR  → Netlify Deploy Preview (review the page)
main → optional branch deploy (trunk; not one.majesta.net)
v*  → netlify deploy --prod  → https://one.majesta.net
      (same tag as ghcr.io/majestanet/one-api:X.Y.Z)
```

If you later prefer the other site’s “`main` is live” DX, flip production auto-publish on — that is an explicit tradeoff (docs can describe unreleased wire). `/v/X.Y/` snapshots still belong **in the build output**, not as old Netlify deploy permalinks.

---

## Release pipeline (additions)

```text
feature branch → PR
  ci.yml: existing Go/IDE jobs
        + (path filter) astro build if tools/one-docs or allowlisted docs change
  Netlify: Deploy Preview (ignore unless docs paths changed)

merge to main
  optional: Netlify branch deploy (not production)

git tag vX.Y.Z → release.yml (unchanged product artifacts)
              → docs-release job (same tag):
                   cd tools/one-docs && npm ci && npm run build
                   netlify deploy --prod --dir=dist
                   (build also writes /v/X.Y/ snapshot into dist)
```

Do **not** attach the site tarball to the GitHub Release (quota). Link `https://one.majesta.net/v/X.Y/` from the Release notes instead.

`scripts/assert-product-boundary.sh` already forbids `tools/` in images. Keep `tools/one-docs` on that exclude list. Add root `netlify.toml` to `.dockerignore` when the file exists.

---

## Agent loop (concrete)

Path of record: **merge event → impact report → docs-update agent → draft PR**. Same-PR edits are optional. CI never writes prose. Detail: [public-docs-site-build-plan.md](./public-docs-site-build-plan.md) · playbook [agent-public-docs.md](./agent-public-docs.md).

1. `docs-impact.yml` on merge to `main` (path-filtered) runs `scripts/docs-impact.sh` and opens a `docs-update` issue unless the PR already updated mapped pages, is labeled `docs-update`, or is docs-only.
2. The docs-publisher agent (Cursor Automation or pasted prompt) patches allowlisted sources and opens a **draft** PR labeled `docs-update`. It does not merge and does not deploy production.
3. Feature playbooks ([agent-api-families.md](./agent-api-families.md), [agent-deploy.md](./agent-deploy.md), builder connect) **may** update `/api/...`, `/install`, or `/connect` in the same PR; that suppresses the issue (`already_updated`).
4. Do **not** grant an agent production deploy credentials (`NETLIFY_AUTH_TOKEN`).

Post-merge is allowed **only** as that docs PR — never as a push to `one.majesta.net`.

---

## Phased delivery

Execute [public-docs-site-build-plan.md](./public-docs-site-build-plan.md). Summary:

| Phase | Outcome |
|---|---|
| 0 | This design + build plan + playbook + [BP-067](../../backlog/BP-067-public-docs-site.md) |
| 1 | `content-map.yaml` + `scripts/docs-impact.sh` + merge-event issue |
| 2 | Astro Starlight scaffold, `make docs-check`, CI path-filter, Deploy Previews |
| 3 | Docs-update agent / Cursor Automation recipe |
| 4 | Custom domain + `v*` `netlify deploy --prod` + `/v/X.Y/` |
| 5 | Later: generated OpenAPI + Scalar (not `/describe`) |

---

## Non-goals

- Publishing `backlog/` or live vulnerability detail ([BP-026](../../backlog/BP-026-oss-security-public-backlog.md)).
- Serving docs from product images or `GET /docs` on an install.
- Using `/describe` or a seeded demo install as the public API catalog.
- Restoring Node OpenAPI stubs (ADR-005).
- A CMS, customer-login docs portal, or per-customer doc tenants.
- Making Control IDE the docs host.
- Generating markdown or OpenAPI in the Astro/`make docs` build (Phases 1–4).
- Auto-publishing production (`one.majesta.net`) from `main` or from an agent without a `v*` tag.
- A second static host (Vercel, Cloudflare Pages) in addition to Netlify.
- Netlify Functions, Identity, or Forms for this site (static only).
- A long-lived `docs-prod` git branch (trunk + tags, same as product).

---

## Success criteria

- A visitor on `one.majesta.net` can install (Path A/B), pin `One-API-Revision`, call Client/Metadata/Deploy, and connect MCP/`one` without opening agent playbooks.
- Merge of a customer-facing route PR with no public-page edit produces a `docs-update` issue (once Phase 1 lands); merge of a `docs-update` PR does not.
- Each `v*` tag that publishes GHCR also publishes a docs snapshot with the same version string.
- Public pages are allowlisted; agent playbooks stay on GitHub.
- Product image audit still rejects `docs/` and `tools/`.
- Product CI never generates markdown.
