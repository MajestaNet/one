# Module: agents_starter

Always-on package that clones day-one **AgentSpecs** onto the install as **customer-owned** rows so admins can fully edit and Deploy-promote them. Seeded with `AUTO_SEED` alongside `core` — not an optional enable.

Customers can define additional AgentSpecs anytime via Metadata / Deploy; this package only seeds starter playbooks. Each starter binds a **job class** (`query|customize|ship|govern|operate|skill`) with a Majesta One-managed harness floor ([BP-064](../backlog/BP-064-install-agent-runtime.md)). `primarySection` remains a compatibility alias for optional Control IDE docks ([BP-053](../backlog/BP-053-agent-section-harness.md)). Stored tokens `run` / `ship` are required by BindSpec (`operate`↔`run`, `ship`↔`ship`); four-tile docks alias them to Operate / Build.

## What lands

| AgentSpec `apiName` | Primary section | Job class | Purpose | Notes |
|---|---|---|---|---|
| `AdminSetup` | `govern` | `govern` | First-week admin setup assistance | `harness.govern.admin`; `requireApproval=true` |
| `MetadataBuilder` | `build` | `customize` | Customer metadata design assistance | `harness.customize.metadata`; Metadata-oriented instructions |
| `RunCoach` | `run` | `operate` | Personal graph organization + declarative Tool coaching | Compat section `run`; job class `operate`; hosted loop ignores `graphCalls` |
| `ShipGuide` | `ship` | `ship` | Validate / promote readiness guidance | `harness.ship.release`; Ship via `one` / MCP `org_*` |
| `AccountGuide` | `settings` | `govern` | Account / Hosting / Inference orientation | `harness.settings.install` until catalog merge |

Clone semantics ([ADR-010](../adr/010-customer-agentic-platform.md)):

- Inserted with `ownership=custom`, `package_name=customer.default`, plus `primary_section` / `job_class` / `harness_id` / `harness_version`
- `ON CONFLICT (api_name) DO NOTHING` — never overwrites customer edits
- Product image holds **templates** only; live rows are customer implementation
- Package version **1.3.4** retunes newly cloned ShipGuide / AccountGuide / RunCoach instructions for CLI + hosted-loop language (existing customer-owned rows remain untouched). Version **1.3.3** records `jobClass` on newly cloned starters (BP-064). Version **1.3.2** adds `search` to AdminSetup `allowedTools` (Client find). Version **1.3.1** teaches newly seeded RunCoach rows to emit a `oneEffects` JSON fence for graphCalls / proposals (BP-056 remediation). Version **1.3.0** added Curator / Doer / Publisher roles. Existing customer-owned rows remain untouched. Version 1.2.0 added the reference-only `graph.*` IDE bridge.

Run graphs remain principal-private war rooms. Org reuse happens through published ToolSpecs, never shared personal graph ACLs. Hosted inbound event→graph curation is **not** required to close [BP-006](../backlog/BP-006-agent-guardrails.md); it stays Operate-adjacent / frozen chrome. Hosted `/agents/runs` tool execution is [hosted-agent-tool-loop-build-plan.md](../architecture/hosted-agent-tool-loop-build-plan.md).

## Recommended principal

Create `principal_type=agent`, assign Roles (e.g. `MetadataDeveloper` for MetadataBuilder; Client-capable Role for AdminSetup), assign permission sets, issue credentials. Do not log plaintext secrets.

## Related

- [customer-agents.md](../customer-agents.md)
- [architecture/agent-section-harness-build-plan.md](../architecture/agent-section-harness-build-plan.md)
- [modules/README.md](./README.md)
