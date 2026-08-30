---
name: one-ship
description: Validate and deploy a one/v1 repo versus a connected install. Use when packing, org validate, org deploy, or retrieve.
---

# Ship

Job class: `ship`. Path of record:

```bash
one org validate -dir .
one org deploy -dir .
```

Same Deploy SoR as MCP `org_validate` / `org_deploy` / `pack` / `org_retrieve` (`deploy` scope + promote capability). `one org deploy --suite` runs customer tests **after** apply so a first deploy can create the suite. Always approve `org_deploy`.

## Rules

- Repo → org only. Do not peer-promote bundles from one install to another.
- Validate until green, then deploy. `--dry-run` on deploy does not mutate.
- Ops image rolls are **not** MCP tools. Do not call `/ops/v1` mutate from builder agents.
- Hosted `/agents/runs` does **not** execute Deploy `org_*` in v1.
