# Agent playbook: public docs (`one.majesta.net`)

For agents that write **customer-facing** Majesta One docs, scaffold `tools/one-docs`, or consume a merge-event impact report. Follow this before editing public pages.

**Design:** [public-docs-site.md](./public-docs-site.md)  
**Build plan:** [public-docs-site-build-plan.md](./public-docs-site-build-plan.md)  
**Backlog:** [BP-067](../../backlog/BP-067-public-docs-site.md)  
**Specialist:** `.cursor/agents/docs-publisher.md`

The Astro site is a **publisher**. This playbook is the **writer**. Product CI must not generate markdown.

---

## Plane fence (mandatory)

| May edit | Must not edit |
|---|---|
| Allowlisted public markdown listed in `tools/one-docs/content-map.yaml` (until that file exists: the IA sources in the build plan) | `cmd/`, `internal/`, `migrations/`, `deploy/` (product plane) |
| `tools/one-docs/**` (Starlight, sidebar, content map) | `tools/control-ide/**` |
| `scripts/docs-impact.sh`, `scripts/testdata/docs-impact/`, `.github/workflows/docs-impact.yml` when the task is **Phase 1–2 scaffold** | Product `make ci` Go jobs; image COPY allowlist |
| This playbook, [public-docs-site.md](./public-docs-site.md), [public-docs-site-build-plan.md](./public-docs-site-build-plan.md), [BP-067](../../backlog/BP-067-public-docs-site.md) | `backlog/BP-*` except BP-067 status when de-risking this item |

Phase 1–2 scaffold may also touch root `netlify.toml`, `.dockerignore`, `Makefile`, and the `one_docs` path-filter in `.github/workflows/ci.yml`. That is still vendor plane — not Control IDE, not Go handlers.

---

## Where to look

| Concern | Path |
|---|---|
| Design (host, IA, versioning, Netlify) | [public-docs-site.md](./public-docs-site.md) |
| Phases, impact table, paste prompts | [public-docs-site-build-plan.md](./public-docs-site-build-plan.md) |
| Family HTTP contract (customer cut) | [api-families.md](../api-families.md) |
| Install Path A/B | [self-host.md](../self-host.md) |
| MCP + CLI connect | [builder-connect.md](../builder-connect.md), [customer-connect.md](../customer-connect.md) |
| `one` DX | [customer-repo.md](../customer-repo.md), [customer-developer-workflow.md](../customer-developer-workflow.md) |
| API revision pin | [ADR-025](../adr/025-api-revision-versioning.md) |
| Managed packages | [docs/modules/](../modules/README.md) |
| Release / digest pin | [release-cicd.md](../release-cicd.md) |
| Product vs customer | [customer-customizations.md](../customer-customizations.md) |
| Module map row | [module-map.md](./module-map.md) |

---

## Roles

| Role | Path of record | Optional |
|---|---|---|
| Feature agent (`api-families`, `deploy-ops`, …) | Land the code PR | Update the mapped public page in the **same PR** when the context is already loaded |
| Docs-update agent (this playbook) | After merge: read impact JSON, patch mapped pages, open a **draft** PR labeled `docs-update` | — |
| CI / Astro | Build Starlight; fail on missing content-map files | — |
| `release.yml` on `v*` | `netlify deploy --prod` | — |

Do **not** treat “docs in a later feature PR” as the writer path. The merge-event agent is. Do **not** auto-push to `one.majesta.net`.

---

## What to do (change types)

### A. Docs-update job (event)

1. Read the impact JSON (`pages[]`). Skip if `skip: true` or `already_updated: true`.
2. For each `path`, open the mapped `sources` and update the **customer** wording: path, method, scope, what it does, what it does **not** do.
3. If a family page is too long, add a thin `docs/api/<family>.md` and point the content map at it (same PR).
4. Do not dump mux listings, playbook checklists, or backlog IDs onto the public page.
5. Open a draft PR labeled `docs-update`. Do not merge. Do not deploy production.

### B. Optional same-PR (feature agent)

If you already changed `internal/httpapi/**`, `cmd/one/**`, or install/connect docs, you **may** update the mapped public source in the same PR. That sets `already_updated` and suppresses the issue. It is not required.

### C. Scaffold (Phases 1–2 only)

Follow the Phase 1–2 prompt in the build plan. Pin Starlight versions in [tech-stack.md](../tech-stack.md). Keep the Astro job out of product `make ci`.

### D. Tone and IA

- Audience: operators, builders, ISVs — not coding agents.
- Lead with **MCP + `one` + family HTTP**.
- Mention Control IDE as an **optional JWT client**, not the product shell ([ADR-030](../adr/030-install-agent-runtime.md)).
- Footer may link “Source & contributing” to GitHub. Do not nav to playbooks, BPs, or Control IDE internals.

---

## GitHub event → agent

1. `.github/workflows/docs-impact.yml` runs on merged PRs to `main` (path-filtered) and on `workflow_dispatch`.
2. `scripts/docs-impact.sh` emits JSON. Loop guards: `docs-update` label, docs-only diff, `already_updated`, no mapped pages.
3. Workflow opens/updates an issue labeled `docs-update` containing the JSON and the docs-update paste prompt.
4. **Launcher (recommended):** Cursor Automation on that issue (dashboard). **Fallback:** human pastes the prompt into a cloud agent. The playbook does not depend on a Cursor API token in GitHub secrets.

Permissions for the impact workflow: `contents: read`, `issues: write`. No `NETLIFY_*`.

---

## Checklist (docs-update PR)

- [ ] Every `pages[].path` in the impact JSON is addressed or explicitly unchanged with a reason in the PR body
- [ ] Only allowlisted sources + `tools/one-docs` map/sidebar changed
- [ ] Customer tone; no agent-playbook voice
- [ ] MCP + `one` + family HTTP first; Control IDE optional
- [ ] PR is **draft**, labeled `docs-update`
- [ ] No merge, no `netlify deploy --prod`, no Netlify token

---

## Explicit non-goals (until the build plan says otherwise)

- Generating OpenAPI or scraping `/describe`
- Publishing `backlog/`, live vulns, or `.cursor/` manuals
- Serving docs from `one-api` or a customer install
- Editing Control IDE
- Granting or using `NETLIFY_AUTH_TOKEN`
- Auto-merging the docs PR
