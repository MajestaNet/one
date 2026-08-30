# ADR-029: Platform actions (package-gated Client verbs)

## Status

Accepted (plan locked; implementation phased — see [platform-actions-build-plan.md](../architecture/platform-actions-build-plan.md))

## Context

Managed packages today ship **data model** (objects/fields) with `ownership=managed`. Customer **code automations** are TypeScript in the customer repo ([ADR-014](./014-customer-code-automations.md)). That split is correct: CRM process varies per install, and product ≠ customer implementation.

The remaining gap is **integrity verbs** that must not be reimplemented in guest TypeScript: Lead convert, record merge, later Quote accept / Order-from-Quote. Customers still need to **call** those verbs from custom automations (copy customer fields, branch on stage, wrap convert). Shipping each verb as a one-off Client route (`POST /convertLead`, `POST /merge`, …) does not scale. Shipping them as locked guest TS inside a managed package fights ADR-014 (automations are customer-owned) and Deploy reject-managed rules.

BP-049 already listed Lead convert as a follow-up ADR. This record is that ADR, generalized so convert is the first **platform action**, not a one-off.

## Decision

### 1. Nouns (do not conflate)

| Noun | Author | Lives where | How callers invoke |
|---|---|---|---|
| **Platform action** | Product Go | Image registry (`internal/packages` + `internal/actions`) | `POST /client/v1/actions/{apiName}` and guest `ctx.invokeAction` |
| **Customer automation** | Customer TypeScript | Install DB + customer repo | Triggers, `POST /client/v1/automations/{apiName}/runs` ([BP-047](../../backlog/BP-047-integrations-callable-oauth.md)) |
| **AgentSpec** | Customer (cloned starters) | `agent_playbooks` | `POST /client/v1/agents/runs` ([ADR-010](./010-customer-agentic-platform.md)) |

Platform actions are **not** Metadata automations. They are not Deploy-promoted. They version with the product image. Customers wrap them; they do not fork them.

Starter clone templates (the `agents_starter` pattern) remain valid for **example customer automations**. They are not a substitute for integrity verbs.

### 2. One Client surface that scales

Do **not** add a new HTTP family. Do **not** add a dedicated route per verb.

| Method | Path | Role |
|---|---|---|
| `GET` | `/client/v1/actions` | Invocable catalog for this principal on this install (package-gated) |
| `GET` | `/client/v1/actions/{apiName}` | Describe one action (input/output schema + package gates) |
| `POST` | `/client/v1/actions/{apiName}` | Invoke (sync 200 or async 202) |

Scope: `client`. Family ownership stays ADR-004 (data ops → Client). Additive inside `/client/v1`; pin via [ADR-025](./025-api-revision-versioning.md) only if invoke behavior later diverges for pinned clients.

Metadata `GET /packages` / `GET /packages/{name}` lists **declared** actions on the module (what enabling the pack unlocks), including when the pack is disabled. Client `GET /actions` lists only actions whose **required** packages are enabled.

### 3. Registry is compile-time Go, gated at runtime by `package_installs`

Each managed module may declare `Actions []ActionDef` next to `Objects` (same image catalog as [ADR-020](./020-cdm-managed-packages.md)).

```text
packages.Register(Module{
  Name: "lead_marketing",
  Objects: [Lead, Campaign, …],
  Actions: [{ APIName: "lead.convert", RequiresPackages: ["lead_marketing"], OptionalPackages: ["sales"], SyncSafe: true }],
})
```

- **No kernel table** for action definitions. Availability is `package_installs.enabled` for every name in `RequiresPackages`.
- Soft-disable of a required package **hides** the action from the Client catalog and fails invoke with `409 PACKAGE_NOT_ENABLED`.
- Optional packages gate **options**, not the action itself (`createOpportunity: true` requires `sales`).
- Adding a future verb is: Go handler + `ActionDef` on the owning module + tests + module doc. No new mux route.

`apiName` is a stable dotted `noun.verb` (`lead.convert`, `record.merge`, `quote.accept`). It does **not** include the package name. Platform actions never use the customer `__c` suffix.

### 4. Customer TypeScript calls actions through one frozen SDK method

Amend [ADR-014](./014-customer-code-automations.md) §7: add **one** host RPC, `invokeAction`. Do **not** add `ctx.convertLead` (or any per-verb method). New product verbs must not require an SDK surface change.

```ts
export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const converted = await ctx.invokeAction({
    apiName: "lead.convert",
    input: { leadId: ctx.trigger.recordId, createOpportunity: true },
  });
  // customer-owned mapping after the integrity verb
  await ctx.updateRecord({
    objectApiName: "Account",
    recordId: String(converted.accountId),
    data: { Region__c: ctx.trigger.data?.Region__c },
  });
  return { ok: true, data: converted };
}
```

