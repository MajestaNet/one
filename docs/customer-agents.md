# Customer agents (customer-facing)

How to run **agentic workflows inside a Majesta One install**. This is not the vendor/dev agent plane (`AGENTS.md`, `.cursor/`, architecture playbooks). **Builder** agents (coding agents) connect to the **same install** via MCP/HTTP — they do not live in Control IDE ([ADR-030](./adr/030-install-agent-runtime.md) · [builder-connect.md](./builder-connect.md)).

See [ADR-010](./adr/010-customer-agentic-platform.md) for product decisions. Connect recipes: [customer-connect.md](./customer-connect.md). Runtime design: [agent-runtime-build-plan.md](./architecture/agent-runtime-build-plan.md). Hosted run execution: [hosted-agent-tool-loop-build-plan.md](./architecture/hosted-agent-tool-loop-build-plan.md).

## Concepts

| Noun | Meaning |
|---|---|
| **AgentSpec** | Metadata definition (`/metadata/v1/agents/playbooks`) — instructions, goal template, tool/object allowlists, approval |
| **Agent principal** | `users` row with `principal_type=agent`, Roles + permission sets, Majesta One JWT / client credential |
| **Agent run** | Client execution (`/client/v1/agents/runs`) of an AgentSpec |
| **MCP gateway** | Install-local Streamable HTTP adapter at `POST /mcp` that projects Client/Metadata/Deploy builder tools under the same AuthZ ([customer-connect.md](./customer-connect.md)) |

## Day-one path

1. Ensure `FEATURE_FLAGS` includes `agents` (MCP stays dark without it; Marketplace often omits until hosted tool execution is complete — [BP-006](../backlog/BP-006-agent-guardrails.md)).
2. With `AUTO_SEED`, starter AgentSpecs (**AdminSetup**, **MetadataBuilder**, **RunCoach**, **ShipGuide**, **AccountGuide**) are cloned as **customer-owned** rows with a required **primary section** (compat) + Majesta One harness floor (fully editable; re-seed never overwrites). New work binds harnesses to **job class** (`query|customize|ship|govern|operate|skill`) — [BP-064](../backlog/BP-064-install-agent-runtime.md). Customers may create more AgentSpecs via Metadata anytime (Control IDE wizard is optional/frozen).
3. Create an agent principal (`POST /client/v1/principals` with `principalType=agent`), assign Roles/permission sets, issue a credential.
4. Connect an external agent runtime to `POST /mcp` with that credential (or call Client/Metadata HTTP directly). Client config snippets: [customer-connect.md](./customer-connect.md).
5. Example: `describe_object` then `create_record` — both succeed only if the principal has Client scope and object create grants.

Customers do **not** author the product MCP server. Optional custom vertical tools use the TypeScript scaffold under [`tools/one-mcp`](../tools/one-mcp/) (customer-hosted; vendor plane).

## Custom AgentSpecs

Customers create/update/delete AgentSpecs via Metadata (`metadata.customize`). Fields include `instructions` (system prompt / agent.md body), **`primarySection`** (compat alias for Control IDE docks; required on create unless `jobClass` is set — [BP-064](../backlog/BP-064-install-agent-runtime.md)), optional **`jobClass`**, and server-managed `harnessId` / `harnessVersion`. Customer-owned specs promote via Deploy like other customization. Moving an agent to another section or job class re-applies that harness floor.

## Skills (automation grants)

AgentSpecs may list **`allowedSkills`**: automation `api_name`s the agent is permitted to use as skills. Execution still requires the agent principal’s permission-set `automationAccess` / `canRun`. **`invoke_skill`** on hosted runs and MCP is shipped (live playbook grant ∩ PS `canRun`; enqueue `automation.run`). Outbound HTTPS from those automations is this item’s connector/egress surface ([outbound-otel-build-plan.md](./architecture/outbound-otel-build-plan.md); already shipped). Deploy validate fails closed if `allowedSkills` names a typo; `one project init` writes `skills/skill/SKILL.md` ([BP-014](../backlog/BP-014-agent-outbound-integrations.md) · [03-bp-014-skill-invoke.md](./architecture/agentic-remainders/03-bp-014-skill-invoke.md)).

## Inference + harness

Outbound LLM / BYO model configuration: [BP-052](../backlog/BP-052-customer-inference.md) ([inference-build-plan.md](./architecture/inference-build-plan.md)) — Settings → Inference, Deploy Native DO modes, streaming on `/client/v1/agents/runs`. **Generation is not an approval event:** streaming chat runs the LLM immediately (`approved: false` still recorded). `requireApproval` parks **writes** on the hosted loop at `awaiting_tool_approval` ([hosted-agent-tool-loop-build-plan.md](./architecture/hosted-agent-tool-loop-build-plan.md)); Control IDE Apply still handles `graphCalls` / `proposal` (the hosted executor ignores them). JSON `POST .../approve` queues a worker job with `resume`; SSE approve continues in-process and must not also enqueue.

**Hosted tool loop:** a Client run executes admitted MCP tools as the reconstructed run actor (`query` / `get_record` immediately; `create_record` / `update_record` / `invoke_skill` / `invoke_action` after approve when required). That is how an in-product agent does AgentSpec work without an external MCP host in the middle. External MCP (`POST /mcp`) is a different path — builders already hit HTTP. Metadata upserts and Deploy `org_*` stay on MCP / family HTTP in v1, not on `/agents/runs`.

**Job-class harness (Phases 1–2):** every AgentSpec may bind `jobClass` (`query|customize|ship|govern|operate|skill`). Creating an AgentSpec accepts `jobClass` XOR `primarySection` and fills the other. Run-time `Apply` uses the job-class catalog when `jobClass` is set; otherwise the section catalog so existing `primarySection` YAML still applies. Customers may widen tools/skills within AuthZ but **cannot PATCH away the floor**. Builder MCP (`invoke_action`, `invoke_skill`, Metadata upserts, `org_validate` / `org_deploy` / `pack` / `org_retrieve`, `install_version`) is an adapter over existing family HTTP — [BP-064](../backlog/BP-064-install-agent-runtime.md) · [agent-runtime-build-plan.md](./architecture/agent-runtime-build-plan.md). Agents may still appear in a matching IDE dock if that client is used; builders do not need the IDE.

## Related

- [customer-connect.md](./customer-connect.md) — Paths A/B/C (UI JWT · service · MCP)
- [builder-connect.md](./builder-connect.md) — builder MCP + CLI
- [architecture/agent-runtime-build-plan.md](./architecture/agent-runtime-build-plan.md)
- [architecture/hosted-agent-tool-loop-build-plan.md](./architecture/hosted-agent-tool-loop-build-plan.md) — hosted `/agents/runs` execution (BP-006 remaining)
- [customer-customizations.md](./customer-customizations.md)
- [modules/agents-starter.md](./modules/agents-starter.md)
- [api-families.md](./api-families.md)
- [architecture/customization-authz.md](./architecture/customization-authz.md)
- [tools/one-mcp](../tools/one-mcp/) — optional customer MCP scaffold
