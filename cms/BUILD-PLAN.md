# Build plan — Majesta CMS aggregator

**Status:** Design seed (no scaffold). Execute **in the CMS repo** after this folder is extracted.  
**Design:** [DESIGN.md](./DESIGN.md) · **Agent:** [AGENT.md](./AGENT.md) · **Netlify:** [NETLIFY.md](./NETLIFY.md)

## Goal

1. Publish customer-facing docs per product on its own subdomain.
2. Keep One in lockstep with `v*` (same version string as GHCR).
3. Guarantee a writer path when a source repo changes — without generating prose in CI.

## Phased delivery (CMS repo)

### Phase 0 — Extract (this seed)

Copy `cms/` to the new repo. Do not implement Astro or Netlify in product repos.

### Phase 1 — Layout + content maps (no Netlify required)

1. `sites/<product>/content-map.yaml` + `pin` file. Fail `docs-check` if a mapped source path is missing **at the pin**.
2. Shared ignore: vendor playbooks, `*-build-plan.md`, `backlog/`, `.cursor/`, `AGENTS.md` stay off every map.
3. Fixture tests for the **notify payload** parser ([SOURCE-CONTRACT.md](./SOURCE-CONTRACT.md)) — no product Go packages.

### Phase 2 — Starlight per site

1. Scaffold Astro Starlight under `sites/one` (Node 22). Pin versions in this repo’s stack note when you add `package.json`.
2. Build-time fetch of the source repo at `pin`. Include mapped markdown; do not commit a second wiki.
3. Per-site `netlify.toml` in the package directory ([NETLIFY.md](./NETLIFY.md)).
4. `make docs-check` (or npm equivalent) in **this** repo only.
5. Connect the Netlify GitHub app for Deploy Previews. Custom domain can wait until Phase 4.

### Phase 3 — CMS agent

Playbook: [AGENT.md](./AGENT.md). Trigger: `repository_dispatch` (or issue labeled `cms-update`) with the notify payload. Output: **draft** PR on this repo. Forbidden: merge, Netlify prod, source-repo product code.

Cursor Automation (dashboard) is the recommended launcher. Fallback: paste the same prompt into a cloud agent. Do not store a Cursor API token in GitHub secrets unless you later choose that.

### Phase 4 — Domains + production

1. One Netlify site per product; CNAME each subdomain ([NETLIFY.md](./NETLIFY.md)).
2. Production auto-publish from **CMS** `main` **on** (reviewed overlay). That is not the same as publishing from a **source** `main`.
3. For One: production pin is `v*`; assemble `/v/X.Y/` in `sites/one` dist; keep last N=2 or 3 minors.
4. Release notes on the product repo may link `https://one.majesta.net/v/X.Y/`. Do not attach a site tarball to the product GitHub Release.

### Phase 5 — Contract hardening (later)

OpenAPI from product Go + Scalar inside Starlight. Not in the first scaffold.

## Success criteria

- A visitor on `one.majesta.net` can install Path A/B, pin `One-API-Revision`, call family HTTP, and connect MCP/`one` without opening agent playbooks.
- Product `make ci` / image audit never build or copy this repo.
- CMS agent PRs are draft; production moves only after a human merge.
- One’s live subdomain tracks a `v*` pin, not `ide` trunk.
- Playbooks and backlog stay off every public map.

## Implementation prompt (CMS repo, Phases 1–2)

```text
Scaffold the Majesta CMS aggregator Phases 1–2 per BUILD-PLAN.md.

Read first: DESIGN.md, NETLIFY.md, sites/README.md, sites/one/README.md,
sites/one/content-map.yaml, SOURCE-CONTRACT.md.

Scope: this CMS repo only. Starlight under sites/one; pin + overlay;
content-map check; notify payload fixtures. Per-site netlify.toml.

Do not edit Majesta One (or any product) cmd/, internal/, migrations/,
deploy/, tools/control-ide. Do not add NETLIFY_* to product repos.
Do not generate OpenAPI. Do not attach custom domains until Phase 4.
```
