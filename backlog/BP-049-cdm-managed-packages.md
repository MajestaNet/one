# BP-049: Managed domain packages

- **Severity:** Medium
- **Status:** Partially mitigated
- **Area:** `internal/seed`, `internal/packages`, `docs/modules`, ADR-020

## Problem

Majesta One used CDM only as naming inspiration (`PriceList`) while Account/Contact stayed thin and optional CRM domains (activities, Lead/Campaign, multi-address, Unit) were missing. Customers could not enable curated Microsoft CDM applicationCommon / crmCommon / industry shapes without forking the party model or dumping the full CDM library.

## Why it matters

CRM interop and enableable domain packs need a stable party spine plus optional packages. Shipping full CDM (~50 packs / thousands of entities) would overwhelm the enable catalog and product image; inventing a Party base would break ADR-008/011 join and AuthZ simplicity.

## Mitigation (shipped)

- [ADR-020](../docs/adr/020-cdm-managed-packages.md): curated 1B + industry scope; party model 2A (Account/Contact apiNames + optional AccountId + CDM attributes).
- `core` `2.0.0` Account/Contact CDM attribute parity ([cdm-mapping.md](../docs/architecture/cdm-mapping.md)).
- New optional packages: `address`, `activities`, `lead_marketing`.
- Evolved `catalog` / `sales` / `service` to `2.0.0` (Unit/UnitGroup, Competitor, CDM fields) without duplicate object graphs.
- **Industry packs (v1 curated):** `healthcare`, `financial_services`, `retail`, `sustainability`, `education`, `automotive`, `nonprofit`, `marketing_events`, `portals`, `project_service` — [cdm-industry-packages.md](../docs/architecture/cdm-industry-packages.md).
- ADR-008 / ADR-011 amendments; module docs + agent playbook updates.

## Remaining follow-ups

- Lead → Account/Contact/Opportunity convert — **shipped** as platform action `lead.convert` ([ADR-029](../docs/adr/029-platform-actions.md) · [BP-061](./BP-061-platform-actions.md) Phases 1–4)
- **Activity regarding expansion** (Case / Opportunity via **explicit lookups**, not `polymorphic_lookup`) — product follow-up when non-party parents need work-item timelines. Explicitly **not** an HV promotion of Task/Appointment/PhoneCall/Email ([ADR-013](../docs/adr/013-high-volume-flexible-storage.md)). There is no product Message channel object ([ADR-032](../docs/adr/032-retire-messages-polymorphic-lookup.md)).
- Deeper industry fidelity (more CDM entities per vertical) without apiName collisions
- Dynamics `operationsCommon` (Finance / SupplyChain) packs
- Optional mapping YAML → Go codegen
- Hard uninstall of soft-disabled packages ([BP-007](../docs/adr/020-cdm-managed-packages.md))
- Invoice / Payment (billing v2 — Order/OrderLine shipped on [BP-044](./BP-044-billing-module-order-from-quote.md))

## Activity Feed (shipped with this mitigation slice)

- Activities = `flexible` CRM work items; Client `GET /client/v1/activity-feed` composes them for Account/Contact regarding.
- Optional `messages` / `Message` / `polymorphic_lookup` **retired** ([ADR-032](../docs/adr/032-retire-messages-polymorphic-lookup.md)).

## Related

- BP-007 (package versioning)
- ADR-008 / ADR-011 / ADR-013 / ADR-020 / [ADR-032](../docs/adr/032-retire-messages-polymorphic-lookup.md)
- BP-019 (Operate UI package gating)
- BP-044 (future billing)
- [activities.md](../docs/modules/activities.md)
- [messages.md](../docs/modules/messages.md) (retired stub)
