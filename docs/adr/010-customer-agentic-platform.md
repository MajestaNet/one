# ADR-010: Customer agentic platform (AgentSpec, MCP-over-API, plane separation)

## Status

Accepted

## Context

Customers need day-one agentic workflows (definitions, principals, external agent runtimes via MCP) without inventing a parallel AuthZ system or confusing Majesta One **vendor/dev** agents (`docs/`, `.cursor/`, `AGENTS.md`) with **customer runtime** agents on an install.

Playbook definitions already live on Metadata; runs on Client ([ADR-004](./004-three-api-families.md)). BP-006 requires principal parity and server-side enforcement. Hosted `agent.run` tool execution remains incomplete; external agents can still act by calling Majesta One APIs under their own credentials.

## Decision

### 1. AgentSpec is Metadata

`agent_playbooks` rows are **AgentSpecs**: customer-customizable definitions with `instructions` (system prompt / agent.md body), `goal_template`, `allowed_tools`, `object_scopes`, `require_approval`, plus `ownership` / `package_name` like other metadata.

- Customers CRUD AgentSpecs via `/metadata/v1/agents/playbooks` (`metadata.customize`).
- Mutate/delete only when `ownership=custom`.
- Customer-owned AgentSpecs promote via Deploy bundles (same as automations).

### 2. MCP is an adapter, not a fourth API family

An install-local MCP gateway (same process as `cmd/api`) projects family HTTP tools over **Streamable HTTP** at `POST /mcp` (**stateless JSON** responses in v1; no SSE sessions). Every tool maps to an existing HTTP family path. AuthN/AuthZ stay in Go: family scopes, system capabilities, object/FLS. MCP invents no capabilities (e.g. “describe object” is Client `GET /describe/{object}` or Metadata `GET /objects/{apiName}` depending on the tool).

v1 catalog is Client/Metadata (+ agent runs). **Builder** projections (pack/validate/deploy, `invoke_skill` / `invoke_action`) land under [BP-064](../../backlog/BP-064-install-agent-runtime.md) / [ADR-030](./030-install-agent-runtime.md) only when the HTTP path already exists. **Ops mutate** stays out of MCP v1. MCP is gated by `FEATURE_FLAGS` including `agents`. Optional customer-hosted stdio / custom tools live under vendor-plane [`tools/one-mcp`](../../tools/one-mcp/) — never in the product image. Connect recipes: [customer-connect.md](../customer-connect.md) · [builder-connect.md](../builder-connect.md).

### 3. Starter agents: always-on clone

Always-on managed package `agents_starter` holds **templates** in product seed. On `AUTO_SEED` (and product upgrades), templates are **cloned** to `ownership=custom` AgentSpecs if the `api_name` is missing. Clone never overwrites customer edits. Customers fully modify clones, define additional AgentSpecs anytime via Metadata, and Deploy-promote them.

### 4. Plane separation (nouns)

| Plane | Nouns | Ships in product image? |
|---|---|---|
| Vendor/dev | `AGENTS.md`, domain agents, architecture playbooks | No |
| Product | AgentSpec schema, MCP gateway, starter **templates** | Yes |
| Customer implementation | Customer AgentSpecs, agent principals, MCP credentials | Install DB only |

Managed starter **instructions** must not reference vendor paths (`.cursor/`, `BP-*`, `internal/`).

### 5. Out of scope (deferred)

Hosted in-process LLM tool loop → [BP-006](../../backlog/BP-006-agent-guardrails.md) / [hosted-agent-tool-loop-build-plan.md](../architecture/hosted-agent-tool-loop-build-plan.md) (shipped).  
Outbound connectors / secret refs / egress / AgentSpec skills → [BP-014](../../backlog/BP-014-agent-outbound-integrations.md) ([outbound-otel-build-plan.md](../architecture/outbound-otel-build-plan.md)). BYO LLM provider keys on AgentSpec remain deferred with BP-006.

## Consequences

- External agent runtimes connect via MCP + Majesta One JWT/API key as `principal_type=agent|service|user`.
- Product docs for customers use AgentSpec / principal / MCP; vendor agent routing docs stay separate.
- Marketplace continues to recommend omitting `agents` from `FEATURE_FLAGS` until BP-006 tool execution is complete; MCP stays dark without the flag. Builder connect is documented in [builder-connect.md](../builder-connect.md) even while the hosted loop is incomplete — external MCP still requires the flag.

## Amendment — install is the runtime (2026-08)

[ADR-030](./030-install-agent-runtime.md) locks the product wedge on the **install** (job-class harnesses, skills, hosted loop, builder MCP), not Control IDE docks. MCP may project Deploy pack/validate/apply when those HTTP paths exist ([BP-064](../../backlog/BP-064-install-agent-runtime.md)); it still invents no capabilities. Hosted in-process tool loop remains [BP-006](../../backlog/BP-006-agent-guardrails.md); execution spec is [hosted-agent-tool-loop-build-plan.md](../architecture/hosted-agent-tool-loop-build-plan.md) (v1 catalog is Client tools; Metadata/Deploy stay on MCP / family HTTP). Coding agents and bots are first-class Path C clients ([builder-connect.md](../builder-connect.md)).

## Related

- [ADR-004](./004-three-api-families.md) · [ADR-005](./005-go-runtime.md) · [ADR-006](./006-jwt-auth.md) · [ADR-030](./030-install-agent-runtime.md)
- [customization-authz.md](../architecture/customization-authz.md) · [customer-agents.md](../customer-agents.md) · [customer-connect.md](../customer-connect.md)
- [agent-runtime-build-plan.md](../architecture/agent-runtime-build-plan.md) · [hosted-agent-tool-loop-build-plan.md](../architecture/hosted-agent-tool-loop-build-plan.md)
