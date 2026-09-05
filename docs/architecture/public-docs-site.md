# Public docs (`one.majesta.net`)

Customer-facing Majesta One docs (install, connect, CLI, family APIs) are published on **`one.majesta.net`** by a **separate Majesta CMS aggregator** (its own public Git repo, Netlify, one site per product subdomain). That aggregator is **not** this monorepo.

**Status:** publisher extracted — do **not** scaffold Astro, `tools/one-docs`, `netlify.toml`, or a docs-impact workflow here.  
**This repo:** GitHub `docs/` remain operator/contributor markdown. No runtime or CI dependency on the CMS.  
**Backlog:** [BP-067](../../backlog/BP-067-public-docs-site.md)

This file is a pointer plus the **source map** the CMS overlay should use after a notify / `cms-update` payload. Host, pin, CMS agent, and Netlify multi-site rules live in the CMS repository.

## What stays here

- Customer-cut family HTTP: [`docs/api/`](../api/) (`client`, `metadata`, `deploy`, `ops`, `auth`) plus overview [`api-families.md`](../api-families.md).
- Managed module pages [`docs/modules/*.md`](../modules/README.md) (already customer voice).
- Public object catalog [`docs/objects.md`](../objects.md) (core tables + module index).
- Other operator markdown (`docs/self-host.md`, connect/CLI/install).
- Agent playbooks and build plans — **GitHub only**, never a public nav target.

## CMS overlay (after notify — not this repo)

Do **not** generate a second wiki in CMS overlays. Do **not** generate OpenAPI here (later, from Go, CMS Phase 5). Do **not** default the production pin to `main`. Production still waits for a **`v*`** tag and must fail closed if the pin is unset.

| Public route (suggested) | Source in this repo | `include` | Notes |
|---|---|---|---|
| `/api` | `docs/api-families.md` | `true` | Overview only. **Do not paste this file onto the five family routes.** |
| `/api/client` | `docs/api/client.md` | `true` | |
| `/api/metadata` | `docs/api/metadata.md` | `true` | |
| `/api/deploy` | `docs/api/deploy.md` | `true` | |
| `/api/ops` | `docs/api/ops.md` | `true` | |
| `/api/auth` | `docs/api/auth.md` | `true` | |
| `/modules` · `/modules/*` | `docs/modules/README.md` · `docs/modules/*.md` | `true` | Package source is already customer voice. |
| `/objects` | `docs/objects.md` | `true` | Static catalog. |
| — | `docs/data-model.md` | `false` | Contributor storage/performance. Not the public objects page. |
| — | `GET /client/v1/describe` | **never** | Authenticated runtime schema. Forbidden as a public catalog. |
| — | `docs/architecture/**`, `backlog/` | `false` | Playbooks and risk list stay on GitHub. |

Preview/CI may use fixtures. Mapped, includable files still do not reach production until a `v*` pin is set.

## What must not land here

- `tools/one-docs`, root `netlify.toml`, `make docs` / `make docs-check` Starlight targets
- `scripts/docs-impact.sh`, `.github/workflows/docs-impact.yml`, `NETLIFY_*` secrets
- Serving docs from `one-api`, a customer install, or Control IDE
- Auto-publishing `one.majesta.net` from this repo’s `main`

Production on `one.majesta.net` is expected to follow **`v*` tags** (same version string as GHCR), not trunk. Feature agents may still edit GitHub `docs/` in a product PR; they are not required to, and they do not deploy the subdomain.

## Related

- [BP-067](../../backlog/BP-067-public-docs-site.md)
- [agent-public-docs.md](./agent-public-docs.md) — fence for this repo
- [release-cicd.md](../release-cicd.md) — product tags publish images, not the docs site
- [ADR-025](../adr/025-api-revision-versioning.md) · [ADR-030](../adr/030-install-agent-runtime.md) · [BP-026](../../backlog/BP-026-oss-security-public-backlog.md)
