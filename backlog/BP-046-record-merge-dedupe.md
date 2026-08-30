# BP-046: Record merge and duplicate detection

- **Severity:** Medium
- **Status:** Open
- **Area:** `internal/dataengine`, `internal/httpapi`, `internal/metadata`, `migrations/`
- **Design:** remainder: [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md) · [data-model.md](../docs/data-model.md), [ADR-009](../docs/adr/009-record-audit-authz-packaging.md) · [ADR-029](../docs/adr/029-platform-actions.md)
- **Identified:** Headless 360 backlog review (2026-07) — mentioned in BP-020 as optional; no product API

## Problem

Customer 360 implementations require **deduplication and merge** — combine duplicate Accounts/Contacts, reparent lookups, and preserve audit history. Majesta One has:

- No merge API
- No duplicate-detection job or “possible dupes” hint surface
- [BP-020](./BP-043-cross-object-search-api.md) lists merge as “optional later” only after record UX

Headless integrators running MDM or data-quality tools expect a governed merge primitive, not ad-hacent DELETE + manual relink.

## Why it matters

- Party spine ([data-model.md](../docs/data-model.md)) is Account + Contact only — dupes directly harm 360 trust
- Ingest without [BP-041](./BP-041-record-external-id-upsert-bulk.md) upsert increases duplicate volume
- Agents and automations should not implement unsafe merge logic in Deno ([BP-009](./BP-009-no-in-kernel-language.md))

## Scope (target)

1. **Client API:** `POST /client/v1/actions/record.merge` — register on `core` in the [platform actions catalog](../docs/adr/029-platform-actions.md). **Do not** add a sibling `/merge` route.
2. **DataEngine:** transactional reparent of lookups (Contact.AccountId, Opportunity parties, Case.AccountId, etc.); hard-delete duplicate per product policy (no recycle bin)
3. **AuthZ:** `delete` + `update` on involved objects; sharing checks on both sides ([BP-003](./BP-003-enterprise-auth.md))
4. **Audit:** merge event in audit log / optional [BP-042](./BP-042-change-feed-cdc-consumer.md) envelope
5. **v1 duplicate hints (optional):** batch job or query API flagging candidates by email/phone/name — thin, not full MDM

## Depends on / pairs with

- [BP-020](./BP-043-cross-object-search-api.md) — Operate “flag dupes” consumes hints API
- [BP-041](./BP-041-record-external-id-upsert-bulk.md) — prevention vs cure
- [BP-043](./BP-043-cross-object-search-api.md) — find candidates
- [BP-018](../docs/adr/030-install-agent-runtime.md) — merge UX in IDE after API exists

## Explicit non-goals

- Person Accounts or Household merge graphs ([ADR-011](../docs/adr/011-sales-service-managed-modules.md))
- Incumbent CRM duplicate-set parity
- Auto-merge without explicit principal action in v1
- Cross-object merge (Account + Contact into one) — master must be same object type

## Related

- Remainder (slot 11): [11-bp-041-046-061-headless-client.md](../docs/architecture/agentic-remainders/11-bp-041-046-061-headless-client.md)
- [BP-020](./BP-043-cross-object-search-api.md) · [customer-ide-ux.md](../docs/customer-ide-ux.md)
- [BP-061](./BP-061-platform-actions.md) · [ADR-029](../docs/adr/029-platform-actions.md) — merge **must** register as `record.merge` on the actions catalog
