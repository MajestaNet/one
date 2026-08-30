# ADR-032: Retire Messages module and polymorphic lookups

## Status

Accepted

## Context

Pre-GA productization asked whether two surfaces should stay in the install:

1. **`polymorphic_lookup`** — a first-class field type whose only product consumer was `Message.ParentId` / `ParentType`.
2. **Optional managed module `messages`** — a high-volume `Message` object positioned as a CRM channel / timeline SoR, composed with `activities` via `GET /client/v1/activity-feed`.

Those choices conflicted with locked decisions elsewhere:

- [ADR-020](./020-cdm-managed-packages.md) / [ADR-003](./003-sql-query-engine.md) reject a polymorphic party / `parentCustomer` column for join and AuthZ simplicity. Optional `activities` already use **explicit dual lookups** (`RegardingAccountId` / `RegardingContactId`), not polymorphic regarding.
- [ADR-022](./022-agent-conversations.md) already stores Control / hosted agent chat on kernel `agent_conversations` + `agent_conversation_messages`. Execution audit is kernel `agent_runs`. Mutation audit is kernel `audit_log`. Planned customer-visible debug logs are `ExecutionRun` / `ExecutionLogEntry` ([BP-033](../../backlog/BP-033-customer-runtime-isolation.md)) — not a CRM Message row.
- There is no in-kernel mailer or CTI ([ADR-011](./011-sales-service-managed-modules.md) §10 / [BP-038](../../backlog/BP-038-no-product-mailer-byo-alerts.md)). `Channel=Chat` on Message was a CRM classification, not agent transcripts — but the overlap confused builders.
- Shipping `polymorphic_lookup` on the Metadata allowlist would freeze a hard-to-remove customer contract for a type the product itself does not use.

High-volume **storage** ([ADR-013](./013-high-volume-flexible-storage.md) `records_hv` + `storage_mode=high_volume`) remains valid. Message was only the first *example* object, not the reason the ladder exists.

## Decision

1. **Remove** optional managed package `messages` and object `Message` from the product catalog. Kernel migration `0060` deletes leftover Message metadata/rows and drops `records_hv_message` RANGE partitions.
2. **Remove** `polymorphic_lookup` from the canonical field-type catalog ([ADR-017](./017-canonical-field-types.md)). Metadata create rejects it. Parent relationships stay typed `lookup` / `master_detail` with `referenceTo`.
3. **Do not** store agent or chat transcripts as business records. Agent threads stay on `/client/v1/agents/conversations` (ADR-022). Hosted tool-loop audit stays on `agent_runs`. Do not revive a CRM Message object as an “audit trail of all agent / chat interactions.”
4. **Keep** `storage_mode=high_volume` / `records_hv`, query guardrails, and partition roll. The next product HV consumer is planned `ExecutionLogEntry` (BP-033), not a channel object.
5. **Keep** optional `activities` and `GET /client/v1/activity-feed` as a **work-item** composition (Task / Appointment / PhoneCall / Email) for Account/Contact regarding. The feed is no longer a Message + Activity two-plane.

## Consequences

- Installs that enabled `messages` lose Message describe/CRUD after image upgrade (destructive, same class as migration `0012`).
- Activity Feed no longer logs channel crumbs; Email/PhoneCall activity records remain the structured shapes for those work items.
- Control IDE Operate compose-Message UI is deleted in lockstep so the optional client does not call a removed object ([BP-065](../../backlog/BP-065-ide-backend-coupling.md)).
- Broader activity regarding (Case / Opportunity) remains an explicit-lookup follow-up on [BP-049](../../backlog/BP-049-cdm-managed-packages.md) — still not polymorphic.

## Related

- [ADR-013](./013-high-volume-flexible-storage.md) · [ADR-017](./017-canonical-field-types.md) · [ADR-020](./020-cdm-managed-packages.md) · [ADR-022](./022-agent-conversations.md)
- [modules/messages.md](../modules/messages.md) (retired stub) · [modules/activities.md](../modules/activities.md)
- [BP-049](../../backlog/BP-049-cdm-managed-packages.md) · [BP-001](../../backlog/BP-001-jsonb-query-scale.md)
