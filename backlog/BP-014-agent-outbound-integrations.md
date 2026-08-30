# BP-014: Customer outbound integrations (connectors, secret refs, agent skills)

- **Severity:** Medium
- **Status:** Partially mitigated (automation outbound, `allowedSkills` grants, Govern connector catalog, and **`invoke_skill` on MCP + hosted loop verified shipped**; Deploy skill-name validation + happy-path tests + skill-job-class DX landed. **Keep** `invoke_skill`. Status stays Partially mitigated until an owner accepts Deferred items as BP-047/033 follow-ons.)
- **Track:** Keep (`invoke_skill` regression guard). Control IDE Govern catalog chrome is frozen. Hosted multi-tool loop is [BP-006](./BP-006-agent-guardrails.md) mitigated — do not reopen. Remainder design: [03-bp-014-skill-invoke.md](../docs/architecture/agentic-remainders/03-bp-014-skill-invoke.md).
- **Area:** `internal/automation`, `internal/egress`, `internal/worker`, `internal/httpapi` (Metadata), `internal/deploy`, `internal/mcp`, `internal/agentloop`, `migrations/`
- **Design:** [outbound-otel-build-plan.md](../docs/architecture/outbound-otel-build-plan.md) · [03-bp-014-skill-invoke.md](../docs/architecture/agentic-remainders/03-bp-014-skill-invoke.md) (remainder) · [ADR-010](../docs/adr/010-customer-agentic-platform.md) · [ADR-014](../docs/adr/014-customer-code-automations.md)

## Problem

Customer AgentSpecs and MCP-over-API let external runtimes call Majesta One under AuthZ. Customers also need **outbound** HTTPS from async automations (and later agents) without baking secrets into product Git, giving Deno network, or inventing a parallel AuthZ plane.

## Why it matters

Without a documented config surface, teams hard-code endpoints in instructions or fork the product. Buyers expect BYO / VPC endpoints and allowlisted egress for automation and agent workloads.

## Shipped

1. Install **secret refs** + **connectors** + **egress allowlist** (Metadata; secrets install-local; Deploy refs only)
2. Async Deno SDK `ctx.http` / `ctx.connector` via Go host RPC (guest stays deny-net; sync rejected)
3. AgentSpec **`allowedSkills`** — grant named automation functions as skills (Metadata create/PATCH + worker existence check + **Deploy name-check** against bundle automations or install `metadata_automations`)
4. Shared OTEL spans for outbound calls ([BP-008](./BP-008-production-packaging.md))
5. Control IDE **Govern → Integrations** catalog for static-secret and OAuth connectors, with guided secret binding, egress allowlisting, OAuth connect/status, and installed-connector management (**frozen** — no new chrome)
6. **`invoke_skill`** on MCP (`internal/mcp`) and the hosted loop (`internal/agentloop` via `mcp.CallTool`) — deny when not in `allowedSkills` or PS `canRun`; enqueue `automation.run` as Client invoke. Verified 2026-08 against deny tests plus happy-path enqueue (`TestMCPInvokeSkillEnqueuesAutomationRun`, `TestHostedLoopInvokeSkillEnqueuesAutomationRun`). Metadata unknown `allowedSkills` is 400 (`TestMetadataPlaybookUnknownAllowedSkillRejected`).

## Remainder (do not mark Mitigated until an owner accepts deferred items as follow-ons)

Phases 1–4 of [03-bp-014-skill-invoke.md](../docs/architecture/agentic-remainders/03-bp-014-skill-invoke.md) landed (docs retarget, happy-path + Metadata 400 tests, Deploy `allowedSkills` name-check, `skills/skill` DX). Status stays **Partially mitigated** until an owner accepts OAuth ExecutionRun as BP-047/033 and sync outbound as never (see Deferred). This item does not auto-flip Mitigated.

## Deferred

- AgentSpec LLM base URL / provider keys → [BP-052](./BP-052-customer-inference.md) (not this item)
- Sync automation outbound (forbidden by ADR-014)
- Connector OAuth / Client HTTP invoke remainders → [BP-047](./BP-047-integrations-callable-oauth.md); ExecutionRun projection → [BP-033](./BP-033-customer-runtime-isolation.md)
- Hosted agent multi-tool execution is **done** ([BP-006](./BP-006-agent-guardrails.md) mitigated). The connector registry does not widen AgentSpec execution authority.

## Explicit non-goals

- Blocking MCP-over-API or customer AgentSpec CRUD on this item
- Customer npm/`fetch` inside Deno
- Replacing BP-033 customer debug objects with OTEL

## Related

- [BP-006](./BP-006-agent-guardrails.md) — hosted tool execution as run actor (**mitigated**; do not reopen)
- [BP-008](./BP-008-production-packaging.md) — operator OTEL
- [ADR-010](../docs/adr/010-customer-agentic-platform.md) · [ADR-014](../docs/adr/014-customer-code-automations.md)
- [customer-agents.md](../docs/customer-agents.md) · [automation-sdk.md](../docs/automation-sdk.md)
- Remainder plan: [03-bp-014-skill-invoke.md](../docs/architecture/agentic-remainders/03-bp-014-skill-invoke.md)