- Run-as = automation starter principal (ADR-014). The action uses that principal’s object/FLS/sharing. No hidden elevation.
- Sync automations may call only `syncSafe` actions; those share the triggering Postgres transaction (fail → full rollback). Async-only actions are rejected in sync, same as `ctx.http` / `ctx.connector`.
- Unit harness mocks `invokeAction` like other SDK methods. Guests still cannot import npm or call Client HTTP.

### 5. AuthZ floor is object CRUD, not a parallel action ACL (v1)

Invoke requires:

1. Scope `client`.
2. Every **required** package enabled; optional packages enabled when the request uses their options.
3. Caller / run-as permission-set grants (object + field + sharing) on **every record the action reads or writes**.

v1 does **not** add `actionAccess` on permission sets. Revisit only if a verb cannot be expressed as object CRUD (then a follow-up ADR, mirroring `automationAccess`). Admin still bypasses object stubs the same way as other Client writes.

### 6. First verb: `lead.convert`

Owning package: `lead_marketing`. Required packages: `lead_marketing` (implies `core`). Optional: `sales` when `createOpportunity` is true.

| Input | Rules |
|---|---|
| `leadId` | Required. Lead must exist and be readable. |
| `accountId` | Optional existing Account. Else create Account; `Company` or a derived name is required. |
| `contactId` | Optional existing Contact. Else create Contact from Lead name fields (`LastName` required on Lead). |
| `createOpportunity` | Default `false`. `true` requires `sales` enabled; creates Opportunity (`StageName=Prospecting`, `CloseDate` = today + 30d unless provided) with Account and/or Contact. |
| `opportunityName` / `opportunityCloseDate` | Optional; ignored unless `createOpportunity`. |
| `convertedStatus` | Optional; v1 only `Converted` (already on Lead.Status picklist). |

Output: `{ leadId, accountId, contactId, opportunityId?, alreadyConverted }`. If Lead is already `Converted` with AccountId+ContactId, return existing ids with `alreadyConverted: true` (no second Account/Contact/Opportunity). Do not auto-copy customer custom fields — that is the wrapping automation’s job.

Convert is **sync** and **syncSafe**. It uses DataEngine create/update in one transaction (same path as ADR-014 sync automations). It does not send email ([ADR-011](./011-sales-service-managed-modules.md) §10).

### 7. Reserved catalog (claim names now; implement when the owning BP lands)

| apiName | Owning package | Requires | Optional | Track |
|---|---|---|---|---|
| `lead.convert` | `lead_marketing` | `lead_marketing` | `sales` (`createOpportunity`) | this ADR, Phase 2 |
| `record.merge` | `core` | `core` | — | [BP-046](../../backlog/BP-046-record-merge-dedupe.md) |
| `quote.accept` | `sales` | `sales`, `catalog` | `billing` (`createOrder`) | [BP-044](../../backlog/BP-044-billing-module-order-from-quote.md) / [ADR-031](./031-billing-managed-module.md) — implemented |

Do not register pricing engines, CPQ rules, or customer process as platform actions. Those stay customer automations or future `cpq` objects ([ADR-011](./011-sales-service-managed-modules.md) thin catalog).

## Consequences

- Client integrations, Control IDE, MCP (later), and guest automations share one Go implementation per verb.
- Enabling `lead_marketing` unlocks convert; enabling `sales` unlocks the Opportunity option; disabling either fails closed.
- ADR-011 / ADR-020 “convert is a follow-up” is this record. `sales` still does not own Lead or convert.
- ADR-014 SDK freeze grows by exactly one method (`invokeAction`). Per-verb TS helpers remain forbidden.
- [BP-061](../../backlog/BP-061-platform-actions.md) tracks implementation. Merge and Quote-accept **must** register on this catalog rather than inventing sibling routes.

## Explicit non-goals (this ADR)

- Customer-defined platform actions / in-kernel plugins
- Per-verb Client routes or per-verb `ctx` methods
- Permission-set `actionAccess` catalog (v1)
- Drag-and-drop / prompt-as-runtime for integrity verbs
- Managed locked TypeScript automations inside packages
- Auto-mapping customer custom fields on convert
- Person Accounts, Lead in `sales` / `core`, product mailer
- Composite action subrequests and MCP `invoke_action` in v1 (follow-ons)

## Related

- Build plan: [platform-actions-build-plan.md](../architecture/platform-actions-build-plan.md)
- [ADR-004](./004-three-api-families.md) · [ADR-011](./011-sales-service-managed-modules.md) · [ADR-014](./014-customer-code-automations.md) · [ADR-020](./020-cdm-managed-packages.md) · [ADR-025](./025-api-revision-versioning.md)
- [BP-049](../../backlog/BP-049-cdm-managed-packages.md) · [BP-046](../../backlog/BP-046-record-merge-dedupe.md) · [BP-044](../../backlog/BP-044-billing-module-order-from-quote.md) · [BP-009](../../backlog/BP-009-no-in-kernel-language.md) · [BP-047](../../backlog/BP-047-integrations-callable-oauth.md)
