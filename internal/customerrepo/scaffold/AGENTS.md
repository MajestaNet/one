# Customer implementation agents

This repository is a **one/v1** customer implementation: metadata, automations, tests, and AgentSpecs for one commercial customer. It is not Majesta One product source.

Builders (coding agents, CI, humans) talk to the **install**, not to an in-IDE agent host.

## Connect

1. Claim or sign in to the install; create an `agent` or `service` principal with the Roles you need (`client`, `metadata`, `deploy`).
2. Point an MCP host at `POST /mcp` with `Authorization: Bearer <one_jwt>` and `One-API-Revision` from `GET /version`.
3. Prefer the install MCP. Stdio scaffolds are a fallback when the host cannot speak Streamable HTTP.

See `skills/connect/SKILL.md`.

## Job classes

AgentSpecs bind a **job class** (`query` | `customize` | `ship` | `govern` | `operate` | `skill`). `primarySection` is a compatibility alias. Customers may widen tools/skills within AuthZ; they may **not** drop the harness floor.

| Job class | Typical work | Skill |
|---|---|---|
| `query` | Describe / query / search | `skills/query/SKILL.md` |
| `customize` | Customer metadata upserts | `skills/customize/SKILL.md` |
| `ship` | Validate and deploy vs org | `skills/ship/SKILL.md` |
| `govern` | Principals, permission sets, install policy | `skills/govern/SKILL.md` |
| `skill` | Invoke named automations in `allowedSkills` | `skills/skill/SKILL.md` |
| `operate` | Record mutate + platform actions | Hosted loop write tools (no dedicated skill file) |

## Ship

```bash
one auth login --base-url https://<install> --token "$ONE_JWT" --alias test
one org validate -dir .
one org deploy -dir .
```

`org validate` then `org deploy` is the Ship path. MCP `org_validate` / `org_deploy` / `pack` are the same Deploy APIs. Do not peer-promote between installs.

## Rules

- AuthZ is install-local. `401`/`403` means the principal lacks grants.
- Do not mutate `ownership=managed` metadata.
- Do not put secrets, API keys, or business records in Git.
- Browser Client Experience apps stay `/auth/v1` + `/client/v1` only — no Metadata or Deploy from the browser.
- Hosted `/client/v1/agents/runs` executes Client read/write + skill/action invoke. Metadata upserts and Deploy `org_*` stay on MCP / family HTTP / this CLI.
