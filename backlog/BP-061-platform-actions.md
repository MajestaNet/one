# BP-061: Platform actions (package-gated Client verbs)

- **Severity:** High
- **Status:** Partially mitigated (Phases 1–4 shipped: catalog, `lead.convert`, guest `invokeAction`; Phase 5 `quote.accept` shipped on BP-044; `record.merge` remains)
- **Area:** `internal/actions` (planned), `internal/packages`, `internal/httpapi`, `internal/dataengine`, `internal/automation`, `internal/seed`, `internal/authz`
- **Design:** [ADR-029](../docs/adr/029-platform-actions.md) · [platform-actions-build-plan.md](../docs/architecture/platform-actions-build-plan.md) · remainder (`record.merge`): [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md)

## Problem

Managed packages ship objects/fields only. Customer automations are guest TypeScript ([ADR-014](../docs/adr/014-customer-code-automations.md)). Integrity verbs (Lead convert, record merge, later Quote accept) must be **product Go** — guests must not reimplement multi-object tx, and locked package TS must not become a second process SKU.

Without a **single Client catalog**, each verb becomes a one-off route (`POST /convertLead`, `POST /merge`) that does not scale, is hard to package-gate, and is awkward to call from customer TS (HTTP from Deno is banned; per-verb `ctx` methods explode the frozen SDK).

BP-049 listed Lead convert as a follow-up ADR. That ADR is now [ADR-029](../docs/adr/029-platform-actions.md); convert is the first registered action, not a special snowflake.

## Why it matters

- Headless 360 and customer automations both need convert/merge as **the same** implementation
- `lead.convert` must be unusable until `lead_marketing` is enabled; Opportunity creation until `sales` is enabled
- Guest `ctx.invokeAction` is the scalable TS bridge (one frozen SDK method)
- BP-046 / BP-044 must not invent sibling APIs

## Direction (locked)

Per **ADR-029**:

1. Client `GET/POST /client/v1/actions/{apiName}` — no per-verb mux routes
2. Compile-time `ActionDef` on managed modules; runtime gate = `package_installs.enabled`
3. Customer TS: `ctx.invokeAction({ apiName, input })` only
4. AuthZ v1 = object/FLS/sharing of caller or run-as; no PS `actionAccess` yet
5. First verb: `lead.convert` (sync, syncSafe)
6. Reserved names: `record.merge` (BP-046), `quote.accept` (BP-044)

## Implementation phases

See [platform-actions-build-plan.md](../docs/architecture/platform-actions-build-plan.md).

| Phase | Status |
|---|---|
| 0 Docs / ADR-029 | Done |
| 1 Registry + Client catalog/invoke shell | Done |
| 2 `lead.convert` | Done |
| 3 Guest `invokeAction` | Done |
| 4 Package docs / Metadata catalog polish | Done |
| 5 Follow-on verbs via BP-046 / BP-044 | `quote.accept` Done (BP-044); `record.merge` Pending (BP-046) |
| 6 Optional PS `actionAccess` (new ADR only if needed) | Not started |

## Explicit non-goals

- Managed locked TypeScript automations in packages
- Prompt templates as convert runtime
- Per-verb Client routes or per-verb `ctx` methods
- Customer-defined platform actions
- Product mailer on convert

## Related

- Remainder (`record.merge`, slot 11): [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md)
- [ADR-029](../docs/adr/029-platform-actions.md)
- [ADR-014](../docs/adr/014-customer-code-automations.md) · [BP-009](./BP-009-no-in-kernel-language.md)
- [ADR-011](../docs/adr/011-sales-service-managed-modules.md) · [ADR-020](../docs/adr/020-cdm-managed-packages.md) · [BP-049](./BP-049-cdm-managed-packages.md)
- [BP-046](./BP-046-record-merge-dedupe.md) · [BP-044](./BP-044-billing-module-order-from-quote.md)
- [BP-047](./BP-047-integrations-callable-oauth.md) (customer automation invoke — different noun)
- [BP-006](./BP-006-agent-guardrails.md) (later MCP `invoke_action`)
