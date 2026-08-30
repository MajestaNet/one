# Module: `activities`

Optional managed module: activity records — Task, Appointment, PhoneCall, Email. Regarding links use explicit dual lookups (`RegardingAccountId` / `RegardingContactId`); no polymorphic regarding.

Does **not** own apiName `Note` (see [`notes`](./notes.md)). Email is a **record shape**, not a product mailer ([ADR-011](../adr/011-sales-service-managed-modules.md) §10).

Activities are structured CRM **work items** on `flexible` storage. There is no product `messages` / `Message` channel object ([ADR-032](../adr/032-retire-messages-polymorphic-lookup.md)). Do **not** set `storage_mode=high_volume` on Task / Appointment / PhoneCall / Email ([ADR-013](../adr/013-high-volume-flexible-storage.md)).

Operate and Client compose enabled activity objects via **Activity Feed** (`GET /client/v1/activity-feed`). Broader regarding (Case / Opportunity) is a separate follow-up using explicit lookups ([BP-049](../../backlog/BP-049-cdm-managed-packages.md)).

Agent / chat transcripts are **not** activity rows: see [ADR-022](../adr/022-agent-conversations.md) (`/client/v1/agents/conversations`) and kernel `agent_runs`.

## Dependency

- `core` (must be installed)

## Version

`1.1.0` (`ActivitiesPackageVersion` in `internal/seed`).

## Objects

| Object | Extra fields (beyond shared activity set) |
|---|---|
| Task | DueDate, PercentComplete |
| Appointment | Location |
| PhoneCall | PhoneNumber, Direction (Inbound/Outbound) |
| Email | FromAddress, ToAddress, CcAddress |

Shared: Subject (required), Status, Priority, ScheduledStart, ScheduledEnd, Description, RegardingAccountId, RegardingContactId.

Flexible `records` storage; `ownership=managed`, `package_name=activities`.

## Enable

```http
POST /metadata/v1/packages/activities/enable
```
