# BP-069: Managed package automations (toggle + description + IDE consume)

- **Severity:** Medium
- **Status:** Mitigated
- **Area:** `internal/packages`, `internal/seed`, `internal/metadata`, `internal/httpapi`, `internal/dataengine`, `internal/automation`, `internal/deploy`, `migrations/`, `tools/control-ide` (Phase 6 consume only)
- **Design:** [ADR-033](../docs/adr/033-managed-package-automations.md) · [package-automations-build-plan.md](../docs/architecture/package-automations-build-plan.md)

## Problem

Managed packages ship objects/fields and platform actions, not **default process**. Customers who enable `sales` or `lead_marketing` must hand-write automations to call `lead.convert` / `quote.accept`. There is no install-level way to turn product process on or off via Metadata, no persisted functional description on automations, and the Control IDE Automations tool only understands customer-owned files.

Cloning starters to `ownership=custom` would put package process in customer Git, skip product upgrades of the wrap, and confuse Deploy. PATCH of managed automations is 403 today (`AssertCustomerMutable`).

## Why it matters

- SI installs expect package process **on** at enable, with a documented off switch
- Headless and IDE builders need the **same** Metadata toggle (install is the SoR)
- Descriptions make the Automations list operable without reading TypeScript
- Integrity verbs stay Go ([ADR-029](../docs/adr/029-platform-actions.md)); wraps stay guest TS that call `invokeAction`

## Direction (locked)

Per **ADR-033**:

1. `Module.Automations` seeded as `ownership=managed` on package enable
2. Insert `active=true`; upgrades preserve `active`
3. `PATCH /metadata/v1/automations/{apiName}` `{ "active" }` is the on/off switch (`metadata.build` + admin)
4. `description` ≤500 chars on every automation row
5. Dispatch skips inactive and skip when the owning pack is soft-disabled
6. Control IDE uplifts **existing** Automations + Packages panels only

## Implementation phases

See [package-automations-build-plan.md](../docs/architecture/package-automations-build-plan.md).

| Phase | Status |
|---|---|
| 0 Docs / ADR-033 | Done |
| 1 Description column + GET-by-name (custom) | Done |
| 2 Registry + SyncAutomationManaged | Done |
| 3 PATCH managed `active` + package catalog | Done |
| 4 Dispatch gates + proving wraps (`Lead_ConvertOnConvertedStatus`, `Quote_AcceptOnStatusAccepted`) | Done |
| 5 Deploy reject managed automations | Done |
| 6 Control IDE Automations + Packages consume | Done |
| 7 Module docs + MCP honesty | Done (`patch_automation` maps to Metadata PATCH) |

## Explicit non-goals

- Clone-on-enable of package automations
- Customer-editable managed source
- Locked TS as the integrity verb (convert/accept stay Go)
- New IDE tiles/modes
- Auto-map `__c` on convert/accept

## Related

- [ADR-014](../docs/adr/014-customer-code-automations.md) · [BP-009](./BP-009-no-in-kernel-language.md)
- [ADR-029](../docs/adr/029-platform-actions.md) · [BP-061](./BP-061-platform-actions.md)
- [ADR-030](../docs/adr/030-install-agent-runtime.md) · [BP-066](./BP-066-ide-demo-client-fidelity.md) (honest JWT consume)
- [customer-customizations.md](../docs/customer-customizations.md)
