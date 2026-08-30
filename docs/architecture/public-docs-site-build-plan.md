# Public docs site — build plan

**Status:** Active (design locked; scaffold not started)  
**Backlog:** [BP-067](../../backlog/BP-067-public-docs-site.md)  
**Design:** [public-docs-site.md](./public-docs-site.md)  
**Playbook:** [agent-public-docs.md](./agent-public-docs.md)  
**Depends on:** [monorepo.md](../monorepo.md) · [release-cicd.md](../release-cicd.md) · [api-families.md](../api-families.md) · [ADR-005](../adr/005-go-runtime.md) · [ADR-025](../adr/025-api-revision-versioning.md) · [ADR-030](../adr/030-install-agent-runtime.md)

Executable plan for `one.majesta.net`: Astro Starlight as a **publisher only**, plus a merge-event docs agent that updates allowlisted markdown so product CI never writes documentation.

This change set is the plan + playbook. Follow-up agents execute Phases 1–4 via the paste-ready prompts at the end.

---

## Goal

Ship a customer-facing static site that:

1. Publishes allowlisted product docs (install, connect, CLI, family APIs, modules, upgrades) on `one.majesta.net`.
2. Stays in lockstep with `v*` product tags (same version string as GHCR).
3. Has a **guaranteed writer path** when routes, CLI, or install docs change — without putting prose generation in the build.

```text
feature merge → docs-impact (deterministic)
             → docs-update agent (optional same-PR already done → no-op)
             → draft docs PR → Netlify Deploy Preview
v* tag     → netlify deploy --prod → one.majesta.net (+ /v/X.Y/ snapshot)
```

---

## Locked decisions

Three roles, never mixed:

| Role | Owner | Must not |
|---|---|---|
| **Write** | Docs-update agent (path of record) or optional same-PR edit | Deploy `one.majesta.net`; touch Go product logic |
| **Validate** | `make docs-check` + CI Astro build | Generate markdown or OpenAPI |
| **Publish** | Netlify: Deploy Preview on PRs; `netlify deploy --prod` only on `v*` | Run on every `main` push; live in product images |

| Decision | Choice |
|---|---|
| Source of truth | Allowlisted files under `docs/` (plus short wrappers). No second wiki tree |
| Publisher | Astro Starlight under `tools/one-docs` (vendor plane; Node 22) |
| Host | Netlify (vendor static). Not Path A. Not `one-api` |
| Production URL | `https://one.majesta.net/` = latest `v*` tag. Snapshots at `/v/X.Y/` |
| Writer path of record | Merge to `main` (and `workflow_dispatch`) → impact report → docs agent opens a **draft** PR labeled `docs-update` |
| Same-PR docs | Optional and preferred when the feature agent has the context. Not required |
| Launcher | GitHub Action always opens/updates a `docs-update` issue with the impact JSON + paste-ready prompt. Cursor Automation (dashboard) is the recommended consumer; interchangeable because the playbook lives in git |
| API docs now | Curated markdown per family. Split to `docs/api/*` only when a family page is too long |
| API docs later | Generated OpenAPI from Go (or checked-in YAML CI-diffed against the mux) + Scalar inside Starlight. **Phase 5.** Not Node stubs. Never from `GET /describe` |
| Secrets | `NETLIFY_AUTH_TOKEN` / `NETLIFY_SITE_ID` in GitHub Environment `docs-production` only. **Never** granted to coding agents |

### What this amends

[public-docs-site.md](./public-docs-site.md) previously required same-PR docs updates and treated post-merge rewrites as a failure mode. That failed in practice (feature agents skip docs) and tempted a future build to invent prose.

**New rule:** post-merge is allowed **only** as a docs PR. It is never a direct push to the subdomain. CI never writes markdown.

### Loop guards

Skip the impact workflow (no issue, no agent) when any of:

1. The merged PR is labeled `docs-update`.
2. The diff only touches allowlisted public markdown and/or `tools/one-docs/**`.
3. The impact script reports `already_updated: true` (mapped public pages were in the same PR).
4. No changed paths map to a public page.

