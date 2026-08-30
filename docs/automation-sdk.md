# Customer automation SDK (frozen v1)

Guest TypeScript surface for sandboxed automations ([ADR-014](./adr/014-customer-code-automations.md)).  
Runtime: Deno default-deny; only `one:automation` imports are allowed.

## Programming model

```ts
import type { AutomationContext, AutomationResult } from "one:automation";

export default async function run(ctx: AutomationContext): Promise<AutomationResult> {
  const { id } = await ctx.createRecord({
    objectApiName: "Opportunity",
    data: { Name: String(ctx.trigger.data?.Name ?? ""), Amount: ctx.trigger.data?.Amount },
  });
  ctx.log("created", id);
  return { ok: true };
}
```

## Frozen methods

| Method | Sync | Async | Notes |
|---|---|---|---|
| `getRecord` | yes | yes | AuthZ as run-as |
| `createRecord` | yes | yes | Nested sync capped (depth/ops) |
| `updateRecord` | yes | yes | |
| `deleteRecord` | limited | yes | Sync may reject until fully wired |
| `query` | limited | yes | Prefer filters; AuthZ read |
| `log` | yes | yes | Captured on the host |
| `invokeAction` | **syncSafe only in sync** | yes | Product [platform actions](./adr/029-platform-actions.md) (`lead.convert`, …). One frozen method — not per-verb `ctx.convertLead`. Package-gated; AuthZ as run-as. |
| `http` | **no** | yes | Host HTTPS; secretRef / url; egress allowlist ([BP-014](../backlog/BP-014-agent-outbound-integrations.md)) |
| `connector` | **no** | yes | Resolves install connector + secret |

**Async outbound (BP-014 / ADR-014 Phase 7):** `http` and `connector` are **async-only** host RPCs (Deno stays deny-net). See [outbound-otel-build-plan.md](./architecture/outbound-otel-build-plan.md). npm/JSR/std imports remain banned.

**Fence vs CRM Canvas:** guest automations must **not** import Control IDE canvas UI, `@one/client`, or React. Canvas is an Operate/IDE concern ([ADR-018](./adr/018-crm-canvas-document.md)); automations stay on `one:automation` / ambient `ctx` only.

**Platform actions:** `ctx.invokeAction({ apiName, input })` is the only guest path into product Go verbs. HTTP from Deno to `/client/v1/actions` is forbidden. See [platform-actions-build-plan.md](./architecture/platform-actions-build-plan.md).

## Unit tests (`tests/automations/**`)

Deploy step `automationUnitPass` runs the file under the unit harness with a mock host:

```ts
import type { AutomationUnitContext } from "one:automation";

export default async function run(ctx: AutomationUnitContext) {
  await ctx.runUnderTest({
    trigger: {
      action: "create",
      objectApiName: "Account",
      recordId: "acc-1",
      data: { Name: "Acme", Amount: 100 },
    },
  });
  const { calls } = await ctx.getCalls({ method: "createRecord" });
  if (calls.length !== 1) throw new Error("expected one create");
  return { ok: true };
}
```

Helpers: `runUnderTest`, `getCalls`, `clearCalls` (unit harness only).

## Vendor type stubs

Optional IDE/editor stubs (not shipped to customers as npm): `tools/automation-sdk/`.

## Deploy test steps

| Type | Purpose |
|---|---|
| `automationUnitPass` | `testFile` + `automationApiName` → Deno unit harness |
| `automationContract` | Fixture create → invoke automation → `expectObjectApiName` + filters |

Promote gate: set `DEPLOY_REQUIRED_TEST_SUITES=CreateAccountFromContact` (comma-separated) so non-dry-run promote requires those suites green.
