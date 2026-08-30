# BP-067: Public docs site + event-triggered docs updates

- **Severity:** Medium
- **Status:** Open (plan landed; implementation not started)
- **Area:** `tools/one-docs` + allowlisted `docs/` (vendor plane); `.github/workflows/docs-impact.yml`, `scripts/docs-impact.sh`
- **Design:** [public-docs-site.md](../docs/architecture/public-docs-site.md)
- **Build plan:** [public-docs-site-build-plan.md](../docs/architecture/public-docs-site-build-plan.md)
- **Playbook:** [agent-public-docs.md](../docs/architecture/agent-public-docs.md)
- **Related:** [ADR-005](../docs/adr/005-go-runtime.md) · [ADR-025](../docs/adr/025-api-revision-versioning.md) · [ADR-030](../docs/adr/030-install-agent-runtime.md) · [release-cicd.md](../docs/release-cicd.md) · [BP-026](./BP-026-oss-security-public-backlog.md)

## Problem

Majesta One is API-first with **no embedded UI**. Customer-facing docs (install, connect, CLI, family APIs) live in this monorepo mixed with agent playbooks and backlog items. There is no public host yet, and no guaranteed path for agents to update the customer corpus when routes or install docs change.

Two failure modes:

1. **No publisher** — operators must read GitHub `docs/` (and often playbooks) to install or pin `One-API-Revision`.
2. **No writer path** — playbooks asked feature agents to update public pages in the same PR; that is skipped, and a future Astro build must not invent the missing prose.

## Why it matters

A visitor on `one.majesta.net` should install (Path A/B), pin the API revision, call family HTTP, and connect MCP/`one` without opening `.cursor/` manuals. Docs that describe a released API must follow `v*` tags, same as GHCR. Product images must never contain `docs/` or `tools/`.

## Direction (locked)

Follow [public-docs-site-build-plan.md](../docs/architecture/public-docs-site-build-plan.md).

| Phase | Outcome |
|---|---|
| 0 | Design + this BP + playbook (this PR) |
| 1 | `content-map.yaml` + `scripts/docs-impact.sh` + `docs-impact.yml` (merge → `docs-update` issue) |
| 2 | Astro Starlight under `tools/one-docs`; `make docs-check`; CI path-filter build; Netlify Deploy Previews |
| 3 | Docs-update agent / Cursor Automation consumes the issue; draft `docs-update` PR |
| 4 | `one.majesta.net` on `v*` via `netlify deploy --prod` + `/v/X.Y/` snapshots |
| 5 | Later: generated OpenAPI + Scalar (not `/describe`, not Node stubs) |

**Write / validate / publish stay separate.** CI never writes markdown. Agents never hold `NETLIFY_AUTH_TOKEN`. Same-PR docs edits remain optional.

## Explicit non-goals

- Serving docs from `one-api` or a customer install
- Dumping `backlog/` or live vulnerability detail onto the subdomain ([BP-026](./BP-026-oss-security-public-backlog.md))
- Making Control IDE the docs host
- Auto-publishing production from `main`
- Folding Astro into product `make ci`

## Implementation agent prompt

Paste after this docs PR is merged. Implement **Phases 1–2 first**.

```text
Implement Majesta One public docs site Phases 1–2 per
docs/architecture/public-docs-site-build-plan.md.

Read first:
- that plan (locked decisions, content map, impact path table)
- docs/architecture/public-docs-site.md
- docs/architecture/agent-public-docs.md
- backlog/BP-067-public-docs-site.md

Scope: tools/one-docs, scripts/docs-impact.sh, docs-impact.yml,
ci.yml one_docs filter, Makefile docs/docs-check, netlify.toml.
No cmd/, internal/, migrations/, deploy/, tools/control-ide.
No custom domain, no NETLIFY_* secrets, no OpenAPI.
```
