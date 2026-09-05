# API families — historical build plan (shipped)

**Superseded as customer docs.** Do not execute the original in-tree phases. Endpoint catalogs live in [`docs/api/`](../api/). The public overview is [`docs/api-families.md`](../api-families.md).

| Family | Customer page |
|---|---|
| Client | [api/client.md](../api/client.md) |
| Metadata | [api/metadata.md](../api/metadata.md) |
| Deploy | [api/deploy.md](../api/deploy.md) |
| Ops | [api/ops.md](../api/ops.md) |
| Auth | [api/auth.md](../api/auth.md) |

Design: [ADR-004](../adr/004-three-api-families.md). Agent playbook: [agent-api-families.md](./agent-api-families.md). Phases A–F (surface split, ownership, bundles, customer tests, repo→org promote, Ops upgrades) shipped; [BP-010](../../backlog/BP-010-three-api-families.md) is mitigated.

When adding or moving a route, update the matching `docs/api/{family}.md` page — not this file, and not by pasting the overview five times.
