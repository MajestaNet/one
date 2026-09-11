# Module: `lead_marketing`

Optional managed module: **Lead**, **Campaign**, and **MarketingList** (+ member). Lead is **not** part of `sales` or always-on `core` ([ADR-011](../adr/011-sales-service-managed-modules.md) amended by [ADR-020](../adr/020-cdm-managed-packages.md)).

Lead → Account/Contact/Opportunity convert is platform action **`lead.convert`** ([ADR-029](../adr/029-platform-actions.md)): product Go on the Client catalog, callable from customer TypeScript via `ctx.invokeAction`. The action is **unavailable** until this package is enabled; `createOpportunity` additionally requires `sales`.

## Dependency

- `core` (must be installed)

## Version

`1.3.0` (`LeadMarketingPackageVersion` in `internal/seed`). Adds managed automation `Lead_ConvertOnConvertedStatus` (process wrap around `lead.convert`).

## Objects

| Object | Key fields | Notes |
|---|---|---|
| Lead | LastName (required), FirstName, Company, Email, Phone, Status, Source, Description, AccountId, ContactId | Prospect prior to Opportunity |
| Campaign | Name (required), Status, Type, StartDate, EndDate, Description | Marketing campaign container |
| MarketingList | Name (required), Type (Static/Dynamic), MemberType (Account/Contact/Lead), Description | Audience list |
| MarketingListMember | MarketingListId (required), AccountId, ContactId, LeadId | Member row; use fields matching MemberType |

Flexible `records` storage; `ownership=managed`, `package_name=lead_marketing`.

## Platform actions

| apiName | Requires | Optional | Sync | Notes |
|---|---|---|---|---|
| `lead.convert` | `lead_marketing` | `sales` (`createOpportunity`) | yes | Client `POST /client/v1/actions/lead.convert`; guest `ctx.invokeAction`. Shipped (ADR-029 / BP-061). |

## Package automations (shipped — [ADR-033](../adr/033-managed-package-automations.md))

Seeded on enable as `ownership=managed`, default **on**. Disable with `PATCH /metadata/v1/automations/{apiName}` `{ "active": false }` (`metadata.build` + admin). Soft-disable of this pack skips dispatch even if `active` is still true.

| apiName | Trigger | Execution | Description |
|---|---|---|---|
| `Lead_ConvertOnConvertedStatus` | Lead update | sync | When Lead.Status becomes Converted, runs `lead.convert` in the same transaction. Does not copy customer custom fields. |

## Enable

```http
POST /metadata/v1/packages/lead_marketing/enable
```