Never grant agents `NETLIFY_AUTH_TOKEN`.

---

## Content map (Phase 1 artifact)

`tools/one-docs/content-map.yaml` is the allowlist. The Starlight build **includes** these files; it does not copy them into a parallel tree as the source of truth.

Vendor playbooks, `*-build-plan.md`, `backlog/`, `.cursor/`, `AGENTS.md`, Control IDE internals, and `sdk/aws/docs/*` stay **off** the map (GitHub only).

```yaml
# tools/one-docs/content-map.yaml (target shape)
pages:
  /:
    sources: [README.md, docs/glossary.md]
    note: Short rewrite of product nouns. Do not dump the full README.
  /install:
    sources: [docs/self-host.md]
  /connect:
    sources: [docs/builder-connect.md, docs/customer-connect.md]
  /cli:
    sources: [docs/customer-repo.md, docs/customer-developer-workflow.md]
  /api/families:
    sources: [docs/api-families.md]
    note: Customer cut — drop agent-playbook asides.
  /api/revision:
    sources: [docs/adr/025-api-revision-versioning.md]
    note: Pin One-API-Revision; min/current. Not a parallel site tree.
  /api/client:
    sources: [docs/api-families.md]
    note: Split to docs/api/client.md when the family page is too long.
  /api/metadata:
    sources: [docs/api-families.md]
  /api/deploy:
    sources: [docs/api-families.md]
  /api/ops:
    sources: [docs/api-families.md]
  /api/auth:
    sources: [docs/api-families.md]
  /modules:
    sources: [docs/modules/README.md, docs/modules/*.md]
  /customization:
    sources: [docs/customer-customizations.md]
  /upgrades:
    sources: [docs/product-upgrades.md, docs/ops.md]
    note: Customer cut — image roll vs Deploy promote.
  /security:
    sources: [docs/security.md]
    note: Pointer to SECURITY.md. No live vuln detail (BP-026).
  /releases:
    sources: [docs/release-cicd.md]
    note: Customer cut — tag, digest pin, no channel-roll internals.
```

IA matches [public-docs-site.md](./public-docs-site.md) § Public IA. Control IDE is mentioned as an optional JWT client, not the product shell.

---

## Impact path table

`scripts/docs-impact.sh` maps a merge SHA range to public pages. First matching prefix wins; a file may map to more than one page (emit unique `path` values).

| Changed paths (glob) | Public page(s) |
|---|---|
| `internal/httpapi/client_*.go`, `internal/httpapi/server.go` (Client `Handle` lines) | `/api/client`, `/api/families` |
| `internal/httpapi/metadata_routes.go`, `internal/httpapi/sharing_routes.go` | `/api/metadata` |
| `internal/httpapi/deploy_routes.go`, `internal/httpapi/deploy_cloud_routes.go` | `/api/deploy` |
| `internal/httpapi/ops_routes.go` | `/api/ops` |
| `internal/httpapi/auth_routes.go`, `internal/httpapi/install_claim_routes.go` | `/api/auth` |
| `internal/httpapi/scim_routes.go` | `/connect` |
| `internal/httpapi/mcp_routes.go`, `internal/mcp/**`, `tools/one-mcp/**` | `/connect` |
| `internal/httpapi/revision.go`, `internal/compat/**` | `/api/revision` |
| `cmd/one/**` | `/cli` |
| `docs/self-host.md`, `deploy/docker-compose.yml`, `deploy/helm/**`, `deploy/digitalocean/**` | `/install` |
| `docs/builder-connect.md`, `docs/customer-connect.md` | `/connect` |
| `docs/customer-repo.md`, `docs/customer-developer-workflow.md` | `/cli` |
| `docs/api-families.md` | `/api/families` |
| `docs/adr/025-api-revision-versioning.md` | `/api/revision` |
| `docs/customer-customizations.md` | `/customization` |
| `docs/product-upgrades.md`, `docs/ops.md` | `/upgrades` |
| `docs/release-cicd.md` | `/releases` |
| `docs/security.md`, `SECURITY.md` | `/security` |
| `docs/modules/**` | `/modules` |
| `README.md`, `docs/glossary.md` | `/` |
| `docs/api/**` | matching `/api/...` page |

