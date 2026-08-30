# Public docs (`one.majesta.net`)

Customer-facing Majesta One docs (install, connect, CLI, family APIs) are published on **`one.majesta.net`** by a **separate Majesta CMS aggregator** (its own public Git repo, Netlify, one site per product subdomain). That aggregator is **not** this monorepo.

**Status:** publisher extracted — do **not** scaffold Astro, `tools/one-docs`, `netlify.toml`, or a docs-impact workflow here.  
**This repo:** GitHub `docs/` remain operator/contributor markdown. No runtime or CI dependency on the CMS.  
**Backlog:** [BP-067](../../backlog/BP-067-public-docs-site.md)

This file is a pointer only. Host, pin + overlay, CMS agent, and Netlify multi-site rules live in the CMS repository (seed was designed for one-time copy out of this tree).

## What stays here

- Allowlisted-quality product markdown operators already read on GitHub (`docs/self-host.md`, [api-families.md](../api-families.md), connect/CLI/install docs).
- Agent playbooks and build plans — **GitHub only**, never a public nav target.

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
