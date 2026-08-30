# ADR-030: Install as agent runtime (Control IDE optional)

## Status

Accepted (amended: Control IDE is optional and may be refactored to clean the install — [BP-065](../../backlog/BP-065-ide-backend-coupling.md))

## Context

Majesta One is API-first ([docs/architecture.md](../architecture.md)): the product runtime has **no embedded UI**. Control IDE under `tools/control-ide` is a vendor-plane JWT client ([ADR-012](./012-customer-repo-and-control-ide.md)). Customer agents are already principals that call Client/Metadata (or the install-local MCP adapter) under the same AuthZ model ([ADR-010](./010-customer-agentic-platform.md)).

Builder environments (coding agents) now own the **edit files / run tests / open PRs / call tools** job. Binding the product wedge to Control IDE launcher sections ([BP-053](../../backlog/BP-053-agent-section-harness.md)) and hosting agents inside Electron races those tools and couples harness IP to chrome customers can skip.

End users still need apps. That path is customer-hosted Client Experiences ([ADR-019](./019-client-experience-oss-kits.md)), not a second admin IDE.

## Decision

### 1. The install is the product

The Go install is the **governed agent runtime**: AuthZ, metadata, Deploy/Ship, AgentSpec, job-class harnesses, skills, hosted tool loop, inference routing, MCP gateway. Coding agents, CI, bots, `one`, and any optional UI are **clients**.

Do not treat Control IDE as a coding-agent host. Do not treat Control IDE docks as where agents “live.”

### 2. Three surfaces, one AuthZ model

| Surface | Who | How they talk to the install |
|---|---|---|
| **Builder DX** | Admins, SIs, coding agents | MCP (`POST /mcp`), family HTTP, `one` CLI, customer Git (`one/v1`) |
| **End-user DX** | Business users in browser/mobile | Client Experience kits + Connected Apps (`/auth/v1` + `/client/v1` only) — [ADR-019](./019-client-experience-oss-kits.md) |
| **Optional Control IDE** | Humans who prefer a desktop client | Thin JWT client of the same APIs. Refactor the client when that removes install coupling ([BP-065](../../backlog/BP-065-ide-backend-coupling.md)). Do not add Electron-only product capability (in-IDE coding-agent host, license as install gate, Operate as end-user CRM) |

### 3. Harnesses bind to job classes, not IDE tiles

Product-owned harness floors remain the wedge ([BP-053](../../backlog/BP-053-agent-section-harness.md) delivered section floors). The SoR for new work is **job class** (`query` \| `customize` \| `ship` \| `govern` \| `operate` \| `skill`), not launcher tiles. `primarySection` stays a compatibility alias. Design: [agent-runtime-build-plan.md](../architecture/agent-runtime-build-plan.md) · [BP-064](../../backlog/BP-064-install-agent-runtime.md).

### 4. MCP and the hosted loop share one tool contract

MCP remains an adapter, not a fourth API family ([ADR-010](./010-customer-agentic-platform.md)). Shared contract: **MCP tool names** + `mcp.CallTool` AuthZ. Builder jobs (Metadata CRUD, pack/validate/deploy vs org, skill/action invoke) are reachable from MCP hosts. Hosted `/client/v1/agents/runs` executes a **v1 subset** (Client read + gated write + `invoke_skill` / `invoke_action`) per [hosted-agent-tool-loop-build-plan.md](../architecture/hosted-agent-tool-loop-build-plan.md) / [BP-006](../../backlog/BP-006-agent-guardrails.md). Metadata upserts, Deploy `org_*`, and `install_version` stay on MCP / family HTTP in v1 — they are builder jobs, not hosted-run jobs. Deploy/Ops stay out of MCP until the matching HTTP path exists; then the adapter may project them. Ops **mutate** stays out of MCP v1 (read-only install/version is allowed).

### 5. Control IDE is optional — refactor it when that cleans the install

Control IDE is an optional JWT client, **not** the GA product path. New Electron-only product capability (license onboarding, private update CDN, in-IDE coding-agent host, Operate as the end-user CRM) stays out.

The client is **not frozen against cleanup**. Prefer changing `tools/control-ide` when that lets the Go install drop IDE-shaped AuthN, chrome-only Client routes, `ide.*` caps, or Electron Apply coaching ([BP-065](../../backlog/BP-065-ide-backend-coupling.md) · [ide-backend-coupling-review.md](../architecture/ide-backend-coupling-review.md)). Do not keep kernel tables or mint defaults “because the IDE still calls them” if the IDE can be updated in the same change set.

Ship for builders is `one` + Deploy API ([BP-048](../../backlog/BP-048-one-cli.md)), not Control IDE Ship panels.

### 6. Licensing and OSS

The entire repository is Apache-2.0. Control IDE may remain unused for GA; do not block OSS install, MCP, or CLI on IDE entitlement. One public product repo stays the default ([monorepo.md](../monorepo.md)).

## Consequences

- Priority among open work favors BP-006, BP-064, BP-048, BP-014 (skills invoke), BP-052 (install inference SoR), BP-040, plus identity/distribution already on the GA path — not Operate UX or IDE commercial distribution.
- Starter AgentSpecs keep `primarySection` for compat; new docs and Bind paths speak job class.
- Control IDE playbook applies when a task touches `tools/control-ide` — including lockstep refactors that remove install coupling (BP-065).
- Vendor domain agents (`AGENTS.md`, `.cursor/`) are not customer runtime agents; customer agents are install principals.

## Related

- [ADR-004](./004-three-api-families.md) · [ADR-010](./010-customer-agentic-platform.md) · [ADR-012](./012-customer-repo-and-control-ide.md) · [ADR-019](./019-client-experience-oss-kits.md)
- [agent-runtime-build-plan.md](../architecture/agent-runtime-build-plan.md) · [hosted-agent-tool-loop-build-plan.md](../architecture/hosted-agent-tool-loop-build-plan.md) · [ide-backend-coupling-review.md](../architecture/ide-backend-coupling-review.md) · [BP-064](../../backlog/BP-064-install-agent-runtime.md) · [BP-065](../../backlog/BP-065-ide-backend-coupling.md) · [BP-006](../../backlog/BP-006-agent-guardrails.md)
- [customer-agents.md](../customer-agents.md) · [customer-connect.md](../customer-connect.md)