`already_updated: true` when every mapped public **source** file (from the content map) also appears in the same PR diff.

### Impact JSON (stdout)

```json
{
  "base": "<merge-base sha>",
  "head": "<merge-head sha>",
  "merged_pr": 123,
  "skip": false,
  "skip_reason": null,
  "already_updated": false,
  "pages": [
    {
      "path": "/api/client",
      "sources": ["docs/api-families.md"],
      "changed_paths": ["internal/httpapi/client_extras.go"]
    }
  ]
}
```

`skip_reason` when `skip` is true: `docs-update-label` | `docs-only-diff` | `no-mapped-paths` | `already_updated`.

Fixture tests live next to the script (example diffs under `scripts/testdata/docs-impact/`). No Go product packages.

---

## Phased delivery

### Phase 0 — Design (done)

[public-docs-site.md](./public-docs-site.md) + this plan + [agent-public-docs.md](./agent-public-docs.md) + [BP-067](../../backlog/BP-067-public-docs-site.md). No Astro yet.

### Phase 1 — Allowlist + impact script (no Astro required)

1. Add `tools/one-docs/content-map.yaml` (shape above). Fail if a `sources` path is missing.
2. Add `scripts/docs-impact.sh` + fixture tests (`scripts/testdata/docs-impact/`).
3. Add `.github/workflows/docs-impact.yml`:
   - `on.pull_request.types: [closed]` (and `workflow_dispatch`).
   - Run only when `github.event.pull_request.merged == true` and `github.event.pull_request.base.ref == 'main'`.
   - Path filters: `internal/httpapi/**`, `internal/mcp/**`, `internal/compat/**`, `cmd/one/**`, `deploy/**`, `docs/self-host.md`, `docs/builder-connect.md`, `docs/customer-connect.md`, `docs/customer-repo.md`, `docs/customer-developer-workflow.md`, `docs/api-families.md`, `docs/api/**`, `docs/modules/**`, `docs/customer-customizations.md`, `docs/product-upgrades.md`, `docs/ops.md`, `docs/release-cicd.md`, `docs/security.md`, `docs/glossary.md`, `README.md`, `SECURITY.md`, `tools/one-mcp/**`.
   - Skip if the PR has label `docs-update`.
   - Permissions: `contents: read`, `issues: write`. **No** Netlify secrets.
   - Opens or updates a GitHub issue labeled `docs-update` with the JSON + the docs-update paste prompt. Deduplicate: one open issue per SHA range / merged PR number.
4. Do not fail product `make ci` from this workflow.

### Phase 2 — Scaffold publisher

1. Scaffold `tools/one-docs` (Astro Starlight). Pin Astro + Starlight versions on the **Vendor plane — public docs site** row in [tech-stack.md](../tech-stack.md) in that same PR.
2. Include mapped markdown at **build time** (Starlight content collection or build glob). Do not duplicate files as a second wiki.
3. Root `netlify.toml` as specified in [public-docs-site.md](./public-docs-site.md) (base `tools/one-docs`, `npm ci && npm run build`, publish `dist`, ignore unless docs paths changed, `NODE_VERSION=22`).
4. Add `netlify.toml` to `.dockerignore`.
5. `make docs` (local Starlight build) and `make docs-check` (content-map paths exist + `npm run build`).
6. CI: extend `.github/workflows/ci.yml` `changes` filter with `one_docs` ( `tools/one-docs/**`, allowlisted `docs/` paths, `netlify.toml`, `content-map.yaml` ). Add a Node 22 Astro job (mirror Control IDE `22.22.2`; do **not** fold into Go `make ci`).
7. Keep `scripts/assert-product-boundary.sh` excluding `tools/`. Product image audit still rejects `docs/` and `tools/`.
8. Connect the Netlify GitHub app for Deploy Previews (no custom domain required in this phase).

