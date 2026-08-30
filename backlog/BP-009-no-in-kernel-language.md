# BP-009: Customer domain logic — sandboxed code automations

- **Severity:** Medium
- **Status:** Mitigated (Phases 0–7 landed: AuthZ grants, Deno guest, SDK/Deploy tests, Automations panel, async `ctx.http`/`ctx.connector`. npm allowlist is a non-goal. Customer debug objects stay [BP-033](./BP-033-customer-runtime-isolation.md).)
- **Area:** `internal/automation`, `internal/authz`, `internal/dataengine`, `internal/worker`, `internal/deploy`, `tools/control-ide`
- **Design:** [ADR-014](../docs/adr/014-customer-code-automations.md) · [customer-automations-build.md](../docs/architecture/customer-automations-build.md)
- **Remainder:** [12-bp-008-026-009-047-ops-automations.md](../docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md)

## Problem

v1 avoided a proprietary in-kernel scripting language. JSON `actions` automations + agents + webhooks do not cover deterministic multi-step customer domain flows (pricing, create-related with mappings, reservations). Without a deliberate extension model, logic leaks into sidecars or unversioned scripts.

## Why it matters

Customers need real, testable automation code without forking the product or weakening AuthZ / kernel isolation.

## Direction (locked)

Per **ADR-014**:

1. **TypeScript guest programs** in the customer repo; Build agent chat (or hand-write) — not a drag-and-drop SoT
2. **Deno default-deny** executor; never shares kernel privileges
3. **No third-party libraries in v1** (no npm/JSR/std/URL imports — including Build-agent output); only `one:automation` / ambient `ctx`
4. **Permission sets** hold an automation grant list (`automationAccess` / `allAutomations`)
5. **Run-as = starter** (schedules declare `runAsPrincipalId`)
6. **Sync = same DB transaction**, fail → full rollback; **async** default via platform jobs
7. Unit tests + Deploy test gates before activate/promote

## Implementation phases

See [customer-automations-build.md](../docs/architecture/customer-automations-build.md) Phases 1–7.

| Phase | Status |
|---|---|
| 0 Docs / ADR-014 | Done |
| 1 AuthZ automation grants on PS | Done |
| 2 Code automation metadata + pack/import ban | Done |
| 3 Sync transactional path (abort + rollback) | Done |
| 4 Deno sandbox executor (async + sync) | Done |
| 5 SDK + unit harness + Deploy test steps | Done |
| 6 Control IDE Automations panel (Monaco TS hand-write) | Done (agent write-loop still BP-006; Manual/Client invoke → [BP-047](./BP-047-integrations-callable-oauth.md)) |
| 7 Async outbound connectors | Done (`ctx.http` / `ctx.connector`; sync outbound forbidden — [BP-014](./BP-014-agent-outbound-integrations.md)) |

## Remaining follow-ups (not this BP)

- Customer `ExecutionRun` / `ExecutionLogEntry` debug projection — [BP-033](./BP-033-customer-runtime-isolation.md) Phase 3
- npm allowlists — follow-up ADR only (explicit non-goal)

## Explicit non-goals

- Inventing a proprietary in-kernel language
- Declarative field-map DSL as primary authoring
- npm allowlists before a follow-up ADR
- Go plugins / Python guest / importing `internal/*` from customer code

## Related

- Remainder design: [12-bp-008-026-009-047-ops-automations.md](../docs/architecture/agentic-remainders/12-bp-008-026-009-047-ops-automations.md)
- [ADR-014](../docs/adr/014-customer-code-automations.md)
- [ADR-005](../docs/adr/005-go-runtime.md) — platform remains Go; guest TS is customer implementation
- [ADR-010](../docs/adr/010-customer-agentic-platform.md) — Build AgentSpec
- [BP-006](./BP-006-agent-guardrails.md) — hosted agent tool loop
- [BP-014](./BP-014-agent-outbound-integrations.md) — async outbound connectors
