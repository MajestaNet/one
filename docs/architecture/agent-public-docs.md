# Agent playbook: GitHub docs (this repo)

Customer-facing **GitHub** markdown in this monorepo (install, connect, CLI, family APIs). The public host `one.majesta.net` is a **separate CMS aggregator** — do not implement it here.

**Pointer:** [public-docs-site.md](./public-docs-site.md) · [BP-067](../../backlog/BP-067-public-docs-site.md)

## Plane fence

| May edit | Must not |
|---|---|
| Product markdown under `docs/` that operators already read (self-host, connect, CLI, api-families, modules, upgrades, security, release-cicd, glossary) | `cmd/`, `internal/`, `migrations/`, `deploy/` unless the task is a documented backend change |
| | `tools/one-docs/**`, root `netlify.toml`, `scripts/docs-impact.sh`, CMS/Netlify secrets |
| | `tools/control-ide/**` |
| | Scaffolding or depending on the CMS repo |

There is no `make docs-check` / Astro job in product CI. Verify markdown locally; do not run product `make ci` as a docs-only check.

## Tone

If you edit operator-facing `docs/`, prefer customer wording (path, method, scope, what it does / does not). Lead with MCP + `one` + family HTTP. Control IDE is an optional JWT client ([ADR-030](../adr/030-install-agent-runtime.md)). Do not dump playbook checklists or live vuln detail ([BP-026](../../backlog/BP-026-oss-security-public-backlog.md)).

Same-PR docs edits are optional. They do not publish `one.majesta.net`.