### Phase 3 — Docs-update agent (Cursor Automation recipe)

Playbook: [agent-public-docs.md](./agent-public-docs.md). Specialist: `.cursor/agents/docs-publisher.md`.

**In-repo (required):** impact issue + paste prompt is enough for any coding agent (Cursor, Copilot, human).

**Cursor Automation (recommended launcher, dashboard — not a repo secret):**

| Knob | Value |
|---|---|
| Trigger | New/updated GitHub issue labeled `docs-update`, or merge to `main` when impact JSON `skip` is false |
| Prompt | The **Docs-update job** paste block below |
| Allowed writes | Allowlisted public markdown + `tools/one-docs` (map/sidebar only) |
| Output | Draft PR labeled `docs-update` |
| Forbidden | Merge the PR; `netlify deploy --prod`; edit `cmd/`, `internal/`, `migrations/`, `deploy/`, `tools/control-ide` |

Interchangeable: if Cursor Automation is not configured, a human pastes the same prompt into a cloud agent.

### Phase 4 — Domain + tag publish

1. Connect **this** GitHub repo (`MajestaNet/ide`) as a **second** Netlify site (do not reuse the other Majesta static site).
2. Custom domain `one.majesta.net` (CNAME → Netlify hostname). Production auto-publish from `main` **off**.
3. `release.yml` (or sibling job on `v*`): `PRODUCT_VERSION=X.Y.Z`, `cd tools/one-docs && npm ci && npm run build`, `netlify deploy --prod --dir=dist`. Build also writes `/v/X.Y/` into `dist`. Keep last N=2 or 3 minors on the subdomain.
4. Secrets only in GitHub Environment `docs-production`.
5. Release notes template links `https://one.majesta.net/v/X.Y/`. Do not attach the site tarball to the GitHub Release.

### Phase 5 — Contract hardening (explicitly later)

Not in the first execute-now prompt.

- Per-family OpenAPI generated from Go (or checked-in YAML that CI diffs against the mux).
- Scalar (or Stoplight) page **inside** Starlight.
- Spec `info.version` = API revision integer; `x-product-version` = tag.
- CI: mux route names ⊆ documented paths (tag private/test routes `internal`).
- Revision changelog auto-section from `docs/api/revisions.md` when `API_REVISION_*` changes.

This is **not** restoring deleted Node `openapi/` stubs ([ADR-005](../adr/005-go-runtime.md)). Never generate the public catalog from `GET /describe`.

---

## Workflow (target)

```text
feature branch → PR
  ci.yml: existing Go/IDE jobs
        + (path filter) astro build if tools/one-docs or allowlisted docs change
  Netlify: Deploy Preview (ignore unless docs paths changed)

merge to main
  docs-impact.yml:
    skip if docs-update label / docs-only diff / already_updated / no mapped pages
    else open docs-update issue with impact JSON
  optional: Netlify branch deploy (not production)
  docs-update agent → draft PR (does not merge)

git tag vX.Y.Z → release.yml (unchanged product artifacts)
              → docs-release job (same tag):
                   cd tools/one-docs && npm ci && npm run build
                   netlify deploy --prod --dir=dist
```

---

## Explicit non-goals

- Generating markdown or OpenAPI in the Astro/`make docs` build (Phases 1–4)
- Publishing `backlog/` or live vulnerability detail ([BP-026](../../backlog/BP-026-oss-security-public-backlog.md))
- Serving docs from product images or `GET /docs` on an install
- Using `/describe` or a seeded demo install as the public API catalog
- Restoring Node OpenAPI stubs
- A CMS, customer-login docs portal, or per-customer doc tenants
- Making Control IDE the docs host
- Auto-publishing production from `main` or from an agent without a `v*` tag
- A second static host (Vercel, Cloudflare Pages) in addition to Netlify
- Netlify Functions, Identity, or Forms (static only)
- A long-lived `docs-prod` git branch
- Granting coding agents `NETLIFY_AUTH_TOKEN`
- Folding the Astro build into product `make ci`

