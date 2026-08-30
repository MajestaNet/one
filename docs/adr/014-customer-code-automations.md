# ADR-014: Customer code automations (Deno sandbox, PS grants, sync rollback)

## Status

Accepted (plan locked; implementation phased — see [customer-automations-build.md](../architecture/customer-automations-build.md))

## Context

Declarative JSON automations and AgentSpecs are not enough for deterministic customer domain logic (e.g. create Opportunity from Account with customer-defined mapping). Customers need **real code**, authored primarily via an interactive Build agent (or hand-written), with unit tests and Deploy gates. Product constraints:

- Product ≠ customer implementation ([customer-customizations.md](../customer-customizations.md))
- Platform runtime stays Go-only ([ADR-005](./005-go-runtime.md)); guest code must not share address space privileges with the kernel
- One AuthZ model ([ADR-009](./009-record-audit-authz-packaging.md), [customization-authz.md](../architecture/customization-authz.md))
- BP-009 tracked pressure for a proprietary in-kernel escape hatch; this ADR chooses a **sandboxed guest program** model instead

## Decision

### 1. Automations are TypeScript programs, not a drag-and-drop SoT

- Source of truth: customer repo (`one/v1`) + Metadata definition (trigger, execution mode, bindings).
- Authoring: interactive **Build** AgentSpec + LLM chat generates/edits real `.ts` + tests; humans/external tooling may write the same files.
- Visualization (call graph / promote diff) is **review-only**, never the editor SoT.

### 2. Runtime: Deno, default-deny

- Guest engine: **Deno** with default-deny (no net/fs/env/FFI except the injected Majesta One bridge).
- Platform worker/API invokes the sandbox; customer code never imports product Go packages or opens the install DB DSN.

### 3. Zero third-party libraries (v1 hard ban)

- **No** npm, JSR, URL, or Deno `std` imports — including code produced by Build agents.
- Allowed module graph: **only** the virtual module `one:automation` (types + SDK injected by the runtime). Prefer ambient `ctx` with **zero** import statements when possible; if an import appears, static analysis must allow **only** `one:automation`.
- Reject `require`, dynamic `import()`, `eval`, `Function`, workers, WASM from URLs, and any bare-specifier resolution.
- Rationale: supply-chain risk, reviewability, determinism, and preventing agents from “helpfully” pulling lodash/axios. Revisit only via a later ADR with an explicit allowlist process.

### 4. Permission sets grant automation access

- A permission set may include an **automation list** (and optional `allAutomations`), same bag as object/field grants.
- Creating/editing that list = Metadata PS edit (`authz.manage`). Assigning the PS to a user grants **start/run** of those automations.
- New automations get **deny stubs** on every PS (catalog pattern like objects); Admin / `allAutomations` is the broad grant.

### 5. Run-as = starter principal

| Start path | Run-as |
|---|---|
| Record write triggers automation | Writing actor |
| Manual / API invoke | Invoking principal |
| Schedule / timer | Explicit `runAsPrincipalId` on the definition (required) |

Data AuthZ for side effects = that principal’s object/field grants. No hidden Automation Admin elevation.

### 6. Sync vs async (one programming model)

Customer export shape (only supported entry):

```ts
export default async function run(ctx: AutomationContext): Promise<AutomationResult>
```

| Mode | Behavior |
|---|---|
| `execution: async` (default) | Enqueue `automation.run`; retries/DLQ are platform-owned |
| `execution: sync` | Runs in the **same Postgres transaction** as the triggering write; **any failure aborts and rolls back** the whole unit of work |

Sync forbids outbound HTTP/email/connectors (non-rollbackable). Nested sync depth is capped; cycles fail closed.

### 7. SDK surface (frozen, platform-owned)

`ctx` exposes only: record get/create/update/delete/query (AuthZ-enforced as run-as), log, (async only) allowlisted connector calls via platform config, and **`invokeAction`** for product [platform actions](./029-platform-actions.md) (`lead.convert`, later `record.merge`, …). Do **not** add per-verb methods (`ctx.convertLead`). No customer-facing queues, futures frameworks, or event buses — jobs/outbox stay inside Majesta One.

### 8. Tests and Deploy

- Unit tests run in the same Deno deny-all harness (mock `ctx`).
- Contract/smoke suites on the install gate promote (`/deploy/v1/tests`) and post-upgrade smoke.
- Automations do not activate on a target install unless packs validate and required suites pass.

## Consequences

- [BP-009](../../backlog/BP-009-no-in-kernel-language.md) direction becomes this guest model (not declarative-first, not a proprietary in-kernel language).
- Tech stack: guest TS under Deno is **customer implementation**, not a platform Node sidecar ([ADR-005](./005-go-runtime.md) preserved).
- Sync rollback uses transactional write paths in `internal/dataengine` (Create/Update/Delete + sync mutations share one Postgres TX; Phase 3).
- Outbound integrations for async automations share secret-ref / allowlist patterns with [BP-014](../../backlog/BP-014-agent-outbound-integrations.md).

## Explicit non-goals (this ADR)

- Drag-and-drop automation builder as SoT
- npm/package allowlists in v1
- In-process Go plugins / yaegi / importing `internal/*` from customer code
- Python guest runtime
- Declarative field-map DSL as primary authoring (may appear later as a compile aid only)

## Related

- Build plan: [customer-automations-build.md](../architecture/customer-automations-build.md)
- [ADR-004](./004-three-api-families.md) · [ADR-005](./005-go-runtime.md) · [ADR-010](./010-customer-agentic-platform.md) · [ADR-012](./012-customer-repo-and-control-ide.md) · [ADR-029](./029-platform-actions.md)
- [BP-009](../../backlog/BP-009-no-in-kernel-language.md) · [BP-006](../../backlog/BP-006-agent-guardrails.md) · [BP-014](../../backlog/BP-014-agent-outbound-integrations.md) · [BP-061](../../backlog/BP-061-platform-actions.md)
