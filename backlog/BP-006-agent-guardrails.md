# BP-006: Customer customization AuthZ + hosted tool loop

- **Severity:** High
- **Status:** Mitigated — AuthZ + hosted `/agents/runs` loop shipped
- **Area:** `internal/agentloop`, `internal/httpapi`, `internal/worker`, `internal/mcp`, `internal/authz`

## Product rules (keep)

1. Three principal kinds: `user` | `service` | `agent`.
2. Parity for tenant-allowed actions — Roles + permission sets, not a parallel agent ACL.
3. Critical checks in Go on every mutating path (ownership + grants + playbook allowlists).
4. Grow Client / Metadata / Deploy under that model; do not invent agent-only permissions.

## Outcome

Hosted `/client/v1/agents/runs` executes admitted MCP tools as the run actor. Writes park at `awaiting_tool_approval`. Builder Metadata/Deploy jobs stay on MCP / family HTTP, not the hosted v1 catalog.

Create, approve, and worker resume fail closed if the live playbook cannot be loaded. Run detail and SSE are limited to the run actor, admins, or `govern.agents`.

## Do not reopen

Richer approval matrices, hosted Metadata upsert / Deploy `org_*` from runs, and inbound graph curation are follow-ups — not blockers. External MCP recipes: [customer-connect.md](../docs/customer-connect.md). Skill invoke remainders: [BP-014](./BP-014-agent-outbound-integrations.md).

## Related

- [hosted-agent-tool-loop-build-plan.md](../docs/architecture/hosted-agent-tool-loop-build-plan.md) · [customization-authz.md](../docs/architecture/customization-authz.md) · [ADR-010](../docs/adr/010-customer-agentic-platform.md) · [ADR-030](../docs/adr/030-install-agent-runtime.md)