---

## Risks

| Risk | Mitigation |
|---|---|
| Docs lag a release | Impact issue is the path of record; tag publish still requires the markdown to be on the tagged commit — land the docs PR before the `v*` tag when the change is customer-facing |
| Impact ↔ agent loop | Skip `docs-update` PRs and docs-only diffs |
| Feature PR already updated pages | `already_updated` → no issue |
| Build invents contract | Validate-only CI; OpenAPI is Phase 5 and still not from `/describe` |
| Image bloat | `tools/` already in `.dockerignore`; keep `netlify.toml` excluded |
| Agent deploys prod | No Netlify token in agent secrets; draft PR only |

---

## Success criteria

- A visitor on `one.majesta.net` (once Phase 4 lands) can install (Path A/B), pin `One-API-Revision`, call Client/Metadata/Deploy, and connect MCP/`one` without opening agent playbooks.
- Product `make ci` / image audit never builds or copies the docs site.
- Merge of a Client route PR with no public-page edit produces a `docs-update` issue; merge of a `docs-update` PR does not.
- `one.majesta.net` moves only on `v*`.
- Public pages are allowlisted; agent playbooks stay on GitHub.

---

## Implementation agent prompts

Paste into a **new** agent after this docs PR is merged. Do **not** start Phase 4 (custom domain + prod token) until Phases 1–2 are green and a human has created the Netlify site. Do **not** start Phase 5 in the same PR.

### 1. Implement Phases 1–2 (scaffold)

```text
Implement Majesta One public docs site Phases 1–2 per
docs/architecture/public-docs-site-build-plan.md.

Read first:
- that plan (locked decisions, content map, impact path table)
- docs/architecture/public-docs-site.md
- docs/architecture/agent-public-docs.md
- backlog/BP-067-public-docs-site.md
- docs/tech-stack.md (vendor plane — public docs site)
- docs/monorepo.md (tools/ is vendor plane)

Scope:
- tools/one-docs (Astro Starlight + content-map.yaml)
- scripts/docs-impact.sh + scripts/testdata/docs-impact/
- .github/workflows/docs-impact.yml
- .github/workflows/ci.yml (one_docs path filter + Astro job only)
- Makefile: docs / docs-check
- root netlify.toml + .dockerignore entry
- pin Astro/Starlight versions in docs/tech-stack.md in this PR

Do not edit cmd/, internal/, migrations/, deploy/, tools/control-ide.
Do not connect custom domain or add NETLIFY_* secrets.
Do not generate OpenAPI.

Acceptance:
- content-map.yaml lists the IA pages; missing source → docs-check fail
- docs-impact fixtures: route-only diff maps to /api/client; docs-update
  labeled / docs-only diff skips; already_updated when sources are in the PR
- make docs-check builds Starlight
- CI Astro job is path-filtered and not part of make ci
- assert-product-boundary still excludes tools/
```

### 2. Docs-update job (event agent)

Use when a `docs-update` issue exists (or paste with an impact JSON). Stay in the docs-publisher fence.

```text
Update Majesta One public product docs from a merge-event impact report.

Read first:
- the GitHub issue body (impact JSON) or the JSON pasted into this prompt
- docs/architecture/agent-public-docs.md
- docs/architecture/public-docs-site-build-plan.md (content map + IA)
- tools/one-docs/content-map.yaml (when it exists)

For each pages[].path in the JSON, update the mapped sources so operators
and builders see the new customer-facing behavior. Curated markdown only:
path, method, scope, what it does, what it does not do.

Lead with MCP + one + family HTTP. Control IDE is an optional JWT client.
Do not publish playbooks, BPs, or Control IDE internals.
Do not edit cmd/, internal/, migrations/, deploy/, tools/control-ide
except tools/one-docs map/sidebar if a new stub page is required.

Open a draft PR labeled docs-update against main. Do not merge.
Do not netlify deploy --prod. Do not use NETLIFY_AUTH_TOKEN.
```
