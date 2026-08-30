# BP-042: Change feed / CDC consumer contract

- **Severity:** High
- **Status:** Open
- **Area:** `internal/worker`, `internal/httpapi`, `internal/dataengine`, `migrations/`
- **Design:** remainder: [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md) · [api-families.md](../docs/api-families.md) (Events), [ADR-009](../docs/adr/009-record-audit-authz-packaging.md)
- **Identified:** Headless 360 backlog review (2026-07) — outbox ≠ CDC product surface

## Problem

Majesta One ships **outbox → webhook delivery** and `GET /events` for operators. That is not a first-class **change data capture (CDC)** contract for integrators:

- No stable, versioned **change envelope** per record (create / update / delete with field-level diff policy)
- No **durable subscription** or replay cursor for external consumers (warehouse, search index, audit lake)
- No parity with CRM change-data-capture / event-bus subscribe patterns headless teams expect

Poll-and-ack outbox works for internal webhooks but does not document field masks, ordering guarantees per object, or consumer offset recovery after downtime.

## Why it matters

- Real-time 360 and analytics pipelines need “what changed on Account/Contact/Case” without full-table polling
- Search indexers ([BP-043](./BP-043-cross-object-search-api.md)) and external CDPs need a supported ingest hook
- Reduces integration fragility vs customer-authored triggers-only approaches ([BP-009](./BP-009-no-in-kernel-language.md))

## Scope (target)

1. **Change event model:** `record.created` | `record.updated` | `record.deleted` with object apiName, record id, actor, timestamp; optional changed-field list (respect FLS — never leak unreadable fields)
2. **Client API:** consumer-oriented read/subscribe surface (poll with cursor or long-poll v1; not SSE requirement)
3. **Delivery:** extend outbox/webhook pipeline with CDC-shaped payloads; idempotent delivery ([BP-005](../docs/architecture/agent-worker.md))
4. **Metadata:** opt-in per object or global install policy; retention window documented
5. **Docs:** contrast with internal outbox types vs integrator CDC contract

## Depends on / pairs with

- [BP-005](../docs/architecture/agent-worker.md) — delivery semantics
- [BP-003](./BP-003-enterprise-auth.md) — FLS on change payloads
- [BP-041](./BP-041-record-external-id-upsert-bulk.md) — consumers often upsert by external id
- [BP-043](./BP-043-cross-object-search-api.md) — search index refresh

## Explicit non-goals

- Third-party event-bus wire protocol clone
- Multi-tenant shared streaming bus (dedicated install (one customer per database) — ADR-001)
- Guaranteed sub-second latency SLA (best-effort async)
- Publishing arbitrary customer-defined event types in v1 (record CDC first)

## Related

- Remainder (slot 11): [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md)
- [BP-038](./BP-038-no-product-mailer-byo-alerts.md) — system alerts use outbox; different audience
- [customer-connect.md](../docs/customer-connect.md)
